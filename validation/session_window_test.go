package validation

import (
	"context"
	"errors"
	"testing"
)

// A window bound before its ancestors that no ancestor witness reaches when
// the root closes cannot be undone: binding refuses with a configuration
// error instead of reporting what the buffered engine would not.
func TestBindingRefusesAWindowNoWitnessReaches(t *testing.T) {
	s := schemaScratchFixture(t, 2, "integer", Budgets{})
	s.schema.windows = map[NodeID]boundWindow{99: {size: 1}}
	err := s.bindSourceSchema(context.Background())
	var configuration *ConfigurationError
	if !errors.As(err, &configuration) || configuration.Field != "schema window" {
		t.Fatalf("binding with an unreached window: %v", err)
	}
	s2 := schemaScratchFixture(t, 2, "integer", Budgets{})
	if err := s2.bindSourceSchema(context.Background()); err != nil {
		t.Fatalf("binding without windows: %v", err)
	}
}

// The positions computed from a node's preorder index, depth and subtree
// size are those of a top-down traversal that counts every enter and exit.
func TestEulerPositionsFollowTheTraversal(t *testing.T) {
	s := schemaScratchFixture(t, 1, "integer", Budgets{})
	// root(a(b, c(d)), e, f(g(h)))
	node := func(children ...*factNode) *factNode { return &factNode{children: children} }
	h := node()
	tree := node(node(node(), node(node())), node(), node(node(h)))
	type position struct{ enter, exit uint64 }
	want := map[*factNode]position{}
	step := uint64(0)
	var walk func(*factNode)
	walk = func(n *factNode) {
		p := position{enter: step}
		step++
		for _, child := range n.children {
			walk(child)
		}
		p.exit = step
		step++
		want[n] = p
	}
	walk(tree)
	pre := 0
	var check func(n *factNode, depth int)
	check = func(n *factNode, depth int) {
		var scratch schemaBindScratch
		scratch.account = s.account
		offsets, err := s.subtreeOffsets(context.Background(), n, &scratch)
		if err != nil {
			t.Fatal(err)
		}
		scratch.release()
		if got := (position{eulerEnter(pre, depth), eulerExit(pre, depth, offsets[len(offsets)-1])}); got != want[n] {
			t.Fatalf("node at preorder %d depth %d: %+v, traversal %+v", pre, depth, got, want[n])
		}
		pre++
		for _, child := range n.children {
			check(child, depth+1)
		}
	}
	check(tree, 0)
	if pre != 9 {
		t.Fatalf("checked %d nodes", pre)
	}
}
