package validation

import "context"

// NavigationStateValue retains the source of one sparse effective statement.
type NavigationStateValue[T any] struct {
	Value  Evidence[T]
	Source NodeRef
}

// NavigationVirtualInstrument is one atomic descriptor, including known omission.
type NavigationVirtualInstrument struct{ Library, Name Evidence[string] }

// NavigationPerformance retains the exclusive solo/ensemble statement.
type NavigationPerformance struct {
	Solo          bool
	EnsembleEmpty bool
	Ensemble      Integer
}

// NavigationInstrumentState contains independent encoded fields at an occurrence.
// It is not a physical synthesizer configuration or a hardware reset.
type NavigationInstrumentState struct {
	Instrument                        Evidence[NodeRef]
	Sound                             NavigationStateValue[string]
	Performance                       NavigationStateValue[NavigationPerformance]
	Virtual                           NavigationStateValue[NavigationVirtualInstrument]
	Channel, Bank, Program, Unpitched NavigationStateValue[Integer]
	Name, DeviceName                  NavigationStateValue[string]
	DevicePort                        NavigationStateValue[Integer]
	Volume, Pan, Elevation            NavigationStateValue[Decimal]
}

type navigationInstrumentUpdate struct {
	Source                            NodeRef
	Instrument                        Evidence[NodeRef]
	AllInstruments                    bool
	Sound                             Evidence[string]
	Performance                       Evidence[NavigationPerformance]
	Virtual                           Evidence[NavigationVirtualInstrument]
	Channel, Bank, Program, Unpitched Evidence[Integer]
	Name, DeviceName                  Evidence[string]
	DevicePort                        Evidence[Integer]
	Volume, Pan, Elevation            Evidence[Decimal]
}

func navigationField[T any](destination *NavigationStateValue[T], source NodeRef, value Evidence[T], guard Evidence[bool]) {
	v := navigationConditionalValue(value, guard)
	if v.State != EvidenceAbsent {
		destination.Value = v
		destination.Source = source
	}
}
func navigationApplyInstrumentFields(current *NavigationInstrumentState, u navigationInstrumentUpdate, guard Evidence[bool]) {
	navigationField(&current.Sound, u.Source, u.Sound, guard)
	navigationField(&current.Performance, u.Source, u.Performance, guard)
	navigationField(&current.Virtual, u.Source, u.Virtual, guard)
	navigationField(&current.Channel, u.Source, u.Channel, guard)
	navigationField(&current.Bank, u.Source, u.Bank, guard)
	navigationField(&current.Program, u.Source, u.Program, guard)
	navigationField(&current.Unpitched, u.Source, u.Unpitched, guard)
	navigationField(&current.Name, u.Source, u.Name, guard)
	navigationField(&current.DeviceName, u.Source, u.DeviceName, guard)
	navigationField(&current.DevicePort, u.Source, u.DevicePort, guard)
	navigationField(&current.Volume, u.Source, u.Volume, guard)
	navigationField(&current.Pan, u.Source, u.Pan, guard)
	navigationField(&current.Elevation, u.Source, u.Elevation, guard)
}
func navigationApplyInstruments(ctx context.Context, s *Session, current []NavigationInstrumentState, updates []navigationInstrumentUpdate, guard Evidence[bool]) ([]NavigationInstrumentState, error) {
	if guard.State == EvidenceKnown && !guard.Value {
		return current, nil
	}
	for _, u := range updates {
		if err := s.step(ctx, 1); err != nil {
			return nil, err
		}
		found := false
		for i := range current {
			if err := s.step(ctx, 1); err != nil {
				return nil, err
			}
			localGuard := guard
			match := u.AllInstruments
			if !match && u.Instrument.State == EvidenceKnown && current[i].Instrument.State == EvidenceKnown {
				var err error
				match, err = stateSameRef(ctx, s, u.Instrument.Value, current[i].Instrument.Value)
				if err != nil {
					return nil, err
				}
			}
			if !match && u.Instrument.State != EvidenceKnown && !u.AllInstruments {
				localGuard = stateMergeGap(localGuard, stateUnavailable[NodeRef, bool](u.Instrument))
				if localGuard.State == EvidenceAbsent || localGuard.State == "" {
					localGuard = stateUnknown[bool]("navigation-instrument-target-unavailable")
				}
				match = true
			}
			if match {
				navigationApplyInstrumentFields(&current[i], u, localGuard)
				found = true
			}
		}
		if !found {
			value := NavigationInstrumentState{Instrument: u.Instrument}
			navigationApplyInstrumentFields(&value, u, guard)
			if _, err := stateReserve(ctx, s, value); err != nil {
				return nil, err
			}
			current = append(current, stateClone(value))
		}
	}
	return current, nil
}

