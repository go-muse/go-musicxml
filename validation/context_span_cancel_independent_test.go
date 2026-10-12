package validation

import (
	"context"
	"errors"
	"testing"
)

func TestIndependentContextEmptySpanScratchCancellation(t *testing.T) {
	for name, run := range map[string]func(context.Context, *Session) error{
		"local-tie": func(ctx context.Context, s *Session) error { return spanLocalTiePairs(ctx, s, nil) },
		"vertical":  func(ctx context.Context, s *Session) error { return spanVerticalMembership(ctx, s, nil) },
		"part-group": func(ctx context.Context, s *Session) error {
			return spanPartGroupMembers(&contextReader{ctx: ctx, s: s}, nil)
		},
		"relational": func(ctx context.Context, s *Session) error {
			r := &contextReader{ctx: ctx, s: s}
			spanRelationalFacts(r, RuleKey{"p", 1}, nil, nil)
			return r.err
		},
	} {
		t.Run(name, func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			err := run(ctx, s)
			if !errors.Is(err, context.Canceled) || s.retained != 0 {
				t.Fatalf("pre-canceled helper allocated scratch or skipped cancellation: err=%v retained=%d", err, s.retained)
			}
		})
	}
}
