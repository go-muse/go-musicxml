package musicxml

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func catalogProfile(t *testing.T, engine *validation.Engine, id validation.ProfileID) validation.Profile {
	t.Helper()
	session, err := engine.Begin(context.Background(), validation.RunOptions{Profile: id, DocumentID: "catalog-test"})
	if err != nil {
		t.Fatal(err)
	}
	report, _ := session.Finish(context.Background(), validation.Completion{Syntax: validation.SyntaxNotAssessed})
	return report.Profile
}
func TestBoundedCatalogExactProfilesAndSharedRules(t *testing.T) {
	engine, err := buildModelAssessmentEngine()
	if err != nil {
		t.Fatal(err)
	}
	model := catalogProfile(t, engine, ProfileModelLocal.ID())
	source := catalogProfile(t, engine, ProfileSourceLocal.ID())
	xsd := catalogProfile(t, engine, ProfileModelXSD.ID())
	if len(xsd.Required) != 14 || len(model.Required) != 36 || len(source.Required) != 34 {
		t.Fatalf("unexpected frozen obligation list: xsd%d model%d source%d", len(xsd.Required), len(model.Required), len(source.Required))
	}
	keys := func(p validation.Profile) []validation.RuleKey {
		var out []validation.RuleKey
		for _, key := range p.Required {
			if strings.HasPrefix(string(key.ID), "MX40-LOCAL-") {
				out = append(out, key)
			}
		}
		return out
	}
	if len(keys(model)) != 22 || !reflect.DeepEqual(keys(model), keys(source)) {
		t.Fatalf("local predicate revisions differ: %v %v", keys(model), keys(source))
	}
	if len(keys(xsd)) != 0 {
		t.Fatal("legacy XSD profile acquired local rules")
	}
	for _, p := range []validation.Profile{xsd, model, source} {
		if p.CatalogDigest == "" || p.RegistryCommit != assessmentRegistryCommit || len(p.SupportedRoots) != 3 || len(p.Support) != len(p.Required) {
			t.Errorf("incomplete profile identity: %+v", p)
		}
		for _, s := range p.Support {
			if s.State != validation.ImplementationAvailable {
				t.Errorf("unavailable obligation exposed as ready: %+v", s)
			}
		}
	}
	if strings.Contains(model.NormativeClosure, "spellings, MusicXML prose,") || !strings.Contains(model.NormativeClosure, "unselected MusicXML prose") {
		t.Fatal("local profile contradicts its selected prose scope")
	}
	if !strings.Contains(source.NormativeClosure, "unread external subsets") || !strings.Contains(model.NormativeClosure, "local-only") {
		t.Fatal("bounded profile limitations omitted")
	}
	preview, err := buildSourceAssessmentEngine()
	if err != nil {
		t.Fatal(err)
	}
	old := catalogProfile(t, preview, ProfileSourceXSDPreview.ID())
	if len(old.Required) != 12 || old.CatalogDigest == model.CatalogDigest {
		t.Fatal("preview catalog scope or isolation changed")
	}
	for _, k := range old.Required {
		if !strings.HasPrefix(string(k.ID), "MX40-SOURCE-XSD-") {
			t.Fatalf("preview acquired new rule %s", k.ID)
		}
	}
}
