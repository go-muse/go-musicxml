package xsd

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"sync"
	"testing"
)

func currentNumber(t *testing.T, text string) CurrentScalar {
	t.Helper()
	n, ok := ParseDecimal(text)
	if !ok {
		t.Fatal(text)
	}
	return CurrentScalar{Known: true, Value: SimpleValue{Domain: NumberDomain, Number: n}}
}
func bindCurrent(t *testing.T, typ TypeRef, value CurrentScalar, fixed *string) (*Run, Binding, *testMeter) {
	t.Helper()
	name := QName{Local: "n"}
	compiled, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{name: {Name: name, Type: typ, Fixed: fixed}}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	meter := &testMeter{}
	run, err := NewRun(compiled, meter)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := run.Close(); err != nil {
			t.Error(err)
		}
		if meter.retained != 0 {
			t.Errorf("retained=%d", meter.retained)
		}
	})
	use, _ := compiled.Root(name)
	bound, err := run.Bind(context.Background(), use, Subject{ID: 1, Name: name, Path: "N", CurrentModel: true, Current: value, ChildrenOrderKnown: true, ChildrenComplete: true, AttributesComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	return run, bound, meter
}
func currentRestriction(base string) *SimpleSchema {
	return &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Name: QName{Space: XSDNamespace, Local: base}}}
}
func TestCurrentExactValues(t *testing.T) {
	cases := []struct {
		name, builtin, value string
		valid                bool
	}{
		{"zero", "integer", "0", true}, {"large integer", "integer", "9007199254740993", true}, {"unsigned max", "unsignedLong", "18446744073709551615", true}, {"unsigned overflow", "unsignedLong", "18446744073709551616", false}, {"fraction integer", "integer", "0.5", false}, {"wrong string domain", "string", "1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, b, _ := bindCurrent(t, TypeRef{Name: QName{Space: XSDNamespace, Local: tc.builtin}}, currentNumber(t, tc.value), nil)
			got := r.Results(b).Scalar
			if got.Valid != tc.valid || !got.Complete {
				t.Fatalf("result=%+v", got)
			}
			if tc.valid {
				s := r.Scalar(b)
				if !s.Known || s.Lexeme != "" || s.EffectiveLexeme != "" {
					t.Fatalf("scalar=%+v", s)
				}
			}
		})
	}
}
func TestCurrentNumericFacetsAndPattern(t *testing.T) {
	cases := []struct {
		name            string
		schema          *SimpleSchema
		value           string
		valid, complete bool
		constraint      string
	}{
		{name: "exact decimal boundary", schema: &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Name: QName{Space: XSDNamespace, Local: "decimal"}}, HasMaxInclusive: true, MaxInclusive: "0.1"}, value: "0.1000000000000000055511151231257827021181583404541015625", constraint: "maxInclusive", complete: true},
		{name: "binary fraction digits", schema: &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Name: QName{Space: XSDNamespace, Local: "decimal"}}, HasFractionDigits: true, FractionDigits: 2}, value: "0.1000000000000000055511151231257827021181583404541015625", constraint: "fractionDigits", complete: true},
		{name: "pattern unavailable", schema: &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Name: QName{Space: XSDNamespace, Local: "integer"}}, Patterns: []string{"[0-9]+"}}, value: "4", valid: true},
		{name: "pattern keeps bound failure", schema: &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Name: QName{Space: XSDNamespace, Local: "integer"}}, Patterns: []string{"[0-9]+"}, HasMaxInclusive: true, MaxInclusive: "3"}, value: "4", constraint: "maxInclusive"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, b, _ := bindCurrent(t, TypeRef{InlineSimple: tc.schema}, currentNumber(t, tc.value), nil)
			got := r.Results(b).Scalar
			if got.Valid != tc.valid || got.Complete != tc.complete {
				t.Fatalf("result=%+v", got)
			}
			if tc.constraint != "" && (len(got.Issues) == 0 || got.Issues[0].Constraint != tc.constraint) {
				t.Fatalf("issues=%+v", got.Issues)
			}
		})
	}
}
func TestCurrentStringUnionAndFixed(t *testing.T) {
	numeric := currentRestriction("decimal")
	union := &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Inline: numeric}, {Name: QName{Space: XSDNamespace, Local: "string"}}}}
	r, b, _ := bindCurrent(t, TypeRef{InlineSimple: union}, CurrentScalar{Known: true, Value: SimpleValue{Domain: StringDomain, Text: " 0.1 "}}, nil)
	if got := r.Scalar(b); !got.Known || got.Value.Domain != NumberDomain || got.Lexeme != "" {
		t.Fatalf("scalar=%+v", got)
	}
	fixed := "0.1"
	r, b, _ = bindCurrent(t, TypeRef{Name: QName{Space: XSDNamespace, Local: "decimal"}}, currentNumber(t, "0.1000000000000000055511151231257827021181583404541015625"), &fixed)
	if r.Results(b).Declaration.Valid {
		t.Fatal("binary decimal equated with decimal fixed")
	}
	zero := "0"
	r, b, _ = bindCurrent(t, TypeRef{Name: QName{Space: XSDNamespace, Local: "integer"}}, currentNumber(t, "0"), &zero)
	if !r.Results(b).Declaration.Valid || r.Scalar(b).Defaulted {
		t.Fatal("numeric zero treated as empty")
	}
}

