package musicxml

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationOfficialScoreAllocations(t *testing.T) {
	// Keep this serial: AllocsPerRun reads process-wide counters. This broad
	// allocation ceiling catches eager particle-diagnostic regressions without
	// depending on host speed or an exact compiler allocation count.
	input, err := os.ReadFile("testdata/official/MozaChloSample.musicxml")
	require.NoError(t, err)
	document, err := Decode(bytes.NewReader(input))
	require.NoError(t, err)
	allocations := testing.AllocsPerRun(3, func() {
		require.NoError(t, Validate(document))
	})
	t.Logf("decoded official score validation: %.0f allocations", allocations)
	assert.Less(t, allocations, float64(200000))
}

func TestGrammarFailureDiagnosticsStable(t *testing.T) {
	t.Parallel()
	sequence := func(children ...*validationParticleSchema) *validationParticleSchema {
		return &validationParticleSchema{Kind: validationParticleSequence, Occurrence: validationOccurrence{Min: 1, Max: 1}, Children: children}
	}
	choice := &validationParticleSchema{Kind: validationParticleChoice, Occurrence: validationOccurrence{Min: 1, Max: 1}, Children: []*validationParticleSchema{
		sequence(grammarTestLeaf("a", 1, 1), grammarTestLeaf("c", 1, 1)),
		sequence(grammarTestLeaf("a", 1, 1), grammarTestLeaf("b", 1, 1)),
		sequence(grammarTestLeaf("a", 1, 1), grammarTestLeaf("c", 1, 1)),
	}}
	for _, test := range []struct {
		name     string
		particle *validationParticleSchema
		count    int
		position int
		expected []validationQName
	}{
		{"missing all reverse declaration order", &validationParticleSchema{
			Kind: validationParticleAll, Occurrence: validationOccurrence{Min: 1, Max: 1},
			Children: []*validationParticleSchema{grammarTestLeaf("b", 1, 1), grammarTestLeaf("a", 1, 1)},
		}, 0, 0, []validationQName{{Local: "a"}, {Local: "b"}}},
		{"missing first", choice, 0, 0, []validationQName{{Local: "a"}}},
		{"missing alternatives", choice, 1, 1, []validationQName{{Local: "b"}, {Local: "c"}}},
		{"repeated minimum", grammarTestLeaf("a", 2, 2), 1, 1, []validationQName{{Local: "a"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			for range 3 {
				_, failure, ok := newTestValidationContext(&validationSchemaSet{}).MatchParticle(test.particle, grammarTestNodes(test.count), 0)
				assert.False(t, ok)
				assert.Empty(t, failure.Schema)
				assert.Equal(t, test.position, failure.Position)
				assert.Equal(t, test.expected, failure.Expected)
			}
		})
	}
}

func TestGrammarSuccessfulMatchesDeferDiagnostics(t *testing.T) {
	t.Parallel()
	p := &validationParticleSchema{Kind: validationParticleChoice, Occurrence: validationOccurrence{Min: 1, Max: 1}, Children: []*validationParticleSchema{
		grammarTestLeaf("b", 1, 1), grammarTestLeaf("a", 1, 1),
	}}
	for _, count := range []int{1, 2} {
		m := newValidationParticleMatcher(newTestValidationContext(&validationSchemaSet{}), grammarTestNodes(count))
		result, failure, ok := m.Run(p, 0)
		require.True(t, ok)
		assert.Equal(t, 1, result.Next) // A successful prefix also needs no diagnostics.
		assert.Empty(t, failure)
		assert.False(t, m.Diagnostics)
		for _, entries := range []map[validationParticleKey]validationParticleEndpoints{m.Particles, m.Bodies} {
			for _, entry := range entries {
				assert.Nil(t, entry.Failure)
			}
		}
	}
}

func TestGrammarPreparationCacheIsValidationLocal(t *testing.T) {
	t.Parallel()
	p := grammarTestLeaf("a", 1, 1)
	first := newTestValidationContext(&validationSchemaSet{})
	result, _, ok := first.MatchParticle(p, grammarTestNodes(1), 0)
	require.True(t, ok)
	require.Len(t, result.Elements, 1)
	assert.Same(t, p.Element, result.Elements[0].Schema)
	assert.Contains(t, first.ParticleNullable, p)

	// A schema change between independent validations must be guarded again.
	// A validationContext, like its effective-type cache, is scoped to one
	// immutable schema assessment and is not reused across Validate calls.
	p.Occurrence = validationOccurrence{Min: 2, Max: 1}
	second := newTestValidationContext(&validationSchemaSet{})
	_, failure, ok := second.MatchParticle(p, grammarTestNodes(1), 0)
	assert.False(t, ok)
	assert.NotEmpty(t, failure.Schema)
	assert.NotContains(t, second.ParticleNullable, p)
	assert.Contains(t, first.ParticleNullable, p)
}

func TestGrammarSingletonViewsPreserveAlternatives(t *testing.T) {
	t.Parallel()
	m := newValidationParticleMatcher(newTestValidationContext(&validationSchemaSet{}), grammarTestNodes(2))
	p := grammarTestLeaf("a", 0, 1)
	require.True(t, m.Prepare(p, make(map[*validationParticleSchema]bool)))
	first := m.Match(p, 0)
	second := m.Match(p, 1)
	assert.Equal(t, []int{1, 0}, first.Ends)
	assert.Equal(t, []int{2, 1}, second.Ends)
	assert.Equal(t, len(first.Ends), cap(first.Ends))
	assert.Equal(t, len(second.Ends), cap(second.Ends))
	assert.Equal(t, []int{2}, m.Match(p, 2).Ends)
	assert.Equal(t, []int{1, 0}, first.Ends)
	assert.Equal(t, []int{2, 1}, second.Ends)
}
