package validation

import "context"

// Window entries borrow the source note and retain only its index and an exact
// end. Numeric outputs are owned by the Session arithmetic service; this
// temporary index prepays only its containers and never copies note evidence.
type contextRepeatWindowEntry struct {
	note int
	end  Rational
}

type contextRepeatWindowIndex struct {
	notes        []contextRepeatNote
	part         NodeRef
	measure      uint64
	entries      []contextRepeatWindowEntry
	ordered      bool
	first        int
	previousFrom Rational
}

// Only positive, source-ordered, nonoverlapping events admit a bounded window
// search. Anything unplaced or outside this sufficient domain retains the full
// conservative scan, including its original gap precedence and causes.
func contextRepeatBuildWindowIndex(ctx context.Context, s *Session, notes []contextRepeatNote, part NodeRef, measure uint64) (*contextRepeatWindowIndex, func(), error) {
	if err := s.step(ctx, 1); err != nil {
		return nil, nil, err
	}
	bytes, err := rationalCheckedMul(BudgetBufferedBytes, int64(len(notes)), 64)
	if err != nil {
		return nil, nil, err
	}
	bytes, err = rationalSum(BudgetBufferedBytes, 512, bytes)
	if err != nil {
		return nil, nil, err
	}
	if _, err = AllocationSize(bytes); err != nil {
		return nil, nil, err
	}
	if err = s.account.Charge(BudgetBufferedBytes, bytes); err != nil {
		return nil, nil, err
	}
	release := func() { _ = s.account.Release(BudgetBufferedBytes, bytes) }
	out := &contextRepeatWindowIndex{notes: notes, part: part, measure: measure}
	out.entries = make([]contextRepeatWindowEntry, 0, len(notes))
	for i := range notes {
		note := &notes[i]
		if err = s.step(ctx, 1+int64(len(note.part.Value.DocumentID))+int64(len(part.DocumentID))); err != nil {
			release()
			return nil, nil, err
		}
		if note.part.State != EvidenceKnown {
			return out, release, nil
		}
		if note.part.Value != part {
			continue
		}
		// Window checks placement before measure membership. Even an unplaced
		// note whose nominal measure differs may carry the decisive cause.
		if !note.pos.Offset.Valid() {
			return out, release, nil
		}
		if note.pos.MeasureOrdinal != measure {
			continue
		}
		if note.available.State != EvidenceKnown || !note.duration.Valid() || note.duration.value.Sign() <= 0 {
			return out, release, nil
		}
		if len(out.entries) != 0 {
			cmp, e := s.rationalCmp(ctx, out.entries[len(out.entries)-1].end, note.pos.Offset)
			if e != nil {
				release()
				return nil, nil, e
			}
			if cmp > 0 {
				return out, release, nil
			}
		}
		end, e := s.rationalAdd(ctx, note.pos.Offset, note.duration)
		if e != nil {
			release()
			return nil, nil, e
		}
		out.entries = append(out.entries, contextRepeatWindowEntry{note: i, end: end})
	}
	out.ordered = true
	return out, release, nil
}

func (index *contextRepeatWindowIndex) window(ctx context.Context, s *Session, from, to Rational) ([]contextRepeatNote, Evidence[bool], error) {
	if err := s.step(ctx, 1); err != nil {
		return nil, Evidence[bool]{}, err
	}
	notes := index.notes
	nextFirst := index.first
	if index.ordered {
		// Positive, nonoverlapping intervals have strictly increasing ends and
		// starts. Keep boundary-crossing candidates for Window to diagnose.
		first, last, err := index.bounds(ctx, s, from, to)
		if err != nil {
			return nil, Evidence[bool]{}, err
		}
		nextFirst = first
		if first == last {
			notes = nil
		} else {
			// Interleaved known-other-part or placed-other-measure notes may
			// remain in this borrowed slice; Window already skips them safely.
			notes = notes[index.entries[first].note : index.entries[last-1].note+1]
		}
	}
	out, status, err := contextRepeatWindow(ctx, s, notes, index.part, index.measure, from, to)
	if err == nil && index.ordered {
		index.first, index.previousFrom = nextFirst, from
	}
	return out, status, err
}

// Nondecreasing window starts advance a borrowed cursor in linear total work
// for consecutive beats. A backwards query uses exact binary search instead.
// The cursor starts at the previous first candidate, preserving any event that
// crossed the shared boundary; no event is discarded based on its start alone.
func (index *contextRepeatWindowIndex) bounds(ctx context.Context, s *Session, from, to Rational) (int, int, error) {
	forward := false
	if index.previousFrom.Valid() {
		cmp, err := s.rationalCmp(ctx, index.previousFrom, from)
		if err != nil {
			return 0, 0, err
		}
		forward = cmp <= 0
	}
	if forward {
		first := index.first
		for first < len(index.entries) {
			cmp, err := s.rationalCmp(ctx, index.entries[first].end, from)
			if err != nil {
				return 0, 0, err
			}
			if cmp > 0 {
				break
			}
			first++
		}
		last := first
		for last < len(index.entries) {
			start := index.notes[index.entries[last].note].pos.Offset
			cmp, err := s.rationalCmp(ctx, start, to)
			if err != nil {
				return 0, 0, err
			}
			if cmp >= 0 {
				break
			}
			last++
		}
		return first, last, nil
	}
	left, right := 0, len(index.entries)
	for left < right {
		middle := left + (right-left)/2
		cmp, err := s.rationalCmp(ctx, index.entries[middle].end, from)
		if err != nil {
			return 0, 0, err
		}
		if cmp <= 0 {
			left = middle + 1
		} else {
			right = middle
		}
	}
	first := left
	right = len(index.entries)
	for left < right {
		middle := left + (right-left)/2
		start := index.notes[index.entries[middle].note].pos.Offset
		cmp, err := s.rationalCmp(ctx, start, to)
		if err != nil {
			return 0, 0, err
		}
		if cmp < 0 {
			left = middle + 1
		} else {
			right = middle
		}
	}
	return first, left, nil
}
