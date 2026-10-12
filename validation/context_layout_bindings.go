package validation

import "strconv"

func layoutRole(name string) contextRole { return contextRole("layout." + name) }

var layoutElements = []struct{ name, typ string }{
	{"part-name", "part-name"}, {"part-abbreviation", "part-name"}, {"footnote", "formatted-text"}, {"defaults", "defaults"}, {"note", "note"}, {"figured-bass", "figured-bass"}, {"link", "link"}, {"measure-numbering", "measure-numbering"}, {"credit-image", "image"}, {"part-list", "part-list"}, {"part-name-display", "name-display"}, {"part-abbreviation-display", "name-display"}, {"group-name-display", "name-display"}, {"group-abbreviation-display", "name-display"}, {"group-symbol", "group-symbol"}, {"group-name", "group-name"}, {"group-abbreviation", "group-name"}, {"print", "print"}, {"page-layout", "page-layout"}, {"page-margins", "page-margins"}, {"system-layout", "system-layout"}, {"system-margins", "system-margins"}, {"staff-layout", "staff-layout"}, {"measure-layout", "measure-layout"}, {"scaling", "scaling"}, {"millimeters", "millimeters"}, {"tenths", "tenths"},
	{"page-height", "tenths"}, {"page-width", "tenths"}, {"left-margin", "tenths"}, {"right-margin", "tenths"}, {"top-margin", "tenths"}, {"bottom-margin", "tenths"}, {"system-distance", "tenths"}, {"top-system-distance", "tenths"}, {"staff-distance", "tenths"}, {"measure-distance", "tenths"}, {"system-dividers", "system-dividers"}, {"left-divider", "empty-print-object-style-align"}, {"right-divider", "empty-print-object-style-align"},
	{"lyric", "lyric"}, {"lyric-font", "lyric-font"}, {"lyric-language", "lyric-language"}, {"word-font", "empty-font"}, {"music-font", "empty-font"}, {"text", "text-element-data"}, {"elision", "elision"},
	{"words", "formatted-text-id"}, {"rehearsal", "formatted-text-id"}, {"symbol", "formatted-symbol-id"}, {"display-text", "formatted-text"}, {"accidental-text", "accidental-text"}, {"credit", "credit"}, {"credit-words", "formatted-text-id"}, {"credit-symbol", "formatted-symbol-id"}, {"direction", "direction"}, {"direction-type", "direction-type"}, {"harmony", "harmony"}, {"kind", "kind"}, {"per-minute", "per-minute"}, {"metronome", "metronome"},
	{"dynamics", "dynamics"}, {"tap", "tap"}, {"arrowhead", "empty"}, {"supports", "supports"}, {"encoding", "encoding"}, {"identification", "identification"},
}
var layoutAttributes = []struct{ name, typ string }{
	{"arrangement", "harmony-arrangement"}, {"width", "tenths"}, {"bezier-x", "tenths"}, {"bezier-y", "tenths"}, {"bezier-x2", "tenths"}, {"bezier-y2", "tenths"}, {"bezier-offset", "divisions"}, {"bezier-offset2", "divisions"}, {"halign", "left-center-right"}, {"justify", "left-center-right"}, {"valign", "valign"}, {"enclosure", "enclosure-shape"}, {"font-family", "font-family"}, {"font-style", "font-style"}, {"font-weight", "font-weight"}, {"font-size", "font-size"}, {"line-height", "number-or-normal"}, {"letter-spacing", "number-or-normal"}, {"dir", "text-direction"}, {"color", "color"}, {"rotation", "rotation-degrees"}, {"underline", "number-of-lines"}, {"overline", "number-of-lines"}, {"line-through", "number-of-lines"},
	{"default-x", "tenths"}, {"default-y", "tenths"}, {"relative-x", "tenths"}, {"relative-y", "tenths"}, {"placement", "above-below"}, {"directive", "yes-no"}, {"new-page", "yes-no"}, {"new-system", "yes-no"}, {"page-number", "xs:token"}, {"blank-page", "xs:positiveInteger"}, {"staff-spacing", "tenths"}, {"number", "xs:NMTOKEN"}, {"name", "xs:token"}, {"smufl", "smufl-glyph-name"}, {"hand", "tap-hand"},
}

func contextLayoutBindingSpecs() []contextBindingSpec {
	out := []contextBindingSpec{{Role: layoutRole("@system-mark"), Name: "system", Type: "system-relation", Attribute: true, Count: -1}, {Role: layoutRole("directive"), Owner: "attributes", Name: "directive", Type: "#anonymous", Count: 1}}
	for _, e := range layoutElements {
		out = append(out, contextBindingSpec{Role: layoutRole(e.name), Name: e.name, Type: e.typ, Count: -1})
	}
	for _, a := range layoutAttributes {
		out = append(out, contextBindingSpec{Role: layoutRole("@" + a.name), Name: a.name, Type: a.typ, Attribute: true, Count: -1})
	}
	// Imported anonymous xml:lang uses are exact per-owner declarations, rather
	// than a guessed named schema type.
	for _, owner := range []string{"formatted-text", "formatted-text-id", "text-element-data", "lyric-language", "accidental-text"} {
		out = append(out, contextBindingSpec{Role: layoutRole("lang." + owner), Owner: owner, Space: "http://www.w3.org/XML/1998/namespace", Name: "lang", Type: "#anonymous", Attribute: true, Count: 1})
	}
	for _, a := range []struct{ owner, name, typ string }{{"elision", "smufl", "smufl-lyrics-glyph-name"}, {"measure-numbering", "system", "system-relation-number"}, {"direction", "system", "system-relation"}, {"staff-layout", "number", "staff-number"}, {"page-margins", "type", "margin-type"}, {"supports", "element", "xs:NMTOKEN"}, {"supports", "attribute", "xs:NMTOKEN"}, {"supports", "value", "xs:token"}, {"supports", "type", "yes-no"}} {
		out = append(out, contextBindingSpec{Role: layoutRole(a.owner + ".@" + a.name), Owner: a.owner, Name: a.name, Type: a.typ, Attribute: true, Count: 1})
	}
	for i, r := range layoutConstantRules {
		base := "constant." + strconv.Itoa(i)
		if r.Element != "" {
			out = append(out, contextBindingSpec{Role: layoutRole(base + ".element"), Name: r.Element, Type: r.Type, Count: -1})
		}
		if r.Attribute != "" {
			out = append(out, contextBindingSpec{Role: layoutRole(base + ".attribute"), Name: r.Attribute, Type: r.AttributeType, Attribute: true, Count: -1})
		}
	}
	return append(out, layoutStateBindingSpecs()...)
}
