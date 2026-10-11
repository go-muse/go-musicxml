package musicxml

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEffectiveSimpleContentScalarLayers(t *testing.T) {
	t.Parallel()
	context := newTestValidationContext(&validationSimpleContentGenerated)
	digits := validationSimpleContentGenerated.Types[validationQName{Local: "digits-type"}].Complex
	total := validationSimpleContentGenerated.Types[validationQName{Local: "total-type"}].Complex
	for range 3 {
		effective, ok := context.EffectiveComplex(digits)
		require.True(t, ok)
		again, ok := context.EffectiveComplex(digits)
		require.True(t, ok)
		assert.Same(t, effective, again)
		sibling, ok := context.EffectiveComplex(total)
		require.True(t, ok)
		assert.NotSame(t, effective.Simple.InlineSimple, sibling.Simple.InlineSimple)
		assert.NotSame(t, digits.SimpleContent, effective.Simple.InlineSimple)
		assert.Nil(t, digits.SimpleContent.Base)
		assert.Nil(t, total.SimpleContent.Base)
		assert.Equal(t, "decimal", context.SimpleBuiltin(effective.Simple))
		assert.Equal(t, "decimal", context.SimpleBuiltin(sibling.Simple))
		assert.NotNil(t, context.ValidateSimple(effective.Simple.InlineSimple, ".001"))
		assert.Nil(t, context.ValidateSimple(sibling.Simple.InlineSimple, ".001"))
	}
}

func TestSimpleContentMixedBaseRemainsUnavailable(t *testing.T) {
	t.Parallel()
	// A valid mixed/emptiable-base restriction needs a separate content-category
	// integration. A retained inline scalar must not silently bypass that boundary.
	mixed := &validationComplexSchema{Form: validationComplexDirect, Mixed: true}
	restricted := &validationComplexSchema{Form: validationComplexSimpleContentRestriction,
		Base: &validationTypeRef{Name: validationQName{Local: "mixed"}},
		SimpleContent: &validationSimpleSchema{Form: validationSimpleRestriction,
			Base: &validationSimpleMember{Inline: &validationSimpleSchema{Form: validationSimpleRestriction,
				Base: &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: "integer"}}}}}}
	set := validationSchemaSet{Types: map[validationQName]*validationTypeSchema{
		{Local: "mixed"}: {Complex: mixed}, {Local: "restricted"}: {Complex: restricted},
	}}
	context := newTestValidationContext(&set)
	for range 3 {
		effective, ok := context.EffectiveComplex(restricted)
		assert.False(t, ok)
		assert.Nil(t, effective)
	}
	node, err := parseValidationDocument([]byte(`<value>10</value>`))
	require.NoError(t, err)
	context.ValidateElement(node, &validationElementSchema{Name: validationQName{Local: "value"}, Type: validationTypeRef{Name: validationQName{Local: "restricted"}}}, "/value")
	assert.Error(t, context.AssessmentErr)
	assert.Empty(t, context.Issues)
	assert.NotErrorIs(t, context.result(), ErrInvalidDocument)
}
