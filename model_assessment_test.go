package musicxml

import (
	"context"
	"encoding/xml"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestAssessModelPreflight(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	var nilPartwise *ScorePartwise
	var nilTimewise *ScoreTimewise
	var nilOpus *OpusDocument
	for _, tt := range []struct {
		name    string
		ctx     context.Context
		doc     Document
		options AssessmentOptions
		want    error
	}{
		{"nil-context", nil, NewOpusDocument(), AssessmentOptions{Profile: ProfileModelXSD}, validation.ErrInvalidConfiguration},
		{"canceled", canceled, NewOpusDocument(), AssessmentOptions{Profile: ProfileModelXSD}, context.Canceled},
		{"empty-profile", context.Background(), NewOpusDocument(), AssessmentOptions{}, validation.ErrInvalidConfiguration},
		{"unknown-profile", context.Background(), NewOpusDocument(), AssessmentOptions{Profile: "unknown"}, validation.ErrInvalidConfiguration},
		{"source-profile", context.Background(), NewOpusDocument(), AssessmentOptions{Profile: ProfileSourceXSDPreview}, validation.ErrInvalidConfiguration},
		{"negative-budget", context.Background(), NewOpusDocument(), AssessmentOptions{Profile: ProfileModelXSD, Budgets: validation.Budgets{MaxNodes: -1}}, validation.ErrInvalidConfiguration},
		{"nil-interface", context.Background(), nil, AssessmentOptions{Profile: ProfileModelXSD}, ErrNilDocument},
		{"typed-nil-partwise", context.Background(), nilPartwise, AssessmentOptions{Profile: ProfileModelXSD}, ErrNilDocument},
		{"typed-nil-timewise", context.Background(), nilTimewise, AssessmentOptions{Profile: ProfileModelXSD}, ErrNilDocument},
		{"typed-nil-opus", context.Background(), nilOpus, AssessmentOptions{Profile: ProfileModelXSD}, ErrNilDocument},
	} {
		t.Run(tt.name, func(t *testing.T) {
			report, err := AssessModel(tt.ctx, tt.doc, tt.options)
			if !errors.Is(err, tt.want) || report.Satisfied() {
				t.Fatalf("report=%+v err=%v, want %v", report, err, tt.want)
			}
		})
	}
}

func modelMinimalScore() *ScorePartwise {
	doc := NewScorePartwise(PartDefinition{ID: "P1", Name: "Piano"})
	measure := NewScorePartwiseMeasure("1")
	measure.AddNote(NewRestNote(1))
	doc.Part[0].AddMeasure(measure)
	return doc
}

func TestAssessModelCurrentTargetAndRootMetadata(t *testing.T) {
	// Assess remains the existing public generated model type.
	_ = Assess{}
	partwise := modelMinimalScore()
	timewise := NewScoreTimewise(PartDefinition{ID: "P1", Name: "Piano"})
	measure := NewScoreTimewiseMeasure("1", "P1")
	measure.Part[0].AddNote(NewRestNote(1))
	timewise.AddMeasure(measure)
	for _, doc := range []Document{partwise, timewise, NewOpusDocument()} {
		switch v := doc.(type) {
		case *ScorePartwise:
			v.XMLName = xml.Name{Local: "\x00", Space: "\xff"}
		case *ScoreTimewise:
			v.XMLName = xml.Name{Local: "\x00", Space: "\xff"}
		case *OpusDocument:
			v.XMLName = xml.Name{Local: "\x00", Space: "\xff"}
		}
		for _, profile := range []validation.ProfileID{ProfileModelXSD, ProfileModelLocal} {
			report, err := AssessModel(context.Background(), doc, AssessmentOptions{Profile: profile})
			if err != nil || !report.Satisfied() || report.Target != validation.TargetModel || report.Syntax != validation.SyntaxNotAssessed || report.Profile.ID != profile {
				t.Fatalf("%T %s: report=%+v err=%v", doc, profile, report, err)
			}
		}
	}
}

func TestAssessModelReportsCurrentEditsWithoutMutation(t *testing.T) {
	doc := modelMinimalScore()
	note := doc.Part[0].Measure[0].Content[0].Note
	note.Rest = nil
	note.Pitch = &Pitch{Step: Step("H"), Octave: 4}
	before := *note.Pitch
	report, err := AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelXSD})
	if err != nil || report.Satisfied() || report.Conformance != validation.ConformanceViolated || !report.InputComplete {
		t.Fatalf("current invalid edit was not assessed: %+v %v", report, err)
	}
	if !reflect.DeepEqual(before, *note.Pitch) {
		t.Fatal("assessment changed the caller's model")
	}
	found := false
	for _, f := range report.Findings {
		if f.Location.Position != nil || f.Location.Target != validation.TargetModel {
			t.Errorf("invented source coordinates: %+v", f.Location)
		}
		if f.Outcome == validation.OutcomeFail && f.Reason == "enumeration" {
			found = true
			if !strings.Contains(f.Location.Path, "Pitch.Step") {
				t.Errorf("not a model field path: %q", f.Location.Path)
			}
		}
	}
	if !found {
		t.Fatal("missing current Step failure")
	}
}
