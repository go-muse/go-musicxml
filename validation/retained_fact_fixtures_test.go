package validation

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// Private fixture conversions preserve the public values while allocating an
// independent evidence record for every field, including preclose zero values.
// Tests may therefore mutate their fixtures without changing another node.
func testEvidence[T any](v Evidence[T]) *Evidence[T] { return &v }

func testRetainedHeader(v NodeHeader) retainedHeader {
	return retainedHeader{
		ID: v.ID, Parent: v.Parent, Name: v.Name,
		Type: testEvidence(v.Type), Declaration: testEvidence(v.Declaration),
		Location: v.Location, Bindings: v.Bindings,
	}
}

func testRetainedEnd(v NodeEnd) retainedEnd {
	return retainedEnd{
		Scalar:           retainedScalar{Lexeme: testEvidence(v.Scalar.Lexeme), Value: testEvidence(v.Scalar.Value)},
		ChildrenComplete: testEvidence(v.ChildrenComplete), AttributesComplete: testEvidence(v.AttributesComplete),
		Order: testEvidence(v.Order),
	}
}

func testCopyFactNode(v *factNode) factNode {
	out := *v
	out.header = testRetainedHeader(v.header.value())
	out.end = testRetainedEnd(v.end.value(v.header.ID))
	return out
}

// This oracle uses only the requested flat-tree size. The four containers have
// independent growth schedules: the map starts at eight slots, the sequence and
// child slice at one, and the open stack needs at most two slots. For capacity C
// and initial capacity I, doubling copies I+2I+...+C/2 = C-I existing entries;
// each allocation additionally consumes one step. No runtime capacity, account
// usage, or production growth helper contributes to the expected work.
type testFactStorageGrowth struct {
	capacity            int
	copies, allocations int64
}

type testFactStorageLedger struct {
	nodes, sequence, children, stack testFactStorageGrowth
}

func testExpectedFactGrowth(length, initial int) testFactStorageGrowth {
	if length == 0 {
		return testFactStorageGrowth{}
	}
	capacity, allocations := initial, int64(1)
	for capacity < length {
		capacity *= 2
		allocations++
	}
	return testFactStorageGrowth{capacity, int64(capacity - initial), allocations}
}

func testFlatFactStorageLedger(children int) testFactStorageLedger {
	depth := 1
	if children > 0 {
		depth = 2
	}
	return testFactStorageLedger{
		nodes:    testExpectedFactGrowth(children+1, 8),
		sequence: testExpectedFactGrowth(children+1, 1),
		children: testExpectedFactGrowth(children, 1),
		stack:    testExpectedFactGrowth(depth, 1),
	}
}

func (v testFactStorageLedger) work() int64 {
	var work int64
	for _, growth := range []testFactStorageGrowth{v.nodes, v.sequence, v.children, v.stack} {
		work += growth.copies + growth.allocations
	}
	return work
}

func testFlatFactStorageWork(children int) int64 {
	return testFlatFactStorageLedger(children).work()
}

func TestFlatFactStorageHardcodedGrowthLedger(t *testing.T) {
	// Each triple is {capacity, existing entries copied, allocations}; container
	// order is node map, sequence, root children, then the depth-two open stack.
	for _, tc := range []struct {
		children int
		want     [4]testFactStorageGrowth
		work     int64
	}{
		{0, [4]testFactStorageGrowth{{8, 0, 1}, {1, 0, 1}, {0, 0, 0}, {1, 0, 1}}, 3},
		{1, [4]testFactStorageGrowth{{8, 0, 1}, {2, 1, 2}, {1, 0, 1}, {2, 1, 2}}, 8},
		{2, [4]testFactStorageGrowth{{8, 0, 1}, {4, 3, 3}, {2, 1, 2}, {2, 1, 2}}, 13},
		{3, [4]testFactStorageGrowth{{8, 0, 1}, {4, 3, 3}, {4, 3, 3}, {2, 1, 2}}, 16},
		{4, [4]testFactStorageGrowth{{8, 0, 1}, {8, 7, 4}, {4, 3, 3}, {2, 1, 2}}, 21},
		{7, [4]testFactStorageGrowth{{8, 0, 1}, {8, 7, 4}, {8, 7, 4}, {2, 1, 2}}, 26},
		{8, [4]testFactStorageGrowth{{16, 8, 2}, {16, 15, 5}, {8, 7, 4}, {2, 1, 2}}, 44},
		{9, [4]testFactStorageGrowth{{16, 8, 2}, {16, 15, 5}, {16, 15, 5}, {2, 1, 2}}, 53},
		{15, [4]testFactStorageGrowth{{16, 8, 2}, {16, 15, 5}, {16, 15, 5}, {2, 1, 2}}, 53},
		{16, [4]testFactStorageGrowth{{32, 24, 3}, {32, 31, 6}, {16, 15, 5}, {2, 1, 2}}, 87},
		{17, [4]testFactStorageGrowth{{32, 24, 3}, {32, 31, 6}, {32, 31, 6}, {2, 1, 2}}, 104},
	} {
		t.Run(fmt.Sprint(tc.children), func(t *testing.T) {
			got := testFlatFactStorageLedger(tc.children)
			if entries := [4]testFactStorageGrowth{got.nodes, got.sequence, got.children, got.stack}; entries != tc.want {
				t.Fatalf("growth ledger=%+v, want %+v", entries, tc.want)
			}
			if got := testFlatFactStorageWork(tc.children); got != tc.work {
				t.Fatalf("storage work=%d, want %d", got, tc.work)
			}
		})
	}
}

