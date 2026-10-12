package validation

import "context"

func performanceTimelineFact(ctx context.Context, s *Session, service *timelineService, n *factNode) (TimingContextResult, bool, error) {
	out := newTimingContext("event")
	f, ok := service.Facts[n.header.ID]
	if !ok {
		return out, false, nil
	}
	out.Origin = performancePosition(f.Origin)
	out.Position = performancePosition(f.Position)
	out.End = performancePosition(f.End)
	out.Duration = f.Duration
	out.PrincipalDuration = f.PrincipalDuration
	out.Principal = f.Principal
	out.Units = f.Units
	out.Staff = f.Staff
	out.Voice = f.Voice
	out.Part = f.Part
	if v, ok := service.NoteOrigins[n.header.ID]; ok {
		out.Kind = "note"
		out.ScoreOnset = performancePosition(v.ScoreOnset)
		out.ScoreEnd = performancePosition(v.ScoreEnd)
		out.Chord = v.Chord
		out.Grace = v.Grace
		out.Cue = v.Cue
		out.Attack = performanceAdjustment(v.Attack)
		out.Release = performanceAdjustment(v.Release)
		out.BendsComplete = v.BendsComplete
		if err := stateRetainCount(s, len(v.Bends), 4096); err != nil {
			return out, false, err
		}
		out.Bends = make([]TimingBend, len(v.Bends))
		for i, b := range v.Bends {
			if err := s.step(ctx, 1); err != nil {
				return out, false, err
			}
			out.Bends[i] = TimingBend{Bend: b.Bend, OriginalNote: performancePosition(b.OriginalNote), ReleasePresent: b.ReleasePresent, Release: performanceAdjustment(b.Release), FirstBeat: performancePercent(b.FirstBeat), LastBeat: performancePercent(b.LastBeat), RawBeats: b.RawBeats, Beats: b.Beats, RawAccelerate: b.RawAccelerate, Accelerate: b.Accelerate}
		}
		out.GraceNode = stateUnavailable[graceTimingOrigin, NodeRef](v.GraceTiming)
		if v.GraceTiming.State == EvidenceKnown {
			g := v.GraceTiming.Value
			out.GraceNode = Evidence[NodeRef]{State: EvidenceKnown, Value: g.Grace}
			out.StealPrevious = g.StealPrevious
			out.StealFollowing = g.StealFollowing
			out.MakeTime = TimingAdjustment{Raw: g.MakeTime.Raw, Units: g.MakeTime.Units, Delta: g.MakeTime.Delta}
		}
		if g, ok := service.GraceNeighbors[n.header.ID]; ok {
			out.Previous = performanceDonor(g.Previous)
			out.Following = performanceDonor(g.Following)
		}
	}
	if v, ok := service.FiguredBassOrigins[n.header.ID]; ok {
		out.Kind = "figured-bass"
		out.Anchor = v.Anchor
		out.AnchorPosition = performancePosition(v.AnchorPosition)
		out.AnchorStaff = v.AnchorStaff
		out.AnchorVoice = v.AnchorVoice
		out.DurationRaw = v.DurationRaw
		out.AnchorUnits = v.AnchorUnits
		out.DurationUnits = v.DurationUnits
		out.ChainPosition = performancePosition(v.ChainPosition)
		out.ChainEnd = performancePosition(v.ChainEnd)
		out.GroupStart = v.GroupStart
		out.ChainInterpretation = v.ChainInterpretation
		out.Figures = v.Figures
		out.FiguresComplete = v.FiguresComplete
	}
	if v, ok := service.DirectionOrigins[n.header.ID]; ok {
		out.Kind = "direction"
		out.Anchor = v.Anchor
		out.AnchorPosition = performancePosition(v.AnchorPosition)
		out.AnchorStaff = v.AnchorStaff
		out.AnchorVoice = v.AnchorVoice
	}
	if v, ok := service.Playback.Sounds[n.header.ID]; ok {
		out.Kind = "sound"
		out.PlayComplete = v.PlayComplete
		var e error
		out.Play, e = performancePlayInstructions(ctx, s, v.Play)
		if e != nil {
			return out, false, e
		}
		out.Tempo = v.Tempo
		out.Dynamics = v.Dynamics
		out.Pizzicato = v.Pizzicato
		out.TimeOnly = v.TimeOnly
		out.Pan = v.Pan
		out.Elevation = v.Elevation
	}
	if v, ok := service.Playback.Notes[n.header.ID]; ok {
		out.PlayComplete = v.PlayComplete
		var e error
		out.Play, e = performancePlayInstructions(ctx, s, v.Play)
		if e != nil {
			return out, false, e
		}
		out.Dynamics = v.Dynamics
		out.EndDynamics = v.EndDynamics
		out.Pizzicato = v.Pizzicato
		out.TimeOnly = v.TimeOnly
	}
	var err error
	out.ReferenceVelocity, err = performanceVelocity(ctx, s, out.Dynamics)
	if err != nil {
		return out, false, err
	}
	out.EndReferenceVelocity, err = performanceVelocity(ctx, s, out.EndDynamics)
	if err == nil {
		err = performanceSoundSpatial(ctx, s, service, n, &out)
	}
	return out, true, err
}

