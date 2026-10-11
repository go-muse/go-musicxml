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
	ends    []int                   // descending, unique; deterministic longest-first traversal
	failure *validationMatchFailure // nil until a failed run requests diagnostics
}

// These optional counters support algorithmic regression tests. They are not a
// public budget, cancellation mechanism, or an incomplete-assessment contract.
type validationParticleWork struct {
	particles, bodies, transitions, repeatStates, witnessStates int64
}

type validationParticleMatcher struct {
	context     *validationContext
	children    []*validationNode
	particles   map[validationParticleKey]validationParticleEndpoints
	bodies      map[validationParticleKey]validationParticleEndpoints
	nullable    map[*validationParticleSchema]bool // body, before its occurrence range
	singletons  []int                              // immutable singleton endpoint views, owned by this match
	diagnostics bool                               // collect failure names only after an unsuccessful endpoint pass
	work        *validationParticleWork
}

func newValidationParticleMatcher(c *validationContext, children []*validationNode) *validationParticleMatcher {
	return &validationParticleMatcher{context: c, children: children,
		particles: make(map[validationParticleKey]validationParticleEndpoints),
		bodies:    make(map[validationParticleKey]validationParticleEndpoints),
		nullable:  make(map[*validationParticleSchema]bool)}
}

func (c *validationContext) matchParticle(particle *validationParticleSchema, children []*validationNode, position int) (validationMatchResult, validationMatchFailure, bool) {
	m := newValidationParticleMatcher(c, children)
	// Like effective complex types, this is immutable schema information.
	// Reuse it across elements of one validation, never across documents.
	if c.particleNullable == nil {
		c.particleNullable = m.nullable
	} else {
		m.nullable = c.particleNullable
	}
	return m.run(particle, position)
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
		// Success does not consume branch failures. Replay the same bounded DP
		// only when diagnostics are needed; this also preserves failures from
		// alternatives that are not part of a selected witness. Work counters
		// include both passes, and every memo entry belongs to this matcher.
		clear(m.particles)
		clear(m.bodies)
		m.diagnostics = true
		endpoints = m.match(p, start)
		return validationMatchResult{}, *endpoints.failure, false
	}
	result := validationMatchResult{next: endpoints.ends[0]}
	if !m.witness(p, start, result.next, &result.elements) {
		return validationMatchResult{}, validationMatchFailure{schema: "content-model witness cannot be reconstructed"}, false
	}
	return result, validationMatchFailure{}, true
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

func (m *validationParticleMatcher) initialFailure(start int) *validationMatchFailure {
	if !m.diagnostics {
		return nil
	}
	return &validationMatchFailure{position: start}
}

func (m *validationParticleMatcher) failure(left, right *validationMatchFailure) *validationMatchFailure {
	if !m.diagnostics {
		return nil
	}
	failure := betterValidationFailure(*left, *right)
	return &failure
}

