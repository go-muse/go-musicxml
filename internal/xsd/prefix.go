package xsd

import (
	"encoding/binary"
	"errors"
	"unsafe"
)

// Prefix binding assigns a declaration to each child of an element whose
// complete child list does not match its content model, or is not yet known
// to be complete. A child gets the particle that the children before it and
// the child itself determine, whatever follows: a push matcher consumes the
// children one by one and stops at the first child that no continuation of
// the content model accepts. XSD 1.0 Unique Particle Attribution makes the
// particle of every accepted child unique. Binding also stops, conservatively,
// at a child with two candidate particles (a schema that violates the rule,
// or two references to one global element), at a child that leaves more than
// prefixStateLimit configurations (nested bounded repetitions such as
// (a{1,10}){1,10} count occurrences separately), and at a child whose search
// exceeds prefixVisitLimit configurations.
// Children from the stopping point on stay without a declaration.

// prefixFrame is one open particle on the path from the content model's root
// particle to the leaf that consumed the latest child.
type prefixFrame struct {
	particle *ParticleSchema
	// count is the number of completed occurrences of the particle. For an
	// unbounded particle it saturates at the minimum: higher counts allow
	// the same continuations.
	count uint64
	// index is the sequence child in progress, the chosen choice branch or
	// the all-group member in progress.
	index int
	// used marks the all-group members of the current occurrence that
	// consumed a child. It is never modified after the frame is created.
	used []bool
	// consumed records whether the current occurrence consumed a child.
	consumed bool
}

// prefixNode is an immutable frame with the frames below it: configurations
// share their common frames, so changing the top frame copies one node.
type prefixNode struct {
	frame  prefixFrame
	parent *prefixNode
	depth  int
}

var (
	prefixNodeBytes = (int64(unsafe.Sizeof(prefixNode{})) + 15) &^ 15
	// prefixEntryBytes covers one key string's header and its map slot.
	prefixEntryBytes int64 = 64
)

const (
	// prefixStateLimit bounds the configurations kept for one child. A
	// schema that satisfies Unique Particle Attribution keeps few; more
	// mean the binding cannot be proved.
	prefixStateLimit = 64
	// prefixVisitLimit bounds the configurations one child's search visits,
	// so that content models sharing particles cannot make it exponential.
	prefixVisitLimit = 4096
)

// prefixSearch finds every configuration in which some leaf particle consumes
// the next child, starting from configurations in which the previous child
// was consumed.
type prefixSearch struct {
	c        *Context
	nullable map[*ParticleSchema]bool
	child    *Node
	out      []*prefixNode
	keys     map[string]bool
	visited  map[string]bool
	buffer   []byte
	overflow bool
}

func (s *prefixSearch) node(frame prefixFrame, parent *prefixNode) *prefixNode {
	s.c.step(1)
	s.c.scratch(prefixNodeBytes)
	depth := 1
	if parent != nil {
		depth = parent.depth + 1
	}
	return &prefixNode{frame: frame, parent: parent, depth: depth}
}

// key encodes a configuration for deduplication in the search's reused
// buffer. Particles are compared by identity, as the witnesses that consume
// them are.
func (s *prefixSearch) key(kind byte, top *prefixNode) []byte {
	out := append(s.buffer[:0], kind)
	for n := top; n != nil; n = n.parent {
		s.c.step(1)
		f := &n.frame
		if len(out)+25+len(f.used) > cap(out) {
			grown := s.c.sum(int64(cap(out)), int64(cap(out)), 25, int64(len(f.used)))
			s.c.scratch(grown)
			next := make([]byte, len(out), grown)
			copy(next, out)
			out = next
		}
		out = binary.LittleEndian.AppendUint64(out, uint64(uintptr(unsafe.Pointer(f.particle))))
		out = binary.LittleEndian.AppendUint64(out, f.count)
		out = binary.LittleEndian.AppendUint64(out, uint64(f.index))
		consumed := byte(0)
		if f.consumed {
			consumed = 1
		}
		out = append(out, consumed)
		for _, used := range f.used {
			if used {
				out = append(out, 1)
			} else {
				out = append(out, 0)
			}
		}
	}
	s.buffer = out
	return out
}

// mark records a key in set and reports whether it was new.
func (s *prefixSearch) mark(set map[string]bool, key []byte) bool {
	if set[string(key)] {
		return false
	}
	s.c.scratch(prefixEntryBytes + int64(len(key)))
	set[string(key)] = true
	return true
}

