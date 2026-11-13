package validation

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// Record counts of a finalized timeline service, in this order: all facts,
// note timing origins and note playback contexts, then those of each held in
// compact form, then those of each left in the working maps, then the most
// facts packed at once after the store's first pack. Buffered
// evaluation holds none in compact form.
const (
	timelineCountFacts = iota
	timelineCountNotes
	timelineCountPlays
	timelineCountCompactFacts
	timelineCountCompactNotes
	timelineCountCompactPlays
	timelineCountWorkingFacts
	timelineCountWorkingNotes
	timelineCountWorkingPlays
	// The most facts packed at once after the store's first pack.
	timelineCountLargestLaterPack
	timelineCountKinds
)

// timelineObserver is a test observation point only. When set, it is called
// each time a timeline service finalizes, with the record counts and a
// function that dumps every fact, note timing origin and note playback
// context canonically, as the service's readers see them.
var timelineObserver func(dump func() string, counts [timelineCountKinds]int)

func setTimelineObserver(observer func(func() string, [timelineCountKinds]int)) func(func() string, [timelineCountKinds]int) {
	previous := timelineObserver
	timelineObserver = observer
	return previous
}

func (service *timelineService) observe() {
	if timelineObserver == nil {
		return
	}
	var counts [timelineCountKinds]int
	counts[timelineCountFacts] = service.factCount()
	counts[timelineCountNotes] = len(service.NoteOrigins)
	counts[timelineCountPlays] = service.playbackNoteCount()
	counts[timelineCountWorkingFacts] = len(service.Facts)
	counts[timelineCountWorkingNotes] = len(service.NoteOrigins)
	counts[timelineCountWorkingPlays] = len(service.Playback.Notes)
	if store := service.compact; store != nil {
		counts[timelineCountNotes] += int(store.noteCount) + len(store.overflowNotes)
		counts[timelineCountCompactFacts] = store.factTotal - len(store.overflowFacts)
		counts[timelineCountCompactNotes] = int(store.noteCount)
		counts[timelineCountCompactPlays] = int(store.playCount)
		counts[timelineCountLargestLaterPack] = store.largestLaterPack
	}
	timelineObserver(service.dump, counts)
}

func (service *timelineService) dump() string {
	ids := map[NodeID]bool{}
	for id := range service.Facts {
		ids[id] = true
	}
	for id := range service.NoteOrigins {
		ids[id] = true
	}
	for id := range service.Playback.Notes {
		ids[id] = true
	}
	if store := service.compact; store != nil {
		for at := uint32(0); at < store.eventCount; at++ {
			ids[store.events[at/timelineCompactChunk][at%timelineCompactChunk].id] = true
		}
	}
	sorted := make([]NodeID, 0, len(ids))
	for id := range ids {
		sorted = append(sorted, id)
	}
	slices.Sort(sorted)
	var b strings.Builder
	for _, id := range sorted {
		fmt.Fprintf(&b, "%d", id)
		if f, ok := service.fact(id); ok {
			b.WriteString(" fact=")
			dumpValue(&b, reflect.ValueOf(f))
		}
		if n, ok := service.noteOrigin(id); ok {
			b.WriteString(" note=")
			dumpValue(&b, reflect.ValueOf(n))
		}
		if p, ok := service.playbackNote(id); ok {
			b.WriteString(" play=")
			dumpValue(&b, reflect.ValueOf(p))
		}
		b.WriteByte('\n')
	}
	// The rest of the service, except the records above and the compact
	// store that holds them.
	rest := *service
	rest.Facts, rest.NoteOrigins, rest.Playback.Notes, rest.compact, rest.Final = nil, nil, nil, nil, false
	b.WriteString("service=")
	dumpValue(&b, reflect.ValueOf(rest))
	b.WriteByte('\n')
	return b.String()
}

// dumpValue writes v with pointers followed and no addresses, so that equal
// dumps mean reflect.DeepEqual values.
func dumpValue(b *strings.Builder, v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			b.WriteString("nil")
			return
		}
		b.WriteByte('&')
		dumpValue(b, v.Elem())
	case reflect.Struct:
		b.WriteByte('{')
		for i := 0; i < v.NumField(); i++ {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(v.Type().Field(i).Name)
			b.WriteByte(':')
			dumpValue(b, v.Field(i))
		}
		b.WriteByte('}')
	case reflect.Slice:
		if v.IsNil() {
			b.WriteString("nil")
			return
		}
		fallthrough
	case reflect.Array:
		b.WriteByte('[')
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				b.WriteByte(' ')
			}
			dumpValue(b, v.Index(i))
		}
		b.WriteByte(']')
	case reflect.Map:
		if v.IsNil() {
			b.WriteString("nil")
			return
		}
		entries := make([]string, 0, v.Len())
		for it := v.MapRange(); it.Next(); {
			var e strings.Builder
			dumpValue(&e, it.Key())
			e.WriteByte(':')
			dumpValue(&e, it.Value())
			entries = append(entries, e.String())
		}
		slices.Sort(entries)
		b.WriteString("map[")
		b.WriteString(strings.Join(entries, " "))
		b.WriteByte(']')
	case reflect.Interface:
		if v.IsNil() {
			b.WriteString("nil")
			return
		}
		dumpValue(b, v.Elem())
	case reflect.Func:
		if v.IsNil() {
			b.WriteString("nil")
		} else {
			b.WriteString("func")
		}
	case reflect.String:
		fmt.Fprintf(b, "%q", v.String())
	case reflect.Bool:
		fmt.Fprint(b, v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fmt.Fprint(b, v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		fmt.Fprint(b, v.Uint())
	default:
		fmt.Fprintf(b, "<%s>", v.Kind())
	}
}
