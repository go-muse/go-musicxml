package xsdgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadNameManifestRejectsAmbiguity(t *testing.T) {
	t.Parallel()

	for name, text := range map[string]string{
		"reassigned name": "a.xsd type one\tOne\na.xsd type two\tOne\n",
		"duplicate key":   "a.xsd type one\tOne\na.xsd type one\tUno\n",
		"unexported name": "a.xsd type one\tone\n",
		"missing name":    "a.xsd type one\n",
	} {
		_, err := ReadNameManifest(strings.NewReader(text))
		assert.ErrorIs(t, err, ErrNameManifest, name)
	}
	crlf, err := ReadNameManifest(strings.NewReader("a.xsd type one\tOne\r\n"))
	require.NoError(t, err, "a CRLF checkout reads like LF")
	assert.Equal(t, "a.xsd type one\tOne\n", string(crlf.Format()))
	manifest, err := ReadNameManifest(strings.NewReader("b.xsd type two\tTwo\na.xsd type one\tOne\n"))
	require.NoError(t, err)
	assert.Equal(t, "a.xsd type one\tOne\nb.xsd type two\tTwo\n", string(manifest.Format()), "Format sorts by key")
}

func TestNameManifestAssign(t *testing.T) {
	t.Parallel()

	manifest, err := ReadNameManifest(strings.NewReader("a.xsd type state\tStatus\na.xsd type taken\tTaken\n"))
	require.NoError(t, err)

	// A listed component keeps its name whatever the derivation says.
	name, err := manifest.Assign("a.xsd type state", "State")
	require.NoError(t, err)
	assert.Equal(t, "Status", name)

	// A new component whose derived name is taken gets a suffix, also when
	// the taker is new itself.
	name, err = manifest.Assign("a.xsd type other enumeration \"x\"", "Taken")
	require.NoError(t, err)
	assert.Equal(t, "Taken2", name)
	name, err = manifest.Assign("a.xsd type third enumeration \"x\"", "Taken")
	require.NoError(t, err)
	assert.Equal(t, "Taken3", name)
	name, err = manifest.Assign("a.xsd type other enumeration \"x\"", "Other")
	require.NoError(t, err)
	assert.Equal(t, "Taken2", name, "a component is named once")
	name, err = manifest.Assign("a.xsd type flat enumeration \"1\"", "Taken3")
	require.NoError(t, err)
	assert.Equal(t, "Taken3_2", name, "a name ending in a digit takes its suffix after an underscore")
	delete(manifest.added, "a.xsd type flat enumeration \"1\"")

	assert.Equal(t, map[string]string{
		"a.xsd type other enumeration \"x\"": "Taken2",
		"a.xsd type third enumeration \"x\"": "Taken3",
	}, manifest.Added())
	err = manifest.Incomplete()
	require.ErrorIs(t, err, ErrNameManifest)
	assert.Contains(t, err.Error(), "-update-names")

	var unset *NameManifest
	name, err = unset.Assign("any", "Derived")
	require.NoError(t, err)
	assert.Equal(t, "Derived", name, "no manifest keeps derived names")
	assert.NoError(t, unset.Incomplete())
}

