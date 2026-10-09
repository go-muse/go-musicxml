package musicxml

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validationNilSchema mirrors testdata/validation/nil-contract.xsd. The real
// MusicXML schemas have no nillable declarations, so they cannot exercise the
// positive contract. No schema parser or public raw-source API is added here.
func validationNilSchema() *validationSchemaSet {
	builtin := func(name string) validationTypeRef {
		return validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: name}}
	}
	attributes := []validationAttributeSchema{
		{Name: validationQName{Local: "required"}, Type: builtin("int"), Use: validationAttributeRequired},
		{Name: validationQName{Local: "blocked"}, Type: builtin("string"), Use: validationAttributeProhibited},
		{Name: validationQName{Local: "locked"}, Type: builtin("token"), Constraint: &validationValueConstraint{Kind: "fixed", Value: "kept"}},
		{Name: validationQName{Local: "id"}, Type: builtin("ID")},
		{Name: validationQName{Local: "ref"}, Type: builtin("IDREF")},
		{Name: validationQName{Local: "refs"}, Type: builtin("IDREFS")},
	}
	particle := func(element *validationElementSchema, unbounded bool) *validationParticleSchema {
		return &validationParticleSchema{
			Kind: validationParticleElement, Occurrence: validationOccurrence{Min: 1, Max: 1, Unbounded: unbounded}, Element: element,
		}
	}
	child := particle(&validationElementSchema{Name: validationQName{Local: "child"}, Type: builtin("int")}, false)
	word := &validationSimpleSchema{Form: validationSimpleRestriction,
		Base: &validationSimpleMember{Name: builtin("string").Name}, Enumerations: []string{"ok"}}
	intType := builtin("int")
	baseAttributes := append([]validationAttributeSchema(nil), attributes...)
	baseAttributes[1].Use = validationAttributeOptional
	base := &validationComplexSchema{Form: validationComplexDirect, Particle: child, Attributes: baseAttributes}
	restricted := &validationComplexSchema{Form: validationComplexComplexContentRestriction,
		Base: &validationTypeRef{Name: validationQName{Local: "complexBase"}}, Particle: child, Attributes: attributes}
	simpleBase := &validationComplexSchema{Form: validationComplexSimpleContentExtension, Base: &intType, Attributes: baseAttributes}
	simpleRestricted := &validationComplexSchema{Form: validationComplexSimpleContentRestriction,
		Base: &validationTypeRef{Name: validationQName{Local: "simpleBase"}}, Attributes: []validationAttributeSchema{attributes[1]}}
	derived := &validationComplexSchema{Form: validationComplexComplexContentExtension,
		Base:       &validationTypeRef{Name: validationQName{Local: "complexRestricted"}},
		Attributes: []validationAttributeSchema{{Name: validationQName{Local: "extra"}, Type: builtin("string")}}}
	simpleDerived := &validationComplexSchema{Form: validationComplexSimpleContentExtension,
		Base:       &validationTypeRef{Name: validationQName{Local: "simpleRestricted"}},
		Attributes: []validationAttributeSchema{{Name: validationQName{Local: "extra"}, Type: builtin("string")}}}
	schema := &validationSchemaSet{
		Types: map[validationQName]*validationTypeSchema{
			{Local: "word"}: {Simple: word}, {Local: "complexBase"}: {Complex: base},
			{Local: "complexRestricted"}: {Complex: restricted}, {Local: "simpleRestricted"}: {Complex: simpleRestricted},
			{Local: "complexDerived"}: {Complex: derived}, {Local: "simpleBase"}: {Complex: simpleBase},
			{Local: "simpleDerived"}: {Complex: simpleDerived},
		},
		Elements: make(map[validationQName]*validationElementSchema),
	}
	add := func(name string, reference validationTypeRef, nillable bool) *validationElementSchema {
		element := &validationElementSchema{Name: validationQName{Local: name}, Type: reference, Nillable: nillable}
		schema.Elements[element.Name] = element
		return element
	}
	add("plain", builtin("string"), false)
	add("integer", builtin("int"), true)
	add("named", validationTypeRef{Name: validationQName{Local: "word"}}, true)
	add("inline", validationTypeRef{InlineSimple: word}, true)
	add("simple", builtin("anySimpleType"), true)
	add("fixed", builtin("string"), true).Fixed = validationString("kept")
	add("empty-fixed", builtin("string"), true).Fixed = validationString("")
	add("defaulted", builtin("int"), true).Default = validationString("7")
	add("any", builtin("anyType"), true)
	add("complex", validationTypeRef{Name: validationQName{Local: "complexDerived"}}, true)
	simpleType := validationTypeRef{Name: validationQName{Local: "simpleDerived"}}
	add("simple-content", simpleType, true)
	add("fixed-content", simpleType, true).Fixed = validationString("7")
	add("mixed", validationTypeRef{InlineComplex: &validationComplexSchema{
		Form: validationComplexDirect, Particle: child, Attributes: attributes, Mixed: true,
	}}, true)
	add("references", validationTypeRef{InlineComplex: &validationComplexSchema{
		Form: validationComplexDirect, Particle: particle(&validationElementSchema{Reference: validationQName{Local: "integer"}}, true),
	}}, false)
	add("items", validationTypeRef{InlineComplex: &validationComplexSchema{
		Form: validationComplexDirect, Particle: particle(&validationElementSchema{Name: validationQName{Local: "item"}, Type: simpleType, Nillable: true}, true),
	}}, false)
	return schema
}

