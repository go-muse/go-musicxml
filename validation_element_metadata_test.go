package musicxml

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReferencedNilElementDeclarationIsIncomplete(t *testing.T) {
	t.Parallel()
	for _, particle := range []bool{false, true} {
		name := "root reference"
		if particle {
			name = "particle reference"
		}
		t.Run(name, func(t *testing.T) {
			missing := validationQName{Local: "missing"}
			ref := &validationElementSchema{Reference: missing}
			set := &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{missing: nil}}
			root, source := ref, `<value/>`
			if particle {
				root = &validationElementSchema{Name: validationQName{Local: "value"}, Type: validationTypeRef{InlineComplex: &validationComplexSchema{Form: validationComplexDirect, Particle: &validationParticleSchema{Kind: validationParticleElement, Occurrence: validationOccurrence{Min: 1, Max: 1}, Element: ref}}}}
				source = `<value><missing/></value>`
			}
			c := newTestValidationContext(set)
			node, err := parseValidationDocument([]byte(source))
			require.NoError(t, err)
			c.ValidateElement(node, root, "/value")
			require.Error(t, c.AssessmentErr)
			assert.Empty(t, c.Issues)
			assert.NotErrorIs(t, c.result(), ErrInvalidDocument)
		})
	}
}

func TestFixedUnresolvedComplexTypeIsIncomplete(t *testing.T) {
	t.Parallel()
	for _, cycle := range []bool{false, true} {
		name := "missing base"
		if cycle {
			name = "cyclic base"
		}
		t.Run(name, func(t *testing.T) {
			base := validationTypeRef{Name: validationQName{Local: "missing"}}
			complex := &validationComplexSchema{Form: validationComplexSimpleContentExtension, Base: &base}
			if cycle {
				complex.Base = &validationTypeRef{InlineComplex: complex}
			}
			declaration := &validationElementSchema{Name: validationQName{Local: "value"}, Type: validationTypeRef{InlineComplex: complex}, Fixed: validationString("same")}
			set := &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{{Local: "value"}: declaration}}
			for _, attrs := range []string{"", ` xmlns:i="` + validationXSINamespace + `" i:nil="false"`} {
				c := assessAttributeSource(t, set, `<value`+attrs+`>same</value>`)
				require.Error(t, c.AssessmentErr)
				if attrs == "" {
					assert.Empty(t, c.Issues)
					assert.NotErrorIs(t, c.result(), ErrInvalidDocument)
				} else {
					require.Len(t, c.Issues, 1)
					assert.Equal(t, "nillable", c.Issues[0].Constraint)
					assert.ErrorIs(t, c.result(), ErrInvalidDocument)
				}
			}
		})
	}
	// A fully assessed unequal value remains a genuine fixed-value violation.
	text := validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "string"}}
	valid := &validationElementSchema{Name: validationQName{Local: "value"}, Type: validationTypeRef{InlineComplex: &validationComplexSchema{Form: validationComplexSimpleContentExtension, Base: &text}}, Fixed: validationString("same")}
	set := &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{{Local: "value"}: valid}}
	c := validateAttributeSource(t, set, `<value>different</value>`)
	assertValidationIssues(t, c, []string{"/value:fixed"})
	assert.ErrorIs(t, c.result(), ErrInvalidDocument)
}
