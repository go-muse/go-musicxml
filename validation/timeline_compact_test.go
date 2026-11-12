package validation

import (
	"math/big"
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
	for _, tc := range []struct {
		distinct   bool
		perMeasure int
	}{{false, 48}, {true, 48}, {false, 1}, {false, events}} {
		distinct, perMeasure := tc.distinct, tc.perMeasure
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
				service.Facts[id] = timelineEventFact{Origin: at, Position: at, End: at, Duration: duration, PrincipalDuration: duration, Principal: Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"d", id}}, Units: units, Staff: Evidence[Integer]{State: EvidenceAbsent}, Voice: Evidence[string]{State: EvidenceKnown, Value: "1"}, Part: part}
				if service.NoteOrigins == nil {
					service.NoteOrigins = make(map[NodeID]noteTimingOrigin)
				}
				adjust := noteTimingAdjustment{noteTimingDelta: noteTimingDelta{Raw: absent, Units: units, Delta: Evidence[Rational]{State: EvidenceAbsent}}, Origin: at, Position: at}
				service.NoteOrigins[id] = noteTimingOrigin{Note: NodeRef{"d", id}, Part: part, Staff: Evidence[Integer]{State: EvidenceAbsent}, Voice: Evidence[string]{State: EvidenceKnown, Value: "1"}, ScoreOnset: at, ScoreEnd: at, Duration: duration, Chord: Evidence[bool]{State: EvidenceKnown}, Grace: Evidence[bool]{State: EvidenceKnown}, Cue: Evidence[bool]{State: EvidenceKnown}, Attack: adjust, Release: adjust, GraceTiming: Evidence[graceTimingOrigin]{State: EvidenceAbsent}, BendsComplete: Evidence[bool]{State: EvidenceKnown, Value: true}}
				if service.Playback.Notes == nil {
					service.Playback.Notes = make(map[NodeID]notePlaybackContext)
				}
				service.Playback.Notes[id] = notePlaybackContext{Origin: playbackOrigin{Source: NodeRef{"d", id}, Measure: measure, Part: part, MeasureOrdinal: uint64(first), Position: at, Voice: Evidence[string]{State: EvidenceKnown, Value: "1"}, MeasureComplete: Evidence[bool]{State: EvidenceKnown, Value: true}}, Principal: part, Chord: Evidence[bool]{State: EvidenceKnown}, Play: []playbackInstruction{}, PlayComplete: Evidence[bool]{State: EvidenceKnown, Value: true}}
			}
			if err := service.packTimelineFacts(s); err != nil {
				t.Fatal(err)
			}
		}
		heap, used := compactLiveHeap()-heap0, rationalUsage(t, s, BudgetBufferedBytes)-used0
		store := service.compact
		if store == nil || store.factTotal != events || store.noteCount != events || store.playCount != events {
			t.Fatalf("distinct=%v, %d per measure: packed %+v", distinct, perMeasure, store)
		}
		t.Logf("distinct offsets %v, %d per measure: the store holds %d live bytes, charged %d", distinct, perMeasure, heap, used)
		if heap > used+64<<10 {
			t.Errorf("distinct offsets %v, %d per measure: the store holds %d live bytes but charged %d", distinct, perMeasure, heap, used)
		}
		runtime.KeepAlive(offsets)
		runtime.KeepAlive(service)
	}
}
