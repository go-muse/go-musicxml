package musicxml

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMXLAbsoluteLinkDotSegmentsRoundTrip(t *testing.T) {
	t.Parallel()

	for _, href := range []string{
		"/scores/./first.musicxml",
		"/scores/sub/../first.musicxml",
		"/./scores/first.musicxml",
		"/unused/../scores/first.musicxml#movement",
	} {
		t.Run(href, func(t *testing.T) {
			t.Parallel()
			value := opusPackageWithLink(&OpusScore{Href: href}, []MXLResource{{
				Path: "scores/first.musicxml", Data: []byte(`<score-partwise/>`),
			}})
			value.RootFiles = []MXLRootFile{{FullPath: "collections/main.musicxml"}}
			resolved, err := value.ResolveOpus()
			require.NoError(t, err)
			assert.Equal(t, "scores/first.musicxml", resolved.Content[0].Score.Path)
			first, ok := AsScorePartwise(resolved.Content[0].Score.Target)
			require.True(t, ok)
			first.MovementTitle = Ptr("Edited")
			require.NoError(t, value.SyncResolvedOpus(resolved))

			var output bytes.Buffer
			require.NoError(t, EncodeMXLPackage(&output, value))
			decoded, err := DecodeMXLPackage(bytes.NewReader(output.Bytes()))
			require.NoError(t, err)
			roundTripped, err := decoded.ResolveOpus()
			require.NoError(t, err)
			after, ok := AsScorePartwise(roundTripped.Content[0].Score.Target)
			require.True(t, ok)
			assert.Equal(t, Ptr("Edited"), after.MovementTitle)
			assert.Equal(t, href, roundTripped.Content[0].Score.Link.Href)
		})
	}
}

func TestMXLLinkDotSegmentsRemainBounded(t *testing.T) {
	t.Parallel()

	for _, href := range []string{
		"/../scores/first.musicxml",
		"/scores/../../scores/first.musicxml",
		"/scores/first.musicxml/",
		"/scores/first.musicxml/.",
		"/scores/first.musicxml/child/..",
		"../scores/first.musicxml/",
		"../scores/first.musicxml/.",
		"../scores/first.musicxml/child/..",
		"/scores/../META-INF/container.xml",
		"/scores/../mimetype",
	} {
		t.Run(href, func(t *testing.T) {
			t.Parallel()
			value := opusPackageWithLink(&OpusScore{Href: href}, []MXLResource{{
				Path: "scores/first.musicxml", Data: []byte(`<score-partwise/>`),
			}})
			value.RootFiles = []MXLRootFile{{FullPath: "collections/main.musicxml"}}
			_, err := value.ResolveOpus()
			assert.ErrorIs(t, err, ErrMXLInvalidPath)
		})
	}
}

func TestEncodeMXLRejectsNonXMLRootFilePaths(t *testing.T) {
	t.Parallel()

	for _, char := range []rune{0, 1, 8, 11, 12, 14, 31, 0xfffe, 0xffff} {
		for _, alternate := range []bool{false, true} {
			t.Run(fmt.Sprintf("U+%04X/alternate=%t", char, alternate), func(t *testing.T) {
				t.Parallel()
				filePath := "score" + string(char) + ".musicxml"
				value := &MXLPackage{Document: &ScorePartwise{}}
				if alternate {
					value.RootFiles = []MXLRootFile{{FullPath: "main.musicxml"}, {FullPath: filePath}}
					value.Resources = []MXLResource{{Path: filePath, Data: []byte(`<score-partwise/>`)}}
				} else {
					value.RootFiles = []MXLRootFile{{FullPath: filePath}}
				}
				var output bytes.Buffer
				assert.ErrorIs(t, EncodeMXLPackage(&output, value), ErrMXLInvalidPath)
				assert.Empty(t, output.Bytes())
			})
		}
	}
}

func TestMXLRootFilePathXMLCharactersRoundTrip(t *testing.T) {
	t.Parallel()

	for _, char := range []rune{0x20, 0x7f, 0x85, 0xd7ff, 0xe000, 0xfffd, 0x10000, 0x10ffff} {
		t.Run(fmt.Sprintf("U+%04X", char), func(t *testing.T) {
			t.Parallel()
			filePath := "score" + string(char) + ".musicxml"
			value := &MXLPackage{Document: &ScorePartwise{}, RootFiles: []MXLRootFile{{FullPath: filePath}}}
			var output bytes.Buffer
			require.NoError(t, EncodeMXLPackage(&output, value))
			decoded, err := DecodeMXLPackage(bytes.NewReader(output.Bytes()))
			require.NoError(t, err)
			assert.Equal(t, value.RootFiles, decoded.RootFiles)
		})
	}
}
