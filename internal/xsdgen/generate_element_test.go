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

func TestGenerateElements(t *testing.T) {
	t.Parallel()

	file := parseSchemaFile(t, "elements.xsd", `
<xs:schema
	xmlns:xs="http://www.w3.org/2001/XMLSchema"
	targetNamespace="urn:score"
	elementFormDefault="qualified">
	<xs:element name="score">
		<xs:complexType>
			<xs:sequence>
				<xs:element name="part" maxOccurs="unbounded">
					<xs:complexType>
						<xs:choice minOccurs="0" maxOccurs="unbounded">
							<xs:element name="note" type="xs:string"/>
							<xs:element name="backup" type="xs:decimal"/>
						</xs:choice>
						<xs:attribute name="id" type="xs:ID" use="required"/>
					</xs:complexType>
				</xs:element>
			</xs:sequence>
			<xs:attribute name="version" type="xs:token"/>
		</xs:complexType>
	</xs:element>
</xs:schema>`)

	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)

	actual, err := GenerateElements(
		index,
		"example",
		QName{Namespace: "urn:score", Local: "score"},
	)

	require.NoError(t, err)
	_, err = parser.ParseFile(
		token.NewFileSet(),
		"zz_generated_elements.go",
		actual,
		parser.SkipObjectResolution,
	)
	require.NoError(t, err)

	source := string(actual)
	assert.Contains(t, source, "type Score struct {")
	assert.Contains(t, source, "XMLName xml.Name")
	assert.Contains(t, source, "`xml:\"urn:score score\"`")
	assert.Contains(t, source, "Part    []ScorePart")
	assert.Contains(t, source, "`xml:\"urn:score part\"`")
	assert.Contains(t, source, "type ScorePart struct {")
	assert.Contains(
		t,
		source,
		"Content ScorePartContents `xml:\",any\"`",
	)
	assert.Contains(t, source, "type ScorePartContents []ScorePartContent")
	assert.Contains(t, source, "type ScorePartContent struct {")
	assert.Contains(
		t,
		source,
		"func (value ScorePartContent) MarshalXML(",
	)
}

func TestGenerateElementsDeterministicOrder(t *testing.T) {
	t.Parallel()

	file := parseSchemaFile(t, "elements.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:element name="alpha" type="xs:string"/>
	<xs:element name="beta" type="xs:string"/>
</xs:schema>`)

	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)

	actual, err := GenerateElements(
		index,
		"example",
		QName{Local: "beta"},
		QName{Local: "alpha"},
	)

	require.NoError(t, err)
	assert.Less(
		t,
		strings.Index(string(actual), "type Alpha struct"),
		strings.Index(string(actual), "type Beta struct"),
	)
}

func TestGenerateElementsError(t *testing.T) {
	t.Parallel()

	file := parseSchemaFile(t, "elements.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	<xs:element name="score" type="xs:string"/>
</xs:schema>`)
	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)

	tests := []struct {
		name        string
		index       *Index
		packageName string
		elements    []QName
		wantErr     error
	}{
		{
			name:        "nil index",
			packageName: "example",
			elements:    []QName{{Local: "score"}},
			wantErr:     ErrNilIndex,
		},
		{
			name:        "invalid package",
			index:       index,
			packageName: "invalid-package",
			elements:    []QName{{Local: "score"}},
			wantErr:     ErrInvalidPackageName,
		},
		{
			name:        "no elements",
			index:       index,
			packageName: "example",
			wantErr:     ErrNoElements,
		},
		{
			name:        "unknown element",
			index:       index,
			packageName: "example",
			elements:    []QName{{Local: "missing"}},
			wantErr:     ErrUnresolvedReference,
		},
		{
			name:        "duplicate element",
			index:       index,
			packageName: "example",
			elements: []QName{
				{Local: "score"},
				{Local: "score"},
			},
			wantErr: ErrInvalidElement,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := GenerateElements(
				test.index,
				test.packageName,
				test.elements...,
			)

			assert.Error(t, err)
			assert.True(t, errors.Is(err, test.wantErr))
		})
	}
}
