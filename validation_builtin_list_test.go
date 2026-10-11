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

func validationBuiltinListCases() []validationSourceCase {
	var tests []validationSourceCase
	add := func(name, element, attributes, value, constraint string) {
		source := wrapValidationElement(element, attributes, value)
		if strings.HasPrefix(element, "entities") || strings.Contains(attributes, "entities=") {
			// libxml2 2.9.14 rejects declared ENTITY values in element simpleContent;
			// the equivalent ENTITIES attribute passes. Keep all such element cases internal: datatype rejection masks facets.
			source = `<!DOCTYPE ` + element + ` [<!NOTATION image SYSTEM "image"><!ENTITY aa SYSTEM "aa" NDATA image><!ENTITY bb SYSTEM "bb" NDATA image><!ENTITY cc SYSTEM "cc" NDATA image><!ENTITY ab SYSTEM "ab" NDATA image>]>` + source
		}
		var issues []string
		if constraint != "" {
			issues = []string{"/" + element + constraint}
		}
		incomplete := (strings.HasPrefix(element, "entities") || strings.Contains(attributes, "entities=")) && constraint != ":datatype"
		if incomplete {
			issues = nil
		}
		tests = append(tests, validationSourceCase{name: name, source: source, issues: issues, oracleComparable: !incomplete && !strings.HasPrefix(element, "entities"), assessmentIncomplete: incomplete})
	}
	for _, kind := range []string{"tokens", "refs", "entities"} {
		attrs := ""
		if kind == "refs" {
			attrs = ` a="aa"`
		}
		for _, variant := range []string{"exact", "range", "bridge"} {
			element := kind + "-" + variant
			localAttrs := attrs
			for _, value := range []string{"aa bb", " aa&#x9;bb&#xA;", "aa<!--split--> bb"} {
				if kind == "refs" {
					value = strings.ReplaceAll(value, "bb", "aa")
				}
				add(element+"/two/"+value, element, localAttrs, value, "")
			}
			constraint := ":length"
			if variant == "range" {
				constraint = ":minLength"
			}
			add(element+"/one", element, localAttrs, "ab", constraint)
			constraint = ":length"
			if variant == "range" {
				constraint = ""
			}
			value := "aa bb cc"
			if kind == "refs" {
				value = "aa aa aa"
			}
			add(element+"/three", element, localAttrs, value, constraint)
			constraint = ":length"
			if variant == "range" {
				constraint = ":maxLength"
			}
			add(element+"/four", element, localAttrs, "aa bb cc aa", constraint)
			add(element+"/empty", element, localAttrs, " \t\n", ":datatype")
			add(element+"/bad item", element, localAttrs, "aa ?", ":datatype")
		}
	}
	add("attribute count", "attributes", ` tokens="aa bb" refs="aa aa" entities="aa bb" a="aa"`, "", "")
	add("attribute length", "attributes", ` tokens="ab"`, "", "/@tokens:length")
	add("attribute minimum", "attributes", ` refs="ab"`, "", "/@refs:minLength")
	add("attribute maximum", "attributes", ` refs="aa bb cc aa"`, "", "/@refs:maxLength")
	add("attribute entities", "attributes", ` entities="ab"`, "", "/@entities:length")
	return tests
}

func TestValidateBuiltinListLengths(t *testing.T) {
	t.Parallel()
	for _, test := range validationBuiltinListCases() {
		t.Run(test.name, func(t *testing.T) {
			assertBuiltinListAssessment(t, assessAttributeSource(t, &validationBuiltinListGenerated, test.source), test)
		})
	}
}

func TestBuiltinListLengthsAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	directory, err := filepath.Abs("testdata/validation")
	require.NoError(t, err)
	for _, test := range validationBuiltinListCases() {
		if !test.oracleComparable {
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			assertBuiltinListAssessment(t, assessAttributeSource(t, &validationBuiltinListGenerated, test.source), test)
			assertXMLLintOutcome(t, xmllint, filepath.Join(directory, "builtin-list-contract.xsd"), filepath.Join(directory, "catalog.xml"), test.source, len(test.issues) == 0)
		})
	}
}

func TestBuiltinListLengthsImmutable(t *testing.T) {
	t.Parallel()
	before := snapshotValidationSchema(t, &validationBuiltinListGenerated)
	for worker := 0; worker < 4; worker++ {
		t.Run(string(rune('A'+worker)), func(t *testing.T) {
			t.Parallel()
			for range 3 {
				for _, test := range validationBuiltinListCases() {
					node, err := parseValidationDocument([]byte(test.source))
					require.NoError(t, err)
					saved := snapshotValidationNode(t, node)
					context := newTestValidationContext(&validationBuiltinListGenerated)
					context.ValidateElement(node, validationBuiltinListGenerated.Elements[validationQName{Local: node.Name.Local}], "/"+node.Name.Local)
					context.ValidateIdentityReferences()
					assertBuiltinListAssessment(t, context, test)
					after := snapshotValidationNode(t, node)
					assert.Equal(t, saved, after)
				}
			}
			assert.Equal(t, before, snapshotValidationSchema(t, &validationBuiltinListGenerated))
		})
	}
}

func TestBuiltinListLengthDiagnostics(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ element, value, kind, message string }{
		{"tokens-exact", " ab ", "length", "value length 1 does not equal 2"},
		{"tokens-range", " ab ", "minLength", "value length 1 is less than 2"},
		{"tokens-range", "aa bb cc aa", "maxLength", "value length 4 is greater than 3"},
	} {
		context := validateAttributeSource(t, &validationBuiltinListGenerated, wrapValidationElement(test.element, "", test.value))
		require.Len(t, context.Issues, 1)
		assert.Equal(t, validationKernelIssue{Path: "/" + test.element, Constraint: test.kind, Message: test.message}, context.Issues[0])
	}
}

// Datatype-only ENTITY/ENTITIES helpers remain reusable, but source assessment
// cannot assert membership without unparsed entity declaration facts.
func assertBuiltinListAssessment(t *testing.T, context *validationContext, test validationSourceCase) {
	t.Helper()
	if test.assessmentIncomplete {
		require.Error(t, context.AssessmentErr)
		assert.NotErrorIs(t, context.result(), ErrInvalidDocument)
		assert.Empty(t, context.Issues)
		return
	}
	assertValidationIssues(t, context, test.issues)
}
