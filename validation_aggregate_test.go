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

func validationAggregateCases() []validationSourceCase {
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
		{"integer-enum", []string{"1 0 18446744073709551616", "+01 -0 +018446744073709551616", " 001&#x9;000&#xA;18446744073709551616 "}, []string{"1 0 18446744073709551617", "0 1 18446744073709551616", "1 0", "1 0 18446744073709551616 1", ""}},
		{"decimal-enum", []string{"1.25 0", "+001.2500 -000.000", "1.250 .0"}, []string{"1.2500000000000001 0", "0 1.25", "1.25"}},
		{"token-enum", []string{"a b", " a&#x9;b ", "a<!--split--> b"}, []string{"b a", "ab", "a b b", "a b"}},
		{"union-enum", []string{"1", "+01.0", " 001.000 "}, []string{"2", "1.0000000000000001"}},
		{"collision-enum", []string{"1"}, []string{" 1 ", "+01.0", "01"}},
		{"string-family-enum", []string{"1", " 1 "}, []string{"01", "2"}},
		{"nested-enum", []string{"1", "+01.0"}, []string{"2", "a"}},
		{"list-union-enum", []string{"1 a", "+01 a", " 001 a "}, []string{"a 1", "1 b", "1 a a"}},
		{"collision-items-enum", []string{"1 2"}, []string{"+01 2", "1 +02.0", "01 2"}},
		{"number-items-enum", []string{"1 2", "+01.0 +02.00"}, []string{"1 3"}},
		{"union-list-enum", []string{"1 2", "+01 002", " 1\t2 "}, []string{"1", "2 1", "a b"}},
		{"list-shape-enum", []string{"1", "01"}, []string{"+01"}},
		{"number-shape-enum", []string{"1", "01"}, []string{"+01"}},
		{"inherited-enum", []string{"+01 -0 18446744073709551616"}, []string{"1 0 18446744073709551617"}},
		{"bounded-enum", []string{"01 02"}, []string{"2 1"}},
		{"builtin-enum", []string{"aa bb", " aa&#x9;bb "}, []string{"bb aa", "aa bb aa"}},
		{"member-enum", []string{"1", "+01"}, []string{"x", "2"}},
	} {
		for _, value := range test.accepted {
			add(test.element+"/accept/"+value, test.element, "", value)
		}
		for _, value := range test.rejected {
			add(test.element+"/reject/"+value, test.element, "", value, ":enumeration")
		}
	}
	add("pattern keeps lexical", "pattern-enum", "", "01")
	add("pattern sees first member", "pattern-enum", "", "1", ":pattern")
	add("pattern sees decimal spelling", "pattern-enum", "", "+01.0", ":pattern")
	add("bounded item before enumeration", "bounded-enum", "", "0 2", ":minInclusive")
	add("malformed list item", "integer-enum", "", "1 nope 0", ":datatype")
	add("decimal list exponent", "decimal-enum", "", "1.25 0e0", ":datatype")
	for _, test := range []struct{ element, equal, different string }{
		{"integer-fixed", "+01 -0", "1 1"}, {"decimal-fixed", "+001.2500 -0.0", "1.26 0"}, {"token-fixed", " a\tb ", "b a"},
		{"union-fixed", "+01.0", "2"}, {"collision-fixed", "1", " 1 "}, {"list-union-fixed", "+01 a", "a 1"}, {"union-list-fixed", "+01 02", "2 1"},
	} {
		add(test.element+" equal", test.element, "", test.equal)
		add(test.element+" different", test.element, "", test.different, ":fixed")
		add(test.element+" absent text", test.element, "", "")
	}
	add("fixed equality independent of outer pattern", "pattern-fixed", "", "1", ":pattern")
	add("fixed pattern exact", "pattern-fixed", "", "01")
	add("default content", "default", "", "")
	add("fixed nil", "integer-fixed", ` n:nil="true"`, "", ":fixed")
	add("fixed child", "integer-fixed", "", "1 0<child/>", ":fixed", ":simple-content")
	for _, element := range []string{"complex", "extended", "complex-fixed"} {
		add(element+" normalized values", element, ` code="ok"`, "+001.250 -0.00")
		issues := []string{":enumeration"}
		if element == "complex-fixed" {
			issues = append(issues, ":fixed")
		}
		add(element+" wrong value", element, ` code="ok"`, "1.26 0", issues...)
	}
	add("complex required", "complex", "", "1.25 0", "/@code:required")
	add("extension union attribute", "extended", ` code="ok" extra="+01.0"`, "1.25 0")
	for _, test := range []struct{ name, value string }{
		{"integers", "+01 -0"}, {"decimals", "+001.250 -0.0"}, {"union", "+01.0"}, {"collision", "1"}, {"list-union", "+01 a"}, {"union-list", "+01 02"},
	} {
		add("attribute "+test.name, "attributes", ` `+test.name+`="`+test.value+`"`, "")
	}
	add("builtin fixed equal", "builtin-fixed", "", " aa&#x9;bb ")
	add("builtin fixed unequal", "builtin-fixed", "", "bb aa", ":fixed")
	add("builtin attribute fixed", "attributes", ` builtin=" aa&#x9;bb "`, "")
	add("attribute domain collision", "attributes", ` collision=" 1 "`, "", "/@collision:fixed")
	add("attribute enum", "attributes", ` enum="+01 -0 18446744073709551616"`, "")
	add("attribute outer pattern", "attributes", ` pattern="1"`, "", "/@pattern:pattern")
	add("mixed equal", "mixed", "", " 1 ")
	add("mixed lexical", "mixed", "", "1", ":fixed")
	// Concrete libxml2 2.9.14 divergences: explicit element fixed values use
	// lexical comparison, and singleton lists compare equal to atomic numbers.
	for index := range tests {
		name := tests[index].name
		switch name {
		case "builtin fixed equal", "integer-fixed equal", "decimal-fixed equal", "token-fixed equal", "union-fixed equal", "list-union-fixed equal", "union-list-fixed equal", "complex-fixed normalized values", "list-shape-enum/reject/+01", "number-shape-enum/reject/+01":
			tests[index].oracleComparable = false
		}
	}
	return tests
}

