package musicxml

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOfficialExamplesRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		file          string
		wantParts     int
		wantMeasures  int
		wantMusicData int
	}{
		{
			name:          "chant UTF-8",
			file:          "Chant.musicxml",
			wantParts:     1,
			wantMeasures:  1,
			wantMusicData: 33,
		},
		{
			name:          "Mozart trio UTF-8",
			file:          "MozartTrio.musicxml",
			wantParts:     5,
			wantMeasures:  90,
			wantMusicData: 306,
		},
		{
			name:          "Mozart An Chloe UTF-16",
			file:          "MozaChloSample.musicxml",
			wantParts:     2,
			wantMeasures:  36,
			wantMusicData: 310,
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

			actual, ok := document.(*ScorePartwise)
			require.True(t, ok)
			assert.Equal(t, test.wantParts, len(actual.Part))
			assert.Equal(
				t,
				test.wantMeasures,
				scorePartwiseMeasureCount(actual),
			)
			assert.Equal(
				t,
				test.wantMusicData,
				scorePartwiseMusicDataCount(actual),
			)

			var encoded bytes.Buffer
			err = Encode(&encoded, actual)
			require.NoError(t, err)

			decodedAgain, err := Decode(
				bytes.NewReader(encoded.Bytes()),
			)
			require.NoError(t, err)
			assert.True(
				t,
				reflect.DeepEqual(document, decodedAgain),
				"document model changed after round-trip",
			)

			var compressed bytes.Buffer
			err = EncodeMXL(&compressed, actual)
			require.NoError(t, err)

			decodedCompressed, err := DecodeMXL(
				bytes.NewReader(compressed.Bytes()),
			)
			require.NoError(t, err)
			assert.True(
				t,
				reflect.DeepEqual(document, decodedCompressed),
				"document model changed after MXL round-trip",
			)
		})
	}
}

func scorePartwiseMeasureCount(
	document *ScorePartwise,
) int {
	result := 0
	for _, part := range document.Part {
		result += len(part.Measure)
	}

	return result
}

func scorePartwiseMusicDataCount(
	document *ScorePartwise,
) int {
	result := 0
	for _, part := range document.Part {
		for _, measure := range part.Measure {
			result += len(measure.Content)
		}
	}

	return result
}
