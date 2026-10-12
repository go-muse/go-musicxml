package validation

import (
	"context"
	"strconv"
)

const (
	localMode                    contextRole = "local.mode"
	localNumeralMode             contextRole = "local.numeral-mode"
	localKind                    contextRole = "local.kind"
	localDegreeType              contextRole = "local.degree-type"
	localDegreeAlter             contextRole = "local.degree-alter"
	localDegreeValue             contextRole = "local.degree-value"
	localDegreeSymbol            contextRole = "local.degree-symbol"
	localDegreeText              contextRole = "local.degree-text"
	localLineType                contextRole = "local.line-type"
	localDashLength              contextRole = "local.dash-length"
	localSpaceLength             contextRole = "local.space-length"
	localColor                   contextRole = "local.color"
	localFontFamily              contextRole = "local.font-family-order"
	localSupports                contextRole = "local.supports"
	localSupportsElement         contextRole = "local.supports-element"
	localSupportsAttribute       contextRole = "local.supports-attribute"
	localSupportsValue           contextRole = "local.supports-value"
	localSupportsType            contextRole = "local.supports-type"
	localCreator                 contextRole = "local.creator"
	localEncoder                 contextRole = "local.encoder"
	localMetadataType            contextRole = "local.metadata-type"
	localDistance                contextRole = "local.distance"
	localDistanceType            contextRole = "local.distance-type"
	localLineWidth               contextRole = "local.line-width"
	localLineWidthType           contextRole = "local.line-width-type"
	localEnsemble                contextRole = "local.ensemble"
	localText                    contextRole = "local.lyric-text"
	localPartNameDisplay         contextRole = "local.part-name-display"
	localPartAbbreviationDisplay contextRole = "local.part-abbreviation-display"
	localScorePart               contextRole = "local.score-part"
	localFret                    contextRole = "local.fret"
	localLine                    contextRole = "local.clef-line"
	localLineNumber              contextRole = "local.staff-line-number"
	localPitchOctave             contextRole = "local.pitch-octave"
	localAlter                   contextRole = "local.pitch-alter"
)

