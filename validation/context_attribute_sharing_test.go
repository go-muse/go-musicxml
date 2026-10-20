package validation

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func finalAttributeFixture(t *testing.T) (*Session, *contextBindings) {
	t.Helper()
	s, b := contextReviewSameTypeFixture(t)
	// Finish sets finished before finalizing the schema and evaluating
	// providers; perform that prelude so final readers can be examined.
	s.finished, s.inputComplete = true, true
	if err := s.finalizeSourceSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s, b
}

// Final readers of one session share attribute tables per compiled bindings:
// a second reader borrows the table the first built without repeating its
// work, other bindings get their own entry, a reader before Finish keeps its
// own cache, and cleanup releases the shared entries.
func TestContextAttributeTablesSharedAcrossFinalReaders(t *testing.T) {
	s, b := contextReviewSameTypeFixture(t)
	early := &contextReader{ctx: context.Background(), s: s, b: b}
	if _, ok := early.attributeSelector(s.nodes[2], "required"); !ok || early.err != nil {
		t.Fatal(early.err)
	}
	if len(early.attributeCache) != 1 || s.contextAttributeTables != nil {
		t.Fatalf("reader before Finish: own=%d shared=%d", len(early.attributeCache), len(s.contextAttributeTables))
	}
	s.cleanup()

	s, b = finalAttributeFixture(t)
	steps := func() int64 { return rationalUsage(t, s, BudgetRuleSteps) }
	first := &contextReader{ctx: context.Background(), s: s, b: b}
	before, retained := steps(), s.retained
	if _, ok := first.attributeSelector(s.nodes[2], "required"); !ok || first.err != nil {
		t.Fatal(first.err)
	}
	built := steps() - before
	// A span rollback keeps exactly this charge of the shared tables.
	if s.contextSharedTableBytes == 0 || s.contextSharedTableBytes != s.retained-retained {
		t.Fatalf("shared table charge %d, retained %d", s.contextSharedTableBytes, s.retained-retained)
	}
	second := &contextReader{ctx: context.Background(), s: s, b: b}
	before = steps()
	if _, ok := second.attributeSelector(s.nodes[2], "required"); !ok || second.err != nil {
		t.Fatal(second.err)
	}
	borrowed := steps() - before
	if borrowed >= built || len(first.attributeCache) != 0 || len(second.attributeCache) != 0 || len(s.contextAttributeTables) != 1 {
		t.Fatalf("second reader rebuilt the table: built=%d borrowed=%d own=%d/%d shared=%d", built, borrowed, len(first.attributeCache), len(second.attributeCache), len(s.contextAttributeTables))
	}
	if first.attributeTable(s.nodes[2]) != second.attributeTable(s.nodes[2]) {
		t.Fatal("readers hold different tables")
	}
	other := *b
	third := &contextReader{ctx: context.Background(), s: s, b: &other}
	if _, ok := third.attributeSelector(s.nodes[2], "required"); !ok || third.err != nil || len(s.contextAttributeTables) != 2 {
		t.Fatalf("other bindings shared a table: %v entries=%d", third.err, len(s.contextAttributeTables))
	}
	s.cleanup()
	if s.contextAttributeTables != nil || s.contextAttributeIntern != nil || s.contextSharedTableBytes != 0 || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatal("shared attribute tables survived cleanup")
	}
}

// An interrupted build stores nothing in the shared cache and keeps the
// session's retained storage unchanged; the next reader builds the table.
func TestContextAttributeTablesSharedOnlyWhenBuilt(t *testing.T) {
	s, b := finalAttributeFixture(t)
	defer s.cleanup()
	probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	count := &contextReader{ctx: probe, s: s, b: b}
	if _, ok := count.attributeSelector(s.nodes[2], "required"); !ok || count.err != nil {
		t.Fatal(count.err)
	}
	polls := probe.calls
	s.cleanup()
	for at := 1; at <= polls; at++ {
		s, b := finalAttributeFixture(t)
		retained := s.retained
		r := &contextReader{ctx: &rationalCancelContext{Context: context.Background(), at: at}, s: s, b: b}
		_, _ = r.attributeSelector(s.nodes[2], "required")
		if !errors.Is(r.err, context.Canceled) || len(s.contextAttributeTables) != 0 || len(s.contextAttributeIntern) != 0 || s.retained != retained {
			t.Fatalf("poll %d published a partial table: err=%v shared=%d retained=%d/%d", at, r.err, len(s.contextAttributeTables), s.retained, retained)
		}
		next := &contextReader{ctx: context.Background(), s: s, b: b}
		if _, ok := next.attributeSelector(s.nodes[2], "required"); !ok || next.err != nil || len(s.contextAttributeTables) != 1 {
			t.Fatalf("poll %d: next reader did not build: %v", at, next.err)
		}
		s.cleanup()
	}
}

