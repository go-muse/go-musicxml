package musicxml

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestContextOpusLocalPaginationAcrossTargets(t *testing.T) {
	for _, attr := range []string{"", ` new-page="yes"`, ` new-page="no"`, ` new-page="maybe"`} {
		xml := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="https://invalid.example/never-fetch"` + attr + `/><opus><score xlink:href="nested.musicxml" new-page="yes"/></opus></opus>`
		doc, err := Decode(strings.NewReader(xml))
		if err != nil {
			t.Fatal(err)
		}
		for _, model := range []bool{false, true} {
			var r validation.Report
			if model {
				r, err = AssessModel(context.Background(), doc, ModelOptions{Profile: ProfileModelContext})
			} else {
				r, err = InspectXML(context.Background(), strings.NewReader(xml), SourceOptions{Profile: ProfileSourceContext})
			}
			if err != nil {
				t.Fatal(err)
			}
			var records []validation.ContextResult
			for _, v := range r.Context {
				if v.Property == "layout.opus-new-page" {
					records = append(records, v)
				}
			}
			if len(records) != 2 {
				t.Fatalf("model%t %s: pagination records=%d", model, attr, len(records))
			}
			want := validation.EvidenceKnown
			if attr == "" {
				want = validation.EvidenceUnknown
			}
			if strings.Contains(attr, "maybe") {
				want = validation.EvidenceInvalid
			}
			if records[0].Value.State != want {
				t.Errorf("%s: %+v", attr, records[0].Value)
			}
			if want == validation.EvidenceInvalid && records[0].Value.Cause == nil {
				t.Error("invalid prerequisite lost cause")
			}
			if records[1].Value.State != validation.EvidenceKnown || records[1].Value.Value.Text != "yes" {
				t.Error("nested score lost explicit pagination")
			}
			if records[0].Subject == records[1].Subject {
				t.Error("nested score identity collapsed")
			}
		}
	}
}

func TestContextOpusAssemblyContract(t *testing.T) {
	d, err := buildModelAssessmentDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	opus := d.schemas[validation.AssemblyOpus]
	music := d.schemas[validation.AssemblyMusicXML]
	if _, err = validationNewOpusLayoutContextPlan(music); !errors.Is(err, validation.ErrInvalidConfiguration) {
		t.Fatalf("Opus plan accepted MusicXML: %v", err)
	}
	if _, err = validationNewLocalContextPlan(opus); !errors.Is(err, validation.ErrInvalidConfiguration) {
		t.Fatalf("MusicXML plan accepted Opus: %v", err)
	}
	plan, err := validationNewOpusLayoutContextPlan(opus)
	if err != nil {
		t.Fatal(err)
	}
	wrong := validation.AssemblyMusicXML
	rule := validation.RuleDefinition{Key: validation.RuleKey{ID: "opus-test", Revision: 1}, Class: validation.ClassMandatory, Role: validation.RoleContextProvider, Category: validation.CategoryNormative, Targets: []validation.Target{validation.TargetModel}, Scope: validation.ScopeDocument, Selector: validation.Selector{Assembly: &wrong}, Predicate: "opus-test", Text: validation.RuleText{Title: "test", Explanation: "test", Message: "test"}}
	if _, err = validation.NewCatalog([]validation.RuleDefinition{rule}, nil, []validation.PlanBinding{{ID: "opus-test", Plan: plan}}); !errors.Is(err, validation.ErrInvalidConfiguration) {
		t.Fatalf("wrong assembly selector accepted: %v", err)
	}
	for _, model := range []bool{false, true} {
		var r validation.Report
		if model {
			r, err = AssessModel(context.Background(), NewOpusDocument(), ModelOptions{Profile: ProfileModelContext})
		} else {
			r, err = InspectXML(context.Background(), strings.NewReader(`<opus/>`), SourceOptions{Profile: ProfileSourceContext})
		}
		if err != nil || !r.Satisfied() || len(r.Context) != 1 || r.Context[0].Property != "layout.opus-version" {
			t.Fatalf("empty opus fabricated context: %v %+v", err, r)
		}
	}
}

func TestContextOpusProjectionOwnershipAndLimits(t *testing.T) {
	xml := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="a"/><score xlink:href="b" new-page="no"/></opus>`
	run := func(b validation.Budgets) (validation.Report, error) {
		return InspectXML(context.Background(), strings.NewReader(xml), SourceOptions{Profile: ProfileSourceContext, Budgets: b})
	}
	first, err := run(validation.Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := run(validation.Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	first.Context[0].Layout.Scope.DocumentID = "caller"
	first.Context[0].Origins[0].Path = "caller"
	first.Context[0].Value.Reason = "caller"
	third, err := run(validation.Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(second.Context, third.Context) {
		t.Fatal("caller mutation entered a later context report")
	}
	limited, err := run(validation.Budgets{MaxContextFacts: 1})
	if !errors.Is(err, validation.ErrResourceLimit) || limited.Satisfied() {
		t.Fatalf("context quota lost: %v %+v", err, limited)
	}
	if len(limited.Context) > 1 {
		t.Fatal("quota admitted excess projection")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = InspectXML(ctx, strings.NewReader(xml), SourceOptions{Profile: ProfileSourceContext}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestContextOpusVersionDefaultIsLocal(t *testing.T) {
	for _, attr := range []string{"", ` version="4.0"`, ` version=" future   revision "`, ` version=""`} {
		xml := `<opus` + attr + `><opus version="2.0"/></opus>`
		doc, err := Decode(strings.NewReader(xml))
		if err != nil {
			t.Fatal(err)
		}
		for _, model := range []bool{false, true} {
			var r validation.Report
			if model {
				r, err = AssessModel(context.Background(), doc, ModelOptions{Profile: ProfileModelContext})
			} else {
				r, err = InspectXML(context.Background(), strings.NewReader(xml), SourceOptions{Profile: ProfileSourceContext})
			}
			if err != nil {
				t.Fatal(err)
			}
			var values []validation.ContextResult
			for _, v := range r.Context {
				if v.Property == "layout.opus-version" {
					values = append(values, v)
				}
			}
			if len(values) != 2 {
				t.Fatalf("model%t %q: version facts=%d", model, attr, len(values))
			}
			want := "1.0"
			switch attr {
			case ` version="4.0"`:
				want = "4.0"
			case ` version=" future   revision "`:
				want = "future revision"
			case ` version=""`:
				want = ""
			}
			if values[0].Value.State != validation.EvidenceKnown || values[0].Value.Value.Text != want {
				t.Fatalf("version %+v want%q", values[0].Value, want)
			}
			if values[1].Value.Value.Text != "2.0" || values[0].Subject == values[1].Subject {
				t.Fatal("nested version scope lost")
			}
			if attr == "" && (values[0].Layout.Raw.State != validation.EvidenceAbsent || values[0].Layout.Provenance != validation.ContextDocumentedDefault) {
				t.Fatalf("default provenance %+v", values[0].Layout)
			}
		}
	}
}
