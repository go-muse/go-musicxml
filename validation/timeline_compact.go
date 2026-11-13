package validation

import (
	"context"
	"math"
	"math/big"
	"reflect"
)

// Windowed evaluation keeps the timeline facts of an evaluated measure in a
// compact store instead of the working maps: an event fact, a note's timing
// origin and its playback context. Each record field is encoded without loss
// (evidence state and reason, interned numbers and strings, the frame of a
// position). A record that cannot be encoded, or whose decoded form is not
// reflect.DeepEqual to the original, keeps the original in an overflow map.
// Every read therefore returns the value buffered evaluation would hold.
const (
	// A page of record numbers is 4 KiB, an allocator size class.
	timelineCompactPageShift = 10
	timelineCompactPageSize  = 1 << timelineCompactPageShift

	// Record widths, at least unsafe.Sizeof of each record type, and the
	// records per chunk, so that a chunk fills one 32 KiB allocator size
	// class rather than being rounded up to whole pages
	// (TestTimelineCompactRecordWidths).
	timelineCompactEventBytes int64 = 136
	timelineCompactNoteBytes  int64 = 184
	timelineCompactPlayBytes  int64 = 136
	timelineCompactChunkBytes int64 = 32 << 10
	timelineCompactChunk            = 240
	timelineCompactNoteChunk        = 178
	timelineCompactPlayChunk        = 240

	// One interned value: its slice element and map entry, both doubled
	// for growth. Interned strings and numbers share the original storage.
	// A frame's key and element are two node references and an ordinal.
	timelineCompactInternBytes      int64 = 192
	timelineCompactFrameInternBytes int64 = 320
	// A slice header slot of an outer chunk or page directory, doubled for
	// growth.
	timelineCompactSlotBytes int64 = 48
	// The store and its empty tables (TestTimelineCompactStoreChargeCoversItsHeap
	// measures about 3 KiB).
	timelineCompactStoreBytes int64 = 4096

	// Windowed evaluation starts a compact store only once the working maps
	// hold this many bytes of record reservations: a smaller timeline stays in
	// its working maps, as in buffered evaluation, since the store's first
	// chunks and page would cost more than they save.
	timelineCompactPackBytes int64 = 256 << 10
)

const (
	compactHasFact uint8 = 1 << iota
	compactFactOverflow
	compactNoteOverflow
	compactPlayOverflow
)

type compactHeader struct {
	state  EvidenceState
	reason Code
}

// compactNumberKey identifies an interned number by its canonical text and
// size, and by the shape of its representation: absent, or a numerator with
// a nil or a non-nil word slice. Equal numbers built by different arithmetic
// can differ in that shape, which reflect.DeepEqual observes.
type compactNumberKey struct {
	text  string
	bytes int64
	shape uint8
}

func compactRatShape(v *big.Rat) uint8 {
	switch {
	case v == nil:
		return 0
	case v.Num().Bits() == nil:
		return 1
	}
	return 2
}

func compactIntShape(v *big.Int) uint8 {
	switch {
	case v == nil:
		return 0
	case v.Bits() == nil:
		return 1
	}
	return 2
}

type compactScalar struct {
	header uint16
	value  uint32
}

type compactFlag struct {
	header uint16
	value  bool
}

type compactRef struct {
	header uint16
	doc    uint32
	node   NodeID
}

type compactPosition struct {
	header   uint16
	unproved bool
	frame    uint32
	offset   uint32
}

type compactFact struct {
	origin, position, end       compactPosition
	duration, principalDuration compactScalar
	principal                   compactRef
	units, staff, voice         compactScalar
	part                        compactRef
}

type compactEventRecord struct {
	id         NodeID
	fact       compactFact
	note, play uint32
	flags      uint8
}

type compactAdjustment struct {
	raw, units, delta compactScalar
	origin, position  compactPosition
}

type compactNote struct {
	part              compactRef
	staff, voice      compactScalar
	onset, end        compactPosition
	duration          compactScalar
	chord, grace, cue compactFlag
	attack, release   compactAdjustment
	graceTiming       uint16
	bendsComplete     compactFlag
}

type compactPlay struct {
	measure, part, principal compactRef
	ordinal                  uint64
	position                 compactPosition
	staff, voice, order      compactScalar
	dynamics, endDynamics    compactScalar
	timeOnly                 compactScalar
	measureComplete, chord   compactFlag
	pizzicato, playComplete  compactFlag
	// playEmpty tells an empty instruction list from none.
	playEmpty bool
}

