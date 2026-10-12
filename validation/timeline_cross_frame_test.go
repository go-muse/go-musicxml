package validation

import (
	"context"
	"testing"
)

func TestTimelineCrossFrameJointFloatingOrder(t *testing.T) {
	for _, tc := range []struct {
		name               string
		publication, query int64
		bound              int64
		known              bool
	}{
		{"joint-negative-proof", -2, -1, 0, true},
		{"zero-lower-bound-is-not-strict", -1, -1, 0, true},
		{"positive-overflow-competes", 10, 1, 0, false},
		{"independent-lower-bound", 10, 6, 5, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			h := &timelineState{}
			stateClose(t, s, h, 2)
			p, q := statePosition(t, 0, tc.publication, 1), statePosition(t, 1, tc.query, 1)
			p.UnprovedFrame = true
			q.UnprovedFrame = true
			if tc.bound > 0 {
				if err := h.observeFrameOrigin(ctx, s, statePosition(t, 0, tc.bound, 1), false); err != nil {
					t.Fatal(err)
				}
			}
			statePublish(t, s, &h.Staves, p, stateScope{AllStaves: true}, stateKnown(stateInt(t, 3)), 90)
			got, err := queryState(ctx, s, h, &h.Staves, q, stateKnown(stateOne()), stateWhole[Integer])
			if err != nil {
				t.Fatal(err)
			}
			if (got.Value.State == EvidenceKnown) != tc.known {
				t.Fatalf("result=%+v", got)
			}
			if tc.name == "zero-lower-bound-is-not-strict" && !got.InheritanceUnproved {
				t.Fatalf("possible equality claimed inherited: %+v", got)
			}
		})
	}
}
func TestTimelineCrossFrameConditionalCannotReviveRepairedPointGap(t *testing.T) {
	ctx := context.Background()
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 2)
	gapAt := statePosition(t, 1, -1, 1)
	gapAt.UnprovedFrame = true
	cause := &DecisionRef{Rule: RuleKey{"cross-frame-point-gap", 1}, Subject: NodeRef{"d", 70}}
	if err := addStateGap(ctx, s, &h.Staves, stateGap{Position: &gapAt, Part: gapAt.Part, FirstMeasure: 1, LastMeasure: 1, Scope: stateScope{AllStaves: true}, Availability: Evidence[bool]{State: EvidenceInvalid, Cause: cause}}); err != nil {
		t.Fatal(err)
	}
	early := statePosition(t, 0, 1, 1)
	got, err := queryState(ctx, s, h, &h.Staves, early, stateKnown(stateOne()), stateWhole[Integer])
	if err != nil || got.Value.State != EvidenceInvalid || got.Value.Cause == nil {
		t.Fatalf("later negative point gap excluded: %+v %v", got, err)
	}
	statePublish(t, s, &h.Staves, statePosition(t, 1, 0, 1), stateScope{AllStaves: true}, stateKnown(stateInt(t, 1)), 80)
	condition := stateKnown("2")
	if err := publishState(ctx, s, &h.Staves, statePublication[Integer]{Position: statePosition(t, 1, 1, 1), Scope: stateScope{AllStaves: true}, Value: stateKnown(stateInt(t, 2)), TimeOnly: &condition}); err != nil {
		t.Fatal(err)
	}
	got, err = queryState(ctx, s, h, &h.Staves, statePosition(t, 1, 2, 1), stateKnown(stateOne()), stateWhole[Integer])
	if err != nil || got.Value.State != EvidenceUnknown || got.Value.Cause != nil {
		t.Fatalf("conditional statement revived repaired gap: %+v %v", got, err)
	}
}
func TestTimelineCrossFrameInstrumentInitialConsensus(t *testing.T) {
	for _, same := range []bool{false, true} {
		t.Run(map[bool]string{true: "same", false: "different"}[same], func(t *testing.T) {
			ctx := context.Background()
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			h := &timelineState{}
			stateClose(t, s, h, 2)
			p := statePosition(t, 0, 10, 1)
			p.UnprovedFrame = true
			id := NodeRef{"d", 9}
			h.Definitions = []instrumentDefinition{{Part: p.Part, Instrument: id, Value: stateKnown(instrumentState{Sound: stateKnown("initial")}), Provenance: []stateProvenance{{Node: id, Kind: stateExplicit}}}}
			h.InstrumentChanges = []instrumentChanges{{Instrument: id}}
			value := "different"
			if same {
				value = "initial"
			}
			statePublish(t, s, &h.InstrumentChanges[0].History, p, stateScope{AllStaves: true}, stateKnown(instrumentState{Sound: stateKnown(value)}), 80)
			got, err := h.effectiveInstrumentStatements(ctx, s, statePosition(t, 1, 1, 1), id)
			if err != nil {
				t.Fatal(err)
			}
			if (got.Sound.Value.State == EvidenceKnown) != same {
				t.Fatalf("initial consensus=%+v", got.Sound)
			}
			if same && (len(got.Sound.Provenance) != 2 || !got.Sound.InheritanceUnproved) {
				t.Fatalf("initial proof/ordering metadata lost: %+v", got.Sound)
			}
		})
	}
}
