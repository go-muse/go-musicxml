package validation

import (
	"context"
	"testing"
)

func TestTimelineExcursionGatePreservesDescendantUnitUnavailability(t *testing.T) {
	ctx := context.Background()
	s, v, b := runtimeFixture(t)
	attrs := func(id NodeID, d int64) *factNode {
		return stateNode(b, timelineRoleAttributes, id, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, id+1, stateAtomicDecimal(t, d, 1)))
	}
	forward := stateNode(b, timelineRoleForward, 40, stateAbsent[Atomic](), stateNode(b, timelineRoleForwardDuration, 41, stateAtomicDecimal(t, 10, 1)))
	blocks := []timelineMeasureBlock{runtimeBlock(b, 0, attrs(20, 1), runtimeNote(t, b, 30, 1, false), forward, runtimeNote(t, b, 50, 1, false)), runtimeBlock(b, 1, attrs(60, 2), runtimeNote(t, b, 70, 2, false))}
	for _, block := range blocks {
		if err := v.readMeasure(ctx, s, b, block); err != nil {
			t.Fatal(err)
		}
	}
	if err := v.finalizeExcursionUnits(ctx, s, b, blocks); err != nil {
		t.Fatal(err)
	}
	fact := v.Facts[50]
	if fact.Duration.State == EvidenceKnown || fact.Units.State == EvidenceKnown || !fact.Placement.Displaced {
		t.Fatalf("descendant retained unsupported lexical unit normalization: %+v", fact)
	}
	if first := v.Facts[30]; first.Duration.State != EvidenceKnown {
		t.Fatalf("earlier independently proved duration lost: %+v", first)
	}
}
