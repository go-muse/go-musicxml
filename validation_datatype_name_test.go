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

// XSD 1.0 uses the XML 1.0 Second Edition name productions. XML document
// element/attribute names can use a later repertoire independently.
// https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#Name
// https://www.w3.org/TR/2000/REC-xml-20001006#CharClasses
func TestValidateDatatypeNameRepertoire(t *testing.T) {
	t.Parallel()
	for _, builtin := range []string{"Name", "NCName", "NMTOKEN", "ID", "IDREF", "ENTITY", "QName", "NOTATION", "NMTOKENS", "IDREFS", "ENTITIES"} {
		t.Run(builtin, func(t *testing.T) {
			t.Parallel()
			for _, value := range []string{"_A", "éclair", "Жук", "樂器", "ก", "あ", "ア", "一", "가", "a\u0301", "a\u00b7", "a-._9", " \tЖ\r\n "} {
				assert.Nil(t, validateBuiltin(builtin, value), "%s %q", builtin, value)
			}
			for _, value := range []string{"\u0220x", "a\u0220", "\u200cx", "a\u200c", "\U00010000x", "a\U00010000", "\ufffda", "a\ufffd", "a\u00a0", "a\u0085", "a\u2000", "a/", "a\x00", "a\xff", ""} {
				failure := validateBuiltin(builtin, value)
				if assert.NotNil(t, failure, "%s %q", builtin, value) {
					assert.Equal(t, "datatype", failure.Constraint)
				}
			}
		})
	}
}

func TestValidateDatatypeNameBoundaries(t *testing.T) {
	t.Parallel()
	// Independently selected production endpoints and holes, not generated from
	// the implementation's class strings. Combining marks and extenders can only
	// follow a Name start, but may begin an NMTOKEN.
	for _, test := range []struct {
		value               string
		start, continuation bool
	}{
		{"A", true, true}, {"Z", true, true}, {"a", true, true}, {"z", true, true},
		{"_", true, true}, {":", true, true}, {"-", false, true}, {".", false, true}, {"0", false, true},
		{"\u00c0", true, true}, {"\u00d6", true, true}, {"\u00d7", false, false}, {"\u00d8", true, true},
		{"\u0131", true, true}, {"\u0132", false, false}, {"\u0133", false, false}, {"\u0134", true, true},
		{"\u0217", true, true}, {"\u0218", false, false}, {"\u0220", false, false},
		{"\u0300", false, true}, {"\u0345", false, true}, {"\u0346", false, false},
		{"\u00b7", false, true}, {"\u203f", false, false}, {"\u3007", true, true},
		{"\u3021", true, true}, {"\u3029", true, true}, {"\u302a", false, true},
		{"\u4e00", true, true}, {"\u9fa5", true, true}, {"\u9fa6", false, false},
		{"\uac00", true, true}, {"\ud7a3", true, true}, {"\ud7a4", false, false},
		{"\u200c", false, false}, {"\U00010000", false, false},
	} {
		t.Run(fmt.Sprintf("U+%04X", []rune(test.value)[0]), func(t *testing.T) {
			assert.Equal(t, test.start, validateBuiltin("Name", test.value+"x") == nil, "initial Name character")
			assert.Equal(t, test.continuation, validateBuiltin("Name", "x"+test.value) == nil, "subsequent Name character")
			assert.Equal(t, test.continuation, validateBuiltin("NMTOKEN", test.value) == nil, "NMTOKEN character")
		})
	}
	for _, test := range []struct {
		builtin, value string
		valid          bool
	}{
		{"Name", "a:b:c", true}, {"NCName", "a:b", false}, {"QName", "a:b", true},
		{"QName", "a:b:c", false}, {"QName", ":a", false}, {"QName", "a:", false},
		{"QName", "Ж:樂", true}, {"QName", "\u0220:a", false}, {"QName", "a:\u0220", false},
		{"QName", "a:\u0301", false}, {"QName", "a:a\u0301", true},
		{"NMTOKEN", "\u0301", true}, {"NMTOKEN", "1:a", true}, {"NCName", "1a", false},
		{"Name", "a b", false}, {"NMTOKENS", "a b", true}, {"NMTOKENS", "a\t\u0220", false},
		{"IDREFS", "Ж 樂", true}, {"IDREFS", "Ж \u0220", false},
		{"ENTITIES", "Ж 樂", true}, {"ENTITIES", "Ж \u0220", false},
	} {
		assert.Equal(t, test.valid, validateBuiltin(test.builtin, test.value) == nil, "%s %q", test.builtin, test.value)
	}
}

