//go:build timeline_budget_evidence

package musicxml

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// This opt-in evidence probe measures the selected implementation, rather than
// asserting that its costs or fixture capacities are a permanent API contract.
// It is intentionally excluded from ordinary package test runs.
func pr35BudgetEvidenceLimits() validation.Budgets {
	b := validation.DefaultBudgets()
	b.MaxBufferedBytes = 256 << 20
	b.MaxRuleSteps = 100_000_000
	return b
}

func pr35BudgetEvidenceRun(d Document, profile validation.ProfileID, limits validation.Budgets, consumedContext int64) (validation.Report, *validation.Account, error) {
	ctx := context.Background()
	account, err := validation.NewBudgetAccount(limits)
	if err != nil {
		return validation.Report{}, nil, err
	}
	if err := account.Charge(validation.BudgetContextFacts, consumedContext); err != nil {
		return validation.Report{}, account, err
	}
	var engine *validation.Engine
	if profile == ProfileModelTimeline {
		engine, err = newTimelineAssessmentEngine()
	} else {
		engine, err = newModelAssessmentEngine()
	}
	if err != nil {
		return validation.Report{}, account, err
	}
	session, err := engine.Begin(ctx, validation.RunOptions{DocumentID: modelDocumentID, Profile: profile, Account: account})
	if err != nil {
		return validation.Report{}, account, err
	}
	_, walkErr := walkModel(ctx, d, session, account, modelDocumentID, modelWalkOptions{})
	report, finishErr := session.Finish(ctx, validation.Completion{InputComplete: walkErr == nil, Syntax: validation.SyntaxNotAssessed, Cause: walkErr})
	return report, account, errors.Join(walkErr, finishErr)
}

func pr35BudgetEvidenceResource(t *testing.T, r validation.Report, err error, kind validation.BudgetKind) *validation.ResourceError {
	t.Helper()
	var resource *validation.ResourceError
	if r.Satisfied() || !errors.Is(err, validation.ErrResourceLimit) || !errors.Is(err, validation.ErrIncompleteAssessment) || !errors.As(err, &resource) || resource.Kind != kind {
		t.Fatalf("expected incomplete %s, satisfied=%t err=%v", kind, r.Satisfied(), err)
	}
	return resource
}

