package musicxml

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Captured by TestDeclarationLegacyProfileBaseline on canonical PR40 main
// 5295080 before changing composition. These hash each complete owned Profile
// JSON, including required keys, support, closure text and catalog identity.
func TestSourceDeclarationCompositionPreservesProfiles(t *testing.T) {
	want := map[validation.ProfileID]string{
		ProfileSourceXSDPreview:         "957dd2347a6d7ac1fded6be66e0a82c96492e7870fa6bf0a2ac7f7cbd218120f",
		ProfileSourceInternalDTD:        "6559640eda77c589d64a482c92ac2f9d652b2030ab676f2c7cd374c076802bda",
		ProfileSourceLocal:              "41b4d99a5f6fc70e22bfe3ce0385c1bbe13693e8baacf35970d79facb9ccd542",
		ProfileSourceLinks:              "ff0ec0dd30142325e24d84c36c75bad9b2034092d0d11089c1b6a692f2df9b3d",
		ProfileSourceDictionaries:       "4d2391d0dc93fd9137d42d0432d3bf7bd4490ecb1e4380d4559bdb68dfac907b",
		ProfileSourceSoundsDictionaries: "448ccb8c118aa48a81e814f59c8e31a17a2e89a9683aa87d1d63f8f899f8bbd8",
		ProfileSourceContainer:          "9c839b007ff4cf5e3025bd62d74054270ddd3e5f9c13459932a9cbaf26a1d4ab",
		ProfileSourceSounds:             "d0d82881cbca27d6b5434d9a3522ea180c70c01de0ebd621d11a566511735254",
		ProfileSourceTimeline:           "701abcfe0421aa62a1d14bcdcef5a1f0ff01a568b68094aff43d723ce4777cc5",
		ProfileSourceContext:            "f2547f7b087b4551e47f1a4d1f1b8799cf955ec3f2cb2f56da96257dae6e41fc",
		ProfileModelXSD:                 "3b5bf70f6516b985af5ce3c3eb9f4f1eeacf56614278e53751718bbb779b6b45",
		ProfileModelLocal:               "5a540d0307d39d695e19b4a4b9a1e18a9029a8e27fbe1fa9175521a83980d9b8",
		ProfileModelLinks:               "dcb0c770a09165e69c95dc65244c0a863ed27f605eb255c641c29eaa54d3e9a5",
		ProfileModelDictionaries:        "20f625be8f866db58a259b717a931816485aa98ca86d696e79f346378281ef81",
		ProfileModelTimeline:            "38c486cdfc6d23be969a3f5dc6f2f8666452d6745d7627547ee4d74966924831",
		ProfileModelContext:             "37a63c9325973e67ae03328df45be262101fd83b216562641770e7ddf6ca93bb",
	}
	for id, expected := range want {
		// Preserve the original baseline map; only the explicitly documented
		// figure-number correction migrates the two context-profile hashes.
		expected = migrateFigureNumberProfileHash(t, id, expected)
		t.Run(string(id), func(t *testing.T) {
			var engine *validation.Engine
			var err error
			if strings.Contains(string(id), "/model-") {
				engine, err = modelAssessmentEngineForProfile(id)
			} else {
				engine, err = sourceAssessmentEngineForProfile(id)
			}
			if err != nil {
				t.Fatal(err)
			}
			profile, ok := engine.Profile(id)
			if !ok {
				t.Fatal("existing profile missing")
			}
			encoded, err := json.Marshal(profile)
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(encoded)
			if got := hex.EncodeToString(digest[:]); got != expected {
				t.Fatalf("existing profile changed: got %s want %s", got, expected)
			}
		})
	}
}
