package musicxml

import (
	"bytes"
	"context"
	"testing"
)

func TestRecoveryPackageMidSizedStoredArchiveUsesDefaultSteps(t *testing.T) {
	wire := packageGraphBytes(`<opus/>`, "main.opus", "", assessmentZIPFixture{name: "pad.bin", body: string(bytes.Repeat([]byte{'x'}, 11<<20))})
	b, err := InspectMXL(context.Background(), bytes.NewReader(wire), packageGraphOptions())
	if err != nil || b.ResourceCount() != 2 || !b.EnumerationComplete() {
		t.Fatalf("default-step archive: error=%v resources=%d complete=%v", err, b.ResourceCount(), b.EnumerationComplete())
	}
}