func (m *validationParticleMatcher) singleton(position int) []int {
	if m.singletons == nil {
		m.singletons = make([]int, len(m.children)+1)
		for index := range m.singletons {
			m.singletons[index] = len(m.children) - index
		}
	}
	index := len(m.children) - position
	return m.singletons[index : index+1 : index+1]
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

// Most score states have zero or one endpoint. Promote only actual
// alternatives to a map. Multiple-endpoint slices are privately owned;
// single endpoints share only the matcher's immutable position array.
type validationParticlePositionSet struct {
	first    int
	hasFirst bool
	many     map[int]bool
}

func (s *validationParticlePositionSet) add(position int) {
	if !s.hasFirst {
		s.first, s.hasFirst = position, true
		return
	}
	if s.many != nil {
		s.many[position] = true
	} else if position != s.first {
		s.many = map[int]bool{s.first: true, position: true}
	}
}

func (s *validationParticlePositionSet) ordered(m *validationParticleMatcher) []int {
	if s.many != nil {
		return validationOrderedPositions(s.many)
	}
	if s.hasFirst {
		return m.singleton(s.first)
	}
	return nil
}

func (m *validationParticleMatcher) limit(p *validationParticleSchema, start int) int {
	limit := len(m.children) - start
	if !p.Occurrence.Unbounded && p.Occurrence.Max < uint64(limit) {
		limit = int(p.Occurrence.Max)
	}
	return limit
}

func (m *validationParticleMatcher) match(p *validationParticleSchema, start int) validationParticleEndpoints {
	// A required single occurrence has exactly its body's endpoints. Reusing
	// the immutable slice avoids a second memo entry and repetition maps.
	if !m.diagnostics && !p.Occurrence.Unbounded && p.Occurrence.Min == 1 && p.Occurrence.Max == 1 {
		return m.body(p, start)
	}
	// An optional terminal has at most the consumed and empty endpoints.
	// Both are adjacent views of the matcher's immutable position array.
	if !m.diagnostics && !p.Occurrence.Unbounded && p.Occurrence.Max <= 1 &&
		(p.Kind == validationParticleElement || p.Kind == validationParticleAny) {
		if p.Occurrence.Max == 0 {
			return validationParticleEndpoints{ends: m.singleton(start)}
		}
		result := m.body(p, start)
		if len(result.ends) == 0 {
			result.ends = m.singleton(start)
		} else {
			index := len(m.children) - start - 1
			result.ends = m.singletons[index : index+2 : index+2]
		}
		return result
	}
	key := validationParticleKey{p, start}
	if cached, ok := m.particles[key]; ok {
		return cached
	}
	if m.work != nil {
		m.work.particles++
	}
	result := validationParticleEndpoints{failure: m.initialFailure(start)}
	var accepted validationParticlePositionSet
	if p.Occurrence.Min == 0 || m.nullable[p] {
		accepted.add(start)
	}
	current := []int{start}
	for count := 1; count <= m.limit(p, start) && len(current) > 0; count++ {
		var next validationParticlePositionSet
		for _, position := range current {
			if m.work != nil {
				m.work.repeatStates++
			}
			body := m.body(p, position)
			result.failure = m.failure(result.failure, body.failure)
			for _, end := range body.ends {
				m.transition()
				if end == position {
					continue
				} // any needed empty occurrences can be padding
				next.add(end)
				if m.nullable[p] || uint64(count) >= p.Occurrence.Min {
					accepted.add(end)
				}
			}
		}
		current = next.ordered(m)
	}
	// Even at EOF, inspect the body for accurate missing-child diagnostics. A
	// maxOccurs=0 particle must not attempt a prohibited occurrence.
	if !accepted.hasFirst && (p.Occurrence.Unbounded || p.Occurrence.Max > 0) {
		result.failure = m.failure(result.failure, m.body(p, start).failure)
		for _, position := range current {
			result.failure = m.failure(result.failure, m.body(p, position).failure)
		}
	}
	result.ends = accepted.ordered(m)
	m.particles[key] = result
	return result
}

func (m *validationParticleMatcher) body(p *validationParticleSchema, start int) validationParticleEndpoints {
	key := validationParticleKey{p, start}
	// Terminal bodies are constant work and need no endpoint memo entries.
	// Preserve ordinary caching in the diagnostic replay.
	terminal := !m.diagnostics && (p.Kind == validationParticleElement || p.Kind == validationParticleAny)
	if !terminal {
		if cached, ok := m.bodies[key]; ok {
			return cached
		}
	}
	if m.work != nil {
		m.work.bodies++
	}
	result := validationParticleEndpoints{failure: m.initialFailure(start)}
	switch p.Kind {
	case validationParticleElement:
		e, _ := m.context.resolveElement(p.Element)
		if start < len(m.children) && validationName(m.children[start].Name) == e.Name {
			result.ends = m.singleton(start + 1)
		} else if m.diagnostics {
			result.failure.expected = []validationQName{e.Name}
		}
	case validationParticleAny:
		// Preserve the legacy wildcard behavior. Namespace and processContents
		// semantics need their separate support contract; this repair does not
		// advertise that they have been verified.
		if start < len(m.children) {
			result.ends = m.singleton(start + 1)
		}
	case validationParticleChoice:
		var ends validationParticlePositionSet
		for _, child := range p.Children {
			branch := m.match(child, start)
			result.failure = m.failure(result.failure, branch.failure)
			for _, end := range branch.ends {
				m.transition()
				ends.add(end)
			}
		}
		result.ends = ends.ordered(m)
	case validationParticleSequence:
		current := []int{start}
		for _, child := range p.Children {
			var ends validationParticlePositionSet
			for _, position := range current {
				branch := m.match(child, position)
				result.failure = m.failure(result.failure, branch.failure)
				for _, end := range branch.ends {
					m.transition()
					ends.add(end)
				}
			}
			current = ends.ordered(m)
			if len(current) == 0 {
				break
			}
		}
		result.ends = current
	case validationParticleAll:
		end, failure, ok := m.all(p, start, nil)
		if m.diagnostics {
			result.failure = &failure
		}
		if ok {
			result.ends = m.singleton(end)
		}
	}
	if !terminal {
		m.bodies[key] = result
	}
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
	missing := false
	for index, child := range p.Children {
		if !used[index] && child.Occurrence.Min > 0 {
			missing = true
			if m.diagnostics {
				e, _ := m.context.resolveElement(child.Element)
				failure.expected = append(failure.expected, e.Name)
			}
		}
	}
	return end, failure, !missing
}

// A predecessor layer normally contains one endpoint. Preserve the first
// predecessor on collisions, and promote to a map only for alternatives.
type validationParticlePredecessors struct {
	end, start int
	hasFirst   bool
	many       map[int]int
}

func (p *validationParticlePredecessors) add(end, start int) bool {
	if !p.hasFirst {
		p.end, p.start, p.hasFirst = end, start, true
		return true
	}
	if p.many == nil {
		if p.end == end {
			return false
		}
		p.many = map[int]int{p.end: p.start, end: start}
		return true
	}
	if _, found := p.many[end]; found {
		return false
	}
	p.many[end] = start
	return true
}

func (p *validationParticlePredecessors) previous(end int) (int, bool) {
	if p.many != nil {
		start, found := p.many[end]
		return start, found
	}
	return p.start, p.hasFirst && end == p.end
}

func (p *validationParticlePredecessors) ordered(m *validationParticleMatcher) []int {
	if p.many != nil {
		positions := make([]int, 0, len(p.many))
		for end := range p.many {
			positions = append(positions, end)
		}
		slices.Sort(positions)
		slices.Reverse(positions)
		return positions
	}
	if p.hasFirst {
		return m.singleton(p.end)
	}
	return nil
}

// predecessor layers retain only integer positions. They are local to witness
// reconstruction and released before descending into the selected child paths.
// Zero-progress repetition edges are excluded, exactly as in match.
func (m *validationParticleMatcher) witness(p *validationParticleSchema, start, end int, elements *[]validationElementMatch) bool {
	if start == end {
		return p.Occurrence.Min == 0 || m.nullable[p]
	}
	if !p.Occurrence.Unbounded && p.Occurrence.Max == 1 {
		return m.bodyWitness(p, start, end, elements)
	}
	layers := []validationParticlePredecessors{{}}
	current := []int{start}
	for count := 1; count <= m.limit(p, start) && len(current) > 0; count++ {
		var previous validationParticlePredecessors
		for _, position := range current {
			for _, next := range m.body(p, position).ends {
				m.transition()
				if next <= position || next > end {
					continue
				}
				if previous.add(next, position) {
					if m.work != nil {
						m.work.witnessStates++
					}
				}
			}
		}
		layers = append(layers, previous)
		if _, ok := previous.previous(end); ok && (m.nullable[p] || uint64(count) >= p.Occurrence.Min) {
			positions := make([]int, count+1)
			positions[count] = end
			for index := count; index > 0; index-- {
				positions[index-1], _ = layers[index].previous(positions[index])
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
		current = previous.ordered(m)
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
		layers := make([]validationParticlePredecessors, 0, len(p.Children))
		current := []int{start}
		for _, child := range p.Children {
			var previous validationParticlePredecessors
			for _, position := range current {
				for _, next := range m.match(child, position).ends {
					m.transition()
					if next > end {
						continue
					}
					if previous.add(next, position) {
						if m.work != nil {
							m.work.witnessStates++
						}
					}
				}
			}
			layers = append(layers, previous)
			current = previous.ordered(m)
		}
		if !slices.Contains(current, end) {
			return false
		}
		positions := make([]int, len(p.Children)+1)
		positions[len(p.Children)] = end
		for index := len(p.Children) - 1; index >= 0; index-- {
			positions[index], _ = layers[index].previous(positions[index+1])
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
