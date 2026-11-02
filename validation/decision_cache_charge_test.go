package validation

import (
	"runtime"
	"testing"
)

func decisionCacheHeap() int64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.HeapAlloc)
}

// The charge of cached failed decisions covers the heap the decisions map and
// their owned strings and causes occupy, plus 16 KiB for the runtime's own
// allocations, at every fill of the map's tables:
// counts just past a small table's doubling, around a large table's split
// and in the hundreds of thousands.
func TestFailedDecisionCacheChargeCoversItsHeap(t *testing.T) {
	if raceEnabled {
		t.Skip("the race detector changes the heap")
	}
	rule := RuleDefinition{Key: RuleKey{"test.decision-cache", 1}}
	for _, count := range []int{1, 8, 450, 900, 1800, 10_000, 120_000} {
		s := rationalSession(t, Budgets{MaxBufferedBytes: 1 << 40})
		s.decisions = make(map[decisionID]Decision)
		nodes := make([]factNode, count)
		for i := range nodes {
			nodes[i].header.ID = NodeID(i + 1)
		}
		before := decisionCacheHeap()
		for i := range nodes {
			d := Decision{Rule: rule.Key, Subject: NodeRef{"source", nodes[i].header.ID}, Outcome: OutcomeFail, Reason: "content-model-mismatch", Cause: &DecisionRef{Rule: RuleKey{"test.cause", 1}, Subject: NodeRef{"source", 1}}}
			if err := s.cacheDecision(rule, &nodes[i], d); err != nil {
				t.Fatal(err)
			}
		}
		heap := decisionCacheHeap() - before
		if heap > s.retained+16<<10 { // runtime noise between measurements
			t.Errorf("%d failed decisions: heap %d exceeds the charge %d", count, heap, s.retained)
		}
		runtime.KeepAlive(nodes)
		s.cleanup()
	}
}