type compactInterner struct {
	document      string
	headers       []compactHeader
	headerIndex   map[compactHeader]uint16
	rationals     []Rational
	rationalIndex map[compactNumberKey]uint32
	decimals      []Decimal
	decimalIndex  map[compactNumberKey]uint32
	integers      []Integer
	integerIndex  map[compactNumberKey]uint32
	strings       []string
	stringIndex   map[string]uint32
	frames        []timelineFrame
	frameIndex    map[timelineFrame]uint32
}

// timelineCompactStore is append-only. Its records are addressed by NodeID
// through fixed pages and stored in fixed chunks: growing copies only the
// page directory and the chunk lists, never records.
type timelineCompactStore struct {
	in            compactInterner
	pages         [][]uint32
	events        [][]compactEventRecord
	notes         [][]compactNote
	plays         [][]compactPlay
	eventCount    uint32
	noteCount     uint32
	playCount     uint32
	factTotal     int
	playTotal     int
	overflowFacts map[NodeID]timelineEventFact
	overflowNotes map[NodeID]noteTimingOrigin
	overflowPlays map[NodeID]notePlaybackContext
	// charged is what the store has retained, released at once if a staged
	// unit proof drops the whole store.
	charged int64
	// largestLaterPack is the most facts packed at once after the first pack.
	largestLaterPack int
}

func (store *timelineCompactStore) retain(s *Session, bytes int64) error {
	if err := s.retain(bytes); err != nil {
		return err
	}
	store.charged += bytes
	return nil
}

func newTimelineCompactStore(s *Session) (*timelineCompactStore, error) {
	if err := s.retain(timelineCompactStoreBytes); err != nil {
		return nil, err
	}
	in := compactInterner{
		document:      s.documentID,
		headers:       []compactHeader{{}},
		headerIndex:   map[compactHeader]uint16{{}: 0},
		rationals:     []Rational{{}},
		rationalIndex: map[compactNumberKey]uint32{{}: 0},
		decimals:      []Decimal{{}},
		decimalIndex:  map[compactNumberKey]uint32{{}: 0},
		integers:      []Integer{{}},
		integerIndex:  map[compactNumberKey]uint32{{}: 0},
		strings:       []string{""},
		stringIndex:   map[string]uint32{"": 0},
		frames:        []timelineFrame{{}},
		frameIndex:    map[timelineFrame]uint32{{}: 0},
	}
	return &timelineCompactStore{in: in, charged: timelineCompactStoreBytes}, nil
}

// packDue reports whether windowed evaluation packs the working maps now:
// after every measure once a store exists, and to start one when the maps
// hold timelineCompactPackBytes of record reservations.
func (service *timelineService) packDue() bool {
	return service.compact != nil || service.timelinePendingBytes() >= timelineCompactPackBytes
}

// releaseDroppedStore releases the charge of packed, the store the service
// held before its timing origins were finalized, when a staged unit proof
// replaced the service and so dropped that store.
func (service *timelineService) releaseDroppedStore(s *Session, packed *timelineCompactStore) {
	if packed != nil && service.compact != packed {
		s.releaseFactBytes(packed.charged)
	}
}

// timelinePendingBytes is the record reservations the working maps hold.
func (service *timelineService) timelinePendingBytes() int64 {
	return int64(len(service.Facts))*timelineFactReserveBytes + int64(len(service.NoteOrigins))*timelineNoteReserveBytes + int64(len(service.Playback.Notes))*playbackNoteReserveBytes
}

