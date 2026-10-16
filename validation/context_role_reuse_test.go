package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// This fixture reaches the actual sealed schema-binding phase before handing the
// reader the same immutable facts that final context providers consume.
func contextRoleReuseFixture(t *testing.T) (*Session, *contextBindings, *factNode) {
	t.Helper()
	return contextRoleRepeatedFixture(t, 1)
}

func contextRoleRepeatedFixture(t *testing.T, count int) (*Session, *contextBindings, *factNode) {
	t.Helper()
	owner := strings.Repeat("long-owner-", 80)
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{
		Types:    map[SchemaName]*SchemaType{{Local: owner}: {Complex: &SchemaComplex{Form: SchemaComplexDirect, Particle: &SchemaParticle{Kind: SchemaParticleElement, Occurrence: SchemaOccurrence{Min: uint64(count), Max: uint64(count)}, Element: &SchemaElement{Name: SchemaName{Local: "item"}, Type: localBuiltin("string")}}}}},
		Elements: map[SchemaName]*SchemaElement{{Local: "root"}: {Name: SchemaName{Local: "root"}, Type: localType(owner)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := compileContextBindingSpecs(schema, []contextBindingSpec{
		{Role: "item-role", Owner: owner, Name: "item", Type: "xs:string", Count: 1},
		{Role: "item-alias", Owner: owner, Name: "item", Type: "xs:string", Count: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := contextRoleSchemaSession(t, schema, count)
	return s, b, s.nodes[2]
}

func contextRoleSchemaSession(t *testing.T, schema *CompiledSchema, count int) *Session {
	t.Helper()
	s, err := sourceSchemaEngineFor(t, schema, XSDCheckContent).Begin(context.Background(), RunOptions{DocumentID: "source", Profile: "source-schema-test"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Open(context.Background(), schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	for id := NodeID(2); id < NodeID(count)+2; id++ {
		h := schemaPendingHeader()
		h.ID, h.Parent, h.Name.Local = id, 1, "item"
		if err = s.Open(context.Background(), h); err != nil {
			t.Fatal(err)
		}
		end := schemaLexemeEnd("value")
		end.ID = id
		if err = s.Close(context.Background(), end); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.Close(context.Background(), schemaLexemeEnd("")); err != nil {
		t.Fatal(err)
	}
	// Finish sets finished before finalizing the schema and evaluating providers;
	// perform just that prelude here so the reader can be examined before cleanup.
	s.finished, s.inputComplete = true, true
	if err = s.finalizeSourceSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestContextRoleReuseCompiledColdAndWarm(t *testing.T) {
	for _, root := range []bool{false, true} {
		s, b, n := contextRoleReuseFixture(t)
		if root {
			n = s.root // A known declaration with no roles needs a cached empty set.
		}
		r := &contextReader{ctx: context.Background(), s: s, b: b}
		beforeSteps, beforeBytes := rationalUsage(t, s, BudgetRuleSteps), s.retained
		if got := r.matches(n, "item-role"); got != !root || r.err != nil {
			t.Fatalf("cold root=%v result=%v: %v", root, got, r.err)
		}
		cold := rationalUsage(t, s, BudgetRuleSteps) - beforeSteps
		retained := s.retained - beforeBytes
		if retained <= 0 || retained > 768 {
			t.Errorf("root=%v cache retained %d bytes, want one compact accounted entry", root, retained)
		}
		beforeSteps = rationalUsage(t, s, BudgetRuleSteps)
		for i := 0; i < 100; i++ {
			if got := r.matches(n, "item-role"); got != !root {
				t.Fatal("cached positive classification changed")
			}
			if r.matches(n, "absent-role") {
				t.Fatal("cached negative classification changed")
			}
		}
		warm := rationalUsage(t, s, BudgetRuleSteps) - beforeSteps
		if r.err != nil || warm >= 200*cold/2 {
			t.Errorf("root=%v repeated lookup redoes declaration work: cold=%d warm/200=%g err=%v", root, cold, float64(warm)/200, r.err)
		}
		if s.retained-beforeBytes != retained {
			t.Fatal("repeated role queries retained per-query entries")
		}
		t.Logf("root=%v declaration_bytes=%d cold=%d warm/200=%g retained=%d", root, contextDeclarationBytes(n.header.Declaration.Value), cold, float64(warm)/200, retained)
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("cleanup leaked role cache bytes")
		}
	}
}

func TestContextRoleReuseScopeAndEvidence(t *testing.T) {
	s, b, n := contextRoleReuseFixture(t)
	defer s.cleanup()
	r := &contextReader{ctx: context.Background(), s: s, b: b}
	if !r.matches(n, "item-role") {
		t.Fatal("fixture role missing")
	}
	known := *n.header.Declaration
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceInvalid, EvidenceAbsent, EvidenceUnsupported} {
		n.header.Declaration.State = state
		if r.matches(n, "item-role") || r.err != nil {
			t.Fatalf("cache bypassed evidence guard %v: %v", state, r.err)
		}
	}
	n.header.Declaration = testEvidence(known)
	if !r.matches(n, "item-role") {
		t.Fatal("known evidence did not restore exact membership")
	}
	// Distinct compiled bindings may select different roles for identical uses.
	other, err := compileContextBindingSpecs(s.schema.active, []contextBindingSpec{{Role: "other-role", Owner: b.Roles["item-role"][0].OwnerType.Name.Local, Name: "item", Type: "xs:string", Count: 1}})
	if err != nil {
		t.Fatal(err)
	}
	r.b = other
	if r.matches(n, "item-role") || !r.matches(n, "other-role") || r.err != nil {
		t.Fatalf("borrowed roles crossed bindings: %v", r.err)
	}
	r.b = b
	if !r.matches(n, "item-role") || r.matches(n, "other-role") {
		t.Fatal("return to original bindings selected stale roles")
	}
	// No final-phase certificate: late publication and private mutable fixtures
	// retain exact lookup behavior even when the same reader already ran.
	s.finished = false
	for _, declaration := range []DeclarationID{s.root.header.Declaration.Value, known.Value} {
		n.header.Declaration = testEvidence(stateKnown(declaration))
		if got := r.matches(n, "item-role"); got != (declaration == known.Value) {
			t.Fatal("nonfinal known declaration transition used cached classification")
		}
	}
	n.header.Declaration = testEvidence(stateUnknown[DeclarationID]("pending"))
	if r.matches(n, "item-role") {
		t.Fatal("pending declaration selected a role")
	}
	n.header.Declaration = testEvidence(known)
	if !r.matches(n, "item-role") || r.err != nil {
		t.Fatal("published known declaration retained a negative result")
	}
}

func TestContextRoleReuseExactWorkAndCancellation(t *testing.T) {
	for _, warm := range []bool{false, true} {
		probe, b, n := contextRoleReuseFixture(t)
		r := &contextReader{ctx: context.Background(), s: probe, b: b}
		if warm && !r.matches(n, "item-role") {
			t.Fatal(r.err)
		}
		before := rationalUsage(t, probe, BudgetRuleSteps)
		polls := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
		r.ctx = polls
		if !r.matches(n, "item-role") || r.err != nil {
			t.Fatal(r.err)
		}
		cost := rationalUsage(t, probe, BudgetRuleSteps) - before
		probe.cleanup()
		for _, limit := range []int64{cost - 1, cost} {
			s, b, n := contextRoleReuseFixture(t)
			r := &contextReader{ctx: context.Background(), s: s, b: b}
			if warm && !r.matches(n, "item-role") {
				t.Fatal(r.err)
			}
			remaining := s.account.Limits().MaxRuleSteps - rationalUsage(t, s, BudgetRuleSteps)
			if err := s.account.Charge(BudgetRuleSteps, remaining-limit); err != nil {
				t.Fatal(err)
			}
			got := r.matches(n, "item-role")
			if limit == cost {
				if !got || r.err != nil {
					t.Fatalf("warm=%v exact=%d: %v", warm, cost, r.err)
				}
			} else if got || !errors.Is(r.err, ErrResourceLimit) {
				t.Fatalf("warm=%v one-below=%d exposed result=%v err=%v", warm, limit, got, r.err)
			}
			s.cleanup()
		}
		for at := 1; at <= polls.calls; at++ {
			s, b, n := contextRoleReuseFixture(t)
			r := &contextReader{ctx: context.Background(), s: s, b: b}
			if warm && !r.matches(n, "item-role") {
				t.Fatal(r.err)
			}
			r.ctx = &rationalCancelContext{Context: context.Background(), at: at}
			if r.matches(n, "item-role") || !errors.Is(r.err, context.Canceled) {
				t.Fatalf("warm=%v cancel poll=%d: %v", warm, at, r.err)
			}
			r.ctx = context.Background()
			if r.matches(n, "item-role") || !errors.Is(r.err, context.Canceled) {
				t.Fatal("first cancellation was not sticky")
			}
			s.cleanup()
			if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatal("canceled cache construction leaked")
			}
		}
		t.Logf("warm=%v exact_work=%d polls=%d", warm, cost, polls.calls)
	}
}

func TestContextRoleReuseMutableAndManualFallback(t *testing.T) {
	for _, mode := range []string{"elements", "roles"} {
		b, n := contextRound3CompatFixture(mode)
		s := rationalSession(t, Budgets{})
		r := &contextReader{ctx: context.Background(), s: s, b: b}
		known := n.header.Declaration.Value
		if !r.matches(n, "role-a") || r.err != nil {
			t.Fatalf("%s manual exact match: %v", mode, r.err)
		}
		if got := rationalUsage(t, s, BudgetRuleSteps); got < contextDeclarationBytes(known) {
			t.Fatalf("%s long-key work unaccounted: %d", mode, got)
		}
		for _, change := range []func(*DeclarationID){
			func(d *DeclarationID) { d.Assembly = AssemblyOpus },
			func(d *DeclarationID) { d.OwnerOccurrence += "-other" },
			func(d *DeclarationID) { d.Occurrence += "-other" },
		} {
			n.header.Declaration = testEvidence(stateKnown(known))
			change(&n.header.Declaration.Value)
			if r.matches(n, "role-a") || r.err != nil {
				t.Fatalf("%s mutable exact identity reused stale result: %v", mode, r.err)
			}
		}
		n.header.Declaration = testEvidence(stateUnknown[DeclarationID]("pending"))
		if r.matches(n, "role-a") {
			t.Fatal("manual pending identity invented a role")
		}
		n.header.Declaration = testEvidence(stateKnown(known))
		if !r.matches(n, "role-a") || r.err != nil || s.retained != 0 {
			t.Fatalf("%s changed manual fallback behavior: %v", mode, r.err)
		}
		s.cleanup()
	}
}

func TestContextRoleReuseExactBufferAdmission(t *testing.T) {
	probe, b, n := contextRoleReuseFixture(t)
	before := probe.retained
	r := &contextReader{ctx: context.Background(), s: probe, b: b}
	if !r.matches(n, "item-role") || r.err != nil {
		t.Fatal(r.err)
	}
	cost := probe.retained - before
	probe.cleanup()
	if cost <= 0 {
		t.Fatal("compiled per-binding reuse must admit retained cache storage")
	}
	for _, limit := range []int64{cost - 1, cost} {
		s, b, n := contextRoleReuseFixture(t)
		before := s.retained
		remaining := s.account.Limits().MaxBufferedBytes - rationalUsage(t, s, BudgetBufferedBytes)
		padding := remaining - limit
		if err := s.account.Charge(BudgetBufferedBytes, padding); err != nil {
			t.Fatal(err)
		}
		r := &contextReader{ctx: context.Background(), s: s, b: b}
		got := r.matches(n, "item-role")
		if limit == cost {
			if !got || r.err != nil || s.retained-before != cost {
				t.Fatalf("exact buffer admission: result=%v retained=%d err=%v", got, s.retained-before, r.err)
			}
			if !r.matches(n, "item-role") || r.err != nil || s.retained-before != cost {
				t.Fatalf("warm lookup charged another cache entry: %v", r.err)
			}
		} else if got || !errors.Is(r.err, ErrResourceLimit) || s.retained != before {
			t.Fatalf("one-below buffer result=%v retained=%d err=%v", got, s.retained-before, r.err)
		}
		if err := s.account.Release(BudgetBufferedBytes, padding); err != nil {
			t.Fatal(err)
		}
		if limit != cost {
			retry := &contextReader{ctx: context.Background(), s: s, b: b}
			if !retry.matches(n, "item-role") || retry.err != nil {
				t.Fatalf("refused allocation poisoned shared account: %v", retry.err)
			}
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("buffer refusal/retry leaked retained bytes")
		}
	}
}
