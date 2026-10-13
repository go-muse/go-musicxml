package musicxml

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validationIdentityCases() []validationSourceCase {
	var tests []validationSourceCase
	add := func(name, element, attrs, content string, comparable bool, issues ...string) {
		source := `<` + element + ` xmlns:i="` + validationXSINamespace + `" xmlns:xs="` + validationXSDNamespace + `"` + attrs + `>` + content + `</` + element + `>`
		tests = append(tests, validationSourceCase{name: name, source: source, oracleComparable: comparable, issues: issues})
	}
	add("dynamic target backward", "root", "", `<item><target i:type="xs:ID"> target </target></item><ref>target</ref>`, true)
	add("dynamic target forward", "root", "", `<ref>target</ref><item><target i:type="xs:ID"> target </target></item>`, true)
	add("dynamic restricted target", "root", "", `<ref>T1</ref><item><target i:type="RestrictedID"> T1 </target></item>`, true)
	add("declared restricted target", "root", "", `<ref>T1</ref><item><restricted>T1</restricted></item>`, true)
	add("ordinary string", "root", "", `<item><target>target</target></item><ref>target</ref>`, false, "/root/ref:IDREF")
	add("ordinary NCName", "root", "", `<item><target i:type="xs:NCName">target</target></item><ref>target</ref>`, false, "/root/ref:IDREF")
	add("two seeded owners", "root", "", `<item><target i:type="xs:ID">target</target></item><item><target i:type="xs:ID">target</target></item><ref>target</ref>`, false, "/root/item[2]/target:ID")
	add("three seeded owners", "root", "", `<ref>target</ref><item><target i:type="xs:ID">target</target></item><item><target i:type="xs:ID">target</target></item><item><target i:type="xs:ID">target</target></item>`, false, "/root/item[2]/target:ID")
	add("two unseeded owners", "root", "", `<item><target i:type="xs:ID">target</target></item><item><target i:type="xs:ID">target</target></item>`, true)
	add("dynamic references unseeded", "root", "", `<dynamic i:type="xs:IDREF">absent</dynamic><dynamic i:type="xs:IDREFS">absent absent</dynamic>`, true)
	add("same parent dynamic", "root", "", `<item><target i:type="xs:ID">target</target><target i:type="xs:ID">target</target></item><ref>target</ref>`, true)
	add("same parent declared", "root", "", `<item><declared>target</declared><declared>target</declared></item>`, true)
	add("attribute plus child one owner", "root", "", `<item id="target"><declared>target</declared></item><ref>target</ref>`, true)
	add("attribute distinct owners", "root", "", `<item id="target"/><item id="target"/>`, true, "/root/item[2]/@id:ID")
	add("root ID lacks owner", "id", "", "target", false, "/id:ID")
	add("root dynamic ID unseeded", "text", ` i:type="xs:ID"`, "target", true)
	add("root reference absent", "ref", "", "target", false, "/ref:IDREF")
	add("root content matching attribute", "content", ` code="ok" id="target"`, "target", true)
	add("root content unrelated attribute", "content", ` code="ok" id="other"`, "target", false, "/content:ID")
	add("invalid lexical candidate", "root", "", `<item><target i:type="xs:ID">two words</target></item><ref>target</ref>`, true, "/root/item/target:datatype", "/root/ref:IDREF")
	add("invalid facet candidate", "root", "", `<item><target i:type="RestrictedID">target</target></item><ref>target</ref>`, true, "/root/item/target:pattern", "/root/ref:IDREF")
	add("invalid simple attribute candidate", "root", "", `<item><target i:type="xs:ID" bad="x">target</target></item><ref>target</ref>`, true, "/root/item/target/@bad:attribute", "/root/ref:IDREF")
	add("invalid simple children candidate", "root", "", `<item><target i:type="xs:ID">target<child/></target></item><ref>target</ref>`, true, "/root/item/target:simple-content")
	add("invalid complex attributes candidate", "root", "", `<item><content>target</content></item><ref>target</ref>`, true, "/root/item/content/@code:required", "/root/ref:IDREF")
	add("valid complex candidate", "root", "", `<item><content code="ok">target</content></item><ref>target</ref>`, true)
	add("invalid parent preserves child", "root", "", `<item bad="x"><target i:type="xs:ID">target</target></item><ref>target</ref>`, true, "/root/item/@bad:attribute")
	add("invalid parent preserves attribute", "root", "", `<item bad="x" id="target"/><ref>target</ref>`, true, "/root/item/@bad:attribute")
	add("fixed mismatch preserves actual ID", "root", "", `<item><fixed i:type="xs:ID">target</fixed></item><ref>target</ref>`, true, "/root/item/fixed:fixed")
	add("nil scalar no candidate", "root", "", `<item><target i:type="xs:ID" i:nil="true"/></item><ref>target</ref>`, false, "/root/ref:IDREF")
	add("nil declared no seed", "root", "", `<item><declared i:nil="true"/></item>`, true)
	add("nil owner keeps attributes", "root", "", `<item i:nil="true" id="target"/><ref>target</ref>`, true)
	add("dynamic default candidate", "root", "", `<item><defaulted i:type="xs:ID"/></item><ref>target</ref>`, true)
	add("default and attribute same owner", "root", "", `<item id="target"><defaulted i:type="xs:ID"/></item><ref>target</ref>`, true)
	add("absent attribute defaults excluded", "root", "", `<item/>`, true)
	add("union member no binding", "root", "", `<item><union>target</union></item><ref>target</ref>`, false, "/root/ref:IDREF")
	add("list item no binding", "root", "", `<item><list>target</list></item><ref>target</ref>`, false, "/root/ref:IDREF")
	add("constructed references excluded", "root", "", `<ref-union>absent</ref-union><ref-list>absent absent</ref-list>`, true)
	add("repeated references bound", "root", "", `<refs>target target</refs><ref>target</ref><item id="target"/>`, true)
	add("repeated references absent", "root", "", `<refs>absent absent</refs><ref>absent</ref>`, false, "/root/refs:IDREF", "/root/refs:IDREF", "/root/ref:IDREF")
	add("nonnillable false keeps candidate", "root", "", `<item><nonnil i:nil="false">target</nonnil></item><ref>target</ref>`, true, "/root/item/nonnil/@{"+validationXSINamespace+"}nil:nillable")
	add("nonnillable true keeps candidate", "root", "", `<item><nonnil i:nil="true">target</nonnil></item><ref>target</ref>`, true, "/root/item/nonnil/@{"+validationXSINamespace+"}nil:nillable")
	add("malformed nil keeps candidate", "root", "", `<item><declared i:nil="maybe">target</declared></item><ref>target</ref>`, true, "/root/item/declared/@{"+validationXSINamespace+"}nil:datatype")
	add("malformed nil keeps root seed", "nullable-id", ` i:nil="maybe"`, "target", true, "/nullable-id/@{"+validationXSINamespace+"}nil:datatype", "/nullable-id:ID")
	add("malformed hint keeps root seed", "id", ` i:noNamespaceSchemaLocation="%wrong"`, "target", false, "/id/@{"+validationXSINamespace+"}noNamespaceSchemaLocation:datatype", "/id:ID")
	add("invalid nomination declared fallback binds", "root", "", `<item><nonnil i:type="xs:integer">target</nonnil></item><ref>target</ref>`, true, "/root/item/nonnil/@{"+validationXSINamespace+"}type:xsi:type")
	add("invalid broader nomination keeps declared restriction", "root", "", `<item><restricted i:type="xs:ID">T1</restricted></item><ref>T1</ref>`, true, "/root/item/restricted/@{"+validationXSINamespace+"}type:xsi:type")
	add("fixed reference does not seed", "root", "", `<fixed-ref>absent</fixed-ref>`, true, "/root/fixed-ref:fixed")
	add("fixed attribute reference does not seed", "root", "", `<item fixed-ref="absent"/>`, true, "/root/item/@fixed-ref:fixed")
	add("restricted reference seeded", "root", "", `<restricted-ref>T1</restricted-ref><item><target i:type="xs:ID">T1</target></item>`, true)
	add("restricted reference absent", "root", "", `<restricted-ref>T1</restricted-ref>`, false, "/root/restricted-ref:IDREF")
	add("restricted references seeded", "root", "", `<restricted-refs>T1 T1</restricted-refs><item id="T1"/>`, true)
	add("complex reference seeded", "root", "", `<content-ref>T1</content-ref><item id="T1"/>`, true)
	add("complex reference absent", "root", "", `<content-ref>T1</content-ref>`, false, "/root/content-ref:IDREF")
	add("invalid attribute ID", "root", "", `<item id="9bad"/><ref>target</ref>`, true, "/root/item/@id:datatype", "/root/ref:IDREF")
	add("invalid reference never seeds", "root", "", `<ref>two words</ref><item><target i:type="xs:ID">target</target></item><item><target i:type="xs:ID">target</target></item>`, true, "/root/ref:datatype")
	add("complex failure preserves own attribute", "root", "", `<item><content id="target">invalid content</content></item><ref>target</ref>`, true, "/root/item/content/@code:required", "/root/item/content:datatype")
	add("defaulted complex attribute owner", "default-content", ` id="target"`, "", true)
	add("abstract declaration preserves ID candidate", "root", "", `<item><abstract-id>target</abstract-id></item><ref>target</ref>`, true, "/root/item/abstract-id:abstract")
	add("abstract actual type no candidate", "root", "", `<item><abstract-content>target</abstract-content></item><ref>target</ref>`, true, "/root/item/abstract-content:abstract", "/root/ref:IDREF")
	return tests
}

