package validation

import (
	"context"
	"math"
	"reflect"
)

// Under the windowed strategy the rules that consume a timeline through the
// facts of one measure — the chord duration bound, the key a numeral
// requires and the barline location — are decided when the measure's window
// closes, right after the window's node rules, instead of in the phases of
// Finish. Each is evaluated as its phase evaluates it: final, with the
// finding key of its phase and node, against the timeline read so far as
// windows closed. That timeline is a speculation, and so are the decisions:
//
//   - the timeline's own assumptions hold (timelineSpeculation); what these
//     rules read of the schema besides is final once the window is bound
//     with no failure proved, as for the window's node rules;
//   - Finish keeps the timeline as read: it does not rebuild it to prove
//     excursion units, which republishes every fact;
//   - the timeline rule passes as the document prerequisite of its consumers;
//   - a key presence asked at a numeral's position reads the same from the
//     final timeline state, whose later measures can still withdraw or
//     qualify an earlier answer.
//
// Until Finish checks them, the decisions are kept apart from the session's
// decisions, grouped by the phase that would have made them: nothing but a
// consumer evaluated in a window reads them, so that every other rule sees
// what the buffered engine shows it. When Finish reaches the phase of a
// consuming rule, it adopts the group of that phase, recording each outcome
// and finding with the key the phase gives it, or, if an assumption failed,
// drops every group left and lets the phases decide, as the buffered engine
// does. A session that ends before a phase never records its group.
func consumerRuleKind(kind planKind) bool {
	switch kind {
	case planChordDuration, planNumeralKey, planBarlineLocation:
		return true
	}
	return false
}

// consumerRule is a rule of the profile that windows decide against a
// timeline, with the timeline rule it consumes and its evaluation phase.
type consumerRule struct {
	key      RuleKey
	timeline RuleKey
	phase    int
}

// consumerEvaluation is the timeline the consumers of a window are being
// decided against, as its speculation read it so far, and the group that
// keeps their decisions.
type consumerEvaluation struct {
	timeline    RuleKey
	speculation *timelineSpeculation
	group       *consumerGroup
}

// consumerRecord is a provisional outcome other than pass or not applicable,
// whole, with the location of its node and its finding key.
type consumerRecord struct {
	rule     RuleKey
	node     NodeID
	decision Decision
	location Location
	at       findingKey
}

// consumerQuery is a key presence a numeral's rule asked of the timeline
// state while its window closed.
type consumerQuery struct {
	position musicalPosition
	staff    Evidence[Integer]
	value    Evidence[bool]
}

// consumerGroup is what windows decided provisionally in place of one
// phase: the normative passes and exclusions they counted, each rule's pass
// and not-applicable outcomes by node, the other outcomes whole, and the key
// presences asked. A barline's group also holds the chord durations its
// evaluations decided, as the barline's phase does.
type consumerGroup struct {
	pass, notApplicable uint64
	decided             map[RuleKey]*outcomeBits
	records             []consumerRecord
	index               map[decisionID]int
	queries             []consumerQuery
	decisions           int
	// charged is what the records, their index, the queries and the
	// per-rule tables retained, apart from what each outcome table charges
	// itself.
	charged int64
}

// windowConsumers is the provisional state of a session: the consuming
// rules in phase order, the group of each, the timeline Finish kept as its
// windows read it, and the number of decisions not yet adopted or dropped.
type windowConsumers struct {
	rules      []consumerRule
	groups     []consumerGroup
	evaluating consumerEvaluation
	adopted    RuleKey
	pending    int
}

const (
	// consumerRecordBytes charges one slot of the records' array, at least
	// the record's size, and consumerIndexBytes one entry of their index
	// with its share of the map's growth.
	consumerRecordBytes int64 = 320
	consumerIndexBytes  int64 = 160
	// consumerQueryBytes charges one slot of the queries' array.
	consumerQueryBytes int64 = 256
	// consumerTableBytes charges a rule's outcome table and its map entry.
	consumerTableBytes int64 = 128
	// consumerStateBytes charges the session's windowConsumers and
	// consumerRuleBytes one rule with its group, each at least its size
	// (TestConsumerRecordWidth).
	consumerStateBytes int64 = 256
	consumerRuleBytes  int64 = 192
)

