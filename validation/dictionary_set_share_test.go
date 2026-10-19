package validation

import (
	"context"
	"errors"
	"testing"
)

func shareTestAccount(t *testing.T, limits Budgets) *Account {
	t.Helper()
	a, err := NewBudgetAccount(limits)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func shareTestBuffered(t *testing.T, a *Account) int64 {
	t.Helper()
	used, err := a.Usage(BudgetBufferedBytes)
	if err != nil {
		t.Fatal(err)
	}
	return used
}

// A shared view reads its parent's entries and fingerprint on the caller's
// account, charges the parent's retained storage until it is freed, keeps the
// parent's entries alive after the parent closes, and admits only the
// caller's account to borrow it.
func TestShareDictionarySetLifecycle(t *testing.T) {
	ctx := context.Background()
	owner, caller, other := shareTestAccount(t, Budgets{}), shareTestAccount(t, Budgets{}), shareTestAccount(t, Budgets{})
	selections := []DictionarySelection{{Manifest: DictionaryManifestSMuFL14Canonical, Relations: []DictionaryRelation{DictionaryRelationCanonicalName}}}
	parent, err := NewDictionarySet(ctx, owner, selections, nil)
	if err != nil {
		t.Fatal(err)
	}
	ownerBytes := shareTestBuffered(t, owner)
	view, err := shareDictionarySet(ctx, parent, caller)
	if err != nil {
		t.Fatal(err)
	}
	if got := shareTestBuffered(t, caller); got != ownerBytes || shareTestBuffered(t, owner) != ownerBytes {
		t.Fatalf("view charged %d, parent retains %d; want both %d", got, shareTestBuffered(t, owner), ownerBytes)
	}
	want, _ := parent.Fingerprint()
	if got, err := view.Fingerprint(); err != nil || got != want {
		t.Fatalf("view fingerprint %x, want %x: %v", got, want, err)
	}
	if _, err := view.borrow(other); err == nil {
		t.Fatal("a foreign account borrowed the view")
	}
	lease, err := view.borrow(caller)
	if err != nil {
		t.Fatal(err)
	}
	if err := parent.Close(); err != nil {
		t.Fatal(err)
	}
	if shareTestBuffered(t, owner) != ownerBytes || len(parent.entries) == 0 {
		t.Fatal("parent freed while a view lives")
	}
	if _, err := shareDictionarySet(ctx, parent, other); err == nil {
		t.Fatal("closed parent shared")
	}
	if err := view.Close(); err != nil {
		t.Fatal(err)
	}
	if len(view.entries) == 0 || shareTestBuffered(t, caller) == 0 {
		t.Fatal("borrowed view freed on Close")
	}
	lease.release()
	if shareTestBuffered(t, caller) != 0 || shareTestBuffered(t, owner) != 0 || parent.entries != nil || parent.borrowers != 0 {
		t.Fatalf("release left caller=%d owner=%d parent borrowers=%d", shareTestBuffered(t, caller), shareTestBuffered(t, owner), parent.borrowers)
	}
}

// A view the caller's budget cannot hold is refused before it exists and
// returns its borrow of the parent.
func TestShareDictionarySetRefusedByTheCallerBudget(t *testing.T) {
	ctx := context.Background()
	owner := shareTestAccount(t, Budgets{})
	parent, err := NewDictionarySet(ctx, owner, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	for name, limits := range map[string]Budgets{
		"buffer": {MaxBufferedBytes: parent.retained - 1},
		"steps":  {MaxRuleSteps: 1},
	} {
		caller := shareTestAccount(t, limits)
		_ = caller.Charge(BudgetRuleSteps, 1)
		view, err := shareDictionarySet(ctx, parent, caller)
		if view != nil || !errors.Is(err, ErrResourceLimit) || shareTestBuffered(t, caller) != 0 || parent.borrowers != 0 {
			t.Fatalf("%s: view=%v error=%v caller=%d parent borrowers=%d", name, view != nil, err, shareTestBuffered(t, caller), parent.borrowers)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if view, err := shareDictionarySet(canceled, parent, shareTestAccount(t, Budgets{})); view != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled share: view=%v error=%v", view != nil, err)
	}
	if view, err := shareDictionarySet(ctx, nil, shareTestAccount(t, Budgets{})); view != nil || !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("nil parent: view=%v error=%v", view != nil, err)
	}
}