func TestXSD10IdentityBindings(t *testing.T) {
	for _, test := range validationIdentityCases() {
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationIdentityGenerated, test.source), test.issues)
		})
	}
}

func TestXSD10IdentityBindingsAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	directory, err := filepath.Abs(filepath.Join("testdata", "validation"))
	require.NoError(t, err)
	for _, test := range validationIdentityCases() {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertValidationIssues(t, validateAttributeSource(t, &validationIdentityGenerated, test.source), test.issues)
			if test.oracleComparable {
				assertXMLLintOutcome(t, xmllint, filepath.Join(directory, "identity-contract.xsd"), filepath.Join(directory, "catalog.xml"), test.source, len(test.issues) == 0)
			}
		})
	}
}

func TestXSD10IdentitySharedInput(t *testing.T) {
	source := `<root xmlns:i="` + validationXSINamespace + `" xmlns:xs="` + validationXSDNamespace + `"><refs>target target</refs><item id="target"><defaulted i:type="xs:ID"/></item></root>`
	root, err := parseValidationDocument([]byte(source))
	require.NoError(t, err)
	before := snapshotValidationNode(t, root)
	schemaBefore := snapshotValidationSchema(t, &validationIdentityGenerated)
	var workers sync.WaitGroup
	for range 16 {
		workers.Go(func() {
			for range 8 {
				c := newTestValidationContext(&validationIdentityGenerated)
				c.ValidateElement(root, validationIdentityGenerated.Elements[validationQName{Local: "root"}], "/root")
				c.ValidateIdentityReferences()
				assert.NoError(t, c.result())
			}
		})
	}
	workers.Wait()
	after := snapshotValidationNode(t, root)
	assert.Equal(t, before, after)
	assert.Equal(t, "", root.Children[1].Children[0].Text.String())
	assert.Equal(t, schemaBefore, snapshotValidationSchema(t, &validationIdentityGenerated))
	missing := validateAttributeSource(t, &validationIdentityGenerated, `<ref>target</ref>`)
	assertValidationIssues(t, missing, []string{"/ref:IDREF"})
}

