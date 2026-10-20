package musicxml

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"unsafe"

	"github.com/go-muse/go-musicxml/validation"
)

// goSizeClasses are the runtime's small-object size classes (Go 1.27).
// Larger objects occupy whole 8 KiB pages.
var goSizeClasses = []int64{8, 16, 24, 32, 48, 64, 80, 96, 112, 128, 144, 160, 176, 192, 208, 224, 240, 256, 288, 320, 352, 384, 416, 448, 480, 512, 576, 640, 704, 768, 896, 1024, 1152, 1280, 1408, 1536, 1792, 2048, 2304, 2688, 3072, 3200, 3456, 4096, 4864, 5376, 6144, 6528, 6784, 6912, 8192, 9472, 9728, 10240, 10880, 12288, 13568, 14336, 16384, 18432, 19072, 20480, 21760, 24576, 27264, 28672, 32768}

func goSizeClass(n int64) int64 {
	if n <= 0 {
		return 0
	}
	if i := sort.Search(len(goSizeClasses), func(i int) bool { return goSizeClasses[i] >= n }); i < len(goSizeClasses) {
		return goSizeClasses[i]
	}
	return (n + 8191) &^ 8191
}

// goAlloc is the heap of an n-byte allocation of type typ: objects with
// pointers above 512 bytes carry an 8-byte malloc header before rounding.
func goAlloc(n int64, typ reflect.Type) int64 {
	if n > 512 && goHasPointers(typ) {
		n += 8
	}
	return goSizeClass(n)
}

func goHasPointers(typ reflect.Type) bool {
	switch typ.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.String, reflect.Map, reflect.Interface, reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return true
	case reflect.Array:
		return typ.Len() > 0 && goHasPointers(typ.Elem())
	case reflect.Struct:
		for i := 0; i < typ.NumField(); i++ {
			if goHasPointers(typ.Field(i).Type) {
				return true
			}
		}
	}
	return false
}

// heapWalk sums the size classes of the distinct heap objects a value
// references: string data, slice backing arrays, pointer targets and boxed
// interface values. It reads unexported fields and counts shared storage once.
type heapWalk struct{ seen map[uintptr]bool }

func (w *heapWalk) object(addr uintptr, size int64) bool {
	if addr == 0 || size == 0 || w.seen[addr] {
		return false
	}
	w.seen[addr] = true
	return true
}

func (w *heapWalk) external(v reflect.Value) int64 {
	var n int64
	switch v.Kind() {
	case reflect.String:
		if v.Len() != 0 && w.object(uintptr(unsafe.Pointer(unsafe.StringData(v.String()))), int64(v.Len())) {
			n += goSizeClass(int64(v.Len()))
		}
	case reflect.Slice:
		if v.Cap() != 0 && w.object(v.Pointer(), int64(v.Cap())*int64(v.Type().Elem().Size())) {
			n += goAlloc(int64(v.Cap())*int64(v.Type().Elem().Size()), v.Type().Elem())
			for i := 0; i < v.Len(); i++ {
				n += w.external(v.Index(i))
			}
		}
	case reflect.Pointer:
		if !v.IsNil() && w.object(v.Pointer(), int64(v.Type().Elem().Size())) {
			n += goAlloc(int64(v.Type().Elem().Size()), v.Type().Elem()) + w.external(v.Elem())
		}
	case reflect.Interface:
		if !v.IsNil() {
			elem := v.Elem()
			if elem.Kind() != reflect.Pointer && elem.Type().Size() > 0 {
				n += goAlloc(int64(elem.Type().Size()), elem.Type())
			}
			n += w.external(elem)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			n += w.external(v.Field(i))
		}
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			n += w.external(v.Index(i))
		}
	case reflect.Map:
		if !v.IsNil() {
			n += 48 + int64(v.Len())*int64(v.Type().Key().Size()+v.Type().Elem().Size())*2
			iter := v.MapRange()
			for iter.Next() {
				n += w.external(iter.Key()) + w.external(iter.Value())
			}
		}
	}
	return n
}

