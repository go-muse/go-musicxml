package validation

import (
	"context"
	"sort"
	"unsafe"
)

// windowSpans is the spans provider's provisional work of one session. Each
// closed window gives its endpoints, gaps and local facts; Finish reads the
// nodes outside windows and associates everything, without reading a
// window's nodes again. Nothing is visible in the report until Finish adopts
// it, and one failed assumption drops it whole.
type windowSpans struct {
	key      RuleKey
	plan     *contextPlan
	timeline RuleKey
	project  bool

	reader    *contextReader
	collector *spanCollector
	// scratch holds the lyric identities of the window being read; kept the
	// part-wide gaps of unnamed lyrics.
	scratch, kept spanScratch

	// current is the window being read and visiting the place, in document
	// order, of the node being read; final is set while Finish reads the
	// nodes outside windows.
	current  evaluationWindow
	visiting int
	final    bool
	// index is the current window's part by node, built when first asked;
	// outer is that of the nodes outside windows, built in Finish.
	index        map[NodeID]Evidence[NodeRef]
	indexScratch spanScratch
	outer        map[NodeID]Evidence[NodeRef]

	// gapLog keeps the gaps of unknown or shared domains in document order:
	// the first cause of a rank is the one an endpoint reports.
	gapLog   []spanWindowGap
	deferred []spanDeferredRole
	ranges   []spanWindowRange
	records  []spanWindowRecord
	skeleton int

	pendingFacts int64
	// recordBytes is the charge of the stashed results and their list;
	// capturedBytes that of what endpoints captured of their nodes.
	recordBytes   int64
	capturedBytes int64

	// Counting windows (no spans projection) keep no endpoints: counted is
	// how many they read, versionFacts the facts published only if an
	// extend without a type starts a span, and tied the association of each
	// tied identity so far, with the domains an unidentified tied endpoint
	// withholds.
	counted      int
	versionFacts int64
	tied         map[spanAssociationKey]*spanTiedRun
	tiedPoisoned map[NodeRef]bool
	tiedAll      bool
	tiedBytes    int64
	// retained is what the provisional state holds of the session's charge,
	// record payloads included; a drop releases it.
	retained int64
	covered  int

	failed  string
	adopted bool
}

// spanCaptured is what the passes of Finish would read from an endpoint's
// node and its note.
type spanCaptured struct {
	origins []Location
	// fan is a beam's fan with its default; present is whether a continued
	// beam states one, or whether a tie's note states the attack or release
	// its role concerns.
	fan     Evidence[Atomic]
	present Evidence[bool]
	tremolo [2]Evidence[Atomic]
}

// spanWindowGap is one incomplete node's gap for a domain.
type spanWindowGap struct {
	order  int
	domain NodeRef
	value  Evidence[bool]
}

// spanDeferredRole is an extend whose role waits for the root's version,
// with the local gaps its window had for it, in the order they are met.
type spanDeferredRole struct {
	e    *spanEndpoint
	gaps [4]Evidence[bool]
	has  [4]bool
}

// spanWindowRange is a run of adjacent windows under one parent.
type spanWindowRange struct {
	first, end NodeID
	parent     NodeID
}

type spanWindowRecord struct {
	result  ContextResult
	charged int64
	order   int
}

var (
	spanCapturedBytes      = allocationSlack(int64(unsafe.Sizeof(spanCaptured{})))
	spanWindowGapBytes     = int64(unsafe.Sizeof(spanWindowGap{}))
	spanDeferredRoleBytes  = int64(unsafe.Sizeof(spanDeferredRole{}))
	spanWindowRangeBytes   = int64(unsafe.Sizeof(spanWindowRange{}))
	spanWindowRecordBytes  = int64(unsafe.Sizeof(spanWindowRecord{}))
	spanWindowStateBytes   = allocationSlack(int64(unsafe.Sizeof(windowSpans{})+unsafe.Sizeof(spanCollector{})+unsafe.Sizeof(spanLyricIndex{})+unsafe.Sizeof(contextReader{})+unsafe.Sizeof(spanDomains{}))) + 1024
	spanWindowLocationSize = allocationSlack(int64(unsafe.Sizeof(Location{})))
)

