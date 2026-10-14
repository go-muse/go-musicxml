package validation

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func dictionaryCoreAccount(t *testing.T) *Account {
	t.Helper()
	a, err := NewBudgetAccount(Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Charge(BudgetBufferedBytes, 17); err != nil {
		t.Fatal(err)
	}
	return a
}

func dictionaryCoreUsage(t *testing.T, a *Account) int64 {
	t.Helper()
	n, err := a.Usage(BudgetBufferedBytes)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestDictionaryConstructorRejectsInvalidContextAndAccount(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		name    string
		ctx     context.Context
		account *Account
		want    error
	}{
		{"nil-context", nil, dictionaryCoreAccount(t), ErrInvalidConfiguration},
		{"nil-account", context.Background(), nil, ErrInvalidConfiguration},
		{"zero-account", context.Background(), &Account{}, ErrInvalidConfiguration},
		{"canceled", ctx, dictionaryCoreAccount(t), context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			set, err := NewDictionarySet(tc.ctx, tc.account, nil, nil)
			if set != nil || !errors.Is(err, tc.want) {
				t.Fatalf("constructor = %v, %v; want %v", set, err, tc.want)
			}
			if tc.account != nil && tc.account.initialized && dictionaryCoreUsage(t, tc.account) != 17 {
				t.Fatal("constructor changed caller's retained storage")
			}
		})
	}
}

func TestDictionaryCloseDefersStoreReleaseUntilAllBorrowersFinish(t *testing.T) {
	ctx := context.Background()
	a := dictionaryCoreAccount(t)
	set, err := NewDictionarySet(ctx, a, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	store := dictionaryCoreUsage(t, a) - 17
	if store <= 0 {
		t.Fatal("dictionary store was not admitted")
	}
	e, unused := sessionFixture(t, Budgets{})
	_, _ = unused.Finish(ctx, Completion{Syntax: SyntaxIncomplete})
	first, err := e.BeginWithDictionaries(ctx, RunOptions{DocumentID: "first", Profile: "test.source", Account: a}, set)
	if err != nil {
		t.Fatal(err)
	}
	second, err := e.BeginWithDictionaries(ctx, RunOptions{DocumentID: "second", Profile: "test.source", Account: a}, set)
	if err != nil {
		t.Fatal(err)
	}
	beforeClose := dictionaryCoreUsage(t, a)
	if err := set.Close(); err != nil {
		t.Fatal(err)
	}
	if err := set.Close(); err != nil {
		t.Fatal(err)
	}
	if got := dictionaryCoreUsage(t, a); got != beforeClose {
		t.Fatalf("close released borrowed store: %d != %d", got, beforeClose)
	}
	if session, err := e.BeginWithDictionaries(ctx, RunOptions{DocumentID: "late", Profile: "test.source", Account: a}, set); session != nil || !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("closed owner admitted a borrower: %v %v", session, err)
	}
	if _, err := first.Finish(ctx, Completion{Syntax: SyntaxIncomplete}); err != nil {
		t.Fatal(err)
	}
	if got := dictionaryCoreUsage(t, a); got <= 17+store {
		t.Fatalf("first finish released another borrower: %d", got)
	}
	if _, err := second.Finish(ctx, Completion{Syntax: SyntaxIncomplete}); err != nil {
		t.Fatal(err)
	}
	if got := dictionaryCoreUsage(t, a); got != 17 {
		t.Fatalf("final borrower retained storage: %d", got)
	}
	if _, err := second.Finish(ctx, Completion{Syntax: SyntaxIncomplete}); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("repeat finish: %v", err)
	}
	if err := set.Close(); err != nil {
		t.Fatal(err)
	}
	if got := dictionaryCoreUsage(t, a); got != 17 {
		t.Fatalf("repeat cleanup touched caller storage: %d", got)
	}
}