func validationNilCases() []validationAttributeCase {
	nilPath := "/@{" + validationXSINamespace + "}nil"
	wrap := func(name, attributes, content string) string {
		return "<" + name + ` xmlns:n="` + validationXSINamespace + `"` + attributes + ">" + content + "</" + name + ">"
	}
	var tests []validationAttributeCase
	add := func(name, element, attributes, content, constraint, path string) {
		tests = append(tests, validationAttributeCase{name: name, source: wrap(element, attributes, content), constraint: constraint, path: "/" + element + path})
	}
	for _, element := range []struct{ name, content, attributes string }{
		{"integer", "7", ""}, {"named", "ok", ""}, {"inline", "ok", ""}, {"simple", "text", ""},
		{"complex", "<child>7</child>", ` required="1"`}, {"simple-content", "7", ` required="1"`},
		{"mixed", "text<child>7</child>more", ` required="1"`}, {"any", "text<child/>", ` arbitrary="yes"`},
	} {
		for _, value := range []string{"true", "1", " &#x9;true&#xD;&#xA; ", " &#x9;1&#xD;&#xA; "} {
			add(element.name+" nil "+value, element.name, element.attributes+` n:nil="`+value+`"`, "", "", "")
		}
		for _, value := range []string{"false", "0", " &#x9;false&#xD;&#xA; ", " &#x9;0&#xD;&#xA; "} {
			add(element.name+" nonnil "+value, element.name, element.attributes+` n:nil="`+value+`"`, element.content, "", "")
		}
		add(element.name+" absent", element.name, element.attributes, element.content, "", "")
		for _, content := range []string{" ", "\t\r\n", "&#xD;", "&#xA0;", "text", "<![CDATA[ ]]>", "<child/>"} {
			add(element.name+" nil content "+content, element.name, element.attributes+` n:nil="true"`, content, "nillable", "")
		}
		add(element.name+" comments and PI", element.name, element.attributes+` n:nil="true"`, "<!--comment--><?note allowed?>", "", "")
	}
	for _, value := range []string{"", " ", "TRUE", "False", "yes", "2", "-0", "+1", "true false", "tr&#x9;ue", "&#xA0;true&#xA0;", "&#x85;1&#x85;", "&#x2003;false&#x2003;"} {
		add("invalid boolean "+value, "simple", ` n:nil="`+value+`"`, "", "datatype", nilPath)
	}
	for _, value := range []string{"true", "1", "false", "0", " &#x9;false&#xA; "} {
		add("nonnillable "+value, "plain", ` n:nil="`+value+`"`, "", "nillable", nilPath)
	}
	for _, element := range []string{"fixed", "empty-fixed", "fixed-content"} {
		attributes := ""
		if element == "fixed-content" {
			attributes = ` required="1"`
		}
		add(element+" nil fixed", element, attributes+` n:nil="true"`, "", "fixed", "")
	}
	add("default does not become nil content", "defaulted", ` n:nil="true"`, "", "", "")
	add("fixed false valid", "fixed", ` n:nil="false"`, "kept", "", "")
	add("fixed false invalid", "fixed", ` n:nil="false"`, "other", "fixed", "")
	add("integer false datatype", "integer", ` n:nil="false"`, "bad", "datatype", "")
	add("integer zero child", "integer", ` n:nil="0"`, "<child/>", "simple-content", "")
	add("named false enumeration", "named", ` n:nil="false"`, "bad", "enumeration", "")
	add("complex false missing child", "complex", ` required="1" n:nil="false"`, "", "content-model", "")
	add("complex zero child datatype", "complex", ` required="1" n:nil="0"`, "<child>bad</child>", "datatype", "/child")
	add("complex false text", "complex", ` required="1" n:nil="false"`, "text<child>7</child>", "element-only", "")
	add("simple content false datatype", "simple-content", ` required="1" n:nil="false"`, "bad", "datatype", "")
	for _, element := range []string{"complex", "simple-content", "mixed"} {
		for _, test := range []struct{ name, attributes, constraint, path string }{
			{"required", "", "required", "/@required"},
			{"datatype", ` required="bad"`, "datatype", "/@required"},
			{"prohibited", ` required="1" blocked="x"`, "prohibited", "/@blocked"},
			{"fixed", ` required="1" locked="other"`, "fixed", "/@locked"},
			{"fixed normalized", ` required="1" locked="&#x9; kept &#xD;"`, "", ""},
			{"unknown", ` required="1" rubbish="x"`, "attribute", "/@rubbish"},
			{"unknown xsi", ` required="1" n:unknown="x"`, "attribute", "/@{" + validationXSINamespace + "}unknown"},
			{"malformed ID", ` required="1" id="9bad"`, "datatype", "/@id"},
			{"malformed IDREF", ` required="1" ref="9bad"`, "datatype", "/@ref"},
			{"malformed IDREFS", ` required="1" refs="good 9bad"`, "datatype", "/@refs"},
		} {
			for _, value := range []string{"true", "false", "0"} {
				content := ""
				if value != "true" {
					content = "<child>7</child>"
					if element == "simple-content" {
						content = "7"
					}
				}
				add(element+" nil "+value+" attribute "+test.name, element, test.attributes+` n:nil="`+value+`"`, content, test.constraint, test.path)
			}
		}
	}
	for _, test := range []struct{ name, attributes, path string }{
		{"unqualified", ` nil="true"`, "/@nil"},
		{"wrong namespace", ` xmlns:xsi="urn:foreign" xsi:nil="true"`, "/@{urn:foreign}nil"},
		{"namespace URI suffix", ` xmlns:xsi="` + validationXSINamespace + `/" xsi:nil="true"`, "/@{" + validationXSINamespace + "/}nil"},
		{"wrong case", ` n:Nil="true"`, "/@{" + validationXSINamespace + "}Nil"},
		{"namespace declaration lookalike", ` xmlns:p="xmlns" p:nil="true"`, "/@{xmlns}nil"},
	} {
		add(test.name, "simple", test.attributes, "", "attribute", test.path)
	}
	add("simple nil still rejects attributes", "simple", ` n:nil="true" rubbish="x"`, "", "attribute", "/@rubbish")
	for _, test := range []validationAttributeCase{
		{name: "reference nillable", source: `<references xmlns:alias="` + validationXSINamespace + `"><integer alias:nil="true"/></references>`},
		{name: "reference nil content", source: `<references xmlns:alias="` + validationXSINamespace + `"><integer alias:nil="true"> </integer></references>`, constraint: "nillable", path: "/references/integer"},
		{name: "prefix rebound to xsi", source: `<items xmlns:n="urn:foreign"><item xmlns:n="` + validationXSINamespace + `" n:nil="true" required="1"/></items>`},
		{name: "prefix rebound and restored", source: `<items xmlns:n="` + validationXSINamespace + `"><item xmlns:n="urn:foreign" n:nil="true" required="1">7</item><item n:nil="true" required="2"/></items>`, constraint: "attribute", path: "/items/item/@{urn:foreign}nil"},
		{name: "nil declared after attribute", source: `<integer alias:nil="true" xmlns:alias="` + validationXSINamespace + `"/>`},
	} {
		tests = append(tests, test)
	}
	return tests
}

