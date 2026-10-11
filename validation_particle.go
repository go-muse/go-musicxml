package musicxml

import "slices"

// Content matching retains endpoints, not parse histories. Occurrence counts
// belong to intermediate repetition states; only a fully constrained particle
// can be memoized by (particle, start). Empty occurrences are padded symbolically.
// Witnesses are reconstructed only for the selected endpoint using temporary
// predecessor tables, never copied into every cached endpoint.
type validationParticleKey struct {
	particle *validationParticleSchema
	start    int
}

type validationParticleEndpoints struct {
	ends    []int // descending, unique; deterministic longest-first traversal
	failure validationMatchFailure
}

// These optional counters support algorithmic regression tests. They are not a
// public budget, cancellation mechanism, or an incomplete-assessment contract.
type validationParticleWork struct {
	particles, bodies, transitions, repeatStates, witnessStates int64
}

type validationParticleMatcher struct {
	context   *validationContext
	children  []*validationNode
	particles map[validationParticleKey]validationParticleEndpoints
	bodies    map[validationParticleKey]validationParticleEndpoints
	nullable  map[*validationParticleSchema]bool // body, before its occurrence range
	work      *validationParticleWork
}

func newValidationParticleMatcher(c *validationContext, children []*validationNode) *validationParticleMatcher {
	return &validationParticleMatcher{context: c, children: children,
		particles: make(map[validationParticleKey]validationParticleEndpoints),
		bodies:    make(map[validationParticleKey]validationParticleEndpoints),
		nullable:  make(map[*validationParticleSchema]bool)}
}

func (c *validationContext) matchParticle(particle *validationParticleSchema, children []*validationNode, position int) (validationMatchResult, validationMatchFailure, bool) {
	return newValidationParticleMatcher(c, children).run(particle, position)
}

func (m *validationParticleMatcher) run(p *validationParticleSchema, start int) (validationMatchResult, validationMatchFailure, bool) {
	if start < 0 || start > len(m.children) {
		return validationMatchResult{}, validationMatchFailure{schema: "invalid content-model position"}, false
	}
	if p == nil {
		return validationMatchResult{next: start}, validationMatchFailure{}, true
	}
	for _, child := range m.children {
		if child == nil {
			return validationMatchResult{}, validationMatchFailure{schema: "missing content-model child"}, false
		}
	}
	if !m.prepare(p, make(map[*validationParticleSchema]bool)) {
		return validationMatchResult{}, validationMatchFailure{schema: "invalid or cyclic content-model metadata"}, false
	}
	endpoints := m.match(p, start)
	if len(endpoints.ends) == 0 {
		return validationMatchResult{}, endpoints.failure, false
	}
	result := validationMatchResult{next: endpoints.ends[0]}
	if !m.witness(p, start, result.next, &result.elements) {
		return validationMatchResult{}, validationMatchFailure{schema: "content-model witness cannot be reconstructed"}, false
	}
	return result, endpoints.failure, true
}

// This structural guard is not a complete XSD schema-validity or UPA checker.
// In particular wildcard namespace/processContents support remains unchanged.
func (m *validationParticleMatcher) prepare(p *validationParticleSchema, active map[*validationParticleSchema]bool) bool {
	if p == nil || active[p] {
		return false
	}
	if _, ok := m.nullable[p]; ok {
		return true
	}
	if !p.Occurrence.Unbounded && p.Occurrence.Min > p.Occurrence.Max {
		return false
	}
	active[p] = true
	defer delete(active, p)
	nullable := false
	switch p.Kind {
	case validationParticleElement:
		if p.Element == nil || p.Any != nil || len(p.Children) != 0 {
			return false
		}
		e, ok := m.context.resolveElement(p.Element)
		if !ok || e == nil || e.Name.Local == "" {
			return false
		}
	case validationParticleAny:
		if p.Any == nil || p.Element != nil || len(p.Children) != 0 {
			return false
		}
	case validationParticleSequence, validationParticleChoice, validationParticleAll:
		if p.Element != nil || p.Any != nil {
			return false
		}
		nullable = p.Kind != validationParticleChoice
		names := make(map[validationQName]bool)
		for _, child := range p.Children {
			if !m.prepare(child, active) {
				return false
			}
			if p.Kind == validationParticleAll {
				if child.Kind != validationParticleElement || child.Occurrence.Unbounded || child.Occurrence.Max > 1 {
					return false
				}
				// A maxOccurs=0 leaf is absent from the effective all group.
				// Still validate its structure above, but do not let its name
				// conflict with a distinct, active particle.
				if child.Occurrence.Max != 0 {
					e, _ := m.context.resolveElement(child.Element)
					if names[e.Name] {
						return false
					}
					names[e.Name] = true
				}
			}
			empty := child.Occurrence.Min == 0 || m.nullable[child]
			if p.Kind == validationParticleChoice {
				nullable = nullable || empty
			} else {
				nullable = nullable && empty
			}
		}
		if p.Kind == validationParticleAll && (p.Occurrence.Unbounded || p.Occurrence.Max > 1) {
			return false
		}
	default:
		return false
	}
	m.nullable[p] = nullable
	return true
}

