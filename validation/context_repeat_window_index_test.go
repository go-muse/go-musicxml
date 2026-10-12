package validation

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
)

func repeatWindowIndexNotes(t *testing.T, count int) []contextRepeatNote {
	t.Helper()
	notes := make([]contextRepeatNote, count)
	for i := range notes {
		notes[i] = contextRepeatNote{
			source: NodeRef{"d", NodeID(i + 10)}, part: stateKnown(NodeRef{"d", 1}),
			pos: statePosition(t, 0, int64(i), 3), duration: mustRational(t, 1, 3),
			available: stateKnown(true), voice: stateKnown("one"),
		}
	}
	return notes
}

func TestContextRepeatWindowIndexMatchesConservativeWindows(t *testing.T) {
	part := NodeRef{"d", 1}
	for _, interleave := range []bool{false, true} {
		notes := repeatWindowIndexNotes(t, 6)
		if interleave {
			var mixed []contextRepeatNote
			for _, note := range notes {
				otherMeasure, otherPart := note, note
				otherMeasure.pos.MeasureOrdinal = 8
				otherMeasure.available = stateUnknown[bool]("irrelevant-other-measure")
				otherPart.part = stateKnown(NodeRef{"other-document", 1})
				otherPart.pos.Offset = Rational{}
				mixed = append(mixed, otherPart, note, otherMeasure)
			}
			notes = mixed
		}
		original := append([]contextRepeatNote(nil), notes...)
		s := rationalSession(t, Budgets{})
		index, release, err := contextRepeatBuildWindowIndex(context.Background(), s, notes, part, 0)
		if err != nil || !index.ordered || len(index.entries) != 6 {
			t.Fatalf("interleave=%t: %v %+v", interleave, err, index)
		}
		queries := [][4]int64{{0, 1, 1, 1}, {1, 3, 2, 3}, {1, 6, 1, 2}, {-1, 1, 0, 1}, {2, 1, 3, 1}, {0, 1, 0, 1}, {1, 2, 1, 6}, {-1, 3, 7, 3}, {1, 6, 1, 2}, {1, 2, 5, 6}}
		for i := int64(0); i < 80; i++ {
			queries = append(queries, [4]int64{(i*7)%17 - 2, 6, (i*11)%19 - 3, 6})
		}
		for _, query := range queries {
			from, to := mustRational(t, query[0], query[1]), mustRational(t, query[2], query[3])
			got, status, e := index.window(context.Background(), s, from, to)
			want, wantStatus, wantErr := contextRepeatWindow(context.Background(), s, notes, part, 0, from, to)
			if e != nil || wantErr != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(status, wantStatus) {
				t.Fatalf("interleave=%t query=%v: got %+v,%+v,%v want %+v,%+v,%v", interleave, query, got, status, e, want, wantStatus, wantErr)
			}
		}
		if !reflect.DeepEqual(notes, original) {
			t.Fatal("borrowed source notes mutated")
		}
		release()
		if used := rationalUsage(t, s, BudgetBufferedBytes); used != s.retained {
			t.Fatalf("temporary index leaked: %d vs retained %d", used, s.retained)
		}
		s.cleanup()
	}
}

func TestContextRepeatWindowIndexFallbackPreservesEvidence(t *testing.T) {
	cause := &DecisionRef{Rule: RuleKey{"window-invalid", 1}, Subject: NodeRef{"d", 99}}
	secondCause := &DecisionRef{Rule: RuleKey{"window-second-invalid", 1}, Subject: NodeRef{"d", 100}}
	invalid := Evidence[bool]{State: EvidenceInvalid, Reason: "original-window-cause", Cause: cause}
	for _, tc := range []struct {
		name string
		edit func([]contextRepeatNote)
	}{
		{"unknown-part-other-measure", func(n []contextRepeatNote) {
			n[2].part = stateUnknown[NodeRef]("unknown-part")
			n[2].pos.MeasureOrdinal = 8
			n[2].available = invalid
		}},
		{"unplaced-other-measure", func(n []contextRepeatNote) {
			n[2].pos.Offset = Rational{}
			n[2].pos.MeasureOrdinal = 8
			n[2].available = invalid
		}},
		{"unavailable-target-note", func(n []contextRepeatNote) { n[0].available = invalid }},
		{"unavailable-outside-window", func(n []contextRepeatNote) { n[2].available = invalid }},
		{"invalid-duration-outside-window", func(n []contextRepeatNote) { n[2].duration = Rational{}; n[2].available = invalid }},
		{"zero-duration", func(n []contextRepeatNote) { n[0].duration = mustRational(t, 0, 1) }},
		{"negative-duration", func(n []contextRepeatNote) { n[0].duration = mustRational(t, -1, 3) }},
		{"descending", func(n []contextRepeatNote) { n[1], n[2] = n[2], n[1] }},
		{"overlap", func(n []contextRepeatNote) { n[0].duration = mustRational(t, 1, 2) }},
		{"chord", func(n []contextRepeatNote) { n[1].pos.Offset = n[0].pos.Offset; n[1].chord = true }},
		{"cause-order", func(n []contextRepeatNote) {
			n[0].part = stateUnknown[NodeRef]("unknown-part")
			n[0].available = invalid
			n[1].available = Evidence[bool]{State: EvidenceInvalid, Reason: "second", Cause: secondCause}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			notes := repeatWindowIndexNotes(t, 3)
			tc.edit(notes)
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			index, release, err := contextRepeatBuildWindowIndex(context.Background(), s, notes, NodeRef{"d", 1}, 0)
			if err != nil || index.ordered {
				t.Fatalf("unsafe domain accepted: %v %+v", err, index)
			}
			defer release()
			from, to := mustRational(t, 0, 1), mustRational(t, 2, 3)
			got, status, err := index.window(context.Background(), s, from, to)
			want, wantStatus, wantErr := contextRepeatWindow(context.Background(), s, notes, NodeRef{"d", 1}, 0, from, to)
			if err != nil || wantErr != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(status, wantStatus) {
				t.Fatalf("fallback changed evidence: got %+v,%+v,%v want %+v,%+v,%v", got, status, err, want, wantStatus, wantErr)
			}
		})
	}
}

