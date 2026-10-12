package validation

import "context"

// One sparse partition per selected graphical range avoids rescanning unrelated
// measures for each repeated measure. Slice capacity/growth is prepaid; values
// borrow immutable content evidence until this proof returns.
func contextRepeatBuildMeasureRoster(ctx context.Context, s *Session, notes []contextRepeatNote, part NodeRef) (map[uint64][]contextRepeatNote, Evidence[bool], func(), error) {
	release, err := contextRepeatIndexReserve(ctx, s, len(notes), len(notes), len(notes), len(notes))
	if err != nil {
		return nil, Evidence[bool]{}, nil, err
	}
	out := make(map[uint64][]contextRepeatNote)
	complete := navigationKnownBool(true)
	for _, note := range notes {
		if err = s.step(ctx, 1+int64(len(note.part.Value.DocumentID)+len(part.DocumentID))); err != nil {
			release()
			return nil, Evidence[bool]{}, nil, err
		}
		if note.part.State != EvidenceKnown {
			complete = stateMergeGap(complete, stateUnavailable[NodeRef, bool](note.part))
			continue
		}
		if note.part.Value != part {
			continue
		}
		if !note.pos.Offset.Valid() {
			complete = stateMergeGap(complete, note.available)
			continue
		}
		out[note.pos.MeasureOrdinal] = append(out[note.pos.MeasureOrdinal], note)
	}
	return out, complete, release, nil
}
