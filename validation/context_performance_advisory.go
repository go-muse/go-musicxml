package validation

import "context"

// TimingAdvisory is optional guidance. A false value is never a mandatory
// conformance failure, and unavailable evidence does not imply compliance.
type TimingAdvisory struct {
	Clause    Code
	Satisfied Evidence[bool]
}

func performanceAdvice(ctx context.Context, s *Session, out *TimingContextResult, clause Code, value Evidence[bool]) error {
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	if err := s.retain(512 + evidenceLogicalBytes(value)); err != nil {
		return err
	}
	out.Advisories = append(out.Advisories, TimingAdvisory{clause, value})
	return nil
}
func cloneTimingAdvice(raw []TimingAdvisory) []TimingAdvisory {
	out := make([]TimingAdvisory, len(raw))
	for i, v := range raw {
		v.Satisfied = cloneEvidence(v.Satisfied)
		out[i] = v
	}
	return out
}
func performanceSwingRatio(ctx context.Context, s *Session, first, second Evidence[Integer]) (Evidence[Rational], Evidence[bool], error) {
	if first.State != EvidenceKnown {
		return stateUnavailable[Integer, Rational](first), stateUnavailable[Integer, bool](first), nil
	}
	if second.State != EvidenceKnown {
		return stateUnavailable[Integer, Rational](second), stateUnavailable[Integer, bool](second), nil
	}
	a, err := s.stateRationalInteger(ctx, first.Value)
	if err != nil {
		return Evidence[Rational]{}, Evidence[bool]{}, err
	}
	b, err := s.stateRationalInteger(ctx, second.Value)
	if err != nil {
		return Evidence[Rational]{}, Evidence[bool]{}, err
	}
	q, err := s.rationalQuo(ctx, a, b)
	if err != nil {
		return Evidence[Rational]{}, Evidence[bool]{}, err
	}
	if err = s.step(ctx, first.Value.ByteSize()+second.Value.ByteSize()+q.ByteSize()); err != nil {
		return Evidence[Rational]{}, Evidence[bool]{}, err
	}
	reduced := q.value.Num().Cmp(first.Value.value) == 0 && q.value.Denom().Cmp(second.Value.value) == 0
	return Evidence[Rational]{State: EvidenceKnown, Value: q}, Evidence[bool]{State: EvidenceKnown, Value: reduced}, nil
}
func performanceMovement(r *contextReader, service *timelineService, n *factNode, out *TimingContextResult) bool {
	backup, forward := r.matches(n, "perf-backup"), r.matches(n, "perf-forward")
	if !backup && !forward {
		return false
	}
	out.Kind = "forward"
	if backup {
		out.Kind = "backup"
	}
	out.CursorAfter = stateUnavailable[ContextPosition, ContextPosition](out.Origin)
	positive := stateUnavailable[Rational, bool](out.Duration)
	within := stateUnknown[bool]("measure-extent-unavailable")
	boundary := stateUnknown[bool]("divisions-boundary-unavailable")
	if out.Duration.State == EvidenceKnown {
		positive = Evidence[bool]{State: EvidenceKnown, Value: out.Duration.Value.value.Sign() > 0}
	}
	if out.Origin.State == EvidenceKnown && out.Duration.State == EvidenceKnown && service != nil {
		from := out.Origin.Value
		p := musicalPosition{Part: from.Part, Measure: from.Measure, MeasureOrdinal: from.MeasureOrdinal, Offset: from.Offset, UnprovedFrame: from.UnprovedFrame}
		delta := out.Duration.Value
		if backup {
			var e error
			delta, e = r.s.rationalSub(r.ctx, timelineZero, delta)
			if e != nil {
				r.err = e
				return true
			}
		}
		to, e := r.s.noteTimingShift(r.ctx, Evidence[musicalPosition]{State: EvidenceKnown, Value: p}, Evidence[Rational]{State: EvidenceKnown, Value: delta})
		if e != nil {
			r.err = e
			return true
		}
		out.CursorAfter = performancePosition(to)
		if to.State == EvidenceKnown {
			if to.Value.Offset.value.Sign() < 0 {
				within = Evidence[bool]{State: EvidenceKnown, Value: false}
			} else {
				a, e := service.State.frameMember(r.ctx, r.s, p)
				if e != nil {
					r.err = e
					return true
				}
				b, e := service.State.frameMember(r.ctx, r.s, to.Value)
				if e != nil {
					r.err = e
					return true
				}
				if a && b {
					within = Evidence[bool]{State: EvidenceKnown, Value: true}
				}
			}
			if within.State == EvidenceKnown && within.Value {
				boundary = Evidence[bool]{State: EvidenceKnown, Value: true}
				lo, hi := p.Offset, to.Value.Offset
				if backup {
					lo, hi = hi, lo
				}
				for _, v := range service.State.Divisions.Publications {
					if !r.step(1) {
						return true
					}
					same, e := stateSameRef(r.ctx, r.s, v.Position.Part, p.Part)
					if e != nil {
						r.err = e
						return true
					}
					if !same || v.Position.MeasureOrdinal != p.MeasureOrdinal {
						continue
					}
					a, e := r.s.rationalCmp(r.ctx, v.Position.Offset, lo)
					if e != nil {
						r.err = e
						return true
					}
					b, e := r.s.rationalCmp(r.ctx, v.Position.Offset, hi)
					if e != nil {
						r.err = e
						return true
					}
					if a > 0 && b < 0 {
						if v.Value.State != EvidenceKnown {
							boundary = stateUnavailable[Decimal, bool](v.Value)
							continue
						}
						if out.Units.State != EvidenceKnown {
							boundary = stateUnavailable[Decimal, bool](out.Units)
							continue
						}
						equal, e := r.s.sameTimelineUnits(r.ctx, out.Units, v.Value)
						if e != nil {
							r.err = e
							return true
						}
						if !equal {
							boundary = Evidence[bool]{State: EvidenceKnown, Value: false}
							break
						}
					}
				}
				for _, g := range service.State.Divisions.Gaps {
					if !r.step(1) {
						return true
					}
					same, e := stateSameRef(r.ctx, r.s, g.Part, p.Part)
					if e != nil {
						r.err = e
						return true
					}
					if same && p.MeasureOrdinal >= g.FirstMeasure && p.MeasureOrdinal <= g.LastMeasure && !(boundary.State == EvidenceKnown && !boundary.Value) {
						boundary = g.Availability
					}
				}
			}
		}
	}
	for _, a := range []TimingAdvisory{{"positive-duration", positive}, {"within-measure", within}, {"no-divisions-boundary-cross", boundary}} {
		if r.err == nil {
			r.err = performanceAdvice(r.ctx, r.s, out, a.Clause, a.Satisfied)
		}
	}
	return true
}

