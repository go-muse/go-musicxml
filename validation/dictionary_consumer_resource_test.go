package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func dictionaryConsumerReader(t *testing.T, selection DictionarySelection, input DictionaryImport) (*contextReader, *DictionarySet) {
	t.Helper()
	s := rationalSession(t, Budgets{})
	set, err := NewDictionarySet(context.Background(), s.account, []DictionarySelection{selection}, []DictionaryImport{input})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := set.borrow(s.account)
	if err != nil {
		t.Fatal(err)
	}
	s.dictionaries = lease
	t.Cleanup(func() { s.cleanup(); _ = set.Close() })
	return &contextReader{ctx: context.Background(), s: s}, set
}
func TestDictionaryIndependentPrimaryCodepointRelation(t *testing.T) {
	selection, input, _ := dictionaryFixture(t, DictionaryManifestSMuFL14Canonical)
	for _, relations := range [][]DictionaryRelation{{DictionaryRelationCanonicalName}, {DictionaryRelationPrimaryCodepoint}, {DictionaryRelationCanonicalName, DictionaryRelationPrimaryCodepoint}} {
		selection.Relations = relations
		r, _ := dictionaryConsumerReader(t, selection, input)
		out, d, err := dictionaryCanonicalLookup(r, stateKnown(Atomic{Kind: AtomicString, Text: "restQuarter"}), nil)
		if err != nil {
			t.Fatal(err)
		}
		canonical, primary := false, false
		for _, relation := range relations {
			canonical = canonical || relation == DictionaryRelationCanonicalName
			primary = primary || relation == DictionaryRelationPrimaryCodepoint
		}
		if (d.Outcome == OutcomePass) != canonical {
			t.Fatalf("codepoint relation authorized mandatory membership: %+v", d)
		}
		if primary {
			if out.Codepoint.State != EvidenceKnown || out.Codepoint.Value != 0xE4E5 {
				t.Fatalf("missing primary codepoint: %+v", out.Codepoint)
			}
		} else if out.Codepoint.State == EvidenceKnown {
			t.Fatal("unselected primary relation exposed")
		}
	}
}
func TestDictionaryIPASpanWorkIsLinearAndCancellationUnconditional(t *testing.T) {
	selection, input, _ := dictionaryFixture(t, DictionaryManifestIPAPositive2015Unicode13)
	run := func(n int) (int64, int64) {
		r, _ := dictionaryConsumerReader(t, selection, input)
		entry, available, err := r.s.dictionaryCapability(r.ctx, selection.Manifest, DictionaryRelationIPASymbol)
		if err != nil || available.State != EvidenceKnown {
			t.Fatal(err)
		}
		before, _ := r.s.account.Usage(BudgetRuleSteps)
		bytesBefore := r.s.retained
		out := dictionaryPayload(stateKnown(Atomic{Kind: AtomicString, Text: strings.Repeat("a?", n)}), nil, selection.Manifest, DictionaryRelationIPASymbol)
		if err := dictionaryIPARelation(r, out, entry); err != nil {
			t.Fatal(err)
		}
		after, _ := r.s.account.Usage(BudgetRuleSteps)
		if len(out.Spans) != 2*n || out.Result.State != EvidenceUnknown {
			t.Fatalf("lost positive/unknown spans: %d %+v", len(out.Spans), out.Result)
		}
		return after - before, r.s.retained - bytesBefore
	}
	work1, bytes1 := run(128)
	work2, bytes2 := run(256)
	if work2 > 2*work1+2048 || bytes2 > 2*bytes1+2048 {
		t.Fatalf("superlinear spans: work %d/%d bytes %d/%d", work1, work2, bytes1, bytes2)
	}
	r, _ := dictionaryConsumerReader(t, selection, input)
	entry, _, err := r.s.dictionaryCapability(r.ctx, selection.Manifest, DictionaryRelationIPASymbol)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r.ctx = ctx
	out := dictionaryPayload(stateKnown(Atomic{Kind: AtomicString, Text: "a"}), nil, selection.Manifest, DictionaryRelationIPASymbol)
	if err := dictionaryIPARelation(r, out, entry); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation hidden: %v", err)
	}
}

