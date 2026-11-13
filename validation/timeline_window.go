package validation

import (
	"context"
	"reflect"
	"strings"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

// Windowed evaluation reads each measure of a partwise score into its
// timeline when the measure's window closes, instead of reading every
// measure in Finish. The parts, their closures, part identities and
// instrument references are facts of the whole document; while a window
// closes they are only assumptions:
//
//   - the root named score-partwise is a partwise score root;
//   - the score-parts of the part-list, in order, are the parts the score's
//     blocks name, each read for its instrument definitions before any measure;
//   - a part element names the score-part whose ID its id attribute spells,
//     and its measures are its only children, each declared;
//   - every part's closure is complete;
//   - no block lacks a part and no part repeats a measure ordinal;
//   - an identity reference resolves to the sole owner known so far;
//   - a schema check with no recorded failure when read records none later,
//     and an effective schema value read as unavailable or invalid reads the
//     same after the document is bound.
//
// Finish checks every assumption against the final document. If all hold,
// the measures read so far are exactly what reading them in Finish would
// give, and Finish reads the rest. If one fails, the speculative timeline is
// dropped and Finish reads the whole document; the failed assumption counts
// as a degradation by its trigger.
type timelineSpeculation struct {
	key     RuleKey
	plan    *timelinePlan
	service *timelineService
	blocks  []timelineMeasureBlock
	// parts are the score-parts read for their definitions, in order;
	// scoreParts maps a score-part ID to its score-part.
	parts      []NodeRef
	scoreParts map[string]NodeRef
	// partState follows each part element: its score-part and its next
	// measure ordinal and child.
	partState map[NodeID]*timelineSpeculativePart
	// identities are the reference values resolved to the sole owner known,
	// and invalidReads the schema checks read with no failure recorded.
	identities   map[string]xsd.SubjectID
	invalidReads []timelineInvalidRead
	scalarReads  []timelineScalarRead
	// retained is what reading measures retained, released when the
	// speculation is dropped.
	retained int64
	started  bool
	stopped  timelineTrigger
}

type timelineSpeculativePart struct {
	part      NodeRef
	ordinal   uint64
	nextChild int
}

// timelineScalarRead is an effective schema value read as unavailable or
// invalid, which binding the document can still give a cause.
type timelineScalarRead struct {
	node      NodeID
	attribute QName
	value     Evidence[Atomic]
}

type timelineInvalidRead struct {
	node  *factNode
	check XSDCheck
}

// timelineTrigger names why windowed evaluation read a timeline in Finish
// instead of when its windows closed.
type timelineTrigger int

const (
	timelineTriggerNone timelineTrigger = iota
	// The score is timewise, or its root is not a known score root.
	timelineTriggerTimewise
	// A window closed unbound: the schema has no window binding.
	timelineTriggerUnbound
	// The part-list, a part element or a part's children are not as assumed.
	timelineTriggerHierarchy
	// A block lacks a part, which leaves a gap in every part. A content
	// error leaves the identity scope incomplete, so that no part element
	// resolves its score-part either.
	timelineTriggerPartlessGap
	// Two part elements name one score-part, so that their measure ordinals
	// repeat. A repeat the speculation did not stop at changes the
	// completeness of a block it read, which the blocks check finds.
	timelineTriggerDuplicateDomain
	// The final blocks or parts differ from those read.
	timelineTriggerBlocks
	// A part's closure is incomplete: the input or a hierarchy is incomplete.
	timelineTriggerIncompleteInput
	// An identity reference resolved otherwise at the end of the document.
	timelineTriggerIdentity
	// A schema check read with no failure recorded one later.
	timelineTriggerLateFailure
	timelineTriggerCount
)

const timelineSpeculationBytes int64 = 1024

// speculativeTimelineKeys are the timeline rules of the profile.
func (s *Session) speculativeTimelineKeys() []RuleKey {
	var keys []RuleKey
	for _, phase := range s.catalog.phases[s.profile.ID] {
		if phase.registration {
			continue
		}
		if s.catalog.plans[s.catalog.rules[phase.rule].Predicate].kind == planTimeline {
			keys = append(keys, phase.rule)
		}
	}
	return keys
}

// readTimelineWindow reads the measure window w into the speculative
// timeline of every timeline rule of the profile.
func (s *Session) readTimelineWindow(ctx context.Context, w evaluationWindow, bound bool) error {
	if s.strategy != strategyWindowed || s.catalog == nil || timelineSpeculationTamper == "off" {
		return nil
	}
	if s.speculations == nil {
		keys := s.speculativeTimelineKeys()
		if len(keys) == 0 {
			return nil
		}
		if err := s.retain(int64(len(keys)) * timelineSpeculationBytes); err != nil {
			return err
		}
		s.speculations = make(map[RuleKey]*timelineSpeculation, len(keys))
		for _, key := range keys {
			plan := s.catalog.plans[s.catalog.rules[key].Predicate].timeline
			s.speculations[key] = &timelineSpeculation{key: key, plan: plan}
		}
	}
	for _, key := range s.speculativeTimelineKeys() {
		spec := s.speculations[key]
		if spec == nil || spec.stopped != timelineTriggerNone {
			continue
		}
		if !bound {
			spec.stop(s, timelineTriggerUnbound)
			continue
		}
		if err := spec.read(ctx, s, s.node(w.root)); err != nil {
			return err
		}
	}
	return nil
}

// stop ends the speculation; Finish then reads the whole timeline.
func (spec *timelineSpeculation) stop(s *Session, trigger timelineTrigger) {
	if spec.stopped == timelineTriggerNone {
		spec.stopped = trigger
	}
}

func (spec *timelineSpeculation) read(ctx context.Context, s *Session, measure *factNode) error {
	b := spec.plan.Bindings
	r := &stateReader{ctx: ctx, s: s, b: b}
	if !spec.started {
		spec.started = true
		// The root is bound in Finish; its name is the assumption.
		if s.root.header.Name != (QName{Local: "score-partwise"}) {
			spec.stop(s, timelineTriggerTimewise)
			return nil
		}
		if err := s.step(ctx, 1); err != nil {
			return err
		}
		before := s.retained
		spec.service = &timelineService{Facts: make(map[NodeID]timelineEventFact)}
		s.speculating = spec
		err := spec.readDefinitions(ctx, s, r)
		s.speculating = nil
		if err != nil {
			return err
		}
		spec.retained += s.retained - before
		if spec.stopped != timelineTriggerNone {
			return nil
		}
	}
	h := b.Hierarchies[0]
	part := s.node(measure.header.Parent)
	if part == nil || part.header.Parent != s.root.header.ID {
		spec.stop(s, timelineTriggerHierarchy)
		return nil
	}
	state := spec.partState[part.header.ID]
	if state == nil {
		ref, ok := spec.partRef(part)
		if !ok {
			spec.stop(s, timelineTriggerHierarchy)
			return nil
		}
		if err := s.retain(256); err != nil {
			return err
		}
		if spec.partState == nil {
			spec.partState = make(map[NodeID]*timelineSpeculativePart)
		}
		// Another part element naming the same score-part repeats its
		// measure ordinals.
		for _, other := range spec.partState {
			if other.part == ref {
				spec.stop(s, timelineTriggerDuplicateDomain)
				return nil
			}
		}
		state = &timelineSpeculativePart{part: ref}
		spec.partState[part.header.ID] = state
	}
	// The measure's earlier siblings are exactly the measures read.
	for ; state.nextChild < len(part.children); state.nextChild++ {
		if !r.step(1) {
			return r.err
		}
		child := part.children[state.nextChild]
		if child.header.Declaration.State != EvidenceKnown || child.header.Declaration.Value != h.Measure.Declaration {
			spec.stop(s, timelineTriggerHierarchy)
			return nil
		}
		if child == measure {
			break
		}
	}
	if state.nextChild >= len(part.children) {
		spec.stop(s, timelineTriggerHierarchy)
		return nil
	}
	state.nextChild++
	if err := s.retain(512); err != nil {
		return err
	}
	block := timelineMeasureBlock{content: measure, measure: measure, part: Evidence[NodeRef]{State: EvidenceKnown, Value: state.part}, ordinal: state.ordinal, complete: timelineBlockComplete(measure), coverage: Evidence[bool]{State: EvidenceKnown, Value: true}}
	state.ordinal++
	before := s.retained
	service := spec.service
	if err := service.State.advanceClosure(ctx, s, block.part.Value, block.ordinal); err != nil {
		return err
	}
	s.speculating = spec
	err := service.readMeasure(ctx, s, b, block)
	s.speculating = nil
	if err != nil {
		return err
	}
	if service.packDue() {
		if err := service.packTimelineFacts(ctx, s); err != nil {
			return err
		}
	}
	spec.retained += s.retained - before
	spec.blocks = append(spec.blocks, block)
	return nil
}

// readDefinitions reads the instrument definitions of every score-part of
// the part-list, in order, as Finish reads those of the parts its blocks
// name.
func (spec *timelineSpeculation) readDefinitions(ctx context.Context, s *Session, r *stateReader) error {
	spec.scoreParts = make(map[string]NodeRef)
	for _, list := range s.root.children {
		if !r.step(1) {
			return r.err
		}
		if !list.closed {
			continue
		}
		for _, child := range list.children {
			if !r.step(1) {
				return r.err
			}
			if !r.matches(child, timelineRoleScorePart) {
				if r.err != nil {
					return r.err
				}
				continue
			}
			id, err := s.schemaEffectiveScalar(ctx, child.header.ID, QName{Local: "id"})
			if err != nil {
				return err
			}
			if id.State != EvidenceKnown || id.Value.Kind != AtomicString {
				spec.stop(s, timelineTriggerHierarchy)
				return nil
			}
			if _, duplicate := spec.scoreParts[id.Value.Text]; duplicate {
				spec.stop(s, timelineTriggerHierarchy)
				return nil
			}
			if err := s.retain(int64(256 + len(id.Value.Text))); err != nil {
				return err
			}
			ref := NodeRef{DocumentID: s.documentID, Node: child.header.ID}
			spec.scoreParts[id.Value.Text] = ref
			spec.parts = append(spec.parts, ref)
		}
	}
	if len(spec.parts) == 0 {
		spec.stop(s, timelineTriggerHierarchy)
		return nil
	}
	for _, part := range spec.parts {
		if err := spec.service.State.readInstrumentDefinitions(ctx, s, spec.plan.Bindings, s.node(part.Node), part); err != nil {
			return err
		}
	}
	return nil
}

// partRef is the score-part a part element's id attribute spells.
func (spec *timelineSpeculation) partRef(part *factNode) (NodeRef, bool) {
	for _, a := range part.attributes {
		if a.Name.Space != "" || a.Name.Local != "id" {
			continue
		}
		var text string
		switch {
		case a.Scalar.Lexeme != nil && a.Scalar.Lexeme.State == EvidenceKnown:
			text = a.Scalar.Lexeme.Value
		case a.Scalar.Value != nil && a.Scalar.Value.State == EvidenceKnown:
			text = a.Scalar.Value.Value.Text
		default:
			return NodeRef{}, false
		}
		ref, ok := spec.scoreParts[strings.Join(strings.Fields(text), " ")]
		return ref, ok
	}
	return NodeRef{}, false
}

// timelineBlockComplete is the completeness timelineBlocks gives a closed
// measure with a known declaration.
func timelineBlockComplete(content *factNode) Evidence[bool] {
	complete := timelineEnumeration(content)
	if content.end.Order.State != EvidenceKnown {
		complete = stateUnavailable[OrderKind, bool](*content.end.Order)
	} else if content.end.Order.Value != OrderSource && content.end.Order.Value != OrderModelContent {
		complete = stateUnknown[bool]("timeline-event-order-unavailable")
	}
	return complete
}

// advanceClosure closes part through ordinal, complete as assumed.
func (h *timelineState) advanceClosure(ctx context.Context, s *Session, part NodeRef, ordinal uint64) error {
	for i := range h.Closures {
		same, err := stateSameRef(ctx, s, h.Closures[i].Part, part)
		if err != nil {
			return err
		}
		if same {
			h.Closures[i].Through = ordinal
			return nil
		}
	}
	return h.closePart(ctx, s, part, ordinal, Evidence[bool]{State: EvidenceKnown, Value: true})
}

// speculativeIdentity resolves a reference that ResolveIdentity leaves
// incomplete before the document ends to the sole owner known so far, and
// records the assumption. Any other incomplete resolution stops the
// speculation.
func (s *Session) speculativeIdentity(value string, result xsd.IdentityResolution) (xsd.IdentityResolution, bool) {
	spec := s.speculating
	if spec == nil || result.Status != xsd.IdentityIncomplete || result.Reason != "identity-scope-incomplete" {
		return result, false
	}
	if result.Owner == 0 || result.SecondOwner != 0 {
		spec.stop(s, timelineTriggerIdentity)
		return result, false
	}
	if err := s.retain(int64(128 + len(value))); err != nil {
		spec.stop(s, timelineTriggerIdentity)
		return result, false
	}
	if spec.identities == nil {
		spec.identities = make(map[string]xsd.SubjectID)
	}
	spec.identities[value] = result.Owner
	result.Status = xsd.IdentityUnique
	result.Reason = ""
	result.ScopeComplete = true
	return result, true
}

// speculativeScalarRead records an effective schema value read as
// unavailable or invalid.
func (s *Session) speculativeScalarRead(node NodeID, attribute QName, v Evidence[Atomic]) {
	spec := s.speculating
	if spec == nil {
		return
	}
	if err := s.retain(int64(192 + len(attribute.Space) + len(attribute.Local))); err != nil {
		spec.stop(s, timelineTriggerLateFailure)
		return
	}
	spec.scalarReads = append(spec.scalarReads, timelineScalarRead{node: node, attribute: attribute, value: v})
}

// speculativeInvalidRead records a schema check read with no failure.
func (s *Session) speculativeInvalidRead(n *factNode, check XSDCheck) {
	spec := s.speculating
	if spec == nil {
		return
	}
	if err := s.retain(64); err != nil {
		spec.stop(s, timelineTriggerLateFailure)
		return
	}
	spec.invalidReads = append(spec.invalidReads, timelineInvalidRead{node: n, check: check})
}

// confirm checks every assumption of the speculation against the final
// blocks, after the session bound the whole document. It returns the
// trigger of the first assumption that failed.
func (spec *timelineSpeculation) confirm(ctx context.Context, s *Session, blocks []timelineMeasureBlock, index timelineScopeIndex, closures []stateClosure) (timelineTrigger, error) {
	if spec.stopped != timelineTriggerNone {
		return spec.stopped, nil
	}
	if !spec.started || spec.service == nil {
		return timelineTriggerHierarchy, nil
	}
	// What measures read is checked first: a content error that leaves a
	// part unresolved shows first in the values its measures read.
	for _, read := range spec.scalarReads {
		v, err := s.schemaEffectiveScalarAt(ctx, read.node, read.attribute)
		if err != nil {
			return 0, err
		}
		if !reflect.DeepEqual(v, read.value) {
			return timelineTriggerLateFailure, nil
		}
	}
	for _, read := range spec.invalidReads {
		v, err := s.schemaInvalidValue(ctx, read.node, read.check)
		if err != nil {
			return 0, err
		}
		if v.Cause != nil {
			return timelineTriggerLateFailure, nil
		}
	}
	for value, owner := range spec.identities {
		result, err := s.schema.run.ResolveIdentity(ctx, value)
		if err != nil {
			return 0, err
		}
		if result.Status != xsd.IdentityUnique || result.Owner != owner {
			return timelineTriggerIdentity, nil
		}
	}
	for _, block := range blocks {
		if err := s.step(ctx, 1); err != nil {
			return 0, err
		}
		if block.part.State != EvidenceKnown {
			return timelineTriggerPartlessGap, nil
		}
	}
	if len(blocks) < len(spec.blocks) || len(index.Parts) != len(spec.parts) {
		return timelineTriggerBlocks, nil
	}
	for i, part := range spec.parts {
		if index.Parts[i] != part {
			return timelineTriggerBlocks, nil
		}
	}
	for i, read := range spec.blocks {
		if err := s.step(ctx, 1); err != nil {
			return 0, err
		}
		final := blocks[i]
		if final.content != read.content || final.measure != read.measure || final.ordinal != read.ordinal || !reflect.DeepEqual(final.part, read.part) || !reflect.DeepEqual(final.complete, read.complete) {
			return timelineTriggerBlocks, nil
		}
	}
	for k, closure := range closures {
		if err := s.step(ctx, 1); err != nil {
			return 0, err
		}
		if k == 0 && timelineSpeculationTamper == "closure" {
			closure.Complete = stateUnknown[bool]("tampered")
		}
		if closure.Complete.State != EvidenceKnown || !closure.Complete.Value {
			return timelineTriggerIncompleteInput, nil
		}
	}
	return timelineTriggerNone, nil
}

// timelineSpeculationTamper is a test switch only. It breaks a recorded
// read of every speculation before Finish confirms it: "scalar" changes the
// first effective value read, "invalid" records a read of the document's
// identity check as having found no failure, "off" reads no timeline as its
// windows close, and "closure" makes the first
// final part closure incomplete, which no document does without first
// leaving its part identities unresolved.
var timelineSpeculationTamper string

func setTimelineSpeculationTamper(kind string) string {
	previous := timelineSpeculationTamper
	timelineSpeculationTamper = kind
	return previous
}

func (spec *timelineSpeculation) tamper(s *Session) {
	switch timelineSpeculationTamper {
	case "scalar":
		if len(spec.scalarReads) > 0 {
			spec.scalarReads[0].value.Reason = "tampered"
		}
	case "invalid":
		spec.invalidReads = append(spec.invalidReads, timelineInvalidRead{node: s.root, check: XSDCheckIdentity})
	}
}

// timelineDegradationObserver is a test observation point only: it receives
// the trigger of each timeline windowed evaluation read in Finish, or
// timelineTriggerNone for one read as its windows closed, with the number of
// measures read then.
var timelineDegradationObserver func(trigger, measures int)

func setTimelineDegradationObserver(observer func(trigger, measures int)) func(int, int) {
	previous := timelineDegradationObserver
	timelineDegradationObserver = observer
	return previous
}
