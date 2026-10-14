package musicxml

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestDictionaryAssessmentExplicitProfilesAndConversion(t *testing.T) {
	ctx := context.Background()
	source := `<opus/>`
	report, err := InspectXMLWithDictionaries(ctx, strings.NewReader(source), DictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: ProfileSourceDictionaries}})
	if err != nil || !report.Requested || report.Profile.ID != ProfileSourceDictionaries || report.Target != validation.TargetSource {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	model, err := AssessModelWithDictionaries(ctx, &OpusDocument{}, DictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: ProfileModelDictionaries}})
	if err != nil || !model.Requested || model.Profile.ID != ProfileModelDictionaries || model.Target != validation.TargetModel {
		t.Fatalf("report=%+v err=%v", model, err)
	}
	decoded, err := DecodeAssessedWithDictionaries(ctx, strings.NewReader(source), DecodeDictionaryAssessmentOptions{Assessment: &DictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: ProfileSourceDictionaries}}})
	if err != nil || decoded.Document == nil || !decoded.Source.Requested || decoded.Source.Profile.ID != ProfileSourceDictionaries {
		t.Fatalf("decoded=%+v err=%v", decoded, err)
	}
	unassessed, err := DecodeAssessedWithDictionaries(ctx, strings.NewReader(source), DecodeDictionaryAssessmentOptions{})
	if err != nil || unassessed.Document == nil || unassessed.Source.Requested {
		t.Fatalf("decoded=%+v err=%v", unassessed, err)
	}
}

func TestDictionaryAssessmentTerminalImportBeforeDocument(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		var bodies []*resourceTestBody
		opts := DictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: ProfileSourceDictionaries, Budgets: validation.Budgets{MaxExternalBytes: 33703 + 16}}}
		opts.Dictionaries.Selections = []validation.DictionarySelection{dictionaryImportSelection(validation.DictionaryManifestStandardSounds40), dictionaryImportSelection(validation.DictionaryManifestSMuFL14Canonical)}
		opts.Dictionaries.Source = dictionaryImportSourceFunc(func(_ context.Context, request DictionaryComponentRequest) (DictionaryComponentResult, error) {
			calls++
			body := &resourceTestBody{Reader: bytes.NewReader(dictionaryImportFixture(t, request.Manifest))}
			bodies = append(bodies, body)
			if canceled && calls == 2 {
				cancel()
			}
			return DictionaryComponentResult{Request: request, State: validation.EvidenceKnown, Body: body}, nil
		})
		report, err := InspectXMLWithDictionaries(ctx, &dictionaryUnexpectedRead{t: t}, opts)
		cancel()
		want := validation.ErrResourceLimit
		if canceled {
			want = context.Canceled
		}
		if !errors.Is(err, want) || report.Requested || calls != 2 {
			t.Fatalf("canceled=%v report=%+v err=%v calls=%d", canceled, report, err, calls)
		}
		for _, body := range bodies {
			if body.closed != 1 {
				t.Fatal(body.closed)
			}
		}
	}
}

func TestDictionaryAssessmentOrdinaryImportFailurePreservesXSD(t *testing.T) {
	for _, mismatch := range []bool{false, true} {
		boom := errors.New("dictionary provider unavailable")
		opts := DictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: ProfileSourceDictionaries}}
		opts.Dictionaries.Selections = []validation.DictionarySelection{dictionaryImportSelection(validation.DictionaryManifestSMuFL14Canonical)}
		opts.Dictionaries.Source = dictionaryImportSourceFunc(func(_ context.Context, request DictionaryComponentRequest) (DictionaryComponentResult, error) {
			if !mismatch {
				return DictionaryComponentResult{}, boom
			}
			raw := dictionaryImportFixture(t, request.Manifest)
			raw[0] ^= 1
			return DictionaryComponentResult{Request: request, State: validation.EvidenceKnown, Body: &resourceTestBody{Reader: bytes.NewReader(raw)}}, nil
		})
		report, err := InspectXMLWithDictionaries(context.Background(), strings.NewReader(`<opus unexpected="bad"/>`), opts)
		if err == nil || !report.Requested || !report.InputComplete || report.Counts.Fail == 0 || report.Conformance != validation.ConformanceViolated {
			t.Fatalf("mismatch=%v report=%+v err=%v", mismatch, report, err)
		}
		if !mismatch && !errors.Is(err, boom) {
			t.Fatal(err)
		}
	}
}

func TestDictionaryAssessmentRejectsBeforeInput(t *testing.T) {
	for _, profile := range []validation.ProfileID{"", ProfileSourceLinks, ProfileModelDictionaries} {
		reader := &dictionaryUnexpectedRead{t: t}
		_, err := InspectXMLWithDictionaries(context.Background(), reader, DictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: profile}})
		if !errors.Is(err, validation.ErrInvalidConfiguration) {
			t.Fatalf("profile=%s err=%v", profile, err)
		}
	}
}

type dictionaryUnexpectedRead struct{ t *testing.T }

func (r *dictionaryUnexpectedRead) Read([]byte) (int, error) {
	r.t.Error("invalid configuration read the document")
	return 0, errors.New("unexpected read")
}
