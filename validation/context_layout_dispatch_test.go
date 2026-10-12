package validation

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"testing"
)

// Keep the pre-dispatch control flow as an independent semantic oracle. Output
// order, evidence causes, origins, applicability, units and provenance all count.
func layoutExhaustiveReference(ctx context.Context, s *Session, key RuleKey, p *contextPlan) error {
	r := &contextReader{ctx: ctx, s: s, b: p.Bindings}
	formatting, err := newLayoutFormatting(r, key)
	if err != nil {
		return err
	}
	for _, id := range s.sequence {
		if !r.step(1) {
			return r.err
		}
		n := s.nodes[id]
		if n == nil {
			continue
		}
		if err := layoutStateNode(ctx, s, key, n, r, p); err != nil {
			return err
		}
		if err := layoutStatic(r, key, n, p); err != nil {
			return err
		}
		if err := formatting.resolve(n); err != nil {
			return err
		}
		for i, rule := range layoutConstantRules {
			base := "constant." + strconv.Itoa(i)
			if rule.Element != "" {
				if !r.matches(n, layoutRole(base+".element")) {
					continue
				}
			} else if !r.attributeOwner(n, layoutRole(base+".attribute")) {
				continue
			}
			var raw Evidence[Atomic]
			if rule.Attribute != "" {
				raw = localRawAttribute(r, n, layoutRole(base+".attribute"))
			} else {
				raw = r.scalar(n)
			}
			if r.err != nil {
				return r.err
			}
			if err := localPublishWithProvenance(ctx, s, key, n, ContextProperty("layout."+rule.Property), raw, localConstantEffective(rule, raw), Evidence[bool]{State: EvidenceKnown, Value: true}, NodeRef{}, ContextUnitNone, localConstantProvenance(rule, raw)); err != nil {
				return err
			}
		}
	}
	if r.err != nil {
		return r.err
	}
	return evaluateLayoutScopes(ctx, s, key, p, formatting.defaults)
}

func layoutDispatchRun(t *testing.T, fast bool, s *Session, p *contextPlan) error {
	t.Helper()
	key := RuleKey{"layout-dispatch-test", 1}
	if !fast {
		return layoutExhaustiveReference(context.Background(), s, key, p)
	}
	_, pending, err := evaluateContextLayout(context.Background(), s, nil, key, p, true)
	if pending {
		t.Fatal("final layout provider stayed pending")
	}
	return err
}

func TestLayoutDispatchMatchesExhaustiveRoleReference(t *testing.T) {
	for _, spec := range contextLayoutBindingSpecs() {
		for _, axis := range []string{"known", "declaration-unknown", "declaration-invalid", "type-unknown", "type-invalid", "value-unknown", "value-invalid", "attributes-incomplete", "children-incomplete"} {
			t.Run(string(spec.Role)+"/"+axis, func(t *testing.T) {
				run := func(fast bool) []ContextResult {
					s, n, r := localEvidenceFixture(t, EvidenceKnown, spec.Role)
					defer s.cleanup()
					n.end.Scalar.Value = stateKnown(localString("1"))
					s.sequence = []NodeID{0, n.header.ID} // An absent sequence entry is harmless.
					if spec.Attribute {
						r.b.Roles = map[contextRole][]SchemaUse{}
						r.b.Attributes[spec.Role] = map[ComponentID]AttributeSelector{n.header.Type.Value: localAttr("flag")}
						localSetAttr(n, "flag", localString("no"))
					}
					switch axis {
					case "declaration-unknown":
						n.header.Declaration = stateUnknown[DeclarationID]("declaration-unavailable")
					case "declaration-invalid":
						n.header.Declaration = Evidence[DeclarationID]{State: EvidenceInvalid, Reason: "declaration-invalid"}
					case "type-unknown":
						n.header.Type = stateUnknown[ComponentID]("type-unavailable")
					case "type-invalid":
						n.header.Type = Evidence[ComponentID]{State: EvidenceInvalid, Reason: "type-invalid"}
					case "value-unknown", "value-invalid":
						state := EvidenceUnknown
						if axis == "value-invalid" {
							state = EvidenceInvalid
						}
						unavailable := Evidence[Atomic]{State: state, Reason: "value-unavailable", Cause: &DecisionRef{Rule: RuleKey{"source-prerequisite", 1}, Subject: NodeRef{"doc", 2}}}
						n.end.Scalar.Value = unavailable
						if spec.Attribute {
							n.attributes[0].Scalar.Value = unavailable
						}
					case "attributes-incomplete":
						n.end.AttributesComplete = stateUnknown[bool]("attributes-incomplete")
					case "children-incomplete":
						n.end.ChildrenComplete = stateUnknown[bool]("children-incomplete")
					}
					if err := layoutDispatchRun(t, fast, s, &contextPlan{Bindings: r.b}); err != nil {
						t.Fatal(err)
					}
					return append([]ContextResult(nil), s.contextResults...)
				}
				if want, got := run(false), run(true); !reflect.DeepEqual(want, got) {
					t.Fatalf("ordered output differs\nreference=%+v\noptimized=%+v", want, got)
				}
			})
		}
	}
}

