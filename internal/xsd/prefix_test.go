package xsd

import (
	"context"
	"math/rand"
	"slices"
	"testing"
)

func prefixLeaf(name string, min, max uint64, unbounded bool) *ParticleSchema {
	return &ParticleSchema{Kind: ParticleElement, Occurrence: Occurrence{Min: min, Max: max, Unbounded: unbounded}, Element: &ElementSchema{Name: QName{Local: name}}}
}

func prefixGroup(kind ParticleKind, min, max uint64, unbounded bool, children ...*ParticleSchema) *ParticleSchema {
	return &ParticleSchema{Kind: kind, Occurrence: Occurrence{Min: min, Max: max, Unbounded: unbounded}, Children: children}
}

func prefixChildren(names ...string) []*Node {
	out := make([]*Node, len(names))
	for i, name := range names {
		out[i] = &Node{Name: QName{Local: name}}
	}
	return out
}

// prefixIndexes binds children with a fresh context and returns the bound
// child indexes and their particles.
func prefixBind(p *ParticleSchema, children []*Node) ([]ElementMatch, *prefixMatcher) {
	c := &Context{}
	matcher, ok := c.newPrefixMatcher(p)
	if !ok {
		return nil, nil
	}
	var out []ElementMatch
	for _, child := range children {
		match, bound := matcher.push(child)
		if !bound {
			break
		}
		out = append(out, match)
	}
	return out, matcher
}

// A child keeps the declaration its prefix determines, whatever follows: in a
// sequence, choice, all group and nested groups, children before the first
// child the content model rejects are bound to their own leaf, and the
// rejected child and every later child are not. A child list that is only
// missing a required trailing element binds every child.
func TestPrefixMatchesBindBeforeTheFailurePoint(t *testing.T) {
	a, b, c := prefixLeaf("a", 1, 1, false), prefixLeaf("b", 0, 0, true), prefixLeaf("c", 1, 1, false)
	sequence := prefixGroup(ParticleSequence, 1, 1, false, a, b, c)
	x, y := prefixLeaf("x", 1, 1, false), prefixLeaf("y", 1, 1, false)
	choice := prefixGroup(ParticleChoice, 1, 0, true, x, y)
	p, q, r := prefixLeaf("p", 1, 1, false), prefixLeaf("q", 0, 1, false), prefixLeaf("r", 1, 1, false)
	all := prefixGroup(ParticleAll, 1, 1, false, p, q, r)
	inner := prefixGroup(ParticleSequence, 1, 1, false, prefixLeaf("m", 1, 1, false), prefixGroup(ParticleChoice, 0, 0, true, prefixLeaf("n", 1, 1, false), prefixLeaf("o", 1, 1, false)))
	nested := prefixGroup(ParticleSequence, 1, 0, true, inner)
	for _, tc := range []struct {
		name     string
		particle *ParticleSchema
		children []string
		want     []*ParticleSchema
	}{
		{"valid sequence", sequence, []string{"a", "b", "b", "c"}, []*ParticleSchema{a, b, b, c}},
		{"trailing bogus", sequence, []string{"a", "b", "c", "bogus"}, []*ParticleSchema{a, b, c}},
		{"error in the middle", sequence, []string{"a", "b", "bogus", "b", "c"}, []*ParticleSchema{a, b}},
		{"misplaced known element", sequence, []string{"a", "c", "b"}, []*ParticleSchema{a, c}},
		{"missing trailing element", sequence, []string{"a", "b"}, []*ParticleSchema{a, b}},
		{"first child rejected", sequence, []string{"c", "a"}, nil},
		{"choice", choice, []string{"y", "x", "x", "z", "y"}, []*ParticleSchema{y, x, x}},
		{"all in any order", all, []string{"r", "q", "p"}, []*ParticleSchema{r, q, p}},
		{"all with repeated member", all, []string{"p", "r", "p", "q"}, []*ParticleSchema{p, r}},
		{"all missing required member", all, []string{"q", "p"}, []*ParticleSchema{q, p}},
		{"nested groups", nested, []string{"m", "n", "o", "m", "o", "z", "m"}, []*ParticleSchema{inner.Children[0], inner.Children[1].Children[0], inner.Children[1].Children[1], inner.Children[0], inner.Children[1].Children[1]}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, matcher := prefixBind(tc.particle, prefixChildren(tc.children...))
			if matcher == nil {
				t.Fatal("content model rejected")
			}
			if len(got) != len(tc.want) {
				t.Fatalf("bound %d children, want %d", len(got), len(tc.want))
			}
			for i, match := range got {
				if match.Index != i || match.Particle != tc.want[i] || match.Schema != tc.want[i].Element {
					t.Fatalf("child %d bound to %v, want %v", i, match.Particle.Element.Name, tc.want[i].Element.Name)
				}
			}
			if matcher.ambiguous {
				t.Fatal("binding stopped as ambiguous")
			}
		})
	}
}

