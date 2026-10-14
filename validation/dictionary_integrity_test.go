package validation

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func dictionaryFixture(t *testing.T, id DictionaryManifestID) (DictionarySelection, DictionaryImport, []byte) {
	t.Helper()
	m, ok := LookupDictionaryManifest(id)
	if !ok {
		t.Fatal("unknown fixture")
	}
	var path string
	switch id {
	case DictionaryManifestSMuFL14Canonical:
		path = "../testdata/dictionaries/smufl-1.4/glyphnames.json"
	case DictionaryManifestSMuFL14Ranges:
		path = "../testdata/dictionaries/smufl-1.4/ranges.json"
	case DictionaryManifestStandardSounds40:
		path = "../testdata/dictionaries/musicxml-4.0/sounds.xml"
	case DictionaryManifestIPAPositive2015Unicode13:
		path = "../testdata/dictionaries/ipa-positive/runtime-positive-bundle.json"
	default:
		t.Fatal("fixture not integrated")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	input := DictionaryImport{Manifest: id, State: EvidenceKnown}
	cp := func(s string) uint32 {
		n, err := strconv.ParseUint(strings.TrimPrefix(s, "U+"), 16, 32)
		if err != nil {
			t.Fatal(err)
		}
		return uint32(n)
	}
	switch id {
	case DictionaryManifestSMuFL14Canonical:
		var rows map[string]struct{ Codepoint string }
		if err := json.Unmarshal(raw, &rows); err != nil {
			t.Fatal(err)
		}
		input.SMuFL = &SMuFLDictionaryData{}
		for name, row := range rows {
			input.SMuFL.Glyphs = append(input.SMuFL.Glyphs, SMuFLGlyph{Name: name, Codepoint: cp(row.Codepoint)})
		}
		slices.SortFunc(input.SMuFL.Glyphs, func(a, b SMuFLGlyph) int { return strings.Compare(b.Name, a.Name) })
	case DictionaryManifestSMuFL14Ranges:
		var rows map[string]struct {
			Start  string   `json:"range_start"`
			End    string   `json:"range_end"`
			Glyphs []string `json:"glyphs"`
		}
		if err := json.Unmarshal(raw, &rows); err != nil {
			t.Fatal(err)
		}
		input.SMuFL = &SMuFLDictionaryData{}
		for id, row := range rows {
			slices.Sort(row.Glyphs)
			slices.Reverse(row.Glyphs)
			input.SMuFL.Ranges = append(input.SMuFL.Ranges, SMuFLRange{ID: id, Start: cp(row.Start), End: cp(row.End), Names: row.Glyphs})
		}
		slices.SortFunc(input.SMuFL.Ranges, func(a, b SMuFLRange) int { return strings.Compare(b.ID, a.ID) })
	case DictionaryManifestStandardSounds40:
		input.Sounds = &StandardSoundsDictionaryData{}
		d := xml.NewDecoder(strings.NewReader(string(raw)))
		for {
			token, err := d.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if start, ok := token.(xml.StartElement); ok && start.Name.Local == "sound" {
				for _, a := range start.Attr {
					if a.Name.Local == "id" {
						input.Sounds.IDs = append(input.Sounds.IDs, a.Value)
					}
				}
			}
		}
		slices.Sort(input.Sounds.IDs)
		slices.Reverse(input.Sounds.IDs)
	case DictionaryManifestIPAPositive2015Unicode13:
		var bundle struct {
			Records []struct {
				Kind   string   `json:"record_type"`
				Fields []string `json:"fields"`
			}
		}
		if err := json.Unmarshal(raw, &bundle); err != nil {
			t.Fatal(err)
		}
		input.IPA = &IPAPositiveDictionaryData{}
		for _, row := range bundle.Records {
			if len(row.Fields) != 3 {
				t.Fatal("invalid reviewed fixture")
			}
			a, b, c := row.Fields[0], row.Fields[1], row.Fields[2]
			switch row.Kind {
			case "symbol":
				input.IPA.Symbols = append(input.IPA.Symbols, IPASymbolEncoding{a, b, c})
			case "example-sequence":
				input.IPA.Examples = append(input.IPA.Examples, IPAAttestedExample{a, b, c})
			case "canonical-equivalence":
				input.IPA.Equivalences = append(input.IPA.Equivalences, IPACanonicalEquivalent{a, b, c})
			default:
				t.Fatal("invalid reviewed record kind")
			}
		}
		slices.Reverse(input.IPA.Symbols)
		slices.Reverse(input.IPA.Examples)
		slices.Reverse(input.IPA.Equivalences)
	}
	return DictionarySelection{Manifest: id, Relations: m.Relations}, input, raw
}

func TestDictionaryReviewedMusicDigestsAndOwnedSortedRecords(t *testing.T) {
	for _, id := range []DictionaryManifestID{DictionaryManifestSMuFL14Canonical, DictionaryManifestSMuFL14Ranges, DictionaryManifestStandardSounds40, DictionaryManifestIPAPositive2015Unicode13} {
		t.Run(string(id), func(t *testing.T) {
			selected, input, _ := dictionaryFixture(t, id)
			a := dictionaryCoreAccount(t)
			set, err := NewDictionarySet(context.Background(), a, []DictionarySelection{selected}, []DictionaryImport{input})
			if err != nil {
				t.Fatal(err)
			}
			if set.entries[0].state != EvidenceKnown || set.entries[0].certificate {
				t.Fatal("typed data falsely claims raw verification")
			}
			entry := &set.entries[0]
			digest, bytes, err := dictionaryDataDigest(context.Background(), a, entry)
			if err != nil {
				t.Fatal(err)
			}
			if bytes != entry.manifest.DataBytes || digest == ([32]byte{}) {
				t.Fatal("wrong transformed data size")
			}
			switch id {
			case DictionaryManifestSMuFL14Canonical:
				if !slices.IsSortedFunc(entry.smufl.Glyphs, func(a, b SMuFLGlyph) int { return strings.Compare(a.Name, b.Name) }) {
					t.Fatal("glyphs not sorted")
				}
				input.SMuFL.Glyphs[0].Name = "caller mutation"
			case DictionaryManifestSMuFL14Ranges:
				input.SMuFL.Ranges[0].Names[0] = "caller mutation"
				input.SMuFL.Ranges[0].ID = "caller mutation"
			case DictionaryManifestStandardSounds40:
				input.Sounds.IDs[0] = "caller mutation"
			case DictionaryManifestIPAPositive2015Unicode13:
				input.IPA.Symbols[0].Encoding = "caller mutation"
				input.IPA.Examples[0].Category = "caller mutation"
				input.IPA.Equivalences[0].EquivalentToEncoding = "caller mutation"
			}
			again, _, err := dictionaryDataDigest(context.Background(), a, entry)
			if err != nil || again != digest {
				t.Fatalf("caller mutation changed store: %v", err)
			}
			if err := set.Close(); err != nil {
				t.Fatal(err)
			}
			if dictionaryCoreUsage(t, a) != 17 {
				t.Fatal("store retention leaked")
			}
		})
	}
}

func TestDictionaryCertificateCannotAuthorizeAlteredRecordsOrForeignAccounts(t *testing.T) {
	ctx := context.Background()
	selected, input, raw := dictionaryFixture(t, DictionaryManifestSMuFL14Canonical)
	a := dictionaryCoreAccount(t)
	m, _ := LookupDictionaryManifest(selected.Manifest)
	cert, err := VerifyDictionaryComponent(ctx, a, m.Component.ID, raw)
	if err != nil {
		t.Fatal(err)
	}
	input.Certificate = &cert
	set, err := NewDictionarySet(ctx, a, []DictionarySelection{selected}, []DictionaryImport{input})
	if err != nil {
		t.Fatal(err)
	}
	if !set.entries[0].certificate {
		t.Fatal("raw proof not recorded")
	}
	_ = set.Close()
	for _, tc := range []struct {
		name    string
		change  func(*DictionaryImport)
		account *Account
	}{
		{"altered", func(i *DictionaryImport) { i.SMuFL.Glyphs[0].Codepoint++ }, a},
		{"duplicate-name", func(i *DictionaryImport) { i.SMuFL.Glyphs[0].Name = i.SMuFL.Glyphs[1].Name }, a},
		{"zero-certificate", func(i *DictionaryImport) { i.Certificate = &DictionaryComponentCertificate{} }, a},
		{"foreign-certificate", func(i *DictionaryImport) {}, dictionaryCoreAccount(t)},
		{"wrong-component-certificate", func(i *DictionaryImport) {
			wrong := cert
			wrong.component = DictionaryComponentSMuFL14Ranges
			i.Certificate = &wrong
		}, a},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, in, _ := dictionaryFixture(t, selected.Manifest)
			in.Certificate = &cert
			tc.change(&in)
			before := dictionaryCoreUsage(t, tc.account)
			set, err := NewDictionarySet(ctx, tc.account, []DictionarySelection{selected}, []DictionaryImport{in})
			if set != nil || !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("forged input accepted: %v %v", set, err)
			}
			if dictionaryCoreUsage(t, tc.account) != before {
				t.Fatal("failed constructor leaked")
			}
		})
	}
}

