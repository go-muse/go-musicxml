package validation

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"runtime"
	"testing"
	"unsafe"
)

// The compact store charges each chunk one 32 KiB size class: the records of
// a chunk fit in it, and a width below a record's real size would hide one
// that does not.
func TestTimelineCompactRecordWidths(t *testing.T) {
	for _, tc := range []struct {
		name        string
		width, real int64
		perChunk    int64
	}{
		{"event", timelineCompactEventBytes, int64(unsafe.Sizeof(compactEventRecord{})), timelineCompactChunk},
		{"note", timelineCompactNoteBytes, int64(unsafe.Sizeof(compactNote{})), timelineCompactNoteChunk},
		{"play", timelineCompactPlayBytes, int64(unsafe.Sizeof(compactPlay{})), timelineCompactPlayChunk},
	} {
		if tc.width < tc.real || tc.width*tc.perChunk > timelineCompactChunkBytes {
			t.Errorf("%s record: width %d, size %d, %d per chunk of %d bytes", tc.name, tc.width, tc.real, tc.perChunk, timelineCompactChunkBytes)
		}
		t.Logf("%s record: width %d, size %d, %d bytes per chunk", tc.name, tc.width, tc.real, tc.width*tc.perChunk)
	}
	if 4*timelineCompactPageSize != 4096 {
		t.Errorf("a page of record numbers is %d bytes", 4*timelineCompactPageSize)
	}
}

func compactLiveHeap() int64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.HeapAlloc)
}