// A schema that violates Unique Particle Attribution can offer one child two
// particles; binding stops there instead of choosing one.
func TestPrefixMatchesStopAtAmbiguousChildren(t *testing.T) {
	first, second := prefixLeaf("a", 0, 1, false), prefixLeaf("a", 1, 1, false)
	p := prefixGroup(ParticleSequence, 1, 1, false, first, second)
	got, matcher := prefixBind(p, prefixChildren("a", "a"))
	if len(got) != 0 || !matcher.ambiguous {
		t.Fatalf("bound %d children of an ambiguous model (ambiguous=%v)", len(got), matcher.ambiguous)
	}
}

// prefixRandomParticle draws a content model over a three-name alphabet.
func prefixRandomParticle(rng *rand.Rand, depth int) *ParticleSchema {
	occurrence := func() (uint64, uint64, bool) {
		switch rng.Intn(5) {
		case 0:
			return 0, 1, false
		case 1:
			return 0, 0, true
		case 2:
			return 1, 0, true
		case 3:
			return 2, 3, false
		}
		return 1, 1, false
	}
	names := []string{"a", "b", "c"}
	if depth == 0 || rng.Intn(3) == 0 {
		min, max, unbounded := occurrence()
		return prefixLeaf(names[rng.Intn(len(names))], min, max, unbounded)
	}
	if rng.Intn(6) == 0 {
		var members []*ParticleSchema
		for _, i := range rng.Perm(len(names))[:1+rng.Intn(len(names))] {
			members = append(members, prefixLeaf(names[i], uint64(rng.Intn(2)), 1, false))
		}
		return prefixGroup(ParticleAll, 1, 1, false, members...)
	}
	kind := ParticleSequence
	if rng.Intn(2) == 0 {
		kind = ParticleChoice
	}
	min, max, unbounded := occurrence()
	var children []*ParticleSchema
	for range 1 + rng.Intn(3) {
		children = append(children, prefixRandomParticle(rng, depth-1))
	}
	return prefixGroup(kind, min, max, unbounded, children...)
}

