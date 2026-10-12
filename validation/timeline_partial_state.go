package validation

import (
	"context"
	"reflect"
)

type statePossiblePublication struct {
	Index    int
	Order    statePositionOrder
	Definite bool
}

const statePossiblePublicationBytes int64 = 64

func stateQueryScopes(ctx context.Context, s *Session, pos musicalPosition, staff Integer) ([2]stateIndexScope, error) {
	if err := s.step(ctx, 2*int64(len(pos.Part.DocumentID))+staff.ByteSize()+1); err != nil {
		return [2]stateIndexScope{}, err
	}
	return [2]stateIndexScope{{Part: pos.Part, AllStaves: true}, {Part: pos.Part, Staff: staff.String()}}, nil
}
func statePartialQuery[T any](ctx context.Context, s *Session, h *timelineState, history *stateHistory[T], pos musicalPosition, staff Integer) (bool, error) {
	member, err := h.frameMember(ctx, s, pos)
	if err != nil {
		return false, err
	}
	if !member {
		return true, nil
	}
	index, err := stateIndex(ctx, s, history)
	if err != nil {
		return false, err
	}
	keys, err := stateQueryScopes(ctx, s, pos, staff)
	if err != nil {
		return false, err
	}
	return index.Scopes[keys[0]].Partial || index.Scopes[keys[1]].Partial, nil
}

