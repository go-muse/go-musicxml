package musicxml

import (
	"context"
	"io/fs"
	"strings"

	"github.com/go-muse/go-musicxml/internal/dictionarydata"
	"github.com/go-muse/go-musicxml/validation"
)

// BundledDictionarySource returns an offline source for the five existing pinned
// dictionary components: SMuFL 1.4 glyphnames and ranges, MusicXML 4.0 standard
// sounds, and the IPA 2015/Unicode 13 positive and representation bundles.
//
// Use it as DictionaryOptions.Source with explicit Selections for inspection or
// standalone assessment. The assessed loaders select their required components
// by default when the source is nil and selections are empty. Each Open returns
// an independent reader over immutable embedded bytes; no file access, network
// access or generation occurs at runtime. The required selections of this
// source, in their canonical order, are imported and verified once per process
// and shared by every call that selects exactly them; each call is charged the
// shared set's retained storage while it holds it. Any other selection is
// imported, accounted, verified and parsed on every call.
//
// Any binary referencing DecodeAssessed, DecodeMXLAssessed, InspectXML,
// AssessModel, InspectXMLWithResources, AssessModelWithResources, InspectMXL
// or AssessMXLPackage embeds all five components, even with custom-source
// options. Direct use of this source also retains all five. Distribute the
// five notice/license files listed in THIRD_PARTY_NOTICES.md, including the
// CC BY-SA and Unicode data notices.
//
// The IPA bundles retain their reviewed, bounded positive-witness authority;
// they are not the original publications or a complete IPA grammar. Data retain
// their separate upstream licenses; see THIRD_PARTY_NOTICES.md.
func BundledDictionarySource() DictionarySource { return bundledDictionarySource{} }

type bundledDictionarySource struct{}

func (bundledDictionarySource) Open(ctx context.Context, request DictionaryComponentRequest) (DictionaryComponentResult, error) {
	if ctx == nil {
		return DictionaryComponentResult{}, &validation.ConfigurationError{Field: "bundled dictionary context", Reason: "must not be nil"}
	}
	if err := ctx.Err(); err != nil {
		return DictionaryComponentResult{}, err
	}
	manifest, ok := validation.LookupDictionaryManifest(request.Manifest)
	if !ok || request.Component != manifest.Component {
		return DictionaryComponentResult{}, &validation.ConfigurationError{Field: "bundled dictionary request", Reason: "must match an exact supported manifest and component pin"}
	}
	raw, ok := dictionarydata.Component(request.Component.ID)
	if !ok {
		return DictionaryComponentResult{}, &validation.ConfigurationError{Field: "bundled dictionary component", Reason: "unsupported component"}
	}
	return DictionaryComponentResult{
		Request: request,
		State:   validation.EvidenceKnown,
		Body:    &bundledDictionaryBody{ctx: ctx, reader: *strings.NewReader(raw)},
	}, nil
}

type bundledDictionaryBody struct {
	ctx    context.Context
	reader strings.Reader
	closed bool
}

func (b *bundledDictionaryBody) Read(p []byte) (int, error) {
	if b.closed {
		return 0, fs.ErrClosed
	}
	if err := b.ctx.Err(); err != nil {
		return 0, err
	}
	return b.reader.Read(p)
}

func (b *bundledDictionaryBody) Close() error {
	b.closed = true
	b.reader.Reset("")
	return nil
}
