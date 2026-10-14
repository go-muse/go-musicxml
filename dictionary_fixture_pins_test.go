package musicxml_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
)

func TestPinnedMusicDictionarySourceBytes(t *testing.T) {
	for _, tc := range []struct {
		path string
		size int
		hash string
	}{
		{"testdata/dictionaries/smufl-1.4/glyphnames.json", 301683, "1d05352599a20983d1c901635dc75d76f063c0987a7bee65f145325fc3e0d29f"},
		{"testdata/dictionaries/smufl-1.4/ranges.json", 98796, "a22be610edadf823de387f4c11716beee0ba2a11534c2bae1897b23fb6869531"},
		{"testdata/dictionaries/musicxml-4.0/sounds.xml", 33703, "a73d52b7215ea567dd54c5f2ea3e9d4f7ea167a40d5770a8fe0e3cb458858b35"},
		{"testdata/dictionaries/ipa-positive/runtime-positive-bundle.json", 26435, "ee4d39dbf8e999d12ae4e0680fc1b55a9704b81a4f049a110bcf5a21c1da2901"},
		{"testdata/dictionaries/ipa-positive/mapping-aid-omissions.json", 9438, "56ac8930b0db382f840c49d2ba36979cf88c8503b0767f8a7c063da9e070b667"},
		{"testdata/dictionaries/ipa-positive/positive-ledger.json", 248426, "829e8438be303739d79a29fbe4076b791bf4d4ba91e8364a05faac250d305b93"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			data, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			if len(data) != tc.size || fmt.Sprintf("%x", sha256.Sum256(data)) != tc.hash {
				t.Fatal("pinned source bytes changed")
			}
		})
	}
}