func validationDatatypeNameCases() []validationSourceCase {
	var result []validationSourceCase
	add := func(name, root, attributes, content, suffix string, oracle bool) {
		var issues []string
		if suffix != "" {
			issues = []string{"/" + root + suffix}
		}
		incomplete := (strings.HasPrefix(name, "entity/") || strings.HasPrefix(name, "entities/") || strings.HasPrefix(name, "attribute/entity/") || strings.HasPrefix(name, "attribute/entities/")) && suffix == ""
		result = append(result, validationSourceCase{name: name, source: wrapValidationElement(root, attributes, content), issues: issues, oracleComparable: oracle && !incomplete, assessmentIncomplete: incomplete})
	}
	for _, root := range []string{"name", "ncname", "nmtoken", "qname", "restricted", "inline", "content"} {
		add(root+"/multilingual", root, "", "Ж樂a&#x301;", "", true)
		add(root+"/whitespace", root, "", " &#x9;Ж&#xA; ", "", true)
		for _, bad := range []string{"\u0220", "\u200c", "\U00010000"} {
			for _, value := range []string{bad + "a", "a" + bad} {
				add(fmt.Sprintf("%s/reject/%U/%t", root, []rune(bad)[0], strings.HasPrefix(value, "a")), root, "", value, ":datatype", true)
			}
		}
	}
	add("name/colon", "name", "", "a:b:c", "", true)
	add("ncname/colon", "ncname", "", "a:b", ":datatype", true)
	add("nmtoken/initial-combining", "nmtoken", "", "&#x301;1:a", "", true)
	add("name/initial-combining", "name", "", "&#x301;a", ":datatype", true)
	add("qname/prefix", "qname", ` xmlns:p="urn:example"`, "p:Ж", "", true)
	add("qname/invalid-local", "qname", ` xmlns:p="urn:example"`, "p:\u0220", ":datatype", true)
	add("qname/three-parts", "qname", ` xmlns:p="urn:example"`, "p:a:b", ":datatype", true)
	add("list/multilingual", "list", "", "Ж 樂", "", true)
	add("list/reject", "list", "", "Ж \u0220", ":datatype", true)
	add("tokens/multilingual", "tokens", "", "Ж &#x301;:1", "", true)
	add("tokens/reject", "tokens", "", "Ж \U00010000", ":datatype", true)
	add("union/name", "union", "", "Ж", "", true)
	add("union/number-fallback", "union", "", "123", "", true)
	add("union/reject", "union", "", "\u0220", ":union", true)
	add("content/attribute", "content", ` code="Ж"`, "樂", "", true)
	add("content/attribute-reject", "content", ` code="Ƞ"`, "樂", "/@code:datatype", true)
	for _, attr := range []string{"name", "ncname", "nmtoken", "qname", "tokens", "id"} {
		add("attribute/"+attr+"/accept", "attributes", ` `+attr+`="Ж"`, "", "", true)
		add("attribute/"+attr+"/reject", "attributes", ` `+attr+`="Ƞ"`, "", "/@"+attr+":datatype", true)
	}
	add("identities/accept", "identities", "", "<id>Ж</id><ref>Ж</ref><refs>Ж Ж</refs>", "", true)
	add("identities/id-reject", "identities", "", "<id>\u0220</id>", "/id:datatype", true)
	add("identities/ref-reject", "identities", "", "<ref>\u0220</ref>", "/ref:datatype", true)
	add("identities/refs-reject", "identities", "", "<refs>\u0220</refs>", "/refs:datatype", true)
	add("attribute/refs-accept", "attributes", ` id="Ж" ref="Ж" refs="Ж Ж"`, "", "", true)
	add("attribute/ref-reject", "attributes", ` ref="Ƞ"`, "", "/@ref:datatype", true)
	add("attribute/refs-reject", "attributes", ` refs="Ƞ"`, "", "/@refs:datatype", true)
	// Retain unparsed declarations in the original source, but the current reader
	// does not retain their facts. Successful ENTITY lexemes are incomplete, never
	// complete-valid; lexical negatives remain provable datatype violations.
	for _, root := range []string{"entity", "entities"} {
		add(root+"/lexical-accept", root, "", "Ж", "", false)
		add(root+"/lexical-reject", root, "", "\u0220", ":datatype", false)
	}
	for _, attr := range []string{"entity", "entities"} {
		add("attribute/"+attr+"/accept", "attributes", ` `+attr+`="Ж"`, "", "", true)
		add("attribute/"+attr+"/reject", "attributes", ` `+attr+`="Ƞ"`, "", "/@"+attr+":datatype", true)
	}
	for i := range result {
		if strings.Contains(result[i].name, "entit") {
			root := "attributes"
			if strings.HasPrefix(result[i].name, "entity/") {
				root = "entity"
			} else if strings.HasPrefix(result[i].name, "entities/") {
				root = "entities"
			}
			result[i].source = `<!DOCTYPE ` + root + ` [<!NOTATION image SYSTEM "urn:image"><!ENTITY Ж SYSTEM "urn:entity" NDATA image>]>` + result[i].source
		}
	}
	return result
}

