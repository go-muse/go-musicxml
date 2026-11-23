package validation

import "unsafe"

// When the spans projection is not selected, the provider publishes nothing
// a report keeps: all that reaches the caller is the number of context facts
// charged. Every fact of an endpoint but one is decided by the endpoint's
// own window, so a window counts its endpoints' facts when it closes and
// keeps no endpoint. The exception is the recommendation a tied start
// publishes when it is paired with a stop on another note: pairing runs
// through the whole document, so windows carry one small state per tied
// identity and Finish adds the pairs that stand.

// spanTiedRun is the association of one tied identity as the windows have
// read it so far: associateSpanEndpoints' scan of the run, resumable.
type spanTiedRun struct {
	domain NodeRef
	// pending is a boundary waiting for its counterpart.
	pending  bool
	role     int
	anchor   Evidence[NodeRef]
	position Evidence[SpanPosition]
	// ambiguous ends the scan, as it does in the buffered pass; dead means
	// no pair of the run stands: an incomplete endpoint, or a start and a
	// stop on one note.
	ambiguous, dead bool
	// pairs counts the pairs on two known notes.
	pairs int64
}

// spanTiedStart is a tied start a window has seen on a note.
type spanTiedStart struct {
	run    *spanTiedRun
	anchor NodeRef
}

var spanTiedRunBytes = allocationSlack(int64(unsafe.Sizeof(spanTiedRun{}))) + 512

// counting reports that windows count facts and keep no endpoints.
func (k *windowSpans) counting() bool { return !k.project }

// countEndpoints counts the facts of endpoints that share no note with any
// other window: the facts about relations within a note and each endpoint's
// own result. It also advances the tied runs.
func (k *windowSpans) countEndpoints(r *contextReader, es []*spanEndpoint) {
	if len(es) == 0 || r.err != nil {
		return
	}
	k.counted += len(es)
	// An extend without a type starts a span in a document of an old
	// version and has no known role otherwise; the root's version is read
	// in Finish. Its role changes which facts about the order on one note
	// are published, so both counts are taken and Finish keeps one.
	typeless := false
	for _, e := range es {
		if e.versionRole {
			typeless = true
			e.payload.Role = stateUnknown[string]("span-extend-type-unspecified")
		}
	}
	before := k.pendingFacts
	k.publishCounted(r, es)
	if r.err != nil {
		return
	}
	if typeless {
		unknown := k.pendingFacts - before
		k.pendingFacts = before
		for _, e := range es {
			if e.versionRole {
				e.payload.Role = Evidence[string]{State: EvidenceKnown, Value: "start"}
			}
		}
		k.publishCounted(r, es)
		if r.err != nil {
			return
		}
		k.versionFacts += k.pendingFacts - before - unknown
		k.pendingFacts = before + unknown
	}
	k.countTied(r, es)
}

// publishCounted publishes the relational facts and the results of es as the
// buffered provider does, with no association established. Nothing is kept:
// the stash counts them.
func (k *windowSpans) publishCounted(r *contextReader, es []*spanEndpoint) {
	c := k.collector
	spanRelationalFacts(r, c.key, es, c.timeline)
	for _, e := range es {
		if !r.step(1) {
			return
		}
		r.err = r.s.publishContextResult(r.ctx, ContextResult{Provider: c.key, Subject: e.ref, Property: ContextPropertySpan, Span: &e.payload})
		if r.err != nil {
			return
		}
	}
}