func TestDictionaryCapabilityFingerprintPreservesActualSelectionStateAndLineage(t *testing.T) {
	ctx := context.Background()
	a := dictionaryCoreAccount(t)
	selected, input, raw := dictionaryFixture(t, DictionaryManifestSMuFL14Canonical)
	makeID := func(sel DictionarySelection, in []DictionaryImport) [32]byte {
		t.Helper()
		set, err := NewDictionarySet(ctx, a, []DictionarySelection{sel}, in)
		if err != nil {
			t.Fatal(err)
		}
		out, err := set.Fingerprint()
		if err != nil {
			t.Fatal(err)
		}
		_ = set.Close()
		if _, err := set.Fingerprint(); !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatal("closed fingerprint readable")
		}
		return out
	}
	derived := makeID(selected, []DictionaryImport{input})
	reverse := selected
	reverse.Relations = slices.Clone(selected.Relations)
	slices.Reverse(reverse.Relations)
	if makeID(reverse, []DictionaryImport{input}) != derived {
		t.Fatal("relation input ordering affects identity")
	}
	if makeID(selected, nil) == derived {
		t.Fatal("availability absent from identity")
	}
	subset := selected
	subset.Relations = []DictionaryRelation{DictionaryRelationCanonicalName}
	if makeID(subset, []DictionaryImport{input}) == derived {
		t.Fatal("relation selection absent from identity")
	}
	cert, err := VerifyDictionaryComponent(ctx, a, DictionaryComponentSMuFL14Glyphnames, raw)
	if err != nil {
		t.Fatal(err)
	}
	input.Certificate = &cert
	if makeID(selected, []DictionaryImport{input}) == derived {
		t.Fatal("this-run certificate absent from identity")
	}
	first := makeID(selected, []DictionaryImport{{Manifest: selected.Manifest, State: EvidenceUnknown, Reason: "missing"}})
	second := makeID(selected, []DictionaryImport{{Manifest: selected.Manifest, State: EvidenceUnknown, Reason: "provider-failed"}})
	if first == second {
		t.Fatal("unavailable reason absent from identity")
	}
}

