package validation

import (
	"errors"
	"reflect"
	"testing"
)

func catalogScheduleFixture(t *testing.T, bindings []PlanBinding) ([]RuleDefinition, []Profile, []PlanBinding) {
	t.Helper()
	base, profiles, _ := catalogFixture(t)
	rules := make([]RuleDefinition, len(bindings))
	profiles[0].Required = nil
	profiles[0].Support = nil
	for i, binding := range bindings {
		rule := cloneRule(base[0])
		rule.Key.ID = RuleID(binding.ID)
		rule.Predicate = binding.ID
		if binding.Plan.kind == planContextValue || binding.Plan.kind == planContextDefault {
			rule.Role = RoleContextProvider
		}
		rules[i] = rule
		profiles[0].Required = append(profiles[0].Required, rule.Key)
		profiles[0].Support = append(profiles[0].Support, RuleSupport{Rule: rule.Key, State: ImplementationAvailable})
	}
	return rules, profiles, bindings
}

func TestCatalogDerivedContextAndIdentitySchedule(t *testing.T) {
	value := Atomic{Kind: AtomicString, Text: "value"}
	provider, _ := NewContextValuePlan("key", RelationDocument, value)
	consumer, _ := NewContextComparisonPlan("key", RelationDocument, ComparisonEqual, value)
	slot := AttributeSelector{Name: QName{Local: "id"}, Declaration: DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "owner", Occurrence: "id"}}
	declaration, _ := NewIdentityDeclarationPlan("index", slot)
	reference, _ := NewIdentityReferencePlan("index", slot)
	plain, _ := NewChildCountPlan([]DeclarationID{slot.Declaration}, 0, -1)
	defaultProvider, err := NewContextDefaultPlan("key", RelationDocument, slot, value)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name               string
		provider, consumer Plan
		fact               bool
	}{
		{"context", provider, consumer, false},
		{"default context", defaultProvider, consumer, false},
		{"fact dependency", provider, plain, true},
		{"identity", declaration, reference, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, p, b := catalogScheduleFixture(t, []PlanBinding{{ID: "a-consumer", Plan: tc.consumer}, {ID: "z-provider", Plan: tc.provider}})
			if tc.fact {
				r[0].Requires = []Dependency{{Kind: DependencyFact, Key: "key", Relation: RelationDocument}}
			}
			c, err := NewCatalog(r, p, b)
			if err != nil {
				t.Fatal(err)
			}
			order, ok := c.Schedule(p[0].ID)
			if !ok {
				t.Fatal("schedule missing")
			}
			want := []RuleKey{r[1].Key, r[0].Key}
			if !reflect.DeepEqual(order, want) {
				t.Fatalf("schedule=%v, want %v", order, want)
			}
			got, _ := c.Rule(r[0].Key)
			if !reflect.DeepEqual(got.Requires, r[0].Requires) {
				t.Fatal("ordering edges changed semantic Requires")
			}
			order[0].ID = "mutated"
			again, _ := c.Schedule(p[0].ID)
			if !reflect.DeepEqual(again, want) {
				t.Fatal("returned schedule aliases catalog")
			}
		})
	}
	var nilCatalog *CompiledCatalog
	if _, ok := nilCatalog.Schedule("test"); ok {
		t.Fatal("nil schedule found")
	}
}

func TestCatalogRejectsFactAndMixedSchedulingCycles(t *testing.T) {
	value := Atomic{Kind: AtomicString, Text: "value"}
	a, _ := NewContextValuePlan("a", RelationDocument, value)
	b, _ := NewContextValuePlan("b", RelationDocument, value)
	for _, unsupported := range []bool{false, true} {
		t.Run(map[bool]string{false: "available", true: "unsupported"}[unsupported], func(t *testing.T) {
			r, p, bindings := catalogScheduleFixture(t, []PlanBinding{{ID: "provider-a", Plan: a}, {ID: "provider-b", Plan: b}})
			r[0].Requires = []Dependency{{Kind: DependencyFact, Key: "b", Relation: RelationDocument}}
			r[1].Requires = []Dependency{{Kind: DependencyFact, Key: "a", Relation: RelationDocument}}
			if unsupported {
				for i := range p[0].Support {
					p[0].Support[i].State = ImplementationUnsupported
					p[0].Support[i].Reason = "unavailable"
				}
			}
			if _, err := NewCatalog(r, p, bindings); !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("fact cycle accepted: %v", err)
			}
		})
	}
	t.Run("mixed explicit and context", func(t *testing.T) {
		consumer, _ := NewContextComparisonPlan("a", RelationDocument, ComparisonEqual, value)
		r, p, bindings := catalogScheduleFixture(t, []PlanBinding{{ID: "provider", Plan: a}, {ID: "consumer", Plan: consumer}})
		r[0].Requires = []Dependency{{Kind: DependencyRule, Rule: r[1].Key, Relation: RelationSameNode}}
		if _, err := NewCatalog(r, p, bindings); !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("mixed cycle accepted: %v", err)
		}
	})
	t.Run("identity", func(t *testing.T) {
		slot := AttributeSelector{Name: QName{Local: "id"}, Declaration: DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "owner", Occurrence: "id"}}
		declaration, _ := NewIdentityDeclarationPlan("index", slot)
		reference, _ := NewIdentityReferencePlan("index", slot)
		r, p, bindings := catalogScheduleFixture(t, []PlanBinding{{ID: "declaration", Plan: declaration}, {ID: "reference", Plan: reference}})
		r[0].Requires = []Dependency{{Kind: DependencyRule, Rule: r[1].Key, Relation: RelationDocument}}
		if _, err := NewCatalog(r, p, bindings); !errors.Is(err, ErrInvalidConfiguration) {
			t.Fatalf("mixed identity cycle accepted: %v", err)
		}
	})
}

