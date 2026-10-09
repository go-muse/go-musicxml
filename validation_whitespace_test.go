package musicxml

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These original-source expectations replay on the pre-repair production tree.
// Only explicit scalar element-fixed equalities affected by libxml2's lexical
// comparison are omitted from the external subset, never from internal tests.
func validationWhitespaceCases() []validationSourceCase {
	var tests []validationSourceCase
	add := func(name, element, attrs, value string, comparable bool, constraints ...string) {
		var issues []string
		for _, constraint := range constraints {
			issues = append(issues, "/"+element+constraint)
		}
		tests = append(tests, validationSourceCase{name: name, source: wrapValidationElement(element, attrs, value), issues: issues, oracleComparable: comparable})
	}
	for _, element := range []string{"pattern", "inline", "list"} {
		for i, value := range []string{"a b", " a  b ", "\ta\nb\t", "&#xD;a&#x9; \nb&#xD;", "<![CDATA[ a ]]><!--split--><?keep x?>  b ", "a&#32;&#32;b"} {
			add(fmt.Sprintf("%s normalized %d", element, i), element, "", value, true)
		}
		add(element+" wrong value", element, "", " a c ", true, ":pattern")
		for _, value := range []string{"a\u00a0b", "a\u0085b", "\u00a0a b", "a b\u0085"} {
			add(element+" non XML S "+value, element, "", value, true, ":pattern")
		}
	}
	for i, value := range []string{"a", " a ", "\ta\n", "&#xD;a&#xD;", " \u00a0 ", " \u0085 "} {
		add(fmt.Sprintf("inherited length %d", i), "length", "", value, true)
	}
	add("inherited length short", "length", "", " \t\n", true, ":length")
	add("inherited length long", "length", "", " a b ", true, ":length")
	for _, value := range []string{"", " \t\n&#xD;"} {
		add("empty collapse "+value, "empty", "", value, true)
		add("minLength "+value, "range", "", value, true, ":minLength")
	}
	add("maxLength normalized", "range", "", " a b c ", true, ":maxLength")
	add("length range normalized", "range", "", " a b ", true)
	add("NBSP not empty", "empty", "", "\u00a0", true, ":length")
	add("NEL not empty", "empty", "", "\u0085", true, ":length")
	for i, value := range []string{"a b", "a\tb", "a\nb", "a&#xD;b"} {
		add(fmt.Sprintf("replace inherited pattern %d", i), "pattern-replace", "", value, true)
	}
	add("replace keeps repeated spaces", "pattern-replace", "", "a\t b", true, ":pattern")
	add("replace keeps padding", "pattern-replace", "", " a b ", true, ":pattern")
	for _, element := range []string{"enum-same", "enum-derived"} {
		for _, value := range []string{"a", " a ", "&#x9;a&#xA;"} {
			add(element+" base scoped "+value, element, "", value, true, ":enumeration")
		}
	}
	add("preserve enum accepts padding", "enum-base", "", " a ", true)
	add("preserve enum rejects unpadded", "enum-base", "", "a", true, ":enumeration")
	for _, value := range []string{"a b", " a \tb ", "a&#xD; b"} {
		add("enum collapsed base "+value, "enum-collapsed", "", value, true)
	}
	add("enum collapse wrong", "enum-collapsed", "", "ab", true, ":enumeration")
	for _, value := range []string{" a b ", " a\tb ", " a&#xD;b "} {
		add("enum replace base "+value, "enum-replaced", "", value, true)
	}
	add("enum replace retains padding", "enum-replaced", "", "a b", true, ":enumeration")
	add("union token first", "union-token-first", "", " a ", true)
	add("union token first refs", "union-token-first", "", "&#xD;a&#x9;", true)
	add("union string first", "union-string-first", "", " a ", true, ":pattern")
	add("union string first unpadded", "union-string-first", "", "a", true)
	add("union no outer backtrack", "union-no-backtrack", "", " a ", true, ":pattern")
	add("union failed member view isolated", "union-isolation", "", " a ", true)
	add("union all fail", "union-isolation", "", " y ", true, ":union")
	add("union selected member outer failure", "union-isolation", "", " x ", true, ":pattern")
	add("union list first normalizes", "list-first", "", " a  b ", true)
	add("union list second not selected", "list-second", "", " a  b ", true, ":pattern")
	add("union mixed normalization", "union-replace", "", " a\tb ", true)
	add("union replace not collapse", "union-replace", "", "a\tb", true, ":pattern")
	for _, element := range []string{"complex", "extended", "complex-inline"} {
		add(element+" inherited facets", element, ` code=" a&#x9; b "`, " \ta\n b ", true)
		add(element+" scalar failure", element, ` code="a b"`, " a c ", true, ":pattern")
		add(element+" attribute failure", element, ` code=" a c "`, "a b", true, "/@code:pattern")
	}
	add("required attribute on nil", "complex", ` n:nil="true"`, "", true, "/@code:required")
	add("nil skips scalar", "complex", ` code="a b" n:nil="true"`, "", true)
	add("nil content still forbidden", "complex", ` code="a b" n:nil="true"`, " ", true, ":nillable")
	add("nil false scalar", "pattern", ` n:nil="false"`, " a  b ", true)
	add("simple child rejected", "pattern", "", "a b<child/>", true, ":simple-content")
	add("complex child rejected", "complex", ` code="a b"`, "a b<child/>", true, ":simple-content")
	for _, element := range []string{"default", "complex-default"} {
		attrs := ""
		if element == "complex-default" {
			attrs = ` code="a b"`
		}
		add(element+" empty substitutes normalized constraint", element, attrs, "", true)
		add(element+" comment only substitutes", element, attrs, "<!--empty-->", true)
		add(element+" whitespace is explicit", element, attrs, " \t ", true, ":pattern")
		add(element+" explicit normalized", element, attrs, " a \tb ", true)
	}
	for _, element := range []string{"preserve", "replace", "collapse", "inherited", "token", "normalized"} {
		fixed := "a b"
		if element == "preserve" || element == "replace" || element == "inherited" {
			fixed = " a b "
		}
		add(element+" fixed exact", element, "", fixed, true)
		add(element+" fixed empty", element, "", "", true)
		add(element+" fixed different", element, "", "a c", true, ":fixed")
	}
	add("preserve fixed no collapse", "preserve", "", "a b", true, ":fixed")
	add("preserve fixed no replace", "preserve", "", " a\tb ", true, ":fixed")
	add("replace fixed retains padding", "replace", "", "a b", true, ":fixed")
	add("replace fixed equivalent", "replace", "", " a\tb ", false)
	add("collapse fixed equivalent", "collapse", "", " \ta  b\n", false)
	add("inherited fixed equivalent", "inherited", "", "a b", false)
	add("token fixed equivalent", "token", "", " a\tb ", false)
	add("normalized fixed equivalent", "normalized", "", "a\tb", false)
	add("collapse fixed NBSP", "collapse", "", "a\u00a0b", true, ":fixed")
	add("collapse fixed NEL", "collapse", "", "a\u0085b", true, ":fixed")
	add("complex fixed equivalent", "complex-fixed", ` code="a b" extra="a&#x9;b"`, " a  b ", false)
	add("complex fixed exact", "complex-fixed", ` code="a b"`, "a b", true)
	add("complex fixed empty", "complex-fixed", ` code="a b"`, "", true)
	add("complex fixed nil", "complex-fixed", ` code="a b" n:nil="true"`, "", true, ":fixed")
	add("fixed nil", "collapse", ` n:nil="true"`, "", true, ":fixed")
	add("fixed child", "collapse", "", "a b<child/>", true, ":fixed", ":simple-content")
	add("complex fixed child", "complex-fixed", ` code="a b"`, "a b<child/>", true, ":fixed", ":simple-content")
	add("attribute preserve refs", "attributes", ` preserve=" a&#x9;b "`, "", true, "/@preserve:fixed")
	add("attribute replace refs", "attributes", ` replace=" a&#x9;b "`, "", true)
	add("attribute collapse refs", "attributes", ` collapse=" &#xD;a&#x9; b&#xA; "`, "", true)
	add("attribute inherited refs", "attributes", ` inherited="a&#x9;b"`, "", true)
	add("attribute inline refs", "attributes", ` inline=" a&#x9; b "`, "", true)
	add("attribute pattern refs", "attributes", ` pattern=" a&#x9; b "`, "", true)
	add("attribute enum base policy", "attributes", ` enum=" a "`, "", true, "/@enum:enumeration")
	add("attribute independent failures", "attributes", ` replace="a b" pattern=" a c "`, "", true, "/@replace:fixed", "/@pattern:pattern")
	add("mixed fixed initial text", "mixed", "", " a b ", true)
	add("mixed fixed retains padding", "mixed", "", "a b", true, ":fixed")
	add("mixed fixed retains tabs", "mixed", "", " a\tb ", true, ":fixed")
	add("siblings different policies", "siblings", "", `<complex code="a b"> a  b </complex><pattern-replace>a&#x9;b</pattern-replace><enum-base> a </enum-base>`, true)
	return tests
}

