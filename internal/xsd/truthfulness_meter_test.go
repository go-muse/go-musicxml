package xsd

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestTruthfulnessNewScansMeterWorkAndCancellation(t *testing.T) {
	cases := []struct {
		name string
		scan func(*Context)
	}{
		{"union-pattern", func(c *Context) {
			union := &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Name: QName{Space: XSDNamespace, Local: "QName"}}, {Name: QName{Space: XSDNamespace, Local: "string"}}}}
			schema := &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Inline: union}, Patterns: []string{"allowed"}}
			if rejectUnionPattern(schema, " "+strings.Repeat("n", 2048)+"\t", "display", c) == nil {
				t.Fatal("lost normalization-independent rejection")
			}
		}},
		{"base64", func(c *Context) {
			if n, ok := binaryLexicalLength("base64Binary", strings.Repeat("Y W J j ", 512), c); !ok || n != 1536 {
				t.Fatal("base64 result")
			}
		}},
		{"hex", func(c *Context) {
			if n, ok := binaryLexicalLength("hexBinary", strings.Repeat("aB", 1024), c); !ok || n != 1024 {
				t.Fatal("hex result")
			}
		}},
		{"float", func(c *Context) {
			if !scalarPatternMatch(floatPattern, strings.Repeat("1", 2048)+"e-2", c) {
				t.Fatal("float result")
			}
		}},
		{"wildcard", func(c *Context) {
			if got := wildcardNamespace(strings.Repeat("urn:one ", 256)+"##local", "", c); !got.allowed || !got.known || !got.valid {
				t.Fatal("wildcard result")
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseline := &scalarMeter{}
			tc.scan(scalarTestContext(baseline, context.Background()))
			exact := &scalarMeter{limit: baseline.steps}
			tc.scan(scalarTestContext(exact, context.Background()))
			cause := errors.New("work stopped")
			short := &scalarMeter{limit: baseline.steps - 1, cause: cause}
			scalarAbort(t, cause, func() { tc.scan(scalarTestContext(short, context.Background())) })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			interrupted := &scalarMeter{cancel: cancel, cancelAt: 3}
			scalarAbort(t, context.Canceled, func() { tc.scan(scalarTestContext(interrupted, ctx)) })
		})
	}
}

func TestTruthfulnessCapabilityAbortKeepsOriginalCause(t *testing.T) {
	// Exercise the exact guard boundary, not an earlier normalization failure.
	for _, builtin := range []string{"float", "date"} {
		s := truthfulnessRestriction(builtin)
		s.HasMinInclusive, s.MinInclusive = true, "0"
		cause := errors.New("guard resource stopped")
		m := &scalarMeter{limit: 1, cause: cause}
		c := scalarTestContext(m, context.Background())
		c.local = &boundState{}
		scalarAbort(t, cause, func() { validateBoundsValue(s, "1", builtin, "1", c) })
		if c.AssessmentErr != nil || c.AssessmentVersion != 0 {
			t.Fatalf("%s interruption became capability outcome: %v", builtin, c.AssessmentErr)
		}
	}
}

func TestTruthfulnessQNameGuardEveryCancellationCleansUp(t *testing.T) {
	s := truthfulnessRestriction("QName")
	s.Patterns = []string{"allowed"}
	outer := &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Inline: s}}
	c, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "n"}: {Name: QName{Local: "n"}, Type: TypeRef{InlineSimple: outer}}}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	u, _ := c.Root(QName{Local: "n"})
	assess := func(cancelAt int64) int64 {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		meter := &scalarMeter{cancel: cancel, cancelAt: cancelAt}
		r, err := NewRun(c, meter)
		if err != nil {
			t.Fatal(err)
		}
		_, err = r.Bind(ctx, u, Subject{ID: 1, Name: QName{Local: "n"}, Text: "allowed", TextKnown: true, AttributesComplete: true, ChildrenComplete: true})
		if cancelAt == 0 && err != nil {
			t.Fatal(err)
		}
		if cancelAt != 0 && !errors.Is(err, context.Canceled) {
			t.Fatalf("checkpoint %d: %v", cancelAt, err)
		}
		if r.context.local != nil {
			t.Fatalf("checkpoint %d leaked local binding scope", cancelAt)
		}
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
		if meter.retained != 0 {
			t.Fatalf("checkpoint %d retained %d after Close", cancelAt, meter.retained)
		}
		return meter.calls
	}
	checkpoints := assess(0)
	for checkpoint := int64(1); checkpoint <= checkpoints; checkpoint++ {
		assess(checkpoint)
	}
}

