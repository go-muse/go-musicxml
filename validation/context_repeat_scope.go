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
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	release, err := contextRepeatIndexReserve(ctx, s, len(rows), len(s.sequence))
	if err != nil {
		return err
	}
	defer release()
	stops := make(map[NodeRef]Evidence[musicalPosition])
	for _, row := range rows {
		if err = s.step(ctx, 1+int64(len(row.source.DocumentID))); err != nil {
			return err
		}
		if _, exists := stops[row.source]; !exists {
			stops[row.source] = row.position
		}
	}
	notes := make(map[NodeRef][]int)
	var unknown, all []int
	for index, id := range s.sequence {
		if err = s.step(ctx, 1); err != nil {
			return err
		}
		note, ok := timeline.Playback.Notes[id]
		if !ok {
			continue
		}
		all = append(all, index)
		if note.Origin.Part.State != EvidenceKnown {
			unknown = append(unknown, index)
			continue
		}
		if err = s.step(ctx, 1+int64(len(note.Origin.Part.Value.DocumentID))); err != nil {
			return err
		}
		notes[note.Origin.Part.Value] = append(notes[note.Origin.Part.Value], index)
	}

	voiceFilters := make([]*contextRepeatVoiceFilter, len(rows))
	var voiceReleases []func()
	defer func() {
		for _, free := range voiceReleases {
			free()
		}
	}()
	for i := range rows {
		if err := s.step(ctx, 1); err != nil {
			return err
		}
		if len(rows[i].value.ExceptVoices) == 0 {
			continue
		}
		filter, free, e := contextRepeatBuildVoiceFilter(ctx, s, rows[i].value.ExceptVoices)
		if e != nil {
			return e
		}
		voiceFilters[i] = filter
		voiceReleases = append(voiceReleases, free)
	}
	for i := range rows {
		if err = s.step(ctx, 1); err != nil {
			return err
		}
		row := &rows[i]
		if row.value.Type.State != EvidenceKnown || row.value.Type.Value != "start" {
			continue
		}
		stop := Evidence[musicalPosition]{State: EvidenceAbsent}
		if row.value.Stop.State == EvidenceKnown {
			stop = stateUnknown[musicalPosition]("context-repeat-stop-position-unavailable")
			if err = s.step(ctx, 1+int64(len(row.value.Stop.Value.DocumentID))); err != nil {
				return err
			}
			if pos, ok := stops[row.value.Stop.Value]; ok {
				stop = pos
			}
		} else if row.value.Stop.State != EvidenceAbsent {
			stop = stateUnavailable[NodeRef, musicalPosition](row.value.Stop)
		}
		if err = s.step(ctx, 1+int64(len(row.value.Part.Value.DocumentID))); err != nil {
			return err
		}
		local, extra := notes[row.value.Part.Value], unknown
		if row.value.Part.State != EvidenceKnown {
			local = all
			extra = nil
		}
		for left, right := 0, 0; left < len(local) || right < len(extra); {
			index := 0
			if right == len(extra) || left < len(local) && local[left] < extra[right] {
				index = local[left]
				left++
			} else {
				index = extra[right]
				right++
			}
			id := s.sequence[index]
			if err := s.step(ctx, 1); err != nil {
				return err
			}
			note, ok := timeline.Playback.Notes[id]
			if !ok {
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
				if voiceFilters[i] != nil && !(v.Applies.State == EvidenceKnown && !v.Applies.Value) {
					applies, e := contextRepeatVoiceApplies(ctx, s, voiceFilters[i], fact.Voice)
					if e != nil {
						return e
					}
					if applies.State == EvidenceKnown && !applies.Value {
						v.Applies = applies
					} else {
						v.Applies = stateMergeGap(v.Applies, applies)
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
