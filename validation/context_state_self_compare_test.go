package validation

import (
	"context"
	"testing"
)

func TestContextStateSelectedPublicationProjectedOnce(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 0)
	at := statePosition(t, 0, 0, 1)
	statePublish(t, s, &h.Staves, at, stateScope{AllStaves: true}, stateKnown(stateInt(t, 2)), 101)
	calls := 0
	got, err := queryState(context.Background(), s, h, &h.Staves, at, stateKnown(stateInt(t, 1)), func(v Integer) Evidence[Integer] { calls++; return stateKnown(v) })
	if err != nil || got.Value.State != EvidenceKnown || got.Value.Value.String() != "2" || len(got.Provenance) != 1 {
		t.Fatalf("%+v %v", got, err)
	}
	if calls != 1 {
		t.Fatalf("selected immutable value projected %d times", calls)
	}
}
