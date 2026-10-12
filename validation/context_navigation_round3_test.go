package validation

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
)

func navigationRound3Rows(t *testing.T, kinds ...NavigationKind) []contextNavigationNode {
	t.Helper()
	rows := make([]contextNavigationNode, len(kinds))
	for i, kind := range kinds {
		rows[i] = contextNavigationNode{event: navigationEvent{Source: NodeRef{"d", NodeID(i + 1)}, Kind: kind}, position: stateKnown(statePosition(t, 0, 0, 1))}
	}
	return rows
}

func TestRound3NavigationOrderingClassification(t *testing.T) {
	for _, tc := range []struct {
		name      string
		kinds     []NavigationKind
		ambiguous bool
	}{
		{"sounds", []NavigationKind{NavigationSound, NavigationSound}, false},
		{"listening", []NavigationKind{NavigationListening, NavigationListening}, false},
		{"sound-listening", []NavigationKind{NavigationSound, NavigationListening}, false},
		{"sound-repeat", []NavigationKind{NavigationSound, NavigationBackward}, false},
		{"repeat-sound", []NavigationKind{NavigationBackward, NavigationSound}, false},
		{"ds-repeat", []NavigationKind{NavigationDalSegno, NavigationBackward}, true},
		{"tocoda-repeat", []NavigationKind{NavigationToCoda, NavigationBackward}, true},
		{"fine-repeat", []NavigationKind{NavigationFine, NavigationBackward}, true},
		{"ds-sound-repeat", []NavigationKind{NavigationDalSegno, NavigationSound, NavigationBackward}, true},
		{"ds-listening-repeat", []NavigationKind{NavigationDalSegno, NavigationListening, NavigationBackward}, true},
		{"duplicate-marker", []NavigationKind{NavigationSegno, NavigationSegno}, true},
		{"different-priority-controls", []NavigationKind{NavigationSegno, NavigationForward, NavigationBackward}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			rows := navigationRound3Rows(t, tc.kinds...)
			before := append([]contextNavigationNode(nil), rows...)
			got, err := navigationOrder(context.Background(), s, &timelineService{}, rows)
			if err != nil {
				t.Fatal(err)
			}
			want := navigationKnownBool(true)
			if tc.ambiguous {
				want = stateUnknown[bool]("navigation-simultaneous-control-unresolved")
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("order = %+v, want %+v", got, want)
			}
			if !reflect.DeepEqual(rows, before) {
				t.Error("stable equal-position row order changed")
			}
		})
	}
}

func TestRound3NavigationPassiveLinearPrefix(t *testing.T) {
	for _, unconfined := range []bool{false, true} {
		for _, kinds := range [][]NavigationKind{
			{NavigationSound, NavigationSound, NavigationNote},
			{NavigationSound, NavigationListening, NavigationNote},
			{NavigationSound, NavigationListening, NavigationDalSegno, NavigationSound, NavigationBackward},
		} {
			s := rationalSession(t, Budgets{})
			rows := navigationRound3Rows(t, kinds...)
			// Put the eventual note or control at the next musical position, so the
			// prefix is tested independently of note/control priority sorting.
			for i := 2; i < len(rows); i++ {
				rows[i].position = stateKnown(statePosition(t, 0, 1, 1))
			}
			if unconfined {
				for i := range rows {
					rows[i].position.Value.UnprovedFrame = true
					rows[i].position.Value.Offset = mustRational(t, 1, 1)
					if i >= 2 {
						rows[i].position.Value.Offset = mustRational(t, 2, 1)
					}
				}
			}
			got, err := navigationLinearPrefix(context.Background(), s, &timelineService{}, rows)
			s.cleanup()
			want := len(rows)
			if len(rows) == 5 {
				want = 2
			}
			if err != nil || got != want {
				t.Errorf("kinds %v: prefix = %d, %v; want %d", kinds, got, err, want)
			}
		}
	}
}

func TestRound3NavigationUnavailableOrderingPreservesCause(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		s := rationalSession(t, Budgets{})
		rows := navigationRound3Rows(t, NavigationSound, NavigationListening)
		cause := &DecisionRef{Rule: RuleKey{"position-source", 1}, Subject: NodeRef{"d", 9}}
		rows[1].position = Evidence[musicalPosition]{State: state, Reason: "position-unavailable", Cause: cause}
		got, err := navigationOrder(context.Background(), s, &timelineService{}, rows)
		if err != nil || !reflect.DeepEqual(got, stateUnavailable[musicalPosition, bool](rows[1].position)) {
			t.Errorf("order = %+v, %v", got, err)
		}
		count, err := navigationLinearPrefix(context.Background(), s, &timelineService{}, rows)
		if err != nil || count != 0 {
			t.Errorf("unplaced row got prefix %d, %v", count, err)
		}
		s.cleanup()
	}
}

