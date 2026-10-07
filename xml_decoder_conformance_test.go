package musicxml

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeUTF16WithoutBOM(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		order binary.ByteOrder
	}{
		{"UTF-16BE", binary.BigEndian},
		{"UTF-16LE", binary.LittleEndian},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			for _, declaration := range []string{
				`<?xml version="1.0" encoding="` + test.name + `"?>`,
				`<?xml version='1.0' encoding = '` + test.name + `'?>`,
			} {
				input := encodeUTF16(declaration+
					`<score-partwise><movement-title>Čajkovskij 𝄞</movement-title></score-partwise>`,
					test.order)[2:]
				for _, reader := range []io.Reader{
					bytes.NewReader(input),
					iotest.OneByteReader(bytes.NewReader(input)),
				} {
					document, err := DecodeScorePartwise(reader)
					require.NoError(t, err)
					assert.Equal(t, Ptr("Čajkovskij 𝄞"), document.MovementTitle)
				}
			}
		})
	}
}

func TestDecodeUTF16DeclarationPrecedence(t *testing.T) {
	t.Parallel()

	for _, order := range []binary.ByteOrder{binary.BigEndian, binary.LittleEndian} {
		for _, bom := range []bool{false, true} {
			for _, test := range []struct {
				name        string
				declaration string
				valid       bool
			}{
				{"generic", `<?xml version="1.0" encoding="UTF-16"?>`, bom},
				{"big endian", `<?xml version="1.0" encoding="UTF-16BE"?>`, bom || order == binary.BigEndian},
				{"little endian", `<?xml version="1.0" encoding="UTF-16LE"?>`, bom || order == binary.LittleEndian},
				{"UTF-8", `<?xml version="1.0" encoding="UTF-8"?>`, bom},
				{"Latin-1", `<?xml version="1.0" encoding="ISO-8859-1"?>`, bom},
				{"unspecified encoding", `<?xml version="1.0"?>`, bom},
				{"no declaration", "", bom},
			} {
				t.Run(order.String()+"/"+map[bool]string{true: "BOM", false: "no BOM"}[bom]+"/"+test.name, func(t *testing.T) {
					t.Parallel()

					// With a byte order mark the mark wins over the declaration,
					// so a non-ASCII title proves the text was not re-decoded.
					input := encodeUTF16(test.declaration+
						`<score-partwise><movement-title>Čajkovskij</movement-title></score-partwise>`,
						order)
					if !bom {
						input = input[2:]
					}
					document, err := DecodeScorePartwise(bytes.NewReader(input))
					if test.valid {
						require.NoError(t, err)
						assert.Equal(t, Ptr("Čajkovskij"), document.MovementTitle)
					} else {
						assert.Error(t, err)
					}
				})
			}
		}
	}
}
