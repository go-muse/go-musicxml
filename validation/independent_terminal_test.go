package validation

import (
	"context"
	"errors"
	"testing"
)

func TestIndependentReviewNonSourceSyntaxAssertionIsContractFailure(t *testing.T) {
	for _, target := range []Target{TargetModel, TargetPackage} {
		t.Run(string(target), func(t *testing.T) {
			rules, profiles, bindings := catalogFixture(t)
			rules[0].Targets = []Target{target}
			profiles[0].Target = target
			catalog, err := NewCatalog(rules, profiles, bindings)
			if err != nil {
				t.Fatal(err)
			}
			engine, _ := NewEngine(catalog)
			s, err := engine.Begin(context.Background(), RunOptions{DocumentID: "doc", Profile: profiles[0].ID})
			if err != nil {
				t.Fatal(err)
			}
			r, err := s.Finish(context.Background(), Completion{Syntax: SyntaxMalformed})
			if !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("invalid non-source syntax contract lost: %v", err)
			}
			if r.Terminal == nil || r.Terminal.Category != CategoryCapability || r.Terminal.Reason != "invalid-configuration" {
				t.Errorf("non-source adapter contract must not manufacture source syntax failure: %+v", r.Terminal)
			}
		})
	}
}

func TestIndependentReviewUnestablishedSourceSyntaxIsContractFailure(t *testing.T) {
	for _, complete := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing-cause", true: "contradictory-complete"}[complete], func(t *testing.T) {
			_, s := sessionFixture(t, Budgets{})
			qaOpenRoot(t, s)
			qaClose(t, s, factEnd(1))
			end := Completion{InputComplete: complete, Syntax: SyntaxMalformed}
			if complete {
				end.Cause = errors.New("adapter syntax claim")
			}
			r, err := s.Finish(context.Background(), end)
			want := Code("invalid-session")
			if complete {
				want = "invalid-configuration"
			}
			if r.Terminal == nil || r.Terminal.Category != CategoryCapability || r.Terminal.Reason != want {
				t.Errorf("unestablished syntax became malformed-source: %+v error=%v", r.Terminal, err)
			}
			if r.Counts.Fail != 1 || r.Conformance != ConformanceViolated {
				t.Errorf("earlier failure disappeared: %+v", r.Counts)
			}
		})
	}
}
