package validation

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"runtime"
	"strconv"
	"strings"
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

func TestIndependentTimelineEveryIndexCancellationBoundary(t *testing.T) {
	probe, h := stateIndexFixture(t, Budgets{})
	counter := &rationalCancelContext{Context: context.Background(), at: math.MaxInt}
	if _, err := stateIndex(counter, probe, h); err != nil {
		t.Fatal(err)
	}
	calls := counter.calls
	probe.cleanup()
	if calls < 30 {
		t.Fatalf("too few observed index poll boundaries: %d", calls)
	}
	for at := 1; at <= calls; at++ {
		s, h := stateIndexFixture(t, Budgets{})
		before := rationalUsage(t, s, BudgetBufferedBytes)
		workBefore := rationalUsage(t, s, BudgetRuleSteps)
		ctx := &rationalCancelContext{Context: context.Background(), at: at}
		index, err := stateIndex(ctx, s, h)
		if !errors.Is(err, context.Canceled) || index != nil || h.index != nil || rationalUsage(t, s, BudgetBufferedBytes) != before || rationalUsage(t, s, BudgetRuleSteps) < workBefore {
			t.Fatalf("poll %d leaked/published/reset state: index=%p err=%v bytes=%d want=%d", at, index, err, rationalUsage(t, s, BudgetBufferedBytes), before)
		}
		for i, p := range h.Publications {
			if p.Value.Value.String() != fmt.Sprint(i) || p.Position.Offset.String() != fmt.Sprint(8-i) {
				t.Fatalf("poll%d changed publication order/value", at)
			}
		}
		rebuilt, err := stateIndex(context.Background(), s, h)
		if err != nil || rebuilt.Count != 8 {
			t.Fatalf("retry poll%d: %v", at, err)
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatalf("poll%d leaked final gauge", at)
		}
	}
	t.Logf("canceled and retried every one of %d index polling boundaries", calls)
}

func TestIndependentTimelineStateValueOutputIsolation(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	h := &timelineState{}
	stateClose(t, s, h, 0)
	cause := &DecisionRef{Rule: RuleKey{ID: "input", Revision: 1}, Subject: NodeRef{"d", 70}}
	value := timeState{Pairs: []timePair{{Beats: stateKnown("3+2"), BeatType: stateKnown("8")}}, Interchangeable: stateKnown(interchangeableState{Pairs: []timePair{{Beats: Evidence[string]{State: EvidenceInvalid, Cause: cause}, BeatType: stateKnown("4")}}})}
	statePublish(t, s, &h.Times, statePosition(t, 0, 1, 3), stateScope{AllStaves: true}, stateKnown(value), 99)
	cause.Subject.Node = 999
	value.Pairs[0].Beats.Value = "99"
	got := stateQuery(t, s, h, &h.Times, statePosition(t, 0, 2, 3), stateKnown(stateInt(t, 1)))
	if got.Value.Value.Pairs[0].Beats.Value != "3+2" || got.Value.Value.Interchangeable.Value.Pairs[0].Beats.Cause.Subject.Node != 70 {
		t.Fatal("publication retained mutable input")
	}
	got.Value.Value.Pairs[0].Beats.Value = "999"
	got.Value.Value.Interchangeable.Value.Pairs[0].Beats.Cause.Subject.Node = 999
	got.Provenance[0].Node.Node = 999
	again := stateQuery(t, s, h, &h.Times, statePosition(t, 0, 2, 3), stateKnown(stateInt(t, 1)))
	if again.Value.Value.Pairs[0].Beats.Value != "3+2" || again.Value.Value.Interchangeable.Value.Pairs[0].Beats.Cause.Subject.Node != 70 || again.Provenance[0].Node.Node != 99 {
		t.Fatal("query retained mutable output")
	}
}

