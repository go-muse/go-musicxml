package validation

import "context"

func evaluateContextPerformance(ctx context.Context, s *Session, _ *factNode, key RuleKey, p *contextPlan, final bool) (Decision, bool, error) {
	if !final {
		return Decision{}, true, nil
	}
	tb, err := p.Schema.sharedTimelineBindings()
	if err != nil {
		return Decision{}, false, err
	}
	sr := &stateReader{ctx: ctx, s: s, b: tb}
	partOrder := int64(0)
	r := &contextReader{ctx: ctx, s: s, b: p.Bindings}
	service := s.timeline[p.Timeline]
	swingHistory, err := performanceBuildSwing(r, service)
	if err != nil {
		return Decision{}, false, err
	}
	for _, id := range s.sequence {
		if !r.step(1) {
			break
		}
		n := s.nodes[id]
		performanceStatic(r, key, n)
		if r.err != nil {
			break
		}
		out := newTimingContext("performance")
		publish := false
		if service != nil && service.Final {
			var err error
			out, publish, err = performanceTimelineFact(ctx, s, service, n)
			if err != nil {
				return Decision{}, false, err
			}
			if publish {
				if err = performanceEffectiveState(ctx, s, service, &out); err != nil {
					return Decision{}, false, err
				}
			}
		}
		if r.matches(n, "perf-note") {
			if !publish {
				performanceUnavailableEvent(s, key, &out)
			}
			publish = true
			out.Kind = "note"
			cue, c := r.child(n, "perf-cue")
			out.Cue = performancePresence(cue, c)
			grace, g := r.child(n, "perf-grace")
			out.Grace = performancePresence(grace, g)
			performanceWrittenPitch(r, n, &out)
			performanceNoteSwing(r, service, &swingHistory, n, &out)
		}
		if r.matches(n, "perf-sound") {
			if !publish {
				out.PlayComplete = Evidence[bool]{State: EvidenceUnsupported, Reason: "timeline-service-unavailable"}
			}
			publish = true
			out.Kind = "sound"
			out.Tempo = stateDecimalValue(r.attribute(n, "perf-tempo"))
			out.Dynamics = stateDecimalValue(r.attribute(n, "perf-dynamics"))
			if r.err == nil {
				out.ReferenceVelocity, r.err = performanceVelocity(ctx, s, out.Dynamics)
			}
		}
		if r.matches(n, "perf-midi-unpitched") {
			publish = true
			out.Kind = "midi-unpitched"
			out.MIDIUnpitched = stateInteger(r.scalar(n))
			out.MIDINote = stateUnavailable[Integer, Rational](out.MIDIUnpitched)
			if out.MIDIUnpitched.State == EvidenceKnown {
				v, e := s.stateRationalInteger(ctx, out.MIDIUnpitched.Value)
				r.err = e
				if e == nil {
					out.MIDINote.Value, r.err = s.rationalSub(ctx, v, performanceRatio(1, 1))
				}
			}
		}
		if r.matches(n, "perf-pan") {
			publish = true
			out.Kind = "pan"
			out.Pan = stateDecimalValue(r.scalar(n))
		}
		if r.matches(n, "perf-elevation") {
			publish = true
			out.Kind = "elevation"
			out.Elevation = stateDecimalValue(r.scalar(n))
		}
		if r.matches(n, "perf-swing") {
			publish = true
			out.Kind = "swing"
			straight, c := r.child(n, "perf-straight")
			out.Straight = performancePresence(straight, c)
			out.SwingType = stateWithDefault(stateText(r.childScalar(n, "perf-swing-type")), "eighth", "swing-type-eighth").Effective
			out.SwingFirst = stateInteger(r.childScalar(n, "perf-swing-first"))
			out.SwingSecond = stateInteger(r.childScalar(n, "perf-swing-second"))
		}
		if r.matches(n, "perf-bend") {
			publish = true
			out.Kind = "bend"
			out.BendAlter = stateDecimalValue(r.childScalar(n, "perf-bend-alter"))
			pre, c := r.child(n, "perf-pre-bend")
			out.PreBend = performancePresence(pre, c)
			rel, c := r.child(n, "perf-release")
			out.BendRelease = performancePresence(rel, c)
			proof := performanceBendRelation(r, n)
			out.BendNegativeApplicable = proof.Applicable
			out.BendNegativeSatisfied = proof.Satisfied
			storePerformanceBend(r, key, n, proof)
		}
		if performanceLocalState(r, sr, n, &out, &partOrder) {
			publish = true
		}
		if performanceLocal(r, key, n, &out) {
			publish = true
		}
		if performanceMovement(r, service, n, &out) {
			publish = true
		}
		if out.Kind == "swing" && r.err == nil {
			var reduced Evidence[bool]
			out.SwingRatio, reduced, r.err = performanceSwingRatio(ctx, s, out.SwingFirst, out.SwingSecond)
			if r.err == nil {
				r.err = performanceAdvice(ctx, s, &out, "swing-ratio-reduced", reduced)
			}
		}
		if out.Kind == "sound" && r.err == nil {
			grouped := performanceInstrumentGrouping(sr, n, out.Part)
			if sr.err != nil {
				r.err = sr.err
			} else {
				r.err = performanceAdvice(ctx, s, &out, "sound-instrument-grouping", grouped)
			}
		}
		if publish && service != nil && service.Final && r.err == nil {
			r.err = performanceStateAdvice(ctx, s, service, n, &out)
		}
		if r.err != nil {
			break
		}
		if publish {
			r.err = publishPerformanceTiming(ctx, s, key, n, &out)
			if r.err != nil {
				break
			}
		}
	}
	return Decision{Outcome: OutcomePass}, false, r.err
}
func performancePresence(n *factNode, e Evidence[bool]) Evidence[bool] {
	if e.State == EvidenceAbsent {
		return Evidence[bool]{State: EvidenceKnown, Value: false}
	}
	if e.State != EvidenceKnown {
		return e
	}
	return Evidence[bool]{State: EvidenceKnown, Value: n != nil}
}

func performanceUnavailableEvent(s *Session, key RuleKey, out *TimingContextResult) {
	gap := Evidence[bool]{State: EvidenceUnsupported, Reason: "timeline-service-unavailable"}
	if service := s.contextServices[key]; service != nil && service.Timeline.State != EvidenceKnown && service.Timeline.State != EvidenceAbsent {
		gap = service.Timeline
	}
	out.Origin = stateUnavailable[bool, ContextPosition](gap)
	out.Position = out.Origin
	out.End = out.Origin
	out.ScoreOnset = out.Origin
	out.ScoreEnd = out.Origin
	out.PlayComplete = gap
	out.Duration = stateUnavailable[bool, Rational](gap)
	out.PrincipalDuration = out.Duration
	out.Principal = stateUnavailable[bool, NodeRef](gap)
	out.Units = stateUnavailable[bool, Decimal](gap)
	out.Part = stateUnavailable[bool, NodeRef](gap)
	out.Staff = stateUnavailable[bool, Integer](gap)
	out.Voice = stateUnavailable[bool, string](gap)
	out.ChromaticDisplacement = out.Duration
	out.DiatonicDisplacement = out.Duration
	out.Staves = out.Staff
	out.Instruments = out.Staff
}
