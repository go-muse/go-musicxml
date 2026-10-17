package validation

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestReviewRetainedContainerCopyWork(t *testing.T) {
	for _, count := range []int{0, 8, 16} {
		for _, short := range []int64{0, 1} {
			for _, kind := range []string{"map", "slice"} {
				t.Run(fmt.Sprintf("%s/%d/short-%d", kind, count, short), func(t *testing.T) {
					s := retainedStorageSession(t)
					defer s.cleanup()
					var values []NodeID
					if kind == "map" {
						if count > 0 {
							s.nodes = make(map[NodeID]*factNode, count)
							s.nodeCapacity = count
							for i := 0; i < count; i++ {
								s.nodes[NodeID(i+1)] = &factNode{}
							}
							if err := s.retain(256 + 48*int64(count)); err != nil {
								t.Fatal(err)
							}
						}
					} else {
						values = make([]NodeID, count, count)
						if err := s.retain(8 * int64(count)); err != nil {
							t.Fatal(err)
						}
					}
					// Independent model: one growth operation plus exactly count copied entries.
					want := int64(count + 1)
					before, _ := s.account.Usage(BudgetRuleSteps)
					hold := s.account.Limits().MaxRuleSteps - before - want + short
					if err := s.account.Charge(BudgetRuleSteps, hold); err != nil {
						t.Fatal(err)
					}
					buffered, _ := s.account.Usage(BudgetBufferedBytes)
					var err error
					if kind == "map" {
						err = s.growFactNodes(context.Background())
					} else {
						values, err = growFactSlice(context.Background(), s, values)
					}
					if short == 0 {
						used, _ := s.account.Usage(BudgetRuleSteps)
						if err != nil || used != before+hold+want {
							t.Fatalf("copy work=%d want=%d err=%v", used-before-hold, want, err)
						}
					} else {
						var re *ResourceError
						if !errors.As(err, &re) || re.Kind != BudgetRuleSteps || re.Requested != want {
							t.Fatalf("one-short: %v", err)
						}
						used, _ := s.account.Usage(BudgetBufferedBytes)
						if used != buffered {
							t.Fatalf("failed growth retained bytes=%d want=%d", used, buffered)
						}
						if kind == "map" && s.nodeCapacity != count || kind == "slice" && cap(values) != count {
							t.Fatal("refused growth changed capacity")
						}
					}
				})
			}
		}
	}
}
