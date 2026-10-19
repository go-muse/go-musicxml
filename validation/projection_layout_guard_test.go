package validation

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unsafe"
)

// layoutGuardSizeClasses are the runtime's small-object size classes (Go
// 1.27); larger objects occupy whole 8 KiB pages.
var layoutGuardSizeClasses = []int64{8, 16, 24, 32, 48, 64, 80, 96, 112, 128, 144, 160, 176, 192, 208, 224, 240, 256, 288, 320, 352, 384, 416, 448, 480, 512, 576, 640, 704, 768, 896, 1024, 1152, 1280, 1408, 1536, 1792, 2048, 2304, 2688, 3072, 3200, 3456, 4096, 4864, 5376, 6144, 6528, 6784, 6912, 8192, 9472, 9728, 10240, 10880, 12288, 13568, 14336, 16384, 18432, 19072, 20480, 21760, 24576, 27264, 28672, 32768}

func layoutGuardClass(n int64) int64 {
	if n <= 0 {
		return 0
	}
	if i := sort.Search(len(layoutGuardSizeClasses), func(i int) bool { return layoutGuardSizeClasses[i] >= n }); i < len(layoutGuardSizeClasses) {
		return layoutGuardSizeClasses[i]
	}
	return (n + 8191) &^ 8191
}

// layoutGuardHeap sums the size classes of the distinct heap objects that v
// references, reading unexported fields and counting shared storage once.
func layoutGuardHeap(v reflect.Value, seen map[uintptr]bool) int64 {
	object := func(addr uintptr) bool {
		if addr == 0 || seen[addr] {
			return false
		}
		seen[addr] = true
		return true
	}
	var n int64
	switch v.Kind() {
	case reflect.String:
		if v.Len() != 0 && object(uintptr(unsafe.Pointer(unsafe.StringData(v.String())))) {
			n += layoutGuardClass(int64(v.Len()))
		}
	case reflect.Slice:
		if v.Cap() != 0 && object(v.Pointer()) {
			n += layoutGuardClass(int64(v.Cap()) * int64(v.Type().Elem().Size()))
			for i := 0; i < v.Len(); i++ {
				n += layoutGuardHeap(v.Index(i), seen)
			}
		}
	case reflect.Pointer:
		if !v.IsNil() && object(v.Pointer()) {
			n += layoutGuardClass(int64(v.Type().Elem().Size())) + layoutGuardHeap(v.Elem(), seen)
		}
	case reflect.Interface:
		if !v.IsNil() {
			if elem := v.Elem(); elem.Kind() != reflect.Pointer && elem.Type().Size() > 0 {
				n += layoutGuardClass(int64(elem.Type().Size()))
			}
			n += layoutGuardHeap(v.Elem(), seen)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			n += layoutGuardHeap(v.Field(i), seen)
		}
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			n += layoutGuardHeap(v.Index(i), seen)
		}
	}
	return n
}

// layoutGuardFill populates every settable field: strings of width bytes,
// slices of elems elements, the admitted pointer types and exact numbers.
func layoutGuardFill(t *testing.T, v reflect.Value, width, elems int, depth int) {
	t.Helper()
	if depth > 6 || !v.CanSet() {
		return
	}
	switch v.Type() {
	case reflect.TypeFor[Integer]():
		n, _ := new(big.Int).SetString("123456789012345678901234567890", 10)
		i, err := NewInteger(n)
		if err != nil {
			t.Fatal(err)
		}
		v.Set(reflect.ValueOf(i))
		return
	case reflect.TypeFor[Decimal]():
		d, err := NewDecimal(big.NewRat(98765432101234567, 8))
		if err != nil {
			t.Fatal(err)
		}
		v.Set(reflect.ValueOf(d))
		return
	case reflect.TypeFor[Rational]():
		r, err := NewRational(big.NewRat(123456789012345, 7))
		if err != nil {
			t.Fatal(err)
		}
		v.Set(reflect.ValueOf(r))
		return
	}
	switch v.Kind() {
	case reflect.String:
		v.SetString(strings.Repeat("s", width))
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(7)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(7)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			layoutGuardFill(t, v.Field(i), width, elems, depth+1)
		}
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), elems, elems))
		for i := 0; i < elems; i++ {
			layoutGuardFill(t, v.Index(i), width, elems, depth+1)
		}
	case reflect.Pointer:
		switch v.Type() {
		case reflect.TypeFor[*DecisionRef](), reflect.TypeFor[*Evidence[string]](), reflect.TypeFor[*musicalPosition]():
			v.Set(reflect.New(v.Type().Elem()))
			layoutGuardFill(t, v.Elem(), width, elems, depth+1)
		}
	}
}

