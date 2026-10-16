package xsd

import (
	"context"
	"fmt"
	"testing"
	"unsafe"
)

func witnessAllocationFixture(t *testing.T, count int, meter Meter) (*Run, *boundState) {
	t.Helper()
	r, err := NewRun(testRunSchema(t), meter)
	if err != nil {
		t.Fatal(err)
	}
	r.current = context.Background()
	u, _ := r.Root(QName{Local: "root"})
	s := &boundState{run: r, actual: &u.original.Type, results: freshResults(), family: "content"}
	owner, _ := r.typeID(s.actual)
	for i := 0; i < count; i++ {
		s.subject.Children = append(s.subject.Children, Child{ID: SubjectID(i + 2), Name: QName{Local: "child"}})
		s.matches = append(s.matches, ElementMatch{Particle: owner.complex.Particle, Index: i})
	}
	r.context.local = s
	t.Cleanup(func() {
		if !r.closed {
			_ = r.Close()
		}
	})
	return r, s
}

func TestWitnessAllocationRetainsOneExactBackingArray(t *testing.T) {
	width := int64(unsafe.Sizeof(Witness{}))
	for _, count := range []int{0, 1, 2, 3, 7, 8, 9, 33} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			m := &boundaryMeter{}
			r, s := witnessAllocationFixture(t, count, m)
			before := r.retained
			r.buildWitnesses(s)
			if len(s.witnesses) != count || cap(s.witnesses) != count {
				t.Errorf("witness backing capacity=%d/%d, want exact %d", len(s.witnesses), cap(s.witnesses), count)
			}
			if got := r.retained - before; got != int64(count)*width {
				t.Errorf("obsolete witness backing allowances retained: got %d, want %d", got, int64(count)*width)
			}
			for i, w := range s.witnesses {
				if w.Child != SubjectID(i+2) || w.Index != i || w.Mode != WitnessDeclared || w.Use.Declaration() != (QName{Local: "child"}) {
					t.Errorf("witness %d changed: %+v", i, w)
				}
			}
			if err := r.Close(); err != nil || m.buffer != 0 {
				t.Fatalf("Close=%v buffered=%d", err, m.buffer)
			}
		})
	}
}
