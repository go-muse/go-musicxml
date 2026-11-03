package xsd

import (
	"encoding/binary"
	"unsafe"
)

// Prefix binding assigns a declaration to each child of an element whose
// complete child list does not match its content model, or is not yet known
// to be complete. A child gets the particle that the children before it and
// the child itself determine, whatever follows: a push matcher consumes the
// children one by one and stops at the first child that no continuation of
// the content model accepts. XSD 1.0 Unique Particle Attribution makes the
// particle of every accepted child unique; a schema that violates it can
// leave a child with two candidate particles, and binding stops there too.
// Children from the stopping point on stay without a declaration.

// prefixFrame is one open particle on the path from the content model's root
// particle to the leaf that consumed the latest child.
type prefixFrame struct {
	particle *ParticleSchema
	// count is the number of completed occurrences of the particle.
	count uint64
	// index is the sequence child in progress, the chosen choice branch or
	// the all-group child in progress.
	index int
	// used marks the all-group children of the current occurrence that
	// consumed a child. It is copied before every change.
	used []bool
	// consumed records whether the current occurrence consumed a child.
	consumed bool
}

// prefixStateLimit bounds the configurations kept for one child. A schema that
// satisfies Unique Particle Attribution keeps one; more mean the binding
// cannot be proved, and binding stops.
const prefixStateLimit = 64

// prefixSearch finds every configuration in which some leaf particle consumes
// the next child, starting from configurations in which the previous child
// was consumed.
type prefixSearch struct {
	c        *Context
	nullable map[*ParticleSchema]bool
	child    *Node
	out      [][]prefixFrame
	keys     map[string]bool
	visited  map[string]bool
	overflow bool
}

// prefixFrameBytes is the charge of one copied frame.
var prefixFrameBytes = int64(unsafe.Sizeof(prefixFrame{}))

func (s *prefixSearch) clone(stack []prefixFrame) []prefixFrame {
	s.c.step(int64(len(stack)) + 1)
	s.c.scratch(s.c.size(int64(len(stack))+1, prefixFrameBytes))
	out := make([]prefixFrame, len(stack), len(stack)+1)
	copy(out, stack)
	return out
}

// key encodes a configuration for deduplication. Particles are compared by
// identity, as the witnesses that consume them are.
func (s *prefixSearch) key(kind byte, stack []prefixFrame) string {
	size := 1
	for _, frame := range stack {
		size += 8 + 8 + 8 + 1 + len(frame.used)
	}
	s.c.step(int64(size))
	s.c.scratch(int64(size))
	out := make([]byte, 0, size)
	out = append(out, kind)
	for _, frame := range stack {
		out = binary.LittleEndian.AppendUint64(out, uint64(uintptr(unsafe.Pointer(frame.particle))))
		out = binary.LittleEndian.AppendUint64(out, frame.count)
		out = binary.LittleEndian.AppendUint64(out, uint64(frame.index))
		consumed := byte(0)
		if frame.consumed {
			consumed = 1
		}
		out = append(out, consumed)
		for _, used := range frame.used {
			if used {
				out = append(out, 1)
			} else {
				out = append(out, 0)
			}
		}
	}
	return string(out)
}

// seen reports whether the search already explored this entry point with
// this configuration, and records it otherwise.
func (s *prefixSearch) seen(kind byte, stack []prefixFrame) bool {
	key := s.key(kind, stack)
	if s.visited[key] {
		return true
	}
	s.c.scratch(64 + int64(len(key)))
	s.visited[key] = true
	return false
}

// accepts reports whether a leaf particle can consume the child.
func (s *prefixSearch) accepts(p *ParticleSchema) bool {
	s.c.step(1)
	switch p.Kind {
	case ParticleElement:
		e, ok := s.c.ResolveElement(p.Element)
		return ok && e != nil && Name(s.child.Name) == e.Name
	case ParticleAny:
		return s.c.local == nil || wildcardNamespace(p.Any.Namespace, s.child.Name.Space, s.c).allowed
	}
	return false
}

// emit records a configuration whose top frame, a leaf, consumes the child.
func (s *prefixSearch) emit(stack []prefixFrame) {
	if s.overflow {
		return
	}
	out := s.clone(stack)
	for i := range out {
		out[i].consumed = true
	}
	key := s.key('e', out)
	if s.keys[key] {
		return
	}
	if len(s.out) == prefixStateLimit {
		s.overflow = true
		return
	}
	s.c.scratch(64 + int64(len(key)))
	s.keys[key] = true
	s.out = appendScratch(s.c, s.out, out)
}

// enter opens particle q under the configuration stack.
func (s *prefixSearch) enter(stack []prefixFrame, q *ParticleSchema) {
	next := s.clone(stack)
	s.start(append(next, prefixFrame{particle: q}))
}

// start begins another occurrence of the top frame's particle, when its
// maximum allows one, and alternatively leaves the particle when its
// occurrences so far, padded with empty ones, satisfy its minimum.
func (s *prefixSearch) start(stack []prefixFrame) {
	if s.overflow || s.seen('s', stack) {
		return
	}
	top := stack[len(stack)-1]
	q := top.particle
	if q.Occurrence.Unbounded || top.count < q.Occurrence.Max {
		fresh := s.clone(stack)
		frame := &fresh[len(fresh)-1]
		frame.consumed, frame.used, frame.index = false, nil, 0
		switch q.Kind {
		case ParticleElement, ParticleAny:
			if s.accepts(q) {
				s.emit(fresh)
			}
		case ParticleSequence:
			if len(q.Children) != 0 {
				s.enter(fresh, q.Children[0])
			}
		case ParticleChoice:
			for i, branch := range q.Children {
				s.c.step(1)
				fresh[len(fresh)-1].index = i
				s.enter(fresh, branch)
			}
		case ParticleAll:
			s.c.scratch(s.c.size(int64(len(q.Children)), 1))
			fresh[len(fresh)-1].used = make([]bool, len(q.Children))
			s.allMembers(fresh)
		}
	}
	if top.count >= q.Occurrence.Min || s.nullable[q] {
		s.leave(stack[:len(stack)-1])
	}
}