func TestLayoutDispatchParentDirectiveWithoutChildRole(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceKnown, EvidenceUnknown, EvidenceInvalid} {
		run := func(fast bool) []ContextResult {
			s, n, r := localEvidenceFixture(t, EvidenceKnown, "unrelated")
			defer s.cleanup()
			parent := localNode(1, 0, "wrapper")
			r.b.Roles[layoutRole("direction-type")] = []SchemaUse{{Declaration: parent.header.Declaration.Value}}
			n.header.Declaration.State = state
			n.header.Declaration.Reason = "child-axis-unavailable"
			s.nodes[1] = parent
			s.sequence = []NodeID{n.header.ID}
			if err := layoutDispatchRun(t, fast, s, &contextPlan{Bindings: r.b}); err != nil {
				t.Fatal(err)
			}
			if len(s.contextResults) != 1 || s.contextResults[0].Property != "layout.directive" {
				t.Fatalf("lost parent-only directive: %+v", s.contextResults)
			}
			return append([]ContextResult(nil), s.contextResults...)
		}
		if want, got := run(false), run(true); !reflect.DeepEqual(want, got) {
			t.Fatalf("parent-only output differs, state=%s", state)
		}
	}
}

func layoutDispatchUnrelatedFixture(t *testing.T, count int, budget Budgets) (*Session, *contextPlan) {
	t.Helper()
	s := rationalSession(t, budget)
	s.documentID = "doc"
	s.nodes = make(map[NodeID]*factNode)
	for i := 1; i <= count; i++ {
		n := localNode(NodeID(i), 0, "unrelated")
		s.nodes[n.header.ID] = n
		s.sequence = append(s.sequence, n.header.ID)
	}
	return s, &contextPlan{Bindings: &contextBindings{Elements: map[contextRole]map[DeclarationID]struct{}{}, ElementRoles: map[DeclarationID][]contextRole{}, OwnerTables: map[ComponentID]*contextAttributeTable{}}}
}

func TestLayoutDispatchUnrelatedCostIsLinearAndReduced(t *testing.T) {
	var prior int64
	for _, count := range []int{16, 64, 256} {
		costs := [2]int64{}
		for i, fast := range []bool{false, true} {
			s, p := layoutDispatchUnrelatedFixture(t, count, Budgets{})
			if err := layoutDispatchRun(t, fast, s, p); err != nil {
				t.Fatal(err)
			}
			if len(s.contextResults) != 0 {
				t.Fatal("unrelated owner published layout facts")
			}
			costs[i] = rationalUsage(t, s, BudgetRuleSteps)
			s.cleanup()
		}
		t.Logf("nodes=%d exhaustive=%d dispatch=%d", count, costs[0], costs[1])
		if costs[1]*2 >= costs[0] {
			t.Errorf("unrelated nodes still pay fixed all-role scans: %v", costs)
		}
		if prior != 0 && costs[1] > 4*prior {
			t.Errorf("nonlinear unrelated dispatch: previous=%d current=%d", prior, costs[1])
		}
		prior = costs[1]
	}
}

