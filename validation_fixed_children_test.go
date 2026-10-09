package musicxml

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// libxml2 2.9.14 accepts a matching fixed mixed value with element
// children and rejects empty CDATA for a mixed empty-string fixed value.
// Those cases are normative regressions, not oracle parity.
func validationFixedChildCases() []validationSourceCase {
	var tests []validationSourceCase
	for _, element := range []struct{ name, value, wrapper string }{
		{"fixed-mixed", "kept", ""},
		{"fixed-mixed-empty", "", ""},
		{"fixed-mixed-derived", "kept", ""},
		{"fixed-mixed", "kept", "value-items"},
		{"local-fixed-mixed", "kept", "value-items"},
	} {
		path := "/" + element.name
		if element.wrapper != "" {
			path = "/" + element.wrapper + path
		}
		for _, nilAttribute := range []string{"", ` n:nil="false"`, ` n:nil="0"`} {
			for _, content := range []struct {
				name, value string
				child       bool
			}{
				{"empty", "", false},
				{"text", element.value, false},
				{"comment and PI", "<!--before--><?note allowed?>" + element.value + "<!--after-->", false},
				{"CDATA", "<![CDATA[" + element.value + "]]>", false},
				{"child without text", "<child>7</child>", true},
				{"child before text", "<child>7</child>" + element.value, true},
				{"child after text", element.value + "<child>7</child>", true},
				{"child surrounded by text", element.value[:len(element.value)/2] + "<child>7</child>" + element.value[len(element.value)/2:], true},
			} {
				source := wrapValidationElement(element.name, ` required="1"`+nilAttribute, content.value)
				if element.wrapper != "" {
					source = "<" + element.wrapper + ">" + source + "</" + element.wrapper + ">"
				}
				test := validationSourceCase{
					name:   element.wrapper + "/" + element.name + nilAttribute + " " + content.name,
					source: source, oracleComparable: !content.child && !(content.name == "CDATA" && element.value == ""),
				}
				if content.child {
					test.issues = []string{path + ":fixed"}
				}
				tests = append(tests, test)
			}
		}
	}
	for _, test := range []struct {
		name, attrs, content string
		issues               []string
	}{
		{"required attribute", "", "kept<child>7</child>", []string{"/fixed-mixed:fixed", "/fixed-mixed/@required:required"}},
		{"unknown attribute", ` required="1" rubbish="x"`, "kept<child>7</child>", []string{"/fixed-mixed:fixed", "/fixed-mixed/@rubbish:attribute"}},
		{"attribute datatype", ` required="bad"`, "kept<child>7</child>", []string{"/fixed-mixed:fixed", "/fixed-mixed/@required:datatype"}},
		{"attribute fixed", ` required="1" locked="other"`, "kept<child>7</child>", []string{"/fixed-mixed:fixed", "/fixed-mixed/@locked:fixed"}},
		{"child datatype", ` required="1"`, "kept<child>bad</child>", []string{"/fixed-mixed:fixed", "/fixed-mixed/child:datatype"}},
		{"child content model", ` required="1"`, "kept<unknown/>", []string{"/fixed-mixed:fixed", "/fixed-mixed/unknown:content-model"}},
		{"true nil", ` required="1" n:nil="true"`, "kept<child>7</child>", []string{"/fixed-mixed:fixed", "/fixed-mixed:nillable"}},
		{"malformed nil", ` required="1" n:nil="maybe"`, "kept<child>7</child>", []string{"/fixed-mixed/@{" + validationXSINamespace + "}nil:datatype", "/fixed-mixed:fixed"}},
		{"text mismatch", ` required="1"`, "other", []string{"/fixed-mixed:fixed"}},
		{"text and child mismatch", ` required="1"`, "other<child>7</child>", []string{"/fixed-mixed:fixed"}},
	} {
		tests = append(tests, validationSourceCase{
			name: test.name, source: wrapValidationElement("fixed-mixed", test.attrs, test.content),
			issues: test.issues, oracleComparable: true,
		})
	}
	for _, test := range []validationSourceCase{
		{name: "builtin simple child", source: `<fixed>kept<child>7</child></fixed>`, issues: []string{"/fixed:fixed", "/fixed:simple-content"}, oracleComparable: true},
		{name: "simple-content child", source: `<fixed-content required="1">7<child>7</child></fixed-content>`, issues: []string{"/fixed-content:fixed", "/fixed-content:simple-content"}, oracleComparable: true},
		{name: "anyType child", source: `<fixed-any>kept<child/></fixed-any>`, issues: []string{"/fixed-any:fixed"}},
		{name: "anyType text", source: `<fixed-any>kept</fixed-any>`, oracleComparable: true},
		{name: "anyType empty", source: `<fixed-any/>`, oracleComparable: true},
		{name: "nonnillable true child", source: `<fixed-mixed-plain xmlns:n="` + validationXSINamespace + `" n:nil="true" required="1">kept<child>7</child></fixed-mixed-plain>`, issues: []string{"/fixed-mixed-plain/@{" + validationXSINamespace + "}nil:nillable", "/fixed-mixed-plain:fixed"}, oracleComparable: true},
		{name: "default permits child", source: `<default-mixed required="1">kept<child>7</child></default-mixed>`, oracleComparable: true},
		{name: "default permits different text", source: `<default-mixed required="1">other<child>7</child></default-mixed>`, oracleComparable: true},
		{name: "unconstrained mixed permits child", source: `<mixed required="1">kept<child>7</child></mixed>`, oracleComparable: true},
	} {
		tests = append(tests, test)
	}
	return tests
}

