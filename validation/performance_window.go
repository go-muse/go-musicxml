package validation

import (
	"context"
	"math"
	"sort"
)

// windowPerformance is the provisional performance work of one session. The
// single sealed provider owns context records; each bend rule owns a group
// adopted only when Finish reaches its phase.
type windowPerformance struct {
	provider *performanceWindowProvider
	bends    []performanceWindowBend
}

type performanceWindowProvider struct {
	key      RuleKey
	plan     *contextPlan
	timeline RuleKey
	phase    int

	partOrder int64
	reader    *contextReader
	swing     performanceSwingHistory
	divisions map[NodeRef]performanceDivisionAdvice
	// The read counts make the next window's advice index an increment, not a
	// reservation proportional to every fact in the document.
	divisionsRead, midiRead, instrumentsRead int

	records      []performanceWindowRecord
	pendingFacts int64
	stateFacts   int64
	indexCharged int64
	charged      int64
	partStart    int
	anchors      []performanceWindowAnchor
	anchorStart  int
	ranges       [][2]NodeID

	adoptedTimeline RuleKey
	project         bool
	failed          string
	failedReported  bool
	adopted         bool
}

type performanceWindowRecord struct {
	result  ContextResult
	charged int64
}

type performanceWindowAnchor struct {
	node   NodeID
	record int
}

type performanceWindowBend struct {
	key, provider RuleKey
	phase         int
	group         consumerGroup
}

const (
	performanceWindowStateBytes int64 = 4096
	performanceWindowBendBytes  int64 = 256
	performanceWindowSlotBytes  int64 = 512
)

func (s *Session) indexWindowPerformance(ctx context.Context) error {
	if s.performanceIndexed {
		return nil
	}
	s.performanceIndexed = true
	var provider *performanceWindowProvider
	var bends []performanceWindowBend
	for phase, entry := range s.catalog.phases[s.profile.ID] {
		r := s.catalog.rules[entry.rule]
		plan := s.catalog.plans[r.Predicate]
		switch plan.kind {
		case planContextPerformance:
			provider = &performanceWindowProvider{key: entry.rule, plan: plan.context, timeline: plan.context.Timeline, phase: phase, divisions: make(map[NodeRef]performanceDivisionAdvice), project: s.contextFamilies&(ContextFamilyTiming|ContextFamilyPerformance) != 0}
			provider.swing.global = Evidence[bool]{State: EvidenceKnown, Value: true}
		case planBendReleaseSign:
			bends = append(bends, performanceWindowBend{key: entry.rule, provider: plan.context.Provider, phase: phase})
		}
	}
	if provider == nil {
		return nil
	}
	if err := s.retain(performanceWindowStateBytes + int64(len(bends))*performanceWindowBendBytes); err != nil {
		return err
	}
	service, err := s.newContextService(ctx, provider.key)
	if err != nil {
		return err
	}
	service.Timeline = Evidence[bool]{State: EvidenceKnown, Value: true}
	s.performance = &windowPerformance{provider: provider, bends: bends}
	return nil
}

func (s *Session) performanceReader(ctx context.Context, p *performanceWindowProvider) *contextReader {
	if p.reader == nil {
		p.reader = &contextReader{s: s, b: p.plan.Bindings}
	}
	p.reader.ctx, p.reader.err = ctx, nil
	return p.reader
}

func (s *Session) evaluateWindowPerformance(ctx context.Context, w evaluationWindow) error {
	if s.strategy != strategyWindowed || s.resourceSnapshot || windowPerformanceOff {
		return nil
	}
	if err := s.indexWindowPerformance(ctx); err != nil {
		return err
	}
	k := s.performance
	if k == nil || k.provider == nil {
		return nil
	}
	p := k.provider
	if p.failed != "" {
		return nil
	}
	spec := s.speculations[p.timeline]
	switch {
	case spec == nil || spec.service == nil || spec.stopped != timelineTriggerNone:
		p.failed = "speculation"
		return nil
	case len(spec.service.unitExcursions) != 0:
		p.failed = "excursions"
		return nil
	case len(spec.service.State.Divisions.Gaps) != 0:
		p.failed = "divisions-gaps"
		return nil
	}
	s.performanceStash = p
	s.speculating = spec
	defer func() {
		s.performanceStash = nil
		s.speculating = nil
	}()
	service := spec.service
	r := s.performanceReader(ctx, p)
	if err := performanceWindowSwing(r, service, w); err != nil {
		return err
	}
	late, err := performanceWindowLateState(r, service, w)
	if err != nil {
		return err
	}
	if late {
		p.failed = "late-state"
		return nil
	}
	if !p.project {
		return s.countWindowPerformance(ctx, w, p, service, r)
	}
	tb, err := p.plan.Schema.sharedTimelineBindings()
	if err != nil {
		return err
	}
	sr := &stateReader{ctx: ctx, s: s, b: tb}
	index, err := s.performanceWindowAdviceIndex(ctx, p, service)
	if err != nil {
		return err
	}
	defer index.release(s)
	for _, id := range s.sequence[w.first:w.end] {
		if !r.step(1) {
			return r.err
		}
		n := s.node(id)
		if err := s.transientScope(func() error {
			return performanceProjectNode(ctx, s, sr, r, p.key, service, index, &p.swing, &p.partOrder, n)
		}); err != nil {
			return err
		}
		if err := s.notePerformanceAnchor(sr, service, n); err != nil {
			return err
		}
		if err := s.evaluateWindowBends(ctx, w, p, n); err != nil {
			return err
		}
	}
	if r.err == nil {
		return s.recordPerformanceRange(p, w)
	}
	return r.err
}