func localStructuralBindingSpecs() []contextBindingSpec {
	return []contextBindingSpec{
		{Role: localClefSign, Name: "sign", Type: "clef-sign", Count: -1}, {Role: localHarmonyFunction, Name: "function", Type: "style-text", Count: -1},
		{Role: localBendAlter, Name: "bend-alter", Type: "semitones", Count: -1}, {Role: localKeyAlter, Name: "key-alter", Type: "semitones", Count: -1}, {Role: localTuningAlter, Name: "tuning-alter", Type: "semitones", Count: -1}, {Role: localRootAlter, Name: "root-alter", Type: "harmony-alter", Count: -1}, {Role: localBassAlter, Name: "bass-alter", Type: "harmony-alter", Count: -1}, {Role: localNumeralAlter, Name: "numeral-alter", Type: "harmony-alter", Count: -1}, {Role: localDashes, Name: "dashes", Type: "dashes", Count: -1}, {Role: localBracket, Name: "bracket", Type: "bracket", Count: -1}, {Role: localWedge, Name: "wedge", Type: "wedge", Count: -1}, {Role: localGlissando, Name: "glissando", Type: "glissando", Count: -1}, {Role: localSlide, Name: "slide", Type: "slide", Count: -1},
		{Role: localFigureNumber, Name: "figure-number", Type: "style-text", Count: -1},
		{Role: localFrame, Name: "frame", Type: "frame", Count: -1}, {Role: localFrameNote, Name: "frame-note", Type: "frame-note", Count: -1}, {Role: localFrameString, Name: "string", Type: "string", Count: -1}, {Role: localFrameStrings, Name: "frame-strings", Type: "xs:positiveInteger", Count: -1},
		{Role: localMode, Name: "mode", Type: "mode", Count: -1}, {Role: localNumeralMode, Name: "numeral-mode", Type: "numeral-mode", Count: -1},
		{Role: localKind, Name: "kind", Type: "kind", Count: -1}, {Role: localDegreeType, Name: "degree-type", Type: "degree-type", Count: -1}, {Role: localDegreeAlter, Name: "degree-alter", Type: "degree-alter", Count: -1}, {Role: localDegreeValue, Name: "degree-value", Type: "degree-value", Count: -1},
		{Role: localDegreeSymbol, Owner: "degree-value", Name: "symbol", Type: "degree-symbol-value", Attribute: true, Count: 1}, {Role: localDegreeText, Owner: "degree-value", Name: "text", Type: "xs:token", Attribute: true, Count: 1},
		{Role: localLineType, Name: "line-type", Type: "line-type", Attribute: true, Count: -1}, {Role: localDashLength, Name: "dash-length", Type: "tenths", Attribute: true, Count: -1}, {Role: localSpaceLength, Name: "space-length", Type: "tenths", Attribute: true, Count: -1},
		{Role: localColor, Name: "color", Type: "color", Attribute: true, Count: -1}, {Role: localFontFamily, Name: "font-family", Type: "font-family", Attribute: true, Count: -1},
		{Role: localSupports, Name: "supports", Type: "supports", Count: -1}, {Role: localSupportsElement, Owner: "supports", Name: "element", Type: "xs:NMTOKEN", Attribute: true, Count: 1}, {Role: localSupportsAttribute, Owner: "supports", Name: "attribute", Type: "xs:NMTOKEN", Attribute: true, Count: 1}, {Role: localSupportsValue, Owner: "supports", Name: "value", Type: "xs:token", Attribute: true, Count: 1}, {Role: localSupportsType, Owner: "supports", Name: "type", Type: "yes-no", Attribute: true, Count: 1},
		{Role: localCreator, Name: "creator", Type: "typed-text", Count: -1}, {Role: localEncoder, Name: "encoder", Type: "typed-text", Count: -1}, {Role: localMetadataType, Owner: "typed-text", Name: "type", Type: "xs:token", Attribute: true, Count: 1},
		{Role: localDistance, Name: "distance", Type: "distance", Count: -1}, {Role: localDistanceType, Owner: "distance", Name: "type", Type: "distance-type", Attribute: true, Count: 1},
		{Role: localLineWidth, Name: "line-width", Type: "line-width", Count: -1}, {Role: localLineWidthType, Owner: "line-width", Name: "type", Type: "line-width-type", Attribute: true, Count: 1},
		{Role: localEnsemble, Name: "ensemble", Type: "positive-integer-or-empty", Count: -1}, {Role: localText, Name: "text", Type: "text-element-data", Count: -1},
		{Role: localPartNameDisplay, Name: "part-name-display", Type: "name-display", Count: -1}, {Role: localPartAbbreviationDisplay, Name: "part-abbreviation-display", Type: "name-display", Count: -1}, {Role: localScorePart, Name: "score-part", Type: "score-part", Count: -1},
		{Role: localFret, Name: "fret", Type: "fret", Count: -1}, {Role: localLine, Name: "line", Type: "staff-line-position", Count: -1}, {Role: localLineNumber, Name: "line", Type: "staff-line", Attribute: true, Count: -1},
		{Role: localPitchOctave, Name: "octave", Type: "octave", Count: -1}, {Role: localAlter, Name: "alter", Type: "semitones", Count: -1},
	}
}

