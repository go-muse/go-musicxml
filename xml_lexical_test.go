package musicxml

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Encoding selection remains within the same original-byte cursor. Check the
// lexical origin and byte spans after a non-UTF-8 declaration without relying
// on the removed decoder/observer handoff implementation.
func TestXMLLexicalProvenanceAfterEncodingSelection(t *testing.T) {
	t.Parallel()
	input := []byte("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><root>\xe9&#32;<![CDATA[&]]></root>")
	for _, fragmented := range []bool{false, true} {
		t.Run(map[bool]string{false: "buffered", true: "one byte"}[fragmented], func(t *testing.T) {
			var inputReader io.Reader = bytes.NewReader(input)
			if fragmented {
				inputReader = iotest.OneByteReader(inputReader)
			}
			reader, err := newSourceXMLTokenReader(inputReader, sourceXMLConfig{Context: context.Background()})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, reader.Close()) })
			var characterTokens int
			for {
				token, err := reader.Token()
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				data, ok := token.(xml.CharData)
				if !ok {
					continue
				}
				meta := reader.Metadata()
				original := input[meta.Span.Start.ByteOffset:meta.Span.End.ByteOffset]
				switch characterTokens {
				case 0:
					assert.Equal(t, "é ", string(data))
					assert.Equal(t, "\xe9&#32;", string(original))
					assert.True(t, meta.Reference)
					assert.False(t, meta.CDATA)
				case 1:
					assert.Equal(t, "&", string(data))
					assert.Equal(t, "<![CDATA[&]]>", string(original))
					assert.True(t, meta.CDATA)
					assert.False(t, meta.Reference)
				default:
					t.Errorf("unexpected character token %q", data)
				}
				characterTokens++
			}
			assert.Equal(t, 2, characterTokens)
			assert.True(t, reader.Summary().InputComplete)
			assert.EqualValues(t, len(input), reader.Summary().BytesConsumed)
		})
	}
}
