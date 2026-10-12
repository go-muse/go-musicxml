package validation

import "context"

// Excursions are recorded at departure, before any descendant can enlarge the
// frame witness. The closed fixed allowance includes concurrent append backing.
const timelineExcursionReserveBytes int64 = 1024
const timelineUnitPublisherBytes int64 = 512
const timelineUnitGateBytes int64 = 512
const timelineUnitStageBytes int64 = 2048

type timelineUnitExcursion struct {
	Node              NodeID
	Part              NodeRef
	Raw               Evidence[Decimal]
	Backwards, Locked bool
	Availability      Evidence[Decimal]
}
type timelineExcursionGate struct {
	Availability      Evidence[Decimal]
	PositiveDirection bool
}
type timelineUnitPublisher struct {
	Node    NodeID
	Part    Evidence[NodeRef]
	Ordinal uint64
	Value   Evidence[Decimal]
}

func (v *timelineService) recordUnitExcursion(ctx context.Context, s *Session, id NodeID, block timelineMeasureBlock, raw Evidence[Decimal], back bool, duration Evidence[Rational]) error {
	if v.unitProofFinal || block.part.State != EvidenceKnown || raw.State == EvidenceKnown && raw.Value.value.Sign() == 0 {
		return nil
	}
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	if err := s.retain(timelineExcursionReserveBytes); err != nil {
		return err
	}
	var err error
	raw, err = stateOwnEvidence(ctx, s, raw)
	if err != nil {
		return err
	}
	gap := Evidence[Decimal]{State: EvidenceUnsupported, Reason: "divisions-cross-frame-placement-unresolved"}
	locked := duration.State != EvidenceKnown
	if locked {
		gap = stateUnavailable[Rational, Decimal](duration)
	}
	gap, err = stateOwnEvidence(ctx, s, gap)
	if err != nil {
		return err
	}
	v.unitExcursions = append(v.unitExcursions, timelineUnitExcursion{Node: id, Part: block.part.Value, Raw: raw, Backwards: back, Locked: locked, Availability: gap})
	return nil
}

// A directional bound survives unavailable normalization only after a complete
// positive-unit-domain proof. No sign is inferred from an unavailable scalar.
func (v *timelineService) movementPlacement(origin Evidence[musicalPosition], prior stateGapPlacement, part Evidence[NodeRef], raw Evidence[Decimal], back bool, positiveDirection ...bool) stateGapPlacement {
	out := stateGapPlacement{Displaced: true}
	positive := len(positiveDirection) != 0 && positiveDirection[0]
	if part.State == EvidenceKnown && v.unitPositive[part.Value] {
		positive = true
	}
	if !positive || raw.State != EvidenceKnown || raw.Value.value.Sign() < 0 {
		return out
	}
	if raw.Value.value.Sign() == 0 {
		return prior
	}
	if back {
		out.Upper = prior.Upper
		if origin.State == EvidenceKnown {
			p := origin.Value
			out.Upper = &p
		}
	} else {
		out.Lower = prior.Lower
		if origin.State == EvidenceKnown {
			p := origin.Value
			out.Lower = &p
		}
	}
	return out
}