func TestPR35TimelineBudgetEvidence(t *testing.T) {
	data, err := os.ReadFile("testdata/official/MozartTrio.musicxml")
	if err != nil {
		t.Fatal(err)
	}
	document, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []validation.ProfileID{ProfileModelLocal, ProfileModelTimeline} {
		t.Run(string(profile), func(t *testing.T) {
			run := func(limits validation.Budgets) (validation.Report, error) {
				return AssessModel(context.Background(), document, ModelOptions{Profile: profile, Budgets: limits})
			}
			r, err := run(validation.Budgets{})
			if err != nil || !r.Satisfied() {
				t.Fatalf("defaults: satisfied=%t err=%v", r.Satisfied(), err)
			}
			t.Log("DEFAULTS satisfied=true")
			r, account, err := pr35BudgetEvidenceRun(document, profile, pr35BudgetEvidenceLimits(), 0)
			if err != nil || !r.Satisfied() {
				t.Fatalf("lifted: satisfied=%t err=%v", r.Satisfied(), err)
			}
			usage := func(kind validation.BudgetKind) int64 {
				t.Helper()
				n, err := account.Usage(kind)
				if err != nil {
					t.Fatal(err)
				}
				return n
			}
			nodes, steps, facts := usage(validation.BudgetNodes), usage(validation.BudgetRuleSteps), usage(validation.BudgetContextFacts)
			if nodes != 2516 {
				t.Fatalf("fixture node count: got %d, want 2516", nodes)
			}
			if live := usage(validation.BudgetBufferedBytes); live != 0 {
				t.Fatalf("post-Finish live bytes: got %d, want zero", live)
			}
			low, high := int64(1), pr35BudgetEvidenceLimits().MaxBufferedBytes
			for low < high {
				mid := low + (high-low)/2
				b := pr35BudgetEvidenceLimits()
				b.MaxBufferedBytes = mid
				r, err := run(b)
				if err == nil && r.Satisfied() {
					high = mid
				} else {
					pr35BudgetEvidenceResource(t, r, err, validation.BudgetBufferedBytes)
					low = mid + 1
				}
			}
			for _, resource := range []struct {
				kind    validation.BudgetKind
				minimum int64
				set     func(*validation.Budgets, int64)
			}{
				{validation.BudgetBufferedBytes, low, func(b *validation.Budgets, n int64) { b.MaxBufferedBytes = n }},
				{validation.BudgetRuleSteps, steps, func(b *validation.Budgets, n int64) { b.MaxRuleSteps = n }},
				{validation.BudgetContextFacts, facts, func(b *validation.Budgets, n int64) { b.MaxContextFacts = n }},
			} {
				b := pr35BudgetEvidenceLimits()
				if resource.kind == validation.BudgetContextFacts && resource.minimum == 0 {
					// The local profile publishes no context facts. One is the
					// smallest expressible positive limit; zero selects defaults.
					b.MaxContextFacts = 1
					r, err := run(b)
					if err != nil || !r.Satisfied() {
						t.Fatalf("local one-fact cap: satisfied=%t err=%v", r.Satisfied(), err)
					}
					r, a, err := pr35BudgetEvidenceRun(document, profile, b, 1)
					used, usageErr := a.Usage(validation.BudgetContextFacts)
					if err != nil || !r.Satisfied() || usageErr != nil || used != 1 {
						t.Fatalf("zero remaining context: satisfied=%t used=%d err=%v usage_err=%v", r.Satisfied(), used, err, usageErr)
					}
					t.Log("BOUNDARY kind=context-facts used=0 smallest_positive_limit=1 satisfied=true; remaining=0 preconsumed=1 satisfied=true (zero configured selects defaults)")
					continue
				}
				resource.set(&b, resource.minimum)
				r, err := run(b)
				if err != nil || !r.Satisfied() {
					t.Fatalf("%s minimum %d: satisfied=%t err=%v", resource.kind, resource.minimum, r.Satisfied(), err)
				}
				t.Logf("BOUNDARY kind=%s limit=%d satisfied=true", resource.kind, resource.minimum)
				if resource.minimum == 1 && resource.kind == validation.BudgetContextFacts {
					// Zero in Budgets means the default, so a literal zero cap is
					// inexpressible. Reserve the one-unit cap before Begin to test
					// a shared account with zero remaining context allowance.
					r, _, err = pr35BudgetEvidenceRun(document, profile, b, 1)
					failure := pr35BudgetEvidenceResource(t, r, err, resource.kind)
					t.Logf("BOUNDARY kind=%s remaining=0 preconsumed=1 satisfied=false error=%v", resource.kind, failure)
				} else {
					resource.set(&b, resource.minimum-1)
					r, err = run(b)
					failure := pr35BudgetEvidenceResource(t, r, err, resource.kind)
					t.Logf("BOUNDARY kind=%s limit=%d satisfied=false error=%v", resource.kind, resource.minimum-1, failure)
				}
			}
			b := pr35BudgetEvidenceLimits()
			b.MaxBufferedBytes, b.MaxRuleSteps, b.MaxContextFacts = low, steps, max(facts, 1)
			r, err = run(b)
			if err != nil || !r.Satisfied() {
				t.Fatalf("combined exact minima: satisfied=%t err=%v", r.Satisfied(), err)
			}
			t.Logf("MEASURE nodes=%d min_buffer=%d steps=%d context_facts=%d buffer_per_node=%.4f steps_per_node=%.4f context_per_node=%.6f combined_minima_satisfied=true post_finish_live_buffer=0", nodes, low, steps, facts, float64(low)/float64(nodes), float64(steps)/float64(nodes), float64(facts)/float64(nodes))
		})
	}
}
