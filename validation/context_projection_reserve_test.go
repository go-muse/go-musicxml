package validation

import (
	"context"
	"reflect"
	"testing"
)

// Standalone projection copies do not append to an existing payload history.
// Their conservative graph measure must suffice without a second growth copy.
func TestContextStandaloneProjectionReserve(t *testing.T) {
	for _, value := range []any{SpanContextResult{}, LayoutContextResult{}, TimingContextResult{}} {
		v := reflect.New(reflect.TypeOf(value)).Elem()
		independentContextFill(v, "projection")
		value = v.Interface()
		probe := rationalSession(t, Budgets{MaxBufferedBytes: 1 << 28})
		n, err := stateMeasure(context.Background(), probe, reflect.ValueOf(value))
		probe.cleanup()
		if err != nil {
			t.Fatal(err)
		}
		if n < int64(reflect.TypeOf(value).Size()) {
			t.Fatal("measure below fixed representation")
		}
		s := rationalSession(t, Budgets{MaxBufferedBytes: n})
		switch p := value.(type) {
		case SpanContextResult:
			_, err = cloneSpanContext(context.Background(), s, &p)
		case LayoutContextResult:
			_, err = cloneLayoutContext(context.Background(), s, &p)
		case TimingContextResult:
			_, err = cloneTimingContext(context.Background(), s, &p)
		}
		s.cleanup()
		if err != nil {
			t.Fatalf("%T standalone payload reserved unrelated append growth: %v", value, err)
		}
	}
}
