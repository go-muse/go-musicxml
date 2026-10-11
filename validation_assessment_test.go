package musicxml

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestXSITypeIncompleteAssessment(t *testing.T) {
	builtin := func(name string) validationTypeRef {
		return validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: name}}
	}
	named := func(name string) validationTypeRef { return validationTypeRef{Name: validationQName{Local: name}} }
	member := func(name string) *validationSimpleMember {
		return &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: name}}
	}
	for _, test := range []struct {
		name      string
		nominated *validationTypeSchema
		declared  validationTypeRef
		attrs     string
		value     string
	}{
		{"missing required ancestry", &validationTypeSchema{Simple: &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Name: validationQName{Local: "missing"}}}}, builtin("string"), ` i:type="candidate"`, "x"},
		{"missing component body", nil, builtin("string"), ` i:type="candidate"`, "x"},
		{"cyclic simple ancestry", &validationTypeSchema{Simple: &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Name: validationQName{Local: "candidate"}}}}, builtin("string"), ` i:type="candidate"`, "x"},
		{"cyclic complex ancestry", &validationTypeSchema{Complex: &validationComplexSchema{Form: validationComplexComplexContentExtension, Base: ptrValidationType(named("candidate"))}}, builtin("anyType"), ` i:type="candidate"`, ""},
		{"cyclic union member", &validationTypeSchema{Simple: &validationSimpleSchema{Form: validationSimpleUnion, Members: []*validationSimpleMember{{Name: validationQName{Local: "candidate"}}}}}, builtin("anySimpleType"), ` i:type="candidate"`, "x"},
		{"missing declared type", &validationTypeSchema{Simple: &validationSimpleSchema{Form: validationSimpleRestriction, Base: member("string")}}, named("missing"), ` i:type="candidate"`, "x"},
		{"entity nomination", nil, builtin("string"), ` i:type="xs:ENTITY"`, "image"},
		{"entity declaration", nil, builtin("ENTITY"), "", "image"},
		{"entity fixed operand", nil, builtin("ENTITY"), ` i:type="xs:ENTITY"`, "image"},
		{"entity declared doctype", nil, builtin("ENTITY"), "", "image"},
	} {
		t.Run(test.name, func(t *testing.T) {
			schema := &validationSchemaSet{Types: map[validationQName]*validationTypeSchema{{Local: "candidate"}: test.nominated}, Elements: map[validationQName]*validationElementSchema{{Local: "value"}: {Name: validationQName{Local: "value"}, Type: test.declared}}}
			if test.name == "entity fixed operand" {
				schema.Elements[validationQName{Local: "value"}].Fixed = validationString("image")
			}
			source := `<value xmlns:xs="` + validationXSDNamespace + `" xmlns:i="` + validationXSINamespace + `"` + test.attrs + `>` + test.value + `</value>`
			if test.name == "entity declared doctype" {
				source = `<!DOCTYPE value [<!NOTATION image SYSTEM "image"><!ENTITY image SYSTEM "image" NDATA image>]>` + source
			}
			c := assessAttributeSource(t, schema, source)
			require.Error(t, c.AssessmentErr)
			assert.Empty(t, c.Issues)
			assert.NotErrorIs(t, c.result(), ErrInvalidDocument)
			var invalid *ValidationError
			assert.False(t, errors.As(c.result(), &invalid))
		})
	}
}
func ptrValidationType(ref validationTypeRef) *validationTypeRef { return &ref }

func TestAssessmentErrorsPreserveIndependentIssues(t *testing.T) {
	source := `<t:builtin xmlns:t="urn:xsi-contract" xmlns:xs="` + validationXSDNamespace + `" xmlns:i="` + validationXSINamespace + `" i:type="xs:ENTITY" bad="x">image</t:builtin>`
	c := assessAttributeSource(t, &validationXSIGenerated, source)
	require.Error(t, c.AssessmentErr)
	require.Len(t, c.Issues, 1)
	assert.Equal(t, "attribute", c.Issues[0].Constraint)
	result := c.result()
	assert.ErrorIs(t, result, ErrInvalidDocument)
	var invalid *ValidationError
	require.ErrorAs(t, result, &invalid)
	assert.Equal(t, c.Issues, invalid.Issues)
	assert.Contains(t, result.Error(), "unparsed entity")
}

