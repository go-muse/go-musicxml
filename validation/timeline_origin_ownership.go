package validation

import (
	"context"
	"reflect"
)

// ownOriginCauses operates only on a newly assembled private record, whose
// slices are already uniquely owned and admitted. Immutable numeric wrappers
// and strings stay shared. Only optional mutable causal records are copied.
// Unlike stateClone this does not duplicate already-owned large collections.
func ownOriginCauses(ctx context.Context, s *Session, value reflect.Value) error {
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	if !value.IsValid() {
		return nil
	}
	if value.CanInterface() {
		switch value.Interface().(type) {
		case Integer, Decimal, Rational:
			return nil
		}
	}
	switch value.Kind() {
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if err := ownOriginCauses(ctx, s, value.Field(i)); err != nil {
				return err
			}
		}
	case reflect.Slice:
		for i := 0; i < value.Len(); i++ {
			if err := ownOriginCauses(ctx, s, value.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Pointer:
		if value.IsNil() {
			return nil
		}
		if value.Type() != reflect.TypeFor[*DecisionRef]() {
			return contract("timeline origin", "unexpected mutable pointer")
		}
		if err := s.retain(128); err != nil {
			return err
		}
		copy := reflect.New(value.Type().Elem())
		copy.Elem().Set(value.Elem())
		value.Set(copy)
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
	default:
		return contract("timeline origin", "unsupported private payload")
	}
	return nil
}
func ownOrigin[T any](ctx context.Context, s *Session, value *T) error {
	return ownOriginCauses(ctx, s, reflect.ValueOf(value).Elem())
}
