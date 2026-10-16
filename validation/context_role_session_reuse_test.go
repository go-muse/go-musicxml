package validation

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestContextRoleSessionReuseAcrossReaders(t *testing.T) {
	for _, empty := range []bool{false, true} {
		s, b, n := contextRoleReuseFixture(t)
		if empty {
			n = s.root
		}
		before := s.retained
		first := &contextReader{ctx: context.Background(), s: s, b: b}
		if got := first.matches(n, "item-role"); got != !empty || first.err != nil {
			t.Fatalf("first exact lookup: result=%v err=%v", got, first.err)
		}
		admitted := s.retained - before
		beforeWork := rationalUsage(t, s, BudgetRuleSteps)
		_ = first.matches(n, "item-role")
		warmWork := rationalUsage(t, s, BudgetRuleSteps) - beforeWork
		beforeWork = rationalUsage(t, s, BudgetRuleSteps)
		for i := 0; i < 20; i++ {
			r := &contextReader{ctx: context.Background(), s: s, b: b}
			if got := r.matches(n, "item-role"); got != !empty || r.err != nil {
				t.Fatalf("reader %d: result=%v err=%v", i, got, r.err)
			}
		}
		if got := s.retained - before; got != admitted {
			t.Errorf("empty=%v reader-local maps accumulated: retained=%d one-entry=%d", empty, got, admitted)
		}
		if got := rationalUsage(t, s, BudgetRuleSteps) - beforeWork; got != 20*warmWork {
			t.Errorf("empty=%v immutable declaration reclassified across readers: work=%d want=%d", empty, got, 20*warmWork)
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("session cleanup leaked shared role storage")
		}
	}
}

func TestContextRoleSessionReuseExactBuffer(t *testing.T) {
	for _, mode := range []string{"first", "same", "other-node", "other-table"} {
		cost := int64(256)
		if mode == "first" {
			cost = 512
		} else if mode == "same" {
			cost = 0
		}
		limits := []int64{cost}
		if cost > 0 {
			limits = append(limits, cost-1)
		}
		for _, limit := range limits {
			s, b, n := contextRoleReuseFixture(t)
			if mode != "first" {
				first := &contextReader{ctx: context.Background(), s: s, b: b}
				if !first.matches(n, "item-role") || first.err != nil {
					t.Fatal(first.err)
				}
			}
			want := true
			if mode == "other-node" {
				n, want = s.root, false
			} else if mode == "other-table" {
				// Equal digests and shared immutable maps do not authorize merging
				// distinct compiled binding-table identities.
				copy := *b
				b = &copy
			}
			before := s.retained
			padding := s.account.Limits().MaxBufferedBytes - rationalUsage(t, s, BudgetBufferedBytes) - limit
			if err := s.account.Charge(BudgetBufferedBytes, padding); err != nil {
				t.Fatal(err)
			}
			r := &contextReader{ctx: context.Background(), s: s, b: b}
			got := r.matches(n, "item-role")
			if limit == cost {
				if got != want || r.err != nil || s.retained-before != cost {
					t.Errorf("%s exact%d: result=%v retained=%d err=%v", mode, cost, got, s.retained-before, r.err)
				}
			} else if !errors.Is(r.err, ErrResourceLimit) || s.retained != before {
				t.Errorf("%s one-below%d: result=%v retained=%d err=%v", mode, limit, got, s.retained-before, r.err)
			}
			if err := s.account.Release(BudgetBufferedBytes, padding); err != nil {
				t.Fatal(err)
			}
			retry := &contextReader{ctx: context.Background(), s: s, b: b}
			if got := retry.matches(n, "item-role"); got != want || retry.err != nil {
				t.Fatalf("%s shared-account retry: result=%v err=%v", mode, got, retry.err)
			}
			if s.retained-before != cost {
				t.Errorf("%s retry allocated duplicate immutable entry: retained=%d want=%d", mode, s.retained-before, cost)
			}
			s.cleanup()
			if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatal("buffer admission/retry left live storage")
			}
		}
	}
}

func TestContextRoleSessionReuseExactWorkAndCancellation(t *testing.T) {
	probe, b, n := contextRoleReuseFixture(t)
	first := &contextReader{ctx: context.Background(), s: probe, b: b}
	if !first.matches(n, "item-role") {
		t.Fatal(first.err)
	}
	before := rationalUsage(t, probe, BudgetRuleSteps)
	polls := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	first.ctx = polls
	if !first.matches(n, "item-role") {
		t.Fatal(first.err)
	}
	cost := rationalUsage(t, probe, BudgetRuleSteps) - before
	probe.cleanup()
	for _, limit := range []int64{cost, cost - 1} {
		s, b, n := contextRoleReuseFixture(t)
		first := &contextReader{ctx: context.Background(), s: s, b: b}
		if !first.matches(n, "item-role") {
			t.Fatal(first.err)
		}
		retained := s.retained
		padding := s.account.Limits().MaxRuleSteps - rationalUsage(t, s, BudgetRuleSteps) - limit
		if err := s.account.Charge(BudgetRuleSteps, padding); err != nil {
			t.Fatal(err)
		}
		next := &contextReader{ctx: context.Background(), s: s, b: b}
		got := next.matches(n, "item-role")
		if limit == cost {
			if !got || next.err != nil {
				t.Errorf("cross-reader exact work%d: %v", cost, next.err)
			}
		} else if got || !errors.Is(next.err, ErrResourceLimit) {
			t.Errorf("cross-reader one-short%d: result=%v err=%v", limit, got, next.err)
		}
		if s.retained != retained {
			t.Fatal("shared lookup changed retained cache storage")
		}
		s.cleanup()
	}
	for at := 1; at <= polls.calls; at++ {
		s, b, n := contextRoleReuseFixture(t)
		first := &contextReader{ctx: context.Background(), s: s, b: b}
		if !first.matches(n, "item-role") {
			t.Fatal(first.err)
		}
		retained := s.retained
		cancelled := &contextReader{ctx: &rationalCancelContext{Context: context.Background(), at: at}, s: s, b: b}
		if cancelled.matches(n, "item-role") || !errors.Is(cancelled.err, context.Canceled) {
			t.Errorf("cross-reader cancellation poll%d: %v", at, cancelled.err)
		}
		retry := &contextReader{ctx: context.Background(), s: s, b: b}
		if !retry.matches(n, "item-role") || retry.err != nil || s.retained != retained {
			t.Errorf("canceled reader poisoned shared reuse at poll%d: %v retained_delta=%d", at, retry.err, s.retained-retained)
		}
		cancelled.ctx = context.Background()
		if cancelled.matches(n, "item-role") || !errors.Is(cancelled.err, context.Canceled) {
			t.Fatal("reader-local cancellation was cleared by another provider")
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("cancellation left live shared storage")
		}
	}
}