// Raw declarations are inventoried independently of reconstructed coordinates.
// A stopped path can hide a publisher's position, never its existence or cause.
func timelineUnitPublishers(ctx context.Context, s *Session, b *timelineBindings, blocks []timelineMeasureBlock) ([]timelineUnitPublisher, func(), error) {
	r := &stateReader{ctx: ctx, s: s, b: b}
	count := int64(0)
	for _, block := range blocks {
		if !r.step(1) {
			return nil, timelineNoScratch, r.err
		}
		if block.complete.State != EvidenceKnown || !block.complete.Value {
			count++
		}
		for _, event := range block.content.children {
			if !r.step(1) {
				return nil, timelineNoScratch, r.err
			}
			if r.matches(event, timelineRoleAttributes) || event.header.Declaration.State != EvidenceKnown && r.potential(event, timelineRoleAttributes) {
				count++
			}
		}
	}
	bytes, err := rationalCheckedMul(BudgetBufferedBytes, count, timelineUnitPublisherBytes)
	if err != nil {
		return nil, timelineNoScratch, err
	}
	_, err = AllocationSize(bytes)
	if err != nil {
		return nil, timelineNoScratch, err
	}
	if err = s.account.Charge(BudgetBufferedBytes, bytes); err != nil {
		return nil, timelineNoScratch, err
	}
	release := func() {
		if bytes != 0 {
			_ = s.account.Release(BudgetBufferedBytes, bytes)
			bytes = 0
		}
	}
	out := make([]timelineUnitPublisher, 0, int(count))
	for _, block := range blocks {
		if !r.step(1) {
			release()
			return nil, timelineNoScratch, r.err
		}
		if block.complete.State != EvidenceKnown || !block.complete.Value {
			out = append(out, timelineUnitPublisher{Part: block.part, Ordinal: block.ordinal, Value: timelineGapAs[bool, Decimal](block.complete, "timeline-divisions-enumeration-unavailable")})
		}
		for _, event := range block.content.children {
			if !r.step(1) {
				release()
				return nil, timelineNoScratch, r.err
			}
			var value Evidence[Decimal]
			switch {
			case r.matches(event, timelineRoleAttributes):
				value = timelineAttributeUnits(r, event)
			case event.header.Declaration.State != EvidenceKnown && r.potential(event, timelineRoleAttributes):
				value = stateUnavailable[DeclarationID, Decimal](event.header.Declaration)
			default:
				continue
			}
			if value.State != EvidenceAbsent {
				out = append(out, timelineUnitPublisher{Node: event.header.ID, Part: block.part, Ordinal: block.ordinal, Value: value})
			}
		}
	}
	if r.err != nil {
		release()
		return nil, timelineNoScratch, r.err
	}
	return out, release, nil
}

func unitPublisherApplies(p timelineUnitPublisher, part NodeRef) bool {
	return p.Part.State != EvidenceKnown || p.Part.Value == part
}
func (p timelineUnitPublisher) gap(v *timelineService, part NodeRef) stateGap {
	g := stateGap{Part: part, FirstMeasure: p.Ordinal, LastMeasure: p.Ordinal, Scope: stateScope{AllStaves: true}, Availability: stateUnavailable[Decimal, bool](p.Value)}
	if fact, ok := v.Facts[p.Node]; ok {
		g.Placement = fact.Placement
		if fact.Origin.State == EvidenceKnown {
			pos := fact.Origin.Value
			g.Position = &pos
		}
	} else {
		g.Placement.Displaced = true
	}
	return g
}

