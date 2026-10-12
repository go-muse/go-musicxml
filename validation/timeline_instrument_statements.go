package validation

import (
	"context"
)

// These queries select encoded statements, not a realized synthesizer setup.
// In particular Virtual is one atomic descriptor; its absent nested fields are
// not inherited from a different descriptor.
type instrumentPerformanceState struct {
	Solo     bool
	Ensemble ensembleState
}
type instrumentStatements struct {
	Sound       stateResult[string]
	Performance stateResult[instrumentPerformanceState]
	Virtual     stateResult[virtualInstrumentState]
}

func statementEvidence[T any](v Evidence[T]) Evidence[T] {
	if v.State == "" {
		v.State = EvidenceAbsent
	}
	return v
}
func instrumentSoundStatement(v instrumentState) Evidence[string] { return statementEvidence(v.Sound) }
func instrumentVirtualStatement(v instrumentState) Evidence[virtualInstrumentState] {
	return statementEvidence(v.Virtual)
}
func instrumentPerformanceStatement(v instrumentState) Evidence[instrumentPerformanceState] {
	solo, ensemble := statementEvidence(v.Solo), statementEvidence(v.Ensemble)
	availability := Evidence[bool]{State: EvidenceKnown, Value: true}
	for _, value := range [2]Evidence[bool]{solo, stateUnavailable[ensembleState, bool](ensemble)} {
		if value.State != EvidenceKnown && value.State != EvidenceAbsent {
			availability = stateMergeGap(availability, value)
		}
	}
	if availability.State != EvidenceKnown {
		return stateUnavailable[bool, instrumentPerformanceState](availability)
	}
	if solo.State == EvidenceKnown && ensemble.State == EvidenceKnown {
		return stateUnknown[instrumentPerformanceState]("timeline-instrument-choice-conflict")
	}
	if solo.State == EvidenceKnown {
		return Evidence[instrumentPerformanceState]{State: EvidenceKnown, Value: instrumentPerformanceState{Solo: true}}
	}
	if ensemble.State == EvidenceKnown {
		return Evidence[instrumentPerformanceState]{State: EvidenceKnown, Value: instrumentPerformanceState{Ensemble: ensemble.Value}}
	}
	return Evidence[instrumentPerformanceState]{State: EvidenceAbsent}
}
func (v *instrumentChanges) discardStatementIndices(s *Session) {
	for i, index := range v.statementIndices {
		stateDiscardIndex(s, index)
		v.statementIndices[i] = nil
	}
}

// A filtered index owns indices only; raw immutable publications remain in their
// original owner. The view passed to queryState has Count exactly rawlen, so it
// never rebuilds or discards this separately owned index. Failed construction
// releases candidate bytes and does not publish a partial cache. Projectors must
// remain closed fixed-field presence accessors, never variable-length traversals.
func statementIndex[T, V any](ctx context.Context, s *Session, raw *stateHistory[T], cached **stateHistoryIndex, project func(T) Evidence[V]) (*stateHistoryIndex, error) {
	if *cached != nil && (*cached).Count == len(raw.Publications) {
		return *cached, nil
	}
	base, err := stateIndex(ctx, s, raw)
	if err != nil {
		return nil, err
	}
	cost, err := rationalCheckedMul(BudgetBufferedBytes, int64(len(raw.Publications)), 256)
	if err != nil {
		return nil, err
	}
	cost, err = rationalCheckedAdd(BudgetBufferedBytes, cost, 256)
	if err != nil {
		return nil, err
	}
	if _, err = AllocationSize(cost); err != nil {
		return nil, err
	}
	if err = s.retain(cost); err != nil {
		return nil, err
	}
	candidate := &stateHistoryIndex{Scopes: make(map[stateIndexScope]stateIndexBucket), Count: len(raw.Publications), bytes: cost}
	ok := false
	defer func() {
		if !ok {
			stateDiscardIndex(s, candidate)
		}
	}()
	for key, bucket := range base.Scopes {
		if err = s.step(ctx, int64(len(key.Part.DocumentID))+int64(len(key.Staff))+1); err != nil {
			return nil, err
		}
		for _, i := range bucket.Publications {
			p := raw.Publications[i]
			// The closed projectors inspect presence only; immutable payload magnitudes
			// and strings are not traversed until query comparison/copy.
			if err = s.step(ctx, 1); err != nil {
				return nil, err
			}
			if p.Value.State == EvidenceKnown && statementEvidence(project(p.Value.Value)).State == EvidenceAbsent {
				continue
			}
			scope := candidate.Scopes[key]
			scope.Publications = append(scope.Publications, i)
			scope.Partial = scope.Partial || p.Position.UnprovedFrame
			candidate.Scopes[key] = scope
		}
	}
	if err = s.step(ctx, 1); err != nil {
		return nil, err
	}
	stateDiscardIndex(s, *cached)
	*cached = candidate
	ok = true
	return candidate, nil
}