// The push matcher agrees with the content-model matcher on random models
// and child lists. A child list the content model matches binds every child
// to its witness particle unless binding stops as ambiguous, every completion
// of up to four children of a bound prefix that the content model matches
// has the bound particles in its witness, and a prefix the push matcher
// rejects has no completion of up to four children that the content model
// matches.
func TestPrefixMatchesAgreeWithTheContentModel(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	names := []string{"a", "b", "c"}
	matched, rejected, ambiguous := 0, 0, 0
	for range 6000 {
		p := prefixRandomParticle(rng, 4)
		var list []string
		for range rng.Intn(9) {
			list = append(list, names[rng.Intn(len(names))])
		}
		children := prefixChildren(list...)
		c := &Context{}
		result, _, ok := c.MatchParticle(p, children, 0)
		if !ok && c.ParticleNullable == nil {
			continue
		}
		got, matcher := prefixBind(p, children)
		if matcher == nil {
			continue
		}
		if matcher.ambiguous {
			ambiguous++
		}
		if ok && result.Next == len(children) {
			matched++
			if matcher.ambiguous {
				continue
			}
			if len(got) != len(children) {
				t.Fatalf("model %v matches %v but only %d children bind", prefixDescribe(p), list, len(got))
			}
			for i, match := range got {
				if match.Particle != result.Elements[i].Particle {
					t.Fatalf("model %v children %v: child %d binds another particle than its witness", prefixDescribe(p), list, i)
				}
			}
			continue
		}
		// Every completion of the bound prefix that the content model
		// matches has the bound particles in its witness.
		for _, suffix := range prefixSuffixes(names, 4) {
			candidate := append(slices.Clone(list[:len(got)]), suffix...)
			result, _, ok := (&Context{}).MatchParticle(p, prefixChildren(candidate...), 0)
			if !ok || result.Next != len(candidate) {
				continue
			}
			for i, match := range got {
				if match.Particle != result.Elements[i].Particle {
					t.Fatalf("model %v: %v binds child %d to another particle than the witness of %v", prefixDescribe(p), list, i, candidate)
				}
			}
		}
		if matcher.ambiguous || len(got) == len(children) {
			continue
		}
		rejected++
		stop := len(got) + 1
		for _, suffix := range prefixSuffixes(names, 4) {
			candidate := append(slices.Clone(list[:stop]), suffix...)
			result, _, ok := (&Context{}).MatchParticle(p, prefixChildren(candidate...), 0)
			if ok && result.Next == len(candidate) {
				t.Fatalf("model %v: push matcher rejects %v, but %v matches", prefixDescribe(p), list[:stop], candidate)
			}
		}
	}
	if matched < 300 || rejected < 300 {
		t.Fatalf("too few cases: %d matched, %d rejected", matched, rejected)
	}
	t.Logf("%d matched, %d rejected, %d ambiguous", matched, rejected, ambiguous)
}

func prefixSuffixes(names []string, length int) [][]string {
	out := [][]string{nil}
	frontier := [][]string{nil}
	for range length {
		var next [][]string
		for _, suffix := range frontier {
			for _, name := range names {
				next = append(next, append(slices.Clone(suffix), name))
			}
		}
		out = append(out, next...)
		frontier = next
	}
	return out
}

func prefixDescribe(p *ParticleSchema) string {
	occurrence := "{" + string(rune('0'+p.Occurrence.Min)) + ","
	if p.Occurrence.Unbounded {
		occurrence += "*}"
	} else {
		occurrence += string(rune('0'+p.Occurrence.Max)) + "}"
	}
	if p.Kind == ParticleElement {
		return p.Element.Name.Local + occurrence
	}
	out := string(p.Kind) + "("
	for i, child := range p.Children {
		if i > 0 {
			out += " "
		}
		out += prefixDescribe(child)
	}
	return out + ")" + occurrence
}

