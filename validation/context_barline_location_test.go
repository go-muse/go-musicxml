package validation

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
)

// Inputs here are already-admitted timeline facts. Adapter tests independently
// exercise real source/model extraction, schema evidence and hierarchy binding.
func barlineBoundaryFixture(t *testing.T, budgets Budgets) (*Session, *timelineService, *timelineBindings, *factNode, musicalPosition) {
	t.Helper()
	s := rationalSession(t, budgets)
	s.documentID = "d"
	b := stateBindings()
	first := runtimeNote(t, b, 20, 4, false)
	backup := stateNode(b, timelineRoleBackup, 30, stateAbsent[Atomic](), stateNode(b, timelineRoleBackupDuration, 31, stateAtomicDecimal(t, 4, 1)))
	second := runtimeNote(t, b, 40, 1, false)
	forward := stateNode(b, timelineRoleForward, 50, stateAbsent[Atomic](), stateNode(b, timelineRoleForwardDuration, 51, stateAtomicDecimal(t, 3, 1)))
	bar := stateNode(b, timelineRoleBarline, 60, stateAbsent[Atomic]())
	content := stateNode(b, timelineRoleMeasure, 10, stateAbsent[Atomic](), first, backup, second, forward, bar)
	service := &timelineService{Final: true, Facts: map[NodeID]timelineEventFact{}}
	for _, row := range []struct {
		id                   NodeID
		onset, duration, end int64
	}{{20, 0, 4, 4}, {30, 4, 4, 0}, {40, 0, 1, 1}, {50, 1, 3, 4}, {60, 4, 0, 4}} {
		onset, end := statePosition(t, 0, row.onset, 1), statePosition(t, 0, row.end, 1)
		end.UnprovedFrame = true
		service.Facts[row.id] = timelineEventFact{Origin: stateKnown(onset), End: stateKnown(end), Duration: stateKnown(mustRational(t, row.duration, 1)), Units: stateKnown(stateDecimal(t, 1, 1)), Principal: stateKnown(NodeRef{"d", row.id}), PrincipalDuration: stateKnown(mustRational(t, row.duration, 1))}
	}
	service.State.frameBounds = map[timelineFrame]Rational{frameOf(statePosition(t, 0, 0, 1)): mustRational(t, 99, 1)}
	return s, service, b, content, statePosition(t, 0, 4, 1)
}
func TestBarlineLocationCertificateDoesNotStrengthenTimeline(t *testing.T) {
	s, service, b, content, at := barlineBoundaryFixture(t, Budgets{})
	defer s.cleanup()
	original := service.Facts[20]
	got, d, err := barlineLocationBoundary(context.Background(), s, service, b, content, at, "right")
	if err != nil || got.State != EvidenceKnown || got.Value.String() != "4" || d.Outcome != "" {
		t.Fatalf("end=%+v decision=%+v err=%v", got, d, err)
	}
	if service.State.frameBounds[frameOf(at)].String() != "99" || !reflect.DeepEqual(original, service.Facts[20]) || !service.Facts[20].End.Value.UnprovedFrame {
		t.Fatal("boundary proof changed shared timeline evidence")
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
		t.Fatal("scratch leaked")
	}
}
func TestBarlineLocationBoundaryBudgetsAndCancellation(t *testing.T) {
	s, service, b, content, at := barlineBoundaryFixture(t, Budgets{})
	counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	if _, _, err := barlineLocationBoundary(counter, s, service, b, content, at, "right"); err != nil {
		t.Fatal(err)
	}
	work := rationalUsage(t, s, BudgetRuleSteps)
	s.cleanup()
	for _, delta := range []int64{-1, 0} {
		s, service, b, content, at := barlineBoundaryFixture(t, Budgets{MaxRuleSteps: work + delta})
		got, d, err := barlineLocationBoundary(context.Background(), s, service, b, content, at, "right")
		if delta < 0 && (!errors.Is(err, ErrResourceLimit) || got.State == EvidenceKnown || d.Outcome == OutcomeFail) || delta == 0 && (err != nil || got.State != EvidenceKnown) {
			t.Fatalf("quota delta=%d: %+v %+v %v", delta, got, d, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatal("quota scratch leak")
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("cleanup leak")
		}
	}
	for stop := 1; stop <= counter.calls; stop++ {
		s, service, b, content, at := barlineBoundaryFixture(t, Budgets{})
		got, d, err := barlineLocationBoundary(&rationalCancelContext{Context: context.Background(), at: stop}, s, service, b, content, at, "right")
		if !errors.Is(err, context.Canceled) || got.State == EvidenceKnown || d.Outcome == OutcomeFail {
			t.Fatalf("cancel %d: %+v %+v %v", stop, got, d, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatal("cancel scratch leak")
		}
		s.cleanup()
	}
}
func TestBarlineLocationIndependentLowerSurvivesUnknownSuffix(t *testing.T) {
	for _, location := range []string{"right", "middle"} {
		s, service, b, content, at := barlineBoundaryFixture(t, Budgets{})
		defer s.cleanup()
		at.Offset = mustRational(t, 2, 1)
		// The later unenumerated mover cannot erase the earlier contained backup
		// departure at 4, and its causal gap is irrelevant to this one-sided proof.
		content.children[3].header.Declaration = Evidence[DeclarationID]{State: EvidenceInvalid, Reason: "later-invalid", Cause: &DecisionRef{Rule: RuleKey{"cause", 1}, Subject: NodeRef{"d", 50}}}
		_, d, err := barlineLocationBoundary(context.Background(), s, service, b, content, at, location)
		want := OutcomeFail
		if location == "middle" {
			want = OutcomePass
		}
		if err != nil || d.Outcome != want {
			t.Fatalf("%s: %+v %v", location, d, err)
		}
	}
}
func TestBarlineLocationRelevantGapsKeepCause(t *testing.T) {
	for _, kind := range []string{"enumeration", "declaration", "duration", "units", "branch"} {
		t.Run(kind, func(t *testing.T) {
			s, service, b, content, at := barlineBoundaryFixture(t, Budgets{})
			defer s.cleanup()
			cause := &DecisionRef{Rule: RuleKey{"cause", 1}, Subject: NodeRef{"d", 40}}
			gap := Evidence[bool]{State: EvidenceInvalid, Reason: "relevant-invalid", Cause: cause}
			switch kind {
			case "enumeration":
				content.end.ChildrenComplete = gap
			case "declaration":
				content.children[2].header.Declaration = stateUnavailable[bool, DeclarationID](gap)
			case "duration":
				f := service.Facts[40]
				f.Duration = stateUnavailable[bool, Rational](gap)
				service.Facts[40] = f
			case "units":
				f := service.Facts[40]
				f.Units = stateUnavailable[bool, Decimal](gap)
				service.Facts[40] = f
			case "branch":
				content.children[2].end.ChildrenComplete = gap
			}
			got, decision, err := barlineLocationBoundary(context.Background(), s, service, b, content, at, "right")
			if err != nil || decision.Outcome != "" || got.State != EvidenceInvalid || got.Reason != gap.Reason || !reflect.DeepEqual(got.Cause, cause) {
				t.Fatalf("lost cause: %+v %+v %v", got, decision, err)
			}
		})
	}
}
func TestBarlineLocationBarlinesCannotSupplyEndpointWitness(t *testing.T) {
	s, service, b, content, at := barlineBoundaryFixture(t, Budgets{})
	defer s.cleanup()
	// Remove the initial long voice and its independent backup departure. The
	// shared provider's deliberately huge bound and the barline at 4 prove no end.
	content.children = content.children[2:]
	f := service.Facts[40]
	f.Origin = stateKnown(statePosition(t, 0, 0, 1))
	service.Facts[40] = f
	got, d, err := barlineLocationBoundary(context.Background(), s, service, b, content, at, "right")
	if err != nil || d.Outcome != "" || got.State != EvidenceUnknown || got.Reason != "barline-measure-end-unproved-excursion" {
		t.Fatalf("circular witness: %+v %+v %v", got, d, err)
	}
}
func TestBarlineLocationKnownFailureRetainedWhenDiagnosticsExhausted(t *testing.T) {
	s, service, b, content, at := barlineBoundaryFixture(t, Budgets{MaxDiagnosticBytes: 1})
	defer s.cleanup()
	at.Offset = mustRational(t, 2, 1)
	_, d, err := barlineLocationBoundary(context.Background(), s, service, b, content, at, "right")
	if err != nil || d.Outcome != OutcomeFail {
		t.Fatalf("proof=%+v %v", d, err)
	}
	rule := RuleDefinition{Key: RuleKey{"barline", 1}, Category: CategoryNormative, Class: ClassMandatory, Role: RolePredicate}
	accumulator := newReportAccumulator(Profile{Target: TargetSource}, s.account)
	if err := accumulator.record(rule, d, Location{Target: TargetSource}); err != nil {
		t.Fatal(err)
	}
	if accumulator.report.Counts.Fail != 1 || !accumulator.report.DiagnosticsTruncated || len(accumulator.report.Findings) != 0 {
		t.Fatalf("lost failure: %+v", accumulator.report)
	}
}

func TestBarlineLocationOmittedMetadataAndInvalidSuffix(t *testing.T) {
	s, service, b, content, at := barlineBoundaryFixture(t, Budgets{})
	defer s.cleanup()
	// A fully enumerated event without its timeline fact is unavailable, never
	// a fabricated zero-length event or a normative position mismatch.
	metadata := stateNode(b, timelineRoleDirection, 70, stateAbsent[Atomic]())
	content.children = append(content.children, metadata)
	got, d, err := barlineLocationBoundary(context.Background(), s, service, b, content, at, "right")
	if err != nil || d.Outcome != "" || got.State != EvidenceUnknown || got.Reason != "barline-event-timing-unavailable" {
		t.Fatalf("omitted fact: %+v %+v %v", got, d, err)
	}
	key := RuleKey{"timeline", 1}
	service.Playback.Barlines = map[NodeID]barlinePlaybackContext{60: {Location: stateKnown("left")}}
	f := service.Facts[60]
	f.Origin = stateKnown(statePosition(t, 0, 0, 1))
	service.Facts[60] = f
	s.timeline = map[RuleKey]*timelineService{key: service}
	decision, pending, err := evaluateBarlineLocation(context.Background(), s, content.children[4], &contextPlan{Timeline: key}, true)
	if err != nil || pending || decision.Outcome != OutcomePass {
		t.Fatalf("independent left poisoned: %+v %v", decision, err)
	}
}

func TestBarlineLocationMalformedKnownRationalIsUnavailable(t *testing.T) {
	s, service, _, content, _ := barlineBoundaryFixture(t, Budgets{})
	defer s.cleanup()
	key := RuleKey{"timeline", 1}
	service.Playback.Barlines = map[NodeID]barlinePlaybackContext{60: {Location: stateKnown("left")}}
	f := service.Facts[60]
	f.Origin.Value.Offset = Rational{}
	service.Facts[60] = f
	s.timeline = map[RuleKey]*timelineService{key: service}
	d, pending, err := evaluateBarlineLocation(context.Background(), s, content.children[4], &contextPlan{Timeline: key}, true)
	if err != nil || pending || d.Outcome != OutcomeUnknown {
		t.Fatalf("malformed known: %+v %v", d, err)
	}
}

func TestBarlineLocationBufferedAdmissionAndFailureRetention(t *testing.T) {
	run := func(limit int64, fail bool) (*Session, Decision, error) {
		s, service, b, content, at := barlineBoundaryFixture(t, Budgets{MaxBufferedBytes: limit})
		if fail {
			at.Offset = mustRational(t, 2, 1)
		}
		end, d, err := barlineLocationBoundary(context.Background(), s, service, b, content, at, "right")
		if err == nil && !fail && end.State != EvidenceKnown {
			t.Fatalf("boundary=%+v", end)
		}
		return s, d, err
	}
	for _, failure := range []bool{false, true} {
		low, high := int64(1), int64(1<<16)
		for low < high {
			mid := (low + high) / 2
			s, _, err := run(mid, failure)
			s.cleanup()
			if err == nil {
				high = mid
			} else if errors.Is(err, ErrResourceLimit) {
				low = mid + 1
			} else {
				t.Fatal(err)
			}
		}
		for _, delta := range []int64{-1, 0} {
			s, d, err := run(low+delta, failure)
			if (err == nil) != (delta == 0) {
				t.Fatalf("failure=%v bytes=%d err=%v", failure, low+delta, err)
			}
			if err == nil && failure {
				rule := RuleDefinition{Key: RuleKey{"barline", 1}, Category: CategoryNormative, Class: ClassMandatory, Role: RolePredicate}
				s.accumulator = newReportAccumulator(Profile{Target: TargetSource}, s.account)
				s.decisions = map[decisionID]Decision{}
				// Reserve all remaining room so dependency-cache publication is refused
				// after the already-established failure has been counted.
				remaining := s.account.Limits().MaxBufferedBytes - rationalUsage(t, s, BudgetBufferedBytes)
				if e := s.retain(remaining); e != nil {
					t.Fatal(e)
				}
				e := s.record(rule, &factNode{header: NodeHeader{ID: 60}}, d)
				if !errors.Is(e, ErrResourceLimit) || s.accumulator.report.Counts.Fail != 1 {
					t.Fatalf("failure lost during cache exhaustion: %v %+v", e, s.accumulator.report)
				}
			}
			if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
				t.Fatal("buffer scratch leak")
			}
			s.cleanup()
			if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatal("cleanup leak")
			}
		}
	}
}

