package validation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
)

func ipaRepresentationFixture(t *testing.T) (DictionarySelection, DictionaryImport, []byte) {
	t.Helper()
	raw, err := os.ReadFile("../testdata/dictionaries/ipa-representation/runtime-representation-bundle.json")
	if err != nil {
		t.Fatal(err)
	}
	var b struct {
		Records []struct {
			Kind   string    `json:"record_type"`
			Fields [3]string `json:"fields"`
		}
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatal(err)
	}
	p := &IPAPositiveDictionaryData{}
	for _, row := range b.Records {
		f := row.Fields
		switch row.Kind {
		case "symbol":
			p.Symbols = append(p.Symbols, IPASymbolEncoding{f[0], f[1], f[2]})
		case "canonical-equivalence":
			p.Equivalences = append(p.Equivalences, IPACanonicalEquivalent{f[0], f[1], f[2]})
		default:
			t.Fatalf("unexpected representation row %q", row.Kind)
		}
	}
	slices.Reverse(p.Symbols)
	slices.Reverse(p.Equivalences)
	return DictionarySelection{Manifest: DictionaryManifestIPARepresentation2015Unicode13, Relations: []DictionaryRelation{DictionaryRelationIPARepresentation}}, DictionaryImport{Manifest: DictionaryManifestIPARepresentation2015Unicode13, State: EvidenceKnown, IPA: p}, raw
}
func ipaRepresentationSet(t *testing.T) (*DictionarySet, *Account) {
	t.Helper()
	selection, input, _ := ipaRepresentationFixture(t)
	a := dictionaryCoreAccount(t)
	d, err := NewDictionarySet(context.Background(), a, []DictionarySelection{selection}, []DictionaryImport{input})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Error(err)
		}
		if got := dictionaryCoreUsage(t, a); got != 17 {
			t.Errorf("retention=%d", got)
		}
	})
	return d, a
}
func TestIPARepresentationAffirmativeWholeWitness(t *testing.T) {
	d, a := ipaRepresentationSet(t)
	for _, s := range []string{"", "   ", "gɫɚɝᶑꞎꭨꭩʳˀˁ", "\u0315", "\u02e2\u031a\u0315", "\u02e2\u0315\u031a", "c\u0327", "ç", "ḉ", "c\u0334\u0327\u0301", "c\u0327\u0334\u0301", "\u0340\u0341\u0344", "\u0301\u0334", "d\u0334\u0322\u0301", "t\ufe20s\ufe21", "t\ufe27s\ufe28", "t\u0301\ufe20s\u0300\ufe21", "d\u0322\ufe20s\ufe21", "t\ufe20s\ufe21t\ufe27s\ufe28", strings.Repeat("a\u0301\u0334", 100)} {
		t.Run(s, func(t *testing.T) {
			before := dictionaryCoreUsage(t, a)
			ok, err := ipaRepresentationWitness(context.Background(), a, d.entries[0].representation, s)
			if err != nil || !ok {
				t.Fatalf("%q: witness=%v err=%v", s, ok, err)
			}
			if got := dictionaryCoreUsage(t, a); got != before {
				t.Fatalf("scratch leak=%d", got-before)
			}
		})
	}
	for _, row := range d.entries[0].ipa.Symbols {
		if ok, err := ipaRepresentationWitness(context.Background(), a, d.entries[0].representation, row.Encoding); err != nil || !ok {
			t.Errorf("atom %s %q: %v %v", row.RowID, row.Encoding, ok, err)
		}
	}
}
func TestIPARepresentationUnknownIsNotNegativeComplement(t *testing.T) {
	d, a := ipaRepresentationSet(t)
	for _, s := range []string{"\u0322", "\u0327", "a\u0322", "a\u0327", "c\u0327\u0327", "ca\u0327", "\ufe20", "\ufe21", "\ufe27", "\ufe28", "t\ufe21s\ufe20", "t\ufe20s\ufe28", "t\ufe20s", "t\ufe20 s\ufe21", "t\ufe20as\ufe21", "ˈ\ufe20s\ufe21", "˥\ufe20s\ufe21", "˞\ufe20s\ufe21", "t\ufe20\ufe20s\ufe21\ufe21", "t\ufe20s\ufe21\ufe20a\ufe21", "t\ufe20\ufe27s\ufe21\ufe28", "t\u0361\ufe20s\ufe21", "t\ufe20s\ufe21\u035ca", "a\u0361t\ufe20s\ufe21", "\u0343", "ᵌ", "꭪", "꭫", "\u00a0", "\u034f", "[a]", "/a/", "\t", "\n", "\r", "\ue9b3", "\U0001df00", "aba?", "\xff"} {
		t.Run(s, func(t *testing.T) {
			ok, err := ipaRepresentationWitness(context.Background(), a, d.entries[0].representation, s)
			if err != nil || ok {
				t.Fatalf("%q: witness=%v err=%v", s, ok, err)
			}
		})
	}
}
func TestIPARepresentationExactPinsMutationAndFifthSelection(t *testing.T) {
	selection, input, raw := ipaRepresentationFixture(t)
	a := dictionaryCoreAccount(t)
	m, ok := LookupDictionaryManifest(selection.Manifest)
	if !ok || m.Rows != 453 || len(m.NegativeRelations) != 0 || m.Component.URL != "" {
		t.Fatalf("manifest=%+v", m)
	}
	cert, err := VerifyDictionaryComponent(context.Background(), a, m.Component.ID, raw)
	if err != nil {
		t.Fatal(err)
	}
	input.Certificate = &cert
	d, err := NewDictionarySet(context.Background(), a, []DictionarySelection{selection}, []DictionaryImport{input})
	if err != nil {
		t.Fatal(err)
	}
	input.IPA.Symbols[0].Encoding = "changed"
	input.IPA.Equivalences[0].EquivalentToEncoding = "changed"
	if ok, err := ipaRepresentationWitness(context.Background(), a, d.entries[0].representation, "ḉ"); err != nil || !ok {
		t.Fatalf("caller mutation: %v %v", ok, err)
	}
	d.Close()
	selections := []DictionarySelection{selection}
	for _, id := range []DictionaryManifestID{DictionaryManifestSMuFL14Canonical, DictionaryManifestSMuFL14Ranges, DictionaryManifestStandardSounds40, DictionaryManifestIPAPositive2015Unicode13} {
		mm, _ := LookupDictionaryManifest(id)
		selections = append(selections, DictionarySelection{id, mm.Relations})
	}
	d, err = NewDictionarySet(context.Background(), a, selections, nil)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
	selections = append(selections, selection)
	if d, err := NewDictionarySet(context.Background(), a, selections, nil); d != nil || !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("sixth=%v %v", d, err)
	}
	if dictionaryCoreUsage(t, a) != 17 {
		t.Fatal("retention leak")
	}
}

