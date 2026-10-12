package validation

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"unsafe"
)

func independentPartialFixture(t *testing.T, invalid bool) (*Session, *timelineState, musicalPosition) {
	t.Helper()
	s := rationalSession(t, Budgets{})
	h := &timelineState{}
	stateClose(t, s, h, 2)
	at := statePosition(t, 0, 0, 1)
	later := statePosition(t, 1, -1, 1)
	later.UnprovedFrame = true
	value := stateKnown(stateDecimal(t, 3, 1))
	if invalid {
		value = Evidence[Decimal]{State: EvidenceInvalid, Reason: "actual-failure", Cause: &DecisionRef{Rule: RuleKey{ID: "actual", Revision: 1}, Subject: NodeRef{"d", 90}}}
	}
	for i, p := range []statePublication[Decimal]{{Position: at, Scope: stateScope{AllStaves: true}, Value: stateKnown(stateDecimal(t, 3, 1))}, {Position: later, Scope: stateScope{AllStaves: true}, Value: value}} {
		p.Provenance = []stateProvenance{{Node: NodeRef{"d", NodeID(80 + i)}, Kind: stateExplicit}}
		if err := publishState(context.Background(), s, &h.Divisions, p); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := stateIndex(context.Background(), s, &h.Divisions); err != nil {
		t.Fatal(err)
	}
	return s, h, at
}

func TestIndependentTimelinePartialQueryCauseAndScratch(t *testing.T) {
	if statePossiblePublicationBytes < int64(unsafe.Sizeof(statePossiblePublication{})) {
		t.Fatal("candidate width exceeds exact-cap allowance")
	}
	for _, invalid := range []bool{false, true} {
		s, h, at := independentPartialFixture(t, invalid)
		before := rationalUsage(t, s, BudgetBufferedBytes)
		contexts := rationalUsage(t, s, BudgetContextFacts)
		got, err := queryStateAtFull(context.Background(), s, h, &h.Divisions, at, stateKnown(stateOne()), stateWhole[Decimal], true, false, nil)
		if err != nil {
			t.Fatal(err)
		}
		extra := int64(0)
		if invalid {
			extra = 128
			if got.Value.State != EvidenceInvalid || got.Value.Cause == nil || got.Value.Cause == h.Divisions.Publications[1].Value.Cause {
				t.Fatal("actual cause was downgraded or borrowed")
			}
			got.Value.Cause.Subject.Node = 999
			again, err := queryStateAtFull(context.Background(), s, h, &h.Divisions, at, stateKnown(stateOne()), stateWhole[Decimal], true, false, nil)
			if err != nil || again.Value.Cause == nil || again.Value.Cause.Subject.Node != 90 {
				t.Fatal("returned cause mutation contaminated retained history")
			}
			extra += 128
		} else if got.Value.State != EvidenceKnown || got.Value.Value.String() != "3" {
			t.Fatalf("definite identical-value consensus lost: %+v", got)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != before+extra || s.retained != before+extra {
			t.Fatal("exceptional candidate or rational scratch leaked")
		}
		if rationalUsage(t, s, BudgetContextFacts) != contexts {
			t.Fatal("query fabricated context publications")
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("cleanup leaked")
		}
	}
}

func TestIndependentTimelinePartialQueryEveryCancellationBoundary(t *testing.T) {
	s, h, at := independentPartialFixture(t, true)
	probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	if _, err := queryStateAtFull(probe, s, h, &h.Divisions, at, stateKnown(stateOne()), stateWhole[Decimal], true, false, nil); err != nil {
		t.Fatal(err)
	}
	s.cleanup()
	for poll := 1; poll <= probe.calls; poll++ {
		s, h, at := independentPartialFixture(t, true)
		before, index := rationalUsage(t, s, BudgetBufferedBytes), h.Divisions.index
		ctx := &rationalCancelContext{Context: context.Background(), at: poll}
		_, err := queryStateAtFull(ctx, s, h, &h.Divisions, at, stateKnown(stateOne()), stateWhole[Decimal], true, false, nil)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("poll%d changed cancellation: %v", poll, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained || s.retained != before || h.Divisions.index != index {
			t.Fatalf("poll%d leaked scratch or replaced retained index", poll)
		}
		if h.Divisions.Publications[1].Value.Cause.Subject.Node != 90 || h.Divisions.Publications[1].Position.Offset.String() != "-1" {
			t.Fatal("cancelled query mutated raw state")
		}
		if _, err := queryStateAtFull(context.Background(), s, h, &h.Divisions, at, stateKnown(stateOne()), stateWhole[Decimal], true, false, nil); err != nil {
			t.Fatalf("retry: %v", err)
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("cleanup leaked")
		}
	}
	t.Logf("all%d exceptional query cancellation polls", probe.calls)
}

func TestIndependentTimelinePartialCandidateAdmissionBeforeTraversal(t *testing.T) {
	// The injected immutable fixture/index isolates candidate admission from the
	// independently tested publication/index builder. Positions need not be read
	// before this deliberately insufficient scratch charge is rejected.
	h := &timelineState{}
	at := statePosition(t, 0, 0, 1)
	history := stateHistory[Decimal]{Publications: make([]statePublication[Decimal], 2)}
	key := stateScopeKey(at.Part, stateScope{AllStaves: true})
	history.index = &stateHistoryIndex{Count: 2, Scopes: map[stateIndexScope]stateIndexBucket{key: {Publications: []int{0, 1}, Partial: true}}}
	needed := 2*statePossiblePublicationBytes + 256
	s := rationalSession(t, Budgets{MaxBufferedBytes: needed - 1})
	defer s.cleanup()
	_, err := queryPartialState(context.Background(), s, h, &history, at, stateKnown(stateOne()), stateWhole[Decimal], true, false, nil)
	if !errors.Is(err, ErrResourceLimit) || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatalf("candidate scratch not rejected before traversal: %v", err)
	}
}

func TestIndependentTimelineCompareSumLiteralLimits(t *testing.T) {
	one, zero := mustRational(t, 1, 1), mustRational(t, 0, 1)
	// Independently derived add1+0:196work/4736scratch; compare1:256/4800.
	// The temporary sum remains live throughout comparison: peak9536, scalar6.
	for _, tc := range []struct {
		name string
		b    Budgets
		pass bool
	}{{"exact", Budgets{MaxRuleSteps: 452, MaxScalarBytes: 6, MaxBufferedBytes: 9536}, true}, {"work-minus1", Budgets{MaxRuleSteps: 451}, false}, {"scalar-minus1", Budgets{MaxScalarBytes: 5}, false}, {"peak-minus1", Budgets{MaxBufferedBytes: 9535}, false}} {
		t.Run(tc.name, func(t *testing.T) {
			s := rationalSession(t, tc.b)
			defer s.cleanup()
			cmp, err := stateCompareSum(context.Background(), s, one, one, zero)
			if tc.pass {
				if err != nil || cmp != 0 || rationalUsage(t, s, BudgetRuleSteps) != 452 {
					t.Fatalf("literal boundary: cmp%d err%v work%d", cmp, err, rationalUsage(t, s, BudgetRuleSteps))
				}
			} else if !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("limit bypass: %v", err)
			}
			if rationalUsage(t, s, BudgetBufferedBytes) != 0 || s.retained != 0 {
				t.Fatal("unescaped sum was retained")
			}
		})
	}
	values := []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(-1, 1), big.NewRat(1, 3), big.NewRat(-2, 5), new(big.Rat).SetFrac(new(big.Int).Lsh(big.NewInt(1), 4096), big.NewInt(3))}
	pairs := 0
	for _, av := range values {
		for _, bv := range values {
			for _, cv := range values {
				a, _ := NewRational(av)
				b, _ := NewRational(bv)
				c, _ := NewRational(cv)
				s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 40})
				cmp, err := stateCompareSum(context.Background(), s, a, b, c)
				want := av.Cmp(new(big.Rat).Add(bv, cv))
				if err != nil || cmp != want || a.value.Cmp(av) != 0 || b.value.Cmp(bv) != 0 || c.value.Cmp(cv) != 0 {
					t.Fatalf("independent sum comparison%d: got%d want%d err%v", pairs, cmp, want, err)
				}
				if rationalUsage(t, s, BudgetBufferedBytes) != 0 || s.retained != 0 {
					t.Fatal("oracle scratch leaked")
				}
				s.cleanup()
				pairs++
			}
		}
	}
	t.Logf("%d independent exact sum comparisons", pairs)
}

