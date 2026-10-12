package validation

import (
	"context"
	"errors"
	"testing"
)

func independentTimelinePublisherGapGroups(v *timelineService) [][]stateGap {
	h := &v.State
	return [][]stateGap{h.StaffDetails.Gaps, h.Divisions.Gaps, h.Keys.Gaps, h.Times.Gaps, h.Staves.Gaps, h.Instruments.Gaps, h.Clefs.Gaps, h.AdditionalClefs.Gaps, h.Transpose.Gaps, h.CreatedPartClefs.Gaps, h.CreatedPartTranspose.Gaps, h.InstrumentGaps, h.MIDIUnknownGaps, v.Playback.Tempo.Gaps, v.Playback.Dynamics.Gaps, v.Playback.Pizzicato.Gaps}
}

func TestIndependentTimelinePublisherPlacementAdmissionAndOwnership(t *testing.T) {
	lower, upper := statePosition(t, 0, 1, 3), statePosition(t, 1, 2, 3)
	cause := &DecisionRef{Rule: RuleKey{"independent-placement", 1}, Subject: NodeRef{"d", 9}}
	gap := Evidence[musicalPosition]{State: EvidenceInvalid, Cause: cause, Reason: "actual-unit-failure"}
	placement := stateGapPlacement{Displaced: true, Lower: &lower, Upper: &upper}
	s := rationalSession(t, Budgets{})
	v := &timelineService{}
	if err := v.gapAllPublishers(context.Background(), s, lower.Part, 1, gap, placement); err != nil {
		t.Fatal(err)
	}
	groups := independentTimelinePublisherGapGroups(v)
	if got := rationalUsage(t, s, BudgetContextFacts); got != int64(len(groups)) {
		t.Fatalf("contexts%d groups%d", got, len(groups))
	}
	firstBytes := rationalUsage(t, s, BudgetBufferedBytes) / int64(len(groups))
	if firstBytes*int64(len(groups)) != rationalUsage(t, s, BudgetBufferedBytes) {
		t.Fatal("closed identical gap publications have inconsistent admission")
	}
	for i, group := range groups {
		if len(group) != 1 {
			t.Fatalf("group%d entries%d", i, len(group))
		}
		g := &group[0]
		if g.Placement.Lower == &lower || g.Placement.Upper == &upper || g.Availability.Cause == cause {
			t.Fatalf("group%d retained borrowed mutable record", i)
		}
		for j := 0; j < i; j++ {
			if g.Placement.Lower == groups[j][0].Placement.Lower || g.Placement.Upper == groups[j][0].Placement.Upper || g.Availability.Cause == groups[j][0].Availability.Cause {
				t.Fatalf("groups%d/%d share mutable copied record", i, j)
			}
		}
	}
	groups[0][0].Placement.Lower.Offset = mustRational(t, 99, 1)
	groups[0][0].Availability.Cause.Rule.ID = "changed-result"
	if lower.Offset.String() != "1/3" || cause.Rule.ID != "independent-placement" || groups[1][0].Placement.Lower.Offset.String() != "1/3" || groups[1][0].Availability.Cause.Rule.ID != cause.Rule.ID {
		t.Fatal("cross-family or caller mutation")
	}
	s.cleanup()
	if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatal("cleanup leaked")
	}
	for _, limit := range []int64{1, 7, 15} {
		s := rationalSession(t, Budgets{MaxContextFacts: limit})
		v := &timelineService{}
		err := v.gapAllPublishers(context.Background(), s, lower.Part, 1, gap, placement)
		count := 0
		for _, group := range independentTimelinePublisherGapGroups(v) {
			count += len(group)
		}
		if !errors.Is(err, ErrResourceLimit) || int64(count) != limit || rationalUsage(t, s, BudgetContextFacts) != limit {
			t.Fatalf("quota%d published%d: %v", limit, count, err)
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("quota cleanup leaked")
		}
	}
	s = rationalSession(t, Budgets{MaxBufferedBytes: firstBytes - 1})
	v = &timelineService{}
	err := v.gapAllPublishers(context.Background(), s, lower.Part, 1, gap, placement)
	if !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("first gap peak-minus1: %v", err)
	}
	for _, group := range independentTimelinePublisherGapGroups(v) {
		if len(group) != 0 {
			t.Fatal("gap published before byte admission")
		}
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatal("failed first admission retained bytes")
	}
	s.cleanup()
	t.Logf("all%d publisher families independently owned; first-gap admission%d bytes", len(groups), firstBytes)
}

func TestIndependentTimelinePublisherPlacementEveryCancellationBoundary(t *testing.T) {
	lower, upper := statePosition(t, 0, 1, 3), statePosition(t, 1, 2, 3)
	cause := &DecisionRef{Rule: RuleKey{"independent-placement", 1}, Subject: NodeRef{"d", 9}}
	gap := Evidence[musicalPosition]{State: EvidenceInvalid, Cause: cause, Reason: "actual-unit-failure"}
	placement := stateGapPlacement{Displaced: true, Lower: &lower, Upper: &upper}
	probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	s := rationalSession(t, Budgets{})
	v := &timelineService{}
	if err := v.gapAllPublishers(probe, s, lower.Part, 1, gap, placement); err != nil {
		t.Fatal(err)
	}
	s.cleanup()
	for poll := 1; poll <= probe.calls; poll++ {
		s := rationalSession(t, Budgets{})
		v := &timelineService{}
		err := v.gapAllPublishers(&rationalCancelContext{Context: context.Background(), at: poll}, s, lower.Part, 1, gap, placement)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("poll%d: %v", poll, err)
		}
		count := 0
		for _, group := range independentTimelinePublisherGapGroups(v) {
			count += len(group)
			for _, g := range group {
				if g.Placement.Lower == &lower || g.Placement.Upper == &upper || g.Availability.Cause == cause {
					t.Fatalf("poll%d published borrowed record", poll)
				}
			}
		}
		if count >= 16 || int64(count) != rationalUsage(t, s, BudgetContextFacts) {
			t.Fatalf("poll%d contexts/count mismatch", poll)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatalf("poll%d scratch leak", poll)
		}
		if lower.Offset.String() != "1/3" || upper.Offset.String() != "2/3" || cause.Rule.ID != "independent-placement" {
			t.Fatalf("poll%d changed caller", poll)
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatalf("poll%d cleanup leak", poll)
		}
	}
	t.Logf("all%d all-family Placement cancellation polls", probe.calls)
}
