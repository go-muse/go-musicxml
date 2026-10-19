package validation

import (
	"context"
	"errors"
	"testing"
)

func ciSession(t *testing.T, limits Budgets) *Session {
	t.Helper()
	a, err := NewBudgetAccount(limits)
	if err != nil {
		t.Fatal(err)
	}
	return &Session{documentID: "doc", profile: Profile{Target: TargetSource}, account: a,
		contexts: map[ContextKey]ContextFact{}, identities: map[string]*identityIndex{}, registered: map[decisionID]bool{}, decisions: map[decisionID]Decision{}, nodes: map[NodeID]*factNode{}}
}
func ciRule(id string) RuleDefinition {
	return RuleDefinition{Key: RuleKey{ID: RuleID(id), Revision: 1}}
}
func ciNode(id, parent NodeID) *factNode {
	return &factNode{header: testRetainedHeader(factHeader(id, parent, "test.slot", "slot")), end: testRetainedEnd(factEnd(id)), closed: true}
}
func ciSelector() AttributeSelector {
	return AttributeSelector{Name: QName{Local: "id"}, Declaration: DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.slot", Occurrence: "test.id"}}
}
func ciStringNode(id NodeID, value string) *factNode {
	n := ciNode(id, 0)
	a := ciSelector()
	n.attributes = []AttributeFact{{Owner: id, Name: a.Name, Declaration: Evidence[DeclarationID]{State: EvidenceKnown, Value: a.Declaration}, Presence: Evidence[bool]{State: EvidenceKnown, Value: true}, Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceUnknown, Reason: "not-needed"}, Value: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: value}}}, Location: Location{Target: TargetSource, DocumentID: "doc", Path: "/slot/@id", Position: &SourcePosition{Line: int(id), Column: 1, CoordinateSpace: "source"}}}}
	return n
}
func ciEvalIdentity(t *testing.T, s *Session, n *factNode, r RuleDefinition, p Plan, final bool) (Decision, bool) {
	t.Helper()
	d, pending, err := evaluateIdentityPlan(context.Background(), s, n, r, p, final)
	if err != nil {
		t.Fatal(err)
	}
	return d, pending
}
func ciEvalContext(t *testing.T, s *Session, n *factNode, r RuleDefinition, p Plan, final bool) (Decision, bool) {
	t.Helper()
	d, pending, err := evaluateContextPlan(context.Background(), s, n, r, p, final)
	if err != nil {
		t.Fatal(err)
	}
	return d, pending
}

func TestIdentityForwardReferenceRequiresCompleteUniqueIndex(t *testing.T) {
	s := ciSession(t, Budgets{})
	declare, _ := NewIdentityDeclarationPlan("ids", ciSelector())
	reference, _ := NewIdentityReferencePlan("ids", ciSelector())
	if err := s.markIdentityProvider("ids"); err != nil {
		t.Fatal(err)
	}
	ref, decl := ciStringNode(1, "key"), ciStringNode(2, "key")
	if d, pending := ciEvalIdentity(t, s, ref, ciRule("reference"), reference, false); !pending {
		t.Fatalf("forward reference resolved early: %+v", d)
	}
	if d, pending := ciEvalIdentity(t, s, decl, ciRule("declaration"), declare, false); !pending {
		t.Fatalf("uniqueness claimed early: %+v", d)
	}
	if d, _ := ciEvalIdentity(t, s, ref, ciRule("reference"), reference, true); d.Outcome != OutcomeUnknown {
		t.Fatalf("incomplete index resolved: %+v", d)
	}
	s.inputComplete = true
	if d, pending := ciEvalIdentity(t, s, ref, ciRule("reference"), reference, true); pending || d.Outcome != OutcomePass {
		t.Fatalf("forward reference: %+v pending=%v", d, pending)
	}
	if d, _ := ciEvalIdentity(t, s, decl, ciRule("declaration"), declare, true); d.Outcome != OutcomePass {
		t.Fatalf("unique declaration: %+v", d)
	}
	used, _ := s.account.Usage(BudgetIDs)
	if used != 2 {
		t.Fatalf("registrations not one-time: %d", used)
	}
}

