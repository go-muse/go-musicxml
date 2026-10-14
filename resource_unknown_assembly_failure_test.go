package musicxml

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestResourceUnknownAssemblyPreservesSyntaxFailure(t *testing.T) {
	for _, media := range []string{"application/vnd.recordare.musicxml+xml", "application/vnd.recordare.musicxml+xml; version=4"} {
		for _, current := range []bool{false, true} {
			t.Run(media+map[bool]string{false: "/source", true: "/current"}[current], func(t *testing.T) {
				var bundle validation.AssessmentBundle
				var err error
				if current {
					doc, decodeErr := Decode(strings.NewReader(`<opus/>`))
					if decodeErr != nil {
						t.Fatal(decodeErr)
					}
					bundle, err = AssessMXLPackage(context.Background(), &MXLPackage{Document: doc, RootFiles: []MXLRootFile{{FullPath: "main.opus"}, {FullPath: "bad.xml", MediaType: media}}, Resources: []MXLResource{{Path: "bad.xml", Data: []byte("\x00")}}}, MXLAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileMXLModel}})
				} else {
					container := `<container><rootfiles><rootfile full-path="main.opus"/><rootfile full-path="bad.xml" media-type="` + media + `"/></rootfiles></container>`
					wire := assessmentZIPBytes([]assessmentZIPFixture{{name: mxlContainerPath, body: container}, {name: "main.opus", body: `<opus/>`}, {name: "bad.xml", body: "\x00"}}, nil, "")
					bundle, err = InspectMXL(context.Background(), bytes.NewReader(wire), packageGraphOptions())
				}
				var syntax *xml.SyntaxError
				if !errors.As(err, &syntax) || errors.Is(err, validation.ErrInvalidConfiguration) {
					t.Fatalf("syntax cause/configuration distinction: %v", err)
				}
				failure, ok := bundle.FailureReport()
				if !ok || failure.Syntax != validation.SyntaxMalformed || bundle.Satisfied() {
					t.Fatalf("failure lost: present=%t report=%+v", ok, failure)
				}
			})
		}
	}
}

func TestResourceUnknownAssemblyPrimaryAndExternal(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(fmt.Sprintf("external=%t", external), func(t *testing.T) {
			body := &resourceTestBody{Reader: strings.NewReader("\x00")}
			calls := 0
			options := LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraph}}
			source := "\x00"
			if external {
				source = `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="https://example.test/bad.xml"/></opus>`
				options.Dependencies = []ResourceDependency{ResourceDocuments}
				options.Resolver = graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
					calls++
					return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "bad", Representation: validation.ResourceSourceXML, Digest: fmt.Sprintf("%x", sha256.Sum256([]byte("\x00")))}, RetrievalURI: "https://example.test/bad.xml", Body: body}, nil
				})
			}
			bundle, err := InspectXMLWithResources(context.Background(), strings.NewReader(source), options)
			var syntax *xml.SyntaxError
			if !errors.As(err, &syntax) || errors.Is(err, validation.ErrInvalidConfiguration) || errors.Is(err, validation.ErrInvalidSession) {
				t.Fatalf("cause: %v", err)
			}
			failure, ok := bundle.FailureReport()
			if !ok || failure.Syntax != validation.SyntaxMalformed || bundle.Satisfied() {
				t.Fatalf("failure lost: %+v", failure)
			}
			if external && (calls != 1 || body.closed != 1) {
				t.Fatalf("resolver calls=%d closes=%d", calls, body.closed)
			}
		})
	}
}

func TestResourceUnknownAssemblyBeforeRoot(t *testing.T) {
	for _, source := range []string{"", "<", "<unsupported/>"} {
		t.Run(fmt.Sprintf("%q", source), func(t *testing.T) {
			bundle, err := InspectXMLWithResources(context.Background(), strings.NewReader(source), LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraph}})
			if errors.Is(err, validation.ErrInvalidConfiguration) || errors.Is(err, validation.ErrInvalidSession) {
				t.Fatalf("document input became adapter misuse: %v", err)
			}
			failure, ok := bundle.FailureReport()
			if !ok || !failure.Requested || bundle.Satisfied() {
				t.Fatalf("unavailable-root report lost: error=%v report=%+v", err, failure)
			}
		})
	}
}