func (s *Session) indexWindowSpans(ctx context.Context) error {
	if s.spansIndexed {
		return nil
	}
	s.spansIndexed = true
	var k *windowSpans
	providers := 0
	for _, entry := range s.catalog.phases[s.profile.ID] {
		r := s.catalog.rules[entry.rule]
		plan := s.catalog.plans[r.Predicate]
		if plan.kind != planContextSpans {
			continue
		}
		providers++
		k = &windowSpans{key: entry.rule, plan: plan.context, timeline: plan.context.Timeline, project: s.contextFamilies&ContextFamilySpans != 0}
	}
	// A profile has one spans provider; windows read for no other shape.
	if providers != 1 || k.timeline == (RuleKey{}) {
		return nil
	}
	service, err := s.newContextService(ctx, k.key)
	if err != nil {
		return err
	}
	if err := s.retain(spanWindowStateBytes); err != nil {
		return err
	}
	k.retained = spanWindowStateBytes
	k.scratch = spanScratch{s: s}
	k.indexScratch = spanScratch{s: s}
	k.kept = spanScratch{s: s}
	domains := &spanDomains{window: k}
	k.collector = &spanCollector{key: k.key, service: service, domains: domains, gaps: make(map[NodeRef]Evidence[bool]), localGaps: make(map[NodeRef]Evidence[bool]), window: k}
	k.collector.lyrics = &spanLyricIndex{scratch: &k.scratch, kept: &k.kept, domains: domains}
	k.reader = &contextReader{s: s, b: k.plan.Bindings}
	s.spans = k
	return nil
}

// windowProviderReading reports that a window's context provider is reading:
// what it reads is as final as a bound window's facts are, and what it
// journals in the timeline's speculation belongs to that speculation.
func (s *Session) windowProviderReading() bool {
	return s.performanceStash != nil || s.spansStash != nil
}

// unreadWindowSpans ends the windows' work at a window that closed without
// being read: nothing later windows read could be adopted.
func (s *Session) unreadWindowSpans(ctx context.Context) error {
	if s.strategy != strategyWindowed || s.resourceSnapshot || windowSpansOff {
		return nil
	}
	if err := s.indexWindowSpans(ctx); err != nil {
		return err
	}
	s.dropWindowSpans("unbound")
	return nil
}

// evaluateWindowSpans reads a closed, bound window for the spans provider.
func (s *Session) evaluateWindowSpans(ctx context.Context, w evaluationWindow) error {
	if s.strategy != strategyWindowed || s.resourceSnapshot || windowSpansOff {
		return nil
	}
	if err := s.indexWindowSpans(ctx); err != nil {
		return err
	}
	k := s.spans
	if k == nil || k.failed != "" {
		return nil
	}
	spec := s.speculations[k.timeline]
	if spec == nil || spec.service == nil || spec.stopped != timelineTriggerNone {
		s.dropWindowSpans("speculation")
		return nil
	}
	// An excursion waiting for its unit proof ends in a rebuilt timeline.
	if len(spec.service.unitExcursions) != 0 {
		s.dropWindowSpans("excursions")
		return nil
	}
	s.spansStash = k
	s.speculating = spec
	mark, persistent := k.mark(s), k.persistentBytes()
	err := k.read(ctx, s, spec.service, w)
	if own := k.since(s, mark); k.counting() && err == nil {
		// A counting window leaves its lists and tied runs and nothing else.
		kept := k.persistentBytes() - persistent
		if own > kept {
			s.releaseFactBytes(own - kept)
		}
		k.retained += kept
	} else {
		// Stashed results are the windows' own, unlike what a mark sets
		// apart.
		k.retained += own + k.recordBytes - mark.records
	}
	s.speculating = nil
	s.spansStash = nil
	// The lyric identities and scopes of a note and the window's domain
	// index end with the window.
	k.collector.lyrics.identities, k.collector.lyrics.scopes, k.index = nil, nil, nil
	k.scratch.release()
	k.indexScratch.release()
	if err != nil {
		return err
	}
	if windowSpansTamper != "" {
		k.tamper()
	}
	return nil
}