func navigationInstrumentRaw(source NodeRef, id Evidence[NodeRef], value Evidence[instrumentState]) navigationInstrumentUpdate {
	out := navigationInstrumentUpdate{Source: source, Instrument: id}
	if value.State != EvidenceKnown {
		out.Sound = stateUnavailable[instrumentState, string](value)
		out.Performance = stateUnavailable[instrumentState, NavigationPerformance](value)
		out.Virtual = stateUnavailable[instrumentState, NavigationVirtualInstrument](value)
		return out
	}
	out.Sound = value.Value.Sound
	p := instrumentPerformanceStatement(value.Value)
	out.Performance = stateUnavailable[instrumentPerformanceState, NavigationPerformance](p)
	if p.State == EvidenceKnown {
		out.Performance.Value = NavigationPerformance{Solo: p.Value.Solo, EnsembleEmpty: p.Value.Ensemble.Empty, Ensemble: p.Value.Ensemble.Number}
	}
	out.Virtual = stateUnavailable[virtualInstrumentState, NavigationVirtualInstrument](value.Value.Virtual)
	if value.Value.Virtual.State == EvidenceKnown {
		out.Virtual.Value = NavigationVirtualInstrument{Library: value.Value.Virtual.Value.Library, Name: value.Value.Virtual.Value.Name}
	}
	return out
}

func navigationSoundOwner(ctx context.Context, s *Session, timeline *timelineService, id NodeID) (NodeID, error) {
	for id != 0 {
		if err := s.step(ctx, 1); err != nil {
			return 0, err
		}
		if _, ok := timeline.Playback.Sounds[id]; ok {
			return id, nil
		}
		node := s.nodes[id]
		if node == nil {
			return 0, nil
		}
		id = node.header.Parent
	}
	return 0, nil
}

