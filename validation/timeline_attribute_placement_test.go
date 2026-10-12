package validation

import (
	"context"
	"testing"
)

func TestTimelineAttributeGapPlacementScopeAndRepair(t *testing.T) {
	for _, bounded := range []bool{false, true} {
		t.Run(map[bool]string{false: "unconfined", true: "proved-bounds"}[bounded], func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			s.documentID = "d"
			h := &timelineState{}
			stateClose(t, s, h, 3)
			b := stateBindings()
			all := stateScope{AllStaves: true}
			at := statePosition(t, 0, 0, 1)
			statePublish(t, s, &h.Keys, at, all, stateKnown(keyState{Traditional: true, Fifths: stateKnown(stateInt(t, 0))}), 1)
			statePublish(t, s, &h.Divisions, at, all, stateKnown(stateDecimal(t, 1, 1)), 2)
			attrs := stateNode(b, timelineRoleAttributes, 10, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesKey, 11, stateAbsent[Atomic](), stateNode(b, timelineRoleKeyFifths, 12, stateAtomicInteger(t, 2))))
			stateAttribute(b, attrs.children[0], timelineRoleKeyNumber, Atomic{Kind: AtomicInteger, Integer: stateInt(t, 2)})
			cause := &DecisionRef{Rule: RuleKey{"movement-proof", 1}, Subject: NodeRef{"d", 9}}
			unavailable := Evidence[musicalPosition]{State: EvidenceUnsupported, Reason: "movement-units-unproved", Cause: cause}
			placement := stateGapPlacement{Displaced: true}
			lower, upper := statePosition(t, 0, 1, 1), statePosition(t, 0, 2, 1)
			if bounded {
				placement.Lower = &lower
				placement.Upper = &upper
			}
			if err := h.gapAttributes(context.Background(), s, b, attrs, at.Part, 1, unavailable, placement); err != nil {
				t.Fatal(err)
			}
			repair := statePosition(t, 2, 0, 1)
			statePublish(t, s, &h.Keys, repair, all, stateKnown(keyState{Traditional: true, Fifths: stateKnown(stateInt(t, 3))}), 30)
			for _, q := range []musicalPosition{at, statePosition(t, 0, 1, 1), repair} {
				got := stateQuery(t, s, h, &h.Keys, q, stateKnown(stateInt(t, 2)))
				want := EvidenceUnsupported
				if bounded && (q.MeasureOrdinal == 2 || q.Offset.String() == "0") {
					want = EvidenceKnown
				}
				if got.Value.State != want || want == EvidenceUnsupported && (got.Value.Cause == nil || *got.Value.Cause != *cause) {
					t.Fatalf("at%v/%s = %+v, want %s", q.MeasureOrdinal, q.Offset.String(), got, want)
				}
				if other := stateQuery(t, s, h, &h.Keys, q, stateKnown(stateInt(t, 1))); other.Value.State != EvidenceKnown {
					t.Fatalf("unrelated staff tainted: %+v", other)
				}
				if units := stateQuery(t, s, h, &h.Divisions, q, stateKnown(stateInt(t, 2))); units.Value.State != EvidenceKnown {
					t.Fatalf("unrelated family tainted: %+v", units)
				}
			}
			if len(h.Keys.Gaps) != 1 || h.Keys.Gaps[0].Placement.Displaced != true {
				t.Fatal("placement was dropped")
			}
			if bounded {
				lower.Offset = mustRational(t, 99, 1)
				upper.Offset = mustRational(t, 100, 1)
				if h.Keys.Gaps[0].Placement.Lower.Offset.String() != "1" || h.Keys.Gaps[0].Placement.Upper.Offset.String() != "2" {
					t.Fatal("placement endpoints alias caller")
				}
			}
			cause.Rule.ID = "mutated"
			if h.Keys.Gaps[0].Availability.Cause.Rule.ID != "movement-proof" {
				t.Fatal("cause aliases caller")
			}
		})
	}
}

func TestTimelineBroadGapPlacementPropagation(t *testing.T) {
	for _, kind := range []string{"attributes", "instrument", "all"} {
		t.Run(kind, func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			h := &timelineState{}
			at := statePosition(t, 0, 1, 1)
			gap := Evidence[musicalPosition]{State: EvidenceUnknown, Reason: "displaced-origin"}
			placement := stateGapPlacement{Displaced: true, Upper: &at}
			var err error
			switch kind {
			case "attributes":
				err = h.gapAttributeFamilies(context.Background(), s, at.Part, 1, gap, placement)
			case "instrument":
				err = h.gapInstrumentPublisher(context.Background(), s, at.Part, 1, gap, placement)
			case "all":
				err = h.gapAll(context.Background(), s, at.Part, 1, gap, placement)
			}
			if err != nil {
				t.Fatal(err)
			}
			groups := [][]stateGap{}
			if kind != "instrument" {
				groups = append(groups, h.StaffDetails.Gaps, h.Divisions.Gaps, h.Keys.Gaps, h.Times.Gaps, h.Staves.Gaps, h.Instruments.Gaps, h.Clefs.Gaps, h.AdditionalClefs.Gaps, h.Transpose.Gaps, h.CreatedPartClefs.Gaps, h.CreatedPartTranspose.Gaps)
			}
			if kind != "attributes" {
				groups = append(groups, h.InstrumentGaps, h.MIDIUnknownGaps)
			}
			for i, g := range groups {
				if len(g) != 1 || !g[0].Placement.Displaced || g[0].Placement.Upper == nil || g[0].Placement.Upper == &at {
					t.Fatalf("family%d lost owned placement: %+v", i, g)
				}
			}
			at.Offset = mustRational(t, 99, 1)
			for i, g := range groups {
				if g[0].Placement.Upper.Offset.String() != "1" {
					t.Fatalf("family%d aliases caller", i)
				}
			}
		})
	}
}