func (s *Session) recordPerformanceRange(p *performanceWindowProvider, w evaluationWindow) error {
	first, end := s.sequence[w.first], s.sequence[w.end-1]+1
	if len(p.ranges) == cap(p.ranges) {
		grown := max(8, 2*cap(p.ranges))
		if err := s.retain(int64(grown) * 32); err != nil {
			return err
		}
		next := make([][2]NodeID, len(p.ranges), grown)
		copy(next, p.ranges)
		s.releaseFactBytes(int64(cap(p.ranges)) * 32)
		p.charged += int64(grown-cap(p.ranges)) * 32
		p.ranges = next
	}
	p.ranges = append(p.ranges, [2]NodeID{first, end})
	return nil
}

func (s *Session) evaluateWindowBends(ctx context.Context, w evaluationWindow, p *performanceWindowProvider, n *factNode) error {
	k := s.performance
	service := s.contextServices[p.key]
	if service == nil || service.Performance == nil {
		return nil
	}
	if _, ok := service.Performance.Bends[n.header.ID]; !ok {
		return nil
	}
	for i := range k.bends {
		b := &k.bends[i]
		if b.provider != p.key {
			continue
		}
		s.performanceBendEvaluating = b
		s.accumulator.at = findingKey{stage: stagePhases, phase: b.phase, trigger: int64(w.first)}
		for trigger, node := range s.sequence[w.first:w.end] {
			if node == n.header.ID {
				s.accumulator.at.trigger = int64(w.first + trigger)
			}
		}
		err := s.evaluate(ctx, n, b.key, true)
		s.performanceBendEvaluating = nil
		if err != nil {
			return err
		}
	}
	return nil
}

// countWindowPerformance counts facts whose payloads are not selected. Bend
// nodes still use the full local projection because their consumer reads proof.
func (s *Session) countWindowPerformance(ctx context.Context, w evaluationWindow, p *performanceWindowProvider, service *timelineService, r *contextReader) error {
	tb, err := p.plan.Schema.sharedTimelineBindings()
	if err != nil {
		return err
	}
	sr := &stateReader{ctx: ctx, s: s, b: tb}
	index := &performanceAdviceIndex{divisions: p.divisions}
	for _, id := range s.sequence[w.first:w.end] {
		if !r.step(1) {
			return r.err
		}
		n := s.node(id)
		if performanceRoles(r.roles(n), nil).has(performanceBend) {
			if err := s.transientScope(func() error {
				return performanceProjectNode(ctx, s, sr, r, p.key, service, index, &p.swing, &p.partOrder, n)
			}); err != nil {
				return err
			}
			if err := s.evaluateWindowBends(ctx, w, p, n); err != nil {
				return err
			}
		} else {
			count, err := performanceCountNode(r, sr, service, n)
			if err != nil {
				return err
			}
			p.pendingFacts, err = rationalCheckedAdd(BudgetContextFacts, p.pendingFacts, int64(count))
			if err != nil {
				return err
			}
		}
		if r.err != nil {
			return r.err
		}
	}
	return s.recordPerformanceRange(p, w)
}

