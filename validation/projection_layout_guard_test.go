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

// layoutGuardShape describes a filled value: strings of width bytes, outer
// slices of elems elements, exact numbers of about bits binary digits, and
// whether every pointer, rather than only the admitted state pointers, is set.
type layoutGuardShape struct {
	width, elems, bits int
	anyPointer         bool
}

// layoutGuardFill populates every settable field of v as shape describes.
func layoutGuardFill(t *testing.T, v reflect.Value, shape layoutGuardShape, depth int) {
	t.Helper()
	if depth > 6 || !v.CanSet() {
		return
	}
	magnitude := new(big.Int).Lsh(big.NewInt(1), uint(max(shape.bits, 1)))
	magnitude.Add(magnitude, big.NewInt(12345))
	switch v.Type() {
	case reflect.TypeFor[Integer]():
		i, err := NewInteger(magnitude)
		if err != nil {
			t.Fatal(err)
		}
		v.Set(reflect.ValueOf(i))
		return
	case reflect.TypeFor[Decimal]():
		d, err := NewDecimal(new(big.Rat).SetFrac(magnitude, big.NewInt(8)))
		if err != nil {
			t.Fatal(err)
		}
		v.Set(reflect.ValueOf(d))
		return
	case reflect.TypeFor[Rational]():
		r, err := NewRational(new(big.Rat).SetFrac(magnitude, big.NewInt(7)))
		if err != nil {
			t.Fatal(err)
		}
		v.Set(reflect.ValueOf(r))
		return
	}
	switch v.Kind() {
	case reflect.String:
		v.SetString(strings.Repeat("s", shape.width))
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(7)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(7)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			layoutGuardFill(t, v.Field(i), shape, depth+1)
		}
	case reflect.Slice:
		// Only the outermost slices take the requested length; nested ones
		// stay short, so that the filled graph stays small.
		n := shape.elems
		if depth > 1 {
			n = min(shape.elems, 3)
		}
		v.Set(reflect.MakeSlice(v.Type(), n, n))
		for i := 0; i < n; i++ {
			layoutGuardFill(t, v.Index(i), shape, depth+1)
		}
	case reflect.Pointer:
		switch v.Type() {
		case reflect.TypeFor[*DecisionRef](), reflect.TypeFor[*Evidence[string]](), reflect.TypeFor[*musicalPosition]():
		default:
			if !shape.anyPointer {
				return
			}
		}
		v.Set(reflect.New(v.Type().Elem()))
		layoutGuardFill(t, v.Elem(), shape, depth+1)
	}
}

