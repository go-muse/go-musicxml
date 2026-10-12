package validation

import "context"

const (
	contextRepeatNoteDot              contextRole = "repeat-note-dot"
	contextRepeatNoteAccidental       contextRole = "repeat-note-accidental"
	contextRepeatNoteBeam             contextRole = "repeat-note-beam"
	contextRepeatNoteLyric            contextRole = "repeat-note-lyric"
	contextRepeatNoteNotehead         contextRole = "repeat-note-notehead"
	contextRepeatNoteTimeModification contextRole = "repeat-note-time-modification"
	contextRepeatNoteNotations        contextRole = "repeat-note-notations"
	contextRepeatNoteTie              contextRole = "repeat-note-tie"
	contextRepeatNoteGrace            contextRole = "repeat-note-grace"
	contextRepeatNotationSlur         contextRole = "repeat-notation-slur"
	contextRepeatNotationTied         contextRole = "repeat-notation-tied"
	contextRepeatCursorBackup         contextRole = "repeat-cursor-backup"
	contextRepeatAnyNote              contextRole = "repeat-any-note"
	contextRepeatNotePitch            contextRole = "repeat-note-pitch"
	contextRepeatNoteRest             contextRole = "repeat-note-rest"
	contextRepeatNoteUnpitched        contextRole = "repeat-note-unpitched"
	contextRepeatNoteDuration         contextRole = "repeat-note-duration"
	contextRepeatNoteChord            contextRole = "repeat-note-chord"
	contextRepeatNoteVoice            contextRole = "repeat-note-voice"
	contextRepeatNoteStaff            contextRole = "repeat-note-staff"
	contextRepeatNoteType             contextRole = "repeat-note-type"
	contextRepeatNoteStem             contextRole = "repeat-note-stem"
	contextRepeatNoteInstrument       contextRole = "repeat-note-instrument"
	contextRepeatInstrumentID         contextRole = "repeat-instrument-id"
	contextRepeatPitchStep            contextRole = "repeat-pitch-step"
	contextRepeatPitchAlter           contextRole = "repeat-pitch-alter"
	contextRepeatPitchOctave          contextRole = "repeat-pitch-octave"
	contextRepeatUnpitchedStep        contextRole = "repeat-unpitched-step"
	contextRepeatUnpitchedOctave      contextRole = "repeat-unpitched-octave"
)

func contextRepeatContentBindingSpecs() []contextBindingSpec {
	return []contextBindingSpec{
		{Role: contextRepeatCursorBackup, Name: "backup", Type: "backup", Count: -1},
		{Role: contextRepeatNoteDot, Owner: "note", Name: "dot", Type: "empty-placement", Count: 1},
		{Role: contextRepeatNoteAccidental, Owner: "note", Name: "accidental", Type: "accidental", Count: 1},
		{Role: contextRepeatNoteBeam, Owner: "note", Name: "beam", Type: "beam", Count: 1},
		{Role: contextRepeatNoteLyric, Owner: "note", Name: "lyric", Type: "lyric", Count: 1},
		{Role: contextRepeatNoteNotehead, Owner: "note", Name: "notehead", Type: "notehead", Count: 1},
		{Role: contextRepeatNoteTimeModification, Owner: "note", Name: "time-modification", Type: "time-modification", Count: 1},
		{Role: contextRepeatNoteNotations, Owner: "note", Name: "notations", Type: "notations", Count: 1},
		{Role: contextRepeatNoteTie, Owner: "note", Name: "tie", Type: "tie", Count: 2},
		{Role: contextRepeatNoteGrace, Owner: "note", Name: "grace", Type: "grace", Count: 1},
		{Role: contextRepeatNotationSlur, Owner: "notations", Name: "slur", Type: "slur", Count: 1},
		{Role: contextRepeatNotationTied, Owner: "notations", Name: "tied", Type: "tied", Count: 1},
		{Role: contextRepeatAnyNote, Name: "note", Type: "note", Count: -1},
		{Role: contextRepeatNotePitch, Owner: "note", Name: "pitch", Type: "pitch", Count: 4},
		{Role: contextRepeatNoteRest, Owner: "note", Name: "rest", Type: "rest", Count: 4},
		{Role: contextRepeatNoteUnpitched, Owner: "note", Name: "unpitched", Type: "unpitched", Count: 4},
		{Role: contextRepeatNoteDuration, Owner: "note", Name: "duration", Type: "positive-divisions", Count: 2},
		{Role: contextRepeatNoteChord, Owner: "note", Name: "chord", Type: "empty", Count: 4},
		{Role: contextRepeatNoteVoice, Owner: "note", Name: "voice", Type: "xs:string", Count: 1},
		{Role: contextRepeatNoteStaff, Owner: "note", Name: "staff", Type: "xs:positiveInteger", Count: 1},
		{Role: contextRepeatNoteType, Owner: "note", Name: "type", Type: "note-type", Count: 1},
		{Role: contextRepeatNoteStem, Owner: "note", Name: "stem", Type: "stem", Count: 1},
		{Role: contextRepeatNoteInstrument, Owner: "note", Name: "instrument", Type: "instrument", Count: 1},
		{Role: contextRepeatInstrumentID, Owner: "instrument", Name: "id", Type: "xs:IDREF", Attribute: true, Count: 1},
		{Role: contextRepeatPitchStep, Owner: "pitch", Name: "step", Type: "step", Count: 1},
		{Role: contextRepeatPitchAlter, Owner: "pitch", Name: "alter", Type: "semitones", Count: 1},
		{Role: contextRepeatPitchOctave, Owner: "pitch", Name: "octave", Type: "octave", Count: 1},
		{Role: contextRepeatUnpitchedStep, Owner: "unpitched", Name: "display-step", Type: "step", Count: 1},
		{Role: contextRepeatUnpitchedOctave, Owner: "unpitched", Name: "display-octave", Type: "octave", Count: 1},
	}
}

