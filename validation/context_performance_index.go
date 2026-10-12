package validation

import (
	"context"
	"sort"
)

// Provider-local indexes borrow immutable timeline records. Their reservation is
// released at the provider boundary, including partial builds and interruption.
type performanceMeasure struct {
	Part    NodeRef
	Ordinal uint64
}

type performanceGapQueries struct {
	ordinals []uint64
	next     []int
}

func (x *performanceAdviceIndex) release(s *Session) {
	if x == nil || x.bytes == 0 {
		return
	}
	_ = s.account.Release(BudgetBufferedBytes, x.bytes)
	x.bytes = 0
}

func performanceIndexReservation(s *Session, service *timelineService) (int64, error) {
	count := int64(0)
	if service != nil {
		if len(service.State.Divisions.Gaps) != 0 {
			count = int64(len(service.Facts))
		}
		for _, n := range []int{len(service.State.Divisions.Publications), len(service.State.Divisions.Gaps), len(service.State.MIDI), len(service.State.NoteInstruments)} {
			var err error
			count, err = rationalCheckedAdd(BudgetBufferedBytes, count, int64(n))
			if err != nil {
				return 0, err
			}
		}
	}
	// Bounds include sparse map entries, slice growth, sort/union-find scratch,
	// and borrowed reference headers on either supported pointer width.
	cost, err := rationalCheckedMul(BudgetBufferedBytes, count, 1024)
	if err != nil {
		return 0, err
	}
	cost, err = rationalCheckedAdd(BudgetBufferedBytes, cost, 1024)
	if err != nil {
		return 0, err
	}
	if _, err = AllocationSize(cost); err != nil {
		return 0, err
	}
	if err = s.account.Charge(BudgetBufferedBytes, cost); err != nil {
		return 0, err
	}
	return cost, nil
}

// Assign gaps to observed query measures, never to every integer in an encoded
// interval. Reverse source order plus successor links assigns each measure once,
// preserving the former last-applicable-gap precedence in O((facts+gaps) log n).
func performanceIndexGaps(ctx context.Context, s *Session, service *timelineService, x *performanceAdviceIndex) error {
	if len(service.State.Divisions.Gaps) == 0 {
		return nil
	}
	queries := make(map[NodeRef]*performanceGapQueries)
	seen := make(map[performanceMeasure]bool)
	// Facts are admitted from this sequence. Keeping its stable order makes
	// sorting cost and every interruption boundary reproducible.
	for _, id := range s.sequence {
		if err := s.step(ctx, 1); err != nil {
			return err
		}
		f, ok := service.Facts[id]
		if !ok {
			continue
		}
		if f.Origin.State != EvidenceKnown {
			continue
		}
		p := f.Origin.Value
		if err := s.step(ctx, 1+int64(len(p.Part.DocumentID))); err != nil {
			return err
		}
		key := performanceMeasure{p.Part, p.MeasureOrdinal}
		if seen[key] {
			continue
		}
		seen[key] = true
		q := queries[p.Part]
		if q == nil {
			q = &performanceGapQueries{}
			queries[p.Part] = q
		}
		q.ordinals = append(q.ordinals, p.MeasureOrdinal)
	}
	for _, q := range queries {
		if err := s.step(ctx, 1); err != nil {
			return err
		}
		var sortErr error
		sort.Slice(q.ordinals, func(i, j int) bool {
			if sortErr != nil {
				return false
			}
			sortErr = s.step(ctx, 1)
			return sortErr == nil && q.ordinals[i] < q.ordinals[j]
		})
		if sortErr != nil {
			return sortErr
		}
		q.next = make([]int, len(q.ordinals)+1)
		for i := range q.next {
			if err := s.step(ctx, 1); err != nil {
				return err
			}
			q.next[i] = i
		}
	}
	find := func(q *performanceGapQueries, i int) (int, error) {
		for q.next[i] != i {
			if err := s.step(ctx, 1); err != nil {
				return 0, err
			}
			q.next[i] = q.next[q.next[i]]
			i = q.next[i]
		}
		return i, nil
	}
	for i := len(service.State.Divisions.Gaps) - 1; i >= 0; i-- {
		g := service.State.Divisions.Gaps[i]
		if err := s.step(ctx, 1+int64(len(g.Part.DocumentID))); err != nil {
			return err
		}
		q := queries[g.Part]
		if q == nil {
			continue
		}
		lo, hi := 0, len(q.ordinals)
		for lo < hi {
			if err := s.step(ctx, 1); err != nil {
				return err
			}
			mid := lo + (hi-lo)/2
			if q.ordinals[mid] < g.FirstMeasure {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		at, err := find(q, lo)
		if err != nil {
			return err
		}
		for at < len(q.ordinals) && q.ordinals[at] <= g.LastMeasure {
			if err := s.step(ctx, 1+int64(len(g.Part.DocumentID))); err != nil {
				return err
			}
			x.movementGaps[performanceMeasure{g.Part, q.ordinals[at]}] = g.Availability
			next, err := find(q, at+1)
			if err != nil {
				return err
			}
			q.next[at] = next
			at = next
		}
	}
	return nil
}
