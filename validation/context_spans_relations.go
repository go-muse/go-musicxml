package validation

import (
	"context"
	"math/big"
)

// A local visual stub is independent of score timing and remote enumeration.
func spanLocalTiePairs(ctx context.Context, s *Session, es []*spanEndpoint) error {
	if err := s.retain(512); err != nil {
		return err
	}
	last := make(map[NodeRef]*spanEndpoint)
	for _, e := range es {
		if err := s.step(ctx, 1); err != nil {
			return err
		}
		if e.payload.Family != SpanTied || e.payload.Anchor.State != EvidenceKnown {
			continue
		}
		a := e.payload.Anchor.Value
		prev := last[a]
		if prev != nil && prev.payload.Role.State == EvidenceKnown && prev.payload.Role.Value == "start" && e.payload.Role.State == EvidenceKnown && e.payload.Role.Value == "stop" && prev.payload.Identity == e.payload.Identity {
			same := e.payload.Identity == SpanIdentityImpliedTie
			if !same {
				x, xok, err := spanAtomicKey(ctx, s, prev.payload.Number)
				if err != nil {
					return err
				}
				y, yok, err := spanAtomicKey(ctx, s, e.payload.Number)
				if err != nil {
					return err
				}
				same = xok && yok && x == y
			}
			if same && prev.complete.State == EvidenceKnown && prev.complete.Value && e.complete.State == EvidenceKnown && e.complete.Value {
				if err := spanSetAssociation(ctx, s, prev, []NodeRef{prev.ref, e.ref}); err != nil {
					return err
				}
				if err := spanSetAssociation(ctx, s, e, []NodeRef{prev.ref, e.ref}); err != nil {
					return err
				}
				delete(last, a)
				continue
			}
		}
		if _, ok := last[a]; !ok {
			if err := s.retain(256 + int64(len(a.DocumentID))); err != nil {
				return err
			}
		}
		last[a] = e
	}
	return nil
}

type spanPartListMember struct {
	ref   NodeRef
	order uint64
}

