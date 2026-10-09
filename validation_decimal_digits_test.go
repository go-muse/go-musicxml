package musicxml

import (
	"fmt"
	"math/big"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Keep these source regressions independent of decimal implementation helpers so
// the same fixture and tests can demonstrate the pre-repair failures.
func validationDecimalDigitCases() []validationSourceCase {
	var tests []validationSourceCase
	add := func(name, element, attrs, value string, comparable bool, constraints ...string) {
		var issues []string
		for _, constraint := range constraints {
			issues = append(issues, "/"+element+constraint)
		}
		tests = append(tests, validationSourceCase{name: name, source: wrapValidationElement(element, attrs, value), issues: issues, oracleComparable: comparable})
	}
	for _, test := range []struct {
		element, constraint string
		accepted, rejected  []string
	}{
		{"total-one", "totalDigits", []string{"0", "-000.000", "+0009.000", ".1", "0.10", " \t+000.1000\r\n"}, []string{"10", "1.1", ".01", "0.001", "-.00100"}},
		{"total-three", "totalDigits", []string{"999", "999.000", "99.9", "9.99", ".999", ".001", "000.001000", "-000.01000", "100.0"}, []string{"1000", "99.99", "0.0001", "-000.0001000"}},
		{"fraction-zero", "fractionDigits", []string{"0", "+000.000", "-00100.000", "1.", strings.Repeat("9", 310) + ".000"}, []string{".1", "1.01", "-000.0001"}},
		{"fraction-two", "fractionDigits", []string{"0", "+000.000", "1.23000", "0.0100", "-000.10000", "123456.1200"}, []string{".001", "1.00100", "-000.01001"}},
		{"digits", "fractionDigits", []string{"0.00000", "001.23000", "99.900", "999.000", "000.01000"}, []string{".001", "-.999"}},
		{"digits-layered", "totalDigits", []string{"0.000", "001.2000", ".12000", "000.01000", "99.000"}, []string{"123", "1.23000"}},
		{"digits-pattern", "pattern", []string{"01.200", "00.010"}, []string{"1.2", "01.20", "+01.200", "01.2000"}},
		{"digits-enum", "enumeration", []string{"1.2", "+0001.20000"}, []string{"1.21"}},
	} {
		for _, value := range test.accepted {
			add(test.element+"/accept/"+value, test.element, "", value, len(value) < 30)
		}
		for _, value := range test.rejected {
			add(test.element+"/reject/"+value, test.element, "", value, true, ":"+test.constraint)
		}
	}
	add("combined total first", "digits", "", "1.234", true, ":totalDigits")
	add("layered inherited fraction", "digits-layered", "", ".001", true, ":fractionDigits")
	for _, value := range []string{"", " ", ".", "1e0", "NaN", "INF", "١", "１", "1 0", "\u00a01", "1\u0085"} {
		add("lexical/"+value, "digits", "", value, true, ":datatype")
	}
	for _, element := range []string{"digits-default", "digits-fixed"} {
		add(element+"/empty", element, "", "", true)
		add(element+"/explicit", element, "", "+001.2000", true)
	}
	add("fixed alternate", "digits-fixed", "", "1.20000", false) // libxml2 uses lexical element-fixed equality.
	add("simple nil", "digits", ` n:nil="true"`, "", true)
	add("simple nil false", "digits", ` n:nil="false"`, "001.2300", true)
	add("simple child", "digits", "", "<decimal>1.2</decimal>", true, ":simple-content")
	add("scalar inherited facets", "digits-scalar", "", "001.2300", true)
	add("scalar fraction rejected", "digits-scalar", "", ".001", true, ":fractionDigits")
	add("scalar total rejected", "digits-scalar", "", "1000", true, ":totalDigits")
	add("attribute padding", "digits-scalar", ` total="000.00100" fraction="1.23000" digits="&#x9;+001.2000&#xA;"`, "0.0", true)
	add("attribute total rejected", "digits-scalar", ` total="0.0001"`, "0", true, "/@total:totalDigits")
	add("attribute fraction rejected", "digits-scalar", ` fraction="1.00100"`, "0", true, "/@fraction:fractionDigits")
	add("attribute combined rejected", "digits-scalar", ` digits=".001"`, "0", true, "/@digits:fractionDigits")
	add("nil still checks attributes", "digits-scalar", ` n:nil="true" fraction=".001"`, "", true, "/@fraction:fractionDigits")
	add("list items", "digits-list", "", "001.23000 -000.01000 0.000", true)
	add("list total rejected", "digits-list", "", "1.2 1000", true, ":totalDigits")
	add("list fraction rejected", "digits-list", "", "1.2 .001", true, ":fractionDigits")
	add("union decimal member", "digits-union", "", "001.2000", true)
	add("union string member", "digits-union", "", "none", true)
	add("union no member", "digits-union", "", ".001", true, ":union")
	return tests
}

func TestValidateDecimalDigitFacets(t *testing.T) {
	t.Parallel()
	for _, test := range validationDecimalDigitCases() {
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationDecimalGenerated, test.source), test.issues)
		})
	}
}

func TestDecimalDigitFacetsAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	directory, err := filepath.Abs(filepath.Join("testdata", "validation"))
	require.NoError(t, err)
	for _, test := range validationDecimalDigitCases() {
		if !test.oracleComparable {
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationDecimalGenerated, test.source), test.issues)
			assertXMLLintOutcome(t, xmllint, filepath.Join(directory, "decimal-contract.xsd"), filepath.Join(directory, "catalog.xml"), test.source, len(test.issues) == 0)
		})
	}
}

func TestDecimalDigitFacetArbitraryWidths(t *testing.T) {
	t.Parallel()
	context := newTestValidationContext(&validationDecimalGenerated)
	for _, width := range []int{25, 310, 4096} {
		for _, test := range []struct {
			value           string
			total, fraction uint64
		}{
			{strings.Repeat("9", width) + ".1200", uint64(width + 2), 2},
			{"0." + strings.Repeat("0", width) + "100", uint64(width + 1), uint64(width + 1)},
			{"1" + strings.Repeat("0", width) + ".000", uint64(width + 1), 0},
			{strings.Repeat("0", width) + "." + strings.Repeat("0", width), 1, 0},
		} {
			for _, sign := range []string{"", "+", "-"} {
				value := " \t" + sign + "000" + test.value + "\r\n"
				t.Run(fmt.Sprintf("%d/%d/%d/%s", width, test.total, test.fraction, sign), func(t *testing.T) {
					schema := &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: "decimal"}}, HasTotalDigits: true, TotalDigits: test.total, HasFractionDigits: true, FractionDigits: test.fraction}
					assert.Nil(t, context.validateSimple(schema, value))
					if test.total > 1 {
						schema.TotalDigits--
						failure := context.validateSimple(schema, value)
						if assert.NotNil(t, failure) {
							assert.Equal(t, "totalDigits", failure.constraint)
						}
						schema.TotalDigits++
					}
					if test.fraction > 0 {
						schema.FractionDigits--
						failure := context.validateSimple(schema, value)
						if assert.NotNil(t, failure) {
							assert.Equal(t, "fractionDigits", failure.constraint)
						}
					}
				})
			}
		}
	}
}

// Use a reduced rational and integer powers, independent of the production
// decimal digit views. XSD 1.0 limits both coefficient magnitude and scale.
func decimalDigitCountsFromRational(t *testing.T, value string) (uint64, uint64) {
	t.Helper()
	number, ok := new(big.Rat).SetString(value)
	require.True(t, ok)
	denominator := new(big.Int).Set(number.Denom())
	var exponents [2]uint64
	for index, factor := range []int64{2, 5} {
		divisor := big.NewInt(factor)
		for new(big.Int).Mod(denominator, divisor).Sign() == 0 {
			denominator.Quo(denominator, divisor)
			exponents[index]++
		}
	}
	require.Equal(t, int64(1), denominator.Int64())
	scale := max(exponents[0], exponents[1])
	power := new(big.Int).Exp(big.NewInt(10), new(big.Int).SetUint64(scale), nil)
	coefficient := new(big.Int).Mul(number.Num(), power.Quo(power, number.Denom()))
	coefficient.Abs(coefficient)
	return max(uint64(len(coefficient.String())), scale), scale
}

func TestDecimalDigitFacetsRationalOracle(t *testing.T) {
	t.Parallel()
	random := rand.New(rand.NewPCG(29, 71))
	for index := range 1000 {
		var value strings.Builder
		if index%2 == 0 {
			value.WriteByte('-')
		}
		for range random.IntN(10) + 1 {
			value.WriteByte(byte('0' + random.IntN(10)))
		}
		whole := value.String()
		value.WriteByte('.')
		for range random.IntN(20) + 1 {
			value.WriteByte(byte('0' + random.IntN(10)))
		}
		value.WriteString(strings.Repeat("0", random.IntN(10)))
		// Preserve the original fractional stream and also exercise scale-zero
		// values, optional trailing points, and coefficient growth from whole zeros.
		wholeZeros := whole + strings.Repeat("0", index%10+1)
		for _, literal := range []string{value.String(), whole, whole + ".", wholeZeros, wholeZeros + "."} {
			total, fraction := decimalDigitCountsFromRational(t, literal)
			limitsToCheck := [][2]uint64{{total, fraction}, {total + 1, fraction + 1}}
			if total > 1 {
				limitsToCheck = append(limitsToCheck, [2]uint64{total - 1, fraction})
			}
			for _, limits := range limitsToCheck {
				schema := &validationSimpleSchema{HasTotalDigits: true, TotalDigits: limits[0], HasFractionDigits: true, FractionDigits: limits[1]}
				failure := validateDigitFacets(schema, literal, "decimal")
				if limits[0] < total {
					if assert.NotNil(t, failure, "%q", literal) {
						assert.Equal(t, "totalDigits", failure.constraint)
					}
				} else {
					assert.Nil(t, failure, "%q", literal)
				}
			}
			if fraction > 0 {
				failure := validateDigitFacets(&validationSimpleSchema{HasFractionDigits: true, FractionDigits: fraction - 1}, literal, "decimal")
				if assert.NotNil(t, failure, "%q", literal) {
					assert.Equal(t, "fractionDigits", failure.constraint)
				}
			}
		}
	}
}

