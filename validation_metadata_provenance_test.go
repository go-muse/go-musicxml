package musicxml

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIncompleteMetadataFixedAttributeCacheProvenance(t *testing.T) {
	for _, independentIssue := range []bool{false, true} {
		t.Run(fmt.Sprint(independentIssue), func(t *testing.T) {
			base := &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: "string"}}, Final: validationDerivationUnion}
			union := &validationSimpleSchema{Form: validationSimpleUnion, Members: []*validationSimpleMember{{Inline: base}}}
			c := newTestValidationContext(&validationSchemaSet{})
			attributes := []validationAttributeSchema{{Name: validationQName{Local: "value"}, Type: validationTypeRef{InlineSimple: union}, Constraint: &validationValueConstraint{Kind: "fixed", Value: "same"}}}
			for repeat := range 3 {
				extra := ""
				if independentIssue {
					extra = ` bad="x"`
				}
				node, err := parseValidationDocument([]byte(`<value value="same"` + extra + `/>`))
				require.NoError(t, err)
				before := c.assessmentVersion
				c.validateAttributes(node, attributes, nil, fmt.Sprintf("/value[%d]", repeat+1))
				require.Error(t, c.assessmentErr)
				assert.Greater(t, c.assessmentVersion, before)
				if independentIssue {
					require.Len(t, c.issues, repeat+1)
					for _, issue := range c.issues {
						assert.Equal(t, "attribute", issue.Constraint)
					}
					assert.ErrorIs(t, c.result(), ErrInvalidDocument)
				} else {
					assert.Empty(t, c.issues)
					assert.NotErrorIs(t, c.result(), ErrInvalidDocument)
				}
			}
			// An earlier technical result must not suppress independent proven facts.
			node, err := parseValidationDocument([]byte(`<control value="different"/>`))
			require.NoError(t, err)
			attributes[0].Type = validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "string"}}
			before := c.assessmentVersion
			c.validateAttributes(node, attributes, nil, "/control")
			assert.Equal(t, before, c.assessmentVersion)
			require.NotEmpty(t, c.issues)
			assert.Equal(t, "fixed", c.issues[len(c.issues)-1].Constraint)
			assert.ErrorIs(t, c.result(), ErrInvalidDocument)
		})
	}
}

func TestIncompleteMetadataSharedFixedAttributes(t *testing.T) {
	base := &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: "string"}}, Final: validationDerivationUnion}
	union := &validationSimpleSchema{Form: validationSimpleUnion, Members: []*validationSimpleMember{{Inline: base}}}
	var attributes []validationAttributeSchema
	for _, name := range []string{"first", "second"} {
		attributes = append(attributes, validationAttributeSchema{Name: validationQName{Local: name}, Type: validationTypeRef{InlineSimple: union}, Constraint: &validationValueConstraint{Kind: "fixed", Value: "same"}})
	}
	root := &validationElementSchema{Name: validationQName{Local: "value"}, Type: validationTypeRef{InlineComplex: &validationComplexSchema{Form: validationComplexDirect, Attributes: attributes}}}
	c := assessAttributeSource(t, &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{{Local: "value"}: root}}, `<value first="same" second="same"/>`)
	require.Error(t, c.assessmentErr)
	assert.Contains(t, c.assessmentErr.Error(), "union member final forbids union construction")
	assert.Empty(t, c.issues)
	assert.NotErrorIs(t, c.result(), ErrInvalidDocument)
}
