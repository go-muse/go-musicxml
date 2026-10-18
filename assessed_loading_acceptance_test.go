package musicxml

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestAssessedModelAcceptancePreservesIndependentRepresentationFailure(t *testing.T) {
	report := validation.Report{Requested: true, Profile: validation.Profile{ID: ProfileModelSemantics, Target: validation.TargetModel}, Target: validation.TargetModel, InputComplete: true, Syntax: validation.SyntaxNotAssessed, Conformance: validation.ConformanceConformant, AssessmentComplete: true}
	if !report.Satisfied() {
		t.Fatal("invalid successful-report control")
	}
	if err := modelAcceptanceError(report, modelWalkStatus{}); err != nil {
		t.Fatal(err)
	}
	// The walker records a discovered representation problem before admitting its
	// diagnostic. Its independent status must survive a missing stored finding.
	if err := modelAcceptanceError(report, modelWalkStatus{HasProblems: true}); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("representation failure disappeared: %v", err)
	}
	report.AssessmentComplete = false
	if err := modelAcceptanceError(report, modelWalkStatus{HasProblems: true}); !errors.Is(err, ErrInvalidDocument) || !errors.Is(err, ErrIncompleteAssessment) {
		t.Fatalf("independent invalid/incomplete axes lost: %v", err)
	}
	if err := modelAcceptanceError(validation.Report{}, modelWalkStatus{}); !errors.Is(err, ErrIncompleteAssessment) {
		t.Fatalf("unrequested model accepted: %v", err)
	}
}

func TestAssessedLoadingAllDocumentRoots(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"partwise", mandatoryScore("1", `<note><rest/><duration>1</duration></note>`)},
		{"timewise", `<score-timewise version="4.0"><part-list><score-part id="P1"><part-name>Music</part-name></score-part></part-list><measure number="1"><part id="P1"><attributes><divisions>1</divisions></attributes><note><rest/><duration>1</duration></note></part></measure></score-timewise>`},
		{"opus", `<opus version="4.0"><title>Collection</title><opus><title>Nested</title></opus></opus>`},
	} {
		for _, explicit := range []bool{false, true} {
			t.Run(tc.name+"/"+mandatoryLoaderName(explicit), func(t *testing.T) {
				result, err := mandatoryDecode(explicit, context.Background(), strings.NewReader(tc.source), nil)
				if err != nil || result.Document == nil || !result.Source.Satisfied() || !result.Model.Satisfied() {
					t.Fatalf("document=%T source=%v model=%v err=%v", result.Document, result.Source.Satisfied(), result.Model.Satisfied(), err)
				}
				switch tc.name {
				case "partwise":
					if _, ok := result.Document.(*ScorePartwise); !ok {
						t.Fatalf("wrong root %T", result.Document)
					}
				case "timewise":
					if _, ok := result.Document.(*ScoreTimewise); !ok {
						t.Fatalf("wrong root %T", result.Document)
					}
				case "opus":
					if _, ok := result.Document.(*OpusDocument); !ok {
						t.Fatalf("wrong root %T", result.Document)
					}
				}
			})
		}
	}
}