func performanceCountNode(r *contextReader, sr *stateReader, service *timelineService, n *factNode) (int, error) {
	elements, attributes := r.roles(n), r.attributeRoles(n)
	if r.err != nil {
		return 0, r.err
	}
	roles := performanceRoles(elements, attributes)
	if sr.matches(n, timelineRoleMeasure) {
		roles |= performanceStateRole
	}
	count := 0
	for _, role := range attributes {
		d, ok := performanceDefaultsByRole[role]
		if !ok || d.owner == "mordent" && (n.header.Type.State != EvidenceKnown || n.header.Type.Value != contextNamedType("mordent")) || d.owner == "empty-trill-sound" && n.header.Type.State == EvidenceKnown && n.header.Type.Value == contextNamedType("mordent") {
			continue
		}
		if !r.step(1) {
			return 0, r.err
		}
		count++
	}
	for _, role := range []performanceNodeRoles{performanceFingering, performancePluck, performancePrefix, performanceSuffix} {
		if roles.has(role) {
			count++
		}
	}
	hasFact := service != nil && service.hasFact(n.header.ID)
	if !hasFact && !roles.timing() {
		return count, r.err
	}
	publish := hasFact || roles.has(performanceNote) || roles.has(performanceSound) || roles.has(performanceMIDIUnpitched) || roles.has(performancePan) || roles.has(performanceElevation) || roles.has(performanceSwingRole) || roles.has(performanceBend) || roles.has(performanceMovementRole)
	if roles.has(performanceStateRole) {
		switch {
		case sr.matches(n, timelineRoleMeasure):
			publish = true
		case sr.matches(n, timelineRoleScorePart):
			publish = true
		case sr.matches(n, timelineRoleAttributesKey):
			if r.complete(n).State == EvidenceKnown && r.s.performanceStash != nil {
				r.s.performanceStash.stateFacts++
			}
			publish = true
		case sr.matches(n, timelineRoleAttributesTime):
			publish = true
		case sr.matches(n, timelineRoleAttributesTranspose):
			publish = true
		case sr.matches(n, timelineRoleAttributesStaffDetails):
			publish = true
		}
	}
	if roles.has(performanceFret) || roles.has(performanceLevel) || roles.has(performanceRootAlter) || roles.has(performanceTrill) {
		publish = true
	}
	if roles.has(performanceAccidental) && r.matches(r.s.node(n.header.Parent), "perf-ornaments") {
		publish = true
	}
	if sr.err != nil {
		return 0, sr.err
	}
	if publish {
		count++
	}
	return count, r.err
}

func (s *Session) performanceWindowAdviceIndex(ctx context.Context, p *performanceWindowProvider, service *timelineService) (*performanceAdviceIndex, error) {
	newDivisions := len(service.State.Divisions.Publications) - p.divisionsRead
	newMIDI := len(service.State.MIDI) - p.midiRead
	newInstruments := len(service.State.NoteInstruments) - p.instrumentsRead
	count, err := rationalCheckedAdd(BudgetBufferedBytes, int64(newDivisions+newMIDI+newInstruments), 1)
	if err != nil {
		return nil, err
	}
	cost, err := rationalCheckedMul(BudgetBufferedBytes, count, 1024)
	if err != nil {
		return nil, err
	}
	if err := s.account.Charge(BudgetBufferedBytes, cost); err != nil {
		return nil, err
	}
	x := &performanceAdviceIndex{bytes: cost, movementDivisions: make(map[performanceMeasure][]int), movementGaps: make(map[performanceMeasure]Evidence[bool]), midiByParent: make(map[NodeID][]int), divisions: p.divisions, instruments: make(map[NodeID]Evidence[bool])}
	for i := p.divisionsRead; i < len(service.State.Divisions.Publications); i++ {
		v := service.State.Divisions.Publications[i]
		if err := s.step(ctx, 1+int64(len(v.Position.Part.DocumentID))); err != nil {
			x.release(s)
			return nil, err
		}
		part := v.Position.Part
		x.movementDivisions[performanceMeasure{part, v.Position.MeasureOrdinal}] = append(x.movementDivisions[performanceMeasure{part, v.Position.MeasureOrdinal}], i)
		d, ok := p.divisions[part]
		if !ok {
			if err := s.retain(256 + int64(len(part.DocumentID))); err != nil {
				x.release(s)
				return nil, err
			}
			d.complete = Evidence[bool]{State: EvidenceKnown}
		}
		d.count++
		d.initial = v.Position.MeasureOrdinal == 0 && !v.Position.UnprovedFrame && v.Position.Offset.value.Sign() == 0
		if v.Value.State != EvidenceKnown {
			d.complete = stateUnavailable[Decimal, bool](v.Value)
		}
		p.divisions[part] = d
	}
	p.divisionsRead = len(service.State.Divisions.Publications)
	for _, set := range service.State.NoteInstruments[p.instrumentsRead:] {
		if err := s.step(ctx, 1); err != nil {
			x.release(s)
			return nil, err
		}
		if _, ok := x.instruments[set.Note.Node]; ok {
			continue
		}
		identified := set.Complete
		if identified.State == EvidenceKnown {
			identified.Value = len(set.References) > 0
			for _, ref := range set.References {
				if ref.State != EvidenceKnown {
					identified = stateUnavailable[NodeRef, bool](ref)
					break
				}
			}
		}
		x.instruments[set.Note.Node] = identified
	}
	p.instrumentsRead = len(service.State.NoteInstruments)
	for i := p.midiRead; i < len(service.State.MIDI); i++ {
		m := service.State.MIDI[i]
		if err := s.step(ctx, 1); err != nil {
			x.release(s)
			return nil, err
		}
		if n := s.node(m.Source.Node); n != nil {
			x.midiByParent[n.header.Parent] = append(x.midiByParent[n.header.Parent], i)
		}
	}
	p.midiRead = len(service.State.MIDI)
	return x, nil
}

