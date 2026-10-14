package validation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestDictionaryProjectionOwnsNestedEvidence(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	input := ContextResult{Provider: RuleKey{ID: "dictionary", Revision: 1}, Subject: NodeRef{"d", 1}, Property: ContextDictionaryIPA, Value: Evidence[Atomic]{State: EvidenceAbsent}, Dictionary: &DictionaryContextResult{Relation: DictionaryRelation("ipa-symbol"), Input: stateKnown(Atomic{Kind: AtomicString, Text: "a?"}), Result: stateUnknown[bool]("partial-repertoire"), Spans: []DictionarySpan{{Start: 0, End: 1, Text: "a", Result: stateKnown(true), RowID: "a"}, {Start: 1, End: 2, Text: "?", Result: stateUnknown[bool]("unlisted")}}}}
	if err := s.publishContextResult(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	input.Dictionary.Spans[0].Text = "mutated"
	input.Dictionary.Spans[1].Result.Reason = "mutated"
	got := s.contextResults[0].Dictionary
	if got.Spans[0].Text != "a" || got.Spans[1].Result.Reason != "unlisted" {
		t.Fatalf("aliased: %+v", got)
	}
	if !resourceSelectedContext(ContextDictionaryIPA) {
		t.Fatal("dictionary projection omitted from resource snapshot")
	}
}

func TestDictionaryProjectionOmittedFromOldJSON(t *testing.T) {
	b, err := json.Marshal(ContextResult{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "Dictionary") {
		t.Fatalf("old JSON changed: %s", b)
	}
}

func TestDictionaryProjectionAdmissionExactAndOneUnder(t *testing.T) {
	payload := &DictionaryContextResult{Relation: DictionaryRelation("ipa-symbol"), Input: stateKnown(Atomic{Kind: AtomicString, Text: "a"}), Result: stateUnknown[bool]("partial"), Spans: []DictionarySpan{{Start: 0, End: 1, Text: "a", Result: stateKnown(true), RowID: "row"}}}
	probe := rationalSession(t, Budgets{})
	_, err := cloneDictionaryContext(context.Background(), probe, payload)
	if err != nil {
		t.Fatal(err)
	}
	cost := probe.retained
	probe.cleanup()
	for _, limit := range []int64{cost - 1, cost} {
		s := rationalSession(t, Budgets{MaxBufferedBytes: limit})
		copy, err := cloneDictionaryContext(context.Background(), s, payload)
		if limit == cost && (err != nil || copy == nil) {
			t.Fatalf("exact: %v", err)
		}
		if limit < cost && (!errors.Is(err, ErrResourceLimit) || copy != nil) {
			t.Fatalf("one under: %v", err)
		}
		s.cleanup()
	}
}

func TestDictionaryResourceCapabilityIdentity(t *testing.T) {
	a := packageResourceKey{pin: testResourcePin("same"), profile: "same", digest: "same", dictionary: [32]byte{1}}
	b := a
	b.dictionary[0] = 2
	if a == b {
		t.Fatal("different certified capabilities alias")
	}
	s := snapshotTestSession(t, "1")
	// A missing selected relation and no selected relation are distinct actual
	// capability states, even though neither grants complete membership.
	set, err := NewDictionarySet(context.Background(), s.account, []DictionarySelection{{Manifest: DictionaryManifestSMuFL14Canonical, Relations: []DictionaryRelation{DictionaryRelationCanonicalName}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := set.borrow(s.account)
	if err != nil {
		t.Fatal(err)
	}
	s.dictionaries = lease
	want := s.dictionaryFingerprint()
	if want == ([32]byte{}) {
		t.Fatal("lost selected capability identity")
	}
	_ = set.Close()
	_, snapshot, err := s.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	if snapshot.dictionary != want {
		t.Fatal("snapshot identity changed after owner Close")
	}
}

func TestDictionarySnapshotGetAndOccurrenceRemapOwnNestedCauses(t *testing.T) {
	s := snapshotTestSession(t, "1")
	cause := &DecisionRef{Rule: RuleKey{ID: "scalar", Revision: 1}, Subject: NodeRef{"source", 1}}
	raw := Evidence[Atomic]{State: EvidenceInvalid, Reason: "bad", Cause: cause}
	payload := &DictionaryContextResult{Relation: DictionaryRelationIPASymbol, Input: raw, Result: Evidence[bool]{State: EvidenceInvalid, Reason: "bad", Cause: cause}, Spans: []DictionarySpan{{Text: "?", End: 1, Result: Evidence[bool]{State: EvidenceInvalid, Reason: "bad", Cause: cause}}}}
	in := ContextResult{Provider: RuleKey{ID: "dictionary", Revision: 1}, Subject: NodeRef{"source", 1}, Property: ContextDictionaryIPA, Value: Evidence[Atomic]{State: EvidenceAbsent}, Dictionary: payload, Origins: []Location{{DocumentID: "source", Target: TargetSource, Path: "/root"}}}
	if err := s.publishContextResult(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	_, snapshot, err := s.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	first, ok, err := snapshot.ContextAt(context.Background(), 0)
	if err != nil || !ok {
		t.Fatal(err)
	}
	first.Dictionary.Spans[0].Result.Cause.Subject.DocumentID = "mutated"
	again, ok, err := snapshot.ContextAt(context.Background(), 0)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if again.Dictionary.Spans[0].Result.Cause.Subject.DocumentID != "source" {
		t.Fatal("getter aliases nested evidence")
	}
	rebound, err := resourceLiveCopy(context.Background(), snapshot.account, again, "source", "child-2")
	if err != nil {
		t.Fatal(err)
	}
	if rebound.Subject.DocumentID != "child-2" || rebound.Origins[0].DocumentID != "child-2" || rebound.Dictionary.Input.Cause.Subject.DocumentID != "child-2" || rebound.Dictionary.Spans[0].Result.Cause.Subject.DocumentID != "child-2" {
		t.Fatalf("partial occurrence remap: %+v", rebound)
	}
	if again.Dictionary.Input.Cause.Subject.DocumentID != "source" {
		t.Fatal("remap mutated canonical copy")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := snapshot.ContextAt(canceled, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled getter: %v", err)
	}
}

func TestDictionaryProjectionRejectsMixedPayloadAndChargesEveryString(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	in := ContextResult{Provider: RuleKey{ID: "dictionary", Revision: 1}, Subject: NodeRef{"d", 1}, Property: ContextDictionaryIPA, Dictionary: &DictionaryContextResult{}, Layout: &LayoutContextResult{}}
	if err := s.publishContextResult(context.Background(), in); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("mixed projection: %v", err)
	}
	small := &DictionaryContextResult{Relation: DictionaryRelationIPASymbol, Spans: []DictionarySpan{{Text: "a", RowID: "row", Category: "symbol", Equivalent: "b", Result: Evidence[bool]{State: EvidenceUnknown, Reason: "gap"}}}}
	before := s.retained
	if _, err := cloneDictionaryContext(context.Background(), s, small); err != nil {
		t.Fatal(err)
	}
	base := s.retained - before
	large := stateClone(*small)
	large.Spans[0].RowID = strings.Repeat("r", 1001)
	before = s.retained
	if _, err := cloneDictionaryContext(context.Background(), s, &large); err != nil {
		t.Fatal(err)
	}
	if delta := s.retained - before - base; delta != 998 {
		t.Fatalf("owned string storage delta=%d", delta)
	}
}

func TestDictionaryProjectionScalarCapsCoverDerivedValues(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload *DictionaryContextResult
		limit   int64
	}{
		{"input", &DictionaryContextResult{Input: stateKnown(Atomic{Kind: AtomicString, Text: "input"})}, 5},
		{"default", &DictionaryContextResult{Input: Evidence[Atomic]{State: EvidenceAbsent}, Effective: stateKnown(Atomic{Kind: AtomicString, Text: "brassMuteClosed"})}, 15},
		{"span", &DictionaryContextResult{Spans: []DictionarySpan{{Text: "encoded", End: 7}}}, 7},
		{"equivalent", &DictionaryContextResult{Spans: []DictionarySpan{{Text: "é", Equivalent: "é", End: 2}}}, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, limit := range []int64{tc.limit - 1, tc.limit} {
				s := rationalSession(t, Budgets{MaxScalarBytes: limit})
				copy, err := cloneDictionaryContext(context.Background(), s, tc.payload)
				if limit < tc.limit {
					if !errors.Is(err, ErrResourceLimit) || copy != nil || s.retained != 0 {
						t.Fatalf("derived scalar bypass: copy=%v retained=%d err=%v", copy, s.retained, err)
					}
				} else if err != nil || copy == nil {
					t.Fatalf("exact scalar cap: %v", err)
				}
				s.cleanup()
			}
		})
	}
	s := rationalSession(t, Budgets{MaxScalarBytes: 1})
	defer s.cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := cloneDictionaryContext(ctx, s, &DictionaryContextResult{Effective: stateKnown(Atomic{Kind: AtomicString, Text: "brassMuteClosed"})}); !errors.Is(err, context.Canceled) {
		t.Fatalf("scalar cap hid cancellation: %v", err)
	}
}
