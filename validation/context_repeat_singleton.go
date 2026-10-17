package validation

import "context"

// Accumulate work in the budget width, before any native-int addition.
// In particular, repeated borrowed document identities need not fit one host
// allocation even though each individual string does.
func contextRepeatSingletonWork(ctx context.Context, s *Session, counts ...int64) error {
	work, err := rationalSum(BudgetRuleSteps, counts...)
	if err != nil {
		return err
	}
	return s.step(ctx, work)
}

// This is a necessary singleton-event counterexample, independent of the
// sufficient representation equality above. Its deliberately closed domain is
// ordinary explicit note/attributes data throughout one canonical part, one
// explicit staff-qualified transpose publication, and independently contained
// onsets and ends. It is about encoded pitch, never acoustic realization.
type contextRepeatSingletonNote struct {
	source     NodeRef
	pos, end   musicalPosition
	origin     musicalPosition
	duration   Rational
	staff      Integer
	voice      string
	instrument NodeRef
	pitch      Rational
	rest       bool
}
type contextRepeatSingletonPart struct {
	complete   Evidence[bool]
	notes      []contextRepeatSingletonNote
	measures   map[uint64][]contextRepeatSingletonNote
	transpose  *statePublication[transposeState]
	transforms int
}

// Borrowed maps/records are construction-local. All maps and slice backing
// have one admitted owner; derived numbers use the Session arithmetic service.
// The 1024-byte/node envelope includes the exactly sized event-capacity note
// slice and per-measure append slices (old plus new capacity), with map growth.
// Every admitted ordinary rest has at least six nodes: note, rest, duration,
// instrument, voice and staff; a pitched note has at least eight. This leaves
// ample headroom for both copies and their growth even on 64-bit hosts. Each
// non-note event contributes its own node allowance to the event-capacity slice.
// Instrument-map storage is separately prepaid per NoteInstruments record;
// part-map records, map growth and pending evidence are prepaid per repeat row.
// TestIndependentSingletonRepeatStorageWidths guards these concrete shapes.
func contextRepeatProveSingletons(r *contextReader, timeline *timelineService, rows []contextRepeatRecord, index *contextRepeatProofIndex, plan *timelinePlan) error {
	if !r.step(1) {
		return r.err
	}
	if plan == nil || plan.Bindings == nil {
		return nil
	}
	needed := false
	for i := range rows {
		if !r.step(1) {
			return r.err
		}
		if contextRepeatSingletonCandidate(rows[i]) {
			needed = true
			break
		}
	}
	if !needed {
		return nil
	}
	count, err := rationalSum(BudgetBufferedBytes, int64(len(r.s.sequence)), int64(len(timeline.State.NoteInstruments)), int64(len(rows)))
	if err != nil {
		return err
	}
	capacity, err := AllocationSize(count)
	if err != nil {
		return err
	}
	free, err := contextRepeatScratch(r.ctx, r.s, capacity, 1024)
	if err != nil {
		return err
	}
	defer free()
	parts := make(map[NodeRef]*contextRepeatSingletonPart)
	for _, row := range rows {
		if !r.step(1 + int64(len(row.value.Part.Value.DocumentID))) {
			return r.err
		}
		if contextRepeatSingletonCandidate(row) {
			parts[row.value.Part.Value] = &contextRepeatSingletonPart{complete: navigationKnownBool(true), measures: make(map[uint64][]contextRepeatSingletonNote)}
		}
	}
	// Exactly one explicit publication is a stronger, independently proved
	// effective-transformation domain. Missing, changed or ambiguous histories
	// are not silently projected to zero or repaired by positive.available.
	for i := range timeline.State.Transpose.Publications {
		p := &timeline.State.Transpose.Publications[i]
		if !r.step(1 + int64(len(p.Position.Part.DocumentID))) {
			return r.err
		}
		if part := parts[p.Position.Part]; part != nil {
			part.transforms++
			part.transpose = p
		}
	}
	for _, g := range timeline.State.Transpose.Gaps {
		if !r.step(1 + int64(len(g.Part.DocumentID))) {
			return r.err
		}
		if part := parts[g.Part]; part != nil {
			gap := g.Availability
			if gap.State == EvidenceKnown || gap.State == EvidenceAbsent || gap.State == "" {
				gap = stateUnknown[bool]("context-repeat-singleton-transpose-history-incomplete")
			}
			part.complete = stateMergeGap(part.complete, gap)
		}
	}
	// Even initial playback mappings are outside this encoded-pitch theorem.
	// Reject only the selected canonical part; no unrelated part is poisoned.
	for _, record := range timeline.State.MIDI {
		if !r.step(1 + int64(len(record.Part.DocumentID))) {
			return r.err
		}
		if part := parts[record.Part]; part != nil {
			part.complete = stateMergeGap(part.complete, stateUnknown[bool]("context-repeat-singleton-playback-mapping-unresolved"))
		}
	}
	for _, record := range timeline.State.Play {
		if !r.step(1 + int64(len(record.Part.DocumentID))) {
			return r.err
		}
		if part := parts[record.Part]; part != nil {
			part.complete = stateMergeGap(part.complete, stateUnknown[bool]("context-repeat-singleton-playback-mapping-unresolved"))
		}
	}
	instruments := make(map[NodeID]noteInstrumentSet)
	for _, v := range timeline.State.NoteInstruments {
		if !r.step(1) {
			return r.err
		}
		instruments[v.Note.Node] = v
	}
	sr := &stateReader{ctx: r.ctx, s: r.s, b: plan.Bindings}
	// Walk in source order, not map order, to retain causal precedence.
	for _, row := range rows {
		if !r.step(1 + int64(len(row.value.Part.Value.DocumentID))) {
			return r.err
		}
		part := parts[row.value.Part.Value]
		if part == nil || part.notes != nil || part.complete.State != EvidenceKnown || part.transforms != 1 {
			continue
		}
		// A non-nil empty slice records construction even when no note is admitted.
		part.notes = make([]contextRepeatSingletonNote, 0, len(index.events[row.value.Part.Value]))
		for _, id := range index.events[row.value.Part.Value] {
			if !r.step(1) {
				return r.err
			}
			n := r.s.nodes[id]
			if r.matches(n, contextRepeatAnyNote) {
				note, status, e := contextRepeatSingletonReadNote(r, timeline, n, instruments[id])
				if e != nil {
					return e
				}
				part.complete = stateMergeGap(part.complete, status)
				if status.State == EvidenceKnown && status.Value {
					if len(part.notes) != 0 {
						previous := part.notes[len(part.notes)-1]
						if previous.pos.MeasureOrdinal > note.pos.MeasureOrdinal {
							part.complete = stateUnknown[bool]("context-repeat-singleton-order-unresolved")
						}
						if previous.pos.MeasureOrdinal == note.pos.MeasureOrdinal {
							cmp, e := r.s.rationalCmp(r.ctx, previous.end.Offset, note.pos.Offset)
							if e != nil {
								return e
							}
							if cmp > 0 {
								part.complete = stateUnknown[bool]("context-repeat-singleton-overlap-unresolved")
							}
						}
					}
					part.notes = append(part.notes, note)
					part.measures[note.pos.MeasureOrdinal] = append(part.measures[note.pos.MeasureOrdinal], note)
				}
			} else if sr.matches(n, timelineRoleAttributes) {
				status := contextRepeatSingletonTree(r, sr, n, 0)
				part.complete = stateMergeGap(part.complete, status)
			} else {
				part.complete = stateMergeGap(part.complete, stateUnknown[bool]("context-repeat-singleton-event-domain-unresolved"))
			}
			if r.err != nil {
				return r.err
			}
			if sr.err != nil {
				return sr.err
			}
		}
	}
	pending := make([]Evidence[bool], len(rows))
	for i := range rows {
		row := &rows[i]
		if !r.step(1 + int64(len(row.value.Part.Value.DocumentID))) {
			return r.err
		}
		if !contextRepeatSingletonCandidate(*row) {
			continue
		}
		part := parts[row.value.Part.Value]
		if part == nil || part.transforms != 1 {
			continue
		}
		result, e := contextRepeatSingletonRow(r.ctx, r.s, timeline, index, rows, *row, part)
		if e != nil {
			return e
		}
		// Preserve established equality and older independent negatives. When this
		// theorem is applicable its own unresolved/invalid prerequisite is causal.
		if result.State != "" {
			if result.State == EvidenceAbsent {
				result = stateUnknown[bool]("context-repeat-singleton-explicit-context-required")
			}
			pending[i] = result
		}
	}
	// Publish only after every proof and its final work admission succeeded.
	// This fixed-field commit is prepaid; interruption cannot expose a partial
	// negative result from this independent theorem.
	if !r.step(int64(len(rows)) + 1) {
		return r.err
	}
	for i, result := range pending {
		if result.State != "" {
			rows[i].value.ContentEquivalent = result
		}
	}
	return r.err
}
func contextRepeatSingletonCandidate(row contextRepeatRecord) bool {
	v := row.value
	return v.ContentEquivalent.State == EvidenceUnknown && (v.Kind == RepeatStyleBeat || v.Kind == RepeatStyleMeasure) && v.Type.State == EvidenceKnown && v.Type.Value == "start" && v.Part.State == EvidenceKnown && v.NoteScopesComplete.State == EvidenceKnown && v.NoteScopesComplete.Value && v.Complete.State == EvidenceKnown && v.Complete.Value && v.Stop.State == EvidenceKnown && row.position.State == EvidenceKnown && v.AllStaves.State == EvidenceKnown && !v.AllStaves.Value && v.Staff.State == EvidenceKnown && len(v.ExceptVoices) == 0
}

