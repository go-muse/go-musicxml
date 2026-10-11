package validation

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"unsafe"
)

func TestCatalogDigestCanonicalPlanDeclarationSet(t *testing.T) {
	r, p, b := catalogFixture(t)
	children := []DeclarationID{
		{Assembly: AssemblyMusicXML, OwnerOccurrence: "owner", Occurrence: "a"},
		{Assembly: AssemblyOpus, OwnerOccurrence: "owner", Occurrence: "b"},
	}
	var err error
	b[0].Plan, err = NewChildCountPlan(children, 1, -1)
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewCatalog(r, p, b)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := a.Profile(p[0].ID)
	children[0], children[1] = children[1], children[0]
	b[0].Plan, err = NewChildCountPlan(children, 1, -1)
	if err != nil {
		t.Fatal(err)
	}
	p[0].CatalogDigest = before.CatalogDigest
	if _, err := NewCatalog(r, p, b); err != nil {
		t.Fatalf("equivalent child declaration set changed digest: %v", err)
	}
}

func TestCatalogLimitsCheckedBeforeSnapshot(t *testing.T) {
	cases := []struct {
		name   string
		change func(*[]RuleDefinition, *[]Profile, *[]PlanBinding)
	}{
		{"rules", func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) { *r = make([]RuleDefinition, 65537) }},
		{"profiles", func(_ *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) { *p = make([]Profile, 4097) }},
		{"plans", func(_ *[]RuleDefinition, _ *[]Profile, b *[]PlanBinding) { *b = make([]PlanBinding, 65537) }},
		{"source ID slots", func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) {
			(*r)[0].Sources[0].IDs = make([]string, (64<<20)/128+1)
		}},
		{"rule text", func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) {
			(*r)[0].Text.Title = strings.Repeat("x", 4097)
		}},
		{"closure text", func(_ *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) {
			(*p)[0].NormativeClosure = strings.Repeat("x", 16385)
		}},
		{"URI", func(_ *[]RuleDefinition, p *[]Profile, _ *[]PlanBinding) {
			(*p)[0].SupportedRoots[0].Space = strings.Repeat("x", 4097)
		}},
		{"identity", func(r *[]RuleDefinition, _ *[]Profile, _ *[]PlanBinding) {
			(*r)[0].Key.ID = RuleID(strings.Repeat("x", 1025))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, p, b := catalogFixture(t)
			tc.change(&r, &p, &b)
			_, err := NewCatalog(r, p, b)
			var config *ConfigurationError
			if !errors.As(err, &config) || config.Field != "catalog size" {
				t.Fatalf("expected early size error, got %v", err)
			}
		})
	}
	t.Run("typed plan aggregate", func(t *testing.T) {
		r, p, b := catalogFixture(t)
		plan, err := NewAtomicComparisonPlan(ComparisonEqual, Atomic{Kind: AtomicString, Text: strings.Repeat("x", 2<<20)})
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 33; i++ {
			b = append(b, PlanBinding{ID: fmt.Sprintf("large-plan-%d", i), Plan: plan})
		}
		_, err = NewCatalog(r, p, b)
		var config *ConfigurationError
		if !errors.As(err, &config) || config.Field != "catalog size" {
			t.Fatalf("expected aggregate parameter rejection, got %v", err)
		}
	})
	t.Run("exact field limits", func(t *testing.T) {
		r, p, b := catalogFixture(t)
		r[0].Text.Title = strings.Repeat("x", 4096)
		r[0].Sources[0].IDs[0] = strings.Repeat("i", 1024)
		p[0].NormativeClosure = strings.Repeat("n", 16384)
		if _, err := NewCatalog(r, p, b); err != nil {
			t.Fatal(err)
		}
	})
}

