package validation

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func reviewDocumentSession(t *testing.T, limits Budgets) *Session {
	t.Helper()
	e := reviewDocumentEngine(t, nil)
	s, err := e.Begin(context.Background(), RunOptions{DocumentID: "doc", Profile: "test.source", Budgets: limits})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func reviewDocumentEngine(t *testing.T, edit func(*[]RuleDefinition)) *Engine {
	t.Helper()
	rules, profiles, _ := catalogFixture(t)
	pass, err := NewChildCountPlan([]DeclarationID{{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: "test.child"}}, 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	rules[0].Key = RuleKey{ID: "a-prerequisite", Revision: 1}
	rules[0].Selector.Declaration.Occurrence = "test.child"
	rules[0].Predicate = "pass"
	consumer := cloneRule(rules[0])
	consumer.Key.ID = "b-consumer"
	consumer.Requires = []Dependency{{Kind: DependencyRule, Rule: rules[0].Key, Relation: RelationDocument}}
	rules = append(rules, consumer)
	if edit != nil {
		edit(&rules)
	}
	profiles[0].Required, profiles[0].Support = nil, nil
	for _, rule := range rules {
		profiles[0].Required = append(profiles[0].Required, rule.Key)
		profiles[0].Support = append(profiles[0].Support, RuleSupport{Rule: rule.Key, State: ImplementationAvailable})
	}
	c, err := NewCatalog(rules, profiles, []PlanBinding{{ID: "pass", Plan: pass}})
	if err != nil {
		t.Fatal(err)
	}
	e, err := NewEngine(c)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestReviewDocumentPrerequisiteLinearWork(t *testing.T) {
	var previous int64
	for _, count := range []int{1000, 5000} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			s := reviewDocumentSession(t, Budgets{})
			account := s.account
			qaOpenRoot(t, s)
			for i := 0; i < count; i++ {
				id := NodeID(i + 2)
				if err := s.OpenNode(context.Background(), factHeader(id, 1, "test.child", "child")); err != nil {
					t.Fatal(err)
				}
				qaClose(t, s, factEnd(id))
			}
			qaClose(t, s, factEnd(1))
			report, err := s.Finish(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
			work, _ := account.Usage(BudgetRuleSteps)
			// The original bound covers scheduler work; retained-container
			// growth has an independently derived exact copy-work ledger.
			storage := testFlatFactStorageWork(count)
			if work < storage {
				t.Fatalf("total work=%d below predicted storage work=%d", work, storage)
			}
			work -= storage
			t.Logf("children=%d scheduler-steps=%d storage-steps=%d", count, work, storage)
			if err != nil {
				t.Fatalf("same-selector document prerequisite exhausted default work: %v", err)
			}
			if !report.Satisfied() || report.Counts.Pass != uint64(2*count) || report.Counts.NotApplicable != 2 {
				t.Fatalf("report=%+v", report)
			}
			if work > int64(12*count+20) {
				t.Errorf("work=%d exceeds linear bound for %d children", work, count)
			}
			if previous != 0 && work > 5*previous {
				t.Errorf("5x input used more than 5x work: %d -> %d", previous, work)
			}
			previous = work
			for _, kind := range []BudgetKind{BudgetBufferedBytes, BudgetDepth} {
				if used, _ := account.Usage(kind); used != 0 {
					t.Errorf("%s leaked %d", kind, used)
				}
			}
		})
	}
}

func TestReviewSuccessfulDecisionRetentionIsCompact(t *testing.T) {
	_, s := sessionFixture(t, Budgets{MaxBufferedBytes: 1 << 20})
	defer s.cleanup()
	rule := s.catalog.rules[s.catalog.orders[s.profile.ID][0]]
	before := s.retained
	for i := 0; i < 10000; i++ {
		outcome := OutcomePass
		if i%2 != 0 {
			outcome = OutcomeNotApplicable
		}
		n := &factNode{header: testRetainedHeader(factHeader(NodeID(i+1), 0, "test.root", "root")), end: testRetainedEnd(NodeEnd{})}
		if err := s.record(rule, n, Decision{Outcome: outcome}); err != nil {
			t.Fatalf("compact result %d: %v", i, err)
		}
	}
	if got := s.retained - before; got > 64*10000+512 {
		t.Fatalf("successful results retained %d bytes; want bounded compact per-entry storage", got)
	}
	if len(s.decisions) != 0 {
		t.Fatalf("full problem cache retained %d successful decisions", len(s.decisions))
	}
}

func TestReviewOversizedBindingsRejectBeforeHeaderAccounting(t *testing.T) {
	_, s := sessionFixture(t, Budgets{})
	defer s.cleanup()
	before := s.retained
	h := factHeader(1, 0, "test.root", "root")
	h.Bindings = make([]Binding, 65537)
	if err := s.OpenNode(context.Background(), h); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("oversized bindings: %v", err)
	}
	if s.retained != before || s.depth != 0 || len(s.nodes) != 0 {
		t.Fatal("oversized bindings reached header traversal/accounting")
	}
}