// seen reports whether the search already explored this entry point with
// this configuration, and records it otherwise.
func (s *prefixSearch) seen(kind byte, top *prefixNode) bool {
	if s.overflow || !s.mark(s.visited, s.key(kind, top)) {
		return true
	}
	if len(s.visited) > prefixVisitLimit {
		s.overflow = true
		return true
	}
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
// Every frame's current occurrence has then consumed a child, so the kept
// configuration is a fresh copy of the whole path.
func (s *prefixSearch) emit(top *prefixNode) {
	if s.overflow {
		return
	}
	s.c.scratch(s.c.size(int64(top.depth), int64(unsafe.Sizeof(prefixFrame{}))))
	frames := make([]prefixFrame, top.depth)
	for n, i := top, top.depth-1; n != nil; n, i = n.parent, i-1 {
		s.c.step(1)
		frames[i] = n.frame
		frames[i].consumed = true
	}
	var out *prefixNode
	for _, frame := range frames {
		out = s.node(frame, out)
	}
	if !s.mark(s.keys, s.key('e', out)) {
		return
	}
	if len(s.out) == prefixStateLimit {
		s.overflow = true
		return
	}
	s.out = appendScratch(s.c, s.out, out)
}

// enter opens particle q under the configuration top.
func (s *prefixSearch) enter(top *prefixNode, q *ParticleSchema) {
	s.start(s.node(prefixFrame{particle: q}, top))
}

// replace returns top with its frame changed.
func (s *prefixSearch) replace(top *prefixNode, frame prefixFrame) *prefixNode {
	return s.node(frame, top.parent)
}

// start begins another occurrence of the top frame's particle, when its
// maximum allows one, and alternatively leaves the particle when its
// occurrences so far, padded with empty ones, satisfy its minimum.
func (s *prefixSearch) start(top *prefixNode) {
	if s.seen('s', top) {
		return
	}
	frame := top.frame
	q := frame.particle
	if q.Occurrence.Unbounded || frame.count < q.Occurrence.Max {
		fresh := prefixFrame{particle: q, count: frame.count}
		switch q.Kind {
		case ParticleElement, ParticleAny:
			if s.accepts(q) {
				s.emit(s.replace(top, fresh))
			}
		case ParticleSequence:
			if len(q.Children) != 0 {
				s.enter(s.replace(top, fresh), q.Children[0])
			}
		case ParticleChoice:
			for i, branch := range q.Children {
				s.c.step(1)
				fresh.index = i
				s.enter(s.replace(top, fresh), branch)
			}
		case ParticleAll:
			s.c.scratch(s.c.size(int64(len(q.Children)), 1))
			fresh.used = make([]bool, len(q.Children))
			s.allMembers(s.replace(top, fresh))
		}
	}
	if frame.count >= q.Occurrence.Min || s.nullable[q] {
		s.leave(top.parent)
	}
}

// allMembers lets every unused member of the top frame's all group consume
// the child. Members are single elements: a member that consumes nothing is
// not chosen, so it stays available for a later child.
func (s *prefixSearch) allMembers(top *prefixNode) {
	q := top.frame.particle
	for i, member := range q.Children {
		s.c.step(1)
		if top.frame.used[i] || member.Occurrence.Max == 0 || !s.accepts(member) {
			continue
		}
		frame := top.frame
		frame.index = i
		s.emit(s.node(prefixFrame{particle: member}, s.replace(top, frame)))
	}
}

// finish completes the current occurrence of the top frame, which consumed
// at least one child.
func (s *prefixSearch) finish(top *prefixNode) {
	frame := top.frame
	q := frame.particle
	if !q.Occurrence.Unbounded || frame.count < q.Occurrence.Min {
		frame.count++
	}
	s.start(s.replace(top, frame))
}

// leave continues in the parent top after its current child particle left.
func (s *prefixSearch) leave(top *prefixNode) {
	if top == nil || s.seen('l', top) {
		return
	}
	frame := top.frame
	q := frame.particle
	switch q.Kind {
	case ParticleSequence:
		if frame.index+1 < len(q.Children) {
			frame.index++
			s.enter(s.replace(top, frame), q.Children[frame.index])
			return
		}
	case ParticleAll:
		s.c.scratch(s.c.size(int64(len(frame.used)), 1))
		used := make([]bool, len(frame.used))
		copy(used, frame.used)
		used[frame.index] = true
		frame.used = used
		top = s.replace(top, frame)
		s.allMembers(top)
		for i, member := range q.Children {
			s.c.step(1)
			if !used[i] && member.Occurrence.Min > 0 && member.Occurrence.Max > 0 {
				return
			}
		}
	}
	// The body is complete. An occurrence that consumed nothing adds no
	// path: start already offered to leave the particle instead.
	if frame.consumed {
		s.finish(top)
	}
}

// prefixMatcher consumes the children of one element in order and binds
// each to the leaf particle its prefix determines, until the first child it
// cannot bind. Between children it keeps only the surviving configurations,
// charged as scratch; the rest of each child's search is released.
type prefixMatcher struct {
	c        *Context
	particle *ParticleSchema
	nullable map[*ParticleSchema]bool
	states   []*prefixNode
	// kept is the scratch charge of states.
	kept    int64
	next    int
	stopped bool
	// retained keeps states charged as retained storage between operations,
	// for an open element; otherwise they are operation scratch.
	retained bool
	// ambiguous records that binding stopped at a child with two candidate
	// particles or too many configurations, rather than at a child the
	// content model rejects.
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
	// As in MatchParticle, prepared nullability is immutable schema
	// information shared by the run's later matches.
	if c.ParticleNullable == nil {
		c.reserve(64)
		c.ParticleNullable = matcher.Nullable
	} else {
		matcher.Nullable = c.ParticleNullable
	}
	c.scratch(64)
	if !matcher.Prepare(particle, make(map[*ParticleSchema]bool)) {
		return nil, false
	}
	return &prefixMatcher{c: c, particle: particle, nullable: matcher.Nullable}, true
}

// releaseScratchBytes returns n bytes of operation scratch whose storage is
// no longer referenced, before the operation ends.
func (c *Context) releaseScratchBytes(n int64) {
	if c == nil || c.run == nil || n <= 0 {
		return
	}
	r := c.run
	if n > r.scratchRetained {
		panic(runAbort{errors.New("xsd: scratch release exceeds charge")})
	}
	if err := r.meter.Release(n); err != nil {
		panic(runAbort{err})
	}
	r.scratchRetained -= n
}

// retainScratch turns n bytes of operation scratch into retained storage.
func (c *Context) retainScratch(n int64) {
	if c == nil || c.run == nil || n <= 0 {
		return
	}
	r := c.run
	if n > r.scratchRetained {
		panic(runAbort{errors.New("xsd: retained scratch exceeds charge")})
	}
	r.scratchRetained -= n
	r.retained += n
}

// dropKept releases the surviving configurations and their charge.
func (m *prefixMatcher) dropKept() {
	c := m.c
	m.states = nil
	if m.kept == 0 || c == nil || c.run == nil {
		m.kept = 0
		return
	}
	if !m.retained {
		c.releaseScratchBytes(m.kept)
	} else {
		r := c.run
		r.retained -= m.kept
		if err := r.meter.Release(m.kept); err != nil {
			m.kept = 0
			panic(runAbort{err})
		}
	}
	m.kept = 0
}

// scratchCharged is the operation scratch charged so far.
func (c *Context) scratchCharged() int64 {
	if c == nil || c.run == nil {
		return 0
	}
	return c.run.scratchRetained
}

// keptBytes is the storage of the surviving configurations: the slice and
// every node on their paths, with its all-group marks.
func (m *prefixMatcher) keptBytes(states []*prefixNode) int64 {
	c := m.c
	kept := c.size(int64(cap(states)), int64(unsafe.Sizeof((*prefixNode)(nil))))
	for _, state := range states {
		for n := state; n != nil; n = n.parent {
			c.step(1)
			kept = c.sum(kept, prefixNodeBytes, int64(len(n.frame.used)))
		}
	}
	return kept
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
	before := c.scratchCharged()
	c.scratch(256)
	c.scratch(128)
	s := prefixSearch{c: c, nullable: m.nullable, child: child, keys: make(map[string]bool), visited: make(map[string]bool), buffer: make([]byte, 0, 128)}
	if m.next == 0 {
		s.enter(nil, m.particle)
	}
	for _, state := range m.states {
		s.finish(state)
	}
	out, overflow := s.out, s.overflow
	stop := overflow || len(out) == 0
	var leaf *ParticleSchema
	if !stop {
		leaf = out[0].frame.particle
		for _, state := range out[1:] {
			c.step(1)
			if state.frame.particle != leaf {
				stop = true
				break
			}
		}
	}
	// Keep the surviving configurations, which emit copied, and drop the
	// search with the previous configurations.
	var kept int64
	if !stop {
		kept = min(m.keptBytes(out), c.scratchCharged()-before)
	}
	searched := c.scratchCharged() - before
	m.states, s = nil, prefixSearch{}
	c.releaseScratchBytes(searched - kept)
	m.dropKept()
	if m.retained {
		c.retainScratch(kept)
	}
	m.kept = kept
	if stop {
		m.stopped, m.ambiguous = true, overflow || leaf != nil
		return ElementMatch{}, false
	}
	m.states = out
	match := ElementMatch{Index: m.next, Particle: leaf}
	if leaf.Kind == ParticleElement {
		match.Schema, _ = c.ResolveElement(leaf.Element)
	} else if c.local != nil {
		c.noteWildcardAdmission(wildcardNamespace(leaf.Any.Namespace, child.Name.Space, c), leaf.Any.ProcessContents)
	}
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
	matcher.dropKept()
	return result
}
