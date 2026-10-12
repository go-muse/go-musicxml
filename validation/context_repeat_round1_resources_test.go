package validation

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestRound1RepeatNonstickyQuota(t *testing.T) {
	s := rationalSession(t, Budgets{MaxRuleSteps: 7})
	defer s.cleanup()
	d := DeclarationID{Occurrence: "repeat"}
	n := &factNode{header: NodeHeader{ID: 1, Declaration: stateKnown(d)}, closed: true, end: NodeEnd{ChildrenComplete: stateKnown(true), Order: stateKnown(OrderSource)}}
	s.nodes = map[NodeID]*factNode{1: n}
	s.sequence = []NodeID{1}
	p := &contextPlan{Bindings: &contextBindings{Elements: map[contextRole]map[DeclarationID]struct{}{contextMeasureRepeat: {d: {}}}}}
	got, _, err := evaluateContextRepeats(context.Background(), s, n, RuleKey{"r", 1}, p, true)
	if !errors.Is(err, ErrResourceLimit) || got.Outcome == OutcomePass {
		t.Fatalf("refused role charge became success: %+v %v", got, err)
	}
	for _, service := range s.contextServices {
		if service.Repeats != nil && service.Repeats.Final {
			t.Fatal("partial rows finalized")
		}
	}
}
func TestRound1RepeatWindowScratchIsReleased(t *testing.T) {
	s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: 512 << 20})
	defer s.cleanup()
	const count = 1000
	notes := repeatWindowIndexNotes(t, count)
	part := NodeRef{"d", 1}
	beat := mustRational(t, 1, 3)
	start := statePosition(t, 0, 1, 3)
	end := statePosition(t, 0, count, 3)
	stop := NodeRef{"d", 5000}
	row := contextRepeatRecord{source: NodeRef{"d", 4000}, position: stateKnown(start), value: RepeatContextResult{Kind: RepeatStyleBeat, Type: stateKnown("start"), Part: stateKnown(part), AllStaves: stateKnown(true), Beat: stateKnown(beat), Stop: stateKnown(stop)}}
	rows := []contextRepeatRecord{row, {source: stop, position: stateKnown(end)}}
	if err := contextRepeatBeatProof(context.Background(), s, notes, rows, &row); err != nil {
		t.Fatal(err)
	}
	if s.retained > 1500*count {
		t.Fatalf("temporary note/window arrays retained permanently: %d", s.retained)
	}
	if got := rationalUsage(t, s, BudgetBufferedBytes); got != s.retained {
		t.Fatalf("unreleased scratch: gauge=%d retained=%d", got, s.retained)
	}
	t.Logf("1000 windows retain %d admitted arithmetic bytes; no temporary arrays", s.retained)
}
func round1RepeatIndexedFixture(t *testing.T, count int) (*Session, *contextReader, *timelineService, []contextRepeatNote, []contextRepeatRecord) {
	s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: 512 << 20})
	s.nodes = map[NodeID]*factNode{}
	part := NodeRef{"d", 1}
	timeline := &timelineService{Facts: map[NodeID]timelineEventFact{}}
	notes := make([]contextRepeatNote, 2*count)
	rows := make([]contextRepeatRecord, 0, 2*count)
	s.nodes[1] = &factNode{header: NodeHeader{ID: 1, Declaration: stateKnown(DeclarationID{Occurrence: "part"})}, closed: true, end: NodeEnd{ChildrenComplete: stateKnown(true), Order: stateKnown(OrderSource)}}
	s.sequence = append(s.sequence, 1)
	for i := range notes {
		pos := statePosition(t, uint64(i), 0, 1)
		pos.Part = part
		notes[i] = contextRepeatNote{source: NodeRef{"d", NodeID(10000 + i)}, part: stateKnown(part), pos: pos, duration: mustRational(t, 1, 1), available: stateKnown(true), rest: true, staff: stateAbsent[Integer](), voice: stateAbsent[string](), step: stateAbsent[string](), alter: stateAbsent[Decimal](), octave: stateAbsent[Integer](), instrument: stateAbsent[string]()}
		id := notes[i].source.Node
		s.nodes[id] = &factNode{header: NodeHeader{ID: id, Parent: 1, Declaration: stateKnown(DeclarationID{Occurrence: "note"})}, closed: true, end: NodeEnd{ChildrenComplete: stateKnown(true), Order: stateKnown(OrderSource)}}
		s.sequence = append(s.sequence, id)
		timeline.Facts[id] = timelineEventFact{Part: stateKnown(part), Position: stateKnown(pos), Origin: stateKnown(pos), Duration: stateKnown(notes[i].duration)}
	}
	for i := 0; i < count; i++ {
		start := notes[2*i+1].pos
		stop := statePosition(t, uint64(2*i+2), 0, 1)
		stop.Part = part
		id := NodeRef{"d", NodeID(20000 + 2*i)}
		end := NodeRef{"d", id.Node + 1}
		row := contextRepeatRecord{source: id, position: stateKnown(start), value: RepeatContextResult{Kind: RepeatStyleMeasure, Type: stateKnown("start"), Part: stateKnown(part), AllStaves: stateKnown(true), PatternMeasures: stateKnown(stateOne()), Complete: stateKnown(true), Stop: stateKnown(end)}}
		rows = append(rows, row, contextRepeatRecord{source: end, position: stateKnown(stop), value: RepeatContextResult{Kind: RepeatStyleMeasure, Type: stateKnown("stop"), Part: stateKnown(part)}})
	}
	return s, &contextReader{ctx: context.Background(), s: s, b: &contextBindings{}}, timeline, notes, rows
}
func TestRound1RepeatSharedScopeLinearWork(t *testing.T) {
	measure := func(count int) int64 {
		s, r, timeline, notes, rows := round1RepeatIndexedFixture(t, count)
		defer s.cleanup()
		roster, freeRoster, err := contextRepeatBuildContentRoster(context.Background(), s, notes)
		if err != nil {
			t.Fatal(err)
		}
		defer freeRoster()
		index, freeIndex, err := contextRepeatBuildProofIndex(r, timeline, rows)
		if err != nil {
			t.Fatal(err)
		}
		defer freeIndex()
		for i := range rows {
			if err := contextRepeatProveRosterRow(context.Background(), s, timeline, roster, rows, &rows[i], stateKnown(true), index); err != nil {
				t.Fatal(err)
			}
			if rows[i].value.Type.Value == "start" && (rows[i].value.ContentEquivalent.State != EvidenceKnown || !rows[i].value.ContentEquivalent.Value) {
				t.Fatalf("content=%+v", rows[i].value.ContentEquivalent)
			}
		}
		return rationalUsage(t, s, BudgetRuleSteps)
	}
	small, large := measure(100), measure(1000)
	t.Logf("shared repeated content scopes: %d -> %d steps", small, large)
	if large > 15*small {
		t.Fatalf("repeated scope proof remains quadratic: %d -> %d", small, large)
	}
}
func TestRound1RepeatProofIndexesCancellationAndGauge(t *testing.T) {
	run := func(ctx context.Context, s *Session, r *contextReader, timeline *timelineService, notes []contextRepeatNote, rows []contextRepeatRecord) error {
		r.ctx = ctx
		roster, freeRoster, err := contextRepeatBuildContentRoster(ctx, s, notes)
		if err != nil {
			return err
		}
		defer freeRoster()
		index, freeIndex, err := contextRepeatBuildProofIndex(r, timeline, rows)
		if err != nil {
			return err
		}
		defer freeIndex()
		for i := range rows {
			if err := contextRepeatProveRosterRow(ctx, s, timeline, roster, rows, &rows[i], stateKnown(true), index); err != nil {
				return err
			}
		}
		return nil
	}
	s, r, timeline, notes, rows := round1RepeatIndexedFixture(t, 2)
	counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	if err := run(counter, s, r, timeline, notes, rows); err != nil {
		t.Fatal(err)
	}
	work := rationalUsage(t, s, BudgetRuleSteps)
	s.cleanup()
	for at := 1; at <= counter.calls; at++ {
		s, r, timeline, notes, rows := round1RepeatIndexedFixture(t, 2)
		err := run(&rationalCancelContext{Context: context.Background(), at: at}, s, r, timeline, notes, rows)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("poll %d: %v", at, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatal("index scratch leak")
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("cleanup leak")
		}
	}
	for _, delta := range []int64{-1, 0} {
		s, r, timeline, notes, rows := round1RepeatIndexedFixture(t, 2)
		account, err := NewBudgetAccount(Budgets{MaxRuleSteps: work + delta})
		if err != nil {
			t.Fatal(err)
		}
		s.account = account
		err = run(context.Background(), s, r, timeline, notes, rows)
		if (err == nil) != (delta == 0) {
			t.Fatalf("quota %d: %v", delta, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatal("quota leaked index")
		}
		s.cleanup()
	}
	t.Logf("interrupted all %d shared proof polls", counter.calls)
}
func TestRound1RepeatProofCoveragePartIsolationAndUnknownMembership(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		s := rationalSession(t, Budgets{})
		defer s.cleanup()
		cause := &DecisionRef{Rule: RuleKey{"structural-content", 1}, Subject: NodeRef{"d", 9}}
		n := &factNode{header: NodeHeader{ID: 9, Declaration: Evidence[DeclarationID]{State: EvidenceInvalid, Reason: "bad-note-declaration", Cause: cause}}, closed: true, end: NodeEnd{ChildrenComplete: stateKnown(true), Order: stateKnown(OrderSource)}}
		s.nodes = map[NodeID]*factNode{9: n}
		s.sequence = []NodeID{9}
		part := stateKnown(NodeRef{"d", 2})
		if unknown {
			part = stateUnknown[NodeRef]("part-unplaced")
		}
		timeline := &timelineService{Facts: map[NodeID]timelineEventFact{9: {Part: part}}}
		r := &contextReader{ctx: context.Background(), s: s, b: &contextBindings{}}
		index, free, err := contextRepeatBuildProofIndex(r, timeline, nil)
		if err != nil {
			t.Fatal(err)
		}
		got := index.partCoverage(stateKnown(NodeRef{"d", 1}))
		if unknown {
			if got.State != EvidenceInvalid || got.Cause != cause {
				t.Fatalf("unknown member lost original cause: %+v", got)
			}
		} else if got.State != EvidenceKnown || !got.Value {
			t.Fatalf("other part contaminated: %+v", got)
		}
		free()
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatal("index scratch leak")
		}
	}
}
func TestRound1RepeatHugeClosureIsNotWrapped(t *testing.T) {
	s, r, timeline, notes, rows := round1RepeatIndexedFixture(t, 1)
	defer s.cleanup()
	rows[0].position.Value.MeasureOrdinal = math.MaxUint64
	rows[0].value.Stop = stateAbsent[NodeRef]()
	timeline.State.Closures = []stateClosure{{Part: rows[0].value.Part.Value, Through: math.MaxUint64, Complete: stateKnown(true)}}
	index, free, err := contextRepeatBuildProofIndex(r, timeline, rows)
	if err != nil {
		t.Fatal(err)
	}
	defer free()
	if err := contextRepeatProveRow(context.Background(), s, timeline, notes, rows, &rows[0], stateKnown(true), index); err != nil {
		t.Fatal(err)
	}
	if rows[0].value.ContentEquivalent.State == EvidenceKnown {
		t.Fatal("overflowed part end became known content")
	}
}

