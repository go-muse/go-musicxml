package validation

import (
	"context"
	"errors"
	"testing"
)

func TestIndependentContextVoiceFilterAdmission(t *testing.T) {
	values := []Evidence[string]{stateKnown("x"), stateUnknown[string]("unknown-exclusion"), stateKnown("y")}
	const bytes = 512 + 3*1024
	for _, limit := range []int64{bytes, bytes - 1} {
		s := rationalSession(t, Budgets{MaxBufferedBytes: limit})
		filter, release, err := contextRepeatBuildVoiceFilter(context.Background(), s, values)
		if limit == bytes {
			if err != nil || filter == nil {
				t.Fatalf("exact filter admission: %v", err)
			}
			release()
		} else if err == nil || filter != nil || release != nil {
			t.Fatalf("partial filter at one-under: %v", err)
		}
		if got := rationalUsage(t, s, BudgetBufferedBytes); got != 0 {
			t.Fatalf("retained %d", got)
		}
		s.cleanup()
	}
	probe := &rationalCancelContext{Context: context.Background(), at: 1000000}
	s := rationalSession(t, Budgets{})
	_, release, err := contextRepeatBuildVoiceFilter(probe, s, values)
	if err != nil {
		t.Fatal(err)
	}
	release()
	s.cleanup()
	for at := 1; at <= probe.calls; at++ {
		s := rationalSession(t, Budgets{})
		filter, release, err := contextRepeatBuildVoiceFilter(&rationalCancelContext{Context: context.Background(), at: at}, s, values)
		if !errors.Is(err, context.Canceled) || filter != nil || release != nil || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatalf("build poll%d: %v", at, err)
		}
		s.cleanup()
	}
	t.Logf("all %d filter-build cancellation polls", probe.calls)
	s = rationalSession(t, Budgets{})
	defer s.cleanup()
	filter, release, err := contextRepeatBuildVoiceFilter(context.Background(), s, values)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	for _, voice := range []Evidence[string]{stateKnown("x"), stateUnknown[string]("unknown-voice")} {
		probe = &rationalCancelContext{Context: context.Background(), at: 1000000}
		if _, err = contextRepeatVoiceApplies(probe, s, filter, voice); err != nil {
			t.Fatal(err)
		}
		for at := 1; at <= probe.calls; at++ {
			if _, err = contextRepeatVoiceApplies(&rationalCancelContext{Context: context.Background(), at: at}, s, filter, voice); !errors.Is(err, context.Canceled) {
				t.Fatalf("query poll%d: %v", at, err)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if filter, release, err := contextRepeatBuildVoiceFilter(ctx, s, nil); !errors.Is(err, context.Canceled) || filter != nil || release != nil {
		t.Fatalf("empty pre-cancel accepted: %v", err)
	}
}
