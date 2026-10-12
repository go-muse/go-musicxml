package validation

import "testing"

func TestLocalPrintingDefaultsAndOverrides(t *testing.T) {
	absent := Evidence[bool]{State: EvidenceAbsent}
	known := func(v bool) Evidence[bool] { return Evidence[bool]{State: EvidenceKnown, Value: v} }
	tests := []struct {
		name                                      string
		object, dot, lyric, leger, rest           Evidence[bool]
		wantObject, wantDot, wantLyric, wantLeger bool
		active                                    bool
	}{
		{"ordinary", absent, absent, absent, absent, known(false), true, true, true, true, true},
		{"invisible", known(false), absent, absent, absent, known(false), false, false, false, false, true},
		{"independent overrides", known(false), known(true), known(true), known(true), known(false), false, true, true, true, true},
		{"rest", absent, absent, absent, known(true), known(true), true, true, true, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := localPrinting(tt.object, tt.dot, tt.lyric, tt.leger, tt.rest)
			for i, p := range []Evidence[bool]{v.Object, v.Dot, v.Lyric, v.Leger, v.LegerActive} {
				want := []bool{tt.wantObject, tt.wantDot, tt.wantLyric, tt.wantLeger, tt.active}[i]
				if p.State != EvidenceKnown || p.Value != want {
					t.Fatalf("field %d: %+v want %v", i, p, want)
				}
			}
		})
	}
}
func TestLocalDefaultsPreserveCausalEvidence(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		raw := Evidence[bool]{State: state, Reason: "input", Cause: &DecisionRef{Rule: RuleKey{"invalid", 1}, Subject: NodeRef{"score", 2}}}
		got := localPrinting(raw, Evidence[bool]{State: EvidenceAbsent}, Evidence[bool]{State: EvidenceKnown, Value: true}, Evidence[bool]{State: EvidenceAbsent}, Evidence[bool]{State: EvidenceKnown, Value: false})
		if got.Object.State != state || got.Dot.State != state || got.Leger.State != state || got.Dot.Cause == nil || got.Dot.Cause.Rule.ID != "invalid" {
			t.Fatalf("lost causal state: %+v", got)
		}
		if got.Lyric.State != EvidenceKnown || !got.Lyric.Value {
			t.Fatal("unrelated explicit override blocked")
		}
	}
}
func TestLocalNoteTypeSizeMatrix(t *testing.T) {
	for _, tt := range []struct {
		grace, cue bool
		want       string
	}{{false, false, "full"}, {true, false, "cue"}, {false, true, "cue"}, {true, true, "grace-cue"}} {
		got := localNoteSize(Evidence[string]{State: EvidenceAbsent}, Evidence[bool]{State: EvidenceKnown, Value: tt.grace}, Evidence[bool]{State: EvidenceKnown, Value: tt.cue})
		if got.State != EvidenceKnown || got.Value != tt.want {
			t.Fatalf("%+v -> %+v", tt, got)
		}
	}
	got := localNoteSize(Evidence[string]{State: EvidenceKnown, Value: "large"}, Evidence[bool]{State: EvidenceUnknown}, Evidence[bool]{State: EvidenceUnknown})
	if got.State != EvidenceKnown || got.Value != "large" {
		t.Fatal("explicit size lost")
	}
}
func TestLocalLyricPrintOverride(t *testing.T) {
	no := Evidence[bool]{State: EvidenceKnown, Value: false}
	yes := Evidence[bool]{State: EvidenceKnown, Value: true}
	if got := localInheritedBool(Evidence[bool]{State: EvidenceAbsent}, no); got.State != EvidenceKnown || got.Value {
		t.Fatal("absent lyric overrode note")
	}
	if got := localInheritedBool(yes, no); got.State != EvidenceKnown || !got.Value {
		t.Fatal("explicit lyric did not override note")
	}
}
