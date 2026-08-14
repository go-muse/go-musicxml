package musicxml

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMXLPackageSyncResolvedOpus(t *testing.T) {
	t.Parallel()

	firstXML := []byte(
		`<score-partwise version="4.0">` +
			`<part-list></part-list>` +
			`</score-partwise>`,
	)
	secondXML := []byte(
		"<?xml version=\"1.0\"?>\n" +
			"<score-timewise>\n" +
			"  <part-list></part-list>\n" +
			"</score-timewise>\n",
	)
	appendixXML := []byte(
		`<opus>` +
			`<score xmlns:xlink="http://www.w3.org/1999/xlink" ` +
			`xlink:href="../scores/second.musicxml"/>` +
			`<opus-link xmlns:xlink="http://www.w3.org/1999/xlink" ` +
			`xlink:href="main.musicxml"/>` +
			`</opus>`,
	)
	image := []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0xff}

	value := &MXLPackage{
		Document: &OpusDocument{
			Title: stringPointer("Original collection"),
			Content: []OpusDocumentContent{
				{
					Score: &OpusScore{
						Href: "../scores/first.musicxml",
					},
				},
				{
					OpusLink: &OpusLink{
						Href: "appendix.musicxml",
					},
				},
				{
					Score: &OpusScore{
						Href: "../scores/first.musicxml",
					},
				},
			},
		},
		RootFiles: []MXLRootFile{
			{
				FullPath:  "collections/main.musicxml",
				MediaType: musicXMLMIMEType,
			},
		},
		Resources: []MXLResource{
			{
				Path: "scores/first.musicxml",
				Data: firstXML,
			},
			{
				Path: "scores/second.musicxml",
				Data: secondXML,
			},
			{
				Path: "collections/appendix.musicxml",
				Data: appendixXML,
			},
			{
				Path: "images/cover.png",
				Data: image,
			},
		},
	}

	resolved, err := value.ResolveOpus()
	require.NoError(t, err)

	first, ok := resolved.Content[0].Score.Target.(*ScorePartwise)
	require.True(t, ok)
	first.MovementTitle = stringPointer("Edited first score")
	assert.Same(t, first, resolved.Content[2].Score.Target)

	appendix := resolved.Content[1].OpusLink.Target
	appendix.Document.Title = stringPointer("Edited appendix")
	require.Len(t, appendix.Content, 2)
	assert.Same(t, resolved, appendix.Content[1].OpusLink.Target)
	value.Document.(*OpusDocument).Title = stringPointer(
		"Edited collection",
	)

	err = value.SyncResolvedOpus(resolved)
	require.NoError(t, err)

	assert.Equal(
		t,
		[]string{
			"scores/first.musicxml",
			"scores/second.musicxml",
			"collections/appendix.musicxml",
			"images/cover.png",
		},
		mxlResourcePaths(value.Resources),
	)
	assert.Equal(
		t,
		secondXML,
		mxlResourceData(value.Resources, "scores/second.musicxml"),
	)
	assert.Equal(
		t,
		image,
		mxlResourceData(value.Resources, "images/cover.png"),
	)

	firstDocument, err := Decode(bytes.NewReader(
		mxlResourceData(value.Resources, "scores/first.musicxml"),
	))
	require.NoError(t, err)
	firstRoundTripped, ok := firstDocument.(*ScorePartwise)
	require.True(t, ok)
	assert.Equal(
		t,
		stringPointer("Edited first score"),
		firstRoundTripped.MovementTitle,
	)

	appendixDocument, err := Decode(bytes.NewReader(
		mxlResourceData(
			value.Resources,
			"collections/appendix.musicxml",
		),
	))
	require.NoError(t, err)
	appendixRoundTripped, ok := appendixDocument.(*OpusDocument)
	require.True(t, ok)
	assert.Equal(
		t,
		stringPointer("Edited appendix"),
		appendixRoundTripped.Title,
	)

	resourcesAfterFirstSync := cloneMXLResources(value.Resources)
	err = value.SyncResolvedOpus(resolved)
	require.NoError(t, err)
	assert.Equal(t, resourcesAfterFirstSync, value.Resources)

	var encoded bytes.Buffer
	err = EncodeMXLPackage(&encoded, value)
	require.NoError(t, err)

	decoded, err := DecodeMXLPackage(bytes.NewReader(encoded.Bytes()))
	require.NoError(t, err)
	decodedRoot, ok := decoded.Document.(*OpusDocument)
	require.True(t, ok)
	assert.Equal(
		t,
		stringPointer("Edited collection"),
		decodedRoot.Title,
	)

	decodedResolved, err := decoded.ResolveOpus()
	require.NoError(t, err)
	decodedFirst, ok := decodedResolved.Content[0].
		Score.Target.(*ScorePartwise)
	require.True(t, ok)
	assert.Equal(
		t,
		stringPointer("Edited first score"),
		decodedFirst.MovementTitle,
	)
	assert.Equal(
		t,
		stringPointer("Edited appendix"),
		decodedResolved.Content[1].OpusLink.Target.Document.Title,
	)
}

