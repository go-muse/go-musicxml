package musicxml

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Preserve historical captures in their original guards. Exactly these complete
// Profile JSON identities migrate with IPA rule/plan revision2, its new pin and
// precise closure wording. Shared catalogs also migrate their other profiles'
// CatalogDigest, even where the selected rules themselves remain unchanged.
func migrateIPARepresentationProfileHash(t *testing.T, id validation.ProfileID, historical string) string {
	t.Helper()
	changes := map[validation.ProfileID][2]string{
		ProfileSourceDictionaries:            {"4d2391d0dc93fd9137d42d0432d3bf7bd4490ecb1e4380d4559bdb68dfac907b", "7713fd75714d24bcb49ac71dffc3e9ec9e8ae642cccffcb310a2ca211bfd5fc6"},
		ProfileSourceSoundsDictionaries:      {"448ccb8c118aa48a81e814f59c8e31a17a2e89a9683aa87d1d63f8f899f8bbd8", "94a540d749c6bd05a10c2990efcfcbd43e388044ebc8a6b381cd684e243d5962"},
		ProfileModelDictionaries:             {"20f625be8f866db58a259b717a931816485aa98ca86d696e79f346378281ef81", "8bddd6162b1c8aefcecb0514fa35dcc42da251008917e335763a2ac02f2dc16b"},
		ProfileSourceInternalDTDLocal:        {"66ade1cc00540aebc746f1a9cc6a1cf0f3515c37d8a8c547093bbe686f5f6ff0", "d8eb98268bcc9651380ff7ebf0e9c87abe513467b0fb4e9c7ac0e16bbcb43934"},
		ProfileSourceInternalDTDLinks:        {"db744ca59d2ea60a0db07ef7e6693fee44ae2148a3481cbf9be80cdfb654bb01", "418bd4afe5756cbebc1c0254148d5b1a54649162c92af2312333bfc5530bb4fc"},
		ProfileSourceInternalDTDDictionaries: {"7ef8ca4eff9b989c8482b2cf7f98ebe8823ea98e40236461246186f59ec7792a", "3190e63530c1dd4d46bbe3d5fd113817906814a2e0c1a162a8b1dc82b96931f7"},
		ProfileResourceGraphDictionaries:     {"ac509b748631e72b90f10e6c5f2496d542dd7a906e3bd7c5a80bf825065e9741", "80fb2fe78ecd080cef9a57366e81a74c830fd8f4fb186adfe4b81c1fefaf2dfe"},
		ProfileMXLSourceDictionaries:         {"14a5570d39d1f3c8524a603b5590aba0bfaf95cf9c0b1cc06b9c4cc91523f222", "3a5b6cb1addd7da5ce1a53f50e56ecb1c5ed63ce048d0e6c42ecef855598f31c"},
		ProfileMXLModelDictionaries:          {"a04e03d483a3b13eb90b7e0f122268b1091fc5eefb71bae3a16bd23b99d6b7ec", "54b724f9afa5f1465186350bfa28c107c1fd5add87803999b3475314d17c40e1"},
	}
	pair, ok := changes[id]
	if !ok {
		return historical
	}
	if historical != pair[0] {
		t.Fatalf("unexpected pre-IPA-migration profile %s: %s", id, historical)
	}
	return pair[1]
}

