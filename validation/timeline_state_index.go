package validation

import (
	"context"
	"sort"
)

// The index is sparse in observed canonical parts and exact staff identities.
// Each publication occurs in one bucket, independent of the encoded staff count.
// Global and explicit-staff buckets compete by musical position, never by scope
// specificity. The index is built transactionally and invalidated on publication.
type stateIndexScope struct {
	Part      NodeRef
	AllStaves bool
	Staff     string
}
type stateHistoryIndex struct {
	Scopes map[stateIndexScope][]int
	Count  int
	bytes  int64
}
type stateSelection struct {
	Groups [2][]int
	Best   int
}

func stateScopeKey(part NodeRef, scope stateScope) stateIndexScope {
	key := stateIndexScope{Part: part, AllStaves: scope.AllStaves}
	if !scope.AllStaves {
		key.Staff = scope.Staff.String()
	}
	return key
}
func stateDiscardIndex(s *Session, index *stateHistoryIndex) {
	if index == nil {
		return
	}
	_ = s.account.Release(BudgetBufferedBytes, index.bytes)
	s.retained -= index.bytes
}
func stateIndex[T any](ctx context.Context, s *Session, h *stateHistory[T]) (*stateHistoryIndex, error) {
	if h.index != nil && h.index.Count == len(h.Publications) {
		return h.index, nil
	}
	if e := s.step(ctx, int64(len(h.Publications))+1); e != nil {
		return nil, e
	}
	cost, e := rationalCheckedMul(BudgetBufferedBytes, int64(len(h.Publications)), 256)
	if e != nil {
		return nil, e
	}
	cost, e = rationalCheckedAdd(BudgetBufferedBytes, cost, 256)
	if e != nil {
		return nil, e
	}
	if _, e = AllocationSize(cost); e != nil {
		return nil, e
	}
	if e = s.retain(cost); e != nil {
		return nil, e
	}
	index := &stateHistoryIndex{Scopes: make(map[stateIndexScope][]int), Count: len(h.Publications), bytes: cost}
	success := false
	defer func() {
		if !success {
			stateDiscardIndex(s, index)
		}
	}()
	for i, p := range h.Publications {
		if e = s.step(ctx, int64(len(p.Position.Part.DocumentID))+p.Scope.Staff.ByteSize()+1); e != nil {
			return nil, e
		}
		key := stateScopeKey(p.Position.Part, p.Scope)
		index.Scopes[key] = append(index.Scopes[key], i)
	}
	// Sorting does not publish the candidate. Once a comparison is interrupted,
	// further comparator calls are constant no-ops and the partial sort is discarded.
	var compareError error
	for _, bucket := range index.Scopes {
		if e = s.step(ctx, 1); e != nil {
			return nil, e
		}
		sort.Slice(bucket, func(i, j int) bool {
			if compareError != nil {
				return false
			}
			var cmp int
			cmp, compareError = stateComparePosition(ctx, s, h.Publications[bucket[i]].Position, h.Publications[bucket[j]].Position)
			if compareError != nil {
				return false
			}
			if cmp == 0 {
				return bucket[i] < bucket[j]
			}
			return cmp < 0
		})
		if compareError != nil {
			return nil, compareError
		}
	}
	if e = s.step(ctx, 1); e != nil {
		return nil, e
	}
	stateDiscardIndex(s, h.index)
	h.index = index
	success = true
	return index, nil
}
func selectState[T any](ctx context.Context, s *Session, h *stateHistory[T], pos musicalPosition, staff Integer, inclusive bool) (stateSelection, error) {
	out := stateSelection{Best: -1}
	index, e := stateIndex(ctx, s, h)
	if e != nil {
		return out, e
	}
	if e = s.step(ctx, 2*int64(len(pos.Part.DocumentID))+staff.ByteSize()+1); e != nil {
		return out, e
	}
	keys := [2]stateIndexScope{{Part: pos.Part, AllStaves: true}, {Part: pos.Part, Staff: staff.String()}}
	ends := [2]int{}
	for k, key := range keys {
		bucket := index.Scopes[key]
		lo, hi := 0, len(bucket)
		for lo < hi {
			mid := lo + (hi-lo)/2
			cmp, e := stateComparePosition(ctx, s, h.Publications[bucket[mid]].Position, pos)
			if e != nil {
				return out, e
			}
			if cmp < 0 || inclusive && cmp == 0 {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		out.Groups[k] = bucket
		ends[k] = lo
		if lo == 0 {
			continue
		}
		candidate := bucket[lo-1]
		if out.Best < 0 {
			out.Best = candidate
		} else {
			cmp, e := stateComparePosition(ctx, s, h.Publications[candidate].Position, h.Publications[out.Best].Position)
			if e != nil {
				return out, e
			}
			if cmp > 0 {
				out.Best = candidate
			}
		}
	}
	if out.Best < 0 {
		out.Groups = [2][]int{}
		return out, nil
	}
	best := h.Publications[out.Best].Position
	for k, bucket := range out.Groups {
		end := ends[k]
		start := end
		for start > 0 {
			cmp, e := stateComparePosition(ctx, s, h.Publications[bucket[start-1]].Position, best)
			if e != nil {
				return out, e
			}
			if cmp != 0 {
				break
			}
			start--
		}
		out.Groups[k] = bucket[start:end]
	}
	return out, nil
}
