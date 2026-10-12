package validation

import (
	"context"
	"testing"
)

func TestTimelineFinalizersCrossMeasureOrigins(t *testing.T) {
	s, service, b := runtimeFixture(t)
	attr := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 3, 1)))
	figure := stateNode(b, timelineRoleFiguredBass, 30, stateAbsent[Atomic]())
	direction := stateNode(b, timelineRoleDirection, 40, stateAbsent[Atomic]())
	blocks := []timelineMeasureBlock{runtimeBlock(b, 0, attr, figure, direction), runtimeBlock(b, 1, runtimeNote(t, b, 50, 1, false))}
	for _, block := range blocks {
		if err := service.readMeasure(context.Background(), s, b, block); err != nil {
			t.Fatal(err)
		}
	}
	if len(service.FiguredBassOrigins) != 0 || len(service.DirectionOrigins) != 0 {
		t.Fatal("cross-measure origins published before document-wide closure")
	}
	if err := service.finalizeTimingOrigins(context.Background(), s, b, blocks); err != nil {
		t.Fatal(err)
	}
	figureOrigin := service.FiguredBassOrigins[30]
	if figureOrigin.Anchor.State != EvidenceKnown || figureOrigin.Anchor.Value.Node != 50 {
		t.Fatalf("figure anchor=%+v", figureOrigin.Anchor)
	}
	directionOrigin := service.DirectionOrigins[40]
	if directionOrigin.Anchor.State != EvidenceKnown || directionOrigin.Anchor.Value.Node != 50 {
		t.Fatalf("direction anchor=%+v", directionOrigin.Anchor)
	}
}
