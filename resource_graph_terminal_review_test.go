package musicxml

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"testing"
)

func TestResourceGraphPackageQuotaHasNoLifecycleNoise(t *testing.T) {
	data := packageGraphBytes(`<opus/>`, "main.opus", "")
	for _, limit := range []int64{1, 2, 3} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			calls := 0
			resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
				calls++
				return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "archive", Representation: validation.ResourceRawMXL, Digest: fmt.Sprintf("%x", sha256.Sum256(data))}, RetrievalURI: "https://e.test/book.mxl", Body: &resourceTestBody{Reader: bytes.NewReader(data)}}, nil
			})
			source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="book.mxl"/></opus>`
			opts := LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraph, Budgets: validation.Budgets{MaxLinkedDocuments: limit}}, Identity: ResourceIdentity{RetrievalURI: "https://e.test/root.opus"}, Dependencies: []ResourceDependency{ResourceDocuments}, Resolver: resolver}
			b, err := InspectXMLWithResources(context.Background(), strings.NewReader(source), opts)
			if calls != 1 {
				t.Fatalf("calls=%d", calls)
			}
			if limit < 3 {
				if !errors.Is(err, validation.ErrResourceLimit) || errors.Is(err, validation.ErrInvalidSession) {
					t.Fatalf("err=%v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if limit < 3 {
				report := b.PackageReport()
				if b.Satisfied() || report.AssessmentComplete || report.Terminal == nil || report.Terminal.Category != validation.CategoryResource {
					t.Fatalf("lost terminal report: %+v", report)
				}
				var cause *validation.ResourceError
				if report.Terminal.Reason != "resource-limit" || !errors.As(err, &cause) || cause.Kind != validation.BudgetLinkedDocuments || cause.Used != limit || cause.Limit != limit {
					t.Fatalf("report=%+v cause=%+v", report.Terminal, cause)
				}
				if b.ResourceCount() < 1 {
					t.Fatal("lost intrinsic root report")
				}

			}
			_, measuredErr := finalQAMeasuredXML(t, context.Background(), source, opts)
			if errors.Is(measuredErr, validation.ErrResourceLimit) != (limit < 3) || errors.Is(measuredErr, validation.ErrInvalidSession) || calls != 2 {
				t.Fatalf("measured err=%v calls=%d", measuredErr, calls)
			}

		})
	}
}

func TestResourceGraphTerminalCancellationCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	data := packageGraphBytes(`<opus/>`, "main.opus", "")
	body := &resourceTestBody{Reader: bytes.NewReader(data)}
	resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		cancel()
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "archive", Representation: validation.ResourceRawMXL, Digest: fmt.Sprintf("%x", sha256.Sum256(data))}, RetrievalURI: "https://e.test/book.mxl", Body: body}, nil
	})
	b, err := finalQAMeasuredXML(t, ctx, `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="book.mxl"/></opus>`, LinkedAssessmentOptions{Graph: GraphOptions{Context: validation.ContextFamilyAll, Profile: ProfileResourceGraph}, Identity: ResourceIdentity{RetrievalURI: "https://e.test/root.opus"}, Dependencies: []ResourceDependency{ResourceDocuments}, Resolver: resolver})
	r := b.PackageReport()
	if !errors.Is(err, context.Canceled) || errors.Is(err, validation.ErrInvalidSession) || b.Satisfied() || r.AssessmentComplete || r.Terminal == nil || r.Terminal.Reason != "canceled" || body.closed != 1 {
		t.Fatalf("err=%v report=%+v closed=%d", err, r, body.closed)
	}
}
