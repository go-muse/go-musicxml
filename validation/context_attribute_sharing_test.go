package validation

import (
	"context"
	"errors"
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
	before := steps()
	if _, ok := first.attributeSelector(s.nodes[2], "required"); !ok || first.err != nil {
		t.Fatal(first.err)
	}
	built := steps() - before
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
	if s.contextAttributeTables != nil || s.contextAttributeIntern != nil || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
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

// A session that omits the projection publishes no context records, keeps
// the source-origin index its verdicts read, and refuses a resource snapshot,
// which needs the projection; the refusal releases everything.
func TestOmitContextSessionPublishesNoRecords(t *testing.T) {
	row := ContextResult{Provider: RuleKey{"omit", 1}, Subject: NodeRef{"source", 1}, Property: ContextPropertySpanNormalDots, Value: Evidence[Atomic]{State: EvidenceAbsent}}
	s := rationalSession(t, Budgets{})
	s.omitContext = true
	facts := rationalUsage(t, s, BudgetContextFacts)
	if err := s.publishContextResult(context.Background(), row); err != nil {
		t.Fatal(err)
	}
	if len(s.contextResults) != 0 || s.retained != 0 || s.contextReportRetained != 0 || rationalUsage(t, s, BudgetContextFacts) != facts+1 {
		t.Fatalf("omitted record kept: records=%d retained=%d facts=%d", len(s.contextResults), s.retained, rationalUsage(t, s, BudgetContextFacts)-facts)
	}
	s.cleanup()

	schema, err := CompileSchema(AssemblyMusicXML, SchemaData{Elements: map[QName]*SchemaElement{{Local: "root"}: {Name: QName{Local: "root"}, Type: localBuiltin("string")}}})
	if err != nil {
		t.Fatal(err)
	}
	session, err := sourceSchemaEngineFor(t, schema, XSDCheckContent).Begin(context.Background(), RunOptions{DocumentID: "source", Profile: "source-schema-test", OmitContext: true})
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
		t.Fatalf("snapshot of an omitted projection: snapshot=%v err=%v", snapshot != nil, err)
	}
	if used := rationalUsage(t, session, BudgetBufferedBytes); used != 0 {
		t.Fatalf("refused snapshot left %d bytes", used)
	}
}
