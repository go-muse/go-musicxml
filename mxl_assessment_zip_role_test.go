package musicxml

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestZIPReviewAutomaticResourceCapsPublic(t *testing.T) {
	child := `<opus>` + strings.Repeat(" ", 80) + `</opus>`
	container := `<container><rootfiles><rootfile full-path="main.opus"/><rootfile full-path="extra.opus"/></rootfiles></container>`
	wire := assessmentZIPBytes([]assessmentZIPFixture{{name: mxlContainerPath, body: container}, {name: "main.opus", body: `<opus/>`}, {name: "extra.opus", body: child}}, nil, "")
	for _, capName := range []string{"document", "resource"} {
		for _, delta := range []int64{0, -1} {
			o := packageGraphOptions()
			if capName == "document" {
				o.MXL.MaxDocumentBytes = int64(len(child)) + delta
			} else {
				o.MXL.MaxResourceBytes = int64(len(child)) + delta
			}
			bundle, err := InspectMXL(context.Background(), bytes.NewReader(wire), o)
			if delta == 0 && (err != nil || bundle.ResourceCount() != 3 || !bundle.EnumerationComplete()) {
				t.Fatalf("exact linked XML %s: %v", capName, err)
			}
			if delta < 0 && !errors.Is(err, ErrMXLTooLarge) {
				t.Fatalf("linked XML %s one over: %v", capName, err)
			}
		}
	}
}

func TestZIPReviewAutomaticNestedCapsPublic(t *testing.T) {
	inner := packageGraphBytes(`<opus/>`, "main.opus", "", assessmentZIPFixture{name: "pad.bin", body: strings.Repeat("x", 8192)})
	container := `<container><rootfiles><rootfile full-path="inner.mxl" media-type="` + mxlMIMEType + `"/></rootfiles></container>`
	outer := assessmentZIPBytes([]assessmentZIPFixture{{name: mxlMIMETypePath, body: mxlMIMEType}, {name: mxlContainerPath, body: container}, {name: "inner.mxl", body: string(inner), method: zip.Deflate}}, nil, "")
	if len(outer) >= len(inner)-1 {
		t.Fatal("fixture must keep outer archive below nested cap")
	}
	for _, capName := range []string{"archive", "resource"} {
		for _, delta := range []int64{0, -1} {
			o := packageGraphOptions()
			o.MXL.MaxDocumentBytes = 7
			if capName == "archive" {
				o.MXL.MaxArchiveBytes = int64(len(inner)) + delta
			} else {
				o.MXL.MaxResourceBytes = int64(len(inner)) + delta
			}
			bundle, err := InspectMXL(context.Background(), bytes.NewReader(outer), o)
			if delta == 0 && (err != nil || bundle.ResourceCount() != 3 || !bundle.EnumerationComplete()) {
				t.Fatalf("exact nested %s: %v", capName, err)
			}
			if delta < 0 && !errors.Is(err, ErrMXLTooLarge) {
				t.Fatalf("nested %s one over: %v", capName, err)
			}
		}
	}
}
