package musicxml

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Pin the complete selected profiles as well as their catalog identities. The
// two hashes have different meanings: Profile JSON also covers exact selection,
// closure wording, target, roots and support metadata.
func TestSemanticProfilesExactFinalIdentity(t *testing.T) {
	for _, tc := range []struct {
		id               validation.ProfileID
		target           validation.Target
		catalog, profile string
	}{
		{ProfileSourceSemantics, validation.TargetSource, "9c1a974d3c3f1d664d91f76929bc8302a389811d018664c5e48bb8d62f59fe6b", "32f5e18154cf299202cbdf7abb8bd080a811077bdbe4bdab82e30d63586566b2"},
		{ProfileModelSemantics, validation.TargetModel, "e85bf8df6094ca313a4f36523eafc6bad2cd4140bfce933b9c3fe026d70bd8f9", "3ec86e6676086552bdf57492671eee6f7ec2101d805d44aac960116174782ab4"},
		{ProfileResourceGraphSemantics, validation.TargetPackage, "6aa620fbc44c979f58e00829af02c201059837013f0c1488db4c93c21bf8a55c", "9c48afc11c720fd27091ddd33d3eb8daf9bf132435d0ecdd838fb4d04a1c34f2"},
		{ProfileMXLSourceSemantics, validation.TargetPackage, "6aa620fbc44c979f58e00829af02c201059837013f0c1488db4c93c21bf8a55c", "cd09e76351aa69a5e5f76ccd5df227717851d6d62fc7a50eeb3c6e151fdee28a"},
		{ProfileMXLModelSemantics, validation.TargetPackage, "6aa620fbc44c979f58e00829af02c201059837013f0c1488db4c93c21bf8a55c", "1886b7a9d4158deadae62f533ae29b7f9d6cd6f8c521bc0be05052ade5fcf315"},
	} {
		t.Run(string(tc.id), func(t *testing.T) {
			var p validation.Profile
			var ok bool
			if tc.target == validation.TargetPackage {
				c, err := semanticPackageAssessmentCatalog()
				if err != nil {
					t.Fatal(err)
				}
				p, ok = c.Profile(tc.id)
			} else {
				e, err := buildSemanticAssessmentEngine(tc.target)
				if err != nil {
					t.Fatal(err)
				}
				p, ok = e.Profile(tc.id)
			}
			if !ok || p.ID != tc.id || p.Target != tc.target || p.CatalogDigest != tc.catalog {
				t.Fatalf("semantic identity drift: %+v", p)
			}
			raw, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(raw)
			if got := hex.EncodeToString(sum[:]); got != tc.profile {
				t.Fatalf("complete Profile JSON changed: %s, want %s", got, tc.profile)
			}
		})
	}
}