func TestIndependentTimelineIntegerEmbeddingScalarBoundary(t *testing.T) {
	v, err := NewInteger(new(big.Int).Lsh(big.NewInt(1), 8192))
	if err != nil {
		t.Fatal(err)
	}
	for _, limit := range []int64{v.ByteSize() - 1, v.ByteSize(), v.ByteSize() + 1} {
		s := rationalSession(t, Budgets{MaxScalarBytes: limit})
		got, err := s.stateRationalInteger(context.Background(), v)
		if limit < v.ByteSize() {
			if !errors.Is(err, ErrResourceLimit) || got.Valid() || rationalUsage(t, s, BudgetBufferedBytes) != 0 || rationalUsage(t, s, BudgetRuleSteps) != 0 {
				t.Fatalf("scalar precheck: limit=%d err=%v", limit, err)
			}
		} else if err != nil && !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("wrong enlarged rational bound error: %v", err)
		}
		// The rational planner may conservatively need a larger scalar envelope than
		// the integer's input payload; it must never fabricate a musical failure.
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("integer embedding leaked scratch")
		}
	}
}

func TestIndependentTimelineSharedAccountCleanupIsolation(t *testing.T) {
	account, err := NewBudgetAccount(Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	a, b := &Session{account: account}, &Session{account: account}
	for i, s := range []*Session{a, b} {
		service, err := s.newTimelineService(context.Background(), RuleKey{ID: "shared-immutable-plan", Revision: 1})
		if err != nil {
			t.Fatal(err)
		}
		for j := 0; j <= i; j++ {
			if err := service.storeFact(context.Background(), s, NodeID(j+1), timelineEventFact{}); err != nil {
				t.Fatal(err)
			}
		}
	}
	retainedB := b.retained
	a.cleanup()
	if a.timeline != nil || b.timeline == nil || rationalUsage(t, b, BudgetBufferedBytes) != retainedB {
		t.Fatal("cleanup reset another session's gauge/state")
	}
	a.cleanup()
	b.cleanup()
	if b.timeline != nil || rationalUsage(t, b, BudgetBufferedBytes) != 0 {
		t.Fatal("cleanup left timeline gauge")
	}
}

func TestIndependentTimelineLiteralEmbeddingInteractions(t *testing.T) {
	// For integer 2: input bytes=2, outer embedding allowance=2+128=130.
	// One-limb numerator/denominator in and out + one four-byte text chunk +
	// fixed unit => 4*(1+1+1+1+1+1)^2 = 144 rational work units.
	// Rational scalar envelope is two magnitude bytes plus two text bytes=4.
	// Scratch is 4096+64*(1+1+0+0+2+2+1)=4544; no production cost helper.
	for _, work := range []int64{129, 130, 273, 274, 275} {
		s := rationalSession(t, Budgets{MaxRuleSteps: work})
		v, err := s.stateRationalInteger(context.Background(), stateInt(t, 2))
		if work < 274 {
			if !errors.Is(err, ErrResourceLimit) || v.Valid() || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatalf("work%d output=%s err=%v", work, v, err)
			}
		} else if err != nil || v.String() != "2" || rationalUsage(t, s, BudgetRuleSteps) != 274 {
			t.Fatalf("literal work%d result=%s err=%v charged=%d", work, v, err, rationalUsage(t, s, BudgetRuleSteps))
		}
		s.cleanup()
	}
	for _, scalar := range []int64{1, 2, 3, 4, 5} {
		s := rationalSession(t, Budgets{MaxScalarBytes: scalar})
		v, err := s.stateRationalInteger(context.Background(), stateInt(t, 2))
		if scalar < 4 {
			if !errors.Is(err, ErrResourceLimit) || v.Valid() || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatalf("scalar%d result=%s err=%v", scalar, v, err)
			}
		} else if err != nil || v.String() != "2" {
			t.Fatalf("scalar%d result=%s err=%v", scalar, v, err)
		}
		s.cleanup()
	}
	raw := new(big.Rat).Set(new(big.Rat).SetInt(big.NewInt(2)))
	output := int64(cap(raw.Num().Bits())+cap(raw.Denom().Bits()))*int64(strconv.IntSize/8) + 129
	peak := int64(130+4544) + output
	for _, limit := range []int64{peak - 1, peak, peak + 1} {
		s := rationalSession(t, Budgets{MaxBufferedBytes: limit})
		v, err := s.stateRationalInteger(context.Background(), stateInt(t, 2))
		if limit < peak {
			if !errors.Is(err, ErrResourceLimit) || v.Valid() || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatalf("peak%d result=%s err=%v", limit, v, err)
			}
		} else if err != nil || v.String() != "2" || rationalUsage(t, s, BudgetBufferedBytes) != output {
			t.Fatalf("peak%d result=%s err=%v bytes=%d", limit, v, err, rationalUsage(t, s, BudgetBufferedBytes))
		}
		if rationalUsage(t, s, BudgetRuleSteps) != 274 {
			t.Fatalf("peak%d refunded work", limit)
		}
		s.cleanup()
	}
}