type contextRepeatNote struct {
	source     NodeRef
	part       Evidence[NodeRef]
	pos        musicalPosition
	duration   Rational
	chord      bool
	staff      Evidence[Integer]
	voice      Evidence[string]
	rest       bool
	unpitched  bool
	step       Evidence[string]
	alter      Evidence[Decimal]
	octave     Evidence[Integer]
	instrument Evidence[string]
	available  Evidence[bool]
}

// This invariant presence index is scoped to one content-proof construction.
// Borrowed part identities retain exact document identity and bounded hash work.
func contextRepeatTransformParts(ctx context.Context, s *Session, timeline *timelineService) (map[NodeRef]bool, func(), error) {
	release, err := contextRepeatIndexReserve(ctx, s, len(timeline.State.Transpose.Publications), len(timeline.State.Transpose.Gaps))
	if err != nil {
		return nil, nil, err
	}
	out := make(map[NodeRef]bool)
	for _, p := range timeline.State.Transpose.Publications {
		if err = s.step(ctx, 1+int64(len(p.Position.Part.DocumentID))); err != nil {
			release()
			return nil, nil, err
		}
		out[p.Position.Part] = true
	}
	for _, g := range timeline.State.Transpose.Gaps {
		if err = s.step(ctx, 1+int64(len(g.Part.DocumentID))); err != nil {
			release()
			return nil, nil, err
		}
		out[g.Part] = true
	}
	return out, release, nil
}

