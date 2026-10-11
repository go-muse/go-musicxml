package validation

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func catalogDependencyFixture(t *testing.T) ([]RuleDefinition, []Profile, []PlanBinding) {
	t.Helper()
	rules, profiles, plans := catalogFixture(t)
	provider := rules[0]
	provider.Key = RuleKey{ID: "test.provider", Revision: 1}
	predecessor := rules[0]
	predecessor.Key = RuleKey{ID: "test.provider", Revision: 2}
	provider.Requires = []Dependency{{Kind: DependencyRule, Rule: predecessor.Key, Relation: RelationDocument}}
	rules[0].Requires = []Dependency{{Kind: DependencyRule, Rule: provider.Key, Relation: RelationSameNode}}
	rules = append(rules, provider, predecessor)
	profiles[0].Support = append(profiles[0].Support,
		RuleSupport{Rule: provider.Key, State: ImplementationAvailable},
		RuleSupport{Rule: predecessor.Key, State: ImplementationAvailable})
	return rules, profiles, plans
}

func TestCatalogDependencyClosureUsesFullRevision(t *testing.T) {
	rules, profiles, plans := catalogDependencyFixture(t)
	c, err := NewCatalog(rules, profiles, plans)
	if err != nil {
		t.Fatal(err)
	}
	want := []RuleKey{rules[2].Key, rules[1].Key, rules[0].Key}
	if !reflect.DeepEqual(c.orders[profiles[0].ID], want) {
		t.Fatalf("dependency order = %v, want %v", c.orders[profiles[0].ID], want)
	}
	for _, key := range want {
		if _, ok := c.Rule(key); !ok {
			t.Errorf("missing exact revision %v", key)
		}
	}
	for _, relation := range []RelationKind{RelationSameNode, RelationParent, RelationDocument} {
		t.Run(string(relation), func(t *testing.T) {
			r, p, b := catalogDependencyFixture(t)
			r[0].Requires[0].Relation = relation
			if _, err := NewCatalog(r, p, b); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCatalogRejectsIncompleteTransitiveInventory(t *testing.T) {
	tests := map[string]func(*[]RuleDefinition, *[]Profile, *[]PlanBinding){
		"missing transitive support": func(_ *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) { (*p)[0].Support = (*p)[0].Support[:2] },
		"wrong dependency revision":  func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { (*r)[1].Requires[0].Rule.Revision = 99 },
		"transitive target mismatch": func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { (*r)[2].Targets = []Target{TargetModel} },
		"extra support": func(r *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) {
			extra := (*r)[2]
			extra.Key.ID = "unselected"
			*r = append(*r, extra)
			(*p)[0].Support = append((*p)[0].Support, RuleSupport{Rule: extra.Key, State: ImplementationAvailable})
		},
		"unsupported cycle": func(r *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) {
			(*r)[2].Requires = []Dependency{{Kind: DependencyRule, Rule: (*r)[0].Key, Relation: RelationSameNode}}
			for i := range (*p)[0].Support {
				(*p)[0].Support[i].State = ImplementationUnsupported
				(*p)[0].Support[i].Reason = "deferred"
			}
		},
		"unused cycle": func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) {
			extra := (*r)[2]
			extra.Key.ID = "unselected"
			extra.Requires = []Dependency{{Kind: DependencyRule, Rule: extra.Key, Relation: RelationSameNode}}
			*r = append(*r, extra)
		},
	}
	for name, edit := range tests {
		t.Run(name, func(t *testing.T) {
			r, p, b := catalogDependencyFixture(t)
			edit(&r, &p, &b)
			_, err := NewCatalog(r, p, b)
			if !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("error=%v, want configuration rejection", err)
			}
		})
	}
}

func TestCatalogRejectsMalformedMetadata(t *testing.T) {
	tests := map[string]func(*[]RuleDefinition, *[]Profile, *[]PlanBinding){
		"rule id":       func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { (*r)[0].Key.ID = " " },
		"class":         func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { (*r)[0].Class = "invented" },
		"role":          func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { (*r)[0].Role = "invented" },
		"scope":         func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { (*r)[0].Scope = "invented" },
		"target":        func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { (*r)[0].Targets = []Target{"invented"} },
		"targets empty": func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { (*r)[0].Targets = nil },
		"duplicate target": func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) {
			(*r)[0].Targets = append((*r)[0].Targets, TargetSource)
		},
		"assembly": func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) {
			(*r)[0].Selector.Declaration.Assembly = "invented"
		},
		"declaration owner": func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) {
			(*r)[0].Selector.Declaration.OwnerOccurrence = ""
		},
		"declaration occurrence": func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) {
			(*r)[0].Selector.Declaration.Occurrence = ""
		},
		"selector missing": func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { (*r)[0].Selector = Selector{} },
		"component kind": func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) {
			(*r)[0].Selector = Selector{Component: &ComponentID{Assembly: AssemblyMusicXML, Kind: "invented", Name: QName{Local: "type"}}}
		},
		"component identity": func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) {
			(*r)[0].Selector = Selector{Component: &ComponentID{Assembly: AssemblyMusicXML, Kind: ComponentType}}
		},
		"anonymous named component": func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) {
			(*r)[0].Selector = Selector{Component: &ComponentID{Assembly: AssemblyMusicXML, Kind: ComponentType, Name: QName{Local: "type"}, Occurrence: "anonymous"}}
		},
		"predicate missing":    func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { (*r)[0].Predicate = "" },
		"title missing":        func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { (*r)[0].Text.Title = "" },
		"explanation missing":  func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { (*r)[0].Text.Explanation = "" },
		"message missing":      func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { (*r)[0].Text.Message = "" },
		"sources missing":      func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { (*r)[0].Sources = nil },
		"source ids missing":   func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { (*r)[0].Sources[0].IDs = nil },
		"source id empty":      func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { (*r)[0].Sources[0].IDs[0] = "" },
		"source url empty":     func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { (*r)[0].Sources[0].URL = "" },
		"source locator empty": func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { (*r)[0].Sources[0].Locator = "" },
		"profile id missing":   func(_ *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) { (*p)[0].ID = "" },
		"version missing":      func(_ *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) { (*p)[0].MusicXMLVersion = "" },
		"registry pin missing": func(_ *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) { (*p)[0].RegistryCommit = "" },
		"closure missing":      func(_ *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) { (*p)[0].NormativeClosure = "" },
		"roots missing":        func(_ *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) { (*p)[0].SupportedRoots = nil },
		"malformed root": func(_ *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) {
			(*p)[0].SupportedRoots[0] = QName{Space: "urn:no-local"}
		},
		"duplicate root": func(_ *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) {
			(*p)[0].SupportedRoots = append((*p)[0].SupportedRoots, (*p)[0].SupportedRoots[0])
		},
		"duplicate required": func(_ *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) {
			(*p)[0].Required = append((*p)[0].Required, (*p)[0].Required[0])
		},
		"unknown support":       func(_ *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) { (*p)[0].Support[0].State = "invented" },
		"available with reason": func(_ *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) { (*p)[0].Support[0].Reason = "unavailable" },
		"duplicate rule":        func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { *r = append(*r, (*r)[0]) },
		"duplicate profile":     func(_ *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) { *p = append(*p, (*p)[0]) },
		"duplicate plan":        func(_ *[]RuleDefinition, _ *[]Profile, b *[]PlanBinding) { *b = append(*b, (*b)[0]) },
		"zero plan":             func(_ *[]RuleDefinition, _ *[]Profile, b *[]PlanBinding) { (*b)[0].Plan = Plan{} },
		"plan id missing":       func(_ *[]RuleDefinition, _ *[]Profile, b *[]PlanBinding) { (*b)[0].ID = "" },
		"relation": func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) {
			(*r)[0].Requires = []Dependency{{Kind: DependencyExternal, Key: "service", Relation: "invented"}}
		},
		"dependency kind": func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) {
			(*r)[0].Requires = []Dependency{{Kind: "invented", Key: "service", Relation: RelationSameNode}}
		},
		"rule dependency with key": func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) {
			(*r)[0].Requires = []Dependency{{Kind: DependencyRule, Rule: (*r)[0].Key, Key: "ambiguous", Relation: RelationSameNode}}
		},
		"external dependency with rule": func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) {
			(*r)[0].Requires = []Dependency{{Kind: DependencyExternal, Rule: (*r)[0].Key, Key: "service", Relation: RelationSameNode}}
		},
		"empty capability key": func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) {
			(*r)[0].Requires = []Dependency{{Kind: DependencyFact, Relation: RelationSameNode}}
		},
		"oversized text": func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) {
			(*r)[0].Text.Explanation = strings.Repeat("x", (1<<20)+1)
		},
	}
	for name, edit := range tests {
		t.Run(name, func(t *testing.T) {
			r, p, b := catalogFixture(t)
			edit(&r, &p, &b)
			_, err := NewCatalog(r, p, b)
			if !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("error=%v, want configuration rejection", err)
			}
		})
	}
}