// packTimelineFacts moves every record of the working maps into the compact
// store and gives the service new, empty working maps, so that neither the
// records nor the old map tables stay reachable. The compacted records'
// reservations are released only then; after a failure the working maps
// stay as they are and keep their reservations. Packing charges no rule
// steps: each record was charged when it was published.
func (service *timelineService) packTimelineFacts(ctx context.Context, s *Session) error {
	if len(service.Facts) == 0 && len(service.NoteOrigins) == 0 && len(service.Playback.Notes) == 0 {
		return nil
	}
	if service.compact == nil {
		store, err := newTimelineCompactStore(s)
		if err != nil {
			return err
		}
		service.compact = store
	}
	store := service.compact
	if store.eventCount != 0 {
		store.largestLaterPack = max(store.largestLaterPack, len(service.Facts))
	}
	var released int64
	for id, fact := range service.Facts {
		if err := ctx.Err(); err != nil {
			return err
		}
		bytes, err := store.putFact(s, id, fact)
		if err != nil {
			return err
		}
		released += bytes
	}
	for id, note := range service.NoteOrigins {
		if err := ctx.Err(); err != nil {
			return err
		}
		bytes, err := store.putNote(s, id, note)
		if err != nil {
			return err
		}
		released += bytes
	}
	for id, play := range service.Playback.Notes {
		if err := ctx.Err(); err != nil {
			return err
		}
		bytes, err := store.putPlay(s, id, play)
		if err != nil {
			return err
		}
		released += bytes
	}
	if timelinePackObserver != nil {
		timelinePackObserver()
	}
	service.Facts = make(map[NodeID]timelineEventFact)
	service.NoteOrigins = nil
	if service.Playback.Notes != nil {
		service.Playback.Notes = make(map[NodeID]notePlaybackContext)
	}
	s.releaseFactBytes(released)
	return nil
}

// timelinePackObserver is a test observation point only: it runs when every
// record is in the store and the working maps are still reachable.
var timelinePackObserver func()

func (store *timelineCompactStore) slot(s *Session, id NodeID) (*uint32, error) {
	page := uint64(id) >> timelineCompactPageShift
	if page >= uint64(math.MaxInt32) {
		return nil, contract("timeline compact store", "node identity out of range")
	}
	if int(page) >= len(store.pages) {
		size := max(int(page)+1, 2*len(store.pages))
		if err := store.retain(s, int64(size)*timelineCompactSlotBytes); err != nil {
			return nil, err
		}
		pages := make([][]uint32, size)
		copy(pages, store.pages)
		store.pages = pages
	}
	if store.pages[page] == nil {
		if err := store.retain(s, 4*timelineCompactPageSize+timelineCompactSlotBytes); err != nil {
			return nil, err
		}
		store.pages[page] = make([]uint32, timelineCompactPageSize)
	}
	return &store.pages[page][uint64(id)&(timelineCompactPageSize-1)], nil
}

func (store *timelineCompactStore) find(id NodeID) *compactEventRecord {
	page := uint64(id) >> timelineCompactPageShift
	if page >= uint64(len(store.pages)) || store.pages[page] == nil {
		return nil
	}
	at := store.pages[page][uint64(id)&(timelineCompactPageSize-1)]
	if at == 0 {
		return nil
	}
	at--
	return &store.events[at/timelineCompactChunk][at%timelineCompactChunk]
}

func (store *timelineCompactStore) record(s *Session, id NodeID) (*compactEventRecord, error) {
	slot, err := store.slot(s, id)
	if err != nil {
		return nil, err
	}
	if *slot != 0 {
		at := *slot - 1
		return &store.events[at/timelineCompactChunk][at%timelineCompactChunk], nil
	}
	if store.eventCount == math.MaxUint32-1 {
		return nil, contract("timeline compact store", "record count out of range")
	}
	if store.eventCount%timelineCompactChunk == 0 {
		if err := store.retain(s, timelineCompactChunkBytes+timelineCompactSlotBytes); err != nil {
			return nil, err
		}
		store.events = append(store.events, make([]compactEventRecord, timelineCompactChunk))
	}
	at := store.eventCount
	store.eventCount++
	*slot = at + 1
	rec := &store.events[at/timelineCompactChunk][at%timelineCompactChunk]
	rec.id = id
	return rec, nil
}

func (store *timelineCompactStore) putFact(s *Session, id NodeID, fact timelineEventFact) (int64, error) {
	rec, err := store.record(s, id)
	if err != nil {
		return 0, err
	}
	if rec.flags&compactHasFact != 0 {
		return 0, contract("timeline fact", "duplicate publication")
	}
	e := compactEncoder{s: s, store: store, in: &store.in, ok: true}
	c := e.fact(fact)
	if e.err != nil {
		return 0, e.err
	}
	rec.flags |= compactHasFact
	store.factTotal++
	if e.ok && reflect.DeepEqual(store.in.fact(c), fact) {
		rec.fact = c
		return timelineFactReserveBytes, nil
	}
	// The copy has a reservation of its own: the working map's record stays
	// reachable until the pack ends and releases that one.
	if err := store.retain(s, timelineFactReserveBytes); err != nil {
		return 0, err
	}
	if store.overflowFacts == nil {
		store.overflowFacts = make(map[NodeID]timelineEventFact)
	}
	store.overflowFacts[id] = fact
	rec.flags |= compactFactOverflow
	return timelineFactReserveBytes, nil
}