func contextRepeatReadNote(r *contextReader, timeline *timelineService, transformed map[NodeRef]bool, id NodeID) (contextRepeatNote, error) {
	note := contextRepeatNote{source: NodeRef{r.s.documentID, id}, available: navigationKnownBool(true)}
	node := r.s.nodes[id]
	fact, ok := timeline.Facts[id]
	if !ok {
		note.available = stateUnknown[bool]("context-repeat-note-timing-unavailable")
		return note, nil
	}
	note.part = fact.Part
	if _, present := r.child(node, contextRepeatNoteGrace); present.State == EvidenceKnown {
		note.available = stateUnknown[bool]("context-repeat-grace-content-unresolved")
		return note, r.err
	}
	for _, v := range []Evidence[bool]{stateUnavailable[musicalPosition, bool](fact.Position), stateUnavailable[Rational, bool](fact.Duration)} {
		if v.State != EvidenceKnown {
			note.available = stateMergeGap(note.available, v)
		}
	}
	if note.available.State != EvidenceKnown {
		return note, nil
	}
	note.pos, note.duration, note.staff, note.voice = fact.Position.Value, fact.Duration.Value, fact.Staff, fact.Voice
	note.available = stateMergeGap(note.available, r.complete(node))
	playback := timeline.Playback.Notes[id]
	if playback.Chord.State != EvidenceKnown {
		note.available = stateMergeGap(note.available, stateUnavailable[bool, bool](playback.Chord))
	} else {
		note.chord = playback.Chord.Value
	}
	// Pass-qualified notes are not a universally equivalent unqualified pattern.
	if playback.TimeOnly.State != EvidenceAbsent {
		note.available = stateMergeGap(note.available, stateUnknown[bool]("context-repeat-pass-content-unresolved"))
	}
	// Intended duration is authoritative for dots and time-modification;
	// engraving does not alter the pitch/rhythm comparison. Sound ties and
	// performance-bearing notation are a separate unresolved musical domain.
	for _, ch := range node.children {
		allowed := false
		for _, role := range []contextRole{contextRepeatNotePitch, contextRepeatNoteRest, contextRepeatNoteUnpitched, contextRepeatNoteDuration, contextRepeatNoteChord, contextRepeatNoteVoice, contextRepeatNoteStaff, contextRepeatNoteType, contextRepeatNoteStem, contextRepeatNoteInstrument, contextRepeatNoteDot, contextRepeatNoteAccidental, contextRepeatNoteBeam, contextRepeatNoteLyric, contextRepeatNoteNotehead, contextRepeatNoteTimeModification, contextRepeatNoteNotations} {
			if r.matches(ch, role) {
				allowed = true
				break
			}
		}
		if r.matches(ch, contextRepeatNoteTie) {
			note.available = stateMergeGap(note.available, stateUnknown[bool]("context-repeat-playback-tie-unresolved"))
			allowed = true
		}
		if r.matches(ch, contextRepeatNoteNotations) {
			for _, mark := range ch.children {
				if !r.matches(mark, contextRepeatNotationSlur) && !r.matches(mark, contextRepeatNotationTied) {
					note.available = stateMergeGap(note.available, stateUnknown[bool]("context-repeat-performance-notation-unresolved"))
				}
			}
		}
		if r.err != nil {
			return note, r.err
		}
		if !allowed {
			note.available = stateMergeGap(note.available, stateUnknown[bool]("context-repeat-note-equivalence-domain-unresolved"))
		}
	}
	pitch, pp := r.child(node, contextRepeatNotePitch)
	rest, rp := r.child(node, contextRepeatNoteRest)
	unpitched, up := r.child(node, contextRepeatNoteUnpitched)
	if pp.State == EvidenceKnown {
		note.step = stateText(r.childScalar(pitch, contextRepeatPitchStep))
		note.alter = stateDecimalValue(r.childScalar(pitch, contextRepeatPitchAlter))
		note.octave = stateInteger(r.childScalar(pitch, contextRepeatPitchOctave))
	} else if rp.State == EvidenceKnown {
		note.rest = rest != nil
	} else if up.State == EvidenceKnown {
		note.unpitched = unpitched != nil
		note.step = stateText(r.childScalar(unpitched, contextRepeatUnpitchedStep))
		note.octave = stateInteger(r.childScalar(unpitched, contextRepeatUnpitchedOctave))
	} else {
		note.available = stateMergeGap(note.available, stateUnknown[bool]("context-repeat-pitch-content-unavailable"))
	}
	instrument, ip := r.child(node, contextRepeatNoteInstrument)
	if ip.State == EvidenceKnown {
		note.instrument = stateText(r.attribute(instrument, contextRepeatInstrumentID))
	} else {
		note.instrument = stateUnavailable[bool, string](ip)
	}
	// Every selected field must be known or independently known absent.
	for _, v := range []Evidence[bool]{stateUnavailable[Integer, bool](note.staff), stateUnavailable[string, bool](note.voice), stateUnavailable[string, bool](note.step), stateUnavailable[Decimal, bool](note.alter), stateUnavailable[Integer, bool](note.octave), stateUnavailable[string, bool](note.instrument)} {
		if v.State != "" && v.State != EvidenceKnown && v.State != EvidenceAbsent {
			note.available = stateMergeGap(note.available, v)
		}
	}
	// A known absent encoded transformation is independent of a missing staff.
	// Do not call a staff-specific state selector merely to prove an empty history.
	if !r.step(1 + int64(len(note.pos.Part.DocumentID))) {
		return note, r.err
	}
	if transformed[note.pos.Part] {
		note.available = stateMergeGap(note.available, stateUnknown[bool]("context-repeat-transposed-equivalence-unresolved"))
	}

	return note, r.err
}

