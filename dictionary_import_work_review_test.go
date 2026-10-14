package musicxml

import (
	"context"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// These are complete import costs, including the core's independently admitted
// immutable copy, for every relation of each selected pin on a fresh account.
// The fixture source uses bytes.Reader, which fills each 4096-byte request;
// provider short/empty reads can consume additional bounded transport steps.
func TestDictionaryImportPinnedWorkCosts(t *testing.T) {
	ids := []validation.DictionaryManifestID{validation.DictionaryManifestSMuFL14Canonical, validation.DictionaryManifestSMuFL14Ranges, validation.DictionaryManifestStandardSounds40, validation.DictionaryManifestIPAPositive2015Unicode13}
	want := map[validation.DictionaryManifestID]int64{
		validation.DictionaryManifestSMuFL14Canonical:         2635787,
		validation.DictionaryManifestSMuFL14Ranges:            1735474,
		validation.DictionaryManifestStandardSounds40:         435365,
		validation.DictionaryManifestIPAPositive2015Unicode13: 210493,
		"all": 5016969,
	}
	for _, id := range append(ids, "all") {
		t.Run(string(id), func(t *testing.T) {
			a := dictionaryImportAccount(t, validation.Budgets{})
			selected := []validation.DictionaryManifestID{id}
			if id == "all" {
				selected = ids
			}
			var calls []DictionaryComponentRequest
			o := DictionaryOptions{Source: dictionaryImportFixtureSource(t, &calls)}
			for _, manifest := range selected {
				o.Selections = append(o.Selections, dictionaryImportSelection(manifest))
			}
			set, err := loadDictionarySet(context.Background(), a, o)
			if err != nil || set == nil {
				t.Fatalf("set=%v err=%v", set, err)
			}
			work, _ := a.Usage(validation.BudgetRuleSteps)
			t.Logf("full component import work: %d", work)
			if work != want[id] {
				t.Fatalf("work=%d want=%d; reassess the documented import cost", work, want[id])
			}
			if err := set.Close(); err != nil {
				t.Fatal(err)
			}
			dictionaryImportNoLive(t, a)
		})
	}
}
