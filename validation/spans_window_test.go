package validation

import (
	"context"
	"testing"
)

func spanWindowUnit(t *testing.T) (*contextReader, *windowSpans) {
	t.Helper()
	r := spanUnitReader(t)
	k := &windowSpans{scratch: spanScratch{s: r.s}, kept: spanScratch{s: r.s}, indexScratch: spanScratch{s: r.s}, reader: r}
	k.collector = &spanCollector{key: RuleKey{"spans", 1}, gaps: map[NodeRef]Evidence[bool]{}, localGaps: map[NodeRef]Evidence[bool]{}, domains: &spanDomains{window: k}, window: k}
	t.Cleanup(func() {
		k.scratch.release()
		k.kept.release()
		k.indexScratch.release()
	})
	return r, k
}

// Windows and the nodes outside them are read out of document order, and of
// two gaps of one rank an endpoint reports the first: the gaps are folded in
// document order, whatever order they were noted in.
func TestWindowSpansGapsFoldInDocumentOrder(t *testing.T) {
	r, k := spanWindowUnit(t)
	c := k.collector
	domain := NodeRef{r.s.documentID, 100}
	for order := 20; order > 0; order-- {
		k.visiting = order
		reason := Code("later")
		if order == 1 {
			reason = "first"
		}
		c.windowGap(r, domain, stateUnknown[bool](reason))
		if r.err != nil {
			t.Fatal(r.err)
		}
	}
	k.visiting = 5
	c.windowGap(r, NodeRef{}, Evidence[bool]{State: EvidenceInvalid, Reason: "invalid"})
	k.visiting = 3
	c.windowGap(r, NodeRef{}, stateUnknown[bool]("weaker"))
	if _, err := r.s.finishWindowSpans(context.Background(), r, c); err != nil {
		t.Fatal(err)
	}
	if got := c.gaps[domain]; got.State != EvidenceUnknown || got.Reason != "first" {
		t.Fatalf("gap of the domain: %+v", got)
	}
	if got := c.gaps[NodeRef{}]; got.State != EvidenceInvalid || got.Reason != "invalid" {
		t.Fatalf("gap of the unknown domain: %+v", got)
	}
}

// A window answers a domain question from the facts of its own nodes, as the
// buffered index does for them; a question that leaves the window stops the
// windows' work, because later windows add to the answer.
func TestWindowSpansDomainsOfAWindow(t *testing.T) {
	r, k := spanWindowUnit(t)
	s := r.s
	part := spanUnitNode(r, spanRole_partList, nil)
	first := len(s.sequence)
	measure := spanUnitNode(r, spanRole_direction, part)
	one := spanUnitNode(r, spanRole_note, measure)
	inner := spanUnitNode(r, spanRole_slur, one)
	two := spanUnitNode(r, spanRole_note, measure)
	bare := spanUnitNode(r, spanRole_direction, measure)
	end := len(s.sequence)
	p1, p2 := NodeRef{s.documentID, 501}, NodeRef{s.documentID, 502}
	service := &timelineService{Facts: map[NodeID]timelineEventFact{
		one.header.ID: {Part: Evidence[NodeRef]{State: EvidenceKnown, Value: p1}},
		two.header.ID: {Part: Evidence[NodeRef]{State: EvidenceKnown, Value: p1}},
	}}
	k.collector.timeline = service
	k.current = evaluationWindow{root: measure.header.ID, first: first, end: end}
	if got := k.inheritedDomain(r, inner); got.State != EvidenceKnown || got.Value != p1 {
		t.Fatalf("a node under a placed note: %+v", got)
	}
	if got := k.inheritedDomain(r, bare); got.State != EvidenceKnown || got.Value != p1 || k.failed != "" {
		t.Fatalf("a node whose window has one part: %+v, failed %q", got, k.failed)
	}
	if _, in := k.index[part.header.ID]; in {
		t.Fatal("the window's index reaches above its root")
	}
	service.Facts[two.header.ID] = timelineEventFact{Part: Evidence[NodeRef]{State: EvidenceKnown, Value: p2}}
	k.index = nil
	if got := k.inheritedDomain(r, bare); got.State != EvidenceUnknown || got.Reason != "span-multiple-part-domain" {
		t.Fatalf("a node whose window has two parts: %+v", got)
	}
	if got := k.inheritedDomain(r, inner); got.State != EvidenceKnown || got.Value != p1 {
		t.Fatalf("a node under a placed note of a mixed window: %+v", got)
	}
	delete(service.Facts, one.header.ID)
	delete(service.Facts, two.header.ID)
	k.index = nil
	if got := k.inheritedDomain(r, inner); got.State == EvidenceKnown || k.failed != "domain" {
		t.Fatalf("a question that leaves the window: %+v, failed %q", got, k.failed)
	}
}