func TestCurrentRestrictionChainsAndUnionSelection(t *testing.T) {
	pattern := currentRestriction("integer")
	pattern.Patterns = []string{"[0-9]+"}
	outer := &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Inline: pattern}, HasMaxInclusive: true, MaxInclusive: "3"}
	r, b, _ := bindCurrent(t, TypeRef{InlineSimple: outer}, currentNumber(t, "4"), nil)
	if got := r.Results(b).Scalar; got.Valid || got.Complete || len(got.Issues) != 1 || got.Issues[0].Constraint != "maxInclusive" {
		t.Fatalf("lost outer bound: %+v", got)
	}
	union := &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Inline: pattern}, {Name: QName{Space: XSDNamespace, Local: "integer"}}}}
	r, b, _ = bindCurrent(t, TypeRef{InlineSimple: union}, currentNumber(t, "4"), nil)
	if got := r.Results(b).Scalar; !got.Valid || got.Complete || r.Scalar(b).Known {
		t.Fatalf("unresolved first member selected later: %+v", got)
	}
	restrictedUnion := &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Inline: &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Name: QName{Space: XSDNamespace, Local: "integer"}}, {Name: QName{Space: XSDNamespace, Local: "string"}}}}}, Enumerations: []string{"5"}}
	union = &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Inline: restrictedUnion}}}
	r, b, _ = bindCurrent(t, TypeRef{InlineSimple: union}, currentNumber(t, "4"), nil)
	if got := r.Results(b).Scalar; !got.Valid || !got.Complete {
		t.Fatalf("union-variety member was not flattened: %+v", got)
	}
	r, b, _ = bindCurrent(t, TypeRef{InlineSimple: restrictedUnion}, currentNumber(t, "4"), nil)
	if got := r.Results(b).Scalar; got.Valid || !got.Complete {
		t.Fatalf("direct union restriction was ignored: %+v", got)
	}
}

