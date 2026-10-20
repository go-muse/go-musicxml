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

// An outcome stored in the overflow map stays readable after the dense table
// grows over its ID, and storing another outcome for a node replaces it.
func TestOutcomeBitsOverflowSurvivesGrowth(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	s.sequence = make([]NodeID, 1)
	var b outcomeBits
	if err := b.set(s, 5000, successfulNotApplicable); err != nil || len(b.overflow) != 1 {
		t.Fatalf("not in overflow: %v %d", err, len(b.overflow))
	}
	s.sequence = make([]NodeID, 500)
	for _, id := range []NodeID{4097, 5001} {
		if err := b.set(s, id, successfulPass); err != nil {
			t.Fatal(err)
		}
	}
	if uint64(len(b.words))*32 < 5000 {
		t.Fatalf("dense table does not cover 5000: %d words", len(b.words))
	}
	if got := b.get(5000); got != successfulNotApplicable {
		t.Fatalf("overflow outcome lost after growth: %d", got)
	}
	if err := b.set(s, 4097, successfulNotApplicable); err != nil {
		t.Fatal(err)
	}
	if got := b.get(4097); got != successfulNotApplicable {
		t.Fatalf("replaced outcome reads %d", got)
	}
}
