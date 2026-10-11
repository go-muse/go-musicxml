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
				before := c.AssessmentVersion
				c.ValidateAttributes(node, attributes, nil, fmt.Sprintf("/value[%d]", repeat+1))
				require.Error(t, c.AssessmentErr)
				assert.Greater(t, c.AssessmentVersion, before)
				if independentIssue {
					require.Len(t, c.Issues, repeat+1)
					for _, issue := range c.Issues {
						assert.Equal(t, "attribute", issue.Constraint)
					}
					assert.ErrorIs(t, c.result(), ErrInvalidDocument)
				} else {
					assert.Empty(t, c.Issues)
					assert.NotErrorIs(t, c.result(), ErrInvalidDocument)
				}
			}
			// An earlier technical result must not suppress independent proven facts.
			node, err := parseValidationDocument([]byte(`<control value="different"/>`))
			require.NoError(t, err)
			attributes[0].Type = validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "string"}}
			before := c.AssessmentVersion
			c.ValidateAttributes(node, attributes, nil, "/control")
			assert.Equal(t, before, c.AssessmentVersion)
			require.NotEmpty(t, c.Issues)
			assert.Equal(t, "fixed", c.Issues[len(c.Issues)-1].Constraint)
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
	require.Error(t, c.AssessmentErr)
	assert.Contains(t, c.AssessmentErr.Error(), "union member final forbids union construction")
	assert.Empty(t, c.Issues)
	assert.NotErrorIs(t, c.result(), ErrInvalidDocument)
}

func TestIncompleteMetadataReplayKeepsOriginalCause(t *testing.T) {
	for _, kind := range []string{"missing", "cycle", "union final"} {
		t.Run(kind, func(t *testing.T) {
			var identity validationTypeIdentity
			switch kind {
			case "missing":
				identity.Simple = &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Name: validationQName{Local: "missing-type"}}}
			case "cycle":
				simple := &validationSimpleSchema{Form: validationSimpleRestriction}
				simple.Base = &validationSimpleMember{Inline: simple}
				identity.Simple = simple
			case "union final":
				base := &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: "string"}}, Final: validationDerivationUnion}
				identity.Simple = &validationSimpleSchema{Form: validationSimpleUnion, Members: []*validationSimpleMember{{Inline: base}}}
			}
			c := newTestValidationContext(&validationSchemaSet{})
			require.False(t, c.ValidTypeMetadata(identity))
			require.Error(t, c.AssessmentErr)
			original := c.AssessmentErr
			message := original.Error()
			for range 10 {
				before := c.AssessmentVersion
				assert.False(t, c.ValidTypeMetadata(identity))
				assert.Equal(t, before+1, c.AssessmentVersion)
				assert.Same(t, original, c.AssessmentErr)
				assert.Equal(t, message, c.result().Error())
			}
		})
	}
}

func TestIncompleteMetadataReplayColdFallbackAndNewCycle(t *testing.T) {
	identity := validationTypeIdentity{Builtin: "missing-builtin"}
	c := newTestValidationContext(&validationSchemaSet{})
	// A future cache producer must not allow unknown metadata to return nil.
	c.TypeMetadata = map[validationTypeIdentity]uint8{identity: 3}
	assert.False(t, c.ValidTypeMetadata(identity))
	require.Error(t, c.AssessmentErr)
	assert.Equal(t, uint64(1), c.AssessmentVersion)
	original := c.AssessmentErr
	assert.False(t, c.ValidTypeMetadata(identity))
	assert.Same(t, original, c.AssessmentErr)
	assert.Equal(t, uint64(2), c.AssessmentVersion)

	// State 1 is a newly discovered cycle, not a replay of state 3. Record its
	// first cause even when an unrelated incomplete operation happened before.
	simple := &validationSimpleSchema{Form: validationSimpleRestriction}
	simple.Base = &validationSimpleMember{Inline: simple}
	assert.False(t, c.ValidTypeMetadata(validationTypeIdentity{Simple: simple}))
	assert.Contains(t, c.AssessmentErr.Error(), "cached type metadata assessment is incomplete")
	assert.Contains(t, c.AssessmentErr.Error(), "cyclic type derivation metadata")
	assert.NotSame(t, original, c.AssessmentErr)
	assert.NotErrorIs(t, c.result(), ErrInvalidDocument)
}
