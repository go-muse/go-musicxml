package validation

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"unsafe"
)

func TestLocalDistinctMaterializesEachEffectiveValueOnce(t *testing.T) {
	schema := localSchema(t)
	beam := localUse(t, schema, "note", "beam", false).Declaration
	number := localSchemaAttr(t, schema, "beam", "number")
	plan, err := NewDistinctChildAttributePlan([]DeclarationID{beam}, number)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []Target{TargetSource, TargetModel} {
		for _, count := range []int{8, 32, 128} {
			t.Run(fmt.Sprintf("%s/children=%d", target, count), func(t *testing.T) {
				note := lt("note")
				for i := 0; i < count; i++ {
					child := lt("beam")
					// The first known-absent attribute exercises effective default 1.
					if i != 0 {
						child.attrs = map[string]string{"number": fmt.Sprint(100 + i)}
					}
					note.children = append(note.children, child)
				}
				s := localAdmit(t, localEngine(t, schema, target), target, lt("root", note), nil)
				defer s.cleanup()
				n := s.root.children[0]
				ctx := context.Background()
				before, _ := s.account.Usage(BudgetBufferedBytes)
				// Establish the resource cost of exactly one authoritative lookup
				// per selected child; this stays valid if numeric accounting evolves.
				for _, child := range n.children {
					value, err := s.localScalar(ctx, child, &number)
					if err != nil || value.State != EvidenceKnown {
						t.Fatalf("effective value unavailable: %+v %v", value, err)
					}
				}
				after, _ := s.account.Usage(BudgetBufferedBytes)
				once := after - before
				if once <= 0 {
					t.Fatal("fixture does not exercise exact numeric materialization")
				}
				for run := 0; run < 3; run++ {
					before, _ = s.account.Usage(BudgetBufferedBytes)
					d, pending, err := s.evaluateLocalPlan(ctx, n, plan.local, true)
					if err != nil || pending || d.Outcome != OutcomePass {
						t.Fatalf("distinct evaluation: %+v pending=%v err=%v", d, pending, err)
					}
					after, _ = s.account.Usage(BudgetBufferedBytes)
					if got := after - before; got != once {
						t.Errorf("run %d materialized earlier values again or leaked scratch: retained=%d once=%d", run, got, once)
					}
				}
				s.cleanup()
				if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
					t.Errorf("session cleanup leaked %d bytes", used)
				}
			})
		}
	}
}

func TestLocalDistinctSelectionWorkScalesWithChildren(t *testing.T) {
	const count = 64
	declarations := make([]DeclarationID, 4096)
	for i := range declarations {
		declarations[i] = localDecl(fmt.Sprintf("beam.%04d", i))
	}
	plan, err := NewDistinctChildAttributePlan(declarations, localAttr("number"))
	if err != nil {
		t.Fatal(err)
	}
	n := localNode(1, 0, "note")
	for i := 0; i < count; i++ {
		child := localNode(NodeID(i+2), 1, "beam.4095")
		localSetAttr(child, "number", localInteger(t, int64(i)))
		n.children = append(n.children, child)
	}
	// Selection and lookup are linear; the exact equality comparisons remain
	// pairwise and are included separately rather than claiming linear total work.
	limit := int64(count*len(declarations) + count*(count-1)/2 + 10*count)
	account, err := NewBudgetAccount(Budgets{MaxRuleSteps: limit})
	if err != nil {
		t.Fatal(err)
	}
	s := &Session{account: account}
	d, pending, err := s.evaluateLocalPlan(context.Background(), n, plan.local, true)
	if err != nil || pending || d.Outcome != OutcomePass {
		t.Fatalf("selection re-scanned prior children: %+v pending=%v err=%v", d, pending, err)
	}
	if used, _ := account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("temporary materialization leaked %d bytes", used)
	}
}

func distinctScratchFixture(t *testing.T, count int, budgets Budgets) (*Session, *factNode, Plan) {
	t.Helper()
	plan, err := NewDistinctChildAttributePlan([]DeclarationID{localDecl("beam")}, localAttr("number"))
	if err != nil {
		t.Fatal(err)
	}
	account, err := NewBudgetAccount(budgets)
	if err != nil {
		t.Fatal(err)
	}
	n := localNode(1, 0, "note")
	for i := 0; i < count; i++ {
		child := localNode(NodeID(i+2), 1, "beam")
		child.header.Location.Path = fmt.Sprintf("/note/beam[%d]", i+1)
		localSetAttr(child, "number", localInteger(t, int64(i)))
		n.children = append(n.children, child)
	}
	return &Session{account: account}, n, plan
}

