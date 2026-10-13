package musicxml

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestPackageMetadataPublicEntryPointsInvokeSelectedRoles(t *testing.T) {
	for _, mode := range []string{"raw", "current", "source-graph", "model-graph"} {
		t.Run(mode, func(t *testing.T) {
			data := packageRoleData()
			roleCalls := 0
			resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
				if q.Roles != nil {
					roleCalls++
					return packageRoleReply(q), nil
				}
				if q.Document == nil {
					t.Fatal("unexpected request")
				}
				return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "role-package", Representation: validation.ResourceRawMXL, Digest: fmt.Sprintf("%x", sha256.Sum256(data))}, RetrievalURI: "https://e.test/book.mxl", Body: &resourceTestBody{Reader: bytes.NewReader(data)}}, nil
			})
			var b validation.AssessmentBundle
			var err error
			switch mode {
			case "raw":
				o := packageGraphOptions()
				o.Dependencies = []ResourceDependency{ResourceRoles}
				o.Resolver = resolver
				b, err = InspectMXL(context.Background(), bytes.NewReader(data), o)
			case "current":
				d, e := Decode(strings.NewReader(`<opus/>`))
				if e != nil {
					t.Fatal(e)
				}
				o := MXLAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileMXLModel}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceRoles}}
				b, err = AssessMXLPackage(context.Background(), &MXLPackage{Document: d, RootFiles: []MXLRootFile{{FullPath: "part.xml"}, {FullPath: "score.xml"}}, Resources: []MXLResource{{Path: "score.xml", Data: []byte(`<opus/>`)}}}, o)
			default:
				source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="https://e.test/book.mxl"/></opus>`
				o := LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraph}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments, ResourceRoles}}
				if mode == "source-graph" {
					b, err = InspectXMLWithResources(context.Background(), strings.NewReader(source), o)
				} else {
					d, e := Decode(strings.NewReader(source))
					if e != nil {
						t.Fatal(e)
					}
					b, err = AssessModelWithResources(context.Background(), d, o)
				}
			}
			if err != nil || roleCalls != 1 {
				t.Fatalf("calls%d err%v", roleCalls, err)
			}
			found := false
			for _, f := range b.PackageReport().Findings {
				found = found || f.Reason == "score-must-precede-parts" && f.Outcome == validation.OutcomeFail
			}
			if !found || b.Satisfied() {
				t.Fatalf("exact role failure absent: %+v", b.PackageReport().Findings)
			}
		})
	}
}