func TestDictionaryTypedConstructionExactWorkRetentionAndScalarBudgets(t *testing.T) {
	ctx := context.Background()
	selected, input, _ := dictionaryFixture(t, DictionaryManifestSMuFL14Canonical)
	probe := dictionaryCoreAccount(t)
	set, err := NewDictionarySet(ctx, probe, []DictionarySelection{selected}, []DictionaryImport{input})
	if err != nil {
		t.Fatal(err)
	}
	retention := dictionaryCoreUsage(t, probe)
	work, _ := probe.Usage(BudgetRuleSteps)
	_ = set.Close()
	for _, tc := range []struct {
		name     string
		limits   Budgets
		succeeds bool
	}{
		{"exact-retention", Budgets{MaxBufferedBytes: retention}, true},
		{"one-under-retention", Budgets{MaxBufferedBytes: retention - 1}, false},
		{"exact-work", Budgets{MaxRuleSteps: work}, true},
		{"one-under-work", Budgets{MaxRuleSteps: work - 1}, false},
		{"exact-scalar", Budgets{MaxScalarBytes: 44}, true},
		{"one-under-scalar", Budgets{MaxScalarBytes: 43}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := NewBudgetAccount(tc.limits)
			if err != nil {
				t.Fatal(err)
			}
			_ = a.Charge(BudgetBufferedBytes, 17)
			set, err := NewDictionarySet(ctx, a, []DictionarySelection{selected}, []DictionaryImport{input})
			if tc.succeeds {
				if err != nil {
					t.Fatal(err)
				}
				_ = set.Close()
			} else if set != nil || !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("budget accepted: %v %v", set, err)
			}
			if dictionaryCoreUsage(t, a) != 17 {
				t.Fatal("limit failure changed caller retention")
			}
		})
	}
}