func TestRound3NavigationJumpPrerequisitePrecedence(t *testing.T) {
	cause := &DecisionRef{Rule: RuleKey{"target-source", 1}, Subject: NodeRef{"d", 8}}
	guardCause := &DecisionRef{Rule: RuleKey{"guard-source", 1}, Subject: NodeRef{"d", 9}}
	target := stateUnknown[int]("navigation-target-unresolved")
	invalidTarget := Evidence[int]{State: EvidenceInvalid, Reason: "target-invalid", Cause: cause}
	unsupportedTarget := Evidence[int]{State: EvidenceUnsupported, Reason: "target-unsupported", Cause: cause}
	invalidGuard := Evidence[string]{State: EvidenceInvalid, Reason: "guard-invalid", Cause: guardCause}
	unsupportedGuard := Evidence[string]{State: EvidenceUnsupported, Reason: "guard-unsupported", Cause: guardCause}
	for _, tc := range []struct {
		name   string
		target Evidence[int]
		guard  Evidence[string]
		domain navigationPassDomain
		want   Evidence[bool]
	}{
		{"missing-target-unbound-pass", target, stateKnown("1"), navigationUnknownPass, stateUnavailable[int, bool](target)},
		{"missing-target-unknown-guard", target, stateUnknown[string]("guard-unavailable"), navigationUnknownPass, stateUnavailable[int, bool](target)},
		{"invalid-guard", target, invalidGuard, navigationUnknownPass, stateUnavailable[string, bool](invalidGuard)},
		{"unsupported-guard", target, unsupportedGuard, navigationUnknownPass, stateUnavailable[string, bool](unsupportedGuard)},
		{"unsupported-target", unsupportedTarget, stateKnown("1"), navigationUnknownPass, stateUnavailable[int, bool](unsupportedTarget)},
		{"invalid-guard-over-unsupported-target", unsupportedTarget, invalidGuard, navigationUnknownPass, stateUnavailable[string, bool](invalidGuard)},
		{"invalid-target", invalidTarget, stateKnown("1"), navigationUnknownPass, stateUnavailable[int, bool](invalidTarget)},
		{"two-invalid-prerequisites", invalidTarget, invalidGuard, navigationUnknownPass, stateUnavailable[int, bool](invalidTarget)},
		{"false-guard-skips-unresolved-target", target, stateKnown("2"), navigationMovementPass, navigationKnownBool(true)},
		{"false-guard-skips-invalid-target", invalidTarget, stateKnown("2"), navigationMovementPass, navigationKnownBool(true)},
		{"known-target-keeps-unknown-pass", stateKnown(0), stateKnown("1"), navigationUnknownPass, stateUnknown[bool]("navigation-pass-unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			events := []navigationEvent{{Source: NodeRef{"d", 1}, Kind: NavigationNote}, {Source: NodeRef{"d", 2}, Kind: NavigationDalSegno, Target: tc.target, Guard: tc.guard, PassDomain: tc.domain}}
			got, err := runNavigation(context.Background(), s, events)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Complete, tc.want) {
				t.Errorf("complete = %+v, want %+v", got.Complete, tc.want)
			}
			if len(got.Traversal) != 2 || got.Traversal[1].Epoch != 0 {
				t.Errorf("unproved jump executed: %+v", got.Traversal)
			}
		})
	}
}

func TestRound3NavigationUnresolvedTargetKeepsUnknownDomain(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		s := rationalSession(t, Budgets{})
		cause := &DecisionRef{Rule: RuleKey{"label-source", 1}, Subject: NodeRef{"d", 9}}
		label := Evidence[string]{State: state, Reason: "label-unavailable", Cause: cause}
		rows := navigationRound3Rows(t, NavigationNote, NavigationDalSegno)
		rows[1].value.Label = label
		if err := navigationBuildEdges(context.Background(), s, rows); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(rows[1].event.Target, stateUnavailable[string, int](label)) || !reflect.DeepEqual(rows[1].value.Target, stateUnavailable[string, NodeRef](label)) {
			t.Errorf("label evidence lost: %+v", rows[1])
		}
		for _, row := range rows {
			if row.event.PassDomain != navigationUnknownPass {
				t.Error("unresolved D.S. manufactured a pass domain")
			}
		}
		s.cleanup()
	}
}