func TestIndependentTimelineFrameWitnessSurvivesChordOnset(t *testing.T) {
	s, v, b := runtimeFixture(t)
	attributes := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 1, 1)))
	direction := stateNode(b, timelineRoleDirection, 50, stateAbsent[Atomic]())
	block := runtimeBlock(b, 0, attributes, runtimeNote(t, b, 30, 1, false), direction, runtimeNote(t, b, 60, 1, true))
	if err := v.readMeasure(context.Background(), s, b, block); err != nil {
		t.Fatal(err)
	}
	at := timelineAt(block, stateKnown(mustRational(t, 1, 1))).Value
	at.UnprovedFrame = true
	member, err := v.State.frameMember(context.Background(), s, at)
	if err != nil || !member {
		t.Fatalf("chord onset erased witnessed q1 bound: member%t err%v", member, err)
	}
}

func TestIndependentTimelinePartialIndexAndFrameMapShapes(t *testing.T) {
	indexWidth := int64(unsafe.Sizeof(stateIndexScope{})) + int64(unsafe.Sizeof(stateIndexBucket{}))
	// Concurrent old/new map tables plus separately grown publication IDs.
	indexMinimum := 3*indexWidth + 4*int64(unsafe.Sizeof(int(0)))
	if stateIndexPublicationBytes < indexMinimum {
		t.Fatalf("index admission%d below changed map/bucket bound%d", stateIndexPublicationBytes, indexMinimum)
	}
	frameWidth := int64(unsafe.Sizeof(timelineFrame{})) + int64(unsafe.Sizeof(Rational{}))
	frameMinimum := 4*frameWidth + 128
	if timelineFrameReserveBytes < frameMinimum {
		t.Fatalf("frame admission%d below map bound%d", timelineFrameReserveBytes, frameMinimum)
	}
	t.Logf("index minimum%d allowance%d; frame minimum%d allowance%d", indexMinimum, stateIndexPublicationBytes, frameMinimum, timelineFrameReserveBytes)
}