func TestLayoutDispatchNoMatchPropagatesRefusalAndCancellation(t *testing.T) {
	for _, limit := range []int64{1, 20, 100, 300, 1000} {
		s, p := layoutDispatchUnrelatedFixture(t, 64, Budgets{MaxRuleSteps: limit})
		if err := layoutDispatchRun(t, true, s, p); !errors.Is(err, ErrResourceLimit) {
			t.Errorf("limit %d: refusal lost: %v", limit, err)
		}
		if len(s.contextResults) != 0 {
			t.Error("refusal published facts")
		}
		s.cleanup()
		if got := rationalUsage(t, s, BudgetBufferedBytes); got != 0 {
			t.Errorf("retention after cleanup=%d", got)
		}
	}
	s, p := layoutDispatchUnrelatedFixture(t, 1, Budgets{})
	defer s.cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := evaluateContextLayout(ctx, s, nil, RuleKey{"layout-dispatch-test", 1}, p, true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestLayoutDispatchTablesAreExactAndCollisionFree(t *testing.T) {
	registered := [2]map[contextRole]bool{make(map[contextRole]bool), make(map[contextRole]bool)}
	for _, spec := range contextLayoutBindingSpecs() {
		axis := 0
		if spec.Attribute {
			axis = 1
		}
		if registered[axis][spec.Role] {
			t.Fatalf("duplicate registered role %s on axis %d", spec.Role, axis)
		}
		registered[axis][spec.Role] = true
	}
	for axis, table := range []map[contextRole]layoutFamilyMask{layoutElementDispatch, layoutAttributeDispatch} {
		for role, mask := range table {
			if !registered[axis][role] || mask == 0 {
				t.Errorf("unregistered or empty family role %s on axis %d", role, axis)
			}
			if registered[1-axis][role] {
				t.Errorf("cross-axis role collision %s", role)
			}
		}
	}
	if len(layoutConstantRules) > layoutConstantDispatchCapacity {
		t.Fatal("constant dispatch capacity exceeded")
	}
	seen := uint64(0)
	for axis, table := range layoutConstantDispatch {
		for role, bit := range table {
			if !registered[axis][role] || bit == 0 || bit&(bit-1) != 0 || seen&bit != 0 {
				t.Fatalf("unregistered, duplicate or invalid constant role %s: %x", role, bit)
			}
			seen |= bit
		}
	}
	if seen != uint64(1)<<uint(len(layoutConstantRules))-1 {
		t.Fatalf("missing constant bits: %x", seen)
	}
	for i, rule := range layoutConstantRules {
		role := layoutRole("constant." + strconv.Itoa(i) + ".attribute")
		axis := 1
		if rule.Element != "" {
			role = layoutRole("constant." + strconv.Itoa(i) + ".element")
			axis = 0
		}
		if layoutConstantDispatch[axis][role] != uint64(1)<<uint(i) {
			t.Fatalf("constant %d selects incorrect role %s", i, role)
		}
	}
}

func TestLayoutDispatchCombinedRolesPreserveOrderAndIndependentAxes(t *testing.T) {
	for _, unavailable := range []EvidenceState{EvidenceKnown, EvidenceUnknown, EvidenceInvalid} {
		run := func(fast bool) []ContextResult {
			s, n, r := localEvidenceFixture(t, EvidenceKnown, "unrelated")
			defer s.cleanup()
			n.end.Scalar.Value = stateKnown(localString("no"))
			s.sequence = []NodeID{n.header.ID}
			for _, spec := range contextLayoutBindingSpecs() {
				if spec.Attribute {
					name := string(spec.Role)
					r.b.Attributes[spec.Role] = map[ComponentID]AttributeSelector{n.header.Type.Value: localAttr(name)}
					localSetAttr(n, name, localString("no"))
					n.attributes[len(n.attributes)-1].Scalar.Value.State = unavailable
					if unavailable != EvidenceKnown {
						n.attributes[len(n.attributes)-1].Scalar.Value.Reason = "combined-attribute-unavailable"
					}
				} else {
					r.b.Roles[spec.Role] = []SchemaUse{{Declaration: n.header.Declaration.Value}}
				}
			}
			if err := layoutDispatchRun(t, fast, s, &contextPlan{Bindings: r.b}); err != nil {
				t.Fatal(err)
			}
			return append([]ContextResult(nil), s.contextResults...)
		}
		if want, got := run(false), run(true); !reflect.DeepEqual(want, got) {
			t.Fatalf("combined ordered output differs for %s\nreference=%+v\noptimized=%+v", unavailable, want, got)
		}
	}
	elements := []contextRole{layoutStateRole("note"), layoutRole("staff-layout"), layoutRole("constant.10.element"), layoutRole("constant.2.element")}
	attributes := []contextRole{layoutRole("@font-family"), layoutRole("@default-x"), layoutRole("constant.0.attribute")}
	want := layoutDispatchRoles(elements, attributes)
	duplicatedElements := append(append([]contextRole(nil), elements...), elements...)
	duplicatedAttributes := append(append([]contextRole(nil), attributes...), attributes...)
	beforeElements := append([]contextRole(nil), duplicatedElements...)
	beforeAttributes := append([]contextRole(nil), duplicatedAttributes...)
	if got := layoutDispatchRoles(duplicatedElements, duplicatedAttributes); got != want {
		t.Fatalf("duplicate roles repeat output selection: got=%+v want=%+v", got, want)
	}
	if !reflect.DeepEqual(beforeElements, duplicatedElements) || !reflect.DeepEqual(beforeAttributes, duplicatedAttributes) {
		t.Fatal("borrowed role slices mutated")
	}
	if got := layoutDispatchRoles(attributes, elements); got != (layoutNodeDispatch{}) {
		t.Fatalf("roles selected on wrong axis: %+v", got)
	}
	if got := layoutDispatchRoles(nil, nil); got != (layoutNodeDispatch{}) {
		t.Fatalf("nil roles selected: %+v", got)
	}
}

func TestLayoutDispatchConstantCapacityFailsClosed(t *testing.T) {
	original := layoutConstantRules
	defer func() { layoutConstantRules = original }()
	layoutConstantRules = make([]localConstantRule, layoutConstantDispatchCapacity+1)
	s, p := layoutDispatchUnrelatedFixture(t, 1, Budgets{})
	defer s.cleanup()
	if err := layoutDispatchRun(t, true, s, p); err == nil {
		t.Fatal("oversize constant table silently truncated")
	}
	if len(s.contextResults) != 0 || rationalUsage(t, s, BudgetBufferedBytes) != 0 {
		t.Fatal("capacity refusal performed partial evaluation")
	}
}

func TestLayoutDispatchExactAndOneUnderWorkLimits(t *testing.T) {
	for _, selected := range []bool{false, true} {
		fixture := func(budget Budgets) (*Session, *contextPlan) {
			s, p := layoutDispatchUnrelatedFixture(t, 1, budget)
			if selected {
				n := s.nodes[1]
				n.header.Type = stateKnown(contextNamedType("dispatch-owner"))
				p.Bindings.Attributes = map[contextRole]map[ComponentID]AttributeSelector{layoutRole("constant.0.attribute"): {n.header.Type.Value: localAttr("implicit")}}
				// Exercise the compatibility index too; a real compiled plan provides an
				// immutable owner table without retaining this session-local fallback.
				p.Bindings.OwnerTables = nil
			}
			return s, p
		}
		s, p := fixture(Budgets{})
		if err := layoutDispatchRun(t, true, s, p); err != nil {
			t.Fatal(err)
		}
		exact := rationalUsage(t, s, BudgetRuleSteps)
		want := append([]ContextResult(nil), s.contextResults...)
		if selected && len(want) != 1 {
			t.Fatal("constant owner did not publish")
		}
		s.cleanup()
		for _, limit := range []int64{exact - 1, exact} {
			s, p := fixture(Budgets{MaxRuleSteps: limit})
			err := layoutDispatchRun(t, true, s, p)
			if limit == exact {
				if err != nil || !reflect.DeepEqual(want, s.contextResults) {
					t.Fatalf("selected=%t exact=%d: err=%v output=%+v", selected, exact, err, s.contextResults)
				}
			} else if !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("selected=%t one-under=%d refusal lost: %v", selected, limit, err)
			}
			s.cleanup()
			if got := rationalUsage(t, s, BudgetBufferedBytes); got != 0 {
				t.Fatalf("selected=%t limit=%d leaked bytes=%d", selected, limit, got)
			}
		}
	}
}

func TestLayoutDispatchParentOnlyLookupRefusal(t *testing.T) {
	s, p := layoutDispatchUnrelatedFixture(t, 1, Budgets{MaxRuleSteps: 9})
	defer s.cleanup()
	n := s.nodes[1]
	n.header.Parent = 2
	n.header.Declaration = stateUnknown[DeclarationID]("child-unavailable")
	parent := localNode(2, 0, "parent")
	s.nodes[2] = parent
	// The first nine charges reach the parent-only exact declaration lookup;
	// no element or attribute role of the child can select a publishing family.
	if err := layoutDispatchRun(t, true, s, p); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("parent-only negative lookup swallowed refusal: %v", err)
	}
	if got := rationalUsage(t, s, BudgetRuleSteps); got != 9 {
		t.Fatalf("refusal was not at the parent lookup: used %d", got)
	}
	if len(s.contextResults) != 0 {
		t.Fatal("parent lookup refusal published facts")
	}
}