func TestRound1RepeatEmptyCoverageKeepsFutureDisplacement(t *testing.T) {
	part := NodeRef{"d", 1}
	source := statePosition(t, 0, 0, 1)
	future := statePosition(t, 2, -1, 1)
	future.UnprovedFrame = true
	principal := contextRepeatNote{source: NodeRef{"d", 10}, part: stateKnown(part), pos: source, duration: mustRational(t, 1, 1), available: stateKnown(true)}
	displaced := contextRepeatNote{source: NodeRef{"d", 11}, part: stateKnown(part), pos: future, duration: mustRational(t, 1, 1), available: stateKnown(true)}
	contained := displaced
	contained.pos = statePosition(t, 2, 0, 1)
	for _, tc := range []struct {
		name  string
		notes []contextRepeatNote
		want  EvidenceState
	}{
		{"unconfined-future", []contextRepeatNote{principal, displaced}, EvidenceUnknown},
		{"future-omitted", []contextRepeatNote{principal}, EvidenceKnown},
		{"future-contained", []contextRepeatNote{principal, contained}, EvidenceKnown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			scope := &contextRepeatProofScope{notes: tc.notes, complete: stateKnown(true), buckets: map[uint64][]contextRepeatNote{}}
			defer scope.closeOnsets()
			got, err := contextRepeatEmptyMeasure(context.Background(), s, &timelineService{}, scope, part, 0, 1)
			if err != nil || got.State != tc.want || got.State == EvidenceKnown && got.Value {
				t.Fatalf("future onset confinement: %+v %v, want %s(false)", got, err, tc.want)
			}
		})
	}
}