func navigationAttachInstrumentUpdates(ctx context.Context, s *Session, timeline *timelineService, rows []contextNavigationNode) ([]navigationInstrumentUpdate, error) {
	var initial []navigationInstrumentUpdate
	if err := s.retain(256); err != nil {
		return nil, err
	}
	index := make(map[NodeID]int)
	for i, row := range rows {
		if err := s.step(ctx, 1); err != nil {
			return nil, err
		}
		if _, ok := timeline.Playback.Sounds[row.event.Source.Node]; ok {
			if err := s.retain(128); err != nil {
				return nil, err
			}
			index[row.event.Source.Node] = i
		}
	}
	add := func(u navigationInstrumentUpdate, initialPart NodeRef) error {
		if _, err := stateReserve(ctx, s, u); err != nil {
			return err
		}
		if initialPart.Node != 0 {
			if len(rows) > 0 && rows[0].part.State == EvidenceKnown && rows[0].part.Value == initialPart {
				initial = append(initial, stateClone(u))
			}
			return nil
		}
		owner, err := navigationSoundOwner(ctx, s, timeline, u.Source.Node)
		if err != nil {
			return err
		}
		if i, ok := index[owner]; ok {
			rows[i].event.Instruments = append(rows[i].event.Instruments, stateClone(u))
		}
		return nil
	}
	for _, d := range timeline.State.Definitions {
		if err := add(navigationInstrumentRaw(d.Instrument, Evidence[NodeRef]{State: EvidenceKnown, Value: d.Instrument}, d.Value), d.Part); err != nil {
			return nil, err
		}
	}
	for _, changes := range timeline.State.InstrumentChanges {
		for _, p := range changes.History.Publications {
			if err := s.step(ctx, 1); err != nil {
				return nil, err
			}
			if len(p.Provenance) != 1 {
				continue
			}
			if err := add(navigationInstrumentRaw(p.Provenance[0].Node, Evidence[NodeRef]{State: EvidenceKnown, Value: changes.Instrument}, p.Value), NodeRef{}); err != nil {
				return nil, err
			}
		}
	}
	for _, r := range timeline.State.UnresolvedInstrumentChanges {
		if err := add(navigationInstrumentRaw(r.Source, r.Instrument, r.Value), NodeRef{}); err != nil {
			return nil, err
		}
	}
	for _, r := range timeline.State.MIDI {
		u := navigationInstrumentUpdate{Source: r.Source, Instrument: r.Instrument}
		if r.Value.State == EvidenceKnown {
			v := r.Value.Value
			u.Channel, u.Bank, u.Program, u.Unpitched = v.Channel, v.Bank, v.Program, v.Unpitched
			u.Name, u.Volume, u.Pan, u.Elevation = v.Name, v.Volume, v.Pan, v.Elevation
		} else {
			u.Channel = stateUnavailable[midiState, Integer](r.Value)
			u.Bank = u.Channel
			u.Program = u.Channel
			u.Unpitched = u.Channel
			u.Name = stateUnavailable[midiState, string](r.Value)
			u.Volume = stateUnavailable[midiState, Decimal](r.Value)
			u.Pan = u.Volume
			u.Elevation = u.Volume
		}
		part := NodeRef{}
		if r.Position.State == EvidenceAbsent {
			part = r.Part
		}
		if err := add(u, part); err != nil {
			return nil, err
		}
	}
	for _, r := range timeline.State.MIDIDevices {
		u := navigationInstrumentUpdate{Source: r.Source, Instrument: r.Instrument, AllInstruments: r.Instrument.State == EvidenceAbsent}
		if r.Value.State == EvidenceKnown {
			u.DeviceName, u.DevicePort = r.Value.Value.Name, r.Value.Value.Port
		} else {
			u.DeviceName = stateUnavailable[midiDeviceState, string](r.Value)
			u.DevicePort = stateUnavailable[midiDeviceState, Integer](r.Value)
		}
		part := NodeRef{}
		if r.Position.State == EvidenceAbsent {
			part = r.Part
		}
		if err := add(u, part); err != nil {
			return nil, err
		}
	}
	return initial, nil
}

// Unchanged initial declarations survive a control re-entry. A field whose
// provenance is an executed change has no proved carry-versus-reset policy.
func navigationInvalidateField[T any](ctx context.Context, s *Session, v *NavigationStateValue[T], initial [][]navigationInstrumentUpdate) error {
	if v.Value.State == EvidenceAbsent || v.Value.State == "" {
		return nil
	}
	for _, group := range initial {
		for _, u := range group {
			same, err := stateSameRef(ctx, s, v.Source, u.Source)
			if err != nil {
				return err
			}
			if same {
				return nil
			}
		}
	}
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	v.Value = stateUnknown[T]("navigation-reentry-state-unresolved")
	return nil
}
func navigationInvalidateInstrument(ctx context.Context, s *Session, v *NavigationInstrumentState, initial [][]navigationInstrumentUpdate) error {
	if err := navigationInvalidateField(ctx, s, &v.Sound, initial); err != nil {
		return err
	}
	if err := navigationInvalidateField(ctx, s, &v.Performance, initial); err != nil {
		return err
	}
	if err := navigationInvalidateField(ctx, s, &v.Virtual, initial); err != nil {
		return err
	}
	for _, x := range []*NavigationStateValue[Integer]{&v.Channel, &v.Bank, &v.Program, &v.Unpitched, &v.DevicePort} {
		if err := navigationInvalidateField(ctx, s, x, initial); err != nil {
			return err
		}
	}
	for _, x := range []*NavigationStateValue[string]{&v.Name, &v.DeviceName} {
		if err := navigationInvalidateField(ctx, s, x, initial); err != nil {
			return err
		}
	}
	for _, x := range []*NavigationStateValue[Decimal]{&v.Volume, &v.Pan, &v.Elevation} {
		if err := navigationInvalidateField(ctx, s, x, initial); err != nil {
			return err
		}
	}
	return nil
}
