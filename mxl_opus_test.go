package musicxml

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMXLPackageResolveOpus(t *testing.T) {
	t.Parallel()

	root := &OpusDocument{
		Content: []OpusDocumentContent{
			{
				Score: &OpusScore{
					Href: "../scores/My%20Score.musicxml#first",
				},
			},
			{
				Opus: &OpusDocument{
					Content: []OpusDocumentContent{
						{
							OpusLink: &OpusLink{
								Href: "appendix.musicxml",
							},
						},
					},
				},
			},
			{
				OpusLink: &OpusLink{
					Href: "appendix.musicxml",
				},
			},
			{
				Score: &OpusScore{
					Href: "../scores/last.musicxml",
				},
			},
		},
	}
	value := &MXLPackage{
		Document: root,
		RootFiles: []MXLRootFile{
			{
				FullPath:  "collections/main.musicxml",
				MediaType: musicXMLMIMEType,
			},
		},
		Resources: []MXLResource{
			{
				Path: "scores/My Score.musicxml",
				Data: []byte(
					`<score-partwise><part-list/></score-partwise>`,
				),
			},
			{
				Path: "scores/last.musicxml",
				Data: []byte(
					`<score-timewise><part-list/></score-timewise>`,
				),
			},
			{
				Path: "collections/appendix.musicxml",
				Data: []byte(
					`<opus>` +
						`<score xmlns:xlink="http://www.w3.org/1999/xlink" ` +
						`xlink:href="../scores/last.musicxml"/>` +
						`<opus-link xmlns:xlink="http://www.w3.org/1999/xlink" ` +
						`xlink:href="main.musicxml"/>` +
						`</opus>`,
				),
			},
			{
				Path: "images/cover.png",
				Data: []byte("PNG"),
			},
		},
	}

	actual, err := value.ResolveOpus()
	require.NoError(t, err)

	assert.Equal(t, "collections/main.musicxml", actual.Path)
	assert.Same(t, root, actual.Document)
	require.Len(t, actual.Content, 4)

	firstScore := actual.Content[0].Score
	require.NotNil(t, firstScore)
	assert.Same(t, root.Content[0].Score, firstScore.Link)
	assert.Equal(t, "scores/My Score.musicxml", firstScore.Path)
	assert.Equal(t, "first", firstScore.Fragment)
	assert.IsType(t, &ScorePartwise{}, firstScore.Target)

	inline := actual.Content[1].Opus
	require.NotNil(t, inline)
	assert.Equal(t, actual.Path, inline.Path)
	assert.Same(t, root.Content[1].Opus, inline.Document)
	require.Len(t, inline.Content, 1)

	fromInline := inline.Content[0].OpusLink
	direct := actual.Content[2].OpusLink
	require.NotNil(t, fromInline)
	require.NotNil(t, direct)
	assert.Equal(t, "collections/appendix.musicxml", direct.Path)
	assert.Same(t, direct.Target, fromInline.Target)

	appendix := direct.Target
	require.Len(t, appendix.Content, 2)

	lastScore := appendix.Content[0].Score
	require.NotNil(t, lastScore)
	assert.Equal(t, "scores/last.musicxml", lastScore.Path)
	assert.IsType(t, &ScoreTimewise{}, lastScore.Target)

	repeatedScore := actual.Content[3].Score
	require.NotNil(t, repeatedScore)
	assert.Same(t, lastScore.Target, repeatedScore.Target)

	backToRoot := appendix.Content[1].OpusLink
	require.NotNil(t, backToRoot)
	assert.Same(t, actual, backToRoot.Target)

	assert.Equal(t, []byte("PNG"), value.Resources[3].Data)
}

func TestMXLPackageResolveOpusDefaultRootPath(t *testing.T) {
	t.Parallel()

	value := &MXLPackage{
		Document: &OpusDocument{
			Content: []OpusDocumentContent{
				{
					Score: &OpusScore{
						Href: "linked.musicxml",
					},
				},
			},
		},
		Resources: []MXLResource{
			{
				Path: "linked.musicxml",
				Data: []byte(
					`<score-partwise><part-list/></score-partwise>`,
				),
			},
		},
	}

	actual, err := value.ResolveOpus()
	require.NoError(t, err)

	assert.Equal(t, mxlRootFilePath, actual.Path)
	require.Len(t, actual.Content, 1)
	assert.Equal(t, "linked.musicxml", actual.Content[0].Score.Path)
	assert.Empty(t, value.RootFiles)
}

func TestDecodedMXLPackageResolveOpus(t *testing.T) {
	t.Parallel()

	input := makeMXLTestArchive(t, []mxlTestEntry{
		mxlTestContainerEntry(
			`<rootfile full-path="collections/main.musicxml" ` +
				`media-type="` + musicXMLMIMEType + `"/>`,
		),
		mxlTestFileEntry(
			"collections/main.musicxml",
			`<opus>`+
				`<score xmlns:xlink="http://www.w3.org/1999/xlink" `+
				`xlink:href="../scores/first.musicxml"/>`+
				`</opus>`,
		),
		mxlTestFileEntry(
			"scores/first.musicxml",
			`<score-partwise><part-list/></score-partwise>`,
		),
	})

	value, err := DecodeMXLPackage(bytes.NewReader(input))
	require.NoError(t, err)

	actual, err := value.ResolveOpus()
	require.NoError(t, err)

	assert.Equal(t, "collections/main.musicxml", actual.Path)
	require.Len(t, actual.Content, 1)
	assert.Equal(t, "scores/first.musicxml", actual.Content[0].Score.Path)
	assert.IsType(t, &ScorePartwise{}, actual.Content[0].Score.Target)
}

func TestMXLPackageResolveOpusErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		value       *MXLPackage
		wantErr     error
		wantSource  string
		wantHref    string
		wantTarget  string
		wantMessage string
	}{
		{
			name:    "nil package",
			wantErr: ErrNilMXLPackage,
		},
		{
			name:    "nil document",
			value:   &MXLPackage{},
			wantErr: ErrNilDocument,
		},
		{
			name: "typed nil document",
			value: &MXLPackage{
				Document: (*OpusDocument)(nil),
			},
			wantErr: ErrNilDocument,
		},
		{
			name: "score root",
			value: &MXLPackage{
				Document: &ScorePartwise{},
			},
			wantErr: ErrMXLNotOpus,
		},
		{
			name:    "cyclic opus model",
			value:   cyclicOpusPackage(),
			wantErr: ErrDocumentCycle,
		},
		{
			name: "excessively deep opus model",
			value: &MXLPackage{
				Document: nestedOpusDocument(
					maximumDocumentDepth + 1,
				),
			},
			wantErr: ErrDocumentTooDeep,
		},
		{
			name: "malformed URI",
			value: opusPackageWithLink(
				&OpusScore{Href: "%zz"},
				nil,
			),
			wantErr:    ErrMXLInvalidLink,
			wantSource: mxlRootFilePath,
			wantHref:   "%zz",
		},
		{
			name: "external URI",
			value: opusPackageWithLink(
				&OpusScore{Href: "https://example.com/score.musicxml"},
				nil,
			),
			wantErr:    ErrMXLExternalLink,
			wantSource: mxlRootFilePath,
			wantHref:   "https://example.com/score.musicxml",
		},
		{
			name: "query URI",
			value: opusPackageWithLink(
				&OpusScore{Href: "score.musicxml?version=1"},
				nil,
			),
			wantErr:    ErrMXLInvalidLink,
			wantSource: mxlRootFilePath,
			wantHref:   "score.musicxml?version=1",
		},
		{
			name: "path escapes archive",
			value: &MXLPackage{
				Document: &OpusDocument{
					Content: []OpusDocumentContent{
						{
							Score: &OpusScore{
								Href: "../../../score.musicxml",
							},
						},
					},
				},
				RootFiles: []MXLRootFile{
					{FullPath: "collections/main.musicxml"},
				},
			},
			wantErr:    ErrMXLInvalidPath,
			wantSource: "collections/main.musicxml",
			wantHref:   "../../../score.musicxml",
		},
		{
			name: "missing document",
			value: opusPackageWithLink(
				&OpusScore{Href: "missing.musicxml"},
				nil,
			),
			wantErr:    ErrMXLLinkedDocumentNotFound,
			wantSource: mxlRootFilePath,
			wantHref:   "missing.musicxml",
			wantTarget: "missing.musicxml",
		},
		{
			name: "invalid linked document",
			value: opusPackageWithLink(
				&OpusScore{Href: "linked.musicxml"},
				[]MXLResource{
					{
						Path: "linked.musicxml",
						Data: []byte("<not-musicxml/>"),
					},
				},
			),
			wantErr:    ErrMXLLinkedDocumentInvalid,
			wantSource: mxlRootFilePath,
			wantHref:   "linked.musicxml",
			wantTarget: "linked.musicxml",
		},
		{
			name: "score resolves to opus",
			value: opusPackageWithLink(
				&OpusScore{Href: "other.musicxml"},
				[]MXLResource{
					{
						Path: "other.musicxml",
						Data: []byte("<opus/>"),
					},
				},
			),
			wantErr:    ErrMXLLinkedDocumentType,
			wantSource: mxlRootFilePath,
			wantHref:   "other.musicxml",
			wantTarget: "other.musicxml",
		},
		{
			name: "opus link resolves to score",
			value: opusPackageWithOpusLink(
				"linked.musicxml",
				[]MXLResource{
					{
						Path: "linked.musicxml",
						Data: []byte(
							`<score-partwise><part-list/>` +
								`</score-partwise>`,
						),
					},
				},
			),
			wantErr:    ErrMXLLinkedDocumentType,
			wantSource: mxlRootFilePath,
			wantHref:   "linked.musicxml",
			wantTarget: "linked.musicxml",
		},
		{
			name: "empty content variant",
			value: &MXLPackage{
				Document: &OpusDocument{
					Content: []OpusDocumentContent{{}},
				},
			},
			wantMessage: "must contain exactly one value",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual, err := test.value.ResolveOpus()

			assert.Nil(t, actual)
			if test.wantMessage != "" {
				assert.ErrorContains(t, err, test.wantMessage)
				return
			}
			assert.ErrorIs(t, err, test.wantErr)

			if test.wantSource == "" {
				return
			}

			var linkErr *MXLLinkError
			require.ErrorAs(t, err, &linkErr)
			assert.Equal(t, test.wantSource, linkErr.SourcePath)
			assert.Equal(t, test.wantHref, linkErr.Href)
			assert.Equal(t, test.wantTarget, linkErr.TargetPath)
		})
	}
}

func cyclicOpusPackage() *MXLPackage {
	root := NewOpusDocument()
	root.AddOpus(root)

	return &MXLPackage{Document: root}
}

func opusPackageWithLink(
	link *OpusScore,
	resources []MXLResource,
) *MXLPackage {
	return &MXLPackage{
		Document: &OpusDocument{
			Content: []OpusDocumentContent{
				{Score: link},
			},
		},
		Resources: resources,
	}
}

func opusPackageWithOpusLink(
	href string,
	resources []MXLResource,
) *MXLPackage {
	return &MXLPackage{
		Document: &OpusDocument{
			Content: []OpusDocumentContent{
				{
					OpusLink: &OpusLink{
						Href: href,
					},
				},
			},
		},
		Resources: resources,
	}
}
