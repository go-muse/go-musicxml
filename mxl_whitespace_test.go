package musicxml

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMXLRootFileWhitespaceNormalization(t *testing.T) {
	t.Parallel()

	input := makeMXLTestArchive(t, []mxlTestEntry{
		mxlTestContainerEntry(
			`<rootfile full-path=" &#x9;scores/main&#x9; &#xD;&#xA;score.musicxml &#xA;" ` +
				`media-type=" &#x9;` + musicXMLMIMEType + `&#xD; "/>` +
				`<rootfile full-path=" &#xA;alternate/score.pdf &#x9;" media-type=" application/pdf "/>`,
		),
		mxlTestFileEntry("scores/main score.musicxml", `<score-partwise/>`),
		mxlTestFileEntry("images/ cover  .png ", "PNG"),
		mxlTestFileEntry("alternate/score.pdf", "PDF"),
		mxlTestFileEntry("metadata/\tname\u00a0", "metadata"),
	})

	_, err := DecodeMXL(bytes.NewReader(input))
	require.NoError(t, err)
	value, err := DecodeMXLPackage(bytes.NewReader(input))
	require.NoError(t, err)
	assert.Equal(t, []MXLRootFile{
		{FullPath: "scores/main score.musicxml", MediaType: musicXMLMIMEType},
		{FullPath: "alternate/score.pdf", MediaType: "application/pdf"},
	}, value.RootFiles)
	assert.Equal(t, []MXLResource{
		{Path: "images/ cover  .png ", Data: []byte("PNG")},
		{Path: "alternate/score.pdf", Data: []byte("PDF")},
		{Path: "metadata/\tname\u00a0", Data: []byte("metadata")},
	}, value.Resources)

	var encoded bytes.Buffer
	require.NoError(t, EncodeMXLPackage(&encoded, value))
	roundTripped, err := DecodeMXLPackage(bytes.NewReader(encoded.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, value, roundTripped)
}

func TestMXLRootFileWhitespaceValidation(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		rootFile string
		entry    string
		wantErr  error
	}{
		{"empty", " &#x9; ", "score.musicxml", ErrMXLInvalidPath},
		{"parent path", " ../score.musicxml ", "score.musicxml", ErrMXLInvalidPath},
		{"reserved path", " META-INF/container.xml ", "score.musicxml", ErrMXLInvalidPath},
		{"non-XML whitespace", "&#xA0;score.musicxml&#xA0;", "score.musicxml", ErrMXLRootFileNotFound},
		{"literal ZIP spaces", "score  name.musicxml", "score  name.musicxml", ErrMXLRootFileNotFound},
		{"literal ZIP padding", " score.musicxml ", " score.musicxml ", ErrMXLRootFileNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			input := makeMXLTestArchive(t, []mxlTestEntry{
				mxlTestContainerEntry(`<rootfile full-path="` + test.rootFile + `"/>`),
				mxlTestFileEntry(test.entry, `<score-partwise/>`),
			})
			_, err := DecodeMXLPackage(bytes.NewReader(input))
			assert.ErrorIs(t, err, test.wantErr)
		})
	}

	t.Run("duplicate normalized root files", func(t *testing.T) {
		input := makeMXLTestArchive(t, []mxlTestEntry{
			mxlTestContainerEntry(`<rootfile full-path="score.musicxml"/><rootfile full-path=" &#x9;score.musicxml "/>`),
			mxlTestFileEntry("score.musicxml", `<score-partwise/>`),
		})
		_, err := DecodeMXLPackage(bytes.NewReader(input))
		assert.ErrorIs(t, err, ErrMXLDuplicateEntry)
	})
}

