package validation

import (
	"context"
	"math/big"
)

type performanceDefault struct {
	role             contextRole
	owner, name, typ string
	fallback         Atomic
}

var performanceDefaults = []performanceDefault{
	{"perf-bend-accelerate", "bend", "accelerate", "yes-no", Atomic{Kind: AtomicString, Text: "no"}},
	{"perf-bend-beats", "bend", "beats", "trill-beats", performanceDecimalConstant(4)},
	{"perf-bend-first-beat", "bend", "first-beat", "percent", performanceDecimalConstant(25)},
	{"perf-bend-last-beat", "bend", "last-beat", "percent", performanceDecimalConstant(75)},
	{"perf-slide-accelerate", "slide", "accelerate", "yes-no", Atomic{Kind: AtomicString, Text: "no"}},
	{"perf-slide-beats", "slide", "beats", "trill-beats", performanceDecimalConstant(4)},
	{"perf-slide-first-beat", "slide", "first-beat", "percent", performanceDecimalConstant(25)},
	{"perf-slide-last-beat", "slide", "last-beat", "percent", performanceDecimalConstant(75)},
	{"perf-empty-trill-sound-start-note", "empty-trill-sound", "start-note", "start-note", Atomic{Kind: AtomicString, Text: "upper"}},
	{"perf-empty-trill-sound-trill-step", "empty-trill-sound", "trill-step", "trill-step", Atomic{Kind: AtomicString, Text: "whole"}},
	{"perf-empty-trill-sound-two-note-turn", "empty-trill-sound", "two-note-turn", "two-note-turn", Atomic{Kind: AtomicString, Text: "none"}},
	{"perf-empty-trill-sound-accelerate", "empty-trill-sound", "accelerate", "yes-no", Atomic{Kind: AtomicString, Text: "no"}},
	{"perf-empty-trill-sound-beats", "empty-trill-sound", "beats", "trill-beats", performanceDecimalConstant(4)},
	{"perf-empty-trill-sound-second-beat", "empty-trill-sound", "second-beat", "percent", performanceDecimalConstant(25)},
	{"perf-empty-trill-sound-last-beat", "empty-trill-sound", "last-beat", "percent", performanceDecimalConstant(75)},
	{"perf-horizontal-turn-start-note", "horizontal-turn", "start-note", "start-note", Atomic{Kind: AtomicString, Text: "upper"}},
	{"perf-horizontal-turn-trill-step", "horizontal-turn", "trill-step", "trill-step", Atomic{Kind: AtomicString, Text: "whole"}},
	{"perf-horizontal-turn-two-note-turn", "horizontal-turn", "two-note-turn", "two-note-turn", Atomic{Kind: AtomicString, Text: "none"}},
	{"perf-horizontal-turn-accelerate", "horizontal-turn", "accelerate", "yes-no", Atomic{Kind: AtomicString, Text: "no"}},
	{"perf-horizontal-turn-beats", "horizontal-turn", "beats", "trill-beats", performanceDecimalConstant(4)},
	{"perf-horizontal-turn-second-beat", "horizontal-turn", "second-beat", "percent", performanceDecimalConstant(25)},
	{"perf-horizontal-turn-last-beat", "horizontal-turn", "last-beat", "percent", performanceDecimalConstant(75)},
	{"perf-mordent-start-note", "mordent", "start-note", "start-note", Atomic{Kind: AtomicString, Text: "main"}},
	{"perf-mordent-trill-step", "mordent", "trill-step", "trill-step", Atomic{Kind: AtomicString, Text: "whole"}},
	{"perf-mordent-two-note-turn", "mordent", "two-note-turn", "two-note-turn", Atomic{Kind: AtomicString, Text: "none"}},
	{"perf-mordent-accelerate", "mordent", "accelerate", "yes-no", Atomic{Kind: AtomicString, Text: "no"}},
	{"perf-mordent-beats", "mordent", "beats", "trill-beats", performanceDecimalConstant(3)},
	{"perf-mordent-second-beat", "mordent", "second-beat", "percent", performanceDecimalConstant(12)},
	{"perf-mordent-last-beat", "mordent", "last-beat", "percent", performanceDecimalConstant(24)},
	{"perf-wavy-line-start-note", "wavy-line", "start-note", "start-note", Atomic{Kind: AtomicString, Text: "upper"}},
	{"perf-wavy-line-trill-step", "wavy-line", "trill-step", "trill-step", Atomic{Kind: AtomicString, Text: "whole"}},
	{"perf-wavy-line-two-note-turn", "wavy-line", "two-note-turn", "two-note-turn", Atomic{Kind: AtomicString, Text: "none"}},
	{"perf-wavy-line-accelerate", "wavy-line", "accelerate", "yes-no", Atomic{Kind: AtomicString, Text: "no"}},
	{"perf-wavy-line-beats", "wavy-line", "beats", "trill-beats", performanceDecimalConstant(4)},
	{"perf-wavy-line-second-beat", "wavy-line", "second-beat", "percent", performanceDecimalConstant(25)},
	{"perf-wavy-line-last-beat", "wavy-line", "last-beat", "percent", performanceDecimalConstant(75)},
	{"perf-fingering-substitution", "fingering", "substitution", "yes-no", Atomic{Kind: AtomicString, Text: "no"}},
	{"perf-fingering-alternate", "fingering", "alternate", "yes-no", Atomic{Kind: AtomicString, Text: "no"}},
	{"perf-level-reference", "level", "reference", "yes-no", Atomic{Kind: AtomicString, Text: "no"}},
	{"perf-level-type", "level", "type", "start-stop-single", Atomic{Kind: AtomicString, Text: "single"}},
	{"perf-stick-parentheses", "stick", "parentheses", "yes-no", Atomic{Kind: AtomicString, Text: "no"}},
	{"perf-stick-dashed-circle", "stick", "dashed-circle", "yes-no", Atomic{Kind: AtomicString, Text: "no"}},
	{"perf-root-alter-location", "root-alter", "location", "left-right", Atomic{Kind: AtomicString, Text: "right"}},
	{"perf-cancel-location", "cancel", "location", "cancel-location", Atomic{Kind: AtomicString, Text: "left"}},
	{"perf-double-above", "double", "above", "yes-no", Atomic{Kind: AtomicString, Text: "no"}},
	{"perf-figured-bass-parentheses", "figured-bass", "parentheses", "yes-no", Atomic{Kind: AtomicString, Text: "no"}},
}

