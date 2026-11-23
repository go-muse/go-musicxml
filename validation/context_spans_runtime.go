package validation

import (
	"context"
	"math/big"
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

// The family table is static; exact declarations and their role sets are owned
// by the shared compiled bindings rather than reindexed by each provider run.
var spanFamilyByRole = func() map[contextRole]spanFamilyBinding {
	out := make(map[contextRole]spanFamilyBinding, len(spanFamilies))
	for _, binding := range spanFamilies {
		out[binding.node] = binding
	}
	return out
}()

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
		n = r.s.node(n.header.Parent)
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
	interrupted := false
	t.forEachFact(func(id NodeID, f timelineEventFact) bool {
		if !r.step(1) {
			interrupted = true
			return false
		}
		if f.Part.State != EvidenceKnown {
			return true
		}
		for n := r.s.node(id); n != nil; n = r.s.node(n.header.Parent) {
			if !r.step(1 + int64(len(f.Part.Value.DocumentID))) {
				interrupted = true
				return false
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
				interrupted = true
				return false
			}
			out[n.header.ID] = f.Part
		}
		return true
	})
	if interrupted {
		return nil
	}
	return out
}

// spanDomains answers which part a node belongs to by its timeline facts.
// The buffered provider indexes the whole document; a window answers from its
// own nodes and notes a question that leaves them.
type spanDomains struct {
	index  map[NodeID]Evidence[NodeRef]
	window *windowSpans
}

func (d *spanDomains) inherited(r *contextReader, n *factNode) Evidence[NodeRef] {
	if d == nil {
		return spanInheritedDomain(r, n, nil)
	}
	if d.window != nil {
		return d.window.inheritedDomain(r, n)
	}
	return spanInheritedDomain(r, n, d.index)
}