func TestFlatFactStorageAmortizedAndExactWorkBoundaries(t *testing.T) {
	counts := []int{0, 1, 2}
	for power := 4; power <= 1024; power *= 2 {
		counts = append(counts, power-1, power, power+1)
	}
	for _, count := range counts {
		storage := testFlatFactStorageWork(count)
		if storage > 8*int64(count+1) {
			t.Fatalf("children=%d storage=%d exceeds amortized bound %d", count, storage, 8*(count+1))
		}
		// Exactly the original excluded-consumer scheduler ledger: two selector
		// checks per child, two child-count steps per child, and five fixed steps.
		total := 4*int64(count) + 5 + storage
		for _, short := range []int64{0, 1} {
			t.Run(fmt.Sprintf("children-%d/short-%d", count, short), func(t *testing.T) {
				e := reviewDocumentEngine(t, func(rules *[]RuleDefinition) {
					(*rules)[0].Selector.Declaration.Occurrence = "test.root"
					(*rules)[1].Selector.Declaration.Occurrence = "never-selected"
				})
				ctx := context.Background()
				s, err := e.Begin(ctx, RunOptions{DocumentID: "doc", Profile: "test.source", Budgets: Budgets{MaxRuleSteps: total - short}})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(s.cleanup)
				account := s.account
				err = s.OpenNode(ctx, factHeader(1, 0, "test.root", "root"))
				for i := 0; err == nil && i < count; i++ {
					id := NodeID(i + 2)
					err = s.OpenNode(ctx, factHeader(id, 1, "test.child", "child"))
					if err == nil {
						err = s.CloseNode(ctx, factEnd(id))
					}
				}
				if err == nil {
					err = s.CloseNode(ctx, factEnd(1))
				}
				if short == 0 {
					if err != nil {
						t.Fatalf("exact storage admission failed: %v", err)
					}
					want := testFlatFactStorageLedger(count)
					got := [4]int{s.nodeCapacity, cap(s.sequence), cap(s.root.children), cap(s.stack)}
					capacities := [4]int{want.nodes.capacity, want.sequence.capacity, want.children.capacity, want.stack.capacity}
					if got != capacities {
						t.Fatalf("capacities=%v, want %v", got, capacities)
					}
				}
				report, finishErr := s.Finish(ctx, Completion{InputComplete: err == nil, Syntax: SyntaxWellFormed, Cause: err})
				used, _ := account.Usage(BudgetRuleSteps)
				if short == 0 {
					if err != nil || finishErr != nil || !report.Satisfied() || used != total {
						t.Fatalf("exact total=%d used=%d report=%+v ingress=%v finish=%v", total, used, report, err, finishErr)
					}
				} else {
					var resource *ResourceError
					if !errors.As(finishErr, &resource) || resource.Kind != BudgetRuleSteps || resource.Limit != total-short || used != total-short || report.Terminal == nil || report.AssessmentComplete {
						t.Fatalf("one-short total=%d used=%d report=%+v ingress=%v finish=%v", total, used, report, err, finishErr)
					}
				}
				for _, kind := range []BudgetKind{BudgetBufferedBytes, BudgetDepth} {
					if used, _ := account.Usage(kind); used != 0 {
						t.Fatalf("%s leaked %d", kind, used)
					}
				}
			})
		}
	}
}
