package validation

import (
	"context"
	"reflect"
)

// contextRetainProjection admits one standalone, exact-length owned graph.
// stateMeasure already conservatively counts every fixed field, nested value,
// string and immutable numeric magnitude. Unlike an append to state history,
// this copy has no old payload backing array that must coexist with a new one.
// Report.Context's independent backing growth is reserved by its publisher.
// Keep this narrower contract separate from accepted timeline state accounting.
func contextRetainProjection(ctx context.Context, s *Session, value any) error {
	n, err := stateMeasure(ctx, s, reflect.ValueOf(value))
	if err != nil {
		return err
	}
	if _, err = AllocationSize(n); err != nil {
		return err
	}
	return s.retain(n)
}
