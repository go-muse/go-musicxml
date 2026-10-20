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
	want := 32 + nodeObjectBytes + run.context.builderAllowance(int64(len("first")))
	if got := run.retained - before; got != want {
		t.Fatalf("third owner charged %d, want %d", got, want)
	}
}

// A rejected subject keeps its charge for the node map: deleting the
// placeholder nodes of unexpected children frees the node objects but not the
// map's backing storage. A root with an empty content model and 10,000 or
// 100,000 unexpected children binds none of them; after it completes, the
// run's live heap never exceeds its retained charge.
func TestRejectedChildrenKeepTheNodeMapCharged(t *testing.T) {
	c, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "root"}: {Name: QName{Local: "root"}, Type: TypeRef{InlineComplex: &ComplexSchema{Form: ComplexDirect}}}}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	for _, count := range []int{10_000, 100_000} {
		subject := Subject{ID: 1, Name: QName{Local: "root"}, Path: "/root", TextKnown: true, AttributesComplete: true, ChildrenComplete: true}
		for i := range count {
			subject.Children = append(subject.Children, Child{ID: SubjectID(i + 2), Name: QName{Local: "unexpected"}, Path: "/root/unexpected"})
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
		if r.Results(b).Content.Valid {
			t.Fatalf("%d: unexpected children accepted", count)
		}
		if _, err := r.Complete(ctx, b, nil); err != nil {
			t.Fatal(err)
		}
		heap := completedRetentionHeap() - before
		if heap > m.buffer || len(r.nodes) != 1 {
			t.Fatalf("%d children: live heap %d exceeds the retained charge %d (nodes %d)", count, heap, m.buffer, len(r.nodes))
		}
		runtime.KeepAlive(subject)
		if err := r.Close(); err != nil || m.buffer != 0 {
			t.Fatalf("close left %d: %v", m.buffer, err)
		}
	}
}

// The node map is charged by the most entries ever live, not per element: a
// root with one part of 200 measures of 20 notes inserts 4,202 nodes, yet at
// most the measures of the part and the notes of one measure are live
// together, so the run charges 222 map entries.
func TestNodeMapChargeFollowsThePeak(t *testing.T) {
	str := TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}}
	element := func(name string, count uint64, child *ElementSchema) *ElementSchema {
		return &ElementSchema{Name: QName{Local: name}, Type: TypeRef{InlineComplex: &ComplexSchema{Form: ComplexDirect, Particle: &ParticleSchema{Kind: ParticleElement, Occurrence: Occurrence{Min: count, Max: count}, Element: child}}}}
	}
	note := &ElementSchema{Name: QName{Local: "note"}, Type: str}
	root := element("root", 1, element("part", 200, element("measure", 20, note)))
	c, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "root"}: root}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	r, err := NewRun(c, &boundaryMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	next := SubjectID(1)
	var bind func(use Use, id, parent SubjectID, name string, children int, childName string, grand func(SubjectID) (int, string)) Completion
	bind = func(use Use, id, parent SubjectID, name string, children int, childName string, grand func(SubjectID) (int, string)) Completion {
		t.Helper()
		subject := Subject{ID: id, Parent: parent, Name: QName{Local: name}, Path: "/" + name, TextKnown: true, AttributesComplete: true, ChildrenComplete: true}
		if children == 0 {
			subject.Text = "n"
		}
		for range children {
			next++
			subject.Children = append(subject.Children, Child{ID: next, Name: QName{Local: childName}, Path: "/" + childName})
		}
		b, err := r.Bind(ctx, use, subject)
		if err != nil {
			t.Fatal(err)
		}
		var completions []Completion
		for _, w := range r.Witnesses(b) {
			count, grandName := grand(w.Child)
			completions = append(completions, bind(w.Use, w.Child, id, childName, count, grandName, func(SubjectID) (int, string) {
				if childName == "part" {
					return 20, "note"
				}
				return 0, ""
			}))
		}
		completion, err := r.Complete(ctx, b, completions)
		if err != nil || !completion.Complete || !completion.DeclarationValid {
			t.Fatalf("%s: %+v %v", name, completion, err)
		}
		return completion
	}
	u, _ := r.Root(QName{Local: "root"})
	bind(u, 1, 0, "root", 1, "part", func(SubjectID) (int, string) { return 200, "measure" })
	// reserveNode charges a map entry exactly when it raises nodesPeak.
	if next != 4202 || r.nodesPeak != 222 || len(r.nodes) != 1 {
		t.Fatalf("inserted %d nodes, peak %d, live %d", next, r.nodesPeak, len(r.nodes))
	}
}
