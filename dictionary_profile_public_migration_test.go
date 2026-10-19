package musicxml

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// This capture was executed from published main 5295080. It preserves complete
// profiles rather than treating the old digest as a failure-message annotation.
func TestDictionaryProfilesPublishedBaselineMigration(t *testing.T) {
	raw, err := os.ReadFile("testdata/validation/pr41-public-dictionary-profile-baseline.json")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != "6467d2d8e547270cc8b770dffb610f0f6ad0589b788605aa3c4e450c58c426b9" {
		t.Fatal("published dictionary baseline capture changed")
	}
	var capture struct {
		Commit   string               `json:"commit"`
		Profiles []validation.Profile `json:"profiles"`
	}
	if err := json.Unmarshal(raw, &capture); err != nil {
		t.Fatal(err)
	}
	if capture.Commit != "5295080b2030be6d616c714e7c8bcad47185c742" || len(capture.Profiles) != 3 {
		t.Fatal("wrong published dictionary baseline")
	}
	baseline := make(map[validation.ProfileID]validation.Profile)
	for _, p := range capture.Profiles {
		if _, exists := baseline[p.ID]; exists {
			t.Fatalf("duplicate baseline %s", p.ID)
		}
		baseline[p.ID] = p
	}
	for _, id := range []validation.ProfileID{ProfileSourceDictionaries, ProfileModelDictionaries, ProfileSourceSoundsDictionaries} {
		target := validation.TargetSource
		var engine *validation.Engine
		if id == ProfileModelDictionaries {
			target = validation.TargetModel
			engine, err = modelAssessmentEngineForProfile(id)
		} else {
			engine, err = sourceAssessmentEngineForProfile(id)
		}
		if err != nil {
			t.Fatal(err)
		}
		old, ok := baseline[id]
		if !ok || old.Target != target || old.CatalogDigest != "21c731b5d19ebf918bff42fb3a15cac2fae6053b5b8006faf3f32b6108a8db21" {
			t.Fatalf("wrong public dictionary baseline %s: %+v", id, old)
		}
		current, ok := engine.Profile(id)
		if !ok || current.Target != target || current.CatalogDigest != migrateValidationLinesCatalog(migrateDecimalContractCatalog("d8199be864c2baa081aca5887ca0ac188052ba22f6a690ea9bb3b436dc054c57")) {
			t.Fatalf("wrong current dictionary migration %s: %+v", id, current)
		}
	}
}
