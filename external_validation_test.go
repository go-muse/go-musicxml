package musicxml

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodedCorpusConformsToMusicXMLSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		t.Skip("xmllint is not installed; CI runs this test with libxml2")
	}

	entries, err := os.ReadDir(musicXMLTestSuiteDirectory)
	require.NoError(t, err)
	schemaDirectory, err := filepath.Abs(filepath.Join("schema", "musicxml-4.0"))
	require.NoError(t, err)

	// These upstream fixtures are intentionally invalid before a round trip.
	invalidFixtures := map[string]struct{}{
		"41g-PartNoId.xml":         {},
		"74a-FiguredBass.xml":      {},
		"99d-AccordionInvalid.xml": {},
	}
	checked := 0

	for _, entry := range entries {
		extension := strings.ToLower(filepath.Ext(entry.Name()))
		if entry.IsDir() ||
			(extension != ".xml" && extension != ".musicxml") {
			continue
		}
		if _, invalid := invalidFixtures[entry.Name()]; invalid {
			continue
		}
		checked++

		t.Run(entry.Name(), func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join(
				musicXMLTestSuiteDirectory,
				entry.Name(),
			))
			require.NoError(t, err)

			document, err := Decode(bytes.NewReader(input))
			require.NoError(t, err)

			var encoded bytes.Buffer
			require.NoError(t, Encode(&encoded, document))

			command := exec.Command(
				xmllint,
				"--noout",
				"--schema",
				filepath.Join(schemaDirectory, "musicxml.xsd"),
				"-",
			)
			command.Env = append(
				os.Environ(),
				"XML_CATALOG_FILES="+filepath.Join(
					schemaDirectory,
					"catalog.xml",
				),
			)
			command.Stdin = bytes.NewReader(encoded.Bytes())
			output, err := command.CombinedOutput()
			require.NoErrorf(
				t,
				err,
				"re-encoded document does not conform:\n%s",
				output,
			)
		})
	}

	assert.Equal(t, 146, checked)
}
