package musicxml

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validationContentWhitespaceDocuments() []struct{ name, start, first, second, end, schema string } {
	const partList = `<part-list><score-part id="P1"><part-name>Music</part-name></score-part></part-list>`
	return []struct{ name, start, first, second, end, schema string }{
		{"partwise", `<score-partwise version="4.0">`, partList, `<part id="P1"><measure number="1"/></part>`, `</score-partwise>`, "musicxml.xsd"},
		{"timewise", `<score-timewise version="4.0">`, partList, `<measure number="1"><part id="P1"/></measure>`, `</score-timewise>`, "musicxml.xsd"},
		{"opus", `<opus xmlns:xlink="http://www.w3.org/1999/xlink">`, `<title>Music</title>`, `<score xlink:href="score.musicxml"/>`, `</opus>`, "opus.xsd"},
	}
}

// XSD 1.0 cvc-complex-type 2.3 permits only XML S in element-only content:
// U+0020, U+0009, U+000A, and U+000D, not Unicode's broader whitespace set.
// https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-complex-type
// Test original source: Decode cannot retain this text in the public models.
// libxml2 2.9.14 rejects whitespace-only and empty CDATA in element-only
// content. Keep those normative cases outside the compatible oracle set.
func validationContentWhitespaceCases() []validationSourceCase {
	var tests []validationSourceCase
	addComplex := func(name, attributes, content string, issues ...string) {
		tests = append(tests, validationSourceCase{
			name: name, source: `<complex xmlns:n="` + validationXSINamespace + `"` + attributes + `>` + content + `</complex>`, issues: issues, oracleComparable: true,
		})
	}
	// Exercise every boundary character in each XML representation once, using
	// the inherited element-only type rather than multiplying by every root.
	for _, character := range []rune{
		' ', '\t', '\n', '\r',
		'\u00a0', '\u0085', '\u1680',
		'\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005',
		'\u2006', '\u2007', '\u2008', '\u2009', '\u200a',
		'\u2028', '\u2029', '\u202f', '\u205f', '\u3000',
	} {
		var issues []string
		switch character {
		case ' ', '\t', '\n', '\r':
		default:
			issues = []string{"/complex:element-only"}
		}
		for _, representation := range []struct{ name, text string }{
			{"literal", string(character)},
			{"reference", fmt.Sprintf("&#x%X;", character)},
			{"CDATA", "<![CDATA[" + string(character) + "]]>"},
		} {
			addComplex(fmt.Sprintf("U+%04X %s", character, representation.name), ` required="1" extra="inherited"`,
				" \t"+representation.text+"<child>7</child>\r\n", issues...)
			tests[len(tests)-1].oracleComparable = len(issues) != 0 || representation.name != "CDATA"
		}
	}
	for _, content := range []struct{ name, text string }{
		{"no text", ""},
		{"all XML whitespace", " \t\r\n&#x20;&#x9;&#xA;&#xD;"},
		{"empty CDATA", "<![CDATA[]]>"},
		{"comments and PI", "<!--\u00a0text--><?note \u0085text?>"},
	} {
		addComplex(content.name, ` required="1"`, content.text+"<child>7</child>"+content.text)
		tests[len(tests)-1].oracleComparable = content.name != "empty CDATA"
	}
	for _, content := range []struct{ name, text string }{
		{"ordinary text", "text"}, {"digit", "7"}, {"non-ASCII text", "é"},
		{"zero width space", "\u200b"}, {"embedded BOM", "\ufeff"},
		{"split character data", "\u00a0<!--ignored--><?note ignored?><![CDATA[\u0085]]>&#x1680;"},
	} {
		addComplex(content.name, ` required="1"`, content.text+"<child>7</child>", "/complex:element-only")
	}
	for _, document := range validationContentWhitespaceDocuments() {
		for _, position := range []string{"before", "between", "after"} {
			for _, content := range []struct{ name, text string }{
				{"XML whitespace", " \t\r\n&#xD;<!--\u00a0--><?note \u0085?>"},
				{"Unicode whitespace", " \t<!--ignored-->&#xA0;<![CDATA[\u0085]]>\r\n"},
			} {
				before, between, after := "", "", ""
				switch position {
				case "before":
					before = content.text
				case "between":
					between = content.text
				case "after":
					after = content.text
				}
				test := validationSourceCase{
					name: document.name + " " + position + " " + content.name, schema: document.schema, oracleComparable: true,
					source: document.start + before + document.first + between + document.second + after + document.end,
				}
				if content.name == "Unicode whitespace" {
					root := strings.TrimPrefix(document.end, "</")
					test.issues = []string{"/" + strings.TrimSuffix(root, ">") + ":element-only"}
				}
				tests = append(tests, test)
			}
		}
	}
	for _, test := range []struct {
		name, attributes, content string
		issues                    []string
	}{
		{"missing required attribute", "", "&#xA0;<child>7</child>", []string{"/complex:element-only", "/complex/@required:required"}},
		{"unknown attribute", ` required="1" rubbish="x"`, "&#xA0;<child>7</child>", []string{"/complex:element-only", "/complex/@rubbish:attribute"}},
		{"attribute datatype", ` required="bad"`, "&#xA0;<child>7</child>", []string{"/complex:element-only", "/complex/@required:datatype"}},
		{"attribute fixed", ` required="1" locked="other"`, "&#xA0;<child>7</child>", []string{"/complex:element-only", "/complex/@locked:fixed"}},
		{"attribute prohibited", ` required="1" blocked="x"`, "&#xA0;<child>7</child>", []string{"/complex:element-only", "/complex/@blocked:prohibited"}},
		{"child datatype", ` required="1"`, "&#xA0;<child>bad</child>", []string{"/complex:element-only", "/complex/child:datatype"}},
		{"missing child", ` required="1"`, "&#xA0;", []string{"/complex:element-only", "/complex:content-model"}},
		{"unexpected child", ` required="1"`, "&#xA0;<unknown/>", []string{"/complex:element-only", "/complex/unknown:content-model"}},
		{"nil true empty", ` required="1" n:nil="true"`, "", nil},
		{"nil one comments and PI", ` required="1" n:nil="1"`, "<!--\u00a0--><?note \u0085?>", nil},
		{"nil true XML whitespace", ` required="1" n:nil="true"`, " &#x9;&#xD;&#xA;", []string{"/complex:nillable"}},
		{"nil true Unicode whitespace", ` required="1" n:nil="true"`, "&#xA0;", []string{"/complex:nillable"}},
		{"nil true text and child", ` required="1" n:nil="true"`, "&#xA0;<child>bad</child>", []string{"/complex:nillable"}},
		{"nil true attribute assessment", ` n:nil="true"`, "&#xA0;", []string{"/complex:nillable", "/complex/@required:required"}},
		{"nil false Unicode whitespace", ` required="1" n:nil="false"`, "&#xA0;<child>7</child>", []string{"/complex:element-only"}},
		{"nil zero Unicode whitespace", ` required="1" n:nil="0"`, "&#xA0;<child>7</child>", []string{"/complex:element-only"}},
		{"malformed nil still assesses text", ` required="1" n:nil="maybe"`, "&#xA0;<child>7</child>", []string{"/complex:element-only", "/complex/@{" + validationXSINamespace + "}nil:datatype"}},
	} {
		addComplex(test.name, test.attributes, test.content, test.issues...)
	}
	return append(tests, []validationSourceCase{
		{name: "mixed Unicode text", source: `<mixed required="1">text &#x85;<![CDATA[ ]]><child>7</child> 　</mixed>`, oracleComparable: true},
		{name: "mixed child datatype", source: `<mixed required="1">&#xA0;<child>bad</child></mixed>`, issues: []string{"/mixed/child:datatype"}, oracleComparable: true},
		{name: "string Unicode text", source: `<plain> &#x85;<![CDATA[ ]]> 　</plain>`, oracleComparable: true},
		{name: "simple content value", source: `<simple-content required="1">7</simple-content>`, oracleComparable: true},
		{name: "simple content Unicode whitespace", source: `<simple-content required="1">&#xA0;7</simple-content>`, issues: []string{"/simple-content:datatype"}, oracleComparable: true},
		{name: "simple content children", source: `<simple-content required="1">&#xA0;<child>7</child></simple-content>`, issues: []string{"/simple-content:simple-content"}, oracleComparable: true},
		{name: "empty complex rejects Unicode text", source: `<value-items><target id="target">&#xA0;</target></value-items>`, issues: []string{"/value-items/target:element-only"}, oracleComparable: true},
		{name: "production simple content Unicode text", schema: "musicxml.xsd", source: strings.Replace(validationAttributeScore, "Music</part-name>", "\u00a0&#x85;<![CDATA[\u1680]]>Music\u2028\u3000</part-name>", 1), oracleComparable: true},
		{name: "nested partwise Unicode text", schema: "musicxml.xsd", source: strings.Replace(validationAttributeScore, `<measure number="1" implicit="yes">`, `<measure number="1" implicit="yes">&#xA0;`, 1), issues: []string{"/score-partwise/part/measure:element-only"}, oracleComparable: true},
	}...)
}