// Additional local properties preserve source semantics without imposing an
// invented closed vocabulary or requiring a renderer's actual geometry.
const (
	ContextMinorMode               ContextProperty = "local.minor-mode"
	ContextHarmonyActive           ContextProperty = "local.harmony-active"
	ContextHarmonyDescription      ContextProperty = "local.harmony-description"
	ContextDegreeReference         ContextProperty = "local.degree-reference"
	ContextDegreeTextComposition   ContextProperty = "local.degree-text-composition"
	ContextDashLength              ContextProperty = "local.dash-length"
	ContextSpaceLength             ContextProperty = "local.space-length"
	ContextColorAlpha              ContextProperty = "local.color-alpha"
	ContextColorRGB                ContextProperty = "local.color-rgb"
	ContextFontFamilyOrder         ContextProperty = "local.font-family-order"
	ContextSupportsElement         ContextProperty = "local.supports-element"
	ContextSupportsAttribute       ContextProperty = "local.supports-attribute"
	ContextSupportsValue           ContextProperty = "local.supports-value"
	ContextSupportsType            ContextProperty = "local.supports-type"
	ContextMetadataRole            ContextProperty = "local.metadata-role"
	ContextDistanceType            ContextProperty = "local.distance-type"
	ContextLineWidthType           ContextProperty = "local.line-width-type"
	ContextEnsembleSize            ContextProperty = "local.ensemble-size"
	ContextLyricLiteralText        ContextProperty = "local.lyric-literal-text"
	ContextPartNameDisplay         ContextProperty = "local.part-name-display-priority"
	ContextPartAbbreviationDisplay ContextProperty = "local.part-abbreviation-display-priority"
	ContextOpenString              ContextProperty = "local.fret-open-string"
	ContextStaffLineBasis          ContextProperty = "local.staff-line-basis"
	ContextOctaveBasis             ContextProperty = "local.octave-basis"
	ContextAlterSemitones          ContextProperty = "local.alter-semitones"
)