func assertDatatypeNameAssessment(t *testing.T, context *validationContext, test validationSourceCase) {
	t.Helper()
	if test.assessmentIncomplete {
		require.Error(t, context.AssessmentErr)
		assert.Contains(t, context.AssessmentErr.Error(), "unparsed entity")
		assert.Empty(t, context.Issues)
		assert.NotErrorIs(t, context.result(), ErrInvalidDocument)
		return
	}
	assertValidationIssues(t, context, test.issues)
}

func TestValidateDatatypeNameSources(t *testing.T) {
	t.Parallel()
	for _, test := range validationDatatypeNameCases() {
		t.Run(test.name, func(t *testing.T) {
			assertDatatypeNameAssessment(t, assessAttributeSource(t, &validationDatatypeNameGenerated, test.source), test)
		})
	}
}

func TestDatatypeNamesAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	directory, err := filepath.Abs("testdata/validation")
	require.NoError(t, err)
	for _, test := range validationDatatypeNameCases() {
		if !test.oracleComparable {
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertDatatypeNameAssessment(t, assessAttributeSource(t, &validationDatatypeNameGenerated, test.source), test)
			assertXMLLintOutcome(t, xmllint, filepath.Join(directory, "datatype-name-contract.xsd"), filepath.Join(directory, "catalog.xml"), test.source, len(test.issues) == 0)
		})
	}
}

func TestDatatypeNamesImmutable(t *testing.T) {
	t.Parallel()
	before := snapshotValidationSchema(t, &validationDatatypeNameGenerated)
	for worker := range 4 {
		t.Run(fmt.Sprint(worker), func(t *testing.T) {
			t.Parallel()
			for range 2 {
				for _, test := range validationDatatypeNameCases() {
					node, err := parseValidationDocument([]byte(test.source))
					require.NoError(t, err)
					saved := snapshotValidationNode(t, node)
					context := newTestValidationContext(&validationDatatypeNameGenerated)
					context.ValidateElement(node, validationDatatypeNameGenerated.Elements[validationQName{Local: node.Name.Local}], "/"+node.Name.Local)
					context.ValidateIdentityReferences()
					assertDatatypeNameAssessment(t, context, test)
					assert.Equal(t, saved, snapshotValidationNode(t, node))
				}
			}
			assert.Equal(t, before, snapshotValidationSchema(t, &validationDatatypeNameGenerated))
		})
	}
}

func TestDatatypeNamesPreserveXMLReading(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"\u0220", "\u200c", "\U00010000"} {
		node, err := parseValidationDocument([]byte(`<Ж Ж="` + name + `">` + name + `</Ж>`))
		require.NoError(t, err)
		assert.Equal(t, "Ж", node.Name.Local)
		require.Len(t, node.Attrs, 1)
		assert.Equal(t, "Ж", node.Attrs[0].Name.Local)
		assert.Equal(t, name, node.Attrs[0].Value)
		assert.Equal(t, name, node.Text.String())
		// Permissive typed decoding still retains ordinary string values and skips
		// unknown elements; XSD datatype checking stays separate.
		source := `<score-partwise><work><work-title>` + name + `</work-title></work><unknown/><part-list><score-part id="P1"><part-name>Music</part-name></score-part></part-list></score-partwise>`
		document, err := Decode(strings.NewReader(source))
		require.NoError(t, err)
		score, ok := document.(*ScorePartwise)
		require.True(t, ok)
		require.NotNil(t, score.Work)
		require.NotNil(t, score.Work.WorkTitle)
		assert.Equal(t, name, *score.Work.WorkTitle)
	}
}

func validationDatatypeNameProductionCases() []validationSourceCase {
	var cases []validationSourceCase
	for _, value := range []string{"Ж", "\u0220", "\u200c", "\U00010000"} {
		valid := value == "Ж"
		var issues []string
		if !valid {
			issues = []string{"/score-partwise/part-list/score-part/@id:datatype", "/score-partwise/part/@id:datatype"}
		}
		source := `<score-partwise version="4.0"><part-list><score-part id="` + value + `"><part-name>Test</part-name></score-part></part-list><part id="` + value + `"><measure number="1"/></part></score-partwise>`
		cases = append(cases, validationSourceCase{name: fmt.Sprintf("ID/%U", []rune(value)[0]), source: source, issues: issues, oracleComparable: true})
	}
	return cases
}

func TestDatatypeNamesProduction(t *testing.T) {
	t.Parallel()
	for _, test := range validationDatatypeNameProductionCases() {
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &scoreValidationSchema, test.source), test.issues)
		})
	}
}

func TestDatatypeNamesProductionAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	directory, err := filepath.Abs("schema/musicxml-4.0")
	require.NoError(t, err)
	for _, test := range validationDatatypeNameProductionCases() {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertValidationIssues(t, validateAttributeSource(t, &scoreValidationSchema, test.source), test.issues)
			assertXMLLintOutcome(t, xmllint, filepath.Join(directory, "musicxml.xsd"), filepath.Join(directory, "catalog.xml"), test.source, len(test.issues) == 0)
		})
	}
}
