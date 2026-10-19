package validation

import (
	"context"
	"errors"
	"testing"
)

func TestContextProjectionOwnedAndCleanup(t *testing.T) {
	s := rationalSession(t, Budgets{})
	result := ContextResult{Provider: RuleKey{"context-test", 1}, Subject: NodeRef{"score", 7}, Property: ContextPropertySpanNormalDots, Value: Evidence[Atomic]{State: EvidenceUnknown, Reason: "missing-field", Cause: &DecisionRef{Rule: RuleKey{"cause", 1}, Subject: NodeRef{"score", 9}}}}
	// Direct publication exercises ownership independently of family semantics.
	if err := s.publishContextResult(context.Background(), result); err != nil {
		t.Fatal(err)
	}
	result.Value.Cause.Rule.ID = "changed"
	if len(s.contextResults) != 1 || s.contextResults[0].Value.Cause.Rule.ID != "cause" {
		t.Fatal("projection aliases input")
	}
	owned := s.contextResults
	s.cleanup()
	if s.contextResults != nil || owned[0].Subject.Node != 7 {
		t.Fatal("cleanup lost result or retained service")
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("leaked %d bytes", used)
	}
}
func TestContextProjectionCanceledPublication(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.publishContextResult(ctx, ContextResult{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	if len(s.contextResults) != 0 || s.retained != 0 {
		t.Fatal("partial publication")
	}
}
func TestContextProjectionQuotaBeforeAllocation(t *testing.T) {
	s := rationalSession(t, Budgets{MaxBufferedBytes: 1})
	defer s.cleanup()
	if err := s.publishContextResult(context.Background(), ContextResult{Provider: RuleKey{"context-test", 1}, Subject: NodeRef{"score", 7}, Property: ContextPropertySpanNormalDots, Value: Evidence[Atomic]{State: EvidenceAbsent}}); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("got %v", err)
	}
	if len(s.contextResults) != 0 || cap(s.contextResults) != 0 || s.retained != 0 {
		t.Fatal("allocated before admission")
	}
}
func TestContextPlanRejectsInvalidSchema(t *testing.T) {
	if _, err := NewLocalContextPlan(nil); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("got %v", err)
	}
	if _, err := NewSpanContextPlan(ContextPlanSpec{}); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("got %v", err)
	}
}

