package musicxml

import (
	"context"
	"github.com/go-muse/go-musicxml/validation"
	"math"
	"reflect"
)

// resourceWorkingLease accounts for root-retained copies from live indexed
// getters or resolver metadata. Getter-copy scratch and the longer root working
// lifetime are separate owners; no storage survives release in root tables.
func resourceWorkingLease(ctx context.Context, a *validation.Account, value any) (func(), error) {
	var bytes int64
	var walk func(reflect.Value, int) error
	add := func(n int64) error {
		if n < 0 || bytes > math.MaxInt64-n {
			return &validation.ConfigurationError{Field: "resource working value", Reason: "size overflow"}
		}
		bytes += n
		return nil
	}
	walk = func(v reflect.Value, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := a.Charge(validation.BudgetRuleSteps, 1); err != nil {
			return err
		}
		if depth > 64 {
			return &validation.ConfigurationError{Field: "resource working value", Reason: "closed value nesting exceeded"}
		}
		if !v.IsValid() {
			return nil
		}
		if err := add(64); err != nil {
			return err
		}
		switch v.Kind() {
		case reflect.Interface, reflect.Pointer:
			if !v.IsNil() {
				return walk(v.Elem(), depth+1)
			}
		case reflect.String:
			return add(int64(v.Len()))
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if err := walk(v.Field(i), depth+1); err != nil {
					return err
				}
			}
		case reflect.Array, reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				if err := walk(v.Index(i), depth+1); err != nil {
					return err
				}
			}
		case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64:
		default:
			return &validation.ConfigurationError{Field: "resource working value", Reason: "unsupported open value kind"}
		}
		return nil
	}
	if ctx == nil || a == nil {
		return nil, &validation.ConfigurationError{Field: "resource working value", Reason: "context and account required"}
	}
	if err := walk(reflect.ValueOf(value), 0); err != nil {
		return nil, err
	}
	if _, err := validation.AllocationSize(bytes); err != nil {
		return nil, err
	}
	if err := a.Charge(validation.BudgetBufferedBytes, bytes); err != nil {
		return nil, err
	}
	released := false
	return func() {
		if !released {
			_ = a.Release(validation.BudgetBufferedBytes, bytes)
			released = true
		}
	}, nil
}
