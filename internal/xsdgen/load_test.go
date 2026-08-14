package xsdgen

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	set, err := Load(schemaSetFS(), "schemas/root.xsd", "schemas/catalog.xml")
	require.NoError(t, err)
	require.NotNil(t, set.Root)

	assert.Equal(t, "schemas/root.xsd", set.Root.Path)
	assert.Equal(
		t,
		[]string{
			"schemas/root.xsd",
			"schemas/base.xsd",
			"schemas/included.xsd",
		},
		schemaPaths(set.Files),
	)

	file, found := set.Lookup("schemas/base.xsd")
	require.True(t, found)
	assert.Equal(t, "urn:base", file.Schema.TargetNamespace)

	_, found = set.Lookup("schemas/missing.xsd")
	assert.False(t, found)
}

func TestLoadError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		fsys        fs.FS
		rootPath    string
		catalogPath string
		wantErr     error
	}{
		{
			name:        "nil filesystem",
			rootPath:    "schemas/root.xsd",
			catalogPath: "schemas/catalog.xml",
			wantErr:     ErrNilFS,
		},
		{
			name: "root outside schema directory",
			fsys: fstest.MapFS{
				"root.xsd":            schemaFile("urn:root", ""),
				"schemas/catalog.xml": catalogFile(),
			},
			rootPath:    "root.xsd",
			catalogPath: "schemas/catalog.xml",
			wantErr:     ErrInvalidSchemaPath,
		},
		{
			name: "unresolved absolute location",
			fsys: fstest.MapFS{
				"schemas/catalog.xml": catalogFile(),
				"schemas/root.xsd": schemaFile(
					"urn:root",
					`<xs:import namespace="urn:dependency" `+
						`schemaLocation="https://schemas.example/unknown.xsd"/>`,
				),
			},
			rootPath:    "schemas/root.xsd",
			catalogPath: "schemas/catalog.xml",
			wantErr:     ErrUnresolvedSchemaLocation,
		},
		{
			name: "path escapes schema directory",
			fsys: fstest.MapFS{
				"schemas/catalog.xml": catalogFile(),
				"schemas/root.xsd": schemaFile(
					"urn:root",
					`<xs:include schemaLocation="../outside.xsd"/>`,
				),
				"outside.xsd": schemaFile("urn:root", ""),
			},
			rootPath:    "schemas/root.xsd",
			catalogPath: "schemas/catalog.xml",
			wantErr:     ErrInvalidSchemaPath,
		},
		{
			name: "import namespace mismatch",
			fsys: fstest.MapFS{
				"schemas/catalog.xml": catalogFile(),
				"schemas/root.xsd": schemaFile(
					"urn:root",
					`<xs:import namespace="urn:expected" `+
						`schemaLocation="https://schemas.example/base.xsd"/>`,
				),
				"schemas/base.xsd": schemaFile("urn:actual", ""),
			},
			rootPath:    "schemas/root.xsd",
			catalogPath: "schemas/catalog.xml",
			wantErr:     ErrNamespaceMismatch,
		},
		{
			name: "mapped schema is missing",
			fsys: fstest.MapFS{
				"schemas/catalog.xml": catalogFile(),
				"schemas/root.xsd": schemaFile(
					"urn:root",
					`<xs:import namespace="urn:base" `+
						`schemaLocation="https://schemas.example/base.xsd"/>`,
				),
			},
			rootPath:    "schemas/root.xsd",
			catalogPath: "schemas/catalog.xml",
			wantErr:     fs.ErrNotExist,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			set, err := Load(
				test.fsys,
				test.rootPath,
				test.catalogPath,
			)

			assert.Nil(t, set)
			assert.ErrorIs(t, err, test.wantErr)
		})
	}
}

func schemaSetFS() fstest.MapFS {
	return fstest.MapFS{
		"schemas/catalog.xml": catalogFile(),
		"schemas/root.xsd": schemaFile(
			"urn:root",
			`<xs:import namespace="urn:base" `+
				`schemaLocation="https://schemas.example/base.xsd"/>`,
		),
		"schemas/base.xsd": schemaFile(
			"urn:base",
			`<xs:include schemaLocation="included.xsd"/>`,
		),
		"schemas/included.xsd": schemaFile(
			"urn:base",
			`<xs:import namespace="urn:root" `+
				`schemaLocation="https://schemas.example/root.xsd"/>`,
		),
	}
}

func catalogFile() *fstest.MapFile {
	return &fstest.MapFile{
		Data: []byte(`
<catalog xmlns="urn:oasis:names:tc:entity:xmlns:xml:catalog">
	<group>
		<uri
			name="https://schemas.example/root.xsd"
			uri="root.xsd"/>
		<uri
			name="https://schemas.example/base.xsd"
			uri="base.xsd"/>
	</group>
</catalog>`),
	}
}

func schemaFile(
	targetNamespace string,
	content string,
) *fstest.MapFile {
	return &fstest.MapFile{
		Data: []byte(
			`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" ` +
				`targetNamespace="` + targetNamespace + `">` +
				content +
				`</xs:schema>`,
		),
	}
}

func schemaPaths(files []*SchemaFile) []string {
	result := make([]string, len(files))

	for index, file := range files {
		result[index] = file.Path
	}

	return result
}