func TestDictionaryPartialConstructorFailurePreservesCallerAndCumulativeWork(t *testing.T) {
	ctx := context.Background()
	a := dictionaryCoreAccount(t)
	one, input, _ := dictionaryFixture(t, DictionaryManifestSMuFL14Canonical)
	two, bad, _ := dictionaryFixture(t, DictionaryManifestStandardSounds40)
	bad.Sounds.IDs[0] = bad.Sounds.IDs[1]
	set, err := NewDictionarySet(ctx, a, []DictionarySelection{one, two}, []DictionaryImport{input, bad})
	if set != nil || !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("bad later import accepted: %v %v", set, err)
	}
	if dictionaryCoreUsage(t, a) != 17 {
		t.Fatal("partial construction leaked")
	}
	if n, _ := a.Usage(BudgetRuleSteps); n < 1000 {
		t.Fatal("failed import refunded work")
	}
}

type dictionaryCancelContext struct {
	context.Context
	checks   atomic.Int64
	cancelAt int64
}

func (c *dictionaryCancelContext) Err() error {
	if c.checks.Add(1) >= c.cancelAt {
		return context.Canceled
	}
	return nil
}

func TestDictionaryVerifierExactBudgetsCorruptionAndFinalCancellation(t *testing.T) {
	_, _, raw := dictionaryFixture(t, DictionaryManifestStandardSounds40)
	id := DictionaryComponentStandardSounds40
	ctx := context.Background()
	probe := dictionaryCoreAccount(t)
	cert, err := VerifyDictionaryComponent(ctx, probe, id, raw)
	if err != nil || cert == (DictionaryComponentCertificate{}) {
		t.Fatalf("proof: %v", err)
	}
	work, _ := probe.Usage(BudgetRuleSteps)
	for _, tc := range []struct {
		name   string
		limits Budgets
		want   bool
	}{
		{"exact-work", Budgets{MaxRuleSteps: work}, true}, {"one-under-work", Budgets{MaxRuleSteps: work - 1}, false},
		{"exact-scratch", Budgets{MaxBufferedBytes: 529}, true}, {"one-under-scratch", Budgets{MaxBufferedBytes: 528}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := NewBudgetAccount(tc.limits)
			_ = a.Charge(BudgetBufferedBytes, 17)
			proof, err := VerifyDictionaryComponent(ctx, a, id, raw)
			if tc.want {
				if err != nil {
					t.Fatal(err)
				}
			} else if proof != (DictionaryComponentCertificate{}) || !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("proof: %+v %v", proof, err)
			}
			if dictionaryCoreUsage(t, a) != 17 {
				t.Fatal("scratch leaked")
			}
			for _, k := range []BudgetKind{BudgetExternalBytes, BudgetInputBytes} {
				if n, _ := a.Usage(k); n != 0 {
					t.Fatalf("verifier counted transport %s", k)
				}
			}
		})
	}
	broken := slices.Clone(raw)
	broken[len(broken)/2] ^= 1
	if proof, err := VerifyDictionaryComponent(ctx, probe, id, broken); proof != (DictionaryComponentCertificate{}) || !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("corruption: %+v %v", proof, err)
	}
	count := &dictionaryCancelContext{Context: ctx, cancelAt: 1 << 60}
	if _, err := VerifyDictionaryComponent(count, probe, id, raw); err != nil {
		t.Fatal(err)
	}
	final := &dictionaryCancelContext{Context: ctx, cancelAt: count.checks.Load()}
	if proof, err := VerifyDictionaryComponent(final, probe, id, raw); proof != (DictionaryComponentCertificate{}) || !errors.Is(err, context.Canceled) {
		t.Fatalf("final cancellation: %+v %v", proof, err)
	}
	if dictionaryCoreUsage(t, probe) != 17 {
		t.Fatal("canceled verifier leaked")
	}
}