// TestGenerateWithNameManifest renames a type through the manifest and gives
// a new constant that collides with a type name a suffix.
func TestGenerateWithNameManifest(t *testing.T) {
	t.Parallel()

	file := parseSchemaFile(t, "types.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:simpleType name="state">
		<xs:restriction base="xs:token">
			<xs:enumeration value="open"/>
		</xs:restriction>
	</xs:simpleType>
	<xs:simpleType name="start-stop">
		<xs:restriction base="xs:token">
			<xs:enumeration value="continue"/>
		</xs:restriction>
	</xs:simpleType>
	<xs:simpleType name="start-stop-continue">
		<xs:restriction base="xs:token"/>
	</xs:simpleType>
</xs:schema>`)
	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)
	manifest, err := ReadNameManifest(strings.NewReader("types.xsd type state\tStatus\n"))
	require.NoError(t, err)
	index.SetNameManifest(manifest)

	actual, err := GenerateSimpleTypes(index, "example")
	require.NoError(t, err)
	assert.Contains(t, string(actual), "type Status string")
	assert.Contains(t, string(actual), "StatusOpen Status = \"open\"", "constants follow the listed type name")
	assert.Contains(t, string(actual), "type StartStopContinue string")
	assert.Contains(t, string(actual), "StartStopContinue2 StartStop = \"continue\"", "the colliding new constant gets a suffix")
	assert.Equal(t, "StartStopContinue2", manifest.Added()["types.xsd type start-stop enumeration \"continue\""])
}

// TestMusicXMLNameManifestMatchesGeneration runs the four model generations
// of generate.go with the committed manifest: they assign exactly its keys,
// add none and leave it in canonical form.
func TestMusicXMLNameManifestMatchesGeneration(t *testing.T) {
	t.Parallel()

	manifestPath := filepath.Join("names", "musicxml-4.0.txt")
	committed, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	manifest, err := LoadNameManifest(manifestPath)
	require.NoError(t, err)
	assert.Equal(t, string(committed), string(manifest.Format()), "the manifest is sorted and canonical")

	schemaFS := os.DirFS(filepath.Join("..", "..", "schema", "musicxml-4.0"))
	index := func(schema string) *Index {
		set, err := Load(schemaFS, schema, "catalog.xml")
		require.NoError(t, err)
		index, err := NewIndex(set)
		require.NoError(t, err)
		index.SetNameManifest(manifest)
		return index
	}
	_, err = GenerateSimpleTypes(index("musicxml.xsd"), "musicxml")
	require.NoError(t, err)
	_, err = GenerateComplexTypesWithOptions(index("musicxml.xsd"), "musicxml", ComplexGenerationOptions{
		ModelMetadata:       true,
		OrderedContentTypes: []QName{{Local: "credit"}, {Local: "lyric"}, {Local: "metronome"}},
	})
	require.NoError(t, err)
	_, err = GenerateElementsWithModelMetadata(index("musicxml.xsd"), "musicxml", QName{Local: "score-partwise"}, QName{Local: "score-timewise"})
	require.NoError(t, err)
	_, err = GenerateDocument(index("opus.xsd"), "musicxml", DocumentGenerationOptions{
		ModelMetadata: true,
		Element:       QName{Local: "opus"},
		GoName:        "OpusDocument",
		TypeNames:     []TypeNameOverride{{Name: QName{Local: "score"}, GoName: "OpusScore"}},
		ExternalTypes: []QName{{Local: "yes-no"}},
	})
	require.NoError(t, err)

	assert.Empty(t, manifest.Added(), "every generated package-scope name is listed")
	assert.Equal(t, manifest.Keys(), manifest.Used(), "every listed name is generated")
}

// useMusicXMLGenerationInputs gives index the documentation notice and the
// name manifest that generate.go passes to the model generations.
func useMusicXMLGenerationInputs(t *testing.T, index *Index) {
	t.Helper()
	notice, err := os.ReadFile(filepath.Join("notices", "musicxml-4.0.txt"))
	require.NoError(t, err)
	index.SetDocumentationNotice(string(notice))
	manifest, err := LoadNameManifest(filepath.Join("names", "musicxml-4.0.txt"))
	require.NoError(t, err)
	index.SetNameManifest(manifest)
}

// TestNameManifestRenamesDocumentRoot renames the named root type of the opus
// document through the manifest: the root struct and every reference take the
// new name, while its choice types keep their own listed names.
func TestNameManifestRenamesDocumentRoot(t *testing.T) {
	t.Parallel()

	committed, err := os.ReadFile(filepath.Join("names", "musicxml-4.0.txt"))
	require.NoError(t, err)
	renamed := strings.Replace(string(committed), "opus.xsd type opus\tOpusDocument\n", "opus.xsd type opus\tOpusDoc\n", 1)
	require.NotEqual(t, string(committed), renamed)
	manifest, err := ReadNameManifest(strings.NewReader(renamed))
	require.NoError(t, err)

	set, err := Load(os.DirFS(filepath.Join("..", "..", "schema", "musicxml-4.0")), "opus.xsd", "catalog.xml")
	require.NoError(t, err)
	index, err := NewIndex(set)
	require.NoError(t, err)
	index.SetNameManifest(manifest)
	actual, err := GenerateDocument(index, "musicxml", DocumentGenerationOptions{
		Element:       QName{Local: "opus"},
		GoName:        "OpusDocument",
		TypeNames:     []TypeNameOverride{{Name: QName{Local: "score"}, GoName: "OpusScore"}},
		ExternalTypes: []QName{{Local: "yes-no"}},
	})
	require.NoError(t, err)
	assert.Contains(t, string(actual), "type OpusDoc struct")
	assert.Contains(t, string(actual), "Opus     *OpusDoc\n")
	assert.NotContains(t, string(actual), "type OpusDocument struct")
	assert.NotRegexp(t, `\bOpusDocument\b`, string(actual), "no reference to the old name remains")
	assert.Contains(t, string(actual), "type OpusDocumentContent struct", "a listed choice type keeps its name")
	assert.Empty(t, manifest.Added(), "the rename needs no new entries")
}
