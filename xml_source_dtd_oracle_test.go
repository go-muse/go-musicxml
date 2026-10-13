package musicxml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

type dtdOracleObservation uint8

const (
	dtdOracleClean dtdOracleObservation = iota
	dtdOracleParserError
	dtdOracleValidityCascade
	dtdOracleValidityOnly
	dtdOraclePredefinedError
)

type dtdReaderDisposition uint8

const (
	dtdReaderComplete dtdReaderDisposition = iota
	dtdReaderMalformed
	dtdReaderIncomplete
	dtdReaderUnsupported
)

// The oracle observations describe libxml2, not normative XML classifications.
// Reader dispositions are independently pinned to the XML 1.0 Fifth Edition
// bounded profile. In particular, a validity-error cascade can exit 1, and the
// standalone/PE cases deliberately record disagreements instead of voting on
// well-formedness. External identifiers in this closed corpus are limited to
// missing.ent, relative to a fresh empty directory; no arbitrary path is run.
func TestSourceInternalDTDAgainstXMLLint(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint is required but was not found in PATH")
		}
		t.Skip("xmllint is not installed; Linux CI requires this differential")
	}
	if version, err := exec.Command(xmllint, "--version").CombinedOutput(); err != nil {
		t.Fatal(err)
	} else {
		t.Logf("oracle version: %s", version)
	}
	for _, tc := range []struct {
		name, input string
		oracle      dtdOracleObservation
		reader      dtdReaderDisposition
		signature   string
	}{
		{"internal-text", `<!DOCTYPE r [<!ENTITY e 'text'>]><r>&e;</r>`, dtdOracleClean, dtdReaderComplete, ""},
		{"internal-markup", `<!DOCTYPE r [<!ENTITY e '<child/>'>]><r>&e;</r>`, dtdOracleClean, dtdReaderComplete, ""},
		{"nested-text", `<!DOCTYPE r [<!ENTITY a 'ok'><!ENTITY b '&a;'>]><r>&b;</r>`, dtdOracleClean, dtdReaderComplete, ""},
		{"quote-in-attribute", `<!DOCTYPE r [<!ENTITY e '&#34;'>]><r a="&e;"/>`, dtdOracleClean, dtdReaderComplete, ""},
		{"numeric-whitespace", `<!DOCTYPE r [<!ATTLIST r a NMTOKENS 'a&#9;b'>]><r/>`, dtdOracleValidityOnly, dtdReaderComplete, "invalid default value"},
		{"whole-parameter-declaration", `<!DOCTYPE r [<!ENTITY % p "<!ENTITY e 'ok'>">%p;]><r>&e;</r>`, dtdOracleClean, dtdReaderComplete, ""},
		{"literal-parameter-comment", `<!DOCTYPE r [<!ENTITY % p '<!-- &#37;p; -->'>%p;]><r/>`, dtdOracleClean, dtdReaderComplete, ""},
		{"crossed-element-boundary", `<!DOCTYPE r [<!ENTITY e '<child>'>]><r>&e;</child></r>`, dtdOracleParserError, dtdReaderMalformed, ""},
		{"recursive-general", `<!DOCTYPE r [<!ENTITY e '&e;'>]><r>&e;</r>`, dtdOracleParserError, dtdReaderMalformed, ""},
		{"less-than-attribute", `<!DOCTYPE r [<!ENTITY e '<'>]><r a="&e;"/>`, dtdOracleParserError, dtdReaderMalformed, ""},
		{"unused-malformed-default", `<!DOCTYPE r [<!ENTITY e '<'><!ATTLIST unused a CDATA '&e;'>]><r/>`, dtdOracleParserError, dtdReaderMalformed, ""},
		{"partial-parameter-declaration", `<!DOCTYPE r [<!ENTITY % p '<!ENTITY e'>%p; 'ok'>]><r/>`, dtdOracleParserError, dtdReaderMalformed, ""},
		{"undeclared-general", `<!DOCTYPE r []><r>&e;</r>`, dtdOracleParserError, dtdReaderMalformed, ""},
		{"unparsed-general", `<!DOCTYPE r [<!NOTATION n SYSTEM "missing.ent"><!ENTITY e SYSTEM "missing.ent" NDATA n>]><r>&e;</r>`, dtdOracleParserError, dtdReaderMalformed, "unparsed entity"},
		{"external-in-attribute", `<!DOCTYPE r [<!ENTITY e SYSTEM "missing.ent">]><r a='&e;'/>`, dtdOracleParserError, dtdReaderMalformed, "external entity"},
		{"parameter-in-physical-declaration", `<!DOCTYPE r [<!ENTITY f "%p;">]><r/>`, dtdOracleParserError, dtdReaderMalformed, ""},
		{"illegal-character", `<!DOCTYPE r [<!ENTITY e "&#0;">]><r>&e;</r>`, dtdOracleParserError, dtdReaderMalformed, ""},
		{"reference-outside-root", `<!DOCTYPE r [<!ENTITY e "text">]>&e;<r/>`, dtdOracleParserError, dtdReaderMalformed, ""},
		{"bare-ampersand-replacement", `<!DOCTYPE r [<!ENTITY e "&#38;">]><r>&e;</r>`, dtdOracleParserError, dtdReaderMalformed, ""},
		// XML 1.0 section 4.1: standalone=yes requires a declaration outside PEs.
		{"standalone-parameter-declaration-divergence", `<?xml version="1.0" standalone="yes"?><!DOCTYPE r [<!ENTITY % p "<!ENTITY e 'x'>">%p;]><r>&e;</r>`, dtdOracleClean, dtdReaderMalformed, ""},
		// PE declaration precedence is listed under VC Entity Declared, not WFC.
		{"undeclared-parameter-validity-divergence", `<!DOCTYPE r [%p;]><r/>`, dtdOracleParserError, dtdReaderIncomplete, "PEReference"},
		{"standalone-undeclared-parameter", `<?xml version="1.0" standalone="yes"?><!DOCTYPE r [%p;]><r/>`, dtdOracleParserError, dtdReaderMalformed, "PEReference"},
		{"parameter-in-parameter-declaration-bounded-gap", `<!DOCTYPE r [<!ENTITY % pe "x"><!ENTITY % outer "<!ENTITY f &#39;&#37;pe;&#39;>">%outer;]><r>&f;</r>`, dtdOracleClean, dtdReaderUnsupported, ""},
		// A validating constraint violation can make libxml2 report an internal
		// parser cascade and exit 1. The bounded reader remains non-validating.
		{"duplicate-element-parameter-validity-cascade", `<!DOCTYPE r [<!ENTITY % p "<!ELEMENT r EMPTY>">%p;%p;]><r/>`, dtdOracleValidityCascade, dtdReaderComplete, "Redefinition of element"},
		{"predefined-lt-valid", `<!DOCTYPE r [<!ENTITY lt "&#38;#60;">]><r/>`, dtdOracleClean, dtdReaderComplete, ""},
		{"predefined-amp-valid", `<!DOCTYPE r [<!ENTITY amp "&#38;#38;">]><r/>`, dtdOracleClean, dtdReaderComplete, ""},
		{"predefined-gt-valid", `<!DOCTYPE r [<!ENTITY gt ">">]><r/>`, dtdOracleClean, dtdReaderComplete, ""},
		{"predefined-apos-valid", `<!DOCTYPE r [<!ENTITY apos "&#39;">]><r/>`, dtdOracleClean, dtdReaderComplete, ""},
		{"predefined-quot-valid", `<!DOCTYPE r [<!ENTITY quot "&#34;">]><r/>`, dtdOracleClean, dtdReaderComplete, ""},
		// libxml2 reports these redeclaration errors with exit zero. The bounded
		// reader preserves incomplete closure instead of inventing a WFC failure.
		{"predefined-lt-invalid", `<!DOCTYPE r [<!ENTITY lt "x">]><r/>`, dtdOraclePredefinedError, dtdReaderIncomplete, "invalid redeclaration of predefined entity"},
		{"predefined-amp-invalid", `<!DOCTYPE r [<!ENTITY amp "x">]><r/>`, dtdOraclePredefinedError, dtdReaderIncomplete, "invalid redeclaration of predefined entity"},
		{"predefined-gt-invalid", `<!DOCTYPE r [<!ENTITY gt "x">]><r/>`, dtdOraclePredefinedError, dtdReaderIncomplete, "invalid redeclaration of predefined entity"},
		{"predefined-apos-invalid", `<!DOCTYPE r [<!ENTITY apos "x">]><r/>`, dtdOraclePredefinedError, dtdReaderIncomplete, "invalid redeclaration of predefined entity"},
		{"predefined-quot-invalid", `<!DOCTYPE r [<!ENTITY quot "x">]><r/>`, dtdOraclePredefinedError, dtdReaderIncomplete, "invalid redeclaration of predefined entity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := dtdOracleFixtureSafe(tc.input); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(xmllint, "--nonet", "--noent", "--noout", "-")
			command.Dir = t.TempDir()
			command.Stdin = bytes.NewBufferString(tc.input)
			output, oracleErr := command.CombinedOutput()
			if err := dtdOracleCheckObservation(tc.oracle, tc.signature, output, oracleErr); err != nil {
				t.Fatalf("oracle observation: %v: %s", err, output)
			}
			_, _, summary, got := sourceEntityScan(t, tc.input)
			var syntax *xml.SyntaxError
			var unsupported *sourceXMLUnsupportedError
			switch tc.reader {
			case dtdReaderComplete:
				if got != nil || !summary.SyntaxComplete {
					t.Fatalf("want complete bounded syntax: error=%v summary=%+v", got, summary)
				}
			case dtdReaderMalformed:
				if !errors.As(got, &syntax) {
					t.Fatalf("want malformed: %v", got)
				}
			case dtdReaderIncomplete:
				if got != nil || summary.SyntaxComplete || !summary.InputComplete {
					t.Fatalf("want incomplete syntax after complete input: error=%v summary=%+v", got, summary)
				}
			case dtdReaderUnsupported:
				if !errors.As(got, &unsupported) {
					t.Fatalf("want bounded unsupported: %v", got)
				}
			}
		})
	}
}

