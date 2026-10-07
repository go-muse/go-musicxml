package musicxml

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMXLContainerForeignNamespaceLookalikes(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		metadata string
	}{
		{"namespace declaration", `<container><rootfiles><rootfile full-path="score.musicxml" xmlns:full-path="urn:vendor"/></rootfiles></container>`},
		{"media type", `<container xmlns:v="urn:vendor"><rootfiles><rootfile full-path="score.musicxml" v:media-type="application/pdf"/></rootfiles></container>`},
		{"attribute", `<container xmlns:v="urn:vendor"><rootfiles><rootfile full-path="score.musicxml" v:full-path="wrong.musicxml"/></rootfiles></container>`},
		{"element", `<container xmlns:v="urn:vendor"><rootfiles><v:rootfile full-path="wrong.musicxml"/><rootfile full-path="score.musicxml"/></rootfiles></container>`},
		{"rootfiles", `<container xmlns:v="urn:vendor"><v:rootfiles><rootfile full-path="wrong.musicxml"/></v:rootfiles><rootfiles><rootfile full-path="score.musicxml"/></rootfiles></container>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			input := makeMXLTestArchive(t, []mxlTestEntry{
				mxlTestFileEntry(mxlContainerPath, test.metadata),
				mxlTestFileEntry("score.musicxml", `<score-partwise><movement-title>Correct</movement-title></score-partwise>`),
				mxlTestFileEntry("wrong.musicxml", `<score-partwise><movement-title>Wrong</movement-title></score-partwise>`),
				mxlTestFileEntry("urn:vendor", `<score-partwise><movement-title>Wrong</movement-title></score-partwise>`),
			})
			value, err := DecodeMXLPackage(bytes.NewReader(input))
			require.NoError(t, err)
			score, ok := AsScorePartwise(value.Document)
			require.True(t, ok)
			assert.Equal(t, Ptr("Correct"), score.MovementTitle)
			assert.Equal(t, []MXLRootFile{{FullPath: "score.musicxml"}}, value.RootFiles)
			scoreOnly, err := DecodeMXLScorePartwise(bytes.NewReader(input))
			require.NoError(t, err)
			assert.Equal(t, Ptr("Correct"), scoreOnly.MovementTitle)

			var output bytes.Buffer
			require.NoError(t, EncodeMXLPackage(&output, value))
			roundTripped, err := DecodeMXLPackage(bytes.NewReader(output.Bytes()))
			require.NoError(t, err)
			assert.Equal(t, value, roundTripped)
		})
	}
}

func TestMXLContainerQualifiedContentCannotSupplyMetadata(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		metadata string
		wantErr  error
	}{
		{"root", `<v:container xmlns:v="urn:vendor"><rootfiles><rootfile full-path="score.musicxml"/></rootfiles></v:container>`, ErrMXLInvalidContainer},
		{"attribute", `<container xmlns:v="urn:vendor"><rootfiles><rootfile v:full-path="score.musicxml"/></rootfiles></container>`, ErrMXLInvalidPath},
		{"element", `<container xmlns:v="urn:vendor"><rootfiles><v:rootfile full-path="score.musicxml"/></rootfiles></container>`, ErrMXLRootFileNotFound},
		{"rootfiles", `<container xmlns:v="urn:vendor"><v:rootfiles><rootfile full-path="score.musicxml"/></v:rootfiles></container>`, ErrMXLRootFileNotFound},
		{"tail", `<container><rootfiles><rootfile full-path="score.musicxml"/></rootfiles></container><v:ignored xmlns:v="urn:vendor"/>`, ErrMXLInvalidContainer},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := makeMXLTestArchive(t, []mxlTestEntry{
				mxlTestFileEntry(mxlContainerPath, test.metadata),
				mxlTestFileEntry("score.musicxml", `<score-partwise/>`),
			})
			_, err := DecodeMXLPackage(bytes.NewReader(input))
			assert.ErrorIs(t, err, test.wantErr)
			_, err = DecodeMXL(bytes.NewReader(input))
			assert.ErrorIs(t, err, test.wantErr)
		})
	}
}
