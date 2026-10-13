package validation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func sourceOriginFixture(t *testing.T, limits Budgets) (*Engine, *Session, RuleKey) {
	t.Helper()
	base := sourceSchemaEngine(t, XSDCheckScalar)
	profile, _ := base.Profile("source-schema-test")
	profile.CatalogDigest = ""
	var rules []RuleDefinition
	var plans []PlanBinding
	for _, key := range profile.Required {
		r, _ := base.catalog.Rule(key)
		rules = append(rules, r)
		plans = append(plans, PlanBinding{ID: r.Predicate, Plan: base.catalog.plans[r.Predicate]})
	}
	plan, err := NewSourceOriginPlan(AssemblyMusicXML)
	if err != nil {
		t.Fatal(err)
	}
	key := RuleKey{"test.source-origin", 1}
	assembly := AssemblyMusicXML
	rules = append(rules, RuleDefinition{Key: key, Targets: []Target{TargetSource}, Scope: ScopeDocument, Category: CategoryNormative, Class: ClassMandatory, Role: RoleContextProvider, Selector: Selector{Assembly: &assembly}, Predicate: "source-origin", Text: RuleText{Title: "Source origin", Explanation: "Retain explicitly supplied source origins.", Message: "Source origin evidence is incomplete."}, Sources: []SourceRef{{IDs: []string{"XML-1.0"}, URL: "https://www.w3.org/TR/xml/", Locator: "3.3.2"}}})
	plans = append(plans, PlanBinding{ID: "source-origin", Plan: plan})
	profile.Required = append(profile.Required, key)
	profile.Support = append(profile.Support, RuleSupport{Rule: key, State: ImplementationAvailable})
	c, err := NewCatalog(rules, []Profile{profile}, plans)
	if err != nil {
		t.Fatal(err)
	}
	e, err := NewEngine(c)
	if err != nil {
		t.Fatal(err)
	}
	s, err := e.Begin(context.Background(), RunOptions{DocumentID: "source", Profile: profile.ID, Budgets: limits})
	if err != nil {
		t.Fatal(err)
	}
	return e, s, key
}
func sourceOriginPhysical() SourceOriginContextResult {
	return SourceOriginContextResult{Subject: NodeRef{"source", 1}, Kind: SourceOriginPhysical, Specified: stateKnown(true), Physical: stateKnown(Location{Target: TargetSource, DocumentID: "source", Path: "/root", Position: &SourcePosition{ByteOffset: 12, Line: 2, Column: 1, CoordinateSpace: "xml"}}), Declaration: stateAbsent[Location](), Use: stateAbsent[Location](), AncestryComplete: stateKnown(true)}
}
func TestSourceOriginOwnedIngressAndReport(t *testing.T) {
	e, s, key := sourceOriginFixture(t, Budgets{})
	ctx := context.Background()
	if got, ok := e.SourceOriginProvider("source-schema-test", AssemblyMusicXML); !ok || got != key {
		t.Fatalf("provider %v %v", got, ok)
	}
	if _, ok := e.SourceOriginProvider("missing", AssemblyMusicXML); ok {
		t.Fatal("unknown profile selected")
	}
	h := schemaPendingHeader()
	h.Location = sourceOriginPhysical().Physical.Value
	if err := s.Open(ctx, h); err != nil {
		t.Fatal(err)
	}
	in := sourceOriginPhysical()
	if err := s.SourceOrigin(ctx, key, in); err != nil {
		t.Fatal(err)
	}
	in.Physical.Value.Position.ByteOffset = 999
	if err := s.Close(ctx, schemaLexemeEnd("1")); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteSourceOrigins(ctx, key, stateKnown(true)); err != nil {
		t.Fatal(err)
	}
	report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Satisfied() || len(report.Context) != 1 {
		t.Fatalf("report %+v", report)
	}
	origin := report.Context[0].SourceOrigin
	if origin == nil || origin.Physical.Value.Position.ByteOffset != 12 || origin.Subject != (NodeRef{"source", 1}) {
		t.Fatalf("origin %+v", origin)
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("leaked %d", used)
	}
	data, err := json.Marshal(report.Context[0])
	if err != nil || !strings.Contains(string(data), `"SourceOrigin":`) {
		t.Fatalf("JSON %s %v", data, err)
	}
}
func TestSourceOriginMissingSealCannotPass(t *testing.T) {
	_, s, _ := sourceOriginFixture(t, Budgets{})
	ctx := context.Background()
	if err := s.Open(ctx, schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(ctx, schemaLexemeEnd("1")); err != nil {
		t.Fatal(err)
	}
	r, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	if r.AssessmentComplete || len(r.Findings) != 1 || r.Findings[0].Decision.Outcome != OutcomeUnknown {
		t.Fatalf("unsealed evidence passed %+v", r)
	}
}
func TestSourceOriginOldJSONUnchanged(t *testing.T) {
	r := ContextResult{Provider: RuleKey{"old", 1}, Subject: NodeRef{"source", 1}, Property: ContextPropertySpanNormalDots, Value: stateAbsent[Atomic]()}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "SourceOrigin") {
		t.Fatalf("old shape changed: %s", data)
	}
}
func TestSourceOriginSnapshotExplicitlyRejected(t *testing.T) {
	_, s, key := sourceOriginFixture(t, Budgets{})
	ctx := context.Background()
	h := schemaPendingHeader()
	h.Location = sourceOriginPhysical().Physical.Value
	if err := s.Open(ctx, h); err != nil {
		t.Fatal(err)
	}
	if err := s.SourceOrigin(ctx, key, sourceOriginPhysical()); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(ctx, schemaLexemeEnd("1")); err != nil {
		t.Fatal(err)
	}
	if err := s.CompleteSourceOrigins(ctx, key, stateKnown(true)); err != nil {
		t.Fatal(err)
	}
	r, snapshot, err := s.FinishWithResourceSnapshot(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if !errors.Is(err, ErrIncompleteAssessment) || snapshot != nil || len(r.Context) != 1 || r.Context[0].SourceOrigin == nil {
		t.Fatalf("silent loss: %+v %v %v", r, snapshot, err)
	}
}

func sourceOriginAttribute(name QName, location Location) AttributeFact {
	return AttributeFact{Owner: 1, Name: name, Declaration: stateUnknown[DeclarationID]("unbound-attribute"), Presence: stateKnown(true), Scalar: Scalar{Lexeme: stateKnown("value"), Value: stateUnknown[Atomic]("not-assessed")}, Location: location}
}
func TestSourceOriginDefaultAndEntityAttributeOwnership(t *testing.T) {
	for _, kind := range []SourceOriginKind{SourceOriginDefault, SourceOriginEntity} {
		t.Run(string(kind), func(t *testing.T) {
			_, s, key := sourceOriginFixture(t, Budgets{})
			defer s.cleanup()
			ctx := context.Background()
			if err := s.Open(ctx, schemaPendingHeader()); err != nil {
				t.Fatal(err)
			}
			name := QName{Space: "urn:attribute", Local: "value"}
			in := sourceOriginPhysical()
			in.Attribute = &name
			in.Kind = kind
			in.Physical = stateAbsent[Location]()
			in.Specified = stateKnown(false)
			in.Declaration = stateKnown(Location{Target: TargetSource, DocumentID: "source", Path: "/!DOCTYPE", Position: &SourcePosition{ByteOffset: 8, Line: 1, Column: 9, CoordinateSpace: "xml"}})
			in.Use = stateKnown(Location{Target: TargetSource, DocumentID: "source", Path: "/root", Position: &SourcePosition{ByteOffset: 75, Line: 2, Column: 1, CoordinateSpace: "xml"}})
			in.Ancestry = []SourceOriginStep{{Entity: "entity", Declaration: in.Declaration, Use: in.Use}}
			if err := s.Attribute(ctx, sourceOriginAttribute(name, Location{Target: TargetSource, DocumentID: "source", Path: "/root/@value"})); err != nil {
				t.Fatal(err)
			}
			if err := s.SourceOrigin(ctx, key, in); err != nil {
				t.Fatal(err)
			}
			name.Local = "changed"
			in.Declaration.Value.Position.ByteOffset = 999
			in.Ancestry[0].Entity = "changed"
			in.Ancestry[0].Use.Value.Position.ByteOffset = 999
			out := s.contextResults[0].SourceOrigin
			if out.Attribute.Local != "value" || out.Declaration.Value.Position.ByteOffset != 8 || out.Ancestry[0].Entity != "entity" || out.Use.Value.Position.ByteOffset != 75 {
				t.Fatalf("aliased %+v", out)
			}
		})
	}
}
func TestSourceOriginRejectsUnownedAndContradictoryIngress(t *testing.T) {
	tests := map[string]func(*SourceOriginContextResult){
		"wrong-document":         func(v *SourceOriginContextResult) { v.Subject.DocumentID = "other" },
		"wrong-node":             func(v *SourceOriginContextResult) { v.Subject.Node = 2 },
		"unadmitted-attribute":   func(v *SourceOriginContextResult) { v.Attribute = &QName{Local: "missing"} },
		"physical-site-mismatch": func(v *SourceOriginContextResult) { v.Physical.Value.Position.ByteOffset++ },
		"physical-no-coordinate": func(v *SourceOriginContextResult) { v.Physical.Value.Position = nil },
		"physical-unspecified":   func(v *SourceOriginContextResult) { v.Specified = stateKnown(false) },
		"physical-ancestry": func(v *SourceOriginContextResult) {
			v.Ancestry = []SourceOriginStep{{Entity: "entity", Declaration: v.Physical, Use: v.Physical}}
		},
		"default-element":         func(v *SourceOriginContextResult) { v.Kind = SourceOriginDefault },
		"unknown-kind":            func(v *SourceOriginContextResult) { v.Kind = "invented" },
		"missing-component-state": func(v *SourceOriginContextResult) { v.Use = Evidence[Location]{} },
		"unknown-value":           func(v *SourceOriginContextResult) { v.Physical.State = EvidenceUnknown; v.Physical.Reason = "unknown" },
		"invalid-cause": func(v *SourceOriginContextResult) {
			v.Specified = Evidence[bool]{State: EvidenceInvalid, Reason: "invalid"}
		},
		"wrong-site-document": func(v *SourceOriginContextResult) { v.Physical.Value.DocumentID = "other" },
		"over-cap-ancestry":   func(v *SourceOriginContextResult) { v.Ancestry = make([]SourceOriginStep, SourceOriginAncestryLimit+1) },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			_, s, key := sourceOriginFixture(t, Budgets{})
			defer s.cleanup()
			ctx := context.Background()
			h := schemaPendingHeader()
			h.Location = sourceOriginPhysical().Physical.Value
			if err := s.Open(ctx, h); err != nil {
				t.Fatal(err)
			}
			in := sourceOriginPhysical()
			change(&in)
			before := s.retained
			if err := s.SourceOrigin(ctx, key, in); err == nil {
				t.Fatal("accepted malformed origin")
			}
			if len(s.contextResults) != 0 || len(s.sourceOriginSeen) != 0 || s.retained != before {
				t.Fatal("partial malformed publication")
			}
		})
	}
}
func TestSourceOriginProviderAndPhaseGates(t *testing.T) {
	for _, mode := range []string{"wrong-provider", "closed", "duplicate", "absent-attribute", "unselected"} {
		t.Run(mode, func(t *testing.T) {
			_, s, key := sourceOriginFixture(t, Budgets{})
			defer s.cleanup()
			ctx := context.Background()
			h := schemaPendingHeader()
			h.Location = sourceOriginPhysical().Physical.Value
			if err := s.Open(ctx, h); err != nil {
				t.Fatal(err)
			}
			in := sourceOriginPhysical()
			switch mode {
			case "wrong-provider":
				key.Revision++
			case "closed":
				if err := s.Close(ctx, schemaLexemeEnd("1")); err != nil {
					t.Fatal(err)
				}
			case "duplicate":
				if err := s.SourceOrigin(ctx, key, in); err != nil {
					t.Fatal(err)
				}
			case "absent-attribute":
				name := QName{Local: "x"}
				in.Attribute = &name
				a := sourceOriginAttribute(name, h.Location)
				a.Presence = stateAbsent[bool]()
				a.Scalar = Scalar{Lexeme: stateAbsent[string](), Value: stateAbsent[Atomic]()}
				if err := s.Attribute(ctx, a); err != nil {
					t.Fatal(err)
				}
			case "unselected":
				s.catalog.sourceOrigins = nil
			}
			if err := s.SourceOrigin(ctx, key, in); err == nil {
				t.Fatal("accepted wrong provider or lifecycle")
			}
		})
	}
}
func TestSourceOriginSealCoverageAndAvailability(t *testing.T) {
	for _, mode := range []string{"missing-origin", "early", "double", "unknown", "unsupported", "known-false", "unknown-presence", "incomplete-input"} {
		t.Run(mode, func(t *testing.T) {
			_, s, key := sourceOriginFixture(t, Budgets{})
			ctx := context.Background()
			h := schemaPendingHeader()
			h.Location = sourceOriginPhysical().Physical.Value
			if err := s.Open(ctx, h); err != nil {
				t.Fatal(err)
			}
			if mode != "missing-origin" {
				if err := s.SourceOrigin(ctx, key, sourceOriginPhysical()); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "early" {
				if err := s.CompleteSourceOrigins(ctx, key, stateKnown(true)); err == nil {
					t.Fatal("early seal")
				}
				s.cleanup()
				return
			}
			if mode == "unknown-presence" {
				a := sourceOriginAttribute(QName{Local: "x"}, h.Location)
				a.Presence = stateUnknown[bool]("not-enumerated")
				if err := s.Attribute(ctx, a); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.Close(ctx, schemaLexemeEnd("1")); err != nil {
				t.Fatal(err)
			}
			complete := stateKnown(true)
			if mode == "unknown" {
				complete = stateUnknown[bool]("reader-origin-gap")
			}
			if mode == "unsupported" {
				complete = Evidence[bool]{State: EvidenceUnsupported, Reason: "reader-origin-gap"}
			}
			if mode == "known-false" {
				complete = stateKnown(false)
			}
			err := s.CompleteSourceOrigins(ctx, key, complete)
			if mode == "missing-origin" {
				if !errors.Is(err, ErrInvalidConfiguration) {
					t.Fatalf("missing coverage %v", err)
				}
				s.cleanup()
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if mode == "double" {
				if err := s.CompleteSourceOrigins(ctx, key, complete); err == nil {
					t.Fatal("double seal")
				}
				s.cleanup()
				return
			}
			r, err := s.Finish(ctx, Completion{InputComplete: mode != "incomplete-input", Syntax: SyntaxWellFormed})
			if err != nil {
				t.Fatal(err)
			}
			if r.AssessmentComplete || r.Satisfied() {
				t.Fatalf("unavailable origin completed %+v", r)
			}
			found := false
			for _, f := range r.Findings {
				if f.Decision.Rule == key {
					found = true
					if f.Decision.Outcome != OutcomeUnknown && f.Decision.Outcome != OutcomeUnsupported {
						t.Fatalf("outcome %+v", f)
					}
				}
			}
			if !found {
				t.Fatal("origin gap disappeared")
			}
		})
	}
}
func sourceOriginPublication() ContextResult {
	p := sourceOriginPhysical()
	return ContextResult{Provider: RuleKey{"origin", 1}, Subject: p.Subject, Property: ContextPropertySourceOrigin, Value: stateAbsent[Atomic](), SourceOrigin: &p}
}
func TestSourceOriginPublicationExactBudgetsAndCancellation(t *testing.T) {
	in := sourceOriginPublication()
	ctx := context.Background()
	probe := rationalSession(t, Budgets{})
	cancelProbe := &rationalCancelContext{Context: ctx, at: 1 << 30}
	if err := probe.publishContextResult(cancelProbe, in); err != nil {
		t.Fatal(err)
	}
	bytes := rationalUsage(t, probe, BudgetBufferedBytes)
	steps := rationalUsage(t, probe, BudgetRuleSteps)
	calls := cancelProbe.calls
	probe.cleanup()
	for _, kind := range []BudgetKind{BudgetBufferedBytes, BudgetRuleSteps} {
		for _, delta := range []int64{-1, 0} {
			limits := Budgets{}
			if kind == BudgetBufferedBytes {
				limits.MaxBufferedBytes = bytes + delta
			} else {
				limits.MaxRuleSteps = steps + delta
			}
			s := rationalSession(t, limits)
			err := s.publishContextResult(ctx, in)
			if delta == 0 {
				if err != nil || len(s.contextResults) != 1 {
					t.Fatalf("exact %s: %v", kind, err)
				}
			} else if !errors.Is(err, ErrResourceLimit) || len(s.contextResults) != 0 || s.retained != 0 {
				t.Fatalf("one-under %s: %v results=%d retained=%d", kind, err, len(s.contextResults), s.retained)
			}
			s.cleanup()
		}
	}
	for at := 1; at <= calls; at++ {
		s := rationalSession(t, Budgets{})
		c := &rationalCancelContext{Context: ctx, at: at}
		err := s.publishContextResult(c, in)
		if !errors.Is(err, context.Canceled) || len(s.contextResults) != 0 || s.retained != 0 {
			t.Fatalf("cancel %d/%d: %v results=%d retained=%d", at, calls, err, len(s.contextResults), s.retained)
		}
		s.cleanup()
	}
}

func sourceOriginCatalogInputs(t *testing.T) ([]RuleDefinition, []Profile, []PlanBinding, RuleKey) {
	t.Helper()
	e, s, key := sourceOriginFixture(t, Budgets{})
	defer s.cleanup()
	p, _ := e.Profile("source-schema-test")
	p.CatalogDigest = ""
	var r []RuleDefinition
	var b []PlanBinding
	for _, k := range p.Required {
		rule, _ := e.catalog.Rule(k)
		r = append(r, rule)
		b = append(b, PlanBinding{ID: rule.Predicate, Plan: e.catalog.plans[rule.Predicate]})
	}
	return r, []Profile{p}, b, key
}
func TestSourceOriginCatalogSelectionIsSealed(t *testing.T) {
	for _, mode := range []string{"model", "node", "predicate", "dependency", "selector-mismatch", "missing-schema", "duplicate", "unsupported", "unselected", "forged-generic"} {
		t.Run(mode, func(t *testing.T) {
			r, p, b, key := sourceOriginCatalogInputs(t)
			idx := len(r) - 1
			switch mode {
			case "model":
				r[idx].Targets = []Target{TargetSource, TargetModel}
			case "node":
				r[idx].Scope = ScopeNode
			case "predicate":
				r[idx].Role = RolePredicate
			case "dependency":
				r[idx].Requires = []Dependency{{Kind: DependencyRule, Rule: r[0].Key, Relation: RelationDocument}}
			case "selector-mismatch":
				a := AssemblyOpus
				r[idx].Selector.Assembly = &a
			case "missing-schema":
				r = r[1:]
				b = b[1:]
				p[0].Required = p[0].Required[1:]
				p[0].Support = p[0].Support[1:]
			case "duplicate":
				duplicate := cloneRule(r[idx])
				duplicate.Key.ID = "other.origin"
				r = append(r, duplicate)
				p[0].Required = append(p[0].Required, duplicate.Key)
				p[0].Support = append(p[0].Support, RuleSupport{Rule: duplicate.Key, State: ImplementationAvailable})
			case "unsupported":
				p[0].Support[idx].State = ImplementationUnsupported
				p[0].Support[idx].Reason = "not-supported"
			case "unselected":
				p[0].Required = p[0].Required[:idx]
				p[0].Support = p[0].Support[:idx]
			case "forged-generic":
				generic, err := NewContextValuePlan("source.origin", RelationDocument, Atomic{Kind: AtomicString, Text: "forged"})
				if err != nil {
					t.Fatal(err)
				}
				b[idx].Plan = generic
				r[idx].Selector = Selector{Declaration: &DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "test", Occurrence: "root"}}
			}
			c, err := NewCatalog(r, p, b)
			if mode != "unsupported" && mode != "unselected" && mode != "forged-generic" {
				if !errors.Is(err, ErrInvalidConfiguration) {
					t.Fatalf("accepted incompatible %s: %v", mode, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			e, err := NewEngine(c)
			if err != nil {
				t.Fatal(err)
			}
			if got, ok := e.SourceOriginProvider(p[0].ID, AssemblyMusicXML); ok || got != (RuleKey{}) {
				t.Fatalf("nonavailable plan selected %v %v, original %v", got, ok, key)
			}
		})
	}
	for _, assembly := range []Assembly{"", "unknown"} {
		if _, err := NewSourceOriginPlan(assembly); !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("invalid assembly accepted %q: %v", assembly, err)
		}
	}
}
func TestSourceOriginIngressCancellationAndExactContextFactLimit(t *testing.T) {
	_, probe, key := sourceOriginFixture(t, Budgets{})
	h := schemaPendingHeader()
	h.Location = sourceOriginPhysical().Physical.Value
	if err := probe.Open(context.Background(), h); err != nil {
		t.Fatal(err)
	}
	counter := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	if err := probe.SourceOrigin(counter, key, sourceOriginPhysical()); err != nil {
		t.Fatal(err)
	}
	calls := counter.calls
	probe.cleanup()
	for at := 1; at <= calls; at++ {
		_, s, key := sourceOriginFixture(t, Budgets{})
		if err := s.Open(context.Background(), h); err != nil {
			t.Fatal(err)
		}
		before := s.retained
		err := s.SourceOrigin(&rationalCancelContext{Context: context.Background(), at: at}, key, sourceOriginPhysical())
		if !errors.Is(err, context.Canceled) || len(s.contextResults) != 0 || len(s.sourceOriginSeen) != 0 || s.retained != before {
			t.Fatalf("ingress cancel %d/%d: %v retained %d/%d", at, calls, err, s.retained, before)
		}
		s.cleanup()
	}
	_, s, key := sourceOriginFixture(t, Budgets{MaxContextFacts: 1})
	defer s.cleanup()
	ctx := context.Background()
	if err := s.Open(ctx, h); err != nil {
		t.Fatal(err)
	}
	if err := s.SourceOrigin(ctx, key, sourceOriginPhysical()); err != nil {
		t.Fatal(err)
	}
	a := sourceOriginAttribute(QName{Local: "x"}, h.Location)
	if err := s.Attribute(ctx, a); err != nil {
		t.Fatal(err)
	}
	in := sourceOriginPhysical()
	in.Attribute = &a.Name
	before := s.retained
	if err := s.SourceOrigin(ctx, key, in); !errors.Is(err, ErrResourceLimit) || len(s.contextResults) != 1 || len(s.sourceOriginSeen) != 1 || s.retained != before {
		t.Fatalf("fact cap: %v", err)
	}
}
func TestSourceOriginPublicationRejectsMismatchedPayload(t *testing.T) {
	for _, mode := range []string{"missing", "wrong-property", "two-payloads", "wrong-subject"} {
		t.Run(mode, func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			in := sourceOriginPublication()
			switch mode {
			case "missing":
				in.SourceOrigin = nil
			case "wrong-property":
				in.Property = ContextPropertySpanNormalDots
			case "two-payloads":
				in.Span = &SpanContextResult{}
			case "wrong-subject":
				in.SourceOrigin.Subject.Node++
			}
			if err := s.publishContextResult(context.Background(), in); !errors.Is(err, ErrInvalidConfiguration) || len(s.contextResults) != 0 || s.retained != 0 {
				t.Fatalf("mismatch accepted: %v", err)
			}
		})
	}
}

func TestSourceOriginUnavailableComponentsRemainIncomplete(t *testing.T) {
	for _, mode := range []string{"kind", "declaration", "ancestry", "specified"} {
		t.Run(mode, func(t *testing.T) {
			_, s, key := sourceOriginFixture(t, Budgets{})
			ctx := context.Background()
			h := schemaPendingHeader()
			h.Location = sourceOriginPhysical().Physical.Value
			if err := s.Open(ctx, h); err != nil {
				t.Fatal(err)
			}
			in := sourceOriginPhysical()
			switch mode {
			case "kind":
				in.Kind = SourceOriginUnavailable
			case "declaration":
				in.Declaration = stateUnknown[Location]("declaration-not-read")
			case "ancestry":
				in.Kind = SourceOriginEntity
				in.Declaration = in.Physical
				in.Use = in.Physical
				in.AncestryComplete = stateUnknown[bool]("expansion-chain-incomplete")
			case "specified":
				in.Kind = SourceOriginUnavailable
				in.Specified = stateUnknown[bool]("specifiedness-unavailable")
			}
			if err := s.SourceOrigin(ctx, key, in); err != nil {
				t.Fatal(err)
			}
			if err := s.Close(ctx, schemaLexemeEnd("1")); err != nil {
				t.Fatal(err)
			}
			if err := s.CompleteSourceOrigins(ctx, key, stateKnown(true)); err != nil {
				t.Fatal(err)
			}
			r, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
			if err != nil {
				t.Fatal(err)
			}
			if r.AssessmentComplete || len(r.Findings) != 1 || r.Findings[0].Decision.Reason != "source-origin-components-unavailable" || len(r.Context) != 1 {
				t.Fatalf("gap lost %+v", r)
			}
		})
	}
}
func TestSourceOriginPhysicalDeclaredAttributeAndEntityValue(t *testing.T) {
	for _, entity := range []bool{false, true} {
		_, s, key := sourceOriginFixture(t, Budgets{})
		ctx := context.Background()
		h := schemaPendingHeader()
		h.Location = sourceOriginPhysical().Physical.Value
		if err := s.Open(ctx, h); err != nil {
			t.Fatal(err)
		}
		name := QName{Local: "x"}
		a := sourceOriginAttribute(name, h.Location)
		if err := s.Attribute(ctx, a); err != nil {
			t.Fatal(err)
		}
		in := sourceOriginPhysical()
		in.Attribute = &name
		in.Declaration = in.Physical
		in.Declaration.Value = cloneLocation(in.Physical.Value)
		in.Declaration.Value.Position.ByteOffset = 2
		if entity {
			in.Kind = SourceOriginEntity
			in.Use = in.Physical
			in.Ancestry = []SourceOriginStep{{Entity: "e", Declaration: in.Declaration, Use: in.Use}}
		}
		if err := s.SourceOrigin(ctx, key, in); err != nil {
			t.Fatalf("declared physical attribute entity=%v: %v", entity, err)
		}
		if !s.contextResults[0].SourceOrigin.Specified.Value {
			t.Fatal("lost physical specifiedness")
		}
		s.cleanup()
	}
}
func TestSourceOriginAncestryBoundaryAndComponentCauses(t *testing.T) {
	in := sourceOriginPublication()
	v := in.SourceOrigin
	v.Kind = SourceOriginEntity
	v.Declaration = v.Physical
	v.Use = v.Physical
	v.Ancestry = make([]SourceOriginStep, SourceOriginAncestryLimit)
	for i := range v.Ancestry {
		v.Ancestry[i] = SourceOriginStep{Entity: "entity", Declaration: v.Declaration, Use: v.Use}
	}
	s := rationalSession(t, Budgets{})
	if err := s.publishContextResult(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	s.cleanup()
	for _, change := range []func(*SourceOriginContextResult){
		func(v *SourceOriginContextResult) { v.AncestryComplete = stateKnown(false) },
		func(v *SourceOriginContextResult) { v.Ancestry = nil },
		func(v *SourceOriginContextResult) {
			v.Ancestry[0].Declaration = Evidence[Location]{State: EvidenceInvalid, Reason: "invalid"}
		},
	} {
		bad := stateClone(in)
		change(bad.SourceOrigin)
		s := rationalSession(t, Budgets{})
		if err := s.publishContextResult(context.Background(), bad); !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("invalid ancestry %v", err)
		}
		s.cleanup()
	}
	cause := &DecisionRef{Rule: RuleKey{"reader", 1}, Subject: NodeRef{"source", 1}}
	v.Ancestry[0].Declaration = Evidence[Location]{State: EvidenceInvalid, Reason: "invalid-declaration", Cause: cause}
	v.AncestryComplete = Evidence[bool]{State: EvidenceKnown, Value: false, Reason: "ancestry-incomplete"}
	s = rationalSession(t, Budgets{})
	if err := s.publishContextResult(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	cause.Rule.ID = "changed"
	if s.contextResults[0].SourceOrigin.Ancestry[0].Declaration.Cause.Rule.ID != "reader" {
		t.Fatal("cause aliased")
	}
	s.cleanup()
}

func TestSourceOriginEventAndScalarBoundaries(t *testing.T) {
	for _, events := range []int64{1, 2} {
		_, s, key := sourceOriginFixture(t, Budgets{MaxEvents: events})
		ctx := context.Background()
		h := schemaPendingHeader()
		h.Location = sourceOriginPhysical().Physical.Value
		if err := s.Open(ctx, h); err != nil {
			t.Fatal(err)
		}
		before := s.retained
		err := s.SourceOrigin(ctx, key, sourceOriginPhysical())
		if events == 2 {
			if err != nil {
				t.Fatal(err)
			}
		} else if !errors.Is(err, ErrResourceLimit) || len(s.contextResults) != 0 || s.retained != before {
			t.Fatalf("event cap: %v", err)
		}
		s.cleanup()
	}
	in := sourceOriginPublication()
	v := in.SourceOrigin
	v.Kind = SourceOriginEntity
	v.Declaration = v.Physical
	v.Use = v.Physical
	v.Ancestry = []SourceOriginStep{{Entity: "entity", Declaration: v.Declaration, Use: v.Use}}
	for _, limit := range []int64{5, 6} {
		s := rationalSession(t, Budgets{MaxScalarBytes: limit})
		err := s.publishContextResult(context.Background(), in)
		if limit == 6 {
			if err != nil {
				t.Fatal(err)
			}
		} else if !errors.Is(err, ErrResourceLimit) || len(s.contextResults) != 0 || s.retained != 0 {
			t.Fatalf("scalar cap: %v", err)
		}
		s.cleanup()
	}
}
func TestSourceOriginSyntheticPositionAndClosedOwnerAreRejected(t *testing.T) {
	for _, mode := range []string{"synthetic-position", "closed-child"} {
		t.Run(mode, func(t *testing.T) {
			_, s, key := sourceOriginFixture(t, Budgets{})
			defer s.cleanup()
			ctx := context.Background()
			h := schemaPendingHeader()
			h.Location = sourceOriginPhysical().Physical.Value
			if err := s.Open(ctx, h); err != nil {
				t.Fatal(err)
			}
			in := sourceOriginPhysical()
			if mode == "synthetic-position" {
				in.Kind = SourceOriginEntity
				in.Specified = stateKnown(false)
				in.Declaration = in.Physical
				in.Use = in.Physical
				in.Physical = stateAbsent[Location]()
				in.Ancestry = []SourceOriginStep{{Entity: "entity", Declaration: in.Declaration, Use: in.Use}}
			} else {
				child := h
				child.ID = 2
				child.Parent = 1
				if err := s.Open(ctx, child); err != nil {
					t.Fatal(err)
				}
				end := schemaLexemeEnd("1")
				end.ID = 2
				if err := s.Close(ctx, end); err != nil {
					t.Fatal(err)
				}
				in.Subject.Node = 2
			}
			if err := s.SourceOrigin(ctx, key, in); err == nil || len(s.contextResults) != 0 {
				t.Fatalf("contradiction accepted %v", err)
			}
		})
	}
}

func TestSourceOriginTerminalPrefixSurvivesFinish(t *testing.T) {
	_, s, key := sourceOriginFixture(t, Budgets{})
	ctx := context.Background()
	h := schemaPendingHeader()
	h.Location = sourceOriginPhysical().Physical.Value
	if err := s.Open(ctx, h); err != nil {
		t.Fatal(err)
	}
	in := sourceOriginPhysical()
	if err := s.SourceOrigin(ctx, key, in); err != nil {
		t.Fatal(err)
	}
	in.Physical.Value.Position.ByteOffset = 999
	cause := errors.New("truncated XML")
	r, err := s.Finish(ctx, Completion{InputComplete: false, Syntax: SyntaxMalformed, Cause: cause})
	if !errors.Is(err, cause) || r.AssessmentComplete || len(r.Context) != 1 || r.Context[0].SourceOrigin.Physical.Value.Position.ByteOffset != 12 || s.contextResults != nil {
		t.Fatalf("prefix lost %+v %v", r, err)
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("cleanup leaked %d", used)
	}
}
