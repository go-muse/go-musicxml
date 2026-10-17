package validation

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"
)

func soundsTestFixture(t *testing.T) (*Session, *contextPlan) {
	t.Helper()
	s := rationalSession(t, Budgets{})
	s.documentID = "sounds-source"
	owner, child := localNode(1, 0, "sound"), localNode(2, 1, "ensemble")
	b := &contextBindings{Elements: map[contextRole]map[DeclarationID]struct{}{}, Attributes: map[contextRole]map[ComponentID]AttributeSelector{}}
	for i, n := range []*factNode{owner, child} {
		n.header.Declaration.Value.Assembly = AssemblySounds
		n.header.Type = testEvidence(Evidence[ComponentID]{State: EvidenceKnown, Value: contextAssemblyType(AssemblySounds, []string{"sound", "ensemble"}[i])})
		n.header.Location = Location{Target: TargetSource, DocumentID: s.documentID, Path: "/sounds/sound", Position: &SourcePosition{Line: 1, Column: 2}}
		role := []contextRole{soundsSound, soundsEnsemble}[i]
		b.Elements[role] = map[DeclarationID]struct{}{n.header.Declaration.Value: {}}
	}
	for _, v := range []struct {
		n          *factNode
		role       contextRole
		name, text string
	}{{owner, soundsID, "id", "example"}, {owner, soundsSoundPrimary, "primary", "yes"}, {child, soundsEnsemblePrimary, "primary", "no"}, {child, soundsNumber, "number", "many:players"}} {
		a := localAttr(v.name)
		a.Declaration.Assembly = AssemblySounds
		b.Attributes[v.role] = map[ComponentID]AttributeSelector{v.n.header.Type.Value: a}
		v.n.attributes = append(v.n.attributes, AttributeFact{Owner: v.n.header.ID, Name: a.Name, Declaration: Evidence[DeclarationID]{State: EvidenceKnown, Value: a.Declaration}, Presence: Evidence[bool]{State: EvidenceKnown, Value: true}, Scalar: Scalar{Value: localKnownText(v.text)}, Location: v.n.header.Location})
	}
	child.end.Scalar.Value = testEvidence(localKnownText(" library  value "))
	owner.children = []*factNode{child}
	s.nodes = map[NodeID]*factNode{1: owner, 2: child}
	s.sequence = []NodeID{1, 2}
	return s, &contextPlan{Bindings: b}
}

func TestSoundsMappingUnavailableEvidence(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		s, p := soundsTestFixture(t)
		cause := &DecisionRef{Rule: RuleKey{"source-cause", 1}, Subject: NodeRef{s.documentID, 2}}
		missing := Evidence[Atomic]{State: state, Reason: "source-value-gap", Cause: cause}
		s.nodes[1].attributes[0].Scalar.Value = missing
		s.nodes[1].attributes[1].Scalar.Value = missing
		s.nodes[2].attributes[0].Scalar.Value = missing
		s.nodes[2].attributes[1].Scalar.Value = missing
		s.nodes[2].end.Scalar.Value = testEvidence(missing)
		s.nodes[1].end.ChildrenComplete = testEvidence(Evidence[bool]{State: state, Reason: "source-enumeration-gap", Cause: cause})
		d, pending, err := evaluateContextSoundsMapping(context.Background(), s, nil, RuleKey{"sounds-test", 1}, p, true)
		if err != nil || pending || d.Outcome != OutcomePass || len(s.contextResults) != 2 {
			t.Fatalf("projection failed: %+v %v", d, err)
		}
		for i, row := range s.contextResults {
			v := row.SoundMapping
			values := []Evidence[Atomic]{v.SoundID, v.Primary}
			if i == 1 {
				values = append(values, v.LibraryID, v.Number)
			}
			for _, e := range values {
				if e.State != state || e.Cause == nil || e.Cause.Rule.ID != "source-cause" {
					t.Errorf("unavailable %s replaced: %+v", state, e)
				}
			}
			if v.Content.State != state || v.Content.Cause == nil {
				t.Errorf("content gap lost: %+v", v.Content)
			}
		}
		cause.Rule.ID = "caller"
		if s.contextResults[1].SoundMapping.Number.Cause.Rule.ID != "source-cause" {
			t.Fatal("cause alias")
		}
		s.cleanup()
	}
}

