package validation

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"strings"
	"testing"
	"unsafe"
)

func TestIndependentTimelineExcursionPartHashWorkAdmission(t *testing.T) {
	s := rationalSession(t, Budgets{MaxRuleSteps: 64})
	defer s.cleanup()
	part := NodeRef{DocumentID: strings.Repeat("document-id-", 1024), Node: 1}
	unavailable := stateUnknown[Decimal]("missing-divisions")
	v := &timelineService{Facts: map[NodeID]timelineEventFact{9: {Units: unavailable}}, unitExcursions: []timelineUnitExcursion{{Node: 9, Part: part, Raw: stateKnown(stateDecimal(t, 1, 1)), Locked: true, Availability: unavailable}}}
	err := v.finalizeExcursionUnits(context.Background(), s, stateBindings(), nil)
	if !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("large part hash completed with only64 work units: %v (work%d)", err, rationalUsage(t, s, BudgetRuleSteps))
	}
	if v.unitProofFinal || len(v.Facts) != 1 {
		t.Fatal("failed proof partially replaced original service")
	}
}

func TestIndependentTimelineExcursionClosedShape(t *testing.T) {
	checks := []struct {
		name           string
		need, admitted int64
	}{
		{"excursion", 3 * int64(unsafe.Sizeof(timelineUnitExcursion{})), timelineExcursionReserveBytes},
		{"publisher", int64(unsafe.Sizeof(timelineUnitPublisher{})), timelineUnitPublisherBytes},
		{"gate", 4 * int64(unsafe.Sizeof(NodeID(0))+unsafe.Sizeof(timelineExcursionGate{})), timelineUnitGateBytes},
		{"positive", 4 * int64(unsafe.Sizeof(NodeRef{})+unsafe.Sizeof(bool(false))), timelineUnitGateBytes},
		{"stage", int64(unsafe.Sizeof(timelineService{})) + 512, timelineUnitStageBytes},
	}
	for _, c := range checks {
		if c.need > c.admitted {
			t.Errorf("%s shape%d exceeds%d", c.name, c.need, c.admitted)
		}
		t.Logf("%s shape%d allowance%d", c.name, c.need, c.admitted)
	}
	var items []timelineUnitExcursion
	for i := int64(1); i <= 4096; i++ {
		old := cap(items)
		items = append(items, timelineUnitExcursion{})
		peak := cap(items)
		if peak != old {
			peak += old
		}
		if int64(peak)*int64(unsafe.Sizeof(timelineUnitExcursion{})) > i*timelineExcursionReserveBytes {
			t.Fatalf("excursion%d append peak exceeds admitted backing", i)
		}
	}
}

func independentExcursionStageFixture(t *testing.T, restored bool) (*Session, *timelineService, *timelineBindings, []timelineMeasureBlock) {
	t.Helper()
	s, v, b := runtimeFixture(t)
	attrs := func(id NodeID, n int64) *factNode {
		return stateNode(b, timelineRoleAttributes, id, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, id+1, stateAtomicDecimal(t, n, 1)))
	}
	var blocks []timelineMeasureBlock
	if restored {
		backward := stateNode(b, timelineRoleBackup, 50, stateAbsent[Atomic](), stateNode(b, timelineRoleBackupDuration, 51, stateAtomicDecimal(t, 4, 1)))
		blocks = []timelineMeasureBlock{runtimeBlock(b, 0, attrs(20, 1), runtimeNote(t, b, 30, 1, false)), runtimeBlock(b, 1, backward, runtimeNote(t, b, 60, 1, false)), runtimeBlock(b, 2, attrs(80, 2), runtimeNote(t, b, 90, 1, false))}
	} else {
		forward := stateNode(b, timelineRoleForward, 50, stateAbsent[Atomic](), stateNode(b, timelineRoleForwardDuration, 51, stateAtomicDecimal(t, 10, 1)))
		blocks = []timelineMeasureBlock{runtimeBlock(b, 0, attrs(20, 1), runtimeNote(t, b, 30, 1, false), forward, runtimeNote(t, b, 60, 1, false)), runtimeBlock(b, 1, attrs(80, 2), runtimeNote(t, b, 90, 1, false))}
	}
	for _, block := range blocks {
		if err := v.readMeasure(context.Background(), s, b, block); err != nil {
			t.Fatal(err)
		}
	}
	if len(v.unitExcursions) == 0 {
		t.Fatal("fixture has no exceptional unit proof")
	}
	return s, v, b, blocks
}