func TestTruthfulnessGuardedListPrecheckIsBounded(t *testing.T) {
	integer := truthfulnessRestriction("integer")
	integer.HasMinInclusive, integer.MinInclusive = true, "0"
	item := &SimpleMember{Inline: &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Name: QName{Space: XSDNamespace, Local: "QName"}}, {Inline: integer}}}}
	text := strings.Repeat("p:name ", 32) + "-1"
	scan := func(c *Context) {
		if c.precheckListLexical(item, text) == nil {
			t.Fatal("lost exact later failure")
		}
	}
	baseline := &scalarMeter{}
	scan(scalarTestContext(baseline, context.Background()))
	exact := &scalarMeter{limit: baseline.steps}
	scan(scalarTestContext(exact, context.Background()))
	cause := errors.New("list precheck exhausted")
	short := &scalarMeter{limit: baseline.steps - 1, cause: cause}
	scalarAbort(t, cause, func() { scan(scalarTestContext(short, context.Background())) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	interrupted := &scalarMeter{cancel: cancel, cancelAt: 25}
	scalarAbort(t, context.Canceled, func() { scan(scalarTestContext(interrupted, ctx)) })
}

func TestTruthfulnessRestrictedUnionListEveryCancellationCleansUp(t *testing.T) {
	union := &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Name: QName{Space: XSDNamespace, Local: "QName"}}, {Name: QName{Space: XSDNamespace, Local: "integer"}}}}
	item := &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Inline: union}, Enumerations: []string{"0"}}
	list := &SimpleSchema{Form: SimpleList, Item: &SimpleMember{Inline: item}}
	c, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "n"}: {Name: QName{Local: "n"}, Type: TypeRef{InlineSimple: list}}}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	u, _ := c.Root(QName{Local: "n"})
	assess := func(cancelAt int64) int64 {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		meter := &scalarMeter{cancel: cancel, cancelAt: cancelAt}
		r, err := NewRun(c, meter)
		if err != nil {
			t.Fatal(err)
		}
		_, err = r.Bind(ctx, u, Subject{ID: 1, Name: QName{Local: "n"}, Text: "p:name -1", TextKnown: true, AttributesComplete: true, ChildrenComplete: true})
		if cancelAt == 0 && err != nil {
			t.Fatal(err)
		}
		if cancelAt != 0 && !errors.Is(err, context.Canceled) {
			t.Fatalf("checkpoint %d: %v", cancelAt, err)
		}
		if r.context.local != nil {
			t.Fatalf("checkpoint %d leaked local binding", cancelAt)
		}
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
		if meter.retained != 0 {
			t.Fatalf("checkpoint %d retained %d", cancelAt, meter.retained)
		}
		return meter.calls
	}
	for checkpoint, total := int64(1), assess(0); checkpoint <= total; checkpoint++ {
		assess(checkpoint)
	}
}

func TestTruthfulnessUnionProofEveryCancellationCleansUp(t *testing.T) {
	for _, badLater := range []bool{false, true} {
		later := truthfulnessRestriction("token")
		if badLater {
			later = truthfulnessRestriction("string")
			later.Patterns = []string{"["}
		}
		union := &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Name: QName{Space: XSDNamespace, Local: "QName"}}, {Inline: later}}}
		outer := &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Inline: union}, Patterns: []string{" .* "}}
		compiled, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "n"}: {Name: QName{Local: "n"}, Type: TypeRef{InlineSimple: outer}}}}, CompileLimits{})
		if err != nil {
			t.Fatal(err)
		}
		u, _ := compiled.Root(QName{Local: "n"})
		assess := func(cancelAt int64) int64 {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			meter := &scalarMeter{cancel: cancel, cancelAt: cancelAt}
			r, err := NewRun(compiled, meter)
			if err != nil {
				t.Fatal(err)
			}
			b, err := r.Bind(ctx, u, Subject{ID: 1, Name: QName{Local: "n"}, Text: " p:name ", TextKnown: true, AttributesComplete: true, ChildrenComplete: true})
			if cancelAt == 0 && err != nil {
				t.Fatal(err)
			}
			if cancelAt != 0 && !errors.Is(err, context.Canceled) {
				t.Fatalf("badLater=%v checkpoint %d: %v", badLater, cancelAt, err)
			}
			if badLater && cancelAt != 0 && r.owns(b) && !r.Results(b).Scalar.Valid {
				t.Fatalf("badLater=%v checkpoint %d invented invalidity", badLater, cancelAt)
			}
			if r.context.local != nil || r.scratchRetained != 0 {
				t.Fatalf("badLater=%v checkpoint %d leaked trial scope or scratch", badLater, cancelAt)
			}
			if err := r.Close(); err != nil {
				t.Fatal(err)
			}
			if meter.retained != 0 {
				t.Fatalf("badLater=%v checkpoint %d retained %d", badLater, cancelAt, meter.retained)
			}
			return meter.calls
		}
		for checkpoint, total := int64(1), assess(0); checkpoint <= total; checkpoint++ {
			assess(checkpoint)
		}
	}
}
