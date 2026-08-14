package xsdgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanExpandedMusicXMLComplexTypes(t *testing.T) {
	t.Parallel()

	schemaFS := os.DirFS(filepath.Join(
		"..",
		"..",
		"schema",
		"musicxml-4.0",
	))

	set, err := Load(schemaFS, "musicxml.xsd", "catalog.xml")
	require.NoError(t, err)

	index, err := NewIndex(set)
	require.NoError(t, err)

	plans, err := PlanExpandedComplexTypes(index)
	require.NoError(t, err)
	require.Len(t, plans, 224)

	stats := complexPlanStats{
		particles: make(map[ParticleKind]int),
	}

	for _, plan := range plans {
		assertExpandedComplexTypeDefinition(t, plan.Definition)
		collectComplexDefinitionStats(plan.Definition, &stats)
	}

	assert.Zero(t, stats.particles[ParticleGroup])
	assert.Equal(t, 528, stats.particles[ParticleElement])
	assert.Equal(t, 39, stats.particles[ParticleChoice])
	assert.Equal(t, 172, stats.particles[ParticleSequence])
	assert.Zero(t, stats.particles[ParticleAll])
	assert.Zero(t, stats.particles[ParticleAny])
	assert.Equal(t, 89, stats.unboundedParticles)
	assert.Equal(t, 5, stats.boundedRepeatedParticles)
	assert.Equal(t, 1378, stats.attributes)
	assert.Equal(t, 60, stats.requiredAttributes)
	assert.Zero(t, stats.attributeGroups)

	clef := requireComplexTypePlan(t, plans, "clef")
	sign := requireElementPlan(
		t,
		clef.Definition.Particle,
		"sign",
	)
	assert.Equal(t, "clef-sign", sign.element.Source.Type)

	link := requireComplexTypePlan(t, plans, "link")
	require.Empty(t, link.Definition.AttributeGroups)
	assert.Contains(
		t,
		attributeLocalNames(link.Definition.Attributes),
		"href",
	)
}

func assertExpandedComplexTypeDefinition(
	t *testing.T,
	definition *ComplexTypeDefinition,
) {
	t.Helper()

	require.NotNil(t, definition)
	assert.Empty(t, definition.AttributeGroups)
	assertExpandedParticle(t, definition.Particle)

	names := make(map[expandedName]struct{})
	for _, attribute := range definition.Attributes {
		name := expandedName{
			namespace: attribute.Name.Namespace,
			local:     attribute.Name.Local,
		}
		assert.NotContains(t, names, name)
		names[name] = struct{}{}
	}
}

func assertExpandedParticle(
	t *testing.T,
	particle *ParticlePlan,
) {
	t.Helper()

	if particle == nil {
		return
	}

	assert.NotEqual(t, ParticleGroup, particle.Kind)

	if particle.Element != nil &&
		particle.Element.Type != nil &&
		particle.Element.Type.InlineComplex != nil {
		assertExpandedComplexTypeDefinition(
			t,
			particle.Element.Type.InlineComplex,
		)
	}

	for index := range particle.Children {
		assertExpandedParticle(t, &particle.Children[index])
	}
}

func attributeLocalNames(attributes []AttributePlan) []string {
	result := make([]string, len(attributes))
	for index, attribute := range attributes {
		result[index] = attribute.Name.Local
	}

	return result
}