func TestCatalogDerivedScheduleDoesNotSelectProviders(t *testing.T) {
	value := Atomic{Kind: AtomicString, Text: "value"}
	provider, _ := NewContextValuePlan("key", RelationDocument, value)
	consumer, _ := NewContextComparisonPlan("key", RelationDocument, ComparisonEqual, value)
	r, p, b := catalogScheduleFixture(t, []PlanBinding{{ID: "a-consumer", Plan: consumer}, {ID: "z-provider", Plan: provider}})
	p[0].Required = p[0].Required[:1]
	p[0].Support = p[0].Support[:1]
	c, err := NewCatalog(r, p, b)
	if err != nil {
		t.Fatal(err)
	}
	order, _ := c.Schedule(p[0].ID)
	if !reflect.DeepEqual(order, []RuleKey{r[0].Key}) {
		t.Fatalf("unselected provider added: %v", order)
	}
	if _, ok := c.Schedule("missing"); ok {
		t.Fatal("unknown profile has schedule")
	}
}

func TestCatalogDerivedScheduleKeepsContextScopesSeparate(t *testing.T) {
	value := Atomic{Kind: AtomicString, Text: "value"}
	for _, relations := range [][2]RelationKind{{RelationDocument, RelationSameNode}, {RelationSameNode, RelationDocument}, {RelationDocument, RelationParent}, {RelationParent, RelationDocument}} {
		t.Run(string(relations[0])+"-"+string(relations[1]), func(t *testing.T) {
			provider, _ := NewContextValuePlan("key", relations[0], value)
			consumer, _ := NewContextComparisonPlan("key", relations[1], ComparisonEqual, value)
			r, p, b := catalogScheduleFixture(t, []PlanBinding{{ID: "a-consumer", Plan: consumer}, {ID: "z-provider", Plan: provider}})
			c, err := NewCatalog(r, p, b)
			if err != nil {
				t.Fatal(err)
			}
			order, _ := c.Schedule(p[0].ID)
			if !reflect.DeepEqual(order, []RuleKey{r[0].Key, r[1].Key}) {
				t.Fatalf("disjoint scope inferred ordering: %v", order)
			}
		})
	}
}

func TestCatalogDerivedScheduleEdgeBudgetAndDeduplication(t *testing.T) {
	value := Atomic{Kind: AtomicString, Text: "value"}
	provider, _ := NewContextValuePlan("key", RelationDocument, value)
	consumer, _ := NewContextComparisonPlan("key", RelationDocument, ComparisonEqual, value)
	r, p, b := catalogScheduleFixture(t, []PlanBinding{{ID: "consumer", Plan: consumer}, {ID: "provider-a", Plan: provider}, {ID: "provider-b", Plan: provider}})
	// One prerequisite appears as both a semantic and a derived edge. It is one
	// schedule edge, while preserving its explicit semantic meaning separately.
	r[0].Requires = []Dependency{{Kind: DependencyRule, Rule: r[1].Key, Relation: RelationDocument}, {Kind: DependencyFact, Key: "key", Relation: RelationDocument}}
	c, err := NewCatalog(r, p, b)
	if err != nil {
		t.Fatal(err)
	}
	closure := map[RuleKey]bool{r[0].Key: true, r[1].Key: true, r[2].Key: true}
	exact := catalogBudget{remaining: 2 * catalogSlotBytes}
	if _, err := catalogProfileOrder(c, closure, &exact); err != nil {
		t.Fatalf("two unique edges should fit: %v", err)
	}
	if exact.remaining != 0 {
		t.Fatalf("edge accounting=%d, want 0", exact.remaining)
	}
	short := catalogBudget{remaining: catalogSlotBytes}
	if _, err := catalogProfileOrder(c, closure, &short); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("derived-edge expansion over budget accepted: %v", err)
	}
	// The same budget object is shared by all profiles, so individually small
	// schedules cannot evade the aggregate compiled-edge limit.
	if _, err := catalogProfileOrder(c, closure, &exact); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("schedule budget reset: %v", err)
	}
}

func TestCatalogDerivedOrderingIgnoresOtherKeysAndIndexes(t *testing.T) {
	value := Atomic{Kind: AtomicString, Text: "value"}
	provider, _ := NewContextValuePlan("provided", RelationDocument, value)
	consumer, _ := NewContextComparisonPlan("different", RelationDocument, ComparisonEqual, value)
	slot := AttributeSelector{Name: QName{Local: "id"}, Declaration: DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "owner", Occurrence: "id"}}
	declaration, _ := NewIdentityDeclarationPlan("declared", slot)
	reference, _ := NewIdentityReferencePlan("different", slot)
	for _, pair := range [][2]Plan{{consumer, provider}, {reference, declaration}} {
		r, p, b := catalogScheduleFixture(t, []PlanBinding{{ID: "a-consumer", Plan: pair[0]}, {ID: "z-provider", Plan: pair[1]}})
		c, err := NewCatalog(r, p, b)
		if err != nil {
			t.Fatal(err)
		}
		order, _ := c.Schedule(p[0].ID)
		if !reflect.DeepEqual(order, []RuleKey{r[0].Key, r[1].Key}) {
			t.Fatalf("unrelated provider influenced schedule: %v", order)
		}
	}
}
