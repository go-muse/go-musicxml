package musicxml

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validationXSICases() []validationSourceCase {
	var tests []validationSourceCase
	add := func(name, element, attrs, content string, constraints ...string) {
		var issues []string
		for _, constraint := range constraints {
			issues = append(issues, "/"+element+constraint)
		}
		tests = append(tests, validationSourceCase{name: name, source: `<t:` + element + ` xmlns:t="urn:xsi-contract" xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:i="http://www.w3.org/2001/XMLSchema-instance"` + attrs + `>` + content + `</t:` + element + `>`, issues: issues, oracleComparable: true})
	}
	const xsi = "/@{http://www.w3.org/2001/XMLSchema-instance}type:xsi:type"
	add("same simple", "text", ` i:type="t:Text"`, "anything")
	add("restricted simple", "text", ` i:type="t:Narrow"`, " ok ")
	add("restricted rejection", "text", ` i:type="t:Narrow"`, "wrong", ":enumeration")
	add("namespace provenance lookalike", "text", ` xmlns:p="xmlns" p:z="urn:wrong" xmlns:z="urn:xsi-contract" i:type="z:Narrow"`, "ok", "/@{xmlns}z:attribute")
	add("prefix alias", "text", ` xmlns:z="urn:xsi-contract" i:type="z:Narrow"`, "ok")
	add("default QName", "text", ` xmlns="urn:xsi-contract" i:type="Narrow"`, "ok")
	add("unprefixed no namespace", "text", ` i:type="Narrow"`, "ok", xsi)
	add("unbound prefix", "text", ` i:type="absent:Narrow"`, "ok", xsi)
	add("QName pinned local repertoire", "builtin", ` i:type="xs:Ƞ"`, "ok", xsi)
	add("malformed QName", "text", ` i:type="t:a:b"`, "ok", xsi)
	add("unknown named type", "text", ` i:type="t:Missing"`, "ok", xsi)
	add("unknown builtin", "text", ` i:type="xs:Missing"`, "ok", xsi)
	add("QName XML whitespace", "text", ` i:type="&#x9;t:Narrow&#xA;"`, "ok")
	add("unrelated named", "text", ` i:type="t:Base"`, "ok", xsi)
	add("unrelated builtin", "text", ` i:type="xs:integer"`, "1", xsi)
	add("builtin descendant", "builtin", ` i:type="xs:NCName"`, "good")
	add("builtin descendant reject", "builtin", ` i:type="xs:NCName"`, "bad name", ":datatype")
	add("nilled ENTITY needs no value facts", "builtin", ` i:type="xs:ENTITY" i:nil="true"`, "")
	add("ENTITY lexical failure proved", "builtin", ` i:type="xs:ENTITY"`, "two words", ":datatype")
	add("ENTITIES not string derived", "builtin", ` i:type="xs:ENTITIES"`, "one two", xsi)
	add("integer chain", "decimal", ` i:type="xs:unsignedByte"`, "255")
	add("integer boundary", "decimal", ` i:type="xs:unsignedByte"`, "256", ":datatype")
	add("anonymous unequal", "anonymous", ` i:type="t:Text"`, "ok", xsi)
	add("any simple list", "simple", ` i:type="t:TextList"`, "a b")
	add("any type complex", "any", ` i:type="t:Base"`, "")
	add("list restriction", "list", ` i:type="t:Pair"`, "a b")
	add("list restriction reject", "list", ` i:type="t:Pair"`, "a", ":length")
	add("list item not derived", "builtin", ` i:type="t:TextList"`, "a", xsi)
	add("union member", "union", ` i:type="xs:integer"`, "1")
	add("union member descendant", "union", ` i:type="xs:positiveInteger"`, "1")
	add("union restriction", "union", ` i:type="t:ChoiceRestriction"`, "1")
	add("union restriction reject", "union", ` i:type="t:ChoiceRestriction"`, "2", ":enumeration")
	add("simple to complex extension", "text", ` i:type="t:TextAttrs" code="yes"`, "ok")
	add("complex extension", "base", ` i:type="t:Extended" code="yes"`, `<t:a>x</t:a><t:b>2</t:b>`)
	add("complex extension required", "base", ` i:type="t:Extended"`, `<t:b>2</t:b>`, "/@code:required")
	add("complex extension content", "base", ` i:type="t:Extended" code="yes"`, "", ":content-model")
	add("complex restriction", "base", ` i:type="t:Restricted"`, "")
	add("complex restriction attr", "base", ` i:type="t:Restricted" id="own"`, "", "/@id:prohibited")
	add("restriction inherits attributes", "attribute-base", ` i:type="t:AttributeRestriction" code="1" label="ok"`, "")
	add("restriction inherits required", "attribute-base", ` i:type="t:AttributeRestriction"`, "", "/@code:required")
	add("nilled restriction inherits required", "attribute-base", ` i:type="t:AttributeRestriction" i:nil="true"`, "", "/@code:required")
	add("restriction inherited datatype", "attribute-base", ` i:type="t:AttributeRestriction" code="bad"`, "", "/@code:datatype")
	add("restricted wildcard", "wildcard-text", ` i:type="t:RestrictedWildcard" extra="invalid"`, "", "/@extra:attribute")
	add("same wildcard allowed", "wildcard-text", ` i:type="t:WildcardText" extra="valid"`, "")
	add("substitution bit not derivation block", "block-substitution", ` i:type="t:Extended" code="yes"`, `<t:b>1</t:b>`)
	add("builtin restriction blocked", "blocked-builtin", ` i:type="xs:token"`, "text", xsi)
	add("builtin same blocked", "blocked-builtin", ` i:type="xs:string"`, "text")
	add("block extension", "block-extension", ` i:type="t:Extended"`, "", xsi)
	add("allow restriction", "block-extension", ` i:type="t:Restricted"`, "")
	add("block restriction", "block-restriction", ` i:type="t:Restricted"`, "", xsi)
	add("allow extension", "block-restriction", ` i:type="t:Extended" code="yes"`, `<t:b>2</t:b>`)
	add("block same", "block-all", ` i:type="t:Base"`, "")
	add("element final not block", "element-final", ` i:type="t:Extended" code="yes"`, `<t:b>2</t:b>`)
	add("complex type block", "type-block", ` i:type="t:BlockedChild"`, "", xsi)
	add("abstract declared", "abstract-type", ` code="yes"`, "", ":abstract")
	add("abstract same nomination", "abstract-type", ` i:type="t:Abstract" code="yes"`, "", ":abstract")
	add("abstract nil nomination", "abstract-type", ` i:type="t:Abstract" i:nil="true" code="yes"`, "", ":abstract")
	add("concrete substitution", "abstract-type", ` i:type="t:Concrete" code="yes"`, "")
	add("abstract element", "abstract-element", ` i:type="t:Concrete" code="yes"`, "", ":abstract")
	add("final same", "final", ` i:type="t:Final"`, "")
	add("recursive type valid", "recursive", ` i:type="t:Recursive"`, `<t:child><t:child/></t:child>`)
	add("simple content restriction", "attrs", ` i:type="t:TextRestrict" code=" yes "`, "ok")
	add("simple content enum", "attrs", ` i:type="t:TextRestrict" code="yes"`, "wrong", ":enumeration")
	add("simple content fixed attr", "attrs", ` i:type="t:TextRestrict" code="no"`, "ok", "/@code:fixed")
	add("nilled derived required", "attrs", ` i:type="t:TextRestrict" i:nil="true"`, "", "/@code:required")
	add("nilled derived suppress scalar", "attrs", ` i:type="t:TextRestrict" i:nil="true" code="yes"`, "")
	add("nilled derived identity", "text", ` i:type="t:TextAttrs" i:nil="true" code="yes" ref="absent"`, "", "/@ref:IDREF")
	add("non nillable dynamic", "block-all", ` i:type="t:Base" i:nil="true"`, "", "/@{http://www.w3.org/2001/XMLSchema-instance}nil:nillable")
	add("invalid nomination keeps ordinary assessment", "attrs", ` i:type="t:Missing"`, "", xsi, "/@code:required")
	add("canonical integer default", "default-int", ` i:type="t:CanonicalInt"`, "")
	add("canonical decimal default", "default-decimal", ` i:type="t:CanonicalDecimal"`, "")
	add("canonical integer fixed", "fixed-int", ` i:type="t:CanonicalInt"`, "")
	add("narrow default rejected", "default-int", ` i:type="t:Positive"`, "", ":minInclusive")
	add("fixed actual type", "fixed-int", ` i:type="t:CanonicalInt"`, "1")
	add("fixed actual value mismatch", "fixed-int", ` i:type="xs:positiveInteger"`, "2", ":fixed")
	add("fixed actual lexical failure", "fixed-int", ` i:type="t:CanonicalInt"`, "+01", ":pattern")
	add("mixed default empty dynamic", "default-mixed", ` i:type="t:Empty"`, "", ":default")
	add("empty mixed default empty dynamic", "default-empty-mixed", ` i:type="t:Empty"`, "", ":default")
	add("mixed default required particle", "default-optional", ` i:type="t:MixedRequired"`, "", ":default")
	add("inherited scope", "scope", ` xmlns:z="urn:xsi-contract"`, `<t:text i:type="z:Narrow">ok</t:text><t:text xmlns:z="http://www.w3.org/2001/XMLSchema" i:type="z:string">ok</t:text>`, "/text[2]"+xsi)
	add("scope not sibling leak", "scope", ` xmlns:z="urn:xsi-contract"`, `<t:text xmlns:z="http://www.w3.org/2001/XMLSchema" i:type="z:string">ok</t:text><t:text i:type="z:Narrow">ok</t:text>`, "/text"+xsi)
	add("default undeclaration", "scope", ` xmlns="urn:xsi-contract"`, `<t:text xmlns="" i:type="Narrow">ok</t:text>`, "/text"+xsi)
	// Exact-source supplemental JAXP agrees with the normative expectations for
	// these libxml2 2.9.14 divergences; internal assessment still executes here.
	for index := range tests {
		switch tests[index].name {
		case "QName XML whitespace", "nilled derived identity", "canonical integer default", "canonical decimal default", "canonical integer fixed", "fixed actual type":
			tests[index].oracleComparable = false
		}
	}
	return tests
}

