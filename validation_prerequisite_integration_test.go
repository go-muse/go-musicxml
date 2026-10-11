package musicxml

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrammarMetadataAssessmentBoundary(t *testing.T) {
	for _, dynamicDefault := range []bool{false, true} {
		for _, independentFailure := range []bool{false, true} {
			name := "ordinary"
			if dynamicDefault {
				name = "dynamic default"
			}
			if independentFailure {
				name += " with independent issue"
			}
			t.Run(name, func(t *testing.T) {
				particle := &validationParticleSchema{Kind: validationParticleSequence, Occurrence: validationOccurrence{Min: 1, Max: 1}}
				particle.Children = []*validationParticleSchema{particle}
				complex := &validationComplexSchema{Form: validationComplexDirect, Mixed: true, Particle: particle,
					Attributes: []validationAttributeSchema{{Name: validationQName{Local: "ref"}, Type: validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "IDREF"}}}}}
				declaration := &validationElementSchema{Name: validationQName{Local: "value"}, Type: validationTypeRef{InlineComplex: complex}}
				set := &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{{Local: "value"}: declaration}}
				attrs := ` ref="later"`
				if dynamicDefault {
					declaration.Type = validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "anyType"}}
					declaration.Default = validationString("kept")
					set.Types = map[validationQName]*validationTypeSchema{{Local: "actual"}: {Complex: complex}}
					attrs += ` xmlns:i="` + validationXSINamespace + `" i:type="actual"`
				}
				if independentFailure {
					attrs += ` bad="value"`
				}
				context := assessAttributeSource(t, set, `<value`+attrs+`/>`)
				require.Error(t, context.AssessmentErr)
				assert.Contains(t, context.AssessmentErr.Error(), "content-model")
				require.Len(t, context.References, 1)
				if independentFailure {
					require.Len(t, context.Issues, 1)
					assert.Equal(t, "attribute", context.Issues[0].Constraint)
					assert.Equal(t, "/value/@bad", context.Issues[0].Path)
					assert.ErrorIs(t, context.result(), ErrInvalidDocument)
				} else {
					assert.Empty(t, context.Issues)
					assert.NotErrorIs(t, context.result(), ErrInvalidDocument)
				}
				assert.Same(t, particle, particle.Children[0])
			})
		}
	}
}

func TestXSIQNameUsesDatatypeRepertoire(t *testing.T) {
	// Newer codepoints occur in attribute values, not markup names: the legacy
	// XML decoder rejects some newer XML names before XSD assessment.
	source := `<t:builtin xmlns:t="urn:xsi-contract" xmlns:xs="` + validationXSDNamespace + `" xmlns:i="` + validationXSINamespace + `" i:type="xs:Ƞ">ok</t:builtin>`
	context := validateAttributeSource(t, &validationXSIGenerated, source)
	require.Len(t, context.Issues, 1)
	assert.Equal(t, "xsi:type", context.Issues[0].Constraint)
	assert.Contains(t, context.Issues[0].Message, "not a bound QName")
	node := &validationNode{Namespaces: map[string]string{"Ƞ": validationXSDNamespace, "xs": validationXSDNamespace}}
	for _, value := range []string{"Ƞ:token", "xs:Ƞ"} {
		_, ok := validationInstanceQName(node, value)
		assert.False(t, ok, value)
	}
}

func TestXSITypeIdentityUsesDeclaration(t *testing.T) {
	// XSD 1.0 sic-id separates declaration-type entry eligibility from actual
	// ID bindings. A root scalar ID seeds an entry but has no in-scope parent
	// to bind it; dynamically chosen identity types do not seed entries.
	for _, test := range []struct {
		declared, actual string
		ids, refs        int
	}{
		{"string", "ID", 0, 0},
		{"string", "IDREF", 0, 0},
		{"anySimpleType", "IDREFS", 0, 0},
		{"ID", "ID", 1, 0},
		{"IDREF", "IDREF", 0, 1},
		{"IDREFS", "IDREFS", 0, 1},
	} {
		t.Run(test.declared+"/"+test.actual, func(t *testing.T) {
			declaration := &validationElementSchema{Name: validationQName{Local: "value"}, Type: validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: test.declared}}}
			set := &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{{Local: "value"}: declaration}}
			source := `<value xmlns:i="` + validationXSINamespace + `" xmlns:xs="` + validationXSDNamespace + `" i:type="xs:` + test.actual + `"> target </value>`
			context := validateAttributeSource(t, set, source)
			assert.Len(t, context.Identifiers, test.ids)
			assert.Len(t, context.References, test.refs)
			assert.Len(t, context.IdentityEntries, test.ids+test.refs)
			assert.Empty(t, context.IdentityBindings, "root scalar has no containing owner")
			if test.ids != 0 {
				assert.Contains(t, context.Identifiers, "target")
			}
			if test.ids != 0 {
				assertValidationIssues(t, context, []string{"/value:ID"})
			} else if test.refs == 0 {
				assertValidationIssues(t, context, nil)
			} else {
				assert.Equal(t, "target", context.References[0].Value)
				assertValidationIssues(t, context, []string{"/value:IDREF"})
			}
		})
	}
}
