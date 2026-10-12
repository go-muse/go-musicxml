package validation

import (
	"context"
	"math/big"
)

// These are score-relative timing origins, not a rendered performance. In
// particular, swing and grace stealing are later playback transformations.
// Raw absence survives even where omission means no adjustment.
type noteTimingDelta struct {
	Raw, Units Evidence[Decimal]
	Delta      Evidence[Rational]
}
type noteTimingAdjustment struct {
	noteTimingDelta
	Origin, Position Evidence[musicalPosition]
}
type noteTimingPercent struct {
	Raw, Percent     Evidence[Decimal]
	Origin, Position Evidence[musicalPosition]
}
type bendTimingOrigin struct {
	Bend                      NodeRef
	OriginalNote              Evidence[musicalPosition]
	ReleasePresent            Evidence[bool]
	Release                   noteTimingAdjustment
	FirstBeat, LastBeat       noteTimingPercent
	RawBeats, Beats           Evidence[Decimal]
	RawAccelerate, Accelerate Evidence[bool]
}
type graceTimingOrigin struct {
	Grace                         NodeRef
	StealPrevious, StealFollowing Evidence[Decimal]
	PreviousNote, FollowingNote   Evidence[NodeRef]
	MakeTime                      noteTimingDelta
}
type noteTimingOrigin struct {
	Note                 NodeRef
	Part                 Evidence[NodeRef]
	Staff                Evidence[Integer]
	Voice                Evidence[string]
	ScoreOnset, ScoreEnd Evidence[musicalPosition]
	Duration             Evidence[Rational]
	Chord, Grace, Cue    Evidence[bool]
	Attack, Release      noteTimingAdjustment
	GraceTiming          Evidence[graceTimingOrigin]
	Bends                []bendTimingOrigin
	BendsComplete        Evidence[bool]
}

// Fixed documented defaults have bounded immutable representations. All
// arithmetic involving assessed values still passes through Session accounting.
var noteTimingHundred = Decimal{value: big.NewRat(100, 1), text: "100", bytes: 5}
var noteTimingFirstBeat = Decimal{value: big.NewRat(25, 1), text: "25", bytes: 4}
var noteTimingLastBeat = Decimal{value: big.NewRat(75, 1), text: "75", bytes: 4}
var noteTimingBeats = Decimal{value: big.NewRat(4, 1), text: "4", bytes: 3}

