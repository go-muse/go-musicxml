package validation

import (
	"errors"
	"strings"
	"testing"
)

func packageFixture(t *testing.T, checks ...PackageCheck) ([]PackageRuleBinding, []PackageProfileBinding) {
	t.Helper()
	base, profiles, _ := catalogFixture(t)
	p := profiles[0]
	p.ID = "test.package"
	p.Target = TargetPackage
	p.SupportedRoots = nil
	p.Required = nil
	p.Support = nil
	var rules []PackageRuleBinding
	for _, check := range checks {
		plan, err := NewPackagePlan(check)
		if err != nil {
			t.Fatal(err)
		}
		r := cloneRule(base[0])
		r.Key = RuleKey{ID: RuleID(check), Revision: 1}
		r.Targets = []Target{TargetPackage}
		r.Scope = ScopePackage
		r.Selector = Selector{}
		r.Predicate = string(check)
		rules = append(rules, PackageRuleBinding{Rule: r, Selector: PackageSelector{Kind: packageCheckSubject(check)}, Plan: plan})
		p.Required = append(p.Required, r.Key)
		p.Support = append(p.Support, RuleSupport{Rule: r.Key, State: ImplementationAvailable})
	}
	return rules, []PackageProfileBinding{{Profile: p}}
}
func TestPackageCatalogPreservesXMLInvariantsAndOwnsMetadata(t *testing.T) {
	rules, profiles := packageFixture(t, PackageCheckContainerPath)
	c, err := NewPackageCatalog(rules, profiles)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := c.Profile("test.package")
	if !ok || p.Target != TargetPackage || len(p.SupportedRoots) != 0 || len(p.CatalogDigest) != 64 {
		t.Fatalf("profile=%+v", p)
	}
	rules[0].Rule.Sources[0].IDs[0] = "edited"
	profiles[0].Profile.Required[0].ID = "edited"
	r, _ := c.Rule(RuleKey{ID: RuleID(PackageCheckContainerPath), Revision: 1})
	if r.Rule.Sources[0].IDs[0] == "edited" {
		t.Fatal("input metadata alias")
	}
	r.Rule.Sources[0].IDs[0] = "edited"
	again, _ := c.Rule(r.Rule.Key)
	if again.Rule.Sources[0].IDs[0] == "edited" {
		t.Fatal("getter metadata alias")
	}
	xmlRules, xmlProfiles, plans := catalogFixture(t)
	xmlRules[0].Selector = Selector{}
	if _, err = NewCatalog(xmlRules, xmlProfiles, plans); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatal("XML selector invariant weakened")
	}
	xmlRules, xmlProfiles, plans = catalogFixture(t)
	xmlProfiles[0].SupportedRoots = nil
	if _, err = NewCatalog(xmlRules, xmlProfiles, plans); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatal("XML roots invariant weakened")
	}
}
func TestPackageCatalogRejectsWrongMetadataSelectorAndDigest(t *testing.T) {
	tests := map[string]func([]PackageRuleBinding, []PackageProfileBinding){
		"XML selector": func(r []PackageRuleBinding, p []PackageProfileBinding) {
			a := AssemblyContainer
			r[0].Rule.Selector.Assembly = &a
		},
		"XML roots": func(r []PackageRuleBinding, p []PackageProfileBinding) {
			p[0].Profile.SupportedRoots = []QName{{Local: "container"}}
		},
		"wrong target":    func(r []PackageRuleBinding, p []PackageProfileBinding) { p[0].Profile.Target = TargetSource },
		"wrong scope":     func(r []PackageRuleBinding, p []PackageProfileBinding) { r[0].Rule.Scope = ScopeDocument },
		"wrong selector":  func(r []PackageRuleBinding, p []PackageProfileBinding) { r[0].Selector.Kind = PackageSubjectEdge },
		"unknown check":   func(r []PackageRuleBinding, p []PackageProfileBinding) { r[0].Plan = PackagePlan{} },
		"missing support": func(r []PackageRuleBinding, p []PackageProfileBinding) { p[0].Profile.Support = nil },
		"wrong digest": func(r []PackageRuleBinding, p []PackageProfileBinding) {
			p[0].Profile.CatalogDigest = strings.Repeat("0", 64)
		},
		"bad intrinsic digest": func(r []PackageRuleBinding, p []PackageProfileBinding) {
			p[0].Intrinsic = []IntrinsicProfileBinding{{Assembly: AssemblyOpus, Target: TargetSource, Profile: "source", CatalogDigest: "bad"}}
		},
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			r, p := packageFixture(t, PackageCheckContainerPath)
			change(r, p)
			if _, err := NewPackageCatalog(r, p); !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("accepted: %v", err)
			}
		})
	}
}
func TestPackageCatalogDigestIncludesIntrinsicInterpretation(t *testing.T) {
	r, p := packageFixture(t, PackageCheckContainerPath)
	c, err := NewPackageCatalog(r, p)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := c.Profile("test.package")
	p[0].Intrinsic = []IntrinsicProfileBinding{{Assembly: AssemblyOpus, Target: TargetSource, Profile: "intrinsic", CatalogDigest: strings.Repeat("a", 64)}}
	c, err = NewPackageCatalog(r, p)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := c.Profile("test.package")
	if a.CatalogDigest == b.CatalogDigest {
		t.Fatal("fixed intrinsic interpretation missing from digest")
	}
}
