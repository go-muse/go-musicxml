package validation

import (
	"context"
	"strings"
	"testing"
)

func TestContextRoleDynamicArgumentDoesNotAllocate(t *testing.T) {
	s, bindings, n := contextRoleReuseFixture(t)
	defer s.cleanup()
	owner := bindings.Roles["item-role"][0].OwnerType.Name.Local
	b, err := compileContextBindingSpecs(s.schema.active, []contextBindingSpec{
		{Role: layoutRole("a"), Owner: owner, Name: "item", Type: "xs:string", Count: 1},
		{Role: layoutRole("b"), Owner: owner, Name: "item", Type: "xs:string", Count: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	r := &contextReader{ctx: context.Background(), s: s, b: b}
	// Warm the admitted cache before measuring only dynamic role arguments.
	if !r.matches(n, layoutRole("a")) || r.err != nil {
		t.Fatalf("warm role lookup: %v", r.err)
	}
	// Runtime suffixes exercise the same short concatenation as layout callers;
	// a compile-time constant role would hide an escaping-parameter regression.
	names := strings.Split("a,b,z", ",")
	allocations := testing.AllocsPerRun(1000, func() {
		for i, name := range names {
			if got := r.matches(n, layoutRole(name)); got != (i < 2) || r.err != nil {
				t.Fatalf("dynamic role %q: result=%v err=%v", name, got, r.err)
			}
		}
	})
	if allocations != 0 {
		t.Fatalf("warm dynamic role lookups allocated %g objects; want zero", allocations)
	}
}
