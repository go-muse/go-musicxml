package validation

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"
)

func resourceTestNode(id, parent NodeID, name string) *factNode {
	d := DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "owner", Occurrence: name}
	return &factNode{header: NodeHeader{ID: id, Parent: parent, Name: QName{Local: name}, Declaration: Evidence[DeclarationID]{State: EvidenceKnown, Value: d}, Location: Location{DocumentID: "d", Target: TargetSource}}, closed: true, end: NodeEnd{ID: id, ChildrenComplete: Evidence[bool]{State: EvidenceKnown, Value: true}, AttributesComplete: Evidence[bool]{State: EvidenceKnown, Value: true}, Order: Evidence[OrderKind]{State: EvidenceKnown, Value: OrderSource}}}
}
func resourceTestReader(t *testing.T, nodes ...*factNode) (*contextReader, *resourceLinkIndex) {
	t.Helper()
	s := rationalSession(t, Budgets{})
	s.documentID = "d"
	s.profile = Profile{ID: "test", Target: TargetSource}
	s.catalog = &CompiledCatalog{orders: map[ProfileID][]RuleKey{}}
	s.nodes = map[NodeID]*factNode{}
	b := &contextBindings{assembly: AssemblyMusicXML, Elements: map[contextRole]map[DeclarationID]struct{}{}}
	for _, name := range []string{"link", "bookmark", "score-part"} {
		b.Elements[resourceRole(name)] = map[DeclarationID]struct{}{{Assembly: AssemblyMusicXML, OwnerOccurrence: "owner", Occurrence: name}: {}}
	}
	for _, n := range nodes {
		s.nodes[n.header.ID] = n
		s.sequence = append(s.sequence, n.header.ID)
		if parent := s.nodes[n.header.Parent]; parent != nil {
			parent.children = append(parent.children, n)
		}
	}
	r := &contextReader{ctx: context.Background(), s: s, b: b}
	idx := newResourceLinkIndex(r)
	if r.err != nil {
		t.Fatal(r.err)
	}
	t.Cleanup(func() { idx.release(s); s.cleanup() })
	return r, idx
}
func resourceTestText(s string) Evidence[Atomic] {
	return Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: s}}
}
func resourceTestPosition(s string) Evidence[Atomic] {
	v, _ := new(big.Int).SetString(s, 10)
	n, _ := NewInteger(v)
	return Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger, Integer: n}}
}
func TestResourceAttachmentSiblingAndOrdinalMatrix(t *testing.T) {
	cases := []struct {
		name, element, position string
		state                   EvidenceState
		target                  NodeID
	}{
		{"absent-element-marker", "", "99999999999999999999999999999999", EvidenceKnown, 3},
		{"explicit-skip-marker", "beam", "2", EvidenceKnown, 6},
		{"missing-position", "beam", "", EvidenceUnknown, 0},
		{"large-position", "beam", strings.Repeat("9", 512), EvidenceAbsent, 0},
		{"missing-descendant", "rest", "1", EvidenceAbsent, 0},
		{"selector-colon", "m:beam", "1", EvidenceUnsupported, 0},
		{"nmtoken-is-not-name", "123", "1", EvidenceUnsupported, 0},
		{"anchor-not-descendant", "note", "1", EvidenceAbsent, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := resourceTestNode(1, 0, "measure")
			link := resourceTestNode(2, 1, "link")
			marker := resourceTestNode(3, 1, "bookmark")
			note := resourceTestNode(4, 1, "note")
			beam1 := resourceTestNode(5, 4, "beam")
			beam2 := resourceTestNode(6, 4, "beam")
			r, index := resourceTestReader(t, root, link, marker, note, beam1, beam2)
			e := Evidence[Atomic]{State: EvidenceAbsent}
			p := e
			if tc.element != "" {
				e = resourceTestText(tc.element)
			}
			if tc.position != "" {
				p = resourceTestPosition(tc.position)
			}
			got := index.attachment(r, link, e, p)
			if got.State != tc.state || got.Value.Node != tc.target {
				t.Fatalf("got %+v, want %s node %d", got, tc.state, tc.target)
			}
		})
	}
}
func TestResourceAttachmentGlobalDescendantOrdinal(t *testing.T) {
	for _, position := range []string{"1", "2", "3"} {
		root := resourceTestNode(1, 0, "measure")
		link := resourceTestNode(2, 1, "link")
		anchor := resourceTestNode(3, 1, "direction")
		a := resourceTestNode(4, 3, "direction-type")
		word1 := resourceTestNode(5, 4, "words")
		b := resourceTestNode(6, 3, "direction-type")
		word2 := resourceTestNode(7, 6, "words")
		r, index := resourceTestReader(t, root, link, anchor, a, word1, b, word2)
		got := index.attachment(r, link, resourceTestText("words"), resourceTestPosition(position))
		want := EvidenceKnown
		wantNode := NodeID(5)
		if position == "2" {
			wantNode = 7
		}
		if position == "3" {
			wantNode = 0
			want = EvidenceAbsent
		}
		if got.State != want || got.Value.Node != wantNode {
			t.Fatalf("position %s got %+v, want node %d", position, got, wantNode)
		}
	}
}
func TestResourceAttachmentIncompleteAndInvalidAxes(t *testing.T) {
	root := resourceTestNode(1, 0, "measure")
	link := resourceTestNode(2, 1, "link")
	anchor := resourceTestNode(3, 1, "note")
	beam := resourceTestNode(4, 3, "beam")
	cause := &DecisionRef{Rule: RuleKey{"missing-subtree", 1}, Subject: NodeRef{"d", 3}}
	anchor.end.ChildrenComplete = Evidence[bool]{State: EvidenceUnknown, Reason: "omitted-child", Cause: cause}
	r, index := resourceTestReader(t, root, link, anchor, beam)
	got := index.attachment(r, link, resourceTestText("beam"), resourceTestPosition("1"))
	if got.State != EvidenceUnknown || got.Cause != cause {
		t.Fatalf("enumeration cause lost: %+v", got)
	}
	invalid := Evidence[Atomic]{State: EvidenceInvalid, Reason: "invalid-position", Cause: cause}
	// The position is explicitly ignored when element is absent, including an
	// independently invalid position. XSD keeps its independent failure.
	got = index.attachment(r, link, Evidence[Atomic]{State: EvidenceAbsent}, invalid)
	if got.State != EvidenceKnown || got.Value.Node != 3 {
		t.Fatalf("ignored position leaked into attachment: %+v", got)
	}
	got = index.attachment(r, link, invalid, resourceTestPosition("1"))
	if got.State != EvidenceInvalid || got.Cause != cause {
		t.Fatalf("invalid selector cause lost: %+v", got)
	}
}
func TestResourceProjectionOwnedClosedBranchesAndRemapping(t *testing.T) {
	s := rationalSession(t, Budgets{})
	s.documentID = "d"
	defer s.cleanup()
	cause := &DecisionRef{Rule: RuleKey{"cause", 1}, Subject: NodeRef{"d", 7}}
	payload := &ResourceContextResult{Owner: NodeRef{"d", 1}, Part: Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{"d", 2}}, Link: &ResourceLinkContext{Kind: ResourceScoreLink, Href: ResourceAttribute{Presence: Evidence[bool]{State: EvidenceKnown, Value: true}, Raw: Evidence[string]{State: EvidenceKnown, Value: "#"}, Value: Evidence[Atomic]{State: EvidenceInvalid, Cause: cause}}, Attachment: Evidence[NodeRef]{State: EvidenceUnknown, Cause: cause}, Fragment: Evidence[string]{State: EvidenceKnown, Value: ""}}}
	input := ContextResult{Provider: RuleKey{"resource", 1}, Subject: NodeRef{"d", 7}, Property: ContextResourceLink, Value: Evidence[Atomic]{State: EvidenceAbsent}, Resource: payload}
	if err := s.publishContextResult(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	cause.Rule.ID = "changed"
	payload.Part.Value.Node = 999
	out := s.contextResults[0].Resource
	if out.Link.Href.Value.Cause.Rule.ID != "cause" || out.Part.Value.Node != 2 {
		t.Fatal("borrowed resource payload")
	}
	clone := cloneResourceContextValue(out)
	remapResourceContextValue(clone, "d", "adopted")
	if clone.Owner.DocumentID != "adopted" || clone.Part.Value.DocumentID != "adopted" || clone.Link.Href.Value.Cause.Subject.DocumentID != "adopted" || clone.Link.Attachment.Cause.Subject.DocumentID != "adopted" || out.Owner.DocumentID != "d" {
		t.Fatalf("incomplete or borrowed remap: %+v", clone)
	}
	bad := *payload
	bad.Group = &ResourceGroupContext{}
	input.Resource = &bad
	if err := s.publishContextResult(context.Background(), input); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("multiple payloads accepted: %v", err)
	}
}
func TestResourceProjectionBudgetAndCancellation(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		limits := Budgets{MaxBufferedBytes: 1}
		if cancel {
			limits = Budgets{}
		}
		s := rationalSession(t, limits)
		s.documentID = "d"
		ctx, stop := context.WithCancel(context.Background())
		if cancel {
			stop()
		}
		err := s.publishContextResult(ctx, ContextResult{Provider: RuleKey{"resource", 1}, Subject: NodeRef{"d", 1}, Property: ContextResourceGroup, Value: Evidence[Atomic]{State: EvidenceAbsent}, Resource: &ResourceContextResult{Group: &ResourceGroupContext{Value: Evidence[string]{State: EvidenceKnown, Value: "group"}}}})
		if cancel && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if !cancel && !errors.Is(err, ErrResourceLimit) {
			t.Fatal(err)
		}
		if len(s.contextResults) != 0 {
			t.Fatal("partial payload escaped")
		}
		s.cleanup()
		used, _ := s.account.Usage(BudgetBufferedBytes)
		if used != 0 {
			t.Fatalf("buffer leaked %d", used)
		}
		stop()
	}
}