// allMembers lets every unused member of the top frame's all group consume
// the child. Members are single elements: a member that consumes nothing is
// not chosen, so it stays available for a later child.
func (s *prefixSearch) allMembers(stack []prefixFrame) {
	q := stack[len(stack)-1].particle
	for i, member := range q.Children {
		s.c.step(1)
		if stack[len(stack)-1].used[i] || member.Occurrence.Max == 0 || !s.accepts(member) {
			continue
		}
		next := s.clone(stack)
		next[len(next)-1].index = i
		s.emit(append(next, prefixFrame{particle: member}))
	}
}

// finish completes the current occurrence of the top frame, which consumed
// at least one child.
func (s *prefixSearch) finish(stack []prefixFrame) {
	next := s.clone(stack)
	next[len(next)-1].count++
	s.start(next)
}

// leave continues in the parent after its current child particle left.
func (s *prefixSearch) leave(stack []prefixFrame) {
	if len(stack) == 0 || s.overflow || s.seen('l', stack) {
		return
	}
	top := stack[len(stack)-1]
	q := top.particle
	switch q.Kind {
	case ParticleSequence:
		if top.index+1 < len(q.Children) {
			next := s.clone(stack)
			next[len(next)-1].index++
			s.enter(next, q.Children[top.index+1])
			return
		}
	case ParticleAll:
		next := s.clone(stack)
		frame := &next[len(next)-1]
		used := make([]bool, len(frame.used))
		s.c.scratch(s.c.size(int64(len(used)), 1))
		copy(used, frame.used)
		used[frame.index] = true
		frame.used = used
		s.allMembers(next)
		for i, member := range q.Children {
			s.c.step(1)
			if !used[i] && member.Occurrence.Min > 0 && member.Occurrence.Max > 0 {
				return
			}
		}
		stack = next
	}
	// The body is complete. An occurrence that consumed nothing adds no
	// path: start already offered to leave the particle instead.
	if stack[len(stack)-1].consumed {
		s.finish(stack)
	}
}

// prefixMatcher consumes the children of one element in order and binds
// each to the leaf particle its prefix determines, until the first child it
// cannot bind.
type prefixMatcher struct {
	c        *Context
	particle *ParticleSchema
	nullable map[*ParticleSchema]bool
	states   [][]prefixFrame
	next     int
	stopped  bool
	// ambiguous records that binding stopped at a child with two candidate
	// particles rather than at a child the content model rejects.
	ambiguous bool
}

// newPrefixMatcher prepares a push matcher for a content model, or reports
// that its metadata cannot be matched.
func (c *Context) newPrefixMatcher(particle *ParticleSchema) (*prefixMatcher, bool) {
	c.step(1)
	if particle == nil {
		return nil, false
	}
	matcher := NewParticleMatcher(c, nil)
	if c.ParticleNullable != nil {
		matcher.Nullable = c.ParticleNullable
	}
	c.scratch(64)
	if !matcher.Prepare(particle, make(map[*ParticleSchema]bool)) {
		return nil, false
	}
	return &prefixMatcher{c: c, particle: particle, nullable: matcher.Nullable}, true
}

// push binds the next child. Once a child cannot be bound, it and every later
// child report false.
func (m *prefixMatcher) push(child *Node) (ElementMatch, bool) {
	c := m.c
	c.step(1)
	if m.stopped || child == nil {
		m.stopped = true
		return ElementMatch{}, false
	}
	c.scratch(256)
	s := prefixSearch{c: c, nullable: m.nullable, child: child, keys: make(map[string]bool), visited: make(map[string]bool)}
	if m.next == 0 {
		s.enter(nil, m.particle)
	}
	for _, state := range m.states {
		s.finish(state)
	}
	if s.overflow || len(s.out) == 0 {
		m.stopped, m.ambiguous = true, s.overflow
		return ElementMatch{}, false
	}
	leaf := s.out[0][len(s.out[0])-1].particle
	for _, state := range s.out[1:] {
		c.step(1)
		if state[len(state)-1].particle != leaf {
			m.stopped, m.ambiguous = true, true
			return ElementMatch{}, false
		}
	}
	match := ElementMatch{Index: m.next, Particle: leaf}
	if leaf.Kind == ParticleElement {
		match.Schema, _ = c.ResolveElement(leaf.Element)
	} else if c.local != nil {
		c.noteWildcardAdmission(wildcardNamespace(leaf.Any.Namespace, child.Name.Space, c), leaf.Any.ProcessContents)
	}
	m.states = s.out
	m.next++
	return match, true
}

// prefixMatches returns the leaf matches of the children that a prefix of the
// child list binds, in child order.
func (c *Context) prefixMatches(particle *ParticleSchema, children []*Node) []ElementMatch {
	matcher, ok := c.newPrefixMatcher(particle)
	if !ok {
		return nil
	}
	var result []ElementMatch
	for _, child := range children {
		match, bound := matcher.push(child)
		if !bound {
			break
		}
		result = appendRetained(c, result, match)
	}
	return result
}
