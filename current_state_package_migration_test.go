package musicxml

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Revision 2 of MX40-PACKAGE-schema-hint-pairs applies the pairing to current
// model resources as they would be encoded. Every package catalog shares that
// rule, so each catalog digest and each complete package Profile JSON moves,
// including source and graph profiles whose source-resource behaviour is
// unchanged. Earlier captures keep their guards and migrate through these maps.
var currentStatePackageCatalogDigests = map[string]string{
	"554a97e25d6494c2e0550519fd4f6d2a439ca2f31e01a82baa7e28a00ddd2088": "bcc5d047e4de9f02a61cfdd86f57297f9655006ea0fb6f04ef38628d05dcc28f", // ordinary
	"f80db03b147d43021db39d1762880c2ef8e83967c1002c37b92229eb0feabd44": "bbb60bcd61359b41fda394d95e29d93c42c8c0526b101e98f375b698204d6658", // dictionary
	"69d813b09c15ea5083d382fe8864a41179e93cf52c4b04bfb140493070df7648": "948b7a2d4d16e42dae71c648bf566012e4aeb39591f6093eb196259b275c5dbb", // dtd
	"e7be6720d91cdc81becf07f5312b6c86039f27290ccb72139643186118cc4a89": "94392cf23145a815149b494559174e9c3524d01d1cf13fe0a55c0ebb4b0520aa", // semantic
}

var currentStatePackageProfileHashes = map[validation.ProfileID][2]string{
	ProfileResourceGraph:                        {"f0eab5da83c93f08e02a90df65fe37db13e5a00966ccb5ff5659628cea6dd291", "0c4ce4ce5969151bf75380d76039a7db7461c0597b844ece5866a76de6198e93"},
	ProfileMXLSource:                            {"a6a6f73f18882b11db2402358f5584f2541d366fcd9b811eeb22b17745fc9b05", "e04420acfbf1ecb5003447144996a5c6dbb7bbfc381d752fc76667fd64303a17"},
	ProfileMXLModel:                             {"4edd959ead2e80905663ac6f508a7ef8cea7337c7a5f90a214e787580d0027d8", "ed04026dfc05a64ee427cfedb7c3a933cb8b51929c0b7cb76f2508ac0868d3ed"},
	ProfileResourceGraphDictionaries:            {"80fb2fe78ecd080cef9a57366e81a74c830fd8f4fb186adfe4b81c1fefaf2dfe", "08830a06118f1fe882ea7211aa4b5552a97c7653cbb2a0fefa3a6e112c9e7c6f"},
	ProfileMXLSourceDictionaries:                {"3a5b6cb1addd7da5ce1a53f50e56ecb1c5ed63ce048d0e6c42ecef855598f31c", "22c1d06c778e820b6baaa51802e0bbb9b43612e3a08d56a4b15ba0cc47853849"},
	ProfileMXLModelDictionaries:                 {"54b724f9afa5f1465186350bfa28c107c1fd5add87803999b3475314d17c40e1", "c3e9922ed26ebe889306e07769d57f9aa5c07946a84caa1bacaae20106695b49"},
	ProfileResourceGraphInternalDTDDictionaries: {"6485ea8f1e332cff87b0e5cff9ed73748a461c4e748f279492a724f3ee3993e0", "7dbc72d33fa734b1dc9cf49c74272b7fd2cffb88ceeac1bc301692841101a497"},
	ProfileMXLSourceInternalDTDDictionaries:     {"9e3f0119d72de817e8006031733c3aec8a9bca1f70b22a06c15cebbeb9919e94", "2dfa360e102b8301cf53c893eae7ee2d9d73c76fc5254bae07dddeea98f95f2e"},
	ProfileMXLModelInternalDTDDictionaries:      {"f25ec1936ac81d090820f665c1152b5fa6fbd8e9a93a6ac5fed7337258e370af", "4d1b782b8277c653820898e2abae51d8dc0bce42301d057ecd4bf7a74a44d08e"},
	ProfileResourceGraphSemantics:               {"6a31be186ffde08b3200e47e3f4f231b1c7f4f5a85e7974b14a274babb103902", "16ce6ba1ca182022b2de75bcb2ce1a6177284186a9d918bdef5edeb1a96612e2"},
	ProfileMXLSourceSemantics:                   {"3f66d7fc32e67703f6f101b712b0d1590df5a73c9c38cf806569778895ee1f8e", "afaaf944f27a5a02b8beccac1f4c11eb685690ce3fd789f5aa64f43b0076b616"},
	ProfileMXLModelSemantics:                    {"80b49b7fb23033452638593cdf7775c8db5ca39de8fe9ea7f2054c0fec87213f", "12bfdb571374eca60c2dccd68634fedfcde96a858b36d436c59d3b3d0614aa64"},
}