func TestIndependentTimelineExcursionStageEveryCancellationBoundary(t *testing.T) {
	for _, restored := range []bool{false, true} {
		s, v, b, blocks := independentExcursionStageFixture(t, restored)
		probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
		original := v.Facts
		if err := v.finalizeExcursionUnits(probe, s, b, blocks); err != nil {
			t.Fatal(err)
		}
		if !v.unitProofFinal || reflect.ValueOf(original).Pointer() == reflect.ValueOf(v.Facts).Pointer() {
			t.Fatal("mixed-unit stage was not atomically replaced")
		}
		if known := v.Facts[50].Duration.State == EvidenceKnown; known != restored {
			t.Fatalf("restored%t final movement known%t", restored, known)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatal("completed stage leaked inventory scratch")
		}
		s.cleanup()
		for poll := 1; poll <= probe.calls; poll++ {
			s, v, b, blocks := independentExcursionStageFixture(t, restored)
			snapshot := *v
			facts := maps.Clone(v.Facts)
			frameBounds := maps.Clone(v.State.frameBounds)
			err := v.finalizeExcursionUnits(&rationalCancelContext{Context: context.Background(), at: poll}, s, b, blocks)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("restored%t poll%d: %v", restored, poll, err)
			}
			if !reflect.DeepEqual(*v, snapshot) || !reflect.DeepEqual(v.Facts, facts) || !reflect.DeepEqual(v.State.frameBounds, frameBounds) {
				t.Fatalf("restored%t poll%d partially replaced original service", restored, poll)
			}
			if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
				t.Fatalf("restored%t poll%d leaked scratch", restored, poll)
			}
			if poll == probe.calls || poll%257 == 0 {
				if err := v.finalizeExcursionUnits(context.Background(), s, b, blocks); err != nil {
					t.Fatalf("retry poll%d: %v", poll, err)
				}
			}
			s.cleanup()
			if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatalf("poll%d cleanup leaked", poll)
			}
		}
		t.Logf("restored%t all%d staged reconstruction cancellation polls", restored, probe.calls)
	}
}

func TestIndependentTimelineUnitPublisherScratchAndCancellation(t *testing.T) {
	s, v, b, blocks := independentExcursionStageFixture(t, false)
	_ = v
	before := rationalUsage(t, s, BudgetBufferedBytes)
	probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	publishers, release, err := timelineUnitPublishers(probe, s, b, blocks)
	if err != nil || len(publishers) != 2 {
		t.Fatalf("inventory: %v len%d", err, len(publishers))
	}
	if got := rationalUsage(t, s, BudgetBufferedBytes) - before; got != 2*timelineUnitPublisherBytes {
		t.Fatalf("scratch%d", got)
	}
	release()
	if err = s.account.Charge(BudgetBufferedBytes, 8192); err != nil {
		t.Fatal(err)
	}
	release()
	if got := rationalUsage(t, s, BudgetBufferedBytes); got != before+8192 {
		t.Fatal("repeated release stole unrelated scratch")
	}
	_ = s.account.Release(BudgetBufferedBytes, 8192)
	for poll := 1; poll <= probe.calls; poll++ {
		_, release, err := timelineUnitPublishers(&rationalCancelContext{Context: context.Background(), at: poll}, s, b, blocks)
		release()
		if !errors.Is(err, context.Canceled) || rationalUsage(t, s, BudgetBufferedBytes) != before {
			t.Fatalf("inventory poll%d: %v; scratch retained", poll, err)
		}
	}
	remaining := DefaultBudgets().MaxBufferedBytes - before - (2*timelineUnitPublisherBytes - 1)
	if err = s.account.Charge(BudgetBufferedBytes, remaining); err != nil {
		t.Fatal(err)
	}
	_, release, err = timelineUnitPublishers(context.Background(), s, b, blocks)
	release()
	if !errors.Is(err, ErrResourceLimit) || rationalUsage(t, s, BudgetBufferedBytes) != before+remaining {
		t.Fatalf("inventory allocated before full admission: %v", err)
	}
	_ = s.account.Release(BudgetBufferedBytes, remaining)
	s.cleanup()
	if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatal("inventory cleanup leaked")
	}
	t.Logf("all%d inventory cancellation polls; idempotent release and admission-minus1", probe.calls)
}
