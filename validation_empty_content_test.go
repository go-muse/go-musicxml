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

// XSD 1.0 cvc-complex-type 2.1 forbids every character information item in
// empty content, including XML S. This suite covers types already represented
// by an absent effective particle, not normalization of explicit empty groups.
// https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/#cvc-complex-type
func validationEmptyContentCases() []validationSourceCase {
	var tests []validationSourceCase
	add := func(name, element, attributes, content string, comparable bool, issues ...string) {
		tests = append(tests, validationSourceCase{
			name: name, source: wrapValidationElement(element, attributes, content),
			issues: issues, oracleComparable: comparable,
		})
	}
	for _, element := range []string{"empty", "empty-extended", "empty-restricted", "emptied-restriction"} {
		for _, content := range []struct {
			name, source string
			valid        bool
			comparable   bool
		}{
			{"empty", "", true, true},
			{"comment", "<!--comment with space -->", true, true},
			{"PI", "<?note some text?>", true, true},
			{"comment and PI", "<!--before--><?note some text?><!--after-->", true, true},
			// libxml2 2.9.14 treats the empty CDATA node as content even
			// though it contributes no character information items.
			{"empty CDATA", "<![CDATA[]]>", true, false},
			{"empty CDATA and comment", "<![CDATA[]]><!--comment--><?note text?>", true, false},
			{"space", " ", false, true},
			{"tab", "\t", false, true},
			{"LF", "\n", false, true},
			{"CR", "\r", false, true},
			{"CRLF", "\r\n", false, true},
			{"XML S", " \t\r\n", false, true},
			{"space reference", "&#32;", false, true},
			{"tab reference", "&#x9;", false, true},
			{"LF reference", "&#10;", false, true},
			{"CR reference", "&#xD;", false, true},
			{"CDATA space", "<![CDATA[ ]]>", false, true},
			{"CDATA XML S", "<![CDATA[\t\r\n ]]>", false, true},
			{"mixed token origins", "<!--before--> <![CDATA[]]><?note text?>&#x9;", false, true},
			{"ordinary text", "text", false, true},
			{"NBSP", "\u00a0", false, true},
			{"NEL reference", "&#x85;", false, true},
			{"Unicode CDATA", "<![CDATA[\u2003]]>", false, true},
		} {
			for _, nilAttribute := range []string{"", ` n:nil="false"`, ` n:nil="0"`} {
				var issues []string
				if !content.valid {
					issues = []string{"/" + element + ":element-only"}
				}
				add(element+"/"+content.name+nilAttribute, element, ` required="1"`+nilAttribute, content.source, content.comparable, issues...)
			}
		}
		add(element+"/self closing", element, ` required="1"`, "", true)
		tests[len(tests)-1].source = strings.Replace(tests[len(tests)-1].source, "></"+element+">", "/>", 1)
		for _, value := range []string{"true", "1"} {
			add(element+"/true nil "+value, element, ` required="1" n:nil="`+value+`"`, "", true)
			add(element+"/true nil with space "+value, element, ` required="1" n:nil="`+value+`"`, " ", true, "/"+element+":nillable")
		}
		add(element+"/malformed nil and space", element, ` required="1" n:nil="maybe"`, " ", true,
			"/"+element+":element-only", "/"+element+"/@{"+validationXSINamespace+"}nil:datatype")
		add(element+"/child", element, ` required="1"`, "<child/>", true, "/"+element+"/child:content-model")
		add(element+"/space and child", element, ` required="1"`, " <child/> ", true,
			"/"+element+":element-only", "/"+element+"/child:content-model")
	}

	for _, test := range []struct {
		name, element, attributes, content string
		issues                             []string
	}{
		{"missing required", "empty", "", " ", []string{"/empty/@required:required", "/empty:element-only"}},
		{"attribute datatype", "empty", ` required="bad"`, " ", []string{"/empty/@required:datatype", "/empty:element-only"}},
		{"attribute fixed", "empty", ` required="1" locked="other"`, " ", []string{"/empty/@locked:fixed", "/empty:element-only"}},
		{"unknown attribute", "empty", ` required="1" unknown="x"`, " ", []string{"/empty/@unknown:attribute", "/empty:element-only"}},
		{"inherited attribute", "empty-extended", ` required="1" extra="kept"`, " ", []string{"/empty-extended:element-only"}},
		{"prohibited attribute", "empty-restricted", ` required="1" extra="bad"`, " ", []string{"/empty-restricted/@extra:prohibited", "/empty-restricted:element-only"}},
		{"resolved identity", "empty", ` required="1" id="own" ref="own"`, " ", []string{"/empty:element-only"}},
		{"unresolved identity", "empty", ` required="1" id="own" ref="missing"`, " ", []string{"/empty:element-only", "/empty/@ref:IDREF"}},
		{"true nil still checks attributes", "empty", ` n:nil="true"`, "", []string{"/empty/@required:required"}},
		{"nullable particle permits XML S", "optional-nonmixed", ` required="1"`, " \t\r\n&#xD;", nil},
		{"nullable particle with child", "optional-nonmixed", ` required="1"`, " <child>7</child> ", nil},
		{"nullable particle rejects Unicode", "optional-nonmixed", ` required="1"`, "&#xA0;", []string{"/optional-nonmixed:element-only"}},
		{"added particle permits XML S", "empty-with-child", ` required="1"`, " <child>7</child> ", nil},
		{"added particle requires child", "empty-with-child", ` required="1"`, " ", []string{"/empty-with-child:content-model"}},
		{"added particle assesses child", "empty-with-child", ` required="1"`, " <child>bad</child> ", []string{"/empty-with-child/child:datatype"}},
		{"mixed particleless content", "mixed-empty", ` required="1"`, " text \u00a0&#x85;<![CDATA[\u2003]]>", nil},
		{"inherited mixed particleless content", "mixed-empty-extended", ` required="1" extra="kept"`, " text \u00a0&#x85;<![CDATA[\u2003]]>", nil},
		{"mixed still forbids children", "mixed-empty", ` required="1"`, "text<child/>", []string{"/mixed-empty/child:content-model"}},
		{"simple content", "simple-content", ` required="1"`, "7", nil},
		{"anyType content", "any", "", " text <child/> ", nil},
		{"local inline empty", "value-items", "", `<target id="own"> </target>`, []string{"/value-items/target:element-only"}},
		{"repeated inline empties", "value-items", "", `<target id="first"> </target><target id="second">&#9;</target>`, []string{"/value-items/target:element-only", "/value-items/target[2]:element-only"}},
	} {
		add(test.name, test.element, test.attributes, test.content, true, test.issues...)
	}
	return tests
}