func TestValidateAggregateValues(t *testing.T) {
	t.Parallel()
	for _, test := range validationAggregateCases() {
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationAggregateGenerated, test.source), test.issues)
		})
	}
}

func TestAggregateValuesAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	directory, err := filepath.Abs("testdata/validation")
	require.NoError(t, err)
	for _, test := range validationAggregateCases() {
		if !test.oracleComparable {
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assertValidationIssues(t, validateAttributeSource(t, &validationAggregateGenerated, test.source), test.issues)
			assertXMLLintOutcome(t, xmllint, filepath.Join(directory, "aggregate-contract.xsd"), filepath.Join(directory, "catalog.xml"), test.source, len(test.issues) == 0)
		})
	}
}

func TestAggregateValuesImmutable(t *testing.T) {
	t.Parallel()
	before := snapshotValidationSchema(t, &validationAggregateGenerated)
	for worker := 0; worker < 4; worker++ {
		t.Run(string(rune('A'+worker)), func(t *testing.T) {
			t.Parallel()
			for range 3 {
				for _, test := range validationAggregateCases() {
					node, err := parseValidationDocument([]byte(test.source))
					require.NoError(t, err)
					saved := snapshotValidationNode(t, node)
					context := newTestValidationContext(&validationAggregateGenerated)
					context.ValidateElement(node, validationAggregateGenerated.Elements[validationQName{Local: node.Name.Local}], "/"+node.Name.Local)
					context.ValidateIdentityReferences()
					assertValidationIssues(t, context, test.issues)
					after := snapshotValidationNode(t, node)
					assert.Equal(t, saved, after)
				}
			}
			assert.Equal(t, before, snapshotValidationSchema(t, &validationAggregateGenerated))
		})
	}
}

func TestAggregateDiagnosticSpelling(t *testing.T) {
	t.Parallel()
	context := validateAttributeSource(t, &validationAggregateGenerated, `<integer-enum> +0001 -0 18446744073709551617 </integer-enum>`)
	require.Len(t, context.Issues, 1)
	assert.Equal(t, `value " +0001 -0 18446744073709551617 " is not one of the allowed values`, context.Issues[0].Message)
	context = validateAttributeSource(t, &validationAggregateGenerated, `<integer-fixed> +0001 -0 </integer-fixed>`)
	assert.Empty(t, context.Issues)
	context = validateAttributeSource(t, &validationAggregateGenerated, `<integer-fixed> +0001 -01 </integer-fixed>`)
	require.Len(t, context.Issues, 1)
	assert.Equal(t, `value " +0001 -01 " does not equal fixed value "1 0"`, context.Issues[0].Message)
}

