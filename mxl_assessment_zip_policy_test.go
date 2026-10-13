package musicxml

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestZIPReviewProcessingPolicyBridge(t *testing.T) {
	for _, tc := range []struct {
		name   string
		data   []byte
		legacy error
		reason string
	}{
		{"unsafe-path", assessmentZIPBytes([]assessmentZIPFixture{{name: "../a"}}, nil, ""), ErrMXLInvalidPath, "unsafe or noncanonical archive path"},
		{"duplicate-name", assessmentZIPBytes([]assessmentZIPFixture{{name: "a"}, {name: "a"}}, nil, ""), ErrMXLDuplicateEntry, "duplicate archive name"},
		{"symlink", func() []byte {
			b := assessmentZIPBytes([]assessmentZIPFixture{{name: "a"}}, nil, "")
			p := assessmentZIPCentral(b)
			assessmentZIPSet16(b, p+4, 3<<8|20)
			assessmentZIPSet32(b, p+38, 0120777<<16)
			return b
		}(), nil, "symbolic-link archive entry"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bundle, err := InspectMXL(context.Background(), bytes.NewReader(tc.data), packageGraphOptions())
			var policy *mxlAssessmentZIPPolicyError
			if !errors.Is(err, validation.ErrProcessingPolicy) || !errors.As(err, &policy) || policy.reason != tc.reason {
				t.Fatalf("policy identity/detail lost: %v", err)
			}
			if tc.legacy != nil && (!errors.Is(err, tc.legacy) || !errors.Is(err, ErrInvalidMXL)) {
				t.Fatalf("legacy path identity lost: %v", err)
			}
			report := bundle.PackageReport()
			if bundle.Satisfied() || report.AssessmentComplete || report.Conformance == validation.ConformanceViolated || report.Counts.Fail != 0 || report.Terminal == nil || report.Terminal.Category != validation.CategoryPolicy || report.Terminal.Reason != "processing-policy" {
				t.Fatalf("policy refusal became normative or lost terminal category: %+v", report)
			}
		})
	}
}

func TestZIPReviewNonPolicyErrorsStayUnmarked(t *testing.T) {
	for _, tc := range []struct {
		name   string
		data   []byte
		format bool
	}{
		{"malformed-envelope", []byte("not a ZIP archive"), true},
		{"unsupported-zip64", reviewZIP64Envelope(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bundle, err := InspectMXL(context.Background(), bytes.NewReader(tc.data), packageGraphOptions())
			if err == nil || errors.Is(err, validation.ErrProcessingPolicy) {
				t.Fatalf("non-policy error marked as policy: %v", err)
			}
			if tc.format && !errors.Is(err, zip.ErrFormat) {
				t.Fatalf("format identity lost: %v", err)
			}
			if !tc.format {
				var capability *mxlAssessmentZIPCapabilityError
				if !errors.As(err, &capability) {
					t.Fatalf("capability detail lost: %v", err)
				}
			}
			report := bundle.PackageReport()
			if report.Terminal == nil || report.Terminal.Category != validation.CategoryCapability || report.Terminal.Reason != "assessment-interrupted" {
				t.Fatalf("ordinary classification changed: %+v", report.Terminal)
			}
		})
	}
}

func TestZIPReviewPolicyJoinedCauseIdentities(t *testing.T) {
	sentinel := errors.New("transport detail")
	cause := errors.Join(ErrMXLInvalidPath, sentinel)
	original := &mxlAssessmentZIPPolicyError{reason: "refused test path", cause: cause}
	wrapped := &validation.IncompleteError{Reason: "transport-refused", Cause: original}
	var policy *mxlAssessmentZIPPolicyError
	if !errors.Is(wrapped, validation.ErrProcessingPolicy) || !errors.Is(wrapped, ErrMXLInvalidPath) || !errors.Is(wrapped, ErrInvalidMXL) || !errors.Is(wrapped, sentinel) || !errors.As(wrapped, &policy) || policy != original {
		t.Fatalf("joined policy bridge lost identity: %v", wrapped)
	}
}

func TestZIPReviewPolicyTerminalPrecedence(t *testing.T) {
	resource := &validation.ResourceError{Kind: validation.BudgetInputBytes, Limit: 1, Requested: 2}
	for _, tc := range []struct {
		name   string
		cause  error
		reason validation.Code
	}{
		{"canceled", context.Canceled, "canceled"},
		{"deadline", context.DeadlineExceeded, "deadline-exceeded"},
		{"resource", resource, "resource-limit"},
		{"resource-and-cancellation", errors.Join(context.Canceled, resource), "resource-limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, err := startResourceGraph(context.Background(), LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileMXLSource}}, ProfileMXLSource)
			if err != nil {
				t.Fatal(err)
			}
			policy := &mxlAssessmentZIPPolicyError{reason: "refused test path", cause: ErrMXLInvalidPath}
			bundle, err := g.finish(errors.Join(policy, tc.cause))
			report := bundle.PackageReport()
			if !errors.Is(err, validation.ErrProcessingPolicy) || !errors.Is(err, ErrMXLInvalidPath) || !errors.Is(err, tc.cause) {
				t.Fatalf("joined cause lost: %v", err)
			}
			if report.Terminal == nil || report.Terminal.Category != validation.CategoryResource || report.Terminal.Reason != tc.reason || report.AssessmentComplete || report.Conformance == validation.ConformanceViolated || report.Counts.Fail != 0 {
				t.Fatalf("policy overrode existing interruption: %+v", report.Terminal)
			}
		})
	}
}