func TestValidateNilContracts(t *testing.T) {
	t.Parallel()
	for _, test := range validationNilCases() {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertValidationNilCase(t, validationNilSchema(), test)
		})
	}
}

func assertValidationNilCase(t *testing.T, schema *validationSchemaSet, test validationAttributeCase) {
	t.Helper()
	context := validateAttributeSource(t, schema, test.source)
	if test.constraint == "" {
		assert.Empty(t, context.issues)
		return
	}
	require.Len(t, context.issues, 1)
	assert.Equal(t, test.constraint, context.issues[0].Constraint)
	assert.Equal(t, test.path, context.issues[0].Path)
}

// Identity checks are independently asserted: xmllint/libxml2 does not reliably
// enforce unresolved IDREF(S), so an oracle acceptance is not evidence for them.
func TestValidateNilIdentityTracking(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, source, constraint, path string }{
		{"nilled ID and forward references", `<items xmlns:n="` + validationXSINamespace + `"><item n:nil="true" required="1" ref="later" refs="later later"/><item n:nil="true" required="2" id="later"/></items>`, "", ""},
		{"duplicate nilled IDs", `<items xmlns:n="` + validationXSINamespace + `"><item n:nil="true" required="1" id="same"/><item n:nil="true" required="2" id="same"/></items>`, "ID", "/items/item[2]/@id"},
		{"nilled unresolved IDREF", `<simple-content xmlns:n="` + validationXSINamespace + `" n:nil="true" required="1" ref="missing"/>`, "IDREF", "/simple-content/@ref"},
		{"nilled unresolved IDREFS", `<simple-content xmlns:n="` + validationXSINamespace + `" n:nil="true" required="1" id="present" refs="present missing"/>`, "IDREF", "/simple-content/@refs"},
		{"false retains ID tracking", `<items xmlns:n="` + validationXSINamespace + `"><item n:nil="false" required="1" ref="later">7</item><item n:nil="0" required="2" id="later">8</item></items>`, "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			context := validateAttributeSource(t, validationNilSchema(), test.source)
			if test.constraint != "" {
				require.Len(t, context.issues, 1)
				assert.Equal(t, test.constraint, context.issues[0].Constraint)
				assert.Equal(t, test.path, context.issues[0].Path)
				return
			}
			assert.Empty(t, context.issues)
			assert.Equal(t, map[string]string{"later": "/items/item[2]/@id"}, context.identifiers)
			require.NotEmpty(t, context.references)
			for _, reference := range context.references {
				assert.Equal(t, "later", reference.value)
			}
		})
	}
}

