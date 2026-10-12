package validation

import (
	"context"
	"errors"
	"testing"
	"unsafe"
)

func independentNoteInstrumentRecord() noteInstrumentSet {
	return noteInstrumentSet{Part: NodeRef{"d", 1}, Note: NodeRef{"d", 2}, Complete: stateKnown(true)}
}

func TestIndependentTimelineNoteInstrumentAppendAllowance(t *testing.T) {
	width := int64(unsafe.Sizeof(noteInstrumentSet{}))
	if timelineNoteInstrumentReserveBytes < 3*width {
		t.Fatalf("allowance%d below triple fixed shape%d", timelineNoteInstrumentReserveBytes, width)
	}
	var records []noteInstrumentSet
	for count := 1; count <= 4096; count++ {
		old := cap(records)
		records = append(records, noteInstrumentSet{})
		peak := cap(records)
		if cap(records) != old {
			peak += old
		}
		if int64(peak)*width > int64(count)*timelineNoteInstrumentReserveBytes {
			t.Fatalf("append%d live capacities%d width%d exceed admitted%d", count, peak, width, int64(count)*timelineNoteInstrumentReserveBytes)
		}
	}
	if 128 < int64(unsafe.Sizeof(Evidence[NodeRef]{})) {
		t.Fatal("exact reference vector admission narrower than element")
	}
}

func TestIndependentTimelineNoteInstrumentFixedPeak(t *testing.T) {
	for _, short := range []bool{false, true} {
		limit := 2 * timelineNoteInstrumentReserveBytes
		if short {
			limit--
		}
		s := rationalSession(t, Budgets{MaxBufferedBytes: limit})
		h := &timelineState{}
		err := h.addNoteInstruments(context.Background(), s, independentNoteInstrumentRecord())
		if short {
			if !errors.Is(err, ErrResourceLimit) || len(h.NoteInstruments) != 0 {
				t.Fatalf("peak-minus1 published: %v", err)
			}
		} else if err != nil || len(h.NoteInstruments) != 1 || rationalUsage(t, s, BudgetBufferedBytes) != timelineNoteInstrumentReserveBytes {
			t.Fatalf("exact peak rejected or scratch retained: %v bytes%d", err, rationalUsage(t, s, BudgetBufferedBytes))
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatal("fixed scratch leaked")
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("cleanup leaked")
		}
	}
}

func TestIndependentTimelineNoteInstrumentBorrowedReferences(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	n := independentNoteInstrumentRecord()
	cause := &DecisionRef{Rule: RuleKey{ID: "actual", Revision: 1}, Subject: NodeRef{"d", 91}}
	n.Complete = Evidence[bool]{State: EvidenceInvalid, Reason: "actual-failure", Cause: cause}
	n.References = []Evidence[NodeRef]{{State: EvidenceInvalid, Reason: "actual-failure", Cause: cause}, {State: EvidenceKnown, Value: NodeRef{"d", 92}}}
	for i := 0; i < 2; i++ {
		if err := h.addNoteInstruments(context.Background(), s, n); err != nil {
			t.Fatal(err)
		}
	}
	cause.Subject.Node = 999
	n.References[1].Value.Node = 999
	for _, stored := range h.NoteInstruments {
		if stored.Complete.Cause == cause || stored.Complete.Cause.Subject.Node != 91 || stored.References[0].Cause == cause || stored.References[0].Cause.Subject.Node != 91 || stored.References[1].Value.Node != 92 {
			t.Fatal("borrowed reference/cause escaped into owned records")
		}
		if stored.Complete.Cause == stored.References[0].Cause {
			t.Fatal("distinct evidence retained shared mutable Cause")
		}
	}
	h.NoteInstruments[0].References[0].Cause.Subject.Node = 555
	h.NoteInstruments[0].References[1].Value.Node = 555
	if h.NoteInstruments[1].References[0].Cause.Subject.Node != 91 || h.NoteInstruments[1].References[1].Value.Node != 92 {
		t.Fatal("publications share mutable reference vector or cause")
	}
	if rationalUsage(t, s, BudgetContextFacts) != 2 {
		t.Fatal("context quota missing")
	}
}

func TestIndependentTimelineNoteInstrumentEveryCancellationBoundary(t *testing.T) {
	record := independentNoteInstrumentRecord()
	cause := &DecisionRef{Rule: RuleKey{ID: "actual", Revision: 1}, Subject: NodeRef{"d", 91}}
	record.Complete = Evidence[bool]{State: EvidenceInvalid, Cause: cause}
	for i := 0; i < 7; i++ {
		record.References = append(record.References, Evidence[NodeRef]{State: EvidenceInvalid, Cause: cause})
	}
	probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	s := rationalSession(t, Budgets{})
	if err := (&timelineState{}).addNoteInstruments(probe, s, record); err != nil {
		t.Fatal(err)
	}
	s.cleanup()
	for at := 1; at <= probe.calls; at++ {
		s := rationalSession(t, Budgets{})
		h := &timelineState{}
		ctx := &rationalCancelContext{Context: context.Background(), at: at}
		err := h.addNoteInstruments(ctx, s, record)
		if !errors.Is(err, context.Canceled) || len(h.NoteInstruments) != 0 {
			t.Fatalf("poll%d published or changed cancellation: %v", at, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatalf("poll%d scratch leaked", at)
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("cleanup leaked")
		}
	}
	t.Logf("checked all%d note-instrument cancellation polls", probe.calls)
}
