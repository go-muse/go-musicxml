package validation

// These are local semantic facts and advice. Dictionary-dependent canonical
// names, codepoint ranges and general glyph matching remain unavailable here.
const (
	ContextGlyphType               ContextProperty = "local.glyph-type"
	ContextGlyphName               ContextProperty = "local.glyph-name"
	ContextGlyphSemanticTarget     ContextProperty = "local.glyph-semantic-target"
	ContextGlyphTypeMatch          ContextProperty = "local.glyph-type-match"
	ContextSymbolGlyphName         ContextProperty = "local.symbol-glyph-name"
	ContextSymbolSpecializedMarkup ContextProperty = "local.symbol-specialized-markup"
	ContextArrowDirection          ContextProperty = "local.arrow-direction"
	ContextArrowStyle              ContextProperty = "local.arrow-style"
	ContextCircularArrow           ContextProperty = "local.circular-arrow"
	ContextArrowHeadOnly           ContextProperty = "local.arrow-head-only"
	ContextArrowGlyphMatch         ContextProperty = "local.arrow-glyph-match"
	ContextNoteheadTextRecommended ContextProperty = "local.notehead-text-recommended"
)

func contextGlyphPropertyValid(p ContextProperty) bool {
	switch p {
	case ContextArrowDirection, ContextArrowStyle, ContextCircularArrow, ContextGlyphType, ContextGlyphName, ContextGlyphSemanticTarget, ContextGlyphTypeMatch, ContextSymbolGlyphName, ContextSymbolSpecializedMarkup, ContextArrowHeadOnly, ContextArrowGlyphMatch, ContextNoteheadTextRecommended:
		return true
	}
	return false
}
func contextGlyphBindingSpecs() []contextBindingSpec {
	return []contextBindingSpec{
		{Role: "glyph.local", Name: "glyph", Type: "glyph", Count: -1},
		{Role: "glyph.type", Owner: "glyph", Name: "type", Type: "glyph-type", Attribute: true, Count: 1},
		{Role: "glyph.symbol", Name: "symbol", Type: "formatted-symbol-id", Count: -1},
		{Role: "glyph.arrow", Name: "arrow", Type: "arrow", Count: -1},
		{Role: "glyph.arrow-direction", Owner: "arrow", Name: "arrow-direction", Type: "arrow-direction", Count: 1},
		{Role: "glyph.arrow-style", Owner: "arrow", Name: "arrow-style", Type: "arrow-style", Count: 1},
		{Role: "glyph.circular-arrow", Owner: "arrow", Name: "circular-arrow", Type: "circular-arrow", Count: 1},
		{Role: "glyph.arrowhead", Owner: "arrow", Name: "arrowhead", Type: "empty", Count: 1},
		{Role: "glyph.arrow-name", Owner: "arrow", Name: "smufl", Type: "smufl-glyph-name", Attribute: true, Count: 1},
		{Role: "glyph.notehead-name", Owner: "notehead", Name: "smufl", Type: "smufl-glyph-name", Attribute: true, Count: 1},
	}
}
func glyphSemanticTarget(t Evidence[Atomic]) Evidence[Atomic] {
	if t.State != EvidenceKnown {
		return t
	}
	switch t.Value.Text {
	case "quarter-rest", "g-clef-ottava-bassa", "c-clef", "f-clef", "percussion-clef", "octave-shift-up-8", "octave-shift-down-8", "octave-shift-continue-8", "octave-shift-up-15", "octave-shift-down-15", "octave-shift-continue-15", "octave-shift-up-22", "octave-shift-down-22", "octave-shift-continue-22":
		return t
	}
	return stateUnknown[Atomic]("application-specific-glyph-type")
}
func glyphTypeMatch(t, name Evidence[Atomic]) Evidence[Atomic] {
	if t.State != EvidenceKnown {
		return t
	}
	if name.State != EvidenceKnown {
		return name
	}
	// Sufficient positive examples stated in the pinned MusicXML glyph prose.
	// This is not a closed dictionary: other names are not rejected.
	match := false
	switch t.Value.Text {
	case "quarter-rest":
		match = name.Value.Text == "restQuarter" || name.Value.Text == "restQuarterOld" || name.Value.Text == "restQuarterZ"
	case "g-clef-ottava-bassa":
		match = name.Value.Text == "gClef8vb" || name.Value.Text == "gClef8vbOld" || name.Value.Text == "gClef8vbCClef"
	case "octave-shift-up-8":
		match = name.Value.Text == "ottava" || name.Value.Text == "ottavaBassa" || name.Value.Text == "ottavaBassaBa" || name.Value.Text == "ottavaBassaVb" || name.Value.Text == "octaveBassa"
	}
	if match {
		return Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "yes"}}
	}
	return stateUnknown[Atomic]("smufl-semantic-dictionary-unavailable")
}
func localGlyphFacts(r *contextReader, key RuleKey, n *factNode) error {
	active := Evidence[bool]{State: EvidenceKnown, Value: true}
	publish := func(property ContextProperty, raw, value Evidence[Atomic]) error {
		return localPublish(r.ctx, r.s, key, n, property, raw, value, active, NodeRef{}, ContextUnitNone)
	}
	if r.matches(n, "glyph.local") {
		typ := r.attribute(n, "glyph.type")
		presence := r.presence(n, "glyph.type")
		if r.s.schema != nil && (presence.State == EvidenceAbsent || presence.State == EvidenceKnown && !presence.Value) {
			invalid, err := r.s.schemaInvalidValue(r.ctx, n, XSDCheckAttributes)
			if err != nil {
				return err
			}
			if invalid.State == EvidenceInvalid {
				typ = invalid
			}
		}
		name := r.scalar(n)
		for _, v := range []struct {
			p          ContextProperty
			raw, value Evidence[Atomic]
		}{{ContextGlyphType, typ, typ}, {ContextGlyphName, name, name}, {ContextGlyphSemanticTarget, typ, glyphSemanticTarget(typ)}, {ContextGlyphTypeMatch, name, glyphTypeMatch(typ, name)}} {
			if r.err != nil {
				return r.err
			}
			if err := publish(v.p, v.raw, v.value); err != nil {
				return err
			}
		}
	}
	if r.matches(n, "glyph.symbol") {
		name := r.scalar(n)
		advice := stateUnknown[Atomic]("smufl-specialized-markup-dictionary-unavailable")
		if name.State != EvidenceKnown {
			advice = name
		}
		if r.err != nil {
			return r.err
		}
		if err := publish(ContextSymbolGlyphName, name, name); err != nil {
			return err
		}
		if err := publish(ContextSymbolSpecializedMarkup, name, advice); err != nil {
			return err
		}
	}
	if r.matches(n, "glyph.arrow") {
		for _, field := range []struct {
			role     contextRole
			property ContextProperty
		}{{"glyph.arrow-direction", ContextArrowDirection}, {"glyph.arrow-style", ContextArrowStyle}, {"glyph.circular-arrow", ContextCircularArrow}} {
			value := r.childScalar(n, field.role)
			if r.err != nil {
				return r.err
			}
			if err := publish(field.property, value, value); err != nil {
				return err
			}
		}
		head := localPresence(r, n, "glyph.arrowhead")
		raw := localRawAttribute(r, n, "glyph.arrow-name")
		match := stateUnknown[Atomic]("smufl-arrow-description-dictionary-unavailable")
		if raw.State != EvidenceKnown {
			match = raw
		}
		if r.err != nil {
			return r.err
		}
		if err := publish(ContextArrowHeadOnly, localBoolAtomic(head), localBoolAtomic(head)); err != nil {
			return err
		}
		if err := publish(ContextArrowGlyphMatch, raw, match); err != nil {
			return err
		}
	}
	if r.matches(n, localNotehead) {
		raw := localRawAttribute(r, n, "glyph.notehead-name")
		typ := r.scalar(n)
		advice := stateUnknown[Atomic]("smufl-note-name-range-unavailable")
		if typ.State != EvidenceKnown {
			advice = typ
		} else if raw.State == EvidenceAbsent && typ.Value.Text != "other" {
			active = Evidence[bool]{State: EvidenceKnown, Value: false}
			advice = Evidence[Atomic]{State: EvidenceAbsent}
		} else if raw.State != EvidenceKnown && raw.State != EvidenceAbsent {
			advice = raw
		}
		if r.err != nil {
			return r.err
		}
		if err := publish(ContextNoteheadTextRecommended, raw, advice); err != nil {
			return err
		}
	}
	return r.err
}