func (v *timelineService) finalizeExcursionUnits(ctx context.Context, s *Session, b *timelineBindings, blocks []timelineMeasureBlock) error {
	if v.unitProofFinal || len(v.unitExcursions) == 0 {
		return nil
	}
	publishers, release, err := timelineUnitPublishers(ctx, s, b, blocks)
	if err != nil {
		return err
	}
	defer release()
	// Admit all map entry/growth storage before allocation. Maps contain only
	// immutable borrowed values during this exception-only proof transaction.
	if err = stateRetainCount(s, len(v.unitExcursions), timelineUnitGateBytes*2); err != nil {
		return err
	}
	if _, err = AllocationSize(int64(len(v.unitExcursions))); err != nil {
		return err
	}
	positive := make(map[NodeRef]bool, len(v.unitExcursions))
	gates := make(map[NodeID]timelineExcursionGate, len(v.unitExcursions))
	for _, exc := range v.unitExcursions {
		if err = s.step(ctx, 1+int64(len(exc.Part.DocumentID))); err != nil {
			return err
		}
		units := v.Facts[exc.Node].Units
		uniform, domain := units.State == EvidenceKnown, true
		seen := false
		gap := exc.Availability
		for _, pub := range publishers {
			if err = s.step(ctx, 1+int64(len(exc.Part.DocumentID))+int64(len(pub.Part.Value.DocumentID))); err != nil {
				return err
			}
			if !unitPublisherApplies(pub, exc.Part) {
				continue
			}
			seen = true
			valid := pub.Value.State == EvidenceKnown && pub.Value.Value.value != nil && pub.Value.Value.value.Sign() > 0
			domain = domain && valid
			same, e := s.sameTimelineUnits(ctx, units, pub.Value)
			if e != nil {
				return e
			}
			uniform = uniform && valid && same

		}
		positive[exc.Part] = domain && seen
		if !uniform || exc.Locked {
			gates[exc.Node] = timelineExcursionGate{Availability: gap, PositiveDirection: domain && seen}
		}
	}
	if len(gates) == 0 {
		v.unitProofFinal = true
		return nil
	}
	// Least-proof construction: every unresolved excursion is stopped before
	// any bound or publisher coordinate is used to certify it. Restoring only
	// independently proved gates grows knowledge; a cycle cannot start itself.
	stage, err := v.rebuildUnitStage(ctx, s, b, blocks, gates, positive)
	if err != nil {
		return err
	}
	for {
		progress := 0
		for _, exc := range v.unitExcursions {
			if err = s.step(ctx, 1); err != nil {
				return err
			}
			if _, ok := gates[exc.Node]; !ok {
				continue
			}
			proved, cause, e := stage.proveExcursion(ctx, s, exc, publishers)
			if e != nil {
				return e
			}
			if proved && !exc.Locked {
				delete(gates, exc.Node)
				progress++
			} else {
				gap := exc.Availability
				if cause.State != EvidenceAbsent && cause.State != EvidenceKnown && cause.State != "" {
					if exc.Locked {
						gap = timelineUnitWorst(gap, cause)
					} else {
						gap = cause
					}
				}
				gate := gates[exc.Node]
				gate.Availability = gap
				if !gate.PositiveDirection {
					positive, e := stage.proveExcursionDirection(ctx, s, exc, publishers)
					if e != nil {
						return e
					}
					if positive {
						gate.PositiveDirection = true
						progress++
					}
				}
				gates[exc.Node] = gate
			}
		}
		if progress == 0 {
			break
		}
		stage, err = v.rebuildUnitStage(ctx, s, b, blocks, gates, positive)
		if err != nil {
			return err
		}
	}
	// Rebuild once with final owned causes; the prior stage stays immutable.
	stage, err = v.rebuildUnitStage(ctx, s, b, blocks, gates, positive)
	if err != nil {
		return err
	}
	stage.unitProofFinal = true
	*v = *stage
	return nil
}

