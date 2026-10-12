package validation

import (
	"testing"
	"unsafe"
)

// An empty block can contribute independent order, coverage and enumeration
// gaps. Only two real notes are needed elsewhere to enable the grace pass.
func TestIndependentTimelineGraceEmptyBlockScratchGrowth(t *testing.T) {
	const realNotes int64 = 2
	var gaps []graceScopedGap
	width := int64(unsafe.Sizeof(graceScopedGap{}))
	for blocks := int64(1); blocks <= 4096; blocks++ {
		admitted := realNotes*timelineGraceScratchPerNote + blocks*timelineGraceScratchPerBlock + timelineGraceScratchFixed
		for i := 0; i < 3; i++ {
			old := cap(gaps)
			gaps = append(gaps, graceScopedGap{})
			peak := cap(gaps)
			if peak != old {
				peak += old
			}
			lowerBound := int64(peak) * width
			if lowerBound > admitted {
				t.Fatalf("blocks%d noteCount%d gapWidth%d liveGapCap%d lowerBound%d exceeds all admitted scratch%d before maps/streams", blocks, realNotes, width, peak, lowerBound, admitted)
			}
		}
	}
	t.Logf("closed gap record width%d", width)
}