func assertContentWhitespaceCase(t *testing.T, test validationSourceCase) {
	t.Helper()
	context := validateAttributeSource(t, validationSchemaFor(test.schema), test.source)
	assertValidationIssues(t, context, test.issues)
	for _, issue := range context.issues {
		if issue.Constraint == "element-only" {
			assert.Equal(t, "character data is not allowed", issue.Message)
		}
	}
}

func TestValidateElementOnlyXMLWhitespace(t *testing.T) {
	t.Parallel()
	for _, test := range validationContentWhitespaceCases() {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertContentWhitespaceCase(t, test)
		})
	}
}

// Runtime and xmllint receive identical original source bytes, never a decoded
// model. The fixtures and pinned production schemas are the generated inputs.
func TestElementOnlyXMLWhitespaceAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	for _, test := range validationContentWhitespaceCases() {
		if !test.oracleComparable {
			continue // Covered normatively without freezing an oracle bug.
		}
		t.Run(test.name, func(t *testing.T) {
			assertContentWhitespaceCase(t, test)
			directory, schema := filepath.Join("schema", "musicxml-4.0"), test.schema
			if schema == "" {
				directory, schema = filepath.Join("testdata", "validation"), "nil-contract.xsd"
			}
			directory, err := filepath.Abs(directory)
			require.NoError(t, err)
			assertXMLLintOutcome(t, xmllint, filepath.Join(directory, schema), filepath.Join(directory, "catalog.xml"), test.source, len(test.issues) == 0)
		})
	}
}