// This oracle is intentionally independent of the runtime's reduction loop.
// First find the earliest decisive blocker, then compare the last missing and
// unsupported positions, then fall back to the first unknown or applicability.
func reviewPrerequisiteOracle(decisions []Decision, complete, enumerationComplete bool) Decision {
	for _, d := range decisions {
		if d.Outcome != OutcomeFail && d.Outcome != OutcomeBlocked {
			continue
		}
		if d.Outcome == OutcomeFail {
			d.Outcome, d.Reason = OutcomeBlocked, "failed-prerequisite"
			d.Cause = &DecisionRef{Rule: d.Rule, Subject: d.Subject}
		} else if d.Cause == nil {
			d.Cause = &DecisionRef{Rule: d.Rule, Subject: d.Subject}
		}
		return d
	}
	if !complete || !enumerationComplete {
		return Decision{Outcome: OutcomeUnknown, Reason: "prerequisite-document-incomplete"}
	}
	missing, unsupported, unknown := -1, -1, -1
	applicable := false
	for i, d := range decisions {
		switch d.Outcome {
		case "":
			missing = i
		case OutcomeNotApplicable:
		case OutcomeUnsupported:
			unsupported = i
			applicable = true
		case OutcomeUnknown:
			if unknown == -1 {
				unknown = i
			}
			applicable = true
		default:
			applicable = true
		}
	}
	if unsupported > missing {
		return decisions[unsupported]
	}
	if missing >= 0 {
		return Decision{Outcome: OutcomeUnknown, Reason: "prerequisite-not-established"}
	}
	if unknown >= 0 {
		return decisions[unknown]
	}
	if !applicable {
		return Decision{Outcome: OutcomeUnknown, Reason: "prerequisite-not-applicable"}
	}
	return Decision{Outcome: OutcomePass}
}

