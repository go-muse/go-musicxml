package musicxml_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"testing"

	musicxml "github.com/go-muse/go-musicxml"
	"github.com/go-muse/go-musicxml/validation"
)

var bundledManifestIDs = []validation.DictionaryManifestID{
	validation.DictionaryManifestSMuFL14Canonical,
	validation.DictionaryManifestSMuFL14Ranges,
	validation.DictionaryManifestStandardSounds40,
	validation.DictionaryManifestIPAPositive2015Unicode13,
	validation.DictionaryManifestIPARepresentation2015Unicode13,
}

func bundledRequest(t *testing.T, id validation.DictionaryManifestID) musicxml.DictionaryComponentRequest {
	t.Helper()
	m, ok := validation.LookupDictionaryManifest(id)
	if !ok {
		t.Fatal(id)
	}
	return musicxml.DictionaryComponentRequest{Manifest: id, Component: m.Component}
}

func TestBundledDictionarySourcePinnedBytesWithoutFiles(t *testing.T) {
	// None of the source fixtures or generated assets are available in cwd.
	t.Chdir(t.TempDir())
	source := musicxml.BundledDictionarySource()
	for _, id := range bundledManifestIDs {
		t.Run(string(id), func(t *testing.T) {
			q := bundledRequest(t, id)
			r, err := source.Open(context.Background(), q)
			if err != nil || r.Request != q || r.State != validation.EvidenceKnown || r.Reason != "" || r.Body == nil {
				t.Fatalf("result=%+v err=%v", r, err)
			}
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if err := r.Body.Close(); err != nil {
				t.Fatal(err)
			}
			if int64(len(raw)) != q.Component.Bytes || fmt.Sprintf("%x", sha256.Sum256(raw)) != q.Component.SHA256 {
				t.Fatal("embedded bytes differ from the existing pin")
			}
		})
	}
}

func TestBundledDictionarySourceRejectsNonexactRequests(t *testing.T) {
	q := bundledRequest(t, validation.DictionaryManifestSMuFL14Canonical)
	for _, tc := range []struct {
		name   string
		change func(*musicxml.DictionaryComponentRequest)
	}{
		{"manifest", func(q *musicxml.DictionaryComponentRequest) { q.Manifest = "latest" }},
		{"mismatched manifest", func(q *musicxml.DictionaryComponentRequest) { q.Manifest = validation.DictionaryManifestSMuFL14Ranges }},
		{"component", func(q *musicxml.DictionaryComponentRequest) { q.Component.ID = "unknown" }},
		{"mismatched component", func(q *musicxml.DictionaryComponentRequest) {
			q.Component = bundledRequest(t, validation.DictionaryManifestSMuFL14Ranges).Component
		}},
		{"length", func(q *musicxml.DictionaryComponentRequest) { q.Component.Bytes++ }},
		{"digest", func(q *musicxml.DictionaryComponentRequest) { q.Component.SHA256 = strings.Repeat("0", 64) }},
		{"url", func(q *musicxml.DictionaryComponentRequest) { q.Component.URL += "?changed" }},
		{"source kind", func(q *musicxml.DictionaryComponentRequest) {
			q.Component.SourceKind = validation.DictionarySourceReviewedDerivedBundle
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := q
			tc.change(&changed)
			r, err := musicxml.BundledDictionarySource().Open(context.Background(), changed)
			if !errors.Is(err, validation.ErrInvalidConfiguration) || r.Body != nil || r.State == validation.EvidenceKnown {
				t.Fatalf("result=%+v err=%v", r, err)
			}
		})
	}
}

