package musicxml

import (
	"context"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestSourceDeclarationCompositionLinkProvenance(t *testing.T) {
	for _, tc := range []struct {
		name, declaration, score string
		physical                 bool
		raw                      string
	}{
		{"defaulted", `<!ATTLIST score xlink:href CDATA "child.xml">`, `<score/>`, false, ""},
		{"physical-normalized", `<!ATTLIST score xlink:href NMTOKEN #IMPLIED>`, `<score xlink:href="  child.xml  "/>`, true, "  child.xml  "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := `<!DOCTYPE opus [` + tc.declaration + `]><opus version="4.0" xmlns:xlink="http://www.w3.org/1999/xlink">` + tc.score + `</opus>`
			report, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceInternalDTDLinks})
			if err != nil || !report.Satisfied() {
				t.Fatalf("error=%v findings=%s", err, localFindings(report))
			}
			count := 0
			for _, row := range report.Context {
				if row.Resource == nil || row.Resource.Link == nil || row.Resource.Link.Kind != validation.ResourceOpusScore {
					continue
				}
				count++
				href := row.Resource.Link.Href
				if href.Value.State != validation.EvidenceKnown || href.Value.Value.Kind != validation.AtomicString || href.Value.Value.Text != "child.xml" {
					t.Fatalf("effective href=%+v", href.Value)
				}
				if tc.physical {
					if href.Presence.State != validation.EvidenceKnown || !href.Presence.Value || href.Raw.State != validation.EvidenceKnown || href.Raw.Value != tc.raw {
						t.Fatalf("physical provenance=%+v", href)
					}
				} else if href.Presence.State != validation.EvidenceAbsent || href.Raw.State != validation.EvidenceAbsent {
					t.Fatalf("default fabricated physical evidence: %+v", href)
				}
			}
			if count != 1 {
				t.Fatalf("exact Opus score projection count=%d", count)
			}
		})
	}
}

func TestSourceDeclarationCompositionDefaultedSchemaHints(t *testing.T) {
	input := `<!DOCTYPE opus [<!ATTLIST opus xsi:schemaLocation CDATA "urn:example schema.xsd">]><opus version="4.0" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"/>`
	report, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{Profile: ProfileSourceInternalDTDLinks})
	if err != nil || !report.Satisfied() {
		t.Fatalf("error=%v findings=%s", err, localFindings(report))
	}
	count := 0
	for _, row := range report.Context {
		if row.Resource == nil || row.Resource.SchemaHints == nil {
			continue
		}
		count++
		hint := row.Resource.SchemaHints
		if hint.Raw.State != validation.EvidenceAbsent || hint.Complete.State != validation.EvidenceKnown || !hint.Complete.Value || len(hint.Pairs) != 1 || hint.Pairs[0].Namespace.State != validation.EvidenceKnown || hint.Pairs[0].Namespace.Value != "urn:example" || hint.Pairs[0].Location.State != validation.EvidenceKnown || hint.Pairs[0].Location.Value != "schema.xsd" {
			t.Fatalf("defaulted effective hint lost or fabricated physical spelling: %+v", hint)
		}
	}
	if count != 1 {
		t.Fatalf("hint projections=%d", count)
	}
}