func TestIdentityDuplicateProofAndCausalAmbiguitySurviveIncompleteIndex(t *testing.T) {
	s := ciSession(t, Budgets{})
	declare, _ := NewIdentityDeclarationPlan("ids", ciSelector())
	reference, _ := NewIdentityReferencePlan("ids", ciSelector())
	r := ciRule("unique")
	first, second, ref := ciStringNode(1, "k"), ciStringNode(2, "k"), ciStringNode(3, "k")
	ciEvalIdentity(t, s, first, r, declare, false)
	// Another declaration plan bound to the same containing owner is not a duplicate.
	if d, pending := ciEvalIdentity(t, s, first, ciRule("same-owner"), declare, false); !pending || d.Outcome == OutcomeFail {
		t.Fatalf("same owner collision: %+v", d)
	}
	d, pending := ciEvalIdentity(t, s, second, r, declare, false)
	if pending || d.Outcome != OutcomeFail || len(d.Related) != 2 {
		t.Fatalf("duplicate not proven early: %+v", d)
	}
	if d.Subject != (NodeRef{DocumentID: "doc", Node: 2}) {
		t.Fatalf("bad duplicate subject: %+v", d)
	}
	s.markIdentityIncomplete("ids")
	d, pending = ciEvalIdentity(t, s, ref, ciRule("reference"), reference, true)
	want := DecisionRef{Rule: r.Key, Subject: NodeRef{DocumentID: "doc", Node: 2}}
	if pending || d.Outcome != OutcomeBlocked || d.Cause == nil || *d.Cause != want || len(d.Related) != 2 {
		t.Fatalf("ambiguous reference lost cause: %+v", d)
	}
	first.attributes[0].Location.Position.Line = 999
	if d.Related[0].Position.Line != 1 {
		t.Fatal("identity origins alias input")
	}
	if d, _ := ciEvalIdentity(t, s, first, r, declare, true); d.Outcome != OutcomeBlocked || d.Cause == nil || *d.Cause != want {
		t.Fatalf("original declaration claimed unique: %+v", d)
	}
}

func TestIdentityMissingTargetDependsOnCompleteProviderEnumeration(t *testing.T) {
	for _, mode := range []string{"complete", "prefix", "unavailable", "terminal", "unselected"} {
		t.Run(mode, func(t *testing.T) {
			s := ciSession(t, Budgets{})
			p, _ := NewIdentityReferencePlan("ids", ciSelector())
			s.inputComplete = mode != "prefix"
			if mode != "unselected" {
				if err := s.markIdentityProvider("ids"); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "unavailable" {
				s.markIdentityIncomplete("ids")
			}
			if mode == "terminal" {
				s.terminalCause = errors.New("transport failed")
			}
			d, _ := ciEvalIdentity(t, s, ciStringNode(1, "missing"), ciRule("reference"), p, true)
			want := OutcomeUnknown
			if mode == "complete" {
				want = OutcomeFail
			}
			if mode == "unselected" {
				want = OutcomeUnsupported
			}
			if d.Outcome != want {
				t.Fatalf("%s: %+v", mode, d)
			}
		})
	}
}

func TestIdentityEvidenceAndTypedStringContract(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceAbsent, EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		t.Run(string(state), func(t *testing.T) {
			s := ciSession(t, Budgets{})
			p, _ := NewIdentityDeclarationPlan("ids", ciSelector())
			n := ciStringNode(1, "unused")
			evidence := Evidence[Atomic]{State: state}
			if state != EvidenceAbsent {
				evidence.Reason = "upstream-unavailable"
			}
			if state == EvidenceInvalid {
				evidence.Cause = &DecisionRef{Rule: ciRule("lexical").Key, Subject: NodeRef{DocumentID: "doc", Node: 1}}
			}
			n.attributes[0].Scalar.Value = evidence
			if state == EvidenceAbsent {
				n.attributes[0].Presence = Evidence[bool]{State: EvidenceAbsent}
			}
			d, pending := ciEvalIdentity(t, s, n, ciRule("declaration"), p, true)
			want := map[EvidenceState]Outcome{EvidenceAbsent: OutcomeNotApplicable, EvidenceUnknown: OutcomeUnknown, EvidenceUnsupported: OutcomeUnsupported, EvidenceInvalid: OutcomeBlocked}[state]
			if pending || d.Outcome != want {
				t.Fatalf("evidence outcome: %+v", d)
			}
			if state == EvidenceInvalid && (d.Cause == nil || *d.Cause != *evidence.Cause) {
				t.Fatal("invalid cause lost")
			}
			if state != EvidenceAbsent && !s.identities["ids"].incomplete {
				t.Fatal("unavailable declaration did not taint index")
			}
		})
	}
	s := ciSession(t, Budgets{})
	p, _ := NewIdentityDeclarationPlan("ids", ciSelector())
	n := ciStringNode(1, "true")
	n.attributes[0].Scalar.Value.Value.Kind = AtomicBoolean
	d, pending, err := evaluateIdentityPlan(context.Background(), s, n, ciRule("declaration"), p, false)
	if err != nil || pending || d.Outcome != OutcomeUnsupported || d.Reason != "identity-family-unavailable" {
		t.Fatalf("valid non-string identity must be unsupported without coercion or a terminal contract error: decision=%+v pending=%t err=%v", d, pending, err)
	}
	if !s.identities["ids"].incomplete {
		t.Error("unsupported declaration family did not keep the index incomplete")
	}
}