func TestXSD10IdentityLocalGates(t *testing.T) {
	// Separate the successful declaration seed facts from actual-type bindings.
	for _, test := range []struct {
		name, content   string
		seeds, bindings int
	}{
		{"abstract declaration", `<abstract-id>target</abstract-id>`, 0, 1},
		{"nonnillable false", `<nonnil i:nil="false">target</nonnil>`, 0, 1},
		{"nonnillable true", `<nonnil i:nil="true">target</nonnil>`, 0, 1},
		{"invalid nomination", `<nonnil i:type="xs:integer">target</nonnil>`, 0, 1},
		{"malformed nil", `<declared i:nil="maybe">target</declared>`, 1, 1},
		{"malformed hint", `<declared i:noNamespaceSchemaLocation="%wrong">target</declared>`, 1, 1},
		{"illegal simple attribute", `<declared bad="x">target</declared>`, 0, 0},
		{"illegal complex attribute", `<content>target</content>`, 0, 0},
		{"complex own attribute independent", `<content id="target">bad content</content>`, 1, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := `<root xmlns:i="` + validationXSINamespace + `" xmlns:xs="` + validationXSDNamespace + `"><item>` + test.content + `</item></root>`
			c := validateAttributeSource(t, &validationIdentityGenerated, source)
			assert.Len(t, c.IdentityEntries, test.seeds)
			assert.Len(t, c.IdentityBindings, test.bindings)
		})
	}
}

