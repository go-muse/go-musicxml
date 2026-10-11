package musicxml

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validationPrimitiveCases() []validationSourceCase {
	var tests []validationSourceCase
	add := func(name, element, attrs, value string, constraints ...string) {
		var issues []string
		for _, constraint := range constraints {
			issues = append(issues, "/"+element+constraint)
		}
		tests = append(tests, validationSourceCase{name: name, source: wrapValidationElement(element, attrs, value), issues: issues, oracleComparable: true})
	}
	for _, item := range []struct {
		element        string
		valid, invalid []string
	}{
		{"list", []string{"true false", "1 0", " 1&#x9;false ", "tr<!--split-->ue 0"}, []string{"false true", "1 1", "true", "true false true", ""}},
		{"boolean-enum", []string{"1", "true", " true "}, []string{"false", "0", "1.0", "+1"}},
		{"number-enum", []string{"1", "1.0", "+01.0"}, []string{"true", "false", "0"}},
		{"boolean-uri-enum", []string{"true", "1", "&#x9;1&#xA;"}, []string{"false", "0", "TRUE", "path"}},
		{"items", []string{"true false", "1 0"}, []string{"1.0 0", "true 0.0", "false true"}},
		{"nested", []string{"true", " true "}, []string{"1", "1.0", "false"}},
		{"nested-list", []string{"true path", " true&#x9;path "}, []string{"1 path", "true paTh", "path true"}},
	} {
		for _, value := range item.valid {
			add(item.element+"/accept/"+value, item.element, "", value)
		}
		for _, value := range item.invalid {
			add(item.element+"/reject/"+value, item.element, "", value, ":enumeration")
		}
	}
	for _, item := range []struct{ element, equal, different string }{
		{"boolean-fixed", "1", "false"}, {"false-fixed", "0", "true"}, {"list-fixed", "1 0", "0 1"}, {"uri-fixed", " relative/path ", "relative/other"},
	} {
		add(item.element+" equal", item.element, "", item.equal)
		add(item.element+" different", item.element, "", item.different, ":fixed")
		add(item.element+" empty", item.element, "", "")
	}
	add("default empty", "boolean-default", "", "")
	add("boolean fixed lexical pattern", "pattern-fixed", "", "1", ":pattern")
	add("boolean pattern exact", "pattern-fixed", "", "true")
	add("complex equality", "complex", ` code="ok"`, "1 0")
	add("complex empty", "complex", ` code="ok"`, "")
	add("complex required", "complex", "", "1 0", "/@code:required")
	add("nilled fixed", "boolean-fixed", ` n:nil="true"`, "", ":fixed")
	add("false nil ordinary", "boolean-fixed", ` n:nil="0"`, "1")
	add("fixed child", "boolean-fixed", "", "true<child/>", ":fixed", ":simple-content")
	add("attributes equal", "attributes", ` boolean="1" false="0" list="1 0" union="1" uri=" relative/path "`, "")
	add("attribute different", "attributes", ` boolean="false"`, "", "/@boolean:fixed")
	add("attribute invalid", "attributes", ` boolean="yes"`, "", "/@boolean:fixed", "/@boolean:datatype")
	add("list invalid item", "list", "", "yes false", ":datatype")
	for _, item := range []struct{ attribute, equal, different string }{
		{"space", "  foo&#x9;&#xA;bar ", "foo%20bar"},
		{"escaped", " foo%2Fbar ", "foo%2fbar"},
		{"unicode", " https://例.example/楽譜 ", "https://例.example/楽譜2"},
		{"host", "https://EXAMPLE.com/x", "https://example.com/x"},
		{"scheme", "HTTP://example.com/x", "http://example.com/x"},
		{"relative", "foo/./bar", "foo/bar"},
		{"tilde", "foo/%7Ebar", "foo/~bar"},
	} {
		add("URI "+item.attribute+" equal", "uri-attributes", " "+item.attribute+`="`+item.equal+`"`, "")
		add("URI "+item.attribute+" distinct", "uri-attributes", " "+item.attribute+`="`+item.different+`"`, "", "/@"+item.attribute+":fixed")
	}
	for _, value := range []string{"true", "1", "false", "0", "&#x9;true&#xA;"} {
		add("boolean lexical accept/"+value, "boolean-value", "", value)
	}
	for _, value := range []string{"TRUE", "False", "+1", "01", "1.0", "true false", " true ", ""} {
		add("boolean lexical reject/"+value, "boolean-value", "", value, ":datatype")
	}
	// libxml2 2.9.14 compares these explicit element-fixed values lexically.
	// Keep the normative cases internally; retain all attribute-fixed controls
	// and invalid outcomes in the same-byte external matrix.
	for index := range tests {
		switch tests[index].name {
		case "boolean-fixed equal", "false-fixed equal", "list-fixed equal", "uri-fixed equal", "complex equality", "false nil ordinary":
			tests[index].oracleComparable = false
		}
	}
	return tests
}