func TestIdentityExactStringsAndIndependentIndexes(t *testing.T) {
	s := ciSession(t, Budgets{})
	s.inputComplete = true
	for _, index := range []string{"first", "second"} {
		p, _ := NewIdentityDeclarationPlan(index, ciSelector())
		if d, _ := ciEvalIdentity(t, s, ciStringNode(1, " key "), ciRule(index), p, true); d.Outcome != OutcomePass {
			t.Fatalf("cross-index collision: %+v", d)
		}
	}
	p, _ := NewIdentityReferencePlan("first", ciSelector())
	if d, _ := ciEvalIdentity(t, s, ciStringNode(2, "key"), ciRule("reference"), p, true); d.Outcome != OutcomeFail {
		t.Fatalf("lexical normalization invented: %+v", d)
	}
}

func TestIdentityDeclarationsAndReferencesShareBudget(t *testing.T) {
	s := ciSession(t, Budgets{MaxIDs: 2})
	declare, _ := NewIdentityDeclarationPlan("ids", ciSelector())
	reference, _ := NewIdentityReferencePlan("ids", ciSelector())
	ciEvalIdentity(t, s, ciStringNode(1, "k"), ciRule("declaration"), declare, false)
	ciEvalIdentity(t, s, ciStringNode(2, "k"), ciRule("reference"), reference, false)
	if _, _, err := evaluateIdentityPlan(context.Background(), s, ciStringNode(3, "k"), ciRule("reference"), reference, false); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("references bypassed ID cap: %v", err)
	}
}

func TestContextPublicationIsCopiedAndScopeExact(t *testing.T) {
	for _, relation := range []RelationKind{RelationSameNode, RelationParent, RelationDocument} {
		t.Run(string(relation), func(t *testing.T) {
			s := ciSession(t, Budgets{})
			s.inputComplete = true
			s.nodes[1] = ciNode(1, 0)
			n := ciNode(2, 1)
			n.header.Location.Position = &SourcePosition{Line: 2, Column: 1, CoordinateSpace: "source"}
			value := Atomic{Kind: AtomicString, Text: "original"}
			p, _ := NewContextValuePlan("effective", relation, value)
			c, _ := NewContextComparisonPlan("effective", relation, ComparisonEqual, value)
			if d, pending := ciEvalContext(t, s, n, ciRule("provider"), p, false); pending || d.Outcome != OutcomePass {
				t.Fatalf("provider: %+v", d)
			}
			if d, pending := ciEvalContext(t, s, n, ciRule("consumer"), c, false); !pending {
				t.Fatalf("consumer decided before all providers: %+v", d)
			}
			n.header.Location.Position.Line = 999
			p.expected.Text = "mutated"
			d, pending := ciEvalContext(t, s, n, ciRule("consumer"), c, true)
			if pending || d.Outcome != OutcomePass {
				t.Fatalf("copied context lost: %+v", d)
			}
			at := NodeID(2)
			if relation == RelationParent {
				at = 1
			}
			if relation == RelationDocument {
				at = 0
			}
			fact := s.contexts[ContextKey{Name: "effective", At: NodeRef{DocumentID: "doc", Node: at}}]
			if fact.Value.Value.Text != "original" || len(fact.Providers) != 1 || fact.Providers[0] != ciRule("provider").Key || fact.Origins[0].Position.Line != 2 {
				t.Fatalf("context lost ownership/provenance: %+v", fact)
			}
		})
	}
}

func TestContextConflictingProvidersStayUnknown(t *testing.T) {
	s := ciSession(t, Budgets{})
	n := ciNode(1, 0)
	one, _ := NewContextValuePlan("effective", RelationDocument, Atomic{Kind: AtomicString, Text: "one"})
	two, _ := NewContextValuePlan("effective", RelationDocument, Atomic{Kind: AtomicString, Text: "two"})
	comparison, _ := NewContextComparisonPlan("effective", RelationDocument, ComparisonEqual, Atomic{Kind: AtomicString, Text: "two"})
	ciEvalContext(t, s, n, ciRule("first"), one, false)
	ciEvalContext(t, s, n, ciRule("second"), two, false)
	d, _ := ciEvalContext(t, s, n, ciRule("consumer"), comparison, true)
	if d.Outcome != OutcomeUnknown {
		t.Fatalf("conflicting context last-wins: %+v", d)
	}
	fact := s.contexts[ContextKey{Name: "effective", At: NodeRef{DocumentID: "doc"}}]
	if fact.Value.State != EvidenceUnknown || len(fact.Providers) != 2 || len(fact.Origins) != 2 {
		t.Fatalf("conflict provenance lost: %+v", fact)
	}
}

