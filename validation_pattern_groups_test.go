package musicxml

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Synthetic exact-source XSD 1.0 regressions: alternatives within a restriction,
// intersection across restriction layers, and unchanged lexical facet subjects.
// https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/#src-multiple-patterns
func validationPatternGroupCases() []validationSourceCase {
	return []validationSourceCase{
		{name: "choice/accept/0", source: "<choice xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">A1</choice>", oracleComparable: true, issues: nil},
		{name: "choice/accept/1", source: "<choice xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">B2</choice>", oracleComparable: true, issues: nil},
		{name: "layer/accept/0", source: "<layer xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">A1</layer>", oracleComparable: true, issues: nil},
		{name: "layer/accept/1", source: "<layer xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">A2</layer>", oracleComparable: true, issues: nil},
		{name: "layer/accept/2", source: "<layer xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">B1</layer>", oracleComparable: true, issues: nil},
		{name: "layer/accept/3", source: "<layer xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">B2</layer>", oracleComparable: true, issues: nil},
		{name: "empty/accept/0", source: "<empty xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\"></empty>", oracleComparable: true, issues: nil},
		{name: "empty/accept/1", source: "<empty xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">x</empty>", oracleComparable: true, issues: nil},
		{name: "collapse/accept/0", source: "<collapse xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">A B</collapse>", oracleComparable: true, issues: nil},
		{name: "collapse/accept/1", source: "<collapse xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\"> \tA  B\n</collapse>", oracleComparable: true, issues: nil},
		{name: "collapse/accept/2", source: "<collapse xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">C D</collapse>", oracleComparable: true, issues: nil},
		{name: "collapse/accept/3", source: "<collapse xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">\nC\t\tD\r</collapse>", oracleComparable: true, issues: nil},
		{name: "replace/accept/0", source: "<replace xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\"> A B </replace>", oracleComparable: true, issues: nil},
		{name: "replace/accept/1", source: "<replace xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">\tA\tB\n</replace>", oracleComparable: true, issues: nil},
		{name: "replace/accept/2", source: "<replace xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">\rC\tD\n</replace>", oracleComparable: true, issues: nil},
		{name: "numeric/accept/0", source: "<numeric xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">01.20</numeric>", oracleComparable: true, issues: nil},
		{name: "numeric/accept/1", source: "<numeric xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">+1.2</numeric>", oracleComparable: true, issues: nil},
		{name: "numeric/accept/2", source: "<numeric xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\"> \t01.20\n</numeric>", oracleComparable: true, issues: nil},
		{name: "integer/accept/0", source: "<integer xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">01</integer>", oracleComparable: true, issues: nil},
		{name: "integer/accept/1", source: "<integer xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">+1</integer>", oracleComparable: true, issues: nil},
		{name: "integer/accept/2", source: "<integer xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">\n+1\t</integer>", oracleComparable: true, issues: nil},
		{name: "inline/accept/0", source: "<inline xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">A1</inline>", oracleComparable: true, issues: nil},
		{name: "inline/accept/1", source: "<inline xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">B2</inline>", oracleComparable: true, issues: nil},
		{name: "list/accept/0", source: "<list xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">01 2</list>", oracleComparable: true, issues: nil},
		{name: "list/accept/1", source: "<list xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">3 04</list>", oracleComparable: true, issues: nil},
		{name: "list/accept/2", source: "<list xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\"> \t01\n 2\r</list>", oracleComparable: true, issues: nil},
		{name: "list-items/accept/0", source: "<list-items xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">01 +1</list-items>", oracleComparable: true, issues: nil},
		{name: "list-items/accept/1", source: "<list-items xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">+1 01</list-items>", oracleComparable: true, issues: nil},
		{name: "union/accept/0", source: "<union xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">01</union>", oracleComparable: true, issues: nil},
		{name: "union/accept/1", source: "<union xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">none</union>", oracleComparable: true, issues: nil},
		{name: "union/accept/2", source: "<union xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\"> \t01\n</union>", oracleComparable: true, issues: nil},
		{name: "union/accept/3", source: "<union xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\"> \tnone\n</union>", oracleComparable: true, issues: nil},
		{name: "preserve-union/accept/0", source: "<preserve-union xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">01</preserve-union>", oracleComparable: true, issues: nil},
		{name: "preserve-union/accept/1", source: "<preserve-union xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">none</preserve-union>", oracleComparable: true, issues: nil},
		{name: "choice/reject/0", source: "<choice xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">C1</choice>", oracleComparable: true, issues: []string{"/choice:pattern"}},
		{name: "choice/reject/1", source: "<choice xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\"></choice>", oracleComparable: true, issues: []string{"/choice:pattern"}},
		{name: "choice/reject/2", source: "<choice xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\"> A1 </choice>", oracleComparable: true, issues: []string{"/choice:pattern"}},
		{name: "choice/reject/3", source: "<choice xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">A11</choice>", oracleComparable: true, issues: []string{"/choice:pattern"}},
		{name: "layer/reject/0", source: "<layer xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">C1</layer>", oracleComparable: true, issues: []string{"/layer:pattern"}},
		{name: "layer/reject/1", source: "<layer xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">A3</layer>", oracleComparable: true, issues: []string{"/layer:pattern"}},
		{name: "layer/reject/2", source: "<layer xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">C3</layer>", oracleComparable: true, issues: []string{"/layer:pattern"}},
		{name: "empty/reject/0", source: "<empty xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">y</empty>", oracleComparable: true, issues: []string{"/empty:pattern"}},
		{name: "empty/reject/1", source: "<empty xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">xx</empty>", oracleComparable: true, issues: []string{"/empty:pattern"}},
		{name: "collapse/reject/0", source: "<collapse xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">AB</collapse>", oracleComparable: true, issues: []string{"/collapse:pattern"}},
		{name: "collapse/reject/1", source: "<collapse xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">A\u00a0B</collapse>", oracleComparable: true, issues: []string{"/collapse:pattern"}},
		{name: "replace/reject/0", source: "<replace xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">A B</replace>", oracleComparable: true, issues: []string{"/replace:pattern"}},
		{name: "replace/reject/1", source: "<replace xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\"> A  B </replace>", oracleComparable: true, issues: []string{"/replace:pattern"}},
		{name: "numeric/reject/0", source: "<numeric xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">1.2</numeric>", oracleComparable: true, issues: []string{"/numeric:pattern"}},
		{name: "numeric/reject/1", source: "<numeric xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">1.20</numeric>", oracleComparable: true, issues: []string{"/numeric:pattern"}},
		{name: "numeric/reject/2", source: "<numeric xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">01.200</numeric>", oracleComparable: true, issues: []string{"/numeric:pattern"}},
		{name: "integer/reject/0", source: "<integer xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">1</integer>", oracleComparable: true, issues: []string{"/integer:pattern"}},
		{name: "integer/reject/1", source: "<integer xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">001</integer>", oracleComparable: true, issues: []string{"/integer:pattern"}},
		{name: "integer/reject/2", source: "<integer xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">+01</integer>", oracleComparable: true, issues: []string{"/integer:pattern"}},
		{name: "inline/reject/0", source: "<inline xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">C1</inline>", oracleComparable: true, issues: []string{"/inline:pattern"}},
		{name: "inline/reject/1", source: "<inline xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">A3</inline>", oracleComparable: true, issues: []string{"/inline:pattern"}},
		{name: "list/reject/0", source: "<list xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">1 2</list>", oracleComparable: true, issues: []string{"/list:pattern"}},
		{name: "list/reject/1", source: "<list xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">3 4</list>", oracleComparable: true, issues: []string{"/list:pattern"}},
		{name: "list/reject/2", source: "<list xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">01 2 3</list>", oracleComparable: true, issues: []string{"/list:pattern"}},
		{name: "list-items/reject/0", source: "<list-items xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">01 1</list-items>", oracleComparable: true, issues: []string{"/list-items:pattern"}},
		{name: "union/reject/0", source: "<union xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">1</union>", oracleComparable: true, issues: []string{"/union:pattern"}},
		{name: "union/reject/1", source: "<union xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">+01</union>", oracleComparable: true, issues: []string{"/union:pattern"}},
		{name: "preserve-union/reject/0", source: "<preserve-union xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\"> 01 </preserve-union>", oracleComparable: true, issues: []string{"/preserve-union:pattern"}},
		{name: "preserve-union/reject/1", source: "<preserve-union xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\"> none </preserve-union>", oracleComparable: true, issues: []string{"/preserve-union:pattern"}},
		{name: "numeric/invalid-base", source: "<numeric xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">NaN</numeric>", oracleComparable: true, issues: []string{"/numeric:datatype"}},
		{name: "union/no-member", source: "<union xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">bad</union>", oracleComparable: true, issues: []string{"/union:union"}},
		{name: "list/invalid-item", source: "<list xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">01 nope</list>", oracleComparable: true, issues: []string{"/list:datatype"}},
		{name: "attributes/first", source: "<attributes xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" choice=\"A1\"></attributes>", oracleComparable: true, issues: nil},
		{name: "attributes/last", source: "<attributes xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" choice=\"B2\"></attributes>", oracleComparable: true, issues: nil},
		{name: "attributes/normalize", source: "<attributes xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" choice=\"B2\" space=\"&#x9;C&#xA; D&#xD;\"></attributes>", oracleComparable: true, issues: nil},
		{name: "attributes/fixed-value-equal", source: "<attributes xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" choice=\"A1\" fixed=\"+1.2\"></attributes>", oracleComparable: true, issues: nil},
		{name: "attributes/none", source: "<attributes xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" choice=\"C1\"></attributes>", oracleComparable: true, issues: []string{"/attributes/@choice:pattern"}},
		{name: "attributes/space-none", source: "<attributes xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" choice=\"B2\" space=\"AB\"></attributes>", oracleComparable: true, issues: []string{"/attributes/@space:pattern"}},
		{name: "attributes/missing", source: "<attributes xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\"></attributes>", oracleComparable: true, issues: []string{"/attributes/@choice:required"}},
		{name: "scalar/accept/A1", source: "<scalar xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"B2\">A1</scalar>", oracleComparable: true, issues: nil},
		{name: "scalar/accept/B2", source: "<scalar xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"B2\">B2</scalar>", oracleComparable: true, issues: nil},
		{name: "scalar/reject/C1", source: "<scalar xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"B2\">C1</scalar>", oracleComparable: true, issues: []string{"/scalar:pattern"}},
		{name: "scalar/reject/A3", source: "<scalar xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"B2\">A3</scalar>", oracleComparable: true, issues: []string{"/scalar:pattern"}},
		{name: "complex-inline/accept/A1", source: "<complex-inline xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"B2\">A1</complex-inline>", oracleComparable: true, issues: nil},
		{name: "complex-inline/accept/B2", source: "<complex-inline xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"B2\">B2</complex-inline>", oracleComparable: true, issues: nil},
		{name: "complex-inline/reject/C1", source: "<complex-inline xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"B2\">C1</complex-inline>", oracleComparable: true, issues: []string{"/complex-inline:pattern"}},
		{name: "complex-inline/reject/A3", source: "<complex-inline xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"B2\">A3</complex-inline>", oracleComparable: true, issues: []string{"/complex-inline:pattern"}},
		{name: "complex-inline/local-reject", source: "<complex-inline xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"A1\">A2</complex-inline>", oracleComparable: true, issues: []string{"/complex-inline:pattern"}},
		{name: "scalar/attribute-and-scalar", source: "<scalar xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"C1\">C1</scalar>", oracleComparable: true, issues: []string{"/scalar/@code:pattern", "/scalar:pattern"}},
		{name: "scalar/required", source: "<scalar xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">B2</scalar>", oracleComparable: true, issues: []string{"/scalar/@code:required"}},
		{name: "default/empty", source: "<default xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\"></default>", oracleComparable: true, issues: nil},
		{name: "default/explicit", source: "<default xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">B2</default>", oracleComparable: true, issues: nil},
		{name: "fixed/empty", source: "<fixed xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\"></fixed>", oracleComparable: true, issues: nil},
		{name: "fixed/explicit", source: "<fixed xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">B2</fixed>", oracleComparable: true, issues: nil},
		{name: "scalar-default/empty", source: "<scalar-default xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"B2\"></scalar-default>", oracleComparable: true, issues: nil},
		{name: "scalar-default/explicit", source: "<scalar-default xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"B2\">B2</scalar-default>", oracleComparable: true, issues: nil},
		{name: "scalar-fixed/empty", source: "<scalar-fixed xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"B2\"></scalar-fixed>", oracleComparable: true, issues: nil},
		{name: "scalar-fixed/explicit", source: "<scalar-fixed xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"B2\">B2</scalar-fixed>", oracleComparable: true, issues: nil},
		{name: "fixed/mismatch", source: "<fixed xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">A1</fixed>", oracleComparable: true, issues: []string{"/fixed:fixed"}},
		{name: "fixed/mismatch-and-pattern", source: "<fixed xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">C1</fixed>", oracleComparable: true, issues: []string{"/fixed:fixed", "/fixed:pattern"}},
		{name: "fixed/nil", source: "<fixed xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" n:nil=\"true\"></fixed>", oracleComparable: true, issues: []string{"/fixed:fixed"}},
		{name: "scalar-fixed/mismatch", source: "<scalar-fixed xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"B2\">A1</scalar-fixed>", oracleComparable: true, issues: []string{"/scalar-fixed:fixed"}},
		{name: "scalar-fixed/mismatch-and-pattern", source: "<scalar-fixed xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"B2\">C1</scalar-fixed>", oracleComparable: true, issues: []string{"/scalar-fixed:fixed", "/scalar-fixed:pattern"}},
		{name: "scalar-fixed/nil", source: "<scalar-fixed xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"B2\" n:nil=\"true\"></scalar-fixed>", oracleComparable: true, issues: []string{"/scalar-fixed:fixed"}},
		{name: "numeric-fixed/exact", source: "<numeric-fixed xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">01.20</numeric-fixed>", oracleComparable: true, issues: nil},
		// libxml2 compares element fixed spellings lexically here; retain the
		// normative internal expectation and exclude only this oracle case.
		{name: "numeric-fixed/equal-alternative", source: "<numeric-fixed xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">+1.2</numeric-fixed>", oracleComparable: false, issues: nil},
		{name: "numeric-fixed/equal-but-pattern-fails", source: "<numeric-fixed xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">1.2</numeric-fixed>", oracleComparable: true, issues: []string{"/numeric-fixed:pattern"}},
		{name: "choice/nil", source: "<choice xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" n:nil=\"true\"></choice>", oracleComparable: true, issues: nil},
		{name: "choice/nil-false", source: "<choice xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" n:nil=\"false\">B2</choice>", oracleComparable: true, issues: nil},
		{name: "choice/nil-content", source: "<choice xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" n:nil=\"true\">B2</choice>", oracleComparable: true, issues: []string{"/choice:nillable"}},
		{name: "choice/child", source: "<choice xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">A1<child/></choice>", oracleComparable: true, issues: []string{"/choice:simple-content"}},
		{name: "choice/comments-pi-cdata", source: "<choice xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\"><![CDATA[B]]><!--note--><?pi ok?>&#x32;</choice>", oracleComparable: true, issues: nil},
		{name: "scalar/nil", source: "<scalar xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"B2\" n:nil=\"true\"></scalar>", oracleComparable: true, issues: nil},
		{name: "scalar/nil-false", source: "<scalar xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"B2\" n:nil=\"false\">B2</scalar>", oracleComparable: true, issues: nil},
		{name: "scalar/nil-content", source: "<scalar xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"B2\" n:nil=\"true\">B2</scalar>", oracleComparable: true, issues: []string{"/scalar:nillable"}},
		{name: "scalar/child", source: "<scalar xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"B2\">A1<child/></scalar>", oracleComparable: true, issues: []string{"/scalar:simple-content"}},
		{name: "scalar/comments-pi-cdata", source: "<scalar xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"B2\"><![CDATA[B]]><!--note--><?pi ok?>&#x32;</scalar>", oracleComparable: true, issues: nil},
		{name: "scalar/nil-attribute", source: "<scalar xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" code=\"C1\" n:nil=\"true\"></scalar>", oracleComparable: true, issues: []string{"/scalar/@code:pattern"}},
		{name: "scalar/nil-missing-attribute", source: "<scalar xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" n:nil=\"true\"></scalar>", oracleComparable: true, issues: []string{"/scalar/@code:required"}},
		{name: "length/accept/AA", source: "<length xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">AA</length>", oracleComparable: true, issues: nil},
		{name: "length/accept/BB", source: "<length xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">BB</length>", oracleComparable: true, issues: nil},
		{name: "facets/accept/1.2", source: "<facets xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">1.2</facets>", oracleComparable: true, issues: nil},
		{name: "facets/accept/+1.2", source: "<facets xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">+1.2</facets>", oracleComparable: true, issues: nil},
		{name: "length/reject/A", source: "<length xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">A</length>", oracleComparable: true, issues: []string{"/length:length"}},
		{name: "length/reject/BBB", source: "<length xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">BBB</length>", oracleComparable: true, issues: []string{"/length:length"}},
		{name: "facets/reject/9.0", source: "<facets xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">9.0</facets>", oracleComparable: true, issues: []string{"/facets:maxInclusive"}},
		{name: "facets/reject/+9.0", source: "<facets xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">+9.0</facets>", oracleComparable: true, issues: []string{"/facets:maxInclusive"}},
		{name: "facets/fraction/1.23", source: "<facets xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">1.23</facets>", oracleComparable: true, issues: []string{"/facets:fractionDigits"}},
		{name: "facets/fraction/+1.23", source: "<facets xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\">+1.23</facets>", oracleComparable: true, issues: []string{"/facets:fractionDigits"}},
		{name: "strengthened/first", source: "<strengthened> \tA  B\n</strengthened>", oracleComparable: true, issues: nil},
		{name: "strengthened/last", source: "<strengthened>\nC\tD\r</strengthened>", oracleComparable: true, issues: nil},
		{name: "strengthened/base-reject", source: "<strengthened>A C</strengthened>", oracleComparable: true, issues: []string{"/strengthened:pattern"}},
		{name: "strengthened/non-xml-space", source: "<strengthened>A\u00a0B</strengthened>", oracleComparable: true, issues: []string{"/strengthened:pattern"}},
		{name: "inherited-space/first", source: "<inherited-space> \tA  B\n</inherited-space>", oracleComparable: true, issues: nil},
		{name: "inherited-space/last", source: "<inherited-space>\nC\tD\r</inherited-space>", oracleComparable: true, issues: nil},
		{name: "inherited-space/base-reject", source: "<inherited-space>A C</inherited-space>", oracleComparable: true, issues: []string{"/inherited-space:pattern"}},
		{name: "inherited-space/non-xml-space", source: "<inherited-space>A\u00a0B</inherited-space>", oracleComparable: true, issues: []string{"/inherited-space:pattern"}},
		{name: "total/12", source: "<total>12</total>", oracleComparable: true, issues: nil},
		{name: "total/+12", source: "<total>+12</total>", oracleComparable: true, issues: nil},
		{name: "total/123", source: "<total>123</total>", oracleComparable: true, issues: []string{"/total:totalDigits"}},
		{name: "total/+123", source: "<total>+123</total>", oracleComparable: true, issues: []string{"/total:totalDigits"}},
	}
}

func TestValidatePatternGroups(t *testing.T) {
	t.Parallel()
	for _, test := range validationPatternGroupCases() {
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationPatternGenerated, test.source), test.issues)
		})
	}
}

// Both validators receive the exact same original XML source, without model
// decoding or re-encoding. Linux CI requires this oracle root.
func TestPatternGroupsAgainstSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this external XSD test")
	}
	directory, err := filepath.Abs(filepath.Join("testdata", "validation"))
	require.NoError(t, err)
	for _, test := range validationPatternGroupCases() {
		if !test.oracleComparable {
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationPatternGenerated, test.source), test.issues)
			assertXMLLintOutcome(t, xmllint, filepath.Join(directory, "pattern-contract.xsd"), filepath.Join(directory, "catalog.xml"), test.source, len(test.issues) == 0)
		})
	}
}

func TestPatternGroupMetadataErrors(t *testing.T) {
	t.Parallel()
	context := newTestValidationContext(&validationSchemaSet{})
	for _, invalid := range []string{"[", `\p{IsBasicLatin}`} {
		// The block escape is legal XSD but unsupported by the current Go regex
		// translator. This is an internal metadata contract, not an XSD-validity claim.
		for index, patterns := range [][]string{
			{"A1", invalid}, {invalid, "A1"},
			{"A1", "B2", invalid}, {"A1", invalid, "B2"},
			{"B2", "A1", invalid}, {"B2", invalid, "A1"},
			{invalid, "A1", "B2"}, {invalid, "B2", "A1"},
		} {
			for _, value := range []string{"A1", "B2", "C3", ""} {
				t.Run(fmt.Sprintf("%s/%d/%q", invalid, index, value), func(t *testing.T) {
					schema := &validationSimpleSchema{
						Form:     validationSimpleRestriction,
						Base:     &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: "string"}},
						Patterns: patterns,
					}
					before := append([]string(nil), patterns...)
					failure := context.validateSimple(schema, value)
					if assert.NotNil(t, failure) {
						assert.Equal(t, "schema", failure.constraint)
						assert.Contains(t, failure.message, "invalid generated XSD pattern")
					}
					assert.Equal(t, before, patterns)
				})
			}
		}
	}
}

func TestPatternGroupDiagnostics(t *testing.T) {
	t.Parallel()
	context := newTestValidationContext(&validationSchemaSet{})
	schema := &validationSimpleSchema{
		Form:     validationSimpleRestriction,
		Base:     &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: "string"}},
		Patterns: []string{"A1"},
	}
	failure := context.validateSimple(schema, "B2")
	if assert.NotNil(t, failure) {
		assert.Equal(t, "pattern", failure.constraint)
		assert.Equal(t, `value "B2" does not match XSD pattern "A1"`, failure.message)
	}
	// Group failures name every alternative, using original source spelling even
	// though matching uses the effective normalized lexical view.
	for _, patterns := range [][]string{{"A1"}, {"A1", "B2"}} {
		schema := &validationSimpleSchema{
			Form:       validationSimpleRestriction,
			Base:       &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: "string"}},
			Patterns:   patterns,
			WhiteSpace: "collapse",
		}
		value := " \tC3\n"
		failure := context.validateSimple(schema, value)
		require.NotNil(t, failure)
		assert.Equal(t, "pattern", failure.constraint)
		if len(patterns) == 1 {
			assert.Equal(t, fmt.Sprintf("value %q does not match XSD pattern %q", value, patterns[0]), failure.message)
		} else {
			assert.Equal(t, fmt.Sprintf("value %q does not match any XSD pattern in %q", value, patterns), failure.message)
		}
	}
}

func TestPatternGroupMetadataAndSourceImmutability(t *testing.T) {
	t.Parallel()
	snapshot := func() map[string]string {
		result := make(map[string]string)
		for name, schema := range validationPatternGenerated.Types {
			encoded, err := json.Marshal(schema)
			require.NoError(t, err)
			result["type:"+name.Space+"\x00"+name.Local] = string(encoded)
		}
		for name, schema := range validationPatternGenerated.Elements {
			encoded, err := json.Marshal(schema)
			require.NoError(t, err)
			result["element:"+name.Space+"\x00"+name.Local] = string(encoded)
		}
		return result
	}
	metadataBefore := snapshot()
	for range 3 {
		for _, test := range validationPatternGroupCases() {
			root, err := parseValidationDocument([]byte(test.source))
			require.NoError(t, err)
			textBefore := root.Text.String()
			attributesBefore, err := json.Marshal(root.Attrs)
			require.NoError(t, err)
			context := newTestValidationContext(&validationPatternGenerated)
			element := validationPatternGenerated.Elements[validationName(root.Name)]
			require.NotNil(t, element)
			for range 3 {
				context.issues = nil
				context.validateElement(root, element, "/"+root.Name.Local)
				assertValidationIssues(t, context, test.issues)
			}
			assert.Equal(t, textBefore, root.Text.String(), test.name)
			attributesAfter, err := json.Marshal(root.Attrs)
			require.NoError(t, err)
			assert.Equal(t, attributesBefore, attributesAfter, test.name)
		}
	}
	assert.Equal(t, metadataBefore, snapshot())
}

func TestPatternGroupValidPermutations(t *testing.T) {
	t.Parallel()
	context := newTestValidationContext(&validationSchemaSet{})
	for _, patterns := range [][]string{
		{"A1", "B2", "C3"}, {"A1", "C3", "B2"},
		{"B2", "A1", "C3"}, {"B2", "C3", "A1"},
		{"C3", "A1", "B2"}, {"C3", "B2", "A1"},
	} {
		schema := &validationSimpleSchema{
			Form:     validationSimpleRestriction,
			Base:     &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: "string"}},
			Patterns: patterns,
		}
		for _, value := range []string{"A1", "B2", "C3"} {
			assert.Nil(t, context.validateSimple(schema, value), "%v / %q", patterns, value)
		}
		for _, value := range []string{"D4", ""} {
			failure := context.validateSimple(schema, value)
			if assert.NotNil(t, failure) {
				assert.Equal(t, "pattern", failure.constraint)
			}
		}
	}
	for _, patterns := range [][]string{{"", "x"}, {"x", ""}, {}} {
		schema := &validationSimpleSchema{
			Form:     validationSimpleRestriction,
			Base:     &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: "string"}},
			Patterns: patterns,
		}
		assert.Nil(t, context.validateSimple(schema, ""))
		assert.Nil(t, context.validateSimple(schema, "x"))
		if len(patterns) == 0 {
			assert.Nil(t, context.validateSimple(schema, "anything"))
		} else {
			failure := context.validateSimple(schema, "y")
			if assert.NotNil(t, failure) {
				assert.Equal(t, "pattern", failure.constraint)
			}
		}
	}
}