// Every visited node is independently complete; attributes are a separate
// axis. Only exact admitted attribute uses are allowed, including defaults.
func contextRepeatSingletonAttributes(r *contextReader, n *factNode, exact *AttributeSelector, allowed ...contextRole) Evidence[bool] {
	status := r.complete(n)
	if status.State != EvidenceKnown || !status.Value {
		return status
	}
	if n.end.AttributesComplete.State != EvidenceKnown {
		return *n.end.AttributesComplete
	}
	if !n.end.AttributesComplete.Value {
		return stateUnknown[bool]("context-repeat-singleton-attributes-incomplete")
	}
	for _, a := range n.attributes {
		if !r.step(1) {
			return stateUnknown[bool]("context-interrupted")
		}
		if a.Presence.State == EvidenceAbsent || a.Presence.State == EvidenceKnown && !a.Presence.Value {
			continue
		}
		if a.Presence.State != EvidenceKnown {
			return a.Presence
		}
		if a.Declaration.State != EvidenceKnown {
			return stateUnavailable[DeclarationID, bool](a.Declaration)
		}
		found := exact != nil && r.sameDeclaration(a.Declaration.Value, exact.Declaration)
		for _, role := range allowed {
			selector, ok := r.attributeSelector(n, role)
			if ok && r.sameDeclaration(a.Declaration.Value, selector.Declaration) {
				found = true
				break
			}
		}
		if !found {
			return stateUnknown[bool]("context-repeat-singleton-attribute-domain-unresolved")
		}
	}
	return status
}

