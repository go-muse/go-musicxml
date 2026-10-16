package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSourceDTDIDValueAdmissionBeforeComparison(t *testing.T) {
	t.Run("oversized", func(t *testing.T) {
		s, key := dtdIDSession(t)
		defer s.cleanup()
		f := dtdIDAdmittedAttribute(t, s, key, "a", "value")
		f.Value = strings.Repeat("x", int(s.account.Limits().MaxScalarBytes)+1)
		e := s.SourceDTDIDAttribute(context.Background(), f)
		var resource *ResourceError
		if !errors.As(e, &resource) || resource.Kind != BudgetScalarBytes {
			t.Fatalf("oversized input compared before cap: %v", e)
		}
	})
	t.Run("work", func(t *testing.T) {
		s, key := dtdIDSession(t)
		defer s.cleanup()
		f := dtdIDAdmittedAttribute(t, s, key, "a", "value")
		used, _ := s.account.Usage(BudgetRuleSteps)
		hold := s.account.Limits().MaxRuleSteps - used - int64(len(f.Value))
		if e := s.account.Charge(BudgetRuleSteps, hold); e != nil {
			t.Fatal(e)
		}
		if e := s.SourceDTDIDAttribute(context.Background(), f); !errors.Is(e, ErrResourceLimit) {
			t.Fatalf("input compared before work admission: %v", e)
		}
	})
	t.Run("canceled", func(t *testing.T) {
		s, key := dtdIDSession(t)
		defer s.cleanup()
		f := dtdIDAdmittedAttribute(t, s, key, "a", "value")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if e := s.SourceDTDIDAttribute(ctx, f); !errors.Is(e, context.Canceled) {
			t.Fatalf("canceled ingress: %v", e)
		}
	})
}
