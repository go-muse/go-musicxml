package validation

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// Compare the filter against the unfiltered evaluator, including causal states.
// These expected routes are deliberately spelled out independently of its role
// switch so an accidentally omitted relevant role cannot confirm itself.
func TestDictionaryRelevanceMatchesUnfilteredEvaluation(t *testing.T) {
	for _, tc := range []struct {
		operation DictionaryOperation
		roles     []contextRole
	}{
		{DictionaryStandardSounds, []contextRole{"dictionary.sound", "dictionary.sounds.score-instrument", "dictionary.sounds.instrument-change"}},
		{DictionaryIPA, []contextRole{"dictionary.ipa.play"}},
		{DictionaryRendering, []contextRole{"dictionary.stopped", "dictionary.open", "dictionary.half-muted", "dictionary.tremolo", "dictionary.notehead", "dictionary.smufl.direction-type", "dictionary.smufl.appearance", "dictionary.arrow", "dictionary.mordent", "dictionary.inverted-mordent"}},
	} {
		for _, role := range append(tc.roles, "unrelated") {
			for _, evidence := range []EvidenceState{EvidenceKnown, EvidenceUnknown, EvidenceInvalid} {
				t.Run(string(tc.operation)+"/"+string(role)+"/"+string(evidence), func(t *testing.T) {
					n := localNode(1, 0, "subject")
					n.header.Type = testEvidence(stateKnown(contextNamedType("xs:string")))
					n.end.Scalar.Value = testEvidence(stateKnown(Atomic{Kind: AtomicString, Text: "a"}))
					decl := n.header.Declaration.Value
					cause := &DecisionRef{Rule: RuleKey{ID: "schema", Revision: 1}, Subject: NodeRef{"doc", 1}}
					if evidence != EvidenceKnown {
						n.header.Declaration = testEvidence(Evidence[DeclarationID]{State: evidence, Reason: "declaration-unavailable", Cause: cause})
					}
					b := &contextBindings{assembly: AssemblyMusicXML, ElementRoles: map[DeclarationID][]contextRole{decl: {role}}, Elements: map[contextRole]map[DeclarationID]struct{}{role: {decl: {}}}, OwnerTables: map[ComponentID]*contextAttributeTable{}}
					if role == "dictionary.sound" {
						b.assembly = AssemblySounds
					}
					run := func(filtered bool) (Decision, []ContextResult) {
						s := rationalSession(t, Budgets{})
						defer s.cleanup()
						s.documentID = "doc"
						p := &dictionaryPlan{Bindings: b, Operation: tc.operation}
						key := RuleKey{ID: "dictionary", Revision: 1}
						var d Decision
						var err error
						if filtered {
							d, _, err = s.evaluateDictionary(context.Background(), n, RuleDefinition{Key: key}, p, true)
						} else {
							r := &contextReader{ctx: context.Background(), s: s, b: b}
							switch tc.operation {
							case DictionaryStandardSounds:
								d, err = dictionarySoundsNode(r, n, key)
							case DictionaryIPA:
								d, err = dictionaryIPANode(r, n, key)
							case DictionaryRendering:
								d, err = dictionaryRenderingNode(r, n, key)
							}
						}
						if err != nil {
							t.Fatal(err)
						}
						return d, append([]ContextResult(nil), s.contextResults...)
					}
					before, rowsBefore := run(false)
					after, rowsAfter := run(true)
					if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(rowsBefore, rowsAfter) {
						t.Fatalf("filter altered decision/evidence: before=%+v after=%+v rows=%+v/%+v", before, after, rowsBefore, rowsAfter)
					}
				})
			}
		}
	}
}

func TestDictionaryAssessedTypeCandidateKeepsExactIdentity(t *testing.T) {
	good := contextNamedType("smufl-glyph-name")
	for _, tc := range []struct {
		name  string
		alter func(*ComponentID)
	}{
		{"exact", func(*ComponentID) {}},
		{"assembly", func(v *ComponentID) { v.Assembly = AssemblySounds }},
		{"namespace", func(v *ComponentID) { v.Name.Space = "urn:other" }},
		{"kind", func(v *ComponentID) { v.Kind = ComponentElement }},
		{"occurrence", func(v *ComponentID) { v.Occurrence = "other-owner" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			typ := good
			tc.alter(&typ)
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			s.documentID = "doc"
			n := localNode(1, 0, "not-a-dictionary-element")
			n.header.Type = testEvidence(stateKnown(typ))
			n.end.Scalar.Value = testEvidence(stateKnown(Atomic{Kind: AtomicString, Text: "restQuarter"}))
			b := &contextBindings{assembly: AssemblyMusicXML, OwnerTables: map[ComponentID]*contextAttributeTable{}}
			p := &dictionaryPlan{Bindings: b, Operation: DictionaryCanonical, Types: []ComponentID{good}}
			_, _, err := s.evaluateDictionary(context.Background(), n, RuleDefinition{Key: RuleKey{ID: "dictionary", Revision: 1}}, p, true)
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if tc.name == "exact" {
				want = 1
			}
			if len(s.contextResults) != want {
				t.Fatalf("same-name identity admitted/excluded wrong subject: %+v", s.contextResults)
			}
		})
	}
}

func TestDictionaryRelevanceObservesCancellationAndQuota(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		s := rationalSession(t, Budgets{})
		defer s.cleanup()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		want := ErrResourceLimit
		if canceled {
			cancel()
			want = context.Canceled
		} else {
			used, _ := s.account.Usage(BudgetRuleSteps)
			if err := s.account.Charge(BudgetRuleSteps, s.account.Limits().MaxRuleSteps-used); err != nil {
				t.Fatal(err)
			}
		}
		n := localNode(1, 0, "irrelevant")
		p := &dictionaryPlan{Operation: DictionaryRendering, Bindings: &contextBindings{assembly: AssemblyMusicXML, ElementRoles: map[DeclarationID][]contextRole{}}}
		_, _, err := s.evaluateDictionary(ctx, n, RuleDefinition{}, p, true)
		if !errors.Is(err, want) {
			t.Fatalf("filter hid interruption: %v", err)
		}
	}
}

// The candidate switch is a fast superset filter for the authoritative plan
// list. A newly reviewed type must never be silently filtered out before the
// exact ComponentID check in dictionaryCanonicalNode.
func TestDictionaryAssessedTypeCandidateCoversPlanNames(t *testing.T) {
	if len(dictionarySMuFLTypeNames) == 0 {
		t.Fatal("dictionary plan type list must not be empty")
	}
	seen := make(map[string]bool, len(dictionarySMuFLTypeNames))
	for _, name := range dictionarySMuFLTypeNames {
		if name == "" || seen[name] {
			t.Fatalf("empty or duplicate dictionary plan type: %q", name)
		}
		seen[name] = true
		t.Run(name, func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			r := &contextReader{ctx: context.Background(), s: s}
			candidate := dictionarySMuFLTypeCandidate(r, contextNamedType(name))
			if r.err != nil {
				t.Fatal(r.err)
			}
			if !candidate {
				t.Fatalf("reviewed plan type %q is excluded by candidate filter", name)
			}
		})
	}
	for _, name := range []string{"", "string", "smufl", "Smufl-glyph-name", "smufl-glyph-name-extra"} {
		t.Run("unrelated/"+name, func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			r := &contextReader{ctx: context.Background(), s: s}
			candidate := dictionarySMuFLTypeCandidate(r, contextNamedType(name))
			if r.err != nil {
				t.Fatal(r.err)
			}
			if candidate {
				t.Fatalf("unrelated type %q admitted as candidate", name)
			}
		})
	}
}