// The compact store's charge covers the heap it holds, on a store that
// dominates the heap: 24 000 facts, note timing origins and note playback
// contexts packed measure by measure, with four distinct offsets and with an
// offset of their own each, one measure per event (a frame of its own each),
// and all in one measure (as after a staged unit proof, when the working maps
// held every record). The working records' reservations are released as they
// are packed; the numbers are admitted before the baseline, as sessions admit
// them before publishing records. 64 KiB allows for the runtime's own
// allocations between two measurements.
func TestTimelineCompactStoreChargeCoversItsHeap(t *testing.T) {
	if testing.Short() {
		t.Skip("measures the heap")
	}
	const events = 24000
	// overflow selects the kinds that stay whole: 1 facts, 2 note origins,
	// 4 playback contexts.
	for _, tc := range []struct {
		distinct   bool
		overflow   int
		perMeasure int
	}{{false, 0, 48}, {true, 0, 48}, {false, 0, 1}, {false, 0, events}, {false, 1, 48}, {false, 2, 48}, {false, 4, 48}, {false, 7, 48}} {
		distinct, overflow, perMeasure := tc.distinct, tc.overflow, tc.perMeasure
		extra := int64(0) // what the test charges beyond the records' reservations
		offsets := make([]Rational, events)
		for i := range offsets {
			n := int64(i % 4)
			if distinct {
				n = int64(i)
			}
			offsets[i] = mustRational(t, n, 7)
		}
		units := Evidence[Decimal]{State: EvidenceKnown, Value: Decimal{value: big.NewRat(4, 1), text: "4", bytes: 3}}
		s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 40})
		s.documentID = "d"
		service := &timelineService{Facts: make(map[NodeID]timelineEventFact)}
		part := Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"d", 1}}
		heap0, used0 := compactLiveHeap(), rationalUsage(t, s, BudgetBufferedBytes)
		// While a pack runs, the working maps are reachable and keep their
		// reservations.
		packs := 0
		timelinePackObserver = func() {
			packs++
			if packs%50 != 1 {
				return
			}
			if heap, used := compactLiveHeap()-heap0, rationalUsage(t, s, BudgetBufferedBytes)-used0; heap > used+64<<10 {
				t.Errorf("distinct offsets %v, overflow %v, %d per measure: during a pack the run holds %d live bytes but charged %d", distinct, overflow, perMeasure, heap, used)
			}
		}
		for first := 0; first < events; first += perMeasure {
			measure := NodeRef{"d", NodeID(first + 2)}
			for i := first; i < first+perMeasure; i++ {
				id := NodeID(i + 3)
				at := Evidence[musicalPosition]{State: EvidenceKnown, Value: musicalPosition{Part: part.Value, Measure: measure, MeasureOrdinal: uint64(first), Offset: offsets[i]}}
				duration := Evidence[Rational]{State: EvidenceKnown, Value: offsets[i]}
				absent := Evidence[Decimal]{State: EvidenceAbsent}
				if err := s.retain(timelineFactReserveBytes + timelineNoteReserveBytes + playbackNoteReserveBytes); err != nil {
					t.Fatal(err)
				}
				fact := timelineEventFact{Origin: at, Position: at, End: at, Duration: duration, PrincipalDuration: duration, Principal: Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"d", id}}, Units: units, Staff: Evidence[Integer]{State: EvidenceAbsent}, Voice: Evidence[string]{State: EvidenceKnown, Value: "1"}, Part: part}
				grace := Evidence[graceTimingOrigin]{State: EvidenceAbsent}
				var play []playbackInstruction
				// None of these is encodable; each stays whole.
				if overflow&1 != 0 {
					fact.Placement.Displaced = true
				}
				if overflow&2 != 0 {
					grace = Evidence[graceTimingOrigin]{State: EvidenceKnown}
				}
				if overflow&4 != 0 {
					if err := s.retain(512); err != nil {
						t.Fatal(err)
					}
					extra += 512
					play = []playbackInstruction{{Source: NodeRef{"d", id}}}
				}
				service.Facts[id] = fact
				if service.NoteOrigins == nil {
					service.NoteOrigins = make(map[NodeID]noteTimingOrigin)
				}
				adjust := noteTimingAdjustment{noteTimingDelta: noteTimingDelta{Raw: absent, Units: units, Delta: Evidence[Rational]{State: EvidenceAbsent}}, Origin: at, Position: at}
				service.NoteOrigins[id] = noteTimingOrigin{Note: NodeRef{"d", id}, Part: part, Staff: Evidence[Integer]{State: EvidenceAbsent}, Voice: Evidence[string]{State: EvidenceKnown, Value: "1"}, ScoreOnset: at, ScoreEnd: at, Duration: duration, Chord: Evidence[bool]{State: EvidenceKnown}, Grace: Evidence[bool]{State: EvidenceKnown}, Cue: Evidence[bool]{State: EvidenceKnown}, Attack: adjust, Release: adjust, GraceTiming: grace, BendsComplete: Evidence[bool]{State: EvidenceKnown, Value: true}}
				if service.Playback.Notes == nil {
					service.Playback.Notes = make(map[NodeID]notePlaybackContext)
				}
				service.Playback.Notes[id] = notePlaybackContext{Origin: playbackOrigin{Source: NodeRef{"d", id}, Measure: measure, Part: part, MeasureOrdinal: uint64(first), Position: at, Voice: Evidence[string]{State: EvidenceKnown, Value: "1"}, MeasureComplete: Evidence[bool]{State: EvidenceKnown, Value: true}}, Principal: part, Chord: Evidence[bool]{State: EvidenceKnown}, Play: play, PlayComplete: Evidence[bool]{State: EvidenceKnown, Value: true}}
			}
			if err := service.packTimelineFacts(context.Background(), s); err != nil {
				t.Fatal(err)
			}
		}
		timelinePackObserver = nil
		heap, used := compactLiveHeap()-heap0, rationalUsage(t, s, BudgetBufferedBytes)-used0
		store := service.compact
		notes, plays := events, events
		if overflow&2 != 0 {
			notes = 0
		}
		if overflow&4 != 0 {
			plays = 0
		}
		if store == nil || store.factTotal != events || int(store.noteCount) != notes || int(store.playCount) != plays || len(store.overflowFacts) != events*(overflow&1) {
			t.Fatalf("distinct=%v, %d per measure: packed %+v", distinct, perMeasure, store)
		}
		// Every record's reservation was released, so the run's charge is
		// the store's own, which a dropped store releases.
		if store.charged != used-extra {
			t.Errorf("distinct offsets %v, overflow %v, %d per measure: the store counts %d charged bytes, the account %d", distinct, overflow, perMeasure, store.charged, used-extra)
		}
		t.Logf("distinct offsets %v, overflow %v, %d per measure: the store holds %d live bytes, charged %d", distinct, overflow, perMeasure, heap, used)
		if heap > used+64<<10 {
			t.Errorf("distinct offsets %v, overflow %v, %d per measure: the store holds %d live bytes but charged %d", distinct, overflow, perMeasure, heap, used)
		}
		runtime.KeepAlive(offsets)
		runtime.KeepAlive(service)
	}
}

// A store's fixed charge covers the store and its empty tables.
func TestTimelineCompactStoreFixedCharge(t *testing.T) {
	if testing.Short() {
		t.Skip("measures the heap")
	}
	s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 40})
	const stores = 2000
	kept := make([]*timelineCompactStore, stores)
	heap0 := compactLiveHeap()
	for i := range kept {
		store, err := newTimelineCompactStore(s)
		if err != nil {
			t.Fatal(err)
		}
		kept[i] = store
	}
	heap := (compactLiveHeap() - heap0) / stores
	runtime.KeepAlive(kept)
	t.Logf("a store holds %d live bytes, charged %d", heap, timelineCompactStoreBytes)
	if heap > timelineCompactStoreBytes {
		t.Fatalf("a store holds %d live bytes but is charged %d", heap, timelineCompactStoreBytes)
	}
}

