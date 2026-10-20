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

// layoutGuardAlloc is the heap an allocation of n bytes of type typ occupies:
// objects with pointers above 512 bytes carry an 8-byte malloc header before
// rounding (Go 1.22 and later).
func layoutGuardAlloc(n int64, typ reflect.Type) int64 {
	if n > 512 && layoutGuardHasPointers(typ) {
		n += 8
	}
	return layoutGuardClass(n)
}

func layoutGuardHasPointers(typ reflect.Type) bool {
	switch typ.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.String, reflect.Map, reflect.Interface, reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return true
	case reflect.Array:
		return typ.Len() > 0 && layoutGuardHasPointers(typ.Elem())
	case reflect.Struct:
		for i := 0; i < typ.NumField(); i++ {
			if layoutGuardHasPointers(typ.Field(i).Type) {
				return true
			}
		}
	}
	return false
}

// allocationSlack bounds the heap of every allocation size, with and without
// the malloc header, through the small size classes and the page-rounded
// large objects.
func TestAllocationSlackBoundsEveryAllocationSize(t *testing.T) {
	pointers, scalars := reflect.TypeFor[*int](), reflect.TypeFor[byte]()
	for n := int64(1); n <= 160<<10; n++ {
		for _, typ := range []reflect.Type{pointers, scalars} {
			if heap := layoutGuardAlloc(n, typ); allocationSlack(n) < heap {
				t.Fatalf("%d bytes of %s: slack %d, heap %d", n, typ, allocationSlack(n), heap)
			}
		}
	}
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
			n += layoutGuardAlloc(int64(v.Cap())*int64(v.Type().Elem().Size()), v.Type().Elem())
			for i := 0; i < v.Len(); i++ {
				n += layoutGuardHeap(v.Index(i), seen)
			}
		}
	case reflect.Pointer:
		if !v.IsNil() && object(v.Pointer()) {
			n += layoutGuardAlloc(int64(v.Type().Elem().Size()), v.Type().Elem()) + layoutGuardHeap(v.Elem(), seen)
		}
	case reflect.Interface:
		if !v.IsNil() {
			if elem := v.Elem(); elem.Kind() != reflect.Pointer && elem.Type().Size() > 0 {
				n += layoutGuardAlloc(int64(elem.Type().Size()), elem.Type())
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
		// Only the outermost slices take the requested length; nested ones
		// stay short, so that the filled graph stays small.
		n := elems
		if depth > 1 {
			n = min(elems, 3)
		}
		v.Set(reflect.MakeSlice(v.Type(), n, n))
		for i := 0; i < n; i++ {
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
	// 2100 elements put slices of 16-byte and larger elements past 32 KiB,
	// where the page rounding applies.
	for _, shape := range []struct{ width, elems int }{{1, 3}, {7, 3}, {40, 3}, {1000, 3}, {0, 64}, {0, 2100}} {
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
	for len(s.contextResults) < 1024 {
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
		if heap := layoutGuardAlloc(int64(cap(s.contextResults))*int64(unsafe.Sizeof(ContextResult{})), reflect.TypeFor[ContextResult]()); contextBackingBytes(cap(s.contextResults)) < heap {
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

// While a rollback snapshot may restore an earlier header, growth keeps the
// replaced array charged; a commit releases it from the session and the
// projection subtotal, a rollback leaves the restored array's charge.
func TestContextBackingHeldUntilCommit(t *testing.T) {
	row := ContextResult{Provider: RuleKey{"backing", 1}, Subject: NodeRef{"source", 1}, Property: ContextPropertySpanNormalDots, Value: Evidence[Atomic]{State: EvidenceAbsent}}
	ctx := context.Background()
	for _, commit := range []bool{true, false} {
		s := rationalSession(t, Budgets{})
		for range 8 {
			if err := s.publishContextResult(ctx, row); err != nil {
				t.Fatal(err)
			}
		}
		record := contextBackingBytes(8)
		before := s.retained
		previous := s.holdContextBacking()
		if err := s.publishContextResult(ctx, row); err != nil {
			t.Fatal(err)
		}
		grown := s.retained - before
		if cap(s.contextResults) != 16 || s.contextBackingHeld != record || grown <= contextBackingBytes(16) {
			t.Fatalf("held growth: cap=%d held=%d grown=%d", cap(s.contextResults), s.contextBackingHeld, grown)
		}
		s.releaseHeldContextBacking(previous, commit)
		want := grown
		if commit {
			want -= record
		}
		if got := s.retained - before; got != want || s.contextBackingHold || s.contextBackingHeld != 0 || s.contextReportRetained != s.retained || rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatalf("commit=%v: retained delta %d, want %d; subtotal %d of %d", commit, got, want, s.contextReportRetained, s.retained)
		}
		s.cleanup()
	}
}

// A span evaluation that grows Report.Context and then fails restores the
// earlier header with exactly its charge; only the context service, created
// before the evaluation's snapshot, stays charged.
func TestSpanRollbackAfterGrowthRestoresAccounting(t *testing.T) {
	control, _ := spanRollbackFixture(t)
	before := control.s.retained
	if _, err := control.s.newContextService(context.Background(), RuleKey{"spans", 1}); err != nil {
		t.Fatal(err)
	}
	service := control.s.retained - before
	r, note := spanRollbackFixture(t)
	retained, subtotal, results := r.s.retained, r.s.contextReportRetained, r.s.contextResults
	_, _, err := evaluateContextSpans(context.Background(), r.s, note, RuleKey{"spans", 1}, &contextPlan{Bindings: r.b}, true)
	if !errors.Is(err, ErrResourceLimit) || cap(r.s.contextResults) != 8 || &r.s.contextResults[0] != &results[0] || r.s.retained != retained+service || r.s.contextReportRetained != subtotal || r.s.contextBackingHeld != 0 || r.s.contextBackingHold {
		t.Fatalf("rollback: err=%v cap=%d retained=%d/%d+%d subtotal=%d/%d", err, cap(r.s.contextResults), r.s.retained, retained, service, r.s.contextReportRetained, subtotal)
	}
	if used, _ := r.s.account.Usage(BudgetContextFacts); used != 9 {
		t.Fatalf("fixture did not grow before rollback: context facts=%d", used)
	}
	if used, _ := r.s.account.Usage(BudgetBufferedBytes); used != r.s.retained {
		t.Fatalf("account %d, session %d", used, r.s.retained)
	}
}

// spanRollbackFixture holds a slur span whose publication grows a full
// eight-record Report.Context and then exhausts MaxContextFacts.
func spanRollbackFixture(t *testing.T) (*contextReader, *factNode) {
	t.Helper()
	return spanGrowthFixture(t, Budgets{MaxContextFacts: 9})
}

// spanGrowthFixture holds a slur span whose publication grows a full
// eight-record Report.Context, under limits.
func spanGrowthFixture(t *testing.T, limits Budgets) (*contextReader, *factNode) {
	t.Helper()
	r := spanUnitReader(t)
	account, err := NewBudgetAccount(limits)
	if err != nil {
		t.Fatal(err)
	}
	r.s.account = account
	note := spanUnitNode(r, spanRole_note, nil)
	for _, role := range []string{"start", "stop"} {
		node := spanUnitNode(r, spanRole_slur, note)
		spanUnitAttribute(r, node, spanRole_slur_type, spanString(role))
	}
	row := ContextResult{Provider: RuleKey{"retained-report", 1}, Subject: NodeRef{r.s.documentID, note.header.ID}, Property: ContextPropertySpanNormalDots, Value: Evidence[Atomic]{State: EvidenceAbsent}}
	for range 8 {
		if err := r.s.publishContextResult(context.Background(), row); err != nil {
			t.Fatal(err)
		}
	}
	return r, note
}

// A successful span evaluation that grows Report.Context keeps the replaced
// array charged until it commits: its peak includes that array on top of what
// remains charged afterwards, so a buffer one byte short of both refuses it.
func TestSpanEvaluationHoldsReplacedBacking(t *testing.T) {
	r, note := spanGrowthFixture(t, Budgets{})
	if _, _, err := evaluateContextSpans(context.Background(), r.s, note, RuleKey{"spans", 1}, &contextPlan{Bindings: r.b}, true); err != nil || cap(r.s.contextResults) != 16 {
		t.Fatalf("control: err=%v cap=%d", err, cap(r.s.contextResults))
	}
	peak := r.s.retained + contextBackingBytes(8)
	for _, spare := range []int64{-1, 0} {
		r, note := spanGrowthFixture(t, Budgets{MaxBufferedBytes: peak + spare})
		_, _, err := evaluateContextSpans(context.Background(), r.s, note, RuleKey{"spans", 1}, &contextPlan{Bindings: r.b}, true)
		if spare < 0 && !errors.Is(err, ErrResourceLimit) || spare == 0 && err != nil {
			t.Fatalf("buffer %d: err=%v", peak+spare, err)
		}
	}
}
