package musicxml

import (
	"context"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Literal profile IDs keep the initial regression executable before the new
// catalog exists. The initial failure is profile rejection, not a build error.
func TestSourceDeclarationCompositionProfiles(t *testing.T) {
	profiles := []validation.ProfileID{
		"musicxml-4.0/source-internal-dtd-local",
		"musicxml-4.0/source-internal-dtd-links",
		"musicxml-4.0/source-internal-dtd-links-dictionaries",
	}
	for _, profile := range profiles {
		t.Run(string(profile), func(t *testing.T) {
			input := `<!DOCTYPE score-partwise [<!ATTLIST score-partwise version CDATA "4.0">]>` + strings.Replace(assessedScore("1", ""), ` version="4.0"`, "", 1)
			r := &assessmentCountingReader{source: strings.NewReader(input)}
			var report validation.Report
			var err error
			if strings.HasSuffix(string(profile), "-dictionaries") {
				report, err = InspectXMLWithDictionaries(context.Background(), r, DictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: profile}})
			} else {
				report, err = InspectXML(context.Background(), r, AssessmentOptions{Profile: profile})
			}
			if err != nil || !report.Satisfied() || report.Profile.ID != profile || report.Syntax != validation.SyntaxWellFormed || !report.InputComplete {
				t.Fatalf("profile=%s error=%v satisfied=%t syntax=%s findings=%+v", profile, err, report.Satisfied(), report.Syntax, report.Findings)
			}
			if r.closed || r.bytes != len(input) {
				t.Fatalf("source ownership: closed=%t bytes=%d want=%d", r.closed, r.bytes, len(input))
			}
			foundDefault := false
			for _, row := range report.Context {
				origin := row.SourceOrigin
				if origin != nil && row.Subject.Node == 1 && origin.Attribute != nil && origin.Attribute.Local == "version" {
					foundDefault = origin.Kind == validation.SourceOriginDefault && origin.Specified.State == validation.EvidenceKnown && !origin.Specified.Value && origin.Physical.State == validation.EvidenceAbsent
				}
			}
			if !foundDefault {
				t.Fatal("effective version lost its physical-absence/default provenance")
			}
		})
	}
}

func TestSourceDeclarationCompositionReachedEntity(t *testing.T) {
	const profile validation.ProfileID = "musicxml-4.0/source-internal-dtd-local"
	for _, tc := range []struct {
		name, declaration string
		conformance       validation.ConformanceState
	}{
		{"unparsed", `<!ENTITY picture SYSTEM "picture.bin" NDATA image><!NOTATION image SYSTEM "image/png">`, validation.ConformanceConformant},
		{"unparsed-notation-unresolved", `<!ENTITY picture SYSTEM "picture.bin" NDATA image>`, validation.ConformanceConformant},
		{"parsed", `<!ENTITY picture "text">`, validation.ConformanceViolated},
		{"complete-absent", "", validation.ConformanceViolated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := assessedScore("1", "")
			input = strings.Replace(input, "<part-list>", `<movement-title xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema" xsi:type="xs:ENTITY">picture</movement-title><part-list>`, 1)
			if tc.declaration != "" {
				input = "<!DOCTYPE score-partwise [" + tc.declaration + "]>" + input
			}
			report, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: profile})
			if err != nil || report.Conformance != tc.conformance || report.Syntax != validation.SyntaxWellFormed || !report.InputComplete {
				t.Fatalf("error=%v conformance=%s want=%s syntax=%s findings=%+v", err, report.Conformance, tc.conformance, report.Syntax, report.Findings)
			}
			if report.Satisfied() != (tc.conformance == validation.ConformanceConformant) {
				t.Fatalf("unexpected satisfaction: %+v", report)
			}
		})
	}
}
