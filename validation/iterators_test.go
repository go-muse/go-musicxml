package validation

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// Contexts yields what ContextAt returns, in order; a canceled context or a
// closed snapshot is yielded once as an error; breaking stops the sequence.
func TestResourceSnapshotIterators(t *testing.T) {
	s := snapshotTestSession(t, "1")
	for i := range 3 {
		payload := &DictionaryContextResult{Relation: DictionaryRelationIPASymbol, Input: Evidence[Atomic]{State: EvidenceAbsent}, Result: Evidence[bool]{State: EvidenceUnknown, Reason: "gap"}}
		in := ContextResult{Provider: RuleKey{ID: "dictionary", Revision: 1}, Subject: NodeRef{"source", NodeID(i + 1)}, Property: ContextDictionaryIPA, Value: Evidence[Atomic]{State: EvidenceAbsent}, Dictionary: payload}
		if err := s.publishContextResult(context.Background(), in); err != nil {
			t.Fatal(err)
		}
	}
	_, snapshot, err := s.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	var got []ContextResult
	for result, err := range snapshot.Contexts(context.Background()) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, result)
	}
	if len(got) != snapshot.ContextCount() || len(got) != 3 {
		t.Fatalf("yielded %d of %d contexts", len(got), snapshot.ContextCount())
	}
	for i, result := range got {
		want, _, _ := snapshot.ContextAt(context.Background(), i)
		if !reflect.DeepEqual(result, want) {
			t.Errorf("context %d differs from ContextAt", i)
		}
	}
	seen := 0
	for range snapshot.Contexts(context.Background()) {
		seen++
		break
	}
	ids := 0
	for _, err := range snapshot.IDs(context.Background()) {
		if err != nil {
			t.Fatal(err)
		}
		ids++
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	var failures []error
	for result, err := range snapshot.Contexts(canceled) {
		if result.Provider.ID != "" {
			t.Error("a failure carried a result")
		}
		failures = append(failures, err)
	}
	if seen != 1 || ids != snapshot.IDCount() || len(failures) != 1 || !errors.Is(failures[0], context.Canceled) {
		t.Fatalf("break=%d ids=%d/%d failures=%v", seen, ids, snapshot.IDCount(), failures)
	}
	if err := snapshot.Close(); err != nil {
		t.Fatal(err)
	}
	failures = nil
	for _, err := range snapshot.Contexts(context.Background()) {
		failures = append(failures, err)
	}
	if len(failures) != 1 || failures[0] == nil {
		t.Fatalf("closed snapshot: %v", failures)
	}
}