func TestValidateNilIdentityTypeForms(t *testing.T) {
	t.Parallel()
	for _, element := range []string{"complex", "simple-content", "mixed"} {
		t.Run(element, func(t *testing.T) {
			source := `<` + element + ` xmlns:n="` + validationXSINamespace + `" n:nil="true" required="1" id=" &#x9;known&#xA; " ref="known" refs="known&#x9;known"/>`
			context := validateAttributeSource(t, validationNilSchema(), source)
			assert.Empty(t, context.issues)
			assert.Equal(t, map[string]string{"known": "/" + element + "/@id"}, context.identifiers)
			require.Len(t, context.references, 3)
			for _, reference := range context.references {
				assert.Equal(t, "known", reference.value)
			}
		})
	}
	for _, simpleContent := range []bool{false, true} {
		schema := validationNilSchema()
		id := validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "ID"}}
		element, attributes := "simple", ""
		if simpleContent {
			schema.Types[validationQName{Local: "simpleBase"}].Complex.Base = &id
			element, attributes = "simple-content", ` required="1" id="attribute-id"`
		} else {
			schema.Elements[validationQName{Local: "simple"}].Type = id
		}
		for _, nilled := range []bool{false, true} {
			value, content := "false", "content-id"
			if nilled {
				value, content = "true", ""
			}
			source := `<` + element + ` xmlns:n="` + validationXSINamespace + `" n:nil="` + value + `"` + attributes + `>` + content + `</` + element + `>`
			context := validateAttributeSource(t, schema, source)
			assert.Empty(t, context.issues)
			want := map[string]string{}
			if simpleContent {
				want["attribute-id"] = "/simple-content/@id"
			}
			if !nilled {
				want["content-id"] = "/" + element
			}
			assert.Equal(t, want, context.identifiers, "simpleContent=%t nilled=%t", simpleContent, nilled)
		}
	}
}

