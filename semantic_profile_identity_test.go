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
		{ProfileSourceSemantics, validation.TargetSource, "e7e6d4195bb87a483d410c8a4f9cbd43eff9a1d488d93ffa02d8102b50211a63", "c15a9728c81ff126911ab99d68d776c28a3d368fd89dbbe61163802654c8c475"},
		{ProfileModelSemantics, validation.TargetModel, "e85bf8df6094ca313a4f36523eafc6bad2cd4140bfce933b9c3fe026d70bd8f9", "3ec86e6676086552bdf57492671eee6f7ec2101d805d44aac960116174782ab4"},
		{ProfileResourceGraphSemantics, validation.TargetPackage, "e7be6720d91cdc81becf07f5312b6c86039f27290ccb72139643186118cc4a89", "6a31be186ffde08b3200e47e3f4f231b1c7f4f5a85e7974b14a274babb103902"},
		{ProfileMXLSourceSemantics, validation.TargetPackage, "e7be6720d91cdc81becf07f5312b6c86039f27290ccb72139643186118cc4a89", "3f66d7fc32e67703f6f101b712b0d1590df5a73c9c38cf806569778895ee1f8e"},
		{ProfileMXLModelSemantics, validation.TargetPackage, "e7be6720d91cdc81becf07f5312b6c86039f27290ccb72139643186118cc4a89", "80b49b7fb23033452638593cdf7775c8db5ca39de8fe9ea7f2054c0fec87213f"},
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