func assertFixedChildCase(t *testing.T, test validationSourceCase) {
	t.Helper()
	context := validateAttributeSource(t, validationSchemaFor(test.schema), test.source)
	assertValidationIssues(t, context, test.issues)
}

func TestValidateFixedElementChildren(t *testing.T) {
	t.Parallel()
	for _, test := range validationFixedChildCases() {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertFixedChildCase(t, test)
		})
	}
}

func TestFixedElementChildrenAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	directory, err := filepath.Abs(filepath.Join("testdata", "validation"))
	require.NoError(t, err)
	for _, test := range validationFixedChildCases() {
		if !test.oracleComparable {
			continue // The independent normative suite covers this oracle gap.
		}
		t.Run(test.name, func(t *testing.T) {
			assertFixedChildCase(t, test)
			assertXMLLintOutcome(t, xmllint, filepath.Join(directory, "nil-contract.xsd"),
				filepath.Join(directory, "catalog.xml"), test.source, len(test.issues) == 0)
		})
	}
}

func TestFixedElementChildrenPreserveAssessment(t *testing.T) {
	t.Parallel()
	for _, reference := range []string{"own", "missing"} {
		source := `<fixed-mixed required="1" id="own" ref="` + reference + `">kept<child>7</child></fixed-mixed>`
		context := validateAttributeSource(t, &validationNilGenerated, source)
		assert.Equal(t, map[string]string{"own": "/fixed-mixed/@id"}, context.identifiers)
		assert.Equal(t, []validationIdentityReference{{value: reference, path: "/fixed-mixed/@ref"}}, context.references)
		want := []string{"/fixed-mixed:fixed"}
		if reference == "missing" {
			want = append(want, "/fixed-mixed/@ref:IDREF")
		}
		assertFixedChildCase(t, validationSourceCase{source: source, issues: want})
	}
	// A fixed constraint failure does not stop the ordinary matched-child walk.
	source := `<fixed-mixed required="1" ref="target">kept<child>7</child><identity>target</identity></fixed-mixed>`
	context := validateAttributeSource(t, &validationNilGenerated, source)
	assert.Equal(t, map[string]string{"target": "/fixed-mixed/identity"}, context.identifiers)
	assert.Equal(t, []validationIdentityReference{{value: "target", path: "/fixed-mixed/@ref"}}, context.references)
	assertFixedChildCase(t, validationSourceCase{source: source, issues: []string{"/fixed-mixed:fixed"}})
	context = validateAttributeSource(t, &validationNilGenerated, `<fixed-mixed required="1">other<child>7</child></fixed-mixed>`)
	require.Len(t, context.issues, 1)
	assert.Equal(t, "element with a fixed value must not contain child elements", context.issues[0].Message)
	root, err := parseValidationDocument([]byte(`<fixed-mixed required="1">kept<child>7</child></fixed-mixed>`))
	require.NoError(t, err)
	require.Len(t, root.Children, 1)
	child := root.Children[0]
	originalAttributes := append([]validationAttribute(nil), root.Attrs...)
	for range 2 {
		context := validationContext{schema: &validationNilGenerated, effective: make(map[*validationComplexSchema]*validationEffectiveComplex), identifiers: make(map[string]string)}
		context.validateElement(root, validationNilGenerated.Elements[validationQName{Local: "fixed-mixed"}], "/fixed-mixed")
		require.Len(t, context.issues, 1)
		assert.Equal(t, "fixed", context.issues[0].Constraint)
		assert.Equal(t, "kept", root.Text.String())
		assert.Equal(t, originalAttributes, root.Attrs)
		require.Len(t, root.Children, 1)
		assert.Same(t, child, root.Children[0])
		assert.Equal(t, "7", child.Text.String())
	}
}