func TestReviewDocumentPrerequisiteIndependentOracle(t *testing.T) {
	states := []Outcome{"", OutcomePass, OutcomeNotApplicable, OutcomeUnknown, OutcomeUnsupported, OutcomeFail, OutcomeBlocked}
	key := RuleKey{ID: "a-prerequisite", Revision: 1}
	dependency := Dependency{Kind: DependencyRule, Rule: key, Relation: RelationDocument}
	cases := 0
	for _, first := range states {
		for _, second := range states {
			for _, third := range states {
				for _, complete := range []bool{false, true} {
					for _, enumerationComplete := range []bool{false, true} {
						s := reviewDocumentSession(t, Budgets{})
						s.inputComplete, s.enumerationIncomplete = complete, !enumerationComplete
						s.documentPrerequisites[key].finalized = true
						rule := s.catalog.rules[key]
						decisions := make([]Decision, 3)
						for i, outcome := range []Outcome{first, second, third} {
							node := NodeID(i + 1)
							s.sequence = append(s.sequence, node)
							if outcome == "" {
								continue
							}
							d := Decision{Outcome: outcome, Rule: key, Subject: NodeRef{"doc", node}}
							if outcome != OutcomePass && outcome != OutcomeNotApplicable {
								d.Reason = Code(fmt.Sprintf("reason-%d", i))
							}
							if outcome == OutcomeBlocked && i%2 == 0 {
								d.Cause = &DecisionRef{Rule: RuleKey{ID: "upstream", Revision: 2}, Subject: NodeRef{"other-document", NodeID(99 - i)}}
							}
							decisions[i] = d
							n := &factNode{header: testRetainedHeader(factHeader(node, 0, "test.child", "child")), end: testRetainedEnd(NodeEnd{})}
							if err := s.record(rule, n, d); err != nil {
								t.Fatal(err)
							}
						}
						want := reviewPrerequisiteOracle(decisions, complete, enumerationComplete)
						n := &factNode{header: testRetainedHeader(factHeader(99, 0, "test.child", "child")), end: testRetainedEnd(NodeEnd{})}
						got, pending, err := s.dependency(context.Background(), n, dependency, true)
						if err != nil || pending || !reflect.DeepEqual(got, want) {
							t.Fatalf("states=%v complete=%v/%v got=%+v pending=%v err=%v want=%+v", []Outcome{first, second, third}, complete, enumerationComplete, got, pending, err, want)
						}
						before, _ := s.account.Usage(BudgetRuleSteps)
						again, pending, err := s.dependency(context.Background(), n, dependency, true)
						after, _ := s.account.Usage(BudgetRuleSteps)
						if err != nil || pending || !reflect.DeepEqual(again, want) || after-before != 1 {
							t.Fatalf("aggregate not cached exactly: before=%d after=%d got=%+v err=%v", before, after, again, err)
						}
						s.cleanup()
						cases++
					}
				}
			}
		}
	}
	t.Logf("checked %d outcome/completeness combinations", cases)
}

func TestReviewUnusedDocumentPrerequisiteDoesNotScan(t *testing.T) {
	const count = 2000
	// Each child performs two selector checks. The root's selected prerequisite
	// also counts children (two steps each), plus three initialization steps.
	// No document aggregate is demanded. Separately reserve the exact growth
	// work predicted from this flat tree's node count, without runtime readings.
	storage := testFlatFactStorageWork(count)
	e := reviewDocumentEngine(t, func(rules *[]RuleDefinition) {
		(*rules)[0].Selector.Declaration.Occurrence = "test.root"
		(*rules)[1].Selector.Declaration.Occurrence = "never-selected"
	})
	s, err := e.Begin(context.Background(), RunOptions{DocumentID: "doc", Profile: "test.source", Budgets: Budgets{MaxRuleSteps: 4*count + 5 + storage}})
	if err != nil {
		t.Fatal(err)
	}
	qaOpenRoot(t, s)
	for i := 0; i < count; i++ {
		id := NodeID(i + 2)
		if err := s.OpenNode(context.Background(), factHeader(id, 1, "test.child", "child")); err != nil {
			t.Fatal(err)
		}
		qaClose(t, s, factEnd(id))
	}
	qaClose(t, s, factEnd(1))
	account := s.account
	report, err := s.Finish(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	work, _ := account.Usage(BudgetRuleSteps)
	if err != nil || !report.Satisfied() {
		t.Fatalf("excluded consumers triggered document scan: work=%d report=%+v err=%v", work, report, err)
	}
	work -= storage
	if work != 4*count+5 {
		t.Fatalf("unexpected work: %d", work)
	}
}

func TestReviewDocumentPrerequisiteFinalizationAndScope(t *testing.T) {
	for _, scope := range []ScopeKind{ScopeNode, ScopeDocument} {
		t.Run(string(scope), func(t *testing.T) {
			s := reviewDocumentSession(t, Budgets{})
			defer s.cleanup()
			key := RuleKey{ID: "a-prerequisite", Revision: 1}
			dependency := Dependency{Kind: DependencyRule, Rule: key, Relation: RelationDocument}
			rule := s.catalog.rules[key]
			rule.Scope = scope
			s.catalog.rules[key] = rule
			root := &factNode{header: testRetainedHeader(factHeader(17, 0, "test.root", "root")), end: testRetainedEnd(NodeEnd{})}
			s.root = root
			s.sequence = []NodeID{17, 2}
			s.inputComplete = true
			if err := s.record(rule, root, Decision{Outcome: OutcomePass}); err != nil {
				t.Fatal(err)
			}
			child := &factNode{header: testRetainedHeader(factHeader(2, 17, "test.child", "child")), end: testRetainedEnd(NodeEnd{})}
			if err := s.record(rule, child, Decision{Outcome: OutcomeFail, Reason: "child-failure"}); err != nil {
				t.Fatal(err)
			}
			if _, pending, err := s.dependency(context.Background(), child, dependency, false); err != nil || !pending {
				t.Fatalf("pre-Finish did not defer: %v %v", pending, err)
			}
			if _, _, err := s.dependency(context.Background(), child, dependency, true); !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("unfinalized phase was read: %v", err)
			}
			if s.documentPrerequisites[key].ready {
				t.Fatal("unfinalized or registration phase populated cache")
			}
			s.documentPrerequisites[key].finalized = true
			got, pending, err := s.dependency(context.Background(), child, dependency, true)
			want := OutcomeBlocked
			if scope == ScopeDocument {
				want = OutcomePass
			}
			if err != nil || pending || got.Outcome != want {
				t.Fatalf("scope=%s got=%+v pending=%v err=%v", scope, got, pending, err)
			}
			if scope == ScopeNode && (got.Cause == nil || got.Cause.Subject.Node != 2) {
				t.Fatalf("wrong exact child cause: %+v", got)
			}
		})
	}
}

