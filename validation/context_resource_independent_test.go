package validation

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"
)

func independentContextFill(v reflect.Value, word string) {
	if !v.IsValid() {
		return
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).CanSet() {
				independentContextFill(v.Field(i), word)
			}
		}
	case reflect.Pointer:
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		independentContextFill(v.Elem(), word)
	case reflect.Slice:
		if v.IsNil() {
			v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		}
		for i := 0; i < v.Len(); i++ {
			independentContextFill(v.Index(i), word)
		}
	case reflect.String:
		v.SetString(word)
	case reflect.Bool:
		v.SetBool(word == "original")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if word == "original" {
			v.SetInt(1)
		} else {
			v.SetInt(2)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if word == "original" {
			v.SetUint(1)
		} else {
			v.SetUint(2)
		}
	}
}

func TestIndependentContextEveryPayloadOwnsMutableGraph(t *testing.T) {
	for _, fixture := range []struct {
		name     string
		property ContextProperty
		payload  any
	}{
		{"span", ContextPropertySpan, &SpanContextResult{}},
		{"repeat", ContextPropertyRepeat, &RepeatContextResult{}},
		{"navigation", ContextPropertyNavigation, &NavigationContextResult{}},
		{"layout", ContextPrintObject, &LayoutContextResult{}},
		{"timing", ContextPropertyTiming, &TimingContextResult{}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			independentContextFill(reflect.ValueOf(fixture.payload).Elem(), "original")
			in := ContextResult{Provider: RuleKey{"resource-auditor", 1}, Subject: NodeRef{"d", 1}, Property: fixture.property, Value: stateAbsent[Atomic]()}
			switch p := fixture.payload.(type) {
			case *SpanContextResult:
				in.Span = p
			case *RepeatContextResult:
				in.Repeat = p
			case *NavigationContextResult:
				in.Navigation = p
			case *LayoutContextResult:
				in.Layout = p
			case *TimingContextResult:
				in.Timing = p
			}
			s := rationalSession(t, Budgets{})
			if err := s.publishContextResult(context.Background(), in); err != nil {
				t.Fatal(err)
			}
			before, err := json.Marshal(s.contextResults)
			if err != nil {
				t.Fatal(err)
			}
			independentContextFill(reflect.ValueOf(fixture.payload).Elem(), "mutation")
			after, err := json.Marshal(s.contextResults)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("mutable input field or nested causal witness escaped into report")
			}
			s.cleanup()
			if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatal("retention leaked after cleanup")
			}
		})
	}
}

func TestIndependentContextEveryPayloadAdmissionBoundaries(t *testing.T) {
	for _, property := range []ContextProperty{ContextPropertySpan, ContextPropertyRepeat, ContextPropertyNavigation, ContextPrintObject, ContextPropertyTiming} {
		t.Run(string(property), func(t *testing.T) {
			in := ContextResult{Provider: RuleKey{"p", 1}, Subject: NodeRef{"d", 1}, Property: property, Value: stateAbsent[Atomic]()}
			switch property {
			case ContextPropertySpan:
				in.Span = &SpanContextResult{Members: []NodeRef{{"d", 2}, {"d", 3}}}
			case ContextPropertyRepeat:
				in.Repeat = &RepeatContextResult{Notes: []RepeatNoteScope{{Source: NodeRef{"d", 2}}}}
			case ContextPropertyNavigation:
				in.Navigation = &NavigationContextResult{Traversal: []NavigationOccurrence{{Source: NodeRef{"d", 2}}}}
			case ContextPrintObject:
				in.Layout = &LayoutContextResult{Scope: NodeRef{"d", 2}}
			case ContextPropertyTiming:
				in.Timing = &TimingContextResult{Figures: []NodeRef{{"d", 2}, {"d", 3}}}
			}
			probe := rationalSession(t, Budgets{})
			ctx := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
			if err := probe.publishContextResult(ctx, in); err != nil {
				t.Fatal(err)
			}
			bytes, steps, facts := rationalUsage(t, probe, BudgetBufferedBytes), rationalUsage(t, probe, BudgetRuleSteps), rationalUsage(t, probe, BudgetContextFacts)
			polls := ctx.calls
			probe.cleanup()
			for _, limits := range []Budgets{{MaxBufferedBytes: bytes - 1}, {MaxRuleSteps: steps - 1}, {MaxContextFacts: facts - 1}} {
				if limits.MaxContextFacts == 0 && limits.MaxBufferedBytes == 0 && limits.MaxRuleSteps == 0 {
					continue
				} // zero selects default, not zero allowance.
				s := rationalSession(t, limits)
				err := s.publishContextResult(context.Background(), in)
				if !errors.Is(err, ErrResourceLimit) || len(s.contextResults) != 0 || s.retained != 0 {
					t.Fatalf("failed admission published/retained: %+v err=%v records=%d bytes=%d", limits, err, len(s.contextResults), s.retained)
				}
				s.cleanup()
			}
			exact := rationalSession(t, Budgets{MaxBufferedBytes: bytes, MaxRuleSteps: steps, MaxContextFacts: facts})
			if err := exact.publishContextResult(context.Background(), in); err != nil {
				t.Fatalf("exact admission: %v", err)
			}
			exact.cleanup()
			for at := 1; at <= polls; at++ {
				s := rationalSession(t, Budgets{})
				err := s.publishContextResult(&rationalCancelContext{Context: context.Background(), at: at}, in)
				if !errors.Is(err, context.Canceled) || len(s.contextResults) != 0 || s.retained != 0 {
					t.Fatalf("poll %d: error=%v visible=%d bytes=%d", at, err, len(s.contextResults), s.retained)
				}
				s.cleanup()
			}
			t.Logf("exact retained=%d steps=%d facts=%d; cancellation polls=%d", bytes, steps, facts, polls)
		})
	}
}
