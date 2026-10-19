package validation

import "context"

// The roster borrows the once-read content values and stores source-ordered
// indices. Unplaced part identities remain in a separate ordered list so a
// consumer cannot silently discard their causal gap.
type contextRepeatContentRoster struct {
	notes   []contextRepeatNote
	parts   map[NodeRef][]int
	unknown []int
}

func contextRepeatBuildContentRoster(ctx context.Context, s *Session, notes []contextRepeatNote) (*contextRepeatContentRoster, func(), error) {
	release, err := contextRepeatIndexReserve(ctx, s, len(notes))
	if err != nil {
		return nil, nil, err
	}
	out := &contextRepeatContentRoster{notes: notes, parts: make(map[NodeRef][]int)}
	for i, note := range notes {
		if err = s.step(ctx, 1+int64(len(note.part.Value.DocumentID))); err != nil {
			release()
			return nil, nil, err
		}
		if note.part.State != EvidenceKnown {
			out.unknown = append(out.unknown, i)
		} else {
			out.parts[note.part.Value] = append(out.parts[note.part.Value], i)
		}
	}
	return out, release, nil
}

func (roster *contextRepeatContentRoster) partNotes(ctx context.Context, s *Session, part NodeRef) ([]contextRepeatNote, func(), error) {
	if err := s.step(ctx, 1+int64(len(part.DocumentID))); err != nil {
		return nil, nil, err
	}
	local := roster.parts[part]
	count, err := rationalCheckedAdd(BudgetBufferedBytes, int64(len(local)), int64(len(roster.unknown)))
	if err != nil {
		return nil, nil, err
	}
	bytes, err := rationalCheckedMul(BudgetBufferedBytes, count, 2048)
	if err != nil {
		return nil, nil, err
	}
	capacity, err := allocationSize(count)
	if err != nil {
		return nil, nil, err
	}
	if _, err = allocationSize(bytes); err != nil {
		return nil, nil, err
	}
	if err = s.account.Charge(BudgetBufferedBytes, bytes); err != nil {
		return nil, nil, err
	}
	release := func() { _ = s.account.Release(BudgetBufferedBytes, bytes) }
	out := make([]contextRepeatNote, 0, capacity)
	for left, right := 0, 0; left < len(local) || right < len(roster.unknown); {
		if err = s.step(ctx, 1); err != nil {
			release()
			return nil, nil, err
		}
		index := 0
		if right == len(roster.unknown) || left < len(local) && local[left] < roster.unknown[right] {
			index = local[left]
			left++
		} else {
			index = roster.unknown[right]
			right++
		}
		out = append(out, roster.notes[index])
	}
	return out, release, nil
}
