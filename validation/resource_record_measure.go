package validation

import (
	"context"
	"reflect"
)

// resourceRecordMeasureAs sizes the closed source-origin payload, whose
// inline evidence headers are already part of the containing record. The fixed
// record is counted once, with allocation headroom. Every nonnil pointer, slice
// backing array and cloned string is counted independently, including aliases:
// resourceCopyInto intentionally does not preserve mutable aliases.
// This is not a general reflection entry point; resourceMeasureAs elects the
// exact library-defined root type, and numeric wrappers are immutable leaves.
func resourceRecordMeasureAs(ctx context.Context, a *Account, v reflect.Value, to string, allocation bool) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if err := a.Charge(BudgetRuleSteps, 1); err != nil {
		return 0, err
	}
	var total int64
	add := func(n int64) error {
		var err error
		total, err = rationalCheckedAdd(BudgetBufferedBytes, total, n)
		return err
	}
	if allocation {
		if err := add(int64(v.Type().Size()) + 32); err != nil {
			return 0, err
		}
	}
	if field, ok := resourceNumericFields[v.Type()]; ok {
		// The containing record already owns the wrapper header. Retain its cached
		// immutable magnitude allowance, independently for each occurrence.
		if err := add(v.Field(field).Int() + 64); err != nil {
			return 0, err
		}
		return total, nil
	}
	if to != "" && (v.Type() == reflect.TypeFor[NodeRef]() || v.Type() == reflect.TypeFor[Location]()) {
		// Copying first clones the intrinsic identity, then replaces it. Both may
		// coexist until the scratch copy ends; never subtract the original bytes.
		n, err := rationalCheckedMul(BudgetBufferedBytes, int64(len(to)), 2)
		if err != nil {
			return 0, err
		}
		if err = add(n); err != nil {
			return 0, err
		}
		if err = add(16); err != nil {
			return 0, err
		}
	}
	child := func(value reflect.Value, owned bool) error {
		n, err := resourceRecordMeasureAs(ctx, a, value, to, owned)
		if err != nil {
			return err
		}
		return add(n)
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if err := child(v.Field(i), false); err != nil {
				return 0, err
			}
		}
	case reflect.Pointer:
		if !v.IsNil() {
			if err := child(v.Elem(), true); err != nil {
				return 0, err
			}
		}
	case reflect.Slice:
		if !v.IsNil() {
			// The copier allocates exact len, never preserves source spare capacity.
			n, err := rationalCheckedMul(BudgetBufferedBytes, int64(v.Len()), int64(v.Type().Elem().Size()))
			if err != nil {
				return 0, err
			}
			if err = add(n); err != nil {
				return 0, err
			}
			if err = add(32); err != nil {
				return 0, err
			}
			for i := 0; i < v.Len(); i++ {
				if err = child(v.Index(i), false); err != nil {
					return 0, err
				}
			}
		}
	case reflect.String:
		blocks := int64(v.Len()) / resourceCopyChunk
		if v.Len()%resourceCopyChunk != 0 {
			blocks++
		}
		if err := a.Charge(BudgetRuleSteps, blocks); err != nil {
			return 0, err
		}
		if v.Len() != 0 {
			// Two lengths plus a minimum allocation allowance bound rounding and the
			// clone's independently owned storage, even for repeated string contents.
			n, err := rationalCheckedMul(BudgetBufferedBytes, int64(v.Len()), 2)
			if err != nil {
				return 0, err
			}
			if err = add(n); err != nil {
				return 0, err
			}
			if err = add(16); err != nil {
				return 0, err
			}
		}
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
	default:
		return 0, contract("resource record payload", "unsupported private payload")
	}
	if _, err := allocationSize(total); err != nil {
		return 0, err
	}
	return total, nil
}