// kind is a closed structural level, never a name-based semantic lookup:
// 0 attributes, 1 transpose, 2 measure-style, 3 repeat, 4 pitch, 5 note,
// 6 instrument, 7 scalar, 8 empty. Every recursive edge strictly reduces
// the remaining MusicXML shape depth; arbitrary input depth cannot recurse.
func contextRepeatSingletonTree(r *contextReader, sr *stateReader, n *factNode, kind int) Evidence[bool] {
	var allowed []contextRole
	var exact *AttributeSelector
	switch kind {
	case 1:
		if !r.step(1) {
			return stateUnknown[bool]("context-interrupted")
		}
		if sr == nil || sr.b == nil {
			return stateUnknown[bool]("context-repeat-singleton-transpose-binding-unavailable")
		}
		selector, ok := sr.b.attribute(timelineRoleTransposeNumber)
		if !ok {
			return stateUnknown[bool]("context-repeat-singleton-transpose-binding-unavailable")
		}
		exact = &selector
	case 2:
		allowed = []contextRole{contextRepeatStaff}
	case 3:
		allowed = []contextRole{contextBeatType, contextMeasureType, contextBeatSlashes, contextMeasureSlashes, contextBeatDots}
	case 6:
		allowed = []contextRole{contextRepeatInstrumentID}
	}
	status := contextRepeatSingletonAttributes(r, n, exact, allowed...)
	if status.State != EvidenceKnown || !status.Value {
		return status
	}
	for _, ch := range n.children {
		if !r.step(1) {
			return stateUnknown[bool]("context-interrupted")
		}
		next := -1
		switch kind {
		case 0:
			switch {
			case sr.matches(ch, timelineRoleAttributesDivisions), sr.matches(ch, timelineRoleAttributesStaves):
				next = 7
			case sr.matches(ch, timelineRoleAttributesTranspose):
				next = 1
			case r.matches(ch, contextRepeatStyle):
				next = 2
			}
		case 1:
			switch {
			case sr.matches(ch, timelineRoleTransposeChromatic), sr.matches(ch, timelineRoleTransposeDiatonic), sr.matches(ch, timelineRoleTransposeOctaveChange):
				next = 7
			}
		case 2:
			if r.matches(ch, contextBeatRepeat) || r.matches(ch, contextMeasureRepeat) {
				next = 3
			}
		case 3:
			if r.matches(ch, contextBeatDisplayType) {
				next = 7
			}
			if r.matches(ch, contextBeatDisplayDot) {
				next = 8
			}
		case 4:
			if r.matches(ch, contextRepeatPitchStep) || r.matches(ch, contextRepeatPitchAlter) || r.matches(ch, contextRepeatPitchOctave) {
				next = 7
			}
		case 5:
			switch {
			case r.matches(ch, contextRepeatNotePitch):
				next = 4
			case r.matches(ch, contextRepeatNoteRest), r.matches(ch, contextRepeatNoteDot):
				next = 8
			case r.matches(ch, contextRepeatNoteInstrument):
				next = 6
			case r.matches(ch, contextRepeatNoteDuration), r.matches(ch, contextRepeatNoteVoice), r.matches(ch, contextRepeatNoteStaff), r.matches(ch, contextRepeatNoteType), r.matches(ch, contextRepeatNoteAccidental), r.matches(ch, contextRepeatNoteStem):
				next = 7
			}
		}
		if next < 0 {
			return stateUnknown[bool]("context-repeat-singleton-content-domain-unresolved")
		}
		status = contextRepeatSingletonTree(r, sr, ch, next)
		if status.State != EvidenceKnown || !status.Value {
			return status
		}
	}
	if kind == 7 {
		value := r.scalar(n)
		if value.State != EvidenceKnown {
			return stateUnavailable[Atomic, bool](value)
		}
	}
	return status
}

