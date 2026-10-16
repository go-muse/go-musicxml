package musicxml

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Characterize the explicitly documented readiness limitation. The independent
// local-header rule is covered separately; a package-incomplete report must not
// imply that child XML or the full entry metadata was assessed. This fixture
// supplies no root metadata identity, so it retains no file metadata either.
func TestMimetypeUnframedCentralExtraStopsChildEvidence(t *testing.T) {
	for _, local := range []string{"", "\xfe\xca\x00\x00"} {
		b, err := InspectMXL(context.Background(), bytes.NewReader(mimetypeHeaderScopeZIP(local, "\xfe", "")), MXLAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileMXLSource}})
		var capability *mxlAssessmentZIPCapabilityError
		if !errors.As(err, &capability) || capability.feature != "unframed mimetype central extra field" {
			t.Fatalf("expected exact central framing capability: %v", err)
		}
		if b.EnumerationComplete() || b.PackageReport().AssessmentComplete || b.Satisfied() || b.ResourceCount() != 0 || b.AdoptionCount() != 0 || b.FileMetadataCount() != 0 {
			t.Fatalf("fabricated child completion: enumeration=%v report=%+v resources=%d adoptions=%d metadata=%d", b.EnumerationComplete(), b.PackageReport(), b.ResourceCount(), b.AdoptionCount(), b.FileMetadataCount())
		}
		wantFails := uint64(0)
		if local != "" {
			wantFails = 1
		}
		if b.PackageReport().Counts.Fail != wantFails {
			t.Fatalf("lost independent local proof: %+v", b.PackageReport().Counts)
		}
		for _, f := range b.PackageReport().Findings {
			if f.Outcome == validation.OutcomeFail && (f.Rule.ID != "MX40-PACKAGE-mimetype-no-extra" || f.Reason != "mimetype-local-extra-field") {
				t.Fatalf("invented central validity failure: %+v", f)
			}
		}
	}
}