func TestBarlineLocationPlanRequiresExactNodeAndTimelineContract(t *testing.T) {
	schema := localSchema(t)
	key := RuleKey{"timeline", 1}
	for _, spec := range []ContextPlanSpec{{}, {Schema: schema}, {TimelineProvider: key}} {
		if _, err := NewBarlineLocationPlan(spec); !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("accepted incomplete spec: %+v %v", spec, err)
		}
	}
	// Full-schema constructor success is exercised by both adapter catalogs.
	// This miniature schema supplies only the sealed contract test roles.
	plan := Plan{kind: planBarlineLocation, context: &contextPlan{Schema: schema, Timeline: key, Bindings: contextCoreBindings(t), revision: contextSemanticRevision}}
	typ := contextNamedType("barline")
	rule := RuleDefinition{Key: RuleKey{"barline", 1}, Class: ClassMandatory, Role: RolePredicate, Category: CategoryNormative, Targets: []Target{TargetSource, TargetModel}, Scope: ScopeNode, Selector: Selector{Component: &typ}, Requires: []Dependency{{Kind: DependencyRule, Rule: key, Relation: RelationDocument}}}
	if err := plan.validateContextFor(rule); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*RuleDefinition){func(r *RuleDefinition) { r.Scope = ScopeDocument }, func(r *RuleDefinition) { r.Role = RoleContextProvider }, func(r *RuleDefinition) { x := contextNamedType("note"); r.Selector.Component = &x }, func(r *RuleDefinition) { r.Requires = nil }} {
		bad := rule
		mutate(&bad)
		if err := plan.validateContextFor(bad); !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("accepted bad rule: %+v %v", bad, err)
		}
	}
}