func TestReviewCompactCachePreservesMissingAndIdempotence(t *testing.T) {
	_, s := sessionFixture(t, Budgets{})
	defer s.cleanup()
	key := s.catalog.orders[s.profile.ID][0]
	rule := s.catalog.rules[key]
	dependency := Dependency{Kind: DependencyRule, Rule: key, Relation: RelationSameNode}
	for i, outcome := range []Outcome{OutcomePass, OutcomeNotApplicable, ""} {
		node := &factNode{header: testRetainedHeader(factHeader(NodeID(i+1), 0, "test.root", "root")), end: testRetainedEnd(NodeEnd{})}
		if outcome != "" {
			if err := s.record(rule, node, Decision{Outcome: outcome}); err != nil {
				t.Fatal(err)
			}
			retained := s.retained
			counts := s.accumulator.report.Counts
			if err := s.record(rule, node, Decision{Outcome: OutcomeFail}); err != nil {
				t.Fatal(err)
			}
			if s.retained != retained || s.accumulator.report.Counts != counts {
				t.Fatal("duplicate compact result changed counts or retained bytes")
			}
		}
		got, pending, err := s.dependency(context.Background(), node, dependency, true)
		want := Decision{Outcome: OutcomePass}
		if outcome == OutcomeNotApplicable {
			want = Decision{Outcome: OutcomeUnknown, Reason: "prerequisite-not-applicable"}
		}
		if outcome == "" {
			want = Decision{Outcome: OutcomeUnknown, Reason: "prerequisite-not-established"}
		}
		if err != nil || pending || !reflect.DeepEqual(got, want) {
			t.Fatalf("outcome=%s got=%+v want=%+v pending=%v err=%v", outcome, got, want, pending, err)
		}
	}
}

func TestReviewDocumentCacheCancellationAndWorkExhaustion(t *testing.T) {
	key := RuleKey{ID: "a-prerequisite", Revision: 1}
	dep := Dependency{Kind: DependencyRule, Rule: key, Relation: RelationDocument}
	for _, cached := range []bool{false, true} {
		for _, canceled := range []bool{false, true} {
			s := reviewDocumentSession(t, Budgets{})
			s.inputComplete = true
			s.documentPrerequisites[key].finalized = true
			n := &factNode{header: testRetainedHeader(factHeader(1, 0, "test.child", "child")), end: testRetainedEnd(NodeEnd{})}
			s.sequence = []NodeID{1}
			if err := s.record(s.catalog.rules[key], n, Decision{Outcome: OutcomePass}); err != nil {
				t.Fatal(err)
			}
			if cached {
				if _, _, err := s.dependency(context.Background(), n, dep, true); err != nil {
					t.Fatal(err)
				}
			}
			ctx := context.Background()
			want := error(ErrResourceLimit)
			if canceled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
				want = context.Canceled
			} else {
				used, _ := s.account.Usage(BudgetRuleSteps)
				if err := s.account.Charge(BudgetRuleSteps, s.account.Limits().MaxRuleSteps-used); err != nil {
					t.Fatal(err)
				}
			}
			if _, _, err := s.dependency(ctx, n, dep, true); !errors.Is(err, want) {
				t.Fatalf("cached=%v canceled=%v lost interruption: %v", cached, canceled, err)
			}
			if !cached && s.documentPrerequisites[key].ready {
				t.Fatal("interrupted aggregate cached")
			}
			s.cleanup()
		}
	}
}