func contextCoreBindings(t *testing.T) *contextBindings {
	t.Helper()
	b, err := compileContextBindingSpecs(localSchema(t), []contextBindingSpec{
		{Role: "core-beam", Owner: "note", Name: "beam", Type: "beam", Count: 1},
		{Role: "core-number", Owner: "beam", Name: "number", Type: "xs:integer", Attribute: true, Count: 1},
		{Role: "core-all-beam", Name: "beam", Type: "beam", Count: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func TestContextBindingExactRolesAndWrongShape(t *testing.T) {
	b := contextCoreBindings(t)
	if len(b.Roles["core-beam"]) != 1 || len(b.Attributes["core-number"]) != 1 || b.Roles["core-all-beam"][0].Declaration != b.Roles["core-beam"][0].Declaration {
		t.Fatal("exact role mismatch")
	}
	for _, spec := range []contextBindingSpec{
		{Role: "wrong", Owner: "note", Name: "beam", Type: "slur", Count: 1},
		{Role: "wrong", Owner: "note", Name: "beam", Type: "beam", Count: 2},
		{Role: "wrong", Name: "absent", Type: "beam", Count: -1},
	} {
		if _, err := compileContextBindingSpecs(localSchema(t), []contextBindingSpec{spec}); !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("accepted wrong shape %+v: %v", spec, err)
		}
	}
}
func TestContextReaderExactOwnerPresenceAndInterruption(t *testing.T) {
	b := contextCoreBindings(t)
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	n := localNode(1, 0, "beam")
	n.header.Type = testEvidence(Evidence[ComponentID]{State: EvidenceKnown, Value: contextNamedType("beam")})
	n.header.Declaration = testEvidence(Evidence[DeclarationID]{State: EvidenceKnown, Value: b.Roles["core-beam"][0].Declaration})
	r := &contextReader{ctx: context.Background(), s: s, b: b}
	if !r.matches(n, "core-beam") || !r.attributeOwner(n, "core-number") {
		t.Fatal("exact roles not selected")
	}
	if e := r.presence(n, "core-number"); e.State != EvidenceAbsent {
		t.Fatalf("omission=%+v", e)
	}
	n.header.Type.Value = contextNamedType("slur")
	if r.attributeOwner(n, "core-number") {
		t.Fatal("local name fallback selected wrong owner")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r.ctx = ctx
	_ = r.attribute(n, "core-number")
	if !errors.Is(r.err, context.Canceled) {
		t.Fatalf("lost cancellation: %v", r.err)
	}
}
func TestContextProjectionEveryCancellationBoundary(t *testing.T) {
	result := ContextResult{Provider: RuleKey{"context-test", 1}, Subject: NodeRef{"score", 7}, Property: ContextPropertySpanNormalDots, Value: Evidence[Atomic]{State: EvidenceUnknown, Reason: "missing-field", Cause: &DecisionRef{Rule: RuleKey{"cause", 1}, Subject: NodeRef{"score", 9}}}, Origins: []Location{{Target: TargetSource, DocumentID: "score", Path: "/note", Position: &SourcePosition{Line: 1, Column: 2, CoordinateSpace: "xml"}}}}
	probe := rationalSession(t, Budgets{})
	ctx := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	if err := probe.publishContextResult(ctx, result); err != nil {
		t.Fatal(err)
	}
	calls := ctx.calls
	probe.cleanup()
	for at := 1; at <= calls; at++ {
		s := rationalSession(t, Budgets{})
		c := &rationalCancelContext{Context: context.Background(), at: at}
		err := s.publishContextResult(c, result)
		if !errors.Is(err, context.Canceled) || len(s.contextResults) != 0 || s.retained != 0 {
			t.Fatalf("at %d: err=%v results=%d retained=%d", at, err, len(s.contextResults), s.retained)
		}
		if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("at %d leaked %d", at, used)
		}
		s.cleanup()
	}
}
func TestContextReportSurvivesFinishAndOptionalGap(t *testing.T) {
	ctx := context.Background()
	s, err := sourceSchemaEngine(t, XSDCheckScalar).Begin(ctx, RunOptions{DocumentID: "source", Profile: "source-schema-test"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.OpenNode(ctx, schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	if err = s.CloseNode(ctx, schemaLexemeEnd("3")); err != nil {
		t.Fatal(err)
	}
	result := ContextResult{Provider: RuleKey{"context-test", 1}, Subject: NodeRef{"source", 1}, Property: ContextPropertySpanNormalDots, Value: Evidence[Atomic]{State: EvidenceUnknown, Reason: "optional-geometry"}}
	if err = s.publishContextResult(ctx, result); err != nil {
		t.Fatal(err)
	}
	report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Context) != 1 || report.Context[0].Value.Reason != "optional-geometry" || !report.Satisfied() {
		t.Fatalf("projection/completeness=%+v", report)
	}
	if s.contextResults != nil || s.contextServices != nil {
		t.Fatal("session retained projection")
	}
	if _, err = s.Finish(ctx, Completion{}); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("repeat Finish=%v", err)
	}
}
func TestContextTimelineDependencyIsFieldLocal(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	n := localNode(1, 0, "root")
	timeline := RuleKey{"timeline", 1}
	other := RuleKey{"other", 1}
	key := RuleKey{"context", 1}
	cause := &DecisionRef{Rule: RuleKey{"source", 1}, Subject: NodeRef{"doc", 2}}
	s.documentPrerequisites = map[RuleKey]*documentPrerequisite{
		timeline: {finalized: true, ready: true, decision: Decision{Outcome: OutcomeBlocked, Reason: "invalid-time", Cause: cause}},
		other:    {finalized: true, ready: true, decision: Decision{Outcome: OutcomeUnknown, Reason: "other-missing"}},
	}
	p := Plan{kind: planContextSpans, context: &contextPlan{Timeline: timeline}}
	r := RuleDefinition{Key: key, Requires: []Dependency{{Kind: DependencyRule, Rule: timeline, Relation: RelationDocument}}}
	d, pending, err := s.contextDependencies(context.Background(), n, r, p, true)
	if err != nil || pending || d.Outcome != OutcomePass {
		t.Fatalf("local dependency blocked: %+v,%v,%v", d, pending, err)
	}
	fact := s.contextServices[key].Timeline
	if fact.State != EvidenceInvalid || fact.Cause == nil || *fact.Cause != *cause || fact.Cause == cause {
		t.Fatal("timeline cause lost or aliased")
	}
	r.Requires = append(r.Requires, Dependency{Kind: DependencyRule, Rule: other, Relation: RelationDocument})
	d, pending, err = s.contextDependencies(context.Background(), n, r, p, true)
	if err != nil || pending || d.Outcome != OutcomeUnknown || d.Reason != "other-missing" {
		t.Fatalf("unrelated dependency bypassed: %+v,%v,%v", d, pending, err)
	}
}
func TestContextProjectionWorkExcludesAllocationHeadroom(t *testing.T) {
	s := rationalSession(t, Budgets{MaxRuleSteps: 128})
	defer s.cleanup()
	if err := s.publishContextResult(context.Background(), ContextResult{Provider: RuleKey{"p", 1}, Subject: NodeRef{"d", 1}, Property: ContextPropertySpanNormalDots, Value: Evidence[Atomic]{State: EvidenceAbsent}}); err != nil {
		t.Fatalf("fixed allocator reserve charged as work: %v", err)
	}
}

func TestContextBendConsumerExactProviderContract(t *testing.T) {
	schema := localSchema(t)
	b := contextCoreBindings(t)
	provider := RuleKey{"performance", 1}
	bend := contextNamedType("bend")
	p := Plan{kind: planBendReleaseSign, context: &contextPlan{Schema: schema, Provider: provider, Bindings: b, revision: contextSemanticRevision}}
	r := RuleDefinition{Key: RuleKey{"bend", 1}, Class: ClassMandatory, Role: RolePredicate, Category: CategoryNormative, Targets: []Target{TargetSource, TargetModel}, Scope: ScopeNode, Selector: Selector{Component: &bend}, Requires: []Dependency{{Kind: DependencyRule, Rule: provider, Relation: RelationDocument}}}
	c := &CompiledCatalog{rules: map[RuleKey]RuleDefinition{provider: {Key: provider, Predicate: "performance"}}, plans: map[string]Plan{"performance": {kind: planContextPerformance, context: &contextPlan{Schema: schema, Bindings: b, revision: contextSemanticRevision}}}}
	if err := p.validateContextRules(r, c, map[RuleKey]bool{provider: true}); err != nil {
		t.Fatal(err)
	}
	c.plans["performance"] = Plan{kind: planContextLayout, context: &contextPlan{Schema: schema, Bindings: b, revision: contextSemanticRevision}}
	if err := p.validateContextRules(r, c, map[RuleKey]bool{provider: true}); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("accepted wrong provider kind: %v", err)
	}
	if err := p.validateContextRules(r, c, map[RuleKey]bool{}); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("accepted unselected provider: %v", err)
	}
	r.Requires = nil
	if err := p.validateContextFor(r); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("accepted missing dependency: %v", err)
	}
}
func TestContextBindingCacheFailureIsOwned(t *testing.T) {
	schema := miniatureSourceSchema(t)
	_, first := schema.sharedContextBindings()
	if first == nil {
		t.Fatal("incomplete schema accepted")
	}
	e, ok := first.(*ConfigurationError)
	if !ok {
		t.Fatalf("wrong error %T", first)
	}
	e.Reason = "caller mutation"
	_, second := schema.sharedContextBindings()
	copy, ok := second.(*ConfigurationError)
	if !ok || copy == e || copy.Reason == "caller mutation" {
		t.Fatalf("memoized error exposed: %v", second)
	}
	if schema.snapshot().context != nil {
		t.Fatal("mutable compilation cache retained in snapshot")
	}
}

func TestContextGraphicalRepeatConsumerExactProviderContract(t *testing.T) {
	schema := localSchema(t)
	b := contextCoreBindings(t)
	provider := RuleKey{"repeats", 1}
	for _, name := range []string{"beat-repeat", "measure-repeat"} {
		typ := contextNamedType(name)
		p := Plan{kind: planGraphicalRepeatContent, context: &contextPlan{Schema: schema, Provider: provider, Bindings: b, revision: contextSemanticRevision}}
		r := RuleDefinition{Key: RuleKey{"content", 1}, Class: ClassMandatory, Role: RolePredicate, Category: CategoryNormative, Targets: []Target{TargetSource, TargetModel}, Scope: ScopeNode, Selector: Selector{Component: &typ}, Requires: []Dependency{{Kind: DependencyRule, Rule: provider, Relation: RelationDocument}}}
		c := &CompiledCatalog{rules: map[RuleKey]RuleDefinition{provider: {Key: provider, Predicate: "repeats"}}, plans: map[string]Plan{"repeats": {kind: planContextRepeats, context: &contextPlan{Schema: schema, Bindings: b, revision: contextSemanticRevision}}}}
		if err := p.validateContextRules(r, c, map[RuleKey]bool{provider: true}); err != nil {
			t.Fatal(err)
		}
		c.plans["repeats"] = Plan{kind: planContextNavigation, context: &contextPlan{Schema: schema, Bindings: b, revision: contextSemanticRevision}}
		if err := p.validateContextRules(r, c, map[RuleKey]bool{provider: true}); !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("wrong provider accepted: %v", err)
		}
	}
}