func dtdOracleFixtureSafe(input string) error {
	remaining := strings.ReplaceAll(input, `SYSTEM "missing.ent"`, "")
	if strings.Contains(remaining, "SYSTEM") || strings.Contains(remaining, "PUBLIC") {
		return errors.New("oracle fixture contains an unapproved external identifier")
	}
	return nil
}
func dtdOracleCheckObservation(want dtdOracleObservation, signature string, output []byte, err error) error {
	text := string(output)
	if signature != "" && !strings.Contains(text, signature) {
		return errors.New("missing pinned diagnostic signature")
	}
	if want == dtdOracleValidityOnly {
		if err != nil || !strings.Contains(text, "validity error") || strings.Contains(text, "parser error") {
			return errors.New("wanted nonfatal validity diagnostic")
		}
		return nil
	}
	if want == dtdOraclePredefinedError {
		if err != nil {
			return errors.New("wanted exit-zero predefined-entity diagnostic")
		}
		const diagnostic = "xmlAddEntity: invalid redeclaration of predefined entity"
		line := strings.TrimSpace(text)
		if line == "error : "+diagnostic {
			return nil
		}
		for _, name := range []string{"lt", "amp", "gt", "apos", "quot"} {
			if line == "warning : "+diagnostic+" '"+name+"'" {
				return nil
			}
		}
		return errors.New("wanted only the pinned predefined-entity diagnostic")
	}
	if want == dtdOracleClean {
		if err != nil || len(bytes.TrimSpace(output)) != 0 {
			return errors.New("wanted clean oracle observation")
		}
		return nil
	}
	// libxml2 2.12.10 reports the repeated declaration's validity diagnostic
	// without the older 2.9.14 parser/internal-error cascade and exits zero.
	// Admit only that pinned diagnostic, never an arbitrary successful exit.
	if want == dtdOracleValidityCascade && err == nil {
		if signature == "" || !strings.Contains(text, "validity error") || strings.Contains(text, "parser error") || strings.Contains(text, "internal error") || strings.Contains(text, "warning:") || strings.Contains(text, "warning :") {
			return errors.New("wanted standalone repeated-declaration validity diagnostic")
		}
		for _, line := range strings.Split(text, "\n") {
			if strings.Contains(line, "error :") && (!strings.Contains(line, "validity error :") || !strings.Contains(line, signature)) {
				return errors.New("unexpected additional oracle error")
			}
		}
		return nil
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		return errors.New("wanted oracle exit 1, not an infrastructure/other failure")
	}
	if !strings.Contains(text, "parser error") {
		return errors.New("missing parser diagnostic")
	}
	if want == dtdOracleValidityCascade {
		if !strings.Contains(text, "validity error") || !strings.Contains(text, "internal error") {
			return errors.New("missing validity/internal-error cascade")
		}
	} else if strings.Contains(text, "validity error") || strings.Contains(text, "internal error") {
		return errors.New("unexpected validity/internal-error cascade")
	}
	return nil
}