func (s *Session) notePerformanceAnchor(r *stateReader, service *timelineService, n *factNode) error {
	p := s.performanceStash
	if p == nil || len(p.records) == 0 {
		return nil
	}
	needed := false
	if origin, ok := service.noteOrigin(n.header.ID); ok {
		needed = origin.Grace.State != EvidenceKnown || origin.Grace.Value
	}
	if r.matches(n, timelineRoleDirection) || r.matches(n, timelineRoleFiguredBass) {
		needed = true
	}
	if r.err != nil {
		return r.err
	}
	if !needed {
		return nil
	}
	at := len(p.records) - 1
	if p.records[at].result.Subject.Node != n.header.ID || p.records[at].result.Timing == nil {
		return nil
	}
	if len(p.anchors) == cap(p.anchors) {
		grown := max(8, 2*cap(p.anchors))
		if err := s.retain(int64(grown) * 64); err != nil {
			return err
		}
		next := make([]performanceWindowAnchor, len(p.anchors), grown)
		copy(next, p.anchors)
		s.releaseFactBytes(int64(cap(p.anchors)) * 64)
		p.charged += int64(grown-cap(p.anchors)) * 64
		p.anchors = next
	}
	p.anchors = append(p.anchors, performanceWindowAnchor{node: n.header.ID, record: at})
	return nil
}

// performanceWindowLateState rejects a publication whose displaced position or
// gaps can change records projected from an earlier window. A displacement
// within the window is already known before its nodes are projected.
func performanceWindowLateState(r *contextReader, service *timelineService, w evaluationWindow) (bool, error) {
	for _, id := range r.s.sequence[w.first:w.end] {
		if !r.step(1) {
			return false, r.err
		}
		if !r.matches(r.s.node(id), "perf-sound") {
			continue
		}
		sound, ok := service.Playback.Sounds[id]
		if !ok {
			continue
		}
		position := sound.Origin.Position
		if position.State == EvidenceKnown && (position.Value.MeasureOrdinal < sound.Origin.MeasureOrdinal || position.Value.Offset.value.Sign() < 0) {
			return true, nil
		}
		if position.State != EvidenceKnown && sound.Origin.Placement.Displaced {
			return true, nil
		}
	}
	if r.err != nil {
		return false, r.err
	}
	late := len(service.Playback.Tempo.Gaps) != 0 || len(service.Playback.Dynamics.Gaps) != 0 || len(service.Playback.Pizzicato.Gaps) != 0
	return late || len(r.s.performanceStash.swing.history.Gaps) != 0, nil
}

