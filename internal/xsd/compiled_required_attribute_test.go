package xsd

import "testing"

func TestUseRequiredAttributeMetadataIsExactAndImmutable(t *testing.T) {
	input := SchemaData{Elements: map[QName]*ElementSchema{{Local: "root"}: {Name: QName{Local: "root"}, Type: TypeRef{InlineComplex: &ComplexSchema{Form: ComplexDirect, Attributes: []AttributeSchema{
		{Name: QName{Local: "required"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}}, Use: AttributeRequired},
		{Name: QName{Local: "optional"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}}, Use: AttributeOptional},
		{Name: QName{Local: "prohibited"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}}, Use: AttributeProhibited},
	}}}}}}
	compiled, err := Compile(input, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	input.Elements[QName{Local: "root"}].Type.InlineComplex.Attributes[0].Use = AttributeOptional
	count := 0
	for _, use := range compiled.Uses() {
		want := use.IsAttribute() && use.Declaration().Local == "required"
		if use.IsRequiredAttribute() != want {
			t.Errorf("required metadata %s: got %t want %t", use.Declaration().Local, use.IsRequiredAttribute(), want)
		}
		if use.IsRequiredAttribute() {
			count++
		}
	}
	if count != 1 || (Use{}).IsRequiredAttribute() {
		t.Fatal("required use identity lost")
	}
}
