package validation

import (
	"context"
	"errors"
	"testing"
)

func TestDictionaryManifestsAreClosedAndOwned(t *testing.T) {
	ids := []DictionaryManifestID{DictionaryManifestSMuFL14Canonical, DictionaryManifestSMuFL14Ranges, DictionaryManifestStandardSounds40, DictionaryManifestIPAPositive2015Unicode13}
	for _, id := range ids {
		m, ok := LookupDictionaryManifest(id)
		if !ok || m.ID != id || len(m.Relations) == 0 || m.DataSHA256 == "" || m.Component.Bytes <= 0 {
			t.Fatalf("incomplete manifest: %+v", m)
		}
		original := m.Relations[0]
		m.Relations[0] = "caller-mutated"
		again, ok := LookupDictionaryManifest(id)
		if !ok || again.Relations[0] != original {
			t.Fatal("manifest slice aliases public copy")
		}
	}
	if _, ok := LookupDictionaryManifest("unknown"); ok {
		t.Fatal("unknown manifest accepted")
	}
}

func TestDictionaryComponentVerifierRejectsZeroWrongLengthAndCanceled(t *testing.T) {
	a := dictionaryCoreAccount(t)
	before := dictionaryCoreUsage(t, a)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		ctx   context.Context
		id    DictionaryComponentID
		bytes []byte
		want  error
	}{
		{nil, DictionaryComponentSMuFL14Glyphnames, nil, ErrInvalidConfiguration},
		{context.Background(), "unknown", nil, ErrInvalidConfiguration},
		{context.Background(), DictionaryComponentSMuFL14Glyphnames, nil, ErrInvalidConfiguration},
		{ctx, DictionaryComponentSMuFL14Glyphnames, nil, context.Canceled},
	} {
		cert, err := VerifyDictionaryComponent(tc.ctx, a, tc.id, tc.bytes)
		if cert != (DictionaryComponentCertificate{}) || !errors.Is(err, tc.want) {
			t.Fatalf("verification = %+v %v", cert, err)
		}
		if dictionaryCoreUsage(t, a) != before {
			t.Fatal("failed verifier leaked scratch")
		}
	}
}

func TestDictionaryConstructorRejectsContradictoryAndDuplicateImports(t *testing.T) {
	ctx := context.Background()
	a := dictionaryCoreAccount(t)
	canonical := DictionarySelection{Manifest: DictionaryManifestSMuFL14Canonical, Relations: []DictionaryRelation{DictionaryRelationCanonicalName}}
	good := DictionaryImport{Manifest: canonical.Manifest, State: EvidenceUnknown, Reason: "unavailable"}
	for _, tc := range []struct {
		name     string
		selected []DictionarySelection
		imports  []DictionaryImport
	}{
		{"duplicate-selection", []DictionarySelection{canonical, canonical}, nil},
		{"duplicate-relation", []DictionarySelection{{Manifest: canonical.Manifest, Relations: []DictionaryRelation{DictionaryRelationCanonicalName, DictionaryRelationCanonicalName}}}, nil},
		{"empty-relation", []DictionarySelection{{Manifest: canonical.Manifest}}, nil},
		{"unknown-relation", []DictionarySelection{{Manifest: canonical.Manifest, Relations: []DictionaryRelation{"unknown"}}}, nil},
		{"wrong-manifest-relation", []DictionarySelection{{Manifest: canonical.Manifest, Relations: []DictionaryRelation{DictionaryRelationStandardSound}}}, nil},
		{"extra-import", nil, []DictionaryImport{good}},
		{"duplicate-import", []DictionarySelection{canonical}, []DictionaryImport{good, good}},
		{"zero-state", []DictionarySelection{canonical}, []DictionaryImport{{Manifest: canonical.Manifest}}},
		{"unknown-no-reason", []DictionarySelection{canonical}, []DictionaryImport{{Manifest: canonical.Manifest, State: EvidenceUnknown}}},
		{"unknown-with-data", []DictionarySelection{canonical}, []DictionaryImport{{Manifest: canonical.Manifest, State: EvidenceUnknown, Reason: "unavailable", SMuFL: &SMuFLDictionaryData{}}}},
		{"unknown-with-certificate", []DictionarySelection{canonical}, []DictionaryImport{{Manifest: canonical.Manifest, State: EvidenceUnknown, Reason: "unavailable", Certificate: &DictionaryComponentCertificate{}}}},
		{"known-no-data", []DictionarySelection{canonical}, []DictionaryImport{{Manifest: canonical.Manifest, State: EvidenceKnown}}},
		{"known-wrong-union", []DictionarySelection{canonical}, []DictionaryImport{{Manifest: canonical.Manifest, State: EvidenceKnown, Sounds: &StandardSoundsDictionaryData{}}}},
		{"known-empty-data", []DictionarySelection{canonical}, []DictionaryImport{{Manifest: canonical.Manifest, State: EvidenceKnown, SMuFL: &SMuFLDictionaryData{}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := dictionaryCoreUsage(t, a)
			set, err := NewDictionarySet(ctx, a, tc.selected, tc.imports)
			if set != nil || !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("constructor = %v %v", set, err)
			}
			if dictionaryCoreUsage(t, a) != before {
				t.Fatal("failed constructor leaked storage")
			}
		})
	}
}