func (k *windowSpans) read(ctx context.Context, s *Session, service *timelineService, w evaluationWindow) error {
	c, r := k.collector, k.reader
	r.ctx, r.err = ctx, nil
	c.timeline = service
	k.current = w
	start := len(c.endpoints)
	for i, id := range s.sequence[w.first:w.end] {
		k.visiting = w.first + i
		c.visit(r, w.first+i, s.node(id))
		if r.err != nil {
			return r.err
		}
	}
	// A window's endpoints meet the gaps of their own notes, frames and
	// metronomes here: every node that can add to them is in the window.
	for _, e := range c.endpoints[start:] {
		if !r.step(1) {
			return r.err
		}
		if !e.versionRole {
			c.mergeLocalGap(e)
			continue
		}
		if k.counting() {
			continue
		}
		if err := k.deferRole(s, e); err != nil {
			return err
		}
	}
	if k.counting() {
		// Nothing later needs these endpoints: their facts are counted and
		// the tied runs advanced while the window's nodes are at hand.
		k.countAndDiscard(r)
		if r.err != nil {
			return r.err
		}
	}
	if err := k.addRange(s, w); err != nil {
		return err
	}
	k.covered++
	return nil
}

// deferRole keeps the local gaps of an extend whose role waits for Finish.
func (k *windowSpans) deferRole(s *Session, e *spanEndpoint) error {
	c := k.collector
	d := spanDeferredRole{e: e}
	if e.payload.Domain.State == EvidenceKnown {
		d.gaps[0], d.has[0] = c.localGaps[e.payload.Domain.Value]
	}
	if e.payload.Anchor.State == EvidenceKnown {
		d.gaps[1], d.has[1] = c.localGaps[e.payload.Anchor.Value]
		if e.payload.Family == SpanLyricExtend {
			d.gaps[2], d.has[2] = c.lyrics.scopes[spanLyricScopeKey{note: e.payload.Anchor.Value, identity: e.lyric}]
			d.gaps[3], d.has[3] = c.lyrics.scopes[spanLyricScopeKey{note: e.payload.Anchor.Value}]
		}
	}
	if len(k.deferred) == cap(k.deferred) {
		grown := max(8, 2*cap(k.deferred))
		if err := s.retain(int64(grown) * spanDeferredRoleBytes); err != nil {
			return err
		}
		next := make([]spanDeferredRole, len(k.deferred), grown)
		copy(next, k.deferred)
		s.releaseFactBytes(int64(cap(k.deferred)) * spanDeferredRoleBytes)
		k.deferred = next
	}
	k.deferred = append(k.deferred, d)
	return nil
}

// addRange notes the nodes a window covered; adjacent windows of one parent
// make one range.
func (k *windowSpans) addRange(s *Session, w evaluationWindow) error {
	first, end := s.sequence[w.first], s.sequence[w.end-1]+1
	parent := s.node(w.root).header.Parent
	// Windows with no node between them have one parent.
	if n := len(k.ranges); n != 0 && k.ranges[n-1].end == first {
		k.ranges[n-1].end = end
		return nil
	}
	if len(k.ranges) == cap(k.ranges) {
		grown := max(8, 2*cap(k.ranges))
		if err := s.retain(int64(grown) * spanWindowRangeBytes); err != nil {
			return err
		}
		next := make([]spanWindowRange, len(k.ranges), grown)
		copy(next, k.ranges)
		s.releaseFactBytes(int64(cap(k.ranges)) * spanWindowRangeBytes)
		k.ranges = next
	}
	k.ranges = append(k.ranges, spanWindowRange{first: first, end: end, parent: parent})
	return nil
}

// windowRange finds the range of windows a node lies in.
func (k *windowSpans) windowRange(id NodeID) (spanWindowRange, bool) {
	at := sort.Search(len(k.ranges), func(i int) bool { return k.ranges[i].end > id })
	if at < len(k.ranges) && k.ranges[at].first <= id {
		return k.ranges[at], true
	}
	return spanWindowRange{}, false
}

// sealedRead reports a read, in Finish, of a node some window covered.
func (k *windowSpans) sealedRead(s *Session, id NodeID) {
	if _, in := k.windowRange(id); in && s.spansContract == nil {
		s.spansContract = contract("window spans", "Finish read a node of a closed window")
	}
}

