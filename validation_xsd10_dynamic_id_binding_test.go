package musicxml

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestXSD10DynamicIDBinding(t *testing.T) {
	builtin := func(name string) validationTypeRef {
		return validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: name}}
	}
	particle := func(element *validationElementSchema, min, max uint64) *validationParticleSchema {
		return &validationParticleSchema{Kind: validationParticleElement, Occurrence: validationOccurrence{Min: min, Max: max}, Element: element}
	}
	sequence := func(children ...*validationParticleSchema) validationTypeRef {
		return validationTypeRef{InlineComplex: &validationComplexSchema{Form: validationComplexDirect, Particle: &validationParticleSchema{Kind: validationParticleSequence, Occurrence: validationOccurrence{Min: 1, Max: 1}, Children: children}}}
	}
	target := &validationElementSchema{Name: validationQName{Local: "target"}, Type: builtin("string")}
	item := &validationElementSchema{Name: validationQName{Local: "item"}, Type: sequence(particle(target, 1, 2))}
	ref := &validationElementSchema{Name: validationQName{Local: "ref"}, Type: builtin("IDREF")}
	root := &validationElementSchema{Name: validationQName{Local: "root"}, Type: sequence(particle(item, 0, 2), particle(ref, 0, 1))}
	schema := &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{{Local: "root"}: root}}
	for _, test := range []struct{ name, content, violation string }{
		{"declared reference binds dynamic ID", `<item><target i:type="xs:ID"> target </target></item><ref>target</ref>`, ""},
		{"declared reference cannot bind ordinary string", `<item><target i:type="xs:string">target</target></item><ref>target</ref>`, "IDREF"},
		{"seeded duplicate actual ID bindings", `<item><target i:type="xs:ID">target</target></item><item><target i:type="xs:ID">target</target></item><ref>target</ref>`, "ID"},
		{"same parent is one binding", `<item><target i:type="xs:ID">target</target><target i:type="xs:ID">target</target></item><ref>target</ref>`, ""},
		{"unseeded dynamic IDs do not create table entries", `<item><target i:type="xs:ID">target</target></item><item><target i:type="xs:ID">target</target></item>`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := `<root xmlns:i="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema">` + test.content + `</root>`
			context := assessAttributeSource(t, schema, source)
			require.NoError(t, context.assessmentErr)
			if test.violation == "" {
				assert.Empty(t, context.issues)
				assert.NoError(t, context.result())
			} else {
				assert.ErrorIs(t, context.result(), ErrInvalidDocument)
				var kinds []string
				for _, issue := range context.issues {
					kinds = append(kinds, issue.Constraint)
				}
				assert.Equal(t, []string{test.violation}, kinds)
			}
		})
	}
}
