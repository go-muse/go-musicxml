package musicxml

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validationAttributeScore = `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name font-style="italic">Music</part-name></score-part></part-list><part id="P1"><measure number="1" implicit="yes"><attributes><staves>1</staves></attributes><note><pitch><step>C</step><octave>4</octave></pitch><duration>1</duration><type size="cue">quarter</type></note></measure></part></score-partwise>`

type validationAttributeCase struct {
	name, source, schema, constraint, path string
}

func validationAttributeCases() []validationAttributeCase {
	const stavesPath = "/score-partwise/part/measure/attributes/staves"
	const stepPath = "/score-partwise/part/measure/note/pitch/step"
	tests := []validationAttributeCase{{name: "valid complex simple content", source: validationAttributeScore}}
	for _, element := range []struct {
		name, path string
	}{
		{"staves", stavesPath},
		{"step", stepPath},
	} {
		for _, attribute := range []struct {
			name, source, path string
		}{
			{"unqualified", ` rubbish="x"`, "/@rubbish"},
			{"foreign", ` xmlns:p="urn:foreign" p:rubbish="x"`, "/@{urn:foreign}rubbish"},
			{"literal xmlns URI", ` xmlns:p="xmlns" p:rubbish="x"`, "/@{xmlns}rubbish"},
			{"literal xmlns URI before declaration", ` p:rubbish="x" xmlns:p="xmlns"`, "/@{xmlns}rubbish"},
			{"declaration and ordinary same expanded name", ` xmlns:p="xmlns" xmlns:rubbish="urn:unused" p:rubbish="x"`, "/@{xmlns}rubbish"},
			{"qualified xmlns local name", ` xmlns:p="xmlns" p:xmlns="x"`, "/@{xmlns}xmlns"},
			{"XML namespace", ` xml:lang="en"`, "/@{http://www.w3.org/XML/1998/namespace}lang"},
			{"unknown xsi", ` xmlns:i="http://www.w3.org/2001/XMLSchema-instance" i:unknown="x"`, "/@{http://www.w3.org/2001/XMLSchema-instance}unknown"},
			{"unqualified xsi lookalike", ` nil="false"`, "/@nil"},
			{"foreign xsi lookalike", ` xmlns:i="urn:foreign" i:schemaLocation="urn:example schema.xsd"`, "/@{urn:foreign}schemaLocation"},
			{"hint and unknown xsi", ` xmlns:i="http://www.w3.org/2001/XMLSchema-instance" i:noNamespaceSchemaLocation="musicxml.xsd" i:unknown="x"`, "/@{http://www.w3.org/2001/XMLSchema-instance}unknown"},
			{"namespace declarations", ` xmlns="" xmlns:p="urn:unused"`, ""},
			{"literal xmlns declaration", ` xmlns:p="xmlns"`, ""},
			{"explicit XML declaration", ` xmlns:xml="http://www.w3.org/XML/1998/namespace"`, ""},
			{"schema location", ` xmlns:i="http://www.w3.org/2001/XMLSchema-instance" i:schemaLocation="urn:example https://example.invalid/schema.xsd"`, ""},
			{"no namespace schema location", ` xmlns:i="http://www.w3.org/2001/XMLSchema-instance" i:noNamespaceSchemaLocation="musicxml.xsd"`, ""},
		} {
			constraint := ""
			if attribute.path != "" {
				constraint = "attribute"
			}
			tests = append(tests, validationAttributeCase{
				name: element.name + " " + attribute.name,
				source: strings.Replace(validationAttributeScore, "<"+element.name+">",
					"<"+element.name+attribute.source+">", 1),
				constraint: constraint,
				path:       element.path + attribute.path,
			})
		}
	}
	for _, element := range []struct {
		name, typeName string
	}{
		{"staves", "xs:nonNegativeInteger"},
		{"step", "step"},
	} {
		tests = append(tests, validationAttributeCase{
			name: element.name + " standard xsi type",
			source: strings.Replace(validationAttributeScore, "<"+element.name+">",
				"<"+element.name+` xmlns:i="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema" i:type="`+element.typeName+`">`, 1),
		})
	}
	for _, test := range []struct {
		name, old, replacement, constraint, path string
	}{
		{"required complex attribute", ` number="1"`, "", "required", "/score-partwise/part/measure/@number"},
		{"complex enum", `implicit="yes"`, `implicit="maybe"`, "enumeration", "/score-partwise/part/measure/@implicit"},
		{"complex unknown attribute", `<measure number`, `<measure rubbish="x" number`, "attribute", "/score-partwise/part/measure/@rubbish"},
		{"complex literal xmlns URI", `<measure number`, `<measure xmlns:p="xmlns" p:rubbish="x" number`, "attribute", "/score-partwise/part/measure/@{xmlns}rubbish"},
		{"complex literal xmlns declaration", `<measure number`, `<measure xmlns:p="xmlns" number`, "", ""},
		{"simple content enum", `size="cue"`, `size="bad"`, "enumeration", "/score-partwise/part/measure/note/type/@size"},
		{"simple content datatype", `font-style="italic"`, `font-style="italic" default-x="bad"`, "datatype", "/score-partwise/part-list/score-part/part-name/@default-x"},
		{"simple content unknown attribute", `<type size`, `<type rubbish="x" size`, "attribute", "/score-partwise/part/measure/note/type/@rubbish"},
		{"simple content literal xmlns URI", `<type size`, `<type xmlns:p="xmlns" p:rubbish="x" size`, "attribute", "/score-partwise/part/measure/note/type/@{xmlns}rubbish"},
		{"simple content default omitted", ` size="cue"`, "", "", ""},
		{"simple content default explicit", `size="cue"`, `size="full"`, "", ""},
	} {
		tests = append(tests, validationAttributeCase{
			name: test.name, source: strings.Replace(validationAttributeScore, test.old, test.replacement, 1),
			constraint: test.constraint, path: test.path,
		})
	}
	for _, fixed := range []struct {
		name, source, constraint string
	}{
		{"omitted", "", ""},
		{"explicit", ` xlink:type="simple"`, ""},
		{"normalized", ` xlink:type="&#x9; simple &#xA;"`, ""},
		{"invalid", ` xlink:type="extended"`, "fixed"},
	} {
		tests = append(tests, validationAttributeCase{
			name:   "opus fixed " + fixed.name,
			source: `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="score.musicxml"` + fixed.source + `/></opus>`,
			schema: "opus.xsd", constraint: fixed.constraint,
			path: "/opus/score/@{http://www.w3.org/1999/xlink}type",
		})
	}
	return append(tests, validationSchemaHintCases()...)
}

