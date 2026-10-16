package validation

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"unsafe"
)

// Compile-time envelopes are checked even by cross-architecture compile-only
// tests. The new sizing path uses the actual layout, while these assertions
// also keep the existing closed payload reserves honest on native and 386.
var (
	_ [sourceOriginRecordBytes - int64(unsafe.Sizeof(SourceOriginContextResult{}))]struct{}
	_ [sourceOriginStepBytes - int64(unsafe.Sizeof(SourceOriginStep{}))]struct{}
	_ [sourceOriginQNameBytes - int64(unsafe.Sizeof(QName{}))]struct{}
	_ [sourceOriginPositionBytes - int64(unsafe.Sizeof(SourcePosition{}))]struct{}
	_ [sourceOriginCauseBytes - int64(unsafe.Sizeof(DecisionRef{}))]struct{}
)

func TestResourceRecordMeasurementCoversClosedCloneShapes(t *testing.T) {
	origin := sourceOriginRichPublication().SourceOrigin
	site := origin.Use
	site.Cause = origin.Specified.Cause
	origin.Physical, origin.Declaration, origin.Use = site, site, site
	origin.Ancestry = make([]SourceOriginStep, SourceOriginAncestryLimit, SourceOriginAncestryLimit+10)
	for i := range origin.Ancestry {
		origin.Ancestry[i] = SourceOriginStep{Entity: "aliased-entity", Declaration: site, Use: site}
	}
	for _, value := range []any{SourceOriginContextResult{}, SourceOriginContextResult{Ancestry: make([]SourceOriginStep, 0, 10)}, *origin} {
		t.Run(reflect.TypeOf(value).Name()+fmtRecordShape(value), func(t *testing.T) {
			a, err := NewBudgetAccount(Budgets{MaxRuleSteps: 1 << 40})
			if err != nil {
				t.Fatal(err)
			}
			v := reflect.ValueOf(value)
			measured, err := resourceMeasure(context.Background(), a, v)
			if err != nil {
				t.Fatal(err)
			}
			// Independent storage oracle follows every concrete allocation, including
			// aliased source pointers and string contents at every cloned occurrence.
			concrete := sourceOriginClonedStorage(v, true)
			if measured < concrete {
				t.Fatalf("charge=%d concrete=%d", measured, concrete)
			}
			copy, err := resourceCopyAs(context.Background(), v, "source", strings.Repeat("adopted", 100))
			if err != nil {
				t.Fatal(err)
			}
			remapped, err := resourceMeasureAs(context.Background(), a, v, strings.Repeat("adopted", 100))
			if err != nil {
				t.Fatal(err)
			}
			ancestry := copy.FieldByName("Ancestry")
			if ancestry.Cap() != ancestry.Len() || ancestry.IsNil() != v.FieldByName("Ancestry").IsNil() {
				t.Fatal("copy changed slice nilness or retained uncharged source spare capacity")
			}
			if remapped < sourceOriginClonedStorage(copy, true) {
				t.Fatalf("remap charge=%d storage=%d", remapped, sourceOriginClonedStorage(copy, true))
			}
			if measured < int64(v.Type().Size())+32 {
				t.Fatal("missing root storage")
			}
			t.Logf("fixed=%d clone storage=%d charge=%d remap charge=%d", v.Type().Size(), concrete, measured, remapped)
		})
	}
}
func fmtRecordShape(v any) string {
	if reflect.ValueOf(v).IsZero() {
		return "/zero"
	}
	if reflect.ValueOf(v).FieldByName("Ancestry").Len() == 0 {
		return "/empty-nonnil"
	}
	return "/populated"
}

func TestResourceRecordMeasurementCountsEveryClonedString(t *testing.T) {
	for _, value := range []any{*sourceOriginRichPublication().SourceOrigin} {
		copy, err := resourceCopyAs(context.Background(), reflect.ValueOf(value), "", "")
		if err != nil {
			t.Fatal(err)
		}
		a, err := NewBudgetAccount(Budgets{MaxRuleSteps: 1 << 40})
		if err != nil {
			t.Fatal(err)
		}
		base, err := resourceMeasure(context.Background(), a, copy)
		if err != nil {
			t.Fatal(err)
		}
		var visit func(reflect.Value, string)
		visit = func(v reflect.Value, path string) {
			if _, ok := resourceNumericFields[v.Type()]; ok {
				return
			}
			switch v.Kind() {
			case reflect.Struct:
				for i := 0; i < v.NumField(); i++ {
					visit(v.Field(i), path+"."+v.Type().Field(i).Name)
				}
			case reflect.Pointer:
				if !v.IsNil() {
					visit(v.Elem(), path+"*")
				}
			case reflect.Slice:
				for i := 0; i < v.Len(); i++ {
					visit(v.Index(i), path+"[]")
				}
			case reflect.String:
				old := v.String()
				v.SetString(old + strings.Repeat("x", 777))
				got, e := resourceMeasure(context.Background(), a, copy)
				v.SetString(old)
				want := int64(2 * 777)
				if old == "" {
					want += 16
				}
				if e != nil || got-base != want {
					t.Errorf("%s added=%d want=%d err=%v", path, got-base, want, e)
				}
			}
		}
		visit(copy, copy.Type().Name())
	}
}

func TestResourceRecordMeasurementDoesNotBoxFields(t *testing.T) {
	value := *sourceOriginRichPublication().SourceOrigin
	v := reflect.ValueOf(&value).Elem()
	a, err := NewBudgetAccount(Budgets{MaxRuleSteps: 1 << 40})
	if err != nil {
		t.Fatal(err)
	}
	if allocations := testing.AllocsPerRun(10, func() {
		if _, err := resourceMeasure(context.Background(), a, v); err != nil {
			panic(err)
		}
	}); allocations != 0 {
		t.Fatalf("per-field measurement allocations=%g", allocations)
	}
}