func TestContextMissingAndUnsupportedRelationsCannotPass(t *testing.T) {
	s := ciSession(t, Budgets{})
	p, _ := NewContextComparisonPlan("missing", RelationSameNode, ComparisonEqual, Atomic{Kind: AtomicString, Text: ""})
	n := ciNode(1, 0)
	if d, _ := ciEvalContext(t, s, n, ciRule("consumer"), p, true); d.Outcome != OutcomeUnknown {
		t.Fatalf("missing became default: %+v", d)
	}
	p.relation = RelationMusicalPosition
	if d, _ := ciEvalContext(t, s, n, ciRule("consumer"), p, true); d.Outcome != OutcomeUnsupported {
		t.Fatalf("musical relation invented: %+v", d)
	}
}

func TestContextRegistrationAndBudgets(t *testing.T) {
	s := ciSession(t, Budgets{MaxContextFacts: 1})
	p, _ := NewContextValuePlan("key", RelationDocument, Atomic{Kind: AtomicString, Text: "v"})
	n := ciNode(1, 0)
	ciEvalContext(t, s, n, ciRule("provider"), p, false)
	ciEvalContext(t, s, n, ciRule("provider"), p, true)
	used, _ := s.account.Usage(BudgetContextFacts)
	if used != 1 {
		t.Fatalf("publication repeated: %d", used)
	}
	if _, _, err := evaluateContextPlan(context.Background(), s, ciNode(2, 0), ciRule("provider"), p, false); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("context quota ignored: %v", err)
	}
}

