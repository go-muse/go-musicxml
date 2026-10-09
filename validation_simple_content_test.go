package musicxml

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Raw-source expectations are replayable with the unchanged pre-repair generator
// and runtime. They exercise facets declared on complex simpleContent restrictions.
func validationSimpleContentCases() []validationSourceCase {
	var tests []validationSourceCase
	add := func(name, element, attrs, value string, comparable bool, constraints ...string) {
		var issues []string
		for _, constraint := range constraints {
			issues = append(issues, "/"+element+constraint)
		}
		tests = append(tests, validationSourceCase{name: name, source: wrapValidationElement(element, attrs, value), issues: issues, oracleComparable: comparable})
	}
	for _, test := range []struct {
		element, constraint string
		accepted, rejected  []string
	}{
		{"min", "minInclusive", []string{"1.0000000000000001", "+001.00000000000000010", "2"}, []string{"1", "1.0000000000000000"}},
		{"max", "maxInclusive", []string{"1", "1.0000000000000001"}, []string{"1.0000000000000002", "2"}},
		{"min-exclusive", "minExclusive", []string{"1.2501", "2"}, []string{"1.25", "+001.25000", "1"}},
		{"max-exclusive", "maxExclusive", []string{"1.2499", "-1"}, []string{"1.25", "+001.25000", "2"}},
		{"total", "totalDigits", []string{"0", "-000.000", "000.001000", "999.000"}, []string{"1000", ".0001", "99.99"}},
		{"fraction", "fractionDigits", []string{"1.23000", "+000.000", "12345.12"}, []string{"1.00100", "-.001"}},
		{"enum", "enumeration", []string{"1.2", "+0001.2000", "0", "-000.000"}, []string{"1.2000000000000001", "2"}},
		{"pattern", "pattern", []string{"01.20", "00.00"}, []string{"1.2", "01.200", "+01.20"}},
		{"digits", "fractionDigits", []string{"001.23000", "000.01000", "999"}, []string{".001", "-.001"}},
		{"layered", "minExclusive", []string{"1.2", "1.99"}, []string{"1", "0"}},
		{"extended", "maxExclusive", []string{"1.23"}, []string{"2", "3"}},
		{"anonymous", "maxInclusive", []string{"10.000", "1.23"}, []string{"10.1", "100"}},
		{"inline", "minInclusive", []string{"1.25", "2.49"}, []string{"1.24", "0"}},
	} {
		for _, value := range test.accepted {
			add(test.element+"/accept/"+value, test.element, ` code="ok"`, value, true)
		}
		for _, value := range test.rejected {
			add(test.element+"/reject/"+value, test.element, ` code="ok"`, value, true, ":"+test.constraint)
		}
	}
	add("digits total first", "digits", ` code="ok"`, "1.234", true, ":totalDigits")
	add("layered inherited fraction", "layered", ` code="ok"`, ".001", true, ":fractionDigits")
	add("layered upper bound", "layered", ` code="ok"`, "2", true, ":maxExclusive")
	add("extension inherits fraction", "extended", ` code="ok" extra="4"`, ".001", true, ":fractionDigits")
	add("anonymous inherits digits", "anonymous", ` code="ok"`, ".001", true, ":fractionDigits")
	add("inline local bound", "inline", ` code="ok"`, "2.5", true, ":maxExclusive")
	add("inline local digits", "inline", ` code="ok"`, "1.251", true, ":totalDigits")
	add("inline base upper bound", "inline", ` code="ok"`, "2.76", true, ":maxInclusive")
	for _, value := range []string{"", " ", "1e0", "NaN", ".", "１", "\u00a01", "1\u0085"} {
		add("lexical/"+value, "digits", ` code="ok"`, value, true, ":datatype")
	}
	for _, value := range []string{"18446744073709551616", "+018446744073709551616", "18446744073709551617"} {
		add("integer/"+value, "integer", "", value, true)
	}
	add("integer lower", "integer", "", "18446744073709551615", true, ":minInclusive")
	add("integer upper", "integer", "", "18446744073709551618", true, ":maxInclusive")
	add("integer enum equal", "integer-enum", "", "18446744073709551616", true)
	add("integer enum different", "integer-enum", "", "18446744073709551617", true, ":enumeration")
	add("arbitrary integer", "integer", "", strings.Repeat("9", 4096), false, ":maxInclusive")
	add("arbitrary decimal", "max", ` code="ok"`, "1."+strings.Repeat("0", 4096)+"1", false)
	add("arbitrary decimal reject", "min", ` code="ok"`, "1."+strings.Repeat("0", 4096)+"1", false, ":minInclusive")
	for _, value := range []string{"ab", " é界 ", "  a\tb  "} {
		if value == "  a\tb  " {
			add("length long token", "length", "", value, true, ":length")
		} else {
			add("length/"+value, "length", "", value, true)
		}
	}
	add("length short", "length", "", "a", true, ":length")
	add("length long", "length", "", "abc", true, ":length")
	add("min length", "length-range", "", "a", true, ":minLength")
	add("max length", "length-range", "", "abcd", true, ":maxLength")
	add("length range", "length-range", "", "abc", true)
	add("token enumeration", "token-enum", "", " \tA  B\n", true)
	add("token enumeration reject", "token-enum", "", "AB", true, ":enumeration")
	add("list accepted", "list", "", " \t01.200  -000.0100\n", true)
	add("list too short", "list", "", "1.2", true, ":length")
	add("list too long", "list", "", "1 2 3", true, ":length")
	add("list item datatype", "list", "", "1 invalid", true, ":datatype")
	add("required attribute", "digits", "", "1.2", true, "/@code:required")
	add("removed attribute", "digits", ` code="ok" removed="no"`, "1.2", true, "/@removed:prohibited")
	add("fixed attribute", "digits", ` code="ok" locked="no"`, "1.2", true, "/@locked:fixed")
	add("unknown attribute", "digits", ` code="ok" other="no"`, "1.2", true, "/@other:attribute")
	add("extension attribute", "extended", ` code="ok" extra="invalid"`, "1.2", true, "/@extra:datatype")
	add("attribute then scalar", "digits", ` code="ok" locked="no"`, ".001", true, "/@locked:fixed", ":fractionDigits")
	for _, element := range []string{"default", "fixed"} {
		add(element+" empty", element, ` code="ok"`, "", true)
		add(element+" explicit", element, ` code="ok"`, "+001.2000", true)
	}
	add("fixed equal spelling", "fixed", ` code="ok"`, "1.20", false) // libxml2 compares explicit element-fixed values lexically.
	add("fixed different", "fixed", ` code="ok"`, "1.21", true, ":fixed")
	add("fixed also facets", "fixed", ` code="ok"`, ".001", true, ":fixed", ":fractionDigits")
	add("fixed nil", "fixed", ` code="ok" n:nil="true"`, "", true, ":fixed")
	add("nil true", "digits", ` code="ok" n:nil="true"`, "", true)
	add("nil attribute", "digits", ` n:nil="true"`, "", true, "/@code:required")
	add("nil false facets", "digits", ` code="ok" n:nil="false"`, ".001", true, ":fractionDigits")
	add("nil content", "digits", ` code="ok" n:nil="true"`, ".001", true, ":nillable")
	add("child", "digits", ` code="ok"`, "1.2<length>ab</length>", true, ":simple-content")
	add("comments and PI", "digits", ` code="ok"`, "1<!--x--><?test ok?>.2", true)
	add("CDATA", "digits", ` code="ok"`, "<![CDATA[001.23000]]>", true)
	add("references", "digits", ` code="ok"`, "&#x31;.&#50;", true)
	add("ID and IDREF attributes", "digits", ` code="ok" id="P1" ref="P1"`, "1.2", true)
	// libxml2 2.9.x does not establish the ID/IDREF negative outcomes here.
	add("missing IDREF", "digits", ` code="ok" ref="missing"`, "1.2", false, "/@ref:IDREF")
	add("ID scalar", "references", "", "<identifier>P1</identifier><reference>P1</reference>", true)
	add("ID invalid scalar", "references", "", "<identifier>X1</identifier><reference>X1</reference>", true, "/identifier:pattern", "/reference:IDREF")
	add("ID duplicate", "references", "", "<identifier>P1</identifier><identifier>P1</identifier>", false, "/identifier[2]:ID")
	add("sibling scalar layers", "siblings", "", `<digits code="ok">1.2</digits><total code="ok">.001</total>`, true)
	return tests
}

