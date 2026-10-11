package xsd

import "slices"

// Content matching retains endpoints, not parse histories. Occurrence counts
// belong to intermediate repetition states; only a fully constrained particle
// can be memoized by (particle, start). Empty occurrences are padded symbolically.
// Witnesses are reconstructed only for the selected endpoint using temporary
// predecessor tables, never copied into every cached endpoint.
type ParticleKey struct {
	Particle *ParticleSchema
	Start    int
}

type ParticleEndpoints struct {
	Ends    []int         // descending, unique; deterministic longest-first traversal
	Failure *MatchFailure // nil until a failed run requests diagnostics
}

// These optional counters support algorithmic regression tests. They are not a
// public budget, cancellation mechanism, or an incomplete-assessment contract.
type ParticleWork struct {
	Particles, Bodies, Transitions, RepeatStates, WitnessStates int64
}

type ParticleMatcher struct {
	Context     *Context
	Children    []*Node
	Particles   map[ParticleKey]ParticleEndpoints
	Bodies      map[ParticleKey]ParticleEndpoints
	Nullable    map[*ParticleSchema]bool // body, before its occurrence range
	Singletons  []int                    // immutable singleton endpoint views, owned by this match
	Diagnostics bool                     // collect failure names only after an unsuccessful endpoint pass
	Work        *ParticleWork
}

func NewParticleMatcher(c *Context, children []*Node) *ParticleMatcher {
	return &ParticleMatcher{Context: c, Children: children,
		Particles: make(map[ParticleKey]ParticleEndpoints),
		Bodies:    make(map[ParticleKey]ParticleEndpoints),
		Nullable:  make(map[*ParticleSchema]bool)}
}

func (c *Context) MatchParticle(particle *ParticleSchema, children []*Node, position int) (MatchResult, MatchFailure, bool) {
	m := NewParticleMatcher(c, children)
	// Like effective complex types, this is immutable schema information.
	// Reuse it across elements of one validation, never across documents.
	if c.ParticleNullable == nil {
		c.ParticleNullable = m.Nullable
	} else {
		m.Nullable = c.ParticleNullable
	}
	return m.Run(particle, position)
}

func (m *ParticleMatcher) Run(p *ParticleSchema, start int) (MatchResult, MatchFailure, bool) {
	if start < 0 || start > len(m.Children) {
		return MatchResult{}, MatchFailure{Schema: "invalid content-model position"}, false
	}
	if p == nil {
		return MatchResult{Next: start}, MatchFailure{}, true
	}
	for _, child := range m.Children {
		if child == nil {
			return MatchResult{}, MatchFailure{Schema: "missing content-model child"}, false
		}
	}
	if !m.Prepare(p, make(map[*ParticleSchema]bool)) {
		return MatchResult{}, MatchFailure{Schema: "invalid or cyclic content-model metadata"}, false
	}
	endpoints := m.Match(p, start)
	if len(endpoints.Ends) == 0 {
		// Success does not consume branch failures. Replay the same bounded DP
		// only when diagnostics are needed; this also preserves failures from
		// alternatives that are not part of a selected witness. Work counters
		// include both passes, and every memo entry belongs to this matcher.
		clear(m.Particles)
		clear(m.Bodies)
		m.Diagnostics = true
		endpoints = m.Match(p, start)
		return MatchResult{}, *endpoints.Failure, false
	}
	result := MatchResult{Next: endpoints.Ends[0]}
	if !m.Witness(p, start, result.Next, &result.Elements) {
		return MatchResult{}, MatchFailure{Schema: "content-model witness cannot be reconstructed"}, false
	}
	return result, MatchFailure{}, true
}