// In Finish the nodes outside windows are answered from all facts, without
// reading a window's nodes: a fact in a window reaches the window's parent.
func TestWindowSpansDomainsOutsideWindows(t *testing.T) {
	r, k := spanWindowUnit(t)
	s := r.s
	root := spanUnitNode(r, spanRole_partList, nil)
	left := spanUnitNode(r, spanRole_direction, root)
	leftMeasure := spanUnitNode(r, spanRole_direction, left)
	leftNote := spanUnitNode(r, spanRole_note, leftMeasure)
	right := spanUnitNode(r, spanRole_direction, root)
	rightMeasure := spanUnitNode(r, spanRole_direction, right)
	rightNote := spanUnitNode(r, spanRole_note, rightMeasure)
	outside := spanUnitNode(r, spanRole_direction, root)
	p1, p2 := NodeRef{s.documentID, 501}, NodeRef{s.documentID, 502}
	k.collector.timeline = &timelineService{Facts: map[NodeID]timelineEventFact{
		leftNote.header.ID:  {Part: Evidence[NodeRef]{State: EvidenceKnown, Value: p1}},
		rightNote.header.ID: {Part: Evidence[NodeRef]{State: EvidenceKnown, Value: p2}},
	}}
	k.ranges = []spanWindowRange{
		{first: leftMeasure.header.ID, end: leftNote.header.ID + 1, parent: left.header.ID},
		{first: rightMeasure.header.ID, end: rightNote.header.ID + 1, parent: right.header.ID},
	}
	k.final = true
	r.sealed = k
	if got := k.inheritedDomain(r, left); got.State != EvidenceKnown || got.Value != p1 {
		t.Fatalf("the parent of the first windows: %+v", got)
	}
	if got := k.inheritedDomain(r, right); got.State != EvidenceKnown || got.Value != p2 {
		t.Fatalf("the parent of the second windows: %+v", got)
	}
	if got := k.inheritedDomain(r, outside); got.State != EvidenceUnknown || got.Reason != "span-multiple-part-domain" {
		t.Fatalf("a node under the root of two parts: %+v", got)
	}
	if _, in := k.outer[leftMeasure.header.ID]; in {
		t.Fatal("the index of the nodes outside windows holds a window's node")
	}
	if s.spansContract != nil || k.failed != "" {
		t.Fatalf("contract %v, failed %q", s.spansContract, k.failed)
	}
	if r.matches(leftNote, spanRole_note); s.spansContract == nil {
		t.Fatal("a sealed reader read a window's node unnoticed")
	}
}

// State the collector carries from node to node belongs to windows, which
// are read in document order. The schema declares no metronome or lyric
// outside a measure; one read there, after the windows, is reported as a
// defect instead of being adopted out of order.
func TestWindowSpansOrderedStateOutsideWindows(t *testing.T) {
	for _, role := range []contextRole{spanRole_metronome, spanRole_noteLyric} {
		r, k := spanWindowUnit(t)
		c := k.collector
		c.lyrics = &spanLyricIndex{scratch: &k.scratch, kept: &k.kept, domains: c.domains}
		c.service = &contextService{}
		r.s.spansStash = k
		node := spanUnitNode(r, role, spanUnitNode(r, spanRole_note, nil))
		c.visit(r, 1, node)
		if r.err != nil || r.s.spansContract != nil {
			t.Fatalf("%s in a window: %v, %v", role, r.err, r.s.spansContract)
		}
		k.final = true
		k.outer = map[NodeID]Evidence[NodeRef]{}
		c.visit(r, 1, node)
		if r.err != nil || r.s.spansContract == nil {
			t.Fatalf("%s outside windows: %v, %v", role, r.err, r.s.spansContract)
		}
		r.s.spansStash = nil
	}
}

