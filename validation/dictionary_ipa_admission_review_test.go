package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unsafe"
)

func TestDictionaryIPAMaximaAreImmutableOwnedByteBounds(t *testing.T) {
	selected, input, _ := dictionaryFixture(t, DictionaryManifestIPAPositive2015Unicode13)
	a := dictionaryCoreAccount(t)
	set, err := NewDictionarySet(context.Background(), a, []DictionarySelection{selected}, []DictionaryImport{input})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := set.Close(); err != nil {
			t.Error(err)
		}
		if got := dictionaryCoreUsage(t, a); got != 17 {
			t.Errorf("IPA set cleanup changed caller retention: %d", got)
		}
	})
	entry := &set.entries[0]
	check := func() {
		t.Helper()
		// These independently reviewed fixture maxima are UTF-8 byte lengths,
		// including a two-scalar canonical-equivalence spelling.
		if entry.ipaSymbolMaxBytes != 3 || entry.ipaEquivalentMaxBytes != 3 {
			t.Errorf("IPA maxima = %d, %d; want 3, 3", entry.ipaSymbolMaxBytes, entry.ipaEquivalentMaxBytes)
		}
		for _, row := range entry.ipa.Symbols {
			if len(row.Encoding) > entry.ipaSymbolMaxBytes {
				t.Errorf("symbol %q exceeds cached maximum", row.Encoding)
			}
		}
		for _, row := range entry.ipa.Equivalences {
			if len(row.Encoding) > entry.ipaEquivalentMaxBytes {
				t.Errorf("equivalence %q exceeds cached maximum", row.Encoding)
			}
		}
	}
	check()
	input.IPA.Symbols[0].Encoding = strings.Repeat("x", 16)
	input.IPA.Equivalences[0].Encoding = strings.Repeat("y", 16)
	check()
	// The entry reservation includes all descriptor headers, up to three
	// relations in each owned slice and the bounded unavailable reason.
	// IPA record arrays/string storage are separately reserved by copyIPA.
	fixed := int64(unsafe.Sizeof(dictionaryEntry{})) + 3*3*int64(unsafe.Sizeof(DictionaryRelation(""))) + 128
	if fixed > dictionaryEntryFixedBytes {
		t.Fatalf("entry metadata needs %d bytes, envelope is %d", fixed, dictionaryEntryFixedBytes)
	}
}

func TestDictionaryIPACopyMaximaUseSeparateTablesAndSixteenByteAdmission(t *testing.T) {
	_, input, _ := dictionaryFixture(t, DictionaryManifestIPAPositive2015Unicode13)
	input.IPA.Symbols[0].Encoding = strings.Repeat("ɡ", 8)            // 16 UTF-8 bytes.
	input.IPA.Equivalences[0].Encoding = "a" + strings.Repeat("ɡ", 7) // 15 bytes.
	a := dictionaryCoreAccount(t)
	set := &DictionarySet{initialized: true, account: a}
	t.Cleanup(func() {
		if err := set.Close(); err != nil {
			t.Error(err)
		}
		if got := dictionaryCoreUsage(t, a); got != 17 {
			t.Errorf("IPA copy cleanup changed caller retention: %d", got)
		}
	})
	entry := &dictionaryEntry{}
	if err := set.copyIPA(context.Background(), entry, input.IPA); err != nil {
		t.Fatal(err)
	}
	if entry.ipaSymbolMaxBytes != 16 || entry.ipaEquivalentMaxBytes != 15 {
		t.Fatalf("separate byte maxima = %d, %d; want 16, 15", entry.ipaSymbolMaxBytes, entry.ipaEquivalentMaxBytes)
	}
}

func TestDictionaryIPAConstructorRejectsInvalidEncodingBoundsWithoutRetention(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*IPAPositiveDictionaryData)
	}{
		{"symbol-17-bytes", func(p *IPAPositiveDictionaryData) { p.Symbols[0].Encoding = strings.Repeat("x", 17) }},
		{"last-symbol-17-bytes", func(p *IPAPositiveDictionaryData) { p.Symbols[len(p.Symbols)-1].Encoding = strings.Repeat("x", 17) }},
		{"example-17-bytes", func(p *IPAPositiveDictionaryData) { p.Examples[0].Encoding = strings.Repeat("x", 17) }},
		{"equivalence-17-bytes", func(p *IPAPositiveDictionaryData) { p.Equivalences[0].Encoding = strings.Repeat("x", 17) }},
		{"symbol-invalid-utf8", func(p *IPAPositiveDictionaryData) { p.Symbols[0].Encoding = "\xff" }},
		{"empty-example", func(p *IPAPositiveDictionaryData) { p.Examples[0].Encoding = "" }},
		{"equivalence-nul", func(p *IPAPositiveDictionaryData) { p.Equivalences[0].Encoding = "a\x00" }},
		{"changed-bounded-symbol", func(p *IPAPositiveDictionaryData) { p.Symbols[0].Encoding = strings.Repeat("x", 16) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selected, input, _ := dictionaryFixture(t, DictionaryManifestIPAPositive2015Unicode13)
			tc.change(input.IPA)
			a := dictionaryCoreAccount(t)
			set, err := NewDictionarySet(context.Background(), a, []DictionarySelection{selected}, []DictionaryImport{input})
			if set != nil || !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("invalid or unreviewed IPA rows admitted: %v, %v", set, err)
			}
			if got := dictionaryCoreUsage(t, a); got != 17 {
				t.Fatalf("IPA constructor failure changed caller retention: %d", got)
			}
		})
	}
}