func performanceWindowSwing(r *contextReader, service *timelineService, w evaluationWindow) error {
	p := r.s.performanceStash
	for _, id := range r.s.sequence[w.first:w.end] {
		if !r.step(1) {
			return r.err
		}
		n := r.s.node(id)
		if !r.matches(n, "perf-sound") {
			continue
		}
		swing, c := r.child(n, "perf-swing")
		if c.State == EvidenceAbsent {
			continue
		}
		origin, ok := service.Playback.Sounds[id]
		if !ok {
			p.swing.global = stateUnknown[bool]("swing-origin-unavailable")
			continue
		}
		v := stateUnavailable[bool, performanceSwing](c)
		if c.State == EvidenceKnown {
			v = Evidence[performanceSwing]{State: EvidenceKnown, Value: performanceReadSwing(r, swing)}
		}
		if r.err != nil {
			return r.err
		}
		if origin.Origin.Position.State != EvidenceKnown || origin.Origin.Part.State != EvidenceKnown {
			if origin.Origin.Part.State != EvidenceKnown {
				p.swing.global = stateUnavailable[NodeRef, bool](origin.Origin.Part)
				continue
			}
			gap := stateUnavailable[musicalPosition, bool](origin.Origin.Position)
			if gap.State == EvidenceAbsent || gap.State == EvidenceKnown {
				gap = stateUnknown[bool]("swing-origin-unavailable")
			}
			if err := addStateGap(r.ctx, r.s, &p.swing.history, stateGap{Placement: origin.Origin.Placement, Part: origin.Origin.Part.Value, FirstMeasure: origin.Origin.MeasureOrdinal, LastMeasure: origin.Origin.MeasureOrdinal, Scope: stateScope{AllStaves: true}, Availability: gap}); err != nil {
				return err
			}
			continue
		}
		condition := origin.TimeOnly
		var guard *Evidence[string]
		if !stateUnconditional(&condition) {
			guard = &condition
		}
		publication := statePublication[performanceSwing]{Position: origin.Origin.Position.Value, Scope: stateScope{AllStaves: true}, Value: v, Provenance: []stateProvenance{{Node: NodeRef{r.s.documentID, id}, Kind: stateExplicit}}, TimeOnly: guard}
		if err := publishState(r.ctx, r.s, &p.swing.history, publication); err != nil {
			return err
		}
	}
	return r.err
}

// closePerformancePart finishes part-scoped anchors and the initial-divisions
// advice while the part's nodes and speculative service are still available.
func (s *Session) closePerformancePart(ctx context.Context, n *factNode) error {
	k := s.performance
	if k == nil || k.provider == nil || s.resourceSnapshot {
		return nil
	}
	p := k.provider
	if p.failed != "" || !p.project {
		return nil
	}
	spec := s.speculations[p.timeline]
	if spec == nil || spec.service == nil || spec.partState[n.header.ID] == nil {
		return nil
	}
	if p.anchorStart < len(p.anchors) {
		scratch, err := rationalCheckedMul(BudgetBufferedBytes, int64(len(spec.blocks)), 256)
		if err != nil {
			return err
		}
		if err := s.account.Charge(BudgetBufferedBytes, scratch); err != nil {
			return err
		}
		defer func() { _ = s.account.Release(BudgetBufferedBytes, scratch) }()
		blocks := make([]timelineMeasureBlock, 0, len(spec.blocks))
		for _, block := range spec.blocks {
			if err := s.step(ctx, 1); err != nil {
				return err
			}
			if block.measure.header.Parent == n.header.ID {
				blocks = append(blocks, block)
			}
		}
		service := spec.service
		service.windowOriginsIncremental = true
		before := s.retained
		s.speculating = spec
		err = service.resolveFiguredBassOriginsAll(ctx, s, spec.plan.Bindings, blocks)
		if err == nil {
			err = service.resolveGraceNeighbors(ctx, s, spec.plan.Bindings, blocks)
		}
		if err == nil {
			err = service.resolveDirectionOrigins(ctx, s, spec.plan.Bindings, blocks)
		}
		s.speculating = nil
		spec.retained += s.retained - before
		if err != nil {
			return err
		}
		for _, anchor := range p.anchors[p.anchorStart:] {
			performanceWindowApplyAnchor(service, anchor.node, p.records[anchor.record].result.Timing)
		}
		p.anchorStart = len(p.anchors)
	}
	for _, record := range p.records[p.partStart:] {
		out := record.result.Timing
		if out == nil || out.Part.State != EvidenceKnown {
			continue
		}
		for i := range out.Advisories {
			if out.Advisories[i].Clause != "one-initial-divisions-per-part" {
				continue
			}
			d, ok := p.divisions[out.Part.Value]
			complete := Evidence[bool]{State: EvidenceKnown, Value: false}
			if ok {
				complete = d.complete
				if complete.State == EvidenceKnown {
					complete.Value = d.count == 1 && d.initial
				}
			}
			out.Advisories[i].Satisfied = complete
		}
	}
	p.partStart = len(p.records)
	return nil
}

func performanceWithoutDivisionsAdvice(raw []TimingAdvisory) []TimingAdvisory {
	out := raw[:0]
	for _, advice := range raw {
		if advice.Clause != "one-initial-divisions-per-part" {
			out = append(out, advice)
		}
	}
	return out
}