type performanceDivisionAdvice struct {
	count    int
	initial  bool
	complete Evidence[bool]
}
type performanceAdviceIndex struct {
	divisions   map[NodeRef]performanceDivisionAdvice
	instruments map[NodeID]Evidence[bool]
}

// Build invariant part and note advice once. Entries borrow immutable timeline
// evidence during this provider; publication makes the owned report copy.
func buildPerformanceAdviceIndex(ctx context.Context, s *Session, service *timelineService) (*performanceAdviceIndex, error) {
	if err := s.step(ctx, 1); err != nil {
		return nil, err
	}
	if err := s.retain(256); err != nil {
		return nil, err
	}
	x := &performanceAdviceIndex{divisions: make(map[NodeRef]performanceDivisionAdvice), instruments: make(map[NodeID]Evidence[bool])}
	if service == nil {
		return x, nil
	}
	charge := func(p NodeRef) error {
		if err := s.step(ctx, 1+int64(len(p.DocumentID))); err != nil {
			return err
		}
		if _, ok := x.divisions[p]; !ok {
			return s.retain(512 + int64(len(p.DocumentID)))
		}
		return nil
	}
	for _, v := range service.State.Divisions.Publications {
		p := v.Position.Part
		if err := charge(p); err != nil {
			return nil, err
		}
		d, ok := x.divisions[p]
		if !ok {
			d.complete = Evidence[bool]{State: EvidenceKnown}
		}
		d.count++
		d.initial = v.Position.MeasureOrdinal == 0 && !v.Position.UnprovedFrame && v.Position.Offset.value.Sign() == 0
		if v.Value.State != EvidenceKnown {
			d.complete = stateUnavailable[Decimal, bool](v.Value)
		}
		x.divisions[p] = d
	}
	for _, g := range service.State.Divisions.Gaps {
		if err := charge(g.Part); err != nil {
			return nil, err
		}
		d := x.divisions[g.Part]
		d.complete = g.Availability
		x.divisions[g.Part] = d
	}
	for _, set := range service.State.NoteInstruments {
		if err := s.step(ctx, 1); err != nil {
			return nil, err
		}
		if _, ok := x.instruments[set.Note.Node]; ok {
			continue
		}
		if err := s.retain(512); err != nil {
			return nil, err
		}
		identified := set.Complete
		if identified.State == EvidenceKnown {
			identified.Value = len(set.References) > 0
			for _, ref := range set.References {
				if err := s.step(ctx, 1); err != nil {
					return nil, err
				}
				if ref.State != EvidenceKnown {
					identified = stateUnavailable[NodeRef, bool](ref)
					break
				}
			}
		}
		x.instruments[set.Note.Node] = identified
	}
	return x, nil
}

