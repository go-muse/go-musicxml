package musicxml

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Intrinsic Profile hashes retain the ccc7402 capture. Package hashes are
// captured independently on ed3774a, before this composition and after the
// source-reviewed local-header correction intentionally changed package text
// and catalog identities. The composition itself changes neither baseline.
func TestDTDPackageCompositionPreservesLegacyProfiles(t *testing.T) {
	want := map[validation.ProfileID]string{
		ProfileSourceInternalDTDLocal:        "2d0c9cf4098d84fbcdaaf21ee15257943dff3f09baa5dc163385b9db2637aa1e",
		ProfileSourceInternalDTDLinks:        "367b8b26ab653cfa933e9ad21950e6f1398c8cbcf3ee8bb9799803cf9fa498f7",
		ProfileSourceInternalDTDDictionaries: "bd29ca59b911e13a4739504730e048db8c9f1b6a949904e27d2bd242000f3cfc",
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
			t.Errorf("legacy profile %s changed: got %s want %s", id, got, want[id])
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
