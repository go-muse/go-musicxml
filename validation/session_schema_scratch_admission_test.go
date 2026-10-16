package validation

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"unsafe"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

// Compile-time guards also validate cross-compiled targets that this host
// cannot execute. A slot-envelope regression is a compilation failure.
const _ = uint64(schemaBindFrameBytes) - uint64(unsafe.Sizeof(schemaBindFrame{}))
const _ = uint64(schemaBindCompletionBytes) - uint64(unsafe.Sizeof(xsd.Completion{}))

func TestSchemaBindingScratchSlotEnvelopes(t *testing.T) {
	for _, item := range []struct {
		name  string
		size  uintptr
		limit int64
	}{
		{"frame", unsafe.Sizeof(schemaBindFrame{}), schemaBindFrameBytes},
		{"completion", unsafe.Sizeof(xsd.Completion{}), schemaBindCompletionBytes},
	} {
		if uint64(item.size) > uint64(item.limit) {
			t.Fatalf("%s size %d exceeds admitted slot %d", item.name, item.size, item.limit)
		}
		t.Logf("%s: size %d <= envelope %d", item.name, item.size, item.limit)
	}
}

func TestSchemaBindingScratchGrowthAdmission(t *testing.T) {
	const unrelated int64 = 73
	for _, oldCapacity := range []int{0, 1, 2, 4, 8, 16} {
		for _, short := range []int64{0, 1} {
			t.Run(fmt.Sprintf("old=%d/short=%d", oldCapacity, short), func(t *testing.T) {
				newCapacity := max(1, oldCapacity*2)
				peak := int64(oldCapacity+newCapacity) * schemaBindFrameBytes
				a, err := NewBudgetAccount(Budgets{MaxBufferedBytes: unrelated + peak - short})
				if err != nil {
					t.Fatal(err)
				}
				if err := a.Charge(BudgetBufferedBytes, unrelated); err != nil {
					t.Fatal(err)
				}
				w := schemaBindScratch{account: a}
				defer w.release()
				for i := 0; i < oldCapacity; i++ {
					if err := w.push(context.Background(), schemaBindFrame{next: i}); err != nil {
						t.Fatal(err)
					}
				}
				err = w.push(context.Background(), schemaBindFrame{next: oldCapacity})
				if short != 0 {
					var resource *ResourceError
					if !errors.As(err, &resource) || resource.Kind != BudgetBufferedBytes || resource.Used != unrelated+int64(oldCapacity)*schemaBindFrameBytes || resource.Requested != int64(newCapacity)*schemaBindFrameBytes {
						t.Fatalf("growth did not admit overlap before allocating: %v", err)
					}
					if len(w.frames) != oldCapacity || cap(w.frames) != oldCapacity {
						t.Fatal("refused growth changed backing storage")
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					if cap(w.frames) != newCapacity {
						t.Fatalf("capacity=%d, want %d", cap(w.frames), newCapacity)
					}
					for i, f := range w.frames {
						if f.next != i {
							t.Fatal("growth lost frame state")
						}
					}
					if used, _ := a.Usage(BudgetBufferedBytes); used != unrelated+int64(newCapacity)*schemaBindFrameBytes {
						t.Fatalf("old backing remained charged: %d", used)
					}
				}
				w.release()
				w.release()
				if used, _ := a.Usage(BudgetBufferedBytes); used != unrelated {
					t.Fatalf("release changed unrelated ownership: %d", used)
				}
			})
		}
	}
}

func TestSchemaBindingScratchCompletionAdmissionAndPop(t *testing.T) {
	for _, short := range []int64{0, 1} {
		a, _ := NewBudgetAccount(Budgets{MaxBufferedBytes: schemaBindFrameBytes + 3*schemaBindCompletionBytes - short})
		w := schemaBindScratch{account: a}
		if err := w.push(context.Background(), schemaBindFrame{node: &factNode{}}); err != nil {
			t.Fatal(err)
		}
		err := w.reserve(3, schemaBindCompletionBytes)
		if short != 0 {
			if !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("one-short completions: %v", err)
			}
			if w.frames[0].children != nil || w.bytes != schemaBindFrameBytes {
				t.Fatal("refusal allocated or charged children")
			}
		} else {
			if err != nil {
				t.Fatal(err)
			}
			w.frames[0].children = make([]xsd.Completion, 0, 3)
			w.pop()
			if w.frames[:cap(w.frames)][0].node != nil || w.frames[:cap(w.frames)][0].children != nil {
				t.Fatal("popped frame retains owned or borrowed pointers")
			}
			if used, _ := a.Usage(BudgetBufferedBytes); used != schemaBindFrameBytes {
				t.Fatalf("consumed completions remain charged: %d", used)
			}
		}
		w.release()
		if used, _ := a.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("scratch cleanup: %d", used)
		}
	}
}