func ciRuntime(t *testing.T, specs []struct {
	id, occurrence string
	plan           Plan
	role           RuleRole
}, limits Budgets, requirements ...map[string][]Dependency) *Session {
	t.Helper()
	base, profiles, _ := catalogFixture(t)
	profiles[0].Required = nil
	profiles[0].Support = nil
	rules := make([]RuleDefinition, 0, len(specs))
	bindings := make([]PlanBinding, 0, len(specs))
	for _, spec := range specs {
		r := cloneRule(base[0])
		r.Key = RuleKey{ID: RuleID(spec.id), Revision: 1}
		r.Predicate = spec.id
		r.Role = spec.role
		if len(requirements) != 0 {
			r.Requires = requirements[0][spec.id]
		}
		r.Selector.Declaration.Occurrence = spec.occurrence
		rules = append(rules, r)
		bindings = append(bindings, PlanBinding{ID: spec.id, Plan: spec.plan})
		profiles[0].Required = append(profiles[0].Required, r.Key)
		profiles[0].Support = append(profiles[0].Support, RuleSupport{Rule: r.Key, State: ImplementationAvailable})
	}
	c, err := NewCatalog(rules, profiles, bindings)
	if err != nil {
		t.Fatal(err)
	}
	e, err := NewEngine(c)
	if err != nil {
		t.Fatal(err)
	}
	s, err := e.Begin(context.Background(), RunOptions{DocumentID: "doc", Profile: profiles[0].ID, Budgets: limits})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func ciStreamNode(t *testing.T, s *Session, id, parent NodeID, occurrence, key string) {
	t.Helper()
	ctx := context.Background()
	if err := s.OpenNode(ctx, factHeader(id, parent, occurrence, "slot")); err != nil {
		t.Fatal(err)
	}
	if key != "" {
		a := ciStringNode(id, key).attributes[0]
		if err := s.Attribute(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.CloseNode(ctx, factEnd(id)); err != nil {
		t.Fatal(err)
	}
}
func TestIdentityPublicForwardReferenceAndDuplicateCausality(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		t.Run(map[bool]string{false: "unique", true: "duplicate"}[duplicate], func(t *testing.T) {
			declaration, _ := NewIdentityDeclarationPlan("ids", ciSelector())
			reference, _ := NewIdentityReferencePlan("ids", ciSelector())
			s := ciRuntime(t, []struct {
				id, occurrence string
				plan           Plan
				role           RuleRole
			}{{"a-reference", "test.ref", reference, RolePredicate}, {"z-declaration", "test.decl", declaration, RolePredicate}}, Budgets{})
			ctx := context.Background()
			if err := s.OpenNode(ctx, factHeader(1, 0, "test.root", "root")); err != nil {
				t.Fatal(err)
			}
			ciStreamNode(t, s, 2, 1, "test.ref", "K")
			ciStreamNode(t, s, 3, 1, "test.decl", "K")
			if duplicate {
				ciStreamNode(t, s, 4, 1, "test.decl", "K")
			}
			if err := s.CloseNode(ctx, factEnd(1)); err != nil {
				t.Fatal(err)
			}
			report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
			if err != nil {
				t.Fatal(err)
			}
			if !duplicate {
				if !report.Satisfied() {
					t.Fatalf("public forward reference: %+v", report)
				}
				return
			}
			if report.Counts.Fail != 1 {
				t.Fatalf("duplicate proof count: %+v", report)
			}
			found := false
			for _, finding := range report.Findings {
				if finding.Rule.ID == "a-reference" && finding.Subject.Node == 2 {
					found = true
					if finding.Outcome != OutcomeBlocked || finding.Cause == nil || finding.Cause.Subject.Node != 4 || finding.Cause.Rule.ID != "z-declaration" {
						t.Fatalf("public duplicate cause: %+v", finding)
					}
				}
			}
			if !found {
				t.Fatal("reference decision missing")
			}
		})
	}
}
func TestIdentityPublicEmptyCompleteIndexStillHasProvider(t *testing.T) {
	declaration, _ := NewIdentityDeclarationPlan("ids", ciSelector())
	reference, _ := NewIdentityReferencePlan("ids", ciSelector())
	s := ciRuntime(t, []struct {
		id, occurrence string
		plan           Plan
		role           RuleRole
	}{{"a-reference", "test.ref", reference, RolePredicate}, {"z-declaration", "test.decl", declaration, RolePredicate}}, Budgets{})
	ctx := context.Background()
	if err := s.OpenNode(ctx, factHeader(1, 0, "test.root", "root")); err != nil {
		t.Fatal(err)
	}
	ciStreamNode(t, s, 2, 1, "test.ref", "missing")
	if err := s.CloseNode(ctx, factEnd(1)); err != nil {
		t.Fatal(err)
	}
	report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	if report.Counts.Fail != 1 {
		t.Fatalf("selected but empty index should prove missing once: %+v", report)
	}
}
func TestContextPublicProviderUnavailabilityCannotBeHidden(t *testing.T) {
	provider, _ := NewContextValuePlan("effective", RelationDocument, Atomic{Kind: AtomicString, Text: "known"})
	consumer, _ := NewContextComparisonPlan("effective", RelationDocument, ComparisonEqual, Atomic{Kind: AtomicString, Text: "known"})
	s := ciRuntime(t, []struct {
		id, occurrence string
		plan           Plan
		role           RuleRole
	}{{"a-consumer", "test.root", consumer, RolePredicate}, {"z-provider", "test.provider", provider, RoleContextProvider}}, Budgets{})
	ctx := context.Background()
	if err := s.OpenNode(ctx, factHeader(1, 0, "test.root", "root")); err != nil {
		t.Fatal(err)
	}
	ciStreamNode(t, s, 2, 1, "test.provider", "")
	h := factHeader(3, 1, "test.provider", "slot")
	h.Declaration = Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "binding-missing"}
	if err := s.OpenNode(ctx, h); err != nil {
		t.Fatal(err)
	}
	if err := s.CloseNode(ctx, factEnd(3)); err != nil {
		t.Fatal(err)
	}
	if err := s.CloseNode(ctx, factEnd(1)); err != nil {
		t.Fatal(err)
	}
	report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range report.Findings {
		if finding.Rule.ID == "a-consumer" && finding.Subject.Node == 1 {
			if finding.Outcome != OutcomeUnknown {
				t.Fatalf("unavailable provider hidden: %+v", finding)
			}
			return
		}
	}
	t.Fatalf("unknown context consumer missing (possibly passed): %+v", report)
}
func TestContextMissingScopeAndIncompleteDocumentRemainUnknown(t *testing.T) {
	s := ciSession(t, Budgets{})
	n := ciNode(1, 0)
	p, _ := NewContextValuePlan("key", RelationDocument, Atomic{Kind: AtomicString, Text: "v"})
	comparison, _ := NewContextComparisonPlan("key", RelationDocument, ComparisonEqual, Atomic{Kind: AtomicString, Text: "v"})
	ciEvalContext(t, s, n, ciRule("provider"), p, false)
	if d, _ := ciEvalContext(t, s, n, ciRule("consumer"), comparison, true); d.Outcome != OutcomeUnknown {
		t.Fatalf("incomplete document context passed: %+v", d)
	}
	comparison.relation = RelationParent
	if d, _ := ciEvalContext(t, s, n, ciRule("consumer"), comparison, true); d.Outcome != OutcomeUnknown {
		t.Fatalf("missing parent context passed: %+v", d)
	}
}
func TestContextAndIdentityHonorCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := ciSession(t, Budgets{})
	n := ciStringNode(1, "key")
	identity, _ := NewIdentityDeclarationPlan("ids", ciSelector())
	provider, _ := NewContextValuePlan("key", RelationSameNode, Atomic{Kind: AtomicString, Text: "value"})
	if _, _, err := evaluateIdentityPlan(ctx, s, n, ciRule("identity"), identity, false); !errors.Is(err, context.Canceled) {
		t.Fatalf("identity cancellation lost: %v", err)
	}
	if _, _, err := evaluateContextPlan(ctx, s, n, ciRule("provider"), provider, false); !errors.Is(err, context.Canceled) {
		t.Fatalf("context cancellation lost: %v", err)
	}
	ids, _ := s.account.Usage(BudgetIDs)
	contexts, _ := s.account.Usage(BudgetContextFacts)
	if ids != 0 || contexts != 0 {
		t.Fatal("canceled evaluation mutated publication counts")
	}
}

