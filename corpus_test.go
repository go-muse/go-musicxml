package musicxml

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const musicXMLTestSuiteDirectory = "testdata/corpus/musicxml-test-suite"

var musicXMLTestSuiteDecodeFailures = map[string]struct{}{
	"99d-AccordionInvalid.xml": {},
}

var musicXMLTestSuiteValidationFailures = map[string]struct{}{
	"41g-PartNoId.xml":     {},
	"41h-TooManyParts.xml": {},
	"74a-FiguredBass.xml":  {},
}

func TestMusicXMLTestSuiteRoundTrip(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(musicXMLTestSuiteDirectory)
	require.NoError(t, err)

	tested := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		extension := strings.ToLower(filepath.Ext(entry.Name()))
		if extension != ".xml" &&
			extension != ".musicxml" &&
			extension != ".mxl" {
			continue
		}
		tested++

		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()

			input, err := os.ReadFile(filepath.Join(
				musicXMLTestSuiteDirectory,
				entry.Name(),
			))
			require.NoError(t, err)

			if extension == ".mxl" {
				value := assertMXLPackageRoundTrip(t, input)
				require.NoError(t, Validate(value.Document))
				return
			}

			if _, expected := musicXMLTestSuiteDecodeFailures[entry.Name()]; expected {
				_, err := Decode(bytes.NewReader(input))
				require.Error(t, err)
				return
			}

			document := assertDocumentRoundTrip(t, input)
			validationErr := Validate(document)
			if _, expected := musicXMLTestSuiteValidationFailures[entry.Name()]; expected {
				require.ErrorIs(t, validationErr, ErrInvalidDocument)
				return
			}
			require.NoError(t, validationErr)
		})
	}

	assert.Equal(t, 150, tested)
}

func assertDocumentRoundTrip(
	t testing.TB,
	input []byte,
) Document {
	t.Helper()

	document, err := Decode(bytes.NewReader(input))
	require.NoError(t, err)

	var encodedOnce bytes.Buffer
	err = Encode(&encodedOnce, document)
	require.NoError(t, err)

	decodedAgain, err := Decode(bytes.NewReader(encodedOnce.Bytes()))
	require.NoError(t, err)
	assert.Equal(
		t,
		document,
		decodedAgain,
		"document model changed after round-trip",
	)

	var encodedTwice bytes.Buffer
	err = Encode(&encodedTwice, decodedAgain)
	require.NoError(t, err)
	assertStableEncoding(
		t,
		encodedOnce.Bytes(),
		encodedTwice.Bytes(),
	)

	return document
}

func assertMXLPackageRoundTrip(
	t testing.TB,
	input []byte,
) *MXLPackage {
	t.Helper()

	value, err := DecodeMXLPackage(bytes.NewReader(input))
	require.NoError(t, err)

	var encodedOnce bytes.Buffer
	err = EncodeMXLPackage(&encodedOnce, value)
	require.NoError(t, err)

	decodedAgain, err := DecodeMXLPackage(
		bytes.NewReader(encodedOnce.Bytes()),
	)
	require.NoError(t, err)
	assert.Equal(
		t,
		value,
		decodedAgain,
		"MXL package model changed after round-trip",
	)

	var encodedTwice bytes.Buffer
	err = EncodeMXLPackage(&encodedTwice, decodedAgain)
	require.NoError(t, err)
	assertStableEncoding(
		t,
		encodedOnce.Bytes(),
		encodedTwice.Bytes(),
	)

	return value
}

func assertStableEncoding(
	t testing.TB,
	encodedOnce []byte,
	encodedTwice []byte,
) {
	t.Helper()

	assert.True(
		t,
		bytes.Equal(encodedOnce, encodedTwice),
		"encoding did not reach a byte-stable fixed point: "+
			"first SHA-256 %x, second SHA-256 %x",
		sha256.Sum256(encodedOnce),
		sha256.Sum256(encodedTwice),
	)
}
