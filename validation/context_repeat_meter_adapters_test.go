package validation_test

import (
	"github.com/go-muse/go-musicxml/validation"
	"testing"
)

func TestRepeatAdaptersMeterLateInvalidSelector(t *testing.T) {
	repeatAssessBoth(t, `<attributes><divisions>1</divisions><time><beats>4</beats><beat-type>4</beat-type></time><staves>2</staves><measure-style><slash type="start"/></measure-style></attributes><note><rest/><duration>1</duration><staff>1</staff></note><attributes><time number="0"><beats>2</beats><beat-type>2</beat-type></time></attributes>`, func(t *testing.T, r validation.Report) {
		found := false
		for _, c := range r.Context {
			if c.Repeat == nil || c.Repeat.Kind != validation.RepeatStyleSlash {
				continue
			}
			found = true
			got := c.Repeat.Beat
			if got.State != validation.EvidenceKnown || got.Value.String() != "1" {
				t.Fatalf("late invalid selector contaminated earlier style %+v", got)
			}
		}
		if !found {
			t.Fatal("missing style")
		}
	})
}
