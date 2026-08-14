package musicxml

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncode(t *testing.T) {
	t.Parallel()

	version := MusicXMLVersion
	title := "Fidelio"
	tests := []struct {
		name     string
		document Document
		want     string
		wantErr  error
	}{
		{
			name: "opus",
			document: &OpusDocument{
				Title:   &title,
				Version: &version,
			},
			want: `<opus version="4.0">` +
				`<title>Fidelio</title>` +
				`</opus>`,
		},
		{
			name: "score partwise",
			document: &ScorePartwise{
				Version: &version,
			},
			want: `<score-partwise version="4.0">` +
				`<part-list></part-list>` +
				`</score-partwise>`,
		},
		{
			name: "score timewise",
			document: &ScoreTimewise{
				Version: &version,
			},
			want: `<score-timewise version="4.0">` +
				`<part-list></part-list>` +
				`</score-timewise>`,
		},
		{
			name:    "nil document",
			wantErr: ErrNilDocument,
		},
		{
			name:     "typed nil document",
			document: (*ScorePartwise)(nil),
			wantErr:  ErrNilDocument,
		},
		{
			name:     "typed nil opus",
			document: (*OpusDocument)(nil),
			wantErr:  ErrNilDocument,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var result strings.Builder

			err := Encode(&result, test.document)

			if test.wantErr != nil {
				assert.ErrorIs(t, err, test.wantErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.want, result.String())
		})
	}
}