// migrateCurrentStatePackageCatalog returns the digest a package catalog
// captured before the current-state revision has now.
func migrateCurrentStatePackageCatalog(t *testing.T, before string) string {
	t.Helper()
	after, ok := currentStatePackageCatalogDigests[before]
	if !ok {
		t.Fatalf("unexpected pre-current-state package catalog %s", before)
	}
	return after
}

// migrateCurrentStatePackageProfileHash returns the complete Profile JSON hash
// a package profile captured before the current-state revision has now; other
// profiles keep theirs.
func migrateCurrentStatePackageProfileHash(t *testing.T, id validation.ProfileID, before string) string {
	t.Helper()
	pair, ok := currentStatePackageProfileHashes[id]
	if !ok {
		return before
	}
	if before != pair[0] {
		t.Fatalf("unexpected pre-current-state profile %s: %s", id, before)
	}
	return pair[1]
}

func TestCurrentStatePackageMigrationAndRuleRevision(t *testing.T) {
	if len(currentStatePackageCatalogDigests) != 4 || len(currentStatePackageProfileHashes) != 12 {
		t.Fatalf("incomplete migration tables: %d catalogs, %d profiles", len(currentStatePackageCatalogDigests), len(currentStatePackageProfileHashes))
	}
	for _, tc := range []struct {
		factory func() (*validation.CompiledPackageCatalog, error)
		before  string
		ids     []validation.ProfileID
	}{
		{newPackageAssessmentCatalog, "554a97e25d6494c2e0550519fd4f6d2a439ca2f31e01a82baa7e28a00ddd2088", []validation.ProfileID{ProfileResourceGraph, ProfileMXLSource, ProfileMXLModel}},
		{newDictionaryPackageAssessmentCatalog, "f80db03b147d43021db39d1762880c2ef8e83967c1002c37b92229eb0feabd44", []validation.ProfileID{ProfileResourceGraphDictionaries, ProfileMXLSourceDictionaries, ProfileMXLModelDictionaries}},
		{dtdDictionaryPackageAssessmentCatalog, "69d813b09c15ea5083d382fe8864a41179e93cf52c4b04bfb140493070df7648", []validation.ProfileID{ProfileResourceGraphInternalDTDDictionaries, ProfileMXLSourceInternalDTDDictionaries, ProfileMXLModelInternalDTDDictionaries}},
		{semanticPackageAssessmentCatalog, "e7be6720d91cdc81becf07f5312b6c86039f27290ccb72139643186118cc4a89", []validation.ProfileID{ProfileResourceGraphSemantics, ProfileMXLSourceSemantics, ProfileMXLModelSemantics}},
	} {
		c, err := tc.factory()
		if err != nil {
			t.Fatal(err)
		}
		after := migrateCurrentStatePackageCatalog(t, tc.before)
		if after == tc.before {
			t.Fatalf("missing intentional migration of %s", tc.before)
		}
		for _, id := range tc.ids {
			p, ok := c.Profile(id)
			if !ok || p.CatalogDigest != after {
				t.Fatalf("package %s: %s, want %s (before %s)", id, p.CatalogDigest, after, tc.before)
			}
			raw, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(raw)
			if got, want := hex.EncodeToString(sum[:]), currentStatePackageProfileHashes[id][1]; got != want {
				t.Fatalf("complete Profile JSON %s: %s, want %s", id, got, want)
			}
			revised := 0
			for _, key := range p.Required {
				want := uint32(1)
				if key.ID == "MX40-PACKAGE-schema-hint-pairs" {
					want, revised = 2, revised+1
				}
				if key.Revision != want {
					t.Fatalf("%s: unexpected revision %+v", id, key)
				}
			}
			if revised != 1 {
				t.Fatalf("%s: schema-hint pairing selected %d times", id, revised)
			}
		}
	}
}
