package musicxml

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"testing"
)

func TestFinalQAOpusScoreTargetKinds(t *testing.T) {
	timewise := `<score-timewise><part-list><score-part id="P1"><part-name>P</part-name></score-part></part-list><measure number="1"><part id="P1"/></measure></score-timewise>`
	for _, surface := range []string{"xml", "model", "raw-mxl", "current-mxl"} {
		for _, tc := range []struct {
			name, body string
			wrong      bool
		}{{"partwise", resourceScore(""), false}, {"timewise", timewise, false}, {"opus", `<opus/>`, true}} {
			t.Run(surface+"/"+tc.name, func(t *testing.T) {
				calls := 0
				bodies := []*resourceTestBody{}
				r := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
					calls++
					body := &resourceTestBody{Reader: strings.NewReader(tc.body)}
					bodies = append(bodies, body)
					return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "score-target", Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(tc.body)))}, RetrievalURI: "https://final-qa.test/target.musicxml", Body: body}, nil
				})
				source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="https://final-qa.test/target.musicxml"/></opus>`
				b, e := finalQARun(t, surface, source, context.Background(), finalQAOptions(r))
				if e != nil || calls != 1 || !b.EnumerationComplete() || finalQAWrongKind(b) != tc.wrong {
					t.Fatalf("target kind error=%v calls=%d findings=%+v", e, calls, b.PackageReport().Findings)
				}
				for _, body := range bodies {
					if body.closed != 1 {
						t.Fatal(body.closed)
					}
				}
				if tc.wrong {
					if b.PackageReport().Conformance != validation.ConformanceViolated || b.Satisfied() {
						t.Fatal("known wrong root did not reject")
					}
				} else {
					for _, f := range b.PackageReport().Findings {
						if f.Rule.ID == "MX40-PACKAGE-link-target" {
							t.Fatalf("supported target lost %+v", f)
						}
					}
				}
			})
		}
	}
}