func TestIPARepresentationNormalizationPreservesStableOrderAndRawIntervals(t *testing.T) {
	d, a := ipaRepresentationSet(t)
	for _, tc := range []struct {
		text         string
		runes        []rune
		starts, ends []uint64
	}{
		{"ḉ\u0334", []rune{'c', '\u0334', '\u0327', '\u0301'}, []uint64{0, 3, 0, 0}, []uint64{3, 5, 3, 3}},
		{"\u02e2\u0315\u031a", []rune{'\u02e2', '\u0315', '\u031a'}, []uint64{0, 2, 4}, []uint64{2, 4, 6}},
		{"\u02e2\u031a\u0315", []rune{'\u02e2', '\u031a', '\u0315'}, []uint64{0, 2, 4}, []uint64{2, 4, 6}},
		{"\u0301\u0334a\u0300", []rune{'\u0334', '\u0301', 'a', '\u0300'}, []uint64{2, 0, 4, 5}, []uint64{4, 2, 5, 7}},
	} {
		t.Run(tc.text, func(t *testing.T) {
			before := dictionaryCoreUsage(t, a)
			n, ok, err := ipaRepresentationNormalize(context.Background(), a, d.entries[0].representation, tc.text)
			if err != nil || !ok {
				t.Fatalf("normalize %v %v", ok, err)
			}
			defer n.close()
			if len(n.leaves) != len(tc.runes) {
				t.Fatalf("leaves=%v", n.leaves)
			}
			for i, l := range n.leaves {
				if l.scalar != tc.runes[i] || l.start != tc.starts[i] || l.end != tc.ends[i] {
					t.Errorf("leaf%d=%+v", i, l)
				}
			}
			n.close()
			if dictionaryCoreUsage(t, a) != before {
				t.Fatal("normalization storage retained")
			}
		})
	}
}
func TestIPARepresentationConstructorMalformationsReleaseStorage(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		mutate       func(*DictionaryImport)
	}{
		{"wrong-count", "incorrect IPA representation record counts", func(i *DictionaryImport) { i.IPA.Symbols = i.IPA.Symbols[1:] }},
		{"example-branch", "incorrect IPA representation record counts", func(i *DictionaryImport) { i.IPA.Examples = []IPAAttestedExample{{"a", "example", "X"}} }},
		{"wrong-category", "transformed data identity mismatch", func(i *DictionaryImport) { i.IPA.Symbols[0].Category = "unreviewed-role" }},
		{"bad-nfd", "transformed data identity mismatch", func(i *DictionaryImport) { i.IPA.Equivalences[0].EquivalentToEncoding = "q" }},
		{"cyclic-nfd", "transformed data identity mismatch", func(i *DictionaryImport) { i.IPA.Equivalences[0].EquivalentToEncoding = i.IPA.Equivalences[0].Encoding }},
		{"duplicate", "duplicate record key", func(i *DictionaryImport) { i.IPA.Symbols[0] = i.IPA.Symbols[1] }},
		{"utf8", "invalid record string", func(i *DictionaryImport) { i.IPA.Symbols[0].Encoding = "\xff" }},
		{"wrong-branch", "wrong IPA payload branch", func(i *DictionaryImport) { i.IPA = nil; i.Sounds = &StandardSoundsDictionaryData{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selection, input, _ := ipaRepresentationFixture(t)
			tc.mutate(&input)
			a := dictionaryCoreAccount(t)
			d, err := NewDictionarySet(context.Background(), a, []DictionarySelection{selection}, []DictionaryImport{input})
			var config *ConfigurationError
			if d != nil || !errors.As(err, &config) || config.Reason != tc.reason {
				t.Fatalf("malformation=%v %v, want reason %q", d, err, tc.reason)
			}
			if dictionaryCoreUsage(t, a) != 17 {
				t.Fatal("failed constructor retained storage")
			}
		})
	}
}

