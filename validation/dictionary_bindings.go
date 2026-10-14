package validation

// Fixed reviewed consumers bind exact declarations, not QName homonyms.
var dictionarySMuFLTypeNames = []string{
	"formatted-symbol",
	"formatted-symbol-id",
	"glyph",
	"smufl-accidental-glyph-name",
	"smufl-coda-glyph-name",
	"smufl-glyph-name",
	"smufl-lyrics-glyph-name",
	"smufl-pictogram-glyph-name",
	"smufl-segno-glyph-name",
	"smufl-wavy-line-glyph-name",
}

func dictionaryBindingSpecs(assembly Assembly) []contextBindingSpec {
	if assembly == AssemblySounds {
		return []contextBindingSpec{{Role: "dictionary.sound-id", Owner: "sound", Name: "id", Type: "xs:ID", Attribute: true, Count: 1}, {Role: "dictionary.sound", Owner: "sounds", Name: "sound", Type: "sound", Count: 1}}
	}
	return []contextBindingSpec{
		{Role: "dictionary.smufl.accidental", Owner: "accidental", Name: "smufl", Type: "smufl-accidental-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.smufl.accidental-mark", Owner: "accidental-mark", Name: "smufl", Type: "smufl-accidental-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.smufl.accidental-text", Owner: "accidental-text", Name: "smufl", Type: "smufl-accidental-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.smufl.appearance", Owner: "appearance", Name: "glyph", Type: "glyph", Attribute: false, Count: 1},
		{Role: "dictionary.smufl.arrow", Owner: "arrow", Name: "smufl", Type: "smufl-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.smufl.coda", Owner: "coda", Name: "smufl", Type: "smufl-coda-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.smufl.credit", Owner: "credit", Name: "credit-symbol", Type: "formatted-symbol-id", Attribute: false, Count: 2},
		{Role: "dictionary.smufl.direction-type", Owner: "direction-type", Name: "symbol", Type: "formatted-symbol-id", Attribute: false, Count: 1},
		{Role: "dictionary.smufl.effect", Owner: "effect", Name: "smufl", Type: "smufl-pictogram-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.smufl.elision", Owner: "elision", Name: "smufl", Type: "smufl-lyrics-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.smufl.empty-placement-smufl", Owner: "empty-placement-smufl", Name: "smufl", Type: "smufl-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.smufl.glass", Owner: "glass", Name: "smufl", Type: "smufl-pictogram-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.sounds.instrument-change", Owner: "instrument-change", Name: "instrument-sound", Type: "xs:string", Attribute: false, Count: 1},
		{Role: "dictionary.smufl.key-accidental", Owner: "key-accidental", Name: "smufl", Type: "smufl-accidental-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.smufl.membrane", Owner: "membrane", Name: "smufl", Type: "smufl-pictogram-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.smufl.metal", Owner: "metal", Name: "smufl", Type: "smufl-pictogram-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.smufl.notehead", Owner: "notehead", Name: "smufl", Type: "smufl-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.smufl.other-direction", Owner: "other-direction", Name: "smufl", Type: "smufl-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.smufl.other-notation", Owner: "other-notation", Name: "smufl", Type: "smufl-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.smufl.other-placement-text", Owner: "other-placement-text", Name: "smufl", Type: "smufl-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.smufl.other-text", Owner: "other-text", Name: "smufl", Type: "smufl-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.smufl.pitched", Owner: "pitched", Name: "smufl", Type: "smufl-pictogram-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.ipa.play", Owner: "play", Name: "ipa", Type: "xs:string", Attribute: false, Count: 1},
		{Role: "dictionary.sounds.score-instrument", Owner: "score-instrument", Name: "instrument-sound", Type: "xs:string", Attribute: false, Count: 1},
		{Role: "dictionary.smufl.segno", Owner: "segno", Name: "smufl", Type: "smufl-segno-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.smufl.timpani", Owner: "timpani", Name: "smufl", Type: "smufl-pictogram-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.smufl.tremolo", Owner: "tremolo", Name: "smufl", Type: "smufl-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.smufl.wavy-line", Owner: "wavy-line", Name: "smufl", Type: "smufl-wavy-line-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.smufl.wood", Owner: "wood", Name: "smufl", Type: "smufl-pictogram-glyph-name", Attribute: true, Count: 1},
		{Role: "dictionary.stopped", Owner: "technical", Name: "stopped", Type: "empty-placement-smufl", Attribute: false, Count: 1},
		{Role: "dictionary.open", Owner: "technical", Name: "open", Type: "empty-placement-smufl", Attribute: false, Count: 1},
		{Role: "dictionary.half-muted", Owner: "technical", Name: "half-muted", Type: "empty-placement-smufl", Attribute: false, Count: 1},
		{Role: "dictionary.tremolo", Owner: "ornaments", Name: "tremolo", Type: "tremolo", Attribute: false, Count: 1},
		{Role: "dictionary.tremolo-type", Owner: "tremolo", Name: "type", Type: "tremolo-type", Attribute: true, Count: 1},
		{Role: "dictionary.notehead", Owner: "note", Name: "notehead", Type: "notehead", Attribute: false, Count: 1},
		{Role: "dictionary.symbol-justify", Owner: "formatted-symbol-id", Name: "justify", Type: "left-center-right", Attribute: true, Count: 1},
		{Role: "dictionary.symbol-enclosure", Owner: "formatted-symbol-id", Name: "enclosure", Type: "enclosure-shape", Attribute: true, Count: 1},
		{Role: "dictionary.glyph-type", Owner: "glyph", Name: "type", Type: "glyph-type", Attribute: true, Count: 1},
		{Role: "dictionary.arrow", Owner: "technical", Name: "arrow", Type: "arrow", Attribute: false, Count: 1},
		{Role: "dictionary.arrow-direction", Owner: "arrow", Name: "arrow-direction", Type: "arrow-direction", Attribute: false, Count: 1},
		{Role: "dictionary.arrow-style", Owner: "arrow", Name: "arrow-style", Type: "arrow-style", Attribute: false, Count: 1},
		{Role: "dictionary.circular-arrow", Owner: "arrow", Name: "circular-arrow", Type: "circular-arrow", Attribute: false, Count: 1},
		{Role: "dictionary.arrowhead", Owner: "arrow", Name: "arrowhead", Type: "empty", Attribute: false, Count: 1},
		{Role: "dictionary.mordent", Owner: "ornaments", Name: "mordent", Type: "mordent", Attribute: false, Count: 1},
		{Role: "dictionary.inverted-mordent", Owner: "ornaments", Name: "inverted-mordent", Type: "mordent", Attribute: false, Count: 1},
		{Role: "dictionary.mordent-long", Owner: "mordent", Name: "long", Type: "yes-no", Attribute: true, Count: 1},
	}
}