func (m *validationParticleMatcher) transition() {
	if m.work != nil {
		m.work.transitions++
	}
}

func validationOrderedPositions(seen map[int]bool) []int {
	positions := make([]int, 0, len(seen))
	for position := range seen {
		positions = append(positions, position)
	}
	slices.Sort(positions)
	slices.Reverse(positions)
	return positions
}

func (m *validationParticleMatcher) limit(p *validationParticleSchema, start int) int {
	limit := len(m.children) - start
	if !p.Occurrence.Unbounded && p.Occurrence.Max < uint64(limit) {
		limit = int(p.Occurrence.Max)
	}
	return limit
}

func (m *validationParticleMatcher) match(p *validationParticleSchema, start int) validationParticleEndpoints {
	key := validationParticleKey{p, start}
	if cached, ok := m.particles[key]; ok {
		return cached
	}
	if m.work != nil {
		m.work.particles++
	}
	result := validationParticleEndpoints{failure: validationMatchFailure{position: start}}
	accepted := make(map[int]bool)
	if p.Occurrence.Min == 0 || m.nullable[p] {
		accepted[start] = true
	}
	current := []int{start}
	for count := 1; count <= m.limit(p, start) && len(current) > 0; count++ {
		next := make(map[int]bool)
		for _, position := range current {
			if m.work != nil {
				m.work.repeatStates++
			}
			body := m.body(p, position)
			result.failure = betterValidationFailure(result.failure, body.failure)
			for _, end := range body.ends {
				m.transition()
				if end == position {
					continue
				} // any needed empty occurrences can be padding
				next[end] = true
				if m.nullable[p] || uint64(count) >= p.Occurrence.Min {
					accepted[end] = true
				}
			}
		}
		current = validationOrderedPositions(next)
	}
	// Even at EOF, inspect the body for accurate missing-child diagnostics. A
	// maxOccurs=0 particle must not attempt a prohibited occurrence.
	if len(accepted) == 0 && (p.Occurrence.Unbounded || p.Occurrence.Max > 0) {
		result.failure = betterValidationFailure(result.failure, m.body(p, start).failure)
		for _, position := range current {
			result.failure = betterValidationFailure(result.failure, m.body(p, position).failure)
		}
	}
	result.ends = validationOrderedPositions(accepted)
	m.particles[key] = result
	return result
}

func (m *validationParticleMatcher) body(p *validationParticleSchema, start int) validationParticleEndpoints {
	key := validationParticleKey{p, start}
	if cached, ok := m.bodies[key]; ok {
		return cached
	}
	if m.work != nil {
		m.work.bodies++
	}
	result := validationParticleEndpoints{failure: validationMatchFailure{position: start}}
	switch p.Kind {
	case validationParticleElement:
		e, _ := m.context.resolveElement(p.Element)
		if start < len(m.children) && validationName(m.children[start].Name) == e.Name {
			result.ends = []int{start + 1}
		} else {
			result.failure.expected = []validationQName{e.Name}
		}
	case validationParticleAny:
		// Preserve the legacy wildcard behavior. Namespace and processContents
		// semantics need their separate support contract; this repair does not
		// advertise that they have been verified.
		if start < len(m.children) {
			result.ends = []int{start + 1}
		}
	case validationParticleChoice:
		ends := make(map[int]bool)
		for _, child := range p.Children {
			branch := m.match(child, start)
			result.failure = betterValidationFailure(result.failure, branch.failure)
			for _, end := range branch.ends {
				m.transition()
				ends[end] = true
			}
		}
		result.ends = validationOrderedPositions(ends)
	case validationParticleSequence:
		current := []int{start}
		for _, child := range p.Children {
			ends := make(map[int]bool)
			for _, position := range current {
				branch := m.match(child, position)
				result.failure = betterValidationFailure(result.failure, branch.failure)
				for _, end := range branch.ends {
					m.transition()
					ends[end] = true
				}
			}
			current = validationOrderedPositions(ends)
			if len(current) == 0 {
				break
			}
		}
		result.ends = current
	case validationParticleAll:
		end, failure, ok := m.all(p, start, nil)
		result.failure = failure
		if ok {
			result.ends = []int{end}
		}
	}
	m.bodies[key] = result
	return result
}