func TestMXLPackageSyncResolvedOpusErrors(t *testing.T) {
	t.Parallel()

	t.Run("nil package", func(t *testing.T) {
		t.Parallel()

		err := (*MXLPackage)(nil).SyncResolvedOpus(
			&MXLResolvedOpus{},
		)

		assert.ErrorIs(t, err, ErrNilMXLPackage)
	})

	t.Run("nil resolved opus", func(t *testing.T) {
		t.Parallel()

		err := (&MXLPackage{}).SyncResolvedOpus(nil)

		assert.ErrorIs(t, err, ErrNilMXLResolvedOpus)
	})

	t.Run("different package", func(t *testing.T) {
		t.Parallel()

		value := newMXLOpusSyncTestPackage()
		resolved, err := value.ResolveOpus()
		require.NoError(t, err)

		err = newMXLOpusSyncTestPackage().
			SyncResolvedOpus(resolved)

		assert.ErrorIs(t, err, ErrMXLResolvedOpusMismatch)
	})

	t.Run("linked opus instead of root", func(t *testing.T) {
		t.Parallel()

		value := newMXLOpusSyncTestPackage()
		resolved, err := value.ResolveOpus()
		require.NoError(t, err)

		err = value.SyncResolvedOpus(
			resolved.Content[1].OpusLink.Target,
		)

		assert.ErrorIs(t, err, ErrMXLResolvedOpusMismatch)
	})

	t.Run("root path changed", func(t *testing.T) {
		t.Parallel()

		value := newMXLOpusSyncTestPackage()
		resolved, err := value.ResolveOpus()
		require.NoError(t, err)
		value.RootFiles[0].FullPath = "collections/renamed.musicxml"

		err = value.SyncResolvedOpus(resolved)

		assert.ErrorIs(t, err, ErrMXLResolvedOpusStale)
	})

	t.Run("root document replaced", func(t *testing.T) {
		t.Parallel()

		value := newMXLOpusSyncTestPackage()
		resolved, err := value.ResolveOpus()
		require.NoError(t, err)
		value.Document = &OpusDocument{}

		err = value.SyncResolvedOpus(resolved)

		assert.ErrorIs(t, err, ErrMXLResolvedOpusStale)
	})

	t.Run("linked resource changed", func(t *testing.T) {
		t.Parallel()

		value := newMXLOpusSyncTestPackage()
		resolved, err := value.ResolveOpus()
		require.NoError(t, err)
		value.Resources[0].Data = []byte(
			`<score-partwise version="external">` +
				`<part-list/>` +
				`</score-partwise>`,
		)

		err = value.SyncResolvedOpus(resolved)

		assert.ErrorIs(t, err, ErrMXLResolvedOpusStale)
	})

	t.Run("score target replaced", func(t *testing.T) {
		t.Parallel()

		value := newMXLOpusSyncTestPackage()
		resolved, err := value.ResolveOpus()
		require.NoError(t, err)
		resolved.Content[0].Score.Target = &ScorePartwise{}

		err = value.SyncResolvedOpus(resolved)

		assert.ErrorIs(t, err, ErrMXLResolvedOpusInvalid)
	})

	t.Run("link topology changed", func(t *testing.T) {
		t.Parallel()

		value := newMXLOpusSyncTestPackage()
		resolved, err := value.ResolveOpus()
		require.NoError(t, err)
		resolved.Content[0].Score.Link.Href = "other.musicxml"

		err = value.SyncResolvedOpus(resolved)

		assert.ErrorIs(t, err, ErrMXLResolvedOpusInvalid)
	})
}

func TestMXLPackageSyncResolvedOpusIsAtomic(t *testing.T) {
	t.Parallel()

	value := &MXLPackage{
		Document: &OpusDocument{
			Content: []OpusDocumentContent{
				{
					Score: &OpusScore{
						Href: "first.musicxml",
					},
				},
				{
					Score: &OpusScore{
						Href: "second.musicxml",
					},
				},
			},
		},
		Resources: []MXLResource{
			{
				Path: "first.musicxml",
				Data: []byte(
					`<score-partwise><part-list/></score-partwise>`,
				),
			},
			{
				Path: "second.musicxml",
				Data: []byte(
					`<score-partwise><part-list/></score-partwise>`,
				),
			},
		},
	}

	resolved, err := value.ResolveOpus()
	require.NoError(t, err)

	first, ok := resolved.Content[0].Score.Target.(*ScorePartwise)
	require.True(t, ok)
	first.MovementTitle = stringPointer("Valid edit")

	second, ok := resolved.Content[1].Score.Target.(*ScorePartwise)
	require.True(t, ok)
	second.PartList.Content = []PartListContent{{}}

	before := cloneMXLResources(value.Resources)
	err = value.SyncResolvedOpus(resolved)

	assert.ErrorContains(t, err, "must contain exactly one value")
	assert.Equal(t, before, value.Resources)
}

func newMXLOpusSyncTestPackage() *MXLPackage {
	return &MXLPackage{
		Document: &OpusDocument{
			Content: []OpusDocumentContent{
				{
					Score: &OpusScore{
						Href: "../scores/first.musicxml",
					},
				},
				{
					OpusLink: &OpusLink{
						Href: "appendix.musicxml",
					},
				},
			},
		},
		RootFiles: []MXLRootFile{
			{
				FullPath:  "collections/main.musicxml",
				MediaType: musicXMLMIMEType,
			},
		},
		Resources: []MXLResource{
			{
				Path: "scores/first.musicxml",
				Data: []byte(
					`<score-partwise><part-list/></score-partwise>`,
				),
			},
			{
				Path: "collections/appendix.musicxml",
				Data: []byte(`<opus/>`),
			},
		},
	}
}

func mxlResourcePaths(values []MXLResource) []string {
	result := make([]string, len(values))
	for index := range values {
		result[index] = values[index].Path
	}

	return result
}

func mxlResourceData(
	values []MXLResource,
	resourcePath string,
) []byte {
	for _, value := range values {
		if value.Path == resourcePath {
			return value.Data
		}
	}

	return nil
}

func cloneMXLResources(values []MXLResource) []MXLResource {
	result := make([]MXLResource, len(values))
	for index := range values {
		result[index] = MXLResource{
			Path: values[index].Path,
			Data: bytes.Clone(values[index].Data),
		}
	}

	return result
}
