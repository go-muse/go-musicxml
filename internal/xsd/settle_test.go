package xsd

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// settleSchema has a root of count children whose simple content has the
// given builtin type, with a required attribute, an attribute with a default,
// an optional attribute and an xs:ID attribute.
func settleSchema(t *testing.T, count int, content string) *Compiled {
	t.Helper()
	xs := func(local string) TypeRef { return TypeRef{Name: QName{Space: XSDNamespace, Local: local}} }
	child := &ComplexSchema{Form: ComplexSimpleContentExtension, Base: &TypeRef{Name: QName{Space: XSDNamespace, Local: content}}, Attributes: []AttributeSchema{
		{Name: QName{Local: "required"}, Type: xs("integer"), Use: AttributeRequired},
		{Name: QName{Local: "number"}, Type: xs("integer"), Constraint: &ValueConstraint{Kind: "default", Value: "1"}},
		{Name: QName{Local: "optional"}, Type: xs("string")},
		{Name: QName{Local: "id"}, Type: xs("ID")},
	}}
	root := &ComplexSchema{Form: ComplexDirect, Particle: &ParticleSchema{Kind: ParticleElement, Occurrence: Occurrence{Min: 1, Unbounded: true}, Element: &ElementSchema{Name: QName{Local: "child"}, Type: TypeRef{InlineComplex: child}}}}
	c, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "root"}: {Name: QName{Local: "root"}, Type: TypeRef{InlineComplex: root}}}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

type settleChild struct {
	text  string
	attrs []Attribute
}

func settleSubjects(children []settleChild) (Subject, []Subject) {
	root := Subject{ID: 1, Name: QName{Local: "root"}, Path: "/root", TextKnown: true, AttributesComplete: true, ChildrenComplete: true}
	subjects := make([]Subject, len(children))
	for i, c := range children {
		id := SubjectID(i + 2)
		root.Children = append(root.Children, Child{ID: id, Name: QName{Local: "child"}, Path: "/root/child"})
		subjects[i] = Subject{ID: id, Parent: 1, Name: QName{Local: "child"}, Path: "/root/child", Text: c.text, TextKnown: true, AttributesComplete: true, ChildrenComplete: true, Attributes: c.attrs}
	}
	return root, subjects
}

func settleAttr(name, value string) Attribute {
	return Attribute{Attr: Attr{Name: QName{Local: name}, Value: value}, ValueKnown: true}
}

type settleView struct {
	results    NodeResults
	scalar     ScalarResult
	actual     TypeID
	known      bool
	attributes []AttributeResult
}

// A settled subject reads what its binding read before Settle: the same
// results, and with its typed results kept the same scalar, actual type and
// attribute results in the same order, for valid, invalid, defaulted and
// absent attributes and content. Without them it reads the same results and
// no typed result. Its binding then reads nothing, it cannot be settled or
// bound again, and closing the run releases everything.
func TestSettledSubjectsReadWhatTheirBindingRead(t *testing.T) {
	ctx := context.Background()
	children := []settleChild{
		{"3.5", []Attribute{settleAttr("required", "2"), settleAttr("id", "a")}},
		{"x", []Attribute{settleAttr("required", "x"), settleAttr("number", "7"), settleAttr("optional", "o")}},
		{"", []Attribute{settleAttr("id", "a")}},
		{"12", []Attribute{settleAttr("required", "1"), settleAttr("bogus", "b")}},
	}
	for _, keep := range []bool{true, false} {
		t.Run(fmt.Sprint("keep=", keep), func(t *testing.T) {
			c := settleSchema(t, len(children), "decimal")
			root, subjects := settleSubjects(children)
			m := &boundaryMeter{}
			r, err := NewRun(c, m)
			if err != nil {
				t.Fatal(err)
			}
			use, _ := r.Root(root.Name)
			b, err := r.Bind(ctx, use, root)
			if err != nil {
				t.Fatal(err)
			}
			var completions []Completion
			for i, w := range r.Witnesses(b) {
				bound, err := r.Bind(ctx, w.Use, subjects[i])
				if err != nil {
					t.Fatal(err)
				}
				completion, err := r.Complete(ctx, bound, nil)
				if err != nil {
					t.Fatal(err)
				}
				completions = append(completions, completion)
				before := settleView{results: r.Results(bound), scalar: r.Scalar(bound), attributes: slices.Collect(r.Attributes(bound))}
				before.actual, before.known = r.ActualType(bound)
				settled, err := r.Settle(ctx, bound, keep)
				if err != nil {
					t.Fatal(err)
				}
				after := settleView{results: settled.Results(), scalar: settled.Scalar(), attributes: slices.Collect(settled.Attributes())}
				after.actual, after.known = settled.ActualType()
				if !keep {
					if settled.Typed() || after.known || len(after.attributes) != 0 || !reflect.DeepEqual(after.scalar, ScalarResult{}) {
						t.Fatalf("child %d: released typed results still read: %+v", i, after)
					}
					before = settleView{results: before.results}
				}
				if !settled.Valid() || !reflect.DeepEqual(before, after) {
					t.Fatalf("child %d: settled reads\n%+v\nbinding read\n%+v", i, after, before)
				}
				if got := r.Results(bound); !reflect.DeepEqual(got, NodeResults{}) || !reflect.DeepEqual(r.Scalar(bound), ScalarResult{}) || len(slices.Collect(r.Attributes(bound))) != 0 {
					t.Fatalf("child %d: the binding still reads its results", i)
				}
				if _, err := r.Settle(ctx, bound, keep); err == nil {
					t.Fatalf("child %d: settled twice", i)
				}
				if _, err := r.Bind(ctx, w.Use, subjects[i]); err == nil {
					t.Fatalf("child %d: bound again after Settle", i)
				}
			}
			if _, err := r.Complete(ctx, b, completions); err != nil {
				t.Fatal(err)
			}
			if _, err := r.Settle(ctx, b, keep); err != nil {
				t.Fatal(err)
			}
			if err := r.Close(); err != nil || m.buffer != 0 {
				t.Fatalf("close left %d: %v", m.buffer, err)
			}
		})
	}
}

