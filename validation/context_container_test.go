package validation

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestContainerMetadataOwnedPayloadAndAdmission(t *testing.T) {
	payload := ContainerContextResult{}
	independentContextFill(reflect.ValueOf(&payload).Elem(), "original")
	in := ContextResult{Provider: RuleKey{"container-test", 1}, Subject: NodeRef{"source", 1}, Property: ContextPropertyContainerMetadata, Value: stateAbsent[Atomic](), Container: &payload}
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
		t.Fatal("container projection aliases caller fields")
	}
	independentContextFill(reflect.ValueOf(&payload).Elem(), "original")
	probe.cleanup()
	for _, limits := range []Budgets{{MaxBufferedBytes: bytes - 1}, {MaxRuleSteps: steps - 1}} {
		s := rationalSession(t, limits)
		if err := s.publishContextResult(context.Background(), in); !errors.Is(err, ErrResourceLimit) || len(s.contextResults) != 0 || s.retained != internCharge(s) {
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
		if err := s.publishContextResult(&rationalCancelContext{Context: context.Background(), at: at}, in); !errors.Is(err, context.Canceled) || len(s.contextResults) != 0 || s.retained != internCharge(s) {
			t.Fatalf("poll %d: %v bytes=%d", at, err, s.retained)
		}
		s.cleanup()
	}
	for _, mutate := range []func(*ContextResult){func(r *ContextResult) { r.Container = nil }, func(r *ContextResult) { r.Property = ContextPropertySpanNormalDots }, func(r *ContextResult) { r.Span = &SpanContextResult{} }} {
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

func containerTestFixture(t *testing.T) (*Session, *contextPlan) {
	t.Helper()
	s := rationalSession(t, Budgets{})
	s.documentID = "container-source"
	root, files, first, second := localNode(1, 0, "container"), localNode(2, 1, "rootfiles"), localNode(3, 2, "rootfile"), localNode(4, 2, "rootfile")
	b := &contextBindings{Elements: map[contextRole]map[DeclarationID]struct{}{}, Attributes: map[contextRole]map[ComponentID]AttributeSelector{}}
	for i, n := range []*factNode{root, files, first, second} {
		n.header.Declaration.Value.Assembly = AssemblyContainer
		n.header.Type = testEvidence(Evidence[ComponentID]{State: EvidenceKnown, Value: contextAssemblyType(AssemblyContainer, []string{"container", "rootfiles", "rootfile", "rootfile"}[i])})
		n.header.Location = Location{Target: TargetSource, DocumentID: s.documentID, Path: "/container", Position: &SourcePosition{Line: 1, Column: i + 1}}
		role := []contextRole{containerRoot, containerRootfiles, containerRootfile, containerRootfile}[i]
		b.Elements[role] = map[DeclarationID]struct{}{n.header.Declaration.Value: {}}
	}
	for _, n := range []*factNode{first, second} {
		for _, v := range []struct {
			role       contextRole
			name, text string
		}{{containerFullPath, "full-path", "score.musicxml"}, {containerMediaType, "media-type", "application/vnd.recordare.musicxml+xml"}} {
			a := localAttr(v.name)
			a.Declaration.Assembly = AssemblyContainer
			b.Attributes[v.role] = map[ComponentID]AttributeSelector{n.header.Type.Value: a}
			n.attributes = append(n.attributes, AttributeFact{Owner: n.header.ID, Name: a.Name, Declaration: Evidence[DeclarationID]{State: EvidenceKnown, Value: a.Declaration}, Presence: Evidence[bool]{State: EvidenceKnown, Value: true}, Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceKnown, Value: " " + v.text + " "}, Value: localKnownText(v.text)}, Location: n.header.Location})
		}
	}
	root.children, files.children = []*factNode{files}, []*factNode{first, second}
	s.root, s.nodes, s.sequence = root, map[NodeID]*factNode{1: root, 2: files, 3: first, 4: second}, []NodeID{1, 2, 3, 4}
	return s, &contextPlan{Bindings: b}
}

func TestContainerMetadataUnavailableEvidence(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		for _, field := range []string{"value", "lexeme", "enumeration", "order", "binding"} {
			t.Run(string(state)+"/"+field, func(t *testing.T) {
				s, p := containerTestFixture(t)
				defer s.cleanup()
				cause := &DecisionRef{Rule: RuleKey{"source-cause", 1}, Subject: NodeRef{s.documentID, 3}}
				switch field {
				case "value":
					s.nodes[3].attributes[0].Scalar.Value = Evidence[Atomic]{State: state, Reason: "source-gap", Cause: cause}
				case "lexeme":
					s.nodes[3].attributes[0].Scalar.Lexeme = Evidence[string]{State: state, Reason: "source-gap", Cause: cause}
				case "enumeration":
					s.nodes[2].end.ChildrenComplete = testEvidence(Evidence[bool]{State: state, Reason: "source-gap", Cause: cause})
				case "order":
					s.nodes[2].end.Order = testEvidence(Evidence[OrderKind]{State: state, Reason: "source-gap", Cause: cause})
				case "binding":
					s.nodes[3].header.Declaration = testEvidence(Evidence[DeclarationID]{State: state, Reason: "source-gap", Cause: cause})
				}
				d, pending, err := evaluateContextContainerMetadata(context.Background(), s, nil, RuleKey{"container-test", 1}, p, true)
				if err != nil || pending || d.Outcome != OutcomePass {
					t.Fatalf("projection: %+v %v", d, err)
				}
				expectedCount := 4
				if field == "binding" {
					expectedCount = 3
				}
				if len(s.contextResults) != expectedCount {
					t.Fatalf("result count %d want %d", len(s.contextResults), expectedCount)
				}
				var gotState EvidenceState
				var gotCause *DecisionRef
				switch field {
				case "value":
					e := s.contextResults[2].Container.FullPath
					gotState, gotCause = e.State, e.Cause
				case "lexeme":
					e := s.contextResults[2].Container.FullPathRaw
					gotState, gotCause = e.State, e.Cause
				default:
					e := s.contextResults[1].Container.Content
					gotState, gotCause = e.State, e.Cause
					for _, row := range s.contextResults[2:] {
						if row.Container.Ordinal.State != state || row.Container.Ordinal.Cause == nil || row.Container.Ordinal.Cause.Rule.ID != "source-cause" {
							t.Errorf("ordinal fabricated or lost cause: %+v", row.Container)
						}
					}
				}
				if gotState != state || gotCause == nil || gotCause.Rule.ID != "source-cause" {
					t.Fatalf("causal state replaced: %s %+v", gotState, gotCause)
				}
				cause.Rule.ID = "caller"
				if gotCause.Rule.ID != "source-cause" {
					t.Fatal("output cause aliases input")
				}
			})
		}
	}
}

