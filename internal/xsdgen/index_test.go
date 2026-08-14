package xsdgen

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexResolve(t *testing.T) {
	t.Parallel()

	root := parseSchemaFile(t, "root.xsd", `
<xs:schema
	xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:base="urn:base">
	<xs:simpleType name="local-simple">
		<xs:restriction base="xs:string"/>
	</xs:simpleType>
	<xs:complexType name="local-complex"/>
	<xs:element name="root" type="local-complex"/>
</xs:schema>`)
	base := parseSchemaFile(t, "base.xsd", `
<xs:schema
	xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:base="urn:base"
	targetNamespace="urn:base">
	<xs:simpleType name="shared-simple">
		<xs:restriction base="xs:string"/>
	</xs:simpleType>
	<xs:complexType name="shared-complex"/>
	<xs:element name="shared-element" type="base:shared-complex"/>
	<xs:attribute name="shared-attribute" type="xs:string"/>
	<xs:group name="shared-group">
		<xs:sequence>
			<xs:element ref="base:shared-element"/>
		</xs:sequence>
	</xs:group>
	<xs:attributeGroup name="shared-attributes">
		<xs:attribute ref="base:shared-attribute"/>
	</xs:attributeGroup>
</xs:schema>`)

	index, err := NewIndex(&Set{
		Root:  root,
		Files: []*SchemaFile{root, base},
	})
	require.NoError(t, err)

	tests := []struct {
		name     string
		resolve  func() (*Declaration, error)
		wantName QName
		wantKind DeclarationKind
	}{
		{
			name: "local type",
			resolve: func() (*Declaration, error) {
				return index.ResolveType(root, "local-complex")
			},
			wantName: QName{Local: "local-complex"},
			wantKind: DeclarationComplexType,
		},
		{
			name: "imported type",
			resolve: func() (*Declaration, error) {
				return index.ResolveType(root, "base:shared-simple")
			},
			wantName: QName{
				Namespace: "urn:base",
				Local:     "shared-simple",
			},
			wantKind: DeclarationSimpleType,
		},
		{
			name: "built-in type",
			resolve: func() (*Declaration, error) {
				return index.ResolveType(root, "xs:string")
			},
			wantName: QName{
				Namespace: Namespace,
				Local:     "string",
				Prefix:    "xs",
			},
			wantKind: DeclarationBuiltinSimpleType,
		},
		{
			name: "element ref",
			resolve: func() (*Declaration, error) {
				return index.ResolveElement(root, "base:shared-element")
			},
			wantName: QName{
				Namespace: "urn:base",
				Local:     "shared-element",
			},
			wantKind: DeclarationElement,
		},
		{
			name: "attribute ref",
			resolve: func() (*Declaration, error) {
				return index.ResolveAttribute(root, "base:shared-attribute")
			},
			wantName: QName{
				Namespace: "urn:base",
				Local:     "shared-attribute",
			},
			wantKind: DeclarationAttribute,
		},
		{
			name: "group ref",
			resolve: func() (*Declaration, error) {
				return index.ResolveGroup(root, "base:shared-group")
			},
			wantName: QName{
				Namespace: "urn:base",
				Local:     "shared-group",
			},
			wantKind: DeclarationGroup,
		},
		{
			name: "attribute group ref",
			resolve: func() (*Declaration, error) {
				return index.ResolveAttributeGroup(
					root,
					"base:shared-attributes",
				)
			},
			wantName: QName{
				Namespace: "urn:base",
				Local:     "shared-attributes",
			},
			wantKind: DeclarationAttributeGroup,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual, resolveErr := test.resolve()

			require.NoError(t, resolveErr)
			assert.Equal(t, test.wantName, actual.Name)
			assert.Equal(t, test.wantKind, actual.Kind)
		})
	}
}

func TestIndexLookupIgnoresSourcePrefix(t *testing.T) {
	t.Parallel()

	file := parseSchemaFile(t, "base.xsd", `
<xs:schema
	xmlns:xs="http://www.w3.org/2001/XMLSchema"
	targetNamespace="urn:base">
	<xs:complexType name="value"/>
</xs:schema>`)

	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)

	declaration, found := index.LookupType(QName{
		Namespace: "urn:base",
		Local:     "value",
		Prefix:    "different",
	})

	require.True(t, found)
	assert.Equal(t, DeclarationComplexType, declaration.Kind)
}

func TestIndexResolveError(t *testing.T) {
	t.Parallel()

	file := parseSchemaFile(t, "root.xsd", `
<xs:schema
	xmlns:xs="http://www.w3.org/2001/XMLSchema"
	xmlns:base="urn:base"/>
`)
	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)

	tests := []struct {
		name    string
		index   *Index
		file    *SchemaFile
		value   string
		wantErr error
	}{
		{
			name:    "unresolved declaration",
			index:   index,
			file:    file,
			value:   "base:missing",
			wantErr: ErrUnresolvedReference,
		},
		{
			name:    "undeclared prefix",
			index:   index,
			file:    file,
			value:   "other:missing",
			wantErr: ErrUndeclaredPrefix,
		},
		{
			name:    "invalid source file",
			index:   index,
			value:   "xs:string",
			wantErr: ErrInvalidSchemaFile,
		},
		{
			name:    "nil index",
			file:    file,
			value:   "xs:string",
			wantErr: ErrNilIndex,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			declaration, resolveErr := test.index.ResolveType(
				test.file,
				test.value,
			)

			assert.Nil(t, declaration)
			assert.ErrorIs(t, resolveErr, test.wantErr)
		})
	}
}

func TestNewIndexError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		set     *Set
		wantErr error
	}{
		{
			name:    "nil set",
			wantErr: ErrNilSet,
		},
		{
			name: "invalid schema file",
			set: &Set{
				Files: []*SchemaFile{nil},
			},
			wantErr: ErrInvalidSchemaFile,
		},
		{
			name: "unnamed declaration",
			set: &Set{
				Files: []*SchemaFile{
					parseSchemaFile(t, "unnamed.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:element type="xs:string"/>
</xs:schema>`),
				},
			},
			wantErr: ErrUnnamedDeclaration,
		},
		{
			name: "simple and complex type share a name",
			set: &Set{
				Files: []*SchemaFile{
					parseSchemaFile(t, "duplicate.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:simpleType name="value">
		<xs:restriction base="xs:string"/>
	</xs:simpleType>
	<xs:complexType name="value"/>
</xs:schema>`),
				},
			},
			wantErr: ErrDuplicateDeclaration,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			index, err := NewIndex(test.set)

			assert.Nil(t, index)
			assert.ErrorIs(t, err, test.wantErr)
		})
	}
}

func parseSchemaFile(
	t *testing.T,
	path string,
	input string,
) *SchemaFile {
	t.Helper()

	schema, err := Parse(strings.NewReader(input))
	require.NoError(t, err)

	return &SchemaFile{
		Path:   path,
		Schema: schema,
	}
}