func TestSchemaBindingScratchRejectsOversizeBeforeAllocation(t *testing.T) {
	a, _ := NewBudgetAccount(Budgets{MaxBufferedBytes: math.MaxInt64})
	w := schemaBindScratch{account: a}
	defer w.release()
	for _, pair := range [][2]int64{{math.MaxInt64, 2}, {int64(^uint(0)>>1)/schemaBindFrameBytes + 1, schemaBindFrameBytes}} {
		if err := w.reserve(pair[0], pair[1]); !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("unsafe allocation admitted: %v", err)
		}
		if used, _ := a.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("refused size charged %d", used)
		}
	}
	if err := w.push(nil, schemaBindFrame{}); err == nil || w.frames != nil || w.bytes != 0 {
		t.Fatalf("nil context admitted allocation: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := w.push(ctx, schemaBindFrame{}); !errors.Is(err, context.Canceled) || w.frames != nil || w.bytes != 0 {
		t.Fatalf("canceled allocation admitted: %v", err)
	}
}

type schemaScratchCancelContext struct {
	context.Context
	calls, at int
	when      func() bool
}

func (c *schemaScratchCancelContext) Err() error {
	c.calls++
	if c.at > 0 && c.calls >= c.at || c.when != nil && c.when() {
		return context.Canceled
	}
	return nil
}

func schemaScratchAssertReleased(t *testing.T, s *Session) {
	t.Helper()
	if s.schema.run != nil {
		if err := s.schema.run.Close(); err != nil {
			t.Fatal(err)
		}
		s.schema.run = nil
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != s.retained {
		t.Fatalf("unowned traversal scratch: gauge=%d retained=%d", used, s.retained)
	}
	s.cleanup()
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("cleanup retained %d", used)
	}
}

func TestSchemaBindingScratchEveryCancellationReleasesOwnership(t *testing.T) {
	probe := schemaScratchFixture(t, 3, "string", Budgets{})
	ctx := &schemaScratchCancelContext{Context: context.Background()}
	if err := probe.bindSourceSchema(ctx); err != nil {
		t.Fatal(err)
	}
	schemaScratchAssertReleased(t, probe)
	for at := 1; at <= ctx.calls; at++ {
		s := schemaScratchFixture(t, 3, "string", Budgets{})
		stop := &schemaScratchCancelContext{Context: context.Background(), at: at}
		if err := s.bindSourceSchema(stop); !errors.Is(err, context.Canceled) {
			t.Fatalf("poll %d/%d: %v", at, ctx.calls, err)
		}
		schemaScratchAssertReleased(t, s)
	}
	t.Logf("all %d cancellation polls release traversal scratch", ctx.calls)
}

func TestSchemaBindingScratchKnownFailureSurvivesInterruption(t *testing.T) {
	s := schemaScratchFixture(t, 3, "integer", Budgets{})
	ctx := &schemaScratchCancelContext{Context: context.Background(), when: func() bool { return s.accumulator.report.Counts.Fail > 0 }}
	err := s.bindSourceSchema(ctx)
	if !errors.Is(err, context.Canceled) || s.accumulator.report.Counts.Fail == 0 {
		t.Fatalf("proved scalar failure lost on cancellation: %v / %+v", err, s.accumulator.report)
	}
	// The public finalizer must preserve the proof and the interruption cause,
	// even though binding's traversal storage has already been released.
	report, finishErr := s.Finish(context.Background(), Completion{Syntax: SyntaxIncomplete, Cause: err})
	if !errors.Is(finishErr, context.Canceled) || report.Counts.Fail == 0 || report.Conformance != ConformanceViolated || report.AssessmentComplete {
		t.Fatalf("finalization lost interrupted proof: %v / %+v", finishErr, report)
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("Finish retained %d", used)
	}
}

func TestSchemaBindingScratchRuleStepBoundary(t *testing.T) {
	probe := schemaScratchFixture(t, 3, "string", Budgets{})
	if err := probe.bindSourceSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	steps, _ := probe.account.Usage(BudgetRuleSteps)
	schemaScratchAssertReleased(t, probe)
	for _, short := range []int64{0, 1} {
		s := schemaScratchFixture(t, 3, "string", Budgets{MaxRuleSteps: steps - short})
		err := s.bindSourceSchema(context.Background())
		if short == 0 && err != nil || short != 0 && !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("short=%d: %v", short, err)
		}
		schemaScratchAssertReleased(t, s)
	}
	t.Logf("exact %d / one-short rule-step boundary", steps)
}