func TestValidateEffectiveWhitespace(t *testing.T) {
	t.Parallel()
	for _, test := range validationWhitespaceCases() {
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationWhitespaceGenerated, test.source), test.issues)
		})
	}
}

func TestEffectiveWhitespaceAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	directory, err := filepath.Abs(filepath.Join("testdata", "validation"))
	require.NoError(t, err)
	for _, test := range validationWhitespaceCases() {
		if !test.oracleComparable {
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationWhitespaceGenerated, test.source), test.issues)
			assertXMLLintOutcome(t, xmllint, filepath.Join(directory, "whitespace-contract.xsd"), filepath.Join(directory, "catalog.xml"), test.source, len(test.issues) == 0)
		})
	}
}

func TestEffectiveWhitespaceDiagnosticsAndImmutability(t *testing.T) {
	t.Parallel()
	snapshot := func() map[string]string {
		result := make(map[string]string)
		for name, schema := range validationWhitespaceGenerated.Types {
			encoded, err := json.Marshal(schema)
			require.NoError(t, err)
			result["type:"+name.Local] = string(encoded)
		}
		for name, schema := range validationWhitespaceGenerated.Elements {
			encoded, err := json.Marshal(schema)
			require.NoError(t, err)
			result["element:"+name.Local] = string(encoded)
		}
		return result
	}
	before := snapshot()
	for range 3 {
		for _, test := range validationWhitespaceCases() {
			assertValidationIssues(t, validateAttributeSource(t, &validationWhitespaceGenerated, test.source), test.issues)
		}
	}
	assert.Equal(t, before, snapshot())
	for _, test := range []struct {
		source, text string
		messages     []string
	}{
		{`<complex code=" a&#x9;c "> a&#xD; c </complex>`, " a\r c ", []string{`value " a\tc " does not match XSD pattern "a b"`, `value " a\r c " does not match XSD pattern "a b"`}},
		{`<enum-derived> a </enum-derived>`, " a ", []string{`value " a " is not one of the allowed values`}},
		{`<union-string-first> a </union-string-first>`, " a ", []string{`value " a " does not match XSD pattern "a"`}},
		{`<replace> a&#x9;c </replace>`, " a\tc ", []string{`value " a\tc " does not equal fixed value " a b "`}},
		{`<complex code=" a&#x9; b "> a<![CDATA[  b ]]></complex>`, " a  b ", nil},
	} {
		node, err := parseValidationDocument([]byte(test.source))
		require.NoError(t, err)
		encoded, err := json.Marshal(node)
		require.NoError(t, err)
		attrs := append([]validationAttribute(nil), node.Attrs...)
		context := newTestValidationContext(&validationWhitespaceGenerated)
		name := validationName(node.Name)
		context.validateElement(node, validationWhitespaceGenerated.Elements[name], "/"+name.Local)
		var messages []string
		for _, issue := range context.issues {
			messages = append(messages, issue.Message)
		}
		assert.Equal(t, test.messages, messages)
		after, err := json.Marshal(node)
		require.NoError(t, err)
		assert.Equal(t, string(encoded), string(after))
		assert.Equal(t, attrs, node.Attrs)
		assert.Equal(t, test.text, node.Text.String())
	}
}

