package xsd

import (
	"reflect"
	"testing"
)

func TestSchemaShapeComparesNamesAndKinds(t *testing.T) {
	type name struct{ Space, Local string }
	type swapped struct{ Local, Space string }
	type narrow struct {
		Space string
		Local int
	}
	same := func(a, b any) bool {
		return sameSchemaShape(reflect.TypeOf(a), reflect.TypeOf(b), newShapeMatch())
	}
	if !same(name{}, QName{}) || !same(map[name]*[]uint8{}, map[QName]*[]Derivation{}) {
		t.Fatal("a mirror of the same shape was rejected")
	}
	for label, other := range map[string]any{"field names": swapped{}, "field kinds": narrow{}, "extra field": struct{ Space, Local, Extra string }{}} {
		if same(other, QName{}) {
			t.Errorf("%s: a different shape was accepted", label)
		}
	}
	if same(map[string]bool{}, map[QName]bool{}) || same(map[QName]string{}, map[QName]bool{}) {
		t.Error("map keys or values of another shape were accepted")
	}
	type hidden struct{ space, Local string }
	type split struct {
		Left, Right *QName
	}
	type twin struct{ Space, Local string }
	type mixed struct {
		Left  *QName
		Right *twin
	}
	if same(mixed{}, split{}) {
		t.Error("two source types for one target type were accepted")
	}
	if same(hidden{}, hidden{}) {
		t.Error("an unexported field was accepted")
	}
}