func TestCatalogPreservesUnavailableCapabilityObligations(t *testing.T) {
	cases := []struct {
		name   string
		change func(*RuleDefinition)
	}{
		{"part scope", func(r *RuleDefinition) { r.Scope = ScopePart }},
		{"sibling scope", func(r *RuleDefinition) { r.Scope = ScopeSiblings }},
		{"element selector", func(r *RuleDefinition) {
			r.Selector = Selector{Component: &ComponentID{Assembly: AssemblyMusicXML, Kind: ComponentElement, Name: QName{Local: "root"}}}
		}},
		{"group selector", func(r *RuleDefinition) {
			r.Selector = Selector{Component: &ComponentID{Assembly: AssemblyMusicXML, Kind: ComponentGroup, Name: QName{Local: "root"}}}
		}},
		{"attribute selector", func(r *RuleDefinition) {
			r.Selector = Selector{Component: &ComponentID{Assembly: AssemblyMusicXML, Kind: ComponentAttribute, Name: QName{Local: "root"}}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, p, b := catalogFixture(t)
			tc.change(&r[0])
			if _, err := NewCatalog(r, p, b); err == nil {
				t.Fatal("unimplemented capability advertised available")
			}
			p[0].Support[0].State = ImplementationUnsupported
			p[0].Support[0].Reason = "unimplemented-capability"
			c, err := NewCatalog(r, p, nil)
			if err != nil {
				t.Fatal(err)
			}
			got, _ := c.Profile(p[0].ID)
			if len(got.Support) != 1 || got.Support[0].State != ImplementationUnsupported {
				t.Fatal("unsupported obligation lost")
			}
		})
	}
	for _, relation := range []RelationKind{RelationPart, RelationMusicalPosition} {
		t.Run(string(relation), func(t *testing.T) {
			r, p, b := catalogDependencyFixture(t)
			r[0].Requires[0].Relation = relation
			c, err := NewCatalog(r, p, b)
			if err != nil {
				t.Fatal(err)
			}
			if len(c.orders[p[0].ID]) != 3 {
				t.Fatal("unavailable dependency closure lost")
			}
			got, _ := c.Rule(r[0].Key)
			if got.Requires[0].Relation != relation {
				t.Fatal("unavailable relation erased")
			}
		})
	}
	for _, kind := range []DependencyKind{DependencyFact, DependencyExternal} {
		t.Run(string(kind), func(t *testing.T) {
			r, p, b := catalogFixture(t)
			r[0].Requires = []Dependency{{Kind: kind, Key: "service", Relation: RelationDocument}}
			c, err := NewCatalog(r, p, b)
			if err != nil {
				t.Fatal(err)
			}
			got, _ := c.Rule(r[0].Key)
			if !reflect.DeepEqual(got.Requires, r[0].Requires) {
				t.Fatal("unavailable capability erased")
			}
		})
	}
}

func TestCatalogDigestCanonicalAndIncludesTypedPlan(t *testing.T) {
	r, p, b := catalogDependencyFixture(t)
	r[0].Targets = append(r[0].Targets, TargetModel)
	p[0].SupportedRoots = append(p[0].SupportedRoots, QName{Space: "urn:other", Local: "root"})
	c, err := NewCatalog(r, p, b)
	if err != nil {
		t.Fatal(err)
	}
	original, _ := c.Profile(p[0].ID)
	r[0], r[2] = r[2], r[0]
	r[2].Targets[0], r[2].Targets[1] = r[2].Targets[1], r[2].Targets[0]
	p[0].Support[0], p[0].Support[2] = p[0].Support[2], p[0].Support[0]
	p[0].SupportedRoots[0], p[0].SupportedRoots[1] = p[0].SupportedRoots[1], p[0].SupportedRoots[0]
	p[0].CatalogDigest = original.CatalogDigest
	reordered, err := NewCatalog(r, p, b)
	if err != nil {
		t.Fatalf("equivalent order rejected: %v", err)
	}
	equivalent, _ := reordered.Profile(p[0].ID)
	if equivalent.CatalogDigest != original.CatalogDigest {
		t.Fatal("input permutation changed digest")
	}
	changedPlan, err := NewChildCountPlan([]DeclarationID{{Assembly: AssemblyMusicXML, OwnerOccurrence: "test.owner", Occurrence: "test.child"}}, 2, -1)
	if err != nil {
		t.Fatal(err)
	}
	b[0].Plan = changedPlan
	if _, err := NewCatalog(r, p, b); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("mismatched supplied digest accepted: %v", err)
	}
	p[0].CatalogDigest = ""
	changed, err := NewCatalog(r, p, b)
	if err != nil {
		t.Fatal(err)
	}
	different, _ := changed.Profile(p[0].ID)
	if different.CatalogDigest == original.CatalogDigest {
		t.Fatal("typed plan parameters missing from digest")
	}
}

