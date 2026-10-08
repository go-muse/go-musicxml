package musicxml

import (
	"bytes"
	"encoding/xml"
	"strings"
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

func TestValidationNamespaceDeclarationProvenanceLength(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, source string
		flags        []bool
		wantError    bool
	}{
		{"nil", `<value rubbish="x"/>`, nil, true},
		{"short", `<value xmlns:p="urn:test" p:rubbish="x"/>`, []bool{true}, true},
		{"long", `<value rubbish="x"/>`, []bool{false, true}, true},
		{"nonempty flags without attributes", `<value/>`, []bool{true}, true},
		{"matching", `<value xmlns:p="urn:test" p:rubbish="x"/>`, []bool{true, false}, false},
		{"empty", `<value/>`, nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			raw := xml.NewDecoder(strings.NewReader(test.source))
			tokens := &wellFormedXMLTokenReader{source: rawXMLTokenReader{raw}}
			decoder := xml.NewTokenDecoder(tokens)
			token, err := decoder.Token()
			require.NoError(t, err)
			start, ok := token.(xml.StartElement)
			require.True(t, ok)
			// Simulate a future adapter losing alignment after raw capture.
			tokens.namespaceDeclarations = test.flags
			var node *validationNode
			require.NotPanics(t, func() {
				node, err = readValidationNode(decoder, tokens, start, 1)
			})
			if test.wantError {
				assert.ErrorContains(t, err, "namespace declaration provenance out of sync at <value>")
				assert.Nil(t, node)
				// Fail before consuming any content from the mismatched element.
				next, err := decoder.Token()
				require.NoError(t, err)
				assert.Equal(t, start.End(), next)
				return
			}
			require.NoError(t, err)
			require.Len(t, node.Attrs, len(test.flags))
			for index, flag := range test.flags {
				assert.Equal(t, flag, node.Attrs[index].NamespaceDeclaration)
			}
		})
	}
}
