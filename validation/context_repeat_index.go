package validation

import (
	"context"
	"sort"
)

// Temporary indexes borrow already admitted immutable references/strings. The
// worst-case map headers, groups, slice growth and index cells are prepaid;
// their gauge is released on every return, including interruption.
func contextRepeatIndexReserve(ctx context.Context, s *Session, counts ...int) (func(), error) {
	total := int64(512)
	for _, count := range counts {
		bytes, err := rationalCheckedMul(BudgetBufferedBytes, int64(count), 1024)
		if err != nil {
			return nil, err
		}
		total, err = rationalSum(BudgetBufferedBytes, total, bytes)
		if err != nil {
			return nil, err
		}
	}
	if err := s.step(ctx, 1); err != nil {
		return nil, err
	}
	if err := s.account.Charge(BudgetBufferedBytes, total); err != nil {
		return nil, err
	}
	return func() { _ = s.account.Release(BudgetBufferedBytes, total) }, nil
}

type contextRepeatPartKind struct {
	part NodeRef
	kind RepeatStyleKind
}

func contextRepeatRowGroups(ctx context.Context, s *Session, rows []contextRepeatRecord) (map[contextRepeatPartKind][]int, []int, []int, error) {
	groups := make(map[contextRepeatPartKind][]int)
	var unknown, all []int
	for i, row := range rows {
		if err := s.step(ctx, 1+int64(len(row.value.Part.Value.DocumentID)+len(row.value.Kind))); err != nil {
			return nil, nil, nil, err
		}
		all = append(all, i)
		if row.value.Part.State != EvidenceKnown {
			unknown = append(unknown, i)
			continue
		}
		key := contextRepeatPartKind{row.value.Part.Value, row.value.Kind}
		groups[key] = append(groups[key], i)
	}
	return groups, unknown, all, nil
}

type contextRepeatFastCandidate struct {
	next      int
	ambiguous bool
}

// Fully contained, exact single-scope transition groups have a total score
// order. Their successor is indexed once; displaced/unknown/overlapping scopes
// retain the conservative partial-order path instead of receiving a guess.
func contextRepeatFastRanges(ctx context.Context, s *Session, timeline *timelineService, rows []contextRepeatRecord, groups map[contextRepeatPartKind][]int, unknown []int) (map[int]contextRepeatFastCandidate, error) {
	out := make(map[int]contextRepeatFastCandidate)
	if len(unknown) != 0 {
		return out, nil
	}
	seen := make(map[contextRepeatPartKind]bool)
	for _, row := range rows {
		if err := s.step(ctx, 1+int64(len(row.value.Part.Value.DocumentID)+len(row.value.Kind))); err != nil {
			return nil, err
		}
		key := contextRepeatPartKind{row.value.Part.Value, row.value.Kind}
		if seen[key] {
			continue
		}
		seen[key] = true
		group := groups[key]
		valid := true
		for _, index := range group {
			value := rows[index]
			if value.position.State != EvidenceKnown || !value.position.Value.Offset.Valid() || value.position.Value.UnprovedFrame || value.position.Value.Part != key.part {
				valid = false
				break
			}
			scope, err := contextRepeatSameScope(ctx, s, row.value, value.value)
			if err != nil {
				return nil, err
			}
			if scope.State != EvidenceKnown || !scope.Value {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		if err := s.step(ctx, int64(len(group))); err != nil {
			return nil, err
		}
		ordered := append([]int(nil), group...)
		var compareErr error
		sort.SliceStable(ordered, func(i, j int) bool {
			if compareErr != nil {
				return false
			}
			order, err := timeline.State.positionOrder(ctx, s, rows[ordered[i]].position.Value, rows[ordered[j]].position.Value)
			if err != nil {
				compareErr = err
				return false
			}
			if order != stateBefore && order != stateEqual && order != stateAfter {
				valid = false
			}
			return order == stateBefore
		})
		if compareErr != nil {
			return nil, compareErr
		}
		if !valid {
			continue
		}
		for i := 1; i < len(ordered); i++ {
			order, err := timeline.State.positionOrder(ctx, s, rows[ordered[i-1]].position.Value, rows[ordered[i]].position.Value)
			if err != nil {
				return nil, err
			}
			if order != stateBefore && order != stateEqual {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		for i, index := range ordered {
			if err := s.step(ctx, 1); err != nil {
				return nil, err
			}
			candidate := contextRepeatFastCandidate{next: -1}
			if i+1 < len(ordered) {
				candidate.next = ordered[i+1]
			}
			out[index] = candidate
		}
		for i := 1; i < len(ordered); i++ {
			order, err := timeline.State.positionOrder(ctx, s, rows[ordered[i-1]].position.Value, rows[ordered[i]].position.Value)
			if err != nil {
				return nil, err
			}
			if order == stateEqual {
				for j := i - 2; j <= i; j++ {
					if j >= 0 {
						v := out[ordered[j]]
						v.ambiguous = true
						out[ordered[j]] = v
					}
				}
			}
		}
	}
	return out, nil
}