func TestValidateNilIndependentIssues(t *testing.T) {
	t.Parallel()
	nilPath := "/@{" + validationXSINamespace + "}nil"
	for _, test := range []struct {
		name, source       string
		constraints, paths []string
	}{
		{"invalid and forbidden nil", `<plain xmlns:n="` + validationXSINamespace + `" n:nil="maybe"/>`,
			[]string{"datatype", "nillable"}, []string{"/plain" + nilPath, "/plain" + nilPath}},
		{"invalid nil and required attribute", `<complex xmlns:n="` + validationXSINamespace + `" n:nil="maybe"><child>7</child></complex>`,
			[]string{"datatype", "required"}, []string{"/complex" + nilPath, "/complex/@required"}},
		{"fixed content and required attribute", `<fixed-content xmlns:n="` + validationXSINamespace + `" n:nil="true"> </fixed-content>`,
			[]string{"fixed", "nillable", "required"}, []string{"/fixed-content", "/fixed-content", "/fixed-content/@required"}},
		{"simple content and unknown attribute", `<simple xmlns:n="` + validationXSINamespace + `" n:nil="true" rubbish="x"> </simple>`,
			[]string{"attribute", "nillable"}, []string{"/simple/@rubbish", "/simple"}},
		{"false missing content and required attribute", `<complex xmlns:n="` + validationXSINamespace + `" n:nil="false"/>`,
			[]string{"required", "content-model"}, []string{"/complex/@required", "/complex"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			context := validateAttributeSource(t, validationNilSchema(), test.source)
			require.Len(t, context.issues, len(test.constraints))
			for index, issue := range context.issues {
				assert.Equal(t, test.constraints[index], issue.Constraint)
				assert.Equal(t, test.paths[index], issue.Path)
			}
		})
	}
}

func validationMusicXMLNilCases() []validationAttributeCase {
	const timewise = `<score-timewise version="4.0"><part-list><score-part id="P1"><part-name>Music</part-name></score-part></part-list><measure number="1"><part id="P1"/></measure></score-timewise>`
	const opus = `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="score.musicxml"/></opus>`
	var tests []validationAttributeCase
	for _, target := range []struct{ name, source, marker, path, schema string }{
		{"partwise root", validationAttributeScore, "<score-partwise ", "/score-partwise", ""},
		{"complex content", validationAttributeScore, "<measure ", "/score-partwise/part/measure", ""},
		{"simple content", validationAttributeScore, "<type ", "/score-partwise/part/measure/note/type", ""},
		{"named simple", validationAttributeScore, "<step", "/score-partwise/part/measure/note/pitch/step", ""},
		{"builtin simple", validationAttributeScore, "<staves", "/score-partwise/part/measure/attributes/staves", ""},
		{"timewise root", timewise, "<score-timewise ", "/score-timewise", ""},
		{"opus root", opus, "<opus ", "/opus", "opus.xsd"},
		{"opus score", opus, "<score ", "/opus/score", "opus.xsd"},
	} {
		tests = append(tests, validationAttributeCase{name: target.name + " absent", source: target.source, schema: target.schema})
		for _, value := range []string{"true", "1", "false", "0", " &#x9;false&#xD;&#xA; "} {
			source := strings.Replace(target.source, target.marker, target.marker+` xmlns:alias="`+validationXSINamespace+`" alias:nil="`+value+`" `, 1)
			tests = append(tests, validationAttributeCase{name: target.name + " nil " + value, source: source, schema: target.schema,
				constraint: "nillable", path: target.path + "/@{" + validationXSINamespace + "}nil"})
		}
	}
	return tests
}

func TestValidateMusicXMLNilContracts(t *testing.T) {
	t.Parallel()
	for _, test := range validationMusicXMLNilCases() {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			schema := &scoreValidationSchema
			if test.schema == "opus.xsd" {
				schema = &opusValidationSchema
			}
			assertValidationNilCase(t, schema, test)
		})
	}
}

func TestMusicXMLNilPreservesPublicModelValidation(t *testing.T) {
	t.Parallel()
	for _, test := range validationMusicXMLNilCases() {
		t.Run(test.name, func(t *testing.T) {
			document, err := Decode(strings.NewReader(test.source))
			require.NoError(t, err)
			// Decode drops unsupported nil attributes. Validate checks the encoded
			// model, not the source; this change adds no strict-source public API.
			assert.NoError(t, Validate(document))
			var encoded bytes.Buffer
			require.NoError(t, Encode(&encoded, document))
			assert.NotContains(t, encoded.String(), ":nil=")
		})
	}
}