func contextRepeatNoteEqual(ctx context.Context, s *Session, a, b contextRepeatNote) (bool, error) {
	if err := s.step(ctx, 1); err != nil {
		return false, err
	}
	if a.available.State != EvidenceKnown || b.available.State != EvidenceKnown {
		return false, nil
	}
	if a.chord != b.chord || a.rest != b.rest || a.unpitched != b.unpitched {
		return false, nil
	}
	for _, pair := range [][2]Evidence[string]{{a.voice, b.voice}, {a.step, b.step}, {a.instrument, b.instrument}} {
		if pair[0].State != pair[1].State {
			return false, nil
		}
		if pair[0].State == EvidenceKnown {
			if err := s.step(ctx, int64(len(pair[0].Value)+len(pair[1].Value))); err != nil {
				return false, err
			}
			if pair[0].Value != pair[1].Value {
				return false, nil
			}
		}
	}
	for _, pair := range [][2]Evidence[Integer]{{a.staff, b.staff}, {a.octave, b.octave}} {
		if pair[0].State != pair[1].State {
			return false, nil
		}
		if pair[0].State == EvidenceKnown {
			if err := s.step(ctx, pair[0].Value.ByteSize()+pair[1].Value.ByteSize()); err != nil {
				return false, err
			}
			cmp, e := pair[0].Value.Cmp(pair[1].Value)
			if e != nil || cmp != 0 {
				return false, e
			}
		}
	}
	// Pitch omission and an explicit zero denote the same natural pitch.
	// This comparison does not rewrite either raw evidence field.
	aa, ba := a.alter, b.alter
	for _, v := range []Evidence[Decimal]{aa, ba} {
		if v.State == EvidenceKnown {
			if err := s.step(ctx, v.Value.ByteSize()); err != nil {
				return false, err
			}
		}
	}
	if aa.State == EvidenceAbsent && ba.State == EvidenceKnown && ba.Value.Valid() && ba.Value.value.Sign() == 0 {
		aa = ba
	}
	if ba.State == EvidenceAbsent && aa.State == EvidenceKnown && aa.Value.Valid() && aa.Value.value.Sign() == 0 {
		ba = aa
	}
	if aa.State != ba.State {
		return false, nil
	}
	if aa.State == EvidenceKnown {
		cmp, e := aa.Value.Cmp(ba.Value)
		if e != nil || cmp != 0 {
			return false, e
		}
	}
	cmp, err := s.rationalCmp(ctx, a.pos.Offset, b.pos.Offset)
	if err != nil || cmp != 0 {
		return false, err
	}
	cmp, err = s.rationalCmp(ctx, a.duration, b.duration)
	return cmp == 0, err
}

func contextRepeatProveContent(ctx context.Context, s *Session, p *contextPlan, timeline *timelineService, rows []contextRepeatRecord) error {
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	transformed, releaseTransforms, err := contextRepeatTransformParts(ctx, s, timeline)
	if err != nil {
		return err
	}
	defer releaseTransforms()
	freeNotes, err := contextRepeatScratch(ctx, s, len(timeline.Playback.Notes), 2048)
	if err != nil {
		return err
	}
	defer freeNotes()
	notes := make([]contextRepeatNote, 0, len(timeline.Playback.Notes))
	r := &contextReader{ctx: ctx, s: s, b: p.Bindings}
	index, freeIndex, err := contextRepeatBuildProofIndex(r, timeline, rows)
	if err != nil {
		return err
	}
	defer freeIndex()
	for _, id := range s.sequence {
		if !r.step(1) {
			return r.err
		}
		if _, ok := timeline.Playback.Notes[id]; !ok {
			continue
		}
		note, err := contextRepeatReadNote(r, timeline, transformed, id)
		if err != nil {
			return err
		}
		notes = append(notes, note)
	}
	roster, releaseRoster, err := contextRepeatBuildContentRoster(ctx, s, notes)
	if err != nil {
		return err
	}
	defer releaseRoster()
	for i := range rows {
		if err := s.step(ctx, 1+int64(len(rows[i].value.Part.Value.DocumentID))); err != nil {
			return err
		}
		if err := contextRepeatProveRosterRow(ctx, s, timeline, roster, rows, &rows[i], index.partCoverage(rows[i].value.Part), index); err != nil {
			return err
		}
	}
	return r.err
}

func contextRepeatProveRosterRow(ctx context.Context, s *Session, timeline *timelineService, roster *contextRepeatContentRoster, rows []contextRepeatRecord, row *contextRepeatRecord, coverage Evidence[bool], indexes ...*contextRepeatProofIndex) error {
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	row.value.NoteScopesComplete = coverage
	if (row.value.Kind != RepeatStyleBeat && row.value.Kind != RepeatStyleMeasure) || row.value.Type.State != EvidenceKnown || row.value.Type.Value != "start" {
		return nil
	}
	if coverage.State != EvidenceKnown || !coverage.Value {
		row.value.ContentEquivalent = coverage
		return nil
	}
	if row.value.Part.State != EvidenceKnown {
		row.value.ContentEquivalent = stateUnavailable[NodeRef, bool](row.value.Part)
		if row.value.ContentEquivalent.State == EvidenceAbsent {
			row.value.ContentEquivalent = stateUnknown[bool]("context-repeat-part-unavailable")
		}
		return nil
	}
	var notes []contextRepeatNote
	var release func()
	var err error
	if len(indexes) != 0 {
		notes, release, err = indexes[0].cachedPart(ctx, s, roster, row.value.Part.Value)
	} else {
		notes, release, err = roster.partNotes(ctx, s, row.value.Part.Value)
	}
	if err != nil {
		return err
	}
	defer release()
	return contextRepeatProveRow(ctx, s, timeline, notes, rows, row, coverage, indexes...)
}

