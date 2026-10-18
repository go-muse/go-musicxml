package validation

import "math/big"

func performanceLocalState(r *contextReader, sr *stateReader, n *factNode, out *TimingContextResult, partOrder *int64) bool {
	if r.err != nil {
		return false
	}
	if sr.err != nil {
		r.err = sr.err
		return false
	}
	publish := false
	if sr.matches(n, timelineRoleMeasure) {
		publish = true
		out.Kind = TimingKindMeasureLabel
		out.MeasureLabel = stateText(r.attribute(n, "perf-measure-label"))
	}
	if sr.matches(n, timelineRoleScorePart) {
		publish = true
		out.Kind = TimingKindPartOrder
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
		out.Kind = TimingKindKey
		v := sr.key(n)
		if sr.err != nil {
			r.err = sr.err
			return true
		}
		if r.err = performanceProjectKey(r.ctx, r.s, out, v); r.err != nil {
			return true
		}
		out.ScopeStaff = stateInteger(sr.attribute(n, timelineRoleKeyNumber))
		out.AllStaves = performanceAllStaves(out.ScopeStaff)
	}
	if sr.matches(n, timelineRoleAttributesTime) {
		publish = true
		out.Kind = TimingKindTime
		v := sr.time(n)
		if sr.err != nil {
			r.err = sr.err
			return true
		}
		if r.err = performanceProjectTime(r.ctx, r.s, out, v); r.err != nil {
			return true
		}
		out.ScopeStaff = stateInteger(sr.attribute(n, timelineRoleTimeNumber))
		out.AllStaves = performanceAllStaves(out.ScopeStaff)
	}
	if sr.matches(n, timelineRoleAttributesTranspose) {
		publish = true
		out.Kind = TimingKindTranspose
		v := sr.transpose(n, false)
		if sr.err != nil {
			r.err = sr.err
			return true
		}
		if r.err = performanceProjectTranspose(r.ctx, r.s, out, v); r.err != nil {
			return true
		}
		out.ScopeStaff = stateInteger(sr.attribute(n, timelineRoleTransposeNumber))
		out.AllStaves = performanceAllStaves(out.ScopeStaff)
	}
	if sr.matches(n, timelineRoleAttributesStaffDetails) {
		publish = true
		out.Kind = TimingKindStaffDetails
		v := sr.staffDetails(n)
		if sr.err != nil {
			r.err = sr.err
			return true
		}
		if r.err = performanceProjectStaffDetails(r.ctx, r.s, out, v); r.err != nil {
			return true
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
