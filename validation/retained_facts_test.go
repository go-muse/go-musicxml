package validation

import (
	"context"
	"reflect"
	"testing"
	"unsafe"
)

func TestRetainedFactNodeCompactEnvelope(t *testing.T) {
	if n := unsafe.Sizeof(factNode{}); n > 320 {
		t.Fatalf("retained fact node occupies %d bytes; compact inline envelope is 320", n)
	}
}

func TestRetainedFactCanonicalClosureStorage(t *testing.T) {
	s := representationFixture(t, Budgets{}, false)
	defer s.cleanup()
	representationOpen(t, s)
	for id := NodeID(2); id <= 3; id++ {
		if err := s.Open(context.Background(), representationHeader(id, 1, "test.child", "child")); err != nil {
			t.Fatal(err)
		}
		if err := s.Close(context.Background(), representationEnd(id)); err != nil {
			t.Fatal(err)
		}
	}
	a := reflect.ValueOf(s.nodes[2].end).FieldByName("ChildrenComplete")
	b := reflect.ValueOf(s.nodes[3].end).FieldByName("ChildrenComplete")
	if a.Kind() != reflect.Pointer || b.Kind() != reflect.Pointer {
		t.Fatal("ordinary closure completeness is still stored inline per occurrence")
	}
	if a.Pointer() != b.Pointer() {
		t.Fatal("exact canonical completeness is not shared within session")
	}
}

func TestRetainedFactOpenClosureStaysZero(t *testing.T) {
	s := representationFixture(t, Budgets{}, false)
	defer s.cleanup()
	representationOpen(t, s)
	n := s.nodes[1]
	if n.closed || n.end.ChildrenComplete.State != "" || n.end.AttributesComplete.State != "" || n.end.Order.State != "" || n.end.Scalar.Lexeme.State != "" || n.end.Scalar.Value.State != "" {
		t.Fatal("unclosed occurrence gained closure evidence")
	}
}