func contextRepeatSingletonReadNote(r *contextReader, timeline *timelineService, n *factNode, instrument noteInstrumentSet) (contextRepeatSingletonNote, Evidence[bool], error) {
	out := contextRepeatSingletonNote{source: NodeRef{r.s.documentID, n.header.ID}}
	status := contextRepeatSingletonTree(r, nil, n, 5)
	if r.err != nil {
		return out, status, r.err
	}
	if status.State != EvidenceKnown || !status.Value {
		return out, status, nil
	}
	f, ok := timeline.Facts[n.header.ID]
	if !ok {
		return out, stateUnknown[bool]("context-repeat-singleton-timing-unavailable"), nil
	}
	for _, v := range []Evidence[bool]{stateUnavailable[NodeRef, bool](f.Part), stateUnavailable[musicalPosition, bool](f.Position), stateUnavailable[musicalPosition, bool](f.End), stateUnavailable[Rational, bool](f.Duration), stateUnavailable[Integer, bool](f.Staff), stateUnavailable[string, bool](f.Voice), instrument.Complete} {
		if v.State != EvidenceKnown {
			if v.State == EvidenceAbsent || v.State == "" {
				v = stateUnknown[bool]("context-repeat-singleton-explicit-context-required")
			}
			return out, v, nil
		}
	}
	if !instrument.Complete.Value || len(instrument.References) != 1 {
		return out, stateUnknown[bool]("context-repeat-singleton-instrument-unresolved"), nil
	}
	if instrument.References[0].State != EvidenceKnown {
		return out, stateUnavailable[NodeRef, bool](instrument.References[0]), nil
	}
	if !f.Duration.Value.Valid() || f.Duration.Value.value.Sign() <= 0 || !f.Position.Value.Offset.Valid() || !f.End.Value.Offset.Valid() {
		return out, stateUnknown[bool]("context-repeat-singleton-timing-unavailable"), nil
	}
	for _, p := range []musicalPosition{f.Position.Value, f.End.Value} {
		member, err := timeline.State.frameMember(r.ctx, r.s, p)
		if err != nil {
			return out, status, err
		}
		if !member {
			return out, stateUnknown[bool]("context-repeat-singleton-containment-unresolved"), nil
		}
	}
	if r.err = contextRepeatSingletonWork(r.ctx, r.s, 1, int64(len(f.Part.Value.DocumentID)), int64(len(f.Position.Value.Part.DocumentID)), int64(len(f.End.Value.Part.DocumentID))); r.err != nil {
		return out, status, r.err
	}
	if f.Position.Value.Part != f.Part.Value || f.End.Value.Part != f.Part.Value || f.Position.Value.Measure != f.End.Value.Measure || f.Position.Value.MeasureOrdinal != f.End.Value.MeasureOrdinal {
		return out, stateUnknown[bool]("context-repeat-singleton-containment-unresolved"), nil
	}
	if r.err = contextRepeatSingletonWork(r.ctx, r.s, 1, int64(len(instrument.Part.DocumentID)), int64(len(f.Part.Value.DocumentID)), int64(len(instrument.Note.DocumentID)), int64(len(out.source.DocumentID)), int64(len(instrument.References[0].Value.DocumentID)), int64(len(r.s.documentID))); r.err != nil {
		return out, status, r.err
	}
	if instrument.Part != f.Part.Value || instrument.Note != out.source || instrument.References[0].Value.DocumentID != r.s.documentID {
		return out, stateUnknown[bool]("context-repeat-singleton-instrument-unresolved"), nil
	}
	duration, err := r.s.rationalSub(r.ctx, f.End.Value.Offset, f.Position.Value.Offset)
	if err != nil {
		return out, status, err
	}
	cmp, err := r.s.rationalCmp(r.ctx, duration, f.Duration.Value)
	if err != nil {
		return out, status, err
	}
	if cmp != 0 {
		return out, stateUnknown[bool]("context-repeat-singleton-timing-unavailable"), nil
	}
	out.origin = f.Position.Value
	out.pos, out.end, out.duration, out.staff, out.voice, out.instrument = f.Position.Value, f.End.Value, f.Duration.Value, f.Staff.Value, f.Voice.Value, instrument.References[0].Value
	// Explicit scalar slots, rather than potentially projected/inherited facts.
	staff := stateInteger(r.childScalar(n, contextRepeatNoteStaff))
	voice := stateText(r.childScalar(n, contextRepeatNoteVoice))
	if staff.State != EvidenceKnown {
		return out, stateUnavailable[Integer, bool](staff), r.err
	}
	if voice.State != EvidenceKnown {
		return out, stateUnavailable[string, bool](voice), r.err
	}
	pitch, pp := r.child(n, contextRepeatNotePitch)
	_, rp := r.child(n, contextRepeatNoteRest)
	if rp.State == EvidenceKnown {
		out.rest = true
		return out, status, r.err
	}
	if pp.State != EvidenceKnown {
		return out, stateUnknown[bool]("context-repeat-singleton-pitch-unavailable"), r.err
	}
	value, err := performancePitch(r.ctx, r.s, stateText(r.childScalar(pitch, contextRepeatPitchStep)), stateDecimalValue(r.childScalar(pitch, contextRepeatPitchAlter)), stateInteger(r.childScalar(pitch, contextRepeatPitchOctave)))
	if err != nil {
		return out, status, err
	}
	if r.err != nil {
		return out, status, r.err
	}
	if value.State != EvidenceKnown {
		return out, stateUnavailable[Rational, bool](value), nil
	}
	out.pitch = value.Value
	return out, status, nil
}