// Raw XML is deliberate: Decode discards attributes that the typed model cannot
// represent, so a Decode -> Validate test cannot exercise source permissibility.
func TestValidateElementAttributeContracts(t *testing.T) {
	t.Parallel()
	for _, test := range validationAttributeCases() {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			schema := &scoreValidationSchema
			if test.schema == "opus.xsd" {
				schema = &opusValidationSchema
			}
			context := validateAttributeSource(t, schema, test.source)
			if test.constraint == "" {
				assert.Empty(t, context.issues)
				return
			}
			wantIssues := 1
			if test.constraint == "fixed" {
				wantIssues = 2 // The fixed value also violates the XLink enumeration.
			}
			require.Len(t, context.issues, wantIssues)
			assert.Equal(t, test.constraint, context.issues[0].Constraint)
			assert.Equal(t, test.path, context.issues[0].Path)
		})
	}
}

func TestValidateSimpleElementTypeForms(t *testing.T) {
	t.Parallel()
	simple := &validationSimpleSchema{
		Form: validationSimpleRestriction,
		Base: &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: "string"}},
	}
	for _, test := range []struct {
		name      string
		reference validationTypeRef
	}{
		{"named", validationTypeRef{Name: validationQName{Local: "text"}}},
		{"inline", validationTypeRef{InlineSimple: simple}},
		{"builtin", validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "string"}}},
		{"anySimpleType", validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "anySimpleType"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			schema := &validationSchemaSet{
				Types: map[validationQName]*validationTypeSchema{{Local: "text"}: {Simple: simple}},
				Elements: map[validationQName]*validationElementSchema{
					{Local: "value"}: {Name: validationQName{Local: "value"}, Type: test.reference, Nillable: true},
				},
			}
			for _, input := range []struct{ name, source string }{
				{"value", `<value rubbish="x">text</value>`},
				{"nil", `<value rubbish="x" xmlns:i="http://www.w3.org/2001/XMLSchema-instance" i:nil="true"/>`},
			} {
				t.Run(input.name, func(t *testing.T) {
					context := validateAttributeSource(t, schema, input.source)
					require.Len(t, context.issues, 1)
					assert.Equal(t, "attribute", context.issues[0].Constraint)
					assert.Equal(t, "/value/@rubbish", context.issues[0].Path)
				})
			}
		})
	}
}

func TestValidateSimpleElementAttributeAndContentIssues(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		content, constraint string
	}{
		{"H", "enumeration"},
		{"<child/>", "simple-content"},
	} {
		source := strings.Replace(validationAttributeScore, "<step>C</step>", `<step rubbish="x">`+test.content+`</step>`, 1)
		context := validateAttributeSource(t, &scoreValidationSchema, source)
		require.Len(t, context.issues, 2)
		assert.Equal(t, "attribute", context.issues[0].Constraint)
		assert.Equal(t, test.constraint, context.issues[1].Constraint)
	}
}

func TestValidateAnyTypeAttributesUnchanged(t *testing.T) {
	t.Parallel()
	schema := &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{
		{Local: "value"}: {
			Name: validationQName{Local: "value"},
			Type: validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "anyType"}},
		},
	}}
	context := validateAttributeSource(t, schema, `<value rubbish="x" xmlns:p="urn:foreign" p:other="y"><child/></value>`)
	assert.Empty(t, context.issues)
}

