package musicxml

import (
	"bytes"
	"context"
	"errors"
	"github.com/go-muse/go-musicxml/validation"
	"io"
	"testing"
)

func TestIndependentMXLLocalNestedRepresentationCaps(t *testing.T) {
	inner := packageGraphBytes(`<opus/>`, "main.opus", "")
	root := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="inner.mxl"/></opus>`
	const cap int64 = 128
	for _, mode := range []string{"raw-local-link", "raw-nested-primary", "current-local-link", "xml-control"} {
		t.Run(mode, func(t *testing.T) {
			opts := packageGraphOptions()
			opts.MXL.MaxDocumentBytes = cap
			var b validation.AssessmentBundle
			var err error
			switch mode {
			case "raw-local-link":
				b, err = InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(root, "main.opus", "", assessmentZIPFixture{name: "inner.mxl", body: string(inner)})), opts)
			case "raw-nested-primary":
				b, err = InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(string(inner), "inner.mxl", mxlMIMEType)), opts)
			case "current-local-link":
				d, e := Decode(bytes.NewBufferString(root))
				if e != nil {
					t.Fatal(e)
				}
				opts.Graph.Profile = ProfileMXLModel
				b, err = AssessMXLPackage(context.Background(), &MXLPackage{Document: d, Resources: []MXLResource{{Path: "inner.mxl", Data: inner}}}, opts)
			case "xml-control":
				child := `<opus>` + string(bytes.Repeat([]byte{' '}, 200)) + `</opus>`
				b, err = InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(root, "main.opus", "", assessmentZIPFixture{name: "inner.mxl", body: child})), opts)
			}
			if mode == "xml-control" {
				if !errors.Is(err, ErrMXLTooLarge) || b.EnumerationComplete() {
					t.Fatalf("XML document cap escaped: %v enumeration=%v", err, b.EnumerationComplete())
				}
			} else if err != nil || !b.EnumerationComplete() {
				t.Fatalf("nested ZIP %d > XML cap%d with innerXML7 rejected: %v enumeration=%v", len(inner), cap, err, b.EnumerationComplete())
			}
		})
	}
}

func TestPackageClassificationPrefixMeteringAndFailures(t *testing.T) {
	boom := errors.New("prefix read")
	for _, tc := range []struct {
		name, body        string
		terminal          error
		document, archive int64
		wantCap           bool
	}{
		{"archive", "PK\x03\x04tail", nil, 2, 20, false},
		{"XML", "<opus/>", nil, 2, 20, true},
		{"truncated", "PK", nil, 2, 20, false},
		{"ambiguous", "PK\x01\x02", nil, 2, 20, true},
		{"bytes and error", "PK\x03\x04", boom, 2, 20, false},
		{"cap and error", "oops", boom, 2, 20, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := assessmentZIPAccount(t, validation.Budgets{})
			a := &mxlAssessmentArchive{ctx: context.Background(), account: account, limits: mxlLimits{resourcesSize: 100}}
			body := &resourceTestBody{Reader: &packageClassificationReader{data: []byte(tc.body), err: tc.terminal}}
			r := &mxlAssessmentExpandedReader{archive: a, body: body, limit: 20, automatic: true, documentLimit: tc.document, archiveLimit: tc.archive, declared: int64(len(tc.body))}
			got, err := io.ReadAll(r)
			if errors.Is(err, ErrMXLTooLarge) != tc.wantCap || (tc.terminal != nil && !errors.Is(err, tc.terminal)) {
				t.Fatalf("data %q err %v", got, err)
			}
			want := len(tc.body)
			if tc.wantCap {
				want = min(4, want)
			}
			used, _ := account.Usage(validation.BudgetExternalBytes)
			if used != int64(want) || a.expanded != int64(want) {
				t.Fatalf("prefix charge: external %d expanded %d want %d", used, a.expanded, want)
			}
			if err := r.Close(); err != nil || body.closed != 1 {
				t.Fatal(err, body.closed)
			}
		})
	}
}

type packageClassificationReader struct {
	data []byte
	err  error
}

func (r *packageClassificationReader) Read(p []byte) (int, error) {
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 {
		if r.err != nil {
			return n, r.err
		}
		return n, io.EOF
	}
	return n, nil
}

func TestPackageClassificationIgnoresDeclaredArchiveMedia(t *testing.T) {
	b, err := InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(`<opus/>`, "main.mxl", mxlMIMEType)), packageGraphOptions())
	if err != nil || !b.EnumerationComplete() {
		t.Fatalf("XML content must not become ZIP by MIME: %v", err)
	}
}
