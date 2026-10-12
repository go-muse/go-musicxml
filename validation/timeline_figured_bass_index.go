package validation

import (
	"context"
	"sort"
)

// Sufficient-proof acceleration only: incomplete kinds, coordinates, ownership
// or domains fall back to the full causal scan, rather than filling any gap.
type figuredBassNotePart struct {
	Safe     bool
	Notes    []NodeID
	Floating []NodeID
	Measures map[uint64]NodeRef
}
type figuredBassNoteIndex map[NodeRef]*figuredBassNotePart

func (service *timelineService) buildFiguredBassNoteIndex(r *stateReader, domains []figuredBassDomain) (figuredBassNoteIndex, func(), error) {
	var scratch int64
	release := func() { _ = r.s.account.Release(BudgetBufferedBytes, scratch) }
	charge := func(n int64) error {
		total, err := rationalCheckedAdd(BudgetBufferedBytes, scratch, n)
		if err != nil {
			return err
		}
		if _, err = AllocationSize(total); err != nil {
			return err
		}
		if err = r.s.account.Charge(BudgetBufferedBytes, n); err != nil {
			return err
		}
		scratch = total
		return nil
	}
	fail := func(err error) (figuredBassNoteIndex, func(), error) { release(); return nil, func() {}, err }
	for _, domain := range domains {
		if !r.step(1) {
			return fail(r.err)
		}
		if domain.block.part.State != EvidenceKnown {
			return nil, release, nil
		}
	}
	if err := charge(256); err != nil {
		return fail(err)
	}
	index := make(figuredBassNoteIndex)
	for _, domain := range domains {
		block := domain.block
		part := block.part.Value
		if !r.step(1 + int64(len(part.DocumentID))) {
			return fail(r.err)
		}
		item := index[part]
		if item == nil {
			if err := charge(1024); err != nil {
				return fail(err)
			}
			item = &figuredBassNotePart{Safe: true, Measures: make(map[uint64]NodeRef)}
			index[part] = item
		}
		measure := NodeRef{r.s.documentID, block.measure.header.ID}
		if _, exists := item.Measures[block.ordinal]; exists {
			item.Safe = false
		} else {
			if err := charge(128); err != nil {
				return fail(err)
			}
			item.Measures[block.ordinal] = measure
		}
		if domain.complete.State != EvidenceKnown || !domain.complete.Value {
			item.Safe = false
		}
		for _, candidate := range block.content.children {
			if !r.step(1) {
				return fail(r.err)
			}
			if !r.matches(candidate, timelineRoleNote) {
				if candidate.header.Declaration.State != EvidenceKnown && r.potential(candidate, timelineRoleNote) {
					item.Safe = false
				}
				continue
			}
			_, grace := r.child(candidate, timelineRoleNoteGrace)
			_, chord := r.child(candidate, timelineRoleNoteChord)
			if (grace.State == EvidenceKnown && grace.Value) || (chord.State == EvidenceKnown && chord.Value) {
				continue
			}
			if grace.State != EvidenceAbsent || chord.State != EvidenceAbsent {
				item.Safe = false
				continue
			}
			fact, found := service.Facts[candidate.header.ID]
			if !found || fact.Part.State != EvidenceKnown || !r.sameRef(fact.Part.Value, part) || fact.Position.State != EvidenceKnown {
				item.Safe = false
				continue
			}
			at := fact.Position.Value
			if !r.sameRef(at.Part, part) || !r.sameRef(at.Measure, measure) || at.MeasureOrdinal != block.ordinal {
				item.Safe = false
				continue
			}
			if err := charge(32); err != nil {
				return fail(err)
			}
			member, err := service.State.frameMember(r.ctx, r.s, at)
			if err != nil {
				return fail(err)
			}
			if !member {
				item.Floating = append(item.Floating, candidate.header.ID)
				continue
			}
			item.Notes = append(item.Notes, candidate.header.ID)
		}
	}
	if r.err != nil {
		return fail(r.err)
	}
	for _, item := range index {
		if !r.step(1) {
			return fail(r.err)
		}
		if !item.Safe {
			continue
		}
		// Admit a conservative bounded comparison drain before invoking sort. Once
		// interrupted, the comparator performs no more exact arithmetic.
		levels := int64(1)
		for n := len(item.Notes); n > 1; n /= 2 {
			levels++
		}
		work, err := rationalCheckedMul(BudgetRuleSteps, int64(len(item.Notes)), levels)
		if err != nil {
			return fail(err)
		}
		work, err = rationalCheckedMul(BudgetRuleSteps, work, 32)
		if err != nil {
			return fail(err)
		}
		if !r.step(work) {
			return fail(r.err)
		}
		var interrupted error
		sort.Slice(item.Notes, func(i, j int) bool {
			if interrupted != nil {
				return false
			}
			var cmp int
			cmp, interrupted = figuredBassScoreCompare(r.ctx, r.s, service.Facts[item.Notes[i]].Position.Value, service.Facts[item.Notes[j]].Position.Value)
			return interrupted == nil && cmp < 0
		})
		if interrupted != nil {
			return fail(interrupted)
		}
	}
	return index, release, nil
}