func TestDictionaryBorrowedCapabilitiesAndLookupSurviveOwnerClose(t *testing.T) {
	ctx := context.Background()
	a := dictionaryCoreAccount(t)
	selected, input, _ := dictionaryFixture(t, DictionaryManifestSMuFL14Canonical)
	rangeSelection := DictionarySelection{Manifest: DictionaryManifestSMuFL14Ranges, Relations: []DictionaryRelation{DictionaryRelationRangeMembership}}
	set, err := NewDictionarySet(ctx, a, []DictionarySelection{selected, rangeSelection}, []DictionaryImport{input})
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := set.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	e, unused := sessionFixture(t, Budgets{})
	_, _ = unused.Finish(ctx, Completion{Syntax: SyntaxIncomplete})
	s, err := e.BeginWithDictionaries(ctx, RunOptions{DocumentID: "lookup", Profile: "test.source", Account: a}, set)
	if err != nil {
		t.Fatal(err)
	}
	if err := set.Close(); err != nil {
		t.Fatal(err)
	}
	if s.dictionaryFingerprint() != fingerprint {
		t.Fatal("owner close invalidated session identity")
	}
	entry, known, err := s.dictionaryCapability(ctx, selected.Manifest, DictionaryRelationCanonicalName)
	if err != nil || known.State != EvidenceKnown || !known.Value || entry == nil {
		t.Fatalf("borrowed capability: %+v %v", known, err)
	}
	index, found, err := dictionarySearch(ctx, a, len(entry.smufl.Glyphs), "gClef", func(i int) string { return entry.smufl.Glyphs[i].Name })
	if err != nil || !found || entry.smufl.Glyphs[index].Codepoint != 0xe050 {
		t.Fatalf("borrowed lookup: %d %v %v", index, found, err)
	}
	_, gap, err := s.dictionaryCapability(ctx, rangeSelection.Manifest, DictionaryRelationRangeMembership)
	if err != nil || gap.State != EvidenceUnknown || gap.Reason != "dictionary-not-supplied" {
		t.Fatalf("missing independent range: %+v %v", gap, err)
	}
	if _, err := s.Finish(ctx, Completion{Syntax: SyntaxIncomplete}); err != nil {
		t.Fatal(err)
	}
	if s.dictionaryFingerprint() != ([32]byte{}) || dictionaryCoreUsage(t, a) != 17 {
		t.Fatal("finished borrower retained capability")
	}
}