func TestSourceDTDOracleFixtureScope(t *testing.T) {
	for _, input := range []string{`<!DOCTYPE r SYSTEM "/etc/passwd"><r/>`, `<!DOCTYPE r SYSTEM "https://example.invalid/x"><r/>`, `<!DOCTYPE r PUBLIC "id" "x"><r/>`} {
		if dtdOracleFixtureSafe(input) == nil {
			t.Fatal("unapproved external identifier admitted")
		}
	}
	if err := dtdOracleFixtureSafe(`<!ENTITY e SYSTEM "missing.ent">`); err != nil {
		t.Fatal(err)
	}
	if dtdOracleCheckObservation(dtdOracleClean, "", []byte("parser error: Entity not defined"), nil) == nil {
		t.Fatal("exit zero diagnostic mistaken for clean oracle")
	}
	const predefined = "error : xmlAddEntity: invalid redeclaration of predefined entity\n"
	if err := dtdOracleCheckObservation(dtdOraclePredefinedError, "", []byte(predefined), nil); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, output string
		err          error
	}{
		{"nonzero-error", predefined, errors.New("nonzero oracle exit")},
		{"clean-stderr", "", nil},
		{"parser-error", predefined + "parser error: invalid XML", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if dtdOracleCheckObservation(dtdOraclePredefinedError, "", []byte(tc.output), tc.err) == nil {
				t.Fatal("different oracle outcome accepted as predefined redeclaration diagnostic")
			}
		})
	}
}
