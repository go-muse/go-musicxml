package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unsafe"
)

func TestIndependentTimelineExcursionPartHashWorkAdmission(t *testing.T) {
	s := rationalSession(t, Budgets{MaxRuleSteps: 64})
	defer s.cleanup()
	part := NodeRef{DocumentID: strings.Repeat("document-id-", 1024), Node: 1}
	unavailable := stateUnknown[Decimal]("missing-divisions")
	v := &timelineService{Facts: map[NodeID]timelineEventFact{9: {Units: unavailable}}, unitExcursions: []timelineUnitExcursion{{Node: 9, Part: part, Raw: stateKnown(stateDecimal(t, 1, 1)), Locked: true, Availability: unavailable}}}
	err := v.finalizeExcursionUnits(context.Background(), s, stateBindings(), nil)
	if !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("large part hash completed with only64 work units: %v (work%d)", err, rationalUsage(t, s, BudgetRuleSteps))
	}
	if v.unitProofFinal || len(v.Facts) != 1 {
		t.Fatal("failed proof partially replaced original service")
	}
}

func TestIndependentTimelineExcursionClosedShape(t *testing.T) {
	checks := []struct {
		name           string
		need, admitted int64
	}{
		{"excursion", 3 * int64(unsafe.Sizeof(timelineUnitExcursion{})), timelineExcursionReserveBytes},
		{"publisher", int64(unsafe.Sizeof(timelineUnitPublisher{})), timelineUnitPublisherBytes},
		{"gate", 4 * int64(unsafe.Sizeof(NodeID(0))+unsafe.Sizeof(timelineExcursionGate{})), timelineUnitGateBytes},
		{"positive", 4 * int64(unsafe.Sizeof(NodeRef{})+unsafe.Sizeof(bool(false))), timelineUnitGateBytes},
		{"stage", int64(unsafe.Sizeof(timelineService{})) + 512, timelineUnitStageBytes},
	}
	for _, c := range checks {
		if c.need > c.admitted {
			t.Errorf("%s shape%d exceeds%d", c.name, c.need, c.admitted)
		}
		t.Logf("%s shape%d allowance%d", c.name, c.need, c.admitted)
	}
	var items []timelineUnitExcursion
	for i := int64(1); i <= 4096; i++ {
		old := cap(items)
		items = append(items, timelineUnitExcursion{})
		peak := cap(items)
		if peak != old {
			peak += old
		}
		if int64(peak)*int64(unsafe.Sizeof(timelineUnitExcursion{})) > i*timelineExcursionReserveBytes {
			t.Fatalf("excursion%d append peak exceeds admitted backing", i)
		}
	}
}
