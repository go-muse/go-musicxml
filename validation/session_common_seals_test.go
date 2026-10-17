package validation

import (
	"context"
	"errors"
	"testing"
	"unsafe"
)

func TestCommonExaminationSealsShareOnlyCanonicalEvidence(t *testing.T) {
	s := representationFixture(t, Budgets{}, true)
	defer s.cleanup()
	ctx := context.Background()
	representationOpen(t, s)
	for id := NodeID(2); id <= 4; id++ {
		if err := s.Open(ctx, representationHeader(id, 1, "test.child", "child")); err != nil {
			t.Fatal(err)
		}
		before, _ := s.account.Usage(BudgetBufferedBytes)
		complete := Evidence[bool]{State: EvidenceKnown, Value: true}
		if err := s.CompleteAttributes(ctx, id, complete); err != nil {
			t.Fatal(err)
		}
		fact := representationFact(id)
		if err := s.ModelRepresentation(ctx, fact); err != nil {
			t.Fatal(err)
		}
		after, _ := s.account.Usage(BudgetBufferedBytes)
		if id == 2 && after-before != 192 {
			t.Fatalf("first shared seals cost %d, want 192", after-before)
		}
		if id > 2 && after != before {
			t.Fatalf("duplicate canonical seals retained %d bytes", after-before)
		}
		// Public input objects remain caller-owned. Mutating them cannot alter
		// already admitted evidence, including any shared internal record.
		complete.Value = false
		fact.Complete.Value = false
		if !s.nodes[id].attributeSeal.Value || !s.nodes[id].modelRepresentation.Complete.Value {
			t.Fatal("caller mutation changed seal")
		}
		if id > 2 && (s.nodes[id].attributeSeal != s.nodes[2].attributeSeal || s.nodes[id].modelRepresentation != s.nodes[2].modelRepresentation) {
			t.Fatal("canonical seals still allocate per node")
		}
		if err := s.Close(ctx, representationEnd(id)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ModelRepresentation(ctx, representationFact(1)); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(ctx, representationEnd(1)); err != nil {
		t.Fatal(err)
	}
	r := representationFinish(t, s)
	if !r.Satisfied() {
		t.Fatalf("shared examination changed report: %+v", r)
	}
}

func TestCommonExaminationSealAdmissionAndCleanup(t *testing.T) {
	for _, model := range []bool{false, true} {
		cost := int64(64)
		if model {
			cost = 128
		}
		for _, short := range []int64{0, 1} {
			s := representationFixture(t, Budgets{}, true)
			representationOpen(t, s)
			used, _ := s.account.Usage(BudgetBufferedBytes)
			padding := s.account.Limits().MaxBufferedBytes - used - cost + short
			if err := s.account.Charge(BudgetBufferedBytes, padding); err != nil {
				t.Fatal(err)
			}
			var err error
			if model {
				err = s.ModelRepresentation(context.Background(), representationFact(1))
			} else {
				err = s.CompleteAttributes(context.Background(), 1, Evidence[bool]{State: EvidenceKnown, Value: true})
			}
			if short == 0 && err != nil {
				t.Fatal(err)
			}
			if short != 0 {
				var resource *ResourceError
				if !errors.As(err, &resource) {
					t.Fatalf("one-short admission: %v", err)
				}
				if s.nodes[1].attributeSeal != nil || s.nodes[1].modelRepresentation != nil {
					t.Fatal("failed admission published seal")
				}
			}
			if err := s.account.Release(BudgetBufferedBytes, padding); err != nil {
				t.Fatal(err)
			}
			s.cleanup()
			if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
				t.Fatalf("retained %d", used)
			}
			if s.commonModelSeal != nil || s.commonAttributeSeal != nil {
				t.Fatal("cleanup retains shared record")
			}
		}
	}
}

func TestCommonExaminationSealEnvelopesAndSessionIsolation(t *testing.T) {
	if unsafe.Sizeof(modelRepresentationRecord{}) > 128 || unsafe.Sizeof(Evidence[bool]{}) > 64 || unsafe.Sizeof(Session{}) > 1024 {
		t.Fatal("fixed retained envelope no longer covers representation")
	}
	a := representationFixture(t, Budgets{}, true)
	b := representationFixture(t, Budgets{}, true)
	defer a.cleanup()
	defer b.cleanup()
	for _, s := range []*Session{a, b} {
		representationOpen(t, s)
		if err := s.CompleteAttributes(context.Background(), 1, Evidence[bool]{State: EvidenceKnown, Value: true}); err != nil {
			t.Fatal(err)
		}
		if err := s.ModelRepresentation(context.Background(), representationFact(1)); err != nil {
			t.Fatal(err)
		}
	}
	if a.nodes[1].attributeSeal == b.nodes[1].attributeSeal || a.nodes[1].modelRepresentation == b.nodes[1].modelRepresentation {
		t.Fatal("records escaped account/session ownership")
	}
}

func TestCommonExaminationSealsDoNotShareExceptionalEvidence(t *testing.T) {
	for _, complete := range []Evidence[bool]{
		{State: EvidenceKnown, Value: false},
		{State: EvidenceKnown, Value: true, Reason: "explicit-detail"},
		{State: EvidenceUnknown, Reason: "partial"},
		{State: EvidenceUnsupported, Reason: "unavailable"},
	} {
		t.Run(string(complete.State)+"/"+string(complete.Reason), func(t *testing.T) {
			s := representationFixture(t, Budgets{}, true)
			defer s.cleanup()
			ctx := context.Background()
			representationOpen(t, s)
			for id := NodeID(2); id <= 3; id++ {
				if err := s.Open(ctx, representationHeader(id, 1, "test.child", "child")); err != nil {
					t.Fatal(err)
				}
				if err := s.CompleteAttributes(ctx, id, complete); err != nil {
					t.Fatal(err)
				}
				if err := s.ModelRepresentation(ctx, ModelRepresentationFact{Owner: id, Complete: complete}); err != nil {
					t.Fatal(err)
				}
				if id == 3 && (s.nodes[2].attributeSeal == s.nodes[3].attributeSeal || s.nodes[2].modelRepresentation == s.nodes[3].modelRepresentation) {
					t.Fatal("exceptional evidence was shared")
				}
				end := representationEnd(id)
				end.AttributesComplete = complete
				if err := s.Close(ctx, end); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