func contextRepeatProveRow(ctx context.Context, s *Session, timeline *timelineService, notes []contextRepeatNote, rows []contextRepeatRecord, row *contextRepeatRecord, coverage Evidence[bool], indexes ...*contextRepeatProofIndex) error {
	row.value.NoteScopesComplete = coverage
	if (row.value.Kind != RepeatStyleBeat && row.value.Kind != RepeatStyleMeasure) || row.value.Type.State != EvidenceKnown || row.value.Type.Value != "start" {
		return nil
	}
	if coverage.State != EvidenceKnown || !coverage.Value {
		row.value.ContentEquivalent = coverage
		return nil
	}
	var scope *contextRepeatProofScope
	var release func()
	var err error
	if len(indexes) != 0 {
		scope, release, err = indexes[0].scope(ctx, s, notes, row.value)
	} else {
		selected, status, free, e := contextRepeatScopedNotes(ctx, s, notes, row.value)
		scope = &contextRepeatProofScope{notes: selected, complete: status}
		release, err = free, e
	}
	if err != nil {
		return err
	}
	defer release()
	if scope.complete.State != EvidenceKnown || !scope.complete.Value {
		row.value.ContentEquivalent = scope.complete
		return nil
	}
	notes = scope.notes
	if err := contextRepeatBeatProof(ctx, s, notes, rows, row, indexes...); err != nil {
		return err
	}
	if row.value.Kind != RepeatStyleMeasure || row.position.State != EvidenceKnown || row.value.PatternMeasures.State != EvidenceKnown || row.value.Complete.State != EvidenceKnown || !row.value.Complete.Value {
		return nil
	}
	start := row.position.Value
	if start.Offset.value.Sign() != 0 {
		return nil
	}
	pattern := row.value.PatternMeasures.Value
	cmp, err := navigationCountCompare(ctx, s, pattern, start.MeasureOrdinal)
	if err != nil {
		return err
	}
	if cmp > 0 || pattern.value.Sign() <= 0 {
		return nil
	}
	count := pattern.value.Uint64()
	var end uint64
	endKnown := false
	if row.value.Stop.State == EvidenceKnown {
		candidates := rows
		if len(indexes) != 0 {
			if err := s.step(ctx, 1+int64(len(row.value.Stop.Value.DocumentID))); err != nil {
				return err
			}
			candidates = indexes[0].stopRows(rows, row.value.Stop.Value)
		}
		for _, other := range candidates {
			if err = s.step(ctx, 1); err != nil {
				return err
			}
			if other.source == row.value.Stop.Value && other.position.State == EvidenceKnown && other.position.Value.Offset.value.Sign() == 0 {
				end = other.position.Value.MeasureOrdinal
				endKnown = true
			}
		}
	} else if row.value.Stop.State == EvidenceAbsent {
		closures := timeline.State.Closures
		if len(indexes) != 0 {
			if c, ok := indexes[0].closures[start.Part]; ok {
				closures = []stateClosure{c}
			} else {
				closures = nil
			}
		}
		for _, closure := range closures {
			if err = s.step(ctx, 1); err != nil {
				return err
			}
			if closure.Part == start.Part && closure.Complete.State == EvidenceKnown && closure.Complete.Value {
				if closure.Through == ^uint64(0) {
					return nil
				}
				end = closure.Through + 1
				endKnown = true
			}
		}
	}
	if !endKnown || end <= start.MeasureOrdinal {
		return nil
	}
	buckets, complete := scope.buckets, scope.complete
	if buckets == nil {
		var free func()
		buckets, complete, free, err = contextRepeatBuildMeasureRoster(ctx, s, notes, start.Part)
		if err != nil {
			return err
		}
		defer free()
	}
	if complete.State != EvidenceKnown || !complete.Value {
		row.value.ContentEquivalent = complete
		return nil
	}
	row.value.ContentEquivalent = navigationKnownBool(true)
	for measure := start.MeasureOrdinal; measure < end; measure++ {
		if err = s.step(ctx, 1); err != nil {
			return err
		}
		source := start.MeasureOrdinal - count + (measure-start.MeasureOrdinal)%count
		same, e := contextRepeatSelectedMeasureEqual(ctx, s, buckets[source], buckets[measure])
		if e != nil {
			return e
		}
		if same.State != EvidenceKnown {
			empty, e := contextRepeatEmptyMeasure(ctx, s, timeline, notes, start.Part, source, measure, indexes...)
			if e != nil {
				return e
			}
			if empty.State == EvidenceKnown && !empty.Value {
				same = empty
			}
		}
		if same.State == EvidenceKnown && !same.Value {
			row.value.ContentEquivalent = same
			break
		}
		row.value.ContentEquivalent = stateMergeGap(row.value.ContentEquivalent, same)
		if same.State != EvidenceKnown {
			break
		}
	}
	return nil
}

