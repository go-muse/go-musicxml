package validation

import (
	"context"
	"strings"
	"testing"
)

func TestPerformanceConfirmation(t *testing.T) {
	provider := RuleKey{ID: "performance", Revision: 1}
	for _, tc := range []struct {
		name, failed string
		change       func(*Session, *performanceWindowProvider)
	}{
		{"all hold", "", func(*Session, *performanceWindowProvider) {}},
		{"timeline read again", "speculation", func(_ *Session, p *performanceWindowProvider) { p.adoptedTimeline = RuleKey{} }},
		{"timeline not final", "speculation", func(s *Session, _ *performanceWindowProvider) { s.timeline[provisionalTimeline].Final = false }},
		{"timeline rebuilt", "rebuilt", func(s *Session, _ *performanceWindowProvider) { s.timeline[provisionalTimeline].unitProofFinal = true }},
		{"timeline phase not over", "prerequisite", func(s *Session, _ *performanceWindowProvider) {
			s.documentPrerequisites[provisionalTimeline].finalized = false
		}},
		{"document incomplete", "prerequisite", func(s *Session, _ *performanceWindowProvider) {
			s.documentPrerequisites[provisionalTimeline].decision = Decision{Outcome: OutcomeUnknown, Reason: "prerequisite-document-incomplete"}
		}},
		{"anchors unfinished", "origins", func(_ *Session, p *performanceWindowProvider) {
			p.anchors = append(p.anchors, performanceWindowAnchor{})
		}},
		{"own trigger", "excursions", func(_ *Session, p *performanceWindowProvider) { p.failed = "excursions" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, _, _, _ := provisionalFixture(t)
			p := &performanceWindowProvider{key: provider, timeline: provisionalTimeline, adoptedTimeline: provisionalTimeline}
			s.performance = &windowPerformance{provider: p}
			s.timeline = map[RuleKey]*timelineService{provisionalTimeline: {Final: true}}
			s.documentPrerequisites = map[RuleKey]*documentPrerequisite{provisionalTimeline: {finalized: true, ready: true, decision: Decision{Outcome: OutcomePass}}}
			tc.change(s, p)
			failed, err := s.confirmPerformanceProvider(context.Background(), p)
			if err != nil || failed != tc.failed {
				t.Fatalf("failed=%q err=%v, want %q", failed, err, tc.failed)
			}
		})
	}
}

func TestPerformanceQuestionIndexBelongsToTheSpeculation(t *testing.T) {
	s, _, _, _, _ := provisionalFixture(t)
	spec := &timelineSpeculation{service: &timelineService{}}
	s.speculations = map[RuleKey]*timelineSpeculation{provisionalTimeline: spec}
	p := &performanceWindowProvider{timeline: provisionalTimeline}
	s.performanceStash = p
	h := &stateHistory[bool]{}
	before := s.retained
	index, err := stateIndex(context.Background(), s, h)
	if err != nil {
		t.Fatal(err)
	}
	built := s.retained - before
	if built == 0 || spec.retained != built || index.bytes != built {
		t.Fatalf("index retained %d bytes, speculation counts %d", built, spec.retained)
	}
	s.performanceStash = nil
	stateDiscardIndex(s, h.index)
	h.index = nil
}

func TestPerformanceContextStashIsInvisibleAndReleased(t *testing.T) {
	s, _, _, node, _ := provisionalFixture(t)
	p := &performanceWindowProvider{key: RuleKey{ID: "performance", Revision: 1}}
	s.performance = &windowPerformance{provider: p}
	s.performanceStash = p
	s.contextFamilies = ContextFamilyTiming
	before, interned := s.retained, internCharge(s)
	text := strings.Repeat("long", 4*reportValueBytes)
	result := ContextResult{Provider: p.key, Subject: NodeRef{s.documentID, node.header.ID}, Property: ContextPropertyTiming, Origins: s.nodeOrigins(node), Timing: &TimingContextResult{Kind: TimingKindNote, MeasureLabel: Evidence[string]{State: EvidenceKnown, Value: text}}}
	if err := s.publishContextResult(context.Background(), result); err != nil {
		t.Fatal(err)
	}
	if len(s.contextResults) != 0 || p.pendingFacts != 1 || len(p.records) != 1 || p.records[0].charged == 0 {
		t.Fatalf("results=%d facts=%d records=%d", len(s.contextResults), p.pendingFacts, len(p.records))
	}
	if got := p.records[0].result.Timing.MeasureLabel.Value; got != text {
		t.Fatal("the owned result changed")
	}
	s.dropWindowPerformance("speculation")
	if len(p.records) != 0 || p.pendingFacts != 0 {
		t.Fatal("the dropped provider still holds results")
	}
	if got, want := s.retained-before, internCharge(s)-interned; got != want {
		t.Fatalf("dropped records retain %d bytes, session interning %d", got, want)
	}
}

func TestPerformanceBendWaitsForItsPhase(t *testing.T) {
	s, r, _, node, _ := provisionalFixture(t)
	provider := RuleKey{ID: "performance", Revision: 1}
	p := &performanceWindowProvider{key: provider, adopted: true}
	b := &performanceWindowBend{key: r.Key, provider: provider, phase: 2}
	s.performance = &windowPerformance{provider: p, bends: []performanceWindowBend{*b}}
	s.performanceBendEvaluating = &s.performance.bends[0]
	if err := s.record(r, node, Decision{Outcome: OutcomeFail, Reason: "bend-release-must-be-negative"}); err != nil {
		t.Fatal(err)
	}
	s.performanceBendEvaluating = nil
	if err := s.settlePerformancePhase(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if s.performance.bends[0].group.decisions != 1 {
		t.Fatal("the bend was settled before its phase")
	}
	if _, ok := s.lookupDecision(decisionID{r.Key, node.header.ID}); ok {
		t.Fatal("the bend is visible before its phase")
	}
	s.documentPrerequisites = map[RuleKey]*documentPrerequisite{provider: {finalized: true, ready: true, decision: Decision{Outcome: OutcomePass}}}
	if err := s.settlePerformancePhase(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	if s.performance.bends[0].group.decisions != 0 {
		t.Fatal("the bend group was not adopted")
	}
	if d, ok := s.lookupDecision(decisionID{r.Key, node.header.ID}); !ok || d.Outcome != OutcomeFail {
		t.Fatalf("decision=%+v found=%v", d, ok)
	}
}