func TestResourceAttachmentSparseQueryWorkDoesNotRescanDescendants(t *testing.T) {
	measure := func(size int) int64 {
		root := resourceTestNode(1, 0, "measure")
		nodes := []*factNode{root}
		links := make([]*factNode, size)
		for n := 0; n < size; n++ {
			links[n] = resourceTestNode(NodeID(n+2), 1, "link")
			nodes = append(nodes, links[n])
		}
		anchorID := NodeID(size + 2)
		nodes = append(nodes, resourceTestNode(anchorID, 1, "note"))
		for n := 0; n < size; n++ {
			nodes = append(nodes, resourceTestNode(NodeID(size+3+n), anchorID, "beam"))
		}
		r, index := resourceTestReader(t, nodes...)
		for n, link := range links {
			value, _ := NewInteger(big.NewInt(int64(n + 1)))
			got := index.attachment(r, link, resourceTestText("beam"), Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger, Integer: value}})
			if got.State != EvidenceKnown || got.Value.Node != NodeID(size+3+n) {
				t.Fatalf("query %d: %+v", n, got)
			}
		}
		if r.err != nil {
			t.Fatal(r.err)
		}
		work, _ := r.s.account.Usage(BudgetRuleSteps)
		return work
	}
	small, large := measure(512), measure(1024)
	if large > small*3 {
		t.Fatalf("doubling produced unbounded repeated scans: %d -> %d steps", small, large)
	}
}