func TestSoundsMappingExactDeclarationAndCancellation(t *testing.T) {
	probe, p := soundsTestFixture(t)
	ctx := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	if _, _, err := evaluateContextSoundsMapping(ctx, probe, nil, RuleKey{"sounds-test", 1}, p, true); err != nil {
		t.Fatal(err)
	}
	polls := ctx.calls
	probe.cleanup()
	for at := 1; at <= polls; at++ {
		s, p := soundsTestFixture(t)
		_, _, err := evaluateContextSoundsMapping(&rationalCancelContext{Context: context.Background(), at: at}, s, nil, RuleKey{"sounds-test", 1}, p, true)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation %d lost: %v", at, err)
		}
		for _, row := range s.contextResults {
			if row.SoundMapping == nil || row.SoundMapping.SoundID.State != EvidenceKnown {
				t.Fatal("partial payload escaped")
			}
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("canceled provider retained bytes")
		}
	}
	s, p := soundsTestFixture(t)
	s.nodes[2].header.Declaration.Value.Assembly = AssemblyMusicXML
	if _, _, err := evaluateContextSoundsMapping(context.Background(), s, nil, RuleKey{"sounds-test", 1}, p, true); err != nil {
		t.Fatal(err)
	}
	if len(s.contextResults) != 1 {
		t.Fatal("score ensemble homonym selected as sounds")
	}
	s.cleanup()
	t.Logf("all %d provider cancellation polls checked", polls)
}

func TestSoundsMappingOwnedPayloadAndAdmission(t *testing.T) {
	payload := SoundMappingContextResult{}
	independentContextFill(reflect.ValueOf(&payload).Elem(), "original")
	in := ContextResult{Provider: RuleKey{"sounds-test", 1}, Subject: NodeRef{"source", 1}, Property: ContextPropertySoundMapping, Value: stateAbsent[Atomic](), SoundMapping: &payload}
	probe := rationalSession(t, Budgets{})
	ctx := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	if err := probe.publishContextResult(ctx, in); err != nil {
		t.Fatal(err)
	}
	bytes, steps, polls := rationalUsage(t, probe, BudgetBufferedBytes), rationalUsage(t, probe, BudgetRuleSteps), ctx.calls
	before, _ := json.Marshal(probe.contextResults)
	independentContextFill(reflect.ValueOf(&payload).Elem(), "caller")
	after, _ := json.Marshal(probe.contextResults)
	if string(before) != string(after) {
		t.Fatal("sound projection aliases caller fields")
	}
	independentContextFill(reflect.ValueOf(&payload).Elem(), "original")
	probe.cleanup()
	for _, limits := range []Budgets{{MaxBufferedBytes: bytes - 1}, {MaxRuleSteps: steps - 1}} {
		s := rationalSession(t, limits)
		if err := s.publishContextResult(context.Background(), in); !errors.Is(err, ErrResourceLimit) || len(s.contextResults) != 0 || s.retained != 0 {
			t.Fatalf("one-under admission: %v bytes=%d", err, s.retained)
		}
		s.cleanup()
	}
	exact := rationalSession(t, Budgets{MaxBufferedBytes: bytes, MaxRuleSteps: steps, MaxContextFacts: 1})
	if err := exact.publishContextResult(context.Background(), in); err != nil {
		t.Fatalf("exact admission: %v", err)
	}
	exact.cleanup()
	for at := 1; at <= polls; at++ {
		s := rationalSession(t, Budgets{})
		if err := s.publishContextResult(&rationalCancelContext{Context: context.Background(), at: at}, in); !errors.Is(err, context.Canceled) || len(s.contextResults) != 0 || s.retained != 0 {
			t.Fatalf("poll %d: %v bytes=%d", at, err, s.retained)
		}
		s.cleanup()
	}
	for _, mutate := range []func(*ContextResult){func(r *ContextResult) { r.SoundMapping = nil }, func(r *ContextResult) { r.Property = ContextPropertySpanNormalDots }, func(r *ContextResult) { r.Span = &SpanContextResult{} }} {
		s := rationalSession(t, Budgets{})
		bad := in
		mutate(&bad)
		if err := s.publishContextResult(context.Background(), bad); !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("closed payload union accepted: %v", err)
		}
		s.cleanup()
	}
	t.Logf("exact bytes=%d steps=%d; all %d publication cancellation polls", bytes, steps, polls)
}
