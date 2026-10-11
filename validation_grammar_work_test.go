package musicxml

import (
	"encoding/xml"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func grammarTestLeaf(name string, min, max uint64) *validationParticleSchema {
	return &validationParticleSchema{Kind: validationParticleElement, Occurrence: validationOccurrence{Min: min, Max: max}, Element: &validationElementSchema{Name: validationQName{Local: name}, Type: validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "string"}}}}
}

func grammarTestNodes(count int) []*validationNode {
	nodes := make([]*validationNode, count)
	for index := range nodes {
		nodes[index] = &validationNode{Name: xml.Name{Local: "a"}}
	}
	return nodes
}

func TestGrammarMetadataFailures(t *testing.T) {
	t.Parallel()
	cycle := &validationParticleSchema{Kind: validationParticleSequence, Occurrence: validationOccurrence{Min: 1, Max: 1}}
	cycle.Children = []*validationParticleSchema{cycle}
	indirect := &validationParticleSchema{Kind: validationParticleChoice, Occurrence: validationOccurrence{Min: 0, Max: 1}}
	indirect.Children = []*validationParticleSchema{{Kind: validationParticleSequence, Occurrence: validationOccurrence{Min: 1, Max: 1}, Children: []*validationParticleSchema{indirect}}}
	for name, p := range map[string]*validationParticleSchema{
		"missing child":              {Kind: validationParticleSequence, Occurrence: validationOccurrence{Min: 0, Max: 1}, Children: []*validationParticleSchema{nil}},
		"missing element":            {Kind: validationParticleElement, Occurrence: validationOccurrence{Min: 0, Max: 1}},
		"missing reference":          {Kind: validationParticleElement, Occurrence: validationOccurrence{Min: 0, Max: 1}, Element: &validationElementSchema{Reference: validationQName{Local: "absent"}}},
		"inverted bounds":            grammarTestLeaf("a", 2, 1),
		"unknown kind":               {Kind: "not-a-kind", Occurrence: validationOccurrence{Min: 0, Max: 1}},
		"missing wildcard":           {Kind: validationParticleAny, Occurrence: validationOccurrence{Min: 0, Max: 1}},
		"element with group payload": {Kind: validationParticleElement, Occurrence: validationOccurrence{Min: 0, Max: 1}, Element: grammarTestLeaf("a", 1, 1).Element, Children: []*validationParticleSchema{grammarTestLeaf("b", 1, 1)}},
		"all duplicate":              {Kind: validationParticleAll, Occurrence: validationOccurrence{Min: 1, Max: 1}, Children: []*validationParticleSchema{grammarTestLeaf("a", 0, 1), grammarTestLeaf("a", 0, 1)}},
		"all repeated":               {Kind: validationParticleAll, Occurrence: validationOccurrence{Min: 1, Max: 2}},
		"all child repeated":         {Kind: validationParticleAll, Occurrence: validationOccurrence{Min: 1, Max: 1}, Children: []*validationParticleSchema{grammarTestLeaf("a", 0, 2)}},
		"cycle":                      cycle, "indirect cycle": indirect,
	} {
		t.Run(name, func(t *testing.T) {
			context := newTestValidationContext(&validationSchemaSet{})
			result, failure, ok := context.matchParticle(p, nil, 0)
			assert.False(t, ok)
			assert.NotEmpty(t, failure.schema)
			assert.Empty(t, result.elements)
		})
	}
	for _, position := range []int{-1, 1} {
		_, failure, ok := newTestValidationContext(&validationSchemaSet{}).matchParticle(grammarTestLeaf("a", 1, 1), nil, position)
		assert.False(t, ok)
		assert.NotEmpty(t, failure.schema)
	}
	_, failure, ok := newTestValidationContext(&validationSchemaSet{}).matchParticle(grammarTestLeaf("a", 1, 1), []*validationNode{nil}, 0)
	assert.False(t, ok)
	assert.NotEmpty(t, failure.schema)
}

