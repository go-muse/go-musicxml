package xsd

import (
	"context"
	"fmt"
	"slices"
	"testing"
)

// An open element gives each child, as it opens, the witness Bind gives it
// once the element closes, valid or not, and none from the first child its
// content model rejects. Closing the open element releases its charge: what
// stays charged after the element is bound is what binding alone charges.
func TestOpenWitnessesMatchBind(t *testing.T) {
	ctx := context.Background()
	c := prefixRunSchema(t)
	use, _ := c.Root(QName{Local: "root"})
	for _, children := range [][]string{{"child", "child", "tail"}, {"child", "tail", "bogus"}, {"child", "bogus", "child", "tail"}, {"tail"}} {
		t.Run(fmt.Sprint(children), func(t *testing.T) {
			subject := Subject{ID: 1, Name: QName{Local: "root"}, Path: "/root", TextKnown: true, AttributesComplete: true, ChildrenComplete: true}
			for i, name := range children {
				subject.Children = append(subject.Children, Child{ID: SubjectID(i + 2), Name: QName{Local: name}, Path: "/root/" + name})
			}
			alone := &testMeter{}
			ra, err := NewRun(c, alone)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ra.Bind(ctx, use, subject); err != nil {
				t.Fatal(err)
			}
			want := alone.retained
			_ = ra.Close()
			m := &testMeter{}
			r, err := NewRun(c, m)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			open, err := r.Open(ctx, use, Subject{ID: 1, Name: subject.Name, Path: subject.Path, AttributesComplete: true})
			if err != nil || open == nil {
				t.Fatalf("open=%v err=%v", open, err)
			}
			var pushed []Witness
			for _, child := range subject.Children {
				w, ok, err := r.PushChild(ctx, open, child)
				if err != nil {
					t.Fatal(err)
				}
				if ok {
					pushed = append(pushed, w)
				}
			}
			if err := r.CloseOpen(open); err != nil {
				t.Fatal(err)
			}
			b, err := r.Bind(ctx, use, subject)
			if err != nil {
				t.Fatal(err)
			}
			// The open element leaves only its node, which Bind reuses, and
			// schema caches Bind fills anyway.
			if m.retained != want {
				t.Fatalf("open and bind keep %d bytes charged, bind alone %d", m.retained, want)
			}
			if bound := r.Witnesses(b); !slices.Equal(pushed, bound) {
				t.Fatalf("pushed %+v, Bind gives %+v", pushed, bound)
			}
		})
	}
}

