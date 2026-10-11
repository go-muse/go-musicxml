package validation

import (
	"math/big"
	"reflect"
	"testing"
)

func TestTypedPlanConstructorsRejectInvalidParameters(t *testing.T) {
	declaration := DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "owner", Occurrence: "child"}
	for _, bounds := range [][2]int64{{-1, 1}, {2, 1}, {0, -2}} {
		if _, err := NewChildCountPlan([]DeclarationID{declaration}, bounds[0], bounds[1]); err == nil {
			t.Errorf("accepted bounds%v", bounds)
		}
	}
	if _, err := NewChildCountPlan(nil, 1, -1); err == nil {
		t.Fatal("empty child selection accepted")
	}
	if _, err := NewChildCountPlan([]DeclarationID{declaration, declaration}, 1, -1); err == nil {
		t.Fatal("duplicate child selection accepted")
	}
	if _, err := NewAttributePresencePlan(AttributeSelector{Name: QName{Local: "id"}}, true); err == nil {
		t.Fatal("attribute without exact declaration accepted")
	}
	if _, err := NewAtomicComparisonPlan(ComparisonEqual, Atomic{Kind: AtomicInteger}); err == nil {
		t.Fatal("uninitialized numeric bound accepted")
	}
	if _, err := NewAtomicComparisonPlan("unknown", Atomic{Kind: AtomicString, Text: "x"}); err == nil {
		t.Fatal("unknown comparison accepted")
	}
	if _, err := NewAtomicComparisonPlan(ComparisonLess, Atomic{Kind: AtomicQName, Name: QName{Local: "x"}}); err == nil {
		t.Fatal("QName ordering accepted")
	}
	if _, err := NewContextValuePlan("", RelationDocument, Atomic{Kind: AtomicBoolean, Text: "true"}); err == nil {
		t.Fatal("empty context key accepted")
	}
	if _, err := NewContextValuePlan("fact", RelationMusicalPosition, Atomic{Kind: AtomicBoolean, Text: "true"}); err == nil {
		t.Fatal("unimplemented context storage relation accepted")
	}
	if _, err := NewIdentityDeclarationPlan("", AttributeSelector{Name: QName{Local: "id"}, Declaration: declaration}); err == nil {
		t.Fatal("empty identity index accepted")
	}
}
func TestTypedPlansOwnParametersAndNumericValues(t *testing.T) {
	declarations := []DeclarationID{{Assembly: AssemblyMusicXML, OwnerOccurrence: "owner", Occurrence: "child"}}
	plan, err := NewChildCountPlan(declarations, 1, -1)
	if err != nil {
		t.Fatal(err)
	}
	before := plan.digestValue()
	declarations[0].Occurrence = "changed"
	after := plan.digestValue()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("plan retained caller declarations")
	}
	integer, err := NewInteger(new(big.Int).Lsh(big.NewInt(1), 64))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewAtomicComparisonPlan(ComparisonGreater, Atomic{Kind: AtomicInteger, Integer: integer}); err != nil {
		t.Fatal(err)
	}
}