func TestDictionaryKnownNonmembershipSurvivesSecondaryLookupRefusal(t *testing.T) {
	selection, input, _ := dictionaryFixture(t, DictionaryManifestSMuFL14Canonical)
	probe, _ := dictionaryConsumerReader(t, selection, input)
	before, _ := probe.s.account.Usage(BudgetRuleSteps)
	entry, _, err := probe.s.dictionaryCapability(probe.ctx, selection.Manifest, DictionaryRelationCanonicalName)
	if err != nil {
		t.Fatal(err)
	}
	_, found, err := dictionarySearch(probe.ctx, probe.s.account, len(entry.smufl.Glyphs), "notCanonical", func(i int) string { return entry.smufl.Glyphs[i].Name })
	if err != nil || found {
		t.Fatal(err)
	}
	after, _ := probe.s.account.Usage(BudgetRuleSteps)
	canonicalWork := after - before
	r, _ := dictionaryConsumerReader(t, selection, input)
	used, _ := r.s.account.Usage(BudgetRuleSteps)
	hold := r.s.account.Limits().MaxRuleSteps - used - canonicalWork
	if err := r.s.account.Charge(BudgetRuleSteps, hold); err != nil {
		t.Fatal(err)
	}
	out, d, err := dictionaryCanonicalLookup(r, stateKnown(Atomic{Kind: AtomicString, Text: "notCanonical"}), nil)
	if !errors.Is(err, ErrResourceLimit) || d.Outcome != OutcomeFail || out.Result.State != EvidenceKnown || out.Result.Value {
		t.Fatalf("proved canonical failure erased by secondary lookup: %+v %+v %v", out, d, err)
	}
}

func TestDictionaryKnownNonmembershipSurvivesProjectionRefusal(t *testing.T) {
	selection, input, _ := dictionaryFixture(t, DictionaryManifestSMuFL14Canonical)
	for _, value := range []string{"notCanonical", "restQuarter"} {
		r, _ := dictionaryConsumerReader(t, selection, input)
		s := r.s
		s.documentID = "doc"
		s.decisions = map[decisionID]Decision{}
		rule := reportTestRule()
		s.accumulator = newReportAccumulator(reportTestProfile(), s.account)
		n := localNode(1, 0, "glyph")
		n.header.Type = stateKnown(contextNamedType("smufl-glyph-name"))
		n.end.Scalar.Value = stateKnown(Atomic{Kind: AtomicString, Text: value})
		p := &dictionaryPlan{Operation: DictionaryCanonical, Types: []ComponentID{contextNamedType("smufl-glyph-name")}, Bindings: &contextBindings{assembly: AssemblyMusicXML}}
		used, _ := s.account.Usage(BudgetBufferedBytes)
		hold := s.account.Limits().MaxBufferedBytes - used
		if err := s.account.Charge(BudgetBufferedBytes, hold); err != nil {
			t.Fatal(err)
		}
		_, _, err := s.evaluateDictionary(context.Background(), n, rule, p, true)
		if !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("projection refusal hidden: %v", err)
		}
		err = s.recordProvedPlanInterruption(rule, n, Plan{kind: planDictionary, dictionary: p}, err)
		if !errors.Is(err, ErrResourceLimit) {
			t.Fatal("resource cause lost")
		}
		report := s.accumulator.finish(false, SyntaxWellFormed)
		if value == "notCanonical" {
			if report.Counts.Fail != 1 || report.Conformance != ConformanceViolated {
				t.Fatalf("lost mandatory proof: %+v", report)
			}
		} else if report.Counts.Fail != 0 {
			t.Fatalf("interruption became violation: %+v", report)
		}
		_ = s.account.Release(BudgetBufferedBytes, hold)
	}
}

func TestDictionaryInheritedAttributeUsesOriginalDeclaration(t *testing.T) {
	s, _ := contextReviewSameTypeFixture(t)
	defer s.cleanup()
	b, err := compileContextBindingSpecs(s.schema.active, []contextBindingSpec{{Role: "dictionary.smufl.required", Owner: "A", Name: "required", Type: "xs:string", Attribute: true, Count: 1}})
	if err != nil {
		t.Fatal(err)
	}
	selection, input, _ := dictionaryFixture(t, DictionaryManifestSMuFL14Canonical)
	set, err := NewDictionarySet(context.Background(), s.account, []DictionarySelection{selection}, []DictionaryImport{input})
	if err != nil {
		t.Fatal(err)
	}
	defer set.Close()
	s.dictionaries, err = set.borrow(s.account)
	if err != nil {
		t.Fatal(err)
	}
	reader := &contextReader{ctx: context.Background(), s: s, b: b}
	node := s.nodes[2]
	d, err := dictionaryCanonicalNode(reader, node, RuleKey{ID: "dictionary", Revision: 1}, &dictionaryPlan{Bindings: b})
	if err != nil {
		t.Fatal(err)
	}
	if d.Outcome != OutcomeFail || len(s.contextResults) != 1 {
		t.Fatalf("inherited selected use lost: %+v rows=%d", d, len(s.contextResults))
	}
	result := s.contextResults[0].Dictionary
	if result.Attribute == nil || result.Attribute.Declaration != b.Roles["dictionary.smufl.required"][0].Declaration || result.Input.Value.Text != "value" {
		t.Fatalf("owner substituted actual type for original use: %+v", result)
	}
}