// indexConsumerRules finds, once per session, the profile's rules that
// windows decide against a timeline, in phase order. Deciding in a window
// assumes the timeline rule, which a catalog requires of every consumer, is
// its only prerequisite: a consumer left waiting for another one could not
// be told from an undecided one by the barline that evaluates it. A profile
// with any other consumer decides them all in Finish.
func (s *Session) indexConsumerRules() error {
	var rules []consumerRule
	for phase, entry := range s.catalog.phases[s.profile.ID] {
		r := s.catalog.rules[entry.rule]
		plan := s.catalog.plans[r.Predicate]
		if !consumerRuleKind(plan.kind) {
			continue
		}
		var timeline RuleKey
		if plan.kind == planBarlineLocation {
			timeline = plan.context.Timeline
		} else {
			timeline = plan.timelineConsumer.Provider
		}
		if len(r.Requires) != 1 {
			rules = nil
			break
		}
		rules = append(rules, consumerRule{key: entry.rule, timeline: timeline, phase: phase})
	}
	if err := s.retain(consumerStateBytes + int64(len(rules))*consumerRuleBytes); err != nil {
		return err
	}
	s.consumers = &windowConsumers{rules: rules, groups: make([]consumerGroup, len(rules))}
	return nil
}

// evaluateWindowConsumers decides the timeline consumers of the window's
// nodes, after the window's node rules were evaluated and its measure was
// read into the speculative timeline.
func (s *Session) evaluateWindowConsumers(ctx context.Context, w evaluationWindow) error {
	if windowConsumersOff {
		return nil
	}
	if s.consumers == nil {
		if err := s.indexConsumerRules(); err != nil {
			return err
		}
	}
	k := s.consumers
	// The speculation is not held once the window is decided: Finish may
	// drop it and release what it retained.
	defer func() { s.consumerEvaluating, k.evaluating = nil, consumerEvaluation{} }()
	for i, rule := range k.rules {
		// The speculation just read the window's measure unless it stopped.
		// A timeline with an excursion whose units wait for proof is rebuilt
		// in Finish: nothing is decided against it.
		spec := s.speculations[rule.timeline]
		if spec == nil || spec.stopped != timelineTriggerNone || len(spec.service.unitExcursions) != 0 {
			continue
		}
		k.evaluating = consumerEvaluation{timeline: rule.timeline, speculation: spec, group: &k.groups[i]}
		s.consumerEvaluating = &k.evaluating
		for j, id := range s.sequence[w.first:w.end] {
			s.accumulator.at = findingKey{stage: stagePhases, phase: rule.phase, trigger: int64(w.first + j)}
			if err := s.evaluate(ctx, s.node(id), rule.key, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Session) retainConsumer(g *consumerGroup, n int64) error {
	if err := s.retain(n); err != nil {
		return err
	}
	g.charged += n
	return nil
}

func (s *Session) releaseConsumer(g *consumerGroup, n int64) {
	s.releaseFactBytes(n)
	g.charged -= n
}

// consumerTimeline is the timeline a consuming plan reads: the finalized
// service, or, while a window's consumers are decided, the timeline read so
// far as windows closed.
func (s *Session) consumerTimeline(key RuleKey) *timelineService {
	if c := s.consumerEvaluating; c != nil {
		if c.timeline == key {
			return c.speculation.service
		}
		return nil
	}
	if service := s.timeline[key]; service != nil && service.Final {
		return service
	}
	return nil
}

// provisionalDecision reads a decision windows made provisionally, as the
// dependency cache would hold it.
func (s *Session) provisionalDecision(id decisionID) (Decision, bool) {
	groups := s.consumers.groups
	for i := range groups {
		g := &groups[i]
		if result := g.decided[id.rule].get(id.node); result != 0 {
			outcome := OutcomePass
			if result == successfulNotApplicable {
				outcome = OutcomeNotApplicable
			}
			return Decision{Rule: id.rule, Subject: NodeRef{s.documentID, id.node}, Outcome: outcome}, true
		}
		if at, ok := g.index[id]; ok {
			d := g.records[at].decision
			d.Expected, d.Actual, d.Related = "", "", nil
			return d, true
		}
	}
	return Decision{}, false
}

// stashConsumerDecision keeps a provisional decision apart, in the group of
// the phase being decided.
func (s *Session) stashConsumerDecision(r RuleDefinition, n *factNode, d Decision) error {
	g := s.consumerEvaluating.group
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
		if r.Category == CategoryNormative && r.Class == ClassMandatory && r.Role == RolePredicate {
			if d.Outcome == OutcomePass {
				g.pass++
			} else {
				g.notApplicable++
			}
		}
	} else {
		// Every reader bounds the expected and actual previews; keep them
		// bounded.
		d.Expected = reportBoundedValue(d.Expected, reportValueBytes)
		d.Actual = reportBoundedValue(d.Actual, reportValueBytes)
		location := n.location()
		size := reportStorageSize{limit: math.MaxInt64}
		size.decision(d)
		if err := s.retainConsumer(g, size.total+consumerIndexBytes+int64(len(location.Path))); err != nil {
			return err
		}
		if len(g.records) == cap(g.records) {
			// The new array is charged whole before it is made; the old one
			// is released once replaced.
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
	s.consumers.pending++
	return nil
}

// journalKeyPresence notes a key presence a numeral's rule asked while its
// window closed, to ask it again of the final timeline state.
func (s *Session) journalKeyPresence(position musicalPosition, staff Evidence[Integer], value Evidence[bool]) error {
	c := s.consumerEvaluating
	if c == nil {
		return nil
	}
	g := c.group
	if len(g.queries) == cap(g.queries) {
		grown := max(8, 2*cap(g.queries))
		if err := s.retainConsumer(g, int64(grown)*consumerQueryBytes); err != nil {
			return err
		}
		next := make([]consumerQuery, len(g.queries), grown)
		copy(next, g.queries)
		s.releaseConsumer(g, int64(cap(g.queries))*consumerQueryBytes)
		g.queries = next
	}
	g.queries = append(g.queries, consumerQuery{position: position, staff: staff, value: value})
	return nil
}

// consumerTimelineAdopted notes that Finish kept a timeline as its windows
// read it.
func (s *Session) consumerTimelineAdopted(key RuleKey) {
	if k := s.consumers; k != nil {
		k.adopted = key
	}
	s.performanceTimelineAdopted(key)
}

// consumerIndexBuilt notes a history index a consumer's question built in
// the speculative timeline: the timeline's own, as if a measure's read had
// built it. The next read that publishes to the history discards it, and
// dropping the speculation releases it with the rest.
func (s *Session) consumerIndexBuilt(bytes int64) {
	if c := s.consumerEvaluating; c != nil {
		c.speculation.retained += bytes
	}
	if p := s.performanceStash; p != nil {
		if spec := s.speculations[p.timeline]; spec != nil {
			spec.retained += bytes
		}
	}
}

// consumersPending reports whether windows left provisional decisions.
func (s *Session) consumersPending() bool {
	return s.consumers != nil && s.consumers.pending != 0
}

// settleConsumerPhase adopts what windows decided in place of the phase
// Finish is about to run, or drops every provisional decision left if an
// assumption failed.
func (s *Session) settleConsumerPhase(ctx context.Context, phase int) error {
	k := s.consumers
	for i, rule := range k.rules {
		if rule.phase != phase {
			continue
		}
		failed, err := s.confirmConsumerGroup(ctx, rule, &k.groups[i])
		if err != nil {
			return err
		}
		if failed != "" {
			s.dropWindowConsumers(failed)
			return nil
		}
		return s.adoptConsumerGroup(i)
	}
	return nil
}

// adoptConsumerGroup records the decisions of a group whose assumptions
// hold, with their finding keys, and caches them as the phase would.
func (s *Session) adoptConsumerGroup(i int) error {
	k := s.consumers
	g := k.groups[i]
	k.groups[i] = consumerGroup{}
	k.pending -= g.decisions
	defer s.releaseConsumerGroup(g)
	s.accumulator.report.Counts.Pass += g.pass
	s.accumulator.report.Counts.NotApplicable += g.notApplicable
	for key, bits := range g.decided {
		if err := s.adoptOutcomes(s.catalog.rules[key], bits); err != nil {
			return err
		}
	}
	// The accumulator orders findings by their keys; the records of one
	// key were decided one after another.
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
	if windowConsumersObserver != nil {
		windowConsumersObserver("", g.decisions)
	}
	return nil
}

// The assumptions of the provisional decisions, as the observer names the
// one that failed.
const (
	consumerFailedSpeculation  = "speculation"
	consumerFailedRebuilt      = "rebuilt"
	consumerFailedPrerequisite = "prerequisite"
	consumerFailedQuery        = "query"
	// The session ended before the phases of the decisions left.
	consumerFailedUnsettled = "unsettled"
)

// confirmConsumerGroup checks what a group's decisions assumed and names
// the first assumption that failed.
func (s *Session) confirmConsumerGroup(ctx context.Context, rule consumerRule, g *consumerGroup) (string, error) {
	service := s.timeline[rule.timeline]
	if s.consumers.adopted != rule.timeline || service == nil || !service.Final {
		return consumerFailedSpeculation, nil
	}
	// A timeline rebuilt to prove excursion units republished its facts.
	if service.unitProofFinal {
		return consumerFailedRebuilt, nil
	}
	prerequisite := s.documentPrerequisites[rule.timeline]
	if prerequisite == nil || !prerequisite.finalized {
		return consumerFailedPrerequisite, nil
	}
	d, pending, err := s.dependency(ctx, s.root, Dependency{Kind: DependencyRule, Rule: rule.timeline, Relation: RelationDocument}, true)
	if err != nil {
		return "", err
	}
	if pending || d.Outcome != OutcomePass {
		return consumerFailedPrerequisite, nil
	}
	for i, query := range g.queries {
		result, err := service.State.effectiveKeyPresence(ctx, s, query.position, query.staff)
		if err != nil {
			return "", err
		}
		if i == 0 && windowConsumersTamper == "query" {
			result.Value.Reason = "tampered"
		}
		if !reflect.DeepEqual(result.Value, query.value) {
			return consumerFailedQuery, nil
		}
	}
	return "", nil
}

// adoptOutcomes moves a rule's provisional pass and not-applicable outcomes
// into the session's; no node has an outcome in both.
func (s *Session) adoptOutcomes(r RuleDefinition, bits *outcomeBits) error {
	for w, word := range bits.words {
		for i := 0; word != 0; i, word = i+1, word>>2 {
			if v := successfulOutcome(word & 3); v != 0 {
				if err := s.cacheOutcome(r, NodeID(32*w+i+1), v); err != nil {
					return err
				}
			}
		}
	}
	for id, v := range bits.overflow {
		if err := s.cacheOutcome(r, id, v); err != nil {
			return err
		}
	}
	return nil
}

// releaseConsumerGroup releases what g retained, once nothing reaches it.
func (s *Session) releaseConsumerGroup(g consumerGroup) {
	for _, bits := range g.decided {
		bits.release(s)
	}
	s.releaseFactBytes(g.charged)
}

// dropWindowConsumers drops every provisional decision left, for the
// assumption that failed; the phases of Finish then decide their rules.
func (s *Session) dropWindowConsumers(failed string) {
	k := s.consumers
	if k == nil {
		return
	}
	if windowConsumersObserver != nil && k.pending != 0 {
		windowConsumersObserver(failed, k.pending)
	}
	for i := range k.groups {
		s.releaseConsumerGroup(k.groups[i])
		k.groups[i] = consumerGroup{}
	}
	k.pending = 0
}

// windowConsumersOff is a test switch only: it decides every timeline
// consumer in the phases of Finish under the windowed strategy too.
var windowConsumersOff bool

func setWindowConsumersOff(off bool) bool {
	previous := windowConsumersOff
	windowConsumersOff = off
	return previous
}

// windowConsumersTamper is a test switch only: "query" changes the first
// key presence read from the final state before Finish compares it, which
// no document found changes without rebuilding its timeline.
var windowConsumersTamper string

func setWindowConsumersTamper(kind string) string {
	previous := windowConsumersTamper
	windowConsumersTamper = kind
	return previous
}

// windowConsumersObserver is a test observation point only: it receives,
// each time Finish settles decisions windows made provisionally, the
// assumption that failed (empty when it adopted them) and how many there
// were.
var windowConsumersObserver func(failed string, decisions int)

func setWindowConsumersObserver(observer func(string, int)) func(string, int) {
	previous := windowConsumersObserver
	windowConsumersObserver = observer
	return previous
}