// A subject that is bound and not complete cannot be settled.
func TestSettleRefusesIncompleteSubjects(t *testing.T) {
	ctx := context.Background()
	root, _ := settleSubjects([]settleChild{{"1", nil}})
	r, err := NewRun(settleSchema(t, 1, "decimal"), &boundaryMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	use, _ := r.Root(root.Name)
	b, err := r.Bind(ctx, use, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Settle(ctx, b, false); err == nil {
		t.Fatal("settled a subject before Complete")
	}
	if _, err := r.Settle(ctx, Binding{}, false); err == nil {
		t.Fatal("settled the zero binding")
	}
}

// Settling releases what the run forgets and no more: at hostile sizes, 200
// children each with 40 attribute results (a 16 KiB value among them) and
// 16 KiB of xs:ID content, the run's live heap after every child is settled
// never exceeds its retained charge, with the typed results kept or released;
// and releasing them releases at least their attribute records.
func TestSettledSubjectsKeepTheirHeapCharged(t *testing.T) {
	const count = 200
	ctx := context.Background()
	xs := func(local string) TypeRef { return TypeRef{Name: QName{Space: XSDNamespace, Local: local}} }
	var attrs []AttributeSchema
	for i := range 40 {
		attrs = append(attrs, AttributeSchema{Name: QName{Local: fmt.Sprintf("a%d", i)}, Type: xs("string")})
	}
	child := &ComplexSchema{Form: ComplexSimpleContentExtension, Base: &TypeRef{Name: QName{Space: XSDNamespace, Local: "ID"}}, Attributes: attrs}
	rootType := &ComplexSchema{Form: ComplexDirect, Particle: &ParticleSchema{Kind: ParticleElement, Occurrence: Occurrence{Min: count, Max: count}, Element: &ElementSchema{Name: QName{Local: "child"}, Type: TypeRef{InlineComplex: child}}}}
	c, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "root"}: {Name: QName{Local: "root"}, Type: TypeRef{InlineComplex: rootType}}}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	children := make([]settleChild, count)
	for i := range children {
		var values []Attribute
		for j := range 20 {
			value := fmt.Sprint(j)
			if j == 0 {
				value = strings.Repeat("v", 16<<10)
			}
			values = append(values, settleAttr(fmt.Sprintf("a%d", j), value))
		}
		children[i] = settleChild{text: fmt.Sprintf("i%d", i) + strings.Repeat("x", 16<<10), attrs: values}
	}
	charged := map[bool]int64{}
	for _, keep := range []bool{true, false} {
		root, subjects := settleSubjects(children)
		m := &boundaryMeter{}
		before := completedRetentionHeap()
		r, err := NewRun(c, m)
		if err != nil {
			t.Fatal(err)
		}
		use, _ := r.Root(root.Name)
		b, err := r.Bind(ctx, use, root)
		if err != nil {
			t.Fatal(err)
		}
		var completions []Completion
		var kept []Settled
		for i, w := range r.Witnesses(b) {
			bound, err := r.Bind(ctx, w.Use, subjects[i])
			if err != nil {
				t.Fatal(err)
			}
			completion, err := r.Complete(ctx, bound, nil)
			if err != nil {
				t.Fatal(err)
			}
			completions = append(completions, completion)
			settled, err := r.Settle(ctx, bound, keep)
			if err != nil {
				t.Fatal(err)
			}
			kept = append(kept, settled)
		}
		if _, err := r.Complete(ctx, b, completions); err != nil {
			t.Fatal(err)
		}
		heap := completedRetentionHeap() - before
		if heap > m.buffer {
			t.Fatalf("keep=%v: live heap %d exceeds the retained charge %d", keep, heap, m.buffer)
		}
		charged[keep] = m.buffer
		runtime.KeepAlive(subjects)
		runtime.KeepAlive(kept)
		if err := r.Close(); err != nil || m.buffer != 0 {
			t.Fatalf("close left %d: %v", m.buffer, err)
		}
	}
	// 200 × 20 present attribute records of 336 bytes at least.
	if released := charged[true] - charged[false]; released < count*20*336 {
		t.Fatalf("releasing typed results released %d bytes", released)
	}
}
