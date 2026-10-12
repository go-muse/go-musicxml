package validation

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

// These tests intentionally assert ownership/resource contracts independently of
// the production measurement or selection helpers.
func TestIndependentTimelineFactContextQuota(t *testing.T) {
	s := rationalSession(t, Budgets{MaxContextFacts: 1})
	defer s.cleanup()
	service, err := s.newTimelineService(context.Background(), RuleKey{ID: "independent", Revision: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.storeFact(context.Background(), s, 1, timelineEventFact{}); err != nil {
		t.Fatal(err)
	}
	before := rationalUsage(t, s, BudgetBufferedBytes)
	err = service.storeFact(context.Background(), s, 2, timelineEventFact{})
	if !errors.Is(err, ErrResourceLimit) || !errors.Is(err, ErrIncompleteAssessment) || len(service.Facts) != 1 || rationalUsage(t, s, BudgetBufferedBytes) != before {
		t.Fatalf("second context publication must fail transactionally: err=%v facts=%d context=%d bytes_delta=%d", err, len(service.Facts), rationalUsage(t, s, BudgetContextFacts), rationalUsage(t, s, BudgetBufferedBytes)-before)
	}
}

func TestIndependentTimelineUnavailableQueryOwnsCause(t *testing.T) {
	for _, source := range []string{"closure", "gap", "instrument-closure", "instrument-gap", "staff-query"} {
		t.Run(source, func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			h := &timelineState{}
			at := statePosition(t, 0, 1, 3)
			cause := &DecisionRef{Rule: RuleKey{ID: "upstream", Revision: 1}, Subject: NodeRef{"d", 77}}
			unavailable := Evidence[bool]{State: EvidenceInvalid, Reason: "upstream-failure", Cause: cause}
			if source == "closure" || source == "instrument-closure" {
				if err := h.closePart(context.Background(), s, at.Part, 0, unavailable); err != nil {
					t.Fatal(err)
				}
			} else {
				stateClose(t, s, h, 0)
			}
			if source == "gap" {
				if err := addStateGap(context.Background(), s, &h.Staves, stateGap{Part: at.Part, Scope: stateScope{AllStaves: true}, Availability: unavailable}); err != nil {
					t.Fatal(err)
				}
			}
			if source == "instrument-gap" {
				if err := h.gapAll(context.Background(), s, at.Part, 0, stateUnavailable[bool, musicalPosition](unavailable)); err != nil {
					t.Fatal(err)
				}
			}
			query := func() *DecisionRef {
				if source == "instrument-closure" || source == "instrument-gap" {
					got, err := h.effectiveInstrument(context.Background(), s, at, NodeRef{"d", 9})
					if err != nil {
						t.Fatal(err)
					}
					return got.Value.Cause
				}
				staff := stateKnown(stateInt(t, 1))
				if source == "staff-query" {
					staff = stateUnavailable[bool, Integer](unavailable)
				}
				got, err := queryState(context.Background(), s, h, &h.Staves, at, staff, stateWhole[Integer])
				if err != nil {
					t.Fatal(err)
				}
				return got.Value.Cause
			}
			first := query()
			if first == nil {
				t.Fatal("missing causal evidence")
			}
			first.Subject.Node = 999
			second := query()
			if second == nil || second.Subject.Node != 77 {
				t.Fatalf("caller mutation escaped into retained/input evidence: second=%+v input=%+v", second, cause)
			}
		})
	}
}

func TestIndependentTimelinePublicationContextBoundaries(t *testing.T) {
	for _, family := range []string{"state", "gap", "closure", "definition", "note-instruments", "instrument-change"} {
		t.Run(family, func(t *testing.T) {
			s := rationalSession(t, Budgets{MaxContextFacts: 1})
			defer s.cleanup()
			h := &timelineState{}
			at := statePosition(t, 0, 1, 3)
			publish := func() error {
				switch family {
				case "state":
					return publishState(context.Background(), s, &h.Staves, statePublication[Integer]{Position: at, Scope: stateScope{AllStaves: true}, Value: stateKnown(stateInt(t, 1))})
				case "gap":
					return addStateGap(context.Background(), s, &h.Staves, stateGap{Part: at.Part, Scope: stateScope{AllStaves: true}, Availability: stateUnknown[bool]("gap")})
				case "closure":
					return h.closePart(context.Background(), s, at.Part, 0, stateKnown(true))
				case "definition":
					return h.addInstrumentDefinition(context.Background(), s, instrumentDefinition{Part: at.Part, Instrument: NodeRef{"d", 9}})
				case "note-instruments":
					return h.addNoteInstruments(context.Background(), s, noteInstrumentSet{Part: at.Part, Note: NodeRef{"d", 10}})
				default:
					return h.addInstrumentChange(context.Background(), s, NodeRef{"d", 9}, statePublication[instrumentState]{Position: at, Scope: stateScope{AllStaves: true}})
				}
			}
			if err := publish(); err != nil {
				t.Fatal(err)
			}
			if err := publish(); !errors.Is(err, ErrResourceLimit) || !errors.Is(err, ErrIncompleteAssessment) {
				t.Fatalf("second publication err=%v", err)
			}
			if got := rationalUsage(t, s, BudgetContextFacts); got != 1 {
				t.Fatalf("context=%d", got)
			}
		})
	}
}

func TestIndependentTimelineHugeSparseNondecimalSelection(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 0)
	huge, err := NewInteger(new(big.Int).Lsh(big.NewInt(1), 8192))
	if err != nil {
		t.Fatal(err)
	}
	at := statePosition(t, 0, 1, 3)
	statePublish(t, s, &h.Staves, at, stateScope{AllStaves: true}, stateKnown(huge), 90)
	statePublish(t, s, &h.Keys, at, stateScope{Staff: huge}, stateKnown(keyState{Traditional: true, Fifths: stateKnown(stateInt(t, -3))}), 91)
	got, err := h.effectiveKey(context.Background(), s, statePosition(t, 0, 2, 3), stateKnown(huge))
	if err != nil || got.Value.State != EvidenceKnown || got.Value.Value.Fifths.Value.String() != "-3" || !got.Inherited {
		t.Fatalf("sparse rational query=%+v err=%v", got, err)
	}
	if len(h.Keys.index.Scopes) != 1 || len(h.Staves.Publications) != 1 {
		t.Fatal("expanded encoded staff magnitude")
	}
	if used := rationalUsage(t, s, BudgetBufferedBytes); used > 100000 {
		t.Fatalf("sparse sample unexpected retained bytes=%d", used)
	}
}

func TestIndependentTimelineCheckedCountOverflow(t *testing.T) {
	s := rationalSession(t, Budgets{MaxBufferedBytes: math.MaxInt64})
	defer s.cleanup()
	if err := stateRetainCount(s, 2, math.MaxInt64); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("multiplication overflow=%v", err)
	}
	if got := rationalUsage(t, s, BudgetBufferedBytes); got != 0 {
		t.Fatalf("overflow acquired bytes=%d", got)
	}
	if _, err := AllocationSize(math.MaxInt64); uint64(^uint(0)>>1) < math.MaxInt64 && !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("host int truncation err=%v", err)
	}
}