func TestContextMismatchedFamilyIsCapabilityOutcome(t *testing.T) {
	s := ciSession(t, Budgets{})
	n := ciNode(1, 0)
	p, _ := NewContextValuePlan("key", RelationSameNode, Atomic{Kind: AtomicString, Text: "true"})
	c, _ := NewContextComparisonPlan("key", RelationSameNode, ComparisonEqual, Atomic{Kind: AtomicBoolean, Text: "true"})
	ciEvalContext(t, s, n, ciRule("provider"), p, false)
	d, pending, err := evaluateContextPlan(context.Background(), s, n, ciRule("consumer"), c, true)
	if err != nil || pending || d.Outcome != OutcomeUnsupported {
		t.Fatalf("family mismatch became terminal/normative failure: %+v pending=%v err=%v", d, pending, err)
	}
}
func TestContextConstantRespectsScalarBudgetBeforePublishing(t *testing.T) {
	s := ciSession(t, Budgets{MaxScalarBytes: 2})
	p, _ := NewContextValuePlan("key", RelationSameNode, Atomic{Kind: AtomicString, Text: "abc"})
	if _, _, err := evaluateContextPlan(context.Background(), s, ciNode(1, 0), ciRule("provider"), p, false); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("constant bypassed scalar cap: %v", err)
	}
	if len(s.contexts) != 0 {
		t.Fatal("oversized context published")
	}
}
func TestContextUnavailableProviderCannotBeReplacedByLaterKnownValue(t *testing.T) {
	for _, outcome := range []Outcome{OutcomeUnknown, OutcomeUnsupported, OutcomeBlocked} {
		t.Run(string(outcome), func(t *testing.T) {
			s := ciSession(t, Budgets{})
			n := ciNode(1, 0)
			p, _ := NewContextValuePlan("key", RelationSameNode, Atomic{Kind: AtomicString, Text: "value"})
			c, _ := NewContextComparisonPlan("key", RelationSameNode, ComparisonEqual, Atomic{Kind: AtomicString, Text: "value"})
			d := Decision{Outcome: outcome, Reason: "provider-unavailable"}
			if outcome == OutcomeBlocked {
				d.Cause = &DecisionRef{Rule: ciRule("prerequisite").Key, Subject: NodeRef{DocumentID: "doc", Node: 1}}
			}
			if err := s.markContextIncomplete(context.Background(), n, ciRule("missing-provider"), p, d); err != nil {
				t.Fatal(err)
			}
			ciEvalContext(t, s, n, ciRule("known-provider"), p, false)
			actual, _ := ciEvalContext(t, s, n, ciRule("consumer"), c, true)
			if actual.Outcome != outcome {
				t.Fatalf("unavailable evidence overwritten: %+v", actual)
			}
			if outcome == OutcomeBlocked && (actual.Cause == nil || *actual.Cause != *d.Cause) {
				t.Fatal("context prerequisite cause lost")
			}
		})
	}
}

func TestIdentityPublicDeferredProviderPrecedesReferenceResolution(t *testing.T) {
	declaration, _ := NewIdentityDeclarationPlan("ids", ciSelector())
	reference, _ := NewIdentityReferencePlan("ids", ciSelector())
	ready, _ := NewChildCountPlan([]DeclarationID{ciSelector().Declaration}, 0, -1)
	s := ciRuntime(t, []struct {
		id, occurrence string
		plan           Plan
		role           RuleRole
	}{{"a-reference", "test.ref", reference, RolePredicate}, {"z-declaration", "test.decl", declaration, RolePredicate}, {"zz-ready", "test.root", ready, RolePredicate}}, Budgets{}, map[string][]Dependency{"z-declaration": {{Kind: DependencyRule, Rule: ciRule("zz-ready").Key, Relation: RelationDocument}}})
	ctx := context.Background()
	if err := s.OpenNode(ctx, factHeader(1, 0, "test.root", "root")); err != nil {
		t.Fatal(err)
	}
	ciStreamNode(t, s, 2, 1, "test.ref", "forward")
	ciStreamNode(t, s, 3, 1, "test.decl", "forward")
	if err := s.CloseNode(ctx, factEnd(1)); err != nil {
		t.Fatal(err)
	}
	report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Satisfied() {
		t.Fatalf("deferred provider was not collected before reference resolution: %+v", report)
	}
}