func TestResourceAttachmentForeignAndWrongAssemblyNamesDoNotBind(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		root := resourceTestNode(1, 0, "measure")
		link := resourceTestNode(2, 1, "link")
		anchor := resourceTestNode(3, 1, "note")
		beam := resourceTestNode(4, 3, "beam")
		if foreign {
			beam.header.Name.Space = "urn:foreign"
		} else {
			beam.header.Declaration.Value.Assembly = AssemblySounds
		}
		r, index := resourceTestReader(t, root, link, anchor, beam)
		got := index.attachment(r, link, resourceTestText("beam"), resourceTestPosition("1"))
		if got.State != EvidenceAbsent {
			t.Fatalf("foreign=%v homonym leaked: %+v", foreign, got)
		}
	}
}

func TestResourceHintsOwnedPairsAndRemappedLocations(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	cause := &DecisionRef{Rule: RuleKey{"hint", 1}, Subject: NodeRef{"a", 3}}
	p := &ResourceContextResult{Owner: NodeRef{"a", 3}, Part: Evidence[NodeRef]{State: EvidenceAbsent}, SchemaHints: &ResourceSchemaHintsContext{Raw: Evidence[string]{State: EvidenceKnown, Value: "urn:a a.xsd"}, Pairs: []SchemaHintPair{{Namespace: Evidence[string]{State: EvidenceKnown, Value: "urn:a"}, Location: Evidence[string]{State: EvidenceInvalid, Reason: "invalid-hint", Cause: cause}, Origin: Location{DocumentID: "a", Path: "/n/@hint", Position: &SourcePosition{Line: 4}}}}, Trailing: Evidence[string]{State: EvidenceAbsent}, Complete: Evidence[bool]{State: EvidenceInvalid, Reason: "invalid-hint", Cause: cause}, NoNamespace: resourceAbsentAttribute()}}
	owned, err := cloneResourceContext(context.Background(), s, p)
	if err != nil {
		t.Fatal(err)
	}
	remapResourceContextValue(owned, "a", "b")
	p.SchemaHints.Pairs[0].Origin.Position.Line = 999
	cause.Rule.ID = "changed"
	if owned.SchemaHints.Pairs[0].Origin.DocumentID != "b" || owned.SchemaHints.Pairs[0].Origin.Position.Line != 4 || owned.SchemaHints.Pairs[0].Location.Cause.Subject.DocumentID != "b" || owned.SchemaHints.Complete.Cause.Rule.ID != "hint" {
		t.Fatalf("hint ownership/remap failed: %+v", owned)
	}
}