func TestRound3NavigationOrderingCancellationAndQuota(t *testing.T) {
	run := func(ctx context.Context, s *Session) error {
		rows := navigationRound3Rows(t, NavigationSound, NavigationListening, NavigationSound, NavigationDalSegno)
		if _, err := navigationOrder(ctx, s, &timelineService{}, rows); err != nil {
			return err
		}
		if _, err := navigationLinearPrefix(ctx, s, &timelineService{}, rows); err != nil {
			return err
		}
		rows[3].value.Label = stateKnown("missing")
		if err := navigationBuildEdges(ctx, s, rows); err != nil {
			return err
		}
		// Exercise the early unavailable-label branch under every cancellation
		// and quota boundary as well as normal missing-marker lookup.
		rows[3].value.Label = Evidence[string]{State: EvidenceInvalid, Reason: "label-invalid", Cause: &DecisionRef{Rule: RuleKey{"label", 1}, Subject: NodeRef{"d", 9}}}
		if err := navigationBuildEdges(ctx, s, rows); err != nil {
			return err
		}
		rows[3].event.Guard = stateKnown("1")
		events := make([]navigationEvent, len(rows))
		for i := range rows {
			events[i] = rows[i].event
		}
		_, err := runNavigation(ctx, s, events)
		return err
	}
	probe := rationalSession(t, Budgets{})
	counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	if err := run(counter, probe); err != nil {
		t.Fatal(err)
	}
	work := rationalUsage(t, probe, BudgetRuleSteps)
	probe.cleanup()
	for at := 1; at <= counter.calls; at++ {
		s := rationalSession(t, Budgets{})
		err := run(&rationalCancelContext{Context: context.Background(), at: at}, s)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("poll %d: %v", at, err)
		}
		steps, facts := rationalUsage(t, s, BudgetRuleSteps), rationalUsage(t, s, BudgetContextFacts)
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 || rationalUsage(t, s, BudgetRuleSteps) != steps || rationalUsage(t, s, BudgetContextFacts) != facts {
			t.Fatalf("poll %d leaked/refunded resources", at)
		}
	}
	for _, budget := range []Budgets{{MaxRuleSteps: work - 1}, {MaxBufferedBytes: 1}, {MaxContextFacts: 1}} {
		s := rationalSession(t, budget)
		if err := run(context.Background(), s); !errors.Is(err, ErrResourceLimit) {
			t.Errorf("budget %+v: %v", budget, err)
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Error("budget failure leaked scratch")
		}
	}
	s := rationalSession(t, Budgets{MaxRuleSteps: work})
	if err := run(context.Background(), s); err != nil {
		t.Errorf("exact work budget %d: %v", work, err)
	}
	s.cleanup()
	t.Logf("checked %d cancellation boundaries and exact work limit %d", counter.calls, work)
}

func TestRound3NavigationFineKeepsActivationBoundary(t *testing.T) {
	for _, tc := range []struct {
		guard  Evidence[string]
		domain navigationPassDomain
		want   Evidence[bool]
	}{
		{stateKnown("1"), navigationUnknownPass, stateUnknown[bool]("navigation-pass-unavailable")},
		{stateAbsent[string](), navigationUnknownPass, stateUnknown[bool]("navigation-fine-activation-unresolved")},
		{stateKnown("2"), navigationMovementPass, navigationKnownBool(true)},
	} {
		s := rationalSession(t, Budgets{})
		got, err := runNavigation(context.Background(), s, []navigationEvent{{Source: NodeRef{"d", 1}, Kind: NavigationFine, Guard: tc.guard, PassDomain: tc.domain}})
		if err != nil || !reflect.DeepEqual(got.Complete, tc.want) {
			t.Errorf("fine complete=%+v, %v, want %+v", got.Complete, err, tc.want)
		}
		s.cleanup()
	}
}

func TestRound3NavigationDemotedMarkerOrdering(t *testing.T) {
	for _, kind := range []NavigationKind{NavigationSegno, NavigationCoda} {
		s := rationalSession(t, Budgets{})
		rows := navigationRound3Rows(t, NavigationSound, NavigationBackward)
		rows[0].value = NavigationContextResult{Kind: kind, MarkerActive: navigationKnownBool(false), Label: stateKnown("M")}
		got, err := navigationOrder(context.Background(), s, &timelineService{}, rows)
		if err != nil || !reflect.DeepEqual(got, navigationKnownBool(true)) {
			t.Errorf("inactive %s competes: %+v %v", kind, got, err)
		}
		if rows[0].value.Kind != kind || rows[0].value.MarkerActive.State != EvidenceKnown || rows[0].value.MarkerActive.Value {
			t.Error("demoted marker source fact changed")
		}
		s.cleanup()
	}
}