// Extra gaps are checked against the selected property's publication rather
// than the latest unrelated whole-record publication.
func queryStatementView[T, V any](ctx context.Context, s *Session, h *timelineState, view *stateHistory[T], pos musicalPosition, staff Evidence[Integer], project func(T) Evidence[V], extra ...[]stateGap) (stateResult[V], error) {
	return queryStateAtFull(ctx, s, h, view, pos, staff, project, true, true, nil, extra...)
}

func instrumentStatement[T any](ctx context.Context, s *Session, h *timelineState, pos musicalPosition, id NodeRef, field int, project func(instrumentState) Evidence[T]) (stateResult[T], error) {
	if id.Node == 0 || id.DocumentID != pos.Part.DocumentID {
		return stateResult[T]{}, contract("instrument statement", "canonical matching instrument required")
	}
	var selected *instrumentChanges
	for i := range h.InstrumentChanges {
		if err := s.step(ctx, 1); err != nil {
			return stateResult[T]{}, err
		}
		if h.InstrumentChanges[i].Instrument == id {
			selected = &h.InstrumentChanges[i]
			break
		}
	}
	var view stateHistory[instrumentState]
	var local []stateGap
	if selected != nil {
		index, err := statementIndex(ctx, s, &selected.History, &selected.statementIndices[field], project)
		if err != nil {
			return stateResult[T]{}, err
		}
		view = selected.History
		view.index = index
		local = selected.statementGaps[field]
	}
	// An empty uncached view has no raw publications; its local index is scratch.
	if selected == nil {
		defer func() { stateDiscardIndex(s, view.index) }()
	}
	initial := func() (stateResult[T], error) {
		out := stateResult[T]{Value: Evidence[T]{State: EvidenceAbsent}}
		found := false
		for _, d := range h.Definitions {
			if err := s.step(ctx, 1); err != nil {
				return stateResult[T]{}, err
			}
			if d.Part != pos.Part || d.Instrument != id {
				continue
			}
			value := stateUnavailable[instrumentState, T](d.Value)
			if d.Value.State == EvidenceKnown {
				value = statementEvidence(project(d.Value.Value))
			}
			if found {
				return stateResult[T]{Value: stateUnknown[T]("timeline-instrument-definition-conflict")}, nil
			}
			found = true
			out = stateResult[T]{Value: value, Provenance: d.Provenance, Inherited: true}
		}
		if err := stateRetainCopy(ctx, s, out); err != nil {
			return stateResult[T]{}, err
		}
		return stateClone(out), nil
	}
	return queryStateAtFull(ctx, s, h, &view, pos, Evidence[Integer]{State: EvidenceKnown, Value: stateOne()}, project, true, true, initial, h.InstrumentGaps, h.InstrumentPropertyGaps[field], local)
}

func (h *timelineState) effectiveInstrumentStatements(ctx context.Context, s *Session, pos musicalPosition, id NodeRef) (instrumentStatements, error) {
	out := instrumentStatements{}
	var err error
	out.Sound, err = instrumentStatement(ctx, s, h, pos, id, 0, instrumentSoundStatement)
	if err != nil {
		return instrumentStatements{}, err
	}
	out.Performance, err = instrumentStatement(ctx, s, h, pos, id, 1, instrumentPerformanceStatement)
	if err != nil {
		return instrumentStatements{}, err
	}
	out.Virtual, err = instrumentStatement(ctx, s, h, pos, id, 2, instrumentVirtualStatement)
	if err != nil {
		return instrumentStatements{}, err
	}
	return out, nil
}

