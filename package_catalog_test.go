package musicxml

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestPackageProfilesExactSelectionAndMetadata(t *testing.T) {
	for got, want := range map[validation.ProfileID]string{
		ProfileResourceGraph: "musicxml-4.0/resource-graph",
		ProfileMXLSource:     "musicxml-4.0/mxl-source",
		ProfileMXLModel:      "musicxml-4.0/mxl-model",
	} {
		if string(got) != want {
			t.Errorf("profile ID=%q want %q", got, want)
		}
	}
	catalog, err := newPackageAssessmentCatalog()
	if err != nil {
		t.Fatal(err)
	}
	checks := map[validation.PackageCheck]validation.PackageSubjectKind{
		validation.PackageCheckContainerPath:          validation.PackageSubjectPackage,
		validation.PackageCheckMimetypeStored:         validation.PackageSubjectEntry,
		validation.PackageCheckMimetypeNoExtra:        validation.PackageSubjectEntry,
		validation.PackageCheckMimetypeNoPrefix:       validation.PackageSubjectEntry,
		validation.PackageCheckFirstRootMusicXML:      validation.PackageSubjectRootfile,
		validation.PackageCheckRootfileRoot:           validation.PackageSubjectRootfile,
		validation.PackageCheckFirstMediaType:         validation.PackageSubjectRootfile,
		validation.PackageCheckScoreBeforeParts:       validation.PackageSubjectPackage,
		validation.PackageCheckLinkTarget:             validation.PackageSubjectEdge,
		validation.PackageCheckInstrumentAssociation:  validation.PackageSubjectAssociation,
		validation.PackageCheckGroupMatch:             validation.PackageSubjectEdge,
		validation.PackageCheckMimetypeAdvice:         validation.PackageSubjectPackage,
		validation.PackageCheckEntryNameAdvice:        validation.PackageSubjectEntry,
		validation.PackageCheckCompressionObservation: validation.PackageSubjectEntry,
		validation.PackageCheckImageScaling:           validation.PackageSubjectImage,
		validation.PackageCheckFileMetadataAdvice:     validation.PackageSubjectMetadata,
		validation.PackageCheckSchemaHintPairs:        validation.PackageSubjectResource,
	}
	for _, id := range []validation.ProfileID{ProfileResourceGraph, ProfileMXLSource, ProfileMXLModel} {
		t.Run(string(id), func(t *testing.T) {
			p, ok := catalog.Profile(id)
			if !ok || p.ID != id || p.Target != validation.TargetPackage || len(p.SupportedRoots) != 0 || p.RegistryCommit != assessmentRegistryCommit || len(p.CatalogDigest) != 64 {
				t.Fatalf("incorrect package identity: %+v, found=%v", p, ok)
			}
			if len(p.Required) != 17 || len(p.Support) != len(p.Required) {
				t.Fatalf("sealed catalog inventory: required=%d support=%d", len(p.Required), len(p.Support))
			}
			seen := map[validation.PackageCheck]bool{}
			for i, key := range p.Required {
				r, ok := catalog.Rule(key)
				if !ok || r.Rule.Key != key || key.Revision != 1 || r.Rule.Selector != (validation.Selector{}) || r.Rule.Scope != validation.ScopePackage || !reflect.DeepEqual(r.Rule.Targets, []validation.Target{validation.TargetPackage}) {
					t.Fatalf("package binding identity: %+v", r)
				}
				check := r.Plan.Check()
				if seen[check] {
					t.Errorf("duplicate check %q", check)
				}
				seen[check] = true
				if want, ok := checks[check]; ok && r.Selector.Kind != want {
					t.Errorf("%s selector=%s want %s", check, r.Selector.Kind, want)
				}
				if p.Support[i].Rule != key || p.Support[i].State != validation.ImplementationAvailable {
					t.Errorf("missing exact support: %+v", p.Support[i])
				}
				if len(r.Rule.Sources) == 0 || r.Rule.Text.Title == "" || r.Rule.Text.Explanation == "" || r.Rule.Text.Message == "" {
					t.Errorf("missing diagnostic metadata: %+v", r.Rule)
				}
				for _, source := range r.Rule.Sources {
					if len(source.IDs) == 0 || source.URL == "" || source.Locator == "" {
						t.Errorf("untraceable source: %+v", source)
					}
				}
				switch check {
				case validation.PackageCheckGroupMatch, validation.PackageCheckMimetypeAdvice, validation.PackageCheckEntryNameAdvice, validation.PackageCheckImageScaling, validation.PackageCheckFileMetadataAdvice:
					if r.Rule.Role != validation.RoleAdvisory || r.Rule.Class != validation.ClassRecommendation || r.Rule.Category != validation.CategoryAdvisory {
						t.Errorf("advice promoted to mandatory: %+v", r.Rule)
					}
				case validation.PackageCheckCompressionObservation:
					if r.Rule.Role != validation.RoleContextProvider || r.Rule.Class != validation.ClassPolicy || r.Rule.Category != validation.CategoryCapability {
						t.Errorf("compression observation promoted to norm: %+v", r.Rule)
					}
				case validation.PackageCheckSchemaHintPairs:
					if r.Rule.Role != validation.RolePredicate || r.Rule.Class != validation.ClassPolicy || r.Rule.Category != validation.CategoryCapability {
						t.Errorf("discovery pairing promoted to XSD datatype failure: %+v", r.Rule)
					}
				default:
					if r.Rule.Role != validation.RolePredicate || r.Rule.Class != validation.ClassMandatory || r.Rule.Category != validation.CategoryNormative {
						t.Errorf("mandatory predicate identity lost: %+v", r.Rule)
					}
				}
			}
			for check := range checks {
				if !seen[check] {
					t.Errorf("missing sealed check %q", check)
				}
			}
			for _, retained := range []string{"ZIP/JAR", "DEFLATE", "central-only", "padding", "media-type", "indirection", "association", "schema", "version"} {
				if !strings.Contains(p.NormativeClosure, retained) {
					t.Errorf("closure omitted %q: %s", retained, p.NormativeClosure)
				}
			}
		})
	}
}

