package musicxml

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestIncompleteUnionDerivationPreservesTechnicalState(t *testing.T) {
	for _, badHint := range []bool{false, true} {
		t.Run(map[bool]string{false: "technical-only", true: "independent-hint"}[badHint], func(t *testing.T) {
			union := &validationSimpleSchema{Form: validationSimpleUnion, Members: []*validationSimpleMember{{Name: validationQName{Space: validationXSDNamespace, Local: "integer"}}, {Name: validationQName{Space: validationXSDNamespace, Local: "string"}}}}
			broken := &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Inline: union}, WhiteSpace: "collapse"}
			declaration := &validationElementSchema{Name: validationQName{Local: "value"}, Type: validationTypeRef{InlineSimple: broken}}
			schema := &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{{Local: "value"}: declaration}}
			attrs := ""
			if badHint {
				attrs = ` i:schemaLocation="%GG"`
			}
			node, err := parseValidationDocument([]byte(`<value xmlns:i="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema" i:type="xs:integer"` + attrs + `>1</value>`))
			require.NoError(t, err)
			c := newTestValidationContext(schema)
			for repeat := 0; repeat < 3; repeat++ {
				c.validateElement(node, declaration, "/value")
				c.validateIdentityReferences()
				require.Error(t, c.assessmentErr)
				var invalid *ValidationError
				if badHint {
					require.Len(t, c.issues, repeat+1)
					for _, issue := range c.issues {
						assert.Equal(t, "datatype", issue.Constraint)
						assert.Equal(t, "/value/@{http://www.w3.org/2001/XMLSchema-instance}schemaLocation", issue.Path)
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