func TestContextDefaultUsesOnlyProvenAbsentSlot(t *testing.T) {
	for _, mode := range []string{"present", "explicit-absent", "enumerated-absent", "incomplete", "unknown-presence", "unsupported-presence", "invalid-value", "absent-value"} {
		t.Run(mode, func(t *testing.T) {
			s := ciSession(t, Budgets{})
			n := ciStringNode(1, "explicit")
			fallback := Atomic{Kind: AtomicString, Text: "fallback"}
			p, err := NewContextDefaultPlan("default", RelationSameNode, ciSelector(), fallback)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "explicit-absent":
				n.attributes[0].Presence = Evidence[bool]{State: EvidenceAbsent}
			case "enumerated-absent":
				n.attributes = nil
			case "incomplete":
				n.attributes = nil
				n.end.AttributesComplete = testEvidence(Evidence[bool]{State: EvidenceKnown, Value: false})
			case "unknown-presence":
				n.attributes[0].Presence = Evidence[bool]{State: EvidenceUnknown, Reason: "upstream-missing"}
			case "unsupported-presence":
				n.attributes[0].Presence = Evidence[bool]{State: EvidenceUnsupported, Reason: "upstream-unsupported"}
			case "invalid-value":
				n.attributes[0].Scalar.Value = Evidence[Atomic]{State: EvidenceInvalid, Reason: "upstream-invalid", Cause: &DecisionRef{Rule: ciRule("domain").Key, Subject: NodeRef{DocumentID: "doc", Node: 1}}}
			case "absent-value":
				n.attributes[0].Scalar.Value = Evidence[Atomic]{State: EvidenceAbsent}
			}
			d, pending := ciEvalContext(t, s, n, ciRule("provider"), p, false)
			if pending {
				t.Fatal("default provider stayed pending after close")
			}
			fact := s.contexts[ContextKey{Name: "default", At: NodeRef{DocumentID: "doc", Node: 1}}]
			switch mode {
			case "present":
				if d.Outcome != OutcomePass || fact.Value.State != EvidenceKnown || fact.Value.Value.Text != "explicit" || fact.Origins[0].Path != "/slot/@id" {
					t.Fatalf("explicit value failed to suppress fallback: %+v %+v", d, fact)
				}
			case "explicit-absent", "enumerated-absent":
				if d.Outcome != OutcomePass || fact.Value.State != EvidenceKnown || fact.Value.Value.Text != "fallback" {
					t.Fatalf("proven absence did not use copied fallback: %+v %+v", d, fact)
				}
			default:
				want := OutcomeUnknown
				if mode == "unsupported-presence" {
					want = OutcomeUnsupported
				}
				if mode == "invalid-value" {
					want = OutcomeBlocked
				}
				if d.Outcome != want || fact.Value.State == EvidenceKnown {
					t.Fatalf("unavailable presence/value activated fallback: %+v %+v", d, fact)
				}
			}
		})
	}
}
func TestContextDefaultActualValueAndFallbackAreCopied(t *testing.T) {
	s := ciSession(t, Budgets{})
	n := ciStringNode(1, "actual")
	p, err := NewContextDefaultPlan("default", RelationSameNode, ciSelector(), Atomic{Kind: AtomicString, Text: "fallback"})
	if err != nil {
		t.Fatal(err)
	}
	ciEvalContext(t, s, n, ciRule("provider"), p, false)
	n.attributes[0].Scalar.Value.Value.Text = "changed"
	n.attributes[0].Location.Position.Line = 99
	fact := s.contexts[ContextKey{Name: "default", At: NodeRef{DocumentID: "doc", Node: 1}}]
	if fact.Value.Value.Text != "actual" || fact.Origins[0].Position.Line != 1 {
		t.Fatalf("explicit default source aliases retained: %+v", fact)
	}
}

func TestIdentityPresentSlotWithoutTypedValueIsUnknown(t *testing.T) {
	s := ciSession(t, Budgets{})
	s.inputComplete = true
	p, _ := NewIdentityDeclarationPlan("ids", ciSelector())
	n := ciStringNode(1, "unused")
	n.attributes[0].Scalar.Value = Evidence[Atomic]{State: EvidenceAbsent}
	d, pending := ciEvalIdentity(t, s, n, ciRule("declaration"), p, true)
	if pending || d.Outcome != OutcomeUnknown || !s.identities["ids"].incomplete {
		t.Fatalf("present slot with no typed identity was treated as absent: %+v", d)
	}
}

