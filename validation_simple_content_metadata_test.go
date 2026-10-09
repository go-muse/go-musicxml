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
		effective, ok := context.effectiveComplex(digits)
		require.True(t, ok)
		again, ok := context.effectiveComplex(digits)
		require.True(t, ok)
		assert.Same(t, effective, again)
		sibling, ok := context.effectiveComplex(total)
		require.True(t, ok)
		assert.NotSame(t, effective.simple.InlineSimple, sibling.simple.InlineSimple)
		assert.NotSame(t, digits.SimpleContent, effective.simple.InlineSimple)
		assert.Nil(t, digits.SimpleContent.Base)
		assert.Nil(t, total.SimpleContent.Base)
		assert.Equal(t, "decimal", context.simpleBuiltin(effective.simple))
		assert.Equal(t, "decimal", context.simpleBuiltin(sibling.simple))
		assert.NotNil(t, context.validateSimple(effective.simple.InlineSimple, ".001"))
		assert.Nil(t, context.validateSimple(sibling.simple.InlineSimple, ".001"))
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
		effective, ok := context.effectiveComplex(restricted)
		assert.False(t, ok)
		assert.Nil(t, effective)
	}
	node, err := parseValidationDocument([]byte(`<value>10</value>`))
	require.NoError(t, err)
	context.validateElement(node, &validationElementSchema{Name: validationQName{Local: "value"}, Type: validationTypeRef{Name: validationQName{Local: "restricted"}}}, "/value")
	assertValidationIssues(t, context, []string{"/value:schema"})
}