func TestXSD10IdentityOriginalDefaultOwner(t *testing.T) {
	root, err := parseValidationDocument([]byte(`<default-content id="target"/>`))
	require.NoError(t, err)
	c := newTestValidationContext(&validationIdentityGenerated)
	c.ValidateElement(root, validationIdentityGenerated.Elements[validationQName{Local: "default-content"}], "/default-content")
	c.ValidateIdentityReferences()
	require.NoError(t, c.result())
	require.NotNil(t, c.IdentityBindings["target"])
	assert.Contains(t, c.IdentityBindings["target"].Owners, root, "effective scalar view must not become an owner")
	assert.Empty(t, root.Text.String())
}

func TestXSD10IdentityIncompleteTraversal(t *testing.T) {
	set := validationIdentityGenerated
	set.Types = maps.Clone(set.Types)
	set.Types[validationQName{Local: "IDList"}] = &validationTypeSchema{}
	for _, test := range []struct {
		name, content string
		duplicates    int
	}{
		{"absent binding unknown", `<ref>absent</ref><item><list>absent</list></item>`, 0},
		{"duplicates proved before incomplete", `<ref>target</ref><item><target i:type="xs:ID">target</target></item><item><target i:type="xs:ID">target</target></item><item><list>target</list></item>`, 1},
		{"duplicates proved after incomplete", `<item><list>target</list></item><item><target i:type="xs:ID">target</target></item><item><target i:type="xs:ID">target</target></item><ref>target</ref>`, 1},
		{"unseeded duplicates remain unseeded", `<item><list>target</list></item><item><target i:type="xs:ID">target</target></item><item><target i:type="xs:ID">target</target></item>`, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := assessAttributeSource(t, &set, `<root xmlns:i="`+validationXSINamespace+`" xmlns:xs="`+validationXSDNamespace+`">`+test.content+`</root>`)
			require.Error(t, c.AssessmentErr)
			assert.Len(t, c.Issues, test.duplicates)
			for _, issue := range c.Issues {
				assert.Equal(t, "ID", issue.Constraint)
			}
		})
	}
}

func TestXSD10IdentityOwnerNamespaceCollision(t *testing.T) {
	builtin := func(name string) validationTypeRef {
		return validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: name}}
	}
	target := &validationElementSchema{Name: validationQName{Local: "target"}, Type: builtin("ID")}
	element := func(declaration *validationElementSchema) *validationParticleSchema {
		return &validationParticleSchema{Kind: validationParticleElement, Occurrence: validationOccurrence{Min: 1, Max: 1}, Element: declaration}
	}
	child := func(namespace string) *validationElementSchema {
		return &validationElementSchema{Name: validationQName{Space: namespace, Local: "item"}, Type: validationTypeRef{InlineComplex: &validationComplexSchema{Form: validationComplexDirect, Particle: element(target)}}}
	}
	root := &validationElementSchema{Name: validationQName{Local: "root"}, Type: validationTypeRef{InlineComplex: &validationComplexSchema{Form: validationComplexDirect, Particle: &validationParticleSchema{Kind: validationParticleSequence, Occurrence: validationOccurrence{Min: 1, Max: 1}, Children: []*validationParticleSchema{element(child("urn:first")), element(child("urn:second"))}}}}}
	set := &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{{Local: "root"}: root}}
	c := validateAttributeSource(t, set, `<root xmlns:a="urn:first" xmlns:b="urn:second"><a:item><target>T1</target></a:item><b:item><target>T1</target></b:item></root>`)
	assertValidationIssues(t, c, []string{"/root/item/target:ID"})
	require.NotNil(t, c.IdentityBindings["T1"])
	assert.Len(t, c.IdentityBindings["T1"].Owners, 2)
	assert.Equal(t, c.IdentityBindings["T1"].FirstPath, c.IdentityBindings["T1"].SecondPath)
}

func TestXSD10IdentityStableFinalization(t *testing.T) {
	source := `<root><refs>B A missing</refs><item id="A"/><item id="B"/><item id="A"/><item id="B"/></root>`
	var first []validationKernelIssue
	for run := 0; run < 20; run++ {
		c := validateAttributeSource(t, &validationIdentityGenerated, source)
		if run == 0 {
			first = c.Issues
		}
		assert.Equal(t, first, c.Issues)
	}
	require.Len(t, first, 3)
	assert.Equal(t, "/root/item[4]/@id", first[0].Path)
	assert.Equal(t, "/root/item[3]/@id", first[1].Path)
	assert.Equal(t, "/root/refs", first[2].Path)
}

