package validation

import (
	"context"
	"math/big"
	"reflect"
)

// Scope is explicit even when the source has no IDREF. AllInstruments is only
// supplied for midi-device, whose MusicXML clause explicitly defines omission.
type instrumentTargetScope struct {
	Part, Instrument NodeRef
	AllInstruments   bool
}

func recordTargetScope[T any](record instrumentRecord[T], omittedAll bool) Evidence[instrumentTargetScope] {
	if record.Instrument.State == EvidenceAbsent && omittedAll {
		return Evidence[instrumentTargetScope]{State: EvidenceKnown, Value: instrumentTargetScope{Part: record.Part, AllInstruments: true}}
	}
	if record.Instrument.State != EvidenceKnown {
		if record.Instrument.State == EvidenceAbsent || record.Instrument.State == "" {
			return stateUnknown[instrumentTargetScope]("timeline-required-instrument-id-absent")
		}
		return stateUnavailable[NodeRef, instrumentTargetScope](record.Instrument)
	}
	id := record.Instrument.Value
	if id.Node == 0 || id.DocumentID != record.Part.DocumentID {
		return stateUnknown[instrumentTargetScope]("timeline-instrument-owner-unavailable")
	}
	return Evidence[instrumentTargetScope]{State: EvidenceKnown, Value: instrumentTargetScope{Part: record.Part, Instrument: id}}
}
func midiDeviceScope(ctx context.Context, s *Session, record instrumentRecord[midiDeviceState]) (Evidence[instrumentTargetScope], error) {
	return stateOwnEvidence(ctx, s, recordTargetScope(record, true))
}

// The shared timeline index calls its identity discriminator Staff. Here its
// exact Integer is an internal canonical node key, never a parsed instrument
// count or a numeric ID spelling. No count-sized allocation or iteration occurs.
func instrumentIdentityKey(ctx context.Context, s *Session, id NodeRef) (Integer, error) {
	if id.Node == 0 || id.DocumentID == "" {
		return Integer{}, contract("instrument scope", "canonical instrument required")
	}
	if err := s.step(ctx, int64(len(id.DocumentID))+1); err != nil {
		return Integer{}, err
	}
	if err := s.retain(256); err != nil {
		return Integer{}, err
	}
	return NewInteger(new(big.Int).SetUint64(uint64(id.Node)))
}
func targetStateScope(ctx context.Context, s *Session, target instrumentTargetScope) (stateScope, error) {
	if target.AllInstruments {
		return stateScope{AllStaves: true}, nil
	}
	key, err := instrumentIdentityKey(ctx, s, target.Instrument)
	if err != nil {
		return stateScope{}, err
	}
	return stateScope{Staff: key}, nil
}

type recordStatementStore[T any] struct {
	History stateHistory[T]
	Indices [8]*stateHistoryIndex
	Gaps    [8][]stateGap
}

func (store *recordStatementStore[T]) discardIndices(s *Session) {
	for i, index := range store.Indices {
		stateDiscardIndex(s, index)
		store.Indices[i] = nil
	}
}
func newRecordStatementStore[T any](s *Session) (*recordStatementStore[T], error) {
	if err := s.retain(2048); err != nil {
		return nil, err
	}
	return &recordStatementStore[T]{}, nil
}

func midiStatementPresence(v midiState) [8]Evidence[bool] {
	return [8]Evidence[bool]{stateUnavailable[Integer, bool](statementEvidence(v.Channel)), stateUnavailable[string, bool](statementEvidence(v.Name)), stateUnavailable[Integer, bool](statementEvidence(v.Bank)), stateUnavailable[Integer, bool](statementEvidence(v.Program)), stateUnavailable[Integer, bool](statementEvidence(v.Unpitched)), stateUnavailable[Decimal, bool](statementEvidence(v.Volume)), stateUnavailable[Decimal, bool](statementEvidence(v.Pan)), stateUnavailable[Decimal, bool](statementEvidence(v.Elevation))}
}
func deviceStatementPresence(v midiDeviceState) [8]Evidence[bool] {
	out := [8]Evidence[bool]{}
	for i := range out {
		out[i].State = EvidenceAbsent
	}
	out[0] = stateUnavailable[string, bool](statementEvidence(v.Name))
	out[1] = stateUnavailable[Integer, bool](statementEvidence(v.Port))
	return out
}