// The projection a report keeps, Report.Context, is accounted for at no less
// than the heap it occupies: on corpus documents the retained-report lease of
// line 2 and of the semantic model profile covers the size classes of the
// backing array and of every string, slice, pointer target and number the
// records reference.
func TestContextProjectionAccountingCoversItsHeap(t *testing.T) {
	names, err := filepath.Glob(filepath.Join("testdata", "corpus", "musicxml-test-suite", "*.xml"))
	if err != nil || len(names) < 100 {
		t.Fatalf("corpus: %d files, %v", len(names), err)
	}
	stride, minimum := 3, 40
	if raceEnabled || testing.Short() {
		stride, minimum = 30, 4
	}
	measured := 0
	for i := 0; i < len(names); i += stride {
		data, err := os.ReadFile(names[i])
		if err != nil {
			t.Fatal(err)
		}
		result, err := DecodeAssessed(context.Background(), bytes.NewReader(data), DecodeAssessmentOptions{Lenient: true, Assessment: &AssessmentOptions{Budgets: validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 40}}})
		if err != nil || result.Document == nil {
			continue
		}
		name := filepath.Base(names[i])
		source, model := projectionLeases(t, data, result.Document)
		for line, got := range map[string]projectionLease{"source": source, "model": model} {
			if len(got.report.Context) == 0 {
				continue
			}
			real := projectionHeap(got.report.Context)
			if got.leased < real {
				t.Errorf("%s %s: %d records leased %d bytes, occupy %d", name, line, len(got.report.Context), got.leased, real)
			}
			measured++
			if i == 0 {
				t.Logf("%s %s: %d records leased %d bytes, occupy %d", name, line, len(got.report.Context), got.leased, real)
			}
		}
	}
	if measured < minimum {
		t.Fatalf("measured only %d projections", measured)
	}
}

type projectionLease struct {
	report validation.Report
	leased int64
}

func projectionHeap(records []validation.ContextResult) int64 {
	w := heapWalk{seen: map[uintptr]bool{}}
	return w.external(reflect.ValueOf(records))
}

// projectionLeases runs both strict-loading lines with retained reports and
// returns each report with the bytes its lease held.
func projectionLeases(t *testing.T, data []byte, document Document) (source, model projectionLease) {
	t.Helper()
	normalized, err := assessedLoadingOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	normalized.Assessment.Budgets = validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 40}
	sourceConfig, modelConfig, err := assessedLoadingLines(context.Background(), normalized)
	if err != nil {
		t.Fatal(err)
	}
	defer modelConfig.Dictionaries.Close()
	leased := func(lease *validation.ReportLease) int64 {
		before := mandatoryLiveBuffered(t, sourceConfig.Account)
		if err := lease.Close(); err != nil {
			t.Fatal(err)
		}
		return before - mandatoryLiveBuffered(t, sourceConfig.Account)
	}
	run, err := newSourceAssessmentRunWithConfig(context.Background(), bytes.NewReader(data), DecodeOptions{}, &sourceAssessmentConfig{assessmentRunConfig: sourceConfig, ByteOwner: sourceBytesPrimary}, false)
	if err != nil {
		t.Fatal(err)
	}
	report, lease, err := run.finishRetained(run.drain())
	if err != nil || lease == nil {
		t.Fatalf("source lease: %v", err)
	}
	source = projectionLease{report: report, leased: leased(lease)}
	modelRun, err := newModelAssessmentRun(context.Background(), document, modelConfig)
	if err != nil {
		t.Fatal(err)
	}
	_, walkErr := modelRun.walk(modelWalkOptions{})
	completion, err := modelRun.finishInput(walkErr)
	if err != nil {
		t.Fatal(err)
	}
	report, lease, err = modelRun.session.FinishWithRetainedReport(context.Background(), completion)
	if err != nil || lease == nil {
		t.Fatalf("model lease: %v", err)
	}
	model = projectionLease{report: report, leased: leased(lease)}
	return source, model
}
