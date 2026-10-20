package musicxml

import (
	"context"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestResourceGraphIntrinsicProfileBundlePreservesLegacyChoices(t *testing.T) {
	want := resourceIntrinsicProfiles{
		source:    ProfileSourceLinks.ID(),
		model:     ProfileModelLinks.ID(),
		container: ProfileSourceContainer.ID(),
		sounds:    ProfileSourceSounds.ID(),
	}
	for _, profile := range []validation.ProfileID{ProfileResourceGraph.ID(), ProfileMXLSource.ID(), ProfileMXLModel.ID()} {
		g, err := startResourceGraph(context.Background(), LinkedAssessmentOptions{Graph: GraphOptions{Profile: GraphProfile(profile)}}.internal(), profile)
		if err != nil {
			t.Fatal(err)
		}
		if g.profiles != want {
			t.Errorf("%s profiles=%+v want %+v", profile, g.profiles, want)
		}
		_, _ = g.finish(nil)
	}
}