func TestContextRepeatWindowIndexAdmission(t *testing.T) {
	notes := repeatWindowIndexNotes(t, 3)
	for i := range notes {
		notes[i].part = stateKnown(NodeRef{"other", 1})
	}
	const bytes = 512 + 3*64
	for _, limit := range []int64{bytes, bytes - 1} {
		s := rationalSession(t, Budgets{MaxBufferedBytes: limit})
		index, release, err := contextRepeatBuildWindowIndex(context.Background(), s, notes, NodeRef{"d", 1}, 0)
		if limit == bytes {
			if err != nil || !index.ordered || len(index.entries) != 0 {
				t.Fatalf("exact index admission: %v %+v", err, index)
			}
			release()
		} else if !errors.Is(err, ErrResourceLimit) || index != nil || release != nil {
			t.Fatalf("one-under index admission: %v %+v", err, index)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("index admission leaked bytes")
		}
		s.cleanup()
	}
}

func TestContextRepeatWindowIndexCancellationAndWorkBoundary(t *testing.T) {
	notes := repeatWindowIndexNotes(t, 4)
	from, to := mustRational(t, 1, 3), mustRational(t, 1, 1)
	run := func(ctx context.Context, s *Session) error {
		index, release, err := contextRepeatBuildWindowIndex(ctx, s, notes, NodeRef{"d", 1}, 0)
		if err != nil {
			if index != nil || release != nil {
				t.Fatal("partial index escaped interruption")
			}
			return err
		}
		defer release()
		_, _, err = index.window(ctx, s, from, to)
		if err != nil {
			return err
		}
		_, _, err = index.window(ctx, s, to, mustRational(t, 4, 3))
		return err
	}
	probe := rationalSession(t, Budgets{})
	counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	if err := run(counter, probe); err != nil {
		t.Fatal(err)
	}
	work := rationalUsage(t, probe, BudgetRuleSteps)
	probe.cleanup()
	for at := 1; at <= counter.calls; at++ {
		s := rationalSession(t, Budgets{})
		if err := run(&rationalCancelContext{Context: context.Background(), at: at}, s); !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel poll %d: %v", at, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatalf("cancel poll %d leaked temporary storage", at)
		}
		usedWork := rationalUsage(t, s, BudgetRuleSteps)
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 || rationalUsage(t, s, BudgetRuleSteps) != usedWork {
			t.Fatalf("cancel poll %d leaked or refunded work", at)
		}
	}
	for _, limit := range []int64{work, work - 1} {
		s := rationalSession(t, Budgets{MaxRuleSteps: limit})
		err := run(context.Background(), s)
		if limit == work && err != nil || limit < work && !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("work boundary %d/%d: %v", limit, work, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatal("work refusal leaked temporary storage")
		}
		s.cleanup()
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	index, release, err := contextRepeatBuildWindowIndex(ctx, s, nil, NodeRef{}, 0)
	if !errors.Is(err, context.Canceled) || index != nil || release != nil || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatalf("empty canceled index: %v", err)
	}
	t.Logf("canceled all %d polls; exact work boundary %d", counter.calls, work)
}

func TestContextRepeatWindowIndexSequentialWindowGrowth(t *testing.T) {
	measure := func(count int) int64 {
		s := rationalSession(t, Budgets{})
		defer s.cleanup()
		notes := repeatWindowIndexNotes(t, count)
		index, release, err := contextRepeatBuildWindowIndex(context.Background(), s, notes, NodeRef{"d", 1}, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		for i := 0; i < count; i++ {
			got, status, err := index.window(context.Background(), s, mustRational(t, int64(i), 3), mustRational(t, int64(i+1), 3))
			if err != nil || status.State != EvidenceKnown || !status.Value || len(got) != 1 || got[0].source != notes[i].source || got[0].pos.Offset.String() != "0" {
				t.Fatalf("window %d: %v %+v %+v", i, err, status, got)
			}
		}
		return rationalUsage(t, s, BudgetRuleSteps)
	}
	small, large := measure(100), measure(1000)
	t.Logf("sequential windows with index construction: %d -> %d steps (%.2fx)", small, large, float64(large)/float64(small))
	if large > 15*small {
		t.Fatal("ordinary sequential beat windows still approach quadratic work")
	}
}
