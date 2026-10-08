package musicxml

import (
	"bytes"
	"encoding/xml"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationNamespaceDeclarationProvenance(t *testing.T) {
	t.Parallel()
	const source = `<root xmlns="urn:root" xmlns:p="xmlns" xmlns:rubbish="urn:unused" p:rubbish="root" xml:lang="en"><scope xmlns="" xmlns:p="urn:foreign" p:rubbish="child"><value xmlns:p="xmlns" p:rubbish="nested"/></scope><empty/><value p:rubbish="sibling" xmlns:xml="http://www.w3.org/XML/1998/namespace"/></root>`
	root, err := parseValidationDocument([]byte(source))
	require.NoError(t, err)
	require.Len(t, root.Children, 3)
	require.Len(t, root.Children[0].Children, 1)
	for _, test := range []struct {
		name         string
		node         *validationNode
		namespace    string
		declarations []string
		ordinary     []string
	}{
		{"root", root, "urn:root", []string{"xmlns", "p", "rubbish"}, []string{"/root/@{xmlns}rubbish", "/root/@{http://www.w3.org/XML/1998/namespace}lang"}},
		{"scope", root.Children[0], "", []string{"xmlns", "p"}, []string{"/scope/@{urn:foreign}rubbish"}},
		{"nested", root.Children[0].Children[0], "", []string{"p"}, []string{"/value/@{xmlns}rubbish"}},
		{"empty", root.Children[1], "urn:root", nil, nil},
		{"sibling", root.Children[2], "urn:root", []string{"xml"}, []string{"/value/@{xmlns}rubbish"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.namespace, test.node.Name.Space)
			var declarations, ordinary []string
			for _, attribute := range test.node.Attrs {
				if validationNamespaceDeclaration(attribute) {
					declarations = append(declarations, attribute.Name.Local)
				} else {
					ordinary = append(ordinary, validationAttributePath("/"+test.node.Name.Local, attribute.Name))
				}
			}
			assert.Equal(t, test.declarations, declarations)
			assert.Equal(t, test.ordinary, ordinary)
		})
	}
}

func TestValidationNamespaceDeclarationDoesNotSatisfyRequiredAttribute(t *testing.T) {
	t.Parallel()
	name := validationQName{Space: "xmlns", Local: "rubbish"}
	schema := &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{
		{Local: "value"}: {
			Name: validationQName{Local: "value"},
			Type: validationTypeRef{InlineComplex: &validationComplexSchema{
				Form: validationComplexDirect,
				Attributes: []validationAttributeSchema{{
					Name: name, Use: validationAttributeRequired,
					Type: validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "string"}},
				}},
			}},
		},
	}}
	for _, test := range []struct {
		name, source string
		missing      bool
	}{
		{"declaration only", `<value xmlns:rubbish="urn:unused"/>`, true},
		{"ordinary only", `<value xmlns:p="xmlns" p:rubbish="x"/>`, false},
		{"same expanded name", `<value xmlns:rubbish="urn:unused" xmlns:p="xmlns" p:rubbish="x"/>`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			context := validateAttributeSource(t, schema, test.source)
			if !test.missing {
				assert.Empty(t, context.issues)
				return
			}
			require.Len(t, context.issues, 1)
			assert.Equal(t, "required", context.issues[0].Constraint)
			assert.Equal(t, "/value/@{xmlns}rubbish", context.issues[0].Path)
		})
	}
}

func TestNamespaceProvenancePreservesEncodedDecode(t *testing.T) {
	t.Parallel()
	const source = `DECL<opus xmlns:p="xmlns" xmlns:xml="http://www.w3.org/XML/1998/namespace" p:rubbish="x"><title>Keep</title></opus>`
	for encoding, data := range xmlDeclarationEncodingVariants(source) {
		t.Run(encoding, func(t *testing.T) {
			t.Parallel()
			document, err := DecodeOpusDocument(bytes.NewReader(data))
			require.NoError(t, err)
			assert.Equal(t, xml.Name{Local: "opus"}, document.XMLName)
			assert.Equal(t, Ptr("Keep"), document.Title)
			assert.NoError(t, Validate(document))
		})
	}
}
