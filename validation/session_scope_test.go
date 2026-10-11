package validation

import "testing"

func TestQARound3ContextScopeSameNodeOnlyRelevantPublishers(t *testing.T) {
	value := Atomic{Kind: AtomicString, Text: "known"}
	local, _ := NewContextValuePlan("effective", RelationSameNode, value)
	parent, _ := NewContextValuePlan("effective", RelationParent, Atomic{Kind: AtomicString, Text: "conflict"})
	consumer, _ := NewContextComparisonPlan("effective", RelationSameNode, ComparisonEqual, value)
	for _, possibleParentWriter := range []bool{false, true} {
		t.Run(map[bool]string{false: "local-only", true: "unseen-parent-publisher"}[possibleParentWriter], func(t *testing.T) {
			specs := []qaSpec{{id: "a-local", occurrence: "test.root", p: local, role: RoleContextProvider}, {id: "z-consumer", occurrence: "test.root", p: consumer}}
			if possibleParentWriter {
				specs = append(specs, qaSpec{id: "b-parent", occurrence: "test.missing-child", p: parent, role: RoleContextProvider})
			}
			s := qaSession(t, specs...)
			qaOpenRoot(t, s)
			end := factEnd(1)
			end.ChildrenComplete = Evidence[bool]{State: EvidenceUnknown, Reason: "children-unavailable"}
			qaClose(t, s, end)
			r := qaFinish(t, s, true)
			f := qaFinding(t, r, "z-consumer", 1)
			want := OutcomePass
			if possibleParentWriter {
				want = OutcomeUnknown
			}
			if f.Outcome != want {
				t.Fatalf("outcome=%s want=%s parentWriter=%t", f.Outcome, want, possibleParentWriter)
			}
			if r.Satisfied() {
				t.Fatal("unknown input subject enumeration cannot satisfy full assessment")
			}
		})
	}
}

func TestQARound3ContextScopeParentReadSeesNodeLocalProvider(t *testing.T) {
	value := Atomic{Kind: AtomicString, Text: "known"}
	provider, _ := NewContextValuePlan("effective", RelationSameNode, value)
	consumer, _ := NewContextComparisonPlan("effective", RelationParent, ComparisonEqual, value)
	s := qaSession(t, qaSpec{id: "a-provider", occurrence: "test.root", p: provider, role: RoleContextProvider}, qaSpec{id: "z-consumer", occurrence: "test.child", p: consumer})
	qaOpenRoot(t, s)
	ciStreamNode(t, s, 2, 1, "test.child", "")
	end := factEnd(1)
	end.ChildrenComplete = Evidence[bool]{State: EvidenceUnknown, Reason: "siblings-unavailable"}
	qaClose(t, s, end)
	r := qaFinish(t, s, true)
	f := qaFinding(t, r, "z-consumer", 2)
	if f.Outcome != OutcomePass {
		t.Fatalf("closed parent's local provider suppressed by unrelated unknown siblings: %+v", f)
	}
}

func TestQARound3ContextScopesDoNotProjectDocumentAndNodeFacts(t *testing.T) {
	value := Atomic{Kind: AtomicString, Text: "known"}
	for _, relations := range [][2]RelationKind{{RelationDocument, RelationSameNode}, {RelationSameNode, RelationDocument}} {
		provider, _ := NewContextValuePlan("effective", relations[0], value)
		consumer, _ := NewContextComparisonPlan("effective", relations[1], ComparisonEqual, value)
		s := qaSession(t, qaSpec{id: "a-provider", occurrence: "test.root", p: provider, role: RoleContextProvider}, qaSpec{id: "z-consumer", occurrence: "test.root", p: consumer})
		qaOpenRoot(t, s)
		qaClose(t, s, factEnd(1))
		r := qaFinish(t, s, true)
		f := qaFinding(t, r, "z-consumer", 1)
		if f.Outcome != OutcomeUnknown {
			t.Fatalf("distinct document/node scope collapsed %v: %+v", relations, f)
		}
	}
}