func TestValidateXSIType(t *testing.T) {
	t.Parallel()
	for _, test := range validationXSICases() {
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationXSIGenerated, test.source), test.issues)
		})
	}
}

func TestXSITypeAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	directory, err := filepath.Abs("testdata/validation")
	require.NoError(t, err)
	for _, test := range validationXSICases() {
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationXSIGenerated, test.source), test.issues)
			if test.oracleComparable {
				assertXMLLintOutcome(t, xmllint, filepath.Join(directory, "xsi-type-contract.xsd"), filepath.Join(directory, "catalog.xml"), test.source, len(test.issues) == 0)
			}
		})
	}
	for _, test := range validationXSIDefaultCases() {
		t.Run("defaults/"+test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationXSIDefaultsGenerated, test.source), test.issues)
			assertXMLLintOutcome(t, xmllint, filepath.Join(directory, "xsi-defaults-contract.xsd"), filepath.Join(directory, "catalog.xml"), test.source, len(test.issues) == 0)
		})
	}

}

func TestXSITypeImmutable(t *testing.T) {
	t.Parallel()
	before := snapshotValidationSchema(t, &validationXSIGenerated)
	for worker := range 4 {
		t.Run(string(rune('A'+worker)), func(t *testing.T) {
			t.Parallel()
			for range 3 {
				for _, test := range validationXSICases() {
					node, err := parseValidationDocument([]byte(test.source))
					require.NoError(t, err)
					saved := snapshotValidationNode(t, node)
					c := newTestValidationContext(&validationXSIGenerated)
					c.validateElement(node, validationXSIGenerated.Elements[validationName(node.Name)], "/"+node.Name.Local)
					c.validateIdentityReferences()
					assertValidationIssues(t, c, test.issues)
					assert.Equal(t, saved, snapshotValidationNode(t, node))
				}
			}
			assert.Equal(t, before, snapshotValidationSchema(t, &validationXSIGenerated))
		})
	}
}
