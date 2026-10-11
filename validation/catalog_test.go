package validation

import "testing"

func catalogFixture(t *testing.T) ([]RuleDefinition, []Profile, []PlanBinding) {
	t.Helper()
	plan, err := NewChildCountPlan([]DeclarationID{{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: "test.child"}}, 1, -1)
	if err != nil {
		t.Fatal(err)
	}
	key := RuleKey{ID: "test.child-required", Revision: 1}
	rules := []RuleDefinition{{Key: key, Class: ClassMandatory, Role: RolePredicate, Category: CategoryNormative, Targets: []Target{TargetSource}, Scope: ScopeNode, Selector: Selector{Declaration: &DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: "test.root"}}, Predicate: "child-count", Sources: []SourceRef{{IDs: []string{"test.provenance"}, URL: "https://example.org/spec", Locator: "section 1"}}, Text: RuleText{Title: "Child required", Explanation: "A child is required.", Message: "Missing child"}}}
	profiles := []Profile{{ID: "test.source", MusicXMLVersion: "4.0", RegistryCommit: "3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c", Target: TargetSource, Required: []RuleKey{key}, Support: []RuleSupport{{Rule: key, State: ImplementationAvailable}}, SupportedRoots: []QName{{Local: "root"}}, NormativeClosure: "Only this deliberately bounded fixture obligation."}}
	return rules, profiles, []PlanBinding{{ID: "child-count", Plan: plan}}
}

func TestCatalogOwnsNestedMetadata(t *testing.T) {
	rules, profiles, plans := catalogFixture(t)
	catalog, err := NewCatalog(rules, profiles, plans)
	if err != nil {
		t.Fatal(err)
	}
	rules[0].Selector.Declaration.Occurrence = "changed"
	rules[0].Sources[0].IDs[0] = "changed"
	profiles[0].Required[0].ID = "changed"
	key := RuleKey{ID: "test.child-required", Revision: 1}
	first, ok := catalog.Rule(key)
	if !ok {
		t.Fatal("rule missing")
	}
	if first.Selector.Declaration.Occurrence != "test.root" || first.Sources[0].IDs[0] != "test.provenance" {
		t.Fatal("input aliases retained")
	}
	first.Selector.Declaration.Occurrence = "returned mutation"
	first.Sources[0].IDs[0] = "returned mutation"
	second, _ := catalog.Rule(key)
	if second.Selector.Declaration.Occurrence != "test.root" || second.Sources[0].IDs[0] != "test.provenance" {
		t.Fatal("returned rule aliases retained")
	}
	p, _ := catalog.Profile("test.source")
	if p.CatalogDigest == "" {
		t.Fatal("missing compiled digest")
	}
	p.Support[0].State = ImplementationUnsupported
	q, _ := catalog.Profile("test.source")
	if q.Support[0].State != ImplementationAvailable {
		t.Fatal("returned profile alias retained")
	}
}

func TestCatalogRejectsIncompleteSupportAndMalformedContracts(t *testing.T) {
	tests := map[string]func(*[]RuleDefinition, *[]Profile, *[]PlanBinding){
		"missing support": func(_ *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) { (*p)[0].Support = nil },
		"duplicate support": func(_ *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) {
			(*p)[0].Support = append((*p)[0].Support, (*p)[0].Support[0])
		},
		"missing plan":     func(_ *[]RuleDefinition, _ *[]Profile, p *[]PlanBinding) { *p = nil },
		"empty required":   func(_ *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) { (*p)[0].Required = nil },
		"zero revision":    func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { (*r)[0].Key.Revision = 0 },
		"unknown category": func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { (*r)[0].Category = "other" },
		"two selectors": func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) {
			(*r)[0].Selector.Component = &ComponentID{Assembly: AssemblyMusicXML, Kind: ComponentType, Name: QName{Local: "type"}}
		},
		"target mismatch": func(_ *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) { (*p)[0].Target = TargetModel },
		"unsupported no reason": func(_ *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) {
			(*p)[0].Support[0].State = ImplementationUnsupported
		},
		"cycle": func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) {
			(*r)[0].Requires = []Dependency{{Kind: DependencyRule, Rule: (*r)[0].Key, Relation: RelationSameNode}}
		},
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			r, p, b := catalogFixture(t)
			change(&r, &p, &b)
			if _, err := NewCatalog(r, p, b); err == nil {
				t.Fatal("malformed catalog accepted")
			}
		})
	}
}

func TestCatalogRetainsUnsupportedRequiredRule(t *testing.T) {
	r, p, _ := catalogFixture(t)
	p[0].Support[0].State = ImplementationUnsupported
	p[0].Support[0].Reason = "adapter-not-bound"
	catalog, err := NewCatalog(r, p, nil)
	if err != nil {
		t.Fatal(err)
	}
	profile, _ := catalog.Profile("test.source")
	if len(profile.Required) != 1 || profile.Support[0].State != ImplementationUnsupported {
		t.Fatal("unsupported obligation omitted")
	}
}