func TestDictionaryLookupExaminesLogarithmicallyManyKeysAndChargesLongQueries(t *testing.T) {
	ctx := context.Background()
	a := dictionaryCoreAccount(t)
	selected, input, _ := dictionaryFixture(t, DictionaryManifestSMuFL14Canonical)
	set, err := NewDictionarySet(ctx, a, []DictionarySelection{selected}, []DictionaryImport{input})
	if err != nil {
		t.Fatal(err)
	}
	rows := set.entries[0].smufl.Glyphs
	before, _ := a.Usage(BudgetRuleSteps)
	comparisons := 0
	_, found, err := dictionarySearch(ctx, a, len(rows), strings.Repeat("z", 10000), func(i int) string { comparisons++; return rows[i].Name })
	if err != nil || found || comparisons > 12 {
		t.Fatalf("lookup scanned corpus: %d %v %v", comparisons, found, err)
	}
	after, _ := a.Usage(BudgetRuleSteps)
	if after-before < 10000 || after-before > 150000 {
		t.Fatalf("query work not proportional: %d", after-before)
	}
	_, _, err = dictionarySearch(ctx, a, len(rows), strings.Repeat("x", int(a.Limits().MaxScalarBytes)+1), func(i int) string { t.Fatal("oversize query reached records"); return "" })
	if !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("oversize query accepted: %v", err)
	}
	_ = set.Close()
}

func TestDictionaryHostileTypedIngressIsBoundedBeforeCopy(t *testing.T) {
	ctx := context.Background()
	selected, base, _ := dictionaryFixture(t, DictionaryManifestSMuFL14Canonical)
	for _, tc := range []struct {
		name   string
		change func(*DictionaryImport)
	}{
		{"excessive-count", func(i *DictionaryImport) { i.SMuFL.Glyphs = append(i.SMuFL.Glyphs, SMuFLGlyph{}) }},
		{"excessive-name", func(i *DictionaryImport) { i.SMuFL.Glyphs[0].Name = strings.Repeat("x", 1<<20) }},
		{"invalid-utf8", func(i *DictionaryImport) { i.SMuFL.Glyphs[0].Name = "\xff" }},
		{"embedded-nul", func(i *DictionaryImport) { i.SMuFL.Glyphs[0].Name = "a\x00b" }},
		{"invalid-codepoint", func(i *DictionaryImport) { i.SMuFL.Glyphs[0].Codepoint = 0xd800 }},
		{"extra-branch", func(i *DictionaryImport) { i.SMuFL.Ranges = []SMuFLRange{{}} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := dictionaryCoreAccount(t)
			in := base
			copyData := *base.SMuFL
			copyData.Glyphs = slices.Clone(base.SMuFL.Glyphs)
			in.SMuFL = &copyData
			tc.change(&in)
			set, err := NewDictionarySet(ctx, a, []DictionarySelection{selected}, []DictionaryImport{in})
			if set != nil || !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("hostile input accepted: %v %v", set, err)
			}
			if dictionaryCoreUsage(t, a) != 17 {
				t.Fatal("hostile input leaked")
			}
			if n, _ := a.Usage(BudgetRuleSteps); n > 10000 {
				t.Fatalf("hostile first row caused unbounded work: %d", n)
			}
		})
	}
}

