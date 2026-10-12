package validation

type performanceSwing struct {
	Type          Evidence[string]
	First, Second Evidence[Integer]
	Straight      Evidence[bool]
	Source        NodeRef
}
type performanceSwingHistory struct {
	history stateHistory[performanceSwing]
	global  Evidence[bool]
}

func performanceReadSwing(r *contextReader, n *factNode) performanceSwing {
	straight, c := r.child(n, "perf-straight")
	return performanceSwing{Type: stateWithDefault(stateText(r.childScalar(n, "perf-swing-type")), "eighth", "swing-type-eighth").Effective, First: stateInteger(r.childScalar(n, "perf-swing-first")), Second: stateInteger(r.childScalar(n, "perf-swing-second")), Straight: performancePresence(straight, c), Source: NodeRef{r.s.documentID, n.header.ID}}
}
func performanceBuildSwing(r *contextReader, service *timelineService) (performanceSwingHistory, error) {
	out := performanceSwingHistory{global: Evidence[bool]{State: EvidenceKnown, Value: true}}
	if service == nil || !service.Final {
		out.global = Evidence[bool]{State: EvidenceUnsupported, Reason: "timeline-service-unavailable"}
		return out, nil
	}
	for _, id := range r.s.sequence {
		if !r.step(1) {
			return out, r.err
		}
		n := r.s.nodes[id]
		if !r.matches(n, "perf-sound") {
			continue
		}
		swing, c := r.child(n, "perf-swing")
		if c.State == EvidenceAbsent {
			continue
		}
		origin, ok := service.Playback.Sounds[id]
		if !ok {
			out.global = stateUnknown[bool]("swing-origin-unavailable")
			continue
		}
		v := stateUnavailable[bool, performanceSwing](c)
		if c.State == EvidenceKnown {
			v = Evidence[performanceSwing]{State: EvidenceKnown, Value: performanceReadSwing(r, swing)}
		}
		if r.err != nil {
			return out, r.err
		}
		if origin.Origin.Position.State != EvidenceKnown || origin.Origin.Part.State != EvidenceKnown {
			if origin.Origin.Part.State != EvidenceKnown {
				out.global = stateUnavailable[NodeRef, bool](origin.Origin.Part)
				continue
			}
			gap := stateUnavailable[musicalPosition, bool](origin.Origin.Position)
			if gap.State == EvidenceAbsent || gap.State == EvidenceKnown {
				gap = stateUnknown[bool]("swing-origin-unavailable")
			}
			if err := addStateGap(r.ctx, r.s, &out.history, stateGap{Placement: origin.Origin.Placement, Part: origin.Origin.Part.Value, FirstMeasure: origin.Origin.MeasureOrdinal, LastMeasure: ^uint64(0), Scope: stateScope{AllStaves: true}, Availability: gap}); err != nil {
				return out, err
			}
			continue
		}
		condition := origin.TimeOnly
		p := statePublication[performanceSwing]{Position: origin.Origin.Position.Value, Scope: stateScope{AllStaves: true}, Value: v, Provenance: []stateProvenance{{Node: NodeRef{r.s.documentID, id}, Kind: stateExplicit}}, TimeOnly: &condition}
		if err := publishState(r.ctx, r.s, &out.history, p); err != nil {
			return out, err
		}
	}
	return out, r.err
}
func performanceNoteSwing(r *contextReader, service *timelineService, h *performanceSwingHistory, n *factNode, out *TimingContextResult) {
	typ := stateText(r.childScalar(n, "perf-type"))
	out.SwingRealized = stateUnknown[bool]("swing-pair-pass-unavailable")
	// Known false nominal or grace/type antecedents need no active chronology.
	local, err := performanceSwingEligible(r.ctx, r.s, typ, stateUnknown[string]("active-swing-unavailable"), out.Duration, out.Grace)
	if err != nil {
		r.err = err
		return
	}
	if local.State == EvidenceKnown && !local.Value {
		out.SwingEligible = local
		out.SwingRealized = local
		return
	}

	if h.global.State != EvidenceKnown {
		out.SwingEligible = h.global
		return
	}
	if out.Origin.State != EvidenceKnown {
		out.SwingEligible = stateUnavailable[ContextPosition, bool](out.Origin)
		return
	}
	p := out.Origin.Value
	pos := musicalPosition{Part: p.Part, Measure: p.Measure, MeasureOrdinal: p.MeasureOrdinal, Offset: p.Offset, UnprovedFrame: p.UnprovedFrame}
	active, err := queryStateAtScope(r.ctx, r.s, &service.State, &h.history, pos, Evidence[Integer]{State: EvidenceAbsent}, stateWhole[performanceSwing], true, true, true, nil)
	if err != nil {
		r.err = err
		return
	}
	if active.Value.State != EvidenceKnown {
		out.SwingEligible = stateUnavailable[performanceSwing, bool](active.Value)
		if active.Value.State == EvidenceAbsent {
			out.SwingEligible = Evidence[bool]{State: EvidenceKnown, Value: false}
			out.SwingRealized = Evidence[bool]{State: EvidenceKnown, Value: false}
		}
		return
	}
	v := active.Value.Value
	out.SwingType = v.Type
	out.SwingFirst = v.First
	out.SwingSecond = v.Second
	out.Straight = v.Straight
	if v.Straight.State != EvidenceKnown {
		out.SwingEligible = v.Straight
		return
	}
	if v.Straight.Value {
		out.SwingEligible = Evidence[bool]{State: EvidenceKnown, Value: false}
		out.SwingRealized = Evidence[bool]{State: EvidenceKnown, Value: false}
		return
	}
	out.SwingEligible, r.err = performanceSwingEligible(r.ctx, r.s, typ, v.Type, out.Duration, out.Grace)
	if out.SwingEligible.State == EvidenceKnown && !out.SwingEligible.Value {
		out.SwingRealized = Evidence[bool]{State: EvidenceKnown, Value: false}
	}
}