func (store *timelineCompactStore) putNote(s *Session, id NodeID, note noteTimingOrigin) (int64, error) {
	rec, err := store.record(s, id)
	if err != nil {
		return 0, err
	}
	if rec.note != 0 || rec.flags&compactNoteOverflow != 0 {
		return 0, contract("note timing origins", "duplicate publication")
	}
	e := compactEncoder{s: s, store: store, in: &store.in, ok: true}
	c := e.note(id, note)
	if e.err != nil {
		return 0, e.err
	}
	if e.ok && reflect.DeepEqual(store.in.note(id, c), note) {
		if store.noteCount == math.MaxUint32-1 {
			return 0, contract("timeline compact store", "record count out of range")
		}
		if store.noteCount%timelineCompactNoteChunk == 0 {
			if err := store.retain(s, timelineCompactChunkBytes+timelineCompactSlotBytes); err != nil {
				return 0, err
			}
			store.notes = append(store.notes, make([]compactNote, timelineCompactNoteChunk))
		}
		at := store.noteCount
		store.noteCount++
		store.notes[at/timelineCompactNoteChunk][at%timelineCompactNoteChunk] = c
		rec.note = at + 1
		return timelineNoteReserveBytes, nil
	}
	// The copy has a reservation of its own: the working map's record stays
	// reachable until the pack ends and releases that one.
	if err := store.retain(s, timelineNoteReserveBytes); err != nil {
		return 0, err
	}
	if store.overflowNotes == nil {
		store.overflowNotes = make(map[NodeID]noteTimingOrigin)
	}
	store.overflowNotes[id] = note
	rec.flags |= compactNoteOverflow
	return timelineNoteReserveBytes, nil
}

func (store *timelineCompactStore) putPlay(s *Session, id NodeID, play notePlaybackContext) (int64, error) {
	rec, err := store.record(s, id)
	if err != nil {
		return 0, err
	}
	if rec.play != 0 || rec.flags&compactPlayOverflow != 0 {
		return 0, contract("playback context", "duplicate publication")
	}
	e := compactEncoder{s: s, store: store, in: &store.in, ok: true}
	c := e.play(id, play)
	if e.err != nil {
		return 0, e.err
	}
	store.playTotal++
	if e.ok && reflect.DeepEqual(store.in.play(id, c), play) {
		if store.playCount == math.MaxUint32-1 {
			return 0, contract("timeline compact store", "record count out of range")
		}
		if store.playCount%timelineCompactPlayChunk == 0 {
			if err := store.retain(s, timelineCompactChunkBytes+timelineCompactSlotBytes); err != nil {
				return 0, err
			}
			store.plays = append(store.plays, make([]compactPlay, timelineCompactPlayChunk))
		}
		at := store.playCount
		store.playCount++
		store.plays[at/timelineCompactPlayChunk][at%timelineCompactPlayChunk] = c
		rec.play = at + 1
		return playbackNoteReserveBytes, nil
	}
	// The copy has a reservation of its own: the working map's record stays
	// reachable until the pack ends and releases that one.
	if err := store.retain(s, playbackNoteReserveBytes); err != nil {
		return 0, err
	}
	if store.overflowPlays == nil {
		store.overflowPlays = make(map[NodeID]notePlaybackContext)
	}
	store.overflowPlays[id] = play
	rec.flags |= compactPlayOverflow
	return playbackNoteReserveBytes, nil
}

func (store *timelineCompactStore) fact(id NodeID) (timelineEventFact, bool) {
	rec := store.find(id)
	if rec == nil || rec.flags&compactHasFact == 0 {
		return timelineEventFact{}, false
	}
	if rec.flags&compactFactOverflow != 0 {
		return store.overflowFacts[id], true
	}
	return store.in.fact(rec.fact), true
}

func (store *timelineCompactStore) note(id NodeID) (noteTimingOrigin, bool) {
	rec := store.find(id)
	if rec == nil {
		return noteTimingOrigin{}, false
	}
	if rec.flags&compactNoteOverflow != 0 {
		return store.overflowNotes[id], true
	}
	if rec.note == 0 {
		return noteTimingOrigin{}, false
	}
	at := rec.note - 1
	return store.in.note(id, store.notes[at/timelineCompactNoteChunk][at%timelineCompactNoteChunk]), true
}

