package musicxml

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	t.Parallel()

	version := "4.0"
	title := "Fidelio"
	tests := []struct {
		name    string
		input   string
		want    Document
		wantErr error
	}{
		{
			name: "opus",
			input: `<opus version="4.0">` +
				`<title>Fidelio</title>` +
				`</opus>`,
			want: &OpusDocument{
				XMLName: xml.Name{Local: "opus"},
				Title:   &title,
				Version: &version,
			},
		},
		{
			name:  "score partwise",
			input: `<score-partwise version="4.0"></score-partwise>`,
			want: &ScorePartwise{
				XMLName: xml.Name{Local: "score-partwise"},
				Version: &version,
			},
		},
		{
			name: "score timewise",
			input: `<?xml version="1.0" encoding="UTF-8"?>
				<score-timewise version="4.0"></score-timewise>`,
			want: &ScoreTimewise{
				XMLName: xml.Name{Local: "score-timewise"},
				Version: &version,
			},
		},
		{
			name:    "unsupported root",
			input:   `<sounds></sounds>`,
			wantErr: ErrUnsupportedRoot,
		},
		{
			name: "namespaced MusicXML root",
			input: `<score-partwise xmlns="urn:example">` +
				`</score-partwise>`,
			wantErr: ErrUnsupportedRoot,
		},
		{
			name:    "empty document",
			input:   ``,
			wantErr: ErrEmptyDocument,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual, err := Decode(strings.NewReader(test.input))

			if test.wantErr != nil {
				assert.ErrorIs(t, err, test.wantErr)
				assert.Nil(t, actual)

				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.want, actual)
		})
	}
}

func TestDecodeXMLDepthLimit(t *testing.T) {
	t.Parallel()

	allowed, err := DecodeOpusDocumentWithOptions(
		strings.NewReader(nestedOpusXML(4)),
		DecodeOptions{MaxXMLDepth: 4},
	)
	assert.NoError(t, err)
	assert.NotNil(t, allowed)

	rejected, err := DecodeWithOptions(
		strings.NewReader(nestedOpusXML(5)),
		DecodeOptions{MaxXMLDepth: 4},
	)
	assert.ErrorIs(t, err, ErrXMLTooDeep)
	assert.Nil(t, rejected)

	rejected, err = Decode(
		strings.NewReader(nestedOpusXML(defaultMaxXMLDepth + 1)),
	)
	assert.ErrorIs(t, err, ErrXMLTooDeep)
	assert.Nil(t, rejected)
}

func TestDecodeOptionsRejectInvalidDepth(t *testing.T) {
	t.Parallel()

	allowed, err := DecodeWithOptions(
		strings.NewReader(nestedOpusXML(maximumXMLDepth)),
		DecodeOptions{MaxXMLDepth: maximumXMLDepth},
	)
	assert.NoError(t, err)
	assert.NotNil(t, allowed)

	for _, depth := range []int{-1, maximumXMLDepth + 1} {
		document, err := DecodeWithOptions(
			strings.NewReader(`<opus/>`),
			DecodeOptions{MaxXMLDepth: depth},
		)

		assert.ErrorIs(t, err, ErrInvalidDecodeOptions)
		assert.Nil(t, document)
	}
}

func TestDecodeIgnoresUnknownChildElements(t *testing.T) {
	t.Parallel()

	document, err := Decode(strings.NewReader(
		`<score-partwise version="4.0">` +
			`<part-list><score-part id="P1">` +
			`<part-name>Piano</part-name>` +
			`</score-part></part-list>` +
			`<part id="P1"><measure number="1">` +
			`<attributes><key><fifths>0</fifths>` +
			`<vendor:extension xmlns:vendor="urn:vendor"/>` +
			`<mode>major</mode></key></attributes>` +
			`<note><rest/><duration>1</duration></note>` +
			`</measure></part></score-partwise>`,
	))
	require.NoError(t, err)

	var encoded bytes.Buffer
	require.NoError(t, Encode(&encoded, document))
	assert.NotContains(t, encoded.String(), "extension")
	assert.NoError(t, Validate(document))
}

