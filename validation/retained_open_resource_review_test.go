package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRetainedFactOpenHeaderScratchRefusalIsResourceError(t *testing.T) {
	_, s := sessionFixture(t, Budgets{})
	a := s.account
	used, _ := a.Usage(BudgetBufferedBytes)
	// The empty-binding header needs the scratch map's 256-byte first-group
	// allowance. Refuse that request by exactly one byte, before node storage.
	const scratch = int64(256)
	hold := a.Limits().MaxBufferedBytes - used - scratch + 1
	if err := a.Charge(BudgetBufferedBytes, hold); err != nil {
		t.Fatal(err)
	}
	err := s.Open(context.Background(), factHeader(1, 0, "test.root", "root"))
	var resource *ResourceError
	var session *SessionError
	if !errors.As(err, &resource) || !errors.Is(err, ErrResourceLimit) || errors.Is(err, ErrInvalidSession) || errors.As(err, &session) {
		t.Errorf("valid Open scratch refusal changed error category: %T %v", err, err)
	}
	if err == nil || !strings.Contains(err.Error(), "resource limit exceeded") || !strings.Contains(err.Error(), "buffered-bytes") {
		t.Errorf("resource diagnostic hidden: %v", err)
	}
	if s.facts != nil || s.root != nil || len(s.nodes) != 0 {
		t.Fatal("scratch refusal published unadmitted node storage")
	}
	_, finishErr := s.Finish(context.Background(), Completion{Syntax: SyntaxIncomplete})
	if !errors.Is(finishErr, err) || !errors.Is(finishErr, ErrResourceLimit) || errors.Is(finishErr, ErrInvalidSession) {
		t.Errorf("Finish changed the resource refusal: %v", finishErr)
	}
	if err := a.Release(BudgetBufferedBytes, hold); err != nil {
		t.Fatal(err)
	}
	if used, _ := a.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("Open/Finish leaked %d buffered bytes", used)
	}
	if depth, _ := a.Usage(BudgetDepth); depth != 0 {
		t.Fatalf("Open/Finish leaked depth %d", depth)
	}
}
