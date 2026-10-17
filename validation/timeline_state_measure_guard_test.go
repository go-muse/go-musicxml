package validation

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"testing"
)

// Frozen pre-optimization stateMeasure from ca3165e6b087d17e9e5a0c0e1aaf52a0b6365fc5.
// Keep the complete recursive reference independent of the production walker.
func legacyStateMeasureForGuard(ctx context.Context, s *Session, v reflect.Value) (int64, error) {
	if e := s.step(ctx, 1); e != nil {
		return 0, e
	}
	if !v.IsValid() {
		return 0, nil
	}
	if v.CanInterface() {
		switch n := v.Interface().(type) {
		case Integer:
			return n.ByteSize() + 64, nil
		case Decimal:
			return n.ByteSize() + 64, nil
		case Rational:
			return n.ByteSize() + 64, nil
		}
	}
	n := int64(32)
	add := func(m int64, e error) error {
		if e != nil {
			return e
		}
		var x error
		n, x = rationalCheckedAdd(BudgetBufferedBytes, n, m)
		return x
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if e := add(legacyStateMeasureForGuard(ctx, s, v.Field(i))); e != nil {
				return 0, e
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if e := add(legacyStateMeasureForGuard(ctx, s, v.Index(i))); e != nil {
				return 0, e
			}
		}
	case reflect.Pointer:
		if !v.IsNil() {
			if v.Type() != reflect.TypeFor[*DecisionRef]() && v.Type() != reflect.TypeFor[*Evidence[string]]() && v.Type() != reflect.TypeFor[*musicalPosition]() {
				return 0, contract("timeline state", "unexpected mutable pointer")
			}
			if e := add(legacyStateMeasureForGuard(ctx, s, v.Elem())); e != nil {
				return 0, e
			}
		}
	case reflect.String:
		if e := s.step(ctx, int64(v.Len())); e != nil {
			return 0, e
		}
		if e := add(int64(v.Len()), nil); e != nil {
			return 0, e
		}
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
	default:
		return 0, contract("timeline state", "unsupported private payload")
	}
	return n, nil
}

type stateMeasureGuardAliasInteger = Integer
type stateMeasureGuardNamedInteger Integer
type stateMeasureGuardNamedDecimal Decimal
type stateMeasureGuardNamedRational Rational
type stateMeasureGuardFunc func(context.Context, *Session, reflect.Value) (int64, error)
type stateMeasureGuardOutcome struct {
	Size     int64
	Err      error
	Usage    [budgetKindCount]int64
	Retained int64
	Calls    int
}
type stateMeasureGuardContext struct {
	context.Context
	At, Calls int
	Failure   error
}

