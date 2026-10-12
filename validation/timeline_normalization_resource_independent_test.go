package validation

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"testing"
	"unsafe"
)

func independentNormalizationStage(t *testing.T) (*Session, *timelineService, *timelineService, *timelineBindings, []timelineMeasureBlock) {
	t.Helper()
	s, root, b := runtimeFixture(t)
	attrs := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
	blocks := []timelineMeasureBlock{runtimeBlock(b, 0, attrs, runtimeNote(t, b, 30, 1, false))}
	if err := root.readMeasure(context.Background(), s, b, blocks[0]); err != nil {
		t.Fatal(err)
	}
	// This private stopped stage has a newly completed history whose actual
	// invalid publisher invalidates the previously normalized note.
	stage := *root
	cause := &DecisionRef{Rule: RuleKey{"independent-normalization", 1}, Subject: NodeRef{"d", 77}}
	if err := addStateGap(context.Background(), s, &stage.State.Divisions, stateGap{Part: NodeRef{"d", 1}, FirstMeasure: 0, LastMeasure: 0, Scope: stateScope{AllStaves: true}, Availability: Evidence[bool]{State: EvidenceInvalid, Reason: "actual-unit-cause", Cause: cause}}); err != nil {
		t.Fatal(err)
	}
	if _, err := stateIndex(context.Background(), s, &stage.State.Divisions); err != nil {
		t.Fatal(err)
	}
	return s, root, &stage, b, blocks
}

func TestIndependentTimelineNormalizationGateAdmissionAndOwnership(t *testing.T) {
	if need := 4*int64(unsafe.Sizeof(NodeID(0))+unsafe.Sizeof(timelineNormalizationGate{})) + 128; need > timelineNormalizationGateBytes {
		t.Fatalf("gate map/cause shape%d exceeds%d", need, timelineNormalizationGateBytes)
	}
	s, root, stage, b, blocks := independentNormalizationStage(t)
	originalCause := stage.State.Divisions.Gaps[0].Availability.Cause
	closed, err := root.closeUnitStage(context.Background(), s, b, blocks, nil, nil, nil, stage)
	if err != nil {
		t.Fatal(err)
	}
	gate, ok := closed.unitNormalizations[30]
	if !ok || gate.Restored || gate.Units.State != EvidenceInvalid || gate.Units.Cause == nil || *gate.Units.Cause != *originalCause || gate.Units.Cause == originalCause {
		t.Fatalf("gate is missing or borrowed: %+v", gate)
	}
	originalCause.Subject.Node = 999
	if gate.Units.Cause.Subject.Node != 77 || closed.Facts[30].Duration.State == EvidenceKnown {
		t.Fatal("gate lost cause ownership or failed to stop normalization")
	}
	if len(root.unitNormalizations) != 0 || root.Facts[30].Duration.State != EvidenceKnown {
		t.Fatal("private closure mutated original service")
	}
	// The rebuilt stopped stage has a known independent origin and its own
	// unchanged D=1 publisher. Restoration uses that stage, never its unknown end.
	before := rationalUsage(t, s, BudgetBufferedBytes)
	n, err := closed.restoreNormalizations(context.Background(), s, blocks)
	if err != nil || n != 1 || !closed.unitNormalizations[30].Restored {
		t.Fatalf("direct restoration%d: %v", n, err)
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != before {
		t.Fatal("known restoration retained a redundant gate or result")
	}
	n, err = closed.restoreNormalizations(context.Background(), s, blocks)
	if err != nil || n != 0 {
		t.Fatalf("normalization restored more than once: %d %v", n, err)
	}
	final, err := root.rebuildUnitStage(context.Background(), s, b, blocks, nil, nil, closed.unitNormalizations)
	if err != nil || final.Facts[30].Duration.State != EvidenceKnown || final.Facts[30].Duration.Value.String() != "1" {
		t.Fatalf("restored rebuild: %v %+v", err, final)
	}
	s.cleanup()
	if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatal("normalization cleanup leaked")
	}

	s, root, stage, b, blocks = independentNormalizationStage(t)
	// Isolate gate admission from the larger rational comparison scratch peak:
	// the completed history has an actual invalid gap and no selected baseline.
	stage.State.Divisions.Publications = nil
	stage.State.Divisions.index = nil
	if _, err := stateIndex(context.Background(), s, &stage.State.Divisions); err != nil {
		t.Fatal(err)
	}
	before = rationalUsage(t, s, BudgetBufferedBytes)
	if _, err := stage.State.effectiveDivisionsEvidence(context.Background(), s, stage.Facts[30].Origin.Value); err != nil {
		t.Fatal(err)
	}
	queryBytes := rationalUsage(t, s, BudgetBufferedBytes) - before
	held := s.account.Limits().MaxBufferedBytes - rationalUsage(t, s, BudgetBufferedBytes) - queryBytes - timelineNormalizationGateBytes + 1
	if err := s.account.Charge(BudgetBufferedBytes, held); err != nil {
		t.Fatal(err)
	}
	_, err = root.closeUnitStage(context.Background(), s, b, blocks, nil, nil, nil, stage)
	var resource *ResourceError
	if !errors.As(err, &resource) || resource.Kind != BudgetBufferedBytes || resource.Requested != timelineNormalizationGateBytes || len(stage.unitNormalizations) != 0 {
		t.Fatalf("normalization map allocated before512-byte admission: %v", err)
	}
	_ = s.account.Release(BudgetBufferedBytes, held)
	if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
		t.Fatal("failed normalization leaked scratch")
	}
	s.cleanup()
}

