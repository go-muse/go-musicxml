package validation

import "testing"

// Outcomes of densely numbered nodes live in the bit table; an ID far beyond
// the admitted nodes goes to the overflow map. Both read back as stored and
// leave their neighbours unchanged.
func TestOutcomeBitsDenseAndOverflow(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	s.sequence = make([]NodeID, 8)
	var b outcomeBits
	sparse := NodeID(2*len(s.sequence) + outcomeDenseSlack + 100)
	stored := map[NodeID]successfulOutcome{1: successfulPass, 2: successfulNotApplicable, 33: successfulPass, 64: successfulNotApplicable, sparse: successfulNotApplicable, sparse + 1: successfulPass}
	for id, v := range stored {
		if err := b.set(s, id, v); err != nil {
			t.Fatal(err)
		}
	}
	if len(b.overflow) != 2 {
		t.Fatalf("overflow entries %d, want 2", len(b.overflow))
	}
	for _, id := range []NodeID{1, 2, 3, 32, 33, 34, 64, 65, sparse, sparse + 1, sparse + 2} {
		if got := b.get(id); got != stored[id] {
			t.Errorf("node %d: outcome %d, want %d", id, got, stored[id])
		}
	}
}
