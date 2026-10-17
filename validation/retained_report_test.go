package validation

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestRetainedReportLeaseCopylockMarker(t *testing.T) {
	typ := reflect.TypeFor[ReportLease]()
	marker, ok := typ.FieldByName("noCopy")
	if !ok || marker.Anonymous || marker.Type.Size() != 0 || !reflect.PointerTo(marker.Type).Implements(reflect.TypeFor[sync.Locker]()) || marker.Type.Implements(reflect.TypeFor[sync.Locker]()) {
		t.Fatal("lease must carry a non-embedded zero-size pointer-Locker marker for go vet copylocks")
	}
	// Keeping the zero-size marker first avoids trailing-zero-field padding.
	plain := reflect.TypeFor[struct {
		account  *Account
		retained int64
		closed   bool
	}]()
	if typ.Size() != plain.Size() {
		t.Fatalf("copylock marker changed lease size: %d want %d", typ.Size(), plain.Size())
	}
}

func retainedReportTestSession(t *testing.T, text string) (*Session, int64) {
	t.Helper()
	s := snapshotTestSession(t, text)
	before := s.retained
	row := ContextResult{
		Provider: RuleKey{"retained-report", 1}, Subject: NodeRef{"source", 1},
		Property: ContextPropertySpanNormalDots,
		Value:    Evidence[Atomic]{State: EvidenceUnknown, Reason: "optional-gap", Cause: &DecisionRef{Rule: RuleKey{"cause", 1}, Subject: NodeRef{"source", 1}}},
		Origins:  []Location{{Target: TargetSource, DocumentID: "source", Path: "/root", Position: &SourcePosition{CoordinateSpace: "xml", Line: 1, Column: 1}}},
	}
	if err := s.publishContextResult(context.Background(), row); err != nil {
		t.Fatal(err)
	}
	return s, s.retained - before
}