func TestWhitespaceFixedComparisonScope(t *testing.T) {
	t.Parallel()
	context := newTestValidationContext(&validationWhitespaceGenerated)
	for _, name := range []string{"string", "normalizedString", "token", "language", "Name", "NCName", "NMTOKEN", "ID", "IDREF", "ENTITY"} {
		reference := &validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: name}}
		want := name != "string"
		assert.Equal(t, want, context.simpleValuesEqual(reference, "a\tb", "a b"), name)
		assert.Equal(t, want, context.elementFixedValuesEqual(reference, "a\tb", "a b"), name)
	}
	// Do not infer an atomic string value space from the legacy builtin fallback.
	for _, name := range []string{"list-type", "token-string", "string-token"} {
		reference := &validationTypeRef{Name: validationQName{Local: name}}
		assert.False(t, context.simpleValuesEqual(reference, " a  b ", "a b"), name)
		assert.False(t, context.elementFixedValuesEqual(reference, " a  b ", "a b"), name)
	}
	for _, name := range []string{"boolean", "date", "dateTime", "anyURI", "QName", "IDREFS"} {
		reference := &validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: name}}
		assert.False(t, context.elementFixedValuesEqual(reference, " a ", "a"), name)
	}
	assert.True(t, context.simpleValuesEqual(&validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "decimal"}}, " +001.200 ", "1.2"))
	assert.True(t, context.elementFixedValuesEqual(&validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "integer"}}, " +001 ", "1"))
	assert.False(t, context.elementFixedValuesEqual(&validationTypeRef{Name: validationQName{Local: "replace-type"}}, " a b ", "a b"))
	assert.True(t, context.elementFixedValuesEqual(&validationTypeRef{Name: validationQName{Local: "inherited-type"}}, " a\tb ", "a b"))
}