func nestedOpusXML(depth int) string {
	return strings.Repeat(`<opus>`, depth) +
		strings.Repeat(`</opus>`, depth)
}

func TestDecodeTypedDocuments(t *testing.T) {
	t.Parallel()

	version := "4.0"
	tests := []struct {
		name   string
		input  string
		decode func(io.Reader) (any, error)
		want   any
	}{
		{
			name:  "score partwise",
			input: `<score-partwise version="4.0"></score-partwise>`,
			decode: func(reader io.Reader) (any, error) {
				return DecodeScorePartwise(reader)
			},
			want: &ScorePartwise{
				XMLName: xml.Name{Local: "score-partwise"},
				Version: &version,
			},
		},
		{
			name:  "score timewise",
			input: `<score-timewise version="4.0"></score-timewise>`,
			decode: func(reader io.Reader) (any, error) {
				return DecodeScoreTimewise(reader)
			},
			want: &ScoreTimewise{
				XMLName: xml.Name{Local: "score-timewise"},
				Version: &version,
			},
		},
		{
			name:  "opus",
			input: `<opus version="4.0"></opus>`,
			decode: func(reader io.Reader) (any, error) {
				return DecodeOpusDocument(reader)
			},
			want: &OpusDocument{
				XMLName: xml.Name{Local: "opus"},
				Version: &version,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual, err := test.decode(strings.NewReader(test.input))

			assert.NoError(t, err)
			assert.Equal(t, test.want, actual)
		})
	}
}

func TestDecodeTypedDocumentsRejectUnexpectedRoot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		decode   func(io.Reader) error
		wantName xml.Name
	}{
		{
			name:  "score partwise",
			input: `<score-timewise></score-timewise>`,
			decode: func(reader io.Reader) error {
				_, err := DecodeScorePartwise(reader)
				return err
			},
			wantName: xml.Name{Local: "score-timewise"},
		},
		{
			name:  "score timewise",
			input: `<opus></opus>`,
			decode: func(reader io.Reader) error {
				_, err := DecodeScoreTimewise(reader)
				return err
			},
			wantName: xml.Name{Local: "opus"},
		},
		{
			name:  "opus",
			input: `<score-partwise></score-partwise>`,
			decode: func(reader io.Reader) error {
				_, err := DecodeOpusDocument(reader)
				return err
			},
			wantName: xml.Name{Local: "score-partwise"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.decode(strings.NewReader(test.input))

			assert.ErrorIs(t, err, ErrUnsupportedRoot)

			var rootErr *UnsupportedRootError
			if assert.ErrorAs(t, err, &rootErr) {
				assert.Equal(t, test.wantName, rootErr.Name)
			}
		})
	}
}

func TestDecodeUnsupportedRootError(t *testing.T) {
	t.Parallel()

	document, err := Decode(strings.NewReader(`<sounds></sounds>`))

	assert.Nil(t, document)
	assert.ErrorIs(t, err, ErrUnsupportedRoot)

	var rootErr *UnsupportedRootError
	if assert.ErrorAs(t, err, &rootErr) {
		assert.Equal(t, xml.Name{Local: "sounds"}, rootErr.Name)
	}
}

func TestDecodeNamespacedRootError(t *testing.T) {
	t.Parallel()

	document, err := Decode(strings.NewReader(
		`<score-partwise xmlns="urn:example"></score-partwise>`,
	))

	assert.Nil(t, document)
	assert.ErrorIs(t, err, ErrUnsupportedRoot)

	var rootErr *UnsupportedRootError
	if assert.ErrorAs(t, err, &rootErr) {
		assert.Equal(t, xml.Name{
			Space: "urn:example",
			Local: "score-partwise",
		}, rootErr.Name)
	}
}
