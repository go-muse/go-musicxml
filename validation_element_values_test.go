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

// The stored schema lexeme follows the existing scalar path, including its
// facets. QName/NOTATION schema context and typed equality remain separate.
func validationElementValueCases() []validationAttributeCase {
	var tests []validationAttributeCase
	add := func(name, element, attributes, content, constraint, suffix string) {
		tests = append(tests, validationAttributeCase{
			name: name, source: "<" + element + ` xmlns:n="` + validationXSINamespace + `"` + attributes + ">" + content + "</" + element + ">",
			constraint: constraint, path: "/" + element + suffix,
		})
	}
	for _, element := range []struct{ name, attributes, value string }{
		{"defaulted", "", "7"}, {"default-named", "", "ok"}, {"default-inline", "", "ok"},
		{"default-pattern", "", "07"},
		{"default-empty", "", ""}, {"fixed", "", "kept"}, {"empty-fixed", "", ""},
		{"default-content", ` required="1"`, "7"}, {"fixed-content", ` required="1"`, "7"},
		{"default-mixed", ` required="1"`, "kept"}, {"fixed-mixed", ` required="1"`, "kept"},
	} {
		for _, nilAttribute := range []string{"", ` n:nil="false"`, ` n:nil="0"`, ` n:nil=" &#x9;false&#xD;&#xA; "`} {
			for _, content := range []string{"", "<!--comment-->", "<?note allowed?>", "<!--before--><?note allowed?><!--after-->"} {
				add(element.name+" empty "+nilAttribute+" "+content, element.name, element.attributes+nilAttribute, content, "", "")
			}
		}
		add(element.name+" self closing", element.name, element.attributes, "", "", "")
		tests[len(tests)-1].source = strings.Replace(tests[len(tests)-1].source, "></"+element.name+">", "/>", 1)
		add(element.name+" explicit", element.name, element.attributes, element.value, "", "")
		constraint := ""
		if strings.HasPrefix(element.name, "fixed") || element.name == "empty-fixed" {
			constraint = "fixed"
		}
		add(element.name+" true nil", element.name, element.attributes+` n:nil="true"`, "", constraint, "")
	}
	for _, content := range []string{" ", "\t\n", "&#xD;", "&#xA0;", "<![CDATA[ ]]>", "<!--comment--> "} {
		add("integer whitespace "+content, "defaulted", "", content, "datatype", "")
		add("named whitespace "+content, "default-named", "", content, "enumeration", "")
		add("fixed whitespace "+content, "fixed", "", content, "fixed", "")
		add("mixed default whitespace "+content, "default-mixed", ` required="1"`, content, "", "")
	}
	for _, test := range []struct{ name, element, attrs, content, constraint, suffix string }{
		{"default does not constrain explicit value", "defaulted", "", "8", "", ""},
		{"default does not hide datatype", "defaulted", "", "bad", "datatype", ""},
		{"default does not hide enumeration", "default-named", "", "bad", "enumeration", ""},
		{"default does not hide child", "defaulted", "", "<child/>", "simple-content", ""},
		{"default does not hide simple-content child", "default-content", ` required="1"`, "<child/>", "simple-content", ""},
		{"mixed default retains child", "default-mixed", ` required="1"`, "<child>7</child>", "", ""},
		{"mixed default retains child error", "default-mixed", ` required="1"`, "<child>bad</child>", "datatype", "/child"},
		{"ordinary simple attribute", "defaulted", ` rubbish="x"`, "", "attribute", "/@rubbish"},
		{"required attribute with default", "default-content", "", "", "required", "/@required"},
		{"required attribute with fixed", "fixed-content", "", "", "required", "/@required"},
		{"attribute datatype", "default-content", ` required="bad"`, "", "datatype", "/@required"},
		{"attribute prohibited", "default-content", ` required="1" blocked="x"`, "", "prohibited", "/@blocked"},
		{"attribute fixed", "default-content", ` required="1" locked="other"`, "", "fixed", "/@locked"},
		{"attribute fixed normalized", "default-content", ` required="1" locked="&#x9;kept&#xA;"`, "", "", ""},
		{"malformed nil stays invalid", "defaulted", ` n:nil="maybe"`, "", "datatype", "/@{" + validationXSINamespace + "}nil"},
		{"nil checks original whitespace", "defaulted", ` n:nil="true"`, " ", "nillable", ""},
		{"no default for unconstrained integer", "integer", "", "", "datatype", ""},
		{"no default for required particle", "complex", ` required="1"`, "", "content-model", ""},
		{"schema hint remains permitted", "defaulted", ` n:noNamespaceSchemaLocation="https://example.invalid/schema.xsd"`, "", "", ""},
		{"nonnillable default", "default-plain", "", "", "", ""},
		{"nonnillable false still rejected", "default-plain", ` n:nil="false"`, "", "nillable", "/@{" + validationXSINamespace + "}nil"},
		{"default does not hide pattern", "default-pattern", "", "7", "pattern", ""},
	} {
		add(test.name, test.element, test.attrs, test.content, test.constraint, test.suffix)
	}
	tests = append(tests,
		validationAttributeCase{name: "referenced constraints", source: `<value-items><defaulted/><fixed/></value-items>`},
		validationAttributeCase{name: "omitted optional elements are not created", source: `<value-items/>`},
		validationAttributeCase{name: "required element is not created", source: `<required-value/>`, constraint: "content-model", path: "/required-value"},
		validationAttributeCase{name: "present required element defaults", source: `<required-value><defaulted/></required-value>`},
	)
	// Local declarations carry their own constraints, without a global ref.
	for _, element := range []struct{ name, value, whitespaceFailure string }{
		{"local-default", "7", "datatype"}, {"local-fixed", "kept", "fixed"},
	} {
		for _, test := range []struct{ name, attributes, content, constraint string }{
			{"empty", "", "", ""},
			{"comments", "", "<!--comment-->", ""},
			{"explicit", "", element.value, ""},
			{"false nil", ` n:nil="false"`, "", ""},
			{"zero nil", ` n:nil="0"`, "", ""},
			{"whitespace", "", " ", element.whitespaceFailure},
			{"unknown attribute", ` rubbish="x"`, "", "attribute"},
		} {
			path := "/value-items/" + element.name
			if test.constraint == "attribute" {
				path += "/@rubbish"
			}
			tests = append(tests, validationAttributeCase{
				name:       element.name + " " + test.name,
				source:     `<value-items xmlns:n="` + validationXSINamespace + `"><` + element.name + test.attributes + `>` + test.content + `</` + element.name + `></value-items>`,
				constraint: test.constraint, path: path,
			})
		}
	}
	return tests
}

