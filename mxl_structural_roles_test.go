package musicxml

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// structuralRolesScore is a minimal score whose only part optionally links
// separate part files.
func structuralRolesScore(partLinks string) string {
	return `<score-partwise xmlns:xlink="http://www.w3.org/1999/xlink" version="4.0"><part-list><score-part id="P1">` + partLinks +
		`<part-name>Piano</part-name></score-part></part-list><part id="P1"><measure number="1"><attributes><divisions>1</divisions></attributes>` +
		`<note><rest/><duration>1</duration></note></measure></part></score-partwise>`
}

func structuralRolesArchive(container string, files ...assessmentZIPFixture) []byte {
	entries := []assessmentZIPFixture{{name: mxlMIMETypePath, body: mxlMIMEType}, {name: mxlContainerPath, body: container}}
	return assessmentZIPBytes(append(entries, files...), nil, "")
}

// scoreBeforeParts returns the stored score-before-parts finding. Reports
// store failed and undecided decisions only, so a decided pass or vacuity has
// none.
func scoreBeforeParts(t *testing.T, b validation.AssessmentBundle) (validation.Outcome, validation.Code) {
	t.Helper()
	for _, f := range b.PackageReport().Findings {
		if f.Rule.ID == "MX40-PACKAGE-score-before-parts" {
			return f.Outcome, f.Reason
		}
	}
	return "", ""
}

