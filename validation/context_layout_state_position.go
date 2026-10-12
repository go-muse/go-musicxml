package validation

import (
	"context"
	"math/big"
)

// Reserve all transient integer copies before arithmetic. Values reaching this
// helper already passed the scalar cap; output publication performs its own
// owned-copy accounting. No staff count is expanded into an allocation.
func layoutStateInteger(ctx context.Context, s *Session, values []Integer, compute func() *big.Int) (Integer, error) {
	cost := int64(512)
	for _, v := range values {
		if !v.Valid() {
			return Integer{}, contract("layout integer", "uninitialized operand")
		}
		var e error
		cost, e = rationalCheckedAdd(BudgetBufferedBytes, cost, v.ByteSize()*8)
		if e != nil {
			return Integer{}, e
		}
	}
	if e := s.step(ctx, cost); e != nil {
		return Integer{}, e
	}
	if e := s.account.Charge(BudgetBufferedBytes, cost); e != nil {
		return Integer{}, e
	}
	defer func() { _ = s.account.Release(BudgetBufferedBytes, cost) }()
	value, e := NewInteger(compute())
	if e != nil {
		return Integer{}, e
	}
	if e = s.account.Check(BudgetScalarBytes, value.ByteSize()); e != nil {
		return Integer{}, e
	}
	if e = s.retain(value.ByteSize() + 128); e != nil {
		return Integer{}, e
	}
	return value, nil
}
func layoutStatePrecedingStaff(ctx context.Context, s *Session, staff Integer) (Evidence[Atomic], error) {
	if !staff.Valid() || staff.value.Sign() <= 0 {
		return stateUnknown[Atomic]("alternate-prior-staff-unavailable"), nil
	}
	cmp, e := staff.Cmp(localOne)
	if e != nil {
		return Evidence[Atomic]{}, e
	}
	if cmp <= 0 {
		return stateUnknown[Atomic]("alternate-staff-has-no-preceding-staff"), nil
	}
	previous, e := layoutStateInteger(ctx, s, []Integer{staff}, func() *big.Int { return new(big.Int).Sub(staff.value, localOne.value) })
	if e != nil {
		return Evidence[Atomic]{}, e
	}
	return Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger, Integer: previous}}, nil
}
func layoutStatePosition(ctx context.Context, s *Session, key RuleKey, n *factNode, r *contextReader, p *contextPlan) error {
	isDot := r.matches(n, layoutStateRole("dot"))
	subject := n
	if isDot {
		subject = s.nodes[n.header.Parent]
		if subject == nil {
			return nil
		}
	}
	stepRole, octaveRole := "display-step", "display-octave"
	if isDot {
		pitch, found := r.child(subject, layoutStateRole("pitch"))
		if found.State == EvidenceKnown {
			subject = pitch
			stepRole, octaveRole = "step", "octave"
		} else {
			unpitched, found := r.child(subject, layoutStateRole("unpitched"))
			if found.State == EvidenceKnown {
				subject = unpitched
			} else {
				rest, found := r.child(subject, layoutStateRole("rest"))
				if found.State == EvidenceKnown {
					subject = rest
				}
			}
		}
	}
	step := stateText(r.childScalar(subject, layoutStateRole(stepRole)))
	octave := stateInteger(r.childScalar(subject, layoutStateRole(octaveRole)))
	fact := layoutStateFact(r, p, n)
	result := timelineGapAs[musicalPosition, Integer](fact.Position, "display-position-unavailable")
	service := s.timeline[p.Timeline]
	defaultUnpitched := step.State == EvidenceAbsent && octave.State == EvidenceAbsent && r.matches(subject, layoutStateRole("unpitched"))
	if step.State != EvidenceKnown && step.State != EvidenceAbsent {
		result = stateUnavailable[string, Integer](step)
	} else if octave.State != EvidenceKnown && octave.State != EvidenceAbsent {
		result = octave
	} else if step.State == EvidenceAbsent && !defaultUnpitched {
		result = stateUnknown[Integer]("application-rest-or-missing-written-position")
	} else if service != nil && fact.Position.State == EvidenceKnown {
		staff, e := service.State.resolveStaff(ctx, s, fact.Position.Value, fact.Staff)
		if e != nil {
			return e
		}
		if defaultUnpitched {
			details, e := service.State.effectiveStaffDetails(ctx, s, fact.Position.Value, staff)
			if e != nil {
				return e
			}
			result, e = layoutStateMiddleLine(ctx, s, details.Value)
			if e != nil {
				return e
			}
		} else {
			clef, e := queryState(ctx, s, &service.State, &service.State.Clefs, fact.Position.Value, staff, stateWhole[clefState])
			if e != nil {
				return e
			}
			if clef.Value.State == EvidenceKnown {
				result, e = layoutStateStaffStep(ctx, s, step, octave, clef.Value.Value)
				if e != nil {
					return e
				}
			} else {
				result = timelineGapAs[clefState, Integer](clef.Value, "display-effective-clef-not-encoded")
			}
		}
	}

	raw := r.childScalar(subject, layoutStateRole(stepRole))
	active := Evidence[bool]{State: EvidenceKnown, Value: true}
	property := ContextProperty("layout.display-staff-position")
	value := stateUnavailable[Integer, Atomic](result)
	if result.State == EvidenceKnown {
		value = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger, Integer: result.Value}}
	}
	if isDot {
		property = "layout.dot-placement-active"
		raw = localRawAttribute(r, n, layoutStateRole("empty-placement.@placement"))
		applicable := stateUnavailable[Integer, bool](result)
		if result.State == EvidenceKnown {
			applicable = Evidence[bool]{State: EvidenceKnown, Value: result.Value.value.Bit(0) == 0}
		}
		value = localBoolAtomic(applicable)
	}
	ref := NodeRef{s.documentID, n.header.ID}
	return s.publishContextResult(ctx, ContextResult{Provider: key, Subject: ref, Property: property, Value: value, Origins: []Location{n.header.Location}, Layout: &LayoutContextResult{Part: fact.Part, Scope: layoutStateScope(s, n, fact), Origin: ref, Raw: raw, Applicability: active, Unit: ContextUnitNone, Provenance: ContextExplicit, Staff: fact.Staff, Basis: "diatonic-half-space-offset-from-bottom-staff-line; hidden-lines-included; no-sounding-pitch"}})
}
func layoutStateStaffStep(ctx context.Context, s *Session, step Evidence[string], octave Evidence[Integer], clef clefState) (Evidence[Integer], error) {
	if step.State != EvidenceKnown {
		return timelineGapAs[string, Integer](step, "display-step-not-encoded"), nil
	}
	if octave.State != EvidenceKnown {
		return timelineGapAs[Integer, Integer](octave, "display-octave-not-encoded"), nil
	}
	if clef.Sign.State != EvidenceKnown {
		return stateUnavailable[string, Integer](clef.Sign), nil
	}
	indices := map[string]int64{"C": 0, "D": 1, "E": 2, "F": 3, "G": 4, "A": 5, "B": 6}
	index, ok := indices[step.Value]
	if !ok {
		return stateUnknown[Integer]("display-step-unavailable"), nil
	}
	var refStep, refOct int64
	line := clef.activeLine()
	change := localDefault(clef.OctaveChange, localByteIntegers[0])
	switch clef.Sign.Value {
	case "G":
		refStep, refOct = 4, 4
	case "F":
		refStep, refOct = 3, 3
	case "C":
		refStep, refOct = 0, 4
	case "percussion":
		refStep, refOct = 4, 4
		line = Evidence[Integer]{State: EvidenceKnown, Value: localByteIntegers[2]}
		change = Evidence[Integer]{State: EvidenceKnown, Value: localByteIntegers[0]}
	default:
		return stateUnknown[Integer]("display-clef-reference-unavailable"), nil
	}
	if line.State != EvidenceKnown {
		if line.State == EvidenceAbsent {
			return stateUnknown[Integer]("display-clef-line-unavailable"), nil
		}
		return line, nil
	}
	if change.State != EvidenceKnown {
		return change, nil
	}
	v, e := layoutStateInteger(ctx, s, []Integer{line.Value, octave.Value, change.Value}, func() *big.Int {
		out := new(big.Int).Sub(line.Value.value, big.NewInt(1))
		out.Mul(out, big.NewInt(2))
		oct := new(big.Int).Sub(octave.Value.value, change.Value.value)
		oct.Sub(oct, big.NewInt(refOct))
		oct.Mul(oct, big.NewInt(7))
		out.Add(out, oct)
		return out.Add(out, big.NewInt(index-refStep))
	})
	if e != nil {
		return Evidence[Integer]{}, e
	}
	return Evidence[Integer]{State: EvidenceKnown, Value: v}, nil
}

func layoutStateMiddleLine(ctx context.Context, s *Session, details Evidence[staffDetailsState]) (Evidence[Integer], error) {
	if details.State != EvidenceKnown {
		return timelineGapAs[staffDetailsState, Integer](details, "staff-line-count-not-encoded"), nil
	}
	lines := details.Value.Lines
	if lines.State != EvidenceKnown {
		return timelineGapAs[Integer, Integer](lines, "staff-line-count-not-encoded"), nil
	}
	if !lines.Value.Valid() || lines.Value.value.Sign() <= 0 {
		return stateUnknown[Integer]("staff-has-no-unique-middle-line"), nil
	}
	if lines.Value.value.Bit(0) == 0 {
		return stateUnknown[Integer]("even-staff-has-no-unique-middle-line"), nil
	}
	value, e := layoutStateInteger(ctx, s, []Integer{lines.Value}, func() *big.Int { return new(big.Int).Sub(lines.Value.value, localOne.value) })
	if e != nil {
		return Evidence[Integer]{}, e
	}
	return Evidence[Integer]{State: EvidenceKnown, Value: value}, nil
}