func (v *timelineService) proveExcursion(ctx context.Context, s *Session, exc timelineUnitExcursion, publishers []timelineUnitPublisher) (bool, Evidence[Decimal], error) {
	fact, ok := v.Facts[exc.Node]
	cause := Evidence[Decimal]{State: EvidenceAbsent}
	if !ok || fact.Origin.State != EvidenceKnown || fact.Units.State != EvidenceKnown && !exc.Locked {
		return false, cause, nil
	}
	departure := fact.Origin.Value
	selected, err := v.State.effectiveDivisionsEvidence(ctx, s, departure)
	if err != nil {
		return false, cause, err
	}
	if selected.State != EvidenceKnown {
		return false, selected, nil
	}
	sameUnits, err := s.sameTimelineUnits(ctx, fact.Units, selected)
	if err != nil {
		return false, cause, err
	}
	if !sameUnits && !exc.Locked {
		return false, cause, nil
	}
	units := selected
	duration, err := s.normalizeTimelineDuration(ctx, exc.Raw, units)
	if err != nil {
		return false, cause, err
	}
	if duration.State != EvidenceKnown {
		return false, stateUnavailable[Rational, Decimal](duration), nil
	}
	cursor := timelineCursor{Position: Evidence[Rational]{State: EvidenceKnown, Value: departure.Offset}}
	kind := timelineMovementForward
	if exc.Backwards {
		kind = timelineMovementBackup
	}
	if _, err = cursor.advance(ctx, s, timelineMovement{Kind: kind, Duration: duration}); err != nil {
		return false, cause, err
	}
	if cursor.Position.State != EvidenceKnown {
		return false, cause, nil
	}
	landing := departure
	landing.Offset = cursor.Position.Value
	landing.UnprovedFrame = true
	proved := true
	for _, pub := range publishers {
		if err = s.step(ctx, 1+int64(len(exc.Part.DocumentID))+int64(len(pub.Part.Value.DocumentID))); err != nil {
			return false, cause, err
		}
		if !unitPublisherApplies(pub, exc.Part) {
			continue
		}
		same, e := s.sameTimelineUnits(ctx, units, pub.Value)
		if e != nil {
			return false, cause, e
		}
		if same {
			continue
		}
		g := pub.gap(v, exc.Part)
		// A future region cannot affect a backwards movement. For forward
		// travel, a publisher definitely before departure is already reflected
		// by its known effective departure units.
		excluded := false
		if exc.Backwards {
			applies, e := v.State.gapCouldApply(ctx, s, g, departure, true)
			if e != nil {
				return false, cause, e
			}
			excluded = !applies
		} else {
			before, e := v.State.gapStrictlyBefore(ctx, s, g, departure)
			if e != nil {
				return false, cause, e
			}
			applies, e := v.State.gapCouldApply(ctx, s, g, landing, true)
			if e != nil {
				return false, cause, e
			}
			excluded = before || !applies
		}
		if !excluded && exc.Backwards {
			// A differing old region is harmless only with an independently
			// located same-unit repair after it and at/before the landing.
			for _, repair := range publishers {
				if err = s.step(ctx, 1+int64(len(exc.Part.DocumentID))+int64(len(repair.Part.Value.DocumentID))); err != nil {
					return false, cause, err
				}
				if repair.Part.State != EvidenceKnown || repair.Part.Value != exc.Part {
					continue
				}
				same, e := s.sameTimelineUnits(ctx, units, repair.Value)
				if e != nil {
					return false, cause, e
				}
				if !same {
					continue
				}
				rg := repair.gap(v, exc.Part)
				if rg.Position == nil {
					continue
				}
				after, e := v.State.gapStrictlyBefore(ctx, s, g, *rg.Position)
				if e != nil {
					return false, cause, e
				}
				if !after {
					continue
				}
				order, e := v.State.positionOrder(ctx, s, *rg.Position, landing)
				if e != nil {
					return false, cause, e
				}
				if order.definitelyApplies(true) {
					excluded = true
					break
				}
			}
		}
		if !excluded {
			proved = false
			cause = timelineUnitWorst(cause, pub.Value)
		}
	}
	return proved, cause, nil
}

