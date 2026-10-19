package musicxml

import (
	"github.com/go-muse/go-musicxml/validation"
	"testing"
)

func TestIndependentContextFrozenProfileDigests(t *testing.T) {
	for _, p := range []validation.ProfileID{ProfileModelXSD, ProfileModelLocal, ProfileModelTimeline, ProfileSourceXSDPreview, ProfileSourceLocal, ProfileSourceTimeline} {
		var e *validation.Engine
		var err error
		switch p {
		case ProfileSourceXSDPreview:
			e, err = buildSourceAssessmentEngine()
		case ProfileModelTimeline, ProfileSourceTimeline:
			e, err = buildTimelineAssessmentEngine()
		default:
			e, err = buildModelAssessmentEngine()
		}
		if err != nil {
			t.Fatal(err)
		}
		profile := catalogProfile(t, e, p)
		want := map[validation.ProfileID]string{
			ProfileModelXSD: decimalContractCatalogAfter, ProfileModelLocal: decimalContractCatalogAfter, ProfileSourceLocal: decimalContractCatalogAfter, ProfileSourceXSDPreview: "e6139cbdde4b7eeb977fab3c4c33213300c536a902b5bb313ffa3483ff880760", ProfileModelTimeline: "cf62be51ed16c6a08609a30d78ae513e22f4f3dd3398ce4213251dd80ac043ad", ProfileSourceTimeline: "cf62be51ed16c6a08609a30d78ae513e22f4f3dd3398ce4213251dd80ac043ad"}[p]
		want = migrateDecimalContractCatalog(want)
		if profile.CatalogDigest != want {
			t.Fatalf("frozen profile %s changed from independently measured d605a63 baseline: got %s want %s", p, profile.CatalogDigest, want)
		}
		t.Logf("%s %s %d", p, profile.CatalogDigest, len(profile.Required))
	}
}