func TestIPARepresentationNormalizationShortLongBoundaryStableMultiplicity(t *testing.T) {
	d, a := ipaRepresentationSet(t)
	classes := map[rune]uint8{}
	for _, row := range ipaRepresentationCCCValues {
		classes[row.scalar] = row.ccc
	}
	for _, prefix := range []string{"", "a"} {
		for _, count := range []int{0, 1, 7, 8, 9, 256} {
			text := prefix
			marks := []rune{'\u0315', '\u031a', '\u0301', '\u0334'}
			for i := 0; i < count; i++ {
				text += string(marks[i%len(marks)])
			}
			var want []ipaRepresentationLeaf
			for start, cp := range text {
				want = append(want, ipaRepresentationLeaf{scalar: cp, ccc: classes[cp], mark: cp != 'a', start: uint64(start), end: uint64(start + len(string(cp)))})
			}
			slices.SortStableFunc(want[len(prefix):], func(a, b ipaRepresentationLeaf) int { return int(a.ccc) - int(b.ccc) })
			n, known, err := ipaRepresentationNormalize(context.Background(), a, d.entries[0].representation, text)
			if err != nil || !known {
				t.Fatalf("prefix=%q count=%d: %v %v", prefix, count, known, err)
			}
			if !slices.Equal(n.leaves, want) {
				t.Errorf("prefix=%q count=%d: lost stable order, multiplicity or raw intervals: got%+v want%+v", prefix, count, n.leaves, want)
			}
			n.close()
		}
	}
}
