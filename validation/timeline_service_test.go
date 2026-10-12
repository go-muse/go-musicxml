package validation

import (
	"context"
	"errors"
	"testing"
)

func TestTimelineServiceRetainsBeforeAllocationAndCleansUp(t *testing.T) {
	s := rationalSession(t, Budgets{})
	key := RuleKey{ID: "timeline-test", Revision: 1}
	service, err := s.newTimelineService(context.Background(), key)
	if err != nil || service == nil || service.Final || len(service.Facts) != 0 {
		t.Fatalf("begin=%+v,%v", service, err)
	}
	if same, err := s.newTimelineService(context.Background(), key); err != nil || same != service {
		t.Fatalf("not session-owned: %p,%v", same, err)
	}
	if err = service.storeFact(context.Background(), s, 1, timelineEventFact{Part: timelineKnown(NodeRef{"score", 2})}); err != nil {
		t.Fatal(err)
	}
	if service.Facts[1].Part.Value.Node != 2 {
		t.Fatal("fact not stored")
	}
	s.cleanup()
	if s.timeline != nil {
		t.Fatal("timeline survived cleanup")
	}
	used, err := s.account.Usage(BudgetBufferedBytes)
	if err != nil || used != 0 {
		t.Fatalf("retained=%d,%v", used, err)
	}
}
func TestTimelineServiceFactInterruptionIsTransactional(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	service, err := s.newTimelineService(context.Background(), RuleKey{ID: "timeline-test", Revision: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = service.storeFact(ctx, s, 1, timelineEventFact{}); !errors.Is(err, context.Canceled) || len(service.Facts) != 0 {
		t.Fatalf("canceled=%v,%+v", err, service.Facts)
	}
}
