package validation

import (
	"reflect"
	"testing"
)

func schemaUseFixture() SchemaData {
	stringType := SchemaTypeRef{Name: SchemaName{Space: "http://www.w3.org/2001/XMLSchema", Local: "string"}}
	anonymousSimple := &SchemaSimple{Form: SchemaSimpleRestriction, Base: &SchemaSimpleMember{Name: stringType.Name}}
	named := &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{{Name: SchemaName{Local: "id"}, Type: stringType}}, Particle: &SchemaParticle{Kind: SchemaParticleSequence, Children: []*SchemaParticle{
		{Kind: SchemaParticleElement, Element: &SchemaElement{Name: SchemaName{Local: "same"}, Type: stringType}},
		{Kind: SchemaParticleElement, Element: &SchemaElement{Name: SchemaName{Local: "same"}, Type: SchemaTypeRef{InlineSimple: anonymousSimple}}},
		{Kind: SchemaParticleElement, Element: &SchemaElement{Reference: SchemaName{Local: "global"}}},
	}}}
	anonymous := &SchemaComplex{Form: SchemaComplexDirect, Attributes: []SchemaAttribute{{Name: SchemaName{Local: "anonymous-owner"}, Type: stringType}}, Particle: &SchemaParticle{Kind: SchemaParticleElement, Element: &SchemaElement{Name: SchemaName{Local: "child"}, Type: SchemaTypeRef{InlineComplex: &SchemaComplex{Form: SchemaComplexDirect}}}}}
	return SchemaData{Types: map[SchemaName]*SchemaType{{Local: "T"}: {Complex: named}}, Elements: map[SchemaName]*SchemaElement{
		{Local: "root"}:   {Name: SchemaName{Local: "root"}, Type: SchemaTypeRef{Name: SchemaName{Local: "T"}}},
		{Local: "anon"}:   {Name: SchemaName{Local: "anon"}, Type: SchemaTypeRef{InlineComplex: anonymous}},
		{Local: "global"}: {Name: SchemaName{Local: "global"}, Type: stringType},
	}}
}

func TestSchemaUseBaseline(t *testing.T) {
	schema, err := CompileSchema(AssemblyMusicXML, schemaUseFixture())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := schema.Digest(), "14626735aec29fd64602cb5a0d4981087d5c94731abebe574f7d8fc527392d9a"; got != want {
		t.Errorf("schema metadata digest changed: %s != %s", got, want)
	}
	paths := [][2]string{
		{"/elements/0", "/elements/0"},
		{"/elements/1", "/elements/1"},
		{"/elements/2", "/elements/2"},
		{"/types/0", "/types/0/attributes/0"},
		{"/types/0", "/types/0/records/3/children/0/element"},
		{"/types/0", "/types/0/records/3/children/1/element"},
		{"/types/0", "/types/0/records/3/children/2/element"},
		{"/records/13", "/records/13/attributes/0"},
		{"/records/13", "/records/13/particle/element"},
	}
	var want []DeclarationID
	for _, path := range paths {
		want = append(want, DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: path[0], Occurrence: path[1]})
	}
	if got := schema.Declarations(); !reflect.DeepEqual(got, want) {
		t.Errorf("existing declaration identities changed: %+v != %+v", got, want)
	}
}

func TestCompiledSchemaUseDescriptors(t *testing.T) {
	schema, err := CompileSchema(AssemblyMusicXML, schemaUseFixture())
	if err != nil {
		t.Fatal(err)
	}
	uses := schema.Uses()
	declarations := schema.Declarations()
	if len(uses) != len(declarations) {
		t.Fatalf("uses=%d declarations=%d", len(uses), len(declarations))
	}
	var anonymousOwner ComponentID
	for i, use := range uses {
		if use.Declaration != declarations[i] {
			t.Errorf("declaration %d changed: %+v != %+v", i, use.Declaration, declarations[i])
		}
		if use.Name == (QName{Local: "anon"}) && !use.OwnerTypeKnown {
			anonymousOwner = use.DeclaredType
		}
	}
	if anonymousOwner.Name != (QName{}) || anonymousOwner.Occurrence == "" {
		t.Fatalf("anonymous owner not retained: %+v", anonymousOwner)
	}
	var duplicate []SchemaUse
	var reference SchemaUse
	for _, use := range uses {
		if !use.DeclaredTypeKnown {
			t.Errorf("declared type unavailable: %+v", use)
		}
		switch use.Name.Local {
		case "root":
			if use.Attribute || use.OwnerTypeKnown || use.DeclaredType.Name != (QName{Local: "T"}) {
				t.Errorf("global root: %+v", use)
			}
		case "id":
			if !use.Attribute || !use.OwnerTypeKnown || use.OwnerType.Name != (QName{Local: "T"}) || use.DeclaredType.Name.Local != "string" {
				t.Errorf("named attribute owner: %+v", use)
			}
		case "anonymous-owner", "child":
			if !use.OwnerTypeKnown || use.OwnerType != anonymousOwner {
				t.Errorf("anonymous owner inferred incorrectly: %+v", use)
			}
			if use.Name.Local == "child" && (use.DeclaredType.Name != (QName{}) || use.DeclaredType.Occurrence == "") {
				t.Errorf("anonymous declared type: %+v", use)
			}
		case "same":
			duplicate = append(duplicate, use)
		case "global":
			if use.OwnerTypeKnown {
				reference = use
			}
		}
	}
	if len(duplicate) != 2 || duplicate[0].Declaration == duplicate[1].Declaration || duplicate[0].OwnerType != duplicate[1].OwnerType {
		t.Fatalf("same-name use identities collapsed: %+v", duplicate)
	}
	if duplicate[0].DeclaredType.Name.Local != "string" || duplicate[1].DeclaredType.Name != (QName{}) || duplicate[1].DeclaredType.Occurrence == "" {
		t.Errorf("named/anonymous types: %+v", duplicate)
	}
	if reference.DeclaredType.Name.Local != "string" || reference.OwnerType.Name.Local != "T" {
		t.Errorf("resolved reference metadata: %+v", reference)
	}
	original := append([]SchemaUse(nil), uses...)
	uses[0] = SchemaUse{}
	if !reflect.DeepEqual(schema.Uses(), original) {
		t.Fatal("descriptor slice mutated compiled schema")
	}
	if !reflect.DeepEqual(schema.Declarations(), declarations) {
		t.Fatal("descriptors changed declaration identities")
	}
	for _, schema := range []*CompiledSchema{nil, {}} {
		if schema.Uses() != nil {
			t.Fatal("zero schema exposed descriptors")
		}
	}
}
