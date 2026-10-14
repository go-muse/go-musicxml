package validation

import (
	"context"
	"errors"
	"testing"
)

func TestDictionaryCanonicalPrimaryReusesExactSearch(t *testing.T) {
	selection, input, _ := dictionaryFixture(t, DictionaryManifestSMuFL14Canonical)
	for _, name := range []string{"restQuarter", "notCanonical"} {
		t.Run(name, func(t *testing.T) {
			reference, _ := dictionaryConsumerReader(t, selection, input)
			before, _ := reference.s.account.Usage(BudgetRuleSteps)
			entry, _, err := reference.s.dictionaryCapability(reference.ctx, selection.Manifest, DictionaryRelationCanonicalName)
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = dictionarySearch(reference.ctx, reference.s.account, len(entry.smufl.Glyphs), name, func(i int) string { return entry.smufl.Glyphs[i].Name })
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = reference.s.dictionaryCapability(reference.ctx, selection.Manifest, DictionaryRelationPrimaryCodepoint)
			if err != nil {
				t.Fatal(err)
			}
			after, _ := reference.s.account.Usage(BudgetRuleSteps)
			want := after - before
			r, _ := dictionaryConsumerReader(t, selection, input)
			before, _ = r.s.account.Usage(BudgetRuleSteps)
			out, _, err := dictionaryCanonicalLookup(r, stateKnown(Atomic{Kind: AtomicString, Text: name}), nil)
			if err != nil {
				t.Fatal(err)
			}
			after, _ = r.s.account.Usage(BudgetRuleSteps)
			if after-before > want+2*int64(len(name))+1 {
				t.Errorf("duplicate primary search: work=%d independent capability/search/capability=%d", after-before, want)
			}
			if (out.Codepoint.State == EvidenceKnown) != (name == "restQuarter") {
				t.Fatalf("primary result lost: %+v", out.Codepoint)
			}
		})
	}
}