// Every payload copy is charged at no less than the heap the copy occupies:
// the projections copied through contextRetainProjection, the timeline state
// records and repeat/navigation results measured by retainedLayoutBytes, the
// resource and dictionary results, record values and origins measured by
// retainedProjectionBytes (source origins have their own measure, guarded by
// TestProjectionClonesChargeTheirCopies), and the layout payload measured by
// layoutContextSize. Shapes cover short and long strings, slices past 32 KiB
// and exact numbers of about 32,768 bits, whose limb arrays and text dominate.
func TestRetainedLayoutBytesCoverTheCopyHeap(t *testing.T) {
	shapes := []layoutGuardShape{{1, 3, 64, false}, {7, 3, 64, false}, {40, 3, 64, false}, {1000, 3, 64, false}, {8193, 1, 64, false}, {100000, 1, 64, false}, {0, 64, 64, false}, {0, 2100, 64, false}, {1, 2, 32768, false}}
	layoutTypes := []reflect.Type{
		reflect.TypeFor[SpanContextResult](),
		reflect.TypeFor[TimingContextResult](),
		reflect.TypeFor[SoundMappingContextResult](),
		reflect.TypeFor[ContainerContextResult](),
		reflect.TypeFor[RepeatContextResult](),
		reflect.TypeFor[NavigationContextResult](),
		reflect.TypeFor[DictionarySpan](),
		reflect.TypeFor[musicalPosition](),
		reflect.TypeFor[DecisionRef](),
	}
	projectionTypes := []reflect.Type{
		reflect.TypeFor[ResourceContextResult](),
		reflect.TypeFor[DictionaryContextResult](),
		reflect.TypeFor[Evidence[Atomic]](),
		reflect.TypeFor[Location](),
	}
	for _, shape := range shapes {
		for _, anyPointer := range []bool{false, true} {
			types := layoutTypes
			if anyPointer {
				types = projectionTypes
			}
			shape.anyPointer = anyPointer
			for _, typ := range types {
				value := reflect.New(typ).Elem()
				layoutGuardFill(t, value, shape, 0)
				s := rationalSession(t, Budgets{MaxBufferedBytes: 1 << 40, MaxRuleSteps: 1 << 40, MaxScalarBytes: 1 << 30})
				measure := retainedLayoutBytes
				if anyPointer {
					measure = retainedProjectionBytes
				}
				charged, err := measure(context.Background(), s, value)
				if err != nil {
					t.Fatalf("%s %+v: %v", typ, shape, err)
				}
				holder := reflect.New(typ)
				holder.Elem().Set(stateCopy(value))
				if real := layoutGuardHeap(holder, map[uintptr]bool{}); charged < real {
					t.Errorf("%s %+v: charged %d, the copy occupies %d", typ, shape, charged, real)
				}
			}
		}
		var layout LayoutContextResult
		shape.anyPointer = false
		layoutGuardFill(t, reflect.ValueOf(&layout).Elem(), shape, 0)
		charged, _, err := layoutContextSize(layout)
		if err != nil {
			t.Fatal(err)
		}
		copied := cloneLayoutContextValue(nil, layout)
		if real := layoutGuardHeap(reflect.ValueOf(&copied), map[uintptr]bool{}); charged < real {
			t.Errorf("layout %+v: charged %d, the copy occupies %d", shape, charged, real)
		}
	}
}

