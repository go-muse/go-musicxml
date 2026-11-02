package validation

// Under the windowed strategy a session assesses a document window by window.
// A window is an occurrence whose subtree is complete when it closes: a
// partwise measure inside its part, or a timewise measure. Every node outside
// a window belongs to the skeleton, the score header and the windows'
// ancestors, which stays until Finish. Other documents (opus, container,
// sounds) have no windows: the whole document is skeleton.
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

// closeWindow is called when a window's root closes, with its subtree
// complete. The per-measure engine assesses the window here; until it lands,
// the window is only recorded.
func (s *Session) closeWindow(w evaluationWindow) {
	if windowObserver != nil {
		windowObserver(w.root, w.first, w.end)
	}
}