// Adjacent windows make one range; a node is found in its range and the
// windows' parent in none.
func TestWindowSpansRangesOfAdjacentWindows(t *testing.T) {
	r, k := spanWindowUnit(t)
	s := r.s
	part := spanUnitNode(r, spanRole_partList, nil)
	var roots []*factNode
	for i := 0; i < 2; i++ {
		roots = append(roots, spanUnitNode(r, spanRole_direction, part))
		spanUnitNode(r, spanRole_note, roots[i])
	}
	other := spanUnitNode(r, spanRole_direction, part)
	last := spanUnitNode(r, spanRole_direction, other)
	spanUnitNode(r, spanRole_note, last)
	for i, root := range append(roots, last) {
		first := int(root.header.ID) - 1
		if err := k.addRange(s, evaluationWindow{root: root.header.ID, first: first, end: first + 2}); err != nil {
			t.Fatal(i, err)
		}
	}
	if len(k.ranges) != 2 || k.ranges[0].parent != part.header.ID || k.ranges[1].parent != other.header.ID {
		t.Fatalf("ranges %+v", k.ranges)
	}
	if in, ok := k.windowRange(roots[1].header.ID + 1); !ok || in != k.ranges[0] {
		t.Fatalf("a node of the second window: %+v %v", in, ok)
	}
	for _, outside := range []*factNode{part, other} {
		if _, ok := k.windowRange(outside.header.ID); ok {
			t.Fatalf("node %d outside windows lies in a range", outside.header.ID)
		}
	}
}

// The part-wide gaps of unnamed lyrics outlive the window that found them:
// they are reserved apart from the scratch a window releases.
func TestWindowSpansKeepUnnamedLyricGaps(t *testing.T) {
	r, k := spanWindowUnit(t)
	c := k.collector
	c.lyrics = &spanLyricIndex{scratch: &k.scratch, kept: &k.kept, domains: c.domains}
	note := spanUnitNode(r, spanRole_note, nil)
	k.index = map[NodeID]Evidence[NodeRef]{note.header.ID: {State: EvidenceKnown, Value: NodeRef{r.s.documentID, 500}}}
	c.lyrics.record(r, spanUnitNode(r, spanRole_noteLyric, note))
	c.lyrics.record(r, spanUnitNode(r, spanRole_noteLyric, note))
	if r.err != nil {
		t.Fatal(r.err)
	}
	if len(c.lyrics.unnamed) != 1 || k.kept.bytes == 0 || k.scratch.bytes == 0 {
		t.Fatalf("unnamed gaps %v, kept %d, scratch %d", c.lyrics.unnamed, k.kept.bytes, k.scratch.bytes)
	}
	kept := k.kept.bytes
	k.scratch.release()
	if k.kept.bytes != kept {
		t.Fatal("the window's scratch released the kept gaps")
	}
}

