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

type validationSchemaHintValueCase struct {
	name, value      string // value is original XML attribute source, not decoded text.
	valid            bool
	oracleComparable bool
}

func validationSchemaHintValueCases(list bool) []validationSchemaHintValueCase {
	tests := []validationSchemaHintValueCase{
		{"empty", "", true, true},
		{"XML whitespace only", " &#x9;&#xA;&#xD; ", true, true},
		{"relative", "schemas/musicxml.xsd", true, true},
		{"fragment", "schema.xsd#type", true, true},
		{"escaped percent", "schema%25.xsd", true, true},
		{"escaped brackets", "%5Bschema%5D.xsd", true, true},
		{"Unicode", "https://例.example/楽譜.xsd", true, true},
		{"NBSP inside path", "schema&#xA0;name.xsd", true, true},
		{"NBSP is not list whitespace", "http://[::1]&#xA0;schema.xsd", false, true},
		{"IPv6", "http://[2001:db8::1]/schema.xsd", true, true},
		{"bare percent", "%", false, true},
		{"truncated escape", "schema%2", false, true},
		{"query escape", "schema.xsd?q=%GG", false, true},
		{"opaque escape", "urn:schema:%GG", false, true},
		{"multiple fragments", "schema.xsd#one#two", false, true},
		{"path brackets", "schema[1].xsd", false, true},
		{"malformed authority", "http://[invalid", false, true},
		{"invalid scheme", "1scheme:schema.xsd", false, true},
		// Reuse the independently reviewed anyURI grammar, not libxml2's stricter
		// URI parser. These exact exclusions also exist in validation_uri_test.go.
		{"registry authority", "http://host:abc", true, false},
		{"empty absolute URI", "scheme:", false, false},
	}
	if list {
		tests = append(tests,
			validationSchemaHintValueCase{"one item", "urn:example", true, true},
			validationSchemaHintValueCase{"two items", "urn:example schema.xsd", true, true},
			validationSchemaHintValueCase{"three items", "urn:example schema.xsd orphan.xsd", true, true},
			validationSchemaHintValueCase{"four items", "urn:example schema.xsd urn:other other.xsd", true, true},
			validationSchemaHintValueCase{"XML whitespace separators", " &#x9;urn:example&#xA;schema.xsd&#xD;orphan.xsd  ", true, true},
			validationSchemaHintValueCase{"invalid first item", "% schema.xsd", false, true},
			validationSchemaHintValueCase{"invalid last item", "urn:example schema.xsd %", false, true},
			validationSchemaHintValueCase{"two invalid items one attribute issue", "% http://[invalid", false, true},
			// Splitting on Unicode whitespace would incorrectly accept these as two
			// URI items. Only XML whitespace participates in list tokenization.
			validationSchemaHintValueCase{"NEL is not a separator", "http://[::1]&#x85;schema.xsd", false, true},
			validationSchemaHintValueCase{"em space is not a separator", "http://[::1]&#x2003;schema.xsd", false, true},
		)
	} else {
		tests = append(tests,
			validationSchemaHintValueCase{"spaces inside one URI", "schema name.xsd", true, true},
			validationSchemaHintValueCase{"XML whitespace collapse", " &#x9;schema&#xA; name.xsd&#xD; ", true, true},
		)
	}
	return tests
}

func validationSchemaHintDatatypeCases(carrier bool) []validationSourceCase {
	var tests []validationSourceCase
	for _, kind := range []struct {
		element, attribute string
		list               bool
	}{
		{"locations", "schemaLocation", true}, {"location", "noNamespaceSchemaLocation", false},
	} {
		for _, value := range validationSchemaHintValueCases(kind.list) {
			attr, attrPath := "n:"+kind.attribute, "@{"+validationXSINamespace+"}"+kind.attribute
			if carrier {
				attr, attrPath = "value", "@value"
			}
			test := validationSourceCase{
				name:             kind.attribute + "/" + value.name,
				source:           wrapValidationElement(kind.element, ` `+attr+`="`+value.value+`"`, ""),
				oracleComparable: value.oracleComparable,
			}
			if !value.valid {
				test.issues = []string{"/" + kind.element + "/" + attrPath + ":datatype"}
			}
			tests = append(tests, test)
		}
	}
	return tests
}

