package validation

import (
	"context"
	"errors"
	"testing"
)

func localIsolatedSession(t *testing.T, p Plan, limit int64, root, subject *factNode, nodes ...*factNode) (*Session, RuleKey, *Account) {
	t.Helper()
	rules, profiles, plans := catalogFixture(t)
	declaration := subject.header.Declaration.Value
	rules[0].Selector = Selector{Declaration: &declaration}
	plans[0].Plan = p
	catalog, e := NewCatalog(rules, profiles, plans)
	if e != nil {
		t.Fatal(e)
	}
	account, e := NewBudgetAccount(Budgets{MaxBufferedBytes: limit})
	if e != nil {
		t.Fatal(e)
	}
	s := &Session{catalog: catalog, account: account, documentID: "doc", profile: profiles[0], root: root, inputComplete: true, nodes: map[NodeID]*factNode{}, decisions: map[decisionID]Decision{}}
	for _, n := range append([]*factNode{root}, nodes...) {
		s.nodes[n.header.ID] = n
		s.sequence = append(s.sequence, n.header.ID)
	}
	s.accumulator = newReportAccumulator(profiles[0], account)
	return s, rules[0].Key, account
}
func TestLocalProvedFailureSurvivesOptionalStorageStop(t *testing.T) {
	for _, tc := range []struct {
		name                string
		marker, conditional bool
		limit               int64
		wantFail            uint64
	}{
		{"marker related reservation", true, true, 384, 1},
		{"required marker cache reservation", false, false, 383, 1},
		{"absent conditional marker cache is not failure", false, true, 383, 0},
		{"present required marker cache is not failure", true, false, 383, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := localNode(1, 0, "root")
			owner := localNode(2, 1, "owner")
			value := localNode(3, 2, "value")
			marker := localNode(4, 1, "marker")
			root.children = []*factNode{owner}
			owner.children = []*factNode{value}
			value.end.Scalar.Value = Evidence[Atomic]{State: EvidenceKnown, Value: localInteger(t, 1)}
			nodes := []*factNode{owner, value}
			if tc.marker {
				root.children = append(root.children, marker)
				nodes = append(nodes, marker)
			}
			var p Plan
			var e error
			if tc.conditional {
				p, e = NewDocumentMarkerValuesPlan([]DeclarationID{localDecl("marker")}, []ChildValueRequirement{{Child: localDecl("value"), Expected: localInteger(t, 0)}})
			} else {
				p, e = NewDocumentChildPresencePlan([]DeclarationID{localDecl("marker")})
			}
			if e != nil {
				t.Fatal(e)
			}
			s, key, account := localIsolatedSession(t, p, tc.limit, root, owner, nodes...)
			e = s.evaluate(context.Background(), owner, key, true)
			if !errors.Is(e, ErrResourceLimit) {
				t.Fatalf("expected storage interruption: %v", e)
			}
			if got := s.accumulator.report.Counts.Fail; got != tc.wantFail {
				t.Errorf("proved outcome lost/fabricated: fail=%d want=%d", got, tc.wantFail)
			}
			report, finishErr := s.Finish(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed, Cause: e})
			if !errors.Is(finishErr, e) || report.AssessmentComplete || report.Terminal == nil || report.Terminal.Category != CategoryResource || report.Counts.Fail != tc.wantFail {
				t.Fatalf("resource/failure axes %+v %v", report, finishErr)
			}
			if used, _ := account.Usage(BudgetBufferedBytes); used != 0 {
				t.Fatalf("leaked %d bytes", used)
			}
		})
	}
}
func TestLocalDistinctFailureStorageBoundary(t *testing.T) {
	p, e := NewDistinctChildAttributePlan([]DeclarationID{localDecl("beam")}, localAttr("number"))
	if e != nil {
		t.Fatal(e)
	}
	for _, limit := range []int64{1, 255, 256, 257} {
		t.Run(string(rune(limit)), func(t *testing.T) {
			root := localNode(1, 0, "note")
			a, b := localNode(2, 1, "beam"), localNode(3, 1, "beam")
			localSetAttr(a, "number", localInteger(t, 1))
			localSetAttr(b, "number", localInteger(t, 1))
			root.children = []*factNode{a, b}
			s, key, account := localIsolatedSession(t, p, limit, root, root, a, b)
			e = s.evaluate(context.Background(), root, key, true)
			if !errors.Is(e, ErrResourceLimit) {
				t.Fatalf("expected related/cache bound: %v", e)
			}
			if s.accumulator.report.Counts.Fail != 1 {
				t.Fatalf("duplicate proof erased: %+v", s.accumulator.report.Counts)
			}
			s.cleanup()
			if used, _ := account.Usage(BudgetBufferedBytes); used != 0 {
				t.Fatalf("leaked %d", used)
			}
		})
	}
}

