package musicxml

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validationGrammarCases() []validationSourceCase {
	var tests []validationSourceCase
	add := func(name, element, content, issue string) {
		var issues []string
		if issue != "" {
			issues = []string{"/" + element + issue}
		}
		tests = append(tests, validationSourceCase{name: name, source: "<" + element + ">" + content + "</" + element + ">", issues: issues, oracleComparable: true})
	}
	for _, element := range []string{"nullable-sequence", "nullable-choice", "nullable-unbounded", "nullable-zero", "partition-fixed", "partition-continuation"} {
		for count := 0; count <= 5; count++ {
			issue := ""
			switch {
			case strings.HasPrefix(element, "partition-") && count < 2:
				issue = ":content-model"
			case strings.HasPrefix(element, "partition-") && count > 4:
				issue = "/a[5]:content-model"
			case element == "nullable-zero" && count > 2:
				issue = "/a[3]:content-model"
			case (element == "nullable-sequence" || element == "nullable-choice") && count > 3:
				issue = "/a[4]:content-model"
			}
			add(fmt.Sprintf("%s/%d", element, count), element, strings.Repeat("<a>1</a>", count), issue)
		}
	}
	add("continuation needs shorter early occurrence", "partition-continuation", "<a>1</a><a>2</a><b/>", "")
	add("partition still assesses scalar", "partition-fixed", "<a>1</a><a>bad</a>", "/a[2]:datatype")
	add("empty first branch does not hide consuming branch", "nullable-choice", "<b/><a/><b/>", "")
	add("nullable branch last", "nullable-zero", "<b/><a/>", "")
	add("branch bindings", "branches", "<a>1</a><b/><c>true</c><d/><a>2</a>", "")
	add("branch scalar binding", "branches", "<c>not-bool</c><a>1</a>", "/c:datatype")
	add("branch order violation", "branches", "<b/><a>1</a>", "/b:content-model")
	add("comments and processing instructions", "partition-fixed", "<!--before--><a>1</a><?note later?><a>2</a>", "")
	add("all reverse", "all", "<b/><a>1</a>", "")
	add("all required", "all", "<b/>", ":content-model")
	add("all duplicate", "all", "<a>1</a><a>2</a>", "/a[2]:content-model")
	add("all inactive duplicate names", "all-inactive-name", "<a>1</a>", "")
	add("all inactive duplicate names reverse", "all-inactive-name", "<b>2</b><a>1</a>", "")
	add("all inactive names preserve scalar binding", "all-inactive-name", "<b>bad</b><a>1</a>", "/b:datatype")
	add("all inactive names preserve required", "all-inactive-name", "<b>2</b>", ":content-model")
	add("all inactive names preserve uniqueness", "all-inactive-name", "<a>1</a><a>2</a>", "/a[2]:content-model")
	return tests
}

func TestValidateGrammarParticles(t *testing.T) {
	t.Parallel()
	for _, test := range validationGrammarCases() {
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationGrammarGenerated, test.source), test.issues)
		})
	}
}

func TestGrammarParticlesAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	directory, err := filepath.Abs("testdata/validation")
	require.NoError(t, err)
	for _, test := range validationGrammarCases() {
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationGrammarGenerated, test.source), test.issues)
			assertXMLLintOutcome(t, xmllint, filepath.Join(directory, "grammar-contract.xsd"), filepath.Join(directory, "catalog.xml"), test.source, len(test.issues) == 0)
		})
	}
}

func TestGrammarParticlesPreserveInputs(t *testing.T) {
	t.Parallel()
	before := snapshotValidationSchema(t, &validationGrammarGenerated)
	root, err := parseValidationDocument([]byte(`<partition-fixed><a>1</a><a>2</a></partition-fixed>`))
	require.NoError(t, err)
	for range 3 {
		context := newTestValidationContext(&validationGrammarGenerated)
		context.validateElement(root, validationGrammarGenerated.Elements[validationQName{Local: "partition-fixed"}], "/partition-fixed")
		assert.Empty(t, context.issues)
		assert.Equal(t, "1", root.Children[0].Text.String())
		assert.Equal(t, "2", root.Children[1].Text.String())
	}
	assert.Equal(t, before, snapshotValidationSchema(t, &validationGrammarGenerated))
}

func TestGrammarParticlesConcurrentInputs(t *testing.T) {
	before := snapshotValidationSchema(t, &validationGrammarGenerated)
	root, err := parseValidationDocument([]byte(`<partition-continuation><a>1</a><a>2</a><b/></partition-continuation>`))
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.Equal(t, before, snapshotValidationSchema(t, &validationGrammarGenerated))
		assert.Equal(t, "1", root.Children[0].Text.String())
		assert.Equal(t, "2", root.Children[1].Text.String())
	})
	for index := range 8 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			context := newTestValidationContext(&validationGrammarGenerated)
			context.validateElement(root, validationGrammarGenerated.Elements[validationQName{Local: "partition-continuation"}], "/partition-continuation")
			assert.Empty(t, context.issues)
		})
	}
}
