package validation

import (
	"context"
	"errors"
	"testing"
)

func TestSchemaAttributeScratchExactMapAdmission(t *testing.T) {
	for _, short := range []int64{0, 1} {
		s, n, u, b := schemaAttributeScratchFixture(t)
		before := s.retained
		used, _ := s.account.Usage(BudgetBufferedBytes)
		occupancy := s.account.Limits().MaxBufferedBytes - used - schemaAttributeScratchRootRecordBytes - int64(len(n.attributes))*96 + short
		if err := s.account.Charge(BudgetBufferedBytes, occupancy); err != nil {
			t.Fatal(err)
		}
		rootBefore := n.end.Scalar.Value
		// Two exact header-cache lookups precede the original map phase.
		ctx := &schemaScratchCancelContext{Context: context.Background(), at: 3}
		err := s.publishSchemaFacts(ctx, n, u, b)
		want := error(context.Canceled)
		if short != 0 {
			want = ErrResourceLimit
		}
		if !errors.Is(err, want) {
			t.Fatalf("short=%d: %v", short, err)
		}
		wantPolls := 3 - int(short)
		if ctx.calls != wantPolls || schemaAttributeScratchRootReplacement(t, n, rootBefore) != schemaAttributeScratchRootRecordBytes {
			t.Fatalf("short=%d: did not reach exact map admission after root evidence (polls %d, want %d)", short, ctx.calls, wantPolls)
		}
		if s.retained != before+schemaAttributeScratchRootRecordBytes {
			t.Fatal("temporary index became session-owned beyond the real root evidence")
		}
		if err := s.account.Release(BudgetBufferedBytes, occupancy); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.account.Usage(BudgetBufferedBytes); got != used+schemaAttributeScratchRootRecordBytes {
			t.Fatalf("refused/canceled map still charged: %d, want root-only delta %d", got, used+schemaAttributeScratchRootRecordBytes)
		}
		schemaScratchAssertReleased(t, s)
	}
}

func TestSchemaAttributeScratchEveryCancellationKeepsPublishedEvidence(t *testing.T) {
	probe, n, u, b := schemaAttributeScratchFixture(t)
	ctx := &schemaScratchCancelContext{Context: context.Background()}
	if err := probe.publishSchemaFacts(ctx, n, u, b); err != nil {
		t.Fatal(err)
	}
	schemaScratchAssertReleased(t, probe)
	for at := 1; at <= ctx.calls; at++ {
		s, n, u, b := schemaAttributeScratchFixture(t)
		before := s.retained
		used, _ := s.account.Usage(BudgetBufferedBytes)
		rootBefore := n.end.Scalar.Value
		stop := &schemaScratchCancelContext{Context: context.Background(), at: at}
		err := s.publishSchemaFacts(stop, n, u, b)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("poll %d/%d: %v", at, ctx.calls, err)
		}
		published := schemaAttributeScratchRootReplacement(t, n, rootBefore)
		for _, a := range n.attributes {
			if a.Scalar.Value.State == EvidenceKnown {
				published += 128 + int64(len(a.Scalar.Value.Value.Text))
			}
		}
		if s.retained-before != published {
			t.Fatalf("poll %d changed root/attribute evidence accounting: %d want %d", at, s.retained-before, published)
		}
		if got, _ := s.account.Usage(BudgetBufferedBytes); got != used+published {
			t.Fatalf("poll %d retained local index: %d want %d", at, got, used+published)
		}
		schemaScratchAssertReleased(t, s)
	}
	t.Logf("all %d index/publication polls release the map and retain only published root/attribute evidence", ctx.calls)
}

func TestSchemaAttributeScratchWorkBoundary(t *testing.T) {
	probe, n, u, b := schemaAttributeScratchFixture(t)
	before, _ := probe.account.Usage(BudgetRuleSteps)
	if err := probe.publishSchemaFacts(context.Background(), n, u, b); err != nil {
		t.Fatal(err)
	}
	after, _ := probe.account.Usage(BudgetRuleSteps)
	steps := after - before
	schemaScratchAssertReleased(t, probe)
	for _, short := range []int64{0, 1} {
		s, n, u, b := schemaAttributeScratchFixture(t)
		used, _ := s.account.Usage(BudgetRuleSteps)
		if err := s.account.Charge(BudgetRuleSteps, s.account.Limits().MaxRuleSteps-used-steps+short); err != nil {
			t.Fatal(err)
		}
		err := s.publishSchemaFacts(context.Background(), n, u, b)
		if short == 0 && err != nil || short != 0 && !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("short=%d: %v", short, err)
		}
		schemaScratchAssertReleased(t, s)
	}
}

func TestSchemaAttributeScratchRefusalPreservesPriorFailure(t *testing.T) {
	s, n, u, b := schemaAttributeScratchFixture(t, true)
	s.schema.bound = true
	s.schema.nodes = map[NodeID]schemaNode{1: {binding: b, available: true}}
	if err := s.recordXSDProvedFailures(context.Background(), n); err != nil {
		t.Fatal(err)
	}
	if s.accumulator.report.Counts.Fail == 0 {
		t.Fatal("fixture did not prove invalid attribute")
	}
	used, _ := s.account.Usage(BudgetBufferedBytes)
	occupancy := s.account.Limits().MaxBufferedBytes - used - schemaAttributeScratchRootRecordBytes - int64(len(n.attributes))*96 + 1
	if err := s.account.Charge(BudgetBufferedBytes, occupancy); err != nil {
		t.Fatal(err)
	}
	err := s.publishSchemaFacts(context.Background(), n, u, b)
	if !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("index admission: %v", err)
	}
	if err := s.account.Release(BudgetBufferedBytes, occupancy); err != nil {
		t.Fatal(err)
	}
	report, finishErr := s.Finish(context.Background(), Completion{Syntax: SyntaxIncomplete, Cause: err})
	if !errors.Is(finishErr, ErrResourceLimit) || report.Counts.Fail == 0 || report.Conformance != ConformanceViolated || report.AssessmentComplete {
		t.Fatalf("failure/cause erased: %+v / %v", report, finishErr)
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("Finish retained %d", used)
	}
}
