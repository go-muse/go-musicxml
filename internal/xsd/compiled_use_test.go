package xsd

import (
	"context"
	"testing"
)

func TestCompiledUseDescriptorZeroAndUnresolved(t *testing.T) {
	var zero Use
	if zero.IsAttribute() {
		t.Error("zero use became attribute")
	}
	if _, ok := zero.OwnerType(); ok {
		t.Error("zero use gained an owner")
	}
	if _, ok := zero.DeclaredType(); ok {
		t.Error("zero use gained a declared type")
	}
	name := QName{Local: "root"}
	compiled, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{name: {Name: name, Reference: QName{Local: "unresolved"}}}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	use, found := compiled.Root(name)
	if !found {
		t.Fatal("unresolved reference metadata lost")
	}
	if _, ok := use.DeclaredType(); ok {
		t.Error("invented unresolved reference type")
	}
	if _, ok := use.OwnerType(); ok {
		t.Error("global reference has type owner")
	}
}

func TestCompiledUseDescriptorInheritedOwner(t *testing.T) {
	data := inheritedUseFixture(0)
	data.Types[QName{Local: "Base"}].Complex.Attributes = []AttributeSchema{{Name: QName{Local: "a"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}}}}
	compiled, err := Compile(data, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	run, err := NewRun(compiled, &testMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer run.Close()
	run.current = context.Background()
	use := run.attributeUse(&TypeRef{Name: QName{Local: "DerivedA"}}, QName{Local: "a"})
	owner, known := use.OwnerType()
	if !known || owner.Name() != (QName{Local: "Base"}) || !use.IsAttribute() {
		t.Fatalf("inherited attribute reassigned: %+v, %v", owner, known)
	}
	declared, known := use.DeclaredType()
	if !known || declared.Name() != (QName{Space: XSDNamespace, Local: "string"}) {
		t.Fatalf("inherited attribute type: %+v, %v", declared, known)
	}
	base := compiled.namedTypes[QName{Local: "Base"}]
	particle := compiled.particleUses[particleUseKey{base, base.complex.Particle}][0]
	owner, known = particle.OwnerType()
	if !known || owner != base || particle.IsAttribute() {
		t.Fatalf("particle owner lost: %+v, %v", owner, known)
	}
	declared, known = particle.DeclaredType()
	if !known || declared.Name() != (QName{Space: XSDNamespace, Local: "integer"}) {
		t.Fatalf("reference declared type lost: %+v, %v", declared, known)
	}
}
