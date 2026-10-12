package validation

import (
	"context"
	"errors"
	"testing"
	"unsafe"
)

func TestIndependentTimelineOriginRecordAllowances(t *testing.T) {
	for _, v := range []struct {
		name             string
		width, allowance int64
	}{{"note", int64(unsafe.Sizeof(noteTimingOrigin{})), timelineNoteReserveBytes}, {"direction", int64(unsafe.Sizeof(directionTimingOrigin{})), timelineDirectionReserveBytes}} {
		if v.allowance < v.width+512 {
			t.Errorf("%s allowance%d below fixed%d+512 map headroom", v.name, v.allowance, v.width)
		}
	}
}

func TestIndependentTimelineOriginFixedPeakAdmission(t *testing.T) {
	for _, kind := range []string{"note", "direction"} {
		allowance := timelineNoteReserveBytes
		if kind == "direction" {
			allowance = timelineDirectionReserveBytes
		}
		for _, short := range []bool{false, true} {
			limit := 2 * allowance
			if short {
				limit--
			}
			s, v, b, fact := independentOriginFixture(t, Budgets{MaxBufferedBytes: limit})
			var err error
			if kind == "note" {
				note := stateNode(b, timelineRoleNote, 10, stateAbsent[Atomic]())
				err = v.readNoteOrigins(context.Background(), s, b, note, fact, stateKnown(stateDecimal(t, 3, 1)))
			} else {
				dir := stateNode(b, timelineRoleDirection, 10, stateAbsent[Atomic]())
				block := runtimeBlock(b, 0, dir)
				block.part = stateUnknown[NodeRef]("unknown-owner")
				v.Facts[10] = timelineEventFact{Part: block.part, Origin: stateUnknown[musicalPosition]("unknown-origin"), Position: stateUnknown[musicalPosition]("unknown-position"), Staff: stateAbsent[Integer](), Voice: stateAbsent[string]()}
				err = v.resolveDirectionOrigins(context.Background(), s, b, []timelineMeasureBlock{block})
			}
			if short {
				if !errors.Is(err, ErrResourceLimit) || len(v.NoteOrigins)+len(v.DirectionOrigins) != 0 {
					t.Fatalf("%s peak-1 published or wrong error: %v", kind, err)
				}
			} else {
				if err != nil || len(v.NoteOrigins)+len(v.DirectionOrigins) != 1 {
					t.Fatalf("%s exactpeak failed: %v", kind, err)
				}
				if got := rationalUsage(t, s, BudgetBufferedBytes); got != allowance {
					t.Fatalf("%s fixed temporary scratch retained: got%d want%d", kind, got, allowance)
				}
			}
			if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
				t.Fatalf("%s scratch leaked", kind)
			}
			s.cleanup()
			if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
				t.Fatal("cleanup leaked")
			}
		}
	}
}

func TestIndependentTimelineEventFactFixedPeakAdmission(t *testing.T) {
	if timelineFactReserveBytes < int64(unsafe.Sizeof(timelineEventFact{}))+512 {
		t.Fatal("event fact allowance lost map headroom")
	}
	for _, short := range []bool{false, true} {
		limit := 2 * timelineFactReserveBytes
		if short {
			limit--
		}
		s := rationalSession(t, Budgets{MaxBufferedBytes: limit})
		defer s.cleanup()
		v := &timelineService{Facts: make(map[NodeID]timelineEventFact)}
		err := v.storeFact(context.Background(), s, 1, timelineEventFact{})
		if short {
			if !errors.Is(err, ErrResourceLimit) || len(v.Facts) != 0 {
				t.Fatalf("fact peak-1 published or wrong error: %v", err)
			}
		} else {
			if err != nil || len(v.Facts) != 1 {
				t.Fatalf("exact event peak failed: %v", err)
			}
			if got := rationalUsage(t, s, BudgetBufferedBytes); got != timelineFactReserveBytes {
				t.Fatalf("fixed temporary retained: got%d want%d", got, timelineFactReserveBytes)
			}
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != s.retained {
			t.Fatal("event scratch leaked")
		}
	}
}