func TestIndependentTimelineRuntimeExactModelTenth(t *testing.T) {
	s, service, b := runtimeFixture(t)
	attributes := stateNode(b, timelineRoleAttributes, 20, stateAbsent[Atomic](), stateNode(b, timelineRoleAttributesDivisions, 21, stateAtomicDecimal(t, 3, 1)))
	decimal, err := NewDecimal(new(big.Rat).SetFloat64(0.1))
	if err != nil {
		t.Fatal(err)
	}
	events := []*factNode{attributes}
	for i := 0; i < 1000; i++ {
		id := NodeID(100 + i*10)
		events = append(events, stateNode(b, timelineRoleNote, id, stateAbsent[Atomic](), stateNode(b, timelineRoleNoteDuration, id+1, stateKnown(Atomic{Kind: AtomicDecimal, Decimal: decimal}))))
	}
	if err := service.readMeasure(context.Background(), s, b, runtimeBlock(b, 0, events...)); err != nil {
		t.Fatal(err)
	}
	// Expectations from the independent standard-library value, not normalization
	// or any timeline cost/selection implementation.
	increment := new(big.Rat).Quo(new(big.Rat).SetFloat64(0.1), big.NewRat(3, 1))
	last := service.Facts[100+999*10]
	want := new(big.Rat).Mul(increment, big.NewRat(999, 1)).RatString()
	if last.Position.State != EvidenceKnown || last.Position.Value.Offset.String() != want || last.Duration.Value.String() != increment.RatString() {
		t.Fatalf("model exactness lost: got=%s duration=%s want=%s increment=%s", last.Position.Value.Offset, last.Duration.Value, want, increment)
	}
	if last.Position.Value.Offset.String() == "333/10" {
		t.Fatal("model float silently rounded to source decimal")
	}
	t.Logf("1000 exact-model notes: work=%d retained=%d last=%s", rationalUsage(t, s, BudgetRuleSteps), rationalUsage(t, s, BudgetBufferedBytes), want)
}

func TestIndependentTimelineRejectsBeforeLargePayloadClone(t *testing.T) {
	s := rationalSession(t, Budgets{MaxBufferedBytes: 1})
	defer s.cleanup()
	h := &stateHistory[timeState]{}
	payload := strings.Repeat("3", 1<<20)
	p := statePublication[timeState]{Position: statePosition(t, 0, 1, 3), Scope: stateScope{AllStaves: true}, Value: stateKnown(timeState{Pairs: []timePair{{Beats: stateKnown(payload), BeatType: stateKnown("4")}}})}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	err := publishState(context.Background(), s, h, p)
	runtime.ReadMemStats(&after)
	if !errors.Is(err, ErrResourceLimit) || !errors.Is(err, ErrIncompleteAssessment) || len(h.Publications) != 0 || h.index != nil || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatalf("rejected publication escaped: %v count=%d", err, len(h.Publications))
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 64<<10 {
		t.Fatalf("cloned large string before buffer charge: allocated=%d", allocated)
	} else {
		t.Logf("rejected 1MiB payload clone allocated only %d bytes in preflight/error handling", allocated)
	}
	if rationalUsage(t, s, BudgetRuleSteps) < 1<<20 {
		t.Fatal("large input traversal/string-copy work not metered")
	}
}