func TestValidateSimpleContentRestrictions(t *testing.T) {
	t.Parallel()
	for _, test := range validationSimpleContentCases() {
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationSimpleContentGenerated, test.source), test.issues)
		})
	}
}

func TestSimpleContentRestrictionsAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	directory, err := filepath.Abs(filepath.Join("testdata", "validation"))
	require.NoError(t, err)
	for _, test := range validationSimpleContentCases() {
		if !test.oracleComparable {
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationSimpleContentGenerated, test.source), test.issues)
			assertXMLLintOutcome(t, xmllint, filepath.Join(directory, "simple-content-contract.xsd"), filepath.Join(directory, "catalog.xml"), test.source, len(test.issues) == 0)
		})
	}
}

func TestSimpleContentRestrictionImmutability(t *testing.T) {
	t.Parallel()
	snapshot := func() map[string]string {
		result := make(map[string]string)
		for name, schema := range validationSimpleContentGenerated.Types {
			encoded, err := json.Marshal(schema)
			require.NoError(t, err)
			result["type:"+name.Space+"\x00"+name.Local] = string(encoded)
		}
		for name, schema := range validationSimpleContentGenerated.Elements {
			encoded, err := json.Marshal(schema)
			require.NoError(t, err)
			result["element:"+name.Space+"\x00"+name.Local] = string(encoded)
		}
		return result
	}
	before := snapshot()
	for range 3 {
		for _, test := range validationSimpleContentCases() {
			context := validateAttributeSource(t, &validationSimpleContentGenerated, test.source)
			assertValidationIssues(t, context, test.issues)
		}
	}
	assert.Equal(t, before, snapshot())
	// The source tree, including lexical numeric spelling, remains untouched.
	source := `<digits code="keep" id="P1">+001.23000</digits>`
	node, err := parseValidationDocument([]byte(source))
	require.NoError(t, err)
	nodeBefore, err := json.Marshal(node)
	require.NoError(t, err)
	context := newTestValidationContext(&validationSimpleContentGenerated)
	context.validateElement(node, validationSimpleContentGenerated.Elements[validationQName{Local: "digits"}], "/digits")
	assert.Empty(t, context.issues)
	nodeAfter, err := json.Marshal(node)
	require.NoError(t, err)
	assert.Equal(t, string(nodeBefore), string(nodeAfter))
	assert.Equal(t, "+001.23000", node.Text.String())
}

func TestEffectiveComplexFailureRemainsFailure(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		schema *validationComplexSchema
	}{
		{"missing", &validationComplexSchema{Form: validationComplexSimpleContentRestriction}},
		{"unresolved", &validationComplexSchema{Form: validationComplexSimpleContentRestriction, Base: &validationTypeRef{Name: validationQName{Local: "absent"}}}},
		{"cycle", &validationComplexSchema{Form: validationComplexSimpleContentRestriction, Base: &validationTypeRef{Name: validationQName{Local: "cycle"}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			set := validationSchemaSet{Types: map[validationQName]*validationTypeSchema{{Local: "cycle"}: {Complex: test.schema}}}
			context := newTestValidationContext(&set)
			for range 3 {
				effective, ok := context.effectiveComplex(test.schema)
				assert.False(t, ok)
				assert.Nil(t, effective)
			}
		})
	}
}
