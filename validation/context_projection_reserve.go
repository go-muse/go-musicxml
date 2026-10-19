package validation

import (
	"context"
	"reflect"
)

// contextRetainProjection admits one standalone, exact-length owned graph.
// Unlike an append to state history, this copy has no old payload backing
// array that must coexist with a new one. Report.Context's independent backing
// growth is reserved by its publisher. Keep this narrower contract separate
// from accepted timeline state accounting.
func contextRetainProjection(ctx context.Context, s *Session, value any) error {
	n, err := retainedLayoutBytes(ctx, s, reflect.ValueOf(value))
	if err != nil {
		return err
	}
	if _, err = allocationSize(n); err != nil {
		return err
	}
	return s.retain(n)
}

// retainedLayoutBytes measures what an owned copy of v keeps live: one
// allocation of v's type plus every string, slice backing array and pointer
// target the copy owns, and the immutable numeric storage it shares. Each
// allocation is rounded up by allocationSlack, so the result bounds the heap
// the copy occupies. It admits the same closed private value graph as
// stateMeasure and rejects any other pointer or payload. Work is charged as
// the walk proceeds: every string byte the copy will duplicate, one step per
// 64 visited items and 16 for the call.
func retainedLayoutBytes(ctx context.Context, s *Session, v reflect.Value) (int64, error) {
	if err := s.step(ctx, 16); err != nil || !v.IsValid() {
		return 0, err
	}
	w := retainedLayoutWalk{ctx: ctx, s: s}
	external, err := w.external(v)
	if err != nil {
		return 0, err
	}
	return rationalCheckedAdd(BudgetBufferedBytes, allocationSlack(int64(v.Type().Size())), external)
}

type retainedLayoutWalk struct {
	ctx     context.Context
	s       *Session
	visited int64
}

// allocationSlack bounds the size class the Go allocator rounds an n-byte
// object up to: classes waste at most 12.5%, and the smallest is 8 bytes.
func allocationSlack(n int64) int64 {
	if n <= 0 {
		return 0
	}
	return n + n/8 + 16
}

func (w *retainedLayoutWalk) external(v reflect.Value) (int64, error) {
	if w.visited++; w.visited%64 == 0 {
		if err := w.s.step(w.ctx, 1); err != nil {
			return 0, err
		}
	}
	if v.CanInterface() && v.Kind() == reflect.Struct {
		switch n := v.Interface().(type) {
		case Integer:
			return n.ByteSize() + 64, nil
		case Decimal:
			return n.ByteSize() + 64, nil
		case Rational:
			return n.ByteSize() + 64, nil
		}
	}
	var n int64
	add := func(m int64, e error) error {
		if e != nil {
			return e
		}
		var x error
		n, x = rationalCheckedAdd(BudgetBufferedBytes, n, m)
		return x
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if e := add(w.external(v.Field(i))); e != nil {
				return 0, e
			}
		}
	case reflect.Slice:
		if v.Len() == 0 {
			return 0, nil
		}
		backing, e := rationalCheckedMul(BudgetBufferedBytes, int64(v.Len()), int64(v.Type().Elem().Size()))
		if e != nil {
			return 0, e
		}
		if e = add(allocationSlack(backing), nil); e != nil {
			return 0, e
		}
		for i := 0; i < v.Len(); i++ {
			if e := add(w.external(v.Index(i))); e != nil {
				return 0, e
			}
		}
	case reflect.Pointer:
		if !v.IsNil() {
			if v.Type() != reflect.TypeFor[*DecisionRef]() && v.Type() != reflect.TypeFor[*Evidence[string]]() && v.Type() != reflect.TypeFor[*musicalPosition]() {
				return 0, contract("timeline state", "unexpected mutable pointer")
			}
			if e := add(allocationSlack(int64(v.Type().Elem().Size())), nil); e != nil {
				return 0, e
			}
			if e := add(w.external(v.Elem())); e != nil {
				return 0, e
			}
		}
	case reflect.String:
		if e := w.s.step(w.ctx, int64(v.Len())); e != nil {
			return 0, e
		}
		if e := add(allocationSlack(int64(v.Len())), nil); e != nil {
			return 0, e
		}
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
	default:
		return 0, contract("timeline state", "unsupported private payload")
	}
	return n, nil
}
