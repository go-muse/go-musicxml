package validation

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func rrStream(t *testing.T, s *Session, count int) {
	t.Helper()
	qaOpenRoot(t, s)
	for i := 0; i < count; i++ {
		id := NodeID(i + 2)
		if err := s.Open(context.Background(), factHeader(id, 1, "test.child", "child")); err != nil {
			t.Fatal(err)
		}
		qaClose(t, s, factEnd(id))
	}
	qaClose(t, s, factEnd(1))
}

func TestIndependentReview1200SuccessesMustNotHideLateFailure(t *testing.T) {
	child := DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: "test.child"}
	rootPlan, _ := NewChildCountPlan([]DeclarationID{child}, 1201, -1)
	passPlan, _ := NewChildCountPlan([]DeclarationID{child}, 0, -1)
	s := qaSession(t, qaSpec{id: "a-child-pass", occurrence: "test.child", p: passPlan}, qaSpec{id: "z-root-failure", occurrence: "test.root", p: rootPlan})
	rrStream(t, s, 1200)
	a := s.account
	r, err := s.Finish(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("counts=%+v findings=%d truncated=%t", r.Counts, len(r.Findings), r.DiagnosticsTruncated)
	if r.Counts.Pass != 1200 || r.Counts.Fail != 1 || r.Counts.NotApplicable != 1201 {
		t.Errorf("authoritative counts changed: %+v", r.Counts)
	}
	if len(r.Findings) != 1 || r.Findings[0].Rule.ID != "z-root-failure" || r.Findings[0].Outcome != OutcomeFail || r.DiagnosticsTruncated {
		t.Errorf("success/NA consumed diagnostics before the root failure: findings=%d truncated=%t", len(r.Findings), r.DiagnosticsTruncated)
	}
	for _, k := range []BudgetKind{BudgetDepth, BudgetBufferedBytes} {
		if n, _ := a.Usage(k); n != 0 {
			t.Errorf("leaked %s=%d", k, n)
		}
	}
}

func rrDocumentDependencySession(t *testing.T) *Session {
	child := DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: "test.child"}
	p, _ := NewChildCountPlan([]DeclarationID{child}, 0, -1)
	return qaSession(t, qaSpec{id: "a-prerequisite", occurrence: "test.child", p: p}, qaSpec{id: "z-consumer", occurrence: "test.child", p: p, requires: []Dependency{{Kind: DependencyRule, Rule: RuleKey{ID: "a-prerequisite", Revision: 1}, Relation: RelationDocument}}})
}

func TestIndependentReview5000DocumentDependenciesLinear(t *testing.T) {
	var priorSteps int64
	for _, count := range []int{1000, 5000} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			s := rrDocumentDependencySession(t)
			rrStream(t, s, count)
			a := s.account
			r, err := s.Finish(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
			steps, _ := a.Usage(BudgetRuleSteps)
			// Keep the scheduler oracle independent of the separately bounded
			// retained-tree allocation/copy work.
			storage := testFlatFactStorageWork(count)
			if steps < storage {
				t.Fatalf("total steps=%d below predicted storage work=%d", steps, storage)
			}
			steps -= storage
			t.Logf("children=%d scheduler-steps=%d storage-steps=%d counts=%+v terminal=%v error=%v", count, steps, storage, r.Counts, r.Terminal, err)
			if err != nil || !r.Satisfied() {
				t.Errorf("ordinary document prerequisite stopped at default quota: %v; %+v", err, r.Counts)
			}
			if steps > int64(count)*30+100 {
				t.Errorf("document prerequisite work is not linear: children=%d steps=%d", count, steps)
			}
			if priorSteps != 0 && steps > priorSteps*6 {
				t.Errorf("5x nodes grew rule steps beyond linear slack: %d -> %d", priorSteps, steps)
			}
			priorSteps = steps
			for _, k := range []BudgetKind{BudgetDepth, BudgetBufferedBytes} {
				if n, _ := a.Usage(k); n != 0 {
					t.Errorf("leaked %s=%d", k, n)
				}
			}
		})
	}
}