func TestSchemaBindingScratchBufferedBoundary(t *testing.T) {
	// Find the whole generated traversal's admission threshold without relying
	// on architecture-specific Run sizes, then verify exact and one-short.
	low, high := int64(1), int64(1<<20)
	for low < high {
		middle := low + (high-low)/2
		s := schemaScratchFixture(t, 3, "string", Budgets{})
		used, _ := s.account.Usage(BudgetBufferedBytes)
		occupancy := s.account.Limits().MaxBufferedBytes - used - middle
		if err := s.account.Charge(BudgetBufferedBytes, occupancy); err != nil {
			t.Fatal(err)
		}
		err := s.bindSourceSchema(context.Background())
		if err != nil && !errors.Is(err, ErrResourceLimit) {
			t.Fatal(err)
		}
		if err := s.account.Release(BudgetBufferedBytes, occupancy); err != nil {
			t.Fatal(err)
		}
		schemaScratchAssertReleased(t, s)
		if err != nil {
			low = middle + 1
		} else {
			high = middle
		}
	}
	for _, short := range []int64{0, 1} {
		s := schemaScratchFixture(t, 3, "string", Budgets{})
		used, _ := s.account.Usage(BudgetBufferedBytes)
		occupancy := s.account.Limits().MaxBufferedBytes - used - low + short
		if err := s.account.Charge(BudgetBufferedBytes, occupancy); err != nil {
			t.Fatal(err)
		}
		err := s.bindSourceSchema(context.Background())
		if short == 0 && err != nil || short != 0 && !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("short=%d: %v", short, err)
		}
		if err := s.account.Release(BudgetBufferedBytes, occupancy); err != nil {
			t.Fatal(err)
		}
		schemaScratchAssertReleased(t, s)
	}
	t.Logf("exact %d / one-short additional buffered-byte boundary", low)
}

func TestSchemaBindingScratchKnownFailureSurvivesBufferRefusal(t *testing.T) {
	s := schemaScratchFixture(t, 3, "integer", Budgets{})
	var occupancy int64
	ctx := &schemaScratchCancelContext{Context: context.Background(), when: func() bool {
		if occupancy == 0 && s.accumulator.report.Counts.Fail > 0 {
			used, _ := s.account.Usage(BudgetBufferedBytes)
			occupancy = s.account.Limits().MaxBufferedBytes - used
			if err := s.account.Charge(BudgetBufferedBytes, occupancy); err != nil {
				t.Fatal(err)
			}
		}
		return false
	}}
	err := s.bindSourceSchema(ctx)
	if !errors.Is(err, ErrResourceLimit) || s.accumulator.report.Counts.Fail == 0 {
		t.Fatalf("proved scalar failure lost on buffer refusal: %v / %+v", err, s.accumulator.report)
	}
	if err := s.account.Release(BudgetBufferedBytes, occupancy); err != nil {
		t.Fatal(err)
	}
	report, finishErr := s.Finish(context.Background(), Completion{Syntax: SyntaxIncomplete, Cause: err})
	if !errors.Is(finishErr, ErrResourceLimit) || report.Counts.Fail == 0 || report.Conformance != ConformanceViolated || report.AssessmentComplete {
		t.Fatalf("finalization lost refused proof: %v / %+v", finishErr, report)
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("Finish retained %d", used)
	}
}

func TestSchemaBindingScratchReleaseBeforeCompletionAssignment(t *testing.T) {
	a, _ := NewBudgetAccount(Budgets{})
	w := schemaBindScratch{account: a}
	if err := w.push(context.Background(), schemaBindFrame{}); err != nil {
		t.Fatal(err)
	}
	if err := w.reserve(3, schemaBindCompletionBytes); err != nil {
		t.Fatal(err)
	}
	// The aggregate lease is authoritative even if an interruption occurs
	// before the completion backing slice is assigned to its owning frame.
	if w.frames[0].children != nil {
		t.Fatal("unexpected completion slice")
	}
	w.release()
	if used, _ := a.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("unassigned admission retained %d", used)
	}
}

func TestSchemaBindingScratchSkippedWitnessUsesCompletionCapacity(t *testing.T) {
	s := schemaScratchFixture(t, 2, "string", Budgets{})
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[SchemaName]*SchemaElement{
		{Local: "root"}: {Name: SchemaName{Local: "root"}, Type: SchemaTypeRef{InlineComplex: &SchemaComplex{
			Form: SchemaComplexDirect,
			Particle: &SchemaParticle{Kind: SchemaParticleSequence, Occurrence: SchemaOccurrence{Min: 1, Max: 1}, Children: []*SchemaParticle{
				{Kind: SchemaParticleElement, Occurrence: SchemaOccurrence{Min: 1, Max: 1}, Element: &SchemaElement{Name: SchemaName{Local: "child"}, Type: SchemaTypeRef{Name: SchemaName{Space: xsd.XSDNamespace, Local: "string"}}}},
				{Kind: SchemaParticleAny, Occurrence: SchemaOccurrence{Min: 1, Max: 1}, Any: &SchemaAny{Namespace: "##any", ProcessContents: "skip"}},
			}},
		}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	s.schema.active = schema
	before := s.retained
	if err := s.bindSourceSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	witnesses := s.schema.run.Witnesses(s.schema.nodes[1].binding)
	if len(witnesses) != 2 || witnesses[0].Mode != xsd.WitnessDeclared || witnesses[1].Mode != xsd.WitnessSkip || !s.schema.nodes[3].skipped {
		t.Fatalf("mixed witnesses lost: %+v", witnesses)
	}
	// Map for three nodes, subject projections for root+declared child,
	// root's two child views, and the declared child's retained scalar.
	want := before + 128 + 3*128 + 2*256 + 2*64 + 129
	if s.retained != want {
		t.Fatalf("completion capacity retained after skip: got %d want %d", s.retained, want)
	}
	schemaScratchAssertReleased(t, s)
}