func independentOriginFixture(t *testing.T, budget Budgets) (*Session, *timelineService, *timelineBindings, timelineEventFact) {
	t.Helper()
	s := rationalSession(t, budget)
	s.documentID = "d"
	t.Cleanup(s.cleanup)
	b := stateBindings()
	service := &timelineService{Facts: map[NodeID]timelineEventFact{}}
	fact := timelineEventFact{Part: stateKnown(NodeRef{"d", 1}), Position: stateKnown(statePosition(t, 0, 0, 1)), End: stateKnown(statePosition(t, 0, 1, 3)), Duration: stateKnown(mustRational(t, 1, 3)), Staff: stateAbsent[Integer](), Voice: stateAbsent[string]()}
	return s, service, b, fact
}

func TestIndependentTimelineOriginContextQuota(t *testing.T) {
	t.Run("note", func(t *testing.T) {
		s, service, b, fact := independentOriginFixture(t, Budgets{MaxContextFacts: 1})
		if err := service.readNoteOrigins(context.Background(), s, b, runtimeNote(t, b, 100, 1, false), fact, stateKnown(stateDecimal(t, 3, 1))); err != nil {
			t.Fatal(err)
		}
		err := service.readNoteOrigins(context.Background(), s, b, runtimeNote(t, b, 200, 1, false), fact, stateKnown(stateDecimal(t, 3, 1)))
		if !errors.Is(err, ErrResourceLimit) || !errors.Is(err, ErrIncompleteAssessment) || len(service.NoteOrigins) != 1 || rationalUsage(t, s, BudgetContextFacts) != 1 {
			t.Fatalf("note context quota bypass: err=%v entries=%d context=%d", err, len(service.NoteOrigins), rationalUsage(t, s, BudgetContextFacts))
		}
	})
	t.Run("figured-bass", func(t *testing.T) {
		s, service, b, fact := independentOriginFixture(t, Budgets{MaxContextFacts: 1})
		first := stateNode(b, timelineRoleFiguredBass, 100, stateAbsent[Atomic]())
		second := stateNode(b, timelineRoleFiguredBass, 200, stateAbsent[Atomic]())
		note := runtimeNote(t, b, 300, 1, false)
		service.Facts[100], service.Facts[200], service.Facts[300] = fact, fact, fact
		err := service.resolveFiguredBassOrigins(context.Background(), s, b, runtimeBlock(b, 0, first, second, note))
		if !errors.Is(err, ErrResourceLimit) || !errors.Is(err, ErrIncompleteAssessment) || len(service.FiguredBassOrigins) != 1 || rationalUsage(t, s, BudgetContextFacts) != 1 {
			t.Fatalf("figured bass quota bypass: err=%v entries=%d context=%d", err, len(service.FiguredBassOrigins), rationalUsage(t, s, BudgetContextFacts))
		}
	})
}

func TestIndependentTimelineOriginCausalOwnership(t *testing.T) {
	t.Run("note", func(t *testing.T) {
		s, service, b, fact := independentOriginFixture(t, Budgets{})
		cause := &DecisionRef{Rule: RuleKey{ID: "fact-input", Revision: 1}, Subject: NodeRef{"d", 77}}
		unitsCause := &DecisionRef{Rule: RuleKey{ID: "units-input", Revision: 1}, Subject: NodeRef{"d", 78}}
		fact.Part = Evidence[NodeRef]{State: EvidenceInvalid, Reason: "bad-part", Cause: cause}
		fact.Position = Evidence[musicalPosition]{State: EvidenceInvalid, Reason: "bad-onset", Cause: cause}
		if err := service.readNoteOrigins(context.Background(), s, b, runtimeNote(t, b, 100, 1, false), fact, Evidence[Decimal]{State: EvidenceInvalid, Reason: "bad-units", Cause: unitsCause}); err != nil {
			t.Fatal(err)
		}
		cause.Subject.Node = 999
		unitsCause.Subject.Node = 888
		got := service.NoteOrigins[100]
		if got.Part.Cause == nil || got.Part.Cause.Subject.Node != 77 || got.ScoreOnset.Cause == nil || got.ScoreOnset.Cause.Subject.Node != 77 || got.Attack.Units.Cause == nil || got.Attack.Units.Cause.Subject.Node != 78 {
			t.Fatalf("note retained borrowed cause: part=%+v onset=%+v units=%+v", got.Part.Cause, got.ScoreOnset.Cause, got.Attack.Units.Cause)
		}
	})
	t.Run("figured-bass", func(t *testing.T) {
		s, service, b, fact := independentOriginFixture(t, Budgets{})
		cause := &DecisionRef{Rule: RuleKey{ID: "fact-input", Revision: 1}, Subject: NodeRef{"d", 77}}
		fact.Part = Evidence[NodeRef]{State: EvidenceInvalid, Reason: "bad-part", Cause: cause}
		figure := stateNode(b, timelineRoleFiguredBass, 100, stateAbsent[Atomic]())
		service.Facts[100] = fact
		block := runtimeBlock(b, 0, figure)
		block.complete = Evidence[bool]{State: EvidenceInvalid, Reason: "bad-measure", Cause: cause}
		if err := service.resolveFiguredBassOrigins(context.Background(), s, b, block); err != nil {
			t.Fatal(err)
		}
		cause.Subject.Node = 999
		got := service.FiguredBassOrigins[100]
		if got.Part.Cause == nil || got.Part.Cause.Subject.Node != 77 || got.Anchor.Cause == nil || got.Anchor.Cause.Subject.Node != 77 || got.Position.Cause == nil || got.Position.Cause.Subject.Node != 77 {
			t.Fatalf("figured bass retained borrowed cause: part=%+v anchor=%+v position=%+v", got.Part.Cause, got.Anchor.Cause, got.Position.Cause)
		}
	})
}

