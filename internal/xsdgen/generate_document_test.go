package xsdgen

import (
	"errors"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateDocument(t *testing.T) {
	t.Parallel()

	file := parseSchemaFile(t, "opus.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:simpleType name="yes-no">
		<xs:restriction base="xs:token">
			<xs:enumeration value="yes"/>
			<xs:enumeration value="no"/>
		</xs:restriction>
	</xs:simpleType>
	<xs:complexType name="opus">
		<xs:sequence>
			<xs:element name="title" type="xs:string" minOccurs="0"/>
			<xs:choice minOccurs="0" maxOccurs="unbounded">
				<xs:element name="opus" type="opus"/>
				<xs:element name="opus-link" type="opus-link"/>
				<xs:element name="score" type="score"/>
			</xs:choice>
		</xs:sequence>
		<xs:attribute name="version" type="xs:token"/>
	</xs:complexType>
	<xs:complexType name="opus-link">
		<xs:attribute name="href" type="xs:string" use="required"/>
	</xs:complexType>
	<xs:complexType name="score">
		<xs:attribute name="href" type="xs:string" use="required"/>
		<xs:attribute name="new-page" type="yes-no"/>
	</xs:complexType>
	<xs:element name="opus" type="opus"/>
</xs:schema>`)

	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)

	actual, err := GenerateDocument(
		index,
		"example",
		DocumentGenerationOptions{
			Element: QName{Local: "opus"},
			GoName:  "OpusDocument",
			TypeNames: []TypeNameOverride{{
				Name:   QName{Local: "score"},
				GoName: "OpusScore",
			}},
			ExternalTypes: []QName{{Local: "yes-no"}},
		},
	)
	require.NoError(t, err)

	parsed, err := parser.ParseFile(
		token.NewFileSet(),
		"zz_generated_opus.go",
		actual,
		parser.SkipObjectResolution,
	)
	require.NoError(t, err)
	assert.Equal(t, 4, countTypeDeclarations(parsed))

	source := string(actual)
	assert.NotContains(t, source, "type YesNo ")
	assert.Contains(t, source, "type OpusDocument struct {")
	assert.Contains(t, source, "`xml:\"opus\"`")
	assert.Contains(
		t,
		source,
		"Content []OpusDocumentContent `xml:\",any\"`",
	)
	assert.Contains(t, source, "Opus     *OpusDocument")
	assert.Contains(t, source, "OpusLink *OpusLink")
	assert.Contains(t, source, "Score    *OpusScore")
	assert.Contains(t, source, "NewPage *YesNo")
}

func TestGenerateDocumentError(t *testing.T) {
	t.Parallel()

	file := parseSchemaFile(t, "document.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:complexType name="document"/>
	<xs:simpleType name="value">
		<xs:restriction base="xs:string"/>
	</xs:simpleType>
	<xs:element name="document" type="document"/>
	<xs:element name="simple" type="value"/>
</xs:schema>`)
	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)

	tests := []struct {
		name    string
		index   *Index
		options DocumentGenerationOptions
		wantErr error
	}{
		{
			name: "nil index",
			options: DocumentGenerationOptions{
				Element: QName{Local: "document"},
				GoName:  "Document",
			},
			wantErr: ErrNilIndex,
		},
		{
			name:  "invalid root Go name",
			index: index,
			options: DocumentGenerationOptions{
				Element: QName{Local: "document"},
				GoName:  "document",
			},
			wantErr: ErrInvalidDocumentGeneration,
		},
		{
			name:  "unknown root",
			index: index,
			options: DocumentGenerationOptions{
				Element: QName{Local: "missing"},
				GoName:  "Missing",
			},
			wantErr: ErrUnresolvedReference,
		},
		{
			name:  "simple root",
			index: index,
			options: DocumentGenerationOptions{
				Element: QName{Local: "simple"},
				GoName:  "Simple",
			},
			wantErr: ErrInvalidDocumentGeneration,
		},
		{
			name:  "unknown external type",
			index: index,
			options: DocumentGenerationOptions{
				Element:       QName{Local: "document"},
				GoName:        "Document",
				ExternalTypes: []QName{{Local: "missing"}},
			},
			wantErr: ErrUnresolvedReference,
		},
		{
			name:  "root type is external",
			index: index,
			options: DocumentGenerationOptions{
				Element:       QName{Local: "document"},
				GoName:        "Document",
				ExternalTypes: []QName{{Local: "document"}},
			},
			wantErr: ErrInvalidDocumentGeneration,
		},
		{
			name:  "conflicting root name",
			index: index,
			options: DocumentGenerationOptions{
				Element: QName{Local: "document"},
				GoName:  "Document",
				TypeNames: []TypeNameOverride{{
					Name:   QName{Local: "document"},
					GoName: "OtherDocument",
				}},
			},
			wantErr: ErrInvalidDocumentGeneration,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			source, err := GenerateDocument(
				test.index,
				"example",
				test.options,
			)

			assert.Nil(t, source)
			assert.True(t, errors.Is(err, test.wantErr))
		})
	}
}

func TestGenerateDocumentDeterministic(t *testing.T) {
	t.Parallel()

	file := parseSchemaFile(t, "document.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:complexType name="document">
		<xs:sequence>
			<xs:element name="child" type="child"/>
		</xs:sequence>
	</xs:complexType>
	<xs:complexType name="child"/>
	<xs:element name="document" type="document"/>
</xs:schema>`)
	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)

	first, err := GenerateDocument(
		index,
		"example",
		DocumentGenerationOptions{
			Element: QName{Local: "document"},
			GoName:  "Document",
		},
	)
	require.NoError(t, err)
	second, err := GenerateDocument(
		index,
		"example",
		DocumentGenerationOptions{
			Element: QName{Local: "document"},
			GoName:  "Document",
		},
	)
	require.NoError(t, err)

	assert.Equal(t, string(first), string(second))
	assert.Less(
		t,
		strings.Index(string(first), "type Child struct"),
		strings.Index(string(first), "type Document struct"),
	)
}
