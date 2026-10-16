package validation

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// Record the first and last checkpoint at each index-build call site, including
// lookup callers and both SortFunc comparators. This follows actual stages rather
// than pinning Go's sort-comparison counts or line numbers into the test.
type ipaRepresentationConstructorTrace struct {
	context.Context
	checks      int64
	sites       map[string][2]int64
	comparators map[string]bool
}

func (c *ipaRepresentationConstructorTrace) Err() error {
	c.checks++
	pc, file, line, ok := runtime.Caller(2)
	if !ok || (!strings.HasSuffix(file, "/dictionary_ipa_representation.go") && !strings.HasSuffix(file, "/dictionary_ipa_representation_policy.go")) {
		return nil
	}
	name := runtime.FuncForPC(pc).Name()
	_, callerFile, callerLine, _ := runtime.Caller(3)
	key := fmt.Sprintf("%s:%d/%s:%d", file, line, callerFile, callerLine)
	rangeAt := c.sites[key]
	if rangeAt[0] == 0 {
		rangeAt[0] = c.checks
	}
	rangeAt[1] = c.checks
	c.sites[key] = rangeAt
	// func1 is the support-add closure; func2 and func3 are the two
	// comparators. The assertion below makes either missing stage visible.
	if strings.Contains(name, ".buildIPARepresentation.func") && !strings.HasSuffix(name, ".func1") {
		c.comparators[name] = true
	}
	return nil
}

func TestIPARepresentationConstructorCancellationAndExactWork(t *testing.T) {
	selection, input, _ := ipaRepresentationFixture(t)
	trace := &ipaRepresentationConstructorTrace{Context: context.Background(), sites: map[string][2]int64{}, comparators: map[string]bool{}}
	probe := dictionaryCoreAccount(t)
	d, err := NewDictionarySet(trace, probe, []DictionarySelection{selection}, []DictionaryImport{input})
	if err != nil {
		t.Fatal(err)
	}
	work, _ := probe.Usage(BudgetRuleSteps)
	if err := d.Close(); err != nil || dictionaryCoreUsage(t, probe) != 17 {
		t.Fatalf("probe cleanup: %v", err)
	}
	if len(trace.comparators) != 2 {
		t.Fatalf("did not trace both index sort comparators: %v", trace.comparators)
	}
	cutoffs := []int64{1, trace.checks - 1, trace.checks}
	for _, at := range trace.sites {
		cutoffs = append(cutoffs, at[0], at[1])
	}
	for at := int64(1); at < trace.checks; at += max(1, trace.checks/100) {
		cutoffs = append(cutoffs, at)
	}
	slices.Sort(cutoffs)
	for _, cutoff := range slices.Compact(cutoffs) {
		a := dictionaryCoreAccount(t)
		ctx := &dictionaryCancelContext{Context: context.Background(), cancelAt: cutoff}
		set, err := NewDictionarySet(ctx, a, []DictionarySelection{selection}, []DictionaryImport{input})
		if set != nil || !errors.Is(err, context.Canceled) || ctx.checks.Load() != cutoff {
			t.Fatalf("cancel@%d/%d resumed: set=%v err=%v checks=%d", cutoff, trace.checks, set, err, ctx.checks.Load())
		}
		if dictionaryCoreUsage(t, a) != 17 {
			t.Fatalf("cancel@%d retained partial storage", cutoff)
		}
	}
	for _, limit := range []int64{work - 1, work} {
		a, err := NewBudgetAccount(Budgets{MaxRuleSteps: limit})
		if err != nil {
			t.Fatal(err)
		}
		if err := a.Charge(BudgetBufferedBytes, 17); err != nil {
			t.Fatal(err)
		}
		set, err := NewDictionarySet(context.Background(), a, []DictionarySelection{selection}, []DictionaryImport{input})
		if limit == work {
			if err != nil || set == nil {
				t.Fatalf("exact constructor work=%d: %v", work, err)
			}
			set.Close()
		} else if set != nil || !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("one-under constructor work=%d: set=%v err=%v", limit, set, err)
		}
		if dictionaryCoreUsage(t, a) != 17 {
			t.Fatal("work boundary retained partial storage")
		}
	}
}