// Scoped notes are borrowed shallow values, never a second public projection.
// Exact staff identity can exclude unrelated notes; absent/invalid membership
// remains unavailable and cannot prove an empty target or content equivalence.
func contextRepeatScopedNotes(ctx context.Context, s *Session, notes []contextRepeatNote, row RepeatContextResult) ([]contextRepeatNote, Evidence[bool], func(), error) {
	noop := func() {}
	if err := s.step(ctx, 1); err != nil {
		return nil, Evidence[bool]{}, noop, err
	}
	all := row.AllStaves.State == EvidenceKnown && row.AllStaves.Value
	if all && len(row.ExceptVoices) == 0 {
		return notes, navigationKnownBool(true), noop, nil
	}
	if row.AllStaves.State != EvidenceKnown {
		return nil, row.AllStaves, noop, nil
	}
	if !all && row.Staff.State != EvidenceKnown {
		return nil, stateUnavailable[Integer, bool](row.Staff), noop, nil
	}
	if row.Part.State != EvidenceKnown {
		return nil, stateUnavailable[NodeRef, bool](row.Part), noop, nil
	}
	voices, freeVoices, err := contextRepeatBuildVoiceFilter(ctx, s, row.ExceptVoices)
	if err != nil {
		return nil, Evidence[bool]{}, noop, err
	}
	defer freeVoices()
	bytes, err := rationalCheckedMul(BudgetBufferedBytes, int64(len(notes)), 2048)
	if err != nil {
		return nil, Evidence[bool]{}, noop, err
	}
	if _, err = AllocationSize(bytes); err != nil {
		return nil, Evidence[bool]{}, noop, err
	}
	if err = s.account.Charge(BudgetBufferedBytes, bytes); err != nil {
		return nil, Evidence[bool]{}, noop, err
	}
	release := func() { _ = s.account.Release(BudgetBufferedBytes, bytes) }
	out := make([]contextRepeatNote, 0, len(notes))
	status := navigationKnownBool(true)
	for _, note := range notes {
		if err = s.step(ctx, 1+int64(len(note.part.Value.DocumentID)+len(row.Part.Value.DocumentID))); err != nil {
			release()
			return nil, Evidence[bool]{}, noop, err
		}
		if note.part.State != EvidenceKnown {
			status = stateMergeGap(status, stateUnavailable[NodeRef, bool](note.part))
			continue
		}
		if note.part.Value != row.Part.Value {
			continue
		}
		member := navigationKnownBool(true)
		if !all {
			if note.staff.State != EvidenceKnown {
				member = stateUnavailable[Integer, bool](note.staff)
				if member.State == EvidenceAbsent {
					member = stateUnknown[bool]("context-repeat-note-staff-unavailable")
				}
			} else {
				if err = s.step(ctx, note.staff.Value.ByteSize()+row.Staff.Value.ByteSize()); err != nil {
					release()
					return nil, Evidence[bool]{}, noop, err
				}
				cmp, e := note.staff.Value.Cmp(row.Staff.Value)
				if e != nil {
					release()
					return nil, Evidence[bool]{}, noop, e
				}
				if cmp != 0 {
					continue
				}
			}
		}
		if len(row.ExceptVoices) != 0 {
			applies, e := contextRepeatVoiceApplies(ctx, s, voices, note.voice)
			if e != nil {
				release()
				return nil, Evidence[bool]{}, noop, e
			}
			if applies.State == EvidenceKnown && !applies.Value {
				continue
			}
			member = stateMergeGap(member, applies)
		}

		if member.State != EvidenceKnown || !member.Value {
			status = stateMergeGap(status, member)
			continue
		}
		out = append(out, note)
	}
	return out, status, release, nil
}