func TestCurrentEmptyConstraintAndSourceLaneSeparation(t *testing.T) {
	for _, value := range []CurrentScalar{{Absent: true}, {Known: true, Value: SimpleValue{Domain: StringDomain, Text: ""}}} {
		fixed := "x"
		r, b, _ := bindCurrent(t, TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}}, value, &fixed)
		if got := r.Results(b); !got.Declaration.Valid || got.Declaration.Complete || got.Scalar.Complete || r.Scalar(b).Defaulted {
			t.Fatalf("empty current constraint guessed: %+v", got)
		}
	}
	compiled := testRunSchema(t)
	r, err := NewRun(compiled, &testMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	use, _ := compiled.Root(QName{Local: "root"})
	b, err := r.Bind(context.Background(), use, Subject{ID: 1, Name: QName{Local: "root"}, Current: CurrentScalar{Absent: true}, ChildrenOrderKnown: true, ChildrenComplete: true, AttributesComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Results(b).Scalar.Complete {
		t.Fatal("source unknown treated as current absent")
	}
}

func TestCurrentBooleanAndExactEnumerations(t *testing.T) {
	yes := "1"
	r, b, _ := bindCurrent(t, TypeRef{Name: QName{Space: XSDNamespace, Local: "boolean"}}, CurrentScalar{Known: true, Value: SimpleValue{Domain: BooleanDomain, Boolean: true}}, &yes)
	if got := r.Results(b); !got.Scalar.Valid || !got.Declaration.Valid || !got.Scalar.Complete {
		t.Fatalf("boolean fixed mismatch: %+v", got)
	}
	schema := currentRestriction("decimal")
	schema.Enumerations = []string{"0.1"}
	r, b, _ = bindCurrent(t, TypeRef{InlineSimple: schema}, currentNumber(t, "0.1000000000000000055511151231257827021181583404541015625"), nil)
	if r.Results(b).Scalar.Valid {
		t.Fatal("binary-exact value matched decimal enumeration")
	}
}

func TestCurrentOrderHoleNeverRecoversCompressedWitnesses(t *testing.T) {
	for _, known := range []bool{true, false} {
		r, err := NewRun(testRunSchema(t), &testMeter{})
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		use, _ := r.Root(QName{Local: "root"})
		b, err := r.Bind(context.Background(), use, Subject{ID: 1, Name: QName{Local: "root"}, CurrentModel: true, Current: CurrentScalar{Absent: true}, ChildrenOrderKnown: known, ChildrenComplete: known, AttributesComplete: true, Children: []Child{{ID: 2, Name: QName{Local: "child"}}}})
		if err != nil {
			t.Fatal(err)
		}
		got := r.Results(b).Content
		if !got.Valid || got.Complete != known || (len(r.Witnesses(b)) == 1) != known {
			t.Fatalf("known=%v result=%+v witnesses=%+v", known, got, r.Witnesses(b))
		}
	}
}

func TestCurrentAttributesKeepSlotResultsAndDefaults(t *testing.T) {
	pattern := currentRestriction("integer")
	pattern.Patterns = []string{"[0-9]+"}
	data := SchemaData{Elements: map[QName]*ElementSchema{{Local: "root"}: {Name: QName{Local: "root"}, Type: TypeRef{InlineComplex: &ComplexSchema{Form: ComplexDirect, Attributes: []AttributeSchema{
		{Name: QName{Local: "bad"}, Type: TypeRef{InlineSimple: pattern}},
		{Name: QName{Local: "good"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "integer"}}},
		{Name: QName{Local: "default"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "integer"}}, Constraint: &ValueConstraint{Kind: "default", Value: "7"}},
	}}}}}}
	c, err := Compile(data, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRun(c, &testMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	use, _ := r.Root(QName{Local: "root"})
	b, err := r.Bind(context.Background(), use, Subject{ID: 1, Name: QName{Local: "root"}, CurrentModel: true, Current: CurrentScalar{Absent: true}, ChildrenOrderKnown: true, ChildrenComplete: true, AttributesComplete: true, Attributes: []Attribute{{Attr: Attr{Name: QName{Local: "bad"}}, Current: currentNumber(t, "4")}, {Attr: Attr{Name: QName{Local: "good"}}, Current: currentNumber(t, "5")}}})
	if err != nil {
		t.Fatal(err)
	}
	for a := range r.Attributes(b) {
		switch a.Name.Local {
		case "bad":
			if a.Complete || a.Scalar.Complete || a.TypeValid {
				t.Errorf("incomplete slot marked complete: %+v", a)
			}
		case "good":
			if !a.Complete || !a.TypeValid || !a.Scalar.Known || a.Scalar.Lexeme != "" {
				t.Errorf("independent slot lost: %+v", a)
			}
		case "default":
			if a.Present || !a.Complete || !a.Scalar.Defaulted || a.Scalar.Value.Number.Whole != "7" || a.Scalar.Lexeme != "" {
				t.Errorf("default corrupted: %+v", a)
			}
		}
	}
}

func TestCurrentProvedBoundSurvivesDiagnosticAllocationStop(t *testing.T) {
	cause := errors.New("diagnostic memory stop")
	meter := &scalarMeter{reserveFailure: cause}
	run := &Run{meter: meter, current: context.Background()}
	state := &boundState{run: run, subject: Subject{CurrentModel: true}, results: freshResults(), family: "scalar"}
	c := &Context{run: run, local: state}
	scalarAbort(t, cause, func() {
		validateNumberBounds(&SimpleSchema{HasMaxInclusive: true, MaxInclusive: "3"}, Decimal{Whole: "4"}, "integer", c)
	})
	if state.results.Scalar.Valid {
		t.Fatal("proved bound failure was lost during diagnostic allocation")
	}
}

func TestCurrentIntegerSubtypeEndpoints(t *testing.T) {
	for _, builtin := range []string{"nonPositiveInteger", "negativeInteger", "nonNegativeInteger", "positiveInteger", "long", "int", "short", "byte", "unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte"} {
		minimum, maximum, _ := IntegerLimits(builtin)
		for _, edge := range []struct {
			value string
			delta int64
		}{{minimum, -1}, {maximum, 1}} {
			if edge.value == "" {
				continue
			}
			for _, outside := range []bool{false, true} {
				value, _ := new(big.Int).SetString(edge.value, 10)
				if outside {
					value.Add(value, big.NewInt(edge.delta))
				}
				t.Run(builtin+"/"+value.String(), func(t *testing.T) {
					r, b, _ := bindCurrent(t, TypeRef{Name: QName{Space: XSDNamespace, Local: builtin}}, currentNumber(t, value.String()), nil)
					got := r.Results(b).Scalar
					if got.Valid == outside || !got.Complete {
						t.Fatalf("edge result=%+v", got)
					}
				})
			}
		}
	}
}

func TestCurrentValueWorkIsBounded(t *testing.T) {
	cause := errors.New("current component work stopped")
	meter := &scalarMeter{limit: 64, cause: cause}
	c := scalarTestContext(meter, context.Background())
	scalarAbort(t, cause, func() {
		c.checkCurrent(CurrentScalar{Known: true, Value: SimpleValue{Domain: NumberDomain, Number: Decimal{Fraction: strings.Repeat("1", 4096)}}})
	})
	ctx, cancel := context.WithCancel(context.Background())
	meter = &scalarMeter{cancel: cancel, cancelAt: 8}
	c = scalarTestContext(meter, ctx)
	scalarAbort(t, context.Canceled, func() {
		c.checkCurrent(CurrentScalar{Known: true, Value: SimpleValue{Domain: NumberDomain, Number: Decimal{Fraction: strings.Repeat("1", 4096)}}})
	})
	cancel()
}

func TestCurrentUnionTrialFailureDoesNotPolluteSuccess(t *testing.T) {
	union := &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Name: QName{Space: XSDNamespace, Local: "boolean"}}, {Name: QName{Space: XSDNamespace, Local: "integer"}}}}
	r, b, _ := bindCurrent(t, TypeRef{InlineSimple: union}, currentNumber(t, "4"), nil)
	if got := r.Results(b).Scalar; !got.Valid || !got.Complete || !r.Scalar(b).Known {
		t.Fatalf("trial failure poisoned success: %+v", got)
	}
}

