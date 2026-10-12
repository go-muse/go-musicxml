package validation

import (
	"context"
	"math/big"
	"strconv"
	"strings"
)

type spanFamilyBinding struct {
	family             SpanFamily
	node, role, number contextRole
	identity           SpanIdentityKind
}

var spanFamilies = []spanFamilyBinding{
	{SpanSlur, spanRole_slur, spanRole_slur_type, spanRole_slur_number, SpanIdentityNumber},
	{SpanTied, spanRole_tied, spanRole_tied_type, spanRole_tied_number, SpanIdentityNumber},
	{SpanTie, spanRole_tie, spanRole_tie_type, "", SpanIdentityUnnumbered},
	{SpanBeam, spanRole_beam, "", spanRole_beam_number, SpanIdentityBeamLevel},
	{SpanTuplet, spanRole_tuplet, spanRole_tuplet_type, spanRole_tuplet_number, SpanIdentityNumber},
	{SpanGlissando, spanRole_glissando, spanRole_glissando_type, spanRole_glissando_number, SpanIdentityNumber},
	{SpanSlide, spanRole_slide, spanRole_slide_type, spanRole_slide_number, SpanIdentityNumber},
	{SpanOctaveShift, spanRole_octaveShift, spanRole_octave_shift_type, spanRole_octave_shift_number, SpanIdentityNumber},
	{SpanBracket, spanRole_bracket, spanRole_bracket_type, spanRole_bracket_number, SpanIdentityNumber},
	{SpanDashes, spanRole_dashes, spanRole_dashes_type, spanRole_dashes_number, SpanIdentityNumber},
	{SpanWavyLine, spanRole_wavyLine, spanRole_wavy_line_type, spanRole_wavy_line_number, SpanIdentityNumber},
	{SpanWavyLine, spanRole_barlineWavy, spanRole_wavy_line_type, spanRole_wavy_line_number, SpanIdentityNumber},
	{SpanHammerOn, spanRole_hammerOn, spanRole_hammer_on_pull_off_type, spanRole_hammer_on_pull_off_number, SpanIdentityNumber},
	{SpanPullOff, spanRole_pullOff, spanRole_hammer_on_pull_off_type, spanRole_hammer_on_pull_off_number, SpanIdentityNumber},
	{SpanLyricExtend, spanRole_lyricExtend, spanRole_extend_type, "", SpanIdentityLyric},
	{SpanFigureExtend, spanRole_figureExtend, spanRole_extend_type, "", SpanIdentityUnnumbered},
	{SpanPartGroup, spanRole_partGroup, spanRole_part_group_type, spanRole_part_group_number, SpanIdentityPartGroup},
	{SpanBarre, spanRole_barre, spanRole_barre_type, "", SpanIdentityUnnumbered},
	{SpanArpeggiate, spanRole_arpeggiate, "", spanRole_arpeggiate_number, SpanIdentityNumber},
	{SpanNonArpeggiate, spanRole_nonArpeggiate, spanRole_non_arpeggiate_type, spanRole_non_arpeggiate_number, SpanIdentityNumber},
	{SpanPedal, spanRole_pedal, spanRole_pedal_type, spanRole_pedal_number, SpanIdentityNumber},
	{SpanWedge, spanRole_wedge, spanRole_wedge_type, spanRole_wedge_number, SpanIdentityNumber},
	{SpanGrouping, spanRole_grouping, spanRole_grouping_type, spanRole_grouping_number, SpanIdentityGrouping},
	{SpanOtherNotation, spanRole_otherNotation, spanRole_other_notation_type, spanRole_other_notation_number, SpanIdentityNumber},
	{SpanPrincipalVoice, spanRole_principalVoice, spanRole_principal_voice_type, "", SpanIdentityUnnumbered},
	{SpanMetronomeTied, spanRole_metronomeTied, spanRole_metronome_tied_type, "", SpanIdentityUnnumbered},
	{SpanTremolo, spanRole_tremolo, spanRole_tremolo_type, "", SpanIdentityUnnumbered},
}