// Before values below retain historical pre-IPA checkpoints, including
// unpublished DTD compositions; they are not all public release baselines. The
// public baselines are independently captured and asserted by
// TestPackageProfilesPublishedBaselineMigration and
// TestDictionaryProfilesPublishedBaselineMigration; these checks pin final identity.
func TestIPARepresentationExactCatalogMigrationAndRuleRevision(t *testing.T) {
	for _, tc := range []struct {
		ids           []validation.ProfileID
		before, after string
	}{
		{[]validation.ProfileID{ProfileSourceDictionaries, ProfileModelDictionaries, ProfileSourceSoundsDictionaries}, "21c731b5d19ebf918bff42fb3a15cac2fae6053b5b8006faf3f32b6108a8db21", "d8199be864c2baa081aca5887ca0ac188052ba22f6a690ea9bb3b436dc054c57"},
		{[]validation.ProfileID{ProfileSourceInternalDTDLocal, ProfileSourceInternalDTDLinks, ProfileSourceInternalDTDDictionaries}, "9f9d4a3ef31008332be016a7fe5b86cc10b2a764cf13426735683455ec07e5d0", "b2c6ad587726a0b64f7315f0d053a867e0d1d85b81f87217267d967fe5f9092c"},
		{[]validation.ProfileID{ProfileSourceInternalDTDSoundsDictionaries}, "4d3745b2a8b4a04862dce514a19d1c744770059e8321a99e9c5e083df63bda06", "4d3745b2a8b4a04862dce514a19d1c744770059e8321a99e9c5e083df63bda06"},
	} {
		for _, id := range tc.ids {
			var engine *validation.Engine
			var err error
			if id == ProfileModelDictionaries {
				engine, err = modelAssessmentEngineForProfile(id)
			} else {
				engine, err = sourceAssessmentEngineForProfile(id)
			}
			if err != nil {
				t.Fatal(err)
			}
			p, ok := engine.Profile(id)
			if !ok || p.CatalogDigest != tc.after {
				t.Fatalf("catalog %s: %s expected %s (prior %s)", id, p.CatalogDigest, tc.after, tc.before)
			}
			for _, key := range p.Required {
				if strings.Contains(string(key.ID), "/ipa-positive") && key.Revision != 2 {
					t.Fatalf("IPA rule not deliberately revised: %+v", key)
				}
			}
		}
	}
	for _, tc := range []struct {
		factory       func() (*validation.CompiledPackageCatalog, error)
		ids           []validation.ProfileID
		before, after string
	}{
		{newDictionaryPackageAssessmentCatalog, []validation.ProfileID{ProfileResourceGraphDictionaries, ProfileMXLSourceDictionaries, ProfileMXLModelDictionaries}, "14e5cdb4bc6b9fd2b7315f0059255fd8fc14fd43217e46238b1b64fd503b1333", "f80db03b147d43021db39d1762880c2ef8e83967c1002c37b92229eb0feabd44"},
		{dtdDictionaryPackageAssessmentCatalog, []validation.ProfileID{ProfileResourceGraphInternalDTDDictionaries, ProfileMXLSourceInternalDTDDictionaries, ProfileMXLModelInternalDTDDictionaries}, "4ecfcba585d635aa3a67ae6748985bb63c2b0482e6257cff7ccf39220acad319", "9d55aa68bf6b7fa45a27417ab9800d4e1f7b9744af810004de53cabf7b7be3b2"},
	} {
		c, err := tc.factory()
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range tc.ids {
			p, ok := c.Profile(id)
			if !ok || p.CatalogDigest != tc.after {
				t.Fatalf("package %s: %s expected %s (prior %s)", id, p.CatalogDigest, tc.after, tc.before)
			}
		}
	}
}

func TestIPARepresentationLegacyManifestIdentityUntouched(t *testing.T) {
	raw := dictionaryImportFixture(t, validation.DictionaryManifestIPAPositive2015Unicode13)
	sum := sha256.Sum256(raw)
	m, _ := validation.LookupDictionaryManifest(validation.DictionaryManifestIPAPositive2015Unicode13)
	if len(raw) != 26435 || hex.EncodeToString(sum[:]) != "ee4d39dbf8e999d12ae4e0680fc1b55a9704b81a4f049a110bcf5a21c1da2901" || m.Rows != 197 || m.DataBytes != 9836 || m.DataSHA256 != "47b0a8812835e56e0f8b6527cca13b6d55bb4b3ee231186868e8ec8590804ec6" || m.PolicyRevision != "ipa2015-unicode13-partial-positive-v1" {
		t.Fatalf("legacy identity changed: %+v", m)
	}
	// Unselected new capability leaves the legacy payloads byte-identical to the
	// legacy side of a both-selected run, even when the new whole witness passes.
	for _, value := range []string{"", "aba", "è", "é", "a?b"} {
		text := contextFixtureXML(`<note><rest/><duration>1</duration><play><ipa>`+value+`</ipa></play></note>`, false)
		old := dictionaryRepresentationDirect(t, text, false, true, false, false)
		both := dictionaryRepresentationDirect(t, text, false, true, true, true)
		left, _ := json.Marshal(dictionaryResults(old, validation.ContextDictionaryIPA))
		right, _ := json.Marshal(dictionaryResults(both, validation.ContextDictionaryIPA))
		if string(left) != string(right) {
			t.Fatalf("legacy output changed when representation selected: %s != %s", left, right)
		}
	}
}
