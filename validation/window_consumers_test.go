package validation

import (
	"context"
	"math"
	"reflect"
	"strings"
	"testing"
)

var provisionalTimeline = RuleKey{ID: "timeline", Revision: 1}

// provisionalFixture is a session whose only rule stands for a consumer of
// provisionalTimeline with two phases to decide in, and the consumer
// evaluations of the two.
func provisionalFixture(t *testing.T) (*Session, RuleDefinition, *factNode, *factNode, [2]*consumerEvaluation) {
	t.Helper()
	root := localNode(1, 0, "root")
	owner := localNode(2, 1, "owner")
	child := localNode(3, 2, "value")
	root.children = []*factNode{owner}
	owner.children = []*factNode{child}
	p, err := NewChildCountPlan([]DeclarationID{localDecl("value")}, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	s, key, _ := localIsolatedSession(t, p, 1<<30, root, owner, owner, child)
	s.consumers = &windowConsumers{
		rules:  []consumerRule{{key: key, timeline: provisionalTimeline, phase: 0}, {key: key, timeline: provisionalTimeline, phase: 1}},
		groups: make([]consumerGroup, 2),
	}
	var evaluations [2]*consumerEvaluation
	for i := range evaluations {
		evaluations[i] = &consumerEvaluation{timeline: provisionalTimeline, speculation: &timelineSpeculation{}, group: &s.consumers.groups[i]}
	}
	return s, s.catalog.rules[key], owner, child, evaluations
}

// What windows decide provisionally is read only while a window's timeline
// consumers are decided and counts nothing, and dropping it releases exactly
// what it retained: the outcome tables with their overflow, the records with
// their index and grown array, and the grown array of key presences asked.
func TestConsumerDecisionsAreApartAndReleased(t *testing.T) {
	s, r, owner, child, evaluations := provisionalFixture(t)
	before := s.retained
	s.consumerEvaluating = evaluations[0]
	if err := s.record(r, owner, Decision{Outcome: OutcomeFail, Reason: "failed", Expected: "one", Actual: strings.Repeat("2", 4*reportValueBytes), Related: []Location{child.location()}}); err != nil {
		t.Fatal(err)
	}
	g := &s.consumers.groups[0]
	if kept := g.records[0].decision; len(kept.Actual) > reportValueBytes || kept.Expected != "one" {
		t.Fatalf("kept %d bytes of the actual value and expected %q", len(kept.Actual), kept.Expected)
	}
	if err := s.record(r, child, Decision{Outcome: OutcomePass}); err != nil {
		t.Fatal(err)
	}
	// A node numbered far beyond the admitted nodes goes to the outcome
	// table's overflow.
	sparse := localNode(1<<40, 1, "value")
	if err := s.record(r, sparse, Decision{Outcome: OutcomeNotApplicable}); err != nil {
		t.Fatal(err)
	}
	// More records and key presences than their first arrays hold.
	for i := 0; i < 40; i++ {
		if err := s.record(r, localNode(NodeID(100+i), 1, "value"), Decision{Outcome: OutcomeUnknown, Reason: "unknown"}); err != nil {
			t.Fatal(err)
		}
		if err := s.journalKeyPresence(musicalPosition{}, Evidence[Integer]{State: EvidenceAbsent}, Evidence[bool]{State: EvidenceAbsent}); err != nil {
			t.Fatal(err)
		}
	}
	// A rule that is not a normative mandatory predicate counts no pass.
	advisory := r
	advisory.Category = CategoryAdvisory
	if err := s.record(advisory, localNode(200, 1, "value"), Decision{Outcome: OutcomePass}); err != nil {
		t.Fatal(err)
	}
	if g.pass != 1 || g.notApplicable != 1 {
		t.Fatalf("%d passes and %d exclusions counted provisionally", g.pass, g.notApplicable)
	}
	for _, n := range []*factNode{owner, child, sparse} {
		d, ok := s.lookupDecision(decisionID{r.Key, n.header.ID})
		if !ok || d.Expected != "" || d.Actual != "" || d.Related != nil {
			t.Fatalf("node %d: provisional decision %+v, found %v", n.header.ID, d, ok)
		}
	}
	// Each phase's evaluation reads what the other decided too.
	s.consumerEvaluating = evaluations[1]
	if _, ok := s.lookupDecision(decisionID{r.Key, owner.header.ID}); !ok {
		t.Fatal("a provisional decision is not read from another phase's group")
	}
	other := localNode(300, 1, "value")
	if err := s.record(r, other, Decision{Outcome: OutcomeUnknown, Reason: "unknown"}); err != nil {
		t.Fatal(err)
	}
	s.consumerEvaluating = evaluations[0]
	if _, ok := s.lookupDecision(decisionID{r.Key, other.header.ID}); !ok {
		t.Fatal("a provisional decision is not read from another phase's group")
	}
	s.consumerEvaluating = nil
	for _, n := range []*factNode{owner, child, sparse} {
		if _, ok := s.lookupDecision(decisionID{r.Key, n.header.ID}); ok {
			t.Fatalf("node %d: provisional decision read outside a window's consumers", n.header.ID)
		}
	}
	if counts := s.accumulator.report.Counts; counts.Fail != 0 || counts.Pass != 0 || counts.NotApplicable != 0 || counts.Unknown != 0 {
		t.Fatalf("provisional decisions counted: %+v", counts)
	}
	tables := int64(0)
	for _, bits := range g.decided {
		tables += bits.charged
		if n := len(bits.overflow); n != 0 {
			tables += outcomeOverflowHeaderBytes + int64(n)*outcomeOverflowEntryBytes
		}
	}
	tables += s.consumers.groups[1].charged
	// The group is charged its table, each record with its index entry
	// and location, and the arrays as they are now: a replaced array is
	// released.
	want := consumerTableBytes + int64(len(r.Key.ID)) + int64(cap(g.records))*consumerRecordBytes + int64(cap(g.queries))*consumerQueryBytes
	for _, rec := range g.records {
		size := reportStorageSize{limit: math.MaxInt64}
		size.decision(rec.decision)
		want += size.total + consumerIndexBytes + int64(len(rec.location.Path))
	}
	if g.charged != want {
		t.Fatalf("the group is charged %d bytes, want %d", g.charged, want)
	}
	if s.consumers.pending != 45 || len(g.records) != 41 || len(g.queries) != 40 || s.retained-before != g.charged+tables {
		t.Fatalf("%d decisions, %d records, %d queries; %d bytes retained, %d charged to the group and %d to its tables", s.consumers.pending, len(g.records), len(g.queries), s.retained-before, g.charged, tables)
	}
	var failed string
	previous := setWindowConsumersObserver(func(reason string, _ int) { failed = reason })
	s.dropWindowConsumers(consumerFailedUnsettled)
	setWindowConsumersObserver(previous)
	if failed != consumerFailedUnsettled || s.retained != before || s.consumersPending() {
		t.Fatalf("dropped for %q, %d bytes still retained", failed, s.retained-before)
	}
	// Nothing released stays reachable.
	if !reflect.DeepEqual(s.consumers.groups, make([]consumerGroup, 2)) {
		t.Fatalf("dropped groups keep %+v", s.consumers.groups)
	}
}

// Finish adopts the provisional decisions of a phase only when the timeline
// they read was kept as its windows read it, was not rebuilt, and its rule
// passes as the document prerequisite of its consumers. Adopting records
// each decision once and leaves the other phase's decisions provisional; a
// failed assumption drops those of both.
func TestConsumerConfirmation(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name, failed string
		change       func(s *Session)
	}{
		{"all hold", "", func(*Session) {}},
		{"timeline read again", consumerFailedSpeculation, func(s *Session) { s.consumers.adopted = RuleKey{} }},
		{"timeline not final", consumerFailedSpeculation, func(s *Session) { s.timeline[provisionalTimeline].Final = false }},
		{"timeline rebuilt", consumerFailedRebuilt, func(s *Session) { s.timeline[provisionalTimeline].unitProofFinal = true }},
		{"timeline phase not over", consumerFailedPrerequisite, func(s *Session) { s.documentPrerequisites[provisionalTimeline].finalized = false }},
		{"document incomplete", consumerFailedPrerequisite, func(s *Session) {
			s.documentPrerequisites[provisionalTimeline].decision = Decision{Outcome: OutcomeUnknown, Reason: "prerequisite-document-incomplete"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, owner, child, evaluations := provisionalFixture(t)
			before := s.retained
			sparse := localNode(1<<40, 1, "value")
			later := localNode(4, 1, "value")
			s.consumerEvaluating = evaluations[0]
			if err := s.record(r, owner, Decision{Outcome: OutcomeFail, Reason: "failed"}); err != nil {
				t.Fatal(err)
			}
			for n, d := range map[*factNode]Outcome{child: OutcomePass, sparse: OutcomeNotApplicable} {
				if err := s.record(r, n, Decision{Outcome: d}); err != nil {
					t.Fatal(err)
				}
			}
			s.consumerEvaluating = evaluations[1]
			if err := s.record(r, later, Decision{Outcome: OutcomeFail, Reason: "failed"}); err != nil {
				t.Fatal(err)
			}
			s.consumerEvaluating = nil
			provisional := s.retained - before
			s.timeline = map[RuleKey]*timelineService{provisionalTimeline: {Final: true}}
			s.documentPrerequisites = map[RuleKey]*documentPrerequisite{provisionalTimeline: {finalized: true, ready: true, decision: Decision{Outcome: OutcomePass}}}
			s.consumerTimelineAdopted(provisionalTimeline)
			tc.change(s)
			var settled []string
			previous := setWindowConsumersObserver(func(reason string, decisions int) { settled = append(settled, reason+"/"+string(rune('0'+decisions))) })
			defer setWindowConsumersObserver(previous)
			if err := s.settleConsumerPhase(ctx, 0); err != nil {
				t.Fatal(err)
			}
			// What was adopted or dropped is no longer held.
			if !reflect.DeepEqual(s.consumers.groups[0], consumerGroup{}) {
				t.Fatalf("the settled group keeps %+v", s.consumers.groups[0])
			}
			adopted := tc.failed == ""
			want := []string{tc.failed + "/4"}
			if adopted {
				want = []string{"/3"}
			}
			if len(settled) != 1 || settled[0] != want[0] {
				t.Fatalf("settled %v, want %v", settled, want)
			}
			for _, n := range []*factNode{owner, child, sparse} {
				if _, recorded := s.lookupDecision(decisionID{r.Key, n.header.ID}); recorded != adopted {
					t.Fatalf("node %d: decision recorded: %v", n.header.ID, recorded)
				}
			}
			count := map[bool]uint64{true: 1}[adopted]
			if counts := s.accumulator.report.Counts; counts.Fail != count || counts.Pass != count || counts.NotApplicable != count {
				t.Fatalf("counted %+v, want %d of each", counts, count)
			}
			// The other phase's decision is neither recorded nor dropped by
			// an adoption, and dropped with the rest by a failure.
			if _, recorded := s.lookupDecision(decisionID{r.Key, later.header.ID}); recorded || s.consumersPending() != adopted {
				t.Fatalf("later phase: recorded %v, pending %v", recorded, s.consumersPending())
			}
			if !adopted && s.retained != before {
				t.Fatalf("%d of %d provisional bytes still retained", s.retained-before, provisional)
			}
			if err := s.settleConsumerPhase(ctx, 1); err != nil {
				t.Fatal(err)
			}
			if _, recorded := s.lookupDecision(decisionID{r.Key, later.header.ID}); recorded != adopted || s.consumersPending() {
				t.Fatalf("later phase after its settling: recorded %v, pending %v", recorded, s.consumersPending())
			}
			if !adopted {
				return
			}
			// Adopted decisions leave charged what recording them in their
			// phases charges, and nothing of the provisional store.
			direct, _, _, _, _ := provisionalFixture(t)
			directBefore := direct.retained
			for _, decided := range []struct {
				n *factNode
				d Decision
			}{{owner, Decision{Outcome: OutcomeFail, Reason: "failed"}}, {child, Decision{Outcome: OutcomePass}}, {sparse, Decision{Outcome: OutcomeNotApplicable}}, {later, Decision{Outcome: OutcomeFail, Reason: "failed"}}} {
				if err := direct.record(r, decided.n, decided.d); err != nil {
					t.Fatal(err)
				}
			}
			if got, want := s.retained-before, direct.retained-directBefore; got != want {
				t.Fatalf("adoption leaves %d bytes retained, recording directly %d", got, want)
			}
		})
	}
}

// While a window's timeline consumers are decided, a rule of another kind
// waits for Finish and its evaluation is not observed as one of Finish, only
// the consumed timeline is read, its rule counts as passed for the document,
// and any other prerequisite leaves the node to Finish.
func TestConsumerEvaluationScope(t *testing.T) {
	ctx := context.Background()
	s, r, owner, _, evaluations := provisionalFixture(t)
	s.consumerEvaluating = evaluations[0]
	observed := 0
	previous := setFinishObserver(func(string, *Account) { observed++ })
	defer setFinishObserver(previous)
	if err := s.evaluate(ctx, owner, r.Key, true); err != nil {
		t.Fatal(err)
	}
	if s.consumersPending() {
		t.Fatal("a rule of another kind was decided with a window's consumers")
	}
	if observed != 0 {
		t.Fatal("a window's evaluation was observed as an evaluation of Finish")
	}
	service := &timelineService{}
	s.consumerEvaluating.speculation.service = service
	if s.consumerTimeline(provisionalTimeline) != service || s.consumerTimeline(r.Key) != nil {
		t.Fatal("a window's consumers read a timeline other than the one read as windows closed")
	}
	d, pending, err := s.dependency(ctx, owner, Dependency{Kind: DependencyRule, Rule: provisionalTimeline, Relation: RelationDocument}, true)
	if err != nil || pending || d.Outcome != OutcomePass {
		t.Fatalf("consumed timeline: %+v pending %v, %v", d, pending, err)
	}
	for _, dependency := range []Dependency{
		{Kind: DependencyRule, Rule: r.Key, Relation: RelationDocument},
		{Kind: DependencyRule, Rule: provisionalTimeline, Relation: RelationSameNode},
		{Kind: DependencyFact, Rule: provisionalTimeline, Key: "fact", Relation: RelationDocument},
	} {
		if _, pending, err := s.dependency(ctx, owner, dependency, true); err != nil || !pending {
			t.Fatalf("%+v: pending %v, %v", dependency, pending, err)
		}
	}
	s.consumerEvaluating = nil
	if err := s.evaluate(ctx, owner, r.Key, true); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.lookupDecision(decisionID{r.Key, owner.header.ID}); !ok || observed != 1 {
		t.Fatalf("the rule was decided outside a window's consumers: %v, observed %d times", ok, observed)
	}
}

// Deciding a window's consumers holds the speculation only while it runs:
// Finish may drop the speculation and release what it retained.
func TestConsumerEvaluationHoldsNothingAfterwards(t *testing.T) {
	s, r, owner, _, _ := provisionalFixture(t)
	s.strategy = strategyWindowed
	s.speculations = map[RuleKey]*timelineSpeculation{provisionalTimeline: {service: &timelineService{}}}
	if err := s.evaluateWindowConsumers(context.Background(), evaluationWindow{root: owner.header.ID, first: 1, end: 3}); err != nil {
		t.Fatal(err)
	}
	if s.consumerEvaluating != nil || s.consumers.evaluating != (consumerEvaluation{}) {
		t.Fatalf("the evaluation is still held: %+v", s.consumers.evaluating)
	}
	// The fixture's rule is not a consumer's: nothing was decided.
	if _, ok := s.lookupDecision(decisionID{r.Key, owner.header.ID}); ok || s.consumersPending() {
		t.Fatal("a rule of another kind was decided")
	}
}

// A history index a consumer's question builds in the speculative timeline
// is counted as retained by the speculation, exactly once: the read that
// next publishes to the history discards it on the speculation's account.
// An index built outside a window's consumers is not.
func TestConsumerQuestionIndexBelongsToTheSpeculation(t *testing.T) {
	ctx := context.Background()
	s, _, _, _, evaluations := provisionalFixture(t)
	h := &stateHistory[bool]{}
	before := s.retained
	s.consumerEvaluating = evaluations[0]
	index, err := stateIndex(ctx, s, h)
	if err != nil {
		t.Fatal(err)
	}
	spec := evaluations[0].speculation
	if built := s.retained - before; built == 0 || spec.retained != built || index.bytes != built {
		t.Fatalf("the index retained %d bytes, the speculation counts %d", built, spec.retained)
	}
	// Asked again, the same index answers and nothing more is counted.
	if again, err := stateIndex(ctx, s, h); err != nil || again != index || spec.retained != index.bytes {
		t.Fatalf("asked again: %v, the speculation counts %d", err, spec.retained)
	}
	s.consumerEvaluating = nil
	stateDiscardIndex(s, h.index)
	h.index = nil
	if _, err := stateIndex(ctx, s, h); err != nil {
		t.Fatal(err)
	}
	if spec.retained != index.bytes || s.retained-before != index.bytes {
		t.Fatalf("outside a window's consumers the speculation counts %d, the session %d", spec.retained, s.retained-before)
	}
}
