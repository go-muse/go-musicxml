package validation

import (
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"strings"
	"testing"
)

type deferredItem struct {
	at       findingKey
	truncate bool // truncation from outside the accumulator at this position
	rule     RuleDefinition
	decision Decision
	location Location
}

func deferredItems(random *rand.Rand, rules []RuleDefinition) []deferredItem {
	var items []deferredItem
	for trigger, n := int64(0), 1+random.Intn(25); trigger < int64(n); trigger++ {
		at := findingKey{stage: findingStage(random.Intn(5)), phase: random.Intn(3), trigger: trigger}
		if random.Intn(20) == 0 {
			items = append(items, deferredItem{at: at, truncate: true})
			continue
		}
		// Several findings at one position keep their arrival order.
		for k := 0; k < 1+random.Intn(3); k++ {
			rule := rules[random.Intn(len(rules))]
			expected := strings.Repeat("e", random.Intn(60))
			if random.Intn(12) == 0 {
				expected = strings.Repeat("x", 4000) // a bounded preview, still a large finding
			}
			location := Location{Target: TargetSource, DocumentID: "doc", Path: strings.Repeat("/p", 1+random.Intn(40))}
			if random.Intn(15) == 0 {
				location.Path = strings.Repeat("/q", 1500) // larger than the tight byte quotas
			}
			items = append(items, deferredItem{at: at, rule: rule, decision: Decision{Rule: rule.Key, Subject: NodeRef{"doc", NodeID(trigger + 1)}, Outcome: OutcomeFail, Expected: expected}, location: location})
		}
	}
	return items
}

func deferredFeed(t *testing.T, a *reportAccumulator, item deferredItem) {
	t.Helper()
	a.at = item.at
	if item.truncate {
		a.truncateDiagnostics()
		return
	}
	if err := a.record(item.rule, item.decision, item.location); err != nil {
		t.Fatal(err)
	}
}

// The deferred accumulator stores exactly what the buffered one stores, in
// the same order, whatever order the positions are evaluated in: for random
// findings under tight and loose quotas, with quota already used by an earlier
// session, several findings per position, findings larger than the byte quota,
// rules whose metadata alone exceeds what is left and truncation from outside
// the accumulator, evaluating the positions shuffled gives the same findings,
// rules, truncation flag and diagnostic usage as evaluating them in key order
// in a buffered accumulator, and the candidates' buffer charge returns to
// zero. Evaluated in key order, the deferred accumulator also reports
// truncation at the same moment as the buffered one.
func TestDeferredReportStoresTheBufferedPrefix(t *testing.T) {
	random := rand.New(rand.NewSource(7))
	rules := make([]RuleDefinition, 4)
	for i := range rules {
		rules[i] = RuleDefinition{Key: RuleKey{ID: RuleID(fmt.Sprintf("rule-%d", i)), Revision: 1}, Class: ClassMandatory, Role: RolePredicate, Category: CategoryNormative, Text: RuleText{Title: strings.Repeat("t", 40*i)}}
	}
	cases := 0
	for _, limits := range []Budgets{
		{},
		{MaxDiagnostics: 3},
		{MaxDiagnosticBytes: 2000},
		{MaxDiagnosticBytes: 1200, MaxDiagnostics: 5},
		{MaxDiagnosticBytes: 700},
	} {
		for trial := 0; trial < 60; trial++ {
			items := deferredItems(random, rules)
			slices.SortStableFunc(items, func(x, y deferredItem) int {
				if x.at.less(y.at) {
					return -1
				}
				if y.at.less(x.at) {
					return 1
				}
				return 0
			})
			used := int64(random.Intn(3)) * 300 // an earlier session's diagnostics
			newAccount := func() *Account {
				a, err := NewBudgetAccount(limits)
				if err != nil {
					t.Fatal(err)
				}
				if used > 0 {
					_ = a.chargeDiagnostic(used)
				}
				return a
			}
			buffered := newAccount()
			want := newReportAccumulator(Profile{}, buffered)
			inOrderAccount := newAccount()
			inOrder := newReportAccumulator(Profile{}, inOrderAccount)
			inOrder.deferred = true
			for _, it := range items {
				deferredFeed(t, want, it)
				deferredFeed(t, inOrder, it)
				if want.diagnosticsTruncated() != inOrder.diagnosticsTruncated() {
					t.Fatalf("limits %+v trial %d: truncation seen %v in order, buffered %v", limits, trial, inOrder.diagnosticsTruncated(), want.diagnosticsTruncated())
				}
			}
			// Shuffle whole positions: findings at one position are produced
			// together, in order.
			var positions [][]deferredItem
			for _, it := range items {
				if n := len(positions); n > 0 && positions[n-1][0].at == it.at {
					positions[n-1] = append(positions[n-1], it)
				} else {
					positions = append(positions, []deferredItem{it})
				}
			}
			shuffledAccount := newAccount()
			shuffled := newReportAccumulator(Profile{}, shuffledAccount)
			shuffled.deferred = true
			for _, i := range random.Perm(len(positions)) {
				for _, it := range positions[i] {
					deferredFeed(t, shuffled, it)
				}
			}
			wantReport := want.finish(true, SyntaxWellFormed)
			for name, got := range map[string]*reportAccumulator{"in order": inOrder, "shuffled": shuffled} {
				account := map[string]*Account{"in order": inOrderAccount, "shuffled": shuffledAccount}[name]
				gotReport := got.finish(true, SyntaxWellFormed)
				if !reflect.DeepEqual(wantReport.Findings, gotReport.Findings) || !reflect.DeepEqual(wantReport.Rules, gotReport.Rules) || wantReport.DiagnosticsTruncated != gotReport.DiagnosticsTruncated || !reflect.DeepEqual(wantReport.Counts, gotReport.Counts) {
					t.Fatalf("limits %+v trial %d %s: deferred report differs: %d/%d findings, truncated %v/%v", limits, trial, name, len(gotReport.Findings), len(wantReport.Findings), gotReport.DiagnosticsTruncated, wantReport.DiagnosticsTruncated)
				}
				for _, kind := range []BudgetKind{BudgetDiagnostics, BudgetDiagnosticBytes} {
					w, _ := buffered.Usage(kind)
					g, _ := account.Usage(kind)
					if w != g {
						t.Fatalf("limits %+v trial %d %s: %s usage %d, want %d", limits, trial, name, kind, g, w)
					}
				}
				if left, _ := account.Usage(BudgetBufferedBytes); left != 0 {
					t.Fatalf("limits %+v trial %d %s: deferred candidates left %d buffered bytes", limits, trial, name, left)
				}
				if !reflect.DeepEqual(got.keys, want.keys) {
					t.Fatalf("limits %+v trial %d %s: stored keys %v, want %v", limits, trial, name, got.keys, want.keys)
				}
			}
			cases++
		}
	}
	if cases != 300 {
		t.Fatalf("ran %d cases", cases)
	}
}

