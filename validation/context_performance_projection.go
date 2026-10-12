package validation

import "context"

func performanceProjectKey(ctx context.Context, s *Session, out *TimingContextResult, v Evidence[keyState]) error {
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	out.KeyFifths = stateUnavailable[keyState, Integer](v)
	out.KeyMode = stateUnavailable[keyState, string](v)
	out.KeyCancel = stateUnavailable[keyState, Integer](v)
	out.CancelLocation = stateUnavailable[keyState, string](v)
	if v.State == EvidenceKnown {
		k := v.Value
		out.KeyFifths = k.Fifths
		out.KeyMode = k.Mode
		out.KeyCancel = k.Cancel
		out.CancelLocation = performanceCancelLocation(k)
	}
	return nil
}

func performanceProjectTime(ctx context.Context, s *Session, out *TimingContextResult, v Evidence[timeState]) error {
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	out.TimeSymbol = stateUnavailable[timeState, string](v)
	out.TimeSeparator = stateUnavailable[timeState, string](v)
	out.SenzaMisura = stateUnavailable[timeState, string](v)
	out.TimePrintObject = stateUnavailable[timeState, bool](v)
	if v.State == EvidenceKnown {
		t := v.Value
		out.TimeSymbol = t.displaySymbol().Effective
		out.TimeSeparator = t.displaySeparator().Effective
		out.SenzaMisura = t.SenzaMisura
		out.TimePrintObject = stateWithDefault(t.PrintObject, true, "print-object-yes").Effective
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
	}
	return nil
}

func performanceProjectTranspose(ctx context.Context, s *Session, out *TimingContextResult, v Evidence[transposeState]) error {
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	out.TranspositionChromatic = stateUnavailable[transposeState, Decimal](v)
	out.TranspositionDiatonic = stateUnavailable[transposeState, Integer](v)
	out.TranspositionOctave = stateUnavailable[transposeState, Integer](v)
	out.ChromaticDisplacement = stateUnavailable[transposeState, Rational](v)
	out.DiatonicDisplacement = out.ChromaticDisplacement
	out.DoublingAbove = stateUnavailable[transposeState, bool](v)
	if v.State == EvidenceKnown {
		t := v.Value
		out.TranspositionChromatic = t.Chromatic
		out.TranspositionDiatonic = t.Diatonic
		out.TranspositionOctave = t.OctaveChange
		var err error
		out.ChromaticDisplacement, out.DiatonicDisplacement, err = t.contributions(ctx, s)
		if err != nil {
			return err
		}
		out.DoublingAbove = stateUnavailable[doublingState, bool](t.Double)
		if t.Double.State == EvidenceKnown {
			out.DoublingAbove = t.Double.Value.effectiveAbove().Effective
		}
	}
	return nil
}

func performanceProjectStaffDetails(ctx context.Context, s *Session, out *TimingContextResult, v Evidence[staffDetailsState]) error {
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	out.StaffType = stateUnavailable[staffDetailsState, string](v)
	out.Capo = stateUnavailable[staffDetailsState, Integer](v)
	out.TuningBottomUp = Evidence[bool]{State: EvidenceKnown, Value: true}
	if v.State == EvidenceKnown {
		d := v.Value
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
			var err error
			out.Tunings[i], err = performanceTuning(ctx, s, t, d.Capo)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
