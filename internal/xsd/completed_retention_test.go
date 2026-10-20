package xsd

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
)

func completedRetentionHeap() int64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.HeapAlloc)
}

// What a completed subject keeps stays charged at hostile sizes: 200
// children with 16 KiB string content, whose kept scalars share their node
// text buffers, and an xs:ID attribute each, whose identity owners keep their
// nodes after the parent completes. After completion the run's live heap
// never exceeds its retained charge.
func TestCompletedSubjectsKeepTheirHeapCharged(t *testing.T) {
	const count = 200
	child := &ComplexSchema{Form: ComplexSimpleContentExtension, Base: &TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}}, Attributes: []AttributeSchema{{Name: QName{Local: "id"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "ID"}}}}}
	root := &ComplexSchema{Form: ComplexDirect, Particle: &ParticleSchema{Kind: ParticleElement, Occurrence: Occurrence{Min: count, Max: count}, Element: &ElementSchema{Name: QName{Local: "child"}, Type: TypeRef{InlineComplex: child}}}}
	c, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "root"}: {Name: QName{Local: "root"}, Type: TypeRef{InlineComplex: root}}}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	subject := Subject{ID: 1, Name: QName{Local: "root"}, Path: "/root", TextKnown: true, AttributesComplete: true, ChildrenComplete: true}
	children := make([]Subject, count)
	for i := range children {
		id := SubjectID(i + 2)
		subject.Children = append(subject.Children, Child{ID: id, Name: QName{Local: "child"}, Path: "/root/child"})
		children[i] = Subject{ID: id, Parent: 1, Name: QName{Local: "child"}, Path: "/root/child", Text: strings.Repeat(string(rune('a'+i%26)), 16<<10), TextKnown: true, AttributesComplete: true, ChildrenComplete: true, Attributes: []Attribute{{Attr: Attr{Name: QName{Local: "id"}, Value: fmt.Sprintf("i%d", i)}, ValueKnown: true}}}
	}
	ctx := context.Background()
	m := &boundaryMeter{}
	before := completedRetentionHeap()
	r, err := NewRun(c, m)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := r.Root(subject.Name)
	b, err := r.Bind(ctx, u, subject)
	if err != nil {
		t.Fatal(err)
	}
	var completions []Completion
	for i, w := range r.Witnesses(b) {
		bound, err := r.Bind(ctx, w.Use, children[i])
		if err != nil {
			t.Fatal(err)
		}
		completion, err := r.Complete(ctx, bound, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Scalar(bound).Known {
			t.Fatalf("child %d: scalar not kept", i)
		}
		completions = append(completions, completion)
	}
	if _, err := r.Complete(ctx, b, completions); err != nil {
		t.Fatal(err)
	}
	heap := completedRetentionHeap() - before
	if heap > m.buffer {
		t.Fatalf("live heap %d exceeds the retained charge %d", heap, m.buffer)
	}
	runtime.KeepAlive(children)
	if err := r.Close(); err != nil || m.buffer != 0 {
		t.Fatalf("close left %d: %v", m.buffer, err)
	}
}

// An identity owner keeps its node alive after the parent completes and
// releases the node's charge, so each new owner entry is charged the node as
// well as the entry.
func TestIdentityOwnerEntryChargesTheNode(t *testing.T) {
	run, _ := newIdentityResolverRun(t)
	registerResolverOwner(run, "first", &Node{ID: 10})
	registerResolverOwner(run, "first", &Node{ID: 11})
	// A third owner adds neither the first nor the second owner's evidence,
	// only the entry, its node and the collapsed value's builder.
	before := run.retained
	registerResolverOwner(run, "first", &Node{ID: 12})
	want := 32 + nodeBytes + run.context.builderAllowance(int64(len("first")))
	if got := run.retained - before; got != want {
		t.Fatalf("third owner charged %d, want %d", got, want)
	}
}
