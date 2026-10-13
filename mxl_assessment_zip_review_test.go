package musicxml

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// These fixtures are record-level regressions, independent of archive/zip.Writer.
func reviewZIP64Envelope() []byte {
	b := make([]byte, 56+20+22)
	binary.LittleEndian.PutUint32(b, 0x06064b50)
	binary.LittleEndian.PutUint64(b[4:], 44)
	binary.LittleEndian.PutUint16(b[12:], 45)
	binary.LittleEndian.PutUint16(b[14:], 45)
	binary.LittleEndian.PutUint32(b[56:], 0x07064b50)
	binary.LittleEndian.PutUint32(b[72:], 1)
	binary.LittleEndian.PutUint32(b[76:], 0x06054b50)
	binary.LittleEndian.PutUint16(b[84:], math.MaxUint16)
	binary.LittleEndian.PutUint16(b[86:], math.MaxUint16)
	binary.LittleEndian.PutUint32(b[88:], math.MaxUint32)
	binary.LittleEndian.PutUint32(b[92:], math.MaxUint32)
	return b
}

func TestZIPReviewEOCDFeatureRequiresFraming(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func([]byte)
	}{
		{"volume", func(b []byte) { assessmentZIPSet16(b, 4, 1) }},
		{"count-sentinel", func(b []byte) { assessmentZIPSet16(b, 8, math.MaxUint16); assessmentZIPSet16(b, 10, math.MaxUint16) }},
		{"size-sentinel", func(b []byte) { assessmentZIPSet32(b, 12, math.MaxUint32) }},
		{"offset-sentinel", func(b []byte) { assessmentZIPSet32(b, 16, math.MaxUint32) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := make([]byte, 22)
			assessmentZIPSet32(fake, 0, 0x06054b50)
			tc.mutate(fake)
			b := assessmentZIPBytes([]assessmentZIPFixture{{name: "a", body: "x"}}, nil, string(fake))
			account := assessmentZIPAccount(t, validation.Budgets{})
			a, err := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
			defer assessmentZIPClose(t, a, account)
			var capability *mxlAssessmentZIPCapabilityError
			if !errors.As(err, &capability) || capability.feature != "ambiguous EOCD signature in comment" || a.reader != nil {
				t.Fatalf("comment bytes classified as envelope: %v", err)
			}
		})
	}
	for _, tc := range []struct {
		name    string
		data    []byte
		feature string
	}{
		{"zip64", reviewZIP64Envelope(), "ZIP64 envelope"},
		{"volume", func() []byte { b := assessmentZIPBytes(nil, nil, ""); assessmentZIPSet16(b, 4, 1); return b }(), "multi-volume archive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := assessmentZIPAccount(t, validation.Budgets{})
			a, err := openMXLAssessmentArchive(context.Background(), bytes.NewReader(tc.data), MXLOptions{}, account)
			defer assessmentZIPClose(t, a, account)
			var capability *mxlAssessmentZIPCapabilityError
			if !errors.As(err, &capability) || capability.feature != tc.feature || a.reader != nil {
				t.Fatalf("unsupported framed feature: %v", err)
			}
		})
	}
}

func TestZIPReviewLocatorBytesInCentralComment(t *testing.T) {
	comment := append([]byte("PK\x06\x07"), make([]byte, 16)...)
	b := auditZIPEncode([]auditZIPMember{{name: "a", body: []byte("payload"), comment: comment}}, nil)
	if _, err := zip.NewReader(bytes.NewReader(b), int64(len(b))); err != nil {
		t.Fatal(err)
	}
	account := assessmentZIPAccount(t, validation.Budgets{})
	a, err := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
	defer assessmentZIPClose(t, a, account)
	if err != nil {
		t.Fatal(err)
	}
	if a.entries[0].central.comment != string(comment) {
		t.Fatal("central comment changed")
	}
	if got, err := a.readEntry(0, mxlAssessmentPrimaryDocument); err != nil || string(got) != "payload" {
		t.Fatalf("read: %q %v", got, err)
	}
}

func TestZIPReviewFlagsRemainIndependent(t *testing.T) {
	for _, tc := range []struct {
		name           string
		local, central uint16
		kind           string
	}{
		{"central-EFS", 0, 0x800, ""}, {"local-EFS", 0x800, 0, ""},
		{"central-level", 0, 2, ""}, {"local-level", 4, 0, ""},
		{"central-unsupported", 0, 0x10, "capability"}, {"local-unsupported", 0x10, 0, "capability"},
		{"descriptor", 0, 8, "format"}, {"encryption", 1, 0, "format"}, {"mask", 0, 0x2000, "format"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := assessmentZIPBytes([]assessmentZIPFixture{{name: "caf\xc3\xa9.xml", body: "data", flags: tc.central, localFlags: &tc.local}}, nil, "")
			account := assessmentZIPAccount(t, validation.Budgets{})
			a, err := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
			defer assessmentZIPClose(t, a, account)
			var capability *mxlAssessmentZIPCapabilityError
			switch tc.kind {
			case "":
				if err != nil {
					t.Fatal(err)
				}
			case "capability":
				if !errors.As(err, &capability) || capability.feature != "unreviewed general-purpose ZIP flags" {
					t.Fatalf("unsupported flag classification: %v", err)
				}
			case "format":
				if !errors.Is(err, zip.ErrFormat) {
					t.Fatalf("framing mismatch: %v", err)
				}
			}
			if len(a.entries) != 1 || a.entries[0].local.flags != tc.local || a.entries[0].central.flags != tc.central || !a.entries[0].utf8Name {
				t.Fatal("independent byte/flag observations lost")
			}
		})
	}
}

func TestZIPReviewPathErrorIdentities(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entries []assessmentZIPFixture
		want    error
	}{
		{"unsafe", []assessmentZIPFixture{{name: "../a"}}, ErrMXLInvalidPath},
		{"duplicate", []assessmentZIPFixture{{name: "a"}, {name: "a"}}, ErrMXLDuplicateEntry},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := assessmentZIPBytes(tc.entries, nil, "")
			account := assessmentZIPAccount(t, validation.Budgets{})
			a, err := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
			defer assessmentZIPClose(t, a, account)
			var policy *mxlAssessmentZIPPolicyError
			if !errors.As(err, &policy) || !errors.Is(err, tc.want) || !errors.Is(err, ErrInvalidMXL) {
				t.Fatalf("path error identity: %v", err)
			}
		})
	}
}

func TestZIPReviewMalformedZIP64ChainIsNotFeatureEvidence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func([]byte)
	}{
		{"offset-overflow", func(b []byte) { binary.LittleEndian.PutUint64(b[64:], math.MaxUint64) }},
		{"short-record", func(b []byte) { binary.LittleEndian.PutUint64(b[4:], 43) }},
		{"directory-outside-record", func(b []byte) { binary.LittleEndian.PutUint64(b[48:], 1) }},
		{"directory-size-overflow", func(b []byte) { binary.LittleEndian.PutUint64(b[40:], math.MaxUint64) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := reviewZIP64Envelope()
			tc.mutate(b)
			account := assessmentZIPAccount(t, validation.Budgets{})
			a, err := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{}, account)
			defer assessmentZIPClose(t, a, account)
			if !errors.Is(err, zip.ErrFormat) || a.reader != nil {
				t.Fatalf("unframed ZIP64 chain: %v", err)
			}
		})
	}
}
