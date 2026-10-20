package musicxml

import (
	"context"
	"crypto/sha256"
	"errors"
	"github.com/go-muse/go-musicxml/validation"
	"io"
	"strings"
	"testing"
)

func TestResourceIntrinsicSameAccountSnapshot(t *testing.T) {
	ctx := context.Background()
	a := adapterAccount(t, validation.Budgets{})
	input := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="part.musicxml"/></opus>`
	cfg := adapterSourceConfig(t, a, "child-source", "sets/catalog.opus", ProfileSourceLinks.ID(), sourceBytesAccounted)
	got := assessResourceSource(ctx, strings.NewReader(input), cfg)
	if got.Err != nil || !got.Report.Satisfied() || got.Snapshot == nil || !got.DigestComplete || got.Digest != sha256.Sum256([]byte(input)) {
		t.Fatalf("%+v", got)
	}
	if got.Snapshot.Header().DocumentID != "child-source" {
		t.Fatal(got.Snapshot.Header())
	}
	bytes, _ := a.Usage(validation.BudgetInputBytes)
	if bytes != 0 {
		t.Fatal(bytes)
	}
	if err := got.Snapshot.Close(); err != nil {
		t.Fatal(err)
	}
	live, _ := a.Usage(validation.BudgetBufferedBytes)
	if live != 0 {
		t.Fatal(live)
	}
	doc, err := Decode(strings.NewReader(`<opus/>`))
	if err != nil {
		t.Fatal(err)
	}
	model := assessResourceModel(ctx, doc, adapterModelConfig(t, a, "child-model", "current.opus", ProfileModelLinks.ID()))
	if model.Err != nil || !model.Report.Satisfied() || model.Snapshot == nil || model.DigestComplete || model.Digest != ([32]byte{}) {
		t.Fatalf("%+v", model)
	}
	model.Snapshot.Close()
	live, _ = a.Usage(validation.BudgetBufferedBytes)
	if live != 0 {
		t.Fatal(live)
	}
}
func TestResourceIntrinsicPreservesKnownFailure(t *testing.T) {
	a := adapterAccount(t, validation.Budgets{})
	got := assessResourceSource(context.Background(), strings.NewReader(`<opus invalid="yes"/>`), adapterSourceConfig(t, a, "invalid-source", "", ProfileSourceLinks.ID(), sourceBytesPrimary))
	if got.Report.Conformance != validation.ConformanceViolated || got.Snapshot == nil {
		t.Fatalf("%+v", got)
	}
	got.Snapshot.Close()
	live, _ := a.Usage(validation.BudgetBufferedBytes)
	if live != 0 {
		t.Fatal(live)
	}
}

func TestResourcePrerequisitePublicInspectPreservesJoinedEOF(t *testing.T) {
	boom := errors.New("joined EOF transport")
	for _, profile := range []validation.ProfileID{ProfileSourceXSDPreview.ID(), ProfileSourceLocal.ID(), ProfileSourceTimeline.ID(), ProfileSourceContext.ID(), ProfileSourceLinks.ID()} {
		report, err := InspectXML(context.Background(), &resourceReadError{[]byte(`<opus/>`), errors.Join(io.EOF, boom)}, SourceOptions{Context: validation.ContextFamilyAll, Profile: SourceProfile(profile)})
		if !errors.Is(err, boom) || report.AssessmentComplete || report.Satisfied() {
			t.Fatalf("%s: %v %+v", profile, err, report)
		}
	}
}