func localStructuralPropertyValid(p ContextProperty) bool {
	switch p {
	case ContextMinorMode, ContextHarmonyActive, ContextHarmonyDescription, ContextDegreeReference, ContextDegreeTextComposition, ContextDashLength, ContextSpaceLength, ContextColorAlpha, ContextColorRGB, ContextFontFamilyOrder, ContextSupportsElement, ContextSupportsAttribute, ContextSupportsValue, ContextSupportsType, ContextMetadataRole, ContextDistanceType, ContextLineWidthType, ContextEnsembleSize, ContextLyricLiteralText, ContextPartNameDisplay, ContextPartAbbreviationDisplay, ContextOpenString, ContextStaffLineBasis, ContextOctaveBasis, ContextAlterSemitones, ContextFrameString, ContextFrameMissingString, ContextSupportUnreported, ContextFigureNumber, ContextMinorRaisedSixth, ContextMinorRaisedSeventh, ContextDeprecatedNotation:
		return true
	}
	return false
}
func localKnownText(v string) Evidence[Atomic] {
	return Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: v}}
}
func localStructuralNode(ctx context.Context, s *Session, key RuleKey, n *factNode, r *contextReader) error {
	active := Evidence[bool]{State: EvidenceKnown, Value: true}
	publish := func(prop ContextProperty, raw, value Evidence[Atomic], app Evidence[bool]) error {
		if r.err != nil {
			return r.err
		}
		return localPublish(ctx, s, key, n, prop, raw, value, app, NodeRef{}, ContextUnitNone)
	}
	if r.matches(n, localMode) || r.matches(n, localNumeralMode) {
		raw := r.scalar(n)
		value := raw
		if raw.State == EvidenceKnown && raw.Value.Kind == AtomicString && raw.Value.Text == "minor" {
			value = localKnownText("natural minor")
		}
		if err := publish(ContextMinorMode, raw, value, active); err != nil {
			return err
		}
		if r.matches(n, localNumeralMode) || (raw.State == EvidenceKnown && raw.Value.Text == "minor") {
			minorActive := active
			six, seven := stateUnavailable[Atomic, bool](raw), stateUnavailable[Atomic, bool](raw)
			if raw.State == EvidenceKnown {
				switch raw.Value.Text {
				case "minor", "natural minor":
					six = Evidence[bool]{State: EvidenceKnown, Value: false}
					seven = six
				case "harmonic minor":
					six = Evidence[bool]{State: EvidenceKnown, Value: false}
					seven = Evidence[bool]{State: EvidenceKnown, Value: true}
				case "melodic minor":
					six = Evidence[bool]{State: EvidenceKnown, Value: true}
					seven = six
				default:
					minorActive = Evidence[bool]{State: EvidenceKnown, Value: false}
					six = Evidence[bool]{State: EvidenceAbsent}
					seven = six
				}
			}
			if err := publish(ContextMinorRaisedSixth, raw, localBoolAtomic(six), minorActive); err != nil {
				return err
			}
			if err := publish(ContextMinorRaisedSeventh, raw, localBoolAtomic(seven), minorActive); err != nil {
				return err
			}
		}

	}
	if r.matches(n, localKind) {
		raw := r.scalar(n)
		v := stateUnavailable[Atomic, bool](raw)
		if raw.State == EvidenceKnown {
			v = Evidence[bool]{State: EvidenceKnown, Value: raw.Value.Text != "none"}
		}
		if err := publish(ContextHarmonyActive, raw, localBoolAtomic(v), active); err != nil {
			return err
		}
		if raw.State == EvidenceKnown && raw.Value.Text == "other" {
			if err := publish(ContextHarmonyDescription, raw, localKnownText("added-degrees"), active); err != nil {
				return err
			}
		}
	}
	if r.matches(n, localDegreeAlter) {
		raw := r.scalar(n)
		parent := s.nodes[n.header.Parent]
		kind := r.childScalar(parent, localDegreeType)
		v := kind
		if kind.State == EvidenceKnown {
			switch kind.Value.Text {
			case "add":
				v = localKnownText("dominant-with-minor-seventh")
			case "alter", "subtract":
				v = localKnownText("harmony-kind-degree")
			default:
				v = stateUnknown[Atomic]("degree-operation-unavailable")
			}
		}
		if err := publish(ContextDegreeReference, raw, v, active); err != nil {
			return err
		}
	}
	if r.matches(n, localDegreeValue) {
		symbol := localRawAttribute(r, n, localDegreeSymbol)
		raw := localRawAttribute(r, n, localDegreeText)
		a := stateUnavailable[Atomic, bool](symbol)
		if symbol.State == EvidenceKnown {
			a = active
		} else if symbol.State == EvidenceAbsent {
			a = Evidence[bool]{State: EvidenceKnown, Value: false}
		}
		value := raw
		if a.State == EvidenceKnown && a.Value {
			value = localKnownText("text-follows-symbol")
		}
		if err := publish(ContextDegreeTextComposition, raw, value, a); err != nil {
			return err
		}
	}
	for _, q := range []struct {
		role     contextRole
		property ContextProperty
	}{{localDashLength, ContextDashLength}, {localSpaceLength, ContextSpaceLength}} {
		if r.attributeOwner(n, q.role) {
			raw := localRawAttribute(r, n, q.role)
			line := Evidence[Atomic]{State: EvidenceAbsent}
			if r.attributeOwner(n, localLineType) {
				line = localRawAttribute(r, n, localLineType)
			}
			if line.State == EvidenceAbsent {
				switch {
				case r.matches(n, localDashes):
					line = localKnownText("dashed")
				case r.matches(n, localBracket) || r.matches(n, localWedge) || r.matches(n, localSlide):
					line = localKnownText("solid")
				case r.matches(n, localGlissando):
					line = localKnownText("wavy")
				default:
					line = stateUnknown[Atomic]("line-type-not-specified")
				}
			}
			a := stateUnavailable[Atomic, bool](line)
			if line.State == EvidenceKnown {
				a = Evidence[bool]{State: EvidenceKnown, Value: line.Value.Text == "dashed"}
			}
			if err := publish(q.property, raw, raw, a); err != nil {
				return err
			}
		}
	}
	if r.attributeOwner(n, localColor) {
		raw := localRawAttribute(r, n, localColor)
		if raw.State != EvidenceAbsent {
			rgb, alpha := raw, raw
			if raw.State == EvidenceKnown && raw.Value.Kind == AtomicString {
				text := raw.Value.Text
				av := "FF"
				if len(text) == 9 {
					av = text[1:3]
					text = "#" + text[3:]
				}
				if len(text) == 7 {
					rgb = localKnownText(text)
					a, e := strconv.ParseInt(av, 16, 16)
					if e != nil {
						return contract("local color", "validated color magnitude unavailable")
					}
					num := localByteIntegers[a]
					alpha = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger, Integer: num}}
				} else {
					rgb, alpha = stateUnknown[Atomic]("color-value-unavailable"), stateUnknown[Atomic]("color-value-unavailable")
				}
			}
			if err := publish(ContextColorRGB, raw, rgb, active); err != nil {
				return err
			}
			if err := publish(ContextColorAlpha, raw, alpha, active); err != nil {
				return err
			}
		}
	}
	if r.attributeOwner(n, localFontFamily) {
		raw := localRawAttribute(r, n, localFontFamily)
		if raw.State != EvidenceAbsent {
			value := stateUnavailable[Atomic, Atomic](raw)
			if raw.State == EvidenceKnown {
				value = localKnownText("comma-separated-preference-order")
			}
			if err := publish(ContextFontFamilyOrder, raw, value, active); err != nil {
				return err
			}
		}
	}
	if r.matches(n, localSupports) {
		selector := localRawAttribute(r, n, localSupportsAttribute)
		for _, q := range []struct {
			role     contextRole
			property ContextProperty
		}{{localSupportsElement, ContextSupportsElement}, {localSupportsAttribute, ContextSupportsAttribute}, {localSupportsValue, ContextSupportsValue}, {localSupportsType, ContextSupportsType}} {
			raw := localRawAttribute(r, n, q.role)
			a := active
			if q.role == localSupportsValue {
				if selector.State == EvidenceAbsent {
					a = Evidence[bool]{State: EvidenceKnown, Value: false}
				} else if selector.State != EvidenceKnown {
					a = stateUnavailable[Atomic, bool](selector)
				}
			}
			if err := publish(q.property, raw, raw, a); err != nil {
				return err
			}
		}
	}
	if r.matches(n, localCreator) || r.matches(n, localEncoder) {
		raw := localRawAttribute(r, n, localMetadataType)
		if err := publish(ContextMetadataRole, raw, raw, active); err != nil {
			return err
		}
	}
	for _, q := range []struct {
		element, attribute contextRole
		property           ContextProperty
	}{{localDistance, localDistanceType, ContextDistanceType}, {localLineWidth, localLineWidthType, ContextLineWidthType}} {
		if r.matches(n, q.element) {
			raw := localRawAttribute(r, n, q.attribute)
			if err := publish(q.property, raw, raw, active); err != nil {
				return err
			}
		}
	}
	if r.matches(n, localEnsemble) {
		raw := r.scalar(n)
		value := raw
		if raw.State == EvidenceKnown && raw.Value.Kind == AtomicString && raw.Value.Text == "" {
			value = stateUnknown[Atomic]("ensemble-size-unspecified")
		}
		if err := publish(ContextEnsembleSize, raw, value, active); err != nil {
			return err
		}
	}
	if r.matches(n, localText) {
		raw := r.scalar(n)
		if err := publish(ContextLyricLiteralText, raw, raw, active); err != nil {
			return err
		}
	}
	if r.matches(n, localScorePart) {
		for _, q := range []struct {
			role     contextRole
			property ContextProperty
		}{{localPartNameDisplay, ContextPartNameDisplay}, {localPartAbbreviationDisplay, ContextPartAbbreviationDisplay}} {
			presence := localPresence(r, n, q.role)
			value := localBoolAtomic(presence)
			if err := publish(q.property, Evidence[Atomic]{State: EvidenceAbsent}, value, active); err != nil {
				return err
			}
		}
	}
	if r.matches(n, localFret) {
		raw := r.scalar(n)
		value := stateUnavailable[Atomic, bool](raw)
		if raw.State == EvidenceKnown && raw.Value.Kind == AtomicInteger {
			zero := localByteIntegers[0]
			cmp, e := raw.Value.Integer.Cmp(zero)
			if e != nil {
				return e
			}
			value = Evidence[bool]{State: EvidenceKnown, Value: cmp == 0}
		}
		if err := publish(ContextOpenString, raw, localBoolAtomic(value), active); err != nil {
			return err
		}
	}
	if r.matches(n, localLine) || r.attributeOwner(n, localLineNumber) {
		raw := r.scalar(n)
		if r.attributeOwner(n, localLineNumber) {
			raw = localRawAttribute(r, n, localLineNumber)
		}
		if err := publish(ContextStaffLineBasis, raw, localKnownText("one-is-bottom-line-unbounded-by-line-count"), active); err != nil {
			return err
		}
	}
	if r.matches(n, localPitchOctave) {
		raw := r.scalar(n)
		if err := publish(ContextOctaveBasis, raw, localKnownText("octave-four-begins-at-middle-c"), active); err != nil {
			return err
		}
	}
	if r.matches(n, localAlter) || r.matches(n, localBendAlter) || r.matches(n, localKeyAlter) || r.matches(n, localTuningAlter) || r.matches(n, localRootAlter) || r.matches(n, localBassAlter) || r.matches(n, localNumeralAlter) || r.matches(n, localDegreeAlter) {
		raw := r.scalar(n)
		if err := publish(ContextAlterSemitones, raw, raw, active); err != nil {
			return err
		}
	}
	if n.header.Parent == 0 {
		if err := publish(ContextSupportUnreported, Evidence[Atomic]{State: EvidenceAbsent}, stateUnknown[Atomic]("unreported-encoding-capability"), active); err != nil {
			return err
		}
	}
	if r.matches(n, localFrame) {
		raw := r.childScalar(n, localFrameStrings)
		complete := r.complete(n)
		value := localKnownText("unlisted-strings-muted")
		if complete.State != EvidenceKnown {
			value = stateUnavailable[bool, Atomic](complete)
		}
		if err := publish(ContextFrameMissingString, raw, value, complete); err != nil {
			return err
		}
	}
	if r.matches(n, localFrameNote) {
		raw := r.childScalar(n, localFrameString)
		scope := NodeRef{DocumentID: s.documentID, Node: n.header.Parent}
		if err := localPublish(ctx, s, key, n, ContextFrameString, raw, raw, active, scope, ContextUnitNone); err != nil {
			return err
		}
	}

	if r.matches(n, localFigureNumber) {
		raw := r.scalar(n)
		if err := publish(ContextFigureNumber, raw, raw, active); err != nil {
			return err
		}
	}

	if r.matches(n, localClefSign) || r.matches(n, localHarmonyFunction) {
		raw := r.scalar(n)
		value := stateUnavailable[Atomic, bool](raw)
		if raw.State == EvidenceKnown {
			value = Evidence[bool]{State: EvidenceKnown, Value: r.matches(n, localHarmonyFunction) || raw.Value.Text == "none"}
		}
		if err := publish(ContextDeprecatedNotation, raw, localBoolAtomic(value), active); err != nil {
			return err
		}
	}

	return r.err
}

