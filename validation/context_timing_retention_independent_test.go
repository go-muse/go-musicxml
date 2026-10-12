package validation

import (
	"context"
	"reflect"
	"testing"
)

func TestIndependentContextTimingRetentionShape(t *testing.T) {
	p := newTimingContext("note")
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	n, err := stateMeasure(context.Background(), s, reflect.ValueOf(p))
	if err != nil {
		t.Fatal(err)
	}
	before := s.retained
	if _, err = cloneTimingContext(context.Background(), s, &p); err != nil {
		t.Fatal(err)
	}
	t.Logf("empty field-complete timing: fixed Go struct=%d bytes; recursive logical measure=%d; admitted clone retention=%d; fields=%d", reflect.TypeOf(p).Size(), n, s.retained-before, reflect.TypeOf(p).NumField())
	if s.retained-before < int64(reflect.TypeOf(p).Size()) {
		t.Fatal("payload retention undercharges fixed graph")
	}
}
