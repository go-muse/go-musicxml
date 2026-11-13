package validation

import (
	"context"
	"errors"
	"unsafe"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

// Under the windowed strategy a session assesses a document window by window.
// A window is an occurrence whose subtree is complete when it closes: a
// partwise measure inside its part, or a timewise measure. Every node outside
// a window belongs to the skeleton, the score header and the windows'
// ancestors, which stays until Finish. Other documents (opus, container,
// sounds) have no windows: the whole document is skeleton. A segment is a
// child of a window ancestor that is neither a window nor an ancestor, such
// as the part-list: it is bound with its subtree when it closes, as a window
// is, but it is not a window.
var windowRootPaths = [][]string{{"score-partwise", "part", "measure"}, {"score-timewise", "measure"}}

// evaluationWindow is one window: its root and the nodes it covers, as the
// half-open range [first, end) of the session's document-order sequence.
type evaluationWindow struct {
	root       NodeID
	first, end int
}

// windowObserver receives every closed window in document order and, at
// Finish, the length of the sequence as a window with no root. It is an
// unsynchronized package variable: only serial tests set it.
var windowObserver func(root NodeID, first, end int)

func setWindowObserver(observer func(root NodeID, first, end int)) func(NodeID, int, int) {
	previous := windowObserver
	windowObserver = observer
	return previous
}

// windowBindObserver receives the root of every window bound when it
// closed. It is an unsynchronized package variable: only serial tests set it.
var windowBindObserver func(root NodeID)

func setWindowBindObserver(observer func(root NodeID)) func(NodeID) {
	previous := windowBindObserver
	windowBindObserver = observer
	return previous
}

// opensWindow reports whether a node named name, opened under the current
// stack, roots a window.
func (s *Session) opensWindow(name QName) bool {
	if s.strategy != strategyWindowed || s.window != nil || name.Space != "" {
		return false
	}
	for _, path := range windowRootPaths {
		if len(path) != len(s.stack)+1 || path[len(path)-1] != name.Local {
			continue
		}
		matches := true
		for i, ancestor := range s.stack {
			if ancestor.header.Name.Space != "" || ancestor.header.Name.Local != path[i] {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

// boundWindow is a window bound when its root closed: the use its parent's
// prefix gave it, its root's completion and the number of nodes it covers.
type boundWindow struct {
	use        xsd.Use
	completion xsd.Completion
	size       int
}

// windowAncestor reports whether a node named name, opened under the
// current stack, is an ancestor of windows: the root of a score, or a
// partwise part.
func (s *Session) windowAncestor(name QName) bool {
	if name.Space != "" {
		return false
	}
	for _, path := range windowRootPaths {
		if len(s.stack)+1 >= len(path) || path[len(s.stack)] != name.Local {
			continue
		}
		matches := true
		for i, ancestor := range s.stack {
			if ancestor.header.Name.Space != "" || ancestor.header.Name.Local != path[i] {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

// windowBinding reports whether the session binds windows as they close:
// under the windowed strategy, with a schema not yet bound whose root can be
// bound.
func (s *Session) windowBinding() bool {
	return s.strategy == strategyWindowed && s.schema != nil && s.schema.active != nil && !s.schema.bound && s.root != nil && !s.schemaRootUnsupported()
}

// openWindowAncestor starts matching the children of a window ancestor whose
// attributes are sealed complete, n being the open node on top of the stack.
// Its children's declarations then come from their prefix as they open. An
// ancestor whose declaration is unknown, or that the run cannot open, leaves
// the windows below it to be bound when the root closes.
func (s *Session) openWindowAncestor(ctx context.Context, n *factNode) error {
	if !s.windowBinding() || n.attributeSeal == nil || n.attributeSeal.State != EvidenceKnown || !n.attributeSeal.Value {
		return nil
	}
	state := s.schema
	var use xsd.Use
	var ok bool
	if n == s.root {
		use, ok = state.active.kernel.Root(xsd.QName{Space: n.header.Name.Space, Local: n.header.Name.Local})
	} else {
		use, ok = state.uses[n.header.ID]
	}
	if !ok {
		return nil
	}
	stack := s.stack
	s.stack = s.stack[:len(s.stack)-1]
	ancestor := s.windowAncestor(n.header.Name)
	s.stack = stack
	if !ancestor {
		return nil
	}
	if err := s.ensureSchemaRun(); err != nil {
		return err
	}
	subject, err := s.schemaSubject(ctx, n)
	if err != nil {
		return err
	}
	open, openErr := state.run.Open(ctx, use, subject)
	released := schemaSubjectBytes(n)
	_ = s.account.Release(BudgetBufferedBytes, released)
	s.retained -= released
	if openErr != nil || open == nil {
		return openErr
	}
	if err := s.retain(128); err != nil {
		return errors.Join(err, state.run.CloseOpen(open))
	}
	if state.opened == nil {
		state.opened = make(map[NodeID]*xsd.OpenElement)
	}
	state.opened[n.header.ID] = open
	return nil
}

// pushWindowChild gives child, about to open under the node on top of the
// stack, the declaration its prefix determines when that node is an open
// window ancestor, and keeps it when the child roots a window or is itself a
// window ancestor.
func (s *Session) pushWindowChild(ctx context.Context, child *factNode) error {
	if len(s.stack) == 0 || !s.windowBinding() || s.schema.opened == nil {
		return nil
	}
	state := s.schema
	open := state.opened[s.stack[len(s.stack)-1].header.ID]
	if open == nil {
		return nil
	}
	w, ok, err := state.run.PushChild(ctx, open, xsd.Child{ID: xsd.SubjectID(child.header.ID), Name: xsd.QName{Space: child.header.Name.Space, Local: child.header.Name.Local}, Path: child.path()})
	if err != nil || !ok || w.Mode != xsd.WitnessDeclared {
		return err
	}
	if err := s.retain(windowUseEntryBytes); err != nil {
		return err
	}
	if state.uses == nil {
		state.uses = make(map[NodeID]xsd.Use)
	}
	state.uses[child.header.ID] = w.Use
	if s.opensWindow(child.header.Name) || s.windowAncestor(child.header.Name) {
		return nil
	}
	// A segment: the child opens next in the sequence.
	if err := s.retain(segmentEntryBytes); err != nil {
		return err
	}
	if state.segments == nil {
		state.segments = make(map[NodeID]int)
	}
	state.segments[child.header.ID] = len(s.sequence)
	return nil
}

// Map entries of the per-window tables are charged at three times their key
// and value: hash metadata, overflow buckets and growth more than double the
// bare entry.
var (
	windowUseEntryBytes   = 3 * (int64(unsafe.Sizeof(NodeID(0))) + int64(unsafe.Sizeof(xsd.Use{})))
	segmentEntryBytes     = 3 * (int64(unsafe.Sizeof(NodeID(0))) + int64(unsafe.Sizeof(int(0))))
	boundWindowEntryBytes = 3 * (int64(unsafe.Sizeof(NodeID(0))) + int64(unsafe.Sizeof(boundWindow{})))
)

// closeWindowAncestor stops matching the children of a closed window
// ancestor.
func (s *Session) closeWindowAncestor(n *factNode) error {
	if s.schema == nil || s.schema.opened == nil {
		return nil
	}
	open := s.schema.opened[n.header.ID]
	if open == nil {
		return nil
	}
	delete(s.schema.opened, n.header.ID)
	return s.schema.run.CloseOpen(open)
}

// closeWindow is called when a window's root closes, with its subtree
// complete and the root's model evidence established. A root whose parent's
// prefix gave it a declaration is bound here with its subtree, exactly as
// binding the whole document when the root closes would bind it.
func (s *Session) closeWindow(ctx context.Context, w evaluationWindow) error {
	if windowObserver != nil {
		windowObserver(w.root, w.first, w.end)
	}
	held := len(s.accumulator.heldOutcomes) + len(s.accumulator.heldFindings)
	bound, err := s.bindClosed(ctx, w.root, w.first, w.end)
	if err != nil {
		return err
	}
	// A window whose binding proved a failure leaves that failure held,
	// without the decision rules would read until the root binds.
	proved := len(s.accumulator.heldOutcomes)+len(s.accumulator.heldFindings) != held
	if bound && windowBindObserver != nil {
		windowBindObserver(w.root)
	}
	if err := s.readTimelineWindow(ctx, w, bound); err != nil {
		return err
	}
	if !bound || proved {
		if windowRulesObserver != nil {
			windowRulesObserver(false, s.windowKeptCount())
		}
		return nil
	}
	err = s.evaluateWindowRules(ctx, w)
	if windowRulesObserver != nil {
		windowRulesObserver(true, s.windowKeptCount())
	}
	return err
}

// closeSegment is called when n closes; a segment is bound there with its
// subtree, as a window is.
func (s *Session) closeSegment(ctx context.Context, n *factNode) error {
	if s.schema == nil || s.schema.segments == nil {
		return nil
	}
	first, ok := s.schema.segments[n.header.ID]
	if !ok {
		return nil
	}
	delete(s.schema.segments, n.header.ID)
	bound, err := s.bindClosed(ctx, n.header.ID, first, len(s.sequence))
	if bound && segmentBindObserver != nil {
		segmentBindObserver(n.header.ID, first, len(s.sequence))
	}
	return err
}

// segmentBindObserver receives the root of every segment bound when it
// closed and the sequence range it covers. It is an unsynchronized package
// variable: only serial tests set it.
var segmentBindObserver func(root NodeID, first, end int)

func setSegmentBindObserver(observer func(root NodeID, first, end int)) func(NodeID, int, int) {
	previous := segmentBindObserver
	segmentBindObserver = observer
	return previous
}

// bindClosed binds the closed subtree rooted at root, which covers the
// sequence range [first, end), when its parent's prefix gave it a use.
func (s *Session) bindClosed(ctx context.Context, root NodeID, first, end int) (bool, error) {
	if !s.windowBinding() || s.schema.uses == nil {
		return false, nil
	}
	state := s.schema
	use, ok := state.uses[root]
	if !ok {
		return false, nil
	}
	if err := s.ensureSchemaNodes(); err != nil {
		return false, err
	}
	// Binding the document can still be skipped (a resource limit,
	// cancellation or rejected event before the root closes); the proved
	// failures wait until it is not.
	var consumed int
	s.accumulator.holding = true
	completion, err := s.bindSubtree(ctx, s.node(root), use, first, len(s.stack), &consumed)
	s.accumulator.holding = false
	if err != nil {
		return false, err
	}
	if err := s.retain(boundWindowEntryBytes); err != nil {
		return false, err
	}
	if state.windows == nil {
		state.windows = make(map[NodeID]boundWindow)
	}
	state.windows[root] = boundWindow{use: use, completion: completion, size: end - first}
	return true, nil
}

// eulerEnter is the position of binding a node in a top-down traversal of
// the whole document: its preorder index pre and its depth give the number of
// earlier enter and exit steps.
func eulerEnter(pre, depth int) uint64 {
	return uint64(2*pre - depth)
}

// eulerExit is the position of completing a node with the given number of
// descendants: every descendant enters and exits in between.
func eulerExit(pre, depth, descendants int) uint64 {
	return eulerEnter(pre, depth) + 2*uint64(descendants) + 1
}

// subtreeOffsets returns, for each child of n, the number of nodes before
// its subtree within n's subtree, followed by the number of n's descendants.
func (s *Session) subtreeOffsets(ctx context.Context, n *factNode, scratch *schemaBindScratch) ([]int, error) {
	if err := scratch.reserve(int64(len(n.children))+1, 8); err != nil {
		return nil, err
	}
	offsets := make([]int, len(n.children)+1)
	total := 0
	for i, child := range n.children {
		offsets[i] = total
		size, err := s.subtreeSize(ctx, child)
		if err != nil {
			return nil, err
		}
		total += size
	}
	offsets[len(n.children)] = total
	return offsets, nil
}

// subtreeSize counts the nodes of n's subtree, taking a bound window's size
// as recorded.
func (s *Session) subtreeSize(ctx context.Context, n *factNode) (int, error) {
	if err := s.step(ctx, 1); err != nil {
		return 0, err
	}
	if w, ok := s.schema.windows[n.header.ID]; ok {
		return w.size, nil
	}
	size := 1
	for _, child := range n.children {
		childSize, err := s.subtreeSize(ctx, child)
		if err != nil {
			return 0, err
		}
		size += childSize
	}
	return size, nil
}
