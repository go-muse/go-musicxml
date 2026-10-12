package validation

import (
	"context"
	"testing"
)

// This independently expected growth bound rejects repeated whole-part scans
// for an invariant advisory. A tenfold ordinary input increase permits 15x work.
func TestIndependentContextPerformanceAdvisoryLinearWork(t *testing.T) {
	measure := func(count int) int64 {
		s := rationalSession(t, Budgets{MaxBufferedBytes: 512 << 20, MaxRuleSteps: 1 << 34})
		defer s.cleanup()
		service := &timelineService{}
		for i := 0; i < count; i++ {
			service.State.Divisions.Publications = append(service.State.Divisions.Publications, statePublication[Decimal]{Position: statePosition(t, uint64(i), 0, 1), Value: stateKnown(stateDecimal(t, 1, 1))})
		}
		index, err := buildPerformanceAdviceIndex(context.Background(), s, service)
		if err != nil {
			t.Fatal(err)
		}
		defer index.release(s)
		for i := 0; i < count; i++ {
			out := newTimingContext("event")
			out.Part = stateKnown(NodeRef{"d", 1})
			if err := performanceStateAdvice(context.Background(), s, index, localNode(NodeID(i+100), 0, "note"), &out); err != nil {
				t.Fatal(err)
			}
			if len(out.Advisories) != 1 || out.Advisories[0].Satisfied.State != EvidenceKnown || out.Advisories[0].Satisfied.Value {
				t.Fatal("independent invariant answer incorrect")
			}
		}
		return rationalUsage(t, s, BudgetRuleSteps)
	}
	small, large := measure(100), measure(1000)
	t.Logf("100 events/publications: %d steps; 1000: %d steps; ratio %.2f", small, large, float64(large)/float64(small))
	if large > small*15 {
		t.Fatalf("whole-part invariant advisory is superlinear: %d > 15*%d", large, small)
	}
}
