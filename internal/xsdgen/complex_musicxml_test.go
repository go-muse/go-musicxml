package xsdgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanMusicXMLComplexTypes(t *testing.T) {
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

	plans, err := PlanComplexTypes(index)
	require.NoError(t, err)
	require.Len(t, plans, 224)

	forms := map[ComplexTypeForm]int{}
	stats := complexPlanStats{
		particles: make(map[ParticleKind]int),
	}

	for _, plan := range plans {
		forms[plan.Definition.Form]++
		collectComplexDefinitionStats(plan.Definition, &stats)
	}

	assert.Equal(t, 137, forms[ComplexTypeDirect])
	assert.Equal(
		t,
		82,
		forms[ComplexTypeSimpleContentExtension],
	)
	assert.Zero(t, forms[ComplexTypeSimpleContentRestriction])
	assert.Equal(
		t,
		5,
		forms[ComplexTypeComplexContentExtension],
	)
	assert.Zero(t, forms[ComplexTypeComplexContentRestriction])

	assert.Equal(t, 399, stats.particles[ParticleElement])
	assert.Equal(t, 53, stats.particles[ParticleGroup])
	assert.Equal(t, 32, stats.particles[ParticleChoice])
	assert.Equal(t, 89, stats.particles[ParticleSequence])
	assert.Zero(t, stats.particles[ParticleAll])
	assert.Zero(t, stats.particles[ParticleAny])
	assert.Equal(t, 79, stats.unboundedParticles)
	assert.Equal(t, 5, stats.boundedRepeatedParticles)
	assert.Equal(t, 262, stats.attributes)
	assert.Equal(t, 55, stats.requiredAttributes)
	assert.Equal(t, 296, stats.attributeGroups)

	attributes := requireComplexTypePlan(t, plans, "attributes")
	key := requireElementPlan(
		t,
		attributes.Definition.Particle,
		"key",
	)
	assert.Equal(
		t,
		OccurrenceRange{Min: 0, Unbounded: true},
		key.occurrence,
	)
	assert.Equal(
		t,
		DeclarationComplexType,
		key.element.Type.Declaration.Kind,
	)
	assert.Equal(
		t,
		"key",
		key.element.Type.Declaration.Name.Local,
	)

	directive := requireElementPlan(
		t,
		attributes.Definition.Particle,
		"directive",
	)
	require.NotNil(t, directive.element.Type.InlineComplex)
	assert.Equal(
		t,
		ComplexTypeSimpleContentExtension,
		directive.element.Type.InlineComplex.Form,
	)
	assert.Equal(
		t,
		"string",
		directive.element.Type.InlineComplex.Base.Name.Local,
	)

	accidentalText := requireComplexTypePlan(
		t,
		plans,
		"accidental-text",
	)
	assert.Equal(
		t,
		ComplexTypeSimpleContentExtension,
		accidentalText.Definition.Form,
	)
	assert.Equal(
		t,
		"accidental-value",
		accidentalText.Definition.Base.Name.Local,
	)

	metronomeTuplet := requireComplexTypePlan(
		t,
		plans,
		"metronome-tuplet",
	)
	assert.Equal(
		t,
		ComplexTypeComplexContentExtension,
		metronomeTuplet.Definition.Form,
	)
	assert.Equal(
		t,
		"time-modification",
		metronomeTuplet.Definition.Base.Name.Local,
	)
	require.Len(t, metronomeTuplet.Definition.Attributes, 3)
	assert.True(t, metronomeTuplet.Definition.Attributes[0].Required())

	link := requireComplexTypePlan(t, plans, "link")
	require.NotEmpty(t, link.Definition.AttributeGroups)
	assert.Equal(
		t,
		"link-attributes",
		link.Definition.AttributeGroups[0].Reference.Name.Local,
	)
}

type complexPlanStats struct {
	particles                map[ParticleKind]int
	unboundedParticles       int
	boundedRepeatedParticles int
	attributes               int
	requiredAttributes       int
	attributeGroups          int
}

type foundElementPlan struct {
	element    *ElementPlan
	occurrence OccurrenceRange
}

func collectComplexDefinitionStats(
	definition *ComplexTypeDefinition,
	stats *complexPlanStats,
) {
	stats.attributes += len(definition.Attributes)
	stats.attributeGroups += len(definition.AttributeGroups)

	for _, attribute := range definition.Attributes {
		if attribute.Required() {
			stats.requiredAttributes++
		}
	}

	collectParticleStats(definition.Particle, stats)
}

func collectParticleStats(
	particle *ParticlePlan,
	stats *complexPlanStats,
) {
	if particle == nil {
		return
	}

	stats.particles[particle.Kind]++
	if particle.Occurrence.Unbounded {
		stats.unboundedParticles++
	} else if particle.Occurrence.Max > 1 {
		stats.boundedRepeatedParticles++
	}

	if particle.Element != nil &&
		particle.Element.Type != nil &&
		particle.Element.Type.InlineComplex != nil {
		collectComplexDefinitionStats(
			particle.Element.Type.InlineComplex,
			stats,
		)
	}

	for index := range particle.Children {
		collectParticleStats(&particle.Children[index], stats)
	}
}

func requireElementPlan(
	t *testing.T,
	particle *ParticlePlan,
	name string,
) foundElementPlan {
	t.Helper()

	if particle == nil {
		require.Failf(t, "missing element plan", "name: %s", name)
		return foundElementPlan{}
	}

	if particle.Element != nil &&
		particle.Element.Name.Local == name {
		return foundElementPlan{
			element:    particle.Element,
			occurrence: particle.Occurrence,
		}
	}

	for index := range particle.Children {
		found := findElementPlan(&particle.Children[index], name)
		if found.element != nil {
			return found
		}
	}

	require.Failf(t, "missing element plan", "name: %s", name)
	return foundElementPlan{}
}

func findElementPlan(
	particle *ParticlePlan,
	name string,
) foundElementPlan {
	if particle == nil {
		return foundElementPlan{}
	}

	if particle.Element != nil &&
		particle.Element.Name.Local == name {
		return foundElementPlan{
			element:    particle.Element,
			occurrence: particle.Occurrence,
		}
	}

	for index := range particle.Children {
		found := findElementPlan(&particle.Children[index], name)
		if found.element != nil {
			return found
		}
	}

	return foundElementPlan{}
}