func TestDictionaryBeginRequiresSameAccountAndReleasesFailedBorrow(t *testing.T) {
	ctx := context.Background()
	a := dictionaryCoreAccount(t)
	set, err := NewDictionarySet(ctx, a, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	e, unused := sessionFixture(t, Budgets{})
	_, _ = unused.Finish(ctx, Completion{Syntax: SyntaxIncomplete})
	other := dictionaryCoreAccount(t)
	before := dictionaryCoreUsage(t, a)
	for _, opts := range []RunOptions{
		{DocumentID: "nil-account", Profile: "test.source"},
		{DocumentID: "other-account", Profile: "test.source", Account: other},
		{DocumentID: "override", Profile: "test.source", Account: a, Budgets: Budgets{MaxNodes: 1}},
		{DocumentID: "bad-profile", Profile: "unknown", Account: a},
	} {
		if session, err := e.BeginWithDictionaries(ctx, opts, set); session != nil || !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("begin %+v = %v %v", opts, session, err)
		}
		if got := dictionaryCoreUsage(t, a); got != before {
			t.Fatalf("failed begin leaked: %d != %d", got, before)
		}
	}
	if err := set.Close(); err != nil {
		t.Fatal(err)
	}
	if got := dictionaryCoreUsage(t, a); got != 17 {
		t.Fatalf("failed borrow prevented release: %d", got)
	}
	if got := dictionaryCoreUsage(t, other); got != 17 {
		t.Fatalf("foreign account changed: %d", got)
	}
	if session, err := e.BeginWithDictionaries(ctx, RunOptions{DocumentID: "zero", Profile: "test.source", Account: a}, &DictionarySet{}); session != nil || !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("zero set accepted: %v %v", session, err)
	}
}

func TestDictionaryNilCapabilityPreservesOldBegin(t *testing.T) {
	ctx := context.Background()
	e, unused := sessionFixture(t, Budgets{})
	_, _ = unused.Finish(ctx, Completion{Syntax: SyntaxIncomplete})
	s, err := e.BeginWithDictionaries(ctx, RunOptions{DocumentID: "none", Profile: "test.source"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Finish(ctx, Completion{Syntax: SyntaxIncomplete}); err != nil {
		t.Fatal(err)
	}
}

func TestDictionaryConcurrentBeginAndCloseHaveOneLifetime(t *testing.T) {
	ctx := context.Background()
	a := dictionaryCoreAccount(t)
	set, err := NewDictionarySet(ctx, a, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	e, unused := sessionFixture(t, Budgets{})
	_, _ = unused.Finish(ctx, Completion{Syntax: SyntaxIncomplete})
	var group sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 32; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			s, err := e.BeginWithDictionaries(ctx, RunOptions{DocumentID: "concurrent", Profile: "test.source", Account: a}, set)
			if err != nil {
				if !errors.Is(err, ErrInvalidConfiguration) {
					t.Errorf("unexpected begin error: %v", err)
				}
				return
			}
			if _, err := s.Finish(ctx, Completion{Syntax: SyntaxIncomplete}); err != nil {
				t.Errorf("borrower failed: %v", err)
			}
		}()
	}
	group.Add(1)
	go func() {
		defer group.Done()
		<-start
		if err := set.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()
	close(start)
	group.Wait()
	if err := set.Close(); err != nil {
		t.Fatal(err)
	}
	if got := dictionaryCoreUsage(t, a); got != 17 {
		t.Fatalf("concurrent lifecycle leaked or overreleased: %d", got)
	}
}

func TestDictionaryEmptyStoreExactRetentionLimitAndFailureCleanup(t *testing.T) {
	ctx := context.Background()
	probe := dictionaryCoreAccount(t)
	set, err := NewDictionarySet(ctx, probe, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	needed := dictionaryCoreUsage(t, probe)
	if err := set.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		limit    int64
		succeeds bool
	}{
		{"exact", needed, true}, {"one-under", needed - 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := NewBudgetAccount(Budgets{MaxBufferedBytes: tc.limit})
			if err != nil {
				t.Fatal(err)
			}
			if err := a.Charge(BudgetBufferedBytes, 17); err != nil {
				t.Fatal(err)
			}
			got, err := NewDictionarySet(ctx, a, nil, nil)
			if tc.succeeds {
				if err != nil || got == nil {
					t.Fatalf("exact admission: %v %v", got, err)
				}
				if err := got.Close(); err != nil {
					t.Fatal(err)
				}
			} else if got != nil || !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("under-limit admission: %v %v", got, err)
			}
			if used := dictionaryCoreUsage(t, a); used != 17 {
				t.Fatalf("constructor/close touched caller retention: %d", used)
			}
		})
	}
}