func TestEntityAssessmentDoesNotBecomeUnionFallback(t *testing.T) {
	entity := &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: "ENTITY"}}
	text := &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: "string"}}
	for _, test := range []struct {
		name       string
		simple     *validationSimpleSchema
		value      string
		incomplete bool
	}{
		{"union first unavailable", &validationSimpleSchema{Form: validationSimpleUnion, Members: []*validationSimpleMember{entity, text}}, "image", true},
		{"union earlier valid string", &validationSimpleSchema{Form: validationSimpleUnion, Members: []*validationSimpleMember{text, entity}}, "image", false},
		{"union entity lexically excluded", &validationSimpleSchema{Form: validationSimpleUnion, Members: []*validationSimpleMember{entity, text}}, "two words", false},
		{"list reached entity", &validationSimpleSchema{Form: validationSimpleList, Item: entity}, "image", true},
		{"list empty entity not reached", &validationSimpleSchema{Form: validationSimpleList, Item: entity}, "", false},
		{"restriction reached entity", &validationSimpleSchema{Form: validationSimpleRestriction, Base: entity}, "image", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := newTestValidationContext(&validationSchemaSet{})
			_, failure := c.AssessSimpleValue(test.simple, test.value, test.value)
			if test.incomplete {
				require.NotNil(t, failure)
				require.Error(t, c.AssessmentErr)
				assert.NotErrorIs(t, c.result(), ErrInvalidDocument)
			} else {
				assert.Nil(t, failure)
				assert.NoError(t, c.AssessmentErr)
			}
		})
	}
}

func TestEntityLiteralAndFixedAssessment(t *testing.T) {
	entityUnion := &validationSimpleSchema{Form: validationSimpleUnion, Members: []*validationSimpleMember{{Name: validationQName{Space: validationXSDNamespace, Local: "ENTITY"}}, {Name: validationQName{Space: validationXSDNamespace, Local: "string"}}}}
	for _, test := range []struct {
		name string
		run  func(*validationContext)
	}{
		{"literal", func(c *validationContext) {
			_, ok := c.SimpleLiteralValue(c.ResolveSimpleSemantics(entityUnion), "image")
			assert.False(t, ok)
		}},
		{"enumeration", func(c *validationContext) {
			restriction := &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Inline: entityUnion}, Enumerations: []string{"image"}}
			_, failure := c.AssessSimpleValue(restriction, "two words", "two words")
			require.NotNil(t, failure)
			assert.True(t, validationTechnicalFailure(failure))
		}},
		{"fixed attribute", func(c *validationContext) {
			node, err := parseValidationDocument([]byte(`<value value="two words"/>`))
			require.NoError(t, err)
			c.ValidateAttributes(node, []validationAttributeSchema{{Name: validationQName{Local: "value"}, Type: validationTypeRef{InlineSimple: entityUnion}, Constraint: &validationValueConstraint{Kind: "fixed", Value: "image"}}}, nil, "/value")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := newTestValidationContext(&validationSchemaSet{})
			for range 3 {
				test.run(c)
			}
			assert.Error(t, c.AssessmentErr)
			assert.Empty(t, c.Issues)
			assert.NotErrorIs(t, c.result(), ErrInvalidDocument)
		})
	}
}

func TestFinalInvalidMetadataIsIncomplete(t *testing.T) {
	base := &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: "string"}}, Final: validationDerivationRestriction | validationDerivationList | validationDerivationUnion}
	for _, derived := range []*validationSimpleSchema{
		{Form: validationSimpleRestriction, Base: &validationSimpleMember{Inline: base}},
		{Form: validationSimpleList, Item: &validationSimpleMember{Inline: base}},
		{Form: validationSimpleUnion, Members: []*validationSimpleMember{{Inline: base}}},
	} {
		c := newTestValidationContext(&validationSchemaSet{})
		c.ValidateType(&validationNode{}, &validationTypeRef{InlineSimple: derived}, "/value", false, &validationNode{})
		assert.Error(t, c.AssessmentErr)
		assert.Empty(t, c.Issues)
		assert.NotErrorIs(t, c.result(), ErrInvalidDocument)
	}
}

func TestIncompleteEnumerationCacheDoesNotInventViolations(t *testing.T) {
	union := &validationSimpleSchema{Form: validationSimpleUnion, Members: []*validationSimpleMember{{Name: validationQName{Space: validationXSDNamespace, Local: "ENTITY"}}, {Name: validationQName{Space: validationXSDNamespace, Local: "string"}}}}
	restriction := &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Inline: union}, Enumerations: []string{"image"}}
	c := newTestValidationContext(&validationSchemaSet{})
	for range 3 {
		_, failure := c.AssessSimpleValue(restriction, "two words", "two words")
		require.NotNil(t, failure)
		assert.True(t, validationTechnicalFailure(failure))
		require.Error(t, c.AssessmentErr)
		assert.Empty(t, c.Issues)
		assert.Len(t, c.EnumerationValues, 1)
	}
}

