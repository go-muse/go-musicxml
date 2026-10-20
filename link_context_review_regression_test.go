package musicxml

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestLinkContextReviewDTDHintRowsRetainObservedDescendants(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		profile      validation.ProfileID
	}{
		{"score", resourceScore(strings.Repeat(`<note><rest/><duration>1</duration></note>`, 30)), ProfileSourceLinks.ID()},
		{"opus", `<opus><opus><opus/></opus></opus>`, ProfileSourceLinks.ID()},
		{"container", `<container><rootfiles><rootfile full-path="score.musicxml"/></rootfiles></container>`, ProfileSourceContainer.ID()},
		{"sounds", `<sounds><sound id="wind.flutes.flute"><description>flute</description></sound></sounds>`, ProfileSourceSounds.ID()},
	} {
		for _, observed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/observed=%t", tc.name, observed), func(t *testing.T) {
				source := tc.source
				if observed {
					start := strings.Index(source, "><") + 2
					end := strings.IndexAny(source[start:], " />") + start
					source = source[:end] + ` xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="urn:observed observed.xsd"` + source[end:]
				}
				rootEnd := strings.IndexAny(source[1:], " >") + 1
				source = `<!DOCTYPE ` + source[1:rootEnd] + ` SYSTEM "unread.dtd">` + source
				report, err := InspectXML(context.Background(), strings.NewReader(source), SourceOptions{Context: validation.ContextFamilyAll, Profile: SourceProfile(tc.profile)})
				if err != nil {
					t.Fatal(err)
				}
				var rows []validation.ContextResult
				for _, row := range report.Context {
					if row.Resource != nil && row.Resource.SchemaHints != nil {
						rows = append(rows, row)
					}
				}
				want := 1
				if observed {
					want++
				}
				if len(rows) != want {
					t.Errorf("hint rows=%d want=%d", len(rows), want)
				}
				if len(rows) == 0 {
					t.Fatal("missing document-wide hint completeness gap")
				}
				root := rows[0].Resource
				if root.SchemaHints.Raw.State != validation.EvidenceUnsupported || root.SchemaHints.Raw.Reason != "dtd-infoset-unavailable" {
					t.Errorf("lost root gap: %+v", root.SchemaHints)
				}
				declaration, ok := any(root.SubjectDeclaration).(validation.Evidence[validation.DeclarationID])
				if !ok || declaration.State != validation.EvidenceUnsupported || declaration.Reason != "dtd-infoset-unavailable" {
					t.Errorf("declaration did not preserve typed gap: %+v", root.SubjectDeclaration)
				}
				if observed {
					found := false
					for _, row := range rows[1:] {
						if row.Subject.Node != rows[0].Subject.Node && row.Resource.SchemaHints.Raw.State == validation.EvidenceUnsupported && row.Resource.SchemaHints.Raw.Reason == "dtd-infoset-unavailable" {
							found = true
						}
					}
					if !found {
						t.Error("observed descendant hint discarded with unavailable declaration")
					}
				}
			})
		}
	}
}

func TestLinkContextReviewDTDFactBudgetDoesNotCapElements(t *testing.T) {
	source := `<!DOCTYPE score-partwise SYSTEM "unread.dtd">` + resourceScore(strings.Repeat(`<note><rest/><duration>1</duration></note>`, 100))
	report, err := InspectXML(context.Background(), strings.NewReader(source), SourceOptions{Context: validation.ContextFamilyAll, Profile: ProfileSourceLinks, Budgets: validation.Budgets{MaxContextFacts: 4}})
	if err != nil {
		t.Fatalf("element count consumed hint fact quota: %v", err)
	}
	hints := 0
	for _, row := range report.Context {
		if row.Resource != nil && row.Resource.SchemaHints != nil {
			hints++
		}
	}
	if hints != 1 {
		t.Errorf("hint rows=%d want=1", hints)
	}
}

func TestLinkContextReviewPairingEvidenceAndExactPresence(t *testing.T) {
	for _, tc := range []struct {
		name, attrs string
		raw         validation.EvidenceState
		value       bool
		trailing    string
	}{
		{"absent", "", validation.EvidenceAbsent, true, ""},
		{"empty", ` xsi:schemaLocation=""`, validation.EvidenceKnown, true, ""},
		{"even", ` xsi:schemaLocation="urn:a a.xsd"`, validation.EvidenceKnown, true, ""},
		{"odd", ` xsi:schemaLocation="urn:a a.xsd extra"`, validation.EvidenceKnown, false, "extra"},
		{"no-namespace", ` xsi:noNamespaceSchemaLocation="a.xsd"`, validation.EvidenceAbsent, true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report, facts := linkResources(t, `<opus xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"`+tc.attrs+`/>`)
			if !report.Satisfied() {
				t.Fatalf("optional discovery changed XSD acceptance: %+v", report)
			}
			if len(facts) != 1 {
				t.Fatalf("facts=%d", len(facts))
			}
			h := facts[0].Resource.SchemaHints
			if h == nil || h.Raw.State != tc.raw || h.Complete.State != validation.EvidenceKnown || h.Complete.Value != tc.value || h.Trailing.Value != tc.trailing {
				t.Errorf("pairing=%+v", h)
			}
			if tc.name == "no-namespace" && (h.NoNamespace.Presence.State != validation.EvidenceKnown || !h.NoNamespace.Presence.Value) {
				t.Errorf("noNamespace presence=%+v", h.NoNamespace)
			}
		})
	}
}