func contextRepeatSingletonRow(ctx context.Context, s *Session, timeline *timelineService, index *contextRepeatProofIndex, rows []contextRepeatRecord, row contextRepeatRecord, part *contextRepeatSingletonPart) (Evidence[bool], error) {
	unknown := stateUnknown[bool]("context-repeat-singleton-counterexample-unproved")
	if err := s.step(ctx, 1); err != nil {
		return Evidence[bool]{}, err
	}
	if part.complete.State != EvidenceKnown || !part.complete.Value {
		return part.complete, nil
	}
	p := part.transpose
	if p == nil || part.transforms != 1 || p.TimeOnly != nil || p.Scope.AllStaves {
		return unknown, nil
	}
	if p.Value.State != EvidenceKnown {
		return stateUnavailable[transposeState, bool](p.Value), nil
	}
	if p.Value.Value.Double.State != EvidenceAbsent {
		return stateUnknown[bool]("context-repeat-singleton-transpose-doubling-unresolved"), nil
	}
	if err := contextRepeatSingletonWork(ctx, s, p.Scope.Staff.ByteSize(), row.value.Staff.Value.ByteSize()); err != nil {
		return Evidence[bool]{}, err
	}
	cmp, err := p.Scope.Staff.Cmp(row.value.Staff.Value)
	if err != nil {
		return Evidence[bool]{}, err
	}
	if cmp != 0 {
		return unknown, nil
	}
	displacement, _, err := p.Value.Value.contributions(ctx, s)
	if err != nil {
		return Evidence[bool]{}, err
	}
	if displacement.State != EvidenceKnown {
		return stateUnavailable[Rational, bool](displacement), nil
	}
	stops := index.stopRows(rows, row.value.Stop.Value)
	if len(stops) != 1 || stops[0].position.State != EvidenceKnown {
		return unknown, nil
	}
	start, stop := row.position.Value, stops[0].position.Value
	for _, position := range []musicalPosition{start, stop, p.Position} {
		member, e := timeline.State.frameMember(ctx, s, position)
		if e != nil {
			return Evidence[bool]{}, e
		}
		if !member {
			return stateUnknown[bool]("context-repeat-singleton-containment-unresolved"), nil
		}
	}
	if err = contextRepeatSingletonWork(ctx, s, 1, int64(len(start.Part.DocumentID)), int64(len(stop.Part.DocumentID)), int64(len(p.Position.Part.DocumentID))); err != nil {
		return Evidence[bool]{}, err
	}
	if start.Part != stop.Part || start.Part != p.Position.Part {
		return unknown, nil
	}
	compare := func(a, b contextRepeatSingletonNote) (Evidence[bool], error) {
		order, e := timeline.State.positionOrder(ctx, s, p.Position, a.origin)
		if e != nil {
			return Evidence[bool]{}, e
		}
		if order != stateBefore && order != stateEqual {
			return unknown, nil
		}
		order, e = timeline.State.positionOrder(ctx, s, p.Position, b.origin)
		if e != nil {
			return Evidence[bool]{}, e
		}
		if order != stateBefore && order != stateEqual {
			return unknown, nil
		}
		return contextRepeatSingletonUnequal(ctx, s, a, b, row.value.Staff.Value)
	}
	if row.value.Kind == RepeatStyleMeasure {
		if row.value.PatternMeasures.State != EvidenceKnown || start.Offset.value.Sign() != 0 || stop.Offset.value.Sign() != 0 || start.MeasureOrdinal == 0 || stop.MeasureOrdinal <= start.MeasureOrdinal {
			return unknown, nil
		}
		cmp, e := navigationCountCompare(ctx, s, row.value.PatternMeasures.Value, 1)
		if e != nil {
			return Evidence[bool]{}, e
		}
		if cmp != 0 {
			return unknown, nil
		}
		source := part.measures[start.MeasureOrdinal-1]
		if len(source) != 1 {
			return unknown, nil
		}
		for measure := start.MeasureOrdinal; measure < stop.MeasureOrdinal; measure++ {
			if err = s.step(ctx, 1); err != nil {
				return Evidence[bool]{}, err
			}
			target := part.measures[measure]
			if len(target) != 1 {
				return unknown, nil
			}
			result, e := compare(source[0], target[0])
			if e != nil {
				return Evidence[bool]{}, e
			}
			if result.State == EvidenceKnown && !result.Value {
				return result, nil
			}
		}
		return unknown, nil
	}
	if row.value.Beat.State != EvidenceKnown || !row.value.Beat.Value.Valid() || row.value.Beat.Value.value.Sign() <= 0 || start.Measure != stop.Measure || start.MeasureOrdinal != stop.MeasureOrdinal {
		return unknown, nil
	}
	from, err := s.rationalSub(ctx, start.Offset, row.value.Beat.Value)
	if err != nil {
		return Evidence[bool]{}, err
	}
	if from.value.Sign() < 0 {
		return unknown, nil
	}
	source, found, err := contextRepeatSingletonWindow(ctx, s, part.measures[start.MeasureOrdinal], from, start.Offset)
	if err != nil {
		return Evidence[bool]{}, err
	}
	if !found {
		return unknown, nil
	}
	for from = start.Offset; ; {
		cmp, err = s.rationalCmp(ctx, from, stop.Offset)
		if err != nil {
			return Evidence[bool]{}, err
		}
		if cmp >= 0 {
			return unknown, nil
		}
		to, e := s.rationalAdd(ctx, from, row.value.Beat.Value)
		if e != nil {
			return Evidence[bool]{}, e
		}
		cmp, e = s.rationalCmp(ctx, to, stop.Offset)
		if e != nil {
			return Evidence[bool]{}, e
		}
		if cmp > 0 {
			return unknown, nil
		}
		target, found, e := contextRepeatSingletonWindow(ctx, s, part.measures[start.MeasureOrdinal], from, to)
		if e != nil {
			return Evidence[bool]{}, e
		}
		if !found {
			return unknown, nil
		}
		result, e := compare(source, target)
		if e != nil {
			return Evidence[bool]{}, e
		}
		if result.State == EvidenceKnown && !result.Value {
			return result, nil
		}
		from = to
	}
}

