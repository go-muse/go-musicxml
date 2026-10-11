package validation

import (
	"context"
	"errors"
	"testing"
)

func TestQAZeroSessionFinishFailsClosedWithoutPanic(t *testing.T) {
	for _, priorInput := range []bool{false, true} {
		t.Run(map[bool]string{false: "fresh-zero", true: "after-input-error"}[priorInput], func(t *testing.T) {
			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("zero-valued exported Session.Finish panicked instead of returning a typed error: %v", p)
				}
			}()
			var s Session
			if priorInput {
				if err := s.Open(context.Background(), NodeHeader{}); err == nil {
					t.Fatal("zero session accepted input")
				}
			}
			_, err := s.Finish(context.Background(), Completion{})
			if !errors.Is(err, ErrInvalidSession) && !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("zero session lacks machine-readable rejection: %v", err)
			}
		})
	}
}
