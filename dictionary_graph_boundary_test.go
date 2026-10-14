package musicxml

import (
	"context"
	"errors"
	"testing"
	"unsafe"

	"github.com/go-muse/go-musicxml/validation"
)

func TestDictionaryGraphTypedNilBeforeProvider(t *testing.T) {
	var reader *dictionaryGraphReader
	calls := 0
	source := dictionaryGraphSourceFunc(func(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error) {
		calls++
		return DictionaryComponentResult{}, errors.New("must not import")
	})
	d := DictionaryOptions{Selections: dictionaryGraphSelection(), Source: source}
	for _, run := range []func() error{
		func() error {
			_, e := InspectXMLWithResourcesAndDictionaries(context.Background(), reader, LinkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraphDictionaries}}, Dictionaries: d})
			return e
		},
		func() error {
			_, e := InspectMXLWithDictionaries(context.Background(), reader, MXLDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileMXLSourceDictionaries}}, Dictionaries: d})
			return e
		},
	} {
		if e := run(); !errors.Is(e, ErrNilReader) {
			t.Fatalf("got %v", e)
		}
	}
	if calls != 0 {
		t.Fatalf("provider called %d times", calls)
	}
}

func TestDictionaryGraphFixedRetentionEnvelopes(t *testing.T) {
	// These are the fixed Go values represented by each existing logical charge;
	// dynamic strings are separately charged. Two copies plus fixed map metadata
	// fit the historical cache envelope even with the 32-byte capability key.
	fixed := unsafe.Sizeof(graphResourceKey{}) + unsafe.Sizeof(graphResource{}) + unsafe.Sizeof(graphCanonicalKey{}) + unsafe.Sizeof(validation.ResourcePin{}) + unsafe.Sizeof((*graphResource)(nil))
	if need := 2*fixed + 256; need > 1040 {
		t.Fatalf("cache fixed envelope needs %d, has1040", need)
	}
	if need := 2*unsafe.Sizeof(resourceGraphRun{}) + 512; need > 2048 {
		t.Fatalf("root fixed envelope needs %d, has2048", need)
	}
	t.Logf("fixed cache values=%d root=%d", fixed, unsafe.Sizeof(resourceGraphRun{}))
}

func TestDictionaryGraphCacheExactRetentionAdmission(t *testing.T) {
	pin := validation.ResourcePin{CanonicalID: "resource", Representation: validation.ResourceSourceXML}
	profile := ProfileSourceDictionaries
	need := int64(1040 + 2*(len(pin.CanonicalID)+len(profile)))
	for _, limit := range []int64{need - 1, need} {
		a, err := validation.NewBudgetAccount(validation.Budgets{MaxBufferedBytes: limit})
		if err != nil {
			t.Fatal(err)
		}
		g := &resourceGraphRun{ctx: context.Background(), options: resourceRunOptions{account: a}, cache: map[graphResourceKey]*graphResource{}, pins: map[graphCanonicalKey]validation.ResourcePin{}, dictionaryFingerprint: [32]byte{1}}
		r, err := g.cacheResource(pin, profile, "id", 1, true)
		if limit < need {
			if r != nil || !errors.Is(err, validation.ErrResourceLimit) || len(g.cache) != 0 || len(g.pins) != 0 || g.retained != 0 {
				t.Fatalf("partial cache on refusal: r=%v err=%v retained=%d", r, err, g.retained)
			}
		} else {
			if err != nil || r == nil || g.retained != need {
				t.Fatalf("exact admission: %v retained=%d", err, g.retained)
			}
			if again, e := g.cacheResource(pin, profile, "alias", 1, true); e != nil || again != r || g.retained != need {
				t.Fatalf("cache reuse recharged: %v", e)
			}
		}
		g.close()
		if used, e := a.Usage(validation.BudgetBufferedBytes); e != nil || used != 0 {
			t.Fatalf("cleanup used=%d err=%v", used, e)
		}
	}
}
