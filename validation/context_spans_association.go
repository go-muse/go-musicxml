package validation

import "context"

type spanAssociationKey struct {
	voiceSpecified                     bool
	domain                             NodeRef
	family                             SpanFamily
	identity                           SpanIdentityKind
	number, pitch, guard, voice, lyric string
	grace, cue                         bool
}
type spanAssociationRun struct {
	indices  []int
	complete Evidence[bool]
}

func spanBoundaryRole(e *spanEndpoint) int {
	if e.payload.Role.State != EvidenceKnown {
		return 0
	}
	if e.payload.Family == SpanPedal {
		switch e.payload.Role.Value {
		case "start", "sostenuto", "resume":
			return 1
		case "stop", "discontinue":
			return -1
		case "change", "continue":
			return 2
		}
	}
	switch e.payload.Role.Value {
	case "start", "begin", "up", "down", "crescendo", "diminuendo":
		return 1
	case "stop", "end":
		return -1
	case "continue":
		return 2
	}
	return 0
}
func spanSelfContained(e *spanEndpoint) bool {
	if e.payload.Role.State != EvidenceKnown {
		return false
	}
	r := e.payload.Role.Value
	return (e.payload.Family == SpanTied && r == "let-ring") || r == "single" || (e.payload.Family == SpanTremolo && r == "unmeasured") || (e.payload.Family == SpanBeam && (r == "forward hook" || r == "backward hook"))
}
func spanPairable(f SpanFamily) bool {
	switch f {
	case SpanSlur, SpanTied, SpanTie, SpanBeam, SpanTuplet, SpanGlissando, SpanSlide, SpanOctaveShift, SpanBracket, SpanDashes, SpanWavyLine, SpanHammerOn, SpanPullOff, SpanPartGroup, SpanWedge, SpanLyricExtend, SpanGrouping, SpanOtherNotation, SpanMetronomeTied, SpanTremolo, SpanPedal, SpanPrincipalVoice, SpanBarre:
		return true
	}
	return false
}
func spanAtomicKey(ctx context.Context, s *Session, v Evidence[Atomic]) (string, bool, error) {
	if v.State != EvidenceKnown {
		return "", false, nil
	}
	n := int64(len(v.Value.Text)) + v.Value.Integer.ByteSize() + v.Value.Decimal.ByteSize() + 128
	if err := s.step(ctx, n); err != nil {
		return "", false, err
	}
	switch v.Value.Kind {
	case AtomicString:
		return v.Value.Text, true, nil
	case AtomicInteger:
		if v.Value.Integer.Valid() {
			return v.Value.Integer.String(), true, nil
		}
	}
	return "", false, nil
}
func spanKey(ctx context.Context, s *Session, e *spanEndpoint) (spanAssociationKey, bool, error) {
	p := e.payload
	k := spanAssociationKey{family: p.Family, identity: p.Identity}
	if p.Domain.State != EvidenceKnown {
		return k, false, nil
	}
	k.domain = p.Domain.Value
	if err := s.step(ctx, 1+int64(len(k.domain.DocumentID)+len(e.pitch)+len(e.guard)+len(e.lyric)+len(p.Voice.Value))); err != nil {
		return k, false, err
	}
	var ok bool
	var err error
	switch p.Identity {
	case SpanIdentityNumber, SpanIdentityPartGroup, SpanIdentityBeamLevel, SpanIdentityGrouping:
		k.number, ok, err = spanAtomicKey(ctx, s, p.Number)
		if !ok || err != nil {
			return k, false, err
		}
	case SpanIdentityImpliedTie:
		if e.pitch == "" {
			return k, false, nil
		}
		k.pitch = e.pitch
	case SpanIdentityLyric:
		if e.lyric == "" {
			return k, false, nil
		}
		k.lyric = e.lyric
	case SpanIdentityUnnumbered:
		if p.Family == SpanTie || p.Family == SpanBarre {
			if e.pitch == "" {
				return k, false, nil
			}
			k.pitch = e.pitch
			k.guard = e.guard
		}
	default:
		return k, false, nil
	}
	if p.Family == SpanGrouping {
		k.lyric = e.lyric
	}
	if p.Family == SpanBeam {
		if p.Voice.State == EvidenceKnown {
			k.voice = p.Voice.Value
			k.voiceSpecified = true
		} else if p.Voice.State == EvidenceAbsent {
			k.voice = ""
		} else {
			return k, false, nil
		}
		k.grace = e.grace
		k.cue = e.cue
	}
	return k, true, nil
}

