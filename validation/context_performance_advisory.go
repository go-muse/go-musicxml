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
						boundary = Evidence[bool]{State: EvidenceKnown, Value: false}
						break
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
