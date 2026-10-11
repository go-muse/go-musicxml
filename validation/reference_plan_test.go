package validation

import (
	"reflect"
	"testing"
)

func TestTypedReferencePlanRequiresExactBoundedConfiguration(t *testing.T) {
	good := ReferencePlanSpec{Attribute: localAttr("id"), Targets: []DeclarationID{localDecl("score-instrument")}, Owners: []DeclarationID{localDecl("score-part")}, OwnerReferences: []ReferenceOwner{{Declaration: localDecl("part"), Attribute: localAttr("part-id"), Rule: RuleKey{ID: "part.target", Revision: 1}}}}
	if _, err := NewTypedReferencePlan(good); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*ReferencePlanSpec){
		func(p *ReferencePlanSpec) { p.Attribute.Declaration = DeclarationID{} },
		func(p *ReferencePlanSpec) { p.Targets = nil },
		func(p *ReferencePlanSpec) { p.Targets = append(p.Targets, p.Targets[0]) },
		func(p *ReferencePlanSpec) { p.Owners = nil },
		func(p *ReferencePlanSpec) { p.OwnerReferences[0].Rule = RuleKey{} },
		func(p *ReferencePlanSpec) { p.TargetAncestors = []DeclarationID{{}} },
		func(p *ReferencePlanSpec) { p.TriggerChildren = []DeclarationID{{}} },
	} {
		p := good
		p.Targets = append([]DeclarationID(nil), good.Targets...)
		p.OwnerReferences = append([]ReferenceOwner(nil), good.OwnerReferences...)
		change(&p)
		if _, err := NewTypedReferencePlan(p); err == nil {
			t.Errorf("accepted %+v", p)
		}
	}
}
func TestTypedReferencePlanOwnsConfigurationAndDigest(t *testing.T) {
	spec := ReferencePlanSpec{Attribute: localAttr("id"), Targets: []DeclarationID{localDecl("score-instrument")}, TargetAncestors: []DeclarationID{localDecl("score-part")}, Owners: []DeclarationID{localDecl("score-part")}, OwnerReferences: []ReferenceOwner{{Declaration: localDecl("part"), Attribute: localAttr("part-id"), Rule: RuleKey{ID: "part.target", Revision: 1}}}, TriggerChildren: []DeclarationID{localDecl("midi-channel")}}
	p, err := NewTypedReferencePlan(spec)
	if err != nil {
		t.Fatal(err)
	}
	before := p.digestValue()
	spec.Targets[0].Occurrence = "mutated"
	spec.Owners[0].Occurrence = "mutated"
	spec.OwnerReferences[0].Rule.ID = "mutated"
	spec.TriggerChildren[0].Occurrence = "mutated"
	if !reflect.DeepEqual(before, p.digestValue()) {
		t.Fatal("retained caller slices")
	}
}

