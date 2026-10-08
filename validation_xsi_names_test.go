package musicxml

import (
	"encoding/xml"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationStandardXSIName(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		local                 string
		withoutType, withType bool
	}{
		{"nil", true, true},
		{"schemaLocation", true, true},
		{"noNamespaceSchemaLocation", true, true},
		{"type", false, true},
		{"unknown", false, false},
		{"SchemaLocation", false, false},
		{"noNamespaceSchemalocation", false, false},
		{"Type", false, false},
		{"", false, false},
	} {
		for _, namespace := range []string{
			validationXSINamespace, "", "urn:foreign", "xsi", "xmlns", validationXSINamespace + "/",
		} {
			for _, allowType := range []bool{false, true} {
				want := test.withoutType
				if allowType {
					want = test.withType
				}
				want = want && namespace == validationXSINamespace
				assert.Equal(t, want, validationStandardXSIName(xml.Name{Space: namespace, Local: test.local}, allowType),
					"namespace=%q local=%q allowType=%t", namespace, test.local, allowType)
			}
		}
	}
}

func TestStandardXSINamePreservesTypeAllowance(t *testing.T) {
	t.Parallel()
	text := validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "string"}}
	for _, test := range []struct {
		name      string
		reference validationTypeRef
		allowType bool
	}{
		{"builtin simple", text, true},
		{"inline simple", validationTypeRef{InlineSimple: &validationSimpleSchema{
			Form: validationSimpleRestriction,
			Base: &validationSimpleMember{Name: text.Name},
		}}, true},
		{"complex content", validationTypeRef{InlineComplex: &validationComplexSchema{Form: validationComplexDirect}}, false},
		{"complex simple content", validationTypeRef{InlineComplex: &validationComplexSchema{
			Form: validationComplexSimpleContentExtension, Base: &text,
		}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			schema := &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{
				{Local: "value"}: {Name: validationQName{Local: "value"}, Type: test.reference},
			}}
			// This compatibility check is about name allowance only, not QName
			// resolution or substitution of the requested type.
			source := `<value xmlns:i="` + validationXSINamespace + `" xmlns:xs="` + validationXSDNamespace + `" i:type="xs:string" i:schemaLocation="urn:example schema.xsd" i:noNamespaceSchemaLocation="schema.xsd"/>`
			context := validateAttributeSource(t, schema, source)
			if test.allowType {
				assert.Empty(t, context.issues)
				return
			}
			require.Len(t, context.issues, 1)
			assert.Equal(t, "attribute", context.issues[0].Constraint)
			assert.Equal(t, "/value/@{"+validationXSINamespace+"}type", context.issues[0].Path)
		})
	}
}
