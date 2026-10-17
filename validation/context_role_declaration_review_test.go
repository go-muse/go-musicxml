package validation

import (
	"context"
	"reflect"
	"testing"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

func TestContextRoleSessionReuseCanonicalDeclarationsAcrossNodes(t *testing.T) {
	for _, count := range []int{1, 32} {
		s, b, first := contextRoleRepeatedFixture(t, count)
		// The root and every repeated built-in-typed child have completed real
		// binding. Their declaration pointers must be owned by this schema's
		// canonical pool, never caller evidence or a QName/type approximation.
		seenBindings := make(map[xsd.Binding]bool)
		for _, id := range s.sequence {
			n := s.nodes[id]
			bound := s.schema.nodes[id]
			if !bound.available || bound.binding == (xsd.Binding{}) || seenBindings[bound.binding] {
				t.Fatalf("node %d lacks an independent available run binding", id)
			}
			seenBindings[bound.binding] = true
			canonical := false
			for key, declaration := range s.facts.declarations {
				if declaration == n.header.Declaration {
					canonical = key.BelongsTo(s.schema.active.kernel)
				}
			}
			if !canonical || n.header.Declaration.State != EvidenceKnown || n.header.Declaration.Reason != "" || n.header.Declaration.Cause != nil {
				t.Fatalf("node %d declaration is not schema-owned canonical evidence", id)
			}
			if n != s.root && n.header.Declaration != first.header.Declaration {
				t.Fatal("same exact declaration was not canonical across node bindings")
			}
		}
		if s.root.header.Declaration == first.header.Declaration {
			t.Fatal("different exact root and child declarations were merged")
		}

		// Even identical immutable table contents/digests retain table identity.
		other := *b
		before := s.retained
		for _, table := range []*contextBindings{b, &other, b, &other} {
			for _, id := range s.sequence {
				n := s.nodes[id]
				r := &contextReader{ctx: context.Background(), s: s, b: table}
				if r.matches(n, "item-role") != (n != s.root) || r.matches(n, "item-alias") != (n != s.root) || r.matches(n, "absent-role") || r.err != nil {
					t.Fatalf("node %d exact membership changed: %v", id, r.err)
				}
			}
		}
		if got := len(s.contextElementRoles); got != 4 {
			t.Errorf("%d nodes produced %d cache entries; want two declarations x two tables", len(s.nodes), got)
		}
		if got := s.retained - before; got != 512+3*256 {
			t.Errorf("%d nodes retained %d role-cache bytes; want 1280", len(s.nodes), got)
		}
		s.cleanup()
		if s.contextElementRoles != nil || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("canonical role-cache ownership survived cleanup")
		}
	}
}

func TestContextRoleSessionReuseDeclarationEvidenceIsolation(t *testing.T) {
	s, b, n := contextRoleRepeatedFixture(t, 2)
	defer s.cleanup()
	sibling := s.nodes[3]
	canonical := n.header.Declaration
	var key xsd.CacheIdentity
	for candidate, declaration := range s.facts.declarations {
		if declaration == canonical {
			key = candidate
		}
	}
	if !key.BelongsTo(s.schema.active.kernel) {
		t.Fatal("fixture declaration has no canonical schema identity")
	}
	r := &contextReader{ctx: context.Background(), s: s, b: b}
	if !r.matches(n, "item-role") || r.err != nil {
		t.Fatal(r.err)
	}
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceInvalid, EvidenceAbsent, EvidenceUnsupported, EvidenceKnown} {
		v := *canonical
		v.State, v.Reason = state, "node-specific-evidence"
		v.Cause = &DecisionRef{Rule: RuleKey{ID: "node-specific-cause", Revision: 1}, Subject: NodeRef{DocumentID: "source", Node: n.header.ID}}
		owned, err := s.retainSchemaDeclaration(context.Background(), key, v)
		if err != nil {
			t.Fatal(err)
		}
		if owned == canonical || owned.Cause == v.Cause || !reflect.DeepEqual(*owned, v) {
			t.Fatal("exceptional declaration evidence was canonicalized or borrowed")
		}
		n.header.Declaration = owned
		entries := len(s.contextElementRoles)
		if got := r.matches(n, "item-role"); got != (state == EvidenceKnown) || r.err != nil {
			t.Fatalf("state %s exact membership changed: result=%v err=%v", state, got, r.err)
		}
		if state != EvidenceKnown && len(s.contextElementRoles) != entries {
			t.Fatal("unavailable declaration populated the known-role cache")
		}
		if !reflect.DeepEqual(*n.header.Declaration, v) || sibling.header.Declaration != canonical || !r.matches(sibling, "item-role") || r.err != nil {
			t.Fatal("role reuse changed evidence/cause or sibling membership")
		}
	}
}

func TestContextRoleSessionReuseSharedSchemaIsolation(t *testing.T) {
	first, b, node := contextRoleRepeatedFixture(t, 2)
	defer first.cleanup()
	engine, err := NewEngine(first.catalog)
	if err != nil {
		t.Fatal(err)
	}
	second := contextRoleEngineSession(t, engine, 2)
	defer second.cleanup()
	other := second.nodes[node.header.ID]
	if first.schema.active != second.schema.active || node.header.Declaration == other.header.Declaration || *node.header.Declaration != *other.header.Declaration {
		t.Fatal("shared compiled schema did not produce equal, independently owned declarations")
	}
	for _, s := range []*Session{first, second} {
		before := s.retained
		r := &contextReader{ctx: context.Background(), s: s, b: b}
		for _, id := range []NodeID{2, 3} {
			if !r.matches(s.nodes[id], "item-role") || r.err != nil {
				t.Fatal(r.err)
			}
		}
		if s.retained-before != 512 || len(s.contextElementRoles) != 1 {
			t.Fatal("shared schema/table did not retain one independently charged declaration entry")
		}
	}
	before := rationalUsage(t, second, BudgetBufferedBytes)
	first.cleanup()
	if first.contextElementRoles != nil || rationalUsage(t, first, BudgetBufferedBytes) != 0 {
		t.Fatal("first session retained shared schema-role storage after cleanup")
	}
	r := &contextReader{ctx: context.Background(), s: second, b: b}
	if !r.matches(other, "item-role") || !r.matches(other, "item-alias") || r.matches(other, "absent-role") || r.err != nil || rationalUsage(t, second, BudgetBufferedBytes) != before {
		t.Fatal("first session cleanup changed second session roles or accounted storage")
	}
}
