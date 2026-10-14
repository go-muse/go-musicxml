package musicxml

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func mimetypeHeaderScopeZIP(local, central, other string) []byte {
	return assessmentZIPBytes([]assessmentZIPFixture{
		{name: mxlMIMETypePath, body: mxlMIMEType, localExtra: local, centralExtra: central},
		{name: mxlContainerPath, body: `<container><rootfiles><rootfile full-path="main.opus"/></rootfiles></container>`},
		{name: "main.opus", body: `<opus/>`, localExtra: other, centralExtra: other},
	}, nil, "")
}

func TestPackageGraphMimetypeLocalHeaderExtraScope(t *testing.T) {
	const extra = "\xfe\xca\x00\x00"
	for _, tc := range []struct {
		name, local, central, other string
		fail                        bool
	}{
		{"neither", "", "", "", false},
		{"local-only", extra, "", "", true},
		{"central-only", "", extra, "", false},
		{"central-data", "", "\xfe\xca\x01\x00x", "", false},
		{"central-blocks", "", extra + "\xfe\xca\x01\x00x", "", false},
		{"both", extra, extra, "", true},
		{"unrelated-entry", "", "", extra, false},
		{"unrelated-uninterpreted-extra", "", "", "\xfe", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := InspectMXL(context.Background(), bytes.NewReader(mimetypeHeaderScopeZIP(tc.local, tc.central, tc.other)), packageGraphOptions())
			if err != nil {
				t.Fatal(err)
			}
			r := b.PackageReport()
			if !b.EnumerationComplete() || !r.AssessmentComplete || b.Satisfied() == tc.fail || (r.Counts.Fail != 0) != tc.fail {
				t.Fatalf("wrong header scope: %+v", r)
			}
			found := false
			for _, f := range r.Findings {
				if f.Rule.ID == "MX40-PACKAGE-mimetype-no-extra" {
					if f.Reason != "mimetype-local-extra-field" || f.Outcome != validation.OutcomeFail || f.Location.Entry != mxlMIMETypePath {
						t.Fatalf("wrong no-extra finding: %+v", f)
					}
					found = true
				}
			}
			if found != tc.fail {
				t.Fatalf("missing/extraneous no-extra finding: %+v", r.Findings)
			}
		})
	}
}

func TestPackageGraphMimetypeCentralExtraPreservesZIPFailures(t *testing.T) {
	const extra = "\xfe\xca\x00\x00"
	for _, tc := range []struct {
		name   string
		change func([]byte) []byte
		want   error
	}{
		{"truncated-central-name", func(b []byte) []byte { assessmentZIPSet16(b, assessmentZIPCentral(b)+28, 65535); return b }, zip.ErrFormat},
		{"truncated-central-extra", func(b []byte) []byte { assessmentZIPSet16(b, assessmentZIPCentral(b)+30, 65535); return b }, zip.ErrFormat},
		{"central-name-disagreement", func(b []byte) []byte { b[assessmentZIPCentral(b)+46] = 'M'; return b }, zip.ErrFormat},
		{"central-CRC-disagreement", func(b []byte) []byte { assessmentZIPSet32(b, assessmentZIPCentral(b)+16, 0); return b }, zip.ErrFormat},
		{"payload-CRC", func(b []byte) []byte { b[30+len(mxlMIMETypePath)+int(mxlZIPU16(b, 28))] ^= 1; return b }, zip.ErrChecksum},
		{"missing-directory", func(b []byte) []byte { return b[:len(b)-22] }, zip.ErrFormat},
	} {
		for _, local := range []string{"", extra} {
			name := tc.name + "/central-only"
			if local != "" {
				name = tc.name + "/both"
			}
			t.Run(name, func(t *testing.T) {
				data := tc.change(mimetypeHeaderScopeZIP(local, extra, ""))
				b, err := InspectMXL(context.Background(), bytes.NewReader(data), packageGraphOptions())
				if !errors.Is(err, tc.want) || b.Satisfied() || b.PackageReport().AssessmentComplete {
					t.Fatalf("independent ZIP failure accepted: err=%v report=%+v", err, b.PackageReport())
				}
				found := false
				for _, f := range b.PackageReport().Findings {
					if f.Rule.ID == "MX40-PACKAGE-mimetype-no-extra" && f.Outcome == validation.OutcomeFail && f.Reason == "mimetype-local-extra-field" {
						found = true
					}
				}
				if found != (local != "") {
					t.Fatalf("independent local proof lost/invented: %+v", b.PackageReport().Findings)
				}
			})
		}
	}
}

