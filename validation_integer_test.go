package musicxml

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These are the XSD 1.0 domains, not the corresponding Go model types.
// https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#integer
var validationIntegerDomains = []struct{ name, min, max string }{
	{"integer", "", ""}, {"nonPositiveInteger", "", "0"}, {"negativeInteger", "", "-1"},
	{"long", "-9223372036854775808", "9223372036854775807"}, {"int", "-2147483648", "2147483647"},
	{"short", "-32768", "32767"}, {"byte", "-128", "127"},
	{"nonNegativeInteger", "0", ""}, {"positiveInteger", "1", ""},
	{"unsignedLong", "0", "18446744073709551615"}, {"unsignedInt", "0", "4294967295"},
	{"unsignedShort", "0", "65535"}, {"unsignedByte", "0", "255"},
}

func validationIntegerBuiltinCases() []validationSourceCase {
	values := []string{"0", "+0", "-0", "-000", "+001", "-001", "1", "-1", " \t+0001\r\n", "", " ", "+", "-", "++1", "+-0", "1.0", "1.", ".1", "1e0", "0x1", "1_0", "1 0", "1\t0", "١", "１", "\u00a01", "1\u0085", "\u20031", strings.Repeat("9", 310), "-" + strings.Repeat("9", 310)}
	for _, domain := range validationIntegerDomains {
		for _, bound := range []string{domain.min, domain.max} {
			if bound == "" {
				continue
			}
			number, _ := new(big.Int).SetString(bound, 10)
			for _, delta := range []int64{-1, 0, 1} {
				values = append(values, new(big.Int).Add(number, big.NewInt(delta)).String())
			}
		}
	}
	var tests []validationSourceCase
	for _, domain := range validationIntegerDomains {
		for index, value := range values {
			normalized := strings.Trim(value, " \t\r\n")
			number, valid := new(big.Int).SetString(normalized, 10)
			if valid {
				// math/big is independent of production parsing; reject no extra spellings here.
				for _, bound := range []struct {
					value   string
					minimum bool
				}{{domain.min, true}, {domain.max, false}} {
					if bound.value == "" {
						continue
					}
					limit, _ := new(big.Int).SetString(bound.value, 10)
					if (bound.minimum && number.Cmp(limit) < 0) || (!bound.minimum && number.Cmp(limit) > 0) {
						valid = false
					}
				}
			}
			var issues []string
			if !valid {
				issues = []string{"/" + domain.name + ":datatype"}
			}
			tests = append(tests, validationSourceCase{name: fmt.Sprintf("%s/%d", domain.name, index), source: "<" + domain.name + ">" + value + "</" + domain.name + ">", issues: issues, oracleComparable: !(valid && (len(normalized) > 24 ||
				(strings.HasPrefix(domain.name, "unsigned") && strings.ContainsAny(normalized, "+-")) ||
				((domain.name == "long" || domain.name == "int" || domain.name == "short" || domain.name == "byte") && strings.Contains(value, "\t"))))})
		}
	}
	return tests
}

func validationIntegerFacetCases() []validationSourceCase {
	var tests []validationSourceCase
	add := func(name, element, attrs, value string, constraints ...string) {
		var issues []string
		for _, constraint := range constraints {
			issues = append(issues, "/"+element+constraint)
		}
		tests = append(tests, validationSourceCase{name: name, source: wrapValidationElement(element, attrs, value), issues: issues, oracleComparable: !(len(value) > 100 ||
			((element == "fixed" || element == "scalar") && len(issues) == 0 && value != "" && value != "+00018446744073709551616"))})
	}
	for _, test := range []struct {
		element    string
		accepted   []string
		rejected   []string
		constraint string
	}{
		{"min", []string{"9007199254740993", "9007199254740994"}, []string{"9007199254740992"}, "minInclusive"},
		{"max", []string{"9007199254740992", "9007199254740993"}, []string{"9007199254740994"}, "maxInclusive"},
		{"min-exclusive", []string{"9007199254740994"}, []string{"9007199254740992", "9007199254740993"}, "minExclusive"},
		{"max-exclusive", []string{"9007199254740992"}, []string{"9007199254740993", "9007199254740994"}, "maxExclusive"},
		{"negative-max", []string{"-9007199254740994", "-9007199254740993"}, []string{"-9007199254740992"}, "maxInclusive"},
		{"huge-min", []string{"18446744073709551616", strings.Repeat("9", 310)}, []string{"18446744073709551615"}, "minInclusive"},
		{"enum", []string{"18446744073709551616", "+00018446744073709551616", " \t18446744073709551616\n", "0", "+000", "-000"}, []string{"18446744073709551617", "1"}, "enumeration"},
		{"digits", []string{"0001", "-0001", "+000999", "-0", "000"}, []string{"1000", "-1000"}, "totalDigits"},
		{"pattern", []string{"01", "09"}, []string{"1", "001", "+01"}, "pattern"},
		{"layered", []string{"18446744073709551616", "18446744073709551617"}, []string{"18446744073709551618"}, "maxInclusive"},
	} {
		for _, value := range test.accepted {
			add(test.element+"/accept/"+value, test.element, "", value)
		}
		for _, value := range test.rejected {
			add(test.element+"/reject/"+value, test.element, "", value, ":"+test.constraint)
		}
	}
	add("layered inherited minimum", "layered", "", "18446744073709551615", ":minInclusive")
	for _, element := range []string{"fixed", "scalar"} {
		for _, value := range []string{"", "18446744073709551616", "+00018446744073709551616", " \t18446744073709551616\n"} {
			add(element+"/equal/"+value, element, "", value)
		}
	}
	add("fixed different", "fixed", "", "18446744073709551617", ":fixed")
	add("fixed invalid", "fixed", "", "invalid", ":fixed", ":datatype")
	add("fixed child", "fixed", "", "<integer>18446744073709551616</integer>", ":fixed", ":simple-content")
	add("fixed nil", "fixed", ` n:nil="true"`, "", ":fixed")
	add("default exact", "default", "", "")
	add("attribute fixed without element fixed", "attributes", ` locked="18446744073709551616" number="-18446744073709551616"`, "18446744073709551616")
	add("attribute fixed mismatch without element fixed", "attributes", ` locked="18446744073709551617"`, "18446744073709551616", "/@locked:fixed")
	add("attribute equal", "scalar", ` locked="18446744073709551616" number="-18446744073709551616"`, "18446744073709551616")
	add("attribute different", "scalar", ` locked="18446744073709551617"`, "18446744073709551616", "/@locked:fixed")
	add("list arbitrary width", "list", "", "18446744073709551616 -18446744073709551616 000")
	add("list lexical", "list", "", "18446744073709551616 1.0", ":datatype")
	add("union arbitrary width", "union", "", "18446744073709551616")
	add("union nonnumeric", "union", "", "none")
	add("union invalid", "union", "", "-1", ":union")
	return tests
}

