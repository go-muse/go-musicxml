package validation

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
)

func crossReviewStateBefore[T, V any](ctx context.Context, s *Session, h *timelineState, history *stateHistory[T], pos musicalPosition, staff Evidence[Integer], project func(T) Evidence[V], inclusive, ownedResult, allStavesOnly bool, initial func() (stateResult[V], error), extra ...[]stateGap) (stateResult[V], error) {
	if allStavesOnly && (staff.State != EvidenceAbsent || staff.Value.Valid()) {
		return stateResult[V]{}, contract("timeline query", "all-staves baseline must not name a staff")
	}
	if !ownedResult && reflect.TypeFor[V]() != reflect.TypeFor[Decimal]() {
		return stateResult[V]{}, contract("timeline query", "evidence-only mode requires immutable Decimal")
	}
	out := stateResult[V]{}
	valid, err := statePositionValid(ctx, s, pos)
	if err != nil {
		return out, err
	}
	if !valid {
		return out, contract("timeline query", "known musical position required")
	}
	closure, e := h.closure(ctx, s, pos)
	if e != nil {
		return out, e
	}
	if closure.State != EvidenceKnown || !closure.Value {
		return stateUnavailableResult[bool, V](ctx, s, closure)
	}
	if !allStavesOnly && (staff.State != EvidenceKnown || !staff.Value.Valid()) {
		out.Value = stateUnknown[V]("timeline-staff-unresolved")
		if staff.State != EvidenceAbsent && staff.State != EvidenceKnown {
			return stateUnavailableResult[Integer, V](ctx, s, staff)
		}
		return out, nil
	}
	partial, e := statePartialQuery(ctx, s, h, history, pos, staff.Value)
	if e != nil {
		return out, e
	}
	if !partial {
		checkGaps := func(gaps []stateGap) error {
			for _, g := range gaps {
				if err := s.step(ctx, 1+int64(len(g.Part.DocumentID))+int64(len(pos.Part.DocumentID))); err != nil {
					return err
				}
				if g.Part != pos.Part || !(g.Placement.Displaced || g.Position != nil && g.Position.UnprovedFrame) {
					continue
				}
				applies, err := stateApplies(ctx, s, g.Scope, staff)
				if err != nil {
					return err
				}
				if applies {
					partial = true
					break
				}
			}
			return nil
		}
		if e = checkGaps(history.Gaps); e != nil {
			return out, e
		}
		for _, gaps := range extra {
			if e = checkGaps(gaps); e != nil {
				return out, e
			}
		}
	}
	if partial {
		return queryPartialState(ctx, s, h, history, pos, staff, project, inclusive, ownedResult, initial, extra...)
	}
	selection, e := selectState(ctx, s, history, pos, staff.Value, inclusive)
	if e != nil {
		return out, e
	}
	best := selection.Best
	gapAvailability := Evidence[bool]{State: EvidenceKnown, Value: true}
	var repairPosition *musicalPosition
	hasGaps := len(history.Gaps) != 0
	for _, gaps := range extra {
		hasGaps = hasGaps || len(gaps) != 0
	}
	if hasGaps {
		repairPosition, e = stateRepairPosition(ctx, s, history, staff.Value, selection)
		if e != nil {
			return out, e
		}
	}
	visitGaps := func(gaps []stateGap) error {
		for _, g := range gaps {
			if e = s.step(ctx, 1+int64(len(g.Part.DocumentID))+int64(len(pos.Part.DocumentID))); e != nil {
				return e
			}
			if g.Part != pos.Part || g.FirstMeasure > pos.MeasureOrdinal {
				continue
			}
			applies, x := stateApplies(ctx, s, g.Scope, staff)
			if x != nil {
				return x
			}
			if !applies {
				continue
			}
			unrepaired, err := h.stateGapUnrepaired(ctx, s, g, pos, repairPosition, inclusive)
			if err != nil {
				return err
			}
			if unrepaired {
				gapAvailability = stateMergeGap(gapAvailability, g.Availability)
			}
		}
		return nil
	}
	if e = visitGaps(history.Gaps); e != nil {
		return out, e
	}
	for _, gaps := range extra {
		if e = visitGaps(gaps); e != nil {
			return out, e
		}
	}

	if best < 0 {
		if gapAvailability.State != EvidenceKnown {
			return stateUnavailableResult[bool, V](ctx, s, gapAvailability)
		}
		if initial != nil {
			return initial()
		}
		out.Value = Evidence[V]{State: EvidenceAbsent}
		return out, nil
	}
	selected := history.Publications[best]
	projectValue := func(p statePublication[T]) Evidence[V] {
		value := stateUnavailable[T, V](p.Value)
		if p.Value.State == EvidenceKnown {
			value = project(p.Value.Value)
		}
		return stateConditionalValue(value, p.TimeOnly)
	}
	out.Value = projectValue(selected)
	selectedCost := selected.bytes
	if !ownedResult {
		selectedCost, e = stateMeasure(ctx, s, reflect.ValueOf(out.Value))
		if e != nil {
			return stateResult[V]{}, e
		}
	}
	if e = s.step(ctx, selectedCost); e != nil {
		return stateResult[V]{}, e
	}
	if e = s.step(ctx, 1); e != nil {
		return stateResult[V]{}, e
	}
	initialValue := out.Value
	conflict := false
	for _, group := range selection.Groups {
		for _, publicationIndex := range group {
			p := history.Publications[publicationIndex]
			if e = s.step(ctx, 1+int64(len(p.Position.Part.DocumentID))+int64(len(pos.Part.DocumentID))); e != nil {
				return stateResult[V]{}, e
			}
			if p.Position.Part != pos.Part {
				continue
			}
			applies, x := stateApplies(ctx, s, p.Scope, staff)
			if x != nil {
				return stateResult[V]{}, x
			}
			if !applies {
				continue
			}
			cmp, x := stateComparePosition(ctx, s, p.Position, selected.Position)
			if x != nil {
				return stateResult[V]{}, x
			}
			if cmp != 0 {
				continue
			}
			candidate := projectValue(p)
			candidateCost := p.bytes
			if !ownedResult {
				candidateCost, e = stateMeasure(ctx, s, reflect.ValueOf(candidate))
				if e != nil {
					return stateResult[V]{}, e
				}
			}
			compareCost, x := rationalCheckedAdd(BudgetRuleSteps, candidateCost, selectedCost)
			if x != nil {
				return stateResult[V]{}, x
			}
			if e = s.step(ctx, compareCost); e != nil {
				return stateResult[V]{}, e
			}
			if e = s.step(ctx, 1); e != nil {
				return stateResult[V]{}, e
			}
			if !reflect.DeepEqual(initialValue, candidate) {
				conflict = true
			}
			// Preserve an actual invalid/unsupported publication's causal evidence rather
			// than replacing it with an invented conflict cause.
			if candidate.State == EvidenceInvalid || (candidate.State == EvidenceUnsupported && out.Value.State != EvidenceInvalid) || (candidate.State == EvidenceUnknown && out.Value.State == EvidenceKnown) {
				out.Value = candidate
			}
			for _, pr := range p.Provenance {
				if !ownedResult {
					break
				}
				duplicate := false
				for _, prior := range out.Provenance {
					if e = s.step(ctx, 1+int64(len(prior.Node.DocumentID))+int64(len(pr.Node.DocumentID))); e != nil {
						return stateResult[V]{}, e
					}
					if prior == pr {
						duplicate = true
						break
					}
				}
				if !duplicate {
					if e = stateRetainCopy(ctx, s, pr); e != nil {
						return stateResult[V]{}, e
					}
					out.Provenance = append(out.Provenance, stateClone(pr))
				}
			}
		}
	}
	if conflict && (out.Value.State == EvidenceKnown || out.Value.State == EvidenceAbsent) {
		out.Value = stateUnknown[V]("timeline-state-conflict")
	}
	if stateAvailabilityRank(gapAvailability.State) > stateAvailabilityRank(out.Value.State) {
		out.Value = stateUnavailable[bool, V](gapAvailability)
	}
	if ownedResult {
		if e = stateRetainCopy(ctx, s, out.Value); e != nil {
			return stateResult[V]{}, e
		}
		out.Value = stateClone(out.Value)
	} else {
		out.Value, e = stateOwnEvidence(ctx, s, out.Value)
		if e != nil {
			return stateResult[V]{}, e
		}
	}
	cmp, e := stateComparePosition(ctx, s, selected.Position, pos)
	if e != nil {
		return stateResult[V]{}, e
	}
	out.Inherited = cmp < 0
	return out, nil
}

