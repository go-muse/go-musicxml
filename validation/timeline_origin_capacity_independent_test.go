package validation

import (
	"testing"
	"unsafe"
)

func TestIndependentTimelineOriginCapacityAllowance(t *testing.T) {
	width := int64(unsafe.Sizeof(bendTimingOrigin{}))
	noteWidth := int64(unsafe.Sizeof(noteTimingOrigin{}))
	figureWidth := int64(unsafe.Sizeof(figuredBassTimingOrigin{}))
	t.Logf("fixed note=%d bend=%d figure=%d", noteWidth, width, figureWidth)
	if timelineBendReserveBytes < 3*width {
		t.Errorf("per-bend growth allowance %d is below three record widths %d", timelineBendReserveBytes, 3*width)
	}
	if noteWidth+256 > 4096 || figureWidth+256 > 2048 {
		t.Fatal("fixed origin records outgrew their retained map allowances")
	}
	var values []bendTimingOrigin
	for i := 0; i < 2048; i++ {
		old := cap(values)
		values = append(values, bendTimingOrigin{})
		if cap(values) != old {
			peak := int64(old+cap(values))*width + noteWidth
			reserved := int64(i+1)*timelineBendReserveBytes + 4096
			if peak > reserved {
				t.Errorf("append at count=%d: oldcap=%d newcap=%d backing+fixed lower-bound peak=%d exceeds total note+bend reservation=%d", i+1, old, cap(values), peak, reserved)
			}
		}
	}
}