func TestValidateElementValueConstraints(t *testing.T) {
	t.Parallel()
	for _, test := range validationElementValueCases() {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertValidationNilCase(t, &validationNilGenerated, test)
		})
	}
}

func TestElementValueConstraintsAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	directory, err := filepath.Abs(filepath.Join("testdata", "validation"))
	require.NoError(t, err)
	for _, test := range validationElementValueCases() {
		t.Run(test.name, func(t *testing.T) {
			context := validateAttributeSource(t, &validationNilGenerated, test.source)
			wantValid := test.constraint == ""
			assert.Equal(t, wantValid, len(context.issues) == 0, "internal validation: %v", context.issues)
			assertXMLLintOutcome(t, xmllint, filepath.Join(directory, "nil-contract.xsd"),
				filepath.Join(directory, "catalog.xml"), test.source, wantValid)
		})
	}
}

// The oracle does not reliably enforce unresolved IDREF(S). These checks
// independently assert both the effective references and their diagnostic paths.
func TestElementValueConstraintIdentityTracking(t *testing.T) {
	t.Parallel()
	for _, element := range []string{"default-ref", "fixed-ref", "default-refs", "fixed-refs", "default-reference-content", "fixed-reference-content"} {
		for _, nilAttribute := range []string{"", ` n:nil="false"`} {
			for _, resolved := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s%s resolved=%t", element, nilAttribute, resolved), func(t *testing.T) {
					attributes, targets := "", ""
					if strings.HasSuffix(element, "content") {
						attributes = ` id="own"`
					}
					if resolved {
						targets = `<target id="target"/><target id="other"/>`
					}
					source := `<value-items xmlns:n="` + validationXSINamespace + `"><` + element + attributes + nilAttribute + `/>` + targets + `</value-items>`
					context := validateAttributeSource(t, &validationNilGenerated, source)
					want := []validationIdentityReference{{value: "target", path: "/value-items/" + element}}
					if strings.HasSuffix(element, "refs") {
						want = append(want, validationIdentityReference{value: "other", path: "/value-items/" + element})
					}
					assert.Equal(t, want, context.references)
					if strings.HasSuffix(element, "content") {
						assert.Equal(t, "/value-items/"+element+"/@id", context.identifiers["own"])
					}
					if resolved {
						assert.Empty(t, context.issues)
					} else {
						require.Len(t, context.issues, len(want))
						for _, issue := range context.issues {
							assert.Equal(t, "IDREF", issue.Constraint)
							assert.Equal(t, "/value-items/"+element, issue.Path)
						}
					}
				})
			}
		}
	}
	for _, element := range []string{"default-ref", "default-refs", "default-reference-content", "fixed-ref", "fixed-refs", "fixed-reference-content"} {
		attributes := ""
		if strings.HasSuffix(element, "content") {
			attributes = ` id="own"`
		}
		context := validateAttributeSource(t, &validationNilGenerated, `<`+element+` xmlns:n="`+validationXSINamespace+`" n:nil="true"`+attributes+`/>`)
		if strings.HasPrefix(element, "fixed") {
			require.Len(t, context.issues, 1)
			assert.Equal(t, "fixed", context.issues[0].Constraint)
		} else {
			assert.Empty(t, context.issues)
		}
		assert.Empty(t, context.references, "true nil must not record a value constraint")
	}
	context := validateAttributeSource(t, &validationNilGenerated, `<value-items/>`)
	assert.Empty(t, context.references, "absent elements must not contribute default references")
}