func performanceWindowApplyAnchor(service *timelineService, id NodeID, out *TimingContextResult) {
	if out == nil {
		return
	}
	if _, direction := service.DirectionOrigins[id]; direction {
		out.Advisories = performanceWithoutDivisionsAdvice(out.Advisories)
	} else if _, figured := service.FiguredBassOrigins[id]; figured {
		out.Advisories = performanceWithoutDivisionsAdvice(out.Advisories)
	}
	if g, ok := service.GraceNeighbors[id]; ok {
		out.Previous = performanceDonor(g.Previous)
		out.Following = performanceDonor(g.Following)
	}
	if v, ok := service.FiguredBassOrigins[id]; ok {
		out.Kind = TimingKindFiguredBass
		out.Anchor = v.Anchor
		out.AnchorPosition = performancePosition(v.AnchorPosition)
		out.AnchorStaff = v.AnchorStaff
		out.AnchorVoice = v.AnchorVoice
		out.DurationRaw = v.DurationRaw
		out.AnchorUnits = v.AnchorUnits
		out.DurationUnits = v.DurationUnits
		out.ChainPosition = performancePosition(v.ChainPosition)
		out.ChainEnd = performancePosition(v.ChainEnd)
		out.GroupStart = v.GroupStart
		out.ChainInterpretation = v.ChainInterpretation
		out.Figures = v.Figures
		out.FiguresComplete = v.FiguresComplete
	}
	if v, ok := service.DirectionOrigins[id]; ok {
		out.Kind = TimingKindDirection
		out.Anchor = v.Anchor
		out.AnchorPosition = performancePosition(v.AnchorPosition)
		out.AnchorStaff = v.AnchorStaff
		out.AnchorVoice = v.AnchorVoice
	}
}

func (s *Session) performanceTimelineAdopted(key RuleKey) {
	if k := s.performance; k != nil && k.provider != nil && k.provider.timeline == key {
		k.provider.adoptedTimeline = key
	}
}

func (s *Session) performancePending() bool {
	k := s.performance
	if k == nil {
		return false
	}
	if p := k.provider; p != nil && (p.pendingFacts != 0 || len(p.records) != 0) {
		return true
	}
	for i := range k.bends {
		if k.bends[i].group.decisions != 0 {
			return true
		}
	}
	return false
}

func (s *Session) performanceWindowAdopted(key RuleKey) bool {
	return s.performance != nil && s.performance.provider != nil && s.performance.provider.key == key && s.performance.provider.adopted
}

