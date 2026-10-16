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
	second := runtimeNote(t, b, 40, 2, false)
	forward := stateNode(b, timelineRoleForward, 50, stateAbsent[Atomic](), stateNode(b, timelineRoleForwardDuration, 51, stateAtomicDecimal(t, 2, 1)))
	bar := stateNode(b, timelineRoleBarline, 60, stateAbsent[Atomic]())
	content := stateNode(b, timelineRoleMeasure, 10, stateAbsent[Atomic](), first, backup, second, forward, bar)
	service := &timelineService{Final: true, Facts: map[NodeID]timelineEventFact{}}
	for _, row := range []struct {
		id                   NodeID
		onset, duration, end int64
	}{{20, 0, 4, 4}, {30, 4, 4, 0}, {40, 0, 2, 2}, {50, 2, 2, 4}, {60, 4, 0, 4}} {
		onset, end := statePosition(t, 0, row.onset, 1), statePosition(t, 0, row.end, 1)
		end.UnprovedFrame = true
		service.Facts[row.id] = timelineEventFact{Origin: stateKnown(onset), End: stateKnown(end), Duration: stateKnown(mustRational(t, row.duration, 1)), Units: stateKnown(stateDecimal(t, 1, 1)), Principal: stateKnown(NodeRef{"d", row.id}), PrincipalDuration: stateKnown(mustRational(t, row.duration, 1))}
	}
	key := RuleKey{"timeline", 1}
	s.catalog = &CompiledCatalog{rules: map[RuleKey]RuleDefinition{key: {Key: key, Predicate: "timeline"}}, plans: map[string]Plan{"timeline": {timeline: &timelinePlan{Bindings: b}}}}
	s.nodes = map[NodeID]*factNode{content.header.ID: content}
	service.State.frameBounds = map[timelineFrame]Rational{frameOf(statePosition(t, 0, 0, 1)): mustRational(t, 99, 1)}
	return s, service, b, content, statePosition(t, 0, 4, 1)
}
func barlineBoundaryMakeInterior(t *testing.T, service *timelineService, content *factNode) musicalPosition {
	t.Helper()
	content.children[3], content.children[4] = content.children[4], content.children[3]
	at := statePosition(t, 0, 2, 1)
	fact := service.Facts[60]
	fact.Origin, fact.End = stateKnown(at), stateKnown(at)
	service.Facts[60] = fact
	return at
}

