package validation

import "context"

const (
	timelineDivisionsValueCertificateBytes  int64 = 768
	timelineDivisionsValueGapScratchBytes   int64 = 512
	timelineDivisionsValueScratchFixedBytes int64 = 256
)

// Independently validated raw units remain meaningful when their position is
// unavailable. They can preserve an existing equal-value baseline, but cannot
// supply a baseline, position a publisher, or repair a hidden/invalid source.
type divisionsValueCertificate struct {
	Value      Decimal
	Provenance stateProvenance
}

func (h *timelineState) certifyDivisionsValueGap(ctx context.Context, s *Session, index int, value Decimal, node NodeRef) error {
	if index < 0 || index >= len(h.Divisions.Gaps) || node.Node == 0 || !value.Valid() || value.value.Sign() <= 0 {
		return contract("divisions value", "matching divisions gap and source required")
	}
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	if _, ok := h.divisionsValueGaps[index]; ok {
		return contract("divisions value", "duplicate certificate")
	}
	certificate := divisionsValueCertificate{Value: value, Provenance: stateProvenance{Node: node, Kind: stateExplicit}}
	if err := s.account.Charge(BudgetContextFacts, 1); err != nil {
		return err
	}
	// Map header and old/new bucket coexistence, separately from the owned source.
	if err := s.retain(timelineDivisionsValueCertificateBytes); err != nil {
		return err
	}
	if err := stateRetainCopy(ctx, s, certificate); err != nil {
		return err
	}
	if h.divisionsValueGaps == nil {
		h.divisionsValueGaps = make(map[int]divisionsValueCertificate)
	}
	h.divisionsValueGaps[index] = stateClone(certificate)
	return nil
}

func (h *timelineState) queryDivisions(ctx context.Context, s *Session, pos musicalPosition, owned bool) (stateResult[Decimal], error) {
	staff := Evidence[Integer]{State: EvidenceKnown, Value: stateOne()}
	project := stateWhole[Decimal]
	original, err := queryStateAtMode(ctx, s, h, &h.Divisions, pos, staff, project, true, owned)
	if err != nil || len(h.divisionsValueGaps) == 0 || original.Value.State == EvidenceKnown {
		return original, err
	}
	// Remove only certified value-preserving gaps for an independent baseline
	// proof. Missing baseline and every other causal gap remain in the query.
	count := int64(len(h.Divisions.Gaps))
	bytes, err := rationalCheckedMul(BudgetBufferedBytes, count, timelineDivisionsValueGapScratchBytes)
	if err != nil {
		return stateResult[Decimal]{}, err
	}
	bytes, err = rationalCheckedAdd(BudgetBufferedBytes, bytes, timelineDivisionsValueScratchFixedBytes)
	if err != nil {
		return stateResult[Decimal]{}, err
	}
	if _, err = allocationSize(bytes); err != nil {
		return stateResult[Decimal]{}, err
	}
	if err = s.account.Charge(BudgetBufferedBytes, bytes); err != nil {
		return stateResult[Decimal]{}, err
	}
	defer func() { _ = s.account.Release(BudgetBufferedBytes, bytes) }()
	index, err := stateIndex(ctx, s, &h.Divisions)
	if err != nil {
		return stateResult[Decimal]{}, err
	}
	view := h.Divisions
	view.index = index
	view.Gaps = make([]stateGap, 0, len(h.Divisions.Gaps))
	for i, gap := range h.Divisions.Gaps {
		if err = s.step(ctx, 1); err != nil {
			return stateResult[Decimal]{}, err
		}
		if _, certified := h.divisionsValueGaps[i]; !certified {
			view.Gaps = append(view.Gaps, gap)
		}
	}
	out, err := queryStateAtMode(ctx, s, h, &view, pos, staff, project, true, owned)
	if err != nil {
		return stateResult[Decimal]{}, err
	}
	if out.Value.State != EvidenceKnown {
		return original, nil
	}
	for i, gap := range h.Divisions.Gaps {
		if err = s.step(ctx, 1); err != nil {
			return stateResult[Decimal]{}, err
		}
		certificate, certified := h.divisionsValueGaps[i]
		if !certified {
			continue
		}
		relevant, err := h.divisionsValueGapRelevant(ctx, s, gap, pos, staff)
		if err != nil {
			return stateResult[Decimal]{}, err
		}
		if !relevant {
			continue
		}
		same, err := s.sameTimelineUnits(ctx, out.Value, Evidence[Decimal]{State: EvidenceKnown, Value: certificate.Value})
		if err != nil {
			return stateResult[Decimal]{}, err
		}
		if !same {
			return original, nil
		}
		if !owned {
			continue
		}
		witness := certificate.Provenance
		duplicate := false
		for _, old := range out.Provenance {
			if err = s.step(ctx, 1+int64(len(old.Node.DocumentID))+int64(len(witness.Node.DocumentID))); err != nil {
				return stateResult[Decimal]{}, err
			}
			if old == witness {
				duplicate = true
				break
			}
		}
		if !duplicate {
			if err = stateRetainCopy(ctx, s, witness); err != nil {
				return stateResult[Decimal]{}, err
			}
			out.Provenance = append(out.Provenance, stateClone(witness))
		}
		before := false
		if gap.Placement.Displaced && gap.Placement.Upper != nil {
			relation, err := h.positionOrder(ctx, s, *gap.Placement.Upper, pos)
			if err != nil {
				return stateResult[Decimal]{}, err
			}
			before = relation == stateBefore
		} else if !gap.Placement.Displaced {
			member, err := h.frameMember(ctx, s, pos)
			if err != nil {
				return stateResult[Decimal]{}, err
			}
			before = member && gap.LastMeasure < pos.MeasureOrdinal
		}
		if !before {
			out.Inherited = false
			out.InheritanceUnproved = true
		}
	}
	return out, nil
}

func (h *timelineState) divisionsValueGapRelevant(ctx context.Context, s *Session, gap stateGap, pos musicalPosition, staff Evidence[Integer]) (bool, error) {
	if err := s.step(ctx, 1+int64(len(gap.Part.DocumentID))+int64(len(pos.Part.DocumentID))); err != nil {
		return false, err
	}
	if gap.Part != pos.Part {
		return false, nil
	}
	applies, err := stateApplies(ctx, s, gap.Scope, staff)
	if err != nil || !applies {
		return false, err
	}
	relevant, err := h.gapCouldApply(ctx, s, gap, pos, true)
	if err != nil || !relevant {
		return false, err
	}
	// Only an independently applicable unconditional later publication can make
	// this valid divisions source irrelevant. Equal or uncertain positions do not win.
	for _, publication := range h.Divisions.Publications {
		if err = s.step(ctx, 1+int64(len(publication.Position.Part.DocumentID))+int64(len(pos.Part.DocumentID))); err != nil {
			return false, err
		}
		if publication.Position.Part != pos.Part || !stateUnconditional(publication.TimeOnly) {
			continue
		}
		applies, err = stateApplies(ctx, s, publication.Scope, staff)
		if err != nil {
			return false, err
		}
		if !applies {
			continue
		}
		order, err := h.positionOrder(ctx, s, publication.Position, pos)
		if err != nil {
			return false, err
		}
		if !order.definitelyApplies(true) {
			continue
		}
		repaired, err := h.gapStrictlyBefore(ctx, s, gap, publication.Position)
		if err != nil {
			return false, err
		}
		if repaired {
			return false, nil
		}
	}
	return true, nil
}