func TestValidatePrimitiveValues(t *testing.T) {
	t.Parallel()
	for _, test := range validationPrimitiveCases() {
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationPrimitiveGenerated, test.source), test.issues)
		})
	}
}

func TestPrimitiveValuesAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	directory, err := filepath.Abs("testdata/validation")
	require.NoError(t, err)
	for _, test := range validationPrimitiveCases() {
		if !test.oracleComparable {
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationPrimitiveGenerated, test.source), test.issues)
			assertXMLLintOutcome(t, xmllint, filepath.Join(directory, "primitive-value-contract.xsd"), filepath.Join(directory, "catalog.xml"), test.source, len(test.issues) == 0)
		})
	}
}

func TestPrimitiveValueDomains(t *testing.T) {
	t.Parallel()
	context := newTestValidationContext(&validationPrimitiveGenerated)
	for _, test := range []struct {
		leftType, left, rightType, right string
		equal                            bool
	}{
		{"boolean", "true", "boolean", "1", true}, {"boolean", "false", "boolean", "0", true},
		{"boolean", "true", "boolean", "false", false}, {"boolean", "true", "anyURI", "true", false},
		{"date", "2000-01-01", "anyURI", "2000-01-01", false}, {"float", "1", "double", "1", false},
		{"float", "1", "anyURI", "1", false}, {"hexBinary", "ABCD", "base64Binary", "ABCD", false},
		{"QName", "name", "NOTATION", "name", false}, {"QName", "name", "anyURI", "name", false},
		{"dateTime", "2000-01-01T00:00:00", "string", "2000-01-01T00:00:00", false},
		{"duration", "P1D", "anyURI", "P1D", false}, {"gYear", "2000", "anyURI", "2000", false},
		{"gYearMonth", "2000-01", "anyURI", "2000-01", false}, {"gMonthDay", "--01-01", "anyURI", "--01-01", false},
		{"gDay", "---01", "anyURI", "---01", false}, {"gMonth", "--01", "anyURI", "--01", false},
		{"time", "00:00:00", "string", "00:00:00", false},
		{"string", "true", "anyURI", "true", false}, {"integer", "1", "boolean", "1", false},
		{"integer", "+01", "decimal", "1.0", true}, {"token", " a ", "string", "a", true},
		{"anyURI", " relative/path ", "anyURI", "relative/path", true},
		{"anyURI", "relative/path", "anyURI", "relative/./path", false},
	} {
		t.Run(test.leftType+"/"+test.left+"/"+test.rightType+"/"+test.right, func(t *testing.T) {
			left, failure := context.assessBuiltinValue(test.leftType, test.left, test.left)
			require.Nil(t, failure)
			right, failure := context.assessBuiltinValue(test.rightType, test.right, test.right)
			require.Nil(t, failure)
			assert.Equal(t, test.equal, left.value.equal(right.value))
			assert.Equal(t, test.equal, right.value.equal(left.value))
		})
	}
}

func TestPrimitiveValueImmutability(t *testing.T) {
	t.Parallel()
	schemaBefore := snapshotValidationSchema(t, &validationPrimitiveGenerated)
	for _, test := range validationPrimitiveCases() {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			node, err := parseValidationDocument([]byte(test.source))
			require.NoError(t, err)
			before := snapshotValidationNode(t, node)
			for range 3 {
				context := newTestValidationContext(&validationPrimitiveGenerated)
				context.validateElement(node, validationPrimitiveGenerated.Elements[validationQName{Local: node.Name.Local}], "/"+node.Name.Local)
				context.validateIdentityReferences()
				assertValidationIssues(t, context, test.issues)
				assert.Equal(t, before, snapshotValidationNode(t, node))
			}
			assert.Equal(t, schemaBefore, snapshotValidationSchema(t, &validationPrimitiveGenerated))
		})
	}
}

// URI comparison never dereferences supplied values, even if a responsive URL
// is used. A real control request proves that the counter is connected.
func TestPrimitiveURIValuesDoNotFetch(t *testing.T) {
	t.Parallel()
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { requests.Add(1); w.WriteHeader(http.StatusNoContent) }))
	defer server.Close()
	response, err := server.Client().Get(server.URL + "/control")
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.EqualValues(t, 1, requests.Load())
	requests.Store(0)
	for _, value := range []string{server.URL + "/value", " " + server.URL + "/value "} {
		assertValidationIssues(t, validateAttributeSource(t, &validationPrimitiveGenerated, wrapValidationElement("uri-value", "", value)), nil)
		assertValidationIssues(t, validateAttributeSource(t, &validationPrimitiveGenerated, wrapValidationElement("uri-fixed", "", value)), []string{"/uri-fixed:fixed"})
		assertValidationIssues(t, validateAttributeSource(t, &validationPrimitiveGenerated, wrapValidationElement("attributes", ` uri="`+value+`"`, "")), []string{"/attributes/@uri:fixed"})
	}
	assert.Zero(t, requests.Load())
}