func TestDecimalDigitFacetDiagnosticsAndImmutability(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ value, constraint, message string }{
		{"+000.0100", "totalDigits", "value has 2 digits; maximum is 1"},
		{"-0001.2300", "fractionDigits", "value has 2 fraction digits; maximum is 1"},
	} {
		schema := &validationSimpleSchema{HasTotalDigits: test.constraint == "totalDigits", TotalDigits: 1, HasFractionDigits: test.constraint == "fractionDigits", FractionDigits: 1}
		before := *schema
		failure := validateDigitFacets(schema, test.value, "decimal")
		if assert.NotNil(t, failure) {
			assert.Equal(t, test.constraint, failure.constraint)
			assert.Equal(t, test.message, failure.message)
		}
		assert.Equal(t, before, *schema)
	}
	root, err := parseValidationDocument([]byte(`<digits-scalar digits="+001.2000"> +001.2300 </digits-scalar>`))
	require.NoError(t, err)
	before := root.Text.String()
	declaration := validationDecimalGenerated.Elements[validationQName{Local: "digits-scalar"}]
	context := newTestValidationContext(&validationDecimalGenerated)
	context.validateElement(root, declaration, "/digits-scalar")
	assert.Empty(t, context.issues)
	assert.Equal(t, before, root.Text.String())
	assert.Equal(t, "+001.2000", root.Attrs[0].Value)
}

func TestDecimalDigitFacetsConstantSpace(t *testing.T) {
	// Keep nonparallel because AllocsPerRun observes process-wide allocations.
	value := "-" + strings.Repeat("0", 1<<18) + "1." + strings.Repeat("0", 1<<18) + "1" + strings.Repeat("0", 1<<19)
	schema := &validationSimpleSchema{HasTotalDigits: true, TotalDigits: 1<<18 + 2, HasFractionDigits: true, FractionDigits: 1<<18 + 1}
	assert.Zero(t, testing.AllocsPerRun(5, func() {
		if failure := validateDigitFacets(schema, value, "decimal"); failure != nil {
			panic(failure.message)
		}
	}))
}

func TestDecimalDigitFacetDispatchScope(t *testing.T) {
	t.Parallel()
	// No facet means no new lexical check; ordinary validation already checks
	// the base datatype before reaching this helper.
	assert.Nil(t, validateDigitFacets(&validationSimpleSchema{}, "invalid", "decimal"))
	for _, value := range []string{"", ".", "1e0", "NaN", "١", "\u00a01"} {
		failure := validateDigitFacets(&validationSimpleSchema{HasTotalDigits: true, TotalDigits: 3}, value, "decimal")
		if assert.NotNil(t, failure) {
			assert.Equal(t, "datatype", failure.constraint)
		}
	}
	for _, builtin := range []string{"integer", "positiveInteger", "unsignedLong"} {
		assert.Nil(t, validateDigitFacets(&validationSimpleSchema{HasTotalDigits: true, TotalDigits: 1, HasFractionDigits: true}, "+0001", builtin))
	}
	// Unsupported nondecimal facet combinations retain their lexical fallback;
	// this repair does not add schema-component validation or float semantics.
	for _, builtin := range []string{"string", "token", "float", "double", ""} {
		failure := validateDigitFacets(&validationSimpleSchema{HasFractionDigits: true}, "1.000", builtin)
		if assert.NotNil(t, failure, builtin) {
			assert.Equal(t, "fractionDigits", failure.constraint)
		}
	}
	assert.Nil(t, validateDigitFacets(&validationSimpleSchema{HasFractionDigits: true}, "1.000", "decimal"))
	assert.Nil(t, validateDigitFacets(&validationSimpleSchema{HasTotalDigits: true, TotalDigits: ^uint64(0), HasFractionDigits: true, FractionDigits: ^uint64(0)}, "1.234", "decimal"))
}
