package validation

import (
	"context"
	"testing"
	"unsafe"
)

// Node rule finding keys follow the root-close pass: by node in document
// order, then by the rule's place in the profile's order, so that a window's
// rules and the rules that waited for the root sort as one pass.
func TestNodeRuleKeyOrdersByNodeThenRule(t *testing.T) {
	const rules = 7
	var previous *findingKey
	for i := 0; i < 4; i++ {
		for k := 0; k < rules; k++ {
			key := nodeRuleKey(i, k, rules)
			if key.stage != stageNodeRules {
				t.Fatalf("node %d rule %d: stage %d", i, k, key.stage)
			}
			if previous != nil && !previous.less(key) {
				t.Fatalf("node %d rule %d: key %+v does not follow %+v", i, k, key, *previous)
			}
			previous = &key
		}
	}
	if !(findingKey{stage: stageSchema, trigger: 1 << 62}).less(nodeRuleKey(0, 0, rules)) || !nodeRuleKey(1<<20, rules-1, rules).less(findingKey{stage: stageSchemaFinal}) {
		t.Fatal("node rule keys are not between binding and schema finalization")
	}
}

// windowRuleSession is a session with one child-count rule on owner, which
// has one child.
func windowRuleSession(t *testing.T) (*Session, RuleKey, *factNode) {
	t.Helper()
	root := localNode(1, 0, "root")
	owner := localNode(2, 1, "owner")
	child := localNode(3, 2, "value")
	root.children = []*factNode{owner}
	owner.children = []*factNode{child}
	p, err := NewChildCountPlan([]DeclarationID{localDecl("value")}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	s, key, _ := localIsolatedSession(t, p, 1<<30, root, owner, owner, child)
	return s, key, owner
}

// A window's evaluation decides only the rules of its own nodes. Their
// outcomes wait aside until the root closes and are then recorded once;
// if the root never closes they are forgotten with their decisions, and
// what keeping them aside charged is released.
func TestWindowRuleEvaluationIsHeldAndForgotten(t *testing.T) {
	ctx := context.Background()
	s, key, owner := windowRuleSession(t)
	id := decisionID{key, owner.header.ID}
	s.windowEvaluating = &windowEvaluation{first: 3, last: 3}
	if err := s.evaluate(ctx, owner, key, false); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.lookupDecision(id); ok {
		t.Fatal("a window decided a rule of a node outside it")
	}
	// The window covers owner and its child, the sequence's second and third
	// nodes.
	s.windowKept.windows = [][2]int{{1, 3}}
	s.windowEvaluating = &windowEvaluation{first: 2, last: 3}
	if err := s.evaluate(ctx, owner, key, false); err != nil {
		t.Fatal(err)
	}
	s.windowEvaluating = nil
	if _, ok := s.lookupDecision(id); !ok || s.windowKeptCount() != 1 || s.accumulator.report.Counts.Pass != 0 {
		t.Fatalf("window decision %v, %d kept aside, %d passes recorded", ok, s.windowKeptCount(), s.accumulator.report.Counts.Pass)
	}
	kept := s.retained
	s.forgetWindowDecisions()
	if s.retained > kept {
		t.Fatalf("forgetting charged %d bytes", s.retained-kept)
	}
	if _, ok := s.lookupDecision(id); ok || s.windowKeptCount() != 0 || s.accumulator.report.Counts.Pass != 0 {
		t.Fatalf("forgotten window decision %v, %d kept aside, %d passes recorded", ok, s.windowKeptCount(), s.accumulator.report.Counts.Pass)
	}

	s, key, owner = windowRuleSession(t)
	s.windowEvaluating = &windowEvaluation{first: 2, last: 3}
	if err := s.evaluate(ctx, owner, key, false); err != nil {
		t.Fatal(err)
	}
	s.windowEvaluating = nil
	if err := s.confirmWindowDecisions(); err != nil {
		t.Fatal(err)
	}
	if err := s.evaluate(ctx, owner, key, false); err != nil {
		t.Fatal(err)
	}
	if s.accumulator.report.Counts.Pass != 1 || s.windowKeptCount() != 0 {
		t.Fatalf("confirmed: %d passes recorded, %d kept aside", s.accumulator.report.Counts.Pass, s.windowKeptCount())
	}
}

// A kept record's array slot is charged at least the record's size.
func TestWindowRecordWidth(t *testing.T) {
	if size := int64(unsafe.Sizeof(windowRecord{})); windowRecordBytes < size {
		t.Fatalf("a record slot is charged %d bytes, a record has %d", windowRecordBytes, size)
	}
}

// Under a failing rule, what windows keep aside is released exactly when
// it is forgotten: the record, its array and the window range.
func TestWindowRuleFailureKeptAsideIsReleased(t *testing.T) {
	ctx := context.Background()
	root := localNode(1, 0, "root")
	owner := localNode(2, 1, "owner")
	child := localNode(3, 2, "value")
	root.children = []*factNode{owner}
	owner.children = []*factNode{child}
	p, err := NewChildCountPlan([]DeclarationID{localDecl("value")}, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	s, key, _ := localIsolatedSession(t, p, 1<<30, root, owner, owner, child)
	s.windowKept.windows = [][2]int{{1, 3}}
	s.windowEvaluating = &windowEvaluation{first: 2, last: 3}
	if err := s.evaluate(ctx, owner, key, false); err != nil {
		t.Fatal(err)
	}
	s.windowEvaluating = nil
	if len(s.windowKept.records) != 1 {
		t.Fatalf("%d records kept aside", len(s.windowKept.records))
	}
	want := s.windowKept.records[0].charged + int64(cap(s.windowKept.records))*windowRecordBytes + int64(cap(s.windowKept.windows))*windowRangeBytes
	before := s.retained
	s.forgetWindowDecisions()
	if released := before - s.retained; released != want {
		t.Fatalf("forgetting released %d bytes, want %d", released, want)
	}
}
