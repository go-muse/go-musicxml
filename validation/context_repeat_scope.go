package validation

import "context"

// RepeatNoteScope separates graphical range membership from staff/voice
// applicability. It never changes encoded notes or the performed traversal.
type RepeatNoteScope struct {
	Source  NodeRef
	InRange Evidence[bool]
	Applies Evidence[bool]
}

func contextRepeatNoteScopes(ctx context.Context, s *Session, timeline *timelineService, rows []contextRepeatRecord) error {
	for i := range rows {
		row := &rows[i]
		if row.value.Type.State != EvidenceKnown || row.value.Type.Value != "start" {
			continue
		}
		stop := Evidence[musicalPosition]{State: EvidenceAbsent}
		if row.value.Stop.State == EvidenceKnown {
			stop = stateUnknown[musicalPosition]("context-repeat-stop-position-unavailable")
			for _, other := range rows {
				if err := s.step(ctx, 1); err != nil {
					return err
				}
				if other.source == row.value.Stop.Value {
					stop = other.position
					break
				}
			}
		} else if row.value.Stop.State != EvidenceAbsent {
			stop = stateUnavailable[NodeRef, musicalPosition](row.value.Stop)
		}
		for _, id := range s.sequence {
			if err := s.step(ctx, 1); err != nil {
				return err
			}
			note, ok := timeline.Playback.Notes[id]
			if !ok {
				continue
			}
			if row.value.Part.State == EvidenceKnown && note.Origin.Part.State == EvidenceKnown && row.value.Part.Value != note.Origin.Part.Value {
				continue
			}
			fact := timeline.Facts[id]
			v := RepeatNoteScope{Source: note.Origin.Source, InRange: stateUnknown[bool]("context-repeat-range-membership-unavailable")}
			if row.value.Part.State != EvidenceKnown {
				v.InRange = stateUnavailable[NodeRef, bool](row.value.Part)
			} else if note.Origin.Part.State != EvidenceKnown {
				v.InRange = stateUnavailable[NodeRef, bool](note.Origin.Part)
			} else if row.position.State != EvidenceKnown {
				v.InRange = stateUnavailable[musicalPosition, bool](row.position)
			} else if fact.Position.State != EvidenceKnown {
				v.InRange = stateUnavailable[musicalPosition, bool](fact.Position)
			} else {
				order, err := timeline.State.positionOrder(ctx, s, fact.Position.Value, row.position.Value)
				if err != nil {
					return err
				}
				if order == stateBefore {
					v.InRange = navigationKnownBool(false)
				} else if order == stateEqual || order == stateAfter {
					if stop.State == EvidenceKnown {
						end, err := timeline.State.positionOrder(ctx, s, fact.Position.Value, stop.Value)
						if err != nil {
							return err
						}
						if end == stateBefore {
							v.InRange = navigationKnownBool(true)
						} else if end == stateEqual || end == stateAfter {
							v.InRange = navigationKnownBool(false)
						}
					} else if stop.State == EvidenceAbsent && row.value.Complete.State == EvidenceKnown && row.value.Complete.Value {
						v.InRange = navigationKnownBool(true)
					} else if stop.State != EvidenceAbsent {
						v.InRange = stateUnavailable[musicalPosition, bool](stop)
					}
				}
			}
			v.Applies = v.InRange
			if v.InRange.State != EvidenceKnown || v.InRange.Value {
				scope := row.value.AllStaves
				if scope.State == EvidenceKnown && !scope.Value {
					if fact.Staff.State != EvidenceKnown {
						scope = stateUnavailable[Integer, bool](fact.Staff)
						if scope.State == EvidenceAbsent {
							scope = stateUnknown[bool]("context-repeat-note-staff-unavailable")
						}
					} else if row.value.Staff.State != EvidenceKnown {
						scope = stateUnavailable[Integer, bool](row.value.Staff)
					} else {
						if err := s.step(ctx, fact.Staff.Value.ByteSize()+row.value.Staff.Value.ByteSize()); err != nil {
							return err
						}
						cmp, err := fact.Staff.Value.Cmp(row.value.Staff.Value)
						if err != nil {
							return err
						}
						scope = navigationKnownBool(cmp == 0)
					}
				}
				if scope.State == EvidenceKnown && !scope.Value {
					v.Applies = navigationKnownBool(false)
				} else {
					v.Applies = stateMergeGap(v.Applies, scope)
				}
				for _, except := range row.value.ExceptVoices {
					if err := s.step(ctx, 1+int64(len(except.Value)+len(fact.Voice.Value))); err != nil {
						return err
					}
					if except.State == EvidenceKnown && fact.Voice.State == EvidenceKnown && except.Value == fact.Voice.Value {
						v.Applies = navigationKnownBool(false)
						break
					}
					if v.Applies.State == EvidenceKnown && !v.Applies.Value {
						continue
					}
					if except.State != EvidenceKnown {
						v.Applies = stateMergeGap(v.Applies, stateUnavailable[string, bool](except))
					} else if fact.Voice.State != EvidenceKnown {
						gap := stateUnavailable[string, bool](fact.Voice)
						if gap.State == EvidenceAbsent {
							gap = stateUnknown[bool]("context-repeat-note-voice-unavailable")
						}
						v.Applies = stateMergeGap(v.Applies, gap)
					}
				}
			}
			if _, err := stateReserve(ctx, s, v); err != nil {
				return err
			}
			row.value.Notes = append(row.value.Notes, stateClone(v))
		}
	}
	return nil
}