// Every payload measured by retainedLayoutBytes, the projections copied
// through contextRetainProjection and the timeline state records, is charged
// at no less than the heap its owned copy occupies, for short and long
// strings alike; so is the layout payload measured by layoutContextSize.
func TestRetainedLayoutBytesCoverTheCopyHeap(t *testing.T) {
	for _, shape := range []struct{ width, elems int }{{1, 3}, {7, 3}, {40, 3}, {1000, 3}, {0, 64}} {
		width := shape.width
		for _, typ := range []reflect.Type{
			reflect.TypeFor[SpanContextResult](),
			reflect.TypeFor[TimingContextResult](),
			reflect.TypeFor[SoundMappingContextResult](),
			reflect.TypeFor[ContainerContextResult](),
			reflect.TypeFor[DictionarySpan](),
			reflect.TypeFor[musicalPosition](),
			reflect.TypeFor[DecisionRef](),
		} {
			value := reflect.New(typ).Elem()
			layoutGuardFill(t, value, width, shape.elems, 0)
			s := rationalSession(t, Budgets{MaxBufferedBytes: 1 << 40, MaxRuleSteps: 1 << 40, MaxScalarBytes: 1 << 30})
			charged, err := retainedLayoutBytes(context.Background(), s, value)
			if err != nil {
				t.Fatalf("%s width %d: %v", typ, width, err)
			}
			copied := stateCopy(value)
			holder := reflect.New(typ)
			holder.Elem().Set(copied)
			real := layoutGuardHeap(holder, map[uintptr]bool{})
			if charged < real {
				t.Errorf("%s width %d: charged %d, the copy occupies %d", typ, width, charged, real)
			}
		}
		var layout LayoutContextResult
		layoutGuardFill(t, reflect.ValueOf(&layout).Elem(), width, shape.elems, 0)
		charged, _, err := layoutContextSize(layout)
		if err != nil {
			t.Fatal(err)
		}
		copied := cloneLayoutContextValue(layout)
		if real := layoutGuardHeap(reflect.ValueOf(&copied), map[uintptr]bool{}); charged < real {
			t.Errorf("layout width %d: charged %d, the copy occupies %d", width, charged, real)
		}
	}
}

// Report.Context's backing array is charged exactly for its capacity: a
// publication that grows it admits the new array while the old one is still
// charged, releases the old charge afterward, and a budget that cannot hold
// both refuses the publication without changing the projection.
func TestContextResultsBackingIsChargedForItsCapacity(t *testing.T) {
	row := ContextResult{Provider: RuleKey{"backing", 1}, Subject: NodeRef{"source", 1}, Property: ContextPropertySpanNormalDots, Value: Evidence[Atomic]{State: EvidenceAbsent}}
	s := rationalSession(t, Budgets{})
	ctx := context.Background()
	if err := s.publishContextResult(ctx, row); err != nil {
		t.Fatal(err)
	}
	if err := s.publishContextResult(ctx, row); err != nil {
		t.Fatal(err)
	}
	before := s.retained
	if err := s.publishContextResult(ctx, row); err != nil {
		t.Fatal(err)
	}
	record := s.retained - before
	for len(s.contextResults) < 64 {
		capacity, before := cap(s.contextResults), s.retained
		if err := s.publishContextResult(ctx, row); err != nil {
			t.Fatal(err)
		}
		want := record
		if cap(s.contextResults) != capacity {
			want += contextBackingBytes(cap(s.contextResults)) - contextBackingBytes(capacity)
			if cap(s.contextResults) != 2*capacity {
				t.Fatalf("grew %d to %d", capacity, cap(s.contextResults))
			}
		}
		if got := s.retained - before; got != want {
			t.Fatalf("publication %d charged %d, want %d", len(s.contextResults), got, want)
		}
		if heap := layoutGuardClass(int64(cap(s.contextResults)) * int64(unsafe.Sizeof(ContextResult{}))); contextBackingBytes(cap(s.contextResults)) < heap {
			t.Fatalf("capacity %d charged %d, occupies %d", cap(s.contextResults), contextBackingBytes(cap(s.contextResults)), heap)
		}
	}
	s.cleanup()

	for _, spare := range []int64{-1, 0} {
		probe := rationalSession(t, Budgets{})
		for range 8 {
			if err := probe.publishContextResult(ctx, row); err != nil {
				t.Fatal(err)
			}
		}
		limit := probe.retained + record + contextBackingBytes(16) + spare
		probe.cleanup()
		s := rationalSession(t, Budgets{MaxBufferedBytes: limit})
		for range 8 {
			if err := s.publishContextResult(ctx, row); err != nil {
				t.Fatal(err)
			}
		}
		retained, results := s.retained, s.contextResults
		err := s.publishContextResult(ctx, row)
		if spare < 0 {
			if !errors.Is(err, ErrResourceLimit) || s.retained != retained || len(s.contextResults) != 8 || cap(s.contextResults) != 8 || &s.contextResults[0] != &results[0] {
				t.Fatalf("growth beyond the budget: err=%v retained=%d/%d len=%d cap=%d", err, s.retained, retained, len(s.contextResults), cap(s.contextResults))
			}
		} else if err != nil || cap(s.contextResults) != 16 {
			t.Fatalf("growth within the budget: err=%v cap=%d", err, cap(s.contextResults))
		}
		s.cleanup()
		if used := rationalUsage(t, s, BudgetBufferedBytes); used != 0 {
			t.Fatalf("cleanup left %d", used)
		}
	}
}