// Without a role provider, the MusicXML link semantics decide the
// score-before-parts antecedent: container.xsd puts the score in the first
// rootfile when separate part files exist, and the part files are linked from
// it.
func TestScoreBeforePartsStructuralRoles(t *testing.T) {
	score := structuralRolesScore("")
	for _, tc := range []struct {
		name     string
		archive  []byte
		accepted bool
		outcome  validation.Outcome
		reason   validation.Code
	}{
		{"pdf rendition", structuralRolesArchive(`<container><rootfiles><rootfile full-path="a.musicxml"/><rootfile full-path="a.pdf" media-type="application/pdf"/></rootfiles></container>`,
			assessmentZIPFixture{name: "a.musicxml", body: score}, assessmentZIPFixture{name: "a.pdf", body: "%PDF-1.4\n%%EOF\n"}),
			true, "", ""},
		{"opus with a local score", structuralRolesArchive(`<container><rootfiles><rootfile full-path="root.opus"/></rootfiles></container>`,
			assessmentZIPFixture{name: "root.opus", body: `<opus xmlns:xlink="http://www.w3.org/1999/xlink" version="4.0"><title>T</title><score xlink:href="a.musicxml"/></opus>`},
			assessmentZIPFixture{name: "a.musicxml", body: score}),
			true, "", ""},
		{"first rootfile links its part", structuralRolesArchive(`<container><rootfiles><rootfile full-path="score.musicxml"/></rootfiles></container>`,
			assessmentZIPFixture{name: "score.musicxml", body: structuralRolesScore(`<part-link xlink:href="part.musicxml"/>`)},
			assessmentZIPFixture{name: "part.musicxml", body: score}),
			true, "", ""},
		{"a later document links parts", structuralRolesArchive(`<container><rootfiles><rootfile full-path="a.musicxml"/><rootfile full-path="score.musicxml"/></rootfiles></container>`,
			assessmentZIPFixture{name: "a.musicxml", body: score},
			assessmentZIPFixture{name: "score.musicxml", body: structuralRolesScore(`<part-link xlink:href="part.musicxml"/>`)},
			assessmentZIPFixture{name: "part.musicxml", body: score}),
			false, validation.OutcomeUnknown, "package-score-part-role-evidence-unavailable"},
		{"pdf rendition with directory entries", structuralRolesArchive(`<container><rootfiles><rootfile full-path="a.musicxml"/><rootfile full-path="a.pdf" media-type="application/pdf"/></rootfiles></container>`,
			assessmentZIPFixture{name: "META-INF/"}, assessmentZIPFixture{name: "a.musicxml", body: score}, assessmentZIPFixture{name: "a.pdf", body: "%PDF-1.4\n%%EOF\n"}),
			true, "", ""},
		{"work opus target", structuralRolesArchive(`<container><rootfiles><rootfile full-path="a.musicxml"/></rootfiles></container>`,
			assessmentZIPFixture{name: "a.musicxml", body: `<score-partwise xmlns:xlink="http://www.w3.org/1999/xlink" version="4.0"><work><opus xlink:href="o.opus"/></work>` + score[len(`<score-partwise xmlns:xlink="http://www.w3.org/1999/xlink" version="4.0">`):]},
			assessmentZIPFixture{name: "o.opus", body: `<opus version="4.0"><title>O</title></opus>`}),
			true, "", ""},
		{"opus link target", structuralRolesArchive(`<container><rootfiles><rootfile full-path="root.opus"/></rootfiles></container>`,
			assessmentZIPFixture{name: "root.opus", body: `<opus xmlns:xlink="http://www.w3.org/1999/xlink" version="4.0"><title>T</title><opus-link xlink:href="sub.opus"/></opus>`},
			assessmentZIPFixture{name: "sub.opus", body: `<opus version="4.0"><title>S</title></opus>`}),
			true, "", ""},
		{"xml rendition", structuralRolesArchive(`<container><rootfiles><rootfile full-path="a.musicxml"/><rootfile full-path="b.xml" media-type="application/xml"/></rootfiles></container>`,
			assessmentZIPFixture{name: "a.musicxml", body: score}, assessmentZIPFixture{name: "b.xml", body: score}),
			false, validation.OutcomeUnknown, "package-score-part-role-evidence-unavailable"},
		{"unlinked second score", structuralRolesArchive(`<container><rootfiles><rootfile full-path="a.musicxml"/><rootfile full-path="b.musicxml"/></rootfiles></container>`,
			assessmentZIPFixture{name: "a.musicxml", body: score}, assessmentZIPFixture{name: "b.musicxml", body: score}),
			false, validation.OutcomeUnknown, "package-score-part-role-evidence-unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inspected, err := InspectMXL(context.Background(), bytes.NewReader(tc.archive), MXLAssessmentOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if outcome, reason := scoreBeforeParts(t, inspected); outcome != tc.outcome || reason != tc.reason {
				t.Errorf("InspectMXL score-before-parts %s %s, want %s %s", outcome, reason, tc.outcome, tc.reason)
			}
			result, err := DecodeMXLAssessed(context.Background(), bytes.NewReader(tc.archive), DecodeMXLAssessmentOptions{})
			if result.Accepted != tc.accepted || (err == nil) != tc.accepted {
				t.Fatalf("strict loading accepted=%v error=%v, want accepted=%v", result.Accepted, err, tc.accepted)
			}
			if !tc.accepted && !errors.Is(err, ErrIncompleteAssessment) {
				t.Errorf("an undecided antecedent must stay incomplete: %v", err)
			}
			if tc.accepted && (!inspected.Satisfied() || !result.ModelLine.Satisfied()) {
				t.Errorf("InspectMXL satisfied=%v, line 3 satisfied=%v", inspected.Satisfied(), result.ModelLine.Satisfied())
			}
		})
	}
}

// A later rootfile whose bytes are not XML terminates the package
// assessment as incomplete, without an internal session error.
func TestTerminatedRootfileAssessmentHasNoSessionError(t *testing.T) {
	archive := structuralRolesArchive(`<container><rootfiles><rootfile full-path="a.musicxml"/><rootfile full-path="b.musicxml"/></rootfiles></container>`,
		assessmentZIPFixture{name: "a.musicxml", body: structuralRolesScore("")}, assessmentZIPFixture{name: "b.musicxml", body: "%PDF-1.4 not xml"})
	for _, lenient := range []bool{false, true} {
		_, err := DecodeMXLAssessed(context.Background(), bytes.NewReader(archive), DecodeMXLAssessmentOptions{Lenient: lenient})
		if !errors.Is(err, ErrIncompleteAssessment) || errors.Is(err, validation.ErrInvalidSession) {
			t.Errorf("lenient=%v: %v", lenient, err)
		}
	}
	pkg, err := DecodeMXLPackage(bytes.NewReader(archive))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = AssessMXLPackage(context.Background(), pkg, MXLAssessmentOptions{}); !errors.Is(err, ErrIncompleteAssessment) || errors.Is(err, validation.ErrInvalidSession) {
		t.Errorf("AssessMXLPackage: %v", err)
	}
}

