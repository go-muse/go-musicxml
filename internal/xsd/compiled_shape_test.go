package xsd

import (
	"sync"
	"testing"
)

func shapeType(t *testing.T, ref TypeRef) TypeID {
	t.Helper()
	c, e := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "root"}: {Name: QName{Local: "root"}, Type: ref}}}, CompileLimits{})
	if e != nil {
		t.Fatal(e)
	}
	root, _ := c.Root(QName{Local: "root"})
	typ, ok := root.DeclaredType()
	if !ok {
		t.Fatal("declared type missing")
	}
	return typ
}
func shapeRestriction(base string) *SimpleSchema {
	return &SimpleSchema{Form: SimpleRestriction, Base: &SimpleMember{Name: QName{Space: XSDNamespace, Local: base}}}
}
func TestCompiledContentShapeFamiliesAndSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ref   TypeRef
		want  TypeContentShape
		known bool
	}{
		{"integer", TypeRef{InlineSimple: shapeRestriction("positiveInteger")}, ShapeInteger, true},
		{"decimal", TypeRef{InlineSimple: shapeRestriction("decimal")}, ShapeDecimal, true},
		{"string", TypeRef{InlineSimple: shapeRestriction("token")}, ShapeString, true},
		{"union", TypeRef{InlineSimple: &SimpleSchema{Form: SimpleUnion, Members: []*SimpleMember{{Name: QName{Space: XSDNamespace, Local: "decimal"}}, {Inline: shapeRestriction("string")}}}}, ShapeUnion | ShapeString | ShapeDecimal, true},
		{"elements", TypeRef{InlineComplex: &ComplexSchema{Form: ComplexDirect}}, ShapeComplex | ShapeElements, true},
		{"scalar complex", TypeRef{InlineComplex: &ComplexSchema{Form: ComplexSimpleContentExtension, Base: &TypeRef{InlineSimple: shapeRestriction("integer")}}}, ShapeComplex | ShapeInteger, true},
		{"element extension", TypeRef{InlineComplex: &ComplexSchema{Form: ComplexComplexContentExtension, Base: &TypeRef{InlineComplex: &ComplexSchema{Form: ComplexDirect}}}}, ShapeComplex | ShapeElements, true},
		{"element extension of scalar", TypeRef{InlineComplex: &ComplexSchema{Form: ComplexComplexContentExtension, Base: &TypeRef{InlineSimple: shapeRestriction("decimal")}}}, 0, false},
		{"element extension of mixed", TypeRef{InlineComplex: &ComplexSchema{Form: ComplexComplexContentExtension, Base: &TypeRef{InlineComplex: &ComplexSchema{Form: ComplexDirect, Mixed: true}}}}, 0, false},
		{"mixed", TypeRef{InlineComplex: &ComplexSchema{Form: ComplexDirect, Mixed: true}}, 0, false},
		{"list", TypeRef{InlineSimple: &SimpleSchema{Form: SimpleList, Item: &SimpleMember{Name: QName{Space: XSDNamespace, Local: "integer"}}}}, 0, false},
		{"other primitive", TypeRef{Name: QName{Space: XSDNamespace, Local: "boolean"}}, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			typ := shapeType(t, tc.ref)
			if got, ok := typ.ContentShape(); got != tc.want || ok != tc.known {
				t.Fatalf("shape=%v,%v want %v,%v", got, ok, tc.want, tc.known)
			}
		})
	}
	if shape, ok := (TypeID{}).ContentShape(); shape != 0 || ok {
		t.Fatal("zero type has shape")
	}
	source := shapeRestriction("integer")
	typ := shapeType(t, TypeRef{InlineSimple: source})
	source.Base.Name.Local = "string"
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 20 {
				if shape, ok := typ.ContentShape(); !ok || shape != ShapeInteger {
					t.Error("shared immutable type shape changed")
				}
			}
		})
	}
	wg.Wait()
}
func TestCompiledContentShapeBoundsAndCycles(t *testing.T) {
	for _, count := range []int{128, 129} {
		var base = &SimpleMember{Name: QName{Space: XSDNamespace, Local: "integer"}}
		for range count {
			base = &SimpleMember{Inline: &SimpleSchema{Form: SimpleRestriction, Base: base}}
		}
		typ := shapeType(t, TypeRef{InlineSimple: base.Inline})
		_, ok := typ.ContentShape()
		if ok != (count == 128) {
			t.Fatalf("depth %d supported=%v", count, ok)
		}
	}
	for _, count := range []int{511, 512} {
		members := make([]*SimpleMember, count)
		for i := range members {
			members[i] = &SimpleMember{Name: QName{Space: XSDNamespace, Local: "integer"}}
		}
		typ := shapeType(t, TypeRef{InlineSimple: &SimpleSchema{Form: SimpleUnion, Members: members}})
		_, ok := typ.ContentShape()
		if ok != (count == 511) {
			t.Fatalf("edges %d supported=%v", count, ok)
		}
	}
	loop := &SimpleSchema{Form: SimpleRestriction}
	loop.Base = &SimpleMember{Inline: loop}
	if _, ok := shapeType(t, TypeRef{InlineSimple: loop}).ContentShape(); ok {
		t.Fatal("cyclic scalar metadata gained a shape")
	}
}

func TestCompiledContentShapeComplexExtensionBoundsAndCycles(t *testing.T) {
	for _, count := range []int{128, 129} {
		base := &ComplexSchema{Form: ComplexDirect}
		for range count {
			base = &ComplexSchema{Form: ComplexComplexContentExtension, Base: &TypeRef{InlineComplex: base}}
		}
		_, known := shapeType(t, TypeRef{InlineComplex: base}).ContentShape()
		if known != (count == 128) {
			t.Fatalf("extension depth %d known=%v", count, known)
		}
	}
	loop := &ComplexSchema{Form: ComplexComplexContentExtension}
	loop.Base = &TypeRef{InlineComplex: loop}
	if _, known := shapeType(t, TypeRef{InlineComplex: loop}).ContentShape(); known {
		t.Fatal("cyclic element extension gained a shape")
	}
}