func (store *timelineCompactStore) hasPlay(id NodeID) bool {
	rec := store.find(id)
	return rec != nil && (rec.play != 0 || rec.flags&compactPlayOverflow != 0)
}

func (store *timelineCompactStore) play(id NodeID) (notePlaybackContext, bool) {
	rec := store.find(id)
	if rec == nil {
		return notePlaybackContext{}, false
	}
	if rec.flags&compactPlayOverflow != 0 {
		return store.overflowPlays[id], true
	}
	if rec.play == 0 {
		return notePlaybackContext{}, false
	}
	at := rec.play - 1
	return store.in.play(id, store.plays[at/timelineCompactPlayChunk][at%timelineCompactPlayChunk]), true
}

// forEachFact visits stored facts in packing order until visit returns false.
func (store *timelineCompactStore) forEachFact(visit func(NodeID, timelineEventFact) bool) bool {
	for at := uint32(0); at < store.eventCount; at++ {
		rec := &store.events[at/timelineCompactChunk][at%timelineCompactChunk]
		if rec.flags&compactHasFact == 0 {
			continue
		}
		fact := store.overflowFacts[rec.id]
		if rec.flags&compactFactOverflow == 0 {
			fact = store.in.fact(rec.fact)
		}
		if !visit(rec.id, fact) {
			return false
		}
	}
	return true
}

// compactEncoder encodes one record. ok turns false on a value the encoding
// cannot hold (a causal reference, a displaced placement, a list); err is a
// resource or contract failure of interning.
type compactEncoder struct {
	s     *Session
	store *timelineCompactStore
	in    *compactInterner
	ok    bool
	err   error
}

func (e *compactEncoder) live() bool { return e.ok && e.err == nil }

func (e *compactEncoder) intern(bytes int64) bool {
	if !e.live() {
		return false
	}
	e.err = e.store.retain(e.s, bytes)
	return e.err == nil
}

func (e *compactEncoder) header(state EvidenceState, reason Code, cause *DecisionRef) uint16 {
	if !e.live() {
		return 0
	}
	if cause != nil {
		e.ok = false
		return 0
	}
	key := compactHeader{state, reason}
	if at, found := e.in.headerIndex[key]; found {
		return at
	}
	if len(e.in.headers) > math.MaxUint16 {
		e.ok = false
		return 0
	}
	if !e.intern(timelineCompactInternBytes) {
		return 0
	}
	at := uint16(len(e.in.headers))
	e.in.headers = append(e.in.headers, key)
	e.in.headerIndex[key] = at
	return at
}

func compactInternValue[T any](e *compactEncoder, values *[]T, index map[compactNumberKey]uint32, key compactNumberKey, v T) uint32 {
	if !e.live() {
		return 0
	}
	if at, found := index[key]; found {
		return at
	}
	if uint64(len(*values)) >= math.MaxUint32 {
		e.ok = false
		return 0
	}
	if !e.intern(timelineCompactInternBytes) {
		return 0
	}
	at := uint32(len(*values))
	*values = append(*values, v)
	index[key] = at
	return at
}

func (e *compactEncoder) rational(v Rational) uint32 {
	return compactInternValue(e, &e.in.rationals, e.in.rationalIndex, compactNumberKey{v.text, v.bytes, compactRatShape(v.value)}, v)
}

func (e *compactEncoder) decimal(v Decimal) uint32 {
	return compactInternValue(e, &e.in.decimals, e.in.decimalIndex, compactNumberKey{v.text, v.bytes, compactRatShape(v.value)}, v)
}

func (e *compactEncoder) integer(v Integer) uint32 {
	return compactInternValue(e, &e.in.integers, e.in.integerIndex, compactNumberKey{v.text, v.bytes, compactIntShape(v.value)}, v)
}

func (e *compactEncoder) text(v string) uint32 {
	if !e.live() {
		return 0
	}
	if at, found := e.in.stringIndex[v]; found {
		return at
	}
	if uint64(len(e.in.strings)) >= math.MaxUint32 {
		e.ok = false
		return 0
	}
	if !e.intern(timelineCompactInternBytes) {
		return 0
	}
	at := uint32(len(e.in.strings))
	e.in.strings = append(e.in.strings, v)
	e.in.stringIndex[v] = at
	return at
}