// Effective values are queried at accepted positions and sparse staff identity;
// an unavailable coordinate does not mutate the known local event fields.
func performanceEffectiveState(ctx context.Context, s *Session, service *timelineService, out *TimingContextResult) error {
	out.StaffTopDown = Evidence[bool]{State: EvidenceKnown, Value: true}
	out.TuningBottomUp = Evidence[bool]{State: EvidenceKnown, Value: true}
	if out.Origin.State != EvidenceKnown {
		return nil
	}
	v := out.Origin.Value
	pos := musicalPosition{Part: v.Part, Measure: v.Measure, MeasureOrdinal: v.MeasureOrdinal, Offset: v.Offset, UnprovedFrame: v.UnprovedFrame}
	h := &service.State
	tempo, err := service.Playback.effectiveTempo(ctx, s, h, pos)
	if err != nil {
		return err
	}
	out.EffectiveTempo = tempo.Value
	if err = performanceStateOrigin(ctx, s, out, "tempo", tempo); err != nil {
		return err
	}
	dynamics, err := service.Playback.effectiveDynamics(ctx, s, h, pos)
	if err != nil {
		return err
	}
	out.EffectiveDynamics = dynamics.Value
	if err = performanceStateOrigin(ctx, s, out, "dynamics", dynamics); err != nil {
		return err
	}
	pizzicato, err := service.Playback.effectivePizzicato(ctx, s, h, pos)
	if err != nil {
		return err
	}
	out.EffectivePizzicato = pizzicato.Value
	if err = performanceStateOrigin(ctx, s, out, "pizzicato", pizzicato); err != nil {
		return err
	}
	if out.Pizzicato.State != EvidenceAbsent {
		out.EffectivePizzicato = out.Pizzicato
	}
	if out.Dynamics.State != EvidenceAbsent {
		out.EffectiveDynamics = out.Dynamics
	}
	out.EffectiveVelocity, err = performanceVelocity(ctx, s, out.EffectiveDynamics)
	if err != nil {
		return err
	}
	staves, err := h.effectiveStaves(ctx, s, pos)
	if err != nil {
		return err
	}
	out.Staves = staves.Value
	if err = performanceStateOrigin(ctx, s, out, "staves", staves); err != nil {
		return err
	}
	instruments, err := h.effectiveInstruments(ctx, s, pos)
	if err != nil {
		return err
	}
	out.Instruments = instruments.Value
	if err = performanceStateOrigin(ctx, s, out, "instruments", instruments); err != nil {
		return err
	}
	staff, err := h.resolveStaff(ctx, s, pos, out.Staff)
	if err != nil {
		return err
	}
	out.StaffExists = stateUnavailable[Integer, bool](staff)
	if staff.State == EvidenceKnown && out.Staves.State == EvidenceKnown {
		a, err := s.stateRationalInteger(ctx, staff.Value)
		if err != nil {
			return err
		}
		b, err := s.stateRationalInteger(ctx, out.Staves.Value)
		if err != nil {
			return err
		}
		cmp, err := s.rationalCmp(ctx, a, b)
		if err != nil {
			return err
		}
		out.StaffExists = Evidence[bool]{State: EvidenceKnown, Value: cmp <= 0}
	}
	key, err := h.effectiveKey(ctx, s, pos, out.Staff)
	if err != nil {
		return err
	}
	if err = performanceStateOrigin(ctx, s, out, "key", key); err != nil {
		return err
	}
	if key.Value.State == EvidenceKnown {
		k := key.Value.Value
		out.KeyFifths = k.Fifths
		out.KeyMode = k.Mode
		out.KeyCancel = k.Cancel
		out.CancelLocation = performanceCancelLocation(k)
	} else {
		out.KeyFifths = stateUnavailable[keyState, Integer](key.Value)
		out.KeyMode = stateUnavailable[keyState, string](key.Value)
		out.KeyCancel = stateUnavailable[keyState, Integer](key.Value)
		out.CancelLocation = stateUnavailable[keyState, string](key.Value)
	}
	tim, err := queryState(ctx, s, h, &h.Times, pos, staff, stateWhole[timeState])
	if err != nil {
		return err
	}
	if err = performanceStateOrigin(ctx, s, out, "tim", tim); err != nil {
		return err
	}
	if tim.Value.State == EvidenceKnown {
		t := tim.Value.Value
		if err := stateRetainCount(s, len(t.Pairs), 512); err != nil {
			return err
		}
		out.Meter = make([]TimingMeterPair, len(t.Pairs))
		for i, p := range t.Pairs {
			if err := s.step(ctx, 1); err != nil {
				return err
			}
			out.Meter[i] = TimingMeterPair{Beats: p.Beats, BeatType: p.BeatType}
		}
		out.TimeSymbol = t.displaySymbol().Effective
		out.TimeSeparator = t.displaySeparator().Effective
		out.SenzaMisura = t.SenzaMisura
		out.TimePrintObject = stateWithDefault(t.PrintObject, true, "print-object-yes").Effective
	} else {
		out.TimeSymbol = stateUnavailable[timeState, string](tim.Value)
		out.TimeSeparator = stateUnavailable[timeState, string](tim.Value)
		out.SenzaMisura = stateUnavailable[timeState, string](tim.Value)
		out.TimePrintObject = stateUnavailable[timeState, bool](tim.Value)
	}
	clef, err := queryState(ctx, s, h, &h.Clefs, pos, staff, stateWhole[clefState])
	if err != nil {
		return err
	}
	if err = performanceStateOrigin(ctx, s, out, "clef", clef); err != nil {
		return err
	}
	if clef.Value.State == EvidenceKnown {
		out.ClefSign = clef.Value.Value.Sign
		out.ClefLine = clef.Value.Value.Line
		out.ClefOctave = clef.Value.Value.OctaveChange
	} else {
		out.ClefSign = stateUnavailable[clefState, string](clef.Value)
		out.ClefLine = stateUnavailable[clefState, Integer](clef.Value)
		out.ClefOctave = stateUnavailable[clefState, Integer](clef.Value)
	}
	trans, err := queryState(ctx, s, h, &h.Transpose, pos, staff, stateWhole[transposeState])
	if err != nil {
		return err
	}
	if err = performanceStateOrigin(ctx, s, out, "trans", trans); err != nil {
		return err
	}
	if trans.Value.State == EvidenceKnown {
		t := trans.Value.Value
		out.TranspositionChromatic = t.Chromatic
		out.TranspositionDiatonic = t.Diatonic
		out.TranspositionOctave = t.OctaveChange
		out.ChromaticDisplacement, out.DiatonicDisplacement, err = t.contributions(ctx, s)
		if err != nil {
			return err
		}
		out.DoublingAbove = stateUnavailable[doublingState, bool](t.Double)
		if t.Double.State == EvidenceKnown {
			out.DoublingAbove = t.Double.Value.effectiveAbove().Effective
		}
	} else {
		out.TranspositionChromatic = stateUnavailable[transposeState, Decimal](trans.Value)
		out.TranspositionDiatonic = stateUnavailable[transposeState, Integer](trans.Value)
		out.TranspositionOctave = stateUnavailable[transposeState, Integer](trans.Value)
		out.ChromaticDisplacement = stateUnavailable[transposeState, Rational](trans.Value)
		out.DiatonicDisplacement = stateUnavailable[transposeState, Rational](trans.Value)
		out.DoublingAbove = stateUnavailable[transposeState, bool](trans.Value)
	}
	details, err := h.effectiveStaffDetails(ctx, s, pos, out.Staff)
	if err != nil {
		return err
	}
	if err = performanceStateOrigin(ctx, s, out, "details", details); err != nil {
		return err
	}
	if details.Value.State == EvidenceKnown {
		d := details.Value.Value
		out.StaffType = d.effectiveType().Effective
		out.Capo = d.Capo
		if err := stateRetainCount(s, len(d.Tunings), 2048); err != nil {
			return err
		}
		out.Tunings = make([]TimingTuning, len(d.Tunings))
		for i, t := range d.Tunings {
			if err := s.step(ctx, 1); err != nil {
				return err
			}
			out.Tunings[i], err = performanceTuning(ctx, s, t, d.Capo)
			if err != nil {
				return err
			}
		}
	} else {
		out.StaffType = stateUnavailable[staffDetailsState, string](details.Value)
		out.Capo = stateUnavailable[staffDetailsState, Integer](details.Value)
	}
	return nil
}