// windowGap notes an incomplete node's gap for its domain.
func (c *spanCollector) windowGap(r *contextReader, domain NodeRef, value Evidence[bool]) {
	k := c.window
	if len(k.gapLog) == cap(k.gapLog) {
		grown := max(8, 2*cap(k.gapLog))
		if r.err = r.s.retain(int64(grown) * spanWindowGapBytes); r.err != nil {
			return
		}
		next := make([]spanWindowGap, len(k.gapLog), grown)
		copy(next, k.gapLog)
		r.s.releaseFactBytes(int64(cap(k.gapLog)) * spanWindowGapBytes)
		k.gapLog = next
	}
	k.gapLog = append(k.gapLog, spanWindowGap{order: k.visiting, domain: domain, value: value})
}

// windowOrdered marks state the collector carries from node to node in
// document order: the last metronome, the lyrics of a note. Windows are read
// in that order and the nodes outside them afterwards; the schema declares
// no such node outside a measure, so one found there is a defect.
func (c *spanCollector) windowOrdered(s *Session) {
	if k := c.window; k != nil && k.final && s.spansContract == nil {
		s.spansContract = contract("window spans", "a node outside windows carries state in document order")
	}
}

// inheritedDomain answers from the nodes of the window being read, or, in
// Finish, from the nodes outside windows. A window's question that leaves
// the window depends on facts later windows add: the provider stops.
func (k *windowSpans) inheritedDomain(r *contextReader, n *factNode) Evidence[NodeRef] {
	s := r.s
	if k.final {
		if k.outer == nil {
			k.outer = k.outerDomains(r)
			if r.err != nil {
				return stateUnknown[NodeRef]("context-interrupted")
			}
		}
		return spanInheritedDomain(r, n, k.outer)
	}
	if k.index == nil {
		k.index = k.windowDomains(r)
		if r.err != nil {
			return stateUnknown[NodeRef]("context-interrupted")
		}
	}
	for n != nil {
		if !r.step(1) {
			return stateUnknown[NodeRef]("context-interrupted")
		}
		if d, ok := k.index[n.header.ID]; ok {
			return d
		}
		n = s.node(n.header.Parent)
	}
	if k.failed == "" {
		k.failed = "domain"
	}
	return stateUnknown[NodeRef]("span-part-unavailable")
}

// spanFoldDomain adds one fact's part to a node's domain as the buffered
// index does; it reports whether the node's ancestors may still change.
func spanFoldDomain(index map[NodeID]Evidence[NodeRef], id NodeID, part Evidence[NodeRef]) (changed, inserted bool) {
	old, ok := index[id]
	if !ok {
		index[id] = part
		return true, true
	}
	if old.State != EvidenceKnown || old.Value == part.Value {
		return false, false
	}
	index[id] = stateUnknown[NodeRef]("span-multiple-part-domain")
	return true, false
}

// windowDomains indexes the current window's nodes by the parts of the
// timeline facts beneath them.
func (k *windowSpans) windowDomains(r *contextReader) map[NodeID]Evidence[NodeRef] {
	s, t := r.s, k.collector.timeline
	if r.err = k.indexScratch.reserve(512); r.err != nil {
		return nil
	}
	out := make(map[NodeID]Evidence[NodeRef])
	for _, id := range s.sequence[k.current.first:k.current.end] {
		if !r.step(1) {
			return nil
		}
		f, ok := t.fact(id)
		if !ok || f.Part.State != EvidenceKnown {
			continue
		}
		for n := s.node(id); n != nil; n = s.node(n.header.Parent) {
			if !r.step(1 + int64(len(f.Part.Value.DocumentID))) {
				return nil
			}
			changed, inserted := spanFoldDomain(out, n.header.ID, f.Part)
			if inserted {
				if r.err = k.indexScratch.reserve(256); r.err != nil {
					return nil
				}
			}
			if !changed || n.header.ID == k.current.root {
				break
			}
		}
	}
	return out
}

