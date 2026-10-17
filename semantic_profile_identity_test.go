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
		{ProfileResourceGraphSemantics, validation.TargetPackage, "09d9999d56a1235ce7d97a6329e31217651d1f2b6a6ade33c1dd816b74eacbee", "239af227450fd8b05ea5fc2cf320a4f0db235e2edf698beba85e3274c30bc176"},
		{ProfileMXLSourceSemantics, validation.TargetPackage, "09d9999d56a1235ce7d97a6329e31217651d1f2b6a6ade33c1dd816b74eacbee", "487bc6718936d4a8757bb7b6a9498fd8ec791a0deb260e5584fa5e3342784183"},
		{ProfileMXLModelSemantics, validation.TargetPackage, "09d9999d56a1235ce7d97a6329e31217651d1f2b6a6ade33c1dd816b74eacbee", "3912f2c49ae2d1a9f0ea01f70e760acd20f791c79facee77932b1aed21bf9f59"},
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