// This oracle intentionally does not call the production ranking or cache helpers.
func rrExpectedAggregate(a, b Outcome, complete bool) (Outcome, int) {
	for i, o := range []Outcome{a, b} {
		if o == OutcomeFail || o == OutcomeBlocked {
			return OutcomeBlocked, i
		}
	}
	if !complete {
		return OutcomeUnknown, -1
	}
	for _, o := range []Outcome{a, b} {
		if o == OutcomeUnsupported {
			return OutcomeUnsupported, -1
		}
	}
	for _, o := range []Outcome{a, b} {
		if o == OutcomeUnknown {
			return OutcomeUnknown, -1
		}
	}
	for _, o := range []Outcome{a, b} {
		if o == OutcomePass {
			return OutcomePass, -1
		}
	}
	return OutcomeUnknown, -1
}
func rrCount(c *Counts, o Outcome, n uint64) {
	switch o {
	case OutcomePass:
		c.Pass += n
	case OutcomeFail:
		c.Fail += n
	case OutcomeNotApplicable:
		c.NotApplicable += n
	case OutcomeUnknown:
		c.Unknown += n
	case OutcomeUnsupported:
		c.Unsupported += n
	case OutcomeBlocked:
		c.Blocked += n
	}
}

func TestIndependentReviewDocumentOutcomeOracle(t *testing.T) {
	outcomes := []Outcome{OutcomePass, OutcomeFail, OutcomeNotApplicable, OutcomeUnknown, OutcomeUnsupported, OutcomeBlocked}
	for _, a := range outcomes {
		for _, b := range outcomes {
			for _, input := range []bool{true, false} {
				for _, enumeration := range []bool{true, false} {
					t.Run(fmt.Sprintf("%s_%s_input-%t_enum-%t", a, b, input, enumeration), func(t *testing.T) {
						pred, _ := NewAtomicComparisonPlan(ComparisonEqual, Atomic{Kind: AtomicString, Text: "good"})
						pass, _ := NewChildCountPlan([]DeclarationID{{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: "test.child"}}, 0, -1)
						dep := []Dependency{{Kind: DependencyRule, Rule: RuleKey{ID: "a-prerequisite", Revision: 1}, Relation: RelationDocument}}
						s := qaSession(t, qaSpec{id: "a-prerequisite", occurrence: "test.child", p: pred}, qaSpec{id: "y-consumer", occurrence: "test.root", p: pass, requires: dep}, qaSpec{id: "z-consumer", occurrence: "test.root", p: pass, requires: dep})
						qaOpenRoot(t, s)
						for i, o := range []Outcome{a, b} {
							id := NodeID(i + 2)
							h := factHeader(id, 1, "test.child", "child")
							end := factEnd(id)
							switch o {
							case OutcomePass:
								end.Scalar.Value = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "good"}}
							case OutcomeFail:
								end.Scalar.Value = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "bad"}}
							case OutcomeNotApplicable:
								h.Declaration.Value.Occurrence = "test.other"
							case OutcomeUnknown:
								end.Scalar.Value = Evidence[Atomic]{State: EvidenceUnknown, Reason: "oracle-unknown"}
							case OutcomeUnsupported:
								end.Scalar.Value = Evidence[Atomic]{State: EvidenceUnsupported, Reason: "oracle-unsupported"}
							case OutcomeBlocked:
								end.Scalar.Value = Evidence[Atomic]{State: EvidenceInvalid, Reason: "oracle-invalid", Cause: &DecisionRef{Rule: RuleKey{ID: "upstream", Revision: 1}, Subject: NodeRef{DocumentID: "doc", Node: NodeID(i + 19)}}}
							}
							if err := s.Open(context.Background(), h); err != nil {
								t.Fatal(err)
							}
							qaClose(t, s, end)
						}
						end := factEnd(1)
						if !enumeration {
							end.ChildrenComplete = Evidence[bool]{State: EvidenceUnknown, Reason: "oracle-enumeration"}
						}
						qaClose(t, s, end)
						r := qaFinish(t, s, input)
						want, origin := rrExpectedAggregate(a, b, input && enumeration)
						counts := Counts{NotApplicable: 5}
						rrCount(&counts, a, 1)
						rrCount(&counts, b, 1)
						rrCount(&counts, want, 2)
						if r.Counts != counts {
							t.Fatalf("counts=%+v want=%+v aggregate=%s", r.Counts, counts, want)
						}
						for _, rule := range []RuleID{"y-consumer", "z-consumer"} {
							found := false
							for _, f := range r.Findings {
								if f.Rule.ID != rule || f.Subject.Node != 1 {
									continue
								}
								found = true
								if f.Outcome != want {
									t.Errorf("aggregate=%s want=%s", f.Outcome, want)
								}
								if origin >= 0 {
									o := []Outcome{a, b}[origin]
									node := NodeID(origin + 2)
									causeRule := RuleID("a-prerequisite")
									if o == OutcomeBlocked {
										node = NodeID(origin + 19)
										causeRule = "upstream"
									}
									if f.Cause == nil || f.Cause.Rule.ID != causeRule || f.Cause.Subject.Node != node {
										t.Errorf("wrong first authoritative cause: %+v", f.Cause)
									}
								}
							}
							if want != OutcomePass && !found {
								t.Errorf("missing problem finding %s", rule)
							}
						}
					})
				}
			}
		}
	}
}