func TestGrammarNullableSymbolicCounts(t *testing.T) {
	t.Parallel()
	for _, minimum := range []uint64{0, 1, 3, math.MaxUint64} {
		for _, unbounded := range []bool{false, true} {
			p := &validationParticleSchema{Kind: validationParticleSequence, Occurrence: validationOccurrence{Min: minimum, Max: math.MaxUint64, Unbounded: unbounded}, Children: []*validationParticleSchema{grammarTestLeaf("a", 0, 1)}}
			for _, count := range []int{0, 1, 4} {
				context := newTestValidationContext(&validationSchemaSet{})
				m := newValidationParticleMatcher(context, grammarTestNodes(count))
				m.work = &validationParticleWork{}
				result, failure, ok := m.run(p, 0)
				require.True(t, ok, "min=%d unbounded=%v failure=%+v", minimum, unbounded, failure)
				assert.Equal(t, count, result.next)
				assert.Len(t, result.elements, count)
				assert.LessOrEqual(t, m.work.transitions, int64(100*(count+1)*(count+1)), "work must not depend on numeric occurrence magnitude")
			}
		}
	}
	// A zero maximum accepts no occurrence, even if the body would consume.
	p := grammarTestLeaf("a", 0, 0)
	result, _, ok := newTestValidationContext(&validationSchemaSet{}).matchParticle(p, grammarTestNodes(1), 0)
	require.True(t, ok)
	assert.Zero(t, result.next)
	assert.Empty(t, result.elements)
	// A huge nonnullable minimum fails after input-bounded work.
	p = grammarTestLeaf("a", math.MaxUint64, math.MaxUint64)
	m := newValidationParticleMatcher(newTestValidationContext(&validationSchemaSet{}), grammarTestNodes(4))
	m.work = &validationParticleWork{}
	_, _, ok = m.run(p, 0)
	assert.False(t, ok)
	// Invalid content can require one diagnostic replay of the input-bounded DP.
	assert.LessOrEqual(t, m.work.transitions, int64(8))
}

func TestGrammarWitnessBindingsAndCache(t *testing.T) {
	t.Parallel()
	root, err := parseValidationDocument([]byte(`<branches><a>1</a><b/><c>true</c><d/><a>2</a></branches>`))
	require.NoError(t, err)
	context := newTestValidationContext(&validationGrammarGenerated)
	_, complex, _, _ := context.resolveType(&validationGrammarGenerated.Elements[validationQName{Local: "branches"}].Type)
	for range 3 {
		m := newValidationParticleMatcher(context, root.Children)
		result, _, ok := m.run(complex.Particle, 0)
		require.True(t, ok)
		require.Len(t, result.elements, len(root.Children))
		for index, match := range result.elements {
			assert.Equal(t, index, match.index)
			assert.Equal(t, validationName(root.Children[index].Name), match.schema.Name)
		}
		for _, entries := range []map[validationParticleKey]validationParticleEndpoints{m.particles, m.bodies} {
			for _, entry := range entries {
				for index := 1; index < len(entry.ends); index++ {
					assert.Greater(t, entry.ends[index-1], entry.ends[index])
				}
			}
		}
	}
	// QName comparison must not collapse same-local-name elements.
	p := grammarTestLeaf("a", 1, 1)
	p.Element.Name.Space = "urn:expected"
	nodes := grammarTestNodes(1)
	_, _, ok := context.matchParticle(p, nodes, 0)
	assert.False(t, ok)
	nodes[0].Name.Space = "urn:expected"
	result, _, ok := context.matchParticle(p, nodes, 0)
	require.True(t, ok)
	assert.Same(t, p.Element, result.elements[0].schema)
}

func TestGrammarMatchingWorkBound(t *testing.T) {
	// Nonparallel: AllocsPerRun uses process-wide allocation statistics.
	for _, valid := range []bool{false, true} {
		measure := func(n int) float64 {
			repeated := &validationParticleSchema{Kind: validationParticleSequence, Occurrence: validationOccurrence{Min: uint64(n / 2), Max: uint64(n / 2)}, Children: []*validationParticleSchema{grammarTestLeaf("a", 1, 2)}}
			p := &validationParticleSchema{Kind: validationParticleSequence, Occurrence: validationOccurrence{Min: 1, Max: 1}, Children: []*validationParticleSchema{repeated, grammarTestLeaf("b", 1, 1)}}
			nodes := append(grammarTestNodes(n), &validationNode{Name: xml.Name{Local: "c"}})
			if valid {
				nodes[n].Name.Local = "b"
			}
			var work validationParticleWork
			allocations := testing.AllocsPerRun(3, func() {
				m := newValidationParticleMatcher(newTestValidationContext(&validationSchemaSet{}), nodes)
				m.work = &validationParticleWork{}
				result, failure, ok := m.run(p, 0)
				if valid {
					require.True(t, ok, "%+v", failure)
					require.Equal(t, len(nodes), result.next)
					require.Len(t, result.elements, len(nodes))
				} else {
					require.False(t, ok)
				}
				work = *m.work
				// Exactly four particle components, at most N+1 starts each; every memo
				// entry stores at most N+1 endpoints and no retained witness/history.
				bound := 4 * (len(nodes) + 1)
				require.LessOrEqual(t, len(m.particles), bound)
				require.LessOrEqual(t, len(m.bodies), bound)
				positions := 0
				for _, entry := range m.particles {
					positions += len(entry.ends)
				}
				for _, entry := range m.bodies {
					positions += len(entry.ends)
				}
				require.LessOrEqual(t, positions, 2*bound*(len(nodes)+1))
				N := int64(len(nodes) + 1)
				require.LessOrEqual(t, work.transitions, 8*N*N*N*N)
				require.LessOrEqual(t, work.repeatStates, 4*N*N*N)
			})
			t.Logf("valid=%v children=%d allocations=%.0f particle states=%d body states=%d transitions=%d repetition states=%d witness states=%d", valid, n, allocations, work.particles, work.bodies, work.transitions, work.repeatStates, work.witnessStates)
			return allocations
		}
		small, large := measure(16), measure(32)
		assert.LessOrEqual(t, large, 8*small, "endpoint DP must not enumerate parse histories")
	}
}

