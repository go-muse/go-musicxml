package validation

import (
	"errors"
	"math"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
)

var testBudgetKinds = []struct {
	kind  BudgetKind
	field string
	mode  string
}{
	{BudgetInputBytes, "MaxInputBytes", "cumulative"}, {BudgetExternalBytes, "MaxExternalBytes", "cumulative"}, {BudgetBufferedBytes, "MaxBufferedBytes", "gauge"}, {BudgetDepth, "MaxDepth", "gauge"}, {BudgetNodes, "MaxNodes", "cumulative"}, {BudgetAttributes, "MaxAttributes", "cumulative"}, {BudgetEvents, "MaxEvents", "cumulative"}, {BudgetScalarBytes, "MaxScalarBytes", "item"}, {BudgetIDs, "MaxIDs", "cumulative"}, {BudgetRuleSteps, "MaxRuleSteps", "cumulative"}, {BudgetContextFacts, "MaxContextFacts", "cumulative"}, {BudgetLinkedDocuments, "MaxLinkedDocuments", "cumulative"}, {BudgetDiagnostics, "MaxDiagnostics", "cumulative"}, {BudgetDiagnosticBytes, "MaxDiagnosticBytes", "cumulative"},
}

func testLimits(n int64) Budgets {
	var limits Budgets
	fields := reflect.ValueOf(&limits).Elem()
	for i := 0; i < fields.NumField(); i++ {
		fields.Field(i).SetInt(n)
	}
	return limits
}

func TestBudgetDefaultsFiniteCopiedAndEachFieldNegativeRejected(t *testing.T) {
	account, err := NewBudgetAccount(Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	defaults := DefaultBudgets()
	if got := account.Limits(); got != defaults {
		t.Fatalf("normalized limits = %+v want %+v", got, defaults)
	}
	want := Budgets{MaxInputBytes: 64 << 20, MaxExternalBytes: 64 << 20, MaxBufferedBytes: 64 << 20, MaxDepth: 1024, MaxNodes: 1000000, MaxAttributes: 4000000, MaxEvents: 8000000, MaxScalarBytes: 1 << 20, MaxIDs: 1000000, MaxRuleSteps: 20000000, MaxContextFacts: 1000000, MaxLinkedDocuments: 128, MaxDiagnostics: 1000, MaxDiagnosticBytes: 1 << 20}
	if defaults != want {
		t.Errorf("default contract = %+v want %+v", defaults, want)
	}
	defaults.MaxNodes = 1
	snapshot := account.Limits()
	snapshot.MaxDepth = 1
	if account.Limits() != want || DefaultBudgets() != want {
		t.Error("limits/defaults are mutable through returned value")
	}
	if len(testBudgetKinds) != reflect.TypeFor[Budgets]().NumField() {
		t.Fatal("budget field test coverage incomplete")
	}
	for _, tc := range testBudgetKinds {
		t.Run(tc.field, func(t *testing.T) {
			var limits Budgets
			reflect.ValueOf(&limits).Elem().FieldByName(tc.field).SetInt(-1)
			if account, err := NewBudgetAccount(limits); account != nil || !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("negative limit = %v,%v", account, err)
			}
			reflect.ValueOf(&limits).Elem().FieldByName(tc.field).SetInt(7)
			account, err := NewBudgetAccount(limits)
			if err != nil {
				t.Fatal(err)
			}
			if got := reflect.ValueOf(account.Limits()).FieldByName(tc.field).Int(); got != 7 {
				t.Fatalf("custom %s = %d", tc.field, got)
			}
		})
	}
}

