package xsdgen

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// typeCheckGenerated type-checks generated sources as one package. Helpers
// that the destination package supplies by hand (xml… numeric adapters,
// normalizeValidationWhitespace) are not part of the fixture, so only errors
// about them are tolerated.
func typeCheckGenerated(t *testing.T, sources ...[]byte) {
	t.Helper()
	fset := token.NewFileSet()
	var files []*ast.File
	for index, source := range sources {
		file, err := parser.ParseFile(fset, filepath.Join("generated", string(rune('a'+index))+".go"), source, 0)
		require.NoError(t, err)
		files = append(files, file)
	}
	var failures []string
	config := types.Config{Importer: importer.ForCompiler(fset, "source", nil), Error: func(err error) {
		message := err.Error()
		if strings.Contains(message, "undefined: xml") || strings.Contains(message, "undefined: normalizeValidationWhitespace") {
			return
		}
		failures = append(failures, message)
	}}
	_, _ = config.Check("example", fset, files, nil)
	assert.Empty(t, failures, "generated code does not type-check")
}

func TestEnumerationHelpersShareGoValues(t *testing.T) {
	t.Parallel()

	index, err := NewIndex(&Set{Files: []*SchemaFile{parseSchemaFile(t, "types.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:simpleType name="rank">
		<xs:restriction base="xs:integer">
			<xs:enumeration value="1"/>
			<xs:enumeration value="01"/>
		</xs:restriction>
	</xs:simpleType>
	<xs:simpleType name="level">
		<xs:restriction base="xs:double">
			<xs:enumeration value="0"/>
			<xs:enumeration value="-0.0"/>
		</xs:restriction>
	</xs:simpleType>
	<xs:simpleType name="flag">
		<xs:restriction base="xs:boolean">
			<xs:enumeration value="true"/>
			<xs:enumeration value="1"/>
		</xs:restriction>
	</xs:simpleType>
</xs:schema>`)}})
	require.NoError(t, err)
	actual, err := GenerateSimpleTypes(index, "example")
	require.NoError(t, err)
	typeCheckGenerated(t, actual)
	assert.Contains(t, string(actual), "return []Rank{Rank1, Rank01}", "Values keeps every constant in schema order")
	assert.Contains(t, string(actual), "\tcase Rank1:\n", "IsKnown lists each Go value once")
}

func TestChoiceVariantsAvoidAccessorNames(t *testing.T) {
	t.Parallel()

	index, err := NewIndex(&Set{Files: []*SchemaFile{parseSchemaFile(t, "types.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:complexType name="box">
		<xs:choice maxOccurs="unbounded">
			<xs:element name="value" type="xs:string"/>
			<xs:element name="element-name" type="xs:string"/>
		</xs:choice>
	</xs:complexType>
</xs:schema>`)}})
	require.NoError(t, err)
	actual, err := GenerateComplexTypes(index, "example")
	require.NoError(t, err)
	typeCheckGenerated(t, actual)
	assert.Contains(t, string(actual), "ValueElement       *string")
	assert.Contains(t, string(actual), "func (value BoxContent) ElementName() string")
	assert.Contains(t, string(actual), `name, selected, count = "value", value.ValueElement, count+1`)
}

const choiceRootSchema = `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:element name="root">
		<xs:complexType>
			<xs:choice maxOccurs="unbounded">
				<xs:element name="item">
					<xs:complexType>
						<xs:attribute name="id" type="xs:string"/>
					</xs:complexType>
				</xs:element>
				<xs:element name="label" type="xs:string"/>
			</xs:choice>
		</xs:complexType>
	</xs:element>
</xs:schema>`

// TestNameManifestKeysAnonymousChoiceVariants renames a choice wrapper: the
// anonymous type of its variant keeps its key and its own listed name.
func TestNameManifestKeysAnonymousChoiceVariants(t *testing.T) {
	t.Parallel()

	generate := func(manifest *NameManifest) []byte {
		index, err := NewIndex(&Set{Files: []*SchemaFile{parseSchemaFile(t, "types.xsd", choiceRootSchema)}})
		require.NoError(t, err)
		index.SetNameManifest(manifest)
		actual, err := GenerateElements(index, "example", QName{Local: "root"})
		require.NoError(t, err)
		return actual
	}
	first := NewNameManifest()
	generate(first)
	require.Equal(t, "RootContentItem", first.Added()["types.xsd element root content element item"])

	renamed, err := ReadNameManifest(strings.NewReader(strings.Replace(string(first.Format()), "types.xsd element root content\tRootContent\n", "types.xsd element root content\tRenamedContent\n", 1)))
	require.NoError(t, err)
	actual := generate(renamed)
	typeCheckGenerated(t, actual)
	assert.Contains(t, string(actual), "type RenamedContent struct")
	assert.Contains(t, string(actual), "type RootContentItem struct", "the variant keeps its listed name")
	assert.Empty(t, renamed.Added())
}

// TestNameManifestRenamesExternalType renames yes-no in the shared manifest:
// the opus generation, which treats it as external, refers to the new name.
func TestNameManifestRenamesExternalType(t *testing.T) {
	t.Parallel()

	committed, err := os.ReadFile(filepath.Join("names", "musicxml-4.0.txt"))
	require.NoError(t, err)
	manifest, err := ReadNameManifest(strings.NewReader(strings.Replace(string(committed), "musicxml.xsd type yes-no\tYesNo\n", "musicxml.xsd type yes-no\tBooleanWord\n", 1)))
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
	assert.Contains(t, string(actual), "NewPage *BooleanWord")
	assert.NotRegexp(t, `\bYesNo\b`, string(actual))
	assert.Empty(t, manifest.Added())
}

// TestNameManifestExternalTypeOfSameSchema emits a type in one generation and
// names it as external in another generation of the same schema: the second
// reuses the component's own entry.
func TestNameManifestExternalTypeOfSameSchema(t *testing.T) {
	t.Parallel()

	file := parseSchemaFile(t, "types.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:simpleType name="state">
		<xs:restriction base="xs:token">
			<xs:enumeration value="open"/>
		</xs:restriction>
	</xs:simpleType>
	<xs:element name="root">
		<xs:complexType>
			<xs:attribute name="state" type="state"/>
		</xs:complexType>
	</xs:element>
</xs:schema>`)
	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)
	manifest := NewNameManifest()
	index.SetNameManifest(manifest)
	simple, err := GenerateSimpleTypes(index, "example")
	require.NoError(t, err)
	document, err := GenerateDocument(index, "example", DocumentGenerationOptions{
		Element:       QName{Local: "root"},
		GoName:        "Root",
		ExternalTypes: []QName{{Local: "state"}},
	})
	require.NoError(t, err)
	typeCheckGenerated(t, simple, document)
	assert.Regexp(t, `State +\*State `, string(document))
}