func TestLocalPublicSessionFailureSurvivesPostProofStorageStop(t *testing.T) {
	for _, related := range []bool{false, true} {
		rules, profiles, plans := catalogFixture(t)
		ownerDecl := localDecl("owner")
		rules[0].Selector = Selector{Declaration: &ownerDecl}
		var plan Plan
		var e error
		if related {
			plan, e = NewDocumentMarkerValuesPlan([]DeclarationID{localDecl("marker")}, []ChildValueRequirement{{Child: localDecl("value"), Expected: localInteger(t, 0)}})
		} else {
			plan, e = NewDocumentChildPresencePlan([]DeclarationID{localDecl("marker")})
		}
		if e != nil {
			t.Fatal(e)
		}
		plans[0].Plan = plan
		catalog, e := NewCatalog(rules, profiles, plans)
		if e != nil {
			t.Fatal(e)
		}
		engine, e := NewEngine(catalog)
		if e != nil {
			t.Fatal(e)
		}
		const limit int64 = 65536
		account, e := NewBudgetAccount(Budgets{MaxBufferedBytes: limit})
		if e != nil {
			t.Fatal(e)
		}
		ctx := context.Background()
		s, e := engine.Begin(ctx, RunOptions{DocumentID: "doc", Profile: profiles[0].ID, Account: account})
		if e != nil {
			t.Fatal(e)
		}
		open := func(id, parent NodeID, name string) {
			h := factHeader(id, parent, name, name)
			h.Declaration.Value = localDecl(name)
			if e := s.Open(ctx, h); e != nil {
				t.Fatal(e)
			}
		}
		close := func(id NodeID, nonzero bool) {
			end := factEnd(id)
			if nonzero {
				end.Scalar.Value = Evidence[Atomic]{State: EvidenceKnown, Value: localInteger(t, 1)}
			}
			if e := s.Close(ctx, end); e != nil {
				t.Fatal(e)
			}
		}
		open(1, 0, "root")
		if related {
			open(2, 1, "marker")
			close(2, false)
		}
		open(3, 1, "owner")
		if related {
			open(4, 3, "value")
			close(4, true)
		}
		close(3, false)
		close(1, false)
		used, _ := account.Usage(BudgetBufferedBytes)
		allowance := int64(383)
		if related {
			allowance = 384
		}
		borrowed := limit - used - allowance
		if e := account.Charge(BudgetBufferedBytes, borrowed); e != nil {
			t.Fatal(e)
		}
		report, e := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
		if !errors.Is(e, ErrResourceLimit) || report.Counts.Fail != 1 || report.Conformance != ConformanceViolated || report.AssessmentComplete {
			t.Fatalf("related=%v lost failure/resource axes %+v %v", related, report, e)
		}
		if e := account.Release(BudgetBufferedBytes, borrowed); e != nil {
			t.Fatal(e)
		}
		if used, _ := account.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("leaked %d", used)
		}
	}
}
func TestPlanInterruptionRequiresExactProducerProof(t *testing.T) {
	p, e := NewScalarOneOfPlan([]Atomic{localString("yes")})
	if e != nil {
		t.Fatal(e)
	}
	for _, variant := range []string{"plain-error", "other-node", "other-plan", "nested-error", "matching-proof"} {
		t.Run(variant, func(t *testing.T) {
			root := localNode(1, 0, "root")
			s, key, _ := localIsolatedSession(t, p, 1, root, root)
			r := s.catalog.rules[key]
			bound := s.catalog.plans[r.Predicate]
			cause := &ResourceError{Kind: BudgetBufferedBytes, Limit: 1, Requested: 128}
			proof := &provedPlanInterruption{node: 1, local: bound.local, decision: Decision{Outcome: OutcomeFail, Reason: "known-mismatch"}, cause: cause}
			var interrupted error = proof
			switch variant {
			case "plain-error":
				interrupted = cause
			case "other-node":
				proof.node = 2
			case "other-plan":
				proof.local = p.local
			case "nested-error":
				interrupted = errors.Join(proof)
			}
			err := s.recordProvedPlanInterruption(r, root, bound, interrupted)
			if !errors.Is(err, cause) {
				t.Fatalf("lost resource cause %v", err)
			}
			want := uint64(0)
			if variant == "matching-proof" {
				want = 1
			}
			if s.accumulator.report.Counts.Fail != want {
				t.Fatalf("misattributed proof %+v", s.accumulator.report.Counts)
			}
			s.cleanup()
		})
	}
	a, _ := NewBudgetAccount(Budgets{})
	s := &Session{account: a}
	d, e := s.localCompare(context.Background(), Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger}}, localInteger(t, 0))
	if e == nil || d.Outcome == OutcomeFail {
		t.Fatalf("comparison error fabricated proof %+v %v", d, e)
	}
}

