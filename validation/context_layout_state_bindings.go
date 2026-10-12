package validation

func layoutStateRole(name string) contextRole { return contextRole("layout-state." + name) }
func layoutStateBindingSpecs() []contextBindingSpec {
	var out []contextBindingSpec
	for _, v := range []struct{ name, typ string }{
		{"print", "print"}, {"attributes", "attributes"}, {"clef", "clef"}, {"line", "staff-line-position"}, {"key", "key"}, {"for-part", "for-part"}, {"staff-details", "staff-details"}, {"staff-type", "staff-type"}, {"staff-lines", "xs:nonNegativeInteger"}, {"line-detail", "line-detail"}, {"staff-size", "staff-size"}, {"part-symbol", "part-symbol"}, {"staves", "xs:nonNegativeInteger"}, {"rest", "rest"}, {"stem", "stem"}, {"unpitched", "unpitched"}, {"pitch", "pitch"}, {"step", "step"}, {"octave", "octave"}, {"display-step", "step"}, {"display-octave", "octave"}, {"string", "string"}, {"note", "note"}, {"type", "note-type"}, {"grace", "grace"}, {"cue", "empty"}, {"dot", "empty-placement"}, {"measure-numbering", "measure-numbering"},
	} {
		out = append(out, contextBindingSpec{Role: layoutStateRole(v.name), Name: v.name, Type: v.typ, Count: -1})
	}
	for _, v := range []struct{ owner, name, typ string }{
		{"accord", "string", "string-number"}, {"measure-numbering", "staff", "staff-number"}, {"measure-numbering", "multiple-rest-always", "yes-no"}, {"measure-numbering", "multiple-rest-range", "yes-no"}, {"clef", "number", "staff-number"}, {"clef", "additional", "yes-no"}, {"clef", "size", "symbol-size"}, {"clef", "after-barline", "yes-no"}, {"clef", "print-object", "yes-no"}, {"key", "print-object", "yes-no"}, {"for-part", "number", "staff-number"}, {"staff-details", "number", "staff-number"}, {"staff-details", "print-object", "yes-no"}, {"staff-details", "print-spacing", "yes-no"}, {"line-detail", "line", "staff-line"}, {"line-detail", "print-object", "yes-no"}, {"staff-size", "scaling", "non-negative-decimal"}, {"part-symbol", "top-staff", "staff-number"}, {"part-symbol", "bottom-staff", "staff-number"}, {"rest", "measure", "yes-no"}, {"stem", "relative-y", "tenths"}, {"empty-placement", "placement", "above-below"}, {"note-type", "size", "symbol-size"},
	} {
		out = append(out, contextBindingSpec{Role: layoutStateRole(v.owner + ".@" + v.name), Owner: v.owner, Name: v.name, Type: v.typ, Attribute: true, Count: 1})
	}
	// The same named attribute group is expanded into both hierarchy-specific
	// anonymous measure types. Select its exact declared type, not a path string.
	for _, v := range []struct{ name, typ string }{{"implicit", "yes-no"}, {"number", "xs:token"}, {"text", "measure-text"}} {
		out = append(out, contextBindingSpec{Role: layoutStateRole("measure.@" + v.name), Name: v.name, Type: v.typ, Attribute: true, Count: -1})
	}
	return out
}

var layoutStateProperties = []ContextProperty{
	"layout.advisory-percussion-unpitched",
	"layout.measure-number-staff", "layout.measure-number-multiple-rest-always", "layout.measure-number-multiple-rest-range", "layout.clef-staff", "layout.clef-line", "layout.clef-size", "layout.clef-after-barline", "layout.clef-system-print", "layout.key-system-print", "layout.created-part-staff-scope", "layout.hidden-line-coordinate", "layout.hidden-line-layout-count", "layout.staff-cutaway", "layout.staff-space-scale", "layout.staff-notation-scale", "layout.staff-size-affects-margins", "layout.staff-size-range", "layout.alternate-staff-playback", "layout.alternate-staff-source", "layout.part-symbol", "layout.part-symbol-top", "layout.part-symbol-bottom", "layout.part-symbol-barline-scope-change", "layout.dot-placement-active", "layout.measure-rest", "layout.stemlet", "layout.stem-relative-y", "layout.unpitched-default-position", "layout.display-staff-position", "layout.string-number-basis", "layout.note-size-category", "layout.measure-number-text", "layout.measure-number-display",
}

func layoutStatePropertyValid(p ContextProperty) bool {
	for _, v := range layoutStateProperties {
		if p == v {
			return true
		}
	}
	return false
}
