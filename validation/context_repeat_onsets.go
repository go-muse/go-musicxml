package validation

import "context"

// This lazy, selected-scope proof is needed only for an empty target. Its
// sparse witness map and borrowed availability evidence live with the scope;
// repeated style starts do not rescan all notes or allocate another roster.
func contextRepeatBuildOnsetProof(ctx context.Context, s *Session, timeline *timelineService, notes []contextRepeatNote, part NodeRef) (map[uint64]bool, Evidence[bool], func(), error) {
	release, err := contextRepeatIndexReserve(ctx, s, len(notes))
	if err != nil {
		return nil, Evidence[bool]{}, nil, err
	}
	ok := false
	defer func() {
		if !ok {
			release()
		}
	}()
	witnesses := make(map[uint64]bool)
	complete := navigationKnownBool(true)
	for _, note := range notes {
		if err = s.step(ctx, 1+int64(len(note.part.Value.DocumentID)+len(part.DocumentID))); err != nil {
			return nil, Evidence[bool]{}, nil, err
		}
		if note.part.State != EvidenceKnown {
			complete = stateUnknown[bool]("context-repeat-empty-coverage-unresolved")
			break
		}
		if note.part.Value != part {
			continue
		}
		// Keep availability ahead of coordinate filtering: unsupported grace, ties,
		// cue and playback content may not even have assigned pos yet.
		if note.available.State != EvidenceKnown {
			complete = note.available
			if complete.State == "" || complete.State == EvidenceAbsent {
				complete.State = EvidenceUnknown
				complete.Reason = "context-repeat-empty-coverage-unresolved"
			}
			break
		}
		if !note.available.Value {
			complete = stateUnknown[bool]("context-repeat-empty-coverage-unresolved")
			break
		}
		if !note.pos.Offset.Valid() {
			complete = stateUnknown[bool]("context-repeat-empty-coverage-unresolved")
			break
		}
		member, e := timeline.State.frameMember(ctx, s, note.pos)
		if e != nil {
			return nil, Evidence[bool]{}, nil, e
		}
		if !member {
			complete = stateUnknown[bool]("context-repeat-empty-coverage-unresolved")
			break
		}
		// A rest-only source, orphan chord or unavailable/nonpositive duration
		// supplies no sufficient ordinary scored-onset witness.
		if !note.rest && !note.chord && note.duration.Valid() && note.duration.value.Sign() > 0 {
			witnesses[note.pos.MeasureOrdinal] = true
		}
	}
	if err = s.step(ctx, 1); err != nil {
		return nil, Evidence[bool]{}, nil, err
	}
	ok = true
	return witnesses, complete, release, nil
}

func (scope *contextRepeatProofScope) closeOnsets() {
	if scope.releaseOnsets != nil {
		scope.releaseOnsets()
		scope.releaseOnsets = nil
	}
}
