package musicxml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"os"
	"os/exec"
	"testing"
)

// These are XML 1.0 well-formedness controls, not validating-DTD or XSD
// conformance. All entities are internal: --nonet is defense in depth, not
// permission to retrieve a local external subset. No external identifier occurs.
func TestSourceInternalDTDAgainstXMLLint(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this differential")
	}
	for _, tc := range []struct {
		name, input string
		valid       bool
	}{
		{"internal-text", `<!DOCTYPE r [<!ENTITY e 'text'>]><r>&e;</r>`, true},
		{"internal-markup", `<!DOCTYPE r [<!ENTITY e '<child/>'>]><r>&e;</r>`, true},
		{"nested-text", `<!DOCTYPE r [<!ENTITY a 'ok'><!ENTITY b '&a;'>]><r>&b;</r>`, true},
		{"quote-in-attribute", `<!DOCTYPE r [<!ENTITY e '&#34;'>]><r a="&e;"/>`, true},
		{"numeric-whitespace", `<!DOCTYPE r [<!ATTLIST r a NMTOKENS 'a&#9;b'>]><r/>`, true},
		{"whole-parameter-declaration", `<!DOCTYPE r [<!ENTITY % p "<!ENTITY e 'ok'>">%p;]><r>&e;</r>`, true},
		{"literal-parameter-comment", `<!DOCTYPE r [<!ENTITY % p '<!-- &#37;p; -->'>%p;]><r/>`, true},
		{"crossed-element-boundary", `<!DOCTYPE r [<!ENTITY e '<child>'>]><r>&e;</child></r>`, false},
		{"recursive-general", `<!DOCTYPE r [<!ENTITY e '&e;'>]><r>&e;</r>`, false},
		{"less-than-attribute", `<!DOCTYPE r [<!ENTITY e '<'>]><r a="&e;"/>`, false},
		{"unused-malformed-default", `<!DOCTYPE r [<!ENTITY e '<'><!ATTLIST unused a CDATA '&e;'>]><r/>`, false},
		{"partial-parameter-declaration", `<!DOCTYPE r [<!ENTITY % p '<!ENTITY e'>%p; 'ok'>]><r/>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := exec.Command(xmllint, "--nonet", "--noent", "--noout", "-")
			command.Stdin = bytes.NewBufferString(tc.input)
			output, oracleErr := command.CombinedOutput()
			if (oracleErr == nil) != tc.valid {
				t.Fatalf("oracle valid=%t: %v: %s", tc.valid, oracleErr, output)
			}
			if oracleErr != nil {
				var exit *exec.ExitError
				if !errors.As(oracleErr, &exit) || exit.ExitCode() != 1 {
					t.Fatalf("oracle infrastructure/other failure: %v: %s", oracleErr, output)
				}
			}
			_, _, _, got := sourceEntityScan(t, tc.input)
			if tc.valid && got != nil {
				t.Fatalf("bounded internal reader rejected: %v", got)
			}
			if !tc.valid {
				var syntax *xml.SyntaxError
				if !errors.As(got, &syntax) {
					t.Fatalf("bounded internal reader did not report malformed XML: %v", got)
				}
			}
		})
	}
}