func (e *compactEncoder) frame(v timelineFrame) uint32 {
	if !e.live() {
		return 0
	}
	if at, found := e.in.frameIndex[v]; found {
		return at
	}
	if uint64(len(e.in.frames)) >= math.MaxUint32 {
		e.ok = false
		return 0
	}
	if !e.intern(timelineCompactFrameInternBytes) {
		return 0
	}
	at := uint32(len(e.in.frames))
	e.in.frames = append(e.in.frames, v)
	e.in.frameIndex[v] = at
	return at
}

func (e *compactEncoder) placement(v stateGapPlacement) {
	if v.Displaced || v.Lower != nil || v.Upper != nil {
		e.ok = false
	}
}

func (e *compactEncoder) position(v Evidence[musicalPosition]) compactPosition {
	return compactPosition{
		header:   e.header(v.State, v.Reason, v.Cause),
		unproved: v.Value.UnprovedFrame,
		frame:    e.frame(timelineFrame{v.Value.Part, v.Value.Measure, v.Value.MeasureOrdinal}),
		offset:   e.rational(v.Value.Offset),
	}
}

func (e *compactEncoder) rationalEvidence(v Evidence[Rational]) compactScalar {
	return compactScalar{e.header(v.State, v.Reason, v.Cause), e.rational(v.Value)}
}

func (e *compactEncoder) decimalEvidence(v Evidence[Decimal]) compactScalar {
	return compactScalar{e.header(v.State, v.Reason, v.Cause), e.decimal(v.Value)}
}

func (e *compactEncoder) integerEvidence(v Evidence[Integer]) compactScalar {
	return compactScalar{e.header(v.State, v.Reason, v.Cause), e.integer(v.Value)}
}

func (e *compactEncoder) textEvidence(v Evidence[string]) compactScalar {
	return compactScalar{e.header(v.State, v.Reason, v.Cause), e.text(v.Value)}
}

func (e *compactEncoder) orderEvidence(v Evidence[OrderKind]) compactScalar {
	return compactScalar{e.header(v.State, v.Reason, v.Cause), e.text(string(v.Value))}
}

func (e *compactEncoder) flag(v Evidence[bool]) compactFlag {
	return compactFlag{e.header(v.State, v.Reason, v.Cause), v.Value}
}

func (e *compactEncoder) ref(v NodeRef) compactRef {
	return compactRef{doc: e.text(v.DocumentID), node: v.Node}
}

func (e *compactEncoder) refEvidence(v Evidence[NodeRef]) compactRef {
	out := e.ref(v.Value)
	out.header = e.header(v.State, v.Reason, v.Cause)
	return out
}

func (e *compactEncoder) adjustment(v noteTimingAdjustment) compactAdjustment {
	return compactAdjustment{
		raw:      e.decimalEvidence(v.Raw),
		units:    e.decimalEvidence(v.Units),
		delta:    e.rationalEvidence(v.Delta),
		origin:   e.position(v.Origin),
		position: e.position(v.Position),
	}
}

func (e *compactEncoder) fact(v timelineEventFact) compactFact {
	e.placement(v.Placement)
	return compactFact{
		origin:            e.position(v.Origin),
		position:          e.position(v.Position),
		end:               e.position(v.End),
		duration:          e.rationalEvidence(v.Duration),
		principalDuration: e.rationalEvidence(v.PrincipalDuration),
		principal:         e.refEvidence(v.Principal),
		units:             e.decimalEvidence(v.Units),
		staff:             e.integerEvidence(v.Staff),
		voice:             e.textEvidence(v.Voice),
		part:              e.refEvidence(v.Part),
	}
}

// A note origin is encodable without bends and without grace timing values;
// the rest of its records round-trip field by field.
func (e *compactEncoder) note(id NodeID, v noteTimingOrigin) compactNote {
	e.placement(v.Placement)
	if v.Note != (NodeRef{e.in.document, id}) || v.Bends != nil || v.GraceTiming.State == EvidenceKnown {
		e.ok = false
	}
	return compactNote{
		part:          e.refEvidence(v.Part),
		staff:         e.integerEvidence(v.Staff),
		voice:         e.textEvidence(v.Voice),
		onset:         e.position(v.ScoreOnset),
		end:           e.position(v.ScoreEnd),
		duration:      e.rationalEvidence(v.Duration),
		chord:         e.flag(v.Chord),
		grace:         e.flag(v.Grace),
		cue:           e.flag(v.Cue),
		attack:        e.adjustment(v.Attack),
		release:       e.adjustment(v.Release),
		graceTiming:   e.header(v.GraceTiming.State, v.GraceTiming.Reason, v.GraceTiming.Cause),
		bendsComplete: e.flag(v.BendsComplete),
	}
}