func performanceDecimalConstant(n int64) Atomic {
	d, _ := NewDecimal(big.NewRat(n, 1))
	return Atomic{Kind: AtomicDecimal, Decimal: d}
}
func contextPerformanceBindingSpecs() []contextBindingSpec {
	out := make([]contextBindingSpec, 0, len(performanceDefaults)+len(performanceElements))
	for _, d := range performanceDefaults {
		out = append(out, contextBindingSpec{Role: d.role, Owner: performanceDefaultOwner(d.owner), Name: d.name, Type: d.typ, Attribute: true, Count: 1})
	}
	out = append(out, performanceElements...)
	return out
}

var performanceElements = []contextBindingSpec{
	{Role: "perf-backup", Name: "backup", Type: "backup", Count: -1},
	{Role: "perf-forward", Name: "forward", Type: "forward", Count: -1},
	{Role: "perf-level", Name: "level", Type: "level", Count: -1},
	{Role: "perf-root-print", Owner: "harmony-alter", Name: "print-object", Type: "yes-no", Attribute: true, Count: 1},
	{Role: "perf-root-alter", Owner: "root", Name: "root-alter", Type: "harmony-alter", Count: 1},
	{Role: "perf-note", Name: "note", Type: "note", Count: -1},
	{Role: "perf-sound", Name: "sound", Type: "sound", Count: -1},
	{Role: "perf-swing", Name: "swing", Type: "swing", Count: -1},
	{Role: "perf-swing-first", Name: "first", Type: "xs:positiveInteger", Count: -1},
	{Role: "perf-swing-second", Name: "second", Type: "xs:positiveInteger", Count: -1},
	{Role: "perf-swing-type", Name: "swing-type", Type: "swing-type-value", Count: -1},
	{Role: "perf-straight", Name: "straight", Type: "empty", Count: -1},
	{Role: "perf-type", Name: "type", Type: "note-type", Count: -1},
	{Role: "perf-cue", Name: "cue", Type: "empty", Count: -1},
	{Role: "perf-grace", Name: "grace", Type: "grace", Count: -1},
	{Role: "perf-bend", Name: "bend", Type: "bend", Count: -1},
	{Role: "perf-bend-alter", Name: "bend-alter", Type: "semitones", Count: -1},
	{Role: "perf-pre-bend", Name: "pre-bend", Type: "empty", Count: -1},
	{Role: "perf-release", Name: "release", Type: "release", Count: -1},
	{Role: "perf-fret", Name: "fret", Type: "fret", Count: -1},
	{Role: "perf-midi-unpitched", Name: "midi-unpitched", Type: "midi-128", Count: -1},
	{Role: "perf-pan", Name: "pan", Type: "rotation-degrees", Count: -1},
	{Role: "perf-elevation", Name: "elevation", Type: "rotation-degrees", Count: -1},
	{Role: "perf-tuning", Name: "staff-tuning", Type: "staff-tuning", Count: -1},
	{Role: "perf-staff-details", Name: "staff-details", Type: "staff-details", Count: -1},
	{Role: "perf-pitch", Name: "pitch", Type: "pitch", Count: -1},
	{Role: "perf-pitch-step", Name: "step", Type: "step", Count: -1},
	{Role: "perf-pitch-alter", Name: "alter", Type: "semitones", Count: -1},
	{Role: "perf-pitch-octave", Name: "octave", Type: "octave", Count: -1},
	{Role: "perf-prefix", Name: "prefix", Type: "style-text", Count: -1},
	{Role: "perf-suffix", Name: "suffix", Type: "style-text", Count: -1},
	{Role: "perf-accidental-mark", Name: "accidental-mark", Type: "accidental-mark", Count: -1},
	{Role: "perf-ornaments", Name: "ornaments", Type: "ornaments", Count: -1},
	{Role: "perf-tempo", Owner: "sound", Name: "tempo", Type: "non-negative-decimal", Attribute: true, Count: 1},
	{Role: "perf-dynamics", Owner: "sound", Name: "dynamics", Type: "non-negative-decimal", Attribute: true, Count: 1},
}