func retainRecordStatement[T any](ctx context.Context, s *Session, store *recordStatementStore[T], record instrumentRecord[T], ordinal uint64, omittedAll bool, presence func(T) [8]Evidence[bool]) error {
	if record.Position.State == EvidenceAbsent {
		return nil
	} // initial statement is retained in its raw record
	target := recordTargetScope(record, omittedAll)
	scope := stateScope{AllStaves: true}
	var err error
	if target.State == EvidenceKnown {
		scope, err = targetStateScope(ctx, s, target.Value)
		if err != nil {
			return err
		}
	}
	if record.Position.State == EvidenceKnown && record.Position.Value.Part != record.Part {
		return contract("instrument record", "matching positioned part required")
	}
	if record.Position.State == EvidenceKnown && target.State == EvidenceKnown {
		p := statePublication[T]{Position: record.Position.Value, TimeOnly: record.TimeOnly, Scope: scope, Value: record.Value, Provenance: []stateProvenance{{Node: record.Source, Kind: stateExplicit}}}
		if record.Position.Value.Part != record.Part {
			return contract("instrument record", "matching positioned part required")
		}
		if err = publishState(ctx, s, &store.History, p); err != nil {
			return err
		}
		store.discardIndices(s)
		return nil
	}
	availability := stateUnavailable[musicalPosition, bool](record.Position)
	if record.Position.State == EvidenceKnown {
		availability = Evidence[bool]{State: EvidenceKnown, Value: true}
		ordinal = record.Position.Value.MeasureOrdinal
	}
	if target.State != EvidenceKnown {
		availability = stateMergeGap(availability, stateUnavailable[instrumentTargetScope, bool](target))
	}
	values := [8]Evidence[bool]{}
	if record.Value.State == EvidenceKnown {
		values = presence(record.Value.Value)
	} else {
		for i := range values {
			values[i] = stateUnavailable[T, bool](record.Value)
		}
	}
	for field, value := range values {
		if value.State == EvidenceAbsent {
			continue
		}
		value = stateConditionalValue(value, record.TimeOnly)
		gap := stateGap{Part: record.Part, FirstMeasure: ordinal, LastMeasure: ordinal, Scope: scope, Availability: availability}
		if record.Position.State == EvidenceKnown {
			gap.Position = &record.Position.Value
		}
		if value.State != EvidenceKnown {
			gap.Availability = stateMergeGap(gap.Availability, value)
		}
		if _, err = stateReserve(ctx, s, gap); err != nil {
			return err
		}
		store.Gaps[field] = append(store.Gaps[field], stateClone(gap))
	}
	return nil
}
func (h *timelineState) addMIDIRecord(ctx context.Context, s *Session, record instrumentRecord[midiState], ordinal uint64) error {
	if h.midiStatements == nil {
		var err error
		h.midiStatements, err = newRecordStatementStore[midiState](s)
		if err != nil {
			return err
		}
	}
	if _, err := stateReserve(ctx, s, record); err != nil {
		return err
	}
	if err := retainRecordStatement(ctx, s, h.midiStatements, record, ordinal, false, midiStatementPresence); err != nil {
		return err
	}
	h.MIDI = append(h.MIDI, stateClone(record))
	return nil
}
func (h *timelineState) addMIDIDeviceRecord(ctx context.Context, s *Session, record instrumentRecord[midiDeviceState], ordinal uint64) error {
	if h.deviceStatements == nil {
		var err error
		h.deviceStatements, err = newRecordStatementStore[midiDeviceState](s)
		if err != nil {
			return err
		}
	}
	if _, err := stateReserve(ctx, s, record); err != nil {
		return err
	}
	if err := retainRecordStatement(ctx, s, h.deviceStatements, record, ordinal, true, deviceStatementPresence); err != nil {
		return err
	}
	h.MIDIDevices = append(h.MIDIDevices, stateClone(record))
	return nil
}

func recordStatement[T, V any](ctx context.Context, s *Session, h *timelineState, pos musicalPosition, id NodeRef, store *recordStatementStore[T], records []instrumentRecord[T], field int, omittedAll bool, project func(T) Evidence[V]) (stateResult[V], error) {
	if id.DocumentID != pos.Part.DocumentID {
		return stateResult[V]{}, contract("instrument statement", "canonical matching instrument required")
	}
	key, err := instrumentIdentityKey(ctx, s, id)
	if err != nil {
		return stateResult[V]{}, err
	}
	var view stateHistory[T]
	var gaps []stateGap
	if store != nil {
		index, err := statementIndex(ctx, s, &store.History, &store.Indices[field], project)
		if err != nil {
			return stateResult[V]{}, err
		}
		view = store.History
		view.index = index
		gaps = store.Gaps[field]
	} else {
		defer func() { stateDiscardIndex(s, view.index) }()
	}
	initialValue := func() (stateResult[V], error) {
		out := stateResult[V]{Value: Evidence[V]{State: EvidenceAbsent}}
		found, conflict := false, false
		var initial Evidence[V]
		for _, record := range records {
			if err = s.step(ctx, 1); err != nil {
				return stateResult[V]{}, err
			}
			if record.Part != pos.Part || record.Position.State != EvidenceAbsent || record.Note.Node != 0 {
				continue
			}
			value := stateUnavailable[T, V](record.Value)
			if record.Value.State == EvidenceKnown {
				value = statementEvidence(project(record.Value.Value))
			}
			if value.State == EvidenceAbsent {
				continue
			}
			value = stateConditionalValue(value, record.TimeOnly)
			target := recordTargetScope(record, omittedAll)
			if target.State == EvidenceKnown {
				if !target.Value.AllInstruments && target.Value.Instrument != id {
					continue
				}
			} else if stateAvailabilityRank(target.State) > stateAvailabilityRank(value.State) {
				value = stateUnavailable[instrumentTargetScope, V](target)
			}
			if err = stateRetainCopy(ctx, s, value); err != nil {
				return stateResult[V]{}, err
			}
			if !found {
				out.Value = value
				initial = value
				found = true
			} else {
				if !reflect.DeepEqual(initial, value) {
					conflict = true
				}
				if stateAvailabilityRank(value.State) > stateAvailabilityRank(out.Value.State) {
					out.Value = value
				}
			}
			pr := stateProvenance{Node: record.Source, Kind: stateExplicit}
			if err = stateRetainCopy(ctx, s, pr); err != nil {
				return stateResult[V]{}, err
			}
			out.Provenance = append(out.Provenance, stateClone(pr))
		}
		if conflict && (out.Value.State == EvidenceKnown || out.Value.State == EvidenceAbsent) {
			out.Value = stateUnknown[V]("timeline-state-conflict")
		}
		out.Inherited = found
		if err = stateRetainCopy(ctx, s, out.Value); err != nil {
			return stateResult[V]{}, err
		}
		out.Value = stateClone(out.Value)
		return out, nil
	}
	return queryStateAtFull(ctx, s, h, &view, pos, Evidence[Integer]{State: EvidenceKnown, Value: key}, project, true, true, initialValue, h.MIDIUnknownGaps, gaps)
}

