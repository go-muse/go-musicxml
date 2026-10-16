package musicxml_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
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
	if n, err := first.Body.Read(prefix); n != 0 || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("closed read: n=%d err=%v", n, err)
	}
	raw, err := io.ReadAll(second.Body)
	if err != nil || int64(len(raw)) != q.Component.Bytes || fmt.Sprintf("%x", sha256.Sum256(raw)) != q.Component.SHA256 {
		t.Fatalf("independent body changed: bytes=%d err=%v", len(raw), err)
	}
	// A consumer may mutate its read buffer; no returned slice aliases the asset.
	for i := range raw {
		raw[i] = 0
	}
	third, err := musicxml.BundledDictionarySource().Open(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	defer third.Body.Close()
	raw, err = io.ReadAll(third.Body)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(raw)) != q.Component.SHA256 {
		t.Fatal("consumer mutated shared embedded data")
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
					result, e := musicxml.DecodeAssessedWithDictionaries(context.Background(), strings.NewReader(score), musicxml.DecodeDictionaryAssessmentOptions{Assessment: &o})
					report, err = result.Source, e
					if tc.known && result.Document == nil {
						t.Fatal("known dictionary prevented decoding")
					}
					if !tc.known && errors.Is(err, musicxml.ErrIncompleteAssessment) {
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
