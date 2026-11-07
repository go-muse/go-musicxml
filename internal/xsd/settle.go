package xsd

import (
	"context"
	"errors"
	"iter"
	"unsafe"
)

// Settled is what a completed subject keeps once Settle has made its run
// forget it: the results its rules read and, when the caller kept them, its
// typed results. Readers that finish a document read a settled subject here
// instead of through its Binding, which no longer reads anything.
type Settled struct {
	run *Run
	// results is nil when all five results are fresh.
	results *NodeResults
	// typed is nil when Settle released the typed results.
	typed *settledTyped
}

type settledTyped struct {
	scalar     *ScalarResult
	actual     *TypeRef
	attributes []*AttributeResult
}

var (
	// The bound state's own size, at most what freeing it frees.
	boundStateObjectBytes = int64(unsafe.Sizeof(boundState{}))
	// The typed record rounded up to the allocator's 16-byte size classes.
	settledTypedBytes = (int64(unsafe.Sizeof(settledTyped{})) + 15) &^ 15
)

// settledState replaces a settled subject in Run.states: the subject stays
// bound, so binding it again is refused, and no Binding owns it any more.
var settledState = &boundState{done: true}

// Settle makes the run forget a completed subject and returns what the
// caller keeps of it. keepTyped keeps the scalar, actual type and attribute
// results; without it their retention is released with the bound state's.
// The Binding reads nothing afterwards.
func (r *Run) Settle(ctx context.Context, b Binding, keepTyped bool) (out Settled, err error) {
	if err = r.check(ctx); err != nil {
		return
	}
	if !r.owns(b) || !b.state.done || b.state.bindWork != nil {
		return out, errors.New("xsd: settling a subject that is not complete")
	}
	defer recoverRun(&err)
	s := b.state
	out = Settled{run: r, results: s.final}
	release := boundStateObjectBytes
	if keepTyped {
		r.reserve(settledTypedBytes)
		out.typed = &settledTyped{scalar: s.finalScalar, actual: s.actual, attributes: s.attributes}
	} else {
		release += s.typedCharged
	}
	r.states[s.id] = settledState
	s.run, s.actual, s.attributes, s.final, s.finalScalar = nil, nil, nil, nil, nil
	r.retained -= release
	if err := r.meter.Release(release); err != nil {
		return Settled{}, err
	}
	return out, nil
}

// Valid reports a record returned by Settle.
func (s Settled) Valid() bool { return s.run != nil }

// Typed reports that the typed results were kept.
func (s Settled) Typed() bool { return s.typed != nil }

// Results returns the subject's five results.
func (s Settled) Results() NodeResults {
	if s.run == nil {
		return NodeResults{}
	}
	if s.results == nil {
		return freshResults()
	}
	return *s.results
}

// Scalar returns the kept scalar result, or the zero result.
func (s Settled) Scalar() ScalarResult {
	if s.typed == nil || s.typed.scalar == nil {
		return ScalarResult{}
	}
	return *s.typed.scalar
}

// ActualType returns the kept actual type.
func (s Settled) ActualType() (TypeID, bool) {
	if s.typed == nil || s.typed.actual == nil {
		return TypeID{}, false
	}
	return s.run.typeID(s.typed.actual)
}

// Attributes yields the kept attribute results in the order Run.Attributes
// yields them.
func (s Settled) Attributes() iter.Seq[AttributeResult] {
	return func(yield func(AttributeResult) bool) {
		if s.typed == nil {
			return
		}
		for _, attribute := range s.typed.attributes {
			if !yield(*attribute) {
				return
			}
		}
	}
}