func spanInheritedDomain(r *contextReader, n *factNode, domains map[NodeID]Evidence[NodeRef]) Evidence[NodeRef] {
	for n != nil {
		if !r.step(1) {
			return stateUnknown[NodeRef]("context-interrupted")
		}
		if d, ok := domains[n.header.ID]; ok {
			return d
		}
		n = r.s.node(n.header.Parent)
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

// spanCollector reads span endpoints node by node, in document order. The
// buffered provider gives it the whole document in Finish; a window gives it
// the window's nodes when the window closes, and Finish the rest.
type spanCollector struct {
	key           RuleKey
	timeline      *timelineService
	service       *contextService
	domains       *spanDomains
	gaps          map[NodeRef]Evidence[bool]
	localGaps     map[NodeRef]Evidence[bool]
	lyrics        *spanLyricIndex
	metronomeSide NodeID
	endpoints     []*spanEndpoint
	// window is set while windows collect: an endpoint then keeps what later
	// passes would read from its node, and an extend whose role waits for
	// the root's version is finished in Finish.
	window *windowSpans
}

func (c *spanCollector) extract(r *contextReader, n *factNode, b spanFamilyBinding, order uint64) *spanEndpoint {
	key, t, lyrics := c.key, c.timeline, c.lyrics
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
		p.Role = stateText(r.attribute(n, b.role))
	} else if b.family == SpanBeam {
		p.Role = stateText(r.scalar(n))
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
	fact, located := timelineEventFact{}, false
	if t != nil && anchor != nil {
		fact, located = t.fact(anchor.header.ID)
	}
	// A window asks for the inherited domain only where nothing replaces it:
	// the answer may lie outside the window.
	inherit := c.window == nil || !located && b.family != SpanPartGroup && b.family != SpanMetronomeTied && b.family != SpanBarre
	if inherit {
		p.Domain = c.domains.inherited(r, n)
	}
	if located {
		p.Domain = fact.Part
		p.Staff = fact.Staff
		p.Voice = fact.Voice
		p.Position = spanPosition(fact.Position)
	}
	note := spanAncestor(r, n, spanRole_note)
	if note != nil {
		p.Voice = stateText(r.childScalar(note, spanRole_noteVoice))
		p.Staff = stateInteger(r.childScalar(note, spanRole_noteStaff))
		_, g := r.child(note, spanRole_noteGrace)
		_, c := r.child(note, spanRole_noteCue)
		e.grace = g.State == EvidenceKnown
		e.cue = c.State == EvidenceKnown
		e.categoryKnown = (g.State == EvidenceKnown || g.State == EvidenceAbsent) && (c.State == EvidenceKnown || c.State == EvidenceAbsent)
		e.categoryAvailable = Evidence[bool]{State: EvidenceKnown, Value: true}
		for _, category := range []Evidence[bool]{g, c} {
			if category.State != EvidenceKnown && category.State != EvidenceAbsent {
				e.categoryAvailable = timelineMergeAvailability(e.categoryAvailable, category)
			}
		}
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
		p.Position = Evidence[SpanPosition]{State: EvidenceAbsent}
	}
	if b.family == SpanMetronomeTied {
		p.Domain = spanRef(r.s, spanAncestor(r, n, spanRole_metronome))
		p.Position = Evidence[SpanPosition]{State: EvidenceAbsent}
	}
	if b.family == SpanBarre {
		p.Domain = spanRef(r.s, spanAncestor(r, n, spanRole_frame))
		frameNote := spanAncestor(r, n, spanRole_frameNote)
		p.Anchor = spanRef(r.s, frameNote)
		p.Position = Evidence[SpanPosition]{State: EvidenceAbsent}
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
		var available Evidence[bool]
		e.lyric, available = lyrics.endpoint(r, lyric)
		e.complete = timelineMergeAvailability(e.complete, available)
	}
	if (b.family == SpanLyricExtend || b.family == SpanFigureExtend) && p.Role.State == EvidenceAbsent {
		if c.window != nil && !c.window.final {
			// The root is bound in Finish; its version is read there, and
			// what follows the role here waits with it. Neither family has
			// local facts or a number, and neither is a beam or arpeggio.
			e.versionRole = true
			c.capture(r, n, e)
			return e
		}
		spanExtendRole(r, e)
	}
	spanRoleAvailability(e, b)
	if (b.family == SpanBeam || b.family == SpanArpeggiate || b.family == SpanNonArpeggiate) && !e.categoryKnown {
		e.complete = timelineMergeAvailability(e.complete, spanCategoryAvailability(e))
	}

	mark := c.windowFacts(r)
	spanLocalFacts(r, key, n, note, e)
	c.windowFactsDone(r, mark)
	c.capture(r, n, e)
	return e
}

// spanExtendRole gives an extend without a type the role its document's
// version implies.
func spanExtendRole(r *contextReader, e *spanEndpoint) {
	p := &e.payload
	version := spanDefault(r.attribute(r.s.root, spanRole_version), spanString("1.0"))
	if version.State != EvidenceKnown {
		p.Role = stateUnavailable[Atomic, string](version)
	} else if spanIs(version, "1.0") || spanIs(version, "1.1") || spanIs(version, "2.0") {
		p.Role = Evidence[string]{State: EvidenceKnown, Value: "start"}
	} else {
		p.Role = stateUnknown[string]("span-extend-type-unspecified")
	}
}

// spanRoleAvailability withholds the association of an endpoint whose role,
// number or domain is unavailable.
func spanRoleAvailability(e *spanEndpoint, b spanFamilyBinding) {
	p := &e.payload
	if p.Role.State != EvidenceKnown && p.Role.State != EvidenceAbsent {
		p.Association = stateUnavailable[string, bool](p.Role)
		e.complete = timelineMergeAvailability(e.complete, stateUnavailable[string, bool](p.Role))
	}
	if !spanSelfContained(e) && b.number != "" && p.Number.State != EvidenceKnown && p.Number.State != EvidenceAbsent {
		p.Association = stateUnavailable[Atomic, bool](p.Number)
	}
	if !spanSelfContained(e) && (p.Domain.State == EvidenceInvalid || p.Domain.State == EvidenceUnsupported) {
		p.Association = stateUnavailable[NodeRef, bool](p.Domain)
	}
}

// capture keeps, for an endpoint a window read, what the passes of Finish
// would read from its node and its note.
func (c *spanCollector) capture(r *contextReader, n *factNode, e *spanEndpoint) {
	// A counting provider reads its endpoints' nodes while they are at
	// hand and captures nothing.
	if c.window == nil || c.window.counting() || r.err != nil {
		return
	}
	charge := spanCapturedBytes
	k := &spanCaptured{origins: r.s.nodeOrigins(n)}
	for _, origin := range k.origins {
		charge += spanWindowLocationSize + ownedStringBytes(origin.Path) + 96
	}
	if r.err = r.s.retain(charge); r.err != nil {
		return
	}
	c.window.capturedBytes += charge
	e.captured = k
	role := e.payload.Role
	switch e.payload.Family {
	case SpanBeam:
		if role.State == EvidenceKnown {
			k.fan = spanDefault(r.attribute(n, spanRole_beam_fan), spanString("none"))
			if role.Value == "continue" {
				k.present = r.presence(n, spanRole_beam_fan)
			}
		}
	case SpanTie:
		if role.State == EvidenceKnown && e.payload.Anchor.State == EvidenceKnown {
			attr := spanRole_noteAttack
			if role.Value == "start" {
				attr = spanRole_noteRelease
			}
			k.present = r.presence(r.s.node(e.payload.Anchor.Value.Node), attr)
		}
	case SpanTremolo:
		if role.State == EvidenceKnown && (role.Value == "start" || role.Value == "stop") {
			at := 0
			spanTremoloEach(r, n, c.timeline, func(_ ContextProperty, v Evidence[Atomic]) {
				if at < len(k.tremolo) {
					k.tremolo[at] = v
				}
				at++
			})
		}
	}
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
	beforeReportRetained, beforeShared, beforeIntern := s.contextReportRetained, s.contextSharedTableBytes, s.internRetained
	// beforeResults may be restored, so a backing array replaced meanwhile
	// stays charged until the evaluation commits.
	hold := s.holdContextBacking()
	defer func() {
		s.releaseHeldContextBacking(hold, err == nil)
		if err != nil {
			if len(s.contextResults) > len(beforeResults) {
				clear(s.contextResults[len(beforeResults):])
			}
			s.contextResults = beforeResults
			s.contextReportRetained = beforeReportRetained
			// Shared attribute tables and intern table entries built
			// meanwhile stay in the session and keep their charge.
			delta := s.retained - beforeRetained - (s.contextSharedTableBytes - beforeShared) - (s.internRetained - beforeIntern)
			if delta > 0 {
				_ = s.account.Release(BudgetBufferedBytes, delta)
				s.retained -= delta
			}
		}
	}()
	if err = s.retain(512); err != nil {
		return Decision{}, false, err
	}
	out := &contextSpanService{}
	timeline := s.timeline[p.Timeline]
	// What windows read is adopted here if everything they assumed held;
	// otherwise the whole document is read as the buffered provider reads it.
	r, c, err := s.adoptWindowSpans(ctx, key, p)
	if err != nil {
		return Decision{}, false, err
	}
	scratch := spanScratch{s: s}
	defer scratch.release()
	pending1 := []*spanEndpoint(nil)
	if c == nil {
		r = &contextReader{ctx: ctx, s: s, b: p.Bindings}
		domains := spanDomainIndex(r, timeline)
		if r.err != nil {
			return Decision{}, false, r.err
		}
		if err = s.retain(512); err != nil {
			return Decision{}, false, err
		}
		c = &spanCollector{key: key, timeline: timeline, service: service, domains: &spanDomains{index: domains}, gaps: make(map[NodeRef]Evidence[bool]), localGaps: make(map[NodeRef]Evidence[bool])}
		c.lyrics = &spanLyricIndex{scratch: &scratch, domains: c.domains}
		for order, id := range s.sequence {
			c.visit(r, order, s.node(id))
			if r.err != nil {
				break
			}
		}
		if r.err != nil {
			return Decision{}, false, r.err
		}
		pending1 = c.endpoints
	} else {
		// With no spans projection the windows counted every fact and kept
		// no endpoint: the passes below find nothing to associate.
		// The endpoints windows read met their local gaps when their
		// windows closed; those read here and the waiting extends meet them
		// now.
		if pending1, err = s.finishWindowSpans(ctx, r, c); err != nil {
			return Decision{}, false, err
		}
	}
	out.endpoints = c.endpoints
	gaps, lyrics := c.gaps, c.lyrics
	if !c.mergeLocalGaps(r, pending1) {
		return Decision{}, false, r.err
	}

	for _, e := range out.endpoints {
		if !r.step(1) {
			return Decision{}, false, r.err
		}
		if e.payload.Family == SpanLyricExtend && e.lyric == spanUnnamedLyricIdentity {
			if !r.step(1 + int64(len(e.payload.Domain.Value.DocumentID))) {
				return Decision{}, false, r.err
			}
			if e.payload.Domain.State == EvidenceKnown {
				if gap, ok := lyrics.unnamed[e.payload.Domain.Value]; ok {
					e.complete = timelineMergeAvailability(e.complete, gap)
				}
			}
			if gap, ok := lyrics.unnamed[NodeRef{}]; ok {
				e.complete = timelineMergeAvailability(e.complete, gap)
			}
		}
		if e.payload.Domain.State == EvidenceKnown {
			if gap, ok := gaps[e.payload.Domain.Value]; ok {
				e.complete = timelineMergeAvailability(e.complete, gap)
			}
		}
		// An unknown musical-part domain may cover every part, but cannot
		// make an independently enumerated part-list, frame or metronome incomplete.
		if e.payload.Family != SpanPartGroup && e.payload.Family != SpanBarre && e.payload.Family != SpanMetronomeTied {
			if gap, ok := gaps[NodeRef{}]; ok {
				e.complete = timelineMergeAvailability(e.complete, gap)
			}
		}
	}
	if err = spanLocalTiePairs(ctx, s, out.endpoints); err != nil {
		return Decision{}, false, err
	}
	if err = associateSpanEndpoints(ctx, s, out.endpoints, timeline); err != nil {
		return Decision{}, false, err
	}
	if err = spanVerticalMembership(ctx, s, out.endpoints); err != nil {
		return Decision{}, false, err
	}
	if err = spanPartGroupMembers(r, out.endpoints, c.window); err != nil {
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
		origins := e.origins(s)
		r.err = s.publishContextResult(ctx, ContextResult{Provider: key, Subject: e.ref, Property: ContextPropertySpan, Span: &e.payload, Origins: origins})
		if r.err != nil {
			break
		}
	}
	if r.err != nil {
		return Decision{}, false, r.err
	}
	s.windowSpansAdopted(c)
	out.Final = true
	service.Spans = out
	return Decision{Outcome: OutcomePass}, false, nil
}

// visit reads one node: the gaps an incomplete node leaves, the facts of a
// time modification or a lyric, and the node's endpoint, if it is one.
func (c *spanCollector) visit(r *contextReader, order int, node *factNode) {
	s, key := r.s, c.key
	if !r.step(1) {
		return
	}
	roles := r.roles(node)
	if r.err != nil {
		return
	}
	complete := r.enumerated(node)
	if r.err != nil {
		return
	}
	if complete.State != EvidenceKnown || !complete.Value || node.header.Declaration.State != EvidenceKnown {
		d := c.domains.inherited(r, node)
		if d.State != EvidenceKnown {
			owner := spanAncestor(r, node, spanRole_partList)
			if owner != nil {
				d = spanRef(s, owner)
			}
		}
		if r.err != nil {
			return
		}
		var domain NodeRef
		if d.State == EvidenceKnown {
			domain = d.Value
		}
		if complete.State == EvidenceKnown {
			complete = stateUnavailable[DeclarationID, bool](*node.header.Declaration)
		}
		if c.window != nil {
			// Windows and the nodes outside them are read out of document
			// order; Finish folds their gaps in that order.
			c.windowGap(r, domain, complete)
			if r.err != nil {
				return
			}
		} else if old, ok := c.gaps[domain]; ok {
			c.gaps[domain] = timelineMergeAvailability(old, complete)
		} else {
			if !r.step(1) {
				return
			}
			if r.err = s.retain(256); r.err != nil {
				return
			}
			c.gaps[domain] = complete
		}
	}
	if complete.State != EvidenceKnown || !complete.Value || node.header.Declaration.State != EvidenceKnown {
		anchor := spanAncestor(r, node, spanRole_note, spanRole_frame, spanRole_metronome, spanRole_partList)
		if r.err != nil {
			return
		}
		if anchor != nil {
			a := NodeRef{s.documentID, anchor.header.ID}
			g := complete
			if g.State == EvidenceKnown {
				g = stateUnavailable[DeclarationID, bool](*node.header.Declaration)
			}
			if old, ok := c.localGaps[a]; ok {
				c.localGaps[a] = timelineMergeAvailability(old, g)
			} else {
				if r.err = s.retain(256); r.err != nil {
					return
				}
				c.localGaps[a] = g
			}
		}
	}

	// roles has charged the complete returned traversal. Propagate an
	// interrupted handler before another branch can overwrite its error.
	for _, role := range roles {
		switch role {
		case spanRole_metronome, spanRole_metronomeRelation:
			c.metronomeSide = node.header.ID
			c.windowOrdered(s)
		case spanRole_noteModification:
			mark := c.windowFacts(r)
			spanModificationFacts(r, key, node)
			c.windowFactsDone(r, mark)
		case spanRole_noteLyric:
			// The lyric index keeps its own copies in scratch.
			mark := c.windowFacts(r)
			c.lyrics.record(r, node)
			spanLyricFacts(r, key, node)
			c.windowFactsDone(r, mark)
			c.windowOrdered(s)
		}
		if r.err != nil {
			return
		}
		binding, ok := spanFamilyByRole[role]
		if !ok {
			continue
		}
		e := c.extract(r, node, binding, uint64(order))
		if e == nil || r.err != nil {
			return
		}
		if binding.family == SpanMetronomeTied {
			e.relationSide = c.metronomeSide
			c.windowOrdered(s)
		}
		if c.timeline == nil && c.service.Timeline.State != "" && c.service.Timeline.State != EvidenceAbsent && c.service.Timeline.State != EvidenceKnown {
			if e.payload.Position.State != EvidenceAbsent {
				e.payload.Position = stateUnavailable[bool, SpanPosition](c.service.Timeline)
			}
			if e.payload.Domain.State != EvidenceKnown {
				e.payload.Domain = stateUnavailable[bool, NodeRef](c.service.Timeline)
			}
		}

		if r.err = s.retain(32); r.err != nil {
			return
		}
		c.endpoints = append(c.endpoints, e)
		return
	}
}

// mergeLocalGaps lets each endpoint meet the gaps of its own note, frame,
// metronome or part-list, and a lyric extend those of its note's lyrics.
func (c *spanCollector) mergeLocalGaps(r *contextReader, endpoints []*spanEndpoint) bool {
	for _, e := range endpoints {
		if !r.step(1) {
			return false
		}
		c.mergeLocalGap(e)
	}
	return true
}

func (c *spanCollector) mergeLocalGap(e *spanEndpoint) {
	if e.payload.Domain.State == EvidenceKnown {
		if gap, ok := c.localGaps[e.payload.Domain.Value]; ok {
			e.complete = timelineMergeAvailability(e.complete, gap)
		}
	}
	if e.payload.Anchor.State == EvidenceKnown {
		if gap, ok := c.localGaps[e.payload.Anchor.Value]; ok {
			e.complete = timelineMergeAvailability(e.complete, gap)
		}
		if e.payload.Family == SpanLyricExtend {
			for _, identity := range []string{e.lyric, ""} {
				if gap, ok := c.lyrics.scopes[spanLyricScopeKey{note: e.payload.Anchor.Value, identity: identity}]; ok {
					e.complete = timelineMergeAvailability(e.complete, gap)
				}
			}
		}
	}
}
