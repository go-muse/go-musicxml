package musicxml

import (
	"errors"
	"testing"
)

// The Windows CI runner's libxml2 2.12.10 reports this validity-only diagnostic
// with exit zero. libxml2 2.9.14 additionally emits a parser/internal-error
// cascade and exits one. Neither observation changes the reader's XML domain.
func TestSourceDTDOracleRepeatedDeclarationVersionOutcomes(t *testing.T) {
	const output = "-:1: validity error : Redefinition of element r\n<!DOCTYPE r [<!ENTITY % p \"<!ELEMENT r EMPTY>\">%p;%p;]><r/>\n"
	if err := dtdOracleCheckObservation(dtdOracleValidityCascade, "Redefinition of element", []byte(output), nil); err != nil {
		t.Fatalf("observed libxml2 2.12.10 output rejected: %v", err)
	}
	for _, tc := range []struct {
		name, output string
		err          error
	}{
		{"clean", "", nil},
		{"unrelated-error", output + "error : unrelated failure\n", nil},
		{"unrelated-validity-error", output + "validity error : unrelated declaration\n", nil},
		{"parser-error", output + "parser error : malformed input\n", nil},
		{"internal-error-with-zero-exit", output + "internal error: unexpected failure\n", nil},
		{"warning", output + "warning: failed to load a resource\n", nil},
		{"spaced-warning", output + "warning : failed to load a resource\n", nil},
		{"transport-failure", output, errors.New("process failed to start")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if dtdOracleCheckObservation(dtdOracleValidityCascade, "Redefinition of element", []byte(tc.output), tc.err) == nil {
				t.Fatal("different oracle observation accepted")
			}
		})
	}
}

func TestSourceDTDOraclePredefinedDiagnosticVersions(t *testing.T) {
	// libxml2 v2.12.10 entities.c uses xmlEntitiesWarn with the entity name;
	// v2.9.14 emits the same diagnostic as an unnamed error. Both exit zero.
	// https://github.com/GNOME/libxml2/blob/v2.12.10/entities.c
	const prefix = "warning : xmlAddEntity: invalid redeclaration of predefined entity"
	for _, name := range []string{"lt", "amp", "gt", "apos", "quot"} {
		if err := dtdOracleCheckObservation(dtdOraclePredefinedError, "", []byte(prefix+" '"+name+"'\n"), nil); err != nil {
			t.Fatalf("named predefined warning rejected: %s: %v", name, err)
		}
	}
	for _, output := range []string{prefix + " 'other'", prefix + " 'lt'\nerror : unrelated failure", "warning : unrelated failure"} {
		if dtdOracleCheckObservation(dtdOraclePredefinedError, "", []byte(output), nil) == nil {
			t.Fatal("unrelated warning/error admitted")
		}
	}
}