// Complete enumeration and containment are prerequisites supplied by the
// caller. No mismatch of a representation field alone is a negative proof.
func contextRepeatSingletonUnequal(ctx context.Context, s *Session, a, b contextRepeatSingletonNote, staff Integer) (Evidence[bool], error) {
	unknown := stateUnknown[bool]("context-repeat-singleton-counterexample-unproved")
	if err := contextRepeatSingletonWork(ctx, s, 1, int64(len(a.voice)), int64(len(b.voice)), int64(len(a.instrument.DocumentID)), int64(len(b.instrument.DocumentID)), a.staff.ByteSize(), b.staff.ByteSize(), staff.ByteSize()); err != nil {
		return Evidence[bool]{}, err
	}
	if a.rest || a.voice == "" || a.voice != b.voice || a.instrument != b.instrument || a.instrument.Node == 0 {
		return unknown, nil
	}
	for _, v := range []Integer{a.staff, b.staff} {
		cmp, err := v.Cmp(staff)
		if err != nil {
			return Evidence[bool]{}, err
		}
		if cmp != 0 {
			return unknown, nil
		}
	}
	for _, pair := range [][2]Rational{{a.pos.Offset, b.pos.Offset}, {a.duration, b.duration}} {
		cmp, err := s.rationalCmp(ctx, pair[0], pair[1])
		if err != nil {
			return Evidence[bool]{}, err
		}
		if cmp != 0 {
			return unknown, nil
		}
	}
	if b.rest {
		return navigationKnownBool(false), nil
	}
	cmp, err := s.rationalCmp(ctx, a.pitch, b.pitch)
	if err != nil {
		return Evidence[bool]{}, err
	}
	if cmp != 0 {
		return navigationKnownBool(false), nil
	}
	return unknown, nil
}

