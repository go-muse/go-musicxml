package musicxml

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func groupGraphScore(links, groups string) string {
	return `<score-partwise xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><part-list><score-part id="P1">` + links + `<part-name>P</part-name>` + groups + `</score-part></part-list><part id="P1"><measure number="1"/></part></score-partwise>`
}
func groupGraphRun(t *testing.T, model bool, source string, targets map[string]string) (validation.AssessmentBundle, error) {
	t.Helper()
	var bodies []*resourceTestBody
	r := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		if q.Document == nil {
			t.Fatal("unexpected non-document request")
		}
		data, ok := targets[q.Document.Href]
		if !ok {
			return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceUnknown, Reason: "fixture-target-unavailable"}, nil
		}
		body := &resourceTestBody{Reader: strings.NewReader(data)}
		bodies = append(bodies, body)
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: q.Document.Href, Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte(data)))}, RetrievalURI: "https://e.test/" + q.Document.Href, Body: body}, nil
	})
	opts := LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}, Identity: ResourceIdentity{RetrievalURI: "https://e.test/root.xml"}, Resolver: r, Dependencies: []ResourceDependency{ResourceDocuments}}
	var b validation.AssessmentBundle
	var err error
	if model {
		doc, e := Decode(strings.NewReader(source))
		if e != nil {
			t.Fatal(e)
		}
		b, err = AssessModelWithResources(context.Background(), doc, opts)
	} else {
		b, err = InspectXMLWithResources(context.Background(), strings.NewReader(source), opts)
	}
	for _, body := range bodies {
		if body.closed != 1 {
			t.Errorf("body close count %d", body.closed)
		}
	}
	return b, err
}
func groupGraphAdvice(b validation.AssessmentBundle) []validation.Finding {
	var out []validation.Finding
	for _, f := range b.PackageReport().Findings {
		if f.Rule.ID == "MX40-PACKAGE-group-match" {
			out = append(out, f)
		}
	}
	return out
}
func TestResourceGraphGroupMatchExactStringsRemainOptional(t *testing.T) {
	for _, model := range []bool{false, true} {
		for _, tc := range []struct {
			name, sourceValue, targetGroups string
			mismatch                        bool
		}{
			{"exact-space", " exact group ", `<group> exact group </group>`, false},
			{"space-significant", " exact group ", `<group>exact group</group>`, true},
			{"tab-significant", "exact&#x9;group", `<group>exact group</group>`, true},
			{"linefeed-exact", "exact&#xA;group", `<group>exact&#xA;group</group>`, false},
			{"case-significant", "Group", `<group>group</group>`, true},
			{"empty-exact", "", `<group/>`, false},
			{"no-target-group", "exact group", "", true},
		} {
			t.Run(fmt.Sprintf("model=%v/%s", model, tc.name), func(t *testing.T) {
				source := groupGraphScore(`<part-link xlink:href="part.xml"><group-link>`+tc.sourceValue+`</group-link></part-link>`, "")
				b, err := groupGraphRun(t, model, source, map[string]string{"part.xml": groupGraphScore("", tc.targetGroups)})
				if err != nil {
					t.Fatal(err)
				}
				advice := groupGraphAdvice(b)
				if tc.mismatch {
					if len(advice) != 1 || advice[0].Outcome != validation.OutcomeFail || advice[0].Reason != "group-link-string-mismatch" || advice[0].Category != validation.CategoryAdvisory {
						t.Fatalf("wrong optional mismatch: %+v", advice)
					}
				} else if len(advice) != 0 {
					t.Fatalf("exact match got adverse group advice: %+v", advice)
				}
				if b.PackageReport().Counts.Fail != 0 {
					t.Fatalf("optional advice changed mandatory failure counts: %+v", b.PackageReport())
				}
				_, primary, _ := b.ResourceReport(0)
				wantTarget := validation.TargetSource
				if model {
					wantTarget = validation.TargetModel
				}
				if primary.Target != wantTarget || primary.Counts.Fail != 0 {
					t.Fatal(primary.Target, primary.Counts)
				}
				if !model && !b.Satisfied() {
					t.Fatalf("optional mismatch prevented source acceptance: %+v", b.PackageReport())
				}
			})
		}
	}
}
func TestResourceGraphGroupMatchUsesExactLinkAndTarget(t *testing.T) {
	for _, model := range []bool{false, true} {
		t.Run(fmt.Sprint(model), func(t *testing.T) {
			source := groupGraphScore(`<part-link xlink:href="wrong.xml"><group-link>exact</group-link></part-link><part-link xlink:href="unrelated.xml"/>`, `<group>exact</group>`)
			targets := map[string]string{"wrong.xml": groupGraphScore("", `<group>other</group>`), "unrelated.xml": groupGraphScore("", `<group>exact</group>`)}
			b, err := groupGraphRun(t, model, source, targets)
			if err != nil {
				t.Fatal(err)
			}
			advice := groupGraphAdvice(b)
			if len(advice) != 1 || advice[0].Reason != "group-link-string-mismatch" {
				t.Fatalf("unrelated source/document group satisfied selected wrong target: %+v", advice)
			}
			source = groupGraphScore(`<part-link xlink:href="a.xml"><group-link>A</group-link></part-link><part-link xlink:href="b.xml"><group-link>B</group-link></part-link>`, "")
			b, err = groupGraphRun(t, model, source, map[string]string{"a.xml": groupGraphScore("", `<group>A</group>`), "b.xml": groupGraphScore("", `<group>B</group>`)})
			if err != nil || len(groupGraphAdvice(b)) != 0 {
				t.Fatalf("group links crossed owner scopes: %v %+v", err, groupGraphAdvice(b))
			}
		})
	}
}
func TestResourceGraphGroupMatchUnavailableEvidence(t *testing.T) {
	for _, model := range []bool{false, true} {
		for _, tc := range []struct {
			name, target, reason string
			outcome              validation.Outcome
		}{
			{"missing-target", "", "fixture-target-unavailable", validation.OutcomeUnknown},
			{"target-group-invalid-type", groupGraphScore("", `<group xsi:type="missingType">exact</group>`), "target-group-value-unavailable", validation.OutcomeUnknown},
		} {
			t.Run(fmt.Sprintf("model=%v/%s", model, tc.name), func(t *testing.T) {
				source := groupGraphScore(`<part-link xlink:href="part.xml"><group-link>exact</group-link></part-link>`, "")
				targets := map[string]string{}
				if tc.target != "" {
					targets["part.xml"] = tc.target
				}
				b, err := groupGraphRun(t, model, source, targets)
				if err != nil {
					t.Fatal(err)
				}
				advice := groupGraphAdvice(b)
				if len(advice) != 1 || advice[0].Outcome != tc.outcome || advice[0].Reason != validation.Code(tc.reason) || advice[0].Category != validation.CategoryAdvisory {
					t.Fatalf("unavailable evidence forged group match/mismatch: %+v", advice)
				}
				if b.Satisfied() {
					t.Fatal("required missing/invalid target accepted")
				}
			})
		}
	}
	source := groupGraphScore(`<part-link xlink:href="part.xml"><group-link xsi:type="missingType">exact</group-link></part-link>`, "")
	b, err := groupGraphRun(t, false, source, map[string]string{"part.xml": groupGraphScore("", `<group>exact</group>`)})
	if err != nil {
		t.Fatal(err)
	}
	advice := groupGraphAdvice(b)
	if len(advice) != 1 || advice[0].Outcome != validation.OutcomeBlocked || advice[0].Cause == nil || advice[0].Category != validation.CategoryAdvisory {
		t.Fatalf("invalid source group text was trusted: %+v", advice)
	}
}