func TestBundledDictionarySourceContextAndIndependentOwnership(t *testing.T) {
	q := bundledRequest(t, validation.DictionaryManifestStandardSounds40)
	source := musicxml.BundledDictionarySource()
	if r, err := source.Open(nil, q); !errors.Is(err, validation.ErrInvalidConfiguration) || r.Body != nil {
		t.Fatalf("nil context: result=%+v err=%v", r, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, err := source.Open(ctx, q); !errors.Is(err, context.Canceled) || r.Body != nil {
		t.Fatalf("canceled open: result=%+v err=%v", r, err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	first, err := source.Open(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	second, err := source.Open(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Body.Close()
	prefix := make([]byte, 17)
	if _, err := io.ReadFull(first.Body, prefix); err != nil {
		t.Fatal(err)
	}
	cancel()
	if n, err := first.Body.Read(prefix); n != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled read: n=%d err=%v", n, err)
	}
	if err := first.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if n, err := first.Body.Read(prefix); n != 0 || !errors.Is(err, fs.ErrClosed) {
		t.Fatalf("closed read: n=%d err=%v", n, err)
	}
	raw, err := io.ReadAll(second.Body)
	if err != nil || int64(len(raw)) != q.Component.Bytes || fmt.Sprintf("%x", sha256.Sum256(raw)) != q.Component.SHA256 {
		t.Fatalf("independent body changed: bytes=%d err=%v", len(raw), err)
	}
}

func TestBundledDictionaryPublicAssessmentPreservesSelection(t *testing.T) {
	const score = `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list><part id="P1"><measure number="1"><note><rest/><duration>1</duration><accidental smufl="accidentalSharp">sharp</accidental></note></measure></part></score-partwise>`
	m, _ := validation.LookupDictionaryManifest(validation.DictionaryManifestSMuFL14Canonical)
	selection := []validation.DictionarySelection{{Manifest: m.ID, Relations: []validation.DictionaryRelation{validation.DictionaryRelationCanonicalName}}}
	for _, tc := range []struct {
		name    string
		options musicxml.DictionaryOptions
		known   bool
	}{
		{"explicit", musicxml.DictionaryOptions{Selections: selection, Source: musicxml.BundledDictionarySource()}, true},
		{"unselected", musicxml.DictionaryOptions{Source: musicxml.BundledDictionarySource()}, false},
		{"nil source", musicxml.DictionaryOptions{Selections: selection}, false},
	} {
		for _, mode := range []string{"source", "model", "assessed decode"} {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				o := musicxml.DictionaryAssessmentOptions{Assessment: musicxml.AssessmentOptions{Profile: musicxml.ProfileSourceDictionaries}, Dictionaries: tc.options}
				var report validation.Report
				var err error
				switch mode {
				case "source":
					report, err = musicxml.InspectXMLWithDictionaries(context.Background(), strings.NewReader(score), o)
				case "model":
					doc, e := musicxml.Decode(strings.NewReader(score))
					if e != nil {
						t.Fatal(e)
					}
					o.Assessment.Profile = musicxml.ProfileModelDictionaries
					report, err = musicxml.AssessModelWithDictionaries(context.Background(), doc, o)
				case "assessed decode":
					o.Assessment.Profile = musicxml.ProfileSourceSemantics
					if len(o.Dictionaries.Selections) != 0 {
						o.Dictionaries.Selections = assessedDecodeDictionarySelections()
					}
					reader := strings.NewReader(score)
					result, e := musicxml.DecodeAssessedWithDictionaries(context.Background(), reader, musicxml.DecodeDictionaryAssessmentOptions{Assessment: &o})
					if tc.name == "unselected" {
						if !errors.Is(e, validation.ErrInvalidConfiguration) || result.Document != nil || result.Source.Requested || result.Model.Requested || reader.Len() != len(score) {
							t.Fatalf("incomplete explicit selection was not rejected before input: %+v %v", result, e)
						}
						return
					}
					report, err = result.Source, e
					if tc.known && (result.Document == nil || !result.Source.Satisfied() || !result.Model.Satisfied()) {
						t.Fatalf("known dictionary prevented source/model acceptance: %+v %v", result, err)
					}
					if !tc.known {
						if !errors.Is(err, musicxml.ErrIncompleteAssessment) || result.Document != nil || result.Model.Requested {
							t.Fatalf("unavailable explicit provider was repaired or accepted: %+v %v", result, err)
						}
						err = nil
					}
				}
				if err != nil {
					t.Fatal(err)
				}
				found := 0
				for _, c := range report.Context {
					if c.Property != validation.ContextDictionaryCanonical || c.Dictionary == nil {
						continue
					}
					found++
					if (c.Dictionary.Result.State == validation.EvidenceKnown) != tc.known || (tc.known && (!c.Dictionary.Result.Value || !c.Dictionary.ComponentVerified)) {
						t.Fatalf("dictionary evidence=%+v", c.Dictionary)
					}
				}
				if found != 1 {
					t.Fatalf("canonical evidence count=%d", found)
				}
			})
		}
	}
}

func TestBundledDictionaryPublicEvidenceBoundaries(t *testing.T) {
	t.Run("custom standard sound remains allowed", func(t *testing.T) {
		m, _ := validation.LookupDictionaryManifest(validation.DictionaryManifestStandardSounds40)
		report, err := musicxml.InspectXMLWithDictionaries(context.Background(), strings.NewReader(`<sounds><sound id="keyboard.piano"><any/></sound><sound id="custom.allowed"><solo/></sound></sounds>`), musicxml.DictionaryAssessmentOptions{
			Assessment:   musicxml.AssessmentOptions{Profile: musicxml.ProfileSourceSoundsDictionaries},
			Dictionaries: musicxml.DictionaryOptions{Source: musicxml.BundledDictionarySource(), Selections: []validation.DictionarySelection{{Manifest: m.ID, Relations: m.Relations}}},
		})
		if err != nil || report.Conformance == validation.ConformanceViolated {
			t.Fatalf("report=%+v err=%v", report, err)
		}
		found := 0
		for _, c := range report.Context {
			if c.Property != validation.ContextDictionarySounds || c.Dictionary == nil {
				continue
			}
			found++
			d := c.Dictionary
			if !d.ComponentVerified || d.Result.State != validation.EvidenceKnown || d.Result.Value != (d.Input.Value.Text == "keyboard.piano") {
				t.Fatalf("sound evidence=%+v", d)
			}
		}
		if found != 2 {
			t.Fatalf("sound evidence count=%d", found)
		}
	})
	for _, tc := range []struct {
		name, value string
		manifest    validation.DictionaryManifestID
		property    validation.ContextProperty
		known       bool
	}{
		{"legacy positive witness", "a", validation.DictionaryManifestIPAPositive2015Unicode13, validation.ContextDictionaryIPA, true},
		{"legacy partial gap", "?", validation.DictionaryManifestIPAPositive2015Unicode13, validation.ContextDictionaryIPA, false},
		{"representation witness", "ḉ\u0334", validation.DictionaryManifestIPARepresentation2015Unicode13, validation.ContextDictionaryIPARepresentation, true},
		{"representation partial gap", "a\u0327", validation.DictionaryManifestIPARepresentation2015Unicode13, validation.ContextDictionaryIPARepresentation, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := validation.LookupDictionaryManifest(tc.manifest)
			relations := m.Relations
			if tc.manifest == validation.DictionaryManifestIPAPositive2015Unicode13 {
				relations = []validation.DictionaryRelation{validation.DictionaryRelationIPASymbol}
			}
			score := `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list><part id="P1"><measure number="1"><note><rest/><duration>1</duration><play><ipa>` + tc.value + `</ipa></play></note></measure></part></score-partwise>`
			report, err := musicxml.InspectXMLWithDictionaries(context.Background(), strings.NewReader(score), musicxml.DictionaryAssessmentOptions{
				Assessment:   musicxml.AssessmentOptions{Profile: musicxml.ProfileSourceDictionaries},
				Dictionaries: musicxml.DictionaryOptions{Source: musicxml.BundledDictionarySource(), Selections: []validation.DictionarySelection{{Manifest: m.ID, Relations: relations}}},
			})
			if err != nil || report.Conformance == validation.ConformanceViolated {
				t.Fatalf("report=%+v err=%v", report, err)
			}
			found := 0
			for _, c := range report.Context {
				if c.Property != tc.property || c.Dictionary == nil {
					continue
				}
				d := c.Dictionary
				if d.Manifest != tc.manifest || (tc.manifest == validation.DictionaryManifestIPAPositive2015Unicode13 && d.Relation != validation.DictionaryRelationIPASymbol) {
					continue
				}
				found++
				if !d.ComponentVerified || (d.Result.State == validation.EvidenceKnown) != tc.known || (tc.known && !d.Result.Value) || d.Input.Value.Text != tc.value {
					t.Fatalf("IPA evidence=%+v", d)
				}
				if !tc.known && d.Result.State != validation.EvidenceUnknown {
					t.Fatalf("partial gap became negative evidence: %+v", d)
				}
				if tc.manifest == validation.DictionaryManifestIPAPositive2015Unicode13 && (d.WholeSequence.State != validation.EvidenceUnsupported || report.Conformance != validation.ConformanceUndetermined) {
					t.Fatalf("legacy positive witness became grammar evidence: %+v", d)
				}
			}
			if found != 1 {
				t.Fatalf("IPA evidence count=%d", found)
			}
		})
	}
}