func TestIPARepresentationIndexStructuralGuards(t *testing.T) {
	prepend := func(p *IPAPositiveDictionaryData, encoding, role string) {
		p.Symbols = append([]IPASymbolEncoding{{Encoding: encoding, Category: role, RowID: "synthetic-guard"}}, p.Symbols...)
	}
	for _, tc := range []struct {
		name, reason string
		mutate       func(*IPAPositiveDictionaryData)
	}{
		{"support-capacity", "excessive representation support", func(p *IPAPositiveDictionaryData) { prepend(p, "?", "sign") }},
		{"key-not-scalar", "representation decomposition key is not scalar", func(p *IPAPositiveDictionaryData) { p.Equivalences[0].Encoding = "ab" }},
		{"decomposition-capacity", "excessive representation decomposition", func(p *IPAPositiveDictionaryData) { p.Equivalences[0].EquivalentToEncoding = "aaaa" }},
		{"empty-decomposition", "identity representation decomposition", func(p *IPAPositiveDictionaryData) { p.Equivalences[0].EquivalentToEncoding = "" }},
		{"identity-decomposition", "identity representation decomposition", func(p *IPAPositiveDictionaryData) {
			p.Equivalences[0].EquivalentToEncoding = p.Equivalences[0].Encoding
		}},
		{"incomplete-support", "incomplete representation support", func(p *IPAPositiveDictionaryData) { p.Symbols = nil }},
		{"recursive-decomposition", "nonrecursive representation decomposition required", func(p *IPAPositiveDictionaryData) {
			p.Equivalences = append(p.Equivalences, IPACanonicalEquivalent{Encoding: "a", EquivalentToEncoding: "à", RowID: "synthetic-cycle"})
		}},
		{"atom-capacity", "excessive representation atom", func(p *IPAPositiveDictionaryData) { prepend(p, "aaaa", "letter") }},
		{"independent-mark", "invalid independent representation mark", func(p *IPAPositiveDictionaryData) { prepend(p, "a", "mark") }},
		{"role", "invalid representation role", func(p *IPAPositiveDictionaryData) { prepend(p, "a", "unreviewed-role") }},
		{"nonstarter", "invalid representation starter", func(p *IPAPositiveDictionaryData) { prepend(p, "\u0301", "letter") }},
		{"atom-count", "invalid representation starter", func(p *IPAPositiveDictionaryData) { prepend(p, "a", "letter") }},
		{"multiple-starters", "multiple representation starters", func(p *IPAPositiveDictionaryData) { prepend(p, "ab", "letter") }},
		{"incomplete-atoms", "incomplete representation atoms", func(p *IPAPositiveDictionaryData) {
			p.Symbols = slices.DeleteFunc(p.Symbols, func(r IPASymbolEncoding) bool { return r.Encoding == "a" })
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, input, _ := ipaRepresentationFixture(t)
			tc.mutate(input.IPA)
			a := dictionaryCoreAccount(t)
			d := &DictionarySet{initialized: true, account: a}
			e := &dictionaryEntry{ipa: input.IPA}
			err := d.buildIPARepresentation(context.Background(), e)
			var config *ConfigurationError
			if !errors.As(err, &config) || config.Reason != tc.reason || e.representation != nil {
				t.Fatalf("guard: index=%v err=%v, want %q", e.representation, err, tc.reason)
			}
			if err := d.Close(); err != nil || dictionaryCoreUsage(t, a) != 17 {
				t.Fatalf("failed direct build cleanup: %v", err)
			}
		})
	}
}

func TestIPARepresentationIndexSyntheticMultiMarkAtom(t *testing.T) {
	_, input, _ := ipaRepresentationFixture(t)
	// Replace an atom whose raw scalar remains in the decomposition support.
	// The sealed fixture has at most two leaves; this synthetic case exercises
	// the independently bounded three-leaf sort without changing public data.
	for i := range input.IPA.Symbols {
		if input.IPA.Symbols[i].Encoding == "a" {
			input.IPA.Symbols[i].Encoding = "a\u0301\u0334"
		}
	}
	a := dictionaryCoreAccount(t)
	d := &DictionarySet{initialized: true, account: a}
	defer func() {
		if err := d.Close(); err != nil || dictionaryCoreUsage(t, a) != 17 {
			t.Fatalf("synthetic cleanup: %v", err)
		}
	}()
	e := &dictionaryEntry{ipa: input.IPA}
	trace := &ipaRepresentationConstructorTrace{Context: context.Background(), sites: map[string][2]int64{}, comparators: map[string]bool{}}
	if err := d.buildIPARepresentation(trace, e); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, atom := range e.representation.atoms {
		if atom.leaves[0] == 'a' {
			if atom.count != 3 || atom.leaves != [3]rune{'a', '\u0334', '\u0301'} {
				t.Fatalf("synthetic atom not reordered: %+v", atom)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("synthetic atom missing")
	}
	// The synthetic atom reaches the charged swap and its lookup callers,
	// which the sealed one-mark atoms cannot reach. Cancel at every observed
	// index call site, including those synthetic-only paths.
	for _, checkpoints := range trace.sites {
		for _, cutoff := range checkpoints {
			a := dictionaryCoreAccount(t)
			d := &DictionarySet{initialized: true, account: a}
			e := &dictionaryEntry{ipa: input.IPA}
			ctx := &dictionaryCancelContext{Context: context.Background(), cancelAt: cutoff}
			if err := d.buildIPARepresentation(ctx, e); !errors.Is(err, context.Canceled) || e.representation != nil || ctx.checks.Load() != cutoff {
				t.Fatalf("synthetic cancel@%d resumed: index=%v err=%v checks=%d", cutoff, e.representation, err, ctx.checks.Load())
			}
			if err := d.Close(); err != nil || dictionaryCoreUsage(t, a) != 17 {
				t.Fatalf("synthetic cancel@%d cleanup: %v", cutoff, err)
			}
		}
	}
}

func TestIPARepresentationIndexStorageAdmission(t *testing.T) {
	_, input, _ := ipaRepresentationFixture(t)
	a, err := NewBudgetAccount(Budgets{MaxBufferedBytes: ipaRepresentationIndexBytes + 16})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Charge(BudgetBufferedBytes, 17); err != nil {
		t.Fatal(err)
	}
	d := &DictionarySet{initialized: true, account: a}
	e := &dictionaryEntry{ipa: input.IPA}
	if err := d.buildIPARepresentation(context.Background(), e); !errors.Is(err, ErrResourceLimit) || e.representation != nil {
		t.Fatalf("index storage refusal: index=%v err=%v", e.representation, err)
	}
	if err := d.Close(); err != nil || dictionaryCoreUsage(t, a) != 17 {
		t.Fatalf("index storage cleanup: %v", err)
	}
}
