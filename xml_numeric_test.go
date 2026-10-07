package musicxml

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseXMLUnsignedIntegerLexicalForms(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		lexical string
		want    uint64
	}{
		{"+1", 1},
		{"+0001", 1},
		{" \t+1\r\n", 1},
		{"-0", 0},
		{"-000", 0},
		{"0", 0},
		{"18446744073709551615", 18446744073709551615},
	} {
		actual, err := parseXMLUnsignedInteger(test.lexical, 64)
		require.NoError(t, err, test.lexical)
		assert.Equal(t, test.want, actual, test.lexical)
	}
	actual, err := parseXMLUnsignedInteger("+255", 8)
	require.NoError(t, err)
	assert.Equal(t, uint64(255), actual)
}

func TestParseXMLUnsignedIntegerRejectsInvalidForms(t *testing.T) {
	t.Parallel()
	for _, lexical := range []string{"", " ", "-1", "++1", "+-0", "--0", "1.0", "1e1", "1 0", "0x1", "1_0", " 1", "1 ", "18446744073709551616"} {
		_, err := parseXMLUnsignedInteger(lexical, 64)
		assert.Error(t, err, lexical)
	}
	_, err := parseXMLUnsignedInteger("+256", 8)
	assert.Error(t, err)
}
