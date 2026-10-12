package validation

import (
	"context"
	"errors"
	"testing"
)

func TestContextNavigationExactPassMembership(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	for _, tc := range []struct {
		text string
		pass uint64
		want bool
	}{{"2,10", 2, true}, {"2,10", 1, false}, {"1,1", 1, true}, {"999999999999999999999999999999999999", 1, false}} {
		got, err := navigationPassApplies(context.Background(), s, stateKnown(tc.text), tc.pass)
		if err != nil || got.State != EvidenceKnown || got.Value != tc.want {
			t.Fatalf("%+v: %+v %v", tc, got, err)
		}
	}
	got, err := navigationPassApplies(context.Background(), s, stateAbsent[string](), 1)
	if err != nil || got.State != EvidenceKnown || !got.Value {
		t.Fatalf("all passes: %+v %v", got, err)
	}
	bad := Evidence[string]{State: EvidenceInvalid, Reason: "bad", Cause: &DecisionRef{Rule: RuleKey{"bad", 1}, Subject: NodeRef{"d", 1}}}
	got, err = navigationPassApplies(context.Background(), s, bad, 1)
	if err != nil || got.State != EvidenceInvalid || got.Cause != bad.Cause {
		t.Fatalf("cause: %+v %v", got, err)
	}
}

func TestContextNavigationExplicitRepeatAndNestedFrames(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	events := []navigationEvent{
		{Source: NodeRef{"d", 1}, Kind: NavigationForward},
		{Source: NodeRef{"d", 2}, Kind: NavigationNote},
		{Source: NodeRef{"d", 3}, Kind: NavigationForward},
		{Source: NodeRef{"d", 4}, Kind: NavigationNote},
		{Source: NodeRef{"d", 5}, Kind: NavigationBackward, Target: stateKnown(2), Times: stateKnown(stateInt(t, 2)), AfterJump: stateKnown(true)},
		{Source: NodeRef{"d", 6}, Kind: NavigationBackward, Target: stateKnown(0), Times: stateKnown(stateInt(t, 3)), AfterJump: stateKnown(true)},
	}
	got, err := runNavigation(context.Background(), s, events)
	if err != nil || got.Complete.State != EvidenceKnown || !got.Complete.Value {
		t.Fatalf("%+v %v", got, err)
	}
	var a, b int
	for _, o := range got.Traversal {
		if o.Source.Node == 2 {
			a++
		}
		if o.Source.Node == 4 {
			b++
			if len(o.Frames) != 2 {
				t.Errorf("nested frames lost: %+v", o)
			}
		}
	}
	if a != 3 || b != 6 {
		t.Fatalf("notes repeated %d,%d", a, b)
	}
}

func TestContextNavigationUnknownCountKeepsPrefix(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	got, err := runNavigation(context.Background(), s, []navigationEvent{{Source: NodeRef{"d", 1}, Kind: NavigationForward}, {Source: NodeRef{"d", 2}, Kind: NavigationNote}, {Source: NodeRef{"d", 3}, Kind: NavigationBackward, Target: stateKnown(0), Times: stateAbsent[Integer]()}})
	if err != nil || got.Complete.State != EvidenceUnknown || len(got.Traversal) != 3 {
		t.Fatalf("prefix: %+v %v", got, err)
	}
}

func TestContextNavigationJumpPassDefaults(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	events := []navigationEvent{{Source: NodeRef{"d", 1}, Kind: NavigationNote}, {Source: NodeRef{"d", 2}, Kind: NavigationDaCapo, Target: stateKnown(0), Guard: stateKnown("1"), PassDomain: navigationMovementPass}, {Source: NodeRef{"d", 3}, Kind: NavigationNote}}
	got, err := runNavigation(context.Background(), s, events)
	if err != nil || got.Complete.State != EvidenceKnown || len(got.Traversal) != 5 {
		t.Fatalf("jump: %+v %v", got, err)
	}
	if got.Traversal[2].Epoch != 1 || got.Traversal[3].Played.Value {
		t.Fatalf("jump guard: %+v", got.Traversal)
	}
}