func independentNormalizationExcursion(t *testing.T) (*Session, *timelineService, *timelineBindings, []timelineMeasureBlock) {
	t.Helper()
	s, root, b := runtimeFixture(t)
	attrs := func(id NodeID, units int64) *factNode {
		return stateNode(b, timelineRoleAttributes, id, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, id+1, stateAtomicDecimal(t, units, 1)))
	}
	backup := stateNode(b, timelineRoleBackup, 60, stateAbsent[Atomic](), stateNode(b, timelineRoleBackupDuration, 61, stateAtomicDecimal(t, 6, 1)))
	blocks := []timelineMeasureBlock{runtimeBlock(b, 0, attrs(20, 1), runtimeNote(t, b, 30, 1, false), runtimeNote(t, b, 40, 1, false)), runtimeBlock(b, 1, attrs(50, 3), backup, attrs(70, 2))}
	for _, block := range blocks {
		if err := root.readMeasure(context.Background(), s, b, block); err != nil {
			t.Fatal(err)
		}
	}
	return s, root, b, blocks
}

func TestIndependentTimelineNormalizationClosureEveryCancellationBoundary(t *testing.T) {
	s, root, b, blocks := independentNormalizationExcursion(t)
	probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	if err := root.finalizeExcursionUnits(probe, s, b, blocks); err != nil {
		t.Fatal(err)
	}
	if len(root.unitNormalizations) == 0 || root.Facts[30].Duration.State == EvidenceKnown || root.Facts[40].Position.State == EvidenceKnown {
		t.Fatal("fixture did not exercise completed-history normalization gates")
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
		t.Fatal("completed closure leaked scratch")
	}
	s.cleanup()
	for poll := 1; poll <= probe.calls; poll++ {
		s, root, b, blocks := independentNormalizationExcursion(t)
		snapshot := *root
		facts := maps.Clone(root.Facts)
		bounds := maps.Clone(root.State.frameBounds)
		err := root.finalizeExcursionUnits(&rationalCancelContext{Context: context.Background(), at: poll}, s, b, blocks)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("normalization closure poll%d: %v", poll, err)
		}
		if !reflect.DeepEqual(*root, snapshot) || !reflect.DeepEqual(root.Facts, facts) || !reflect.DeepEqual(root.State.frameBounds, bounds) {
			t.Fatalf("poll%d mutated original service", poll)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatalf("poll%d leaked scratch", poll)
		}
		if poll == probe.calls || poll%509 == 0 {
			if err := root.finalizeExcursionUnits(context.Background(), s, b, blocks); err != nil {
				t.Fatalf("retry poll%d: %v", poll, err)
			}
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatalf("poll%d cleanup leaked", poll)
		}
	}
	t.Logf("all%d complete-history normalization lifecycle cancellation polls", probe.calls)
}

func TestIndependentTimelineNormalizationRebuildContextQuota(t *testing.T) {
	s, root, b, blocks := independentNormalizationExcursion(t)
	defer s.cleanup()
	snapshot := *root
	facts := maps.Clone(root.Facts)
	left := s.account.Limits().MaxContextFacts - rationalUsage(t, s, BudgetContextFacts) - 1
	if err := s.account.Charge(BudgetContextFacts, left); err != nil {
		t.Fatal(err)
	}
	err := root.finalizeExcursionUnits(context.Background(), s, b, blocks)
	var resource *ResourceError
	if !errors.As(err, &resource) || resource.Kind != BudgetContextFacts {
		t.Fatalf("stage publication bypassed shared context quota: %v", err)
	}
	if !reflect.DeepEqual(*root, snapshot) || !reflect.DeepEqual(root.Facts, facts) {
		t.Fatal("context stop partially swapped original service")
	}
	if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
		t.Fatal("context stop leaked scratch")
	}
}

func TestIndependentTimelineNormalizationRestorationEveryCancellationBoundary(t *testing.T) {
	fixture := func() (*Session, *timelineService, []timelineMeasureBlock) {
		s, root, stage, b, blocks := independentNormalizationStage(t)
		closed, err := root.closeUnitStage(context.Background(), s, b, blocks, nil, nil, nil, stage)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := stateIndex(context.Background(), s, &closed.State.Divisions); err != nil {
			t.Fatal(err)
		}
		return s, closed, blocks
	}
	s, stage, blocks := fixture()
	probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	if n, err := stage.restoreNormalizations(probe, s, blocks); err != nil || n != 1 {
		t.Fatalf("direct restoration%d: %v", n, err)
	}
	s.cleanup()
	for poll := 1; poll <= probe.calls; poll++ {
		s, stage, blocks := fixture()
		gates := maps.Clone(stage.unitNormalizations)
		for id, gate := range gates {
			gates[id] = stateClone(gate)
		}
		facts := maps.Clone(stage.Facts)
		_, err := stage.restoreNormalizations(&rationalCancelContext{Context: context.Background(), at: poll}, s, blocks)
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(gates, stage.unitNormalizations) || !reflect.DeepEqual(facts, stage.Facts) {
			t.Fatalf("restoration poll%d: %v", poll, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatalf("restoration poll%d leaked scratch", poll)
		}
		if n, err := stage.restoreNormalizations(context.Background(), s, blocks); err != nil || n != 1 {
			t.Fatalf("retry restoration poll%d: %d %v", poll, n, err)
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("restoration cleanup leaked")
		}
	}
	t.Logf("all%d direct normalization restoration cancellation polls", probe.calls)
}
