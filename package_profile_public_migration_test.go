package musicxml

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// This fixture was executed and captured from published main 5295080, not an
// unpublished intermediate checkpoint. Its complete profile payloads preserve
// the original closure text, rule selection and catalog identities independently
// of the implementation under test. CatalogDigest is not a Profile JSON hash.
func TestPackageProfilesPublishedBaselineMigration(t *testing.T) {
	raw, err := os.ReadFile("testdata/validation/pr41-public-package-profile-baseline.json")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != "5076e5bdd573e268f335178d283b36dd754f3bb80a3bc41b8ca5d21fe86880dd" {
		t.Fatal("published baseline capture changed")
	}
	var capture struct {
		Commit   string               `json:"commit"`
		Profiles []validation.Profile `json:"profiles"`
	}
	if err := json.Unmarshal(raw, &capture); err != nil {
		t.Fatal(err)
	}
	if capture.Commit != "5295080b2030be6d616c714e7c8bcad47185c742" || len(capture.Profiles) != 6 {
		t.Fatalf("wrong public baseline: %+v", capture)
	}
	baseline := make(map[validation.ProfileID]validation.Profile)
	for _, p := range capture.Profiles {
		if _, exists := baseline[p.ID]; exists {
			t.Fatalf("duplicate baseline %s", p.ID)
		}
		baseline[p.ID] = p
	}
	for _, tc := range []struct {
		factory       func() (*validation.CompiledPackageCatalog, error)
		ids           []validation.ProfileID
		before, after string
	}{
		{newPackageAssessmentCatalog, []validation.ProfileID{ProfileResourceGraph, ProfileMXLSource, ProfileMXLModel}, "083b1b94f8b581825b23b81e3c7e39c238741f04f897ba71f7aa9354ea789d97", "554a97e25d6494c2e0550519fd4f6d2a439ca2f31e01a82baa7e28a00ddd2088"},
		{newDictionaryPackageAssessmentCatalog, []validation.ProfileID{ProfileResourceGraphDictionaries, ProfileMXLSourceDictionaries, ProfileMXLModelDictionaries}, "14e5cdb4bc6b9fd2b7315f0059255fd8fc14fd43217e46238b1b64fd503b1333", "f80db03b147d43021db39d1762880c2ef8e83967c1002c37b92229eb0feabd44"},
	} {
		c, err := tc.factory()
		if err != nil {
			t.Fatal(err)
		}
		after := migrateValidationLinesCatalog(migrateDecimalContractCatalog(migrateCurrentStatePackageCatalog(t, tc.after)))
		for _, id := range tc.ids {
			old, ok := baseline[id]
			if !ok || old.CatalogDigest != tc.before || old.Target != validation.TargetPackage {
				t.Fatalf("incorrect published baseline %s: %+v", id, old)
			}
			p, ok := c.Profile(id)
			if !ok || p.CatalogDigest != after || p.Target != old.Target {
				t.Fatalf("current catalog migration %s: %+v", id, p)
			}
			if p.CatalogDigest == old.CatalogDigest {
				t.Fatalf("missing intentional migration %s", id)
			}
		}
	}
}