// Failure-safe metadata probes intentionally include cycles, so the pre-repair
// behavior replay selects the source tests instead of executing these probes.
func TestWhitespaceMetadataFailures(t *testing.T) {
	t.Parallel()
	builtin := func(name string) *validationSimpleMember {
		return &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: name}}
	}
	cycle := &validationSimpleSchema{Form: validationSimpleRestriction}
	cycle.Base = &validationSimpleMember{Inline: cycle}
	unionCycle := &validationSimpleSchema{Form: validationSimpleUnion}
	unionCycle.Members = []*validationSimpleMember{{Inline: unionCycle}, builtin("string")}
	listCycle := &validationSimpleSchema{Form: validationSimpleList}
	listCycle.Item = &validationSimpleMember{Inline: listCycle}
	for _, test := range []struct {
		name   string
		schema *validationSimpleSchema
	}{
		{"nil", nil},
		{"missing base", &validationSimpleSchema{Form: validationSimpleRestriction}},
		{"missing name", &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Name: validationQName{Local: "missing"}}, WhiteSpace: "collapse"}},
		{"invalid policy", &validationSimpleSchema{Form: validationSimpleRestriction, Base: builtin("string"), WhiteSpace: "invalid"}},
		{"invalid builtin", &validationSimpleSchema{Form: validationSimpleRestriction, Base: builtin("invalid")}},
		{"invalid form", &validationSimpleSchema{}},
		{"empty union", &validationSimpleSchema{Form: validationSimpleUnion}},
		{"nil union member", &validationSimpleSchema{Form: validationSimpleUnion, Members: []*validationSimpleMember{nil, builtin("string")}}},
		{"missing item", &validationSimpleSchema{Form: validationSimpleList}},
		{"cycle", cycle}, {"union cycle", unionCycle}, {"list cycle", listCycle},
		{"named cycle", &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Name: validationQName{Local: "named-cycle"}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			context := newTestValidationContext(&validationSchemaSet{Types: map[validationQName]*validationTypeSchema{{Local: "named-cycle"}: {Simple: test.schema}}})
			for range 3 {
				failure := context.validateSimple(test.schema, " a ")
				if assert.NotNil(t, failure) {
					assert.Equal(t, "schema", failure.constraint)
				}
				reference := &validationTypeRef{InlineSimple: test.schema}
				assert.False(t, context.simpleValuesEqual(reference, " a ", "a"))
				assert.False(t, context.elementFixedValuesEqual(reference, " a ", "a"))
			}
		})
	}
	// Valid siblings following a failed lookup remain independently usable.
	context := newTestValidationContext(&validationWhitespaceGenerated)
	for range 3 {
		assert.NotNil(t, context.validateSimple(cycle, "a"))
		assert.Nil(t, context.validateSimple(validationWhitespaceGenerated.Types[validationQName{Local: "pattern-collapse"}].Simple, " a  b "))
	}
	assert.Equal(t, "a\u00a0b\u0085c", collapseValidationWhitespace(" a\u00a0b\u0085c "))
	assert.Equal(t, "a b", collapseValidationWhitespace(strings.Repeat(" \t\r\n", 100)+"a\tb"))
}

func TestWhitespaceInheritedNumericDiagnostics(t *testing.T) {
	t.Parallel()
	context := newTestValidationContext(&validationSchemaSet{})
	for _, builtin := range []string{"integer", "decimal", "double"} {
		base := &validationSimpleSchema{Form: validationSimpleRestriction,
			Base:            &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: builtin}},
			HasMinExclusive: true, MinExclusive: "2"}
		derived := &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Inline: base}}
		value := " \t001 \n"
		failure := context.validateSimple(derived, value)
		require.NotNil(t, failure)
		assert.Equal(t, "minExclusive", failure.constraint)
		assert.Equal(t, fmt.Sprintf("value %q violates minExclusive=%q", value, "2"), failure.message)
		value = " \tinvalid \n"
		failure = context.validateSimple(derived, value)
		require.NotNil(t, failure)
		assert.Equal(t, "datatype", failure.constraint)
		assert.Equal(t, fmt.Sprintf("value %q is not valid for xs:%s", value, builtin), failure.message)
	}
}