// Malformed extra payloads are a separate bounded transport capability. They
// cannot invalidate a proved zero local length or erase a known local defect.
func TestPackageGraphMimetypeCentralExtraFramingIsIndependent(t *testing.T) {
	const localExtra = "\xfe\xca\x00\x00"
	for _, central := range []string{"\xfe", "\xfe\xca", "\xfe\xca\x00", "\xfe\xca\x01\x00", localExtra + "\xfe"} {
		t.Run(fmt.Sprintf("%x", central), func(t *testing.T) {
			var reports []validation.Report
			for _, local := range []string{"", localExtra} {
				data := mimetypeHeaderScopeZIP(local, central, "")
				b, err := InspectMXL(context.Background(), bytes.NewReader(data), packageGraphOptions())
				var capability *mxlAssessmentZIPCapabilityError
				if !errors.As(err, &capability) || capability.feature != "unframed mimetype central extra field" || errors.Is(err, zip.ErrFormat) || b.Satisfied() || b.PackageReport().AssessmentComplete {
					t.Fatalf("central extra framing must stay capability-incomplete: err=%v counts=%+v findings=%+v", err, b.PackageReport().Counts, b.PackageReport().Findings)
				}
				r := b.PackageReport()
				found := false
				for _, f := range r.Findings {
					if f.Rule.ID == "MX40-PACKAGE-mimetype-no-extra" {
						if local == "" || f.Outcome != validation.OutcomeFail || f.Reason != "mimetype-local-extra-field" {
							t.Fatalf("framing masked exact no-extra result: %+v", f)
						}
						found = true
					}
				}
				if found != (local != "") || r.Counts.Fail != mimetypeHeaderScopeCount(local != "") {
					t.Fatalf("local proof lost or invented: counts=%+v findings=%+v", r.Counts, r.Findings)
				}
				reports = append(reports, r)
			}
			// Passes are deliberately not emitted as diagnostics. With identical
			// independent capability failures, only this predicate changes counts.
			want := reports[1].Counts
			want.Pass++
			want.Fail--
			if reports[0].Counts != want {
				t.Fatalf("local-zero no-extra pass missing: central=%+v both=%+v", reports[0].Counts, reports[1].Counts)
			}
		})
	}
}

func mimetypeHeaderScopeCount(v bool) uint64 {
	if v {
		return 1
	}
	return 0
}

func TestMXLAssessmentMimetypeCentralExtraFramingPreservesAssociation(t *testing.T) {
	for _, central := range []string{"\xfe", "\xfe\xca\x01\x00"} {
		account := assessmentZIPAccount(t, validation.Budgets{})
		a, err := openMXLAssessmentArchive(context.Background(), bytes.NewReader(mimetypeHeaderScopeZIP("", central, "")), MXLOptions{}, account)
		var capability *mxlAssessmentZIPCapabilityError
		if !errors.As(err, &capability) || a.ready || !a.inventoryComplete || len(a.entries) != 3 || !a.entries[0].associated || !a.entries[0].trustedIdentity || !a.entries[0].localObserved || a.entries[0].local.extra != "" || a.entries[0].central.extra != central {
			t.Errorf("exact header facts lost/promoted: err=%v archive=%+v", err, a)
		}
		assessmentZIPClose(t, a, account)
	}
}

func TestPackageGraphMimetypeCentralExtraPreservesStoredDuty(t *testing.T) {
	for _, tc := range []struct {
		name          string
		method, flags uint16
		reason        validation.Code
		capability    bool
	}{
		{"deflate", zip.Deflate, 0, "mimetype-local-compressed", false},
		{"encrypted", zip.Store, 1, "mimetype-local-encrypted", true},
		{"unsupported-method", 99, 0, "mimetype-local-compressed", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := assessmentZIPBytes([]assessmentZIPFixture{
				{name: mxlMIMETypePath, body: mxlMIMEType, method: tc.method, flags: tc.flags, centralExtra: "\xfe\xca\x00\x00"},
				{name: mxlContainerPath, body: `<container><rootfiles><rootfile full-path="main.opus"/></rootfiles></container>`},
				{name: "main.opus", body: `<opus/>`},
			}, nil, "")
			b, err := InspectMXL(context.Background(), bytes.NewReader(data), packageGraphOptions())
			var capability *mxlAssessmentZIPCapabilityError
			if tc.capability && !errors.As(err, &capability) || !tc.capability && err != nil {
				t.Fatal(err)
			}
			found := false
			for _, f := range b.PackageReport().Findings {
				if f.Rule.ID == "MX40-PACKAGE-mimetype-no-extra" {
					t.Fatalf("unrelated duty contaminated no-extra: %+v", f)
				}
				if f.Rule.ID == "MX40-PACKAGE-mimetype-stored" && f.Outcome == validation.OutcomeFail && f.Reason == tc.reason {
					found = true
				}
			}
			if !found || b.Satisfied() || b.PackageReport().Conformance != validation.ConformanceViolated {
				t.Fatalf("stored/encrypted duty lost: %+v", b.PackageReport().Findings)
			}
		})
	}
}
