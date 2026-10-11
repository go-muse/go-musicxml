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

func TestIncompleteMetadataReplayKeepsOriginalCause(t *testing.T) {
	for _, kind := range []string{"missing", "cycle", "union final"} {
		t.Run(kind, func(t *testing.T) {
			var identity validationTypeIdentity
			switch kind {
			case "missing":
				identity.simple = &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Name: validationQName{Local: "missing-type"}}}
			case "cycle":
				simple := &validationSimpleSchema{Form: validationSimpleRestriction}
				simple.Base = &validationSimpleMember{Inline: simple}
				identity.simple = simple
			case "union final":
				base := &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: "string"}}, Final: validationDerivationUnion}
				identity.simple = &validationSimpleSchema{Form: validationSimpleUnion, Members: []*validationSimpleMember{{Inline: base}}}
			}
			c := newTestValidationContext(&validationSchemaSet{})
			require.False(t, c.validTypeMetadata(identity))
			require.Error(t, c.assessmentErr)
			original := c.assessmentErr
			message := original.Error()
			for range 10 {
				before := c.assessmentVersion
				assert.False(t, c.validTypeMetadata(identity))
				assert.Equal(t, before+1, c.assessmentVersion)
				assert.Same(t, original, c.assessmentErr)
				assert.Equal(t, message, c.result().Error())
			}
		})
	}
}

func TestIncompleteMetadataReplayColdFallbackAndNewCycle(t *testing.T) {
	identity := validationTypeIdentity{builtin: "missing-builtin"}
	c := newTestValidationContext(&validationSchemaSet{})
	// A future cache producer must not allow unknown metadata to return nil.
	c.typeMetadata = map[validationTypeIdentity]uint8{identity: 3}
	assert.False(t, c.validTypeMetadata(identity))
	require.Error(t, c.assessmentErr)
	assert.Equal(t, uint64(1), c.assessmentVersion)
	original := c.assessmentErr
	assert.False(t, c.validTypeMetadata(identity))
	assert.Same(t, original, c.assessmentErr)
	assert.Equal(t, uint64(2), c.assessmentVersion)

	// State 1 is a newly discovered cycle, not a replay of state 3. Record its
	// first cause even when an unrelated incomplete operation happened before.
	simple := &validationSimpleSchema{Form: validationSimpleRestriction}
	simple.Base = &validationSimpleMember{Inline: simple}
	assert.False(t, c.validTypeMetadata(validationTypeIdentity{simple: simple}))
	assert.Contains(t, c.assessmentErr.Error(), "cached type metadata assessment is incomplete")
	assert.Contains(t, c.assessmentErr.Error(), "cyclic type derivation metadata")
	assert.NotSame(t, original, c.assessmentErr)
	assert.NotErrorIs(t, c.result(), ErrInvalidDocument)
}