// Compatibility aggregate. Property queries above retain exact per-property
// provenance/inheritance. This aggregate never claims a physical playback value.
func (h *timelineState) effectiveInstrument(ctx context.Context, s *Session, pos musicalPosition, id NodeRef) (stateResult[instrumentState], error) {
	fields, err := h.effectiveInstrumentStatements(ctx, s, pos, id)
	if err != nil {
		return stateResult[instrumentState]{}, err
	}
	v := instrumentState{Sound: fields.Sound.Value, Virtual: fields.Virtual.Value}
	performance := fields.Performance.Value
	if performance.State == EvidenceKnown {
		v.Solo = Evidence[bool]{State: EvidenceAbsent}
		v.Ensemble = Evidence[ensembleState]{State: EvidenceAbsent}
		if performance.Value.Solo {
			v.Solo = Evidence[bool]{State: EvidenceKnown, Value: true}
		} else {
			v.Ensemble = Evidence[ensembleState]{State: EvidenceKnown, Value: performance.Value.Ensemble}
		}
	} else {
		v.Solo = stateUnavailable[instrumentPerformanceState, bool](performance)
		v.Ensemble = stateUnavailable[instrumentPerformanceState, ensembleState](performance)
	}
	availability := []Evidence[bool]{stateUnavailable[string, bool](v.Sound), stateUnavailable[instrumentPerformanceState, bool](performance), stateUnavailable[virtualInstrumentState, bool](v.Virtual)}
	allUnavailable := true
	strongest := Evidence[bool]{State: EvidenceKnown, Value: true}
	for _, value := range availability {
		if value.State == EvidenceKnown || value.State == EvidenceAbsent {
			allUnavailable = false
		} else {
			strongest = stateMergeGap(strongest, value)
		}
	}
	if allUnavailable {
		return stateUnavailableResult[bool, instrumentState](ctx, s, strongest)
	}
	out := stateResult[instrumentState]{Value: Evidence[instrumentState]{State: EvidenceKnown, Value: v}, Inherited: fields.Sound.Inherited || fields.Performance.Inherited || fields.Virtual.Inherited, InheritanceUnproved: fields.Sound.InheritanceUnproved || fields.Performance.InheritanceUnproved || fields.Virtual.InheritanceUnproved}
	for _, provenance := range [3][]stateProvenance{fields.Sound.Provenance, fields.Performance.Provenance, fields.Virtual.Provenance} {
		for _, p := range provenance {
			duplicate := false
			for _, old := range out.Provenance {
				if err = s.step(ctx, 1); err != nil {
					return stateResult[instrumentState]{}, err
				}
				if old == p {
					duplicate = true
					break
				}
			}
			if !duplicate {
				if err = stateRetainCopy(ctx, s, p); err != nil {
					return stateResult[instrumentState]{}, err
				}
				out.Provenance = append(out.Provenance, stateClone(p))
			}
		}
	}
	return out, nil
}

// An unplaced observed change can only affect properties it actually states.
func (v *instrumentChanges) addStatementGap(ctx context.Context, s *Session, gap stateGap, value Evidence[instrumentState]) error {
	return addInstrumentStatementGaps(ctx, s, &v.statementGaps, gap, value)
}
func addInstrumentStatementGaps(ctx context.Context, s *Session, gaps *[3][]stateGap, gap stateGap, value Evidence[instrumentState]) error {
	fields := [3]Evidence[bool]{stateUnavailable[instrumentState, bool](value), stateUnavailable[instrumentState, bool](value), stateUnavailable[instrumentState, bool](value)}
	if value.State == EvidenceKnown {
		fields = [3]Evidence[bool]{stateUnavailable[string, bool](instrumentSoundStatement(value.Value)), stateUnavailable[instrumentPerformanceState, bool](instrumentPerformanceStatement(value.Value)), stateUnavailable[virtualInstrumentState, bool](instrumentVirtualStatement(value.Value))}
	}
	for field, presence := range fields {
		if presence.State == EvidenceAbsent {
			continue
		}
		g := gap
		if presence.State != EvidenceKnown {
			g.Availability = stateMergeGap(g.Availability, presence)
		}
		if _, err := stateReserve(ctx, s, g); err != nil {
			return err
		}
		gaps[field] = append(gaps[field], stateClone(g))
	}
	return nil
}