func TestValidateSchemaHintDatatypes(t *testing.T) {
	t.Parallel()
	for _, test := range validationSchemaHintDatatypeCases(false) {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertValidationIssues(t, validateAttributeSource(t, &validationSchemaHintGenerated, test.source), test.issues)
		})
	}
}

// The carrier fixtures have the exact built-in datatypes declared by XSD 1.0
// Part 1 §3.2.7, under ordinary attribute names. Both engines receive the same
// original carrier bytes, including character references. This is datatype
// evidence, not a claim of negative standard-hint parity: libxml2's explicit
// fixed-schema path does not assess malformed xsi hint values.
func TestSchemaHintDatatypesAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	directory, err := filepath.Abs("testdata/validation")
	require.NoError(t, err)
	for _, carrier := range []bool{true, false} {
		kind := "standard hint"
		if carrier {
			kind = "datatype carrier"
		}
		for _, test := range validationSchemaHintDatatypeCases(carrier) {
			t.Run(kind+"/"+test.name, func(t *testing.T) {
				assertValidationIssues(t, validateAttributeSource(t, &validationSchemaHintGenerated, test.source), test.issues)
				if !test.oracleComparable {
					t.Log("internal normative URI grammar case; bounded libxml2 URI-parser discrepancy")
					return
				}
				if !carrier && len(test.issues) > 0 {
					t.Log("negative standard-hint oracle excluded: fixed-schema libxml2 path omits hint datatype assessment; corresponding carrier is checked")
					return
				}
				assertXMLLintOutcome(t, xmllint, filepath.Join(directory, "schema-hint-contract.xsd"), filepath.Join(directory, "catalog.xml"), test.source, len(test.issues) == 0)
			})
		}
	}
}

func TestSchemaHintDatatypesAcrossElementKinds(t *testing.T) {
	t.Parallel()
	for _, target := range []struct{ name, source, marker, path string }{
		{"complex", validationAttributeScore, "<measure ", "/score-partwise/part/measure"},
		{"complex simple", validationAttributeScore, "<type ", "/score-partwise/part/measure/note/type"},
		{"named simple", validationAttributeScore, "<step", "/score-partwise/part/measure/note/pitch/step"},
		{"builtin", validationAttributeScore, "<staves", "/score-partwise/part/measure/attributes/staves"},
	} {
		for _, hint := range []string{"schemaLocation", "noNamespaceSchemaLocation"} {
			t.Run(target.name+"/"+hint, func(t *testing.T) {
				source := strings.Replace(target.source, target.marker, strings.TrimSuffix(target.marker, " ")+` alias:`+hint+`="%" xmlns:alias="`+validationXSINamespace+`" `, 1)
				assertValidationIssues(t, validateAttributeSource(t, &scoreValidationSchema, source), []string{target.path + "/@{" + validationXSINamespace + "}" + hint + ":datatype"})
			})
		}
	}
	// anyType still assesses standard hint datatypes at the element boundary.
	schema := &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{
		{Local: "value"}: {Name: validationQName{Local: "value"}, Type: validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "anyType"}}},
	}}
	assertValidationIssues(t, validateAttributeSource(t, schema, wrapValidationElement("value", ` n:schemaLocation="%" n:noNamespaceSchemaLocation="%"`, "")), []string{
		"/value/@{" + validationXSINamespace + "}schemaLocation:datatype", "/value/@{" + validationXSINamespace + "}noNamespaceSchemaLocation:datatype",
	})
}

