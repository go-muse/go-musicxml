package validation

import (
	"context"
	"reflect"
	"sort"
	"testing"
)

func TestLocalDispatchConstantRolesAreUnique(t *testing.T) {
	seen := map[contextRole]bool{}
	for i, spec := range localConstantRules {
		if spec.Layout {
			continue
		}
		role := localRuleBindings[i].attribute
		if spec.Element != "" {
			role = localRuleBindings[i].element
		}
		if seen[role] {
			t.Fatalf("duplicate constant dispatch role %s", role)
		}
		seen[role] = true
		if got, ok := localDispatchConstants[role]; !ok || got != i {
			t.Fatalf("constant %d lost at %s", i, role)
		}
	}
	if len(seen) != len(localDispatchConstants) {
		t.Fatal("unowned dispatch entry")
	}
}

// Compare the optimized provider with the former exhaustive family dispatch on
// every registered role, including unavailable declaration and attribute axes.
func TestLocalDispatchMatchesExhaustiveRoleReference(t *testing.T) {
	roles := map[contextRole]bool{}
	for role := range localDispatchFamilies {
		roles[role] = true
	}
	for role := range localDispatchConstants {
		roles[role] = true
	}
	for role := range roles {
		for _, attribute := range []bool{false, true} {
			for _, state := range []EvidenceState{EvidenceKnown, EvidenceUnknown, EvidenceInvalid} {
				run := func(fast bool) []ContextResult {
					s, n, r := localEvidenceFixture(t, EvidenceKnown, role)
					defer s.cleanup()
					n.end.Scalar.Value = localKnownText("1")
					s.sequence = []NodeID{n.header.ID}
					if attribute {
						r.b.Roles = map[contextRole][]SchemaUse{}
						r.b.Attributes[role] = map[ComponentID]AttributeSelector{n.header.Type.Value: localAttr("flag")}
						localSetAttr(n, "flag", localString("no"))
						n.attributes[0].Scalar.Value.State = state
						if state != EvidenceKnown {
							n.attributes[0].Scalar.Value.Reason = "attribute-unavailable"
						}
					} else {
						n.header.Declaration.State = state
						if state != EvidenceKnown {
							n.header.Declaration.Reason = "declaration-unavailable"
						}
					}
					key := RuleKey{"local-test", 1}
					if fast {
						_, _, err := evaluateContextLocal(context.Background(), s, n, key, &contextPlan{Bindings: r.b}, true)
						if err != nil {
							t.Fatalf("role %s fast: %v", role, err)
						}
					} else {
						for i, spec := range localConstantRules {
							if !spec.Layout {
								if err := localPublishConstant(r.ctx, s, key, n, r, i); err != nil {
									t.Fatal(err)
								}
							}
						}
						for _, fn := range []func() error{func() error { return localPrintNode(r.ctx, s, key, n, r) }, func() error { return localGlyphFacts(r, key, n) }, func() error { return localStructuralNode(r.ctx, s, key, n, r) }, func() error { return localAdvisoryNode(r.ctx, s, key, n, r) }} {
							if err := fn(); err != nil {
								t.Fatalf("role %s reference: %v", role, err)
							}
						}
					}
					rows := append([]ContextResult(nil), s.contextResults...)
					sort.Slice(rows, func(i, j int) bool { return rows[i].Property < rows[j].Property })
					return rows
				}
				if a, b := run(false), run(true); !reflect.DeepEqual(a, b) {
					t.Fatalf("role=%s attribute=%t state=%s\nreference=%+v\nfast=%+v", role, attribute, state, a, b)
				}
			}
		}
	}
}