// outerDomains indexes the nodes outside windows by the parts of all facts
// beneath them, without reading a window's nodes: a fact in a window reaches
// the window's parent directly.
func (k *windowSpans) outerDomains(r *contextReader) map[NodeID]Evidence[NodeRef] {
	s, t := r.s, k.collector.timeline
	if r.err = s.retain(512); r.err != nil {
		return nil
	}
	out := make(map[NodeID]Evidence[NodeRef])
	if t == nil {
		return out
	}
	interrupted := false
	t.forEachFact(func(id NodeID, f timelineEventFact) bool {
		if !r.step(1) {
			interrupted = true
			return false
		}
		if f.Part.State != EvidenceKnown {
			return true
		}
		from := id
		if in, ok := k.windowRange(id); ok {
			from = in.parent
		}
		for n := s.node(from); n != nil; n = s.node(n.header.Parent) {
			if !r.step(1 + int64(len(f.Part.Value.DocumentID))) {
				interrupted = true
				return false
			}
			changed, inserted := spanFoldDomain(out, n.header.ID, f.Part)
			if inserted {
				if r.err = s.retain(256); r.err != nil {
					interrupted = true
					return false
				}
			}
			if !changed {
				break
			}
		}
		return true
	})
	if interrupted {
		return nil
	}
	return out
}

// spanFactsMark is the session's charge at a point of a window's read.
type spanFactsMark struct {
	retained, shared, intern, journal, records int64
}

// mark notes the session's charge.
func (k *windowSpans) mark(s *Session) spanFactsMark {
	m := spanFactsMark{retained: s.retained, shared: s.contextSharedTableBytes, intern: s.internRetained, records: k.recordBytes}
	if s.speculating != nil {
		m.journal = s.speculating.retained
	}
	return m
}

// since is what the windows' read charged since the mark for storage that
// is neither the session's to keep (shared attribute tables, interned
// strings, the speculation's journal) nor a stashed result.
func (k *windowSpans) since(s *Session, mark spanFactsMark) int64 {
	now := k.mark(s)
	return now.retained - mark.retained - (now.shared - mark.shared) - (now.intern - mark.intern) - (now.journal - mark.journal) - (now.records - mark.records)
}

// windowFacts marks the charge before a window publishes the facts of a
// node. The values read for them are kept by nothing but the results the
// stash holds, which carry their own charge.
func (c *spanCollector) windowFacts(r *contextReader) spanFactsMark {
	k := c.window
	if k == nil || k.final || r.err != nil {
		return spanFactsMark{}
	}
	return k.mark(r.s)
}

// windowFactsDone releases what publishing since the mark charged for values
// nothing keeps.
func (c *spanCollector) windowFactsDone(r *contextReader, mark spanFactsMark) {
	k := c.window
	if k == nil || k.final || r.err != nil {
		return
	}
	if unkept := k.since(r.s, mark); unkept > 0 {
		r.s.releaseFactBytes(unkept)
	}
}

// listBytes is the charge of the lists that carry the windows' work to
// Finish.
func (k *windowSpans) listBytes() int64 {
	return int64(cap(k.records))*spanWindowRecordBytes + int64(cap(k.deferred))*spanDeferredRoleBytes + int64(cap(k.gapLog))*spanWindowGapBytes + int64(cap(k.ranges))*spanWindowRangeBytes
}

// stashSpansResult keeps a result a window or the nodes outside windows
// published, at the place of the node being read.
func (s *Session) stashSpansResult(result ContextResult, charged int64) error {
	k := s.spansStash
	if len(k.records) == cap(k.records) {
		grown := max(8, 2*cap(k.records))
		if err := s.retain(int64(grown) * spanWindowRecordBytes); err != nil {
			return err
		}
		next := make([]spanWindowRecord, len(k.records), grown)
		copy(next, k.records)
		s.releaseFactBytes(int64(cap(k.records)) * spanWindowRecordBytes)
		k.recordBytes += int64(grown-cap(k.records)) * spanWindowRecordBytes
		k.records = next
	}
	k.recordBytes += charged
	k.records = append(k.records, spanWindowRecord{result: result, charged: charged, order: k.visiting})
	return nil
}

// confirmWindowSpans names the assumption of the windows' work that failed,
// or nothing. A rejected speculation has dropped the work already.
func (s *Session) confirmWindowSpans(k *windowSpans) string {
	// A read that could not answer from its window reads no later one.
	if k.failed != "" {
		return k.failed
	}
	// Every window that closed was read.
	if k.covered != s.windowsClosed {
		return "unbound"
	}
	// A window's nodes are told from the others by identity: a range of
	// identities is a range of the document only if identities follow
	// document order, as the adapters give them.
	for i := 1; i < len(s.sequence); i++ {
		if s.sequence[i] <= s.sequence[i-1] {
			return "unordered"
		}
	}
	// A timeline rebuilt after its windows were read may place events anew.
	if service := s.timeline[k.timeline]; service == nil || service.unitProofFinal {
		return "rebuilt"
	}
	return ""
}

