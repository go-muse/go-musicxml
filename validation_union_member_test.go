package musicxml

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validationUnionMemberCases() []validationSourceCase {
	var tests []validationSourceCase
	add := func(name, element, attrs, value string, constraints ...string) {
		var issues []string
		for _, constraint := range constraints {
			issues = append(issues, "/"+element+constraint)
		}
		tests = append(tests, validationSourceCase{name: name, source: wrapValidationElement(element, attrs, value), issues: issues, oracleComparable: true})
	}
	for _, test := range []struct {
		element            string
		accepted, rejected []string
	}{
		{"composed-enum", []string{"1"}, []string{"+01", " 1 ", "1.0", "0"}},
		{"inline-enum", []string{"1"}, []string{"+01", "1.0", "0"}},
		{"deep-enum", []string{"2"}, []string{"+02.0", " 2 ", "1", "x"}},
		{"numeric-enum", []string{"2", "+02.0", " 002.00 "}, []string{"1", "3", "x"}},
		{"fallback-enum", []string{"x", " x "}, []string{"1", "+01", "y"}},
		{"reordered-enum", []string{"1", "+01", "1.00"}, []string{"2", "x"}},
		{"direct", []string{"0"}, []string{"1", "+00", "0.0"}},
		{"direct-numeric", []string{"1", "+01.0"}, []string{"2", "+02.0"}},
		{"union-list", []string{"0", "0 0", ""}, []string{"1", "0 1", "+00"}},
		{"atomic-enum", []string{"1", "+01.0"}, []string{"0", "2"}},
		{"list-enum", []string{"1"}, []string{"+01", "1 2", ""}},
		{"complex", []string{"1"}, []string{"+01"}},
	} {
		for _, value := range test.accepted {
			add(test.element+"/accept/"+value, test.element, "", value)
		}
		for _, value := range test.rejected {
			add(test.element+"/reject/"+value, test.element, "", value, ":enumeration")
		}
	}
	add("element fixed exact", "composed-fixed", "", "1")
	add("element fixed domain", "composed-fixed", "", "+01", ":fixed")
	add("attribute fixed exact", "attributes", ` composed="1"`, "")
	add("attribute fixed domain", "attributes", ` composed="+01"`, "", "/@composed:fixed")
	add("attribute numeric fixed", "attributes", ` numeric="+02.0"`, "")
	// Preserve the independent review's exact original instance bytes. Task2
	// newly accepted the second source when the missing flattening changed the
	// selected literal domain; baseline and the repaired tree both reject it.
	tests = append(tests,
		validationSourceCase{name: "review exact equal", source: `<value>1</value>`, oracleComparable: true},
		validationSourceCase{name: "review exact domain collision", source: `<value>+01</value>`, issues: []string{"/value:enumeration"}, oracleComparable: true})
	return tests
}

func TestValidateUnionMemberFlattening(t *testing.T) {
	t.Parallel()
	for _, test := range validationUnionMemberCases() {
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationUnionMemberGenerated, test.source), test.issues)
		})
	}
}

func TestUnionMemberFlatteningAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	directory, err := filepath.Abs("testdata/validation")
	require.NoError(t, err)
	for _, test := range validationUnionMemberCases() {
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationUnionMemberGenerated, test.source), test.issues)
			assertXMLLintOutcome(t, xmllint, filepath.Join(directory, "union-member-contract.xsd"), filepath.Join(directory, "catalog.xml"), test.source, len(test.issues) == 0)
		})
	}
}

func TestUnionMemberFlatteningImmutable(t *testing.T) {
	t.Parallel()
	before := snapshotValidationSchema(t, &validationUnionMemberGenerated)
	for worker := 0; worker < 4; worker++ {
		t.Run(string(rune('A'+worker)), func(t *testing.T) {
			t.Parallel()
			for range 3 {
				for _, test := range validationUnionMemberCases() {
					node, err := parseValidationDocument([]byte(test.source))
					require.NoError(t, err)
					saved := snapshotValidationNode(t, node)
					context := newTestValidationContext(&validationUnionMemberGenerated)
					context.validateElement(node, validationUnionMemberGenerated.Elements[validationQName{Local: node.Name.Local}], "/"+node.Name.Local)
					assertValidationIssues(t, context, test.issues)
					assert.Equal(t, saved, snapshotValidationNode(t, node))
				}
			}
			assert.Equal(t, before, snapshotValidationSchema(t, &validationUnionMemberGenerated))
		})
	}
}

func TestUnionMemberFlatteningReachedLiteralCache(t *testing.T) {
	t.Parallel()
	context := newTestValidationContext(&validationUnionMemberGenerated)
	schema := validationUnionMemberGenerated.Types[validationQName{Local: "composed-enum"}].Simple
	skipped := validationUnionMemberGenerated.Types[validationQName{Local: "zero"}].Simple
	for range 20 {
		assert.Nil(t, context.validateSimple(schema, "1"))
		failure := context.validateSimple(schema, "+01")
		require.NotNil(t, failure)
		assert.Equal(t, "enumeration", failure.constraint)
		assert.Len(t, context.enumerationValues, 1)
		_, found := context.enumerationValues[validationEnumerationKey{schema: skipped, index: 0}]
		assert.False(t, found, "flattened union-wrapper facets are not reached")
	}
	// The same restricted union remains operative when directly assessed.
	assert.Nil(t, context.validateSimple(skipped, "0"))
	assert.Len(t, context.enumerationValues, 2)
}