func TestCatalogOwnsEveryMetadataCollection(t *testing.T) {
	r, p, b := catalogDependencyFixture(t)
	r[0].Selector = Selector{Component: &ComponentID{Assembly: AssemblyMusicXML, Kind: ComponentType, Name: QName{Local: "rootType"}}}
	c, err := NewCatalog(r, p, b)
	if err != nil {
		t.Fatal(err)
	}
	key := r[0].Key
	original, _ := c.Rule(key)
	profile, _ := c.Profile(p[0].ID)
	r[0].Selector.Component.Name.Local = "mutated"
	r[0].Targets[0] = TargetModel
	r[0].Requires[0].Rule.ID = "mutated"
	r[0].Sources[0].IDs[0] = "mutated"
	p[0].Support[0].Rule.ID = "mutated"
	p[0].SupportedRoots[0].Local = "mutated"
	got, _ := c.Rule(key)
	gotProfile, _ := c.Profile(p[0].ID)
	if !reflect.DeepEqual(got, original) || !reflect.DeepEqual(gotProfile, profile) {
		t.Fatal("input collection mutation changed catalog")
	}
	got.Selector.Component.Occurrence = "mutated"
	got.Targets[0] = TargetPackage
	got.Requires[0].Rule.ID = "mutated"
	got.Sources[0].IDs[0] = "mutated"
	gotProfile.Required[0].ID = "mutated"
	gotProfile.Support[0].Reason = "mutated"
	gotProfile.SupportedRoots[0].Local = "mutated"
	final, _ := c.Rule(key)
	finalProfile, _ := c.Profile(p[0].ID)
	if !reflect.DeepEqual(final, original) || !reflect.DeepEqual(finalProfile, profile) {
		t.Fatal("returned collection mutation changed catalog")
	}
}