func (s *Session) settlePerformancePhase(ctx context.Context, phase int) error {
	k := s.performance
	if k == nil {
		return nil
	}
	p := k.provider
	if p != nil && p.phase == phase && !p.adopted && (p.pendingFacts != 0 || len(p.records) != 0) {
		failed, err := s.confirmPerformanceProvider(ctx, p)
		if err != nil {
			return err
		}
		if failed != "" {
			s.dropWindowPerformance(failed)
		} else if err := s.adoptPerformanceProvider(ctx, p); err != nil {
			return err
		}
	}
	for i := range k.bends {
		b := &k.bends[i]
		if b.phase != phase || b.group.decisions == 0 {
			continue
		}
		if p == nil || !p.adopted {
			s.dropWindowPerformance("prerequisite")
			return nil
		}
		prerequisite := s.documentPrerequisites[b.provider]
		if prerequisite == nil || !prerequisite.finalized {
			s.dropWindowPerformance("prerequisite")
			return nil
		}
		d, pending, err := s.dependency(ctx, s.root, Dependency{Kind: DependencyRule, Rule: b.provider, Relation: RelationDocument}, true)
		if err != nil {
			return err
		}
		if pending || d.Outcome != OutcomePass {
			s.dropWindowPerformance("prerequisite")
			return nil
		}
		if err := s.adoptPerformanceBend(b); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) confirmPerformanceProvider(ctx context.Context, p *performanceWindowProvider) (string, error) {
	if p.failed != "" {
		return p.failed, nil
	}
	service := s.timeline[p.timeline]
	if p.adoptedTimeline != p.timeline || service == nil || !service.Final {
		return "speculation", nil
	}
	if service.unitProofFinal {
		return "rebuilt", nil
	}
	if p.partStart != len(p.records) || p.anchorStart != len(p.anchors) {
		return "origins", nil
	}
	prerequisite := s.documentPrerequisites[p.timeline]
	if prerequisite == nil || !prerequisite.finalized {
		return "prerequisite", nil
	}
	d, pending, err := s.dependency(ctx, s.root, Dependency{Kind: DependencyRule, Rule: p.timeline, Relation: RelationDocument}, true)
	if err != nil {
		return "", err
	}
	if pending || d.Outcome != OutcomePass {
		return "prerequisite", nil
	}
	return "", nil
}

func (s *Session) adoptPerformanceProvider(ctx context.Context, p *performanceWindowProvider) error {
	if err := s.performanceProjectSkeleton(ctx, p); err != nil {
		return err
	}
	facts, err := rationalCheckedAdd(BudgetContextFacts, p.pendingFacts, p.stateFacts)
	if err != nil {
		return err
	}
	if err := s.account.Charge(BudgetContextFacts, facts); err != nil {
		return err
	}
	p.pendingFacts = 0
	p.stateFacts = 0
	if len(p.records) != 0 {
		if err := s.step(ctx, int64(len(p.records))); err != nil {
			return err
		}
		sort.SliceStable(p.records, func(i, j int) bool {
			return p.records[i].result.Subject.Node < p.records[j].result.Subject.Node
		})
	}
	for i, record := range p.records {
		before := s.retained
		if len(s.contextResults) == cap(s.contextResults) {
			if err := s.growContextResults(); err != nil {
				return err
			}
		}
		s.contextResults = append(s.contextResults, record.result)
		s.contextReportRetained += record.charged + s.retained - before
		p.records[i].charged = 0
	}
	count := len(p.records)
	p.records = nil
	p.anchors = nil
	p.ranges = nil
	p.anchorStart = 0
	s.releaseFactBytes(p.charged)
	p.charged = 0
	p.adopted = true
	if windowPerformanceObserver != nil {
		windowPerformanceObserver("", count, 0)
	}
	return nil
}

func (s *Session) performanceProjectSkeleton(ctx context.Context, p *performanceWindowProvider) error {
	service := s.timeline[p.timeline]
	tb, err := p.plan.Schema.sharedTimelineBindings()
	if err != nil {
		return err
	}
	sr := &stateReader{ctx: ctx, s: s, b: tb}
	r := s.performanceReader(ctx, p)
	index := &performanceAdviceIndex{divisions: p.divisions}
	// These ranges were recorded when the windows closed. Do not reconstruct
	// them by reading measure nodes during the accepted Finish path.
	ranges := p.ranges
	s.performanceStash = p
	defer func() { s.performanceStash = nil }()
	at := 0
	for _, id := range s.sequence {
		for at < len(ranges) && id >= ranges[at][1] {
			at++
		}
		if at < len(ranges) && id >= ranges[at][0] && id < ranges[at][1] {
			continue
		}
		if !r.step(1) {
			return r.err
		}
		if !p.project {
			count, err := performanceCountNode(r, sr, service, s.node(id))
			if err != nil {
				return err
			}
			p.pendingFacts, err = rationalCheckedAdd(BudgetContextFacts, p.pendingFacts, int64(count))
			if err != nil {
				return err
			}
			continue
		}
		if err := s.transientScope(func() error {
			return performanceProjectNode(ctx, s, sr, r, p.key, service, index, &p.swing, &p.partOrder, s.node(id))
		}); err != nil {
			return err
		}
	}
	return r.err
}

func (s *Session) stashPerformanceResult(result ContextResult, charged int64) error {
	p := s.performanceStash
	if len(p.records) == cap(p.records) {
		grown := max(8, 2*cap(p.records))
		if err := s.retain(int64(grown) * performanceWindowSlotBytes); err != nil {
			return err
		}
		next := make([]performanceWindowRecord, len(p.records), grown)
		copy(next, p.records)
		s.releaseFactBytes(int64(cap(p.records)) * performanceWindowSlotBytes)
		p.charged += int64(grown-cap(p.records)) * performanceWindowSlotBytes
		p.records = next
	}
	p.records = append(p.records, performanceWindowRecord{result: result, charged: charged})
	return nil
}

func (s *Session) stashPerformanceBend(r RuleDefinition, n *factNode, d Decision) error {
	b := s.performanceBendEvaluating
	g := &b.group
	if d.Outcome == OutcomePass || d.Outcome == OutcomeNotApplicable {
		bits := g.decided[r.Key]
		if bits == nil {
			if err := s.retainConsumer(g, consumerTableBytes+int64(len(r.Key.ID))); err != nil {
				return err
			}
			if g.decided == nil {
				g.decided = make(map[RuleKey]*outcomeBits)
			}
			bits = &outcomeBits{}
			g.decided[r.Key] = bits
		}
		result := successfulPass
		if d.Outcome == OutcomeNotApplicable {
			result = successfulNotApplicable
		}
		if err := bits.set(s, n.header.ID, result); err != nil {
			return err
		}
		if d.Outcome == OutcomePass {
			g.pass++
		} else {
			g.notApplicable++
		}
	} else {
		d.Expected = reportBoundedValue(d.Expected, reportValueBytes)
		d.Actual = reportBoundedValue(d.Actual, reportValueBytes)
		location := n.location()
		size := reportStorageSize{limit: math.MaxInt64}
		size.decision(d)
		if err := s.retainConsumer(g, size.total+consumerIndexBytes+int64(len(location.Path))); err != nil {
			return err
		}
		if len(g.records) == cap(g.records) {
			grown := max(16, 2*cap(g.records))
			if err := s.retainConsumer(g, int64(grown)*consumerRecordBytes); err != nil {
				return err
			}
			next := make([]consumerRecord, len(g.records), grown)
			copy(next, g.records)
			s.releaseConsumer(g, int64(cap(g.records))*consumerRecordBytes)
			g.records = next
		}
		if g.index == nil {
			g.index = make(map[decisionID]int)
		}
		g.index[decisionID{r.Key, n.header.ID}] = len(g.records)
		g.records = append(g.records, consumerRecord{rule: r.Key, node: n.header.ID, decision: d, location: location, at: s.accumulator.at})
	}
	g.decisions++
	return nil
}

func (s *Session) adoptPerformanceBend(b *performanceWindowBend) error {
	g := b.group
	b.group = consumerGroup{}
	defer s.releaseConsumerGroup(g)
	s.accumulator.report.Counts.Pass += g.pass
	s.accumulator.report.Counts.NotApplicable += g.notApplicable
	for key, bits := range g.decided {
		if err := s.adoptOutcomes(s.catalog.rules[key], bits); err != nil {
			return err
		}
	}
	for _, rec := range g.records {
		r := s.catalog.rules[rec.rule]
		s.accumulator.at = rec.at
		if err := s.accumulator.recordOutcome(r, rec.decision); err != nil {
			return err
		}
		if err := s.accumulator.storeDiagnostic(r, rec.decision, rec.location); err != nil {
			return err
		}
		if err := s.cacheDecisionAt(r, rec.node, rec.decision); err != nil {
			return err
		}
	}
	if windowPerformanceObserver != nil {
		windowPerformanceObserver("", 0, g.decisions)
	}
	return nil
}

func (s *Session) stopWindowPerformance(reason string) {
	if k := s.performance; k != nil && k.provider != nil && k.provider.failed == "" {
		k.provider.failed = reason
	}
}

func (s *Session) dropWindowPerformance(reason string) {
	k := s.performance
	if k == nil {
		return
	}
	results, decisions := 0, 0
	if p := k.provider; p != nil {
		if p.swing.history.index != nil {
			stateDiscardIndex(s, p.swing.history.index)
			p.swing.history.index = nil
		}
		results = len(p.records)
		for _, record := range p.records {
			s.releaseFactBytes(record.charged)
		}
		p.records = nil
		p.pendingFacts = 0
		p.stateFacts = 0
		s.releaseFactBytes(p.charged)
		p.charged = 0
		p.anchors = nil
		p.ranges = nil
		p.partStart, p.anchorStart = 0, 0
	}
	for i := range k.bends {
		decisions += k.bends[i].group.decisions
		s.releaseConsumerGroup(k.bends[i].group)
		k.bends[i].group = consumerGroup{}
	}
	if p := k.provider; p != nil && p.failed != "" {
		reason = p.failed
		if p.failedReported {
			return
		}
		p.failedReported = true
	}
	if windowPerformanceObserver != nil && (reason != "unsettled" || results != 0 || decisions != 0) {
		windowPerformanceObserver(reason, results, decisions)
	}
}

var windowPerformanceOff bool

func setWindowPerformanceOff(off bool) bool {
	previous := windowPerformanceOff
	windowPerformanceOff = off
	return previous
}

var windowPerformanceObserver func(reason string, results, decisions int)

func setWindowPerformanceObserver(observer func(string, int, int)) func(string, int, int) {
	previous := windowPerformanceObserver
	windowPerformanceObserver = observer
	return previous
}