func TestRetainedReportTransfersAlreadyAdmittedContext(t *testing.T) {
	s, contextBytes := retainedReportTestSession(t, "3")
	original := s.contextResults
	report, lease, err := s.FinishWithRetainedReport(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil || lease == nil || !report.Satisfied() || len(report.Context) != 1 {
		t.Fatalf("report=%+v lease=%v err=%v", report, lease, err)
	}
	if &report.Context[0] != &original[0] {
		t.Fatal("retained finalizer copied the already-owned projection")
	}
	if s.retained != 0 || s.contextReportRetained != 0 || s.contextResults != nil || s.nodes != nil {
		t.Fatal("source session still owns transferred or intrinsic state")
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != contextBytes+reportLeaseControlBytes {
		t.Fatalf("live charge=%d want context/control=%d", used, contextBytes+reportLeaseControlBytes)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("closed lease leaked %d", used)
	}
	if report.Context[0].Value.Cause.Rule.ID != "cause" {
		t.Fatal("closing lease damaged caller-owned report")
	}
	if _, second, err := s.FinishWithRetainedReport(context.Background(), Completion{}); !errors.Is(err, ErrInvalidSession) || second != nil {
		t.Fatalf("second finish lease=%v err=%v", second, err)
	}
}

func TestRetainedReportClosePreservesCumulativeDiagnosticQuotas(t *testing.T) {
	s, _ := retainedReportTestSession(t, "bad")
	report, lease, err := s.FinishWithRetainedReport(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil || lease == nil || report.Counts.Fail == 0 || len(report.Findings) == 0 {
		t.Fatalf("fixture must retain a failing diagnostic: report=%+v lease=%v err=%v", report, lease, err)
	}
	defer lease.Close()
	kinds := []BudgetKind{BudgetDiagnostics, BudgetDiagnosticBytes}
	before := make([]int64, len(kinds))
	for i, kind := range kinds {
		before[i], err = s.account.Usage(kind)
		if err != nil || before[i] <= 0 {
			t.Fatalf("fixture needs nonzero cumulative %s before first Close: usage=%d err=%v", kind, before[i], err)
		}
	}
	for closeNumber := 1; closeNumber <= 2; closeNumber++ {
		if err := lease.Close(); err != nil {
			t.Fatal(err)
		}
		for i, kind := range kinds {
			after, err := s.account.Usage(kind)
			if err != nil || after != before[i] {
				t.Errorf("Close %d changed cumulative %s: before=%d after=%d err=%v", closeNumber, kind, before[i], after, err)
			}
		}
	}
}

func TestRetainedReportLeavesOrdinaryFinishUnchanged(t *testing.T) {
	a, _ := retainedReportTestSession(t, "bad")
	b, _ := retainedReportTestSession(t, "bad")
	plain, plainErr := a.Finish(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	retained, lease, retainedErr := b.FinishWithRetainedReport(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	defer lease.Close()
	if plainErr != nil || retainedErr != nil || !reflect.DeepEqual(plain, retained) {
		t.Fatalf("changed report: plain=%+v retained=%+v errors=%v/%v", plain, retained, plainErr, retainedErr)
	}
	if used, _ := a.account.Usage(BudgetBufferedBytes); used != 0 || a.contextReportRetained != 0 {
		t.Fatalf("ordinary Finish leaked %d", used)
	}
}

func TestRetainedReportEmptyContextNeedsNoLease(t *testing.T) {
	s := snapshotTestSession(t, "3")
	report, lease, err := s.FinishWithRetainedReport(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil || lease != nil || !report.Satisfied() {
		t.Fatalf("report=%+v lease=%v err=%v", report, lease, err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("empty report leaked %d", used)
	}
}

func TestRetainedReportAdmissionFailureKeepsKnownViolation(t *testing.T) {
	s, _ := retainedReportTestSession(t, "bad")
	used, _ := s.account.Usage(BudgetBufferedBytes)
	hold := s.account.Limits().MaxBufferedBytes - used
	if err := s.account.Charge(BudgetBufferedBytes, hold); err != nil {
		t.Fatal(err)
	}
	report, lease, err := s.FinishWithRetainedReport(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if !errors.Is(err, ErrResourceLimit) || lease != nil || report.Counts.Fail != 1 || report.Conformance != ConformanceViolated || report.AssessmentComplete || report.Terminal == nil || len(report.Context) != 1 {
		t.Fatalf("report=%+v lease=%v err=%v", report, lease, err)
	}
	if err := s.account.Release(BudgetBufferedBytes, hold); err != nil {
		t.Fatal(err)
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("admission refusal leaked %d", used)
	}
}

func TestRetainedReportAdmissionExactBoundary(t *testing.T) {
	for _, spare := range []int64{reportLeaseControlBytes - 1, reportLeaseControlBytes} {
		s, contextBytes := retainedReportTestSession(t, "3")
		used, _ := s.account.Usage(BudgetBufferedBytes)
		hold := s.account.Limits().MaxBufferedBytes - used - spare
		if err := s.account.Charge(BudgetBufferedBytes, hold); err != nil {
			t.Fatal(err)
		}
		report, lease, err := s.FinishWithRetainedReport(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
		if spare < reportLeaseControlBytes {
			if !errors.Is(err, ErrResourceLimit) || lease != nil || report.Satisfied() {
				t.Fatalf("one-under: report=%+v lease=%v err=%v", report, lease, err)
			}
		} else if err != nil || lease == nil || !report.Satisfied() {
			t.Fatalf("exact limit: report=%+v lease=%v err=%v", report, lease, err)
		} else if used, _ := s.account.Usage(BudgetBufferedBytes); used != hold+contextBytes+reportLeaseControlBytes {
			t.Fatalf("transferred contexts were free or double-charged: %d", used)
		}
		if err := lease.Close(); err != nil {
			t.Fatal(err)
		}
		if err := s.account.Release(BudgetBufferedBytes, hold); err != nil {
			t.Fatal(err)
		}
		if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("boundary leaked %d", used)
		}
	}
}

func TestRetainedReportPartialErrorsKeepLease(t *testing.T) {
	transport := errors.New("retained report transport failure")
	for _, canceled := range []bool{false, true} {
		s, _ := retainedReportTestSession(t, "bad")
		ctx := context.Background()
		if canceled {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			cancel()
		}
		report, lease, err := s.FinishWithRetainedReport(ctx, Completion{Syntax: SyntaxIncomplete, Cause: transport})
		if !errors.Is(err, transport) || canceled && !errors.Is(err, context.Canceled) || lease == nil || report.AssessmentComplete || len(report.Context) != 1 {
			t.Fatalf("canceled=%v report=%+v lease=%v err=%v", canceled, report, lease, err)
		}
		if used, _ := s.account.Usage(BudgetBufferedBytes); used == 0 {
			t.Fatal("partial report lost live ownership")
		}
		if err := lease.Close(); err != nil {
			t.Fatal(err)
		}
		if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("partial report leaked %d", used)
		}
	}
}

func TestRetainedReportProjectionCancellationRestoresSubtotal(t *testing.T) {
	row := ContextResult{Provider: RuleKey{"retained-report", 1}, Subject: NodeRef{"source", 1}, Property: ContextPropertySpanNormalDots, Value: Evidence[Atomic]{State: EvidenceAbsent}, Origins: []Location{{Target: TargetSource, DocumentID: "source", Path: "/root"}}}
	probe := rationalSession(t, Budgets{})
	counter := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	if err := probe.publishContextResult(counter, row); err != nil {
		t.Fatal(err)
	}
	probe.cleanup()
	for at := 1; at <= counter.calls; at++ {
		s, _ := retainedReportTestSession(t, "3")
		before, retained := s.contextReportRetained, s.retained
		err := s.publishContextResult(&rationalCancelContext{Context: context.Background(), at: at}, row)
		if !errors.Is(err, context.Canceled) || s.contextReportRetained != before || s.retained != retained || len(s.contextResults) != 1 {
			t.Fatalf("at %d: err=%v subtotal=%d/%d retained=%d/%d", at, err, s.contextReportRetained, before, s.retained, retained)
		}
		s.cleanup()
	}
}

func TestRetainedReportSpanRollbackRestoresSubtotal(t *testing.T) {
	r := spanUnitReader(t)
	account, err := NewBudgetAccount(Budgets{MaxContextFacts: 2})
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
	if err := r.s.publishContextResult(context.Background(), row); err != nil {
		t.Fatal(err)
	}
	before := r.s.contextReportRetained
	_, _, err = evaluateContextSpans(context.Background(), r.s, note, RuleKey{"spans", 1}, &contextPlan{Bindings: r.b}, true)
	if !errors.Is(err, ErrResourceLimit) || len(r.s.contextResults) != 1 || r.s.contextReportRetained != before {
		t.Fatalf("span rollback err=%v rows=%d subtotal=%d/%d", err, len(r.s.contextResults), r.s.contextReportRetained, before)
	}
	if used, _ := account.Usage(BudgetContextFacts); used != 2 {
		t.Fatalf("fixture did not publish before rollback: context facts=%d", used)
	}
}