func noteTimingPresence(e Evidence[bool]) Evidence[bool] {
	if e.State == EvidenceAbsent {
		return Evidence[bool]{State: EvidenceKnown}
	}
	return e
}
func (s *Session) noteTimingDelta(ctx context.Context, raw, units Evidence[Decimal]) (noteTimingDelta, error) {
	out := noteTimingDelta{Raw: raw, Units: units, Delta: Evidence[Rational]{State: EvidenceAbsent}}
	if err := s.step(ctx, 1); err != nil {
		return out, err
	}
	if raw.State == EvidenceAbsent {
		return out, nil
	}
	var err error
	out.Delta, err = s.normalizeTimelineDuration(ctx, raw, units)
	return out, err
}
func (s *Session) noteTimingShift(ctx context.Context, origin Evidence[musicalPosition], delta Evidence[Rational]) (Evidence[musicalPosition], error) {
	if err := s.step(ctx, 1); err != nil {
		return Evidence[musicalPosition]{}, err
	}
	if delta.State != EvidenceKnown {
		return timelineGapAs[Rational, musicalPosition](delta, "note-timing-delta-unavailable"), nil
	}
	if origin.State != EvidenceKnown {
		return timelineGapAs[musicalPosition, musicalPosition](origin, "note-timing-origin-unavailable"), nil
	}
	sum, err := s.rationalAdd(ctx, origin.Value.Offset, delta.Value)
	if err != nil {
		return Evidence[musicalPosition]{}, err
	}
	out := origin
	out.Value.Offset = sum
	return out, nil
}
func (s *Session) noteTimingAdjustment(ctx context.Context, origin Evidence[musicalPosition], raw, units Evidence[Decimal], omissionUnchanged bool) (noteTimingAdjustment, error) {
	delta, err := s.noteTimingDelta(ctx, raw, units)
	out := noteTimingAdjustment{noteTimingDelta: delta, Origin: origin, Position: Evidence[musicalPosition]{State: EvidenceAbsent}}
	if err != nil {
		return out, err
	}
	if raw.State == EvidenceAbsent {
		if omissionUnchanged {
			out.Position = origin
		}
		return out, nil
	}
	out.Position, err = s.noteTimingShift(ctx, origin, delta.Delta)
	return out, err
}
func (r *stateReader) notePercent(n *factNode, role timelineRole, defaultValue Decimal, origin Evidence[musicalPosition], duration Evidence[Rational]) noteTimingPercent {
	raw := stateDecimalValue(r.attribute(n, role))
	out := noteTimingPercent{Raw: raw, Percent: raw, Origin: origin}
	if raw.State == EvidenceAbsent {
		out.Percent = Evidence[Decimal]{State: EvidenceKnown, Value: defaultValue}
	}
	if r.err != nil {
		return out
	}
	fraction, err := r.s.normalizeTimelineDuration(r.ctx, out.Percent, Evidence[Decimal]{State: EvidenceKnown, Value: noteTimingHundred})
	if err != nil {
		r.err = err
		return out
	}
	delta := duration
	if fraction.State != EvidenceKnown {
		delta = stateUnavailable[Rational, Rational](fraction)
	} else if duration.State == EvidenceKnown {
		value, e := r.s.rationalMul(r.ctx, duration.Value, fraction.Value)
		if e != nil {
			r.err = e
			return out
		}
		delta = Evidence[Rational]{State: EvidenceKnown, Value: value}
	}
	out.Position, r.err = r.s.noteTimingShift(r.ctx, origin, delta)
	return out
}
func (r *stateReader) noteBend(n *factNode, fact timelineEventFact, units Evidence[Decimal]) bendTimingOrigin {
	out := bendTimingOrigin{Bend: NodeRef{r.s.documentID, n.header.ID}, OriginalNote: fact.Position}
	release, present := r.child(n, timelineRoleBendRelease)
	out.ReleasePresent = noteTimingPresence(present)
	raw := stateUnavailable[bool, Decimal](present)
	if present.State == EvidenceKnown {
		raw = stateDecimalValue(r.attribute(release, timelineRoleBendReleaseOffset))
	}
	if r.err == nil {
		out.Release, r.err = r.s.noteTimingAdjustment(r.ctx, fact.Position, raw, units, false)
	}
	out.FirstBeat = r.notePercent(n, timelineRoleBendFirstBeat, noteTimingFirstBeat, fact.Position, fact.Duration)
	out.LastBeat = r.notePercent(n, timelineRoleBendLastBeat, noteTimingLastBeat, fact.Position, fact.Duration)
	out.RawBeats = stateDecimalValue(r.attribute(n, timelineRoleBendBeats))
	out.Beats = out.RawBeats
	if out.Beats.State == EvidenceAbsent {
		out.Beats = Evidence[Decimal]{State: EvidenceKnown, Value: noteTimingBeats}
	}
	out.RawAccelerate = stateYesNo(r.attribute(n, timelineRoleBendAccelerate))
	out.Accelerate = out.RawAccelerate
	if out.Accelerate.State == EvidenceAbsent {
		out.Accelerate = Evidence[bool]{State: EvidenceKnown}
	}
	return out
}