// Open declines an element whose children Bind could witness differently:
// incomplete attributes, a schema-instance attribute, or a type without
// element content.
func TestOpenDeclinesUnprovableElements(t *testing.T) {
	ctx := context.Background()
	c := prefixRunSchema(t)
	use, _ := c.Root(QName{Local: "root"})
	r, err := NewRun(c, &testMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for name, subject := range map[string]Subject{
		"incomplete attributes": {ID: 1, Name: QName{Local: "root"}},
		"schema instance":       {ID: 1, Name: QName{Local: "root"}, AttributesComplete: true, Attributes: []Attribute{{Attr: Attr{Name: QName{Space: XSINamespace, Local: "type"}, Value: "t"}, ValueKnown: true}}},
		"another element":       {ID: 1, Name: QName{Local: "other"}, AttributesComplete: true},
	} {
		if open, err := r.Open(ctx, use, subject); err != nil || open != nil {
			t.Errorf("%s: open=%v err=%v", name, open, err)
		}
	}
	simple, err := Compile(SchemaData{Elements: map[QName]*ElementSchema{{Local: "n"}: {Name: QName{Local: "n"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "integer"}}}}}, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	rs, err := NewRun(simple, &testMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	nUse, _ := simple.Root(QName{Local: "n"})
	if open, err := rs.Open(ctx, nUse, Subject{ID: 1, Name: QName{Local: "n"}, AttributesComplete: true}); err != nil || open != nil {
		t.Errorf("simple type: open=%v err=%v", open, err)
	}
}

// identityRun registers IDs and IDREFs at the given document positions, in
// the order listed, and returns the run's identity findings.
func identityRun(t *testing.T, registrations []identityRegistration) []string {
	t.Helper()
	r, err := NewRun(testRunSchema(t), &testMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx := context.Background()
	r.current = ctx
	owners := map[string]*Node{}
	func() {
		defer recoverRun(&err)
		for _, reg := range registrations {
			r.SetIdentityOrder(reg.order)
			kind := TypeRef{Name: QName{Space: XSDNamespace, Local: reg.kind}}
			r.context.RecordIdentity(&kind, reg.value, reg.path)
			if reg.kind == "ID" {
				if owners[reg.path] == nil {
					owners[reg.path] = &Node{Name: QName{Local: reg.path}}
				}
				r.context.RecordIdentityBinding(&kind, reg.value, reg.path, owners[reg.path])
			}
		}
	}()
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Identity(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	var issues []string
	for _, issue := range out.Issues {
		issues = append(issues, issue.Path+": "+issue.Message)
	}
	return issues
}

type identityRegistration struct {
	order             uint64
	kind, value, path string
}

// With document positions set, identity findings do not depend on the order
// subjects are bound in: duplicate IDs name the first two owners in document
// order, and dangling references are reported in document order.
func TestIdentityOrderFollowsDocumentPositions(t *testing.T) {
	inOrder := []identityRegistration{
		{1, "ID", "a", "/first"},
		{2, "IDREF", "missing", "/ref1"},
		{3, "ID", "a", "/second"},
		{4, "IDREF", "gone", "/ref2"},
		{5, "ID", "a", "/third"},
	}
	want := identityRun(t, inOrder)
	if len(want) != 3 {
		t.Fatalf("in-order findings %v", want)
	}
	for _, permutation := range [][]int{{4, 3, 2, 1, 0}, {2, 0, 4, 1, 3}, {1, 4, 0, 3, 2}} {
		var shuffled []identityRegistration
		for _, i := range permutation {
			shuffled = append(shuffled, inOrder[i])
		}
		if got := identityRun(t, shuffled); !slices.Equal(got, want) {
			t.Errorf("order %v: findings %v, want %v", permutation, got, want)
		}
	}
}

// The earliest seed of an ID value orders its entry: a value first registered
// late but seeded earlier by a subject bound later keeps its document
// position among the duplicate-ID findings.
func TestIdentityEntryTakesTheEarliestSeed(t *testing.T) {
	inOrder := []identityRegistration{
		{1, "ID", "a", "/a1"},
		{3, "ID", "b", "/b1"},
		{4, "ID", "b", "/b2"},
		{5, "ID", "a", "/a2"},
	}
	want := identityRun(t, inOrder)
	if len(want) != 2 {
		t.Fatalf("in-order findings %v", want)
	}
	shuffled := []identityRegistration{inOrder[3], inOrder[1], inOrder[2], inOrder[0]}
	if got := identityRun(t, shuffled); !slices.Equal(got, want) {
		t.Fatalf("findings %v, want %v", got, want)
	}
}

// Children bound before their parent, as windows are, leave the run with the
// same charge as binding top-down once the parent completes: the parent
// takes over the charge of the nodes its children created.
func TestBindingChildrenFirstKeepsTheTopDownCharge(t *testing.T) {
	ctx := context.Background()
	c := prefixRunSchema(t)
	use, _ := c.Root(QName{Local: "root"})
	root := Subject{ID: 1, Name: QName{Local: "root"}, Path: "/root", TextKnown: true, AttributesComplete: true, ChildrenComplete: true,
		Children: []Child{{ID: 2, Name: QName{Local: "child"}, Path: "/root/child"}, {ID: 3, Name: QName{Local: "tail"}, Path: "/root/tail"}}}
	child := func(id SubjectID, name string) Subject {
		return Subject{ID: id, Parent: 1, Name: QName{Local: name}, Path: "/root/" + name, Text: "1", TextKnown: true, AttributesComplete: true, ChildrenComplete: true}
	}
	bindLeaf := func(r *Run, w Witness, name string) Completion {
		b, err := r.Bind(ctx, w.Use, child(w.Child, name))
		if err != nil {
			t.Fatal(err)
		}
		done, err := r.Complete(ctx, b, nil)
		if err != nil {
			t.Fatal(err)
		}
		return done
	}
	names := map[SubjectID]string{2: "child", 3: "tail"}

	topDown := &testMeter{}
	r, err := NewRun(c, topDown)
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.Bind(ctx, use, root)
	if err != nil {
		t.Fatal(err)
	}
	var done []Completion
	for _, w := range r.Witnesses(b) {
		done = append(done, bindLeaf(r, w, names[w.Child]))
	}
	if _, err := r.Complete(ctx, b, done); err != nil {
		t.Fatal(err)
	}
	want := topDown.retained
	_ = r.Close()

	childrenFirst := &testMeter{}
	r, err = NewRun(c, childrenFirst)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	open, err := r.Open(ctx, use, Subject{ID: 1, Name: root.Name, Path: root.Path, AttributesComplete: true})
	if err != nil || open == nil {
		t.Fatalf("open=%v err=%v", open, err)
	}
	done = nil
	for _, ch := range root.Children {
		w, ok, err := r.PushChild(ctx, open, ch)
		if err != nil || !ok {
			t.Fatalf("push %v: ok=%v err=%v", ch, ok, err)
		}
		done = append(done, bindLeaf(r, w, names[w.Child]))
	}
	if err := r.CloseOpen(open); err != nil {
		t.Fatal(err)
	}
	b, err = r.Bind(ctx, use, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Complete(ctx, b, done); err != nil {
		t.Fatal(err)
	}
	if childrenFirst.retained != want {
		t.Fatalf("children first keep %d bytes charged, top-down %d", childrenFirst.retained, want)
	}
}