// A playback context is encodable without play instructions.
func (e *compactEncoder) play(id NodeID, v notePlaybackContext) compactPlay {
	o := v.Origin
	e.placement(o.Placement)
	if o.Source != (NodeRef{e.in.document, id}) || len(v.Play) != 0 {
		e.ok = false
	}
	return compactPlay{
		measure:         e.ref(o.Measure),
		part:            e.refEvidence(o.Part),
		principal:       e.refEvidence(v.Principal),
		ordinal:         o.MeasureOrdinal,
		position:        e.position(o.Position),
		staff:           e.integerEvidence(o.Staff),
		voice:           e.textEvidence(o.Voice),
		order:           e.orderEvidence(o.Order),
		dynamics:        e.decimalEvidence(v.Dynamics),
		endDynamics:     e.decimalEvidence(v.EndDynamics),
		timeOnly:        e.textEvidence(v.TimeOnly),
		measureComplete: e.flag(o.MeasureComplete),
		chord:           e.flag(v.Chord),
		pizzicato:       e.flag(v.Pizzicato),
		playComplete:    e.flag(v.PlayComplete),
		playEmpty:       v.Play != nil,
	}
}

func compactEvidence[T any](in *compactInterner, header uint16, v T) Evidence[T] {
	h := in.headers[header]
	return Evidence[T]{State: h.state, Value: v, Reason: h.reason}
}

func (in *compactInterner) position(v compactPosition) Evidence[musicalPosition] {
	f := in.frames[v.frame]
	return compactEvidence(in, v.header, musicalPosition{Part: f.Part, Measure: f.Measure, MeasureOrdinal: f.Ordinal, Offset: in.rationals[v.offset], UnprovedFrame: v.unproved})
}

func (in *compactInterner) rationalEvidence(v compactScalar) Evidence[Rational] {
	return compactEvidence(in, v.header, in.rationals[v.value])
}

func (in *compactInterner) decimalEvidence(v compactScalar) Evidence[Decimal] {
	return compactEvidence(in, v.header, in.decimals[v.value])
}

func (in *compactInterner) integerEvidence(v compactScalar) Evidence[Integer] {
	return compactEvidence(in, v.header, in.integers[v.value])
}

func (in *compactInterner) textEvidence(v compactScalar) Evidence[string] {
	return compactEvidence(in, v.header, in.strings[v.value])
}

func (in *compactInterner) orderEvidence(v compactScalar) Evidence[OrderKind] {
	return compactEvidence(in, v.header, OrderKind(in.strings[v.value]))
}

func (in *compactInterner) flag(v compactFlag) Evidence[bool] {
	return compactEvidence(in, v.header, v.value)
}

func (in *compactInterner) ref(v compactRef) NodeRef {
	return NodeRef{DocumentID: in.strings[v.doc], Node: v.node}
}

func (in *compactInterner) refEvidence(v compactRef) Evidence[NodeRef] {
	return compactEvidence(in, v.header, in.ref(v))
}

func (in *compactInterner) adjustment(v compactAdjustment) noteTimingAdjustment {
	return noteTimingAdjustment{
		noteTimingDelta: noteTimingDelta{Raw: in.decimalEvidence(v.raw), Units: in.decimalEvidence(v.units), Delta: in.rationalEvidence(v.delta)},
		Origin:          in.position(v.origin),
		Position:        in.position(v.position),
	}
}

func (in *compactInterner) fact(v compactFact) timelineEventFact {
	return timelineEventFact{
		Origin:            in.position(v.origin),
		Position:          in.position(v.position),
		End:               in.position(v.end),
		Duration:          in.rationalEvidence(v.duration),
		PrincipalDuration: in.rationalEvidence(v.principalDuration),
		Principal:         in.refEvidence(v.principal),
		Units:             in.decimalEvidence(v.units),
		Staff:             in.integerEvidence(v.staff),
		Voice:             in.textEvidence(v.voice),
		Part:              in.refEvidence(v.part),
	}
}