func TestDictionaryIPADerivedBundleDoesNotAcquireOriginalSourceOrNegativeAuthority(t *testing.T) {
	ctx := context.Background()
	a := dictionaryCoreAccount(t)
	selected, input, raw := dictionaryFixture(t, DictionaryManifestIPAPositive2015Unicode13)
	cert, err := VerifyDictionaryComponent(ctx, a, DictionaryComponentIPAPositive2015Unicode13, raw)
	if err != nil {
		t.Fatal(err)
	}
	input.Certificate = &cert
	set, err := NewDictionarySet(ctx, a, []DictionarySelection{selected}, []DictionaryImport{input})
	if err != nil {
		t.Fatal(err)
	}
	entry := &set.entries[0]
	if !entry.certificate || entry.manifest.Component.SourceKind != DictionarySourceReviewedDerivedBundle || len(entry.manifest.NegativeRelations) != 0 {
		t.Fatalf("IPA authority broadened: %+v", entry.manifest)
	}
	if len(entry.ipa.Symbols) != 170 || len(entry.ipa.Examples) != 18 || len(entry.ipa.Equivalences) != 9 {
		t.Fatal("reviewed partial row counts changed")
	}
	_, exists, err := dictionarySearch(ctx, a, len(entry.ipa.Symbols), "g", func(i int) string { return entry.ipa.Symbols[i].Encoding })
	if err != nil || exists {
		t.Fatal("unreviewed ASCII g became positive")
	}
	idx, exists, err := dictionarySearch(ctx, a, len(entry.ipa.Symbols), "ɡ", func(i int) string { return entry.ipa.Symbols[i].Encoding })
	if err != nil || !exists || entry.ipa.Symbols[idx].RowID != "S-0261" {
		t.Fatal("reviewed IPA g missing")
	}
	_ = set.Close()
	input.IPA.Symbols[0].Category = "altered"
	if set, err := NewDictionarySet(ctx, a, []DictionarySelection{selected}, []DictionaryImport{input}); set != nil || !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("raw bundle authorized altered positive rows: %v %v", set, err)
	}
	if dictionaryCoreUsage(t, a) != 17 {
		t.Fatal("partial IPA constructor leaked")
	}
}

func TestDictionaryConstructorCancellationReleasesEveryPartialOwnedStage(t *testing.T) {
	selected, input, _ := dictionaryFixture(t, DictionaryManifestSMuFL14Canonical)
	count := &dictionaryCancelContext{Context: context.Background(), cancelAt: 1 << 60}
	probe := dictionaryCoreAccount(t)
	complete, err := NewDictionarySet(count, probe, []DictionarySelection{selected}, []DictionaryImport{input})
	if err != nil {
		t.Fatal(err)
	}
	total := count.checks.Load()
	_ = complete.Close()
	for _, cutoff := range []int64{1, 10, 3000, 5000, total - 1, total} {
		t.Run(strconv.FormatInt(cutoff, 10), func(t *testing.T) {
			a := dictionaryCoreAccount(t)
			ctx := &dictionaryCancelContext{Context: context.Background(), cancelAt: cutoff}
			set, err := NewDictionarySet(ctx, a, []DictionarySelection{selected}, []DictionaryImport{input})
			if set != nil || !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation resumed: %v %v", set, err)
			}
			if dictionaryCoreUsage(t, a) != 17 {
				t.Fatal("canceled construction leaked")
			}
		})
	}
}

func TestDictionaryUnknownRelationsAndInputAliasesRemainIndependent(t *testing.T) {
	ctx := context.Background()
	a := dictionaryCoreAccount(t)
	selected, input, _ := dictionaryFixture(t, DictionaryManifestSMuFL14Canonical)
	selected.Relations = []DictionaryRelation{DictionaryRelationCanonicalName}
	set, err := NewDictionarySet(ctx, a, []DictionarySelection{selected}, []DictionaryImport{input})
	if err != nil {
		t.Fatal(err)
	}
	selected.Relations[0] = DictionaryRelationPrimaryCodepoint
	e, unused := sessionFixture(t, Budgets{})
	_, _ = unused.Finish(ctx, Completion{Syntax: SyntaxIncomplete})
	s, err := e.BeginWithDictionaries(ctx, RunOptions{DocumentID: "selected", Profile: "test.source", Account: a}, set)
	if err != nil {
		t.Fatal(err)
	}
	_, known, err := s.dictionaryCapability(ctx, input.Manifest, DictionaryRelationCanonicalName)
	if err != nil || known.State != EvidenceKnown {
		t.Fatal("caller mutated retained selections")
	}
	_, gap, err := s.dictionaryCapability(ctx, input.Manifest, DictionaryRelationPrimaryCodepoint)
	if err != nil || gap.State != EvidenceUnknown || gap.Reason != "dictionary-relation-not-selected" {
		t.Fatalf("unselected relation gained authority: %+v %v", gap, err)
	}
	_, _ = s.Finish(ctx, Completion{Syntax: SyntaxIncomplete})
	_ = set.Close()
}