func (index figuredBassNoteIndex) anchor(ctx context.Context, s *Session, service *timelineService, origin timelineEventFact) (Evidence[NodeRef], timelineEventFact, bool, error) {
	empty := timelineEventFact{}
	r := &stateReader{ctx: ctx, s: s}
	if origin.Part.State != EvidenceKnown || origin.Position.State != EvidenceKnown {
		return Evidence[NodeRef]{}, empty, false, nil
	}
	at := origin.Position.Value
	if !r.sameRef(at.Part, origin.Part.Value) {
		return Evidence[NodeRef]{}, empty, false, r.err
	}
	if err := s.step(ctx, 1+int64(len(at.Part.DocumentID))); err != nil {
		return Evidence[NodeRef]{}, empty, false, err
	}
	item := index[at.Part]
	if item == nil || !item.Safe || !r.sameRef(item.Measures[at.MeasureOrdinal], at.Measure) {
		return Evidence[NodeRef]{}, empty, false, r.err
	}
	member, err := service.State.frameMember(ctx, s, at)
	if err != nil {
		return Evidence[NodeRef]{}, empty, false, err
	}
	if !member {
		return Evidence[NodeRef]{}, empty, false, nil
	}
	ids := item.Notes
	lo, hi := 0, len(ids)
	for lo < hi {
		mid := lo + (hi-lo)/2
		cmp, err := figuredBassScoreCompare(ctx, s, service.Facts[ids[mid]].Position.Value, at)
		if err != nil {
			return Evidence[NodeRef]{}, empty, false, err
		}
		if cmp < 0 {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	// The sorted contained prefix is still useful in a part with floating
	// candidates. Only a possibly competing displaced note requires the causal
	// fallback; distant and independently excluded exceptions preserve this path.
	for _, id := range item.Floating {
		if err := s.step(ctx, 1); err != nil {
			return Evidence[NodeRef]{}, empty, false, err
		}
		fact := service.Facts[id]
		order, err := service.State.positionOrder(ctx, s, fact.Position.Value, at)
		if err != nil {
			return Evidence[NodeRef]{}, empty, false, err
		}
		if order == stateBefore {
			continue
		}
		if lo < len(ids) {
			order, err = service.State.positionOrder(ctx, s, fact.Position.Value, service.Facts[ids[lo]].Position.Value)
			if err != nil {
				return Evidence[NodeRef]{}, empty, false, err
			}
			if order == stateAfter {
				continue
			}
		}
		return Evidence[NodeRef]{}, empty, false, nil
	}
	if lo == len(ids) {
		return stateUnknown[NodeRef]("figured-bass-following-note-unresolved"), empty, true, nil
	}
	best := service.Facts[ids[lo]]
	if lo+1 < len(ids) {
		cmp, err := figuredBassScoreCompare(ctx, s, best.Position.Value, service.Facts[ids[lo+1]].Position.Value)
		if err != nil {
			return Evidence[NodeRef]{}, empty, false, err
		}
		if cmp == 0 {
			return stateUnknown[NodeRef]("figured-bass-simultaneous-notes-unresolved"), empty, true, nil
		}
	}
	return Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{s.documentID, ids[lo]}}, best, true, nil
}
