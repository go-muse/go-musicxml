package musicxml

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
