package validation

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func contextRound3CompatFixture(mode string) (*contextBindings, *factNode) {
	n := localNode(1, 0, strings.Repeat("d", 600))
	n.header.Type = stateKnown(contextNamedType("compat-owner"))
	b := &contextBindings{}
	for _, role := range []contextRole{"role-a", "role-b"} {
		switch mode {
		case "owner":
			if b.Attributes == nil {
				b.Attributes = make(map[contextRole]map[ComponentID]AttributeSelector)
			}
			b.Attributes[role] = map[ComponentID]AttributeSelector{n.header.Type.Value: localAttr(string(role))}
		case "elements":
			if b.Elements == nil {
				b.Elements = make(map[contextRole]map[DeclarationID]struct{})
			}
			b.Elements[role] = map[DeclarationID]struct{}{n.header.Declaration.Value: {}}
		case "roles":
			if b.Roles == nil {
				b.Roles = make(map[contextRole][]SchemaUse)
			}
			b.Roles[role] = []SchemaUse{{Declaration: n.header.Declaration.Value}}
		}
	}
	return b, n
}

func contextRound3CompatRead(t *testing.T, r *contextReader, n *factNode, mode string) bool {
	t.Helper()
	var roles []contextRole
	if mode == "owner" {
		table := r.ownerTable(n.header.Type.Value)
		if table == nil {
			return false
		}
		roles = table.roles
		for _, role := range roles {
			if table.selectors[role] != localAttr(string(role)) {
				t.Errorf("selector lost for %s", role)
			}
		}
	} else {
		roles = r.roles(n)
		if roles == nil {
			return false
		}
	}
	if !slices.Equal(roles, []contextRole{"role-a", "role-b"}) {
		t.Errorf("roles=%v", roles)
	}
	return true
}

func contextRound3CompatRetained(n *factNode, mode string) int64 {
	if mode == "owner" {
		return 512 + 2*256
	}
	return 2*64 + 256 + contextDeclarationBytes(n.header.Declaration.Value)
}

func TestContextRound3CompatCancellationAndSharedRetry(t *testing.T) {
	for _, mode := range []string{"owner", "elements", "roles"} {
		t.Run(mode, func(t *testing.T) {
			b, n := contextRound3CompatFixture(mode)
			probe := &rationalCancelContext{Context: context.Background(), at: 1000000}
			s := rationalSession(t, Budgets{})
			r := &contextReader{ctx: probe, s: s, b: b}
			if !contextRound3CompatRead(t, r, n, mode) || r.err != nil {
				t.Fatal(r.err)
			}
			work := rationalUsage(t, s, BudgetRuleSteps)
			s.cleanup()
			t.Logf("work=%d polls=%d", work, probe.calls)
			for at := 1; at <= probe.calls; at++ {
				s := rationalSession(t, Budgets{})
				r := &contextReader{ctx: &rationalCancelContext{Context: context.Background(), at: at}, s: s, b: b}
				if contextRound3CompatRead(t, r, n, mode) || !errors.Is(r.err, context.Canceled) {
					t.Fatalf("poll %d: %v", at, r.err)
				}
				committed := len(r.ownerCache) != 0 || len(r.elementCache) != 0
				want := int64(0)
				if committed {
					want = contextRound3CompatRetained(n, mode)
				}
				if used := rationalUsage(t, s, BudgetBufferedBytes); s.retained != want || used != want {
					t.Errorf("poll %d: committed=%v retained=%d used=%d want=%d", at, committed, s.retained, used, want)
				}
				// Cancellation is reader-local. A fresh reader on the same account
				// must not inherit scratch from the abandoned construction.
				retry := &contextReader{ctx: context.Background(), s: s, b: b}
				if !contextRound3CompatRead(t, retry, n, mode) || retry.err != nil {
					t.Fatalf("retry poll %d: %v", at, retry.err)
				}
				if used := rationalUsage(t, s, BudgetBufferedBytes); used != want+contextRound3CompatRetained(n, mode) {
					t.Errorf("retry poll %d: used=%d", at, used)
				}
				r.ctx = context.Background()
				if contextRound3CompatRead(t, r, n, mode) || !errors.Is(r.err, context.Canceled) {
					t.Fatalf("poll %d lost first interruption", at)
				}
				s.cleanup()
			}
		})
	}
}

