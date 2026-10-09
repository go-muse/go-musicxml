package musicxml

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestXMLLexicalObserverHandoff(t *testing.T) {
	t.Parallel()
	for _, method := range []string{"Read", "ReadByte"} {
		t.Run(method, func(t *testing.T) {
			const remaining = " <root/> &"
			lexical := newLexicalXMLTokenReader(strings.NewReader("<!--&-->" + remaining))
			token, err := lexical.Token()
			require.NoError(t, err)
			require.IsType(t, xml.Comment{}, token)
			previous := lexical.input
			offset, angle, ampersand := previous.offset, previous.angle, previous.ampersand
			replacement := lexical.switchInput(previous)
			require.NotSame(t, previous, replacement)
			assert.Equal(t, lexical.decoder.InputOffset(), lexical.input.offset)
			assert.EqualValues(t, -1, lexical.input.angle)
			assert.EqualValues(t, -1, lexical.input.ampersand)

			if method == "Read" {
				// The charset converter's upstream Read must still deliver
				// every byte, without maintaining unused old marker state.
				data, err := io.ReadAll(replacement)
				require.NoError(t, err)
				assert.Equal(t, remaining, string(data))
				assert.Equal(t, offset+int64(len(remaining)), lexical.input.offset)
				assert.Equal(t, offset+1, lexical.input.angle)
				assert.Equal(t, offset+int64(len(remaining))-1, lexical.input.ampersand)
			} else {
				var data []byte
				for {
					value, err := previous.ReadByte()
					if err == io.EOF {
						break
					}
					require.NoError(t, err)
					data = append(data, value)
				}
				assert.Equal(t, remaining, string(data))
			}
			assert.Equal(t, offset, previous.offset, "superseded observer kept counting bytes")
			assert.Equal(t, angle, previous.angle, "superseded observer kept tracking angle markers")
			assert.Equal(t, ampersand, previous.ampersand, "superseded observer kept tracking reference markers")
		})
	}
}