func TestContextNavigationHugeRepeatBudgetAndCancellation(t *testing.T) {
	s := rationalSession(t, Budgets{MaxRuleSteps: 200})
	defer s.cleanup()
	huge := stateInt(t, 1000000000)
	_, err := runNavigation(context.Background(), s, []navigationEvent{{Source: NodeRef{"d", 1}, Kind: NavigationForward}, {Source: NodeRef{"d", 2}, Kind: NavigationBackward, Target: stateKnown(0), Times: stateKnown(huge)}})
	if err == nil {
		t.Fatal("unbounded traversal")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = runNavigation(ctx, s, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestContextNavigationFalseGuardDoesNotApplyInvalidUpdate(t *testing.T) {
	bad := Evidence[Decimal]{State: EvidenceInvalid, Reason: "bad", Cause: &DecisionRef{Rule: RuleKey{"bad", 1}, Subject: NodeRef{"d", 2}}}
	got := navigationConditionalValue(bad, stateKnown(false))
	if got.State != EvidenceAbsent {
		t.Fatalf("inactive invalid value became update: %+v", got)
	}
	got = navigationConditionalValue(bad, stateKnown(true))
	if got.State != EvidenceInvalid || got.Cause != bad.Cause {
		t.Fatalf("active invalid cause lost: %+v", got)
	}
}

func TestContextNavigationBackwardApplicabilityIndependent(t *testing.T) {
	unknown := stateUnknown[bool]("ending-unavailable")
	if got := navigationBackwardApplicability(stateKnown("forward"), unknown); got.State != EvidenceKnown || got.Value {
		t.Fatalf("%+v", got)
	}
	if got := navigationBackwardApplicability(stateKnown("backward"), unknown); got.State != EvidenceUnknown {
		t.Fatalf("%+v", got)
	}
	if got := navigationBackwardApplicability(stateKnown("backward"), stateKnown(true)); got.State != EvidenceKnown || got.Value {
		t.Fatalf("%+v", got)
	}
	for _, kind := range []NavigationKind{NavigationDaCapo, NavigationDalSegno, NavigationToCoda} {
		guard, defaulted := navigationDefaultGuard(kind, stateAbsent[string]())
		want := "1"
		if kind == NavigationToCoda {
			want = "2"
		}
		if !defaulted || guard.State != EvidenceKnown || guard.Value != want {
			t.Fatalf("%s %+v", kind, guard)
		}
		guard, defaulted = navigationDefaultGuard(kind, stateKnown("3"))
		if defaulted || guard.Value != "3" {
			t.Fatalf("override %s %+v", kind, guard)
		}
	}
}

func TestContextNavigationNamedTargetAndSectionDomain(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	rows := []contextNavigationNode{
		{event: navigationEvent{Source: NodeRef{"d", 1}, Kind: NavigationNote}},
		{event: navigationEvent{Source: NodeRef{"d", 2}, Kind: NavigationSegno}, value: NavigationContextResult{Label: stateKnown("S")}},
		{event: navigationEvent{Source: NodeRef{"d", 3}, Kind: NavigationNote}},
		{event: navigationEvent{Source: NodeRef{"d", 4}, Kind: NavigationToCoda}, value: NavigationContextResult{Label: stateKnown("C")}},
		{event: navigationEvent{Source: NodeRef{"d", 5}, Kind: NavigationDalSegno}, value: NavigationContextResult{Label: stateKnown("S")}},
		{event: navigationEvent{Source: NodeRef{"d", 6}, Kind: NavigationCoda}, value: NavigationContextResult{Label: stateKnown("C")}},
	}
	if err := navigationBuildEdges(context.Background(), s, rows); err != nil {
		t.Fatal(err)
	}
	if rows[4].event.Target.State != EvidenceKnown || rows[4].event.Target.Value != 1 || rows[3].event.Target.Value != 5 {
		t.Fatalf("targets %+v", rows)
	}
	if rows[0].event.PassDomain != navigationUnknownPass || rows[2].event.PassDomain != navigationMovementPass || rows[5].event.PassDomain != navigationUnknownPass {
		t.Fatalf("section domains %+v", rows)
	}
	rows = append(rows, contextNavigationNode{event: navigationEvent{Source: NodeRef{"d", 7}, Kind: NavigationSegno}, value: NavigationContextResult{Label: stateKnown("S")}})
	if err := navigationBuildEdges(context.Background(), s, rows); err != nil {
		t.Fatal(err)
	}
	if rows[4].event.Target.State == EvidenceKnown {
		t.Fatal("duplicate target accepted")
	}
}

func TestContextNavigationOwnedProjection(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	original := NavigationContextResult{Kind: NavigationTraversal, Traversal: []NavigationOccurrence{{Source: NodeRef{"d", 1}, Played: Evidence[bool]{State: EvidenceUnknown, Reason: "gap", Cause: &DecisionRef{Rule: RuleKey{"cause", 1}, Subject: NodeRef{"d", 2}}}, Frames: []NavigationPassFrame{{Region: NodeRef{"d", 3}, Pass: stateInt(t, 1)}}}}}
	copy, err := cloneNavigationContext(context.Background(), s, &original)
	if err != nil {
		t.Fatal(err)
	}
	original.Traversal[0].Frames[0].Region.Node = 99
	original.Traversal[0].Played.Cause.Rule.ID = "changed"
	if copy.Traversal[0].Frames[0].Region.Node != 3 || copy.Traversal[0].Played.Cause.Rule.ID != "cause" {
		t.Fatalf("aliases %+v", copy)
	}
}

func TestContextNavigationBackwardReentryStateIsNotInferred(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	events := []navigationEvent{
		{Source: NodeRef{"d", 1}, Kind: NavigationForward},
		{Source: NodeRef{"d", 2}, Kind: NavigationNote, PassDomain: navigationSingleRepeatPass},
		{Source: NodeRef{"d", 3}, Kind: NavigationSound, Tempo: stateKnown(stateDecimal(t, 80, 1)), Guard: stateKnown("1"), PassDomain: navigationSingleRepeatPass},
		{Source: NodeRef{"d", 4}, Kind: NavigationBackward, Target: stateKnown(0), Times: stateKnown(stateInt(t, 2))},
	}
	got, err := runNavigation(context.Background(), s, events)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, o := range got.Traversal {
		if o.Source.Node != 2 {
			continue
		}
		seen++
		if seen == 1 && o.Tempo.State != EvidenceAbsent {
			t.Fatalf("initial %+v", o)
		}
		if seen == 2 && o.Tempo.State != EvidenceUnknown {
			t.Fatalf("reentry carried without proof %+v", o)
		}
	}
	if seen != 2 {
		t.Fatalf("seen=%d", seen)
	}
}
