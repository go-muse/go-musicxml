package xsd

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func wildcardUnionResourceSchema(t *testing.T, unavailable bool) (*Compiled, Use, Subject) {
	t.Helper()
	base := "urn:base urn:shared"
	derived := "##local urn:derived urn:shared"
	if unavailable {
		base = "##other"
		derived = "##local"
	}
	typ := wildcardUnionType(false, unionWildcard(base, "skip"), unionWildcard(derived, "skip"), false)
	typ = &ComplexSchema{Form: ComplexComplexContentExtension, Base: &TypeRef{InlineComplex: typ}, AnyAttribute: unionWildcard("urn:last", "skip")}
	c, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "n"}: {Name: QName{Local: "n"}, Type: TypeRef{InlineComplex: typ}}}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	u, _ := c.Root(QName{Local: "n"})
	return c, u, Subject{ID: 1, Name: QName{Local: "n"}, TextKnown: true, AttributesComplete: true, ChildrenComplete: true, Attributes: []Attribute{{Attr: Attr{Name: QName{Space: "urn:base", Local: "a"}, Value: "x"}, ValueKnown: true}}}
}
func TestAttributeWildcardUnionExactBudgetsAndCleanup(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		c, u, subject := wildcardUnionResourceSchema(t, unavailable)
		assess := func(m *boundaryMeter, stop bool) {
			t.Helper()
			r, err := NewRun(c, m)
			if err != nil {
				t.Fatal(err)
			}
			b, err := r.Bind(context.Background(), u, subject)
			if stop {
				if !errors.Is(err, errMeterStop) {
					t.Fatalf("expected original budget cause: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if !stop {
				if got := r.Results(b).Attributes; !got.Valid || got.Complete == unavailable {
					t.Fatalf("result: %+v", got)
				}
			}
			if r.scratchRetained != 0 || r.context.local != nil {
				t.Fatal("operation scope leaked")
			}
			if err := r.Close(); err != nil {
				t.Fatal(err)
			}
			if m.buffer != 0 {
				t.Fatalf("Close retained %d", m.buffer)
			}
		}
		baseline := &boundaryMeter{}
		assess(baseline, false)
		for range 3 {
			exact := &boundaryMeter{workLimit: baseline.work, bufferLimit: baseline.peak}
			assess(exact, false)
			if exact.work != baseline.work || exact.peak != baseline.peak {
				t.Fatalf("nondeterministic %d/%d vs %d/%d", exact.work, exact.peak, baseline.work, baseline.peak)
			}
		}
		assess(&boundaryMeter{workLimit: baseline.work - 1}, true)
		assess(&boundaryMeter{bufferLimit: baseline.peak - 1}, true)
		t.Logf("unavailable=%v work=%d peak=%d", unavailable, baseline.work, baseline.peak)
	}
}
func TestAttributeWildcardUnionEveryCancellationCheckpoint(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		c, u, subject := wildcardUnionResourceSchema(t, unavailable)
		assess := func(stop int64) int64 {
			t.Helper()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			m := &scalarMeter{cancel: cancel, cancelAt: stop}
			r, err := NewRun(c, m)
			if err != nil {
				t.Fatal(err)
			}
			b, err := r.Bind(ctx, u, subject)
			if stop == 0 {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.Is(err, context.Canceled) {
				t.Fatalf("checkpoint %d/%v: %v", stop, unavailable, err)
			}
			if stop != 0 && r.owns(b) && !r.Results(b).Attributes.Valid {
				t.Fatalf("checkpoint %d fabricated invalidity", stop)
			}
			if r.scratchRetained != 0 || r.context.local != nil {
				t.Fatalf("checkpoint %d leaked operation state", stop)
			}
			if err := r.Close(); err != nil {
				t.Fatal(err)
			}
			if m.retained != 0 {
				t.Fatalf("checkpoint %d retained %d", stop, m.retained)
			}
			return m.calls
		}
		total := assess(0)
		for stop := int64(1); stop <= total; stop++ {
			assess(stop)
		}
		t.Logf("unavailable=%v cancellation checkpoints=%d", unavailable, total)
	}
}
func TestAttributeWildcardUnionLongScansAreMetered(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		base := "urn:" + strings.Repeat("a", 2048)
		if duplicate {
			base = strings.Repeat("urn:repeat ", 256)
		}
		scan := func(c *Context) {
			got, unavailable := mergeAnyAttribute(unionWildcard(base, "skip"), unionWildcard("urn:last ##local", "strict"), c)
			if unavailable || got == nil || got.ProcessContents != "strict" {
				t.Fatalf("result: %+v unavailable=%v", got, unavailable)
			}
		}
		m := &scalarMeter{}
		scan(scalarTestContext(m, context.Background()))
		if m.steps < int64(len(base)) {
			t.Fatalf("unmetered input: %d", m.steps)
		}
		scan(scalarTestContext(&scalarMeter{limit: m.steps}, context.Background()))
		cause := errors.New("union work stopped")
		scalarAbort(t, cause, func() { scan(scalarTestContext(&scalarMeter{limit: m.steps - 1, cause: cause}, context.Background())) })
	}
}