func referenceBindings(t *testing.T, schema *CompiledSchema, conditional bool) []localRule {
	t.Helper()
	scorePart := localUse(t, schema, "part-list", "score-part", false).Declaration
	partList := localUse(t, schema, "root", "part-list", false).Declaration
	instrument := localUse(t, schema, "score-part", "score-instrument", false).Declaration
	var owners []ReferenceOwner
	var rules []localRule
	for _, parent := range []string{"root", "measure"} {
		decl := localUse(t, schema, parent, "part", false).Declaration
		a := localSchemaAttr(t, schema, "part", "id")
		p, e := NewTypedReferencePlan(ReferencePlanSpec{Attribute: a, Targets: []DeclarationID{scorePart}, TargetAncestors: []DeclarationID{partList}})
		key := RuleKey{ID: RuleID("part." + parent), Revision: 1}
		if e != nil {
			t.Fatal(e)
		}
		rules = append(rules, localRule{key, decl, p})
		owners = append(owners, ReferenceOwner{Declaration: decl, Attribute: a, Rule: key})
	}
	spec := ReferencePlanSpec{Attribute: localSchemaAttr(t, schema, "reference", "id"), Targets: []DeclarationID{instrument}, Owners: []DeclarationID{scorePart}, OwnerReferences: owners}
	if conditional {
		spec.TriggerChildren = []DeclarationID{localUse(t, schema, "reference", "channel", false).Declaration, localUse(t, schema, "reference", "bank", false).Declaration, localUse(t, schema, "reference", "program", false).Declaration}
	}
	p, e := NewTypedReferencePlan(spec)
	if e != nil {
		t.Fatal(e)
	}
	for _, placement := range [][2]string{{"part", "assignment"}, {"score-part", "assignment"}, {"score-part", "instrument-link"}, {"note", "instrument"}, {"part", "play"}} {
		d := localUse(t, schema, placement[0], placement[1], false).Declaration
		rules = append(rules, localRule{RuleKey{ID: RuleID("ref." + placement[0] + "." + placement[1]), Revision: 1}, d, p})
	}
	spec.Attribute = localSchemaAttr(t, schema, "assess", "player")
	spec.Targets = []DeclarationID{localUse(t, schema, "score-part", "player", false).Declaration}
	spec.TriggerChildren = nil
	p, e = NewTypedReferencePlan(spec)
	if e != nil {
		t.Fatal(e)
	}
	rules = append(rules, localRule{RuleKey{ID: "ref.assess", Revision: 1}, localUse(t, schema, "note", "assess", false).Declaration, p})
	return rules
}
func referenceHeader() *localTree {
	return lt("part-list", la("score-part", "P1", la("score-instrument", "I1"), la("player", "A1")), la("score-part", "P2", la("score-instrument", "I2"), la("player", "A2")))
}
func TestTypedReferenceAuthoritativeResolutionBothTargets(t *testing.T) {
	schema := localSchema(t)
	for _, target := range []Target{TargetSource, TargetModel} {
		for _, tc := range []struct {
			name, id, part string
			other          *localTree
			want           Outcome
			cause          string
		}{
			{name: "forward", id: "I1", part: "P1", want: OutcomePass},
			{name: "wrong kind", id: "P1", part: "P1", want: OutcomeFail},
			{name: "wrong part", id: "I2", part: "P1", want: OutcomeFail},
			{name: "missing", id: "MISSING", part: "P1", want: OutcomeBlocked, cause: "xsd.identity"},
			{name: "invalid", id: "bad id", part: "P1", want: OutcomeBlocked, cause: "xsd.attributes"},
			{name: "ambiguous other kind", id: "I1", part: "P1", other: la("other", "I1"), want: OutcomeBlocked, cause: "xsd.identity"},
			{name: "unrelated bad ID", id: "I1", part: "P1", other: la("other", "bad id"), want: OutcomePass},
			{name: "wrong kind containing part", id: "I1", part: "I1", want: OutcomeBlocked, cause: "part.root"},
			{name: "missing containing part", id: "I1", part: "NO_PART", want: OutcomeBlocked, cause: "xsd.identity"},
		} {
			t.Run(string(target)+"/"+tc.name, func(t *testing.T) {
				root := lt("root", la("part", tc.part, la("assignment", tc.id)), referenceHeader())
				if tc.other != nil {
					root.children = append(root.children, tc.other)
				}
				r := localFeed(t, localEngine(t, schema, target, referenceBindings(t, schema, false)...), target, root, true)
				d, found := localFinding(r, "ref.part.assignment")
				if tc.want == OutcomePass {
					if found {
						t.Fatalf("unexpected reference finding %+v report %+v", d, r)
					}
				} else if !found || d.Outcome != tc.want {
					t.Fatalf("want=%s got=%+v report=%+v", tc.want, d, r)
				}
				if tc.want == OutcomeFail && (d.Expected == "" || d.Actual == "") {
					t.Fatalf("missing expected/actual %+v", d)
				}
				if tc.cause != "" && (d.Cause == nil || string(d.Cause.Rule.ID) != tc.cause) {
					t.Fatalf("cause %s got %+v", tc.cause, d)
				}
				if tc.name == "missing" && r.Counts.Fail != 1 {
					t.Fatalf("duplicate missing failures: %+v", r)
				}
			})
		}
	}
}
func TestTypedReferenceOptionalAndRepeatedScopes(t *testing.T) {
	schema := localSchema(t)
	for _, target := range []Target{TargetSource, TargetModel} {
		root := lt("root", referenceHeader(), lt("measure", la("part", "P1", lt("play"), lt("note", la("instrument", "I1"), la("instrument", "I1"), &localTree{name: "assess", attrs: map[string]string{"type": "yes"}}))), lt("measure", la("part", "P1", la("assignment", "I1"))))
		r := localFeed(t, localEngine(t, schema, target, referenceBindings(t, schema, false)...), target, root, true)
		if !r.Satisfied() {
			t.Fatalf("%s optional or repeated refs: %+v", target, r)
		}
		root = lt("root", referenceHeader(), la("part", "P1", lt("note", &localTree{name: "assess", attrs: map[string]string{"type": "yes", "player": "A2"}})))
		r = localFeed(t, localEngine(t, schema, target, referenceBindings(t, schema, false)...), target, root, true)
		if d, ok := localFinding(r, "ref.assess"); !ok || d.Outcome != OutcomeFail {
			t.Fatalf("player wrong scope %+v", r)
		}
		root = lt("root", referenceHeader(), la("part", "P1", lt("note", lt("assess"))))
		r = localFeed(t, localEngine(t, schema, target, referenceBindings(t, schema, false)...), target, root, true)
		if r.Counts.Fail != 1 {
			t.Fatalf("optional player excused required type: %+v", r)
		}
	}
}
func TestTypedReferenceConditionalUsesSameConsequence(t *testing.T) {
	schema := localSchema(t)
	for _, target := range []Target{TargetSource, TargetModel} {
		for _, trigger := range []bool{false, true} {
			assignment := la("assignment", "I2")
			if trigger {
				assignment.children = []*localTree{{name: "channel", text: "0"}}
			}
			root := lt("root", referenceHeader(), la("part", "P1", assignment))
			rules := referenceBindings(t, schema, true)
			r := localFeed(t, localEngine(t, schema, target, rules...), target, root, true)
			d, found := localFinding(r, "ref.part.assignment")
			if trigger {
				if !found || d.Outcome != OutcomeFail || r.Counts.Fail != 1 {
					t.Fatalf("conditional wrong scope: %+v", r)
				}
			} else if found {
				t.Fatalf("absent trigger selected: %+v", d)
			}
		}
	}
}
func TestTypedReferenceIncompleteAndUnselectedOwner(t *testing.T) {
	schema := localSchema(t)
	rules := referenceBindings(t, schema, false)
	root := lt("root", referenceHeader(), la("part", "P1", la("assignment", "I1")))
	r := localFeed(t, localEngine(t, schema, TargetSource, rules...), TargetSource, root, false)
	if d, ok := localFinding(r, "ref.part.assignment"); !ok || d.Outcome != OutcomeUnsupported {
		t.Fatalf("incomplete identity passed %+v", r)
	}
	r = localFeed(t, localEngine(t, schema, TargetSource, rules[2:]...), TargetSource, root, true)
	if d, ok := localFinding(r, "ref.part.assignment"); !ok || d.Outcome != OutcomeUnsupported || d.Reason != "typed-owner-provider-unavailable" {
		t.Fatalf("missing provider passed %+v", r)
	}
}

