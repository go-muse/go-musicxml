package validation

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func sourceOriginRichPublication() ContextResult {
	in := sourceOriginPublication()
	v := in.SourceOrigin
	v.Kind = SourceOriginUnavailable
	v.Attribute = &QName{Space: "urn:attribute", Local: "value"}
	cause := &DecisionRef{Rule: RuleKey{"reader", 1}, Subject: NodeRef{"source", 1}}
	v.Specified = Evidence[bool]{State: EvidenceInvalid, Reason: "invalid-specifiedness", Cause: cause}
	v.Declaration = Evidence[Location]{State: EvidenceInvalid, Reason: "invalid-declaration", Cause: cause}
	v.AncestryComplete = Evidence[bool]{State: EvidenceInvalid, Reason: "invalid-ancestry", Cause: cause}
	v.Use = stateKnown(cloneLocation(v.Physical.Value))
	v.Use.Value.Entry = "entry.xml"
	v.Use.Value.Path = "/" + strings.Repeat("long-path/", 520)
	v.Use.Reason = "observed-site"
	v.Ancestry = []SourceOriginStep{
		{Entity: "one", Declaration: v.Declaration, Use: v.Use},
		{Entity: "two", Declaration: v.Physical, Use: Evidence[Location]{State: EvidenceInvalid, Reason: "invalid-use", Cause: cause}},
		{Entity: "three", Declaration: stateUnknown[Location]("not-read"), Use: v.Use},
	}
	return in
}

func TestSourceOriginTypedRetentionLayoutBounds(t *testing.T) {
	for _, tc := range []struct {
		value   any
		reserve int64
	}{
		{SourceOriginContextResult{}, sourceOriginRecordBytes},
		{SourceOriginStep{}, sourceOriginStepBytes},
		{QName{}, sourceOriginQNameBytes},
		{SourcePosition{}, sourceOriginPositionBytes},
		{DecisionRef{}, sourceOriginCauseBytes},
	} {
		typ := reflect.TypeOf(tc.value)
		if int64(typ.Size()) > tc.reserve {
			t.Errorf("%s fixed layout %d exceeds reserve %d", typ, typ.Size(), tc.reserve)
		}
	}
}

// sourceOriginClonedStorage independently totals the root, owned pointer/slice
// backing arrays and string data. Nested struct headers are inline, and repeated
// references are counted separately because the copy does not share them.
func sourceOriginClonedStorage(v reflect.Value, root bool) int64 {
	var bytes int64
	if root {
		bytes = int64(v.Type().Size())
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			bytes += sourceOriginClonedStorage(v.Field(i), false)
		}
	case reflect.Pointer:
		if !v.IsNil() {
			bytes += sourceOriginClonedStorage(v.Elem(), true)
		}
	case reflect.Slice:
		bytes += int64(v.Len()) * int64(v.Type().Elem().Size())
		for i := 0; i < v.Len(); i++ {
			bytes += sourceOriginClonedStorage(v.Index(i), false)
		}
	case reflect.String:
		bytes += int64(v.Len())
	}
	return bytes
}