func spanAncestor(r *contextReader, n *factNode, roles ...contextRole) *factNode {
	for n != nil {
		if !r.step(1) {
			return nil
		}
		for _, role := range roles {
			if r.matches(n, role) {
				return n
			}
		}
		if n.header.Parent == 0 {
			return nil
		}
		n = r.s.nodes[n.header.Parent]
	}
	return nil
}
func spanRef(s *Session, n *factNode) Evidence[NodeRef] {
	if n == nil {
		return stateUnknown[NodeRef]("span-anchor-unavailable")
	}
	return Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{s.documentID, n.header.ID}}
}
func spanPosition(p Evidence[musicalPosition]) Evidence[SpanPosition] {
	if p.State != EvidenceKnown {
		return stateUnavailable[musicalPosition, SpanPosition](p)
	}
	v := p.Value
	return Evidence[SpanPosition]{State: EvidenceKnown, Value: SpanPosition{v.Part, v.Measure, v.MeasureOrdinal, v.Offset, v.UnprovedFrame}}
}
func spanDomainIndex(r *contextReader, t *timelineService) map[NodeID]Evidence[NodeRef] {
	if t == nil {
		return nil
	}
	if err := r.s.retain(512); err != nil {
		r.err = err
		return nil
	}
	out := make(map[NodeID]Evidence[NodeRef])
	for id, f := range t.Facts {
		if !r.step(1) {
			return nil
		}
		if f.Part.State != EvidenceKnown {
			continue
		}
		for n := r.s.nodes[id]; n != nil; n = r.s.nodes[n.header.Parent] {
			if !r.step(1 + int64(len(f.Part.Value.DocumentID))) {
				return nil
			}
			if old, ok := out[n.header.ID]; ok {
				if old.State != EvidenceKnown || old.Value == f.Part.Value {
					break
				}
				out[n.header.ID] = stateUnknown[NodeRef]("span-multiple-part-domain")
				continue
			}
			if err := r.s.retain(256); err != nil {
				r.err = err
				return nil
			}
			out[n.header.ID] = f.Part
		}
	}
	return out
}
func spanInheritedDomain(r *contextReader, n *factNode, domains map[NodeID]Evidence[NodeRef]) Evidence[NodeRef] {
	for n != nil {
		if !r.step(1) {
			return stateUnknown[NodeRef]("context-interrupted")
		}
		if d, ok := domains[n.header.ID]; ok {
			return d
		}
		n = r.s.nodes[n.header.Parent]
	}
	return stateUnknown[NodeRef]("span-part-unavailable")
}
func spanPitch(r *contextReader, note *factNode) (string, Evidence[bool]) {
	pitch, pe := r.child(note, spanRole_notePitch)
	if pe.State == EvidenceAbsent {
		return "", stateUnknown[bool]("span-pitched-coordinate-unavailable")
	}
	if pe.State != EvidenceKnown {
		return "", stateUnavailable[bool, bool](pe)
	}
	step := r.childScalar(pitch, spanRole_pitchStep)
	oct := stateInteger(r.childScalar(pitch, spanRole_pitchOctave))
	alt := r.childScalar(pitch, spanRole_pitchAlter)
	if step.State != EvidenceKnown {
		return "", stateUnavailable[Atomic, bool](step)
	}
	if oct.State != EvidenceKnown {
		return "", stateUnavailable[Integer, bool](oct)
	}
	var semitone int64
	switch step.Value.Text {
	case "C":
		semitone = 0
	case "D":
		semitone = 2
	case "E":
		semitone = 4
	case "F":
		semitone = 5
	case "G":
		semitone = 7
	case "A":
		semitone = 9
	case "B":
		semitone = 11
	default:
		return "", stateUnknown[bool]("span-pitch-unavailable")
	}

	if err := r.s.step(r.ctx, oct.Value.ByteSize()+256); err != nil {
		r.err = err
		return "", stateUnknown[bool]("span-pitch-unavailable")
	}
	if err := r.s.retain(oct.Value.ByteSize()*2 + 512); err != nil {
		r.err = err
		return "", stateUnknown[bool]("span-pitch-unavailable")
	}
	base := new(big.Int).Mul(oct.Value.value, big.NewInt(12))
	base.Add(base, big.NewInt(semitone))
	q, err := r.s.rationalResult(r.ctx, new(big.Rat).SetInt(base))
	if err != nil {
		r.err = err
		return "", stateUnknown[bool]("span-pitch-unavailable")
	}
	if alt.State != EvidenceAbsent {
		d := stateDecimalValue(alt)
		if d.State != EvidenceKnown {
			return "", stateUnavailable[Decimal, bool](d)
		}
		a, err := r.s.rationalFromDecimal(r.ctx, d.Value)
		if err != nil {
			r.err = err
			return "", stateUnknown[bool]("span-pitch-unavailable")
		}
		q, err = r.s.rationalAdd(r.ctx, q, a)
		if err != nil {
			r.err = err
			return "", stateUnknown[bool]("span-pitch-unavailable")
		}
	}
	return q.String(), Evidence[bool]{State: EvidenceKnown, Value: true}
}
func spanExtract(r *contextReader, key RuleKey, n *factNode, b spanFamilyBinding, t *timelineService, domains map[NodeID]Evidence[NodeRef], order uint64) *spanEndpoint {
	if err := r.s.retain(3072); err != nil {
		r.err = err
		return nil
	}
	e := &spanEndpoint{ref: NodeRef{r.s.documentID, n.header.ID}, order: order, complete: r.enumerated(n)}
	p := &e.payload
	p.Family = b.family
	p.Identity = b.identity
	p.Role = Evidence[string]{State: EvidenceAbsent}
	p.Number = Evidence[Atomic]{State: EvidenceAbsent}
	p.NumberExplicit = Evidence[bool]{State: EvidenceAbsent}
	p.Staff = Evidence[Integer]{State: EvidenceAbsent}
	p.Voice = Evidence[string]{State: EvidenceAbsent}
	p.Position = stateUnknown[SpanPosition]("span-timeline-position-unavailable")
	p.Association = stateUnknown[bool]("span-correspondence-unresolved")
	if b.role != "" {
		p.Role = spanText(r.attribute(n, b.role))
	} else if b.family == SpanBeam {
		p.Role = spanText(r.scalar(n))
	}
	if b.family == SpanTremolo && p.Role.State == EvidenceAbsent {
		p.Role = Evidence[string]{State: EvidenceKnown, Value: "single"}
	}
	if b.number != "" {
		p.Number = r.attribute(n, b.number)
		p.NumberExplicit = r.presence(n, b.number)
		if b.family == SpanTied && p.Number.State == EvidenceAbsent {
			p.Identity = SpanIdentityImpliedTie
		} else if p.Number.State == EvidenceAbsent {
			if b.family == SpanPartGroup || b.family == SpanGrouping {
				p.Number = spanString("1")
			} else {
				p.Number = spanInteger(1)
			}
		}
	}
	anchor := spanAncestor(r, n, spanRole_note, spanRole_direction, spanRole_barline, spanRole_partList)
	p.Anchor = spanRef(r.s, anchor)
	p.Domain = spanInheritedDomain(r, n, domains)
	if t != nil && anchor != nil {
		if f, ok := t.Facts[anchor.header.ID]; ok {
			p.Domain = f.Part
			p.Staff = f.Staff
			p.Voice = f.Voice
			p.Position = spanPosition(f.Position)
		}
	}
	note := spanAncestor(r, n, spanRole_note)
	if note != nil {
		p.Voice = spanText(r.childScalar(note, spanRole_noteVoice))
		p.Staff = stateInteger(r.childScalar(note, spanRole_noteStaff))
		_, g := r.child(note, spanRole_noteGrace)
		_, c := r.child(note, spanRole_noteCue)
		e.grace = g.State == EvidenceKnown
		e.cue = c.State == EvidenceKnown
		e.categoryKnown = (g.State == EvidenceKnown || g.State == EvidenceAbsent) && (c.State == EvidenceKnown || c.State == EvidenceAbsent)
		e.complete = timelineMergeAvailability(e.complete, r.enumerated(note))
		if b.family == SpanTied || b.family == SpanTie {
			e.pitch, e.pitchAvailable = spanPitch(r, note)
		}
	}
	if b.family == SpanTie {
		g := r.attribute(n, spanRole_tie_time_only)
		if g.State == EvidenceKnown {
			if r.step(int64(len(g.Value.Text)) + 1) {
				if r.err = r.s.retain(int64(len(g.Value.Text)) + 64); r.err == nil {
					e.guard = strings.ReplaceAll(g.Value.Text, " ", "")
				}
			}
		} else if g.State != EvidenceAbsent {
			e.complete = timelineMergeAvailability(e.complete, stateUnavailable[Atomic, bool](g))
		}
	}
	if b.family == SpanTie && note != nil {
		g := r.attribute(note, spanRole_noteTimeOnly)
		if g.State == EvidenceKnown {
			if r.step(int64(len(g.Value.Text)+len(e.guard)) + 32) {
				if r.err = r.s.retain(int64(len(g.Value.Text)+len(e.guard)) + 128); r.err == nil {
					e.guard += "/note:" + strings.ReplaceAll(g.Value.Text, " ", "")
				}
			}
		} else if g.State != EvidenceAbsent {
			e.complete = timelineMergeAvailability(e.complete, stateUnavailable[Atomic, bool](g))
		}
	}

	if b.family == SpanPartGroup {
		p.Domain = spanRef(r.s, spanAncestor(r, n, spanRole_partList))
		p.Anchor = p.Domain
	}
	if b.family == SpanMetronomeTied {
		p.Domain = spanRef(r.s, spanAncestor(r, n, spanRole_metronome))
		p.Position = Evidence[SpanPosition]{State: EvidenceAbsent}
	}
	if b.family == SpanBarre {
		p.Domain = spanRef(r.s, spanAncestor(r, n, spanRole_frame))
		p.Anchor = spanRef(r.s, spanAncestor(r, n, spanRole_frameNote))
		p.Position = Evidence[SpanPosition]{State: EvidenceAbsent}
		frameNote := spanAncestor(r, n, spanRole_frameNote)
		f := r.childScalar(frameNote, spanRole_frameFret)
		if f.State == EvidenceKnown {
			if k, ok, err := spanAtomicKey(r.ctx, r.s, f); err != nil {
				r.err = err
			} else if ok {
				e.pitch = k
			}
		}
	}

	if b.family == SpanGrouping {
		m := r.attribute(n, spanRole_grouping_member_of)
		if m.State == EvidenceKnown {
			e.lyric = m.Value.Text
		} else if m.State != EvidenceAbsent {
			e.complete = timelineMergeAvailability(e.complete, stateUnavailable[Atomic, bool](m))
		}
	}
	if b.family == SpanLyricExtend {
		lyric := spanAncestor(r, n, spanRole_noteLyric)
		number, name := r.attribute(lyric, spanRole_lyric_number), r.attribute(lyric, spanRole_lyric_name)
		if number.State == EvidenceKnown || name.State == EvidenceKnown {
			if (number.State == EvidenceKnown || number.State == EvidenceAbsent) && (name.State == EvidenceKnown || name.State == EvidenceAbsent) {
				if r.step(int64(len(number.Value.Text)+len(name.Value.Text)) + 32) {
					if err := r.s.retain(int64(len(number.Value.Text)+len(name.Value.Text)) + 128); err != nil {
						r.err = err
					} else {
						e.lyric = strconv.Itoa(len(number.Value.Text)) + ":" + number.Value.Text + ":" + name.Value.Text
					}
				}
			}
		}
	}
	if (b.family == SpanLyricExtend || b.family == SpanFigureExtend) && p.Role.State == EvidenceAbsent {
		version := spanDefault(r.attribute(r.s.root, spanRole_version), spanString("1.0"))
		if version.State != EvidenceKnown {
			p.Role = stateUnavailable[Atomic, string](version)
		} else if spanIs(version, "1.0") || spanIs(version, "1.1") || spanIs(version, "2.0") {
			p.Role = Evidence[string]{State: EvidenceKnown, Value: "start"}
		} else {
			p.Role = stateUnknown[string]("span-extend-type-unspecified")
		}
	}
	if p.Role.State != EvidenceKnown && p.Role.State != EvidenceAbsent {
		p.Association = stateUnavailable[string, bool](p.Role)
		e.complete = timelineMergeAvailability(e.complete, stateUnavailable[string, bool](p.Role))
	}
	if b.number != "" && p.Number.State != EvidenceKnown && p.Number.State != EvidenceAbsent {
		p.Association = stateUnavailable[Atomic, bool](p.Number)
	}
	if p.Domain.State == EvidenceInvalid || p.Domain.State == EvidenceUnsupported {
		p.Association = stateUnavailable[NodeRef, bool](p.Domain)
	}
	if b.family == SpanBeam && !e.categoryKnown {
		e.complete = timelineMergeAvailability(e.complete, stateUnknown[bool]("span-beam-note-category-unavailable"))
	}

	spanLocalFacts(r, key, n, note, e)
	return e
}