func TestDictionaryFailedSessionInitializationAndCancellationReleaseBorrow(t *testing.T) {
	ctx := context.Background()
	a := dictionaryCoreAccount(t)
	set, err := NewDictionarySet(ctx, a, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	e, unused := sessionFixture(t, Budgets{})
	_, _ = unused.Finish(ctx, Completion{Syntax: SyntaxIncomplete})
	before := dictionaryCoreUsage(t, a)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if s, err := e.BeginWithDictionaries(canceled, RunOptions{DocumentID: "canceled", Profile: "test.source", Account: a}, set); s != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled begin: %v %v", s, err)
	}
	used, err := a.Usage(BudgetRuleSteps)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Charge(BudgetRuleSteps, a.Limits().MaxRuleSteps-used); err != nil {
		t.Fatal(err)
	}
	if s, err := e.BeginWithDictionaries(ctx, RunOptions{DocumentID: "exhausted", Profile: "test.source", Account: a}, set); s != nil || !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("exhausted begin: %v %v", s, err)
	}
	if got := dictionaryCoreUsage(t, a); got != before {
		t.Fatalf("failed session initialization leaked: %d != %d", got, before)
	}
	if err := set.Close(); err != nil {
		t.Fatal(err)
	}
	if got := dictionaryCoreUsage(t, a); got != 17 {
		t.Fatalf("failed session borrowed forever: %d", got)
	}
	if got, _ := a.Usage(BudgetRuleSteps); got != a.Limits().MaxRuleSteps {
		t.Fatalf("cleanup refunded work: %d", got)
	}
}

func TestDictionaryBeginPreservesContextPrecedenceWhenBorrowBudgetExhausted(t *testing.T) {
	background := context.Background()
	canceled, cancel := context.WithCancel(background)
	cancel()
	for _, tc := range []struct {
		name  string
		ctx   context.Context
		cause error
	}{
		{"nil-context", nil, ErrInvalidConfiguration},
		{"canceled-context", canceled, context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := dictionaryCoreAccount(t)
			set, err := NewDictionarySet(background, a, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			e, unused := sessionFixture(t, Budgets{})
			_, _ = unused.Finish(background, Completion{Syntax: SyntaxIncomplete})
			live := dictionaryCoreUsage(t, a)
			exhausted := a.Limits().MaxBufferedBytes - live
			if err := a.Charge(BudgetBufferedBytes, exhausted); err != nil {
				t.Fatal(err)
			}
			opts := RunOptions{DocumentID: "preflight", Profile: "test.source", Account: a}
			ordinary, baseline := e.Begin(tc.ctx, opts)
			if ordinary != nil || !errors.Is(baseline, tc.cause) {
				t.Fatalf("ordinary Begin precedence changed: %v %v", ordinary, baseline)
			}
			borrowed, got := e.BeginWithDictionaries(tc.ctx, opts, set)
			if borrowed != nil || !errors.Is(got, tc.cause) || errors.Is(got, ErrResourceLimit) || got.Error() != baseline.Error() {
				t.Errorf("dictionary Begin context precedence differs: got %v, ordinary %v", got, baseline)
			}
			if got := dictionaryCoreUsage(t, a); got != a.Limits().MaxBufferedBytes {
				t.Error("failed preflight changed caller account")
			}
			if err := a.Release(BudgetBufferedBytes, exhausted); err != nil {
				t.Fatal(err)
			}
			if err := set.Close(); err != nil {
				t.Fatal(err)
			}
			if dictionaryCoreUsage(t, a) != 17 {
				t.Fatal("failed preflight leaked a lease")
			}
		})
	}
}

func TestDictionaryEmptySetAdmitsFixedConstructorScratchEnvelope(t *testing.T) {
	for _, limit := range []int64{2047, 2048} {
		a, err := NewBudgetAccount(Budgets{MaxBufferedBytes: limit})
		if err != nil {
			t.Fatal(err)
		}
		set, err := NewDictionarySet(context.Background(), a, nil, nil)
		if set != nil {
			defer set.Close()
		}
		if limit < 2048 {
			if set != nil || !errors.Is(err, ErrResourceLimit) {
				t.Errorf("undersized empty-store envelope accepted: %v %v", set, err)
			}
		} else if set == nil || err != nil {
			t.Errorf("exact empty-store envelope refused: %v %v", set, err)
		}
	}
}
