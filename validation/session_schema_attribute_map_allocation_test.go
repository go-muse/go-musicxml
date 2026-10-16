package validation

import (
	"errors"
	"fmt"
	"runtime"
	"testing"
	"unsafe"
)

// Force the index and its first group onto the heap. This is deliberately a
// standalone production-equivalent allocation, independent of the lease's
// sizing arithmetic and conservative even if the caller can stack-allocate it.
//
//go:noinline
func allocatedSchemaAttributeIndex(names []QName) map[QName]int {
	index := make(map[QName]int, len(names))
	for i, name := range names {
		index[name] = i
	}
	return index
}

func TestSchemaAttributeScratchEnvelopeCoversMeanMeasuredMapAllocation(t *testing.T) {
	for _, count := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 14, 15, 16, 28, 29, 56, 57, 112, 113, 224, 225, 448, 449, 896, 897, 1000, 1792, 1793, 4096} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			names := make([]QName, count)
			for i := range names {
				names[i] = QName{Space: "urn:attribute-index", Local: fmt.Sprintf("name-%d", i)}
			}
			// Inputs and the outer holder are allocated before measurement; only
			// the hinted maps and their filled storage remain in the measured
			// interval. Multiple random hash seeds cover ordinary table splits.
			const samples = 64
			indexes := make([]map[QName]int, samples)
			runtime.GC()
			var before, after runtime.MemStats
			var cumulative, maximum uint64
			for i := range indexes {
				runtime.ReadMemStats(&before)
				indexes[i] = allocatedSchemaAttributeIndex(names)
				runtime.ReadMemStats(&after)
				allocated := after.TotalAlloc - before.TotalAlloc
				cumulative += allocated
				maximum = max(maximum, allocated)
			}
			runtime.KeepAlive(indexes)
			// TotalAlloc is process-wide and cumulative: an individual interval
			// can include unrelated runtime allocations and replaced map tables
			// that are already dead. For example, a 1793-entry hint can grow
			// three independent 512-slot tables, leaving all three old arrays in
			// TotalAlloc although only the current old/new overlap is live.
			// Average allocation catches ordinary under-admission without
			// turning that cumulative maximum into a false live-byte contract.
			// Neither measurement proves a universal per-map live peak.
			measured := (cumulative + samples - 1) / samples
			admitted, err := schemaAttributeIndexBytes(int64(count))
			if err != nil {
				t.Fatal(err)
			}
			if uint64(admitted) < measured {
				t.Fatalf("%d attributes: map allocated %d mean bytes/op, lease admits only %d", count, measured, admitted)
			}
			t.Logf("%d attributes: mean measured %d bytes/op <= admitted %d; diagnostic maximum cumulative interval %d bytes", count, measured, admitted, maximum)
		})
	}
}

func TestSchemaAttributeScratchSizingRejectsOverflow(t *testing.T) {
	for _, count := range []int64{1<<63 - 1, int64(^uint(0)>>1)/schemaAttributeIndexSlotBytes + 1} {
		if _, err := schemaAttributeIndexBytes(count); !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("unallocatable attribute index of %d entries: %v", count, err)
		}
	}
}

// The fixed allowance covers a header plus one eight-slot group. At larger
// hints the entry allowance covers three slots, including each control byte,
// beyond the Swiss table's load-factor and power-of-two capacity expansion.
// Allocator rounding and table/directory overhead are checked above using real
// heap bytes rather than inferred from these payload-size guards alone.
const _ = uint64(schemaAttributeIndexHeaderBytes) - 64 - 8*uint64(unsafe.Sizeof(QName{})+unsafe.Sizeof(int(0))+1)
const _ = uint64(schemaAttributeIndexSlotBytes) - 3*uint64(unsafe.Sizeof(QName{})+unsafe.Sizeof(int(0))+1)
