package validation_test

import (
	"github.com/go-muse/go-musicxml/validation"
	"testing"
)

func TestCrossReviewUnplacedSoundRetainsTimelineCause(t *testing.T) {
	repeatAssessHeader(t, `<attributes><divisions>4</divisions></attributes><sound tempo="120"/>`, `<part-list><score-part id="P2"><part-name>Other</part-name></score-part></part-list>`, func(t *testing.T, r validation.Report) {
		for _, c := range r.Context {
			if c.Timing != nil && c.Timing.Kind == "sound" {
				p := c.Timing.Position
				if p.State != validation.EvidenceInvalid || p.Reason != "identity-missing" || p.Cause == nil {
					t.Errorf("position=%+v play=%+v part=%+v tempo=%+v", p, c.Timing.PlayComplete, c.Timing.Part, c.Timing.Tempo)
				}
				return
			}
		}
		t.Fatal("missing sound")
	})
}