func TestIndependentTimelineUnrelatedFloatingGapKeepsIndexedPath(t *testing.T) {
	for _, otherPart := range []bool{false, true} {
		for _, extraGap := range []bool{false, true} {
			s := rationalSession(t, Budgets{})
			h := &timelineState{}
			stateClose(t, s, h, 0)
			for i := int64(0); i < 128; i++ {
				if err := publishState(context.Background(), s, &h.Divisions, statePublication[Decimal]{Position: statePosition(t, 0, i, 1), Scope: stateScope{AllStaves: true}, Value: stateKnown(stateDecimal(t, 3, 1))}); err != nil {
					t.Fatal(err)
				}
			}
			at := statePosition(t, 0, 128, 1)
			if _, err := stateIndex(context.Background(), s, &h.Divisions); err != nil {
				t.Fatal(err)
			}
			query := func(extra ...[]stateGap) (stateResult[Decimal], error) {
				return queryStateAtFull(context.Background(), s, h, &h.Divisions, at, stateKnown(stateOne()), stateWhole[Decimal], true, false, nil, extra...)
			}
			work := rationalUsage(t, s, BudgetRuleSteps)
			if _, err := query(); err != nil {
				t.Fatal(err)
			}
			baseline := rationalUsage(t, s, BudgetRuleSteps) - work
			point := statePosition(t, 0, -1, 1)
			point.UnprovedFrame = true
			gap := stateGap{Position: &point, Part: point.Part, FirstMeasure: 0, LastMeasure: 0, Scope: stateScope{Staff: stateInt(t, 2)}, Availability: stateUnknown[bool]("unrelated-floating-gap")}
			if otherPart {
				point.Part = NodeRef{"d", 2}
				gap.Part = point.Part
				gap.Scope = stateScope{AllStaves: true}
			}
			extras := [][]stateGap{}
			if extraGap {
				extras = append(extras, []stateGap{gap})
			} else if err := addStateGap(context.Background(), s, &h.Divisions, gap); err != nil {
				t.Fatal(err)
			}
			work = rationalUsage(t, s, BudgetRuleSteps)
			got, err := query(extras...)
			used := rationalUsage(t, s, BudgetRuleSteps) - work
			if err != nil || got.Value.State != EvidenceKnown || got.Value.Value.String() != "3" {
				t.Fatalf("unrelated part%t extra%t changed query: %+v %v", otherPart, extraGap, got, err)
			}
			if used > 3*baseline+4096 {
				t.Errorf("unrelated part%t extra%t triggered exceptional scan: baseline%d new%d", otherPart, extraGap, baseline, used)
			}
			s.cleanup()
		}
	}
}
