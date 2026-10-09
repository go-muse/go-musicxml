package musicxml

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These original-source cases do not depend on the new decimal helper, so they
// can be replayed unchanged against the pre-repair runtime.
func validationDecimalCases() []validationSourceCase {
	var tests []validationSourceCase
	add := func(name, element, attrs, value string, comparable bool, constraints ...string) {
		var issues []string
		for _, constraint := range constraints {
			issues = append(issues, "/"+element+constraint)
		}
		tests = append(tests, validationSourceCase{name: name, source: wrapValidationElement(element, attrs, value), issues: issues, oracleComparable: comparable})
	}
	for _, value := range []string{"0", "+0", "-0", "-000.000", "+001.2500", "1.", ".1", "-.1", " \t+0001.25\r\n", "9007199254740993.1", strings.Repeat("9", 310), "0." + strings.Repeat("0", 400) + "1"} {
		add("builtin/"+value, "decimal", "", value, len(value) < 30)
	}
	for _, value := range []string{"", " ", "+", "-", ".", "+.", "1.2.3", "++1", "+-0", "1e0", "0x1", "1_0", "1 0", "1\t0", "INF", "-INF", "NaN", "١", "１", "\u00a01", "1\u0085", "\u20031"} {
		add("lexical/"+value, "decimal", "", value, true, ":datatype")
	}
	for _, test := range []struct {
		element            string
		accepted, rejected []string
		constraint         string
	}{
		{"min", []string{"1.0000000000000001", "+001.00000000000000010", "1.0000000000000002"}, []string{"1", "1.0000000000000000"}, "minInclusive"},
		{"max", []string{"1", "1.0000000000000001"}, []string{"1.0000000000000002"}, "maxInclusive"},
		{"min-exclusive", []string{"1.0000000000000002"}, []string{"1", "1.0000000000000001", "+001.00000000000000010"}, "minExclusive"},
		{"max-exclusive", []string{"1"}, []string{"1.0000000000000001", "1.0000000000000002"}, "maxExclusive"},
		{"negative-max", []string{"-1.0000000000000001", "-1.0000000000000002"}, []string{"-1"}, "maxInclusive"},
		{"huge-min", []string{"18446744073709551616.25", "18446744073709551616.26", strings.Repeat("9", 310)}, []string{"18446744073709551616.24"}, "minInclusive"},
		{"positive", []string{".1", "0." + strings.Repeat("0", 400) + "1"}, []string{"0", "-000.00", "-0." + strings.Repeat("0", 400) + "1"}, "minExclusive"},
		{"layered", []string{"1.0000000000000001", "1.0000000000000002"}, []string{"1.0000000000000003"}, "maxInclusive"},
		{"enum", []string{"1.25", "01.250", "+0001.2500", " \t1.25\n", "0", "+000", "-0.00", ".0"}, []string{"1.2500000000000001", "1"}, "enumeration"},
		{"pattern", []string{"01.25", "00.00"}, []string{"1.25", "+01.25", "01.250"}, "pattern"},
	} {
		for _, value := range test.accepted {
			add(test.element+"/accept/"+value, test.element, "", value, len(value) < 30)
		}
		for _, value := range test.rejected {
			add(test.element+"/reject/"+value, test.element, "", value, len(value) < 30, ":"+test.constraint)
		}
	}
	add("layered inherited", "layered", "", "1", true, ":minInclusive")
	for _, element := range []string{"fixed", "scalar"} {
		for _, value := range []string{"", "1.25", "+0001.2500", " \t1.250\n"} {
			// libxml2 2.9.x compares explicit element-fixed decimal spellings lexically.
			add(element+"/equal/"+value, element, "", value, value == "" || value == "+0001.2500")
		}
		add(element+"/different", element, "", "1.2500000000000001", true, ":fixed")
	}
	for _, value := range []string{"0", "+0.0", "-000.00", ".000"} {
		add("zero-fixed/"+value, "zero-fixed", "", value, value == "-000.00")
	}
	add("fixed invalid", "fixed", "", "invalid", true, ":fixed", ":datatype")
	add("fixed child", "fixed", "", "<decimal>1.25</decimal>", true, ":fixed", ":simple-content")
	add("fixed whitespace is present", "fixed", "", " \t ", true, ":fixed", ":datatype")
	add("fixed nil false", "fixed", ` n:nil="false"`, "1.25", false)
	add("fixed nil", "fixed", ` n:nil="true"`, "", true, ":fixed")
	add("default", "default", "", "", true)
	add("attribute fixed equivalent", "attributes", ` locked="1.2500" number="1.0000000000000001"`, "0", true)
	add("attribute fixed different", "attributes", ` locked="1.2500000000000001"`, "0", true, "/@locked:fixed")
	add("attribute exact bound", "attributes", ` number="1"`, "0", true, "/@number:minInclusive")
	add("attribute whitespace reference", "attributes", ` locked="&#x9;+0001.250&#xA;"`, "0", true)
	add("list members accepted", "list", "", "1.0000000000000001 2", true)
	add("list member exact bound", "list", "", "1 2", true, ":minInclusive")
	add("union decimal member", "union", "", "1.0000000000000001", true)
	add("union string member", "union", "", "none", true)
	add("union decimal bound", "union", "", "1", true, ":union")
	return tests
}

func TestValidateDecimalComparisons(t *testing.T) {
	t.Parallel()
	for _, test := range validationDecimalCases() {
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationDecimalGenerated, test.source), test.issues)
		})
	}
}

func TestDecimalComparisonsAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	directory, err := filepath.Abs(filepath.Join("testdata", "validation"))
	require.NoError(t, err)
	for _, test := range validationDecimalCases() {
		if !test.oracleComparable {
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationDecimalGenerated, test.source), test.issues)
			assertXMLLintOutcome(t, xmllint, filepath.Join(directory, "decimal-contract.xsd"), filepath.Join(directory, "catalog.xml"), test.source, len(test.issues) == 0)
		})
	}
	for _, value := range []string{"0.000000000000000001", "1.0000000000000001", "0", "-0.0", "-0.01", "1e0"} {
		t.Run("divisions/"+value, func(t *testing.T) {
			source := validationDecimalScore(value)
			var issues []string
			if value == "1e0" {
				issues = []string{"/score-partwise/part/measure/attributes/divisions:datatype"}
			} else if value[0] == '-' || value == "0" {
				issues = []string{"/score-partwise/part/measure/attributes/divisions:minExclusive"}
			}
			assertValidationIssues(t, validateAttributeSource(t, &scoreValidationSchema, source), issues)
			assertXMLLintOutcome(t, xmllint, filepath.Join("schema", "musicxml-4.0", "musicxml.xsd"), filepath.Join("schema", "musicxml-4.0", "catalog.xml"), source, len(issues) == 0)
		})
	}
}

func validationDecimalScore(value string) string {
	return `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Music</part-name></score-part></part-list><part id="P1"><measure number="1"><attributes><divisions>` + value + `</divisions></attributes></measure></part></score-partwise>`
}

func TestDecimalSourceAndModelConversion(t *testing.T) {
	t.Parallel()
	source := validationDecimalScore(strings.Repeat("9", 310))
	assert.Empty(t, validateAttributeSource(t, &scoreValidationSchema, source).issues)
	document, err := Decode(strings.NewReader(source))
	require.Error(t, err)
	assert.Nil(t, document)
	var numberError *strconv.NumError
	require.ErrorAs(t, err, &numberError)
	assert.ErrorIs(t, numberError, strconv.ErrRange)
	var validationError *ValidationError
	assert.False(t, errors.As(err, &validationError))
	// Underflow remains the existing typed transport behavior; source assessment
	// must still recognize the strictly positive decimal before information is lost.
	tiny := validationDecimalScore("0." + strings.Repeat("0", 400) + "1")
	assert.Empty(t, validateAttributeSource(t, &scoreValidationSchema, tiny).issues)
}

func TestValidateDecimalArbitraryWidthBounds(t *testing.T) {
	t.Parallel()
	for _, width := range []int{25, 100, 310, 4096} {
		for _, parts := range [][3]string{
			{strings.Repeat("9", width), "1" + strings.Repeat("0", width), "1" + strings.Repeat("0", width-1) + "1"},
			{"0." + strings.Repeat("0", width) + "1", "0." + strings.Repeat("0", width) + "2", "0." + strings.Repeat("0", width) + "3"},
		} {
			for _, negative := range []bool{false, true} {
				lower, equal, upper := parts[0], parts[1], parts[2]
				if negative {
					lower, equal, upper = "-"+upper, "-"+equal, "-"+lower
				}
				for _, test := range []struct {
					name     string
					schema   validationSimpleSchema
					accepted []bool
				}{
					{"minInclusive", validationSimpleSchema{HasMinInclusive: true, MinInclusive: equal}, []bool{false, true, true}},
					{"maxInclusive", validationSimpleSchema{HasMaxInclusive: true, MaxInclusive: equal}, []bool{true, true, false}},
					{"minExclusive", validationSimpleSchema{HasMinExclusive: true, MinExclusive: equal}, []bool{false, false, true}},
					{"maxExclusive", validationSimpleSchema{HasMaxExclusive: true, MaxExclusive: equal}, []bool{true, false, false}},
				} {
					for i, value := range []string{lower, equal, upper} {
						t.Run(fmt.Sprintf("%d/%t/%s/%d/%d", width, negative, test.name, len(equal), i), func(t *testing.T) {
							failure := validateBounds(&test.schema, value, "decimal")
							if test.accepted[i] {
								assert.Nil(t, failure)
							} else if assert.NotNil(t, failure) {
								assert.Equal(t, test.name, failure.constraint)
							}
						})
					}
				}
			}
		}
	}
}

func TestDecimalArbitraryWidthEquality(t *testing.T) {
	t.Parallel()
	context := &validationContext{schema: &validationDecimalGenerated}
	reference := &validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "decimal"}}
	for _, value := range []string{strings.Repeat("9", 310) + ".125", "0." + strings.Repeat("0", 400) + "1"} {
		schema := &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Name: reference.Name}, Enumerations: []string{value}}
		equivalent := "+000" + value + "00"
		different := value + "1"
		assert.Nil(t, context.validateSimple(schema, equivalent))
		failure := context.validateSimple(schema, different)
		if assert.NotNil(t, failure) {
			assert.Equal(t, "enumeration", failure.constraint)
		}
		assert.True(t, context.simpleValuesEqual(reference, value, equivalent))
		assert.True(t, context.elementFixedValuesEqual(reference, value, equivalent))
		assert.False(t, context.simpleValuesEqual(reference, value, different))
		assert.False(t, context.elementFixedValuesEqual(reference, value, different))
	}
}
