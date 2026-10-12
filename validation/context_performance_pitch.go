package validation

import "context"

func performancePitch(ctx context.Context, s *Session, step Evidence[string], alter Evidence[Decimal], oct Evidence[Integer]) (Evidence[Rational], error) {
	if err := s.step(ctx, 1); err != nil {
		return Evidence[Rational]{}, err
	}
	if step.State != EvidenceKnown {
		return stateUnavailable[string, Rational](step), nil
	}
	if oct.State != EvidenceKnown {
		return stateUnavailable[Integer, Rational](oct), nil
	}
	if alter.State != EvidenceKnown && alter.State != EvidenceAbsent {
		return stateUnavailable[Decimal, Rational](alter), nil
	}
	var base int64
	switch step.Value {
	case "C":
		base = 0
	case "D":
		base = 2
	case "E":
		base = 4
	case "F":
		base = 5
	case "G":
		base = 7
	case "A":
		base = 9
	case "B":
		base = 11
	default:
		return stateUnknown[Rational]("pitch-step-unavailable"), nil
	}
	// Absolute semitones use C0=0, not an inferred MIDI instrument/range.
	o, err := s.stateRationalInteger(ctx, oct.Value)
	if err != nil {
		return Evidence[Rational]{}, err
	}
	o, err = s.rationalMul(ctx, o, timelineOctaveSemitones)
	if err != nil {
		return Evidence[Rational]{}, err
	}
	o, err = s.rationalAdd(ctx, o, performanceRatio(base, 1))
	if err != nil {
		return Evidence[Rational]{}, err
	}
	if alter.State == EvidenceKnown {
		a, e := s.rationalFromDecimal(ctx, alter.Value)
		if e != nil {
			return Evidence[Rational]{}, e
		}
		o, err = s.rationalAdd(ctx, o, a)
	}
	return Evidence[Rational]{State: EvidenceKnown, Value: o}, err
}
func performanceTuning(ctx context.Context, s *Session, t staffTuningState, capo Evidence[Integer]) (TimingTuning, error) {
	out := TimingTuning{Line: t.Line, Octave: t.Octave, Step: t.Step, Alter: t.Alter, Capo: capo}
	var err error
	out.OpenPitch, err = performancePitch(ctx, s, t.Step, t.Alter, t.Octave)
	if err != nil {
		return out, err
	}
	out.AdjustedPitch = stateUnavailable[Integer, Rational](capo)
	if capo.State == EvidenceAbsent {
		out.AdjustedPitch = stateUnknown[Rational]("capo-unspecified")
	}
	if capo.State == EvidenceKnown {
		out.AdjustedPitch = out.OpenPitch
		if out.OpenPitch.State == EvidenceKnown {
			c, e := s.stateRationalInteger(ctx, capo.Value)
			if e != nil {
				return out, e
			}
			out.AdjustedPitch.Value, err = s.rationalAdd(ctx, out.OpenPitch.Value, c)
		}
	}
	return out, err
}
func performanceWrittenPitch(r *contextReader, n *factNode, out *TimingContextResult) {
	out.PitchIntent = stateUnknown[Rational]("external-written-or-sounding-intent-unavailable")
	pitch, c := r.child(n, "perf-pitch")
	if c.State != EvidenceKnown {
		out.WrittenPitch = stateUnavailable[bool, Rational](c)
		return
	}
	out.WrittenStep = stateText(r.childScalar(pitch, "perf-pitch-step"))
	out.WrittenAlter = stateDecimalValue(r.childScalar(pitch, "perf-pitch-alter"))
	out.WrittenOctave = stateInteger(r.childScalar(pitch, "perf-pitch-octave"))
	if r.err != nil {
		return
	}
	out.WrittenPitch, r.err = performancePitch(r.ctx, r.s, out.WrittenStep, out.WrittenAlter, out.WrittenOctave)
	if r.err != nil {
		return
	}
	out.SoundingPitch = out.ChromaticDisplacement
	if out.WrittenPitch.State != EvidenceKnown {
		out.SoundingPitch = out.WrittenPitch
	} else if out.ChromaticDisplacement.State == EvidenceKnown {
		out.SoundingPitch.Value, r.err = r.s.rationalAdd(r.ctx, out.WrittenPitch.Value, out.ChromaticDisplacement.Value)
	}
	if out.ChromaticDisplacement.State == EvidenceAbsent {
		out.SoundingPitch = stateUnknown[Rational]("transposition-unspecified")
	}
}