func spanPartGroupMembers(r *contextReader, es []*spanEndpoint) error {
	if err := r.s.retain(1024); err != nil {
		return err
	}
	endpoints := make(map[NodeRef]*spanEndpoint)
	lists := make(map[NodeRef][]spanPartListMember)
	for _, e := range es {
		if !r.step(1) {
			return r.err
		}
		if e.payload.Family == SpanPartGroup {
			if err := r.s.retain(256); err != nil {
				return err
			}
			endpoints[e.ref] = e
		}
	}
	for order, id := range r.s.sequence {
		n := r.s.nodes[id]
		if !r.matches(n, spanRole_scorePart) {
			continue
		}
		owner := spanAncestor(r, n, spanRole_partList)
		if owner == nil {
			continue
		}
		key := NodeRef{r.s.documentID, owner.header.ID}
		if err := r.s.retain(256 + int64(len(key.DocumentID))); err != nil {
			return err
		}
		lists[key] = append(lists[key], spanPartListMember{NodeRef{r.s.documentID, id}, uint64(order)})
	}
	for _, e := range es {
		if !r.step(1) {
			return r.err
		}
		if e.payload.Family != SpanPartGroup || e.payload.Association.State != EvidenceKnown || len(e.payload.Members) != 2 {
			continue
		}
		a, b := endpoints[e.payload.Members[0]], endpoints[e.payload.Members[1]]
		if a == nil || b == nil {
			continue
		}
		if a.order >= b.order {
			e.payload.Association = stateUnknown[bool]("span-part-group-boundary-order")
			e.payload.Members = nil
			continue
		}
		parts := lists[e.payload.Domain.Value]
		lo, hi := 0, len(parts)
		for lo < hi {
			if !r.step(1) {
				return r.err
			}
			mid := lo + (hi-lo)/2
			if parts[mid].order <= a.order {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		for i := lo; i < len(parts) && parts[i].order < b.order; i++ {
			if !r.step(1) {
				return r.err
			}
			if err := r.s.retain(64 + int64(len(parts[i].ref.DocumentID))); err != nil {
				return err
			}
			e.payload.Included = append(e.payload.Included, parts[i].ref)
		}
	}
	return r.err
}

func spanModificationFacts(r *contextReader, key RuleKey, n *factNode) {
	note := spanAncestor(r, n, spanRole_note)
	spanPublish(r, key, n, ContextPropertySpanTimeModificationMeaning, spanString("cumulative-sounding-ratio"))
	spanPublish(r, key, n, ContextPropertySpanTimeModificationActual, r.childScalar(n, spanRole_actualNotes))
	spanPublish(r, key, n, ContextPropertySpanTimeModificationNormal, r.childScalar(n, spanRole_normalNotes))
	typ := r.childScalar(note, spanRole_noteType)
	spanPublish(r, key, n, ContextPropertySpanCurrentType, typ)
	spanPublish(r, key, n, ContextPropertySpanCurrentDots, spanCountChildren(r, note, spanRole_noteDot))
	spanPublish(r, key, n, ContextPropertySpanNormalType, spanDefault(r.childScalar(n, spanRole_normalType), typ))
	spanPublish(r, key, n, ContextPropertySpanNormalDots, spanEffectiveNormalDots(r, n, spanCountChildren(r, note, spanRole_noteDot)))
}
func spanLyricFacts(r *contextReader, key RuleKey, n *factNode) {
	complete := r.complete(n)
	if complete.State != EvidenceKnown || !complete.Value {
		spanPublish(r, key, n, ContextPropertySpanLyricSyllable, stateUnavailable[bool, Atomic](complete))
		return
	}
	var members []NodeRef
	publish := func() {
		if len(members) == 0 || r.err != nil {
			return
		}
		p := SpanContextResult{Family: SpanLyricSyllable, Role: Evidence[string]{State: EvidenceKnown, Value: "syllable"}, Domain: spanRef(r.s, n), Anchor: spanRef(r.s, spanAncestor(r, n, spanRole_note)), Number: Evidence[Atomic]{State: EvidenceAbsent}, NumberExplicit: Evidence[bool]{State: EvidenceAbsent}, Identity: SpanIdentityLyric, Staff: Evidence[Integer]{State: EvidenceAbsent}, Voice: Evidence[string]{State: EvidenceAbsent}, Position: Evidence[SpanPosition]{State: EvidenceAbsent}, Association: Evidence[bool]{State: EvidenceKnown, Value: true}, Members: members}
		r.err = r.s.publishContextResult(r.ctx, ContextResult{Provider: key, Subject: members[0], Property: ContextPropertySpanLyricSyllable, Span: &p, Origins: []Location{n.header.Location}})
		members = nil
	}
	for _, ch := range n.children {
		if !r.step(1) {
			return
		}
		if ch.header.Declaration.State != EvidenceKnown {
			spanPublish(r, key, n, ContextPropertySpanLyricSyllable, stateUnavailable[DeclarationID, Atomic](ch.header.Declaration))
			return
		}
		if r.matches(ch, spanRole_lyricElision) {
			publish()
		}
		if r.matches(ch, spanRole_lyricText) {
			if r.err = r.s.retain(64 + int64(len(r.s.documentID))); r.err != nil {
				return
			}
			members = append(members, NodeRef{r.s.documentID, ch.header.ID})
		}
	}
	publish()
}

// Recommendations remain context projections. A false recommendation is not a
// failed normative predicate or an invented span-closure obligation.
func spanRelationalFacts(r *contextReader, key RuleKey, es []*spanEndpoint, timeline *timelineService) {
	if r.err = r.s.retain(1024); r.err != nil {
		return
	}
	byRef := make(map[NodeRef]*spanEndpoint)
	lastFan := make(map[NodeRef]Evidence[Atomic])
	lastNote := make(map[struct {
		note   NodeRef
		family SpanFamily
	}]*spanEndpoint)
	for _, e := range es {
		if !r.step(1) {
			return
		}
		if r.err = r.s.retain(256 + int64(len(e.ref.DocumentID))); r.err != nil {
			return
		}
		byRef[e.ref] = e
	}
	for _, e := range es {
		if !r.step(1) {
			return
		}
		n := r.s.nodes[e.ref.Node]
		role := e.payload.Role
		var start *spanEndpoint
		if e.payload.Association.State == EvidenceKnown && len(e.payload.Members) == 2 {
			start = byRef[e.payload.Members[0]]
		}
		if e.payload.Family == SpanWedge && role.State == EvidenceKnown && role.Value == "stop" {
			active := stateUnknown[Atomic]("span-wedge-start-required")
			if e.payload.Association.State != EvidenceKnown {
				active = stateUnavailable[bool, Atomic](e.payload.Association)
			}
			if start != nil && start.payload.Role.State == EvidenceKnown {
				active = spanBool(start.payload.Role.Value != "diminuendo")
			}
			spanPublish(r, key, n, ContextPropertySpanWedgeSpreadActive, active)
			if start != nil && start.payload.Role.State == EvidenceKnown {
				active = spanBool(start.payload.Role.Value == "diminuendo")
			}
			spanPublish(r, key, n, ContextPropertySpanWedgeNienteActive, active)
		}
		if e.payload.Family == SpanBeam && role.State == EvidenceKnown {
			fan := spanDefault(r.attribute(n, spanRole_beam_fan), spanString("none"))
			if start != nil && role.Value == "begin" {
				if r.err = r.s.retain(256); r.err != nil {
					return
				}
				lastFan[start.ref] = fan
			}
			if role.Value == "continue" {
				applicable := stateUnknown[Atomic]("span-fan-prior-direction-required")
				present := r.presence(n, spanRole_beam_fan)
				if present.State == EvidenceAbsent || (present.State == EvidenceKnown && !present.Value) {
					applicable = spanBool(false)
				} else if present.State != EvidenceKnown {
					applicable = stateUnavailable[bool, Atomic](present)
				} else if start != nil {
					if prior, ok := lastFan[start.ref]; ok {
						if fan.State == EvidenceKnown && prior.State == EvidenceKnown {
							applicable = spanBool(fan.Value.Text != prior.Value.Text)
						} else if fan.State != EvidenceKnown {
							applicable = stateUnavailable[Atomic, Atomic](fan)
						} else {
							applicable = stateUnavailable[Atomic, Atomic](prior)
						}
					}
				}
				spanPublish(r, key, n, ContextPropertySpanFanApplicable, applicable)
				if start != nil && present.State == EvidenceKnown && present.Value {
					lastFan[start.ref] = fan
				}
			}
		}
		if e.payload.Family == SpanTied && start != nil && e.ref == start.ref {
			stop := byRef[e.payload.Members[1]]
			if stop != nil && start.payload.Anchor.State == EvidenceKnown && stop.payload.Anchor.State == EvidenceKnown && start.payload.Anchor.Value != stop.payload.Anchor.Value {
				value := stateUnknown[Atomic]("span-tie-pitch-unavailable")
				if start.pitch != "" && stop.pitch != "" {
					if !r.step(int64(len(start.pitch)+len(stop.pitch)) + 1) {
						return
					}
					value = spanBool(start.pitch == stop.pitch)
				}
				spanPublish(r, key, n, ContextPropertySpanPitchEquivalentRecommended, value)
			}
		}
		if e.payload.Family == SpanTie && role.State == EvidenceKnown && e.payload.Anchor.State == EvidenceKnown {
			note := r.s.nodes[e.payload.Anchor.Value.Node]
			attr, property := spanRole_noteAttack, ContextPropertySpanTieAttackRecommended
			if role.Value == "start" {
				attr = spanRole_noteRelease
				property = ContextPropertySpanTieReleaseRecommended
			}
			present := r.presence(note, attr)
			value := stateUnavailable[bool, Atomic](present)
			if present.State == EvidenceAbsent {
				value = spanBool(true)
			} else if present.State == EvidenceKnown {
				value = spanBool(!present.Value)
			}
			spanPublish(r, key, n, property, value)
		}
		if e.payload.Anchor.State == EvidenceKnown && role.State == EvidenceKnown {
			k := struct {
				note   NodeRef
				family SpanFamily
			}{e.payload.Anchor.Value, e.payload.Family}
			prev := lastNote[k]
			if prev != nil && prev.payload.Role.State == EvidenceKnown {
				value := stateUnknown[Atomic]("span-same-note-musical-order-unestablished")
				if e.payload.Family == SpanTie && prev.payload.Role.Value != role.Value {
					value = spanBool(prev.payload.Role.Value == "stop" && role.Value == "start")
				}
				if e.payload.Family == SpanTied && prev.payload.Role.Value == "start" && role.Value == "stop" && prev.payload.Association.State == EvidenceKnown && len(prev.payload.Members) == 2 && prev.payload.Members[1] == e.ref {
					value = spanBool(true)
				}
				if e.payload.Family != SpanTie && prev.payload.Association.State == EvidenceKnown && e.payload.Association.State == EvidenceKnown && len(prev.payload.Members) == 2 && len(e.payload.Members) == 2 && prev.payload.Members[0] != e.payload.Members[0] {
					if (prev.payload.Role.Value == "start" && role.Value == "stop") || (prev.payload.Role.Value == "stop" && role.Value == "start") {
						value = spanBool(prev.payload.Role.Value == "stop")
					}
				}
				spanPublish(r, key, n, ContextPropertySpanSameNoteOrderRecommended, value)
			}
			if _, ok := lastNote[k]; !ok {
				if r.err = r.s.retain(256); r.err != nil {
					return
				}
			}
			lastNote[k] = e
		}
		if e.payload.Family == SpanTremolo && role.State == EvidenceKnown && (role.Value == "start" || role.Value == "stop") {
			spanTremoloFacts(r, key, n, e, timeline)
		}
	}
}
func spanTremoloFacts(r *contextReader, key RuleKey, n *factNode, e *spanEndpoint, timeline *timelineService) {
	note := spanAncestor(r, n, spanRole_note)
	mod, me := r.child(note, spanRole_noteModification)
	ratio := stateUnknown[Atomic]("span-tremolo-cumulative-tuplet-ratio")
	ordinary := false
	if me.State == EvidenceAbsent {
		ratio = spanBool(false)
	} else if me.State != EvidenceKnown {
		ratio = stateUnavailable[bool, Atomic](me)
	} else {
		a, b := stateInteger(r.childScalar(mod, spanRole_actualNotes)), stateInteger(r.childScalar(mod, spanRole_normalNotes))
		if a.State != EvidenceKnown {
			ratio = stateUnavailable[Integer, Atomic](a)
		} else if b.State != EvidenceKnown {
			ratio = stateUnavailable[Integer, Atomic](b)
		} else if a.Value.value.Cmp(spanSmallIntegers[2].value) == 0 && b.Value.value.Cmp(spanSmallIntegers[1].value) == 0 {
			ratio = spanBool(true)
			ordinary = true
		}
	}
	spanPublish(r, key, n, ContextPropertySpanTremoloRatioRecommended, ratio)
	duration := stateUnknown[Atomic]("span-tremolo-duration-context-required")
	if timeline != nil && note != nil && (ordinary || me.State == EvidenceAbsent) {
		fact, ok := timeline.Facts[note.header.ID]
		typ := spanText(r.childScalar(note, spanRole_noteType))
		dots := spanCountChildren(r, note, spanRole_noteDot)
		if ok && fact.Duration.State == EvidenceKnown && typ.State == EvidenceKnown && dots.State == EvidenceKnown && dots.Value.Integer.value.IsUint64() {
			denom := int64(0)
			num := int64(1)
			switch typ.Value {
			case "maxima":
				num = 16
				denom = 1
			case "long":
				num = 8
				denom = 1
			case "breve":
				num = 4
				denom = 1
			case "whole":
				num = 2
				denom = 1
			case "half":
				num = 1
				denom = 1
			case "quarter":
				denom = 2
			case "eighth":
				denom = 4
			case "16th":
				denom = 8
			case "32nd":
				denom = 16
			case "64th":
				denom = 32
			case "128th":
				denom = 64
			case "256th":
				denom = 128
			case "512th":
				denom = 256
			case "1024th":
				denom = 512
			}
			if denom > 0 {
				q, err := r.s.rationalResult(r.ctx, new(big.Rat).SetFrac64(num, denom))
				if err != nil {
					r.err = err
					return
				}
				term := q
				two, err := r.s.rationalResult(r.ctx, new(big.Rat).SetInt64(2))
				if err != nil {
					r.err = err
					return
				}
				for count := dots.Value.Integer.value.Uint64(); count > 0; count-- {
					if !r.step(1) {
						return
					}
					term, err = r.s.rationalQuo(r.ctx, term, two)
					if err != nil {
						r.err = err
						return
					}
					q, err = r.s.rationalAdd(r.ctx, q, term)
					if err != nil {
						r.err = err
						return
					}
				}

				cmp, err := r.s.rationalCmp(r.ctx, fact.Duration.Value, q)
				if err != nil {
					r.err = err
					return
				}
				duration = spanBool(cmp == 0)
			}
		} else if ok && fact.Duration.State != EvidenceKnown {
			duration = stateUnavailable[Rational, Atomic](fact.Duration)
		}
	}
	spanPublish(r, key, n, ContextPropertySpanTremoloDurationRecommended, duration)
}
