package xsd

import "testing"

func TestBuiltinTypeResolutionAvoidsTemporaryAllocation(t *testing.T) {
	c := &Context{}
	for _, name := range []string{"string", "normalizedString", "token", "integer", "decimal", "anyType", "not-a-builtin"} {
		reference := &TypeRef{Name: QName{Space: XSDNamespace, Local: name}}
		want := BuiltinSemantics(name) != nil
		if allocations := testing.AllocsPerRun(10, func() {
			simple, complex, builtin, found := c.ResolveType(reference)
			if simple != nil || complex != nil || builtin != name || found != want {
				t.Fatalf("incorrect built-in resolution: %q, %v", builtin, found)
			}
		}); allocations != 0 {
			t.Errorf("%s resolution allocated %v; want 0", name, allocations)
		}
	}
}

func TestBuiltinSemanticsRemainsCallerOwned(t *testing.T) {
	first := BuiltinSemantics("normalizedString")
	first.Whitespace, first.StringValue, first.Builtin = WhitespaceCollapse, false, "changed"
	second := BuiltinSemantics("normalizedString")
	if second.Whitespace != WhitespaceReplace || !second.StringValue || second.Builtin != "normalizedString" {
		t.Fatal("built-in semantics leaked a caller mutation")
	}
}