func TestEncodeMXLRootFileWhitespaceNormalization(t *testing.T) {
	t.Parallel()

	rootFiles := []MXLRootFile{
		{FullPath: " \tscore.musicxml \r\n", MediaType: " " + musicXMLMIMEType + " "},
		{FullPath: " alternate/score.pdf ", MediaType: " application/pdf "},
	}
	value := &MXLPackage{
		Document:  &ScorePartwise{},
		RootFiles: append([]MXLRootFile(nil), rootFiles...),
		Resources: []MXLResource{{Path: "alternate/score.pdf", Data: []byte("PDF")}},
	}
	var encoded bytes.Buffer
	require.NoError(t, EncodeMXLPackage(&encoded, value))
	assert.Equal(t, rootFiles, value.RootFiles)
	roundTripped, err := DecodeMXLPackage(bytes.NewReader(encoded.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, []MXLRootFile{
		{FullPath: "score.musicxml", MediaType: musicXMLMIMEType},
		{FullPath: "alternate/score.pdf", MediaType: "application/pdf"},
	}, roundTripped.RootFiles)
	assert.Equal(t, value.Resources, roundTripped.Resources)

	value.Resources = append(value.Resources, MXLResource{Path: "score.musicxml"})
	assert.ErrorIs(t, EncodeMXLPackage(&bytes.Buffer{}, value), ErrMXLDuplicateEntry)
}

func TestResolveMXLLinkWhitespaceNormalization(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		href     string
		path     string
		fragment string
		wantErr  error
	}{
		{name: "padded", href: " \tchild.musicxml\r\n ", path: "collection/child.musicxml"},
		{name: "collapsed", href: "child\t \r\nscore.musicxml", path: "collection/child score.musicxml"},
		{name: "encoded spaces", href: " %20child%20%20score.musicxml%20 ", path: "collection/ child  score.musicxml "},
		{name: "encoded tab", href: "child%09score.musicxml", path: "collection/child\tscore.musicxml"},
		{name: "non-XML whitespace", href: "\u00a0child.musicxml\u00a0", path: "collection/\u00a0child.musicxml\u00a0"},
		{name: "fragment", href: " child.musicxml#movement%20 \n", path: "collection/child.musicxml", fragment: "movement "},
		{name: "empty", href: " \t\r\n ", path: "collection/main.musicxml"},
		{name: "external", href: " \thttps://example.com/child.musicxml ", wantErr: ErrMXLExternalLink},
		{name: "query", href: " child.musicxml?version=1 ", wantErr: ErrMXLInvalidLink},
		{name: "outside archive", href: " ../../child.musicxml ", wantErr: ErrMXLInvalidPath},
		{name: "invalid escape", href: " child%ZZ.musicxml ", wantErr: ErrMXLInvalidLink},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			path, fragment, err := resolveMXLLinkPath("collection/main.musicxml", test.href)
			if test.wantErr != nil {
				assert.ErrorIs(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.path, path)
			assert.Equal(t, test.fragment, fragment)
		})
	}
}

func TestMXLWhitespaceResolveAndSync(t *testing.T) {
	t.Parallel()

	firstXML := encodeUTF16(`<?xml version="1.0" encoding="UTF-16BE"?><score-partwise/>`, binary.BigEndian)[2:]
	secondXML := []byte("<score-timewise>\n</score-timewise>")
	childXML := []byte(`<opus><score xmlns:xlink="http://www.w3.org/1999/xlink" xlink:href=" %20second%20%20score.musicxml%20 "/></opus>`)
	value := &MXLPackage{
		Document: &OpusDocument{Content: []OpusDocumentContent{
			{Score: &OpusScore{Href: " \tfirst.musicxml \r\n"}},
			{Score: &OpusScore{Href: "first.musicxml"}},
			{OpusLink: &OpusLink{Href: " \nchild.musicxml \t"}},
		}},
		RootFiles: []MXLRootFile{{FullPath: " main.musicxml "}},
		Resources: []MXLResource{
			{Path: "first.musicxml", Data: firstXML},
			{Path: " second  score.musicxml ", Data: secondXML},
			{Path: "child.musicxml", Data: childXML},
			{Path: "images/ cover  .png ", Data: []byte("PNG")},
		},
	}
	before := cloneMXLResources(value.Resources)
	resolved, err := value.ResolveOpus()
	require.NoError(t, err)
	assert.Equal(t, "main.musicxml", resolved.Path)
	assert.Same(t, resolved.Content[0].Score.Target, resolved.Content[1].Score.Target)
	assert.Equal(t, " \tfirst.musicxml \r\n", resolved.Content[0].Score.Link.Href)
	assert.Equal(t, " second  score.musicxml ", resolved.Content[2].OpusLink.Target.Content[0].Score.Path)
	require.NoError(t, value.SyncResolvedOpus(resolved))
	assert.Equal(t, before, value.Resources)

	first, ok := AsScorePartwise(resolved.Content[0].Score.Target)
	require.True(t, ok)
	first.MovementTitle = Ptr("Edited")
	require.NoError(t, value.SyncResolvedOpus(resolved))
	assert.Equal(t, mxlResourcePaths(before), mxlResourcePaths(value.Resources))
	assert.Equal(t, before[1:], value.Resources[1:])
	updated, err := DecodeScorePartwise(bytes.NewReader(value.Resources[0].Data))
	require.NoError(t, err)
	assert.Equal(t, Ptr("Edited"), updated.MovementTitle)

	var encoded bytes.Buffer
	require.NoError(t, EncodeMXLPackage(&encoded, value))
	roundTripped, err := DecodeMXLPackage(bytes.NewReader(encoded.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, value.Resources, roundTripped.Resources)
	_, err = roundTripped.ResolveOpus()
	require.NoError(t, err)
}