// An extend whose role waits for the root's version meets, in Finish, the
// role's own unavailability first and then the local gaps its window had for
// it, in the order the buffered provider meets them: of two gaps of one rank
// the first is reported.
func TestWindowSpansDeferredExtendMeetsItsGapsInOrder(t *testing.T) {
	gap := func(reason Code) Evidence[bool] { return stateUnknown[bool](reason) }
	for first := 0; first < 4; first++ {
		r, k := spanWindowUnit(t)
		c := k.collector
		c.lyrics = &spanLyricIndex{scratch: &k.scratch, kept: &k.kept, domains: c.domains, scopes: map[spanLyricScopeKey]Evidence[bool]{}}
		root := spanUnitNode(r, spanRole_partList, nil)
		versionUse := r.b.Roles[spanRole_version][0]
		r.b.Attributes[spanRole_version][root.header.Type.Value] = AttributeSelector{Name: versionUse.Name, Declaration: versionUse.Declaration}
		spanUnitAttribute(r, root, spanRole_version, spanString("2.0"))
		note := NodeRef{r.s.documentID, 7}
		frame := NodeRef{r.s.documentID, 8}
		e := &spanEndpoint{ref: NodeRef{r.s.documentID, 9}, lyric: "k1:1a:", versionRole: true, complete: Evidence[bool]{State: EvidenceKnown, Value: true}}
		e.payload.Family = SpanLyricExtend
		e.payload.Role = Evidence[string]{State: EvidenceAbsent}
		e.payload.Domain = Evidence[NodeRef]{State: EvidenceKnown, Value: frame}
		e.payload.Anchor = Evidence[NodeRef]{State: EvidenceKnown, Value: note}
		reasons := []Code{"domain", "anchor", "named", "wildcard"}
		if first <= 0 {
			c.localGaps[frame] = gap(reasons[0])
		}
		if first <= 1 {
			c.localGaps[note] = gap(reasons[1])
		}
		if first <= 2 {
			c.lyrics.scopes[spanLyricScopeKey{note: note, identity: e.lyric}] = gap(reasons[2])
		}
		c.lyrics.scopes[spanLyricScopeKey{note: note}] = gap(reasons[3])
		if err := k.deferRole(r.s, e); err != nil {
			t.Fatal(err)
		}
		// The window's gaps end with it; Finish has only what was kept.
		c.localGaps, c.lyrics.scopes = map[NodeRef]Evidence[bool]{}, nil
		if _, err := r.s.finishWindowSpans(context.Background(), r, c); err != nil {
			t.Fatal(err)
		}
		if e.payload.Role.State != EvidenceKnown || e.payload.Role.Value != "start" {
			t.Fatalf("role in version 2.0: %+v", e.payload.Role)
		}
		if e.complete.State != EvidenceUnknown || e.complete.Reason != reasons[first] {
			t.Fatalf("first gap %q: complete %+v", reasons[first], e.complete)
		}
	}
	// A role the version leaves unknown is met before any local gap.
	r, k := spanWindowUnit(t)
	c := k.collector
	c.lyrics = &spanLyricIndex{scratch: &k.scratch, kept: &k.kept, domains: c.domains}
	root := spanUnitNode(r, spanRole_partList, nil)
	versionUse := r.b.Roles[spanRole_version][0]
	r.b.Attributes[spanRole_version][root.header.Type.Value] = AttributeSelector{Name: versionUse.Name, Declaration: versionUse.Declaration}
	spanUnitAttribute(r, root, spanRole_version, spanString("4.0"))
	note := NodeRef{r.s.documentID, 7}
	e := &spanEndpoint{ref: NodeRef{r.s.documentID, 9}, versionRole: true, complete: Evidence[bool]{State: EvidenceKnown, Value: true}}
	e.payload.Family = SpanFigureExtend
	e.payload.Role = Evidence[string]{State: EvidenceAbsent}
	e.payload.Domain = Evidence[NodeRef]{State: EvidenceInvalid, Reason: "domain-invalid"}
	e.payload.Anchor = Evidence[NodeRef]{State: EvidenceKnown, Value: note}
	c.localGaps[note] = gap("anchor")
	if err := k.deferRole(r.s, e); err != nil {
		t.Fatal(err)
	}
	if _, err := r.s.finishWindowSpans(context.Background(), r, c); err != nil {
		t.Fatal(err)
	}
	if e.complete.Reason != "span-extend-type-unspecified" {
		t.Fatalf("complete %+v", e.complete)
	}
	if e.payload.Association.State != EvidenceInvalid || e.payload.Association.Reason != "domain-invalid" {
		t.Fatalf("association %+v", e.payload.Association)
	}
}

// What a window keeps of an endpoint's node is charged: the record, and the
// origins when the spans projection is selected.
func TestWindowSpansCaptureIsCharged(t *testing.T) {
	for _, project := range []bool{false, true} {
		r, k := spanWindowUnit(t)
		k.project = project
		r.s.contextFamilies = ContextFamilySpans
		note := spanUnitNode(r, spanRole_note, nil)
		beam := spanUnitNode(r, spanRole_beam, note)
		e := &spanEndpoint{ref: NodeRef{r.s.documentID, beam.header.ID}}
		e.payload.Family = SpanBeam
		before := r.s.retained
		k.collector.capture(r, beam, e)
		if r.err != nil {
			t.Fatal(r.err)
		}
		want := spanCapturedBytes
		if project {
			want += spanWindowLocationSize + 96
		}
		if e.captured == nil || (len(e.captured.origins) != 0) != project {
			t.Fatalf("project=%v: captured %+v", project, e.captured)
		}
		if charged := r.s.retained - before; charged < want || k.capturedBytes != charged {
			t.Fatalf("project=%v: charged %d and counted %d, the record alone is %d", project, charged, k.capturedBytes, want)
		}
	}
}

