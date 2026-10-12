package validation

import "context"

type spanVerticalKey struct {
	part, measure  NodeRef
	ordinal        uint64
	family         SpanFamily
	number, offset string
}
type spanVerticalRun struct {
	endpoints []*spanEndpoint
	complete  Evidence[bool]
}

// spanVerticalMembership groups simultaneous notation marks, not horizontal
// starts and stops. Staff and voice do not split explicitly coordinated chords.
func spanVerticalMembership(ctx context.Context, s *Session, es []*spanEndpoint) error {
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
		if err := s.retain(512 + int64(len(k.offset))); err != nil {
			return err
		}
		gaps[k] = v
		return nil
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
			if err := setGap(g, p.Association); err != nil {
				return err
			}
			continue
		}
		if p.Position.Value.UnprovedFrame {
			p.Association = stateUnknown[bool]("span-vertical-frame-unproved")
			if err := setGap(spanVerticalKey{part: p.Position.Value.Part, family: p.Family}, p.Association); err != nil {
				return err
			}
			continue
		}
		number, ok, err := spanAtomicKey(ctx, s, p.Number)
		if err != nil {
			return err
		}
		if !ok {
			pos := p.Position.Value
			g := spanVerticalKey{part: pos.Part, measure: pos.Measure, ordinal: pos.MeasureOrdinal, family: p.Family, offset: pos.Offset.String()}
			if err := setGap(g, stateUnavailable[Atomic, bool](p.Number)); err != nil {
				return err
			}
			continue
		}
		pos := p.Position.Value
		k := spanVerticalKey{pos.Part, pos.Measure, pos.MeasureOrdinal, p.Family, number, pos.Offset.String()}
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
		for _, g := range []spanVerticalKey{{family: k.family}, {part: k.part, family: k.family}, {part: k.part, measure: k.measure, ordinal: k.ordinal, family: k.family, offset: k.offset}} {
			if v, ok := gaps[g]; ok {
				run.complete = timelineMergeAvailability(run.complete, v)
			}
		}
		if run.complete.State != EvidenceKnown || !run.complete.Value {
			for _, e := range run.endpoints {
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