func TestCurrentFixedUnionPatternIsLocalCapability(t *testing.T) {
	pattern := currentRestriction("integer")
	pattern.Patterns = []string{"[0-9]+"}
	union := &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Inline: pattern}, {Name: QName{Space: XSDNamespace, Local: "decimal"}}}}
	fixed := "4"
	r, b, _ := bindCurrent(t, TypeRef{InlineSimple: union}, currentNumber(t, "4"), &fixed)
	if got := r.Results(b); !got.Declaration.Valid || got.Declaration.Complete || got.Scalar.Complete {
		t.Fatalf("current capability=%+v", got)
	}
	if r.context.AssessmentErr != nil || r.context.IdentityIncomplete {
		t.Fatalf("unrelated identity closure poisoned: %v, %v", r.context.AssessmentErr, r.context.IdentityIncomplete)
	}
}

func TestCurrentUnknownNumericAttributeDoesNotHideIdentityOwner(t *testing.T) {
	name := QName{Local: "root"}
	schema, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{name: {Name: name, Type: TypeRef{InlineComplex: &ComplexSchema{Form: ComplexDirect, Attributes: []AttributeSchema{
		{Name: QName{Local: "id"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "ID"}}}, {Name: QName{Local: "number"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "decimal"}}},
	}}}}}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRun(schema, &testMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	use, _ := r.Root(name)
	b, err := r.Bind(context.Background(), use, Subject{ID: 1, Name: name, CurrentModel: true, Current: CurrentScalar{Absent: true}, ChildrenOrderKnown: true, ChildrenComplete: true, AttributesComplete: true, Attributes: []Attribute{
		{Attr: Attr{Name: QName{Local: "id"}}, Current: CurrentScalar{Known: true, Value: SimpleValue{Domain: StringDomain, Text: "target"}}}, {Attr: Attr{Name: QName{Local: "number"}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Complete(context.Background(), b, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Finalize(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	result, err := r.ResolveIdentity(context.Background(), "target")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != IdentityUnique || result.Owner != 1 {
		t.Fatalf("numeric value hole hid independent ID owner: %+v", result)
	}
}

func TestCurrentAndSourceShareCompiledConcurrently(t *testing.T) {
	name := QName{Local: "n"}
	compiled, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{name: {Name: name, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "integer"}}}}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	digest := compiled.Digest()
	use, _ := compiled.Root(name)
	var workers sync.WaitGroup
	for index := range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			meter := &testMeter{}
			run, err := NewRun(compiled, meter)
			if err != nil {
				t.Error(err)
				return
			}
			subject := Subject{ID: 1, Name: name, TextKnown: true, Text: "+004", ChildrenComplete: true, AttributesComplete: true}
			if index%2 == 0 {
				subject.CurrentModel = true
				subject.Current = CurrentScalar{Known: true, Value: SimpleValue{Domain: NumberDomain, Number: Decimal{Whole: "4"}}}
				subject.ChildrenOrderKnown = true
				subject.TextKnown = false
				subject.Text = ""
			}
			bound, err := run.Bind(context.Background(), use, subject)
			if err != nil {
				t.Error(err)
			} else {
				got := run.Scalar(bound)
				if !got.Known || !got.Complete || got.Value.Number.Whole != "4" {
					t.Errorf("scalar=%+v", got)
				}
				if subject.CurrentModel && got.Lexeme != "" {
					t.Error("fabricated model lexeme")
				}
			}
			if err := run.Close(); err != nil {
				t.Error(err)
			}
			if meter.retained != 0 {
				t.Errorf("retained=%d", meter.retained)
			}
		}()
	}
	workers.Wait()
	if compiled.Digest() != digest {
		t.Fatal("shared schema changed")
	}
}