// associateSpanEndpoints consumes document-order records. Its ordinary numbered
// association is an explicitly bounded interpretation, never a balance predicate.
// No score-order sort or LIFO stack is used; a stop may be the first boundary.
func associateSpanEndpoints(ctx context.Context, s *Session, es []*spanEndpoint, timeline *timelineService) error {
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	if err := s.retain(512); err != nil {
		return err
	}
	runs := make(map[spanAssociationKey]*spanAssociationRun)
	for i, e := range es {
		if err := s.step(ctx, 1); err != nil {
			return err
		}
		if e.payload.Association.State == EvidenceKnown && e.payload.Association.Value {
			continue
		}
		if spanSelfContained(e) {
			if err := spanSetAssociation(ctx, s, e, []NodeRef{e.ref}); err != nil {
				return err
			}
			continue
		}
		if !spanPairable(e.payload.Family) {
			continue
		}
		k, ok, err := spanKey(ctx, s, e)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		run := runs[k]
		if run == nil {
			if err := s.retain(512 + int64(len(k.number)+len(k.pitch)+len(k.guard)+len(k.voice)+len(k.lyric)+len(k.domain.DocumentID))); err != nil {
				return err
			}
			run = &spanAssociationRun{complete: Evidence[bool]{State: EvidenceKnown, Value: true}}
			runs[k] = run
		}
		if err := s.retain(32); err != nil {
			return err
		}
		run.indices = append(run.indices, i)
		if e.complete.State != EvidenceKnown || !e.complete.Value {
			run.complete = timelineMergeAvailability(run.complete, e.complete)
		}
	}
	for _, run := range runs {
		if err := s.step(ctx, 1); err != nil {
			return err
		}
		if run.complete.State != EvidenceKnown || !run.complete.Value {
			for _, i := range run.indices {
				es[i].payload.Association = stateUnavailable[bool, bool](run.complete)
			}
			continue
		}
		pending := -1
		from := 0
		ambiguous := false
		for j, i := range run.indices {
			if err := s.step(ctx, 1); err != nil {
				return err
			}
			e := es[i]
			role := spanBoundaryRole(e)
			if ambiguous {
				continue
			}
			if role == 0 {
				ambiguous = true
				continue
			}
			if role == 2 {
				if pending < 0 {
					ambiguous = true
				}
				continue
			}
			if pending < 0 {
				pending = i
				from = j
				continue
			}
			prev := es[pending]
			if spanBoundaryRole(prev) == role {
				ambiguous = true
				continue
			}
			a, b := prev, e
			if role == 1 {
				a, b = e, prev
			}
			// If both positions prove reversed chronology, do not invent a musical
			// relation. Equal and unknown chronology do not erase document identity.
			if timeline != nil && a.payload.Position.State == EvidenceKnown && b.payload.Position.State == EvidenceKnown {
				ap, bp := a.payload.Position.Value, b.payload.Position.Value
				ord, err := timeline.State.positionOrder(ctx, s, musicalPosition{Part: ap.Part, Measure: ap.Measure, MeasureOrdinal: ap.MeasureOrdinal, Offset: ap.Offset, UnprovedFrame: ap.UnprovedFrame}, musicalPosition{Part: bp.Part, Measure: bp.Measure, MeasureOrdinal: bp.MeasureOrdinal, Offset: bp.Offset, UnprovedFrame: bp.UnprovedFrame})
				if err != nil {
					return err
				}
				if ord == stateAfter {
					pending = -1
					continue
				}
			}
			for _, member := range run.indices[from : j+1] {
				if err := spanSetAssociation(ctx, s, es[member], []NodeRef{a.ref, b.ref}); err != nil {
					return err
				}
			}
			pending = -1
		}
	}
	return nil
}
func spanSetAssociation(ctx context.Context, s *Session, e *spanEndpoint, members []NodeRef) error {
	if err := s.step(ctx, int64(len(members))+1); err != nil {
		return err
	}
	if err := stateRetainCount(s, len(members), 64); err != nil {
		return err
	}
	e.payload.Members = append([]NodeRef(nil), members...)
	e.payload.Association = Evidence[bool]{State: EvidenceKnown, Value: true}
	return nil
}