func TestXSD10IdentitySkippedChildTraversal(t *testing.T) {
	id := validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "ID"}}
	ref := validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "IDREF"}}
	item := &validationElementSchema{Name: validationQName{Local: "item"}, Type: validationTypeRef{InlineComplex: &validationComplexSchema{Form: validationComplexDirect, Attributes: []validationAttributeSchema{{Name: validationQName{Local: "id"}, Type: id}}}}}
	particle := func(element *validationElementSchema, min, max uint64) *validationParticleSchema {
		return &validationParticleSchema{Kind: validationParticleElement, Occurrence: validationOccurrence{Min: min, Max: max}, Element: element}
	}
	limited := func(name string) *validationElementSchema {
		return &validationElementSchema{Name: validationQName{Local: name}, Type: validationTypeRef{InlineComplex: &validationComplexSchema{Form: validationComplexDirect, Particle: particle(item, 1, 1), Attributes: []validationAttributeSchema{{Name: validationQName{Local: "ref"}, Type: ref}}}}}
	}
	for _, test := range []struct {
		name, source                   string
		emptyType, duplicates, skipped bool
		constraints                    []string
	}{
		{name: "unexpected sibling skips target", source: `<root ref="target"><item id="target"/><unexpected/></root>`, skipped: true, constraints: []string{"content-model"}},
		{name: "empty type skips target", source: `<root ref="target"><item id="target"/></root>`, emptyType: true, skipped: true, constraints: []string{"content-model"}},
		{name: "missing required child skips nothing", source: `<root ref="target"/>`, constraints: []string{"content-model", "IDREF"}},
		{name: "owner text does not skip", source: `<root ref="target">bad<item id="target"/></root>`, constraints: []string{"element-only"}},
		{name: "owner attributes do not skip", source: `<root ref="target" bad="x"><item id="target"/></root>`, constraints: []string{"attribute"}},
		{name: "positive duplicates survive skipped subtree", source: `<root ref="target"><item id="target"/><item id="target"/><limited ref="missing"><item id="missing"/><unexpected/></limited></root>`, duplicates: true, skipped: true, constraints: []string{"content-model", "ID"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := limited("root")
			if test.emptyType {
				root.Type.InlineComplex.Particle = nil
			}
			if test.duplicates {
				root.Type.InlineComplex.Particle = &validationParticleSchema{Kind: validationParticleSequence, Occurrence: validationOccurrence{Min: 1, Max: 1}, Children: []*validationParticleSchema{particle(item, 2, 2), particle(limited("limited"), 1, 1)}}
			}
			set := &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{{Local: "root"}: root}}
			c := validateAttributeSource(t, set, test.source)
			require.NoError(t, c.AssessmentErr, "ordinary invalidity stays separate from technical failure")
			var constraints []string
			for _, issue := range c.Issues {
				constraints = append(constraints, issue.Constraint)
			}
			assert.Equal(t, test.constraints, constraints)
			assert.Equal(t, test.skipped, c.IdentityIncomplete)
		})
	}
}

func TestXSD10IdentitySkippedChildKinds(t *testing.T) {
	builtin := func(name string) validationTypeRef {
		return validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: name}}
	}
	id := builtin("ID")
	for _, test := range []struct {
		name           string
		typ            validationTypeRef
		attrs, content string
		want           []string
		skipped        bool
	}{
		{"simple type", builtin("string"), "", `target<hidden/>`, []string{"simple-content"}, true},
		{"complex simple content", validationTypeRef{InlineComplex: &validationComplexSchema{Form: validationComplexSimpleContentExtension, Base: &id}}, "", `target<hidden/>`, []string{"simple-content"}, true},
		{"abstract actual type", validationTypeRef{InlineComplex: &validationComplexSchema{Form: validationComplexDirect, Abstract: true}}, "", `<hidden/>`, []string{"abstract"}, true},
		{"nil with illegal children", builtin("ID"), ` i:nil="true"`, `<hidden/>`, []string{"nillable"}, true},
		{"nil without children", builtin("ID"), ` i:nil="true"`, "", []string{"IDREF"}, false},
		{"abstract without children", validationTypeRef{InlineComplex: &validationComplexSchema{Form: validationComplexDirect, Abstract: true}}, "", "", []string{"abstract", "IDREF"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := &validationElementSchema{Name: validationQName{Local: "value"}, Type: test.typ, Nillable: true}
			root := &validationElementSchema{Name: validationQName{Local: "root"}, Type: validationTypeRef{InlineComplex: &validationComplexSchema{Form: validationComplexDirect, Attributes: []validationAttributeSchema{{Name: validationQName{Local: "ref"}, Type: builtin("IDREF")}}, Particle: &validationParticleSchema{Kind: validationParticleElement, Occurrence: validationOccurrence{Min: 1, Max: 1}, Element: value}}}}
			set := &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{{Local: "root"}: root}}
			c := validateAttributeSource(t, set, `<root xmlns:i="`+validationXSINamespace+`" ref="target"><value`+test.attrs+`>`+test.content+`</value></root>`)
			require.NoError(t, c.AssessmentErr)
			var constraints []string
			for _, issue := range c.Issues {
				constraints = append(constraints, issue.Constraint)
			}
			assert.Equal(t, test.want, constraints)
			assert.Equal(t, test.skipped, c.IdentityIncomplete)
		})
	}
}