func prefixRunSchema(t *testing.T) *Compiled {
	t.Helper()
	integer := TypeRef{Name: QName{Space: XSDNamespace, Local: "integer"}}
	particle := prefixGroup(ParticleSequence, 1, 1, false,
		&ParticleSchema{Kind: ParticleElement, Occurrence: Occurrence{Min: 1, Unbounded: true}, Element: &ElementSchema{Name: QName{Local: "child"}, Type: integer}},
		&ParticleSchema{Kind: ParticleElement, Occurrence: Occurrence{Min: 1, Max: 1}, Element: &ElementSchema{Name: QName{Local: "tail"}, Type: integer}})
	data := SchemaData{Elements: map[QName]*ElementSchema{{Local: "root"}: {Name: QName{Local: "root"}, Type: TypeRef{InlineComplex: &ComplexSchema{Form: ComplexDirect, Particle: particle}}}}}
	c, err := Compile(data, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// Bind gives a witness to every child before the first child its parent's
// content model rejects, whatever follows, and to none from there on, both
// for a complete child list that fails the content model and for a child
// list that is not known to be complete. The parent's content stays invalid
// or incomplete, and skipped children keep the identity scope incomplete.
func TestBindWitnessesChildrenBeforeTheContentFailure(t *testing.T) {
	ctx := context.Background()
	c := prefixRunSchema(t)
	use, _ := c.Root(QName{Local: "root"})
	for _, tc := range []struct {
		name     string
		children []string
		complete bool
		want     []SubjectID
	}{
		{"trailing bogus", []string{"child", "child", "tail", "bogus"}, true, []SubjectID{2, 3, 4}},
		{"error in the middle", []string{"child", "bogus", "child", "tail"}, true, []SubjectID{2}},
		{"missing tail", []string{"child", "child"}, true, []SubjectID{2, 3}},
		{"incomplete enumeration", []string{"child", "child"}, false, []SubjectID{2, 3}},
		{"incomplete enumeration with an error", []string{"child", "bogus", "child"}, false, []SubjectID{2}},
		{"valid", []string{"child", "tail"}, true, []SubjectID{2, 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := NewRun(c, &testMeter{})
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			subject := Subject{ID: 1, Name: QName{Local: "root"}, Path: "/root", TextKnown: true, AttributesComplete: true, ChildrenComplete: tc.complete}
			for i, name := range tc.children {
				subject.Children = append(subject.Children, Child{ID: SubjectID(i + 2), Name: QName{Local: name}, Path: "/root/" + name})
			}
			b, err := r.Bind(ctx, use, subject)
			if err != nil {
				t.Fatal(err)
			}
			var got []SubjectID
			for _, w := range r.Witnesses(b) {
				if w.Mode != WitnessDeclared || w.Use.Declaration() != (QName{Local: tc.children[w.Index]}) {
					t.Fatalf("witness %+v", w)
				}
				got = append(got, w.Child)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("witnesses for %v, want %v", got, tc.want)
			}
			content := r.Results(b).Content
			valid := tc.name == "valid"
			// Skipped children keep identity absence unproved.
			if !valid && len(tc.want) < len(tc.children) && !r.context.IdentityIncomplete {
				t.Fatal("identity complete although children were skipped")
			}
			if content.Valid != (valid || !tc.complete) || content.Complete != tc.complete {
				t.Fatalf("content valid=%v complete=%v", content.Valid, content.Complete)
			}
		})
	}
}

// An unbounded particle's occurrence count saturates at its minimum, so long
// child lists of valid nested repetitions stay unambiguous and bind entirely.
func TestPrefixMatchesBindLongRepetitions(t *testing.T) {
	a := func(min, max uint64, unbounded bool) *ParticleSchema { return prefixLeaf("a", min, max, unbounded) }
	b := prefixLeaf("b", 0, 1, false)
	for _, tc := range []struct {
		name     string
		particle *ParticleSchema
	}{
		{"(a+)+", prefixGroup(ParticleSequence, 1, 0, true, a(1, 0, true))},
		{"(a*|b)*", prefixGroup(ParticleChoice, 0, 0, true, a(0, 0, true), prefixLeaf("b", 1, 1, false))},
		{"(a*, b?)*", prefixGroup(ParticleSequence, 0, 0, true, a(0, 0, true), b)},
	} {
		names := make([]string, 1000)
		for i := range names {
			names[i] = "a"
		}
		got, matcher := prefixBind(tc.particle, prefixChildren(names...))
		if len(got) != len(names) || matcher.ambiguous {
			t.Errorf("%s: bound %d of %d children (ambiguous=%v)", tc.name, len(got), len(names), matcher.ambiguous)
		}
	}
}

// A content model that shares one particle under many alternatives cannot
// make a child's search exponential: the search stops at prefixVisitLimit
// configurations and binding stops as ambiguous.
func TestPrefixMatchesBoundSharedParticles(t *testing.T) {
	shared := prefixLeaf("a", 1, 1, false)
	for range 18 {
		shared = prefixGroup(ParticleChoice, 0, 1, false, shared, shared)
	}
	r, err := NewRun(testRunSchema(t), &testMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	m := r.meter.(*testMeter)
	r.current = context.Background()
	var got []ElementMatch
	func() {
		defer recoverRun(&err)
		got = r.context.prefixMatches(prefixGroup(ParticleSequence, 1, 1, false, shared, prefixLeaf("z", 1, 1, false)), prefixChildren("b"))
	}()
	if err != nil || len(got) != 0 {
		t.Fatalf("err=%v bound=%d", err, len(got))
	}
	if m.steps > 2_000_000 {
		t.Fatalf("one child's search took %d steps", m.steps)
	}
	t.Logf("%d steps", m.steps)
}

// prefixScratchPeak binds n children of an unbounded ten-member choice and
// returns the peak buffer charge less what the binding keeps.
func prefixScratchPeak(t *testing.T, n int) int64 {
	t.Helper()
	var members []*ParticleSchema
	for i := range 10 {
		members = append(members, prefixLeaf(string(rune('a'+i)), 1, 1, false))
	}
	p := prefixGroup(ParticleChoice, 1, 0, true, members...)
	names := make([]string, n)
	for i := range names {
		names[i] = string(rune('a' + i%10))
	}
	m := &boundaryMeter{}
	r, err := NewRun(testRunSchema(t), m)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	r.current = context.Background()
	var got []ElementMatch
	func() {
		defer recoverRun(&err)
		got = r.context.prefixMatches(p, prefixChildren(names...))
	}()
	if err != nil || len(got) != n {
		t.Fatalf("err=%v bound=%d", err, len(got))
	}
	// Preparing the content model leaves fixed scratch until the operation
	// ends; the configurations of the last child are released.
	if r.scratchRetained > 4<<10 {
		t.Fatalf("binding left %d bytes of scratch", r.scratchRetained)
	}
	return m.peak - m.buffer
}

// Binding keeps only the surviving configurations between children, so its
// scratch does not grow with the number of children.
func TestPrefixMatchesScratchIsPerChild(t *testing.T) {
	small, large := prefixScratchPeak(t, 1000), prefixScratchPeak(t, 8000)
	if small > 64<<10 || large > small+(4<<10) {
		t.Fatalf("scratch peak %d bytes for 1000 children, %d for 8000", small, large)
	}
	t.Logf("scratch peak %d bytes for 1000 children, %d for 8000", small, large)
}

// The parent's content failure is recorded before prefix binding starts: a
// work limit that stops binding still leaves the proved failure.
func TestBindRecordsContentFailureBeforePrefixBinding(t *testing.T) {
	ctx := context.Background()
	c := prefixRunSchema(t)
	use, _ := c.Root(QName{Local: "root"})
	subject := Subject{ID: 1, Name: QName{Local: "root"}, Path: "/root", TextKnown: true, AttributesComplete: true, ChildrenComplete: true}
	var nodes []*Node
	for i := range 200 {
		subject.Children = append(subject.Children, Child{ID: SubjectID(i + 2), Name: QName{Local: "child"}, Path: "/root/child"})
		nodes = append(nodes, &Node{Name: QName{Local: "child"}})
	}
	subject.Children = append(subject.Children, Child{ID: 300, Name: QName{Local: "bogus"}, Path: "/root/bogus"})
	nodes = append(nodes, &Node{Name: QName{Local: "bogus"}})
	bind := func(limit int64) (bool, int64) {
		m := &testMeter{limit: limit}
		r, err := NewRun(c, m)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		b, err := r.Bind(ctx, use, subject)
		if limit == 0 && err != nil {
			t.Fatal(err)
		}
		return !r.Results(b).Content.Valid, m.steps
	}
	failed, total := bind(0)
	if !failed {
		t.Fatal("content failure not proved")
	}
	// The steps of prefix binding alone.
	m := &testMeter{}
	r, err := NewRun(c, m)
	if err != nil {
		t.Fatal(err)
	}
	r.current = ctx
	root, _ := c.Root(QName{Local: "root"})
	r.context.prefixMatches(root.original.Type.InlineComplex.Particle, nodes)
	prefix := m.steps
	_ = r.Close()
	// The smallest work limit that proves the failure.
	lo, hi := int64(1), total
	for lo < hi {
		mid := lo + (hi-lo)/2
		if failed, _ := bind(mid); failed {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	if lo+prefix > total {
		t.Fatalf("failure proved only after %d of %d steps; prefix binding takes %d", lo, total, prefix)
	}
}