func TestResourceProjectionExactAndOneUnderAdmission(t *testing.T) {
	for _, payload := range []*ResourceContextResult{
		{Link: &ResourceLinkContext{Kind: ResourceScoreLink, Href: ResourceAttribute{Raw: Evidence[string]{State: EvidenceKnown, Value: strings.Repeat("uri", 50)}}}},
		{SchemaHints: &ResourceSchemaHintsContext{Raw: Evidence[string]{State: EvidenceKnown, Value: "urn:a a.xsd"}, Pairs: []SchemaHintPair{{Namespace: Evidence[string]{State: EvidenceKnown, Value: "urn:a"}, Location: Evidence[string]{State: EvidenceKnown, Value: "a.xsd"}, Origin: Location{DocumentID: "d", Position: &SourcePosition{Line: 1}}}}}},
	} {
		probe := rationalSession(t, Budgets{})
		size, err := resourceContextSize(context.Background(), probe, payload)
		probe.cleanup()
		if err != nil {
			t.Fatal(err)
		}
		for _, delta := range []int64{0, -1} {
			s := rationalSession(t, Budgets{MaxBufferedBytes: size + delta})
			out, err := cloneResourceContext(context.Background(), s, payload)
			if delta == 0 && (err != nil || out == nil) {
				t.Fatalf("exact admission failed: size=%d err=%v", size, err)
			}
			if delta < 0 && (!errors.Is(err, ErrResourceLimit) || out != nil) {
				t.Fatalf("one-under not transactional: err=%v out=%+v", err, out)
			}
			s.cleanup()
			used, _ := s.account.Usage(BudgetBufferedBytes)
			if used != 0 {
				t.Fatalf("copy leaked %d", used)
			}
		}
	}
}