// Notes are source-ordered with no backup/chord admitted by the independent
// event-domain scan. Binary selection avoids a repeated full-measure scan.
func contextRepeatSingletonWindow(ctx context.Context, s *Session, notes []contextRepeatSingletonNote, from, to Rational) (contextRepeatSingletonNote, bool, error) {
	var out contextRepeatSingletonNote
	if err := s.step(ctx, 1); err != nil {
		return out, false, err
	}
	left, right := 0, len(notes)
	for left < right {
		middle := left + (right-left)/2
		cmp, err := s.rationalCmp(ctx, notes[middle].end.Offset, from)
		if err != nil {
			return out, false, err
		}
		if cmp <= 0 {
			left = middle + 1
		} else {
			right = middle
		}
	}
	if left == len(notes) {
		return out, false, nil
	}
	note := notes[left]
	cmp, err := s.rationalCmp(ctx, note.pos.Offset, from)
	if err != nil {
		return out, false, err
	}
	if cmp < 0 {
		return out, false, nil
	}
	cmp, err = s.rationalCmp(ctx, note.end.Offset, to)
	if err != nil {
		return out, false, err
	}
	if cmp > 0 {
		return out, false, nil
	}
	if left+1 < len(notes) {
		cmp, err = s.rationalCmp(ctx, notes[left+1].pos.Offset, to)
		if err != nil {
			return out, false, err
		}
		if cmp < 0 {
			return out, false, nil
		}
	}
	note.pos.Offset, err = s.rationalSub(ctx, note.pos.Offset, from)
	return note, err == nil, err
}
