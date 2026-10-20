package musicxml

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestIndependentMXLCanonicalPinConsistency(t *testing.T) {
	for _, mode := range []string{"same", "revision", "digest", "revision-and-digest"} {
		t.Run(mode, func(t *testing.T) {
			source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="https://e.test/a.mxl"/><opus-link xlink:href="https://e.test/b.mxl"/></opus>`
			calls := 0
			var bodies []*resourceTestBody
			resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
				calls++
				root := `<opus/>`
				revision := "first"
				if calls == 2 {
					if mode == "revision" || mode == "revision-and-digest" {
						revision = "second"
					}
					if mode == "digest" || mode == "revision-and-digest" {
						root = `<opus><title>changed</title></opus>`
					}
				}
				data := packageGraphBytes(root, "main.opus", "")
				body := &resourceTestBody{Reader: bytes.NewReader(data)}
				bodies = append(bodies, body)
				return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "same-package", Revision: revision, Representation: validation.ResourceRawMXL, Digest: fmt.Sprintf("%x", sha256.Sum256(data))}, RetrievalURI: q.Document.Href, Body: body}, nil
			})
			b, err := InspectXMLWithResources(context.Background(), strings.NewReader(source), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}})
			for i, body := range bodies {
				if body.closed != 1 {
					t.Errorf("body %d close count %d", i, body.closed)
				}
			}
			if mode == "same" {
				if err != nil || !b.EnumerationComplete() {
					t.Fatalf("same verified package should reuse: err=%v enumeration=%v", err, b.EnumerationComplete())
				}
			} else if !errors.Is(err, validation.ErrInvalidConfiguration) {
				t.Fatalf("canonical %s churn admitted: err=%v resources=%d adoptions=%d enumeration=%v", mode, err, b.ResourceCount(), b.AdoptionCount(), b.EnumerationComplete())
			}
		})
	}
}

func TestIndependentMXLLinkDepthIsNotResetByPackage(t *testing.T) {
	opus := func(href string) string {
		return `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="` + href + `"/></opus>`
	}
	root := opus("https://e.test/a.opus")
	calls := 0
	var bodies []*resourceTestBody
	resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		calls++
		var data []byte
		representation := validation.ResourceSourceXML
		switch q.Document.Href {
		case "https://e.test/a.opus":
			data = []byte(opus("https://e.test/b.mxl"))
		case "https://e.test/b.mxl":
			data = packageGraphBytes(opus("https://e.test/c.opus"), "main.opus", "")
			representation = validation.ResourceRawMXL
		case "https://e.test/c.opus":
			data = []byte(opus("https://e.test/d.opus"))
		case "https://e.test/d.opus":
			data = []byte(`<opus/>`)
		default:
			t.Fatalf("unexpected request %+v", q)
		}
		body := &resourceTestBody{Reader: bytes.NewReader(data)}
		bodies = append(bodies, body)
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: q.Document.Href, Representation: representation, Digest: fmt.Sprintf("%x", sha256.Sum256(data))}, RetrievalURI: q.Document.Href, Body: body}, nil
	})
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(root), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph, Budgets: validation.Budgets{MaxDepth: 4}}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}})
	for _, body := range bodies {
		if body.closed != 1 {
			t.Error("close", body.closed)
		}
	}
	var re *validation.ResourceError
	if !errors.As(err, &re) || re.Kind != validation.BudgetDepth || b.EnumerationComplete() {
		t.Fatalf("5-document link chain with MXL hop bypassed depth4: err=%v calls=%d resources=%d enumeration=%v", err, calls, b.ResourceCount(), b.EnumerationComplete())
	}
}
