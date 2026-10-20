package musicxml

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestResourceGraphCoreReviewRawMXLDiscoveryPairing(t *testing.T) {
	for _, tc := range []struct {
		name, hint string
		incomplete bool
	}{
		{"absent", "", false},
		{"empty", `xsi:schemaLocation=""`, false},
		{"even", `xsi:schemaLocation="urn:a a.xsd"`, false},
		{"no-namespace", `xsi:noNamespaceSchemaLocation="opus.xsd"`, false},
		{"odd", `xsi:schemaLocation="urn:a a.xsd extra"`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := packageGraphBytes(`<opus xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" `+tc.hint+`/>`, "main.opus", "")
			b, err := InspectMXL(context.Background(), bytes.NewReader(data), MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLSource}})
			if err != nil {
				t.Fatal(err)
			}
			r := b.PackageReport()
			if b.Satisfied() == tc.incomplete || !r.AssessmentComplete || r.Conformance != validation.ConformanceConformant || r.Counts.Fail != 0 || r.Counts.Unknown != 0 || r.Counts.Unsupported != 0 || r.Counts.Blocked != 0 {
				t.Errorf("discovery policy changed normative or readiness axes: satisfied=%t report=%+v", b.Satisfied(), r)
			}
			var row *validation.ContextResult
			for i := 0; i < b.ResourceCount(); i++ {
				_, intrinsic, ok := b.ResourceReport(i)
				if !ok || !intrinsic.Satisfied() {
					t.Errorf("discovery pairing changed intrinsic XSD validity: %+v", intrinsic)
				}
				for _, v := range intrinsic.Context {
					if v.Resource != nil && v.Resource.SchemaHints != nil && v.Resource.SchemaHints.Complete.State == validation.EvidenceKnown && !v.Resource.SchemaHints.Complete.Value {
						copy := v
						row = &copy
					}
				}
			}
			findings := 0
			for _, f := range r.Findings {
				if f.Rule.ID != "MX40-PACKAGE-schema-hint-pairs" {
					continue
				}
				findings++
				if f.Outcome != validation.OutcomeFail || f.Category != validation.CategoryCapability || f.Reason != "schema-location-unmatched-token" {
					t.Errorf("known incomplete pairing lost: %+v", f)
				}
				if row == nil || f.Cause == nil || *f.Cause != (validation.DecisionRef{Rule: row.Provider, Subject: row.Subject}) {
					t.Errorf("pairing cause does not identify resource row: finding=%+v row=%+v", f, row)
				}
			}
			want := 0
			if tc.incomplete {
				want = 1
			}
			if findings != want || r.BlockingProblems != uint64(want) {
				t.Errorf("pairing findings=%d blockers=%d, want %d", findings, r.BlockingProblems, want)
			}
		})
	}
}

func TestResourceGraphCoreReviewMalformedFailureIdentity(t *testing.T) {
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(`<opus><title>x</opus>`), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}})
	if err == nil {
		t.Fatal("malformed primary accepted")
	}
	summary, ok := b.Failure()
	if !ok || summary.Terminal.Category != validation.CategorySyntax || summary.Terminal.Reason != "malformed-source" {
		t.Errorf("malformed primary identity=%+v exists=%t", summary, ok)
	}
}

// TestResourceGraphCoreReviewCurrentModelHasNoSchemaHints assesses a current
// model as it would be encoded: whatever hints its source had, the model
// carries none, so schema-hint pairing does not apply and nothing is
// reconstructed.
func TestResourceGraphCoreReviewCurrentModelHasNoSchemaHints(t *testing.T) {
	for _, source := range []string{`<opus/>`, `<opus xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="opus.xsd"/>`, `<opus xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="urn:a a.xsd extra"/>`} {
		doc, err := Decode(strings.NewReader(source))
		if err != nil {
			t.Fatal(err)
		}
		b, err := AssessModelWithResources(context.Background(), doc, LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}})
		if err != nil {
			t.Fatal(err)
		}
		if !b.Satisfied() || !b.PackageReport().AssessmentComplete {
			t.Errorf("current model without hints not satisfied: %+v", b.PackageReport())
		}
		for _, f := range b.PackageReport().Findings {
			if f.Rule.ID == "MX40-PACKAGE-schema-hint-pairs" {
				t.Errorf("current model offered a pairing obligation: %+v", f)
			}
		}
		if b.PackageReport().Counts.NotApplicable == 0 {
			t.Error("the pairing was not evaluated as not applicable")
		}
	}
}

func TestResourceGraphCoreReviewUnknownAssemblyUsesFailureChannel(t *testing.T) {
	b, err := InspectXMLWithResources(context.Background(), strings.NewReader(`<unexpected-root/>`), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}})
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