// A session keeps only the records of its selected families; an unselected
// record is counted and checked but costs no storage. A resource snapshot
// refuses a session that lacks the families it copies unless the run was
// prepared for a snapshot; the refusal releases everything.
func TestUnselectedContextIsNotKept(t *testing.T) {
	row := ContextResult{Provider: RuleKey{"omit", 1}, Subject: NodeRef{"source", 1}, Property: ContextPropertySpanNormalDots, Value: Evidence[Atomic]{State: EvidenceAbsent}}
	for _, families := range []ContextFamilies{0, ContextFamilyAll &^ ContextFamilySpans} {
		s := rationalSession(t, Budgets{})
		s.contextFamilies = families
		facts := rationalUsage(t, s, BudgetContextFacts)
		if err := s.publishContextResult(context.Background(), row); err != nil {
			t.Fatal(err)
		}
		if len(s.contextResults) != 0 || len(s.hiddenContexts) != 0 || s.retained != 0 || s.contextReportRetained != 0 || rationalUsage(t, s, BudgetContextFacts) != facts+1 {
			t.Fatalf("context=%b: unselected record kept: records=%d retained=%d facts=%d", families, len(s.contextResults), s.retained, rationalUsage(t, s, BudgetContextFacts)-facts)
		}
		s.cleanup()
	}
	s := rationalSession(t, Budgets{})
	s.contextFamilies = ContextFamilySpans
	if err := s.publishContextResult(context.Background(), row); err != nil {
		t.Fatal(err)
	}
	if len(s.contextResults) != 1 || s.contextReportRetained == 0 {
		t.Fatalf("selected record not kept: records=%d", len(s.contextResults))
	}
	s.cleanup()

	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[QName]*SchemaElement{{Local: "root"}: {Name: QName{Local: "root"}, Type: localBuiltin("string")}}})
	if err != nil {
		t.Fatal(err)
	}
	session, err := sourceSchemaEngineFor(t, schema, XSDCheckContent).Begin(context.Background(), RunOptions{DocumentID: "source", Profile: "source-schema-test", Context: ContextFamilyAll &^ ContextFamilyDictionaries})
	if err != nil {
		t.Fatal(err)
	}
	if err = session.OpenNode(context.Background(), schemaPendingHeader()); err != nil {
		t.Fatal(err)
	}
	if err = session.CloseNode(context.Background(), schemaLexemeEnd("value")); err != nil {
		t.Fatal(err)
	}
	_, snapshot, err := session.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if snapshot != nil || !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("snapshot without its families: snapshot=%v err=%v", snapshot != nil, err)
	}
	if used := rationalUsage(t, session, BudgetBufferedBytes); used != 0 {
		t.Fatalf("refused snapshot left %d bytes", used)
	}
	if _, err = sourceSchemaEngineFor(t, schema, XSDCheckContent).Begin(context.Background(), RunOptions{DocumentID: "source", Profile: "source-schema-test", Context: ContextFamilyAll + 1}); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("unknown family accepted: %v", err)
	}
}

// A run prepared for a snapshot keeps the unselected records the snapshot
// copies outside Report.Context, and the snapshot sees every copied record in
// the order of the complete projection.
func TestResourceSnapshotKeepsHiddenRecordsInOrder(t *testing.T) {
	resource := func(node NodeID) ContextResult {
		return ContextResult{Provider: RuleKey{"hidden", 1}, Subject: NodeRef{"source", node}, Property: ContextResourceLink, Value: Evidence[Atomic]{State: EvidenceAbsent}, Resource: &ResourceContextResult{}}
	}
	sounds := func(node NodeID) ContextResult {
		return ContextResult{Provider: RuleKey{"hidden", 1}, Subject: NodeRef{"source", node}, Property: ContextPropertySoundMapping, Value: Evidence[Atomic]{State: EvidenceAbsent}, SoundMapping: &SoundMappingContextResult{}}
	}
	span := func(node NodeID) ContextResult {
		return ContextResult{Provider: RuleKey{"hidden", 1}, Subject: NodeRef{"source", node}, Property: ContextPropertySpanNormalDots, Value: Evidence[Atomic]{State: EvidenceAbsent}}
	}
	// Resources and spans are selected; sound mappings are kept only for the
	// snapshot, and spans not at all by it.
	s := rationalSession(t, Budgets{})
	s.contextFamilies, s.resourceSnapshot = ContextFamilyResources|ContextFamilySpans, true
	for i, row := range []ContextResult{sounds(1), resource(2), span(3), sounds(4), sounds(5), resource(6), sounds(7)} {
		if err := s.publishValidatedContextResult(context.Background(), row); err != nil {
			t.Fatalf("row %d: %v", i, err)
		}
	}
	if len(s.contextResults) != 3 || len(s.hiddenContexts) != 4 {
		t.Fatalf("records=%d hidden=%d", len(s.contextResults), len(s.hiddenContexts))
	}
	var order []NodeID
	if err := s.snapshotContexts(context.Background(), func(r *ContextResult) error { order = append(order, r.Subject.Node); return nil }); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(order, []NodeID{1, 2, 4, 5, 6, 7}) {
		t.Fatalf("snapshot order %v", order)
	}
	retained := s.retained
	s.cleanup()
	if retained <= 0 || s.hiddenContexts != nil || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatalf("hidden records not released: retained=%d", retained)
	}
}