// This structural guard is not a complete XSD schema-validity or UPA checker.
// In particular wildcard namespace/processContents support remains unchanged.
func (m *ParticleMatcher) Prepare(p *ParticleSchema, active map[*ParticleSchema]bool) bool {
	if p == nil || active[p] {
		return false
	}
	if _, ok := m.Nullable[p]; ok {
		return true
	}
	if !p.Occurrence.Unbounded && p.Occurrence.Min > p.Occurrence.Max {
		return false
	}
	active[p] = true
	defer delete(active, p)
	nullable := false
	switch p.Kind {
	case ParticleElement:
		if p.Element == nil || p.Any != nil || len(p.Children) != 0 {
			return false
		}
		e, ok := m.Context.ResolveElement(p.Element)
		if !ok || e == nil || e.Name.Local == "" {
			return false
		}
	case ParticleAny:
		if p.Any == nil || p.Element != nil || len(p.Children) != 0 {
			return false
		}
	case ParticleSequence, ParticleChoice, ParticleAll:
		if p.Element != nil || p.Any != nil {
			return false
		}
		nullable = p.Kind != ParticleChoice
		names := make(map[QName]bool)
		for _, child := range p.Children {
			if !m.Prepare(child, active) {
				return false
			}
			if p.Kind == ParticleAll {
				if child.Kind != ParticleElement || child.Occurrence.Unbounded || child.Occurrence.Max > 1 {
					return false
				}
				// A maxOccurs=0 leaf is absent from the effective all group.
				// Still validate its structure above, but do not let its name
				// conflict with a distinct, active particle.
				if child.Occurrence.Max != 0 {
					e, _ := m.Context.ResolveElement(child.Element)
					if names[e.Name] {
						return false
					}
					names[e.Name] = true
				}
			}
			empty := child.Occurrence.Min == 0 || m.Nullable[child]
			if p.Kind == ParticleChoice {
				nullable = nullable || empty
			} else {
				nullable = nullable && empty
			}
		}
		if p.Kind == ParticleAll && (p.Occurrence.Unbounded || p.Occurrence.Max > 1) {
			return false
		}
	default:
		return false
	}
	m.Nullable[p] = nullable
	return true
}

func (m *ParticleMatcher) Transition() {
	if m.Work != nil {
		m.Work.Transitions++
	}
}

func (m *ParticleMatcher) InitialFailure(start int) *MatchFailure {
	if !m.Diagnostics {
		return nil
	}
	return &MatchFailure{Position: start}
}

func (m *ParticleMatcher) Failure(left, right *MatchFailure) *MatchFailure {
	if !m.Diagnostics {
		return nil
	}
	failure := BetterFailure(*left, *right)
	return &failure
}

func (m *ParticleMatcher) Singleton(position int) []int {
	if m.Singletons == nil {
		m.Singletons = make([]int, len(m.Children)+1)
		for index := range m.Singletons {
			m.Singletons[index] = len(m.Children) - index
		}
	}
	index := len(m.Children) - position
	return m.Singletons[index : index+1 : index+1]
}