// A published record is charged at no less than the heap its copy occupies,
// with a long scalar string, a long origin path, and a long exact number in
// the value: eight such records leave the projection subtotal at or above the
// measured heap of Report.Context.
func TestContextRecordValueAndOriginsCoverTheirHeap(t *testing.T) {
	long := strings.Repeat("x", 8193)
	// Each record holds its own number: a number shared by all eight would
	// be charged eight times but occupy the heap once.
	huge := func(i int) Integer {
		n, err := NewInteger(new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 32768), big.NewInt(int64(i))))
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	for name, row := range map[string]func(int) ContextResult{
		"long scalar": func(int) ContextResult {
			return ContextResult{Value: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: long}}, Origins: []Location{{Target: TargetSource, DocumentID: "source"}}}
		},
		"long origin": func(int) ContextResult {
			return ContextResult{Value: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "x"}}, Origins: []Location{{Target: TargetSource, DocumentID: "source", Path: long}}}
		},
		"long number": func(i int) ContextResult {
			return ContextResult{Value: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger, Integer: huge(i)}}, Origins: []Location{{Target: TargetSource, DocumentID: "source"}}}
		},
	} {
		s := rationalSession(t, Budgets{})
		for i := range 8 {
			record := row(i)
			record.Provider, record.Subject, record.Property = RuleKey{"record", 1}, NodeRef{"source", 1}, ContextPropertySpanHammerMarking
			if err := s.publishContextResult(context.Background(), record); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
		if real := layoutGuardHeap(reflect.ValueOf(s.contextResults), map[uintptr]bool{}); s.contextReportRetained < real {
			t.Errorf("%s: subtotal %d, the projection occupies %d", name, s.contextReportRetained, real)
		}
		s.cleanup()
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
		if got := s.retained - before; got != want || s.contextBackingHold || s.contextBackingHeld != 0 || s.contextReportRetained != s.retained-internCharge(s) || rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
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
	retained, subtotal, results, interned := r.s.retained, r.s.contextReportRetained, r.s.contextResults, internCharge(r.s)
	_, _, err := evaluateContextSpans(context.Background(), r.s, note, RuleKey{"spans", 1}, &contextPlan{Bindings: r.b}, true)
	// Intern table entries made meanwhile stay owned and charged.
	if !errors.Is(err, ErrResourceLimit) || cap(r.s.contextResults) != 8 || &r.s.contextResults[0] != &results[0] || r.s.retained != retained+service+internCharge(r.s)-interned || r.s.contextReportRetained != subtotal || r.s.contextBackingHeld != 0 || r.s.contextBackingHold {
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

// The retained storage of an exact number, its big value with the limb array
// at its actual capacity and its canonical text, is bounded by
// numericRetainedBytes at every magnitude.
func TestNumericRetainedBytesCoverTheirStorage(t *testing.T) {
	for _, bits := range []uint{1, 64, 4096, 32768, 200000} {
		magnitude := new(big.Int).Lsh(big.NewInt(1), bits)
		magnitude.Add(magnitude, big.NewInt(12345))
		integer, err := NewInteger(magnitude)
		if err != nil {
			t.Fatal(err)
		}
		decimal, err := NewDecimal(new(big.Rat).SetFrac(magnitude, big.NewInt(8)))
		if err != nil {
			t.Fatal(err)
		}
		rational, err := NewRational(new(big.Rat).SetFrac(magnitude, big.NewInt(7)))
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range []any{integer, decimal, rational} {
			charged, ok := numericRetainedBytes(n)
			real := layoutGuardHeap(reflect.ValueOf(n), map[uintptr]bool{})
			if !ok || charged < real {
				t.Errorf("%T of %d bits: charged %d, retains %d", n, bits, charged, real)
			}
		}
	}
}

// The resource, dictionary and source-origin clones charge at no less than
// the heap of the copies they return, for short, long and very long strings
// and long exact numbers.
func TestProjectionClonesChargeTheirCopies(t *testing.T) {
	for _, shape := range []layoutGuardShape{{1, 3, 64, true}, {1000, 3, 64, true}, {8193, 1, 64, true}, {100000, 1, 64, true}, {1, 2, 32768, true}} {
		limits := Budgets{MaxBufferedBytes: 1 << 40, MaxRuleSteps: 1 << 40, MaxScalarBytes: 1 << 30}
		var resource ResourceContextResult
		layoutGuardFill(t, reflect.ValueOf(&resource).Elem(), shape, 0)
		s := rationalSession(t, limits)
		charged, err := resourceContextSize(context.Background(), s, &resource)
		if err != nil {
			t.Fatalf("resource %+v: %v", shape, err)
		}
		if real := layoutGuardHeap(reflect.ValueOf(cloneResourceContextValue(&resource)), map[uintptr]bool{}); charged < real {
			t.Errorf("resource %+v: charged %d, the copy occupies %d", shape, charged, real)
		}
		for name, clone := range map[string]func(*Session) (any, error){
			"dictionary": func(s *Session) (any, error) {
				var p DictionaryContextResult
				layoutGuardFill(t, reflect.ValueOf(&p).Elem(), shape, 0)
				return cloneDictionaryContext(context.Background(), s, &p)
			},
			"source origin": func(s *Session) (any, error) {
				var p SourceOriginContextResult
				layoutGuardFill(t, reflect.ValueOf(&p).Elem(), shape, 0)
				return cloneSourceOriginContext(context.Background(), s, &p)
			},
		} {
			s := rationalSession(t, limits)
			copied, err := clone(s)
			if err != nil {
				t.Fatalf("%s %+v: %v", name, shape, err)
			}
			if real := layoutGuardHeap(reflect.ValueOf(copied), map[uintptr]bool{}); s.retained < real {
				t.Errorf("%s %+v: charged %d, the copy occupies %d", name, shape, s.retained, real)
			}
		}
	}
}

// The retained-storage walk checks the context and charges work as it goes:
// on a 128-member span a context that cancels at its second check stops the
// measurement, and a work allowance too small for the walk refuses it before
// the walk ends; each check is one rule step per 64 visited items.
func TestRetainedMeasurementStopsDuringTheWalk(t *testing.T) {
	span := SpanContextResult{Members: make([]NodeRef, 128), Included: make([]NodeRef, 128)}
	for i := range span.Members {
		span.Members[i] = NodeRef{"d", NodeID(i + 1)}
		span.Included[i] = span.Members[i]
	}
	value := reflect.ValueOf(span)
	probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	s := rationalSession(t, Budgets{})
	if _, err := retainedLayoutBytes(probe, s, value); err != nil {
		t.Fatal(err)
	}
	steps := rationalUsage(t, s, BudgetRuleSteps)
	if probe.calls < 4 {
		t.Fatalf("a %d-step walk checked the context %d times", steps, probe.calls)
	}
	s = rationalSession(t, Budgets{})
	if _, err := retainedLayoutBytes(&rationalCancelContext{Context: context.Background(), at: 2}, s, value); !errors.Is(err, context.Canceled) || s.retained != 0 {
		t.Fatalf("cancellation at the second check: err=%v retained=%d", err, s.retained)
	}
	s = rationalSession(t, Budgets{MaxRuleSteps: steps - 1})
	if _, err := retainedLayoutBytes(context.Background(), s, value); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("walk beyond the work allowance: %v", err)
	}
	var resource ResourceContextResult
	layoutGuardFill(t, reflect.ValueOf(&resource).Elem(), layoutGuardShape{width: 1, elems: 200, bits: 64, anyPointer: true}, 0)
	s = rationalSession(t, Budgets{})
	if _, err := resourceContextSize(&rationalCancelContext{Context: context.Background(), at: 2}, s, &resource); !errors.Is(err, context.Canceled) {
		t.Fatalf("resource measurement ignored cancellation: %v", err)
	}
}

// A record's origins are charged their fixed work per origin and their
// strings, and measured with checkpoints: a walk of 256 origins stops at a
// cancellation seen early in the walk, and publishing them charges at least
// 96 steps per origin.
func TestContextRecordOriginsMeasuredWithCheckpoints(t *testing.T) {
	origins := make([]Location, 256)
	for i := range origins {
		origins[i] = Location{Target: TargetSource, DocumentID: "source", Path: "/p/" + strings.Repeat("x", i%7)}
	}
	s := rationalSession(t, Budgets{})
	if _, err := measureRetainedMetered(&rationalCancelContext{Context: context.Background(), at: 2}, s, reflect.ValueOf(origins), true, true); !errors.Is(err, context.Canceled) {
		t.Fatalf("origins walk ignored cancellation: %v", err)
	}
	s.cleanup()
	s = rationalSession(t, Budgets{})
	defer s.cleanup()
	record := ContextResult{Provider: RuleKey{"origins", 1}, Subject: NodeRef{"source", 1}, Property: ContextPropertySpanHammerMarking, Value: Evidence[Atomic]{State: EvidenceAbsent}, Origins: origins}
	if err := s.publishContextResult(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if steps := rationalUsage(t, s, BudgetRuleSteps); steps < 96*256 {
		t.Fatalf("256 origins charged %d steps", steps)
	}
}

// internCharge is what a session's intern table has charged: its header and
// each entry's slot and string, which rollbacks keep. It is computed from the
// table itself and returns -1, which no charge equals, when the session's
// counter disagrees.
func internCharge(s *Session) int64 {
	table := int64(0)
	for v := range s.strings {
		table += internSlotBytes + ownedStringBytes(v)
	}
	if len(s.strings) != 0 {
		table += internTableBytes
	}
	if table != s.internRetained {
		return -1
	}
	return table
}