func TestPackageProfilesPinCompleteIntrinsicTable(t *testing.T) {
	_, profiles, err := buildPackageAssessmentDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 3 {
		t.Fatalf("profiles=%d", len(profiles))
	}
	for _, binding := range profiles {
		want := map[[2]string]validation.ProfileID{
			{string(validation.AssemblyMusicXML), string(validation.TargetSource)}:  ProfileSourceLinks,
			{string(validation.AssemblyOpus), string(validation.TargetSource)}:      ProfileSourceLinks,
			{string(validation.AssemblyContainer), string(validation.TargetSource)}: ProfileSourceContainer,
			{string(validation.AssemblySounds), string(validation.TargetSource)}:    ProfileSourceSounds,
		}
		if binding.Profile.ID != ProfileMXLSource {
			want[[2]string{string(validation.AssemblyMusicXML), string(validation.TargetModel)}] = ProfileModelLinks
			want[[2]string{string(validation.AssemblyOpus), string(validation.TargetModel)}] = ProfileModelLinks
		}
		if len(binding.Intrinsic) != len(want) {
			t.Errorf("%s intrinsic count=%d want %d", binding.Profile.ID, len(binding.Intrinsic), len(want))
		}
		for _, row := range binding.Intrinsic {
			key := [2]string{string(row.Assembly), string(row.Target)}
			if row.Profile != want[key] {
				t.Errorf("%s has wrong intrinsic row %+v", binding.Profile.ID, row)
			}
			var engine *validation.Engine
			if row.Target == validation.TargetSource {
				engine, err = sourceAssessmentEngineForProfile(row.Profile)
			} else {
				engine, err = modelAssessmentEngineForProfile(row.Profile)
			}
			if err != nil {
				t.Fatal(err)
			}
			intrinsic, ok := engine.Profile(row.Profile)
			if !ok || intrinsic.CatalogDigest != row.CatalogDigest || intrinsic.Target != row.Target {
				t.Errorf("%s unpinned intrinsic interpretation: %+v", binding.Profile.ID, row)
			}
			delete(want, key)
		}
		if len(want) != 0 {
			t.Errorf("%s omitted intrinsic assemblies: %v", binding.Profile.ID, want)
		}
	}
}