// readNoteOrigins consumes the same assessed nodes for source and model input.
// The caller supplies its reviewed units, including an explicit unavailable
// result at disputed mid-measure/chord boundaries; this method never selects
// another declaration to make a missing timing value look known.
func (service *timelineService) readNoteOrigins(ctx context.Context, s *Session, b *timelineBindings, note *factNode, fact timelineEventFact, units Evidence[Decimal]) error {
	r := &stateReader{ctx: ctx, s: s, b: b}
	if !r.matches(note, timelineRoleNote) {
		if r.err != nil {
			return r.err
		}
		return contract("note timing origins", "exact note declaration required")
	}
	if _, exists := service.NoteOrigins[note.header.ID]; exists {
		return contract("note timing origins", "duplicate publication")
	}
	// Admit fixed record and map storage before retaining any new collection.
	if err := s.retain(4096); err != nil {
		return err
	}
	out := noteTimingOrigin{Note: NodeRef{s.documentID, note.header.ID}, Part: fact.Part, Staff: fact.Staff, Voice: fact.Voice, ScoreOnset: fact.Position, Duration: fact.Duration, GraceTiming: Evidence[graceTimingOrigin]{State: EvidenceAbsent}}
	_, chord := r.child(note, timelineRoleNoteChord)
	out.Chord = noteTimingPresence(chord)
	grace, present := r.child(note, timelineRoleNoteGrace)
	out.Grace = noteTimingPresence(present)
	_, cue := r.child(note, timelineRoleNoteCue)
	out.Cue = noteTimingPresence(cue)
	if r.err != nil {
		return r.err
	}
	var err error
	out.ScoreEnd = fact.End
	if out.ScoreEnd.State == "" {
		out.ScoreEnd, err = s.noteTimingShift(ctx, fact.Position, fact.Duration)
		if err != nil {
			return err
		}
	}
	out.Attack, err = s.noteTimingAdjustment(ctx, out.ScoreOnset, stateDecimalValue(r.attribute(note, timelineRoleNoteAttack)), units, true)
	if err != nil {
		return err
	}
	out.Release, err = s.noteTimingAdjustment(ctx, out.ScoreEnd, stateDecimalValue(r.attribute(note, timelineRoleNoteRelease)), units, true)
	if err != nil {
		return err
	}
	if present.State == EvidenceKnown {
		g := graceTimingOrigin{Grace: NodeRef{s.documentID, grace.header.ID}, StealPrevious: stateDecimalValue(r.attribute(grace, timelineRoleGraceStealPrevious)), StealFollowing: stateDecimalValue(r.attribute(grace, timelineRoleGraceStealFollowing)), PreviousNote: stateUnknown[NodeRef]("grace-neighbor-association-unresolved"), FollowingNote: stateUnknown[NodeRef]("grace-neighbor-association-unresolved")}
		g.MakeTime, err = s.noteTimingDelta(ctx, stateDecimalValue(r.attribute(grace, timelineRoleGraceMakeTime)), units)
		if err != nil {
			return err
		}
		out.GraceTiming = Evidence[graceTimingOrigin]{State: EvidenceKnown, Value: g}
	} else {
		out.GraceTiming = stateUnavailable[bool, graceTimingOrigin](present)
	}
	notations, complete := r.children(note, timelineRoleNoteNotations)
	out.BendsComplete = complete
	for _, notation := range notations {
		technicals, complete := r.children(notation, timelineRoleNotationsTechnical)
		if complete.State != EvidenceKnown {
			out.BendsComplete = complete
			continue
		}
		for _, technical := range technicals {
			bends, complete := r.children(technical, timelineRoleTechnicalBend)
			if complete.State != EvidenceKnown {
				out.BendsComplete = complete
				continue
			}
			for _, bend := range bends {
				if !r.step(1) {
					return r.err
				}
				if err := s.retain(4096); err != nil {
					return err
				}
				value := r.noteBend(bend, fact, units)
				if r.err != nil {
					return r.err
				}
				out.Bends = append(out.Bends, value)
			}
		}
	}
	if r.err != nil {
		return r.err
	}
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	if service.NoteOrigins == nil {
		service.NoteOrigins = make(map[NodeID]noteTimingOrigin)
	}
	service.NoteOrigins[note.header.ID] = out
	return nil
}
