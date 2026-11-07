package validation

// Test helpers that store node states without charging, as tests that build
// a session's schema state by hand did with the map it replaced.

func (t *schemaNodeTable) set(id NodeID, v schemaNode) {
	v.set = true
	if id != 0 && id <= NodeID(len(t.dense)) {
		t.dense[id-1] = v
		return
	}
	if t.overflow == nil {
		t.overflow = make(map[NodeID]schemaNode)
	}
	t.overflow[id] = v
}

func (t *schemaNodeTable) remove(id NodeID) {
	if id != 0 && id <= NodeID(len(t.dense)) {
		t.dense[id-1] = schemaNode{}
		return
	}
	delete(t.overflow, id)
}

func schemaNodesOf(nodes map[NodeID]schemaNode) schemaNodeTable {
	var t schemaNodeTable
	for id, v := range nodes {
		t.set(id, v)
	}
	return t
}
