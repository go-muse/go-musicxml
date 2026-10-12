package musicxml

import (
	"errors"
	"github.com/go-muse/go-musicxml/validation"
	"testing"
)

func TestTimelinePlaybackRequiresExactNavigationAndNoteRoles(t *testing.T) {
	for _, tc := range []struct{ owner, name string }{{"barline", "location"}, {"barline", "divisions"}, {"barline", "segno"}, {"barline", "coda"}, {"sound", "segno"}, {"sound", "coda"}, {"sound", "dacapo"}, {"sound", "dalsegno"}, {"sound", "tocoda"}, {"sound", "forward-repeat"}, {"sound", "fine"}, {"sound", "time-only"}, {"note", "time-only"}, {"note", "dynamics"}, {"note", "end-dynamics"}, {"note", "pizzicato"}} {
		t.Run(tc.owner+"/"+tc.name, func(t *testing.T) {
			schema := timelineContractVariantSchema(t, func(data *validation.SchemaData) {
				key := validation.SchemaName{Local: tc.owner}
				typ := *data.Types[key]
				c := *typ.Complex
				c.Attributes = nil
				found := false
				for _, a := range typ.Complex.Attributes {
					if a.Name.Local == tc.name {
						found = true
						continue
					}
					c.Attributes = append(c.Attributes, a)
				}
				if !found {
					t.Fatal("fixture attribute missing")
				}
				typ.Complex = &c
				data.Types[key] = &typ
			})
			if _, err := validation.NewTimelinePlan(validation.TimelinePlanSpec{Schema: schema, PartOwners: timelineContractOwners(t, schema)}); !errors.Is(err, validation.ErrInvalidConfiguration) {
				t.Fatalf("accepted missing navigation/note role: %v", err)
			}
		})
	}
}