const (
	localFrame        contextRole = "local.frame"
	localFrameNote    contextRole = "local.frame-note"
	localFrameString  contextRole = "local.frame-string"
	localFrameStrings contextRole = "local.frame-strings"
)

// Sparse frame membership and unspecified encoder capability are preserved as
// independent facts; arbitrarily large string counts are not expanded.
const (
	ContextFrameString        ContextProperty = "local.frame-present-string"
	ContextFrameMissingString ContextProperty = "local.frame-unlisted-string"
	ContextSupportUnreported  ContextProperty = "local.unreported-encoding-support"
)

const localFigureNumber contextRole = "local.figure-number"

// ContextFigureNumber preserves the original field value without inventing a numeric grammar.
const ContextFigureNumber ContextProperty = "local.figure-number-text"

const (
	localBendAlter    contextRole = "local.bend-alter"
	localKeyAlter     contextRole = "local.key-alter"
	localTuningAlter  contextRole = "local.tuning-alter"
	localRootAlter    contextRole = "local.root-alter"
	localBassAlter    contextRole = "local.bass-alter"
	localNumeralAlter contextRole = "local.numeral-alter"
	localDashes       contextRole = "local.dashes"
	localBracket      contextRole = "local.bracket"
	localWedge        contextRole = "local.wedge"
	localGlissando    contextRole = "local.glissando"
	localSlide        contextRole = "local.slide"
)

// Minor-mode alteration properties are relative to natural minor.
const (
	ContextMinorRaisedSixth   ContextProperty = "local.minor-raised-sixth"
	ContextMinorRaisedSeventh ContextProperty = "local.minor-raised-seventh"
)

const (
	localClefSign        contextRole = "local.clef-sign"
	localHarmonyFunction contextRole = "local.harmony-function"
)

// ContextDeprecatedNotation reports a permitted deprecated notation without rejecting it.
const ContextDeprecatedNotation ContextProperty = "local.deprecated-notation"
