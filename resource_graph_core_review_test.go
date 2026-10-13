package musicxml

import (
	"context"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestResourceGraphCoreReviewMalformedFailureIdentity(t *testing.T) {
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(`<opus><title>x</opus>`), LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraph}})
	if err == nil {
		t.Fatal("malformed primary accepted")
	}
	summary, ok := b.Failure()
	if !ok || summary.Terminal.Category != validation.CategorySyntax || summary.Terminal.Reason != "malformed-source" {
		t.Errorf("malformed primary identity=%+v exists=%t", summary, ok)
	}
}
func TestResourceGraphCoreReviewCurrentModelDiscoveryUnknown(t *testing.T) {
	for _, source := range []string{`<opus/>`, `<opus xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="opus.xsd"/>`} {
		doc, err := Decode(strings.NewReader(source))
		if err != nil {
			t.Fatal(err)
		}
		b, err := AssessModelWithResources(context.Background(), doc, LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraph}})
		if err != nil {
			t.Fatal(err)
		}
		if b.Satisfied() || b.PackageReport().AssessmentComplete {
			t.Error("discarded model xsi history fabricated as complete")
		}
		found := false
		for _, f := range b.PackageReport().Findings {
			if f.Rule.ID == "MX40-PACKAGE-schema-hint-pairs" {
				found = true
				if f.Outcome != validation.OutcomeUnknown || f.Reason != "schema-discovery-pairing-unavailable" {
					t.Errorf("discarded history must remain explicit unknown: %+v", f)
				}
			}
		}
		if !found {
			t.Error("missing current-model discovery limitation")
		}
	}
}

func TestResourceGraphCoreReviewUnknownAssemblyUsesFailureChannel(t *testing.T) {
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(`<unexpected-root/>`), LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraph}})
	if err == nil || b.Satisfied() || b.ResourceCount() != 0 {
		t.Errorf("unestablished assembly admitted as canonical: error=%v resources=%d", err, b.ResourceCount())
	}
	failure, ok := b.Failure()
	if !ok || failure.Target != validation.TargetSource || failure.Terminal.Reason != "unsupported-root" {
		t.Errorf("failed original summary lost: %+v exists=%t", failure, ok)
	}
	original, ok := b.FailureReport()
	if !ok || original.Target != validation.TargetSource || original.Terminal == nil || original.Terminal.Reason != "unsupported-root" {
		t.Errorf("failed original report lost: %+v exists=%t", original, ok)
	}
}