// adoptWindowSpans gives the provider the collector its windows filled, if
// everything they assumed held: it reads the nodes outside windows and moves
// the results windows published into the report, in document order. It
// returns nothing when there is no such work or an assumption failed; the
// provider then reads the whole document.
func (s *Session) adoptWindowSpans(ctx context.Context, key RuleKey, p *contextPlan) (*contextReader, *spanCollector, error) {
	k := s.spans
	// Nothing to adopt: the work never started. Dropped work names its
	// reason again below.
	if k == nil || k.key != key || k.plan != p || k.adopted {
		return nil, nil, nil
	}
	if reason := s.confirmWindowSpans(k); reason != "" {
		s.dropWindowSpans(reason)
		return nil, nil, nil
	}
	c, r := k.collector, k.reader
	r.ctx, r.err = ctx, nil
	c.timeline = s.timeline[k.timeline]
	k.final = true
	r.sealed = k
	k.skeleton = len(c.endpoints)
	s.spansStash = k
	at := 0
	for order, id := range s.sequence {
		for at < len(k.ranges) && id >= k.ranges[at].end {
			at++
		}
		if at < len(k.ranges) && id >= k.ranges[at].first {
			continue
		}
		k.visiting = order
		c.visit(r, order, s.node(id))
		if r.err != nil {
			break
		}
	}
	if k.counting() && r.err == nil {
		k.countOutside(r)
	}
	s.spansStash = nil
	if r.err != nil {
		return nil, nil, r.err
	}
	if err := s.account.Charge(BudgetContextFacts, k.pendingFacts); err != nil {
		return nil, nil, err
	}
	k.pendingFacts = 0
	if len(k.records) != 0 {
		if err := s.step(ctx, int64(len(k.records))); err != nil {
			return nil, nil, err
		}
		sort.SliceStable(k.records, func(i, j int) bool { return k.records[i].order < k.records[j].order })
	}
	for i, record := range k.records {
		before := s.retained
		if len(s.contextResults) == cap(s.contextResults) {
			if err := s.growContextResults(); err != nil {
				return nil, nil, err
			}
		}
		s.contextResults = append(s.contextResults, record.result)
		s.contextReportRetained += record.charged + s.retained - before
		k.records[i] = spanWindowRecord{}
	}
	return r, c, nil
}

// finishWindowSpans puts the endpoints read outside windows among those the
// windows read, folds the gaps in document order and gives the waiting
// extends their roles. It returns the endpoints that have not met their
// local gaps yet.
func (s *Session) finishWindowSpans(ctx context.Context, r *contextReader, c *spanCollector) ([]*spanEndpoint, error) {
	k := c.window
	outside := c.endpoints[k.skeleton:]
	if len(outside) != 0 && k.skeleton != 0 {
		if err := s.step(ctx, int64(len(c.endpoints))); err != nil {
			return nil, err
		}
		if err := s.retain(int64(len(c.endpoints)) * 8); err != nil {
			return nil, err
		}
		merged := make([]*spanEndpoint, 0, len(c.endpoints))
		in := c.endpoints[:k.skeleton]
		i, j := 0, 0
		for i < len(in) || j < len(outside) {
			if j == len(outside) || i < len(in) && in[i].order < outside[j].order {
				merged = append(merged, in[i])
				i++
			} else {
				merged = append(merged, outside[j])
				j++
			}
		}
		c.endpoints = merged
	}
	if len(k.gapLog) != 0 {
		if err := s.step(ctx, int64(len(k.gapLog))); err != nil {
			return nil, err
		}
		sort.SliceStable(k.gapLog, func(i, j int) bool { return k.gapLog[i].order < k.gapLog[j].order })
		for _, gap := range k.gapLog {
			if old, ok := c.gaps[gap.domain]; ok {
				c.gaps[gap.domain] = timelineMergeAvailability(old, gap.value)
				continue
			}
			if err := s.retain(256); err != nil {
				return nil, err
			}
			c.gaps[gap.domain] = gap.value
		}
	}
	for _, d := range k.deferred {
		if !r.step(1) {
			return nil, r.err
		}
		spanExtendRole(r, d.e)
		if r.err != nil {
			return nil, r.err
		}
		spanRoleAvailability(d.e, spanFamilyBinding{family: d.e.payload.Family})
		for i, gap := range d.gaps {
			if d.has[i] {
				d.e.complete = timelineMergeAvailability(d.e.complete, gap)
			}
		}
	}
	return outside, nil
}