func TestBarlineLocationCertificateDoesNotStrengthenTimeline(t *testing.T) {
	s, service, b, content, at := barlineBoundaryFixture(t, Budgets{})
	defer s.cleanup()
	original := service.Facts[20]
	got, d, err := barlineLocationBoundary(context.Background(), s, service, b, content, 60, at, "right")
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
	if _, _, err := barlineLocationBoundary(counter, s, service, b, content, 60, at, "right"); err != nil {
		t.Fatal(err)
	}
	work := rationalUsage(t, s, BudgetRuleSteps)
	s.cleanup()
	for _, delta := range []int64{-1, 0} {
		s, service, b, content, at := barlineBoundaryFixture(t, Budgets{MaxRuleSteps: work + delta})
		got, d, err := barlineLocationBoundary(context.Background(), s, service, b, content, 60, at, "right")
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
		got, d, err := barlineLocationBoundary(&rationalCancelContext{Context: context.Background(), at: stop}, s, service, b, content, 60, at, "right")
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
		at = barlineBoundaryMakeInterior(t, service, content)
		// The later unenumerated mover cannot erase the earlier contained backup
		// departure at 4, and its causal gap is irrelevant to this one-sided proof.
		unknown := stateNode(b, timelineRoleForward, 70, stateAbsent[Atomic]())
		unknown.header.Declaration = Evidence[DeclarationID]{State: EvidenceInvalid, Reason: "later-invalid", Cause: &DecisionRef{Rule: RuleKey{"cause", 1}, Subject: NodeRef{"d", 70}}}
		content.children = append(content.children, unknown)
		_, d, err := barlineLocationBoundary(context.Background(), s, service, b, content, 60, at, location)
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
			got, decision, err := barlineLocationBoundary(context.Background(), s, service, b, content, 60, at, "right")
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
	got, d, err := barlineLocationBoundary(context.Background(), s, service, b, content, 60, at, "right")
	if err != nil || d.Outcome != "" || got.State != EvidenceUnknown || got.Reason != "barline-measure-end-unproved-excursion" {
		t.Fatalf("circular witness: %+v %+v %v", got, d, err)
	}
}
func TestBarlineLocationKnownFailureRetainedWhenDiagnosticsExhausted(t *testing.T) {
	s, service, b, content, at := barlineBoundaryFixture(t, Budgets{MaxDiagnosticBytes: 1})
	defer s.cleanup()
	at = barlineBoundaryMakeInterior(t, service, content)
	_, d, err := barlineLocationBoundary(context.Background(), s, service, b, content, 60, at, "right")
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
	got, d, err := barlineLocationBoundary(context.Background(), s, service, b, content, 60, at, "right")
	if err != nil || d.Outcome != "" || got.State != EvidenceUnknown || got.Reason != "barline-event-timing-unavailable" {
		t.Fatalf("omitted fact: %+v %+v %v", got, d, err)
	}
	key := RuleKey{"timeline", 1}
	service.Playback.Barlines = map[NodeID]barlinePlaybackContext{60: {Location: stateKnown("left")}}
	f := service.Facts[60]
	f.Origin = stateKnown(statePosition(t, 0, 0, 1))
	service.Facts[60] = f
	s.timeline = map[RuleKey]*timelineService{key: service}
	left := content.children[4]
	content.children = append([]*factNode{left}, append(content.children[:4], content.children[5:]...)...)
	decision, pending, err := evaluateBarlineLocation(context.Background(), s, left, &contextPlan{Timeline: key}, true)
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
			at = barlineBoundaryMakeInterior(t, service, content)
		}
		end, d, err := barlineLocationBoundary(context.Background(), s, service, b, content, 60, at, "right")
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

func TestBarlineLocationUnknownMoverBlocksEndButNotKnownLeft(t *testing.T) {
	s, service, b, content, at := barlineBoundaryFixture(t, Budgets{})
	defer s.cleanup()
	cause := &DecisionRef{Rule: RuleKey{"unavailable-mover", 1}, Subject: NodeRef{"d", 50}}
	content.children[3].header.Declaration = Evidence[DeclarationID]{State: EvidenceInvalid, Reason: "unavailable-mover", Cause: cause}
	left := stateNode(b, timelineRoleBarline, 70, stateAbsent[Atomic]())
	left.header.Parent = content.header.ID
	content.children = append([]*factNode{left}, content.children...)
	service.Facts[70] = timelineEventFact{Origin: stateKnown(statePosition(t, 0, 0, 1))}
	got, d, err := barlineLocationBoundary(context.Background(), s, service, b, content, 60, at, "right")
	if err != nil || d.Outcome != "" || got.State != EvidenceInvalid || !reflect.DeepEqual(got.Cause, cause) {
		t.Fatalf("end lost mover cause: %+v %+v %v", got, d, err)
	}
	key := RuleKey{"timeline", 1}
	s.timeline = map[RuleKey]*timelineService{key: service}
	service.Playback.Barlines = map[NodeID]barlinePlaybackContext{70: {Location: stateKnown("left")}}
	decision, pending, err := evaluateBarlineLocation(context.Background(), s, left, &contextPlan{Timeline: key}, true)
	if err != nil || pending || decision.Outcome != OutcomePass {
		t.Fatalf("known earlier left poisoned: %+v %v", decision, err)
	}
}

func TestBarlineLocationMovementSideEvidence(t *testing.T) {
	for _, before := range []bool{true, false} {
		for _, kind := range []string{"metadata", "note", "backup", "forward", "unknown-declaration", "invalid-declaration", "missing-subject", "incomplete", "unordered"} {
			t.Run(map[bool]string{true: "before/", false: "after/"}[before]+kind, func(t *testing.T) {
				s, _, b, content, _ := barlineBoundaryFixture(t, Budgets{})
				defer s.cleanup()
				bar := content.children[4]
				event := stateNode(b, timelineRoleAttributes, 70, stateAbsent[Atomic]())
				want, state := false, EvidenceKnown
				cause := &DecisionRef{Rule: RuleKey{"cause", 1}, Subject: NodeRef{"d", 70}}
				switch kind {
				case "note", "backup", "forward":
					role := map[string]timelineRole{"note": timelineRoleNote, "backup": timelineRoleBackup, "forward": timelineRoleForward}[kind]
					event = stateNode(b, role, 70, stateAbsent[Atomic]())
					want = true
				case "unknown-declaration":
					event.header.Declaration = stateUnknown[DeclarationID]("unavailable-declaration")
					state = EvidenceUnknown
				case "invalid-declaration":
					event.header.Declaration = Evidence[DeclarationID]{State: EvidenceInvalid, Reason: "invalid-declaration", Cause: cause}
					state = EvidenceInvalid
				case "missing-subject":
					bar = stateNode(b, timelineRoleBarline, 80, stateAbsent[Atomic]())
					state = EvidenceUnknown
				case "incomplete":
					content.end.ChildrenComplete = stateUnknown[bool]("incomplete-children")
					state = EvidenceUnknown
				case "unordered":
					content.end.Order = stateKnown(OrderModelFields)
					state = EvidenceUnknown
				}
				content.children = []*factNode{bar, event}
				if before {
					content.children[0], content.children[1] = content.children[1], content.children[0]
				}
				got, err := barlineMovementSide(context.Background(), s, b, content, 60, before)
				if err != nil || got.State != state || state == EvidenceKnown && got.Value != want {
					t.Fatalf("side=%+v want=%s/%v err=%v", got, state, want, err)
				}
				if kind == "invalid-declaration" && !reflect.DeepEqual(got.Cause, cause) {
					t.Fatalf("lost declaration cause: %+v", got)
				}
			})
		}
	}
}

func TestBarlineLocationMovementSideBudgetsCancellationAndNonmutation(t *testing.T) {
	for _, before := range []bool{true, false} {
		run := func(ctx context.Context, budget Budgets) (*Session, Evidence[bool], error) {
			s, service, b, content, _ := barlineBoundaryFixture(t, budget)
			original := append([]*factNode(nil), content.children...)
			fact := service.Facts[60]
			got, err := barlineMovementSide(ctx, s, b, content, 60, before)
			if !reflect.DeepEqual(original, content.children) || !reflect.DeepEqual(fact, service.Facts[60]) {
				t.Fatal("ordered placement check mutated input evidence")
			}
			if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
				t.Fatal("placement check retained scratch")
			}
			return s, got, err
		}
		counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
		s, got, err := run(counter, Budgets{MaxBufferedBytes: 1})
		if err != nil || got.State != EvidenceKnown || got.Value != before {
			t.Fatalf("side=%+v err=%v", got, err)
		}
		work := rationalUsage(t, s, BudgetRuleSteps)
		s.cleanup()
		for _, delta := range []int64{-1, 0} {
			s, got, err := run(context.Background(), Budgets{MaxRuleSteps: work + delta, MaxBufferedBytes: 1})
			if delta < 0 && (!errors.Is(err, ErrResourceLimit) || got.State == EvidenceKnown) || delta == 0 && (err != nil || got.State != EvidenceKnown) {
				t.Fatalf("quota delta=%d side=%+v err=%v", delta, got, err)
			}
			s.cleanup()
		}
		for stop := 1; stop <= counter.calls; stop++ {
			s, got, err := run(&rationalCancelContext{Context: context.Background(), at: stop}, Budgets{})
			if !errors.Is(err, context.Canceled) || got.State == EvidenceKnown {
				t.Fatalf("cancellation poll=%d side=%+v err=%v", stop, got, err)
			}
			s.cleanup()
			if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatal("placement cleanup leak")
			}
		}
	}
}

func TestBarlineLocationTerminalCursorBelowCertifiedEndIsUnknown(t *testing.T) {
	s, service, b, content, at := barlineBoundaryFixture(t, Budgets{})
	defer s.cleanup()
	// Defensively admitted private evidence can put the terminal barline below
	// an otherwise certified endpoint. Neither the lower-bound shortcut nor
	// the final endpoint comparison may manufacture a right-location violation.
	at.Offset = mustRational(t, 2, 1)
	fact := service.Facts[60]
	fact.Origin = stateKnown(at)
	service.Facts[60] = fact
	got, decision, err := barlineLocationBoundary(context.Background(), s, service, b, content, 60, at, "right")
	if err != nil || got.State != EvidenceUnknown || got.Reason != "barline-measure-end-unequal-voices" || decision.Outcome != "" {
		t.Fatalf("terminal cursor compared as a violation: %+v %+v %v", got, decision, err)
	}
}