// Once truncation is known, later findings are dropped before they are copied
// or charged: past the cutoff under the default quotas, recording a finding
// allocates nothing and leaves the pending set and its charge unchanged.
func TestDeferredReportDropsFindingsPastTheCutoff(t *testing.T) {
	account, err := NewBudgetAccount(Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	a := newReportAccumulator(Profile{}, account)
	a.deferred = true
	rule := RuleDefinition{Key: RuleKey{ID: "rule", Revision: 1}, Class: ClassMandatory, Role: RolePredicate, Category: CategoryNormative}
	record := func(i int64) {
		a.at = findingKey{stage: stagePhases, trigger: i}
		if err := a.record(rule, Decision{Rule: rule.Key, Subject: NodeRef{"doc", NodeID(i + 1)}, Outcome: OutcomeFail}, Location{Target: TargetSource, DocumentID: "doc"}); err != nil {
			t.Fatal(err)
		}
	}
	limit := DefaultBudgets().MaxDiagnostics
	for i := int64(0); i <= limit; i++ {
		record(i)
	}
	if a.cutoff == nil || int64(len(a.pending)) != limit {
		t.Fatalf("after %d findings: cutoff %v, %d pending", limit+1, a.cutoff, len(a.pending))
	}
	charged, _ := account.Usage(BudgetBufferedBytes)
	next := limit + 1
	if allocs := testing.AllocsPerRun(1000, func() { record(next); next++ }); allocs != 0 {
		t.Fatalf("a finding past the cutoff allocated %.0f times", allocs)
	}
	if after, _ := account.Usage(BudgetBufferedBytes); after != charged || int64(len(a.pending)) != limit {
		t.Fatalf("past the cutoff: charge %d -> %d, %d pending", charged, after, len(a.pending))
	}
	if r := a.finish(true, SyntaxWellFormed); int64(len(r.Findings)) != limit || !r.DiagnosticsTruncated {
		t.Fatalf("findings %d truncated %v", len(r.Findings), r.DiagnosticsTruncated)
	}
	if left, _ := account.Usage(BudgetBufferedBytes); left != 0 {
		t.Fatalf("left %d buffered bytes", left)
	}
}

// Findings arriving out of key order are simulated in batches: a hundred
// thousand findings in reverse key order never keep more than twice the
// diagnostic count pending, finish without a quadratic rescan and store the
// buffered prefix.
func TestDeferredReportBatchesOutOfOrderFindings(t *testing.T) {
	account, err := NewBudgetAccount(Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	a := newReportAccumulator(Profile{}, account)
	a.deferred = true
	rule := RuleDefinition{Key: RuleKey{ID: "rule", Revision: 1}, Class: ClassMandatory, Role: RolePredicate, Category: CategoryNormative}
	const n = 100_000
	simulations := 0
	for i := int64(n - 1); i >= 0; i-- {
		a.at = findingKey{stage: stagePhases, trigger: i}
		before := a.simulated
		if err := a.record(rule, Decision{Rule: rule.Key, Subject: NodeRef{"doc", NodeID(i + 1)}, Outcome: OutcomeFail}, Location{Target: TargetSource, DocumentID: "doc"}); err != nil {
			t.Fatal(err)
		}
		if a.simulated > before+1 {
			simulations++
		}
		if int64(len(a.pending)) > 2*DefaultBudgets().MaxDiagnostics+1 {
			t.Fatalf("%d pending after %d findings", len(a.pending), n-i)
		}
	}
	if simulations > n/int(DefaultBudgets().MaxDiagnostics) {
		t.Fatalf("%d full simulations for %d findings", simulations, n)
	}
	r := a.finish(true, SyntaxWellFormed)
	if int64(len(r.Findings)) != DefaultBudgets().MaxDiagnostics || !r.DiagnosticsTruncated || r.Findings[0].Decision.Subject.Node != 1 {
		t.Fatalf("findings %d truncated %v first %v", len(r.Findings), r.DiagnosticsTruncated, r.Findings[0].Decision.Subject)
	}
}
