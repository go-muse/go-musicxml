package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Every published property belongs to the family its namespace names, which
// the ContextFamilies documentation lists; an unknown property has none.
func TestContextFamilyOfProperties(t *testing.T) {
	for property, want := range map[ContextProperty]ContextFamilies{
		ContextPropertyTiming:              ContextFamilyTiming,
		ContextPrintObject:                 ContextFamilyLayout,
		ContextNoteheadFilled:              ContextFamilyLayout,
		"layout.opus-version":              ContextFamilyLayout,
		ContextPropertySpan:                ContextFamilySpans,
		ContextPropertySpanLyricSyllable:   ContextFamilySpans,
		ContextPropertySpanLineType:        ContextFamilySpans,
		ContextPropertyRepeat:              ContextFamilyRepeats,
		ContextPropertyNavigation:          ContextFamilyNavigation,
		ContextPropertyNavigationTraversal: ContextFamilyNavigation,
		"performance-figure-prefix":        ContextFamilyPerformance,
		ContextPropertySoundMapping:        ContextFamilySounds,
		ContextResourceImage:               ContextFamilyResources,
		ContextPropertyContainerMetadata:   ContextFamilyContainer,
		ContextPropertySourceOrigin:        ContextFamilySourceOrigins,
		ContextDictionaryCanonical:         ContextFamilyDictionaries,
		"unknown":                          0,
	} {
		if got := (ContextResult{Property: property}).Family(); got != want {
			t.Errorf("%s: family %b, want %b", property, got, want)
		}
	}
	if ContextFamilyAll != contextFamilyEnd-1 || ContextFamilyAll&ContextFamilyDictionaries == 0 {
		t.Fatalf("ContextFamilyAll = %b", ContextFamilyAll)
	}
}

// Intern table entries outlive the publication that made them: a publication
// canceled or refused after interning leaves no record, but the table keeps
// its entries, and their slots and strings stay charged on the account until
// cleanup.
func TestInternEntriesStayChargedAfterRollback(t *testing.T) {
	text := strings.Repeat("x", 7000)
	row := ContextResult{Provider: RuleKey{"intern", 1}, Subject: NodeRef{"source", 1}, Property: ContextPropertySpanNormalDots, Value: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: text}}}
	probe := rationalSession(t, Budgets{})
	counter := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	if err := probe.publishContextResult(counter, row); err != nil {
		t.Fatal(err)
	}
	probe.cleanup()
	interned := 0
	for at := 1; at <= counter.calls; at++ {
		s := rationalSession(t, Budgets{})
		err := s.publishContextResult(&rationalCancelContext{Context: context.Background(), at: at}, row)
		if !errors.Is(err, context.Canceled) || len(s.contextResults) != 0 {
			t.Fatalf("poll %d: err=%v records=%d", at, err, len(s.contextResults))
		}
		entries := int64(0)
		for v := range s.strings {
			entries += internSlotBytes + ownedStringBytes(v)
		}
		if len(s.strings) != 0 {
			entries += internTableBytes
			if _, ok := s.strings[text]; ok {
				interned++
			}
		}
		if s.retained != entries || internCharge(s) != entries || rationalUsage(t, s, BudgetBufferedBytes) != entries {
			t.Fatalf("poll %d: table entries %d bytes, retained %d, intern charge %d", at, entries, s.retained, internCharge(s))
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatalf("poll %d: cleanup left a charge", at)
		}
	}
	if interned == 0 {
		t.Fatal("no cancellation point after the text was interned")
	}
}