func evaluateContextSpans(ctx context.Context, s *Session, n *factNode, key RuleKey, p *contextPlan, final bool) (decision Decision, pending bool, err error) {
	if !final {
		return Decision{}, true, nil
	}
	service, err := s.newContextService(ctx, key)
	if err != nil {
		return Decision{}, false, err
	}
	if service.Spans != nil && service.Spans.Final {
		return Decision{Outcome: OutcomePass}, false, nil
	}
	beforeRetained, beforeResults := s.retained, s.contextResults
	defer func() {
		if err != nil {
			if len(s.contextResults) > len(beforeResults) {
				clear(s.contextResults[len(beforeResults):])
			}
			s.contextResults = beforeResults
			delta := s.retained - beforeRetained
			if delta > 0 {
				_ = s.account.Release(BudgetBufferedBytes, delta)
				s.retained = beforeRetained
			}
		}
	}()
	if err = s.retain(512); err != nil {
		return Decision{}, false, err
	}
	out := &contextSpanService{}
	r := &contextReader{ctx: ctx, s: s, b: p.Bindings}
	timeline := s.timeline[p.Timeline]
	domains := spanDomainIndex(r, timeline)
	if r.err != nil {
		return Decision{}, false, r.err
	}
	if err = s.retain(512); err != nil {
		return Decision{}, false, err
	}
	gaps := make(map[NodeRef]Evidence[bool])
	localGaps := make(map[NodeRef]Evidence[bool])
	var metronomeSide NodeID
	for order, id := range s.sequence {
		node := s.nodes[id]
		if !r.step(1) {
			break
		}
		if r.matches(node, spanRole_metronome) {
			metronomeSide = node.header.ID
		}
		if r.matches(node, spanRole_metronomeRelation) {
			metronomeSide = node.header.ID
		}
		complete := r.enumerated(node)
		if complete.State != EvidenceKnown || !complete.Value || node.header.Declaration.State != EvidenceKnown {
			d := spanInheritedDomain(r, node, domains)
			if d.State != EvidenceKnown {
				owner := spanAncestor(r, node, spanRole_partList)
				if owner != nil {
					d = spanRef(s, owner)
				}
			}
			if d.State == EvidenceKnown {
				if complete.State == EvidenceKnown {
					complete = stateUnavailable[DeclarationID, bool](node.header.Declaration)
				}
				if _, ok := gaps[d.Value]; !ok {
					if r.err = s.retain(256); r.err != nil {
						break
					}
				}
				if old, ok := gaps[d.Value]; ok {
					gaps[d.Value] = timelineMergeAvailability(old, complete)
				} else {
					gaps[d.Value] = complete
				}
			}
		}
		if complete.State != EvidenceKnown || !complete.Value || node.header.Declaration.State != EvidenceKnown {
			anchor := spanAncestor(r, node, spanRole_note, spanRole_frame, spanRole_metronome, spanRole_partList)
			if anchor != nil {
				a := NodeRef{s.documentID, anchor.header.ID}
				g := complete
				if g.State == EvidenceKnown {
					g = stateUnavailable[DeclarationID, bool](node.header.Declaration)
				}
				if old, ok := localGaps[a]; ok {
					localGaps[a] = timelineMergeAvailability(old, g)
				} else {
					if r.err = s.retain(256); r.err != nil {
						break
					}
					localGaps[a] = g
				}
			}
		}

		if r.matches(node, spanRole_noteModification) {
			spanModificationFacts(r, key, node)
		}
		if r.matches(node, spanRole_noteLyric) {
			spanLyricFacts(r, key, node)
		}
		for _, binding := range spanFamilies {
			if !r.matches(node, binding.node) {
				continue
			}
			e := spanExtract(r, key, node, binding, timeline, domains, uint64(order))
			if e == nil {
				break
			}
			if binding.family == SpanMetronomeTied {
				e.relationSide = metronomeSide
			}
			if timeline == nil && service.Timeline.State != "" && service.Timeline.State != EvidenceAbsent && service.Timeline.State != EvidenceKnown {
				e.payload.Position = stateUnavailable[bool, SpanPosition](service.Timeline)
				if e.payload.Domain.State != EvidenceKnown {
					e.payload.Domain = stateUnavailable[bool, NodeRef](service.Timeline)
				}
			}

			if r.err = s.retain(32); r.err != nil {
				break
			}
			out.endpoints = append(out.endpoints, e)
			break
		}
	}
	if r.err != nil {
		return Decision{}, false, r.err
	}
	for _, e := range out.endpoints {
		if e.payload.Domain.State == EvidenceKnown {
			if gap, ok := localGaps[e.payload.Domain.Value]; ok {
				e.complete = timelineMergeAvailability(e.complete, gap)
			}
		}
		if e.payload.Anchor.State == EvidenceKnown {
			if gap, ok := localGaps[e.payload.Anchor.Value]; ok {
				e.complete = timelineMergeAvailability(e.complete, gap)
			}
		}
	}
	if err = spanLocalTiePairs(ctx, s, out.endpoints); err != nil {
		return Decision{}, false, err
	}

	for _, e := range out.endpoints {
		if e.payload.Domain.State == EvidenceKnown {
			if gap, ok := gaps[e.payload.Domain.Value]; ok {
				e.complete = timelineMergeAvailability(e.complete, gap)
			}
		}
	}
	if err = associateSpanEndpoints(ctx, s, out.endpoints, timeline); err != nil {
		return Decision{}, false, err
	}
	if err = spanVerticalMembership(ctx, s, out.endpoints); err != nil {
		return Decision{}, false, err
	}
	if err = spanPartGroupMembers(r, out.endpoints); err != nil {
		return Decision{}, false, err
	}
	spanRelationalFacts(r, key, out.endpoints, timeline)
	if r.err != nil {
		return Decision{}, false, r.err
	}
	for _, e := range out.endpoints {
		if !r.step(1) {
			break
		}
		node := s.nodes[e.ref.Node]
		r.err = s.publishContextResult(ctx, ContextResult{Provider: key, Subject: e.ref, Property: ContextPropertySpan, Span: &e.payload, Origins: []Location{node.header.Location}})
		if r.err != nil {
			break
		}
	}
	if r.err != nil {
		return Decision{}, false, r.err
	}
	out.Final = true
	service.Spans = out
	return Decision{Outcome: OutcomePass}, false, nil
}
