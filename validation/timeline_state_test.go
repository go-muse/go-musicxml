package validation

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func stateInt(t *testing.T, n int64) Integer {
	t.Helper()
	v, e := NewInteger(big.NewInt(n))
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func stateDecimal(t *testing.T, n, d int64) Decimal {
	t.Helper()
	v, e := NewDecimal(big.NewRat(n, d))
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func stateKnown[T any](v T) Evidence[T] { return Evidence[T]{State: EvidenceKnown, Value: v} }
func stateAbsent[T any]() Evidence[T]   { return Evidence[T]{State: EvidenceAbsent} }
func statePosition(t *testing.T, m uint64, n, d int64) musicalPosition {
	return musicalPosition{Part: NodeRef{DocumentID: "d", Node: 1}, Measure: NodeRef{DocumentID: "d", Node: NodeID(10 + m)}, MeasureOrdinal: m, Offset: mustRational(t, n, d)}
}
func stateClose(t *testing.T, s *Session, h *timelineState, through uint64) {
	t.Helper()
	if e := h.closePart(context.Background(), s, NodeRef{DocumentID: "d", Node: 1}, through, stateKnown(true)); e != nil {
		t.Fatal(e)
	}
}
func statePublish[T any](t *testing.T, s *Session, h *stateHistory[T], pos musicalPosition, scope stateScope, v Evidence[T], node NodeID) {
	t.Helper()
	e := publishState(context.Background(), s, h, statePublication[T]{Position: pos, Scope: scope, Value: v, Provenance: []stateProvenance{{Node: NodeRef{DocumentID: "d", Node: node}, Kind: stateExplicit}}})
	if e != nil {
		t.Fatal(e)
	}
}
func stateIdentity[T any](v T) Evidence[T] { return stateKnown(v) }
func stateQuery[T any](t *testing.T, s *Session, h *timelineState, history *stateHistory[T], pos musicalPosition, staff Evidence[Integer]) stateResult[T] {
	t.Helper()
	r, e := queryState(context.Background(), s, h, history, pos, staff, stateIdentity[T])
	if e != nil {
		t.Fatal(e)
	}
	return r
}

func TestTimelineStateScoreOrderAndOverlappingScopes(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 2)
	all := stateScope{AllStaves: true}
	staff1, staff2 := stateKnown(stateInt(t, 1)), stateKnown(stateInt(t, 2))
	// Lexical order is intentionally different from musical order.
	statePublish(t, s, &h.Keys, statePosition(t, 0, 2, 1), all, stateKnown(keyState{Traditional: true, Fifths: stateKnown(stateInt(t, 3))}), 103)
	statePublish(t, s, &h.Keys, statePosition(t, 0, 0, 1), all, stateKnown(keyState{Traditional: true, Fifths: stateKnown(stateInt(t, 0))}), 101)
	statePublish(t, s, &h.Keys, statePosition(t, 0, 1, 1), stateScope{Staff: staff2.Value}, stateKnown(keyState{Traditional: true, Fifths: stateKnown(stateInt(t, 2))}), 102)
	for _, tc := range []struct {
		at    int64
		staff Evidence[Integer]
		want  string
	}{{0, staff1, "0"}, {1, staff1, "0"}, {1, staff2, "2"}, {2, staff1, "3"}, {2, staff2, "3"}} {
		r := stateQuery(t, s, h, &h.Keys, statePosition(t, 0, tc.at, 1), tc.staff)
		if r.Value.State != EvidenceKnown || r.Value.Value.Fifths.Value.String() != tc.want {
			t.Errorf("at%d staff%s = %+v", tc.at, tc.staff.Value, r)
		}
	}
	r := stateQuery(t, s, h, &h.Keys, statePosition(t, 1, 0, 1), staff2)
	if r.Value.Value.Fifths.Value.String() != "3" || !r.Inherited {
		t.Fatalf("carry = %+v", r)
	}
	other := statePosition(t, 0, 2, 1)
	other.Part.Node = 2
	r = stateQuery(t, s, h, &h.Keys, other, staff1)
	if r.Value.State != EvidenceUnknown {
		t.Fatalf("other part leaked: %+v", r)
	}
}
func TestTimelineStateClosureAbsenceAndCountDefaults(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	at := statePosition(t, 0, 0, 1)
	r, e := h.effectiveStaves(context.Background(), s, at)
	if e != nil || r.Value.State != EvidenceUnknown {
		t.Fatalf("unclosed count=%+v,%v", r, e)
	}
	stateClose(t, s, h, 2)
	r, e = h.effectiveStaves(context.Background(), s, at)
	if e != nil || r.Value.State != EvidenceKnown || r.Value.Value.String() != "1" || len(r.Provenance) != 1 || r.Provenance[0].Kind != stateInitialDefault {
		t.Fatalf("default=%+v,%v", r, e)
	}
	for _, history := range []*stateHistory[Decimal]{&h.Divisions} {
		r := stateQuery(t, s, h, history, at, stateKnown(stateInt(t, 1)))
		if r.Value.State != EvidenceAbsent {
			t.Fatalf("invented divisions: %+v", r)
		}
	}
	key := stateQuery(t, s, h, &h.Keys, at, stateKnown(stateInt(t, 1)))
	if key.Value.State != EvidenceAbsent {
		t.Fatalf("invented key %+v", key)
	}
	statePublish(t, s, &h.Staves, at, stateScope{AllStaves: true}, stateKnown(stateInt(t, 0)), 55)
	r, e = h.effectiveStaves(context.Background(), s, statePosition(t, 2, 0, 1))
	if e != nil || r.Value.Value.String() != "0" {
		t.Fatalf("legal zero reset %+v,%v", r, e)
	}
}
func TestTimelineStateSamePositionMergeConflictAndProjection(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 0)
	at := statePosition(t, 0, 1, 3)
	all := stateScope{AllStaves: true}
	staff := stateKnown(stateInt(t, 1))
	a := transposeState{Chromatic: stateKnown(stateDecimal(t, -2, 1)), Diatonic: stateKnown(stateInt(t, -1)), OctaveChange: stateKnown(stateInt(t, -1))}
	statePublish(t, s, &h.Transpose, at, all, stateKnown(a), 11)
	// Equal mathematical positions must merge even though their pointers differ.
	statePublish(t, s, &h.Transpose, statePosition(t, 0, 2, 6), all, stateKnown(a), 12)
	r := stateQuery(t, s, h, &h.Transpose, at, staff)
	if r.Value.State != EvidenceKnown || len(r.Provenance) != 2 {
		t.Fatalf("merge %+v", r)
	}
	b := a
	b.Diatonic = stateKnown(stateInt(t, 5))
	statePublish(t, s, &h.Transpose, at, stateScope{Staff: staff.Value}, stateKnown(b), 13)
	r = stateQuery(t, s, h, &h.Transpose, at, staff)
	if r.Value.State != EvidenceUnknown || r.Value.Reason != "timeline-state-conflict" {
		t.Fatalf("conflict %+v", r)
	}
	chrom, e := queryState(context.Background(), s, h, &h.Transpose, at, staff, func(v transposeState) Evidence[Decimal] { return v.Chromatic })
	if e != nil || chrom.Value.State != EvidenceKnown || chrom.Value.Value.String() != "-2" || len(chrom.Provenance) != 3 {
		t.Fatalf("independent component %+v,%v", chrom, e)
	}
}
func TestTimelineStateInvalidMaskGapAndRecovery(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 3)
	all := stateScope{AllStaves: true}
	staff := stateKnown(stateInt(t, 1))
	cause := &DecisionRef{Rule: RuleKey{ID: "xsd", Revision: 1}, Subject: NodeRef{DocumentID: "d", Node: 33}}
	statePublish(t, s, &h.Divisions, statePosition(t, 0, 0, 1), all, stateKnown(stateDecimal(t, 4, 1)), 10)
	statePublish(t, s, &h.Divisions, statePosition(t, 0, 1, 1), all, Evidence[Decimal]{State: EvidenceInvalid, Reason: "bad-divisions", Cause: cause}, 11)
	r := stateQuery(t, s, h, &h.Divisions, statePosition(t, 0, 2, 1), staff)
	if r.Value.State != EvidenceInvalid || r.Value.Cause == nil || *r.Value.Cause != *cause {
		t.Fatalf("invalid disappeared %+v", r)
	}
	statePublish(t, s, &h.Divisions, statePosition(t, 0, 3, 1), all, stateKnown(stateDecimal(t, 8, 1)), 12)
	r = stateQuery(t, s, h, &h.Divisions, statePosition(t, 0, 3, 1), staff)
	if r.Value.State != EvidenceKnown || r.Value.Value.String() != "8" {
		t.Fatalf("no recovery %+v", r)
	}
	if e := addStateGap(context.Background(), s, &h.Divisions, stateGap{Part: statePosition(t, 0, 0, 1).Part, FirstMeasure: 1, LastMeasure: 1, Scope: all, Availability: Evidence[bool]{State: EvidenceInvalid, Reason: "unplaced-divisions", Cause: cause}}); e != nil {
		t.Fatal(e)
	}
	r = stateQuery(t, s, h, &h.Divisions, statePosition(t, 2, 0, 1), staff)
	if r.Value.State != EvidenceInvalid || r.Value.Reason != "unplaced-divisions" {
		t.Fatalf("gap disappeared %+v", r)
	}
	statePublish(t, s, &h.Divisions, statePosition(t, 2, 0, 1), all, stateKnown(stateDecimal(t, 3, 1)), 13)
	r = stateQuery(t, s, h, &h.Divisions, statePosition(t, 3, 0, 1), staff)
	if r.Value.State != EvidenceKnown || r.Value.Value.String() != "3" {
		t.Fatalf("gap failed recovery %+v", r)
	}
}
func TestTimelineStateUnknownAndHugeStaffRemainSparse(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 1)
	at := statePosition(t, 0, 0, 1)
	huge, _ := NewInteger(new(big.Int).Lsh(big.NewInt(1), 256))
	statePublish(t, s, &h.Staves, at, stateScope{AllStaves: true}, stateKnown(huge), 1)
	statePublish(t, s, &h.Keys, at, stateScope{Staff: huge}, stateKnown(keyState{Traditional: true, Fifths: stateKnown(stateInt(t, 2))}), 2)
	r, e := h.effectiveKey(context.Background(), s, at, stateAbsent[Integer]())
	if e != nil || r.Value.State != EvidenceUnknown || r.Value.Reason != "timeline-staff-unresolved" {
		t.Fatalf("implicit staff1 %+v,%v", r, e)
	}
	r, e = h.effectiveKey(context.Background(), s, at, stateKnown(huge))
	if e != nil || r.Value.State != EvidenceKnown {
		t.Fatalf("huge exact staff %+v,%v", r, e)
	}
	if len(h.Keys.Publications) != 1 || rationalUsage(t, s, BudgetBufferedBytes) > 100000 {
		t.Fatal("staff-count-sized storage")
	}
}
func TestTimelineStateTypedCompositesAndCopyIsolation(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 0)
	at := statePosition(t, 0, 0, 1)
	scope := stateScope{AllStaves: true}
	staff := stateKnown(stateInt(t, 1))
	key := keyState{Alterations: []keyAlteration{{Step: stateKnown("F"), Alter: stateKnown(stateDecimal(t, 1, 2)), Accidental: stateKnown("quarter-sharp")}}, Octaves: []keyOctave{{Number: stateKnown(stateInt(t, 1)), Octave: stateKnown(stateInt(t, 4))}}}
	statePublish(t, s, &h.Keys, at, scope, stateKnown(key), 1)
	key.Alterations[0].Step.Value = "G"
	r := stateQuery(t, s, h, &h.Keys, at, staff)
	if r.Value.Value.Alterations[0].Step.Value != "F" || r.Value.Value.Alterations[0].Alter.Value.String() != "0.5" {
		t.Fatalf("compound alias %+v", r)
	}
	time := timeState{Pairs: []timePair{{Beats: stateKnown("2"), BeatType: stateKnown("4")}, {Beats: stateKnown("3+2"), BeatType: stateKnown("8")}}, Interchangeable: stateKnown(interchangeableState{Pairs: []timePair{{Beats: stateKnown("6"), BeatType: stateKnown("8")}}, Relation: stateKnown("equals")})}
	statePublish(t, s, &h.Times, at, scope, stateKnown(time), 2)
	time.Pairs[1].Beats.Value = "100"
	time.Interchangeable.Value.Pairs[0].Beats.Value = "999"
	tr := stateQuery(t, s, h, &h.Times, at, staff)
	if tr.Value.Value.Pairs[1].Beats.Value != "3+2" || tr.Value.Value.Interchangeable.Value.Pairs[0].Beats.Value != "6" {
		t.Fatalf("time alias %+v", tr)
	}
	statePublish(t, s, &h.Times, statePosition(t, 0, 1, 1), scope, stateKnown(timeState{SenzaMisura: stateKnown("X")}), 3)
	tr = stateQuery(t, s, h, &h.Times, statePosition(t, 0, 1, 1), staff)
	if tr.Value.State != EvidenceKnown || tr.Value.Value.SenzaMisura.Value != "X" {
		t.Fatalf("unmetered absent %+v", tr)
	}
}
func TestTimelineStateClefAndCreatedPartSeparation(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 0)
	at := statePosition(t, 0, 0, 1)
	staff := stateKnown(stateInt(t, 1))
	scope := stateScope{Staff: staff.Value}
	normal := clefState{Sign: stateKnown("G"), Line: stateKnown(stateInt(t, 2))}
	added := clefState{Sign: stateKnown("F"), Line: stateKnown(stateInt(t, 4)), Additional: stateKnown(true)}
	statePublish(t, s, &h.Clefs, at, scope, stateKnown(normal), 1)
	statePublish(t, s, &h.AdditionalClefs, at, scope, stateKnown(added), 2)
	statePublish(t, s, &h.CreatedPartClefs, at, stateScope{AllStaves: true}, stateKnown(clefState{Sign: stateKnown("C")}), 3)
	r := stateQuery(t, s, h, &h.Clefs, at, staff)
	if r.Value.Value.Sign.Value != "G" {
		t.Fatalf("primary overwritten %+v", r)
	}
	ar := stateQuery(t, s, h, &h.AdditionalClefs, at, staff)
	if ar.Value.Value.activeLine().State != EvidenceAbsent || ar.Value.Value.Line.Value.String() != "4" {
		t.Fatalf("additional line active %+v", ar)
	}
}
func TestTimelineStateTransposeContributions(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	v := transposeState{Chromatic: stateKnown(stateDecimal(t, -2, 1)), Diatonic: stateKnown(stateInt(t, -1)), OctaveChange: stateKnown(stateInt(t, -1)), Double: stateKnown(doublingState{Above: stateKnown(true)})}
	c, d, e := v.contributions(context.Background(), s)
	if e != nil || c.State != EvidenceKnown || c.Value.String() != "-14" || d.Value.String() != "-8" {
		t.Fatalf("contributions %v,%v,%v", c, d, e)
	}
	v.Diatonic = stateAbsent[Integer]()
	c, d, e = v.contributions(context.Background(), s)
	if e != nil || c.Value.String() != "-14" || d.State != EvidenceAbsent {
		t.Fatalf("invented spelling %v,%v,%v", c, d, e)
	}
}
func TestTimelineStateResourceAndCancellationAtomicity(t *testing.T) {
	s := rationalSession(t, Budgets{MaxContextFacts: 1})
	defer s.cleanup()
	h := stateHistory[Integer]{}
	at := statePosition(t, 0, 0, 1)
	p := statePublication[Integer]{Position: at, Scope: stateScope{AllStaves: true}, Value: stateKnown(stateInt(t, 1))}
	if e := publishState(context.Background(), s, &h, p); e != nil {
		t.Fatal(e)
	}
	if e := publishState(context.Background(), s, &h, p); !errors.Is(e, ErrResourceLimit) || len(h.Publications) != 1 {
		t.Fatalf("publication atomicity %v %+v", e, h)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if e := publishState(cancelled, s, &h, p); !errors.Is(e, context.Canceled) || len(h.Publications) != 1 {
		t.Fatalf("cancel %v", e)
	}
}