func OrderedPositions(seen map[int]bool) []int {
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
type ParticlePositionSet struct {
	First    int
	HasFirst bool
	Many     map[int]bool
}

func (s *ParticlePositionSet) Add(position int) {
	if !s.HasFirst {
		s.First, s.HasFirst = position, true
		return
	}
	if s.Many != nil {
		s.Many[position] = true
	} else if position != s.First {
		s.Many = map[int]bool{s.First: true, position: true}
	}
}

func (s *ParticlePositionSet) Ordered(m *ParticleMatcher) []int {
	if s.Many != nil {
		return OrderedPositions(s.Many)
	}
	if s.HasFirst {
		return m.Singleton(s.First)
	}
	return nil
}

func (m *ParticleMatcher) Limit(p *ParticleSchema, start int) int {
	limit := len(m.Children) - start
	if !p.Occurrence.Unbounded && p.Occurrence.Max < uint64(limit) {
		limit = int(p.Occurrence.Max)
	}
	return limit
}

func (m *ParticleMatcher) Match(p *ParticleSchema, start int) ParticleEndpoints {
	// A required single occurrence has exactly its body's endpoints. Reusing
	// the immutable slice avoids a second memo entry and repetition maps.
	if !m.Diagnostics && !p.Occurrence.Unbounded && p.Occurrence.Min == 1 && p.Occurrence.Max == 1 {
		return m.Body(p, start)
	}
	// An optional terminal has at most the consumed and empty endpoints.
	// Both are adjacent views of the matcher's immutable position array.
	if !m.Diagnostics && !p.Occurrence.Unbounded && p.Occurrence.Max <= 1 &&
		(p.Kind == ParticleElement || p.Kind == ParticleAny) {
		if p.Occurrence.Max == 0 {
			return ParticleEndpoints{Ends: m.Singleton(start)}
		}
		result := m.Body(p, start)
		if len(result.Ends) == 0 {
			result.Ends = m.Singleton(start)
		} else {
			index := len(m.Children) - start - 1
			result.Ends = m.Singletons[index : index+2 : index+2]
		}
		return result
	}
	key := ParticleKey{p, start}
	if cached, ok := m.Particles[key]; ok {
		return cached
	}
	if m.Work != nil {
		m.Work.Particles++
	}
	result := ParticleEndpoints{Failure: m.InitialFailure(start)}
	var accepted ParticlePositionSet
	if p.Occurrence.Min == 0 || m.Nullable[p] {
		accepted.Add(start)
	}
	current := []int{start}
	for count := 1; count <= m.Limit(p, start) && len(current) > 0; count++ {
		var next ParticlePositionSet
		for _, position := range current {
			if m.Work != nil {
				m.Work.RepeatStates++
			}
			body := m.Body(p, position)
			result.Failure = m.Failure(result.Failure, body.Failure)
			for _, end := range body.Ends {
				m.Transition()
				if end == position {
					continue
				} // any needed empty occurrences can be padding
				next.Add(end)
				if m.Nullable[p] || uint64(count) >= p.Occurrence.Min {
					accepted.Add(end)
				}
			}
		}
		current = next.Ordered(m)
	}
	// Even at EOF, inspect the body for accurate missing-child diagnostics. A
	// maxOccurs=0 particle must not attempt a prohibited occurrence.
	if !accepted.HasFirst && (p.Occurrence.Unbounded || p.Occurrence.Max > 0) {
		result.Failure = m.Failure(result.Failure, m.Body(p, start).Failure)
		for _, position := range current {
			result.Failure = m.Failure(result.Failure, m.Body(p, position).Failure)
		}
	}
	result.Ends = accepted.Ordered(m)
	m.Particles[key] = result
	return result
}

func (m *ParticleMatcher) Body(p *ParticleSchema, start int) ParticleEndpoints {
	key := ParticleKey{p, start}
	// Terminal bodies are constant work and need no endpoint memo entries.
	// Preserve ordinary caching in the diagnostic replay.
	terminal := !m.Diagnostics && (p.Kind == ParticleElement || p.Kind == ParticleAny)
	if !terminal {
		if cached, ok := m.Bodies[key]; ok {
			return cached
		}
	}
	if m.Work != nil {
		m.Work.Bodies++
	}
	result := ParticleEndpoints{Failure: m.InitialFailure(start)}
	switch p.Kind {
	case ParticleElement:
		e, _ := m.Context.ResolveElement(p.Element)
		if start < len(m.Children) && Name(m.Children[start].Name) == e.Name {
			result.Ends = m.Singleton(start + 1)
		} else if m.Diagnostics {
			result.Failure.Expected = []QName{e.Name}
		}
	case ParticleAny:
		// Preserve the legacy wildcard behavior. Namespace and processContents
		// semantics need their separate support contract; this repair does not
		// advertise that they have been verified.
		if start < len(m.Children) {
			result.Ends = m.Singleton(start + 1)
		}
	case ParticleChoice:
		var ends ParticlePositionSet
		for _, child := range p.Children {
			branch := m.Match(child, start)
			result.Failure = m.Failure(result.Failure, branch.Failure)
			for _, end := range branch.Ends {
				m.Transition()
				ends.Add(end)
			}
		}
		result.Ends = ends.Ordered(m)
	case ParticleSequence:
		current := []int{start}
		for _, child := range p.Children {
			var ends ParticlePositionSet
			for _, position := range current {
				branch := m.Match(child, position)
				result.Failure = m.Failure(result.Failure, branch.Failure)
				for _, end := range branch.Ends {
					m.Transition()
					ends.Add(end)
				}
			}
			current = ends.Ordered(m)
			if len(current) == 0 {
				break
			}
		}
		result.Ends = current
	case ParticleAll:
		end, failure, ok := m.All(p, start, nil)
		if m.Diagnostics {
			result.Failure = &failure
		}
		if ok {
			result.Ends = m.Singleton(end)
		}
	}
	if !terminal {
		m.Bodies[key] = result
	}
	return result
}

// XSD 1.0 all children are distinct, singly occurring element particles. There
// is no combinatorial subset search; prepare rejects malformed all metadata.
func (m *ParticleMatcher) All(p *ParticleSchema, start int, elements *[]ElementMatch) (int, MatchFailure, bool) {
	used := make([]bool, len(p.Children))
	end := start
	for end < len(m.Children) {
		found := false
		for index, child := range p.Children {
			m.Transition()
			if used[index] || child.Occurrence.Max == 0 {
				continue
			}
			e, _ := m.Context.ResolveElement(child.Element)
			if Name(m.Children[end].Name) != e.Name {
				continue
			}
			used[index] = true
			found = true
			if elements != nil {
				*elements = append(*elements, ElementMatch{Index: end, Schema: e})
			}
			end++
			break
		}
		if !found {
			break
		}
	}
	failure := MatchFailure{Position: end}
	missing := false
	for index, child := range p.Children {
		if !used[index] && child.Occurrence.Min > 0 {
			missing = true
			if m.Diagnostics {
				e, _ := m.Context.ResolveElement(child.Element)
				failure.Expected = append(failure.Expected, e.Name)
			}
		}
	}
	return end, failure, !missing
}

// A predecessor layer normally contains one endpoint. Preserve the first
// predecessor on collisions, and promote to a map only for alternatives.
type ParticlePredecessors struct {
	End, Start int
	HasFirst   bool
	Many       map[int]int
}

func (p *ParticlePredecessors) Add(end, start int) bool {
	if !p.HasFirst {
		p.End, p.Start, p.HasFirst = end, start, true
		return true
	}
	if p.Many == nil {
		if p.End == end {
			return false
		}
		p.Many = map[int]int{p.End: p.Start, end: start}
		return true
	}
	if _, found := p.Many[end]; found {
		return false
	}
	p.Many[end] = start
	return true
}

func (p *ParticlePredecessors) Previous(end int) (int, bool) {
	if p.Many != nil {
		start, found := p.Many[end]
		return start, found
	}
	return p.Start, p.HasFirst && end == p.End
}

func (p *ParticlePredecessors) Ordered(m *ParticleMatcher) []int {
	if p.Many != nil {
		positions := make([]int, 0, len(p.Many))
		for end := range p.Many {
			positions = append(positions, end)
		}
		slices.Sort(positions)
		slices.Reverse(positions)
		return positions
	}
	if p.HasFirst {
		return m.Singleton(p.End)
	}
	return nil
}

// predecessor layers retain only integer positions. They are local to witness
// reconstruction and released before descending into the selected child paths.
// Zero-progress repetition edges are excluded, exactly as in match.
func (m *ParticleMatcher) Witness(p *ParticleSchema, start, end int, elements *[]ElementMatch) bool {
	if start == end {
		return p.Occurrence.Min == 0 || m.Nullable[p]
	}
	if !p.Occurrence.Unbounded && p.Occurrence.Max == 1 {
		return m.BodyWitness(p, start, end, elements)
	}
	layers := []ParticlePredecessors{{}}
	current := []int{start}
	for count := 1; count <= m.Limit(p, start) && len(current) > 0; count++ {
		var previous ParticlePredecessors
		for _, position := range current {
			for _, next := range m.Body(p, position).Ends {
				m.Transition()
				if next <= position || next > end {
					continue
				}
				if previous.Add(next, position) {
					if m.Work != nil {
						m.Work.WitnessStates++
					}
				}
			}
		}
		layers = append(layers, previous)
		if _, ok := previous.Previous(end); ok && (m.Nullable[p] || uint64(count) >= p.Occurrence.Min) {
			positions := make([]int, count+1)
			positions[count] = end
			for index := count; index > 0; index-- {
				positions[index-1], _ = layers[index].Previous(positions[index])
			}
			// No cached endpoint owns these tables. Only the selected split survives.
			layers = nil
			for index := 0; index < count; index++ {
				if !m.BodyWitness(p, positions[index], positions[index+1], elements) {
					return false
				}
			}
			return true
		}
		current = previous.Ordered(m)
	}
	return false
}

func (m *ParticleMatcher) BodyWitness(p *ParticleSchema, start, end int, elements *[]ElementMatch) bool {
	switch p.Kind {
	case ParticleElement:
		e, _ := m.Context.ResolveElement(p.Element)
		*elements = append(*elements, ElementMatch{Index: start, Schema: e})
		return end == start+1
	case ParticleAny:
		return end == start+1
	case ParticleChoice:
		for _, child := range p.Children {
			if slices.Contains(m.Match(child, start).Ends, end) {
				return m.Witness(child, start, end, elements)
			}
		}
	case ParticleSequence:
		layers := make([]ParticlePredecessors, 0, len(p.Children))
		current := []int{start}
		for _, child := range p.Children {
			var previous ParticlePredecessors
			for _, position := range current {
				for _, next := range m.Match(child, position).Ends {
					m.Transition()
					if next > end {
						continue
					}
					if previous.Add(next, position) {
						if m.Work != nil {
							m.Work.WitnessStates++
						}
					}
				}
			}
			layers = append(layers, previous)
			current = previous.Ordered(m)
		}
		if !slices.Contains(current, end) {
			return false
		}
		positions := make([]int, len(p.Children)+1)
		positions[len(p.Children)] = end
		for index := len(p.Children) - 1; index >= 0; index-- {
			positions[index], _ = layers[index].Previous(positions[index+1])
		}
		layers = nil
		for index, child := range p.Children {
			if !m.Witness(child, positions[index], positions[index+1], elements) {
				return false
			}
		}
		return true
	case ParticleAll:
		next, _, ok := m.All(p, start, elements)
		return ok && next == end
	}
	return false
}
