package validation

import "context"

type spanVerticalKey struct {
	part, measure  NodeRef
	ordinal        uint64
	family         SpanFamily
	number, offset string
	grace, cue     bool
	categoryKnown  bool
}
type spanVerticalRun struct {
	endpoints []*spanEndpoint
	complete  Evidence[bool]
}

// spanVerticalMembership groups simultaneous notation marks, not horizontal
// starts and stops. Staff and voice do not split explicitly coordinated chords.
func spanVerticalMembership(ctx context.Context, s *Session, es []*spanEndpoint) error {
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	if err := s.retain(512); err != nil {
		return err
	}
	runs := make(map[spanVerticalKey]*spanVerticalRun)
	gaps := make(map[spanVerticalKey]Evidence[bool])
	setGap := func(k spanVerticalKey, v Evidence[bool]) error {
		if old, ok := gaps[k]; ok {
			gaps[k] = timelineMergeAvailability(old, v)
			return nil
		}
		if err := s.retain(512 + int64(len(k.offset)+len(k.number))); err != nil {
			return err
		}
		gaps[k] = v
		return nil
	}
	// Missing position broadens only the location axis. For a category gap,
	// keep an established number even when no exact cursor can be used.
	categoryGap := func(e *spanEndpoint, k spanVerticalKey, v Evidence[bool]) (spanVerticalKey, Evidence[bool], error) {
		if e.categoryKnown {
			return k, v, nil
		}
		v = timelineMergeAvailability(spanCategoryAvailability(e), v)
		number, ok, err := spanAtomicKey(ctx, s, e.payload.Number)
		if err != nil {
			return k, v, err
		}
		if ok {
			k.number = number
		} else if e.payload.Number.State != EvidenceKnown && e.payload.Number.State != EvidenceAbsent {
			v = timelineMergeAvailability(v, stateUnavailable[Atomic, bool](e.payload.Number))
		}
		return k, v, nil
	}
	for _, e := range es {
		if err := s.step(ctx, 1); err != nil {
			return err
		}
		p := &e.payload
		if p.Family != SpanArpeggiate && p.Family != SpanNonArpeggiate {
			continue
		}
		if p.Position.State != EvidenceKnown {
			p.Association = stateUnavailable[SpanPosition, bool](p.Position)
			g := spanVerticalKey{family: p.Family}
			if p.Domain.State == EvidenceKnown {
				g.part = p.Domain.Value
			}
			g, v, err := categoryGap(e, g, p.Association)
			if err != nil {
				return err
			}
			p.Association = v
			if err := setGap(g, v); err != nil {
				return err
			}
			continue
		}
		if p.Position.Value.UnprovedFrame {
			p.Association = stateUnknown[bool]("span-vertical-frame-unproved")
			g, v, err := categoryGap(e, spanVerticalKey{part: p.Position.Value.Part, family: p.Family}, p.Association)
			if err != nil {
				return err
			}
			p.Association = v
			if err := setGap(g, v); err != nil {
				return err
			}
			continue
		}
		number, ok, err := spanAtomicKey(ctx, s, p.Number)
		if err != nil {
			return err
		}
		if !ok {
			v := stateUnknown[bool]("span-competing-identity-unproved")
			if p.Number.State != EvidenceKnown {
				v = stateUnavailable[Atomic, bool](p.Number)
			}
			if !e.categoryKnown {
				v = timelineMergeAvailability(spanCategoryAvailability(e), v)
			}
			p.Association = v
			pos := p.Position.Value
			g := spanVerticalKey{part: pos.Part, measure: pos.Measure, ordinal: pos.MeasureOrdinal, family: p.Family, offset: pos.Offset.String(), grace: e.grace, cue: e.cue, categoryKnown: e.categoryKnown}
			if !e.categoryKnown {
				g.grace, g.cue = false, false
			}
			if err := setGap(g, v); err != nil {
				return err
			}
			continue
		}
		pos := p.Position.Value
		k := spanVerticalKey{part: pos.Part, measure: pos.Measure, ordinal: pos.MeasureOrdinal, family: p.Family, number: number, offset: pos.Offset.String(), grace: e.grace, cue: e.cue, categoryKnown: e.categoryKnown}
		if !e.categoryKnown {
			k.grace, k.cue = false, false
			p.Association = timelineMergeAvailability(spanCategoryAvailability(e), e.complete)
			if err := setGap(k, p.Association); err != nil {
				return err
			}
			continue
		}
		run := runs[k]
		if run == nil {
			if err := s.retain(512 + int64(len(number)+len(k.offset))); err != nil {
				return err
			}
			run = &spanVerticalRun{complete: Evidence[bool]{State: EvidenceKnown, Value: true}}
			runs[k] = run
		}
		if err := s.retain(32); err != nil {
			return err
		}
		run.endpoints = append(run.endpoints, e)
		run.complete = timelineMergeAvailability(run.complete, e.complete)
	}
	for k, run := range runs {
		if err := s.step(ctx, 1); err != nil {
			return err
		}
		for _, g := range []spanVerticalKey{{family: k.family}, {family: k.family, number: k.number}, {part: k.part, family: k.family}, {part: k.part, family: k.family, number: k.number}, {part: k.part, measure: k.measure, ordinal: k.ordinal, family: k.family, offset: k.offset, grace: k.grace, cue: k.cue, categoryKnown: true}, {part: k.part, measure: k.measure, ordinal: k.ordinal, family: k.family, number: k.number, offset: k.offset}, {part: k.part, measure: k.measure, ordinal: k.ordinal, family: k.family, offset: k.offset}} {
			if v, ok := gaps[g]; ok {
				run.complete = timelineMergeAvailability(run.complete, v)
			}
		}
		if run.complete.State != EvidenceKnown || !run.complete.Value {
			for _, e := range run.endpoints {
				if err := s.step(ctx, 1); err != nil {
					return err
				}
				e.payload.Association = stateUnavailable[bool, bool](run.complete)
			}
			continue
		}
		if k.family == SpanNonArpeggiate {
			if len(run.endpoints) != 2 {
				continue
			}
			a, b := run.endpoints[0].payload.Role, run.endpoints[1].payload.Role
			if a.State != EvidenceKnown || b.State != EvidenceKnown || !((a.Value == "top" && b.Value == "bottom") || (a.Value == "bottom" && b.Value == "top")) {
				continue
			}
		}
		if err := s.retain(int64(len(run.endpoints))*32 + 32); err != nil {
			return err
		}
		members := make([]NodeRef, 0, len(run.endpoints))
		for _, e := range run.endpoints {
			if err := s.step(ctx, 1); err != nil {
				return err
			}
			members = append(members, e.ref)
		}
		for _, e := range run.endpoints {
			if err := spanSetAssociation(ctx, s, e, members); err != nil {
				return err
			}
		}
	}
	return nil
}