func TestXSD10IdentityAbstractTypeAttributes(t *testing.T) {
	builtin := func(name string) validationTypeRef {
		return validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: name}}
	}
	for _, test := range []struct {
		name, attrs string
		want        []string
		bindings    int
	}{
		{"valid ID attribute binds independently", ` id="target"`, []string{"abstract"}, 1},
		{"invalid ID attribute cannot bind", ` id="9bad"`, []string{"abstract", "datatype", "IDREF"}, 0},
		{"IDREF attribute seeds independently", ` id="target" ref="missing"`, []string{"abstract", "IDREF"}, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			value := &validationElementSchema{Name: validationQName{Local: "value"}, Type: validationTypeRef{InlineComplex: &validationComplexSchema{Form: validationComplexDirect, Abstract: true, Attributes: []validationAttributeSchema{{Name: validationQName{Local: "id"}, Type: builtin("ID")}, {Name: validationQName{Local: "ref"}, Type: builtin("IDREF")}}}}}
			root := &validationElementSchema{Name: validationQName{Local: "root"}, Type: validationTypeRef{InlineComplex: &validationComplexSchema{Form: validationComplexDirect, Attributes: []validationAttributeSchema{{Name: validationQName{Local: "ref"}, Type: builtin("IDREF")}}, Particle: &validationParticleSchema{Kind: validationParticleElement, Occurrence: validationOccurrence{Min: 1, Max: 1}, Element: value}}}}
			set := &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{{Local: "root"}: root}}
			c := validateAttributeSource(t, set, `<root ref="target"><value`+test.attrs+`/></root>`)
			require.NoError(t, c.AssessmentErr)
			var constraints []string
			for _, issue := range c.Issues {
				constraints = append(constraints, issue.Constraint)
			}
			assert.Equal(t, test.want, constraints)
			assert.Len(t, c.IdentityBindings, test.bindings)
			assert.False(t, c.IdentityIncomplete, "attributes were assessed and no children were skipped")
		})
	}
}

func TestXSD10IdentityOpenContentBoundary(t *testing.T) {
	// Open-content declaration discovery/processContents remains deferred. This
	// ordinary-invalidity guard must not suppress the only IDREF issue into nil
	// on an otherwise accepted opaque path; it is not open-content conformance.
	builtin := func(name string) validationTypeRef {
		return validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: name}}
	}
	for _, wildcard := range []bool{false, true} {
		value := &validationElementSchema{Name: validationQName{Local: "value"}, Type: builtin("anyType")}
		particle := &validationParticleSchema{Kind: validationParticleElement, Occurrence: validationOccurrence{Min: 1, Max: 1}, Element: value}
		source := `<root ref="absent"><value><unknown/></value></root>`
		if wildcard {
			particle = &validationParticleSchema{Kind: validationParticleAny, Occurrence: validationOccurrence{Min: 1, Max: 1}, Any: &validationAnySchema{Namespace: "##any", ProcessContents: "lax"}}
			source = `<root ref="absent"><unknown/></root>`
		}
		root := &validationElementSchema{Name: validationQName{Local: "root"}, Type: validationTypeRef{InlineComplex: &validationComplexSchema{Form: validationComplexDirect, Attributes: []validationAttributeSchema{{Name: validationQName{Local: "ref"}, Type: builtin("IDREF")}}, Particle: particle}}}
		set := &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{{Local: "root"}: root}}
		c := validateAttributeSource(t, set, source)
		assert.False(t, c.IdentityIncomplete)
		assert.ErrorIs(t, c.result(), ErrInvalidDocument)
		assertValidationIssues(t, c, []string{"/root/@ref:IDREF"})
	}
}