// windowSpansAdopted ends the provisional state: the endpoints are the
// provider's now, and stay charged with it.
func (s *Session) windowSpansAdopted(c *spanCollector) {
	k := c.window
	if k == nil {
		return
	}
	k.kept.release()
	if k.retained < 0 && s.spansContract == nil {
		s.spansContract = contract("window spans", "released more than it retained")
	}
	// The lists that carried the windows' work to Finish, the tied runs and
	// what the endpoints captured for its passes end here; the endpoints
	// stay charged with the provider, as the buffered provider's do.
	for _, e := range c.endpoints {
		e.captured = nil
	}
	s.releaseFactBytes(k.persistentBytes() + k.capturedBytes)
	k.adopted = true
	k.retained = 0
	k.records, k.deferred, k.gapLog, k.ranges, k.outer = nil, nil, nil, nil, nil
	k.tied, k.tiedPoisoned, k.tiedBytes = nil, nil, 0
	k.collector, k.reader = nil, nil
	if windowSpansObserver != nil {
		windowSpansObserver("", k.counted+len(c.endpoints))
	}
}

// dropWindowSpans forgets what windows read and releases its charge.
func (s *Session) dropWindowSpans(reason string) {
	k := s.spans
	if k == nil || k.adopted || k.collector == nil {
		return
	}
	endpoints := k.counted
	if c := k.collector; c != nil {
		endpoints += len(c.endpoints)
	}
	k.kept.release()
	if k.retained > 0 {
		s.releaseFactBytes(k.retained)
	} else if k.retained < 0 && s.spansContract == nil {
		s.spansContract = contract("window spans", "released more than it retained")
	}
	k.retained = 0
	k.collector, k.reader = nil, nil
	k.records, k.deferred, k.gapLog, k.ranges, k.index, k.outer = nil, nil, nil, nil, nil, nil
	k.tied, k.tiedPoisoned, k.tiedBytes = nil, nil, 0
	k.pendingFacts, k.versionFacts = 0, 0
	if k.failed != "" {
		reason = k.failed
	}
	k.failed = reason
	if windowSpansObserver != nil {
		windowSpansObserver(reason, endpoints)
	}
}

// origins are the endpoint's node origins: kept when a window read it.
func (e *spanEndpoint) origins(s *Session) []Location {
	if e.captured != nil {
		return e.captured.origins
	}
	return s.nodeOrigins(s.node(e.ref.Node))
}

// publish publishes a fact about the endpoint's node.
func (e *spanEndpoint) publish(r *contextReader, key RuleKey, n *factNode, property ContextProperty, v Evidence[Atomic]) {
	if e.captured == nil {
		spanPublish(r, key, n, property, v)
		return
	}
	if r.err != nil {
		return
	}
	r.err = r.s.publishContextResult(r.ctx, ContextResult{Provider: key, Subject: e.ref, Property: property, Value: v, Origins: e.captured.origins})
}

// tamper is a test's way to break one assumption after a window was read.
func (k *windowSpans) tamper() {
	switch windowSpansTamper {
	case "covered":
		k.covered++
	case "retained":
		k.retained -= 1 << 40
	case "captured":
		for _, e := range k.collector.endpoints {
			e.captured = nil
		}
	}
}

var windowSpansOff bool

func setWindowSpansOff(off bool) bool {
	previous := windowSpansOff
	windowSpansOff = off
	return previous
}

var windowSpansTamper string

func setWindowSpansTamper(tamper string) string {
	previous := windowSpansTamper
	windowSpansTamper = tamper
	return previous
}

// windowSpansObserver sees how the windows' work ended: with no reason when
// Finish adopted it, otherwise with the assumption that failed.
var windowSpansObserver func(reason string, endpoints int)

func setWindowSpansObserver(observer func(string, int)) func(string, int) {
	previous := windowSpansObserver
	windowSpansObserver = observer
	return previous
}
