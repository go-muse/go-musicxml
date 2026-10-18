package musicxml

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGeneratedDocumentationCarriesNotice checks that every generated model
// file whose comments copy schema documentation starts with the attribution
// the schema licence requires, and that the notices file explains it.
func TestGeneratedDocumentationCarriesNotice(t *testing.T) {
	t.Parallel()

	notice, err := os.ReadFile("internal/xsdgen/notices/musicxml-4.0.txt")
	require.NoError(t, err)
	require.Contains(t, string(notice), "MusicXML 4.0", "the attribution names the specification and its version")
	require.Contains(t, string(notice), "Final Specification Agreement")

	for _, name := range []string{"zz_generated_simple.go", "zz_generated_types.go", "zz_generated_documents.go", "zz_generated_opus.go"} {
		data, err := os.ReadFile(name)
		require.NoError(t, err)
		header, _, found := strings.Cut(string(data), "\npackage ")
		require.True(t, found, name)
		var text []string
		for _, line := range strings.Split(header, "\n") {
			text = append(text, strings.TrimPrefix(line, "//"))
		}
		assert.Contains(t, strings.Join(strings.Fields(strings.Join(text, " ")), " "), strings.Join(strings.Fields(string(notice)), " "), name)
	}
	// Only musicxml.xsd and opus.xsd documentation is copied: text from the
	// imported xml.xsd and xlink.xsd modules appears nowhere.
	for _, name := range []string{"zz_generated_simple.go", "zz_generated_types.go", "zz_generated_documents.go", "zz_generated_opus.go"} {
		data, err := os.ReadFile(name)
		require.NoError(t, err)
		for _, imported := range []string{"for information about this namespace", "schemas wishing to allow", "subset of XLink attributes"} {
			assert.NotContains(t, strings.Join(strings.Fields(strings.ReplaceAll(string(data), "//", " ")), " "), imported, name)
		}
	}
	notices, err := os.ReadFile("THIRD_PARTY_NOTICES.md")
	require.NoError(t, err)
	assert.Contains(t, string(notices), "internal/xsdgen/notices/musicxml-4.0.txt")
}