func TestEveryBudgetKindExactLimitModeAndFailureAtomicity(t *testing.T) {
	for _, tc := range testBudgetKinds {
		t.Run(string(tc.kind), func(t *testing.T) {
			account, err := NewBudgetAccount(testLimits(3))
			if err != nil {
				t.Fatal(err)
			}
			if tc.mode == "item" {
				for i := 0; i < 3; i++ {
					if err := account.Check(tc.kind, 3); err != nil {
						t.Fatal(err)
					}
				}
				if err := account.Check(tc.kind, 4); !errors.Is(err, ErrResourceLimit) {
					t.Errorf("oversize item = %v", err)
				}
				if err := account.Charge(tc.kind, 1); !errors.Is(err, ErrInvalidConfiguration) {
					t.Errorf("charged item limit = %v", err)
				}
				if err := account.Release(tc.kind, 1); !errors.Is(err, ErrInvalidConfiguration) {
					t.Errorf("released item limit = %v", err)
				}
				if got, _ := account.Usage(tc.kind); got != 0 {
					t.Errorf("item check consumes quota: %d", got)
				}
				return
			}
			if err := account.Charge(tc.kind, 2); err != nil {
				t.Fatal(err)
			}
			if err := account.Charge(tc.kind, 1); err != nil {
				t.Fatal(err)
			}
			err = account.Charge(tc.kind, 1)
			var detail *ResourceError
			if !errors.As(err, &detail) || detail.Kind != tc.kind || detail.Limit != 3 || detail.Used != 3 || detail.Requested != 1 {
				t.Errorf("limit detail = %#v (%v)", detail, err)
			}
			if !errors.Is(err, ErrIncompleteAssessment) {
				t.Errorf("resource must mark incomplete: %v", err)
			}
			if got, _ := account.Usage(tc.kind); got != 3 {
				t.Errorf("failed charge changed usage: %d", got)
			}
			if err := account.Check(tc.kind, 0); !errors.Is(err, ErrInvalidConfiguration) {
				t.Errorf("wrong Check mode = %v", err)
			}
			if tc.mode == "gauge" {
				if err := account.Release(tc.kind, 4); !errors.Is(err, ErrInvalidConfiguration) {
					t.Errorf("underrelease = %v", err)
				}
				if got, _ := account.Usage(tc.kind); got != 3 {
					t.Errorf("underrelease mutation: %d", got)
				}
				if err := account.Release(tc.kind, 3); err != nil {
					t.Fatal(err)
				}
				if got, _ := account.Usage(tc.kind); got != 0 {
					t.Errorf("release = %d", got)
				}
				if err := account.Charge(tc.kind, 3); err != nil {
					t.Error(err)
				}
			} else if err := account.Release(tc.kind, 0); !errors.Is(err, ErrInvalidConfiguration) {
				t.Errorf("cumulative Release = %v", err)
			}
		})
	}
}

func TestBudgetInvalidCallsNeverMutate(t *testing.T) {
	account, _ := NewBudgetAccount(testLimits(10))
	if err := account.Charge(BudgetDepth, 4); err != nil {
		t.Fatal(err)
	}
	for _, tc := range testBudgetKinds {
		for _, operation := range []func(BudgetKind, int64) error{account.Charge, account.Release, account.Check} {
			if err := operation(tc.kind, -1); !errors.Is(err, ErrInvalidConfiguration) {
				t.Errorf("negative %s operation = %v", tc.kind, err)
			}
		}
	}
	for _, kind := range []BudgetKind{"", "unknown", "Nodes"} {
		for _, operation := range []func(BudgetKind, int64) error{account.Charge, account.Release, account.Check} {
			if err := operation(kind, 0); !errors.Is(err, ErrInvalidConfiguration) {
				t.Errorf("unknown %q operation = %v", kind, err)
			}
		}
		if _, err := account.Usage(kind); !errors.Is(err, ErrInvalidConfiguration) {
			t.Errorf("unknown Usage %q = %v", kind, err)
		}
	}
	for _, tc := range testBudgetKinds {
		want := int64(0)
		if tc.kind == BudgetDepth {
			want = 4
		}
		if got, _ := account.Usage(tc.kind); got != want {
			t.Errorf("%s usage changed: %d want %d", tc.kind, got, want)
		}
	}
}

func TestBudgetOverflowFailsWithoutMutation(t *testing.T) {
	account, err := NewBudgetAccount(testLimits(math.MaxInt64))
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []BudgetKind{BudgetInputBytes, BudgetBufferedBytes, BudgetEvents} {
		if err := account.Charge(kind, math.MaxInt64); err != nil {
			t.Fatal(err)
		}
		if err := account.Charge(kind, 1); !errors.Is(err, ErrResourceLimit) {
			t.Errorf("overflow %s = %v", kind, err)
		}
		if got, _ := account.Usage(kind); got != math.MaxInt64 {
			t.Errorf("overflow mutated %s to %d", kind, got)
		}
		if err := account.Charge(kind, 0); err != nil {
			t.Errorf("zero at full quota: %v", err)
		}
	}
}