// Reconstruct only from canonical source blocks. No optimistic cursor/bound is
// copied. Definitions and closure evidence are immutable inputs; public result
// paths retain their usual independent ownership. Every other map is fresh.
func (v *timelineService) rebuildUnitStage(ctx context.Context, s *Session, b *timelineBindings, blocks []timelineMeasureBlock, gates map[NodeID]timelineExcursionGate, positive map[NodeRef]bool) (*timelineService, error) {
	if err := s.step(ctx, 1); err != nil {
		return nil, err
	}
	if err := s.retain(timelineUnitStageBytes); err != nil {
		return nil, err
	}
	out := &timelineService{Facts: make(map[NodeID]timelineEventFact), unitGates: gates, unitPositive: positive, unitProofFinal: true}
	out.State.Closures = v.State.Closures[:len(v.State.Closures):len(v.State.Closures)]
	for _, closure := range v.State.Closures {
		if err := s.step(ctx, 1); err != nil {
			return nil, err
		}
		if node := s.nodes[closure.Part.Node]; node != nil {
			if err := out.State.readInstrumentDefinitions(ctx, s, b, node, closure.Part); err != nil {
				return nil, err
			}
		} else {
			for _, def := range v.State.Definitions {
				if err := s.step(ctx, 1); err != nil {
					return nil, err
				}
				if def.Part == closure.Part {
					if err := out.State.addInstrumentDefinition(ctx, s, def); err != nil {
						return nil, err
					}
				}
			}
			for _, record := range v.State.MIDI {
				if err := s.step(ctx, 1); err != nil {
					return nil, err
				}
				if record.Part == closure.Part && record.Position.State == EvidenceAbsent {
					if err := out.State.addMIDIRecord(ctx, s, record, 0); err != nil {
						return nil, err
					}
				}
			}
			for _, record := range v.State.MIDIDevices {
				if err := s.step(ctx, 1); err != nil {
					return nil, err
				}
				if record.Part == closure.Part && record.Position.State == EvidenceAbsent {
					if err := out.State.addMIDIDeviceRecord(ctx, s, record, 0); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	if err := stateRetainCount(s, len(blocks), 128); err != nil {
		return nil, err
	}
	counts := make(map[timelineDomain]uint64, len(blocks))
	for _, block := range blocks {
		if err := s.step(ctx, 1); err != nil {
			return nil, err
		}
		if block.part.State == EvidenceKnown {
			counts[timelineDomain{block.part.Value.Node, block.ordinal}]++
		} else {
			for _, c := range out.State.Closures {
				if err := out.gapAllPublishers(ctx, s, c.Part, block.ordinal, timelineGapAs[NodeRef, musicalPosition](block.part, "timeline-part-owner-unavailable"), stateGapPlacement{Displaced: true}); err != nil {
					return nil, err
				}
			}
		}
	}
	for _, block := range blocks {
		if err := s.step(ctx, 1); err != nil {
			return nil, err
		}
		if block.part.State == EvidenceKnown && counts[timelineDomain{block.part.Value.Node, block.ordinal}] > 1 {
			block.complete = stateUnknown[bool]("timeline-duplicate-part-measure-domain")
		}
		if err := out.readMeasure(ctx, s, b, block); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Direction is a weaker certificate than normalization. Its premises are read
// only from the previous stopped stage; a newly qualified bound cannot certify
// itself during the same pass.
func (v *timelineService) proveExcursionDirection(ctx context.Context, s *Session, exc timelineUnitExcursion, publishers []timelineUnitPublisher) (bool, error) {
	fact, ok := v.Facts[exc.Node]
	if !ok || fact.Origin.State != EvidenceKnown || exc.Raw.State != EvidenceKnown || exc.Raw.Value.value.Sign() < 0 {
		return false, nil
	}
	departure := fact.Origin.Value
	units, err := v.State.effectiveDivisionsEvidence(ctx, s, departure)
	if err != nil {
		return false, err
	}
	if units.State != EvidenceKnown || units.Value.value.Sign() <= 0 {
		return false, nil
	}
	for _, pub := range publishers {
		if err = s.step(ctx, 1+int64(len(exc.Part.DocumentID))+int64(len(pub.Part.Value.DocumentID))); err != nil {
			return false, err
		}
		if !unitPublisherApplies(pub, exc.Part) || pub.Value.State == EvidenceKnown && pub.Value.Value.value != nil && pub.Value.Value.value.Sign() > 0 {
			continue
		}
		gap := pub.gap(v, exc.Part)
		if exc.Backwards {
			applies, e := v.State.gapCouldApply(ctx, s, gap, departure, true)
			if e != nil {
				return false, e
			}
			if applies {
				return false, nil
			}
		} else {
			before, e := v.State.gapStrictlyBefore(ctx, s, gap, departure)
			if e != nil {
				return false, e
			}
			if !before {
				return false, nil
			}
		}
	}
	return true, nil
}