// Exceptional histories form a partial order, not an ordinal prefix. All scoped
// publishers are candidates unless proved after the query. Only a definitely
// applicable unconditional strictly later statement can dominate one. The
// quadratic exceptional path is bounded by shared work and exact scratch
// admission; ordinary fully contained histories keep their binary-search path.
func queryPartialState[T, V any](ctx context.Context, s *Session, h *timelineState, history *stateHistory[T], pos musicalPosition, staff Evidence[Integer], project func(T) Evidence[V], inclusive, ownedResult bool, initial func() (stateResult[V], error), extra ...[]stateGap) (stateResult[V], error) {
	out := stateResult[V]{}
	index, err := stateIndex(ctx, s, history)
	if err != nil {
		return out, err
	}
	keys, err := stateQueryScopes(ctx, s, pos, staff.Value)
	if err != nil {
		return out, err
	}
	groups := [2][]int{index.Scopes[keys[0]].Publications, index.Scopes[keys[1]].Publications}
	count, err := rationalCheckedAdd(BudgetBufferedBytes, int64(len(groups[0])), int64(len(groups[1])))
	if err != nil {
		return out, err
	}
	bytes, err := rationalCheckedMul(BudgetBufferedBytes, count, statePossiblePublicationBytes)
	if err != nil {
		return out, err
	}
	bytes, err = rationalCheckedAdd(BudgetBufferedBytes, bytes, 256)
	if err != nil {
		return out, err
	}
	if _, err = AllocationSize(bytes); err != nil {
		return out, err
	}
	capacity, err := AllocationSize(count)
	if err != nil {
		return out, err
	}
	if err = s.account.Charge(BudgetBufferedBytes, bytes); err != nil {
		return out, err
	}
	defer func() { _ = s.account.Release(BudgetBufferedBytes, bytes) }()
	candidates := make([]statePossiblePublication, 0, capacity)
	definite := false
	for _, group := range groups {
		for _, i := range group {
			if err = s.step(ctx, 1); err != nil {
				return out, err
			}
			p := &history.Publications[i]
			order, e := h.positionOrder(ctx, s, p.Position, pos)
			if e != nil {
				return out, e
			}
			if !order.possiblyApplies(inclusive) {
				continue
			}
			// Whole-state histories may project a legitimate absent value. Statement
			// views exclude omission before reaching this path.
			d := order.definitelyApplies(inclusive) && stateUnconditional(p.TimeOnly)
			candidates = append(candidates, statePossiblePublication{i, order, d})
			definite = definite || d
		}
	}
	found, conflict := false, false
	var first Evidence[V]
	var firstCost int64
	allBefore, allEqual := true, true
	add := func(value Evidence[V], provenance []stateProvenance, order statePositionOrder) error {
		cost, e := stateMeasure(ctx, s, reflect.ValueOf(value))
		if e != nil {
			return e
		}
		compareCost, e := rationalCheckedAdd(BudgetRuleSteps, cost, firstCost)
		if e != nil {
			return e
		}
		compareCost, e = rationalCheckedAdd(BudgetRuleSteps, compareCost, 1)
		if e != nil {
			return e
		}
		if e = s.step(ctx, compareCost); e != nil {
			return e
		}
		if !found {
			first = value
			firstCost = cost
			out.Value = value
			found = true
		} else {
			if !reflect.DeepEqual(first, value) {
				conflict = true
			}
			if stateAvailabilityRank(value.State) > stateAvailabilityRank(out.Value.State) {
				out.Value = value
			}
		}
		allBefore = allBefore && order == stateBefore
		allEqual = allEqual && order == stateEqual
		if ownedResult {
			for _, pr := range provenance {
				duplicate := false
				for _, old := range out.Provenance {
					if e = s.step(ctx, 1); e != nil {
						return e
					}
					if old == pr {
						duplicate = true
						break
					}
				}
				if !duplicate {
					if e = stateRetainCopy(ctx, s, pr); e != nil {
						return e
					}
					out.Provenance = append(out.Provenance, stateClone(pr))
				}
			}
		}
		return nil
	}
	for _, candidate := range candidates {
		if err = s.step(ctx, 1); err != nil {
			return out, err
		}
		p := &history.Publications[candidate.Index]
		dominated := false
		for _, other := range candidates {
			if err = s.step(ctx, 1); err != nil {
				return out, err
			}
			if !other.Definite || other.Index == candidate.Index {
				continue
			}
			order, e := h.positionOrder(ctx, s, p.Position, history.Publications[other.Index].Position)
			if e != nil {
				return out, e
			}
			if order == stateBefore {
				dominated = true
				break
			}
		}
		if dominated {
			continue
		}
		if err = s.step(ctx, p.bytes); err != nil {
			return out, err
		}
		value := stateUnavailable[T, V](p.Value)
		if p.Value.State == EvidenceKnown {
			value = project(p.Value.Value)
		}
		// A known repeat condition makes this a possible publication. An actual
		// invalid/unsupported condition remains causal evidence, even in consensus.
		if p.TimeOnly != nil && p.TimeOnly.State != EvidenceKnown {
			value = stateConditionalValue(value, p.TimeOnly)
		}
		if err = add(value, p.Provenance, candidate.Order); err != nil {
			return out, err
		}
	}
	if !definite {
		baseline := stateResult[V]{Value: Evidence[V]{State: EvidenceAbsent}, Inherited: true}
		if initial != nil {
			baseline, err = initial()
			if err != nil {
				return out, err
			}
		}
		if err = add(baseline.Value, baseline.Provenance, stateBefore); err != nil {
			return out, err
		}
	}
	if !found {
		out.Value = Evidence[V]{State: EvidenceAbsent}
	}
	if conflict && (out.Value.State == EvidenceKnown || out.Value.State == EvidenceAbsent) {
		out.Value = stateUnknown[V]("timeline-state-order-unresolved")
	}
	gapAvailability := Evidence[bool]{State: EvidenceKnown, Value: true}
	visitGaps := func(gaps []stateGap) error {
		for _, gap := range gaps {
			if err := s.step(ctx, 1); err != nil {
				return err
			}
			if gap.Part != pos.Part {
				continue
			}
			applies, e := stateApplies(ctx, s, gap.Scope, staff)
			if e != nil {
				return e
			}
			if !applies {
				continue
			}
			relevant, e := h.gapCouldApply(ctx, s, gap, pos, inclusive)
			if e != nil {
				return e
			}
			if !relevant {
				continue
			}
			repaired := false
			for _, candidate := range candidates {
				if e = s.step(ctx, 1); e != nil {
					return e
				}
				if !candidate.Definite {
					continue
				}
				publication := history.Publications[candidate.Index]
				repaired, e = h.gapStrictlyBefore(ctx, s, gap, publication.Position)
				if e != nil {
					return e
				}
				if repaired {
					break
				}
			}
			if !repaired {
				gapAvailability = stateMergeGap(gapAvailability, gap.Availability)
			}
		}
		return nil
	}
	if err = visitGaps(history.Gaps); err != nil {
		return out, err
	}
	for _, gaps := range extra {
		if err = visitGaps(gaps); err != nil {
			return out, err
		}
	}
	if stateAvailabilityRank(gapAvailability.State) > stateAvailabilityRank(out.Value.State) {
		out.Value = stateUnavailable[bool, V](gapAvailability)
	}
	out.Inherited = found && allBefore
	out.InheritanceUnproved = found && !allBefore && !allEqual
	if ownedResult {
		if err = stateRetainCopy(ctx, s, out.Value); err != nil {
			return stateResult[V]{}, err
		}
		out.Value = stateClone(out.Value)
	} else {
		out.Value, err = stateOwnEvidence(ctx, s, out.Value)
	}
	return out, err
}