// Enumerate a small bounded language independently of the endpoint matcher.
// This deliberately expands whole words and ordinary empty occurrences; it
// does not share memoization, nullable padding or witness reconstruction logic.
func grammarTestLanguage(t *testing.T, p *validationParticleSchema, length int) map[string]bool {
	t.Helper()
	product := func(left, right map[string]bool) map[string]bool {
		result := make(map[string]bool)
		for a := range left {
			for b := range right {
				if len(a)+len(b) <= length {
					result[a+b] = true
				}
			}
		}
		return result
	}
	body := make(map[string]bool)
	switch p.Kind {
	case validationParticleElement:
		require.Len(t, p.Element.Name.Local, 1)
		require.Empty(t, p.Element.Name.Space)
		body[p.Element.Name.Local] = true
	case validationParticleSequence:
		body[""] = true
		for _, child := range p.Children {
			body = product(body, grammarTestLanguage(t, child, length))
		}
	case validationParticleChoice:
		for _, child := range p.Children {
			for word := range grammarTestLanguage(t, child, length) {
				body[word] = true
			}
		}
	default:
		t.Fatalf("unsupported independent-test grammar kind %q", p.Kind)
	}
	// The generated test schemas have tiny minima. With words capped at length,
	// additional empty repetitions need at most min+length occurrences.
	limit := int(p.Occurrence.Min) + length
	if !p.Occurrence.Unbounded && p.Occurrence.Max < uint64(limit) {
		limit = int(p.Occurrence.Max)
	}
	accepted := make(map[string]bool)
	current := map[string]bool{"": true}
	if p.Occurrence.Min == 0 {
		accepted[""] = true
	}
	for count := 1; count <= limit; count++ {
		current = product(current, body)
		if uint64(count) >= p.Occurrence.Min {
			for word := range current {
				accepted[word] = true
			}
		}
	}
	return accepted
}

func TestGrammarExhaustiveShortLanguages(t *testing.T) {
	t.Parallel()
	words := []string{""}
	level := []string{""}
	for range 4 {
		var next []string
		for _, word := range level {
			for _, letter := range "abcd" {
				next = append(next, word+string(letter))
			}
		}
		words = append(words, next...)
		level = next
	}
	for _, name := range []string{"nullable-sequence", "nullable-choice", "nullable-unbounded", "nullable-zero", "partition-fixed", "partition-continuation", "branches"} {
		t.Run(name, func(t *testing.T) {
			context := newTestValidationContext(&validationGrammarGenerated)
			_, complex, _, _ := context.resolveType(&validationGrammarGenerated.Elements[validationQName{Local: name}].Type)
			want := grammarTestLanguage(t, complex.Particle, 4)
			for _, word := range words {
				nodes := make([]*validationNode, len(word))
				for index, letter := range word {
					nodes[index] = &validationNode{Name: xml.Name{Local: string(letter)}}
				}
				result, _, ok := context.matchParticle(complex.Particle, nodes, 0)
				assert.Equal(t, want[word], ok && result.next == len(nodes), "word %q", word)
				if ok {
					assert.Len(t, result.elements, result.next)
					for index, match := range result.elements {
						assert.Equal(t, index, match.index)
						assert.Equal(t, nodes[index].Name.Local, match.schema.Name.Local)
					}
				}
			}
		})
	}
}