// Internal-only exact-width contract; libxml2's numeric implementation is not
// an arbitrary-precision oracle (the existing numeric fixtures document it).
func TestAggregateWideExactValues(t *testing.T) {
	t.Parallel()
	huge := strings.Repeat("9", 4096)
	context := newTestValidationContext(&validationAggregateGenerated)
	list := &validationSimpleMember{Name: validationQName{Local: "integers"}}
	schema := &validationSimpleSchema{Form: validationSimpleRestriction, Base: list, Enumerations: []string{huge + " 0"}}
	assert.Nil(t, context.ValidateSimple(schema, "+0"+huge+" -0"))
	failure := context.ValidateSimple(schema, huge+" 1")
	require.NotNil(t, failure)
	assert.Equal(t, "enumeration", failure.Constraint)
}

func TestAggregateEmptyListEnumeration(t *testing.T) {
	t.Parallel()
	// libxml2 2.9.14 cannot compile an empty-list enumeration literal: its
	// xmlSchemaCheckFacet reports "value was not computed". Keep it internal.
	context := newTestValidationContext(&validationAggregateGenerated)
	schema := &validationSimpleSchema{Form: validationSimpleRestriction,
		Base: &validationSimpleMember{Name: validationQName{Local: "integers"}}, Enumerations: []string{""}}
	for range 3 {
		for _, value := range []string{"", " ", "\t\n"} {
			assert.Nil(t, context.ValidateSimple(schema, value))
		}
		for _, value := range []string{"0", "1 2"} {
			failure := context.ValidateSimple(schema, value)
			require.NotNil(t, failure)
			assert.Equal(t, "enumeration", failure.Constraint)
		}
	}
}

func TestAggregateEnumerationCacheBound(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"integer-enum", "list-union-enum", "union-list-enum", "collision-items-enum"} {
		t.Run(name, func(t *testing.T) {
			context := newTestValidationContext(&validationAggregateGenerated)
			schema := validationAggregateGenerated.Types[validationQName{Local: name}].Simple
			for count := 1; count <= 100; count++ {
				context.ValidateSimple(schema, strings.Repeat("1 ", count))
				assert.Len(t, context.EnumerationValues, len(schema.Enumerations), "cache keys must name schema literals, never instance values")
			}
		})
	}
}

// These are explicit compatibility limitations, not normative equality claims.
// The bounded carrier does not implement temporal value equality.
func TestAggregateUnimplementedPrimitiveEquality(t *testing.T) {
	t.Parallel()
	context := newTestValidationContext(&validationAggregateGenerated)
	for _, builtin := range []string{"date"} {
		first, second := "2000-01-01Z", "2000-01-01+00:00"
		member := &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: builtin}}
		for _, form := range []validationSimpleForm{validationSimpleList, validationSimpleUnion} {
			aggregate := &validationSimpleSchema{Form: form, Item: member, Members: []*validationSimpleMember{member}}
			schema := &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Inline: aggregate}, Enumerations: []string{first}}
			assert.Nil(t, context.ValidateSimple(schema, first))
			failure := context.ValidateSimple(schema, second)
			require.NotNil(t, failure)
			assert.Equal(t, "enumeration", failure.Constraint)
		}
	}
}

func TestAggregateBuiltinListValues(t *testing.T) {
	t.Parallel()
	context := newTestValidationContext(&validationAggregateGenerated)
	for _, builtin := range []string{"NMTOKENS", "IDREFS"} {
		reference := &validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: builtin}}
		assert.True(t, context.SimpleValuesEqual(reference, " aa\tbb ", "aa bb"), builtin)
		assert.True(t, context.ElementFixedValuesEqual(reference, " aa\tbb ", "aa bb"), builtin)
		assert.False(t, context.SimpleValuesEqual(reference, "aa bb", "bb aa"), builtin)
		assert.False(t, context.SimpleValuesEqual(reference, "aa bb", "aa bb aa"), builtin)
		assert.False(t, context.SimpleValuesEqual(reference, "", ""), builtin)
	}
}