// An incomplete node a window reads goes to the ordered log, not straight
// to the map the buffered provider fills in document order.
func TestWindowSpansNoteGapsOfIncompleteNodes(t *testing.T) {
	r, k := spanWindowUnit(t)
	c := k.collector
	c.lyrics = &spanLyricIndex{scratch: &k.scratch, kept: &k.kept, domains: c.domains}
	c.service = &contextService{}
	part := spanUnitNode(r, spanRole_partList, nil)
	node := spanUnitNode(r, spanRole_direction, part)
	node.end.ChildrenComplete = testEvidence(stateUnknown[bool]("children-unknown"))
	k.final = true
	k.outer = map[NodeID]Evidence[NodeRef]{}
	k.visiting = 4
	c.visit(r, 4, node)
	if r.err != nil {
		t.Fatal(r.err)
	}
	if len(c.gaps) != 0 || len(k.gapLog) != 1 || k.gapLog[0].order != 4 || k.gapLog[0].domain != (NodeRef{r.s.documentID, part.header.ID}) {
		t.Fatalf("gaps %v, log %+v", c.gaps, k.gapLog)
	}
}

// Adoption releases the lists that carried the windows' work to Finish and
// nothing else.
func TestWindowSpansAdoptionReleasesItsLists(t *testing.T) {
	r, k := spanWindowUnit(t)
	s := r.s
	for i := 0; i < 20; i++ {
		k.visiting = i
		k.collector.windowGap(r, NodeRef{}, stateUnknown[bool]("gap"))
		if err := k.deferRole(s, &spanEndpoint{}); err != nil {
			t.Fatal(err)
		}
		s.spansStash = k
		if err := s.stashSpansResult(ContextResult{}, 0); err != nil {
			t.Fatal(err)
		}
		s.spansStash = nil
		k.ranges = append(k.ranges, spanWindowRange{})
	}
	if r.err != nil {
		t.Fatal(r.err)
	}
	lists := int64(cap(k.gapLog))*spanWindowGapBytes + int64(cap(k.deferred))*spanDeferredRoleBytes + int64(cap(k.records))*spanWindowRecordBytes
	if err := s.retain(int64(cap(k.ranges)) * spanWindowRangeBytes); err != nil {
		t.Fatal(err)
	}
	lists += int64(cap(k.ranges)) * spanWindowRangeBytes
	note := spanUnitNode(r, spanRole_note, nil)
	e := &spanEndpoint{ref: NodeRef{s.documentID, note.header.ID}}
	k.collector.capture(r, note, e)
	k.collector.endpoints = []*spanEndpoint{e}
	if r.err != nil || e.captured == nil || k.capturedBytes == 0 {
		t.Fatalf("capture: %v, %+v, %d", r.err, e.captured, k.capturedBytes)
	}
	lists += k.capturedBytes
	if err := k.kept.reserve(64); err != nil {
		t.Fatal(err)
	}
	before := s.retained
	s.spans = k
	s.windowSpansAdopted(k.collector)
	if released := before - s.retained; released != lists || lists == 0 || k.kept.bytes != 0 {
		t.Fatalf("released %d, the lists and captures hold %d; kept gaps still charge %d", released, lists, k.kept.bytes)
	}
	if e.captured != nil {
		t.Fatal("an adopted endpoint keeps what it captured")
	}
	if !k.adopted || k.records != nil || k.deferred != nil || k.gapLog != nil || k.ranges != nil || s.spansContract != nil {
		t.Fatalf("after adoption: %+v, contract %v", k, s.spansContract)
	}
}