func (in *compactInterner) note(id NodeID, v compactNote) noteTimingOrigin {
	return noteTimingOrigin{
		Note:          NodeRef{DocumentID: in.document, Node: id},
		Part:          in.refEvidence(v.part),
		Staff:         in.integerEvidence(v.staff),
		Voice:         in.textEvidence(v.voice),
		ScoreOnset:    in.position(v.onset),
		ScoreEnd:      in.position(v.end),
		Duration:      in.rationalEvidence(v.duration),
		Chord:         in.flag(v.chord),
		Grace:         in.flag(v.grace),
		Cue:           in.flag(v.cue),
		Attack:        in.adjustment(v.attack),
		Release:       in.adjustment(v.release),
		GraceTiming:   compactEvidence(in, v.graceTiming, graceTimingOrigin{}),
		BendsComplete: in.flag(v.bendsComplete),
	}
}

func (in *compactInterner) play(id NodeID, v compactPlay) notePlaybackContext {
	var instructions []playbackInstruction
	if v.playEmpty {
		instructions = []playbackInstruction{}
	}
	return notePlaybackContext{
		Origin: playbackOrigin{
			Source:          NodeRef{DocumentID: in.document, Node: id},
			Measure:         in.ref(v.measure),
			Part:            in.refEvidence(v.part),
			MeasureOrdinal:  v.ordinal,
			Position:        in.position(v.position),
			Staff:           in.integerEvidence(v.staff),
			Voice:           in.textEvidence(v.voice),
			Order:           in.orderEvidence(v.order),
			MeasureComplete: in.flag(v.measureComplete),
		},
		Principal:    in.refEvidence(v.principal),
		Chord:        in.flag(v.chord),
		Dynamics:     in.decimalEvidence(v.dynamics),
		EndDynamics:  in.decimalEvidence(v.endDynamics),
		Pizzicato:    in.flag(v.pizzicato),
		TimeOnly:     in.textEvidence(v.timeOnly),
		Play:         instructions,
		PlayComplete: in.flag(v.playComplete),
	}
}

// Readers of the timeline service. Buffered evaluation has no compact store,
// so each reads its working map exactly as before.

func (service *timelineService) fact(id NodeID) (timelineEventFact, bool) {
	if f, ok := service.Facts[id]; ok {
		return f, true
	}
	if service.compact != nil {
		return service.compact.fact(id)
	}
	return timelineEventFact{}, false
}

// factFast returns the zero fact when id has none, as a map index would.
func (service *timelineService) factFast(id NodeID) timelineEventFact {
	f, _ := service.fact(id)
	return f
}

func (service *timelineService) hasFact(id NodeID) bool {
	if _, ok := service.Facts[id]; ok {
		return true
	}
	if service.compact != nil {
		rec := service.compact.find(id)
		return rec != nil && rec.flags&compactHasFact != 0
	}
	return false
}

func (service *timelineService) factCount() int {
	n := len(service.Facts)
	if service.compact != nil {
		n += service.compact.factTotal
	}
	return n
}

// forEachFact visits every fact, those of the working map first, until
// visit returns false. Like a map range, callers rely on no order.
func (service *timelineService) forEachFact(visit func(NodeID, timelineEventFact) bool) {
	for id, f := range service.Facts {
		if !visit(id, f) {
			return
		}
	}
	if service.compact != nil {
		service.compact.forEachFact(visit)
	}
}

func (service *timelineService) noteOrigin(id NodeID) (noteTimingOrigin, bool) {
	if n, ok := service.NoteOrigins[id]; ok {
		return n, true
	}
	if service.compact != nil {
		return service.compact.note(id)
	}
	return noteTimingOrigin{}, false
}

// noteOriginFast returns the zero origin when id has none, as a map index
// would.
func (service *timelineService) noteOriginFast(id NodeID) noteTimingOrigin {
	n, _ := service.noteOrigin(id)
	return n
}

func (service *timelineService) playbackNote(id NodeID) (notePlaybackContext, bool) {
	if n, ok := service.Playback.Notes[id]; ok {
		return n, true
	}
	if service.compact != nil {
		return service.compact.play(id)
	}
	return notePlaybackContext{}, false
}

func (service *timelineService) hasPlaybackNote(id NodeID) bool {
	if _, ok := service.Playback.Notes[id]; ok {
		return true
	}
	return service.compact != nil && service.compact.hasPlay(id)
}

func (service *timelineService) playbackNoteCount() int {
	n := len(service.Playback.Notes)
	if service.compact != nil {
		n += service.compact.playTotal
	}
	return n
}
