package musicxml

import (
	"bytes"
	"encoding/binary"
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeUTF16(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		order    binary.ByteOrder
		encoding string
	}{
		{
			name:     "big endian",
			order:    binary.BigEndian,
			encoding: "UTF-16",
		},
		{
			name:     "little endian",
			order:    binary.LittleEndian,
			encoding: "UTF-16",
		},
		{
			name:     "explicit big endian",
			order:    binary.BigEndian,
			encoding: "UTF-16BE",
		},
		{
			name:     "explicit little endian",
			order:    binary.LittleEndian,
			encoding: "UTF-16LE",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			input := `<?xml version="1.0" encoding="` +
				test.encoding +
				`"?>` +
				`<score-partwise version="4.0">` +
				`<movement-title>Čajkovskij 𝄞</movement-title>` +
				`</score-partwise>`

			document, err := Decode(
				bytes.NewReader(encodeUTF16(input, test.order)),
			)
			require.NoError(t, err)

			actual, ok := document.(*ScorePartwise)
			require.True(t, ok)
			require.NotNil(t, actual.MovementTitle)
			assert.Equal(
				t,
				"Čajkovskij 𝄞",
				*actual.MovementTitle,
			)
		})
	}
}

func TestDecodeISO88591(t *testing.T) {
	t.Parallel()

	input := append(
		[]byte(
			`<?xml version="1.0" encoding="ISO-8859-1"?>`+
				`<score-partwise version="4.0">`+
				`<movement-title>Caf`,
		),
		0xe9,
	)
	input = append(
		input,
		[]byte(`</movement-title></score-partwise>`)...,
	)

	document, err := Decode(bytes.NewReader(input))
	require.NoError(t, err)

	actual, ok := document.(*ScorePartwise)
	require.True(t, ok)
	require.NotNil(t, actual.MovementTitle)
	assert.Equal(t, "Café", *actual.MovementTitle)
}

func encodeUTF16(
	value string,
	order binary.ByteOrder,
) []byte {
	result := make([]byte, 2)
	if order == binary.BigEndian {
		copy(result, []byte{0xfe, 0xff})
	} else {
		copy(result, []byte{0xff, 0xfe})
	}

	for _, codeUnit := range utf16.Encode([]rune(value)) {
		var encoded [2]byte
		order.PutUint16(encoded[:], codeUnit)
		result = append(result, encoded[:]...)
	}

	return result
}