func TestAttributeWildcardDirectAdmissionBudgetsAndCleanup(t *testing.T) {
	for _, wildcard := range []*AnyAttributeSchema{nil, unionWildcard("", "skip"), unionWildcard(" ", "skip"), unionWildcard("urn:one ##local urn:two", "skip")} {
		for _, extra := range []bool{false, true} {
			typ := &ComplexSchema{Form: ComplexDirect, AnyAttribute: wildcard}
			c, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "n"}: {Name: QName{Local: "n"}, Type: TypeRef{InlineComplex: typ}}}}, CompileLimits{})
			if err != nil {
				t.Fatal(err)
			}
			u, _ := c.Root(QName{Local: "n"})
			subject := Subject{ID: 1, Name: QName{Local: "n"}, TextKnown: true, AttributesComplete: true, ChildrenComplete: true}
			if extra {
				subject.Attributes = []Attribute{{Attr: Attr{Name: QName{Space: "urn:two", Local: "a"}, Value: "x"}, ValueKnown: true}}
			}
			want := !extra || wildcard != nil && wildcard.Namespace != " "
			assess := func(m *boundaryMeter, stop bool) {
				t.Helper()
				r, err := NewRun(c, m)
				if err != nil {
					t.Fatal(err)
				}
				b, err := r.Bind(context.Background(), u, subject)
				if stop && !errors.Is(err, errMeterStop) || !stop && err != nil {
					t.Fatalf("stop=%t: %v", stop, err)
				}
				if !stop {
					if got := r.Results(b).Attributes; got.Valid != want || !got.Complete {
						t.Fatalf("wildcard=%+v extra=%t: %+v", wildcard, extra, got)
					}
				}
				if r.scratchRetained != 0 || r.context.local != nil {
					t.Fatal("direct admission leaked tokenizer scratch")
				}
				if err := r.Close(); err != nil {
					t.Fatal(err)
				}
				if m.buffer != 0 {
					t.Fatalf("Close retained %d", m.buffer)
				}
			}
			baseline := &boundaryMeter{}
			assess(baseline, false)
			assess(&boundaryMeter{workLimit: baseline.work, bufferLimit: baseline.peak}, false)
			assess(&boundaryMeter{workLimit: baseline.work - 1}, true)
			assess(&boundaryMeter{bufferLimit: baseline.peak - 1}, true)
			cancelAssess := func(stop int64) int64 {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				m := &scalarMeter{cancel: cancel, cancelAt: stop}
				r, err := NewRun(c, m)
				if err != nil {
					t.Fatal(err)
				}
				_, err = r.Bind(ctx, u, subject)
				if stop == 0 && err != nil || stop != 0 && !errors.Is(err, context.Canceled) {
					t.Fatalf("checkpoint %d: %v", stop, err)
				}
				if r.scratchRetained != 0 || r.context.local != nil {
					t.Fatalf("checkpoint %d leaked direct tokenizer scratch", stop)
				}
				if err := r.Close(); err != nil {
					t.Fatal(err)
				}
				if m.retained != 0 {
					t.Fatalf("checkpoint %d retained %d", stop, m.retained)
				}
				return m.calls
			}
			for stop, total := int64(1), cancelAssess(0); stop <= total; stop++ {
				cancelAssess(stop)
			}
		}
	}
}