func contextPerformancePropertyValid(p ContextProperty) bool {
	if p == "performance-figure-prefix" || p == "performance-figure-suffix" || p == "performance" || p == "performance-default" || p == "performance-numbering" || p == "performance-origin" || p == "performance-open-string" {
		return true
	}
	for _, d := range performanceDefaults {
		if p == ContextProperty(d.role) {
			return true
		}
	}
	return false
}
func publishPerformanceScalar(r *contextReader, key RuleKey, n *factNode, property ContextProperty, v Evidence[Atomic]) {
	if r.err == nil {
		r.err = r.s.publishContextResult(r.ctx, ContextResult{Provider: key, Subject: NodeRef{r.s.documentID, n.header.ID}, Property: property, Value: v, Origins: []Location{n.header.Location}})
	}
}
func performanceDefaultOwner(owner string) string {
	if owner == "root-alter" {
		return "harmony-alter"
	}
	if owner == "mordent" {
		return "empty-trill-sound"
	}
	return owner
}
func performanceStatic(r *contextReader, key RuleKey, n *factNode) {
	for _, d := range performanceDefaults {
		if d.owner == "root-alter" && !r.matches(n, "perf-root-alter") {
			continue
		}
		if d.owner == "mordent" && (n.header.Type.State != EvidenceKnown || n.header.Type.Value != contextNamedType("mordent")) {
			continue
		}
		if d.owner == "empty-trill-sound" && n.header.Type.State == EvidenceKnown && n.header.Type.Value == contextNamedType("mordent") {
			continue
		}
		if !r.step(1) {
			return
		}
		if !r.attributeOwner(n, d.role) {
			continue
		}
		v := r.attribute(n, d.role)
		if v.State == EvidenceAbsent {
			v = Evidence[Atomic]{State: EvidenceKnown, Value: d.fallback, Reason: "documented-default"}
		}
		publishPerformanceScalar(r, key, n, ContextProperty(d.role), v)
	}
}
func publishPerformanceTiming(ctx context.Context, s *Session, key RuleKey, n *factNode, v *TimingContextResult) error {
	return s.publishContextResult(ctx, ContextResult{Provider: key, Subject: NodeRef{s.documentID, n.header.ID}, Property: ContextPropertyTiming, Origins: []Location{n.header.Location}, Timing: v})
}
