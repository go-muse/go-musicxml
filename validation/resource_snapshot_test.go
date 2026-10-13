package validation

import (
	"context"
	"errors"
	"testing"
)

func snapshotTestSession(t *testing.T, text string, budgets ...Budgets) *Session {
	t.Helper()
	opts := RunOptions{DocumentID: "source", Profile: "source-schema-test"}
	if len(budgets) > 0 {
		opts.Budgets = budgets[0]
	}
	s, err := sourceSchemaEngine(t, XSDCheckScalar).Begin(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Open(context.Background(), schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(context.Background(), schemaLexemeEnd(text)); err != nil {
		t.Fatal(err)
	}
	return s
}
func TestResourceSnapshotFinalizesBeforeCopyAndOwnsReport(t *testing.T) {
	s := snapshotTestSession(t, "not-an-integer")
	r, snapshot, err := s.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil || snapshot == nil {
		t.Fatalf("snapshot=%v err=%v", snapshot, err)
	}
	if r.Counts.Fail != 1 || r.Satisfied() {
		t.Fatalf("lost final intrinsic result: %+v", r)
	}
	canonical, err := snapshot.Report(context.Background())
	if err != nil || canonical.Counts != r.Counts {
		t.Fatalf("canonical=%+v err=%v", canonical, err)
	}
	r.Findings[0].Reason = "edited"
	canonical.Findings[0].Location.Path = "edited"
	again, _ := snapshot.Report(context.Background())
	if again.Findings[0].Reason == "edited" || again.Findings[0].Location.Path == "edited" {
		t.Fatal("snapshot aliases a report")
	}
	if snapshot.Header().DocumentID != "source" || snapshot.Header().Root.Node != 1 {
		t.Fatal("missing exact root")
	}
	if s.nodes != nil || s.accumulator != nil {
		t.Fatal("child was not cleaned")
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used <= 0 {
		t.Fatal("retained report/snapshot became free at child cleanup")
	}
	if _, err = s.Finish(context.Background(), Completion{}); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("second finish=%v", err)
	}
	if err = snapshot.Close(); err != nil {
		t.Fatal(err)
	}
	if err = snapshot.Close(); err != nil {
		t.Fatal(err)
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("leaked %d", used)
	}
	if snapshot.Header() != (ResourceSnapshotHeader{}) || snapshot.ContextCount() != 0 {
		t.Fatal("closed snapshot exposed content")
	}
	if _, err = snapshot.Report(context.Background()); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("closed report=%v", err)
	}
}
func TestResourceSnapshotAdmissionFailurePreservesBestReport(t *testing.T) {
	s := snapshotTestSession(t, "bad")
	// Leave no snapshot capacity, without changing existing assessment admission.
	used, _ := s.account.Usage(BudgetBufferedBytes)
	hold := s.account.Limits().MaxBufferedBytes - used
	if err := s.account.Charge(BudgetBufferedBytes, hold); err != nil {
		t.Fatal(err)
	}
	r, snapshot, err := s.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if !errors.Is(err, ErrResourceLimit) || snapshot != nil {
		t.Fatalf("snapshot=%v err=%v", snapshot, err)
	}
	if r.Counts.Fail != 1 || r.Conformance != ConformanceViolated || r.AssessmentComplete || r.Terminal == nil {
		t.Fatalf("lost violation on admission failure: %+v", r)
	}
	_ = s.account.Release(BudgetBufferedBytes, hold)
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("leaked %d", used)
	}
}
func TestResourceSnapshotCanceledAlwaysReturnsBestReport(t *testing.T) {
	s := snapshotTestSession(t, "bad")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, snapshot, err := s.FinishWithResourceSnapshot(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if !errors.Is(err, context.Canceled) || snapshot != nil || !r.Requested || r.Terminal == nil || r.AssessmentComplete {
		t.Fatalf("report=%+v snapshot=%v err=%v", r, snapshot, err)
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("leaked %d", used)
	}
}

func TestResourceSnapshotEveryCancellationBoundaryCleansLease(t *testing.T) {
	probe := snapshotTestSession(t, "bad")
	counter := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	_, snap, err := probe.FinishWithResourceSnapshot(counter, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	calls := counter.calls
	_ = snap.Close()
	for at := 1; at <= calls; at++ {
		s := snapshotTestSession(t, "bad")
		ctx := &rationalCancelContext{Context: context.Background(), at: at}
		report, snap, err := s.FinishWithResourceSnapshot(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
		if !errors.Is(err, context.Canceled) || !report.Requested || report.AssessmentComplete {
			t.Fatalf("boundary %d: report=%+v err=%v", at, report, err)
		}
		if snap != nil {
			_ = snap.Close()
		}
		if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("boundary %d leaked %d", at, used)
		}
	}
}

func TestResourceSnapshotRetainsActualTypeElementIDs(t *testing.T) {
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[SchemaName]*SchemaElement{{Local: "root"}: {Name: SchemaName{Local: "root"}, Type: SchemaTypeRef{InlineComplex: &SchemaComplex{Form: SchemaComplexDirect, Particle: &SchemaParticle{Kind: "sequence", Occurrence: SchemaOccurrence{Min: 1, Max: 1}, Children: []*SchemaParticle{{Kind: "element", Occurrence: SchemaOccurrence{Min: 1, Max: 1}, Element: &SchemaElement{Name: SchemaName{Local: "value"}, Type: SchemaTypeRef{Name: SchemaName{Space: "http://www.w3.org/2001/XMLSchema", Local: "string"}}}}}}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	e := sourceSchemaEngineFor(t, schema, XSDCheckBinding, XSDCheckDeclaration, XSDCheckAttributes, XSDCheckScalar, XSDCheckContent, XSDCheckIdentity)
	s, err := e.Begin(context.Background(), RunOptions{DocumentID: "source", Profile: "source-schema-test"})
	if err != nil {
		t.Fatal(err)
	}
	h := schemaPendingHeader()
	h.Bindings = []Binding{{Prefix: "xs", URI: "http://www.w3.org/2001/XMLSchema"}}
	if err = s.Open(context.Background(), h); err != nil {
		t.Fatal(err)
	}
	child := h
	child.ID = 2
	child.Parent = 1
	child.Name = QName{Local: "value"}
	child.Location.Path = "/root/value"
	if err = s.Open(context.Background(), child); err != nil {
		t.Fatal(err)
	}
	a := AttributeFact{Owner: 2, Name: QName{Space: "http://www.w3.org/2001/XMLSchema-instance", Local: "type"}, Presence: Evidence[bool]{State: EvidenceKnown, Value: true}, Declaration: Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "unbound"}, Scalar: Scalar{Lexeme: Evidence[string]{State: EvidenceKnown, Value: "xs:ID"}, Value: Evidence[Atomic]{State: EvidenceUnknown, Reason: "unbound"}}, Location: child.Location}
	if err = s.Attribute(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	end := schemaLexemeEnd("element-id")
	end.ID = 2
	if err = s.Close(context.Background(), end); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(context.Background(), schemaLexemeEnd("")); err != nil {
		t.Fatal(err)
	}
	report, snapshot, err := s.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil || !report.Satisfied() {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	defer snapshot.Close()
	if snapshot.IDCount() != 1 {
		t.Fatalf("actual element ID missing: %d", snapshot.IDCount())
	}
	id, ok, err := snapshot.IDAt(context.Background(), 0)
	if err != nil || !ok || id.Value != "element-id" || id.Subject != (NodeRef{"source", 1}) || id.Source != (NodeRef{"source", 2}) || id.Unique.State != EvidenceKnown || !id.Unique.Value {
		t.Fatalf("id=%+v err=%v", id, err)
	}
}

func snapshotElementIDs(t *testing.T, values []string, complete bool) (*Session, Report, *ResourceSnapshot, error) {
	t.Helper()
	id := &SchemaElement{Name: SchemaName{Local: "value"}, Type: SchemaTypeRef{Name: SchemaName{Space: "http://www.w3.org/2001/XMLSchema", Local: "ID"}}}
	item := &SchemaElement{Name: SchemaName{Local: "item"}, Type: SchemaTypeRef{InlineComplex: &SchemaComplex{Form: SchemaComplexDirect, Particle: &SchemaParticle{Kind: "sequence", Occurrence: SchemaOccurrence{Min: 1, Max: 1}, Children: []*SchemaParticle{{Kind: "element", Occurrence: SchemaOccurrence{Min: 1, Max: 1}, Element: id}}}}}}
	root := &SchemaElement{Name: SchemaName{Local: "root"}, Type: SchemaTypeRef{InlineComplex: &SchemaComplex{Form: SchemaComplexDirect, Particle: &SchemaParticle{Kind: "sequence", Occurrence: SchemaOccurrence{Min: 1, Max: 1}, Children: []*SchemaParticle{{Kind: "element", Occurrence: SchemaOccurrence{Min: 0, Max: 10}, Element: item}}}}}}
	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[SchemaName]*SchemaElement{{Local: "root"}: root}})
	if err != nil {
		t.Fatal(err)
	}
	e := sourceSchemaEngineFor(t, schema, XSDCheckBinding, XSDCheckDeclaration, XSDCheckAttributes, XSDCheckScalar, XSDCheckContent, XSDCheckIdentity)
	s, err := e.Begin(context.Background(), RunOptions{DocumentID: "source", Profile: "source-schema-test"})
	if err != nil {
		t.Fatal(err)
	}
	h := schemaPendingHeader()
	if err = s.Open(context.Background(), h); err != nil {
		t.Fatal(err)
	}
	for i, value := range values {
		itemh := h
		itemh.ID = NodeID(2 + i*2)
		itemh.Parent = 1
		itemh.Name = QName{Local: "item"}
		itemh.Location.Path = "/root/item"
		if err = s.Open(context.Background(), itemh); err != nil {
			t.Fatal(err)
		}
		vh := h
		vh.ID = itemh.ID + 1
		vh.Parent = itemh.ID
		vh.Name = QName{Local: "value"}
		vh.Location.Path = "/root/item/value"
		if err = s.Open(context.Background(), vh); err != nil {
			t.Fatal(err)
		}
		end := schemaLexemeEnd(value)
		end.ID = vh.ID
		if err = s.Close(context.Background(), end); err != nil {
			t.Fatal(err)
		}
		end = schemaLexemeEnd("")
		end.ID = itemh.ID
		if err = s.Close(context.Background(), end); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.Close(context.Background(), schemaLexemeEnd("")); err != nil {
		t.Fatal(err)
	}
	r, snap, err := s.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: complete, Syntax: SyntaxWellFormed})
	return s, r, snap, err
}

func TestResourceSnapshotThreeAmbiguousElementOwners(t *testing.T) {
	s, r, snap, err := snapshotElementIDs(t, []string{"dup", "dup", "dup"}, true)
	if err != nil || snap == nil {
		t.Fatalf("snapshot=%v err=%v report=%+v", snap, err, r)
	}
	defer snap.Close()
	if snap.IDCount() != 3 {
		t.Fatalf("IDCount=%d", snap.IDCount())
	}
	for i := 0; i < 3; i++ {
		f, ok, err := snap.IDAt(context.Background(), i)
		if err != nil || !ok {
			t.Fatal(err)
		}
		if f.Subject != (NodeRef{"source", NodeID(2 + i*2)}) {
			t.Errorf("occurrence %d: actual containing owner %+v, want node %d", i, f.Subject, 2+i*2)
		}
		if f.Source != (NodeRef{"source", NodeID(3 + i*2)}) {
			t.Errorf("occurrence %d: source %+v", i, f.Source)
		}
		if f.Unique.State != EvidenceInvalid || f.Unique.Cause == nil {
			t.Errorf("occurrence %d: lost duplicate cause: %+v", i, f.Unique)
		}
	}
	if r.Conformance != ConformanceViolated {
		t.Errorf("lost identity failure: %+v", r)
	}
	_ = snap.Close()
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Errorf("leaked %d", used)
	}
}

func TestResourceSnapshotLiveCopyAdmissionAndWork(t *testing.T) {
	s := snapshotTestSession(t, "bad")
	_, snap, err := s.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()
	before, _ := s.account.Usage(BudgetRuleSteps)
	used, _ := s.account.Usage(BudgetBufferedBytes)
	if _, err = snap.Report(context.Background()); err != nil {
		t.Fatal(err)
	}
	after, _ := s.account.Usage(BudgetRuleSteps)
	if after <= before {
		t.Fatal("live copy bypassed work admission")
	}
	if now, _ := s.account.Usage(BudgetBufferedBytes); now != used {
		t.Fatalf("temporary lease leaked: %d != %d", now, used)
	}
	hold := s.account.Limits().MaxBufferedBytes - used
	if err = s.account.Charge(BudgetBufferedBytes, hold); err != nil {
		t.Fatal(err)
	}
	_, err = snap.Report(context.Background())
	_ = s.account.Release(BudgetBufferedBytes, hold)
	if !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("copy admitted before capacity: %v", err)
	}
}