func TestElementValueConstraintsPreservePublicModels(t *testing.T) {
	t.Parallel()
	// The pinned production schemas have attribute defaults, but no element
	// value constraints. Validate must not materialize defaults into any model.
	for _, source := range []string{
		validationAttributeScore,
		`<score-timewise><part-list><score-part id="P1"><part-name>Music</part-name></score-part></part-list><measure number="1"><part id="P1"/></measure></score-timewise>`,
		`<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="score.musicxml"/></opus>`,
	} {
		document, err := Decode(strings.NewReader(source))
		require.NoError(t, err)
		var before, after bytes.Buffer
		require.NoError(t, Encode(&before, document))
		assert.NoError(t, Validate(document))
		require.NoError(t, Encode(&after, document))
		assert.Equal(t, before.String(), after.String())
	}
}

func TestElementValueConstraintsPreserveSourceNodes(t *testing.T) {
	t.Parallel()
	for _, element := range []string{"defaulted", "fixed", "default-content", "fixed-content", "default-mixed", "fixed-mixed"} {
		attributes := ""
		if strings.Contains(element, "content") || strings.Contains(element, "mixed") {
			attributes = ` required="1" id="own"`
		}
		root, err := parseValidationDocument([]byte("<" + element + attributes + "><!--comment--><?note allowed?></" + element + ">"))
		require.NoError(t, err)
		originalAttributes := append([]validationAttribute(nil), root.Attrs...)
		for range 2 {
			context := validationContext{schema: &validationNilGenerated, effective: make(map[*validationComplexSchema]*validationEffectiveComplex), identifiers: make(map[string]string)}
			context.validateElement(root, validationNilGenerated.Elements[validationQName{Local: element}], "/"+element)
			assert.Empty(t, context.issues)
			assert.Zero(t, root.Text.Len(), "effective value must not replace original source text")
			assert.Empty(t, root.Children)
			assert.Equal(t, originalAttributes, root.Attrs)
		}
	}
}

func TestElementValueConstraintEmptyCDATA(t *testing.T) {
	t.Parallel()
	// An empty CDATA section contributes no character information items. The
	// libxml2 oracle distinguishes its node from absent text, so keep this
	// infoset boundary independent rather than changing the expected behavior.
	for _, element := range []string{"defaulted", "default-named", "fixed"} {
		assertValidationNilCase(t, &validationNilGenerated, validationAttributeCase{
			source: "<" + element + "><![CDATA[]]></" + element + ">",
		})
	}
}
