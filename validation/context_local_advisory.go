package validation

import (
	"context"
	"strings"
)

// Recommendations are observable facts, never normative Findings. Their
// applicability is a separate causal axis; encoded values are not rewritten.
func localAdvisoryRole(name string) contextRole { return contextRole("local.advisory." + name) }

var localAdvisoryDivisionAttributes = []string{"divisions", "bezier-offset", "bezier-offset2", "make-time", "attack", "release", "offset"}

func localAdvisoryBindingSpecs() []contextBindingSpec {
	var out []contextBindingSpec
	for _, e := range []struct{ name, typ string }{{"divisions", "positive-divisions"}, {"duration", "positive-divisions"}, {"offset", "offset"}, {"cancel", "cancel"}, {"root", "root"}, {"numeral", "numeral"}, {"barline", "barline"}, {"harp-pedals", "harp-pedals"}, {"principal-voice", "principal-voice"}} {
		out = append(out, contextBindingSpec{Role: localAdvisoryRole(e.name), Name: e.name, Type: e.typ, Count: -1})
	}
	for _, a := range []struct{ owner, name, typ string }{{"cancel", "location", "cancel-location"}, {"barline", "location", "right-left-middle"}, {"principal-voice", "type", "start-stop"}, {"principal-voice", "symbol", "principal-voice-symbol"}} {
		out = append(out, contextBindingSpec{Role: localAdvisoryRole(a.owner + ".@" + a.name), Owner: a.owner, Name: a.name, Type: a.typ, Attribute: true, Count: 1})
	}
	for _, name := range localAdvisoryDivisionAttributes {
		out = append(out, contextBindingSpec{Role: localAdvisoryRole("divisions.@" + name), Name: name, Type: "divisions", Attribute: true, Count: -1})
	}
	for _, a := range []struct{ name, typ string }{{"implicit", "yes-no"}, {"number", "xs:token"}, {"text", "measure-text"}} {
		out = append(out, contextBindingSpec{Role: localAdvisoryRole("measure.@" + a.name), Name: a.name, Type: a.typ, Attribute: true, Count: -1})
	}
	return out
}

var localAdvisoryProperties = []ContextProperty{"local.advisory-integral-divisions", "local.advisory-cancel-midmeasure", "local.advisory-deprecated-clef", "local.advisory-harmony-none-root", "local.advisory-measure-text-hide", "local.advisory-measure-label-alignment", "local.advisory-barline-order", "local.advisory-harp-pedal-order", "local.advisory-principal-analysis", "local.advisory-principal-stop", "local.advisory-encoder-role", "local.advisory-creator-role", "local.advisory-supports-reporting", "local.advisory-lyric-literal-hyphen"}

var localAdvisoryDivisionProperties = func() []ContextProperty {
	out := make([]ContextProperty, len(localAdvisoryDivisionAttributes))
	for i, name := range localAdvisoryDivisionAttributes {
		out[i] = ContextProperty("local.advisory-integral-divisions." + name)
	}
	return out
}()