func performanceStateAdvice(ctx context.Context, s *Session, index *performanceAdviceIndex, n *factNode, out *TimingContextResult) error {
	if out.Part.State != EvidenceKnown {
		return nil
	}
	if out.Kind == "note" {
		identified := stateUnavailable[Integer, bool](out.Instruments)
		if out.Instruments.State == EvidenceKnown {
			if out.Instruments.Value.value.Cmp(stateOne().value) <= 0 {
				identified = Evidence[bool]{State: EvidenceAbsent}
			} else {
				if err := s.step(ctx, 1); err != nil {
					return err
				}
				var ok bool
				identified, ok = index.instruments[n.header.ID]
				if !ok {
					identified = stateUnknown[bool]("note-instrument-enumeration-unavailable")
				}
			}
		}
		if err := performanceAdvice(ctx, s, out, "multi-instrument-note-identifier", identified); err != nil {
			return err
		}
	}
	if out.Kind == "note" || out.Kind == "event" {
		if err := s.step(ctx, 1+int64(len(out.Part.Value.DocumentID))); err != nil {
			return err
		}
		d, ok := index.divisions[out.Part.Value]
		complete := Evidence[bool]{State: EvidenceKnown, Value: false}
		if ok {
			complete = d.complete
			if complete.State == EvidenceKnown {
				complete.Value = d.count == 1 && d.initial
			}
		}
		if err := performanceAdvice(ctx, s, out, "one-initial-divisions-per-part", complete); err != nil {
			return err
		}
	}
	if out.ChromaticDisplacement.State == EvidenceKnown {
		value := out.ChromaticDisplacement.Value
		cmp, e := s.rationalCmp(ctx, value, timelineOctaveSemitones)
		if e != nil {
			return e
		}
		neg, e := s.rationalCmp(ctx, value, performanceRatio(-12, 1))
		if e != nil {
			return e
		}
		v := Evidence[bool]{State: EvidenceAbsent}
		if cmp >= 0 || neg <= 0 {
			v = stateUnavailable[Integer, bool](out.TranspositionOctave)
			if out.TranspositionOctave.State == EvidenceAbsent {
				v = Evidence[bool]{State: EvidenceKnown, Value: false}
			} else if out.TranspositionOctave.State == EvidenceKnown {
				v = Evidence[bool]{State: EvidenceKnown, Value: out.TranspositionOctave.Value.value.Sign() != 0}
			}
		}
		if e = performanceAdvice(ctx, s, out, "octave-change-for-octave-interval", v); e != nil {
			return e
		}
	}
	return nil
}

func performanceInstrumentGrouping(r *stateReader, n *factNode, part Evidence[NodeRef]) Evidence[bool] {
	complete := r.complete(n)
	if complete.State != EvidenceKnown {
		return complete
	}
	out := Evidence[bool]{State: EvidenceKnown, Value: true}
	var current NodeRef
	seen := []NodeRef{}
	for _, ch := range n.children {
		if !r.step(1) {
			return stateUnknown[bool]("context-interrupted")
		}
		var role timelineRole
		switch {
		case r.matches(ch, timelineRoleSoundMidiInstrument):
			role = timelineRoleMidiInstrumentID
		case r.matches(ch, timelineRoleSoundMidiDevice):
			role = timelineRoleMidiDeviceID
		case r.matches(ch, timelineRoleSoundInstrumentChange):
			role = timelineRoleInstrumentChangeID
		case r.matches(ch, timelineRoleSoundPlay):
			role = timelineRolePlayID
		default:
			continue
		}
		if part.State != EvidenceKnown {
			return stateUnavailable[NodeRef, bool](part)
		}
		id := r.reference(ch, role, part.Value)
		if id.State != EvidenceKnown {
			if id.State == EvidenceAbsent {
				return stateUnknown[bool]("instrument-grouping-selector-unspecified")
			}
			return stateUnavailable[NodeRef, bool](id)
		}
		sameCurrent, e := stateSameRef(r.ctx, r.s, current, id.Value)
		if e != nil {
			r.err = e
			return stateUnknown[bool]("context-interrupted")
		}
		if sameCurrent {
			continue
		}
		for _, old := range seen {
			if !r.step(1) {
				return stateUnknown[bool]("context-interrupted")
			}
			same, e := stateSameRef(r.ctx, r.s, old, id.Value)
			if e != nil {
				r.err = e
				return stateUnknown[bool]("context-interrupted")
			}
			if same {
				return Evidence[bool]{State: EvidenceKnown, Value: false}
			}
		}
		if e := r.s.retain(128); e != nil {
			r.err = e
			return stateUnknown[bool]("context-interrupted")
		}
		seen = append(seen, id.Value)
		current = id.Value
	}
	return out
}