func validationEmptyProductionCases() []validationSourceCase {
	const prefix = `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Music</part-name></score-part></part-list><part id="P1"><measure number="1"><note>`
	const pitch = `<pitch><step>C</step><octave>4</octave></pitch><duration>1</duration>`
	const suffix = `</note></measure></part></score-partwise>`
	var tests []validationSourceCase
	for _, input := range []struct{ name, before, after, path, schema string }{
		{"chord", prefix + `<chord>`, `</chord>` + pitch + suffix, "/score-partwise/part/measure/note/chord", "musicxml.xsd"},
		{"timewise chord", `<score-timewise version="4.0"><part-list><score-part id="P1"><part-name>Music</part-name></score-part></part-list><measure number="1"><part id="P1"><note><chord>`, `</chord>` + pitch + `</note></part></measure></score-timewise>`, "/score-timewise/measure/part/note/chord", "musicxml.xsd"},
		{"staccato", prefix + pitch + `<notations><articulations><staccato placement="above">`, `</staccato></articulations></notations>` + suffix, "/score-partwise/part/measure/note/notations/articulations/staccato", "musicxml.xsd"},
		{"opus score", `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="score.musicxml">`, `</score></opus>`, "/opus/score", "opus.xsd"},
		{"opus link", `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="opus.xml">`, `</opus-link></opus>`, "/opus/opus-link", "opus.xsd"},
	} {
		for _, content := range []struct{ name, source string }{
			{"empty", ""}, {"comment", "<!--comment-->"}, {"PI", "<?note text?>"},
			{"space", " "}, {"tab", "\t"}, {"CR", "\r"}, {"LF", "\n"},
			{"reference", "&#xD;"}, {"CDATA", "<![CDATA[ ]]>"},
		} {
			var issues []string
			if content.name != "empty" && content.name != "comment" && content.name != "PI" {
				issues = []string{input.path + ":element-only"}
			}
			tests = append(tests, validationSourceCase{name: input.name + "/" + content.name, source: input.before + content.source + input.after, schema: input.schema, issues: issues, oracleComparable: true})
		}
	}
	return tests
}