// Equal numbers built by different arithmetic can differ in representation
// (a zero numerator held as nil or as an empty word slice), which
// reflect.DeepEqual observes. Interning keeps them apart, so each decodes to
// a value deeply equal to its own original and both stay compact.
func TestTimelineCompactInternsNumberShapes(t *testing.T) {
	s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 40})
	one := mustRational(t, 1, 7)
	parsed := mustRational(t, 0, 7)
	difference, err := s.rationalAdd(context.Background(), one, Rational{value: new(big.Rat).Neg(one.value), text: "-1/7", bytes: one.bytes})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.text != difference.text || reflect.DeepEqual(parsed, difference) {
		t.Skipf("no two representations of zero: %q %q", parsed.text, difference.text)
	}
	store, err := newTimelineCompactStore(s)
	if err != nil {
		t.Fatal(err)
	}
	e := compactEncoder{s: s, store: store, in: &store.in, ok: true}
	for _, v := range []Rational{parsed, difference, parsed, difference} {
		at := e.rational(v)
		if !e.live() || !reflect.DeepEqual(store.in.rationals[at], v) {
			t.Fatalf("interned %q decodes to a different representation", v.text)
		}
	}
}

// A staged unit proof replaces the service and drops its store; the store's
// whole charge is released then, and only then.
func TestTimelineCompactDroppedStoreReleasesItsCharge(t *testing.T) {
	s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 40})
	s.documentID = "d"
	service := &timelineService{Facts: make(map[NodeID]timelineEventFact)}
	for id := NodeID(1); id <= 300; id++ {
		if err := s.retain(timelineFactReserveBytes); err != nil {
			t.Fatal(err)
		}
		service.Facts[id] = timelineEventFact{Position: Evidence[musicalPosition]{State: EvidenceKnown, Value: musicalPosition{Part: NodeRef{"d", 1}, Offset: mustRational(t, int64(id), 1)}}}
	}
	if err := service.packTimelineFacts(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	packed := service.compact
	used := rationalUsage(t, s, BudgetBufferedBytes)
	if packed == nil || packed.charged == 0 || packed.charged != used {
		t.Fatalf("store charged %v, account %d", packed, used)
	}
	service.releaseDroppedStore(s, packed)
	if after := rationalUsage(t, s, BudgetBufferedBytes); after != used {
		t.Fatalf("a kept store released %d bytes", used-after)
	}
	*service = timelineService{Facts: make(map[NodeID]timelineEventFact)}
	service.releaseDroppedStore(s, packed)
	if after := rationalUsage(t, s, BudgetBufferedBytes); after != 0 {
		t.Fatalf("a dropped store left %d of %d bytes charged", after, used)
	}
}

// countdownContext reports cancellation from its nth Err call on.
type countdownContext struct {
	context.Context
	n int
}

func (c *countdownContext) Err() error {
	if c.n--; c.n < 0 {
		return context.Canceled
	}
	return nil
}

// A pack checks for cancellation before each record. Cancelled midway, it
// releases nothing and leaves the working maps as they were.
func TestTimelineCompactPackCancellation(t *testing.T) {
	for _, after := range []int{0, 1, 150, 299} {
		s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 40})
		s.documentID = "d"
		service := &timelineService{Facts: make(map[NodeID]timelineEventFact)}
		for id := NodeID(1); id <= 300; id++ {
			if err := s.retain(timelineFactReserveBytes); err != nil {
				t.Fatal(err)
			}
			service.Facts[id] = timelineEventFact{Position: Evidence[musicalPosition]{State: EvidenceKnown, Value: musicalPosition{Part: NodeRef{"d", 1}, Offset: mustRational(t, int64(id), 1)}}}
		}
		before := rationalUsage(t, s, BudgetBufferedBytes)
		err := service.packTimelineFacts(&countdownContext{context.Background(), after}, s)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled after %d records: %v", after, err)
		}
		charged := int64(0)
		if service.compact != nil {
			charged = service.compact.charged
		}
		if len(service.Facts) != 300 || rationalUsage(t, s, BudgetBufferedBytes) != before+charged {
			t.Fatalf("cancelled after %d records: %d working facts, usage %d, before %d plus store %d", after, len(service.Facts), rationalUsage(t, s, BudgetBufferedBytes), before, charged)
		}
	}
}
