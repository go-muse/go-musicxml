package musicxml

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Published ordinary package profiles retain the ed3774a capture. The three
// unpublished DTD source profiles intentionally migrate together: their shared
// catalog now elects effective DTD/XSD joined shorthand. These new captures are
// a documented prepublication correction, not a claim of prior digest stability.
func TestDTDPackageCompositionPreservesPublishedProfilesAndMigratesDTDShorthand(t *testing.T) {
	want := map[validation.ProfileID]string{
		ProfileSourceInternalDTDLocal:        "66ade1cc00540aebc746f1a9cc6a1cf0f3515c37d8a8c547093bbe686f5f6ff0",
		ProfileSourceInternalDTDLinks:        "db744ca59d2ea60a0db07ef7e6693fee44ae2148a3481cbf9be80cdfb654bb01",
		ProfileSourceInternalDTDDictionaries: "7ef8ca4eff9b989c8482b2cf7f98ebe8823ea98e40236461246186f59ec7792a",
		ProfileResourceGraph:                 "f0eab5da83c93f08e02a90df65fe37db13e5a00966ccb5ff5659628cea6dd291",
		ProfileMXLSource:                     "a6a6f73f18882b11db2402358f5584f2541d366fcd9b811eeb22b17745fc9b05",
		ProfileMXLModel:                      "4edd959ead2e80905663ac6f508a7ef8cea7337c7a5f90a214e787580d0027d8",
		ProfileResourceGraphDictionaries:     "ac509b748631e72b90f10e6c5f2496d542dd7a906e3bd7c5a80bf825065e9741",
		ProfileMXLSourceDictionaries:         "14a5570d39d1f3c8524a603b5590aba0bfaf95cf9c0b1cc06b9c4cc91523f222",
		ProfileMXLModelDictionaries:          "a04e03d483a3b13eb90b7e0f122268b1091fc5eefb71bae3a16bd23b99d6b7ec",
	}
	check := func(id validation.ProfileID, p validation.Profile) {
		t.Helper()
		b, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		got := fmt.Sprintf("%x", sha256.Sum256(b))
		if got != want[id] {
			t.Errorf("profile %s changed: got %s want %s", id, got, want[id])
		}
	}
	for _, id := range []validation.ProfileID{ProfileSourceInternalDTDLocal, ProfileSourceInternalDTDLinks, ProfileSourceInternalDTDDictionaries} {
		e, err := sourceAssessmentEngineForProfile(id)
		if err != nil {
			t.Fatal(err)
		}
		p, _ := e.Profile(id)
		check(id, p)
	}
	for _, f := range []func() (*validation.CompiledPackageCatalog, error){newPackageAssessmentCatalog, newDictionaryPackageAssessmentCatalog} {
		c, err := f()
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []validation.ProfileID{ProfileResourceGraph, ProfileMXLSource, ProfileMXLModel, ProfileResourceGraphDictionaries, ProfileMXLSourceDictionaries, ProfileMXLModelDictionaries} {
			if p, ok := c.Profile(id); ok {
				check(id, p)
			}
		}
	}
}
