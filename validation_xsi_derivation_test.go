package musicxml

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An unsupported union restriction cannot prove that an xsi:type nomination is
// invalid. Repeated assessment must retain that distinction after cache hits.
func TestIncompleteUnionDerivation(t *testing.T) {
	t.Parallel()
	for _, independentIssue := range []bool{false, true} {
		name := "technical-only"
		if independentIssue {
			name = "independent-nillability"
		}
		t.Run(name, func(t *testing.T) {
			union := &validationSimpleSchema{Form: validationSimpleUnion, Members: []*validationSimpleMember{{Name: validationQName{Space: validationXSDNamespace, Local: "integer"}}, {Name: validationQName{Space: validationXSDNamespace, Local: "string"}}}}
			broken := &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Inline: union}, WhiteSpace: "collapse"}
			declaration := &validationElementSchema{Name: validationQName{Local: "value"}, Type: validationTypeRef{InlineSimple: broken}}
			schema := &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{{Local: "value"}: declaration}}
			attrs := ""
			if independentIssue {
				attrs = ` i:nil="false"`
			}
			node, err := parseValidationDocument([]byte(`<value xmlns:i="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema" i:type="xs:integer"` + attrs + `>1</value>`))
			require.NoError(t, err)
			c := newTestValidationContext(schema)
			for repeat := 0; repeat < 3; repeat++ {
				before := c.assessmentVersion
				c.validateElement(node, declaration, "/value")
				c.validateIdentityReferences()
				require.Error(t, c.assessmentErr)
				assert.Greater(t, c.assessmentVersion, before)
				var invalid *ValidationError
				if independentIssue {
					require.Len(t, c.issues, repeat+1)
					for _, issue := range c.issues {
						assert.Equal(t, "nillable", issue.Constraint)
						assert.Equal(t, "/value/@{http://www.w3.org/2001/XMLSchema-instance}nil", issue.Path)
					}
					assert.True(t, errors.As(c.result(), &invalid))
				} else {
					assert.Empty(t, c.issues)
					assert.False(t, errors.Is(c.result(), ErrInvalidDocument))
					assert.False(t, errors.As(c.result(), &invalid))
				}
			}
		})
	}
}