// A window's charge counts what the windows' work alone keeps: shared
// attribute tables, interned strings and the speculation's journal are the
// session's, and a stashed result carries its own charge. Publishing
// releases, of what it charged, exactly the rest.
func TestWindowSpansChargeOwnership(t *testing.T) {
	r, k := spanWindowUnit(t)
	s := r.s
	s.spansStash = k
	s.speculating = &timelineSpeculation{}
	defer func() { s.spansStash, s.speculating = nil, nil }()
	all := k.mark(s)
	mark := k.collector.windowFacts(r)
	const unkept, table, interned, payload = 1000, 300, 70, 40
	for _, n := range []int64{unkept, table, interned, payload} {
		if err := s.retain(n); err != nil {
			t.Fatal(err)
		}
	}
	s.contextSharedTableBytes += table
	s.internRetained += interned
	s.speculativeScalarRead(1, QName{Local: "a"}, stateUnknown[Atomic]("unavailable"))
	journal := s.speculating.retained
	if journal == 0 {
		t.Fatal("a window provider's speculative read is not the speculation's")
	}
	if err := s.stashSpansResult(ContextResult{}, payload); err != nil {
		t.Fatal(err)
	}
	list := int64(cap(k.records)) * spanWindowRecordBytes
	if k.recordBytes != payload+list {
		t.Fatalf("stash counts %d, holds %d", k.recordBytes, payload+list)
	}
	before := s.retained
	k.collector.windowFactsDone(r, mark)
	if released := before - s.retained; released != unkept {
		t.Fatalf("publishing released %d of %d unkept bytes", released, unkept)
	}
	if got := k.since(s, all); got != 0 {
		t.Fatalf("%d bytes left to the windows' work after the release", got)
	}
	if s.retained != all.retained+table+interned+journal+payload+list {
		t.Fatalf("session holds %d more, want %d", s.retained-all.retained, table+interned+journal+payload+list)
	}
}

// Finish adopts the windows' work only if every window that closed was read,
// no read failed, and the timeline was not rebuilt since.
func TestWindowSpansConfirmation(t *testing.T) {
	r, k := spanWindowUnit(t)
	s := r.s
	k.timeline = RuleKey{"timeline", 1}
	s.timeline = map[RuleKey]*timelineService{k.timeline: {}}
	k.covered, s.windowsClosed = 3, 3
	if got := s.confirmWindowSpans(k); got != "" {
		t.Fatalf("everything held: %q", got)
	}
	k.failed = "domain"
	if got := s.confirmWindowSpans(k); got != "domain" {
		t.Fatalf("a failed read: %q", got)
	}
	k.failed = ""
	s.windowsClosed = 4
	if got := s.confirmWindowSpans(k); got != "unbound" {
		t.Fatalf("an unread window: %q", got)
	}
	s.windowsClosed = 3
	// Identities out of document order: a range of them is no range of
	// the document.
	s.sequence = []NodeID{1, 2, 7, 3, 8}
	if got := s.confirmWindowSpans(k); got != "unordered" {
		t.Fatalf("identities out of order: %q", got)
	}
	s.sequence = []NodeID{1, 2, 2}
	if got := s.confirmWindowSpans(k); got != "unordered" {
		t.Fatalf("a repeated identity: %q", got)
	}
	s.sequence = []NodeID{1, 2, 7, 9}
	if got := s.confirmWindowSpans(k); got != "" {
		t.Fatalf("increasing identities with holes: %q", got)
	}
	s.timeline[k.timeline].unitProofFinal = true
	if got := s.confirmWindowSpans(k); got != "rebuilt" {
		t.Fatalf("a rebuilt timeline: %q", got)
	}
	delete(s.timeline, k.timeline)
	if got := s.confirmWindowSpans(k); got != "rebuilt" {
		t.Fatalf("no timeline: %q", got)
	}
}

// The windows' work ends once: a second drop releases and reports nothing.
func TestWindowSpansDropOnce(t *testing.T) {
	r, k := spanWindowUnit(t)
	s := r.s
	s.spans = k
	if err := s.retain(500); err != nil {
		t.Fatal(err)
	}
	k.retained = 500
	if err := k.kept.reserve(64); err != nil {
		t.Fatal(err)
	}
	var reasons []string
	defer setWindowSpansObserver(setWindowSpansObserver(func(reason string, _ int) { reasons = append(reasons, reason) }))
	before := s.retained
	s.dropWindowSpans("unbound")
	s.dropWindowSpans("unsettled")
	if before-s.retained != 500 || k.kept.bytes != 0 || len(reasons) != 1 || reasons[0] != "unbound" {
		t.Fatalf("released %d, kept %d, reported %v", before-s.retained, k.kept.bytes, reasons)
	}
}