func localAdvisoryPropertyValid(p ContextProperty) bool {
	for _, v := range localAdvisoryProperties {
		if p == v {
			return true
		}
	}
	for _, property := range localAdvisoryDivisionProperties {
		if p == property {
			return true
		}
	}
	return false
}
func localAdvisoryCondition(raw Evidence[Atomic], want string) Evidence[bool] {
	if raw.State != EvidenceKnown {
		return stateUnavailable[Atomic, bool](raw)
	}
	return Evidence[bool]{State: EvidenceKnown, Value: raw.Value.Text == want}
}
func localAdvisoryNode(ctx context.Context, s *Session, key RuleKey, n *factNode, r *contextReader) error {
	active := Evidence[bool]{State: EvidenceKnown, Value: true}
	publish := func(property string, raw, value Evidence[Atomic], app Evidence[bool]) error {
		if r.err != nil {
			return r.err
		}
		return localPublish(ctx, s, key, n, ContextProperty("local.advisory-"+property), raw, value, app, NodeRef{}, ContextUnitNone)
	}
	recommendation := func(property string, raw Evidence[Atomic], text string, app Evidence[bool]) error {
		return publish(property, raw, localKnownText(text), app)
	}
	divisions := func(raw Evidence[Atomic], suffix string) error {
		if raw.State == EvidenceAbsent {
			return nil
		}
		app := stateUnavailable[Atomic, bool](raw)
		value := raw
		if raw.State == EvidenceKnown {
			app = stateUnknown[bool]("divisions-decimal-value-unavailable")
			value = stateUnknown[Atomic]("divisions-decimal-value-unavailable")
		}
		if raw.State == EvidenceKnown && raw.Value.Kind == AtomicDecimal && raw.Value.Decimal.Valid() {
			app = Evidence[bool]{State: EvidenceKnown, Value: !raw.Value.Decimal.value.IsInt()}
			value = localKnownText("prefer-integral-divisions-for-MIDI-interoperability-and-roundoff")
		}
		return publish("integral-divisions"+suffix, raw, value, app)
	}
	for _, name := range []string{"divisions", "duration", "offset"} {
		if r.matches(n, localAdvisoryRole(name)) {
			if e := divisions(r.scalar(n), ""); e != nil {
				return e
			}
		}
	}
	for _, name := range localAdvisoryDivisionAttributes {
		role := localAdvisoryRole("divisions.@" + name)
		if r.attributeOwner(n, role) {
			if e := divisions(localRawAttribute(r, n, role), "."+name); e != nil {
				return e
			}
		}
	}
	if r.matches(n, localAdvisoryRole("cancel")) {
		raw := localRawAttribute(r, n, localAdvisoryRole("cancel.@location"))
		effective := layoutDefault(raw, "left")
		app := localAdvisoryCondition(effective, "before-barline")
		if app.State == EvidenceKnown && app.Value {
			app = stateUnknown[bool]("midmeasure-applicability-requires-musical-position")
		}
		if e := recommendation("cancel-midmeasure", raw, "treat-before-barline-as-left-when-midmeasure", app); e != nil {
			return e
		}
	}
	if r.matches(n, localClefSign) {
		raw := r.scalar(n)
		if e := recommendation("deprecated-clef", raw, "prefer-clef-print-object-over-deprecated-none-sign", localAdvisoryCondition(raw, "none")); e != nil {
			return e
		}
	}
	if r.matches(n, localKind) {
		raw := r.scalar(n)
		app := localAdvisoryCondition(raw, "none")
		value := localKnownText("set-root-step-text-to-empty-to-suppress-root")
		if app.State == EvidenceKnown && app.Value {
			parent := s.nodes[n.header.Parent]
			var subject *factNode
			complete := r.complete(parent)
			if complete.State != EvidenceKnown {
				app = complete
			}
			if parent != nil && complete.State == EvidenceKnown {
				for _, ch := range parent.children {
					if !r.step(1) {
						return r.err
					}
					if ch == n {
						break
					}
					if r.matches(ch, localAdvisoryRole("root")) || r.matches(ch, localAdvisoryRole("numeral")) {
						subject = ch
					}
				}
			}
			if complete.State != EvidenceKnown {
				app = complete
			} else if subject == nil {
				app = stateUnknown[bool]("harmony-root-or-numeral-association-unavailable")
			} else if r.matches(subject, localAdvisoryRole("numeral")) {
				value = Evidence[Atomic]{State: EvidenceUnsupported, Reason: "source-numeral-step-reference-requires-interpretation"}
			}
		}
		if e := publish("harmony-none-root", raw, value, app); e != nil {
			return e
		}
	}
	if r.attributeOwner(n, localAdvisoryRole("measure.@implicit")) {
		raw := localRawAttribute(r, n, localAdvisoryRole("measure.@text"))
		if e := recommendation("measure-text-hide", raw, "use-implicit-yes-to-hide-measure-number-rather-than-empty-text", active); e != nil {
			return e
		}
		app := stateUnknown[bool]("aligned-partwise-left-barlines-not-established")
		if parent := s.nodes[n.header.Parent]; parent != nil && parent.header.Parent == 0 {
			app = Evidence[bool]{State: EvidenceKnown, Value: false}
		}
		if e := recommendation("measure-label-alignment", localRawAttribute(r, n, localAdvisoryRole("measure.@number")), "aligned-partwise-measures-prefer-equal-labels-nonnumeric-and-repeated-labels-remain-permitted", app); e != nil {
			return e
		}
	}
	if r.matches(n, localAdvisoryRole("barline")) {
		raw := localRawAttribute(r, n, localAdvisoryRole("barline.@location"))
		effective := layoutDefault(raw, "right")
		value := effective
		app := stateUnavailable[Atomic, bool](effective)
		if effective.State == EvidenceKnown {
			app = active
			switch effective.Value.Text {
			case "left":
				value = localKnownText("prefer-first-musical-element-except-print-bookmark-link")
			case "right":
				value = localKnownText("prefer-last-musical-element-except-print-bookmark-link")
			default:
				value = Evidence[Atomic]{State: EvidenceAbsent}
				app = Evidence[bool]{State: EvidenceKnown, Value: false}
			}
		}
		if e := publish("barline-order", raw, value, app); e != nil {
			return e
		}
	}
	if r.matches(n, localAdvisoryRole("harp-pedals")) {
		if e := recommendation("harp-pedal-order", Evidence[Atomic]{State: EvidenceAbsent}, "prefer-D-C-B-E-F-G-A-for-easiest-reading", r.complete(n)); e != nil {
			return e
		}
	}
	if r.matches(n, localAdvisoryRole("principal-voice")) {
		symbol := localRawAttribute(r, n, localAdvisoryRole("principal-voice.@symbol"))
		typ := localRawAttribute(r, n, localAdvisoryRole("principal-voice.@type"))
		analysis := stateUnknown[bool]("analysis-only-author-intent-unavailable")
		if symbol.State != EvidenceKnown {
			analysis = stateUnavailable[Atomic, bool](symbol)
		}
		if e := recommendation("principal-analysis", symbol, "analysis-only-principal-voice-prefers-symbol-none", analysis); e != nil {
			return e
		}
		app := layoutStateAnd(localAdvisoryCondition(typ, "stop"), layoutStateNot(localAdvisoryCondition(symbol, "none")))
		if e := recommendation("principal-stop", symbol, "visible-principal-voice-stop-prefers-symbol-plain", app); e != nil {
			return e
		}
	}
	if r.matches(n, localCreator) {
		raw := localRawAttribute(r, n, localMetadataType)
		if e := recommendation("creator-role", raw, "prefer-creator-type-even-for-one-creator-open-role-vocabulary", active); e != nil {
			return e
		}
	}
	if r.matches(n, localEncoder) {
		parent := s.nodes[n.header.Parent]
		app := r.complete(parent)
		count := 0
		if app.State == EvidenceKnown {
			for _, ch := range parent.children {
				if !r.step(1) {
					return r.err
				}
				if ch.header.Declaration.State != EvidenceKnown {
					app = stateUnavailable[DeclarationID, bool](ch.header.Declaration)
					break
				}
				if r.matches(ch, localEncoder) {
					count++
				}
			}
			if app.State == EvidenceKnown {
				app.Value = count > 1
			}
		}
		if e := recommendation("encoder-role", localRawAttribute(r, n, localMetadataType), "multiple-encoders-can-use-open-contribution-role-types", app); e != nil {
			return e
		}
	}
	if n.header.Parent == 0 {
		if e := recommendation("supports-reporting", Evidence[Atomic]{State: EvidenceAbsent}, "report-supported-accidental-beam-stem-encoding-to-interpret-omissions", active); e != nil {
			return e
		}
	}
	if r.matches(n, localText) {
		raw := r.scalar(n)
		app := stateUnavailable[Atomic, bool](raw)
		if raw.State == EvidenceKnown {
			if !r.step(int64(len(raw.Value.Text))) {
				return r.err
			}
			app = Evidence[bool]{State: EvidenceKnown, Value: strings.Contains(raw.Value.Text, "-")}
		}
		if e := recommendation("lyric-literal-hyphen", raw, "reserve-literal-hyphens-for-actually-hyphenated-words-use-syllabic-for-syllable-separation", app); e != nil {
			return e
		}
	}
	return r.err
}