func TestTypedReferenceProviderBindingsRejectCyclesAndMismatch(t *testing.T) {
	schema := localSchema(t)
	rules := referenceBindings(t, schema, false)
	engine := localEngine(t, schema, TargetSource, rules...)
	consumer := rules[2].plan
	providerKey := rules[0].key
	original := engine.catalog.plans[string(providerKey.ID)]
	for _, name := range []string{"self-reference", "wrong-target", "wrong-slot", "conditional-provider", "non-reference"} {
		t.Run(name, func(t *testing.T) {
			bad := original.clone()
			switch name {
			case "self-reference":
				bad.reference.OwnerReferences = consumer.reference.OwnerReferences
				bad.reference.Owners = consumer.reference.Owners
			case "wrong-target":
				bad.reference.Targets = consumer.reference.Targets
			case "wrong-slot":
				bad.reference.Attribute = consumer.reference.Attribute
			case "conditional-provider":
				bad.reference.TriggerChildren = []DeclarationID{localDecl("channel")}
			case "non-reference":
				bad = Plan{kind: planAtomic, comparison: ComparisonEqual, expected: localString("a")}
			}
			engine.catalog.plans[string(providerKey.ID)] = bad
			if e := localRecompileCatalog(engine.catalog); e == nil {
				t.Fatal("accepted invalid provider")
			}
			engine.catalog.plans[string(providerKey.ID)] = original
		})
	}
	r := engine.catalog.rules[providerKey]
	r.Selector.Declaration = &consumer.reference.Targets[0]
	engine.catalog.rules[providerKey] = r
	if e := localRecompileCatalog(engine.catalog); e == nil {
		t.Fatal("accepted wrong provider selector")
	}
}

func localRecompileCatalog(c *CompiledCatalog) error {
	var rules []RuleDefinition
	var profiles []Profile
	var bindings []PlanBinding
	for _, r := range c.rules {
		rules = append(rules, r)
	}
	for _, p := range c.profiles {
		p.CatalogDigest = ""
		profiles = append(profiles, p)
	}
	for id, p := range c.plans {
		bindings = append(bindings, PlanBinding{ID: id, Plan: p})
	}
	_, err := NewCatalog(rules, profiles, bindings)
	return err
}
