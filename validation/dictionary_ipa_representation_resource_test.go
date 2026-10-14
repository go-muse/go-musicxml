package validation

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"unsafe"
)

func TestIPARepresentationRuntimeExactWorkStorageAndCancellation(t *testing.T) {
	d, _ := ipaRepresentationSet(t)
	p := d.entries[0].representation
	if int64(unsafe.Sizeof(*p))+1024 > ipaRepresentationIndexBytes {
		t.Fatal("index reservation below retained structure")
	}
	if unsafe.Sizeof(ipaRepresentationLeaf{}) > 32 {
		t.Fatal("normalization leaf reservation too small")
	}
	for _, text := range []string{"", "ḉ\u0334", "d\u0334\u0322\u0301", "t\u0301\ufe20s\u0300\ufe21", "a" + strings.Repeat("\u0301\u0334", 256)} {
		t.Run(text, func(t *testing.T) {
			probe := dictionaryCoreAccount(t)
			ctx := &dictionaryCancelContext{Context: context.Background(), cancelAt: math.MaxInt64}
			ok, err := ipaRepresentationWitness(ctx, probe, p, text)
			if err != nil || !ok {
				t.Fatalf("probe=%v %v", ok, err)
			}
			work, _ := probe.Usage(BudgetRuleSteps)
			for _, limit := range []int64{1, work / 2, work - 1, work} {
				if limit < 1 {
					continue
				}
				a, err := NewBudgetAccount(Budgets{MaxRuleSteps: limit})
				if err != nil {
					t.Fatal(err)
				}
				a.Charge(BudgetBufferedBytes, 17)
				ok, err := ipaRepresentationWitness(context.Background(), a, p, text)
				if limit == work {
					if err != nil || !ok {
						t.Fatalf("exact work=%d: %v %v", limit, ok, err)
					}
				} else if !errors.Is(err, ErrResourceLimit) {
					t.Fatalf("limited work=%d: %v", limit, err)
				}
				if dictionaryCoreUsage(t, a) != 17 {
					t.Fatal("work rejection leaked scratch")
				}
			}
			// Every context check can reject, including expansion, sorting, candidate
			// search and pair transitions. Sample all checks for short cases, and all
			// stages with bounded strides for the long combining-segment case.
			checks := ctx.checks.Load()
			stride := int64(1)
			if checks > 1000 {
				stride = checks / 100
			}
			for at := int64(1); at <= checks; at += stride {
				a := dictionaryCoreAccount(t)
				c := &dictionaryCancelContext{Context: context.Background(), cancelAt: at}
				ok, err := ipaRepresentationWitness(c, a, p, text)
				if ok || !errors.Is(err, context.Canceled) {
					t.Fatalf("cancel@%d/%d=%v %v", at, checks, ok, err)
				}
				if dictionaryCoreUsage(t, a) != 17 {
					t.Fatalf("cancel@%d retained scratch", at)
				}
			}
			count := 0
			for _, cp := range text {
				row, err := p.lookup(context.Background(), probe, cp)
				if err != nil {
					t.Fatal(err)
				}
				count += int(row.count)
			}
			peak := int64(4096 + 64*count + 17)
			for _, limit := range []int64{peak - 1, peak} {
				a, err := NewBudgetAccount(Budgets{MaxBufferedBytes: limit})
				if err != nil {
					t.Fatal(err)
				}
				a.Charge(BudgetBufferedBytes, 17)
				ok, err := ipaRepresentationWitness(context.Background(), a, p, text)
				if limit == peak {
					if err != nil || !ok {
						t.Fatalf("exact buffer=%d %v %v", peak, ok, err)
					}
				} else if !errors.Is(err, ErrResourceLimit) {
					t.Fatalf("under buffer=%v", err)
				}
				if dictionaryCoreUsage(t, a) != 17 {
					t.Fatal("buffer lifetime leak")
				}
			}
		})
	}
}

func TestIPARepresentationIndexLeaseSurvivesOwnerClose(t *testing.T) {
	selection, input, _ := ipaRepresentationFixture(t)
	a := dictionaryCoreAccount(t)
	d, err := NewDictionarySet(context.Background(), a, []DictionarySelection{selection}, []DictionaryImport{input})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := d.borrow(a)
	if err != nil {
		t.Fatal(err)
	}
	retained := dictionaryCoreUsage(t, a)
	p := d.entries[0].representation
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if dictionaryCoreUsage(t, a) != retained {
		t.Fatal("owner close released active index")
	}
	if ok, err := ipaRepresentationWitness(context.Background(), a, p, "ḉ"); err != nil || !ok {
		t.Fatalf("borrowed witness=%v %v", ok, err)
	}
	if _, err := d.borrow(a); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("closed borrow=%v", err)
	}
	lease.release()
	lease.release()
	if dictionaryCoreUsage(t, a) != 17 {
		t.Fatal("last borrower retained index")
	}
}