func TestContextProvenanceCopiesUseSharedBudgets(t *testing.T) {
	origins := []Location{{Target: TargetSource, DocumentID: "doc", Path: "/first"}, {Target: TargetSource, DocumentID: "doc", Path: "/second"}}
	for _, limits := range []Budgets{{MaxRuleSteps: 2}, {MaxBufferedBytes: 200}} {
		s := ciSession(t, limits)
		if _, err := s.contextRelated(context.Background(), origins); !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("provenance copy bypassed shared work/buffer cap: %v", err)
		}
	}
}

func TestIdentityPublicMissingReferenceHasOneCauseForDependent(t *testing.T) {
	declaration, _ := NewIdentityDeclarationPlan("ids", ciSelector())
	reference, _ := NewIdentityReferencePlan("ids", ciSelector())
	dependent, _ := NewAtomicComparisonPlan(ComparisonEqual, Atomic{Kind: AtomicString, Text: "typed"})
	s := ciRuntime(t, []struct {
		id, occurrence string
		plan           Plan
		role           RuleRole
	}{{"reference", "test.ref", reference, RolePredicate}, {"declaration", "test.decl", declaration, RolePredicate}, {"dependent", "test.ref", dependent, RolePredicate}}, Budgets{}, map[string][]Dependency{"dependent": {{Kind: DependencyRule, Rule: ciRule("reference").Key, Relation: RelationSameNode}}})
	ctx := context.Background()
	if err := s.OpenNode(ctx, factHeader(1, 0, "test.root", "root")); err != nil {
		t.Fatal(err)
	}
	ciStreamNode(t, s, 2, 1, "test.ref", "missing")
	if err := s.CloseNode(ctx, factEnd(1)); err != nil {
		t.Fatal(err)
	}
	report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	if report.Counts.Fail != 1 {
		t.Fatalf("missing reference emitted repeated failure: %+v", report)
	}
	for _, finding := range report.Findings {
		if finding.Rule.ID == "dependent" && finding.Subject.Node == 2 {
			if finding.Outcome != OutcomeBlocked || finding.Cause == nil || finding.Cause.Rule.ID != "reference" || finding.Cause.Subject.Node != 2 {
				t.Fatalf("dependent lost missing-reference cause: %+v", finding)
			}
			return
		}
	}
	t.Fatal("dependent decision missing")
}

func TestIdentityPublicEarlyDuplicateSurvivesCancellation(t *testing.T) {
	declaration, _ := NewIdentityDeclarationPlan("ids", ciSelector())
	s := ciRuntime(t, []struct {
		id, occurrence string
		plan           Plan
		role           RuleRole
	}{{"declaration", "test.decl", declaration, RolePredicate}}, Budgets{})
	ctx := context.Background()
	if err := s.OpenNode(ctx, factHeader(1, 0, "test.root", "root")); err != nil {
		t.Fatal(err)
	}
	ciStreamNode(t, s, 2, 1, "test.decl", "duplicate")
	ciStreamNode(t, s, 3, 1, "test.decl", "duplicate")
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	report, err := s.Finish(canceled, Completion{InputComplete: false, Syntax: SyntaxIncomplete})
	if !errors.Is(err, context.Canceled) || !errors.Is(err, ErrIncompleteAssessment) || report.Counts.Fail != 1 || report.Conformance != ConformanceViolated || report.AssessmentComplete {
		t.Fatalf("early duplicate erased by cancellation: %+v err=%v", report, err)
	}
	retained, _ := s.account.Usage(BudgetBufferedBytes)
	depth, _ := s.account.Usage(BudgetDepth)
	if retained != 0 || depth != 0 {
		t.Fatalf("terminal cleanup leaked gauges: retained=%d depth=%d", retained, depth)
	}
}

func TestContextPublicExplicitProviderDependency(t *testing.T) {
	provider, _ := NewContextValuePlan("effective", RelationDocument, Atomic{Kind: AtomicString, Text: "value"})
	consumer, _ := NewContextComparisonPlan("effective", RelationDocument, ComparisonEqual, Atomic{Kind: AtomicString, Text: "value"})
	s := ciRuntime(t, []struct {
		id, occurrence string
		plan           Plan
		role           RuleRole
	}{{"consumer", "test.child", consumer, RolePredicate}, {"provider", "test.root", provider, RoleContextProvider}}, Budgets{}, map[string][]Dependency{"consumer": {{Kind: DependencyRule, Rule: ciRule("provider").Key, Relation: RelationDocument}}})
	ctx := context.Background()
	if err := s.OpenNode(ctx, factHeader(1, 0, "test.root", "root")); err != nil {
		t.Fatal(err)
	}
	ciStreamNode(t, s, 2, 1, "test.child", "")
	if err := s.CloseNode(ctx, factEnd(1)); err != nil {
		t.Fatal(err)
	}
	report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Satisfied() {
		t.Fatalf("explicit provider-before-consumer failed: %+v", report)
	}
}