func (c *stateMeasureGuardContext) Err() error {
	c.Calls++
	if c.Calls >= c.At {
		return c.Failure
	}
	return nil
}
func stateMeasureGuardRun(t *testing.T, fn stateMeasureGuardFunc, value reflect.Value, limit int64, at int, failure error, nilContext bool) stateMeasureGuardOutcome {
	t.Helper()
	s := rationalSession(t, Budgets{MaxRuleSteps: limit})
	ctx := &stateMeasureGuardContext{Context: context.Background(), At: at, Failure: failure}
	var used context.Context = ctx
	if nilContext {
		used = nil
	}
	size, err := fn(used, s, value)
	return stateMeasureGuardOutcome{size, err, s.account.usage, s.retained, ctx.Calls}
}
func stateMeasureGuardCompare(t *testing.T, value reflect.Value, limit int64, at int, failure error, nilContext bool) stateMeasureGuardOutcome {
	t.Helper()
	want := stateMeasureGuardRun(t, legacyStateMeasureForGuard, value, limit, at, failure, nilContext)
	got := stateMeasureGuardRun(t, stateMeasure, value, limit, at, failure, nilContext)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("limit=%d at=%d nil=%t got=%#v want=%#v", limit, at, nilContext, got, want)
	}
	if want.Err == failure && got.Err != failure {
		t.Fatal("cancellation identity changed")
	}
	for _, target := range []error{failure, ErrInvalidConfiguration, ErrInvalidSession, ErrResourceLimit, ErrIncompleteAssessment} {
		if errors.Is(got.Err, target) != errors.Is(want.Err, target) {
			t.Fatalf("error identity %v differs", target)
		}
	}
	return want
}
func TestStateMeasureGuardLegacyParity(t *testing.T) {
	integer, err := NewInteger(big.NewInt(123))
	if err != nil {
		t.Fatal(err)
	}
	decimal, err := NewDecimal(big.NewRat(5, 2))
	if err != nil {
		t.Fatal(err)
	}
	rational, err := NewRational(big.NewRat(7, 3))
	if err != nil {
		t.Fatal(err)
	}
	cause := &DecisionRef{Rule: RuleKey{ID: "cause-rule", Revision: 2}, Subject: NodeRef{DocumentID: "doc", Node: 3}}
	evidence := Evidence[string]{State: EvidenceInvalid, Value: "body", Reason: "because", Cause: cause}
	position := musicalPosition{Part: NodeRef{"doc", 1}, Measure: NodeRef{"doc", 2}, MeasureOrdinal: 4, Offset: rational}
	private := struct {
		s string
		i Integer
		r Rational
	}{"private", integer, rational}
	privateZero := struct {
		s string
		i Integer
		r Rational
	}{s: "private"}
	compound := struct {
		Text     string
		Number   Integer
		Ratio    Rational
		Decimal  Decimal
		Cause    *DecisionRef
		When     *Evidence[string]
		Position *musicalPosition
		Refs     []NodeRef
		Evidence []Evidence[string]
	}{"score", integer, rational, decimal, cause, &evidence, &position, []NodeRef{{"one", 1}, {"two", 2}}, []Evidence[string]{evidence, {State: EvidenceAbsent, Value: "carried"}}}
	timing := newTimingContext("audit")
	timing.Duration = stateKnown(rational)
	timing.Figures = []NodeRef{{"score", 1}, {"score", 2}}
	timing.StateOrigins = []TimingStateOrigin{{}}
	timing.Voice = evidence
	var iface any = integer
	var ifaceDecimal any = decimal
	var ifaceRational any = rational
	var ifaceAlias any = stateMeasureGuardAliasInteger(integer)
	var ifaceDefined any = stateMeasureGuardNamedInteger(integer)
	var ifacePointer any = &integer
	var ifaceNil any
	var ifaceText any = "plain"
	var ifaceMethod interface{ ByteSize() int64 } = rational
	privateInterface := struct{ hidden any }{integer}
	var assignedInterface any = ifaceMethod
	nestedInterface := struct{ Outer struct{ Inner any } }{}
	nestedInterface.Outer.Inner = rational
	var nestedDynamic any = nestedInterface
	fixtures := []struct {
		Name  string
		Value reflect.Value
	}{
		{"invalid", reflect.Value{}},
		{"integer", reflect.ValueOf(integer)}, {"decimal", reflect.ValueOf(decimal)}, {"rational", reflect.ValueOf(rational)},
		{"integer-addressable", reflect.ValueOf(&integer).Elem()}, {"decimal-addressable", reflect.ValueOf(&decimal).Elem()}, {"rational-addressable", reflect.ValueOf(&rational).Elem()},
		{"integer-zero", reflect.ValueOf(Integer{})}, {"decimal-zero", reflect.ValueOf(Decimal{})}, {"rational-zero", reflect.ValueOf(Rational{})},
		{"exact-alias", reflect.ValueOf(stateMeasureGuardAliasInteger(integer))}, {"defined-integer", reflect.ValueOf(stateMeasureGuardNamedInteger(integer))}, {"defined-decimal", reflect.ValueOf(stateMeasureGuardNamedDecimal(decimal))}, {"defined-rational", reflect.ValueOf(stateMeasureGuardNamedRational(rational))},
		{"defined-integer-zero", reflect.ValueOf(stateMeasureGuardNamedInteger{})}, {"defined-decimal-zero", reflect.ValueOf(stateMeasureGuardNamedDecimal{})}, {"defined-rational-zero", reflect.ValueOf(stateMeasureGuardNamedRational{})},
		{"private-numeric", reflect.ValueOf(private).Field(1)}, {"private-string", reflect.ValueOf(private).Field(0)}, {"private-struct", reflect.ValueOf(private)}, {"private-zero-struct", reflect.ValueOf(privateZero)},
		{"numeric-pointer", reflect.ValueOf(&integer)}, {"numeric-pointer-nil", reflect.ValueOf((*Integer)(nil))}, {"decimal-pointer", reflect.ValueOf(&decimal)}, {"rational-pointer", reflect.ValueOf(&rational)},
		{"allowed-cause", reflect.ValueOf(cause)}, {"allowed-evidence", reflect.ValueOf(&evidence)}, {"allowed-position", reflect.ValueOf(&position)},
		{"allowed-nil-cause", reflect.ValueOf((*DecisionRef)(nil))}, {"allowed-nil-evidence", reflect.ValueOf((*Evidence[string])(nil))}, {"allowed-nil-position", reflect.ValueOf((*musicalPosition)(nil))},
		{"compound", reflect.ValueOf(compound)}, {"compound-addressable", reflect.ValueOf(&compound).Elem()},
		{"timing", reflect.ValueOf(timing)}, {"timing-addressable", reflect.ValueOf(&timing).Elem()},
		{"slice-nil", reflect.ValueOf([]NodeRef(nil))}, {"slice-empty", reflect.ValueOf([]NodeRef{})}, {"slice-number", reflect.ValueOf([]Integer{integer, {}})},
		{"unsupported-map", reflect.ValueOf(map[string]int{"x": 1})}, {"unsupported-map-nil", reflect.ValueOf(map[string]int(nil))}, {"unsupported-array", reflect.ValueOf([1]int{1})}, {"unsupported-float", reflect.ValueOf(1.5)}, {"unsupported-complex", reflect.ValueOf(complex(1, 2))}, {"unsupported-channel", reflect.ValueOf(make(chan int))}, {"unsupported-function", reflect.ValueOf(func() {})}, {"interface-integer", reflect.ValueOf(&iface).Elem()},
		{"interface-decimal", reflect.ValueOf(&ifaceDecimal).Elem()}, {"interface-rational", reflect.ValueOf(&ifaceRational).Elem()}, {"interface-alias", reflect.ValueOf(&ifaceAlias).Elem()}, {"interface-defined", reflect.ValueOf(&ifaceDefined).Elem()}, {"interface-pointer", reflect.ValueOf(&ifacePointer).Elem()}, {"interface-nil", reflect.ValueOf(&ifaceNil).Elem()}, {"interface-string", reflect.ValueOf(&ifaceText).Elem()}, {"interface-method", reflect.ValueOf(&ifaceMethod).Elem()}, {"private-interface", reflect.ValueOf(privateInterface).Field(0)},
		{"assigned-interface", reflect.ValueOf(&assignedInterface).Elem()}, {"nested-interface-container", reflect.ValueOf(nestedInterface)}, {"nested-interface-container-addressable", reflect.ValueOf(&nestedInterface).Elem()}, {"interface-wrapped-container", reflect.ValueOf(&nestedDynamic).Elem()},
	}
	failure := errors.New("audit exact cancellation")
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			probe := stateMeasureGuardCompare(t, fixture.Value, 0, math.MaxInt, failure, false)
			// Nil context, every unit-of-work frontier, and every observed cancellation frontier.
			stateMeasureGuardCompare(t, fixture.Value, 0, math.MaxInt, failure, true)
			for limit := int64(1); limit <= probe.Usage[9]+1; limit++ {
				stateMeasureGuardCompare(t, fixture.Value, limit, math.MaxInt, failure, false)
			}
			for at := 1; at <= probe.Calls+1; at++ {
				stateMeasureGuardCompare(t, fixture.Value, 0, at, failure, false)
			}
			t.Logf("size=%d steps=%d polls=%d error=%v", probe.Size, probe.Usage[9], probe.Calls, probe.Err)
		})
	}
}
func BenchmarkStateMeasureGuard(b *testing.B) {
	integer, err := NewInteger(big.NewInt(123))
	if err != nil {
		b.Fatal(err)
	}
	decimal, err := NewDecimal(big.NewRat(5, 2))
	if err != nil {
		b.Fatal(err)
	}
	rational, err := NewRational(big.NewRat(7, 3))
	if err != nil {
		b.Fatal(err)
	}
	timing := newTimingContext("audit")
	timing.Duration = stateKnown(rational)
	timing.Voice = stateKnown("voice one")
	timing.Figures = []NodeRef{{"score", 1}, {"score", 2}}
	rows := make([]struct {
		Ref      NodeRef
		Name     Evidence[string]
		Integer  Integer
		Decimal  Decimal
		Rational Rational
	}, 32)
	for i := range rows {
		rows[i].Ref = NodeRef{"score", NodeID(i + 1)}
		rows[i].Name = stateKnown("voice one")
		rows[i].Integer = integer
		rows[i].Decimal = decimal
		rows[i].Rational = rational
	}
	for _, fixture := range []struct {
		Name  string
		Value reflect.Value
	}{{"AddressableRows", reflect.ValueOf(rows)}, {"TimingValue", reflect.ValueOf(timing)}, {"TimingAddressable", reflect.ValueOf(&timing).Elem()}} {
		for _, impl := range []struct {
			Name string
			Fn   stateMeasureGuardFunc
		}{{"Legacy", legacyStateMeasureForGuard}, {"Candidate", stateMeasure}} {
			b.Run(fmt.Sprintf("%s/%s", fixture.Name, impl.Name), func(b *testing.B) {
				a, err := NewBudgetAccount(Budgets{MaxRuleSteps: math.MaxInt64})
				if err != nil {
					b.Fatal(err)
				}
				s := &Session{account: a}
				ctx := context.Background()
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					if _, err := impl.Fn(ctx, s, fixture.Value); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// Serial: AllocsPerRun observes process-wide allocation counters. This fixture
// has no numeric wrappers requiring Interface conversion and prepares the
// account and reflected root before measuring, so copying each addressable
// record or scalar into a throwaway interface is always avoidable work.
func TestStateMeasureGuardAvoidsRecordBoxing(t *testing.T) {
	rows := make([]struct {
		Ref      NodeRef
		Name     Evidence[string]
		Selected bool
	}, 24)
	for i := range rows {
		rows[i].Ref = NodeRef{DocumentID: "score", Node: NodeID(i + 1)}
		rows[i].Name = Evidence[string]{State: EvidenceKnown, Value: "voice one"}
		rows[i].Selected = true
	}
	value := reflect.ValueOf(rows)
	s := rationalSession(t, Budgets{})
	ctx := context.Background()
	allocations := testing.AllocsPerRun(10, func() {
		if _, err := stateMeasure(ctx, s, value); err != nil {
			panic(err)
		}
	})
	// Allow small fixed implementation headroom, but no per-record/per-field
	// boxing. The same bound applies independently of pointer width.
	if allocations > 4 {
		t.Fatalf("24 addressable nonnumeric records allocated %g times; want <=4", allocations)
	}
	t.Logf("24 addressable nonnumeric records: %g allocations", allocations)
}
