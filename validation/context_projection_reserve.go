package validation

import (
	"context"
	"math/big"
	"math/bits"
	"reflect"
	"unsafe"
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
// target the copy owns, and the numeric storage it shares (see
// retainedShape). It admits the same closed private value graph as
// stateMeasure and rejects any other pointer or payload.
func retainedLayoutBytes(ctx context.Context, s *Session, v reflect.Value) (int64, error) {
	return retainedBytes(ctx, s, v, false)
}

// retainedProjectionBytes measures a copy of a public context payload like
// retainedLayoutBytes, but follows any pointer: the payload types are a
// closed public vocabulary rather than private timeline state.
func retainedProjectionBytes(ctx context.Context, s *Session, v reflect.Value) (int64, error) {
	return retainedBytes(ctx, s, v, true)
}

func retainedBytes(ctx context.Context, s *Session, v reflect.Value, anyPointer bool) (int64, error) {
	if !v.IsValid() {
		return 0, s.step(ctx, 16)
	}
	shape, err := measureRetained(v, anyPointer)
	if err != nil {
		return 0, err
	}
	if err = shape.charge(ctx, s); err != nil {
		return 0, err
	}
	return rationalCheckedAdd(BudgetBufferedBytes, allocationSlack(int64(v.Type().Size())), shape.bytes)
}

// retainedShape is the storage a copy of a value owns outside the value
// itself: every nonempty string, slice backing array and pointer target, each
// rounded up by allocationSlack, plus the retained storage of every exact
// number (numericRetainedBytes). text counts the string bytes a copy
// duplicates and visited the items walked.
type retainedShape struct {
	bytes, text, visited int64
}

// charge admits the work of measuring and copying: every string byte the copy
// duplicates, one step per 64 visited items and 16 for the call.
func (r retainedShape) charge(ctx context.Context, s *Session) error {
	return s.step(ctx, 16+r.text+r.visited/64)
}

// allocationSlack bounds the heap an n-byte allocation occupies. Small objects
// round up to a size class, at most about 16% larger; objects with pointers
// above 512 bytes also carry an 8-byte malloc header; objects above 32 KiB
// round up to whole 8 KiB pages, which adds at most a quarter at that size and
// less beyond. n/4 + 24 covers all three, for every n.
func allocationSlack(n int64) int64 {
	if n <= 0 {
		return 0
	}
	return n + n/4 + 24
}

// ownedStringBytes is the heap of a copied string.
func ownedStringBytes(s string) int64 { return allocationSlack(int64(len(s))) }

const bigWordBytes = int64(bits.UintSize / 8)

// bigIntRetainedBytes is the heap of a big.Int object and its limb array at
// its actual capacity.
func bigIntRetainedBytes(x *big.Int) int64 {
	if x == nil {
		return 0
	}
	return allocationSlack(int64(unsafe.Sizeof(*x))) + allocationSlack(int64(cap(x.Bits()))*bigWordBytes)
}

// bigRatRetainedBytes is the heap of a big.Rat object and both limb arrays.
func bigRatRetainedBytes(x *big.Rat) int64 {
	if x == nil {
		return 0
	}
	return allocationSlack(int64(unsafe.Sizeof(*x))) + allocationSlack(int64(cap(x.Num().Bits()))*bigWordBytes) + allocationSlack(int64(cap(x.Denom().Bits()))*bigWordBytes)
}

// numericRetainedBytes is the storage an exact number keeps live outside its
// wrapper: the big value with its limb capacity and the canonical text. A copy
// shares this immutable storage with its source; it is still charged, since
// the copy keeps it reachable.
func numericRetainedBytes(v any) (int64, bool) {
	switch n := v.(type) {
	case Integer:
		return bigIntRetainedBytes(n.value) + ownedStringBytes(n.text), true
	case Decimal:
		return bigRatRetainedBytes(n.value) + ownedStringBytes(n.text), true
	case Rational:
		return bigRatRetainedBytes(n.value) + ownedStringBytes(n.text), true
	}
	return 0, false
}

func measureRetained(v reflect.Value, anyPointer bool) (retainedShape, error) {
	w := retainedLayoutWalk{anyPointer: anyPointer}
	bytes, err := w.external(v)
	return retainedShape{bytes: bytes, text: w.text, visited: w.visited}, err
}

type retainedLayoutWalk struct {
	anyPointer    bool
	text, visited int64
}

func (w *retainedLayoutWalk) external(v reflect.Value) (int64, error) {
	w.visited++
	if v.CanInterface() && v.Kind() == reflect.Struct {
		if n, ok := numericRetainedBytes(v.Interface()); ok {
			return n, nil
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
			if !w.anyPointer && v.Type() != reflect.TypeFor[*DecisionRef]() && v.Type() != reflect.TypeFor[*Evidence[string]]() && v.Type() != reflect.TypeFor[*musicalPosition]() {
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
		w.text += int64(v.Len())
		if e := add(allocationSlack(int64(v.Len())), nil); e != nil {
			return 0, e
		}
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
	default:
		return 0, contract("timeline state", "unsupported private payload")
	}
	return n, nil
}