func contextRepeatWindow(ctx context.Context, s *Session, notes []contextRepeatNote, part NodeRef, measure uint64, from, to Rational) (out []contextRepeatNote, status Evidence[bool], release func(), err error) {
	release, err = contextRepeatScratch(ctx, s, len(notes), 2048)
	if err != nil {
		return nil, Evidence[bool]{}, func() {}, err
	}
	defer func() {
		if err != nil {
			release()
			release = func() {}
		}
	}()
	status = navigationKnownBool(true)
	for _, note := range notes {
		if err := s.step(ctx, 1); err != nil {
			return nil, Evidence[bool]{}, release, err
		}
		if note.part.State != EvidenceKnown {
			status = stateMergeGap(status, note.available)
			continue
		}
		if note.part.Value != part {
			continue
		}
		if !note.pos.Offset.Valid() {
			status = stateMergeGap(status, note.available)
			continue
		}
		if note.pos.MeasureOrdinal != measure {
			continue
		}
		before, e := s.rationalCmp(ctx, note.pos.Offset, from)
		if e != nil {
			return nil, Evidence[bool]{}, release, e
		}
		after, e := s.rationalCmp(ctx, note.pos.Offset, to)
		if e != nil {
			return nil, Evidence[bool]{}, release, e
		}
		if after >= 0 {
			continue
		}
		end, e := s.rationalAdd(ctx, note.pos.Offset, note.duration)
		if e != nil {
			return nil, Evidence[bool]{}, release, e
		}
		endBefore, e := s.rationalCmp(ctx, end, from)
		if e != nil {
			return nil, Evidence[bool]{}, release, e
		}
		if endBefore <= 0 {
			continue
		}
		endAfter, e := s.rationalCmp(ctx, end, to)
		if e != nil {
			return nil, Evidence[bool]{}, release, e
		}
		if before < 0 || endAfter > 0 {
			status = stateMergeGap(status, stateUnknown[bool]("context-repeat-cross-boundary-event-unresolved"))
			continue
		}
		if note.available.State != EvidenceKnown {
			status = stateMergeGap(status, note.available)
			continue
		}
		note.pos.Offset, e = s.rationalSub(ctx, note.pos.Offset, from)
		if e != nil {
			return nil, Evidence[bool]{}, release, e
		}
		out = append(out, note)
	}
	return out, status, release, nil
}

func contextRepeatBeatProof(ctx context.Context, s *Session, notes []contextRepeatNote, rows []contextRepeatRecord, row *contextRepeatRecord, indexes ...*contextRepeatProofIndex) error {
	if row.value.Kind != RepeatStyleBeat || row.value.Type.State != EvidenceKnown || row.value.Type.Value != "start" || row.position.State != EvidenceKnown || row.value.Beat.State != EvidenceKnown {
		return nil
	}
	var end Evidence[musicalPosition]
	if row.value.Stop.State == EvidenceKnown {
		candidates := rows
		if len(indexes) != 0 {
			if err := s.step(ctx, 1+int64(len(row.value.Stop.Value.DocumentID))); err != nil {
				return err
			}
			candidates = indexes[0].stopRows(rows, row.value.Stop.Value)
		}
		for i := range candidates {
			if err := s.step(ctx, 1); err != nil {
				return err
			}
			if candidates[i].source == row.value.Stop.Value {
				end = candidates[i].position
			}
		}
	} else if row.value.Stop.State == EvidenceAbsent && len(indexes) != 0 && row.value.Complete.State == EvidenceKnown && row.value.Complete.Value {
		end = indexes[0].partEnd(row.position.Value)
	}
	if end.State != EvidenceKnown {
		row.value.ContentEquivalent = stateUnavailable[musicalPosition, bool](end)
		if end.State == "" || end.State == EvidenceAbsent {
			row.value.ContentEquivalent = stateUnknown[bool]("context-repeat-beat-measure-local-boundary-unresolved")
		}
		return nil
	}
	if end.Value.Measure != row.position.Value.Measure {
		row.value.ContentEquivalent = stateUnknown[bool]("context-repeat-beat-measure-local-boundary-unresolved")
		return nil
	}
	origin := row.position.Value
	beat := row.value.Beat.Value
	sourceStart, err := s.rationalSub(ctx, origin.Offset, beat)
	if err != nil {
		return err
	}
	if sourceStart.value.Sign() < 0 {
		row.value.ContentEquivalent = stateUnknown[bool]("context-repeat-beat-measure-local-source-unresolved")
		return nil
	}
	var windows *contextRepeatWindowIndex
	var releaseWindows func()
	if len(indexes) != 0 {
		windows, releaseWindows, err = indexes[0].windowIndex(ctx, s, notes, row.value, origin)
	} else {
		windows, releaseWindows, err = contextRepeatBuildWindowIndex(ctx, s, notes, origin.Part, origin.MeasureOrdinal)
	}
	if err != nil {
		return err
	}
	defer releaseWindows()
	source, status, freeSource, err := windows.window(ctx, s, sourceStart, origin.Offset)
	if err != nil {
		return err
	}
	defer freeSource()
	if status.State != EvidenceKnown {
		row.value.ContentEquivalent = status
		return nil
	}
	if len(source) == 0 {
		return nil
	}
	row.value.ContentEquivalent = navigationKnownBool(true)
	for from := origin.Offset; ; {
		cmp, err := s.rationalCmp(ctx, from, end.Value.Offset)
		if err != nil {
			return err
		}
		if cmp == 0 {
			return nil
		}
		if cmp > 0 {
			row.value.ContentEquivalent = stateUnknown[bool]("context-repeat-nonintegral-beat-range")
			return nil
		}
		to, err := s.rationalAdd(ctx, from, beat)
		if err != nil {
			return err
		}
		cmp, err = s.rationalCmp(ctx, to, end.Value.Offset)
		if err != nil {
			return err
		}
		if cmp > 0 {
			row.value.ContentEquivalent = stateUnknown[bool]("context-repeat-nonintegral-beat-range")
			return nil
		}
		target, status, freeTarget, err := windows.window(ctx, s, from, to)
		if err != nil {
			return err
		}
		same, compareErr := func() (Evidence[bool], error) {
			defer freeTarget()
			if status.State != EvidenceKnown {
				return status, nil
			}
			if len(target) == 0 {
				for _, note := range source {
					if err := s.step(ctx, 1); err != nil {
						return Evidence[bool]{}, err
					}
					if !note.rest {
						return navigationKnownBool(false), nil
					}
				}
			}
			return contextRepeatSelectedMeasureEqual(ctx, s, source, target)
		}()
		if compareErr != nil {
			return compareErr
		}
		if same.State != EvidenceKnown || !same.Value {
			row.value.ContentEquivalent = same
			return nil
		}
		from = to
	}

}