// Both validators receive identical original bytes. They never validate a
// Decode/Encode round trip, which would discard the source xsi:nil attribute.
func TestNilContractsAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	directory, err := filepath.Abs(filepath.Join("schema", "musicxml-4.0"))
	require.NoError(t, err)
	for _, group := range []struct {
		name  string
		cases []validationAttributeCase
	}{
		{"synthetic", validationNilCases()}, {"MusicXML", validationMusicXMLNilCases()},
	} {
		t.Run(group.name, func(t *testing.T) {
			for _, test := range group.cases {
				t.Run(test.name, func(t *testing.T) {
					schema := validationNilSchema()
					schemaFile := filepath.Join("testdata", "validation", "nil-contract.xsd")
					if group.name == "MusicXML" {
						schema = &scoreValidationSchema
						schemaFile = filepath.Join(directory, "musicxml.xsd")
						if test.schema == "opus.xsd" {
							schema = &opusValidationSchema
							schemaFile = filepath.Join(directory, "opus.xsd")
						}
					}
					context := validateAttributeSource(t, schema, test.source)
					command := exec.Command(xmllint, "--nonet", "--noout", "--schema", schemaFile, "-")
					command.Env = append(os.Environ(), "XML_CATALOG_FILES="+filepath.Join(directory, "catalog.xml"))
					command.Stdin = strings.NewReader(test.source)
					output, err := command.CombinedOutput()
					wantValid := test.constraint == ""
					assert.Equal(t, wantValid, len(context.issues) == 0, "internal validation: %v", context.issues)
					if wantValid {
						assert.NoErrorf(t, err, "independent XSD validation: %s", output)
					} else {
						var exitError *exec.ExitError
						require.ErrorAsf(t, err, &exitError, "independent XSD validation unexpectedly accepted source: %s", output)
						assert.Equalf(t, 3, exitError.ExitCode(), "expected schema-invalid exit, got: %s", output)
					}
				})
			}
		})
	}
}

// Nilling suppresses value/content checks, never resolution of the declared
// type or its effective attributes. These malformed metadata controls are not
// oracle cases because an external validator rejects their schema itself.
func TestValidateNilUnresolvedTypes(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		reference validationTypeRef
	}{
		{"unknown declared type", validationTypeRef{Name: validationQName{Local: "unknown"}}},
		{"invalid complex base", validationTypeRef{InlineComplex: &validationComplexSchema{
			Form: validationComplexComplexContentExtension, Base: &validationTypeRef{Name: validationQName{Local: "unknown"}},
		}}},
		{"invalid simple-content base", validationTypeRef{InlineComplex: &validationComplexSchema{
			Form: validationComplexSimpleContentExtension, Base: &validationTypeRef{Name: validationQName{Local: "unknown"}},
		}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			schema := &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{
				{Local: "value"}: {Name: validationQName{Local: "value"}, Type: test.reference, Nillable: true},
			}}
			context := validateAttributeSource(t, schema, `<value xmlns:n="`+validationXSINamespace+`" n:nil="true"/>`)
			require.Len(t, context.issues, 1)
			assert.Equal(t, "schema", context.issues[0].Constraint)
			assert.Equal(t, "/value", context.issues[0].Path)
		})
	}
}

// libxml2 2.9.14 incorrectly rejects a zero-length CDATA section on a nilled
// node and accepts empty IDREFS. Keep the intended infoset/datatype behavior
// independent of that oracle rather than weakening it to obtain parity.
func TestValidateNilOracleLimitations(t *testing.T) {
	t.Parallel()
	for _, test := range []validationAttributeCase{
		{name: "empty CDATA", source: `<integer xmlns:n="` + validationXSINamespace + `" n:nil="true"><![CDATA[]]></integer>`},
		{name: "empty IDREFS", source: `<simple-content xmlns:n="` + validationXSINamespace + `" n:nil="true" required="1" refs=" "/>`, constraint: "datatype", path: "/simple-content/@refs"},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertValidationNilCase(t, validationNilSchema(), test)
		})
	}
}