func TestValidateEmptyComplexContent(t *testing.T) {
	t.Parallel()
	for _, test := range append(validationEmptyContentCases(), validationEmptyProductionCases()...) {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			context := validateAttributeSource(t, validationSchemaFor(test.schema), test.source)
			assertValidationIssues(t, context, test.issues)
			for _, issue := range context.issues {
				if issue.Constraint == "element-only" {
					assert.Equal(t, "character data is not allowed", issue.Message)
				}
			}
		})
	}
}

func TestEmptyComplexContentAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	for _, test := range append(validationEmptyContentCases(), validationEmptyProductionCases()...) {
		if !test.oracleComparable {
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, validationSchemaFor(test.schema), test.source), test.issues)
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

func TestEmptyComplexContentPreservesAssessment(t *testing.T) {
	t.Parallel()
	for _, reference := range []string{"own", "missing"} {
		t.Run(reference, func(t *testing.T) {
			source := []byte(`<empty-extended required="1" id="own" ref="` + reference + `"> &#x9;<![CDATA[
]]></empty-extended>`)
			original := bytes.Clone(source)
			root, err := parseValidationDocument(source)
			require.NoError(t, err)
			text := root.Text.String()
			attributes := append([]validationAttribute(nil), root.Attrs...)
			for range 2 {
				context := &validationContext{schema: &validationNilGenerated, effective: make(map[*validationComplexSchema]*validationEffectiveComplex), identifiers: make(map[string]string)}
				context.validateElement(root, validationNilGenerated.Elements[validationQName{Local: root.Name.Local}], "/empty-extended")
				context.validateIdentityReferences()
				want := []string{"/empty-extended:element-only"}
				if reference == "missing" {
					want = append(want, "/empty-extended/@ref:IDREF")
				}
				assertValidationIssues(t, context, want)
				assert.Equal(t, map[string]string{"own": "/empty-extended/@id"}, context.identifiers)
				assert.Equal(t, []validationIdentityReference{{value: reference, path: "/empty-extended/@ref"}}, context.references)
				assert.Equal(t, original, source)
				assert.Equal(t, text, root.Text.String())
				assert.Equal(t, attributes, root.Attrs)
				assert.Empty(t, root.Children)
			}
		})
	}
}

func TestEmptyComplexContentPreservesPublicModels(t *testing.T) {
	t.Parallel()
	for _, test := range validationEmptyProductionCases() {
		t.Run(test.name, func(t *testing.T) {
			document, err := Decode(strings.NewReader(test.source))
			require.NoError(t, err, "Decode remains permissive for XSD-invalid source text")
			var before, after bytes.Buffer
			require.NoError(t, Encode(&before, document))
			assert.NoError(t, Validate(document), "Validate cannot recover source text discarded by Decode")
			require.NoError(t, Encode(&after, document))
			assert.Equal(t, before.String(), after.String())
			assertValidationIssues(t, validateAttributeSource(t, validationSchemaFor(test.schema), after.String()), nil)
		})
	}
}

func TestEmptyComplexContentCategoryControls(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		typeName    string
		hasParticle bool
		mixed       bool
	}{
		{"emptyBase", false, false}, {"emptyExtended", false, false},
		{"emptyRestricted", false, false}, {"emptiedRestriction", false, false},
		{"optionalNonmixed", true, false}, {"emptyWithChild", true, false},
		{"mixedEmpty", false, true}, {"mixedEmptyExtended", false, true},
	} {
		t.Run(test.typeName, func(t *testing.T) {
			context := &validationContext{schema: &validationNilGenerated, effective: make(map[*validationComplexSchema]*validationEffectiveComplex)}
			definition := validationNilGenerated.Types[validationQName{Local: test.typeName}]
			require.NotNil(t, definition)
			effective, ok := context.effectiveComplex(definition.Complex)
			require.True(t, ok)
			assert.Equal(t, test.hasParticle, effective.particle != nil)
			assert.Equal(t, test.mixed, effective.mixed)
			assert.Nil(t, effective.simple)
		})
	}
	// An absent document child is not proof that its declared particle is empty.
	for _, content := range []string{"", " ", "\t\r\n", " <child>7</child> "} {
		t.Run(fmt.Sprintf("nullable %q", content), func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationNilGenerated, `<optional-nonmixed required="1">`+content+`</optional-nonmixed>`), nil)
		})
	}
}