func TestElementOnlyXMLWhitespacePreservesAssessment(t *testing.T) {
	t.Parallel()
	for _, reference := range []string{"own", "missing"} {
		t.Run(reference, func(t *testing.T) {
			source := []byte(`<complex required="1" id="own" ref="` + reference + `"> &#xA0;<![CDATA[]]><child>7</child></complex>`)
			originalSource := bytes.Clone(source)
			root, err := parseValidationDocument(source)
			require.NoError(t, err)
			text := root.Text.String()
			attributes := append([]validationAttribute(nil), root.Attrs...)
			require.Len(t, root.Children, 1)
			child := root.Children[0]
			for range 2 {
				context := &validationContext{schema: &validationNilGenerated, effective: make(map[*validationComplexSchema]*validationEffectiveComplex), identifiers: make(map[string]string)}
				context.validateElement(root, validationNilGenerated.Elements[validationQName{Local: "complex"}], "/complex")
				context.validateIdentityReferences()
				want := []string{"/complex:element-only"}
				if reference == "missing" {
					want = append(want, "/complex/@ref:IDREF")
				}
				assertValidationIssues(t, context, want)
				assert.Equal(t, map[string]string{"own": "/complex/@id"}, context.identifiers)
				assert.Equal(t, []validationIdentityReference{{value: reference, path: "/complex/@ref"}}, context.references)
				assert.Equal(t, originalSource, source)
				assert.Equal(t, text, root.Text.String())
				assert.Equal(t, attributes, root.Attrs)
				require.Len(t, root.Children, 1)
				assert.Same(t, child, root.Children[0])
				assert.Equal(t, "7", child.Text.String())
			}
		})
	}
	// A root text error must not prevent IDs or IDREFs in matched children.
	document := validationContentWhitespaceDocuments()[0]
	source := document.start + "&#xA0;" + document.first + document.second + document.end
	context := validateAttributeSource(t, &scoreValidationSchema, source)
	assertValidationIssues(t, context, []string{"/score-partwise:element-only"})
	assert.Equal(t, map[string]string{"P1": "/score-partwise/part-list/score-part/@id"}, context.identifiers)
	assert.Equal(t, []validationIdentityReference{{value: "P1", path: "/score-partwise/part/@id"}}, context.references)
}

func TestElementOnlyXMLWhitespacePreservesPublicModels(t *testing.T) {
	t.Parallel()
	for _, test := range validationContentWhitespaceDocuments() {
		t.Run(test.name, func(t *testing.T) {
			clean := test.start + test.first + test.second + test.end
			source := test.start + "\u00a0&#x85;<![CDATA[\u1680]]>" + test.first + test.second + test.end
			root, err := parseValidationDocument([]byte(source))
			require.NoError(t, err)
			assert.Equal(t, "\u00a0\u0085\u1680", root.Text.String())
			context := validateAttributeSource(t, validationSchemaFor(test.schema), source)
			assertValidationIssues(t, context, []string{"/" + root.Name.Local + ":element-only"})

			document, err := Decode(strings.NewReader(source))
			require.NoError(t, err)
			baseline, err := Decode(strings.NewReader(clean))
			require.NoError(t, err)
			assert.Equal(t, baseline, document, "Decode discards element-only source text")
			var before, after bytes.Buffer
			require.NoError(t, Encode(&before, document))
			// Public Validate assesses the current model. It cannot recover the
			// discarded source text and must neither reject nor mutate this model.
			assert.NoError(t, Validate(document))
			assert.Equal(t, baseline, document)
			require.NoError(t, Encode(&after, document))
			assert.Equal(t, before.String(), after.String())
			for _, character := range []rune{'\u00a0', '\u0085', '\u1680'} {
				assert.NotContains(t, after.String(), string(character))
			}
			assertValidationIssues(t, validateAttributeSource(t, validationSchemaFor(test.schema), after.String()), nil)
		})
	}
}