func TestCatalogScopeRelationValidation(t *testing.T) {
	cases := []struct {
		name               string
		consumer, provider ScopeKind
		relation           RelationKind
		valid              bool
	}{
		{"document aggregate over nodes", ScopeDocument, ScopeNode, RelationDocument, true},
		{"node document prerequisite", ScopeNode, ScopeDocument, RelationDocument, true},
		{"document same scope", ScopeDocument, ScopeDocument, RelationSameNode, true},
		{"node same scope", ScopeNode, ScopeNode, RelationSameNode, true},
		{"document parent consumer", ScopeDocument, ScopeNode, RelationParent, false},
		{"document parent prerequisite", ScopeNode, ScopeDocument, RelationParent, false},
		{"node same document", ScopeNode, ScopeDocument, RelationSameNode, false},
		{"document same node", ScopeDocument, ScopeNode, RelationSameNode, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, p, b := catalogFixture(t)
			provider := r[0]
			provider.Key.ID = "provider"
			provider.Scope = tc.provider
			r[0].Scope = tc.consumer
			r[0].Requires = []Dependency{{Kind: DependencyRule, Rule: provider.Key, Relation: tc.relation}}
			r = append(r, provider)
			p[0].Support = append(p[0].Support, RuleSupport{Rule: provider.Key, State: ImplementationAvailable})
			_, err := NewCatalog(r, p, b)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}

func TestCatalogLongDependencyChain(t *testing.T) {
	r, p, b := catalogFixture(t)
	const count = 5000
	prototype := r[0]
	r = make([]RuleDefinition, count)
	p[0].Support = make([]RuleSupport, count)
	for i := range r {
		r[i] = prototype
		r[i].Key = RuleKey{ID: RuleID(fmt.Sprintf("chain-%05d", i)), Revision: 1}
		if i > 0 {
			r[i].Requires = []Dependency{{Kind: DependencyRule, Rule: r[i-1].Key, Relation: RelationSameNode}}
		}
		p[0].Support[i] = RuleSupport{Rule: r[i].Key, State: ImplementationAvailable}
	}
	p[0].Required = []RuleKey{r[count-1].Key}
	c, err := NewCatalog(r, p, b)
	if err != nil {
		t.Fatal(err)
	}
	order := c.orders[p[0].ID]
	if len(order) != count {
		t.Fatalf("order length=%d", len(order))
	}
	for i, key := range order {
		if key != r[i].Key {
			t.Fatalf("position %d=%v, want %v", i, key, r[i].Key)
		}
	}
}

func TestCatalogPlanSnapshotAndConcurrentAccessors(t *testing.T) {
	r, p, b := catalogFixture(t)
	c, err := NewCatalog(r, p, b)
	if err != nil {
		t.Fatal(err)
	}
	b[0].Plan.children[0].Occurrence = "mutated"
	if c.plans[b[0].ID].children[0].Occurrence != "test.child" {
		t.Fatal("catalog plan aliases input")
	}
	var group sync.WaitGroup
	for i := 0; i < 16; i++ {
		group.Go(func() {
			for j := 0; j < 64; j++ {
				rule, ok := c.Rule(r[0].Key)
				if !ok {
					t.Error("rule missing")
					return
				}
				profile, ok := c.Profile(p[0].ID)
				if !ok {
					t.Error("profile missing")
					return
				}
				if rule.Selector.Declaration.Occurrence != "test.root" || profile.Support[0].Reason != "" {
					t.Error("parallel accessor mutation escaped")
					return
				}
				rule.Selector.Declaration.Occurrence = "mutated"
				rule.Sources[0].IDs[0] = "mutated"
				profile.Support[0].Reason = "mutated"
			}
		})
	}
	group.Wait()
	var nilCatalog *CompiledCatalog
	if _, ok := nilCatalog.Rule(r[0].Key); ok {
		t.Fatal("nil catalog rule found")
	}
	if _, ok := nilCatalog.Profile(p[0].ID); ok {
		t.Fatal("nil catalog profile found")
	}
	if _, ok := c.Rule(RuleKey{ID: r[0].Key.ID, Revision: 99}); ok {
		t.Fatal("unregistered revision found")
	}
}

// A short substring may otherwise keep an arbitrarily large caller allocation
// reachable. Check every string, including enum-typed fields, without mutating
// Go's immutable string storage.
func TestCatalogDetachesStringBackingStorage(t *testing.T) {
	r, p, b := catalogDependencyFixture(t)
	catalogPadMetadataStrings(reflect.ValueOf(&r).Elem())
	catalogPadMetadataStrings(reflect.ValueOf(&p).Elem())
	c, err := NewCatalog(r, p, b)
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range r {
		owned, ok := c.Rule(rule.Key)
		if !ok {
			t.Fatal("missing rule")
		}
		catalogCheckDetachedStrings(t, reflect.ValueOf(rule), reflect.ValueOf(owned), "rule")
	}
	owned, _ := c.Profile(p[0].ID)
	catalogCheckDetachedStrings(t, reflect.ValueOf(p[0]), reflect.ValueOf(owned), "profile")
	for _, rule := range r {
		catalogCheckDetachedStrings(t, reflect.ValueOf(rule), reflect.ValueOf(c.rules[rule.Key]), "stored rule")
	}
	catalogCheckDetachedStrings(t, reflect.ValueOf(p[0]), reflect.ValueOf(c.profiles[p[0].ID]), "stored profile")
}

func catalogPadMetadataStrings(v reflect.Value) {
	switch v.Kind() {
	case reflect.String:
		if v.Len() != 0 {
			s := v.String()
			padded := s + strings.Repeat("padding", 1024)
			v.SetString(padded[:len(s)])
		}
	case reflect.Pointer:
		if !v.IsNil() {
			catalogPadMetadataStrings(v.Elem())
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			catalogPadMetadataStrings(v.Field(i))
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			catalogPadMetadataStrings(v.Index(i))
		}
	}
}
func catalogCheckDetachedStrings(t *testing.T, a, b reflect.Value, path string) {
	t.Helper()
	switch a.Kind() {
	case reflect.String:
		if a.Len() != 0 && a.String() == b.String() && unsafe.StringData(a.String()) == unsafe.StringData(b.String()) {
			t.Errorf("%s retains caller string backing", path)
		}
	case reflect.Pointer:
		if !a.IsNil() {
			catalogCheckDetachedStrings(t, a.Elem(), b.Elem(), path)
		}
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			catalogCheckDetachedStrings(t, a.Field(i), b.Field(i), path+"."+a.Type().Field(i).Name)
		}
	case reflect.Slice:
		for i := 0; i < a.Len(); i++ {
			catalogCheckDetachedStrings(t, a.Index(i), b.Index(i), fmt.Sprintf("%s[%d]", path, i))
		}
	}
}

func TestCatalogRejectsImpossibleRelationsRegardlessSupport(t *testing.T) {
	for _, mode := range []string{"unsupported", "unselected"} {
		t.Run(mode, func(t *testing.T) {
			r, p, b := catalogFixture(t)
			provider := r[0]
			provider.Key.ID = "document-provider"
			provider.Scope = ScopeDocument
			r[0].Requires = []Dependency{{Kind: DependencyRule, Rule: provider.Key, Relation: RelationParent}}
			r = append(r, provider)
			if mode == "unsupported" {
				p[0].Support[0].State = ImplementationUnsupported
				p[0].Support[0].Reason = "not-bound"
				p[0].Support = append(p[0].Support, RuleSupport{Rule: provider.Key, State: ImplementationUnsupported, Reason: "not-bound"})
			} else {
				p = nil
			}
			_, err := NewCatalog(r, p, b)
			if !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("impossible parent-to-document dependency accepted: %v", err)
			}
		})
	}
}