// countTied advances the tied runs by the endpoints of one window, in
// document order, exactly as associateSpanEndpoints scans a run.
func (k *windowSpans) countTied(r *contextReader, es []*spanEndpoint) {
	s := r.s
	// The tied starts of the window's notes, by run and note: scratch that
	// ends with the window, charged and stepped as the buffered pass's is.
	scratch := spanScratch{s: s}
	defer scratch.release()
	var started map[spanTiedStart]bool
	for _, e := range es {
		if e.payload.Family != SpanTied || spanSelfContained(e) {
			continue
		}
		if !r.step(1) {
			return
		}
		key, ok, err := spanKey(r.ctx, s, e)
		if err != nil {
			r.err = err
			return
		}
		if !ok {
			// An endpoint without an identity competes with every run of
			// its domain, or of the document when it has no domain.
			if e.payload.Domain.State != EvidenceKnown {
				k.tiedAll = true
				continue
			}
			if !k.tiedPoisoned[e.payload.Domain.Value] {
				if r.err = k.keepTied(s, 256+int64(len(e.payload.Domain.Value.DocumentID))); r.err != nil {
					return
				}
				if k.tiedPoisoned == nil {
					k.tiedPoisoned = make(map[NodeRef]bool)
				}
				k.tiedPoisoned[e.payload.Domain.Value] = true
			}
			continue
		}
		run := k.tied[key]
		if run == nil {
			if r.err = k.keepTied(s, spanTiedRunBytes+ownedStringBytes(key.number)+ownedStringBytes(key.pitch)+ownedStringBytes(key.guard)+int64(len(key.domain.DocumentID))); r.err != nil {
				return
			}
			if k.tied == nil {
				k.tied = make(map[spanAssociationKey]*spanTiedRun)
			}
			run = &spanTiedRun{domain: key.domain}
			k.tied[key] = run
		}
		if e.complete.State != EvidenceKnown || !e.complete.Value {
			run.dead = true
		}
		// A start and a later stop on one note, anywhere in the run, leave
		// the whole run without pairs on two notes: both are in one window.
		if e.payload.Anchor.State == EvidenceKnown && e.payload.Role.State == EvidenceKnown {
			at := spanTiedStart{run, e.payload.Anchor.Value}
			if !r.step(1 + int64(len(at.anchor.DocumentID))) {
				return
			}
			switch e.payload.Role.Value {
			case "start":
				if !started[at] {
					if r.err = scratch.reserve(256 + int64(len(at.anchor.DocumentID))); r.err != nil {
						return
					}
					if started == nil {
						if r.err = scratch.reserve(512); r.err != nil {
							return
						}
						started = make(map[spanTiedStart]bool)
					}
					started[at] = true
				}
			case "stop":
				if started[at] {
					run.dead = true
				}
			}
		}
		if run.ambiguous {
			continue
		}
		role := spanBoundaryRole(e)
		if role == 0 {
			run.ambiguous = true
			continue
		}
		if role == 2 {
			if !run.pending {
				run.ambiguous = true
			}
			continue
		}
		if !run.pending {
			run.pending, run.role, run.anchor, run.position = true, role, e.payload.Anchor, e.payload.Position
			continue
		}
		if run.role == role {
			run.ambiguous = true
			continue
		}
		// a is the start and b the stop, whichever came first.
		a, ap, b, bp := run.anchor, run.position, e.payload.Anchor, e.payload.Position
		if role == 1 {
			a, ap, b, bp = b, bp, a, ap
		}
		if a.State == EvidenceKnown && b.State == EvidenceKnown && a.Value == b.Value {
			run.ambiguous = true
			continue
		}
		if c := k.collector; c.timeline != nil && ap.State == EvidenceKnown && bp.State == EvidenceKnown {
			x, y := ap.Value, bp.Value
			order, err := c.timeline.State.positionOrder(r.ctx, s, musicalPosition{Part: x.Part, Measure: x.Measure, MeasureOrdinal: x.MeasureOrdinal, Offset: x.Offset, UnprovedFrame: x.UnprovedFrame}, musicalPosition{Part: y.Part, Measure: y.Measure, MeasureOrdinal: y.MeasureOrdinal, Offset: y.Offset, UnprovedFrame: y.UnprovedFrame})
			if err != nil {
				r.err = err
				return
			}
			if order == stateAfter {
				run.pending = false
				continue
			}
		}
		if a.State == EvidenceKnown && b.State == EvidenceKnown {
			run.pairs++
		}
		run.pending = false
	}
}

// countAndDiscard counts the endpoints of the window the collector read and
// lets them go with their list: a window full of endpoints must not leave
// its array behind, uncharged.
func (k *windowSpans) countAndDiscard(r *contextReader) {
	c := k.collector
	k.countEndpoints(r, c.endpoints)
	c.endpoints = nil
}

// countOutside ends a counting provider's work in Finish: it adds the facts
// only the whole document decides. The few endpoints read outside windows
// go through the provider's passes like any buffered endpoint.
func (k *windowSpans) countOutside(r *contextReader) {
	late := k.tiedPairs()
	if k.versionFacts != 0 {
		// The root is bound now: its version says whether an extend
		// without a type starts a span.
		var probe spanEndpoint
		probe.payload.Role = Evidence[string]{State: EvidenceAbsent}
		spanExtendRole(r, &probe)
		if r.err != nil {
			return
		}
		if probe.payload.Role.State == EvidenceKnown {
			late += k.versionFacts
		}
	}
	var err error
	if k.pendingFacts, err = rationalCheckedAdd(BudgetContextFacts, k.pendingFacts, late); err != nil {
		r.err = err
	}
}

// keepTied charges storage of the tied runs.
func (k *windowSpans) keepTied(s *Session, bytes int64) error {
	if err := s.retain(bytes); err != nil {
		return err
	}
	k.tiedBytes += bytes
	return nil
}

// tiedPairs is the number of pairs that stand at the end of the document:
// those of the runs no incomplete endpoint, same-note pair, unidentified
// endpoint or gap of their domain withholds.
func (k *windowSpans) tiedPairs() int64 {
	if k.tiedAll {
		return 0
	}
	withheld := make(map[NodeRef]bool, len(k.tiedPoisoned)+len(k.gapLog))
	for domain := range k.tiedPoisoned {
		withheld[domain] = true
	}
	// A gap of a domain leaves every endpoint of the domain incomplete, and
	// a gap of no domain every endpoint.
	for _, gap := range k.gapLog {
		if gap.domain == (NodeRef{}) {
			return 0
		}
		withheld[gap.domain] = true
	}
	total := int64(0)
	for _, run := range k.tied {
		if !run.dead && !withheld[run.domain] {
			total += run.pairs
		}
	}
	return total
}

// persistentBytes is the charge of what a counting window leaves behind.
func (k *windowSpans) persistentBytes() int64 {
	return k.listBytes() + k.tiedBytes
}
