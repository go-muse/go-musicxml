package musicxml

import (
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOfficialOpusExamplesRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		file       string
		wantLength int
	}{
		{
			name:       "opus links",
			file:       "OpusLink.musicxml",
			wantLength: 2,
		},
		{
			name:       "scores",
			file:       "OpusScore.musicxml",
			wantLength: 16,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			input, err := os.ReadFile(filepath.Join(
				"testdata",
				"official",
				test.file,
			))
			require.NoError(t, err)

			document, err := Decode(bytes.NewReader(input))
			require.NoError(t, err)

			opus, ok := document.(*OpusDocument)
			require.True(t, ok)
			assert.Equal(t, stringPointer(MusicXMLVersion), opus.Version)
			assert.Equal(t, stringPointer("Fidelio"), opus.Title)
			require.Len(t, opus.Content, test.wantLength)

			var encoded bytes.Buffer
			err = Encode(&encoded, opus)
			require.NoError(t, err)

			decoded, err := Decode(bytes.NewReader(encoded.Bytes()))
			require.NoError(t, err)
			assert.Equal(t, opus, decoded)
		})
	}
}

func TestOpusDocumentNestedOrderRoundTrip(t *testing.T) {
	t.Parallel()

	input := []byte(
		`<opus version="4.0">` +
			`<title>Collected works</title>` +
			`<score xmlns:xlink="http://www.w3.org/1999/xlink" ` +
			`xlink:href="scores/first.musicxml" new-page="yes"/>` +
			`<opus>` +
			`<title>Appendix</title>` +
			`<opus-link xmlns:xlink="http://www.w3.org/1999/xlink" ` +
			`xlink:href="appendix.musicxml"/>` +
			`</opus>` +
			`<score xmlns:xlink="http://www.w3.org/1999/xlink" ` +
			`xlink:href="scores/last.musicxml"/>` +
			`</opus>`,
	)

	document, err := Decode(bytes.NewReader(input))
	require.NoError(t, err)

	opus, ok := document.(*OpusDocument)
	require.True(t, ok)
	require.Len(t, opus.Content, 3)

	require.NotNil(t, opus.Content[0].Score)
	assert.Equal(
		t,
		"scores/first.musicxml",
		opus.Content[0].Score.Href,
	)
	assert.Equal(
		t,
		yesNoPointer(YesNoYes),
		opus.Content[0].Score.NewPage,
	)

	nested := opus.Content[1].Opus
	require.NotNil(t, nested)
	assert.Equal(t, stringPointer("Appendix"), nested.Title)
	require.Len(t, nested.Content, 1)
	require.NotNil(t, nested.Content[0].OpusLink)
	assert.Equal(
		t,
		"appendix.musicxml",
		nested.Content[0].OpusLink.Href,
	)

	require.NotNil(t, opus.Content[2].Score)
	assert.Equal(
		t,
		"scores/last.musicxml",
		opus.Content[2].Score.Href,
	)

	var encoded bytes.Buffer
	err = Encode(&encoded, opus)
	require.NoError(t, err)

	names, err := childElementNames(encoded.Bytes())
	require.NoError(t, err)
	assert.Equal(
		t,
		[]string{"title", "score", "opus", "score"},
		names,
	)

	decoded, err := Decode(bytes.NewReader(encoded.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, opus, decoded)
}

func TestOpusDocumentContentErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content OpusDocumentContent
		wantErr string
	}{
		{
			name:    "empty variant",
			wantErr: "must contain exactly one value, got 0",
		},
		{
			name: "multiple variants",
			content: OpusDocumentContent{
				OpusLink: &OpusLink{},
				Score:    &OpusScore{},
			},
			wantErr: "must contain exactly one value, got 2",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var result bytes.Buffer
			err := Encode(&result, &OpusDocument{
				Content: []OpusDocumentContent{test.content},
			})

			assert.ErrorContains(t, err, test.wantErr)
		})
	}
}

func TestOpusDocumentUnknownElement(t *testing.T) {
	t.Parallel()

	document, err := Decode(bytes.NewBufferString(
		`<opus><future-entry/><title>Known</title></opus>`,
	))
	require.NoError(t, err)
	require.NotNil(t, document)

	var encoded bytes.Buffer
	require.NoError(t, Encode(&encoded, document))
	assert.NotContains(t, encoded.String(), "future-entry")
	assert.Contains(t, encoded.String(), "<title>Known</title>")

	decodedAgain, err := Decode(bytes.NewReader(encoded.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, document, decodedAgain)
}

func TestOpusDocumentXMLName(t *testing.T) {
	t.Parallel()

	document, err := Decode(bytes.NewBufferString(`<opus/>`))
	require.NoError(t, err)

	opus, ok := document.(*OpusDocument)
	require.True(t, ok)
	assert.Equal(t, xml.Name{Local: "opus"}, opus.XMLName)
}

func TestOpusMXLPackageResourcesRoundTrip(t *testing.T) {
	t.Parallel()

	version := MusicXMLVersion
	value := &MXLPackage{
		Document: &OpusDocument{
			XMLName: xml.Name{Local: "opus"},
			Version: &version,
			Content: []OpusDocumentContent{
				{
					Score: &OpusScore{
						Href: "scores/first.musicxml",
					},
				},
				{
					OpusLink: &OpusLink{
						Href: "collections/appendix.musicxml",
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
				Data: []byte(`<opus><title>Appendix</title></opus>`),
			},
		},
	}

	var encoded bytes.Buffer
	err := EncodeMXLPackage(&encoded, value)
	require.NoError(t, err)

	actual, err := DecodeMXLPackage(
		bytes.NewReader(encoded.Bytes()),
	)
	require.NoError(t, err)
	assert.Equal(t, value, actual)
}

func yesNoPointer(value YesNo) *YesNo {
	return &value
}