func TestValidateIntegerValueSpaces(t *testing.T) {
	t.Parallel()
	for _, test := range append(validationIntegerBuiltinCases(), validationIntegerFacetCases()...) {
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationIntegerGenerated, test.source), test.issues)
		})
	}
}

func TestIntegerValueSpacesAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	directory, err := filepath.Abs(filepath.Join("testdata", "validation"))
	require.NoError(t, err)
	for _, test := range append(validationIntegerBuiltinCases(), validationIntegerFacetCases()...) {
		if !test.oracleComparable {
			// libxml2 2.9.14 limits integer precision and disagrees on certain
			// inherited lexical forms and numeric fixed equivalence. These
			// remain internal normative/compatibility tests, not parity claims.
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationIntegerGenerated, test.source), test.issues)
			assertXMLLintOutcome(t, xmllint, filepath.Join(directory, "integer-contract.xsd"), filepath.Join(directory, "catalog.xml"), test.source, len(test.issues) == 0)
		})
	}
	for _, value := range []string{"18446744073709551615", "18446744073709551616", "-1", "1.0"} {
		source := validationIntegerScore(value)
		t.Run("staves/"+value, func(t *testing.T) {
			valid := value[0] != '-' && !strings.Contains(value, ".")
			var issues []string
			if !valid {
				issues = []string{"/score-partwise/part/measure/attributes/staves:datatype"}
			}
			assertValidationIssues(t, validateAttributeSource(t, &scoreValidationSchema, source), issues)
			assertXMLLintOutcome(t, xmllint, filepath.Join("schema", "musicxml-4.0", "musicxml.xsd"), filepath.Join("schema", "musicxml-4.0", "catalog.xml"), source, value[0] != '-' && !strings.Contains(value, "."))
		})
	}
}

func validationIntegerScore(value string) string {
	return `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Music</part-name></score-part></part-list><part id="P1"><measure number="1"><attributes><staves>` + value + `</staves></attributes></measure></part></score-partwise>`
}

func TestIntegerSourceAndModelConversion(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"18446744073709551615", "18446744073709551616"} {
		source := validationIntegerScore(value)
		assert.Empty(t, validateAttributeSource(t, &scoreValidationSchema, source).issues)
		document, err := Decode(strings.NewReader(source))
		if value == "18446744073709551616" {
			require.Error(t, err)
			assert.Nil(t, document)
			var numberError *strconv.NumError
			require.ErrorAs(t, err, &numberError)
			assert.ErrorIs(t, numberError, strconv.ErrRange)
			var validationError *ValidationError
			assert.False(t, errors.As(err, &validationError))
		} else {
			require.NoError(t, err)
			assert.NoError(t, Validate(document))
		}
	}
}