func TestContextRound3CompatBufferAdmissionAndSharedRetry(t *testing.T) {
	for _, mode := range []string{"owner", "elements", "roles"} {
		t.Run(mode, func(t *testing.T) {
			b, n := contextRound3CompatFixture(mode)
			retained := contextRound3CompatRetained(n, mode)
			peak := retained
			if mode == "owner" {
				peak += 2 * 256
			}
			for _, spare := range []int64{0, 1} {
				s := rationalSession(t, Budgets{MaxBufferedBytes: peak})
				if err := s.account.Charge(BudgetBufferedBytes, spare); err != nil {
					t.Fatal(err)
				}
				r := &contextReader{ctx: context.Background(), s: s, b: b}
				ok := contextRound3CompatRead(t, r, n, mode)
				if spare == 0 {
					if !ok || r.err != nil || s.retained != retained || rationalUsage(t, s, BudgetBufferedBytes) != retained {
						t.Errorf("exact admission: ok=%v err=%v retained=%d", ok, r.err, s.retained)
					}
					if !contextRound3CompatRead(t, r, n, mode) || r.err != nil || s.retained != retained || rationalUsage(t, s, BudgetBufferedBytes) != retained {
						t.Errorf("cached lookup charged again or lost its live result: %v", r.err)
					}
				} else {
					if ok || !errors.Is(r.err, ErrResourceLimit) || len(r.ownerCache) != 0 || len(r.elementCache) != 0 || s.retained != 0 || rationalUsage(t, s, BudgetBufferedBytes) != spare {
						t.Errorf("one-under leaked: ok=%v err=%v retained=%d used=%d", ok, r.err, s.retained, rationalUsage(t, s, BudgetBufferedBytes))
					}
					for attempt := 0; attempt < 2; attempt++ {
						retry := &contextReader{ctx: context.Background(), s: s, b: b}
						if contextRound3CompatRead(t, retry, n, mode) || !errors.Is(retry.err, ErrResourceLimit) || s.retained != 0 || rationalUsage(t, s, BudgetBufferedBytes) != spare {
							t.Errorf("refused retry %d leaked: %v", attempt, retry.err)
						}
					}
					if err := s.account.Release(BudgetBufferedBytes, spare); err != nil {
						t.Fatal(err)
					}
					retry := &contextReader{ctx: context.Background(), s: s, b: b}
					if !contextRound3CompatRead(t, retry, n, mode) || retry.err != nil {
						t.Errorf("shared-account retry: %v", retry.err)
					}
				}
				s.cleanup()
				if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
					t.Error("cleanup left owned bytes")
				}
			}
		})
	}
}

func TestContextRound3CompatWorkAdmission(t *testing.T) {
	for _, mode := range []string{"owner", "elements", "roles"} {
		t.Run(mode, func(t *testing.T) {
			b, n := contextRound3CompatFixture(mode)
			probe := rationalSession(t, Budgets{})
			r := &contextReader{ctx: context.Background(), s: probe, b: b}
			if !contextRound3CompatRead(t, r, n, mode) || r.err != nil {
				t.Fatal(r.err)
			}
			work := rationalUsage(t, probe, BudgetRuleSteps)
			probe.cleanup()
			for _, limit := range []int64{work - 1, work} {
				s := rationalSession(t, Budgets{MaxRuleSteps: limit})
				r := &contextReader{ctx: context.Background(), s: s, b: b}
				ok := contextRound3CompatRead(t, r, n, mode)
				if limit == work {
					if !ok || r.err != nil {
						t.Errorf("exact work refused: %v", r.err)
					}
				} else if ok || !errors.Is(r.err, ErrResourceLimit) {
					t.Errorf("one-under work exposed an incomplete result: ok=%v err=%v", ok, r.err)
				}
				want := int64(0)
				if len(r.ownerCache) != 0 || len(r.elementCache) != 0 {
					want = contextRound3CompatRetained(n, mode)
				}
				if used := rationalUsage(t, s, BudgetBufferedBytes); used != want || s.retained != want {
					t.Errorf("work %d retained=%d used=%d want=%d", limit, s.retained, used, want)
				}
				s.cleanup()
			}
		})
	}
}