func TestSchemaHintDatatypeNamespacesAndOrdinaryErrors(t *testing.T) {
	t.Parallel()
	source := `<opus xmlns:i="` + validationXSINamespace + `" xmlns:xlink="http://www.w3.org/1999/xlink" i:noNamespaceSchemaLocation="%"><score xmlns:i="urn:foreign" i:schemaLocation="%" xlink:href="a"/><score i:schemaLocation="%" xlink:href="b" xlink:type="extended"/></opus>`
	assertValidationIssues(t, validateAttributeSource(t, &opusValidationSchema, source), []string{
		"/opus/@{" + validationXSINamespace + "}noNamespaceSchemaLocation:datatype",
		"/opus/score/@{urn:foreign}schemaLocation:attribute",
		"/opus/score[2]/@{" + validationXSINamespace + "}schemaLocation:datatype",
		"/opus/score[2]/@{http://www.w3.org/1999/xlink}type:fixed",
		"/opus/score[2]/@{http://www.w3.org/1999/xlink}type:enumeration",
	})
	for _, attribute := range []string{`schemaLocation="%"`, `xmlns:f="urn:foreign" f:schemaLocation="%"`, `n:SchemaLocation="%"`, `n:unrecognized="%"`} {
		context := validateAttributeSource(t, &validationSchemaHintGenerated, wrapValidationElement("locations", " "+attribute, ""))
		require.Len(t, context.issues, 1)
		assert.Equal(t, "attribute", context.issues[0].Constraint)
	}
	// Ordinary required and scalar failures are still reported beside bad hints.
	source = strings.Replace(validationAttributeScore, ` number="1"`, ` xmlns:n="`+validationXSINamespace+`" n:schemaLocation="%" width="bad"`, 1)
	assertValidationIssues(t, validateAttributeSource(t, &scoreValidationSchema, source), []string{
		"/score-partwise/part/measure/@{" + validationXSINamespace + "}schemaLocation:datatype",
		"/score-partwise/part/measure/@number:required", "/score-partwise/part/measure/@width:datatype",
	})
}

func TestSchemaHintDatatypeImmutability(t *testing.T) {
	t.Parallel()
	before := snapshotValidationSchema(t, &validationSchemaHintGenerated)
	for worker := 0; worker < 4; worker++ {
		t.Run(string(rune('A'+worker)), func(t *testing.T) {
			t.Parallel()
			for _, test := range validationSchemaHintDatatypeCases(false) {
				node, err := parseValidationDocument([]byte(test.source))
				require.NoError(t, err)
				saved := snapshotValidationNode(t, node)
				for range 3 {
					context := newTestValidationContext(&validationSchemaHintGenerated)
					context.validateElement(node, validationSchemaHintGenerated.Elements[validationName(node.Name)], "/"+node.Name.Local)
					assertValidationIssues(t, context, test.issues)
					assert.Equal(t, saved, snapshotValidationNode(t, node))
				}
			}
			assert.Equal(t, before, snapshotValidationSchema(t, &validationSchemaHintGenerated))
		})
	}
}

func TestSchemaHintDatatypeXMLReadingBoundary(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		wrapValidationElement("locations", ` n:schemaLocation="&#0;"`, ""),
		wrapValidationElement("locations", ` n:schemaLocation="a" n:schemaLocation="b"`, ""),
	} {
		_, err := parseValidationDocument([]byte(source))
		assert.Error(t, err, "malformed XML never becomes a schema-hint datatype issue")
	}
}

func TestSchemaHintDatatypesOnNilledElements(t *testing.T) {
	t.Parallel()
	for _, element := range []string{"integer", "named", "inline", "default-content"} {
		attrs := ` n:nil="true" n:schemaLocation="%" n:noNamespaceSchemaLocation="%"`
		if element == "default-content" {
			attrs += ` required="1"`
		}
		assertValidationIssues(t, validateAttributeSource(t, &validationNilGenerated, wrapValidationElement(element, attrs, "")), []string{
			"/" + element + "/@{" + validationXSINamespace + "}schemaLocation:datatype", "/" + element + "/@{" + validationXSINamespace + "}noNamespaceSchemaLocation:datatype",
		})
	}
	source := wrapValidationElement("default-content", ` required="1" blocked="x" n:nil="true" n:schemaLocation="%"`, "")
	assertValidationIssues(t, validateAttributeSource(t, &validationNilGenerated, source), []string{
		"/default-content/@{" + validationXSINamespace + "}schemaLocation:datatype", "/default-content/@blocked:prohibited",
	})
}
