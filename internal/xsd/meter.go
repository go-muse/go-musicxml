package xsd

import (
	"fmt"
	"math/bits"
	"regexp/syntax"
	"slices"
	"unsafe"
)

// Resource ownership: source Subject facts and compiled schema DTOs are borrowed.
// The Run owns node-text copies, binding/results, reached metadata caches, scalar
// views, and identity indexes; their conservative allocation allowances last
// until Close. Scratch owns grammar DP/diagnostics, temporary maps, regex/URI
// workspaces, and formatting buffers, and is released at every operation exit.
// Work is charged at visits, edges, comparisons, copies, or byte-linear library
// entry; cancellation is also checked immediately after opaque library calls.
// Identity units are unique seeded strings, retained reference occurrences, and
// distinct actual-ID containing-owner bindings. Work/identity never refund.
// Source nodes/attributes/depth and report diagnostic quotas remain core-owned.
//
// format accounts the bounded worst-case quoted diagnostic expansion before fmt
// allocates its output; a later retained Detail owns its independent allowance.
func format(c *Context, pattern string, args ...any) string {
	if c == nil || c.run == nil {
		return fmt.Sprintf(pattern, args...)
	}
	n := c.sum(int64(len(pattern)), 64)
	for _, arg := range args {
		c.step(1)
		switch value := arg.(type) {
		case string:
			c.bytes(len(value))
			n = c.sum(n, c.size(int64(len(value)), 6))
		case *syntax.Error:
			c.bytes(len(value.Expr))
			n = c.sum(n, 128, c.size(int64(len(value.Expr)), 6))
		case []string:
			for _, item := range value {
				c.bytes(len(item))
				n = c.sum(n, 8, c.size(int64(len(item)), 6))
			}
		default:
			n = c.sum(n, 128)
		}
	}
	c.scratch(n)
	out := fmt.Sprintf(pattern, args...)
	c.step(0)
	return out
}

// appendRetained and appendScratch acquire the entire next backing array before
// allocation. Old arrays remain included in this operation/run's conservative
// allowance until its boundary; no cumulative work or identity charge is undone.
func appendRetained[T any](c *Context, values []T, value T) []T {
	return appendMetered(c, values, value, false)
}
func appendScratch[T any](c *Context, values []T, value T) []T {
	return appendMetered(c, values, value, true)
}
func appendMetered[T any](c *Context, values []T, value T, scratch bool) []T {
	if c != nil && c.run != nil && len(values) == cap(values) {
		capacity := c.sum(int64(cap(values)), int64(max(1, cap(values))))
		n := c.size(capacity, int64(unsafe.Sizeof(value)))
		if scratch {
			c.scratch(n)
		} else {
			c.reserve(n)
		}
		c.step(int64(len(values)))
		next := make([]T, len(values), int(capacity))
		copy(next, values)
		values = next
	}
	return append(values, value)
}

// regexp owns an uninterruptible parser, compiler, and VM. Its syntax tree gives
// a conservative bound on repetition expansion before Compile allocates an
// expanded program. This is resource admission only: Compile and MatchString
// remain the authoritative pattern implementation for both drivers.
func regexpAllowance(pattern string, c *Context) int64 {
	if c == nil || c.run == nil {
		return 0
	}
	c.bytes(len(pattern))
	c.scratch(c.sum(1024, c.size(int64(len(pattern)), 128)))
	tree, err := syntax.Parse(pattern, syntax.Perl)
	c.step(0)
	if err != nil {
		return 0
	}
	var size func(*syntax.Regexp) int64
	size = func(node *syntax.Regexp) int64 {
		c.step(1)
		n := c.sum(1, int64(len(node.Rune)))
		for _, child := range node.Sub {
			n = c.sum(n, size(child))
		}
		if node.Op == syntax.OpRepeat {
			repeat := node.Max
			if repeat < 0 {
				repeat = node.Min + 2
			}
			n = c.size(n, int64(max(1, repeat)))
		}
		return n
	}
	n := size(tree)
	c.step(n)
	c.scratch(c.size(n, 128))
	return n
}

// Endpoint maps have randomized iteration order. Budgeting the library sort's
// observed comparisons would make identical source documents need different
// limits. Admit a deterministic conservative O(n log n) comparison allowance
// before the opaque sort, then check cancellation in every real comparator.
// SortFunc's introspective partition/heapsort algorithm is bounded by this
// allowance; this changes admission only, never the selected endpoint ordering.
func sortPositions(positions []int, c *Context) {
	if c != nil && c.run != nil && len(positions) > 1 {
		n := int64(len(positions))
		c.step(c.size(c.size(n, 8), int64(bits.Len64(uint64(n))+1)))
	}
	slices.SortFunc(positions, func(a, b int) int { c.step(0); return b - a })
}

// A strings.Builder exposes allocator-rounded backing capacity, even when its
// first write/Grow requests an exact length. Before any such allocation admit
// a conservative 2x capacity (at least a minimum size-class allowance). Keep
// that allowance for the existing retained/scratch lifetime instead of relying
// on after-allocation measurement. Empty builders allocate and reserve nothing;
// compatibility nil-run helpers retain their original allocation behavior.
func (c *Context) builderAllowance(n int64) int64 {
	if n == 0 {
		return 0
	}
	if c == nil || c.run == nil {
		return n
	}
	return max(64, c.size(n, 2))
}
