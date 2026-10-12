package validation

import "math/big"

func performanceLocalState(r *contextReader, sr *stateReader, n *factNode, out *TimingContextResult, partOrder *int64) bool {
	publish := false
	if sr.matches(n, timelineRoleMeasure) {
		publish = true
		out.Kind = "measure-label"
		out.MeasureLabel = stateText(r.attribute(n, "perf-measure-label"))
	}
	if sr.matches(n, timelineRoleScorePart) {
		publish = true
		out.Kind = "part-order"
		*partOrder++
		complete := r.enumerated(r.s.nodes[n.header.Parent])
		out.PartOrder = stateUnavailable[bool, Integer](complete)
		if complete.State == EvidenceKnown {
			if r.err = r.s.retain(256); r.err != nil {
				return true
			}
			out.PartOrder.Value, _ = NewInteger(big.NewInt(*partOrder))
		}
	}
	if sr.matches(n, timelineRoleAttributesKey) {
		publish = true
		out.Kind = "key"
		v := sr.key(n)
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
		out.ScopeStaff = stateInteger(sr.attribute(n, timelineRoleKeyNumber))
		out.AllStaves = performanceAllStaves(out.ScopeStaff)
	}
	if sr.matches(n, timelineRoleAttributesTime) {
		publish = true
		out.Kind = "time"
		v := sr.time(n)
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
			if r.err = stateRetainCount(r.s, len(t.Pairs), 512); r.err != nil {
				return true
			}
			out.Meter = make([]TimingMeterPair, len(t.Pairs))
			for i, p := range t.Pairs {
				if !r.step(1) {
					return true
				}
				out.Meter[i] = TimingMeterPair{Beats: p.Beats, BeatType: p.BeatType}
			}
		}
		out.ScopeStaff = stateInteger(sr.attribute(n, timelineRoleTimeNumber))
		out.AllStaves = performanceAllStaves(out.ScopeStaff)
	}
	if sr.matches(n, timelineRoleAttributesTranspose) {
		publish = true
		out.Kind = "transpose"
		v := sr.transpose(n, false)
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
			out.ChromaticDisplacement, out.DiatonicDisplacement, r.err = t.contributions(r.ctx, r.s)
			out.DoublingAbove = stateUnavailable[doublingState, bool](t.Double)
			if t.Double.State == EvidenceKnown {
				out.DoublingAbove = t.Double.Value.effectiveAbove().Effective
			}
		}
		out.ScopeStaff = stateInteger(sr.attribute(n, timelineRoleTransposeNumber))
		out.AllStaves = performanceAllStaves(out.ScopeStaff)
	}
	if sr.matches(n, timelineRoleAttributesStaffDetails) {
		publish = true
		out.Kind = "staff-details"
		v := sr.staffDetails(n)
		out.StaffType = stateUnavailable[staffDetailsState, string](v)
		out.Capo = stateUnavailable[staffDetailsState, Integer](v)
		out.TuningBottomUp = Evidence[bool]{State: EvidenceKnown, Value: true}
		if v.State == EvidenceKnown {
			d := v.Value
			out.StaffType = d.effectiveType().Effective
			out.Capo = d.Capo
			if r.err = stateRetainCount(r.s, len(d.Tunings), 2048); r.err != nil {
				return true
			}
			out.Tunings = make([]TimingTuning, len(d.Tunings))
			for i, t := range d.Tunings {
				if !r.step(1) {
					return true
				}
				out.Tunings[i], r.err = performanceTuning(r.ctx, r.s, t, d.Capo)
				if r.err != nil {
					return true
				}
			}
		}
		out.ScopeStaff = stateInteger(sr.attribute(n, timelineRoleStaffDetailsNumber))
		out.AllStaves = performanceAllStaves(out.ScopeStaff)
	}
	if sr.err != nil && r.err == nil {
		r.err = sr.err
	}
	return publish
}
func performanceAllStaves(staff Evidence[Integer]) Evidence[bool] {
	if staff.State == EvidenceAbsent {
		return Evidence[bool]{State: EvidenceKnown, Value: true}
	}
	if staff.State == EvidenceKnown {
		return Evidence[bool]{State: EvidenceKnown, Value: false}
	}
	return stateUnavailable[Integer, bool](staff)
}
func performanceCancelLocation(k keyState) Evidence[string] {
	if k.Cancel.State == EvidenceAbsent {
		return Evidence[string]{State: EvidenceAbsent}
	}
	return stateWithDefault(k.CancelLocation, "left", "cancel-location-left").Effective
}