// Err polling makes cancellation during initialization deterministic without a
// scheduler race. Begin must check the context while walking the compiled DAG.
type reviewCancelDuringBegin struct {
	context.Context
	calls, after int
}

func (c *reviewCancelDuringBegin) Err() error {
	c.calls++
	if c.calls >= c.after {
		return context.Canceled
	}
	return nil
}

func TestReviewBeginIndexingAccountsWorkAndCleanup(t *testing.T) {
	e := reviewDocumentEngine(t, func(rules *[]RuleDefinition) {
		extra := cloneRule((*rules)[0])
		extra.Key.ID = "c-independent"
		*rules = append(*rules, extra)
	})
	for _, cancel := range []bool{false, true} {
		t.Run(fmt.Sprint(cancel), func(t *testing.T) {
			account, err := NewBudgetAccount(Budgets{MaxRuleSteps: 3})
			if err != nil {
				t.Fatal(err)
			}
			const siblingBytes, siblingDepth = 321, 2
			if err := account.Charge(BudgetBufferedBytes, siblingBytes); err != nil {
				t.Fatal(err)
			}
			if err := account.Charge(BudgetDepth, siblingDepth); err != nil {
				t.Fatal(err)
			}
			ctx := context.Context(context.Background())
			want := error(ErrResourceLimit)
			if cancel {
				ctx = &reviewCancelDuringBegin{Context: ctx, after: 5}
				want = context.Canceled
			}
			s, err := e.Begin(ctx, RunOptions{DocumentID: "doc", Profile: "test.source", Account: account})
			if s != nil || !errors.Is(err, want) || !errors.Is(err, ErrIncompleteAssessment) {
				t.Fatalf("unbounded Begin: session=%v err=%v", s, err)
			}
			for _, item := range []struct {
				kind BudgetKind
				want int64
			}{{BudgetBufferedBytes, siblingBytes}, {BudgetDepth, siblingDepth}, {BudgetRuleSteps, 3}} {
				if got, _ := account.Usage(item.kind); got != item.want {
					t.Errorf("%s=%d want=%d", item.kind, got, item.want)
				}
			}
		})
	}
}

func TestReviewCompactCacheReservesBeforeAllocation(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprint(existing), func(t *testing.T) {
			_, s := sessionFixture(t, Budgets{})
			key := s.catalog.orders[s.profile.ID][0]
			rule := s.catalog.rules[key]
			n := &factNode{header: testRetainedHeader(factHeader(1, 0, "test.root", "root")), end: testRetainedEnd(NodeEnd{})}
			if existing {
				if err := s.record(rule, n, Decision{Outcome: OutcomeNotApplicable}); err != nil {
					t.Fatal(err)
				}
			}
			account := s.account
			used, _ := account.Usage(BudgetBufferedBytes)
			sibling := account.Limits().MaxBufferedBytes - used - 31
			if err := account.Charge(BudgetBufferedBytes, sibling); err != nil {
				t.Fatal(err)
			}
			n.header.ID = 2
			err := s.record(rule, n, Decision{Outcome: OutcomePass})
			if !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("missing per-entry reservation: %v", err)
			}
			if _, ok := s.lookupDecision(decisionID{key, 2}); ok {
				t.Fatal("failed reservation retained compact entry")
			}
			if !existing && s.successful != nil {
				t.Fatal("failed first reservation allocated compact table")
			}
			s.fail(err)
			r, err := s.Finish(context.Background(), Completion{Syntax: SyntaxIncomplete})
			if !errors.Is(err, ErrResourceLimit) || r.Counts.Pass != 1 || r.AssessmentComplete {
				t.Fatalf("authoritative counts lost after cache quota: %+v %v", r, err)
			}
			if got, _ := account.Usage(BudgetBufferedBytes); got != sibling {
				t.Fatalf("cleanup released sibling gauge: got=%d want=%d", got, sibling)
			}
		})
	}
}