func TestIndependentReviewCancellationReleasesRetainedCache(t *testing.T) {
	s := rrDocumentDependencySession(t)
	rrStream(t, s, 1000)
	a := s.account
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if !errors.Is(err, context.Canceled) || r.Terminal == nil {
		t.Errorf("lost cancellation: %+v %v", r.Terminal, err)
	}
	for _, k := range []BudgetKind{BudgetDepth, BudgetBufferedBytes} {
		if n, _ := a.Usage(k); n != 0 {
			t.Errorf("leaked %s=%d", k, n)
		}
	}
}

func TestIndependentReviewCompactSuccessCacheBudget(t *testing.T) {
	p, _ := NewChildCountPlan([]DeclarationID{{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: "test.child"}}, 0, -1)
	specs := make([]qaSpec, 40)
	for i := range specs {
		specs[i] = qaSpec{id: fmt.Sprintf("rule-%02d", i), occurrence: "test.child", p: p}
	}
	initial := qaSession(t, specs...)
	catalog := initial.catalog
	initial.cleanup()
	engine, err := NewEngine(catalog)
	if err != nil {
		t.Fatal(err)
	}
	s, err := engine.Begin(context.Background(), RunOptions{DocumentID: "doc", Profile: "test.source", Budgets: Budgets{MaxBufferedBytes: 8 << 20}})
	if err != nil {
		t.Fatal(err)
	}
	a := s.account
	qaOpenRoot(t, s)
	for i := 0; i < 1000; i++ {
		id := NodeID(i + 2)
		if err = s.Open(context.Background(), factHeader(id, 1, "test.child", "child")); err != nil {
			break
		}
		if err = s.Close(context.Background(), factEnd(id)); err != nil {
			break
		}
	}
	if err == nil {
		err = s.Close(context.Background(), factEnd(1))
	}
	retained, _ := a.Usage(BudgetBufferedBytes)
	t.Logf("40 rules x 1001 nodes: retained=%d error=%v", retained, err)
	r, finishErr := s.Finish(context.Background(), Completion{InputComplete: err == nil, Syntax: SyntaxWellFormed})
	if err != nil || finishErr != nil || !r.Satisfied() {
		t.Errorf("compact success/NA cache should fit 8 MiB: event=%v finish=%v counts=%+v", err, finishErr, r.Counts)
	}
	if err == nil && (r.Counts.Pass != 40000 || r.Counts.NotApplicable != 40) {
		t.Errorf("cache lost successful decisions: %+v", r.Counts)
	}
	for _, k := range []BudgetKind{BudgetDepth, BudgetBufferedBytes} {
		if n, _ := a.Usage(k); n != 0 {
			t.Errorf("leaked %s=%d", k, n)
		}
	}
}

type rrCancelDuringWork struct {
	context.Context
	remaining int
	cancel    context.CancelFunc
}