// XSD 1.0 all children are distinct, singly occurring element particles. There
// is no combinatorial subset search; prepare rejects malformed all metadata.
func (m *validationParticleMatcher) all(p *validationParticleSchema, start int, elements *[]validationElementMatch) (int, validationMatchFailure, bool) {
	used := make([]bool, len(p.Children))
	end := start
	for end < len(m.children) {
		found := false
		for index, child := range p.Children {
			m.transition()
			if used[index] || child.Occurrence.Max == 0 {
				continue
			}
			e, _ := m.context.resolveElement(child.Element)
			if validationName(m.children[end].Name) != e.Name {
				continue
			}
			used[index] = true
			found = true
			if elements != nil {
				*elements = append(*elements, validationElementMatch{index: end, schema: e})
			}
			end++
			break
		}
		if !found {
			break
		}
	}
	failure := validationMatchFailure{position: end}
	for index, child := range p.Children {
		if !used[index] && child.Occurrence.Min > 0 {
			e, _ := m.context.resolveElement(child.Element)
			failure.expected = append(failure.expected, e.Name)
		}
	}
	return end, failure, len(failure.expected) == 0
}

// predecessor layers retain only integer positions. They are local to witness
// reconstruction and released before descending into the selected child paths.
// Zero-progress repetition edges are excluded, exactly as in match.
func (m *validationParticleMatcher) witness(p *validationParticleSchema, start, end int, elements *[]validationElementMatch) bool {
	if start == end {
		return p.Occurrence.Min == 0 || m.nullable[p]
	}
	layers := []map[int]int{nil}
	current := []int{start}
	for count := 1; count <= m.limit(p, start) && len(current) > 0; count++ {
		previous := make(map[int]int)
		for _, position := range current {
			for _, next := range m.body(p, position).ends {
				m.transition()
				if next <= position || next > end {
					continue
				}
				if _, ok := previous[next]; !ok {
					previous[next] = position
					if m.work != nil {
						m.work.witnessStates++
					}
				}
			}
		}
		layers = append(layers, previous)
		if _, ok := previous[end]; ok && (m.nullable[p] || uint64(count) >= p.Occurrence.Min) {
			positions := make([]int, count+1)
			positions[count] = end
			for index := count; index > 0; index-- {
				positions[index-1] = layers[index][positions[index]]
			}
			// No cached endpoint owns these tables. Only the selected split survives.
			layers = nil
			for index := 0; index < count; index++ {
				if !m.bodyWitness(p, positions[index], positions[index+1], elements) {
					return false
				}
			}
			return true
		}
		seen := make(map[int]bool, len(previous))
		for next := range previous {
			seen[next] = true
		}
		current = validationOrderedPositions(seen)
	}
	return false
}

func (m *validationParticleMatcher) bodyWitness(p *validationParticleSchema, start, end int, elements *[]validationElementMatch) bool {
	switch p.Kind {
	case validationParticleElement:
		e, _ := m.context.resolveElement(p.Element)
		*elements = append(*elements, validationElementMatch{index: start, schema: e})
		return end == start+1
	case validationParticleAny:
		return end == start+1
	case validationParticleChoice:
		for _, child := range p.Children {
			if slices.Contains(m.match(child, start).ends, end) {
				return m.witness(child, start, end, elements)
			}
		}
	case validationParticleSequence:
		layers := make([]map[int]int, 0, len(p.Children))
		current := []int{start}
		for _, child := range p.Children {
			previous := make(map[int]int)
			for _, position := range current {
				for _, next := range m.match(child, position).ends {
					m.transition()
					if next > end {
						continue
					}
					if _, ok := previous[next]; !ok {
						previous[next] = position
						if m.work != nil {
							m.work.witnessStates++
						}
					}
				}
			}
			layers = append(layers, previous)
			seen := make(map[int]bool, len(previous))
			for next := range previous {
				seen[next] = true
			}
			current = validationOrderedPositions(seen)
		}
		if !slices.Contains(current, end) {
			return false
		}
		positions := make([]int, len(p.Children)+1)
		positions[len(p.Children)] = end
		for index := len(p.Children) - 1; index >= 0; index-- {
			positions[index] = layers[index][positions[index+1]]
		}
		layers = nil
		for index, child := range p.Children {
			if !m.witness(child, positions[index], positions[index+1], elements) {
				return false
			}
		}
		return true
	case validationParticleAll:
		next, _, ok := m.all(p, start, elements)
		return ok && next == end
	}
	return false
}
