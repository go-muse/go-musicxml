package validation

import "testing"

// Transient storage is charged while it lives and released down to the mark
// of the evaluation that admitted it, so a nested evaluation releases only
// its own; cleanup releases what no evaluation took back.
func TestTransientRetentionFollowsTheEvaluation(t *testing.T) {
	s := rationalSession(t, Budgets{MaxBufferedBytes: 1000})
	outer := s.transient
	if err := s.retainTransient(300); err != nil || rationalUsage(t, s, BudgetBufferedBytes) != 300 {
		t.Fatalf("outer charge: %v usage=%d", err, rationalUsage(t, s, BudgetBufferedBytes))
	}
	inner := s.transient
	if err := s.retainTransient(500); err != nil {
		t.Fatal(err)
	}
	if err := s.retainTransient(201); err == nil {
		t.Fatal("transient storage beyond the buffer was admitted")
	}
	s.releaseTransient(inner)
	if rationalUsage(t, s, BudgetBufferedBytes) != 300 || s.transient != 300 || s.retained != 0 {
		t.Fatalf("nested release: usage=%d transient=%d", rationalUsage(t, s, BudgetBufferedBytes), s.transient)
	}
	s.releaseTransient(outer)
	if rationalUsage(t, s, BudgetBufferedBytes) != 0 || s.transient != 0 {
		t.Fatalf("outer release: usage=%d", rationalUsage(t, s, BudgetBufferedBytes))
	}
	if err := s.retainTransient(400); err != nil {
		t.Fatal(err)
	}
	s.cleanup()
	if rationalUsage(t, s, BudgetBufferedBytes) != 0 || s.transient != 0 {
		t.Fatalf("cleanup left %d", rationalUsage(t, s, BudgetBufferedBytes))
	}
}

// An evaluation's scope releases only what it admitted: a nested evaluation
// returns with the enclosing one's transient storage still charged.
func TestTransientScopeNests(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	err := s.transientScope(func() error {
		if err := s.retainTransient(100); err != nil {
			return err
		}
		if err := s.transientScope(func() error { return s.retainTransient(50) }); err != nil {
			return err
		}
		if got := rationalUsage(t, s, BudgetBufferedBytes); got != 100 {
			t.Fatalf("after the nested scope: usage %d, want 100", got)
		}
		return nil
	})
	if err != nil || rationalUsage(t, s, BudgetBufferedBytes) != 0 || s.transient != 0 {
		t.Fatalf("after the scope: %v usage=%d", err, rationalUsage(t, s, BudgetBufferedBytes))
	}
}
