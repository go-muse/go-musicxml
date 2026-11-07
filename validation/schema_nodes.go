package validation

import "unsafe"

// schemaNodeTable holds the XSD state of every node of a session: a dense
// slice indexed by NodeID−1 for the IDs the session has seen when binding
// starts or a window closes, which callers number from 1 in document order,
// and a map for any other ID.
type schemaNodeTable struct {
	dense    []schemaNode
	overflow map[NodeID]schemaNode
}

var (
	schemaNodeSlotBytes = int64(unsafe.Sizeof(schemaNode{}))
	// A map entry with its share of hash metadata and growth.
	schemaNodeOverflowEntryBytes = 3 * (int64(unsafe.Sizeof(NodeID(0))) + schemaNodeSlotBytes)
)

// The map header and its first groups: about 1 KiB live for one or two
// entries.
const schemaNodeOverflowHeaderBytes int64 = 1024

func (t *schemaNodeTable) get(id NodeID) (schemaNode, bool) {
	if id != 0 && id <= NodeID(len(t.dense)) {
		v := t.dense[id-1]
		return v, v.set
	}
	v, ok := t.overflow[id]
	return v, ok
}

// at returns the node's state, or the zero state.
func (t *schemaNodeTable) at(id NodeID) schemaNode {
	v, _ := t.get(id)
	return v
}

// put stores a node's state; an ID outside the dense slice is charged as a
// map entry.
func (t *schemaNodeTable) put(s *Session, id NodeID, v schemaNode) error {
	v.set = true
	if id != 0 && id <= NodeID(len(t.dense)) {
		t.dense[id-1] = v
		return nil
	}
	if _, exists := t.overflow[id]; !exists {
		charge := schemaNodeOverflowEntryBytes
		if t.overflow == nil {
			charge += schemaNodeOverflowHeaderBytes
		}
		if err := s.retain(charge); err != nil {
			return err
		}
		if t.overflow == nil {
			t.overflow = make(map[NodeID]schemaNode)
		}
	}
	t.overflow[id] = v
	return nil
}

// grow extends the dense slice to the IDs up to n. The new array is admitted
// before it is allocated and the old one released once copied: both live
// while copying.
func (t *schemaNodeTable) grow(s *Session, n int) error {
	if n <= len(t.dense) {
		return nil
	}
	// A quarter more each time keeps copying linear in the number of nodes
	// while windows grow the table by small steps.
	size := max(n, len(t.dense)+len(t.dense)/4)
	if err := s.retain(int64(size) * schemaNodeSlotBytes); err != nil {
		return err
	}
	next := make([]schemaNode, size)
	copy(next, t.dense)
	if old := int64(len(t.dense)) * schemaNodeSlotBytes; old != 0 {
		_ = s.account.Release(BudgetBufferedBytes, old)
		s.retained -= old
	}
	t.dense = next
	// Entries of IDs now in the dense range move there.
	for id, v := range t.overflow {
		if id != 0 && id <= NodeID(len(t.dense)) {
			t.dense[id-1] = v
			delete(t.overflow, id)
		}
	}
	return nil
}