func TestLocalDistinctScratchExactLimitAndCleanup(t *testing.T) {
	const siblingBytes int64 = 73
	// Independently measure the actual backing-array entry, including its
	// node pointer and padding. Accounting must track host layout, not an
	// assumed logical Atomic slot size.
	type exactEntry struct {
		node  *factNode
		value Atomic
	}
	entryBytes := int64(unsafe.Sizeof(exactEntry{}))
	t.Logf("actual scratch entry bytes: %d", entryBytes)
	for _, tc := range []struct {
		count int
		limit int64
		fail  bool
	}{{3, 2*entryBytes - 1, true}, {3, 2 * entryBytes, false}, {4, 6*entryBytes - 1, true}, {4, 6 * entryBytes, false}} {
		t.Run(fmt.Sprintf("children=%d/bytes=%d", tc.count, tc.limit), func(t *testing.T) {
			s, n, plan := distinctScratchFixture(t, tc.count, Budgets{MaxBufferedBytes: tc.limit + siblingBytes})
			if err := s.account.Charge(BudgetBufferedBytes, siblingBytes); err != nil {
				t.Fatal(err)
			}
			for run := 0; run < 3; run++ {
				d, _, err := s.evaluateLocalPlan(context.Background(), n, plan.local, true)
				if errors.Is(err, ErrResourceLimit) != tc.fail || (!tc.fail && (err != nil || d.Outcome != OutcomePass)) {
					t.Fatalf("scratch boundary: %+v err=%v wantResource=%v", d, err, tc.fail)
				}
				if used, _ := s.account.Usage(BudgetBufferedBytes); used != siblingBytes {
					t.Fatalf("scratch cleanup released another owner's bytes or leaked: %d", used)
				}
			}
			if err := s.account.Release(BudgetBufferedBytes, siblingBytes); err != nil {
				t.Fatal(err)
			}
		})
	}
}

type cancelOnDistinctScratch struct {
	context.Context
	account *Account
	seen    bool
}

func (c *cancelOnDistinctScratch) Err() error {
	if bytes, _ := c.account.Usage(BudgetBufferedBytes); bytes != 0 {
		c.seen = true
		return context.Canceled
	}
	return nil
}

func TestLocalDistinctCancellationReleasesScratch(t *testing.T) {
	s, n, plan := distinctScratchFixture(t, 8, Budgets{})
	ctx := &cancelOnDistinctScratch{Context: context.Background(), account: s.account}
	d, _, err := s.evaluateLocalPlan(ctx, n, plan.local, true)
	if !ctx.seen || !errors.Is(err, context.Canceled) || d.Outcome == OutcomeFail {
		t.Fatalf("scratch cancellation: %+v seen=%v err=%v", d, ctx.seen, err)
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("cancellation leaked %d scratch bytes", used)
	}
}

func TestLocalDistinctMixedFamiliesAndEncounterOrder(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		s, n, plan := distinctScratchFixture(t, 5, Budgets{})
		n.children[1].header.Declaration = Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "selection-unavailable"}
		n.children[2].attributes[0].Scalar.Value.Value = localString("0")
		if duplicate {
			n.children[4].attributes[0].Scalar.Value.Value = localInteger(t, 0)
		}
		d, _, err := s.evaluateLocalPlan(context.Background(), n, plan.local, true)
		want := OutcomeUnsupported
		if duplicate {
			want = OutcomeFail
		}
		if err != nil || d.Outcome != want {
			t.Fatalf("duplicate=%v mixed-family outcome: %+v %v", duplicate, d, err)
		}
		if duplicate && (len(d.Related) != 2 || d.Related[0].Path != n.children[0].header.Location.Path || d.Related[1].Path != n.children[4].header.Location.Path) {
			t.Fatalf("encounter-order related locations changed: %+v", d.Related)
		}
		s.cleanup()
		if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("mixed-family cleanup leaked %d bytes", used)
		}
	}
}