func TestDictionaryIPAEmptySpanDoesNotRescanImmutableTable(t *testing.T) {
	selection, input, _ := dictionaryFixture(t, DictionaryManifestIPAPositive2015Unicode13)
	for _, relation := range []DictionaryRelation{DictionaryRelationIPASymbol, DictionaryRelationIPACanonicalEquivalent} {
		t.Run(string(relation), func(t *testing.T) {
			r, _ := dictionaryConsumerReader(t, selection, input)
			entry, _, err := r.s.dictionaryCapability(r.ctx, selection.Manifest, relation)
			if err != nil {
				t.Fatal(err)
			}
			before, _ := r.s.account.Usage(BudgetRuleSteps)
			out := dictionaryPayload(stateKnown(Atomic{Kind: AtomicString, Text: ""}), nil, selection.Manifest, relation)
			if err := dictionaryIPARelation(r, out, entry); err != nil {
				t.Fatal(err)
			}
			after, _ := r.s.account.Usage(BudgetRuleSteps)
			if after-before > 1 {
				t.Errorf("empty sequence scanned immutable rows: work=%d", after-before)
			}
			if len(out.Spans) != 1 || out.Spans[0].Result.State != EvidenceUnknown {
				t.Fatal(out.Spans)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			r.ctx = ctx
			if err := dictionaryIPARelation(r, out, entry); !errors.Is(err, context.Canceled) {
				t.Errorf("empty-sequence cancellation not observed: %v", err)
			}
		})
	}
}

func TestDictionaryPrimaryReuseRequiresIdenticalEntryAndKey(t *testing.T) {
	selection, input, _ := dictionaryFixture(t, DictionaryManifestSMuFL14Canonical)
	for _, change := range []string{"key", "entry"} {
		t.Run(change, func(t *testing.T) {
			r, _ := dictionaryConsumerReader(t, selection, input)
			entry, _, err := r.s.dictionaryCapability(r.ctx, selection.Manifest, DictionaryRelationCanonicalName)
			if err != nil {
				t.Fatal(err)
			}
			prior := dictionaryGlyphSearch{entry: entry, key: "notCanonical", index: 0, found: false}
			if change == "entry" {
				copy := *entry
				prior.entry = &copy
				prior.key = "restQuarter"
			}
			out := dictionaryPayload(stateKnown(Atomic{Kind: AtomicString, Text: "restQuarter"}), nil, selection.Manifest, DictionaryRelationCanonicalName)
			if err := dictionaryPrimaryCodepoint(r, out, prior); err != nil {
				t.Fatal(err)
			}
			if out.Codepoint.State != EvidenceKnown || out.Codepoint.Value != 0xE4E5 {
				t.Fatalf("foreign search reused: %+v", out.Codepoint)
			}
		})
	}
}

func TestDictionaryCanonicalMembershipModeDoesNotReadPrimaryRelation(t *testing.T) {
	selection, input, _ := dictionaryFixture(t, DictionaryManifestSMuFL14Canonical)
	r, _ := dictionaryConsumerReader(t, selection, input)
	before, _ := r.s.account.Usage(BudgetRuleSteps)
	entry, _, err := r.s.dictionaryCapability(r.ctx, selection.Manifest, DictionaryRelationCanonicalName)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = dictionarySearch(r.ctx, r.s.account, len(entry.smufl.Glyphs), "restQuarter", func(i int) string { return entry.smufl.Glyphs[i].Name })
	if err != nil {
		t.Fatal(err)
	}
	after, _ := r.s.account.Usage(BudgetRuleSteps)
	want := after - before
	before, _ = r.s.account.Usage(BudgetRuleSteps)
	out, d, err := dictionaryCanonicalLookupMode(r, stateKnown(Atomic{Kind: AtomicString, Text: "restQuarter"}), nil, false)
	if err != nil {
		t.Fatal(err)
	}
	after, _ = r.s.account.Usage(BudgetRuleSteps)
	if after-before != want || d.Outcome != OutcomePass || out.Codepoint.State != EvidenceAbsent {
		t.Fatalf("membership-only lookup read unused primary relation: work=%d want=%d out=%+v decision=%+v", after-before, want, out, d)
	}
}

func TestDictionaryRangeAdviceIgnoresPrimarySelection(t *testing.T) {
	selection, input, _ := dictionaryFixture(t, DictionaryManifestSMuFL14Canonical)
	run := func(primary bool) (int64, *DictionaryContextResult) {
		selection.Relations = []DictionaryRelation{DictionaryRelationCanonicalName}
		if primary {
			selection.Relations = append(selection.Relations, DictionaryRelationPrimaryCodepoint)
		}
		r, _ := dictionaryConsumerReader(t, selection, input)
		r.b = &contextBindings{assembly: AssemblyMusicXML}
		r.s.documentID = "doc"
		n := localNode(1, 0, "tremolo")
		before, _ := r.s.account.Usage(BudgetRuleSteps)
		raw := Evidence[Atomic]{State: EvidenceAbsent}
		effective := stateKnown(Atomic{Kind: AtomicString, Text: "buzzRoll"})
		if err := dictionaryPublishRange(r, n, RuleKey{ID: "dictionary", Revision: 1}, "tremolo-range", raw, effective, stateKnown(true), []string{"tremolos"}); err != nil {
			t.Fatal(err)
		}
		after, _ := r.s.account.Usage(BudgetRuleSteps)
		if len(r.s.contextResults) != 1 {
			t.Fatal(r.s.contextResults)
		}
		return after - before, r.s.contextResults[0].Dictionary
	}
	absent, without := run(false)
	selected, with := run(true)
	if selected != absent || with.Codepoint.State != EvidenceAbsent || without.Codepoint.State != EvidenceAbsent {
		t.Fatalf("range-only advice used primary relation: selected=%d unselected=%d", selected, absent)
	}
}