func (c *rrCancelDuringWork) Err() error {
	c.remaining--
	if c.remaining == 0 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestIndependentReviewCancellationInsideDocumentAggregate(t *testing.T) {
	s := rrDocumentDependencySession(t)
	rrStream(t, s, 1000)
	a := s.account
	childCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &rrCancelDuringWork{Context: childCtx, remaining: 200, cancel: cancel}
	r, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if !errors.Is(err, context.Canceled) || r.Terminal == nil || r.Terminal.Category != CategoryResource {
		t.Errorf("document aggregate cancellation not classified/preserved: %+v %v", r.Terminal, err)
	}
	if r.Counts.Pass != 1000 || r.Counts.NotApplicable != 2 || r.Counts.Fail != 0 || r.AssessmentComplete {
		t.Errorf("partial scan manufactured/lost decisions: %+v", r.Counts)
	}
	for _, k := range []BudgetKind{BudgetDepth, BudgetBufferedBytes} {
		if n, _ := a.Usage(k); n != 0 {
			t.Errorf("leaked %s=%d", k, n)
		}
	}
}

func TestIndependentReviewExcludedConsumerDoesNotScanDocument(t *testing.T) {
	child := DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: "test.child"}
	p, _ := NewChildCountPlan([]DeclarationID{child}, 0, -1)
	var baseline int64
	for _, excluded := range []bool{false, true} {
		specs := []qaSpec{{id: "a-prerequisite", occurrence: "test.child", p: p}}
		if excluded {
			specs = append(specs, qaSpec{id: "z-excluded", occurrence: "test.never", p: p, requires: []Dependency{{Kind: DependencyRule, Rule: RuleKey{ID: "a-prerequisite", Revision: 1}, Relation: RelationDocument}}})
		}
		s := qaSession(t, specs...)
		rrStream(t, s, 1000)
		a := s.account
		r := qaFinish(t, s, true)
		steps, _ := a.Usage(BudgetRuleSteps)
		if !r.Satisfied() {
			t.Errorf("excluded consumer changed conformance: %+v", r.Counts)
		}
		if !excluded {
			baseline = steps
		} else if steps-baseline > 1011 {
			t.Errorf("excluded consumer performed document scan: extra=%d, want at most 1001 selector checks plus constant work", steps-baseline)
		}
	}
}

func TestIndependentReviewIdentityDuplicateOutranksUnavailableFamily(t *testing.T) {
	for _, unavailableFirst := range []bool{false, true} {
		t.Run(fmt.Sprint(unavailableFirst), func(t *testing.T) {
			declare, _ := NewIdentityDeclarationPlan("ids", ciSelector())
			reference, _ := NewIdentityReferencePlan("ids", ciSelector())
			s := qaSession(t, qaSpec{id: "declare", occurrence: "test.decl", p: declare}, qaSpec{id: "reference", occurrence: "test.ref", p: reference})
			qaOpenRoot(t, s)
			values := []Atomic{{Kind: AtomicString, Text: "K"}, {Kind: AtomicString, Text: "K"}, {Kind: AtomicBoolean, Text: "true"}}
			if unavailableFirst {
				values[0], values[2] = values[2], values[0]
			}
			for i, value := range values {
				id := NodeID(i + 2)
				if err := s.Open(context.Background(), factHeader(id, 1, "test.decl", "child")); err != nil {
					t.Fatal(err)
				}
				attribute := ciStringNode(id, "unused").attributes[0]
				attribute.Scalar.Value = Evidence[Atomic]{State: EvidenceKnown, Value: value}
				if err := s.Attribute(context.Background(), attribute); err != nil {
					t.Fatal(err)
				}
				qaClose(t, s, factEnd(id))
			}
			ciStreamNode(t, s, 5, 1, "test.ref", "K")
			qaClose(t, s, factEnd(1))
			r := qaFinish(t, s, true)
			if r.Terminal != nil || r.Counts.Fail != 1 || r.Counts.Blocked != 2 || r.Counts.Unsupported != 1 || r.Counts.Unknown != 0 {
				t.Errorf("unavailable family erased established collision: counts=%+v terminal=%+v", r.Counts, r.Terminal)
			}
		})
	}
}