func TestValidationIntegerExactOperations(t *testing.T) {
	// Deterministic big.Int comparisons independently check sign, unequal lengths,
	// leading zeros and near-equal magnitudes far beyond floating-point range.
	values := []string{"0", "-000", "+001", "-1", "9007199254740992", "9007199254740993", "18446744073709551616"}
	for _, width := range []int{24, 25, 100, 310, 4096} {
		for _, sign := range []string{"", "-"} {
			values = append(values, sign+strings.Repeat("9", width), sign+"1"+strings.Repeat("0", width), sign+"000"+strings.Repeat("8", width))
		}
	}
	for _, left := range values {
		first, ok := parseValidationInteger(left)
		require.True(t, ok)
		wantFirst, ok := new(big.Int).SetString(left, 10)
		require.True(t, ok)
		for _, right := range values {
			second, ok := parseValidationInteger(right)
			require.True(t, ok)
			wantSecond, ok := new(big.Int).SetString(right, 10)
			require.True(t, ok)
			assert.Equal(t, wantFirst.Cmp(wantSecond), first.compare(second))
		}
	}
	for _, invalid := range []string{"", "+", "-", "++1", "+-0", "1 0", "1.0", "1e0", "\u00a01", "١", "1\xff"} {
		_, ok := parseValidationInteger(invalid)
		assert.False(t, ok, "%q", invalid)
	}
	value := "-" + strings.Repeat("0", 10000) + strings.Repeat("9", 10000)
	assert.Zero(t, testing.AllocsPerRun(10, func() {
		first, ok := parseValidationInteger(value)
		if !ok || first.compare(first) != 0 {
			panic("unexpected integer comparison")
		}
	}))
}

func TestValidateIntegerArbitraryWidthFacets(t *testing.T) {
	t.Parallel()
	context := &validationContext{schema: &validationIntegerGenerated}
	for _, width := range []int{25, 100, 310, 4096} {
		bound := "1" + strings.Repeat("0", width)
		below := strings.Repeat("9", width)
		above := bound[:len(bound)-1] + "1"
		for _, negative := range []bool{false, true} {
			lower, equal, upper := below, bound, above
			if negative {
				lower, equal, upper = "-"+above, "-"+bound, "-"+below
			}
			for _, facet := range []string{"minInclusive", "maxInclusive", "minExclusive", "maxExclusive"} {
				schema := &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: "integer"}}}
				switch facet {
				case "minInclusive":
					schema.HasMinInclusive = true
					schema.MinInclusive = equal
				case "maxInclusive":
					schema.HasMaxInclusive = true
					schema.MaxInclusive = equal
				case "minExclusive":
					schema.HasMinExclusive = true
					schema.MinExclusive = equal
				case "maxExclusive":
					schema.HasMaxExclusive = true
					schema.MaxExclusive = equal
				}
				for index, value := range []string{lower, equal, upper} {
					valid := (strings.HasPrefix(facet, "min") && index > 1) || (strings.HasPrefix(facet, "max") && index < 1) || (strings.HasSuffix(facet, "Inclusive") && index == 1)
					failure := context.validateSimple(schema, value)
					if valid {
						assert.Nil(t, failure)
					} else {
						require.NotNil(t, failure)
						assert.Equal(t, facet, failure.constraint)
					}
				}
			}
		}
	}
	for _, facet := range []string{"minInclusive", "maxInclusive", "minExclusive", "maxExclusive"} {
		schema := &validationSimpleSchema{}
		switch facet {
		case "minInclusive":
			schema.HasMinInclusive = true
			schema.MinInclusive = "1.0"
		case "maxInclusive":
			schema.HasMaxInclusive = true
			schema.MaxInclusive = "NaN"
		case "minExclusive":
			schema.HasMinExclusive = true
			schema.MinExclusive = ""
		case "maxExclusive":
			schema.HasMaxExclusive = true
			schema.MaxExclusive = "1e3"
		}
		failure := validateBounds(schema, "1", "integer")
		require.NotNil(t, failure)
		assert.Equal(t, "schema", failure.constraint)
	}
	// An integral-valued decimal remains decimal, not an integer lexical type.
	decimal := &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: "decimal"}}, HasMinInclusive: true, MinInclusive: "1.0"}
	assert.Nil(t, context.validateSimple(decimal, "1."))
}

func TestIntegerGeneratedLexemesRemainExact(t *testing.T) {
	t.Parallel()
	fixed := validationIntegerGenerated.Elements[validationQName{Local: "fixed"}]
	require.NotNil(t, fixed)
	require.NotNil(t, fixed.Fixed)
	assert.Equal(t, "+00018446744073709551616", *fixed.Fixed)
	enum := validationIntegerGenerated.Types[validationQName{Local: "enum-type"}]
	require.NotNil(t, enum)
	require.NotNil(t, enum.Simple)
	assert.Equal(t, []string{"+00018446744073709551616", "-000"}, enum.Simple.Enumerations)
	root, err := parseValidationDocument([]byte("<fixed> \t+00018446744073709551616 </fixed>"))
	require.NoError(t, err)
	before := root.Text.String()
	context := &validationContext{schema: &validationIntegerGenerated, effective: make(map[*validationComplexSchema]*validationEffectiveComplex), identifiers: make(map[string]string)}
	context.validateElement(root, fixed, "/fixed")
	assert.Empty(t, context.issues)
	assert.Equal(t, before, root.Text.String())
}
