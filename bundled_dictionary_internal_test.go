package musicxml

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestBundledDictionaryImportAllComponentsRetainsAccounting(t *testing.T) {
	ids := []validation.DictionaryManifestID{
		validation.DictionaryManifestSMuFL14Canonical, validation.DictionaryManifestSMuFL14Ranges,
		validation.DictionaryManifestStandardSounds40, validation.DictionaryManifestIPAPositive2015Unicode13,
		validation.DictionaryManifestIPARepresentation2015Unicode13,
	}
	o := DictionaryOptions{}
	var wantBytes int64
	for _, id := range ids {
		o.Selections = append(o.Selections, dictionaryImportSelection(id))
		m, _ := validation.LookupDictionaryManifest(id)
		wantBytes += m.Component.Bytes
	}
	var fingerprint [32]byte
	for _, source := range []DictionarySource{
		BundledDictionarySource(),
		dictionaryImportSourceFunc(func(_ context.Context, q DictionaryComponentRequest) (DictionaryComponentResult, error) {
			var raw []byte
			if q.Manifest == validation.DictionaryManifestIPARepresentation2015Unicode13 {
				var err error
				raw, err = os.ReadFile("testdata/dictionaries/ipa-representation/runtime-representation-bundle.json")
				if err != nil {
					t.Fatal(err)
				}
			} else {
				raw = dictionaryImportFixture(t, q.Manifest)
			}
			return DictionaryComponentResult{Request: q, State: validation.EvidenceKnown, Body: io.NopCloser(bytes.NewReader(raw))}, nil
		}),
	} {
		a := dictionaryImportAccount(t, validation.Budgets{})
		var bodies []io.ReadCloser
		o.Source = dictionaryImportSourceFunc(func(ctx context.Context, q DictionaryComponentRequest) (DictionaryComponentResult, error) {
			r, err := source.Open(ctx, q)
			bodies = append(bodies, r.Body)
			return r, err
		})
		set, err := loadDictionarySet(context.Background(), a, o)
		if err != nil || set == nil {
			t.Fatalf("set=%v err=%v", set, err)
		}
		if len(bodies) != len(ids) {
			t.Fatalf("opens=%d", len(bodies))
		}
		if n, _ := a.Usage(validation.BudgetExternalBytes); n != wantBytes {
			t.Fatalf("external=%d want=%d", n, wantBytes)
		}
		got, err := set.Fingerprint()
		if err != nil {
			t.Fatal(err)
		}
		if fingerprint != ([32]byte{}) && got != fingerprint {
			t.Fatalf("bundled and caller-supplied identity differ: %x != %x", fingerprint, got)
		}
		fingerprint = got
		if err := set.Close(); err != nil {
			t.Fatal(err)
		}
		dictionaryImportNoLive(t, a)
	}
}

func TestBundledDictionaryImporterStillVerifiesAndCloses(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		a := dictionaryImportAccount(t, validation.Budgets{})
		var body io.ReadCloser
		o := DictionaryOptions{
			Selections: []validation.DictionarySelection{dictionaryImportSelection(validation.DictionaryManifestStandardSounds40)},
			Source: dictionaryImportSourceFunc(func(ctx context.Context, q DictionaryComponentRequest) (DictionaryComponentResult, error) {
				r, err := BundledDictionarySource().Open(ctx, q)
				body = r.Body
				if corrupt {
					r.Body = &bundledCorruptReader{ReadCloser: r.Body}
				}
				return r, err
			}),
		}
		set, err := loadDictionarySet(context.Background(), a, o)
		if set == nil || (corrupt != errors.Is(err, ErrDictionaryUnavailable)) || (!corrupt && err != nil) {
			t.Fatalf("corrupt=%v set=%v err=%v", corrupt, set, err)
		}
		if _, err := body.Read(make([]byte, 1)); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("importer did not close body: %v", err)
		}
		set.Close()
		dictionaryImportNoLive(t, a)
	}
}

type bundledCorruptReader struct {
	io.ReadCloser
	changed bool
}

func (r *bundledCorruptReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 && !r.changed {
		p[0] ^= 1
		r.changed = true
	}
	return n, err
}

func TestBundledDictionaryImportHonorsExternalByteLimit(t *testing.T) {
	m, _ := validation.LookupDictionaryManifest(validation.DictionaryManifestStandardSounds40)
	a := dictionaryImportAccount(t, validation.Budgets{MaxExternalBytes: m.Component.Bytes - 1})
	set, err := loadDictionarySet(context.Background(), a, DictionaryOptions{
		Selections: []validation.DictionarySelection{dictionaryImportSelection(m.ID)}, Source: BundledDictionarySource(),
	})
	if set != nil || !errors.Is(err, validation.ErrResourceLimit) {
		t.Fatalf("set=%v err=%v", set, err)
	}
	dictionaryImportNoLive(t, a)
}