func TestReviewManySuccessfulRulesFitFiniteBuffer(t *testing.T) {
	const rulesCount, children = 40, 1000
	e := reviewDocumentEngine(t, func(rules *[]RuleDefinition) {
		base := cloneRule((*rules)[0])
		*rules = nil
		for i := 0; i < rulesCount; i++ {
			r := cloneRule(base)
			r.Key.ID = RuleID(fmt.Sprintf("pass-%02d", i))
			*rules = append(*rules, r)
		}
	})
	s, err := e.Begin(context.Background(), RunOptions{DocumentID: "doc", Profile: "test.source", Budgets: Budgets{MaxBufferedBytes: 8 << 20}})
	if err != nil {
		t.Fatal(err)
	}
	qaOpenRoot(t, s)
	for i := 0; i < children; i++ {
		id := NodeID(i + 2)
		if err := s.OpenNode(context.Background(), factHeader(id, 1, "test.child", "child")); err != nil {
			t.Fatal(err)
		}
		qaClose(t, s, factEnd(id))
	}
	qaClose(t, s, factEnd(1))
	retained := s.retained
	account := s.account
	r, err := s.Finish(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	t.Logf("rules=%d nodes=%d buffered-bytes=%d", rulesCount, children+1, retained)
	if err != nil || !r.Satisfied() || r.Counts.Pass != rulesCount*children || r.Counts.NotApplicable != rulesCount || len(r.Findings) != 0 || r.DiagnosticsTruncated {
		t.Fatalf("valid many-rule document exhausted finite budget: counts=%+v terminal=%v err=%v", r.Counts, r.Terminal, err)
	}
	if used, _ := account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("buffer leaked %d", used)
	}
}

func TestReviewDocumentAggregateInterruptionDuringScan(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		s := reviewDocumentSession(t, Budgets{})
		key := RuleKey{ID: "a-prerequisite", Revision: 1}
		dep := Dependency{Kind: DependencyRule, Rule: key, Relation: RelationDocument}
		s.inputComplete = true
		s.documentPrerequisites[key].finalized = true
		n := &factNode{header: testRetainedHeader(factHeader(1, 0, "test.child", "child")), end: testRetainedEnd(NodeEnd{})}
		for id := NodeID(1); id <= 4; id++ {
			n.header.ID = id
			s.sequence = append(s.sequence, id)
			if err := s.record(s.catalog.rules[key], n, Decision{Outcome: OutcomePass}); err != nil {
				t.Fatal(err)
			}
		}
		ctx := context.Context(context.Background())
		want := error(ErrResourceLimit)
		if cancel {
			ctx = &reviewCancelDuringBegin{Context: ctx, after: 3}
			want = context.Canceled
		} else {
			used, _ := s.account.Usage(BudgetRuleSteps)
			if err := s.account.Charge(BudgetRuleSteps, s.account.Limits().MaxRuleSteps-used-2); err != nil {
				t.Fatal(err)
			}
		}
		before, _ := s.account.Usage(BudgetRuleSteps)
		if _, _, err := s.dependency(ctx, n, dep, true); !errors.Is(err, want) {
			t.Fatalf("cancel=%v scan lost interruption: %v", cancel, err)
		}
		after, _ := s.account.Usage(BudgetRuleSteps)
		if after-before != 2 || s.documentPrerequisites[key].ready {
			t.Fatalf("partial scan incorrectly cached or charged: delta=%d cache=%+v", after-before, s.documentPrerequisites[key])
		}
		if cancel {
			d, pending, err := s.dependency(context.Background(), n, dep, true)
			if err != nil || pending || d.Outcome != OutcomePass || !s.documentPrerequisites[key].ready {
				t.Fatalf("partial result poisoned retry: %+v %v", d, err)
			}
		}
		s.cleanup()
	}
}