func TestValidateComplexSimpleContentIdentityOnce(t *testing.T) {
	t.Parallel()
	id := validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "ID"}}
	base := &validationComplexSchema{
		Form: validationComplexSimpleContentExtension, Base: &id,
		Attributes: []validationAttributeSchema{{Name: validationQName{Local: "id"}, Type: id}},
	}
	derived := &validationComplexSchema{
		Form: validationComplexSimpleContentExtension,
		Base: &validationTypeRef{Name: validationQName{Local: "base"}},
	}
	schema := &validationSchemaSet{
		Types: map[validationQName]*validationTypeSchema{{Local: "base"}: {Complex: base}},
		Elements: map[validationQName]*validationElementSchema{
			{Local: "value"}: {Name: validationQName{Local: "value"}, Type: validationTypeRef{InlineComplex: derived}},
		},
	}
	context := validateAttributeSource(t, schema, `<value id="attribute-id">content-id</value>`)
	assert.Empty(t, context.issues)
	assert.Equal(t, map[string]string{"attribute-id": "/value/@id", "content-id": "/value"}, context.identifiers)
}

func TestValidateUnknownElementTypeRemainsSchemaIssue(t *testing.T) {
	t.Parallel()
	schema := &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{
		{Local: "value"}: {Name: validationQName{Local: "value"}, Type: validationTypeRef{Name: validationQName{Local: "unknown"}}},
	}}
	context := validateAttributeSource(t, schema, `<value rubbish="x">text</value>`)
	require.Len(t, context.issues, 1)
	assert.Equal(t, "schema", context.issues[0].Constraint)
	assert.Equal(t, "/value", context.issues[0].Path)
}

func TestDecodeDoesNotValidateSimpleElementAttributes(t *testing.T) {
	t.Parallel()
	source := strings.Replace(validationAttributeScore, "<staves>", `<staves rubbish="x">`, 1)
	source = strings.Replace(source, "<step>", `<step rubbish="x">`, 1)
	document, err := Decode(strings.NewReader(source))
	require.NoError(t, err)
	// Public Validate assesses the decoded model, not discarded source attributes.
	assert.NoError(t, Validate(document))
}

func validateAttributeSource(t *testing.T, schema *validationSchemaSet, source string) *validationContext {
	t.Helper()
	root, err := parseValidationDocument([]byte(source))
	require.NoError(t, err)
	declaration, found := schema.Elements[validationName(root.Name)]
	require.True(t, found)
	context := &validationContext{
		schema: schema, effective: make(map[*validationComplexSchema]*validationEffectiveComplex),
		identifiers: make(map[string]string),
	}
	context.validateElement(root, declaration, "/"+root.Name.Local)
	context.validateIdentityReferences()
	return context
}

// Both engines receive the same original bytes, with no model round trip.
func TestElementAttributeContractsAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI runs the external XSD conformance test")
	}
	directory, err := filepath.Abs(filepath.Join("schema", "musicxml-4.0"))
	require.NoError(t, err)
	tests := validationAttributeCases()
	// This fixture is also checked internally by TestValidateNamespaceDeclarationLookalike.
	lookalike, err := os.ReadFile(filepath.Join("testdata", "validation", "namespace-declaration-lookalike.musicxml"))
	require.NoError(t, err)
	tests = append(tests, validationAttributeCase{
		name: "namespace declaration lookalike", source: string(lookalike), constraint: "attribute",
	})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			schema := test.schema
			if schema == "" {
				schema = "musicxml.xsd"
			}
			command := exec.Command(xmllint, "--nonet", "--noout", "--schema", filepath.Join(directory, schema), "-")
			command.Env = append(os.Environ(), "XML_CATALOG_FILES="+filepath.Join(directory, "catalog.xml"))
			command.Stdin = strings.NewReader(test.source)
			output, err := command.CombinedOutput()
			if test.constraint == "" {
				assert.NoErrorf(t, err, "independent XSD validation: %s", output)
			} else {
				var exitError *exec.ExitError
				require.ErrorAsf(t, err, &exitError, "independent XSD validation unexpectedly accepted source: %s", output)
				assert.Equalf(t, 3, exitError.ExitCode(), "expected schema-invalid exit, got: %s", output)
			}
		})
	}
}

func TestValidateNamespaceDeclarationLookalike(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile(filepath.Join("testdata", "validation", "namespace-declaration-lookalike.musicxml"))
	require.NoError(t, err)
	context := validateAttributeSource(t, &scoreValidationSchema, string(source))
	want := []string{
		"/score-partwise/part/measure/@{xmlns}rubbish",
		"/score-partwise/part/measure/attributes/staves/@{xmlns}rubbish",
		"/score-partwise/part/measure/note/pitch/step/@{xmlns}rubbish",
	}
	require.Len(t, context.issues, len(want))
	for index, path := range want {
		assert.Equal(t, "attribute", context.issues[index].Constraint)
		assert.Equal(t, path, context.issues[index].Path)
	}
}