func TestIncompleteIdentityTraversalDoesNotProveAbsentTarget(t *testing.T) {
	ref := &validationElementSchema{Name: validationQName{Local: "ref"}, Type: validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "IDREF"}}}
	target := &validationElementSchema{Name: validationQName{Local: "target"}, Type: validationTypeRef{Name: validationQName{Local: "unavailable-target-type"}}}
	root := &validationElementSchema{Name: validationQName{Local: "root"}, Type: validationTypeRef{InlineComplex: &validationComplexSchema{Form: validationComplexDirect, Particle: &validationParticleSchema{Kind: validationParticleSequence, Occurrence: validationOccurrence{Min: 1, Max: 1}, Children: []*validationParticleSchema{
		{Kind: validationParticleElement, Occurrence: validationOccurrence{Min: 1, Max: 1}, Element: ref}, {Kind: validationParticleElement, Occurrence: validationOccurrence{Min: 1, Max: 1}, Element: target},
	}}}}}
	set := &validationSchemaSet{Elements: map[validationQName]*validationElementSchema{{Local: "root"}: root}}
	for _, attrs := range []string{"", ` bad="value"`} {
		c := assessAttributeSource(t, set, `<root`+attrs+`><ref>target</ref><target id="target"/></root>`)
		require.Error(t, c.AssessmentErr)
		require.Len(t, c.References, 1)
		if attrs == "" {
			assert.Empty(t, c.Issues)
			assert.NotErrorIs(t, c.result(), ErrInvalidDocument)
		} else {
			require.Len(t, c.Issues, 1)
			assert.Equal(t, "attribute", c.Issues[0].Constraint)
			assert.ErrorIs(t, c.result(), ErrInvalidDocument)
		}
	}
}

func TestIncompleteDefaultPreservesActualAttributes(t *testing.T) {
	date := validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "date"}}
	set := &validationSchemaSet{Types: map[validationQName]*validationTypeSchema{{Local: "DateAttrs"}: {Complex: &validationComplexSchema{Form: validationComplexSimpleContentExtension, Base: &date, Attributes: []validationAttributeSchema{{Name: validationQName{Local: "code"}, Type: validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "string"}}, Use: validationAttributeRequired}}}}}, Elements: map[validationQName]*validationElementSchema{{Local: "value"}: {Name: validationQName{Local: "value"}, Type: date, Default: validationString("2024-01-01")}}}
	c := assessAttributeSource(t, set, wrapValidationElement("value", ` n:type="DateAttrs"`, ""))
	require.Error(t, c.AssessmentErr)
	require.Len(t, c.Issues, 1)
	assert.Equal(t, "required", c.Issues[0].Constraint)
	assert.Equal(t, "/value/@code", c.Issues[0].Path)
	assert.ErrorIs(t, c.result(), ErrInvalidDocument)
}

func TestAssessmentIssueSinkPreservesTechnicalOutcome(t *testing.T) {
	for _, existingIssue := range []bool{false, true} {
		t.Run(map[bool]string{false: "technical only", true: "joined with proven issue"}[existingIssue], func(t *testing.T) {
			c := newTestValidationContext(&validationSchemaSet{})
			if existingIssue {
				c.AddIssue("/value/@bad", "attribute", "attribute is not permitted")
			}
			c.AddIssue("/value", "assessment", "producer did not record its cause")
			require.Error(t, c.AssessmentErr)
			assert.Contains(t, c.AssessmentErr.Error(), "/value: producer did not record its cause")
			assert.Equal(t, uint64(1), c.AssessmentVersion)
			result := c.result()
			require.Error(t, result)
			assert.Equal(t, existingIssue, errors.Is(result, ErrInvalidDocument))
			var invalid *ValidationError
			assert.Equal(t, existingIssue, errors.As(result, &invalid))
			if existingIssue {
				require.Len(t, invalid.Issues, 1)
				assert.Equal(t, "attribute", invalid.Issues[0].Constraint)
			} else {
				assert.Empty(t, c.Issues)
			}
		})
	}
}

func TestAssessmentIssueSinkDoesNotDuplicateRecordedCause(t *testing.T) {
	c := newTestValidationContext(&validationSchemaSet{})
	failure := c.IncompleteFailure("original metadata cause")
	original := c.AssessmentErr
	version := c.AssessmentVersion
	for range 2 {
		c.AddIssue("/value", failure.Constraint, failure.Message)
		assert.Same(t, original, c.AssessmentErr)
		assert.Equal(t, version, c.AssessmentVersion)
	}
	c.AddIssue("/other", "schema", "independent missing schema")
	assert.Equal(t, version+1, c.AssessmentVersion)
	assert.Contains(t, c.result().Error(), "original metadata cause")
	assert.Contains(t, c.result().Error(), "/other: independent missing schema")
	assert.Empty(t, c.Issues)
	assert.NotErrorIs(t, c.result(), ErrInvalidDocument)
}

func TestAssessmentFailurePreservesRecordedCause(t *testing.T) {
	c := newTestValidationContext(&validationSchemaSet{})
	failure := c.AssessmentFailure("missing cause")
	require.NotNil(t, failure)
	assert.Equal(t, "assessment", failure.Constraint)
	assert.Equal(t, "missing cause", failure.Message)
	require.Error(t, c.AssessmentErr)
	original := c.AssessmentErr
	version := c.AssessmentVersion
	failure = c.AssessmentFailure("already recorded by metadata")
	assert.Equal(t, "already recorded by metadata", failure.Message)
	assert.Same(t, original, c.AssessmentErr)
	assert.Equal(t, version, c.AssessmentVersion)
	assert.NotErrorIs(t, c.result(), ErrInvalidDocument)
}
