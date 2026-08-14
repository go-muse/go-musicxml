package xsdgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadMusicXMLSchemas(t *testing.T) {
	t.Parallel()

	schemaFS := os.DirFS(filepath.Join(
		"..",
		"..",
		"schema",
		"musicxml-4.0",
	))

	set, err := Load(schemaFS, "musicxml.xsd", "catalog.xml")
	require.NoError(t, err)
	require.NotNil(t, set.Root)

	assert.Equal(
		t,
		[]string{
			"musicxml.xsd",
			"xml.xsd",
			"xlink.xsd",
		},
		schemaPaths(set.Files),
	)

	assert.Equal(t, "", set.Root.Schema.TargetNamespace)
	assert.Equal(t, Namespace, set.Root.Schema.Namespaces["xs"])
	assert.Equal(
		t,
		"http://www.w3.org/1999/xlink",
		set.Root.Schema.Namespaces["xlink"],
	)

	xmlLanguage, err := set.Root.Schema.ResolveQName("xml:lang")
	require.NoError(t, err)
	assert.Equal(
		t,
		QName{
			Namespace: XMLNamespace,
			Local:     "lang",
			Prefix:    "xml",
		},
		xmlLanguage,
	)

	xmlSchema, found := set.Lookup("xml.xsd")
	require.True(t, found)
	assert.Equal(
		t,
		"http://www.w3.org/XML/1998/namespace",
		xmlSchema.Schema.TargetNamespace,
	)

	xlinkSchema, found := set.Lookup("xlink.xsd")
	require.True(t, found)
	assert.Equal(
		t,
		"http://www.w3.org/1999/xlink",
		xlinkSchema.Schema.TargetNamespace,
	)
}

func TestIndexMusicXMLSchemas(t *testing.T) {
	t.Parallel()

	schemaFS := os.DirFS(filepath.Join(
		"..",
		"..",
		"schema",
		"musicxml-4.0",
	))

	set, err := Load(schemaFS, "musicxml.xsd", "catalog.xml")
	require.NoError(t, err)

	index, err := NewIndex(set)
	require.NoError(t, err)

	tests := []struct {
		name     string
		resolve  func() (*Declaration, error)
		wantKind DeclarationKind
		wantFile string
	}{
		{
			name: "local complex type",
			resolve: func() (*Declaration, error) {
				return index.ResolveType(set.Root, "attributes")
			},
			wantKind: DeclarationComplexType,
			wantFile: "musicxml.xsd",
		},
		{
			name: "root element",
			resolve: func() (*Declaration, error) {
				return index.ResolveElement(set.Root, "score-partwise")
			},
			wantKind: DeclarationElement,
			wantFile: "musicxml.xsd",
		},
		{
			name: "built-in simple type",
			resolve: func() (*Declaration, error) {
				return index.ResolveType(set.Root, "xs:string")
			},
			wantKind: DeclarationBuiltinSimpleType,
		},
		{
			name: "local group",
			resolve: func() (*Declaration, error) {
				return index.ResolveGroup(set.Root, "music-data")
			},
			wantKind: DeclarationGroup,
			wantFile: "musicxml.xsd",
		},
		{
			name: "XML attribute",
			resolve: func() (*Declaration, error) {
				return index.ResolveAttribute(set.Root, "xml:lang")
			},
			wantKind: DeclarationAttribute,
			wantFile: "xml.xsd",
		},
		{
			name: "XLink attribute",
			resolve: func() (*Declaration, error) {
				return index.ResolveAttribute(set.Root, "xlink:href")
			},
			wantKind: DeclarationAttribute,
			wantFile: "xlink.xsd",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			declaration, resolveErr := test.resolve()

			require.NoError(t, resolveErr)
			assert.Equal(t, test.wantKind, declaration.Kind)

			if test.wantFile == "" {
				assert.True(t, declaration.Builtin())
				assert.Nil(t, declaration.File)

				return
			}

			assert.False(t, declaration.Builtin())
			require.NotNil(t, declaration.File)
			assert.Equal(t, test.wantFile, declaration.File.Path)
		})
	}
}
