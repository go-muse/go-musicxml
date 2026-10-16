// Package dictionarydata holds the committed, byte-identical runtime dictionary
// assets. Explicit generation verifies their existing pins before copying them.
// Their separate licenses and provenance are preserved under data and in the
// repository's THIRD_PARTY_NOTICES.md; the library code license does not replace
// those data terms.
package dictionarydata

import (
	_ "embed"

	"github.com/go-muse/go-musicxml/validation"
)

// Strings keep the shared embedded storage immutable. A source must give each
// caller a separate reader, never a mutable slice backed by these assets.

//go:embed data/smufl-1.4/glyphnames.json
var glyphnames string

//go:embed data/smufl-1.4/ranges.json
var ranges string

//go:embed data/musicxml-4.0/sounds.xml
var sounds string

//go:embed data/ipa-positive/runtime-positive-bundle.json
var ipaPositive string

//go:embed data/ipa-representation/runtime-representation-bundle.json
var ipaRepresentation string

// Component returns shared immutable bytes for an existing closed component ID.
// Returning bytes is not a verification certificate or a capability selection.
func Component(id validation.DictionaryComponentID) (string, bool) {
	switch id {
	case validation.DictionaryComponentSMuFL14Glyphnames:
		return glyphnames, true
	case validation.DictionaryComponentSMuFL14Ranges:
		return ranges, true
	case validation.DictionaryComponentStandardSounds40:
		return sounds, true
	case validation.DictionaryComponentIPAPositive2015Unicode13:
		return ipaPositive, true
	case validation.DictionaryComponentIPARepresentation2015Unicode13:
		return ipaRepresentation, true
	default:
		return "", false
	}
}
