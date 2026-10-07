package musicxml

import (
	"bytes"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeMXLMaximumByteLimits(t *testing.T) {
	t.Parallel()

	input := makeMXLTestArchive(t, []mxlTestEntry{
		mxlTestMIMETypeEntry(),
		mxlTestContainerEntry(`<rootfile full-path="score.musicxml"/>`),
		mxlTestFileEntry("score.musicxml", `<score-partwise/>`),
		mxlTestFileEntry("image.png", "PNG"),
	})
	for _, test := range []struct {
		name    string
		options MXLOptions
	}{
		{"archive", MXLOptions{MaxArchiveBytes: math.MaxInt64}},
		{"metadata", MXLOptions{MaxMetadataBytes: math.MaxInt64}},
		{"document", MXLOptions{MaxDocumentBytes: math.MaxInt64}},
		{"resource", MXLOptions{MaxResourceBytes: math.MaxInt64}},
		{"resources", MXLOptions{MaxResourcesBytes: math.MaxInt64}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual, err := DecodeMXLPackageWithOptions(bytes.NewReader(input), test.options)
			require.NoError(t, err)
			assert.IsType(t, &ScorePartwise{}, actual.Document)
			wantResources := []MXLResource{{Path: "image.png", Data: []byte("PNG")}}
			assert.Equal(t, wantResources, actual.Resources)

			var output bytes.Buffer
			require.NoError(t, EncodeMXLPackage(&output, actual))
			roundTripped, err := DecodeMXLPackage(bytes.NewReader(output.Bytes()))
			require.NoError(t, err)
			assert.Equal(t, wantResources, roundTripped.Resources)

			_, err = DecodeMXLWithOptions(bytes.NewReader(input), test.options)
			assert.NoError(t, err)
		})
	}
}

func TestReadMXLStreamLimitBoundary(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		input   string
		limit   int64
		wantErr bool
	}{
		{"below", "ab", 3, false},
		{"exact", "abc", 3, false},
		{"above", "abcd", 3, true},
		{"maximum", "abc", math.MaxInt64, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			actual, err := readMXLStream(bytes.NewBufferString(test.input), test.limit, "test")
			if test.wantErr {
				assert.ErrorIs(t, err, ErrMXLTooLarge)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, []byte(test.input), actual)
		})
	}
}