func TestContainerMetadataExactDeclarations(t *testing.T) {
	for _, which := range []NodeID{1, 2, 3} {
		for _, axis := range []string{"assembly", "occurrence", "parent-use"} {
			t.Run(string(rune('0'+which))+"/"+axis, func(t *testing.T) {
				s, p := containerTestFixture(t)
				defer s.cleanup()
				switch axis {
				case "assembly":
					s.nodes[which].header.Declaration.Value.Assembly = AssemblySounds
				case "occurrence":
					s.nodes[which].header.Declaration.Value.Occurrence += "other"
				case "parent-use":
					s.nodes[which].header.Declaration.Value.OwnerOccurrence += "other"
				}
				if _, _, err := evaluateContextContainerMetadata(context.Background(), s, nil, RuleKey{"container-test", 1}, p, true); err != nil {
					t.Fatal(err)
				}
				expected := map[NodeID]int{1: 0, 2: 1, 3: 3}[which]
				if len(s.contextResults) != expected {
					t.Fatalf("homonym matched exact binding: %d want %d", len(s.contextResults), expected)
				}
				for _, row := range s.contextResults {
					if row.Subject.Node == which {
						t.Fatal("wrong declaration selected")
					}
					if row.Subject.Node == s.nodes[which].header.Parent && row.Container.Content.State == EvidenceKnown {
						t.Fatal("skipped binding fabricated complete inventory")
					}
					if which == 3 && row.Container.Role == ContainerRootfile && row.Container.Ordinal.State == EvidenceKnown {
						t.Fatal("skipped sibling fabricated first-rootfile order")
					}
				}
			})
		}
	}
}

func TestContainerMetadataCancellationAndFactBudget(t *testing.T) {
	probe, p := containerTestFixture(t)
	ctx := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	if _, _, err := evaluateContextContainerMetadata(ctx, probe, nil, RuleKey{"container-test", 1}, p, true); err != nil {
		t.Fatal(err)
	}
	polls := ctx.calls
	probe.cleanup()
	for at := 1; at <= polls; at++ {
		s, p := containerTestFixture(t)
		_, _, err := evaluateContextContainerMetadata(&rationalCancelContext{Context: context.Background(), at: at}, s, nil, RuleKey{"container-test", 1}, p, true)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("lost cancellation %d/%d: %v", at, polls, err)
		}
		for _, row := range s.contextResults {
			if row.Container == nil || row.Container.Content.State != EvidenceKnown || row.Container.Container.Node != 1 {
				t.Fatal("partial payload escaped")
			}
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("canceled provider retained bytes")
		}
	}
	for limit := int64(1); limit <= 4; limit++ {
		s, p := containerTestFixture(t)
		s.account, _ = NewBudgetAccount(Budgets{MaxContextFacts: limit})
		_, _, err := evaluateContextContainerMetadata(context.Background(), s, nil, RuleKey{"container-test", 1}, p, true)
		if limit < 4 && !errors.Is(err, ErrResourceLimit) || limit == 4 && err != nil {
			t.Fatalf("fact budget %d: %v", limit, err)
		}
		if int64(len(s.contextResults)) != limit {
			t.Fatalf("publication count %d limit %d", len(s.contextResults), limit)
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("fact failure retained bytes")
		}
	}
	t.Logf("all %d provider cancellation polls checked", polls)
}

func TestContainerMetadataRejectsUnprovedSourceOrder(t *testing.T) {
	for _, order := range []OrderKind{OrderModelContent, OrderModelFields} {
		s, p := containerTestFixture(t)
		s.nodes[2].end.Order = testEvidence(Evidence[OrderKind]{State: EvidenceKnown, Value: order})
		_, _, err := evaluateContextContainerMetadata(context.Background(), s, nil, RuleKey{"container-test", 1}, p, true)
		if err != nil {
			t.Fatal(err)
		}
		if s.contextResults[1].Container.Content.State == EvidenceKnown || s.contextResults[2].Container.Ordinal.State == EvidenceKnown {
			t.Fatalf("%s was mislabeled as original source order", order)
		}
		s.cleanup()
	}
}