func TestBudgetUninitializedAccountsFailClosed(t *testing.T) {
	for _, account := range []*Account{nil, new(Account)} {
		for _, operation := range []func(BudgetKind, int64) error{account.Charge, account.Release, account.Check} {
			if err := operation(BudgetNodes, 0); !errors.Is(err, ErrInvalidConfiguration) {
				t.Errorf("uninitialized account = %v", err)
			}
		}
		if _, err := account.Usage(BudgetNodes); !errors.Is(err, ErrInvalidConfiguration) {
			t.Errorf("uninitialized Usage = %v", err)
		}
		if account.Limits() != (Budgets{}) {
			t.Error("uninitialized limits claim defaults")
		}
	}
}

func TestSharedBudgetAccountConcurrentAggregateQuota(t *testing.T) {
	account, _ := NewBudgetAccount(Budgets{MaxNodes: 101})
	var passed atomic.Int64
	var invalid atomic.Int64
	var wg sync.WaitGroup
	for child := 0; child < 20; child++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				err := account.Charge(BudgetNodes, 1)
				if err == nil {
					passed.Add(1)
				} else if !errors.Is(err, ErrResourceLimit) {
					invalid.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	if got, _ := account.Usage(BudgetNodes); got != 101 || passed.Load() != 101 || invalid.Load() != 0 {
		t.Errorf("shared quota: usage=%d successes=%d invalid=%d", got, passed.Load(), invalid.Load())
	}
}

func TestAllocationSizeChecksHostIntWithoutNarrowingLimits(t *testing.T) {
	max := int64(^uint(0) >> 1)
	for _, n := range []int64{0, 1, max} {
		got, err := AllocationSize(n)
		if err != nil || int64(got) != n {
			t.Errorf("AllocationSize(%d) = %d,%v", n, got, err)
		}
	}
	if _, err := AllocationSize(-1); !errors.Is(err, ErrInvalidConfiguration) {
		t.Errorf("negative allocation = %v", err)
	}
	if strconv.IntSize == 32 {
		if _, err := AllocationSize(max + 1); !errors.Is(err, ErrResourceLimit) {
			t.Errorf("host overflow = %v", err)
		}
	}
	account, err := NewBudgetAccount(Budgets{MaxInputBytes: math.MaxInt64})
	if err != nil || account.Limits().MaxInputBytes != math.MaxInt64 {
		t.Errorf("cumulative int64 narrowed: %v", err)
	}
}

func TestDiagnosticPairChargeIsAtomic(t *testing.T) {
	for _, tc := range []struct {
		name   string
		limits Budgets
		first  int64
		failed int64
		kind   BudgetKind
	}{
		{"bytes", Budgets{MaxDiagnostics: 3, MaxDiagnosticBytes: 5}, 3, 3, BudgetDiagnosticBytes},
		{"count", Budgets{MaxDiagnostics: 1, MaxDiagnosticBytes: 10}, 3, 1, BudgetDiagnostics},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account, _ := NewBudgetAccount(tc.limits)
			if err := account.chargeDiagnostic(tc.first); err != nil {
				t.Fatal(err)
			}
			err := account.chargeDiagnostic(tc.failed)
			var resource *ResourceError
			if !errors.As(err, &resource) || resource.Kind != tc.kind {
				t.Fatalf("failed pair = %v", err)
			}
			if count, _ := account.Usage(BudgetDiagnostics); count != 1 {
				t.Errorf("count changed: %d", count)
			}
			if bytes, _ := account.Usage(BudgetDiagnosticBytes); bytes != tc.first {
				t.Errorf("bytes changed: %d", bytes)
			}
			if err := account.chargeDiagnostic(-1); !errors.Is(err, ErrInvalidConfiguration) {
				t.Errorf("negative pair = %v", err)
			}
		})
	}
	var account *Account
	if err := account.chargeDiagnostic(0); !errors.Is(err, ErrInvalidConfiguration) {
		t.Errorf("nil pair = %v", err)
	}
}

func BenchmarkSharedBudgetCharge(b *testing.B) {
	account, _ := NewBudgetAccount(Budgets{MaxRuleSteps: math.MaxInt64})
	b.ReportAllocs()
	for b.Loop() {
		if err := account.Charge(BudgetRuleSteps, 1); err != nil {
			b.Fatal(err)
		}
	}
}
