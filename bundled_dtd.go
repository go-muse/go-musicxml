package musicxml

import (
	"embed"
	"fmt"
	"path"
	"strings"
)

// bundledDTDFiles holds the official MusicXML 4.0 DTD modules from
// github.com/w3c/musicxml tag v4.0 (commit
// 799e2defb2ece0ae7bafe08dcbcac25b2c631d53), published by the W3C Music
// Notation Community Group under the W3C Community Final Specification
// Agreement like the bundled XSD files. midixml.dtd is omitted: it describes
// MIDI XML, not a MusicXML document.
//
//go:embed schema/musicxml-4.0/*.dtd schema/musicxml-4.0/*.mod schema/musicxml-4.0/*.ent
var bundledDTDFiles embed.FS

const bundledDTDDirectory = "schema/musicxml-4.0"

// bundledDTDPin records the exact upstream bytes of one bundled file.
type bundledDTDPin struct {
	Size   int
	SHA256 string
}

var bundledDTDPins = map[string]bundledDTDPin{
	"attributes.mod": {25637, "2481827cf1951341323afb8baf258627d8d759accf52b08e5c42cdc635eaec13"},
	"barline.mod":    {5550, "70c656478c2b118f4d9f0845f558b41f87a377bcb5eb9ea7ec879194defcb82f"},
	"common.mod":     {43542, "20811e895496094fa7ecf556878e6bf7b0b3ec1d36a651796ea0f09ccb3b9502"},
	"container.dtd":  {6027, "6cc702ce25f4baca796163ba1fd38718db12d28ec821759aa4190f21aad0f8e6"},
	"direction.mod":  {55281, "f9012c39c9da5c619231f82728077c3af95c6f0fefb1d7e9a17cdefa8bebfc6c"},
	"identity.mod":   {4209, "b459fe4fc2be8692878722e39d2846121f1099237e93a4d3c25751a4d4a4a7e1"},
	"isolat1.ent":    {6244, "2b64df2e72d297f43172615fbd225da5ea75ea6271e5f7596fb31754eba5948b"},
	"isolat2.ent":    {11234, "021a6e3320da37ecb7e05eac4734f3adff44790b25c61ddb0156d4d4c4bddab4"},
	"layout.mod":     {11551, "517839bc73ad8470899b2c5c3f26d604bc6892b15dee7ebc9dbceba59a8e9d01"},
	"link.mod":       {2984, "35cf25b2fe39ed43c9a24296b3a133587761892fa68b7be4745772038a24fc24"},
	"note.mod":       {55625, "6d5ff3c6a9752c499a46674e4c6582b647cc00bf556a16f3aa710e0a8c5551df"},
	"opus.dtd":       {2981, "4b5df88e6d952eccbe9eb0b19db6fe77875625ef57bfed5c38a6a4586502bd29"},
	"partwise.dtd":   {4687, "fa06d2668efc8ce787e8e6ae348cfc7a23945c6a375f85a6bb621ff28e19f0d2"},
	"score.mod":      {17429, "bdf2c05ccd99f93bf4773929f6a3b29edfaead4a6ad157887bfab81dfd3ce802"},
	"sounds.dtd":     {3528, "d8583a94b475f36df123a94c66d7ec10acf6fab6e6ccf5177c4cf1ba9b4148b4"},
	"timewise.dtd":   {4688, "c7bc76fad5db9ed1162c512bfe2b11fd840328bcb700fd90ea4c6cc97eba25ac"},
}

// bundledDTDPublicIDs maps public identifiers to bundled files exactly as the
// official schema/musicxml-4.0/catalog.xml does: MusicXML 1.0 through 3.1
// identifiers resolve to the backward-compatible 4.0 files. The MIDI entries
// are omitted with midixml.dtd.
var bundledDTDPublicIDs = func() map[string]string {
	ids := map[string]string{
		"ISO 8879:1986//ENTITIES Added Latin 1//EN//XML": "isolat1.ent",
		"ISO 8879:1986//ENTITIES Added Latin 2//EN//XML": "isolat2.ent",
	}
	modules := map[string]string{
		"Attributes": "attributes.mod",
		"Barline":    "barline.mod",
		"Common":     "common.mod",
		"Direction":  "direction.mod",
		"Identity":   "identity.mod",
		"Layout":     "layout.mod",
		"Link":       "link.mod",
		"Note":       "note.mod",
		"Score":      "score.mod",
	}
	for _, version := range []string{"1.0", "1.1", "2.0", "3.0", "3.1", "4.0"} {
		documents := map[string]string{
			"Partwise": "partwise.dtd",
			"Timewise": "timewise.dtd",
			"Opus":     "opus.dtd",
		}
		if version >= "2.0" {
			documents["Container"] = "container.dtd"
		}
		if version >= "3.0" {
			documents["Sounds"] = "sounds.dtd"
		}
		for name, file := range documents {
			ids["-//Recordare//DTD MusicXML "+version+" "+name+"//EN"] = file
		}
		for name, file := range modules {
			ids["-//Recordare//ELEMENTS MusicXML "+version+" "+name+"//EN"] = file
		}
	}
	return ids
}()

// bundledDTDSystemPrefixes are the official locations of the current MusicXML
// DTDs. Their file names carry no version, so they resolve to the 4.0 files.
var bundledDTDSystemPrefixes = []string{
	"http://www.musicxml.org/dtds/",
	"https://www.musicxml.org/dtds/",
	"http://musicxml.org/dtds/",
	"https://musicxml.org/dtds/",
}

// bundledDTDLookup returns the bundled file for an external identifier. A
// known public identifier wins, as in the official catalog's prefer="public"
// group; otherwise an official absolute system identifier, or a relative one
// against an official base, selects the file of the same name.
func bundledDTDLookup(publicID, systemID, baseSystemID string) (string, bool) {
	if file, known := bundledDTDPublicIDs[sourceDTDNormalizePublicID(publicID)]; known {
		return file, true
	}
	pinned := func(file string) (string, bool) {
		if _, ok := bundledDTDPins[file]; !ok {
			return "", false
		}
		return file, true
	}
	for _, prefix := range bundledDTDSystemPrefixes {
		if file, found := strings.CutPrefix(systemID, prefix); found {
			return pinned(file)
		}
	}
	if systemID == "" || strings.Contains(systemID, ":") || strings.HasPrefix(systemID, "/") {
		return "", false
	}
	for _, prefix := range bundledDTDSystemPrefixes {
		if rest, found := strings.CutPrefix(baseSystemID, prefix); found {
			return pinned(path.Clean(path.Join(path.Dir(rest), systemID)))
		}
	}
	return "", false
}

// bundledDTDEntityText returns a bundled file as external entity text, after
// the end-of-line normalization that every external parsed entity receives.
func bundledDTDEntityText(file string) (string, error) {
	data, err := bundledDTDFiles.ReadFile(bundledDTDDirectory + "/" + file)
	if err != nil {
		return "", fmt.Errorf("bundled DTD %s is missing: %w", file, err)
	}
	return sourceDTDExternalText(data)
}

// sourceDTDNormalizePublicID applies the XML 4.2.2 public identifier match
// normalization: whitespace runs become one space and ends are trimmed.
func sourceDTDNormalizePublicID(value string) string {
	return strings.Join(strings.FieldsFunc(value, func(r rune) bool {
		return r == ' ' || r == '\r' || r == '\n' || r == '\t'
	}), " ")
}