func TestRound1RepeatCoverageCausalOrderAcrossScopes(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		s := rationalSession(t, Budgets{})
		defer s.cleanup()
		part := NodeRef{"d", 1}
		a := &DecisionRef{Rule: RuleKey{"local-gap", 1}, Subject: NodeRef{"d", 10}}
		b := &DecisionRef{Rule: RuleKey{"unknown-gap", 1}, Subject: NodeRef{"d", 11}}
		s.nodes = map[NodeID]*factNode{}
		s.sequence = []NodeID{10, 11}
		if reverse {
			s.sequence = []NodeID{11, 10}
		}
		for id, cause := range map[NodeID]*DecisionRef{10: a, 11: b} {
			s.nodes[id] = &factNode{header: NodeHeader{ID: id, Declaration: Evidence[DeclarationID]{State: EvidenceInvalid, Reason: "declaration-gap", Cause: cause}}, closed: true, end: NodeEnd{ChildrenComplete: stateKnown(true), Order: stateKnown(OrderSource)}}
		}
		timeline := &timelineService{Facts: map[NodeID]timelineEventFact{10: {Part: stateKnown(part)}, 11: {Part: stateUnknown[NodeRef]("unplaced")}}}
		r := &contextReader{ctx: context.Background(), s: s, b: &contextBindings{}}
		index, free, err := contextRepeatBuildProofIndex(r, timeline, nil)
		if err != nil {
			t.Fatal(err)
		}
		got := index.partCoverage(stateKnown(part))
		want := a
		if reverse {
			want = b
		}
		if got.State != EvidenceInvalid || got.Cause != want {
			t.Fatalf("reverse=%v original cause order lost: %+v", reverse, got)
		}
		free()
	}
}
