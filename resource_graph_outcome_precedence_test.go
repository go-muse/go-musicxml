package musicxml

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestResourceGraphSchemaHintSiblingPrecedence(t *testing.T) {
	odd := `<opus xsi:schemaLocation="urn:a a.xsd extra"/>`
	invalid := `<opus xsi:schemaLocation="urn:a %zz"/>`
	even := `<opus xsi:schemaLocation="urn:a a.xsd"/>`
	for _, tc := range []struct {
		name, children string
		odd            bool
	}{
		{"invalid-odd", invalid + odd, true},
		{"odd-invalid", odd + invalid, true},
		{"invalid-even", invalid + even, false},
		{"even-invalid", even + invalid, false},
		{"invalid-only", invalid, false},
	} {
		for _, mode := range []string{"source", "raw-package", "model", "current-package"} {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				source := `<opus xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` + tc.children + `</opus>`
				var b validation.AssessmentBundle
				var err error
				switch mode {
				case "source":
					b, err = InspectXMLWithResources(context.Background(), strings.NewReader(source), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}})
				case "raw-package":
					data := packageGraphBytes(source, "main.opus", "")
					b, err = InspectMXL(context.Background(), bytes.NewReader(data), MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLSource}})
				default:
					var doc Document
					doc, err = Decode(strings.NewReader(source))
					if err != nil {
						t.Fatal(err)
					}
					if mode == "model" {
						b, err = AssessModelWithResources(context.Background(), doc, LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}})
					} else {
						b, err = AssessMXLPackage(context.Background(), &MXLPackage{Document: doc, RootFiles: []MXLRootFile{{FullPath: "main.opus"}}}, MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLModel}})
					}
				}
				if err != nil {
					t.Fatal(err)
				}
				// A current model is assessed as it would be encoded: without
				// xsi attributes, so the pairing does not apply to it.
				var want validation.Outcome
				var reason validation.Code
				var cause *validation.DecisionRef
				original := mode == "source" || mode == "raw-package"
				if original {
					want, reason = validation.OutcomeBlocked, "schema-discovery-pairing-unavailable"
					if tc.odd {
						want, reason = validation.OutcomeFail, "schema-location-unmatched-token"
					}
					for i := 0; i < b.ResourceCount(); i++ {
						_, intrinsic, _ := b.ResourceReport(i)
						for _, row := range intrinsic.Context {
							if row.Resource == nil || row.Resource.SchemaHints == nil {
								continue
							}
							hint := row.Resource.SchemaHints.Complete
							if tc.odd && hint.State == validation.EvidenceKnown && !hint.Value {
								cause = &validation.DecisionRef{Rule: row.Provider, Subject: row.Subject}
							} else if !tc.odd && hint.State == validation.EvidenceInvalid && cause == nil {
								cause = hint.Cause
							}
						}
					}
					if cause == nil {
						t.Fatal("fixture did not retain the exact intrinsic hint evidence")
					}
				}
				found := 0
				for _, f := range b.PackageReport().Findings {
					if f.Rule.ID != "MX40-PACKAGE-schema-hint-pairs" {
						continue
					}
					found++
					if !original {
						t.Errorf("current model offered a pairing obligation: %+v", f)
					} else if f.Outcome != want || f.Reason != reason || f.Category != validation.CategoryCapability || !reflect.DeepEqual(f.Cause, cause) {
						t.Errorf("want %s/%s cause=%+v; got %+v", want, reason, cause, f)
					}
				}
				if original && (found != 1 || b.Satisfied() || !b.EnumerationComplete()) {
					t.Errorf("pairing obligation lost: findings=%d satisfied=%t enumeration=%t", found, b.Satisfied(), b.EnumerationComplete())
				}
				if !original && (found != 0 || !b.EnumerationComplete()) {
					t.Errorf("current model pairing: findings=%d enumeration=%t", found, b.EnumerationComplete())
				}
				// The malformed sibling independently violates intrinsic XSD. The
				// discovery policy itself must not invent a normative package failure.
				r := b.PackageReport()
				if r.Counts.Fail != 0 || r.Conformance == validation.ConformanceViolated || r.AssessmentComplete != (!original || tc.odd) {
					t.Errorf("discovery changed package conformance/completion: %+v", r)
				}
			})
		}
	}
}

func TestResourceGraphDirectNoProviderAssociationReason(t *testing.T) {
	for _, mode := range []string{"source", "model", "raw-package", "current-package"} {
		for _, href := range []string{"missing.xml", "https://example.test/part.xml", "direct.xml"} {
			for _, documents := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/documents=%t", mode, href, documents), func(t *testing.T) {
					source := indirectionContextInstrumentScore("P1", "I1", href, "")
					dependencies := []ResourceDependency(nil)
					if documents {
						dependencies = []ResourceDependency{ResourceDocuments}
					}
					var b validation.AssessmentBundle
					var err error
					switch mode {
					case "source":
						b, err = InspectXMLWithResources(context.Background(), strings.NewReader(source), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}, Identity: ResourceIdentity{RetrievalURI: "https://example.test/main.xml"}, Dependencies: dependencies})
					case "raw-package":
						data := packageGraphBytes(source, "main.musicxml", "", assessmentZIPFixture{name: "direct.xml", body: resourceScore("")})
						b, err = InspectMXL(context.Background(), bytes.NewReader(data), MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLSource}, Dependencies: dependencies})
					default:
						var doc Document
						doc, err = Decode(strings.NewReader(source))
						if err != nil {
							t.Fatal(err)
						}
						if mode == "model" {
							b, err = AssessModelWithResources(context.Background(), doc, LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}, Identity: ResourceIdentity{RetrievalURI: "https://example.test/main.xml"}, Dependencies: dependencies})
						} else {
							b, err = AssessMXLPackage(context.Background(), &MXLPackage{Document: doc, RootFiles: []MXLRootFile{{FullPath: "main.musicxml"}}, Resources: []MXLResource{{Path: "direct.xml", Data: []byte(resourceScore(""))}}}, MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLModel}, Dependencies: dependencies})
						}
					}
					if err != nil {
						t.Fatal(err)
					}
					association, absentTarget := 0, 0
					for _, f := range b.PackageReport().Findings {
						if f.Rule.ID == "MX40-PACKAGE-instrument-association" {
							association++
							if f.Outcome != validation.OutcomeUnknown || f.Reason != "instrument-association-evidence-unavailable" || f.Cause != nil {
								t.Errorf("direct no-provider association vocabulary changed: %+v", f)
							}
						}
						if f.Rule.ID == "MX40-PACKAGE-link-target" && f.Outcome == validation.OutcomeFail && f.Reason == "linked-resource-authoritatively-absent" {
							absentTarget++
						}
					}
					wantAbsent := 0
					if (mode == "raw-package" || mode == "current-package") && href == "missing.xml" {
						wantAbsent = 1
					}
					if association != 1 || absentTarget != wantAbsent || b.Satisfied() || !b.EnumerationComplete() {
						t.Errorf("direct obligations changed: association=%d absent=%d want=%d satisfied=%t enumeration=%t", association, absentTarget, wantAbsent, b.Satisfied(), b.EnumerationComplete())
					}
				})
			}
		}
	}
}