func TestContextRoleSessionReuseOwnershipAndEnvelope(t *testing.T) {
	field, ok := reflect.TypeOf(Session{}).FieldByName("contextElementRoles")
	if !ok || field.Type.Kind() != reflect.Map {
		t.Fatal("the session must own the admitted immutable role cache")
	}
	// Eight key/value slots plus a conservative 128-byte group/header allowance
	// must fit the first 512-byte admission on every supported architecture.
	pairBytes := field.Type.Key().Size() + field.Type.Elem().Size()
	if 8*pairBytes+128 > 512 || pairBytes > 128 {
		t.Fatalf("role-cache layout exceeds admitted first-group/entry envelope: pair=%d", pairBytes)
	}
	for i := 0; i < 2; i++ {
		s, b, n := contextRoleReuseFixture(t)
		before := s.retained
		r := &contextReader{ctx: context.Background(), s: s, b: b}
		if !r.matches(n, "item-role") || r.err != nil || s.retained-before != 512 {
			t.Fatalf("independent session%d borrowed unowned cache state: %v delta=%d", i, r.err, s.retained-before)
		}
		if reflect.ValueOf(s).Elem().FieldByIndex(field.Index).Len() != 1 {
			t.Fatal("one exact binding/table pair did not produce one entry")
		}
		s.cleanup()
		if !reflect.ValueOf(s).Elem().FieldByIndex(field.Index).IsNil() || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("cleanup released the gauge without clearing borrowed binding references")
		}
	}
}

func TestContextRoleSessionReuseMissingBindings(t *testing.T) {
	for _, mode := range []string{"missing", "unavailable", "zero"} {
		s, b, n := contextRoleReuseFixture(t)
		before := s.retained
		for _, node := range []*factNode{n, s.root} {
			bound := s.schema.nodes[node.header.ID]
			switch mode {
			case "missing":
				delete(s.schema.nodes, node.header.ID)
			case "unavailable":
				bound.available = false
				s.schema.nodes[node.header.ID] = bound
			case "zero":
				s.schema.nodes[node.header.ID] = schemaNode{available: true}
			}
		}
		for _, node := range []*factNode{n, s.root} {
			r := &contextReader{ctx: context.Background(), s: s, b: b}
			if got := r.matches(node, "item-role"); got != (node == n) || r.err != nil {
				t.Errorf("%s binding changed exact fallback: node=%d result=%v err=%v", mode, node.header.ID, got, r.err)
			}
		}
		if s.retained != before {
			t.Errorf("%s binding admitted an unproved compact cache identity: delta=%d", mode, s.retained-before)
		}
		s.cleanup()
	}
}

func TestContextRoleSessionReuseColdCancellationAndFinish(t *testing.T) {
	probe, b, n := contextRoleReuseFixture(t)
	polls := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	r := &contextReader{ctx: polls, s: probe, b: b}
	if !r.matches(n, "item-role") {
		t.Fatal(r.err)
	}
	probe.cleanup()
	for at := 1; at <= polls.calls; at++ {
		s, b, n := contextRoleReuseFixture(t)
		before := s.retained
		r := &contextReader{ctx: &rationalCancelContext{Context: context.Background(), at: at}, s: s, b: b}
		if r.matches(n, "item-role") || !errors.Is(r.err, context.Canceled) {
			t.Errorf("cold cancellation poll%d: %v", at, r.err)
		}
		// A completed classification may already be shared when cancellation
		// occurs in the later membership comparison. Otherwise retry builds it.
		retry := &contextReader{ctx: context.Background(), s: s, b: b}
		if !retry.matches(n, "item-role") || retry.err != nil || s.retained-before != 512 {
			t.Errorf("cold retry poll%d: err=%v retained_delta=%d", at, retry.err, s.retained-before)
		}
		// The fixture stopped inside the final phase. Exercise Finish's actual
		// terminal cleanup without trying to finalize its already-finished run.
		s.finished = false
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
		if !errors.Is(err, context.Canceled) || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Errorf("Finish cleanup poll%d: %v", at, err)
		}
		field := reflect.ValueOf(s).Elem().FieldByName("contextElementRoles")
		if !field.IsValid() || !field.IsNil() {
			t.Fatal("Finish retained the shared cache after releasing its account")
		}
	}
}