func TestSourceOriginTypedRetentionAllFieldsAndOwnedCopies(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	in := sourceOriginRichPublication()
	v := in.SourceOrigin
	v.Ancestry = append(v.Ancestry, make([]SourceOriginStep, SourceOriginAncestryLimit-len(v.Ancestry))...)
	for i := 3; i < len(v.Ancestry); i++ {
		v.Ancestry[i] = v.Ancestry[0]
	}
	size, err := measureSourceOriginContext(context.Background(), s, v)
	if err != nil {
		t.Fatal(err)
	}
	actual := sourceOriginClonedStorage(reflect.ValueOf(v).Elem(), true)
	if size < actual {
		t.Errorf("typed charge %d undercounts all owned data %d", size, actual)
	}
	if err := s.publishContextResult(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	out := s.contextResults[0].SourceOrigin
	if !reflect.DeepEqual(v, out) {
		t.Fatal("owned copy changed content")
	}
	v.Attribute.Local = "changed"
	v.Physical.Value.Position.ByteOffset = 999
	v.Use.Value.Position.Line = 999
	v.Specified.Cause.Rule.ID = "changed"
	v.Ancestry[0].Entity = "changed"
	v.Ancestry[1].Declaration.Value.Position.Column = 999
	if out.Attribute.Local != "value" || out.Physical.Value.Position.ByteOffset != 12 || out.Use.Value.Position.Line != 2 || out.Specified.Cause.Rule.ID != "reader" || out.Ancestry[0].Entity != "one" || out.Ancestry[1].Declaration.Value.Position.Column != 1 {
		t.Fatal("caller mutation reached owned payload")
	}
	out.Declaration.Cause.Rule.ID = "owned-change"
	if out.Specified.Cause.Rule.ID != "reader" || out.Ancestry[0].Declaration.Cause.Rule.ID != "reader" {
		t.Fatal("separate evidence sites share mutable causes")
	}
	out.Use.Value.Position.Line = 7
	if out.Ancestry[0].Use.Value.Position.Line != 2 {
		t.Fatal("separate sites share mutable positions")
	}
}

func TestSourceOriginTypedRetentionRichExactAdmissionAndCancellation(t *testing.T) {
	in := sourceOriginRichPublication()
	probe := rationalSession(t, Budgets{})
	ctx := context.Background()
	counter := &rationalCancelContext{Context: ctx, at: 1 << 30}
	if err := probe.publishContextResult(counter, in); err != nil {
		t.Fatal(err)
	}
	bytes := rationalUsage(t, probe, BudgetBufferedBytes)
	steps := rationalUsage(t, probe, BudgetRuleSteps)
	calls := counter.calls
	probe.cleanup()
	for _, kind := range []BudgetKind{BudgetBufferedBytes, BudgetRuleSteps} {
		for _, delta := range []int64{-1, 0} {
			limits := Budgets{}
			if kind == BudgetBufferedBytes {
				limits.MaxBufferedBytes = bytes + delta
			} else {
				limits.MaxRuleSteps = steps + delta
			}
			s := rationalSession(t, limits)
			err := s.publishContextResult(ctx, in)
			if delta == 0 {
				if err != nil || len(s.contextResults) != 1 {
					t.Errorf("exact %s rejected: %v", kind, err)
				}
			} else if !errors.Is(err, ErrResourceLimit) || len(s.contextResults) != 0 || s.retained != 0 {
				t.Errorf("one-under %s: err=%v retained=%d rows=%d", kind, err, s.retained, len(s.contextResults))
			}
			s.cleanup()
		}
	}
	for at := 1; at <= calls; at++ {
		s := rationalSession(t, Budgets{})
		err := s.publishContextResult(&rationalCancelContext{Context: ctx, at: at}, in)
		if !errors.Is(err, context.Canceled) || len(s.contextResults) != 0 || s.retained != 0 {
			t.Errorf("cancel %d/%d: err=%v retained=%d rows=%d", at, calls, err, s.retained, len(s.contextResults))
		}
		s.cleanup()
		if used := rationalUsage(t, s, BudgetBufferedBytes); used != 0 {
			t.Fatalf("cleanup retained %d", used)
		}
	}
}

func TestSourceOriginTypedRetentionCountsEveryOwnedString(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	// Detach repeated pointers before changing one leaf at a time. Each distinct
	// evidence site must account for its own copy, including enum/empty fields.
	in := stateClone(sourceOriginRichPublication()).SourceOrigin
	ctx := context.Background()
	base, err := measureSourceOriginContext(ctx, s, in)
	if err != nil {
		t.Fatal(err)
	}
	var visit func(reflect.Value, string)
	visit = func(v reflect.Value, path string) {
		switch v.Kind() {
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				visit(v.Field(i), path+"."+v.Type().Field(i).Name)
			}
		case reflect.Pointer:
			if !v.IsNil() {
				visit(v.Elem(), path)
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				visit(v.Index(i), path+"[]")
			}
		case reflect.String:
			old := v.String()
			replacement := strings.Repeat("x", len(old)+16384)
			v.SetString(replacement)
			measured, err := measureSourceOriginContext(ctx, s, in)
			v.SetString(old)
			if err != nil {
				t.Fatal(err)
			}
			want := int64(2 * (len(replacement) - len(old)))
			if old == "" {
				want += 16
			}
			if measured-base != want {
				t.Errorf("%s copy charge delta=%d, want=%d", path, measured-base, want)
			}
		}
	}
	visit(reflect.ValueOf(in).Elem(), "origin")
}