func TestContextFigureNumberRequiresExactDeclaration(t *testing.T) {
	schema := localSchema(t)
	b := contextCoreBindings(t)
	use := b.Roles["core-beam"][0]
	b.Roles[contextRoleFigureNumber] = []SchemaUse{use}
	p := Plan{kind: planFigureNumberNumeric, context: &contextPlan{Schema: schema, Bindings: b, revision: contextSemanticRevision}}
	d := use.Declaration
	r := RuleDefinition{Key: RuleKey{"numeric", 1}, Class: ClassMandatory, Role: RolePredicate, Category: CategoryNormative, Targets: []Target{TargetSource, TargetModel}, Scope: ScopeNode, Selector: Selector{Declaration: &d}}
	if err := p.validateContextFor(r); err != nil {
		t.Fatal(err)
	}
	typ := contextNamedType("style-text")
	r.Selector = Selector{Component: &typ}
	if err := p.validateContextFor(r); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("broad style-text selector accepted: %v", err)
	}
	r.Selector = Selector{Declaration: &d, Component: &typ}
	if err := p.validateContextFor(r); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("mixed selector accepted: %v", err)
	}
}

func TestContextProjectionRejectsMismatchedPayload(t *testing.T) {
	for _, in := range []ContextResult{
		{Property: ContextPropertySpan},
		{Property: ContextPropertySpan, Timing: &TimingContextResult{}},
		{Property: ContextPropertyTiming, Span: &SpanContextResult{}},
		{Property: ContextPropertySpanNormalDots, Span: &SpanContextResult{}},
		{Property: ContextPrintObject},
	} {
		s := rationalSession(t, Budgets{})
		in.Provider = RuleKey{"p", 1}
		in.Subject = NodeRef{"d", 1}
		err := s.publishContextResult(context.Background(), in)
		if !errors.Is(err, ErrInvalidConfiguration) {
			t.Errorf("property %s accepted invalid payload: %v", in.Property, err)
		}
		if len(s.contextResults) != 0 {
			t.Error("invalid payload published")
		}
		s.cleanup()
	}
}