func TestIndependentTimelineNoteOriginCausalPrecedence(t *testing.T) {
	originCause := &DecisionRef{Rule: RuleKey{ID: "origin-proof", Revision: 1}, Subject: NodeRef{"d", 77}}
	durationCause := &DecisionRef{Rule: RuleKey{ID: "duration-proof", Revision: 1}, Subject: NodeRef{"d", 78}}
	for _, tc := range []struct {
		name   string
		origin Evidence[musicalPosition]
		delta  Evidence[Rational]
		want   *DecisionRef
	}{
		{"invalid-origin-unknown-delta", Evidence[musicalPosition]{State: EvidenceInvalid, Reason: "invalid-origin", Cause: originCause}, stateUnknown[Rational]("unknown-delta"), originCause},
		{"invalid-origin-absent-delta", Evidence[musicalPosition]{State: EvidenceInvalid, Reason: "invalid-origin", Cause: originCause}, stateAbsent[Rational](), originCause},
		{"unknown-origin-invalid-delta", stateUnknown[musicalPosition]("unknown-origin"), Evidence[Rational]{State: EvidenceInvalid, Reason: "invalid-duration", Cause: durationCause}, durationCause},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			got, err := s.noteTimingShift(context.Background(), tc.origin, tc.delta)
			if err != nil || got.State != EvidenceInvalid || got.Cause == nil || *got.Cause != *tc.want {
				t.Fatalf("strong causal failure lost: got=%+v err=%v want=%+v", got, err, tc.want)
			}
		})
	}
	t.Run("unknown-percentage-invalid-duration", func(t *testing.T) {
		s, _, b, fact := independentOriginFixture(t, Budgets{})
		bend := stateNode(b, timelineRoleTechnicalBend, 100, stateAbsent[Atomic]())
		stateAttribute(b, bend, timelineRoleBendFirstBeat, stateAtomicDecimal(t, 25, 1).Value)
		bend.attributes[0].Scalar.Value = Evidence[Atomic]{State: EvidenceUnknown, Reason: "unknown-percentage"}
		r := &stateReader{ctx: context.Background(), s: s, b: b}
		got := r.notePercent(bend, timelineRoleBendFirstBeat, noteTimingFirstBeat, fact.Position, Evidence[Rational]{State: EvidenceInvalid, Reason: "invalid-duration", Cause: durationCause})
		if r.err != nil || got.Position.State != EvidenceInvalid || got.Position.Cause == nil || *got.Position.Cause != *durationCause {
			t.Fatalf("invalid duration replaced by unknown percentage: got=%+v err=%v", got.Position, r.err)
		}
	})
}
