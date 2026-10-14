package validation

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"
	"testing"
)

func TestDictionaryNegativeRelationMetadataMatchesClosedConsumerPolicy(t *testing.T) {
	for _, tc := range []struct {
		id       DictionaryManifestID
		negative []DictionaryRelation
	}{
		{DictionaryManifestSMuFL14Canonical, []DictionaryRelation{DictionaryRelationCanonicalName}},
		{DictionaryManifestSMuFL14Ranges, []DictionaryRelation{DictionaryRelationRangeMembership}},
		{DictionaryManifestStandardSounds40, []DictionaryRelation{DictionaryRelationStandardSound}},
		{DictionaryManifestIPAPositive2015Unicode13, nil},
	} {
		t.Run(string(tc.id), func(t *testing.T) {
			m, ok := LookupDictionaryManifest(tc.id)
			if !ok || !slices.Equal(m.NegativeRelations, tc.negative) {
				t.Fatalf("negative-evidence metadata = %v, want %v", m.NegativeRelations, tc.negative)
			}
			for _, relation := range m.NegativeRelations {
				if !slices.Contains(m.Relations, relation) {
					t.Errorf("negative relation %s is not selected by the manifest", relation)
				}
			}
			if len(m.NegativeRelations) > 0 {
				m.NegativeRelations[0] = DictionaryRelationIPASymbol
			}
			m.NegativeRelations = append(m.NegativeRelations, DictionaryRelationIPAAttestedExample)
			again, ok := LookupDictionaryManifest(tc.id)
			if !ok || !slices.Equal(again.NegativeRelations, tc.negative) {
				t.Fatalf("caller mutation changed reviewed negative-evidence policy: %v", again.NegativeRelations)
			}
		})
	}
}

func TestDictionaryPlanSizeIsConservativeAllocationFreeAndDigestStable(t *testing.T) {
	p := &dictionaryPlan{Operation: DictionaryCanonical, Bindings: &contextBindings{bytes: 37}, Digest: strings.Repeat("d", 64)}
	p.Types = []ComponentID{contextNamedType("smufl-glyph-name")}
	legacy := int64(1024+len(p.Digest)) + p.Bindings.size()
	for _, typ := range p.Types {
		legacy += contextComponentBytes(typ) + 128
	}
	for _, id := range []DictionaryManifestID{DictionaryManifestSMuFL14Canonical, DictionaryManifestSMuFL14Ranges, DictionaryManifestStandardSounds40, DictionaryManifestIPAPositive2015Unicode13} {
		m, ok := LookupDictionaryManifest(id)
		if !ok {
			t.Fatal("reviewed manifest unavailable")
		}
		p.Pins = append(p.Pins, m)
		encoded, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		legacy += int64(len(encoded)) + 256
	}
	before, err := json.Marshal(p.digest())
	if err != nil {
		t.Fatal(err)
	}
	if got := p.size(); got < legacy {
		t.Errorf("plan size %d is below the prior reviewed-pins bound %d", got, legacy)
	}
	var measured int64
	if allocations := testing.AllocsPerRun(100, func() { measured = p.size() }); allocations != 0 {
		t.Errorf("plan size allocated %g times", allocations)
	}
	if measured != p.size() {
		t.Fatal("repeated sizing changed the plan bound")
	}
	after, err := json.Marshal(p.digest())
	if err != nil {
		t.Fatal(err)
	}
	want := `{"Kind":"pinned-dictionary","Operation":"canonical-name","BindingPolicy":"` + p.Digest + `"}`
	if string(before) != want || string(after) != want {
		t.Fatalf("typed digest changed during sizing: before %s, after %s", before, after)
	}
	var absent *dictionaryPlan
	if absent.size() != 0 {
		t.Fatal("nil plan has nonzero size")
	}
}

func TestDictionaryComparisonChecksContextOnceBeforeWorkAndQuota(t *testing.T) {
	ctx := &dictionaryCancelContext{Context: context.Background(), cancelAt: math.MaxInt64}
	a, err := NewBudgetAccount(Budgets{MaxRuleSteps: 26})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Charge(BudgetRuleSteps, 7); err != nil {
		t.Fatal(err)
	}
	if err := dictionaryComparisonStep(ctx, a, 7, 11); err != nil {
		t.Fatal(err)
	}
	if checks := ctx.checks.Load(); checks != 1 {
		t.Errorf("context checked %d times for one comparison", checks)
	}
	if got, err := a.Usage(BudgetRuleSteps); err != nil || got != 26 {
		t.Fatalf("exact comparison usage = %d, %v; want 26", got, err)
	}
	if err := dictionaryComparisonStep(context.Background(), a, 0, 0); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("one-over quota comparison = %v", err)
	}
	if got, err := a.Usage(BudgetRuleSteps); err != nil || got != 26 {
		t.Fatalf("quota refusal changed usage: %d, %v", got, err)
	}
}

func TestDictionaryComparisonContextPrecedesOverflowAndAccount(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		name        string
		ctx         context.Context
		left, right int64
		account     *Account
		want        error
	}{
		{"nil-context-before-overflow", nil, math.MaxInt64, 1, dictionaryCoreAccount(t), ErrInvalidConfiguration},
		{"canceled-before-overflow", canceled, math.MaxInt64, 1, dictionaryCoreAccount(t), context.Canceled},
		{"canceled-before-invalid-account", canceled, 1, 1, nil, context.Canceled},
		{"overflow-before-invalid-account", context.Background(), math.MaxInt64, 1, nil, ErrResourceLimit},
		{"valid-work-invalid-account", context.Background(), 1, 1, nil, ErrInvalidConfiguration},
		{"valid-work-zero-account", context.Background(), 1, 1, &Account{}, ErrInvalidConfiguration},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := dictionaryComparisonStep(tc.ctx, tc.account, tc.left, tc.right); !errors.Is(err, tc.want) {
				t.Fatalf("comparison = %v, want %v", err, tc.want)
			}
			if tc.account != nil && tc.account.initialized {
				if used, err := tc.account.Usage(BudgetRuleSteps); err != nil || used != 0 {
					t.Fatalf("refused comparison charged work: %d, %v", used, err)
				}
			}
		})
	}
}
