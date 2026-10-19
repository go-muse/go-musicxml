package validation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestPackageReviewTerminalJoiningPreservesContainingChain(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		s := beginPackageFixture(t, PackageCheckMimetypeStored)
		inner := contract("fixture transfer", "original-transfer-refusal")
		outer := fmt.Errorf("outer failure context: %w", inner)
		first, next := error(inner), error(outer)
		if reverse {
			first, next = outer, inner
		}
		s.fail(first)
		if got := s.fail(next); got != next {
			t.Fatal("fail changed its immediate return identity")
		}
		if s.terminalCause() != outer || strings.Count(s.terminalCause().Error(), "original-transfer-refusal") != 1 {
			t.Errorf("containing chain duplicated: %v", s.terminalCause())
		}
		b, err := s.finish(context.Background(), PackageCompletion{Cause: outer})
		var original *ConfigurationError
		if !errors.Is(err, inner) || !errors.As(err, &original) || original != inner || strings.Count(err.Error(), "original-transfer-refusal") != 1 {
			t.Errorf("finish lost identity or duplicated cause: %v", err)
		}
		if b.Satisfied() {
			t.Fatal("terminal package certified")
		}
	}
}

func TestPackageReviewTerminalJoiningKeepsIndependentCauses(t *testing.T) {
	s := beginPackageFixture(t, PackageCheckMimetypeStored)
	first := contract("fixture transfer", "first-independent-refusal")
	second := errors.New("second-independent-refusal")
	s.fail(first)
	s.fail(second)
	combined := s.terminalCause()
	s.fail(first)
	s.fail(second)
	if s.terminalCause() != combined {
		t.Fatal("already retained cause joined again")
	}
	_, err := s.finish(context.Background(), PackageCompletion{Cause: combined})
	if !errors.Is(err, first) || !errors.Is(err, second) || strings.Count(err.Error(), first.Error()) != 1 || strings.Count(err.Error(), second.Error()) != 1 {
		t.Fatalf("independent causes lost or duplicated: %v", err)
	}
}