// The official compressed test-suite file, a score with a PDF rendition and
// directory entries, is accepted without a role resolver.
func TestScoreBeforePartsStructuralRolesOnTheCorpus(t *testing.T) {
	archive, err := os.ReadFile(filepath.Join("testdata", "corpus", "musicxml-test-suite", "90a-Compressed-MusicXML.mxl"))
	if err != nil {
		t.Fatal(err)
	}
	inspected, err := InspectMXL(context.Background(), bytes.NewReader(archive), MXLAssessmentOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if outcome, reason := scoreBeforeParts(t, inspected); outcome != "" {
		t.Errorf("InspectMXL score-before-parts %s %s", outcome, reason)
	}
	if result, err := DecodeMXLAssessed(context.Background(), bytes.NewReader(archive), DecodeMXLAssessmentOptions{}); err != nil || !result.Accepted {
		t.Errorf("strict loading: accepted=%v error=%v", result.Accepted, err)
	}
}

// partialRoleReply answers one known role fact for one member and leaves
// every other fact, and the scope, unknown.
func partialRoleReply(q validation.ResourceRequest, entry string, role validation.ResourceRole, value bool) validation.ResourceResult {
	r := validation.ResourceRoleResult{Request: *q.Roles, ScopeComplete: validation.Evidence[bool]{State: validation.EvidenceUnknown, Reason: "partial"},
		Authority: validation.EvidenceAuthority{ID: "fixture-authority", ExactRelationComplete: true, Provenance: []validation.SourceRef{{IDs: []string{"roles"}, URL: "https://example.test/roles", Locator: "exact-member-scope"}}}}
	for _, m := range q.Roles.Members {
		for _, rr := range []validation.ResourceRole{validation.ResourceRoleScore, validation.ResourceRoleIndividualPart} {
			member := m
			ev := validation.Evidence[bool]{State: validation.EvidenceUnknown, Reason: "not-in-table"}
			if m.ArchiveEntry == entry && rr == role {
				ev = validation.Evidence[bool]{State: validation.EvidenceKnown, Value: value}
			}
			r.Resources = append(r.Resources, validation.ResourceRoleFact{Resource: m.Resource, Member: &member, Role: rr, Membership: ev})
		}
	}
	return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Roles: &r}
}

// A role provider's known fact is never overridden by the structural
// inference, even when the provider's answer decides nothing.
func TestScoreBeforePartsProviderFactsWinOverStructure(t *testing.T) {
	score := structuralRolesScore("")
	pdf := structuralRolesArchive(`<container><rootfiles><rootfile full-path="a.musicxml"/><rootfile full-path="a.pdf" media-type="application/pdf"/></rootfiles></container>`,
		assessmentZIPFixture{name: "a.musicxml", body: score}, assessmentZIPFixture{name: "a.pdf", body: "%PDF"})
	linked := structuralRolesArchive(`<container><rootfiles><rootfile full-path="score.musicxml"/></rootfiles></container>`,
		assessmentZIPFixture{name: "score.musicxml", body: structuralRolesScore(`<part-link xlink:href="part.musicxml"/>`)},
		assessmentZIPFixture{name: "part.musicxml", body: score})
	for _, tc := range []struct {
		name    string
		archive []byte
		entry   string
		role    validation.ResourceRole
		value   bool
	}{
		{"primary named a part", pdf, "a.musicxml", validation.ResourceRoleIndividualPart, true},
		{"primary named not a score", linked, "score.musicxml", validation.ResourceRoleScore, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			asked := 0
			options := MXLAssessmentOptions{Dependencies: []ResourceDependency{ResourceRoles}, Resolver: validation.ResourceResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
				if q.Roles == nil {
					return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceUnknown, Reason: "unsupported"}, nil
				}
				asked++
				return partialRoleReply(q, tc.entry, tc.role, tc.value), nil
			})}
			b, err := InspectMXL(context.Background(), bytes.NewReader(tc.archive), options)
			if err != nil {
				t.Fatal(err)
			}
			if outcome, _ := scoreBeforeParts(t, b); asked == 0 || outcome != validation.OutcomeUnknown {
				t.Errorf("provider asked %d times; score-before-parts %q, want unknown", asked, outcome)
			}
		})
	}
}