func crossSelectedValue(t *testing.T, code int, id NodeID) Evidence[Decimal] {
	if code < 2 {
		return stateKnown(stateDecimal(t, int64(code+1), 1))
	}
	states := []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid, EvidenceAbsent}
	v := Evidence[Decimal]{State: states[code-2]}
	if v.State != EvidenceAbsent {
		v.Reason = Code("candidate")
		v.Cause = &DecisionRef{Rule: RuleKey{"candidate", uint32(id)}, Subject: NodeRef{"d", id}}
	}
	return v
}
func crossSelectedFixture(t *testing.T, b Budgets, codes []int, split bool) (*Session, *timelineState, *stateHistory[Decimal], musicalPosition, Evidence[Integer]) {
	s := rationalSession(t, b)
	h := &timelineState{}
	stateClose(t, s, h, 0)
	at := statePosition(t, 0, 0, 1)
	staff := stateKnown(stateInt(t, 1))
	hist := &stateHistory[Decimal]{}
	for i, c := range codes {
		scope := stateScope{AllStaves: true}
		if split && i%2 == 0 {
			scope = stateScope{Staff: staff.Value}
		}
		statePublish(t, s, hist, at, scope, crossSelectedValue(t, c, NodeID(100+i)), NodeID(100+i))
	}
	return s, h, hist, at, staff
}
func TestIndependentStateSelectedReusePermutations(t *testing.T) {
	count := 0
	for a := 0; a < 6; a++ {
		for b := 0; b < 6; b++ {
			for c := 0; c < 6; c++ {
				for _, owned := range []bool{false, true} {
					for _, split := range []bool{false, true} {
						codes := []int{a, b, c}
						var outputs [2]stateResult[Decimal]
						var work [2]int64
						var calls [2]int
						for version := 0; version < 2; version++ {
							s, h, hist, at, staff := crossSelectedFixture(t, Budgets{}, codes, split)
							project := func(v Decimal) Evidence[Decimal] { calls[version]++; return stateKnown(v) }
							var err error
							if version == 0 {
								outputs[version], err = crossReviewStateBefore(context.Background(), s, h, hist, at, staff, project, true, owned, false, nil)
							} else {
								outputs[version], err = queryStateAtScope(context.Background(), s, h, hist, at, staff, project, true, owned, false, nil)
							}
							if err != nil {
								t.Fatal(err)
							}
							work[version] = rationalUsage(t, s, BudgetRuleSteps)
							if !owned && len(outputs[version].Provenance) != 0 {
								t.Fatal("borrowed query added provenance")
							}
							if outputs[version].Value.Cause != nil {
								copy := *outputs[version].Value.Cause
								outputs[version].Value.Cause.Rule.ID = "caller-mutated"
								again, e := queryStateAtScope(context.Background(), s, h, hist, at, staff, project, true, owned, false, nil)
								if e != nil || again.Value.Cause == nil || *again.Value.Cause != copy {
									t.Fatalf("owned cause aliased history: %+v %v", again, e)
								}
								*outputs[version].Value.Cause = copy
							}
							s.cleanup()
							if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
								t.Fatal("cleanup leak")
							}
						}
						if !reflect.DeepEqual(outputs[0], outputs[1]) {
							t.Fatalf("codes%v split%v owned%v changed result/provenance: BEFORE%+v AFTER%+v", codes, split, owned, outputs[0], outputs[1])
						}
						if work[1] >= work[0] {
							t.Fatalf("selected self comparison still charged codes%v: %v", codes, work)
						}
						count++
					}
				}
			}
		}
	}
	t.Logf("%d three-candidate permutations preserve values, exact causes, source-order provenance and owned/borrowed modes", count)
}
func crossSelectedMeasuredQuery(t *testing.T, b Budgets, ctx context.Context, owned bool) (int64, error) {
	s, h, hist, at, staff := crossSelectedFixture(t, b, []int{4, 3, 4}, true)
	defer s.cleanup()
	_, err := queryStateAtScope(ctx, s, h, hist, at, staff, stateWhole[Decimal], true, owned, false, nil)
	return rationalUsage(t, s, BudgetRuleSteps), err
}
func TestIndependentStateSelectedReuseBudgetBoundaries(t *testing.T) {
	for _, owned := range []bool{false, true} {
		work, err := crossSelectedMeasuredQuery(t, Budgets{}, context.Background(), owned)
		if err != nil {
			t.Fatal(err)
		}
		for _, delta := range []int64{0, -1} {
			got, e := crossSelectedMeasuredQuery(t, Budgets{MaxRuleSteps: work + delta}, context.Background(), owned)
			if delta == 0 && (e != nil || got != work) {
				t.Fatalf("exact work failed %d %v", got, e)
			}
			if delta < 0 && !errors.Is(e, ErrResourceLimit) {
				t.Fatalf("one-under gave %v", e)
			}
		}
		counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
		_, err = crossSelectedMeasuredQuery(t, Budgets{}, counter, owned)
		if err != nil {
			t.Fatal(err)
		}
		for at := 1; at <= counter.calls; at++ {
			ctx := &rationalCancelContext{Context: context.Background(), at: at}
			_, e := crossSelectedMeasuredQuery(t, Budgets{}, ctx, owned)
			if !errors.Is(e, context.Canceled) {
				t.Fatalf("cancel%d/%d owned%v: %v", at, counter.calls, owned, e)
			}
		}
		t.Logf("owned=%v exact=%d one-under refused; all %d cancellation polls refused", owned, work, counter.calls)
	}
}