func TestPackageCatalogDigestIncludesIntrinsicInterpretation(t *testing.T) {
	rules, profiles, err := buildPackageAssessmentDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	original, err := validation.NewPackageCatalog(rules, profiles)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := original.Profile(ProfileResourceGraph)
	slices.Reverse(rules)
	slices.Reverse(profiles)
	for i := range profiles {
		slices.Reverse(profiles[i].Intrinsic)
	}
	reordered, err := validation.NewPackageCatalog(rules, profiles)
	if err != nil {
		t.Fatal(err)
	}
	q, _ := reordered.Profile(ProfileResourceGraph)
	if p.CatalogDigest != q.CatalogDigest {
		t.Fatal("construction order changed canonical digest")
	}
	profiles[0].Intrinsic[0].CatalogDigest = strings.Repeat("a", 64)
	changed, err := validation.NewPackageCatalog(rules, profiles)
	if err != nil {
		t.Fatal(err)
	}
	q, _ = changed.Profile(ProfileResourceGraph)
	if p.CatalogDigest == q.CatalogDigest {
		t.Fatal("intrinsic interpretation missing from package digest")
	}
	// Every build owns its metadata. Editing one source table cannot reconfigure
	// the compiled singleton or a later freshly built catalog.
	fresh, err := buildPackageAssessmentCatalog()
	if err != nil {
		t.Fatal(err)
	}
	q, _ = fresh.Profile(ProfileResourceGraph)
	if p.CatalogDigest != q.CatalogDigest {
		t.Fatal("fresh catalog affected by caller metadata changes")
	}
}

func TestPackageProfilesRejectWrongEntryPointProfiles(t *testing.T) {
	c, err := newPackageAssessmentCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []validation.ProfileID{"", "unknown", ProfileSourceLinks, ProfileModelLinks, ProfileSourceContainer, ProfileSourceSounds, ProfileSourceContext, ProfileModelContext} {
		if _, ok := c.Profile(id); ok {
			t.Errorf("nonpackage profile accepted: %s", id)
		}
	}
	for _, id := range []validation.ProfileID{ProfileResourceGraph, ProfileMXLSource, ProfileMXLModel} {
		if _, err := sourceAssessmentEngineForProfile(id); !errors.Is(err, validation.ErrInvalidConfiguration) {
			t.Errorf("package profile accepted by source adapter: %s %v", id, err)
		}
		if _, err := modelAssessmentEngineForProfile(id); !errors.Is(err, validation.ErrInvalidConfiguration) {
			t.Errorf("package profile accepted by model adapter: %s %v", id, err)
		}
	}
}

func TestPackageProfilesDoNotMutateFrozenEight(t *testing.T) {
	if _, err := newPackageAssessmentCatalog(); err != nil {
		t.Fatal(err)
	}
	// Original baseline includes exact profile metadata, complete report content,
	// counts and original JSON field shape, rather than only digest equality.
	TestAuxiliaryFrozenEightProfiles(t)
}

func TestPackageIntrinsicSourceDiscoveryAvailableForEveryAssembly(t *testing.T) {
	// The package pairing consumer requires a real source discovery projection
	// from every permitted intrinsic assembly, including auxiliaries. A pinned
	// digest alone must not hide an omitted prerequisite provider.
	rows, err := packageIntrinsicProfiles()
	if err != nil {
		t.Fatal(err)
	}
	sources := map[validation.Assembly]string{
		validation.AssemblyMusicXML:  resourceScore(""),
		validation.AssemblyOpus:      "<opus></opus>",
		validation.AssemblyContainer: `<container><rootfiles><rootfile full-path="score.musicxml"/></rootfiles></container>`,
		validation.AssemblySounds:    `<sounds><sound id="s"><any/></sound></sounds>`,
	}
	for _, row := range rows {
		if row.Target != validation.TargetSource {
			continue
		}
		t.Run(string(row.Assembly), func(t *testing.T) {
			source := strings.Replace(sources[row.Assembly], ">", ` xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="urn:a a.xsd trailing">`, 1)
			report, err := InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: row.Profile})
			if err != nil || !report.Satisfied() {
				t.Fatalf("odd source hint changed intrinsic datatype validity: %v %+v", err, report)
			}
			found := false
			for _, fact := range report.Context {
				if fact.Resource == nil || fact.Resource.SchemaHints == nil {
					continue
				}
				h := fact.Resource.SchemaHints
				if h.Raw.State == validation.EvidenceKnown && h.Raw.Value == "urn:a a.xsd trailing" {
					found = true
					if h.Complete.State != validation.EvidenceKnown || h.Complete.Value || h.Complete.Reason != "schema-location-unmatched-token" || len(h.Pairs) != 1 || h.Trailing.State != validation.EvidenceKnown || h.Trailing.Value != "trailing" {
						t.Errorf("discovery evidence lost: %+v", h)
					}
				}
			}
			if !found {
				t.Error("selected intrinsic profile omitted source schema-hint discovery evidence")
			}
		})
	}
}