func TestTypedReferenceProvedKindFailureSurvivesRelatedStorageStop(t *testing.T) {
	schema := localSchema(t)
	for _, target := range []Target{TargetSource, TargetModel} {
		const limit int64 = 1 << 20
		account, e := NewBudgetAccount(Budgets{MaxBufferedBytes: limit})
		if e != nil {
			t.Fatal(e)
		}
		engine := localEngine(t, schema, target, referenceBindings(t, schema, false)...)
		s := localAdmit(t, engine, target, lt("root", la("other", "I1"), la("part", "I1")), account)
		s.inputComplete = true
		ctx := context.Background()
		if e = s.finalizeSourceSchema(ctx); e != nil {
			t.Fatal(e)
		}
		var subject *factNode
		for _, n := range s.nodes {
			if n.header.Name.Local == "part" {
				subject = n
			}
		}
		key := RuleKey{ID: "part.root", Revision: 1}
		rule := s.catalog.rules[key]
		p := s.catalog.plans[rule.Predicate]
		before, _ := account.Usage(BudgetBufferedBytes)
		_, d, e := s.referenceResolution(ctx, subject, p.reference.Attribute)
		if e != nil || d.Outcome != OutcomePass {
			t.Fatalf("reference fixture %+v %v", d, e)
		}
		after, _ := account.Usage(BudgetBufferedBytes)
		resolutionBytes := after - before
		borrowed := limit - after - resolutionBytes - 127
		if e = account.Charge(BudgetBufferedBytes, borrowed); e != nil {
			t.Fatal(e)
		}
		e = s.evaluate(ctx, subject, key, true)
		if !errors.Is(e, ErrResourceLimit) || s.accumulator.report.Counts.Fail != 1 {
			t.Fatalf("%s kind proof lost: %+v %v", target, s.accumulator.report.Counts, e)
		}
		syntax := SyntaxWellFormed
		if target == TargetModel {
			syntax = SyntaxNotAssessed
		}
		report, finishErr := s.Finish(ctx, Completion{InputComplete: true, Syntax: syntax, Cause: e})
		if !errors.Is(finishErr, e) || report.Counts.Fail != 1 || report.AssessmentComplete {
			t.Fatalf("typed resource axes %+v %v", report, finishErr)
		}
		if e = account.Release(BudgetBufferedBytes, borrowed); e != nil {
			t.Fatal(e)
		}
		if used, _ := account.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("leaked %d", used)
		}
	}
}

func TestTypedReferenceNestedOwnerProofIsNotCountedForConsumer(t *testing.T) {
	schema := localSchema(t)
	for _, target := range []Target{TargetSource, TargetModel} {
		const limit int64 = 1 << 20
		account, _ := NewBudgetAccount(Budgets{MaxBufferedBytes: limit})
		s := localAdmit(t, localEngine(t, schema, target, referenceBindings(t, schema, false)...), target, lt("root", referenceHeader(), la("part", "I1", la("assignment", "I1"))), account)
		s.inputComplete = true
		ctx := context.Background()
		if e := s.finalizeSourceSchema(ctx); e != nil {
			t.Fatal(e)
		}
		var part, consumer *factNode
		for _, n := range s.nodes {
			switch n.header.Name.Local {
			case "part":
				part = n
			case "assignment":
				consumer = n
			}
		}
		providerKey := RuleKey{ID: "part.root", Revision: 1}
		consumerKey := RuleKey{ID: "ref.part.assignment", Revision: 1}
		before, _ := account.Usage(BudgetBufferedBytes)
		for _, pair := range []struct {
			node *factNode
			key  RuleKey
		}{{part, providerKey}, {consumer, consumerKey}} {
			plan := s.catalog.plans[s.catalog.rules[pair.key].Predicate]
			_, d, e := s.referenceResolution(ctx, pair.node, plan.reference.Attribute)
			if e != nil || d.Outcome != OutcomePass {
				t.Fatalf("fixture reference %+v %v", d, e)
			}
		}
		after, _ := account.Usage(BudgetBufferedBytes)
		borrowed := limit - after - (after - before) - 127
		if e := account.Charge(BudgetBufferedBytes, borrowed); e != nil {
			t.Fatal(e)
		}
		err := s.evaluate(ctx, consumer, consumerKey, true)
		if !errors.Is(err, ErrResourceLimit) || s.accumulator.report.Counts.Fail != 1 {
			t.Fatalf("nested proof lost/double-counted: %+v %v", s.accumulator.report.Counts, err)
		}
		syntax := SyntaxWellFormed
		if target == TargetModel {
			syntax = SyntaxNotAssessed
		}
		report, e := s.Finish(ctx, Completion{InputComplete: true, Syntax: syntax, Cause: err})
		if !errors.Is(e, err) || report.Counts.Fail != 1 {
			t.Fatalf("nested terminal result %+v %v", report, e)
		}
		if f, ok := localFinding(report, string(providerKey.ID)); !ok || f.Outcome != OutcomeFail {
			t.Fatalf("provider proof missing %+v", report)
		}
		if f, ok := localFinding(report, string(consumerKey.ID)); ok && f.Outcome == OutcomeFail {
			t.Fatalf("provider proof attributed to consumer %+v", f)
		}
		if e := account.Release(BudgetBufferedBytes, borrowed); e != nil {
			t.Fatal(e)
		}
		if used, _ := account.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("leaked %d", used)
		}
	}
}
