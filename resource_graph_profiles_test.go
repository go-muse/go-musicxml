package musicxml

import (
	"context"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestResourceGraphIntrinsicProfileBundlePreservesLegacyChoices(t *testing.T) {
	want := resourceIntrinsicProfiles{
		source:    ProfileSourceLinks,
		model:     ProfileModelLinks,
		container: ProfileSourceContainer,
		sounds:    ProfileSourceSounds,
	}
	for _, profile := range []validation.ProfileID{ProfileResourceGraph, ProfileMXLSource, ProfileMXLModel} {
		g, err := startResourceGraph(context.Background(), LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: profile}}, profile)
		if err != nil {
			t.Fatal(err)
		}
		if g.profiles != want {
			t.Errorf("%s profiles=%+v want %+v", profile, g.profiles, want)
		}
		_, _ = g.finish(nil)
	}
}
