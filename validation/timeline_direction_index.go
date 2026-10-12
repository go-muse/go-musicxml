package validation

import (
	"context"
	"sort"
)

// This is only a sufficient-proof fast path. Any uncertain candidate/domain
// falls back to the existing causal scan; the index never fills missing facts.
type directionNotePart struct {
	Safe, UnknownVoice bool
	Notes              []NodeID
	Voices             map[string][]NodeID
	Measures           map[uint64]NodeRef
}
type directionNoteIndex map[NodeRef]*directionNotePart

func (service *timelineService) buildDirectionNoteIndex(r *stateReader, blocks []timelineMeasureBlock) (directionNoteIndex, func(), error) {
	scratch := int64(0)
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
	fail := func(err error) (directionNoteIndex, func(), error) { release(); return nil, timelineNoScratch, err }
	haveDirection := false
	for _, block := range blocks {
		if !r.step(1) {
			return fail(r.err)
		}
		if block.part.State != EvidenceKnown {
			return nil, release, nil
		}
		for _, event := range block.content.children {
			if r.matches(event, timelineRoleDirection) {
				haveDirection = true
			}
		}
	}
	if r.err != nil {
		return fail(r.err)
	}
	if !haveDirection {
		return nil, release, nil
	}
	if err := charge(256); err != nil {
		return fail(err)
	}
	index := make(directionNoteIndex)
	for _, block := range blocks {
		part := block.part.Value
		if !r.step(1 + int64(len(part.DocumentID))) {
			return fail(r.err)
		}
		item := index[part]
		if item == nil {
			if err := charge(1024); err != nil {
				return fail(err)
			}
			item = &directionNotePart{Safe: true, Voices: make(map[string][]NodeID), Measures: make(map[uint64]NodeRef)}
			index[part] = item
		}
		measure := NodeRef{r.s.documentID, block.measure.header.ID}
		if _, ok := item.Measures[block.ordinal]; ok {
			item.Safe = false
		} else {
			if err := charge(128); err != nil {
				return fail(err)
			}
			item.Measures[block.ordinal] = measure
		}
		if block.complete.State != EvidenceKnown || !block.complete.Value || block.coverage.State != EvidenceKnown || !block.coverage.Value {
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
			fact, found := service.Facts[candidate.header.ID]
			if !found || fact.Part.State != EvidenceKnown || fact.Part.Value != part || fact.Position.State != EvidenceKnown {
				item.Safe = false
				continue
			}
			at := fact.Position.Value
			if at.Part != part || at.Measure != measure || at.MeasureOrdinal != block.ordinal {
				item.Safe = false
				continue
			}
			if err := charge(32); err != nil {
				return fail(err)
			}
			item.Notes = append(item.Notes, candidate.header.ID)
			if fact.Voice.State != EvidenceKnown {
				item.UnknownVoice = true
				continue
			}
			voice := fact.Voice.Value
			if !r.step(1 + int64(len(voice))) {
				return fail(r.err)
			}
			if _, found := item.Voices[voice]; !found {
				if err := charge(512); err != nil {
					return fail(err)
				}
			}
			if err := charge(32); err != nil {
				return fail(err)
			}
			item.Voices[voice] = append(item.Voices[voice], candidate.header.ID)
		}
	}
	if r.err != nil {
		return fail(r.err)
	}
	sortIDs := func(ids []NodeID) error {
		var interrupted error
		sort.Slice(ids, func(i, j int) bool {
			if interrupted != nil {
				return false
			}
			var cmp int
			cmp, interrupted = stateComparePosition(r.ctx, r.s, service.Facts[ids[i]].Position.Value, service.Facts[ids[j]].Position.Value)
			return interrupted == nil && cmp < 0
		})
		return interrupted
	}
	for _, item := range index {
		if !r.step(1) {
			return fail(r.err)
		}
		if !item.Safe {
			continue
		}
		if len(item.Notes) > 0 {
			complete, err := service.State.closure(r.ctx, r.s, service.Facts[item.Notes[0]].Position.Value)
			if err != nil {
				return fail(err)
			}
			if complete.State != EvidenceKnown || !complete.Value {
				item.Safe = false
				continue
			}
		}
		if err := sortIDs(item.Notes); err != nil {
			return fail(err)
		}
		for _, ids := range item.Voices {
			if !r.step(1) {
				return fail(r.err)
			}
			if err := sortIDs(ids); err != nil {
				return fail(err)
			}
		}
	}
	return index, release, nil
}

func (index directionNoteIndex) anchor(ctx context.Context, s *Session, service *timelineService, origin timelineEventFact) (Evidence[NodeRef], timelineEventFact, bool, error) {
	empty := timelineEventFact{}
	if origin.Part.State != EvidenceKnown || origin.Origin.State != EvidenceKnown {
		return Evidence[NodeRef]{}, empty, false, nil
	}
	at := origin.Origin.Value
	if at.Part != origin.Part.Value {
		return Evidence[NodeRef]{}, empty, false, nil
	}
	if err := s.step(ctx, 1+int64(len(at.Part.DocumentID))); err != nil {
		return Evidence[NodeRef]{}, empty, false, err
	}
	item := index[at.Part]
	if item == nil || !item.Safe || item.Measures[at.MeasureOrdinal] != at.Measure {
		return Evidence[NodeRef]{}, empty, false, nil
	}
	complete, err := service.State.closure(ctx, s, at)
	if err != nil {
		return Evidence[NodeRef]{}, empty, false, err
	}
	if complete.State != EvidenceKnown || !complete.Value {
		return Evidence[NodeRef]{}, empty, false, nil
	}
	ids := item.Notes
	if origin.Voice.State == EvidenceKnown {
		if item.UnknownVoice {
			return Evidence[NodeRef]{}, empty, false, nil
		}
		if err := s.step(ctx, int64(len(origin.Voice.Value))); err != nil {
			return Evidence[NodeRef]{}, empty, false, err
		}
		ids = item.Voices[origin.Voice.Value]
	} else if origin.Voice.State != EvidenceAbsent {
		return Evidence[NodeRef]{}, empty, false, nil
	}
	lo, hi := 0, len(ids)
	for lo < hi {
		mid := lo + (hi-lo)/2
		cmp, err := stateComparePosition(ctx, s, service.Facts[ids[mid]].Position.Value, at)
		if err != nil {
			return Evidence[NodeRef]{}, empty, false, err
		}
		if cmp < 0 {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo == len(ids) {
		return Evidence[NodeRef]{State: EvidenceAbsent}, empty, true, nil
	}
	best := service.Facts[ids[lo]]
	if lo+1 < len(ids) {
		cmp, err := stateComparePosition(ctx, s, best.Position.Value, service.Facts[ids[lo+1]].Position.Value)
		if err != nil {
			return Evidence[NodeRef]{}, empty, false, err
		}
		if cmp == 0 {
			return stateUnknown[NodeRef]("direction-simultaneous-notes-unresolved"), empty, true, nil
		}
	}
	return Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{s.documentID, ids[lo]}}, best, true, nil
}
