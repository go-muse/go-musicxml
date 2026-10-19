package musicxml

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// Every public entry point rejects a typed-nil reader or writer, such as a nil
// *os.File from a failed os.Open, with the same error as an untyped nil and
// without calling its methods.
func TestEntryPointsRejectTypedNilStreams(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	readers := map[string]io.Reader{"*os.File": (*os.File)(nil), "*bytes.Reader": (*bytes.Reader)(nil)}
	decoders := map[string]func(io.Reader) error{
		"Decode":                  func(r io.Reader) error { _, e := Decode(r); return e },
		"DecodeWithOptions":       func(r io.Reader) error { _, e := DecodeWithOptions(r, DecodeOptions{}); return e },
		"DecodeContext":           func(r io.Reader) error { _, e := DecodeContext(ctx, r, DecodeOptions{}); return e },
		"DecodeScorePartwise":     func(r io.Reader) error { _, e := DecodeScorePartwise(r); return e },
		"DecodeScoreTimewise":     func(r io.Reader) error { _, e := DecodeScoreTimewise(r); return e },
		"DecodeOpusDocument":      func(r io.Reader) error { _, e := DecodeOpusDocument(r); return e },
		"DecodeMXL":               func(r io.Reader) error { _, e := DecodeMXL(r); return e },
		"DecodeMXLContext":        func(r io.Reader) error { _, e := DecodeMXLContext(ctx, r, MXLOptions{}); return e },
		"DecodeMXLScorePartwise":  func(r io.Reader) error { _, e := DecodeMXLScorePartwise(r); return e },
		"DecodeMXLScoreTimewise":  func(r io.Reader) error { _, e := DecodeMXLScoreTimewise(r); return e },
		"DecodeMXLOpusDocument":   func(r io.Reader) error { _, e := DecodeMXLOpusDocument(r); return e },
		"DecodeMXLPackage":        func(r io.Reader) error { _, e := DecodeMXLPackage(r); return e },
		"DecodeMXLPackageContext": func(r io.Reader) error { _, e := DecodeMXLPackageContext(ctx, r, MXLOptions{}); return e },
		"DecodeAssessed":          func(r io.Reader) error { _, e := DecodeAssessed(ctx, r, DecodeAssessmentOptions{}); return e },
		"DecodeMXLAssessed":       func(r io.Reader) error { _, e := DecodeMXLAssessed(ctx, r, DecodeMXLAssessmentOptions{}); return e },
		"InspectXML": func(r io.Reader) error {
			_, e := InspectXML(ctx, r, AssessmentOptions{Profile: ProfileSourceXSDPreview})
			return e
		},
		"InspectXMLWithResources": func(r io.Reader) error {
			_, e := InspectXMLWithResources(ctx, r, LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraph}})
			return e
		},
		"InspectMXL": func(r io.Reader) error { _, e := InspectMXL(ctx, r, packageGraphOptions()); return e },
		"InspectXML dictionaries": func(r io.Reader) error {
			_, e := InspectXML(ctx, r, AssessmentOptions{Profile: ProfileSourceSemantics, Dictionaries: &DictionaryOptions{Source: BundledDictionarySource(), Selections: mandatoryDictionarySelections()}})
			return e
		},
		"InspectMXL dictionaries": func(r io.Reader) error {
			_, e := InspectMXL(ctx, r, MXLAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileMXLSourceDictionaries, Dictionaries: &DictionaryOptions{Source: BundledDictionarySource(), Selections: mandatoryDictionarySelections()}}})
			return e
		},
	}
	for name, decode := range decoders {
		for kind, reader := range readers {
			if err := decode(reader); !errors.Is(err, ErrNilReader) {
				t.Errorf("%s(%s): %v", name, kind, err)
			}
		}
	}

	doc := &ScorePartwise{}
	writers := map[string]func(io.Writer) error{
		"Encode":            func(w io.Writer) error { return Encode(w, doc) },
		"EncodeWithOptions": func(w io.Writer) error { return EncodeWithOptions(w, doc, EncodeOptions{}) },
		"EncodeMXL":         func(w io.Writer) error { return EncodeMXL(w, doc) },
		"EncodeMXLPackage":  func(w io.Writer) error { return EncodeMXLPackage(w, &MXLPackage{Document: doc}) },
	}
	for name, encode := range writers {
		if err := encode((*bytes.Buffer)(nil)); !errors.Is(err, ErrNilWriter) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := (validation.Report{}).Format((*bytes.Buffer)(nil)); !errors.Is(err, validation.ErrInvalidConfiguration) {
		t.Errorf("Report.Format: %v", err)
	}
}

// A resolver that returns a typed-nil body returns no body: the contract check
// rejects the known result, and no method is called on the nil value.
func TestResourceResolverTypedNilBodyIsAbsent(t *testing.T) {
	t.Parallel()

	root := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="child.opus"/></opus>`
	for name, body := range map[string]io.ReadCloser{"*os.File": (*os.File)(nil), "func": readCloserFunc(nil)} {
		resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
			return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "child", Representation: validation.ResourceSourceXML, Digest: strings.Repeat("a", 64)}, RetrievalURI: "https://e.test/child.opus", Body: body}, nil
		})
		_, err := InspectXMLWithResources(context.Background(), strings.NewReader(root), LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraph}, Identity: ResourceIdentity{CanonicalID: "root", RetrievalURI: "https://e.test/root.opus"}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}})
		if !errors.Is(err, validation.ErrInvalidConfiguration) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

type readCloserFunc func([]byte) (int, error)

func (f readCloserFunc) Read(p []byte) (int, error) { return f(p) }
func (f readCloserFunc) Close() error               { return nil }