type midiStatements struct {
	Channel, Bank, Program, Unpitched stateResult[Integer]
	Name                              stateResult[string]
	Volume, Pan, Elevation            stateResult[Decimal]
}
type midiDeviceStatements struct {
	Name stateResult[string]
	Port stateResult[Integer]
}

func (h *timelineState) effectiveMIDIStatements(ctx context.Context, s *Session, pos musicalPosition, id NodeRef) (midiStatements, error) {
	out := midiStatements{}
	var err error
	out.Channel, err = recordStatement(ctx, s, h, pos, id, h.midiStatements, h.MIDI, 0, false, func(v midiState) Evidence[Integer] { return statementEvidence(v.Channel) })
	if err != nil {
		return midiStatements{}, err
	}
	out.Name, err = recordStatement(ctx, s, h, pos, id, h.midiStatements, h.MIDI, 1, false, func(v midiState) Evidence[string] { return statementEvidence(v.Name) })
	if err != nil {
		return midiStatements{}, err
	}
	out.Bank, err = recordStatement(ctx, s, h, pos, id, h.midiStatements, h.MIDI, 2, false, func(v midiState) Evidence[Integer] { return statementEvidence(v.Bank) })
	if err != nil {
		return midiStatements{}, err
	}
	out.Program, err = recordStatement(ctx, s, h, pos, id, h.midiStatements, h.MIDI, 3, false, func(v midiState) Evidence[Integer] { return statementEvidence(v.Program) })
	if err != nil {
		return midiStatements{}, err
	}
	out.Unpitched, err = recordStatement(ctx, s, h, pos, id, h.midiStatements, h.MIDI, 4, false, func(v midiState) Evidence[Integer] { return statementEvidence(v.Unpitched) })
	if err != nil {
		return midiStatements{}, err
	}
	out.Volume, err = recordStatement(ctx, s, h, pos, id, h.midiStatements, h.MIDI, 5, false, func(v midiState) Evidence[Decimal] { return statementEvidence(v.Volume) })
	if err != nil {
		return midiStatements{}, err
	}
	out.Pan, err = recordStatement(ctx, s, h, pos, id, h.midiStatements, h.MIDI, 6, false, func(v midiState) Evidence[Decimal] { return statementEvidence(v.Pan) })
	if err != nil {
		return midiStatements{}, err
	}
	out.Elevation, err = recordStatement(ctx, s, h, pos, id, h.midiStatements, h.MIDI, 7, false, func(v midiState) Evidence[Decimal] { return statementEvidence(v.Elevation) })
	if err != nil {
		return midiStatements{}, err
	}
	return out, nil
}
func (h *timelineState) effectiveMIDIDeviceStatements(ctx context.Context, s *Session, pos musicalPosition, id NodeRef) (midiDeviceStatements, error) {
	out := midiDeviceStatements{}
	var err error
	out.Name, err = recordStatement(ctx, s, h, pos, id, h.deviceStatements, h.MIDIDevices, 0, true, func(v midiDeviceState) Evidence[string] { return statementEvidence(v.Name) })
	if err != nil {
		return midiDeviceStatements{}, err
	}
	out.Port, err = recordStatement(ctx, s, h, pos, id, h.deviceStatements, h.MIDIDevices, 1, true, func(v midiDeviceState) Evidence[Integer] { return statementEvidence(v.Port) })
	if err != nil {
		return midiDeviceStatements{}, err
	}
	return out, nil
}

func (h *timelineState) gapMIDIStatements(ctx context.Context, s *Session, gap stateGap) error {
	if _, err := stateReserve(ctx, s, gap); err != nil {
		return err
	}
	h.MIDIUnknownGaps = append(h.MIDIUnknownGaps, stateClone(gap))
	return nil
}