// An empty measure is a counterexample only when all earlier sounding note
// endpoints are independently proved before its origin. A terminal note end,
// nominal meter, printed barline or missing note count alone is insufficient.
func contextRepeatEmptyMeasure(ctx context.Context, s *Session, timeline *timelineService, notes []contextRepeatNote, part NodeRef, source, target uint64, indexes ...*contextRepeatProofIndex) (Evidence[bool], error) {
	sourceSound := false
	var origin musicalPosition
	originKnown := false
	if len(indexes) != 0 {
		if p, ok := indexes[0].origins[contextRepeatMeasureKey{part, target}]; ok {
			origin, originKnown = p, true
		}
	}
	sequence := s.sequence
	if len(indexes) != 0 {
		sequence = nil
	}
	for _, id := range sequence {
		fact := timeline.Facts[id]
		if err := s.step(ctx, 1); err != nil {
			return Evidence[bool]{}, err
		}
		if fact.Origin.State == EvidenceKnown && fact.Origin.Value.Part == part && fact.Origin.Value.MeasureOrdinal == target && fact.Origin.Value.Offset.value.Sign() == 0 {
			origin = fact.Origin.Value
			originKnown = true
			break
		}
	}
	if !originKnown {
		return stateUnknown[bool]("context-repeat-empty-coverage-unresolved"), nil
	}
	for _, note := range notes {
		if err := s.step(ctx, 1); err != nil {
			return Evidence[bool]{}, err
		}
		if note.part.State != EvidenceKnown {
			return stateUnknown[bool]("context-repeat-empty-coverage-unresolved"), nil
		}
		if note.part.Value != part {
			continue
		}
		if note.available.State != EvidenceKnown || !note.pos.Offset.Valid() {
			return note.available, nil
		}
		if note.pos.UnprovedFrame {
			member, e := timeline.State.frameMember(ctx, s, note.pos)
			if e != nil {
				return Evidence[bool]{}, e
			}
			if !member {
				return stateUnknown[bool]("context-repeat-empty-coverage-unresolved"), nil
			}
		}
		if note.pos.MeasureOrdinal == target {
			return stateUnknown[bool]("context-repeat-content-equivalence-unresolved"), nil
		}
		if note.pos.MeasureOrdinal == source && !note.rest {
			sourceSound = true
		}
		if note.rest || note.pos.MeasureOrdinal > target {
			continue
		}
		fact := timeline.Facts[note.source.Node]
		if fact.End.State != EvidenceKnown {
			return stateUnavailable[musicalPosition, bool](fact.End), nil
		}
		order, err := timeline.State.positionOrder(ctx, s, fact.End.Value, origin)
		if err != nil {
			return Evidence[bool]{}, err
		}
		if order&stateAfter != 0 {
			return stateUnknown[bool]("context-repeat-empty-coverage-unresolved"), nil
		}
	}
	if !sourceSound {
		return stateUnknown[bool]("context-repeat-source-content-unavailable"), nil
	}
	return navigationKnownBool(false), nil
}
