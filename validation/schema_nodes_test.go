package validation

import "testing"

// The node table keeps every state it is given: IDs within the dense slice
// there, ID 0 and IDs beyond it in the overflow map, which growing the slice
// empties into it. Growing admits the new array before releasing the old one,
// overflow entries are charged once each with the map's header on the first,
// and the charges match what the table holds.
func TestSchemaNodeTableKeepsEveryState(t *testing.T) {
	account, err := NewBudgetAccount(Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	s := &Session{account: account}
	var table schemaNodeTable
	if err := table.grow(s, 4); err != nil {
		t.Fatal(err)
	}
	if s.retained != 4*schemaNodeSlotBytes {
		t.Fatalf("dense slice of 4 charged %d", s.retained)
	}
	for _, id := range []NodeID{0, 2, 6, 9} {
		if err := table.put(s, id, schemaNode{reason: Code(rune('a' + id))}); err != nil {
			t.Fatal(err)
		}
	}
	if err := table.put(s, 6, schemaNode{reason: "again"}); err != nil {
		t.Fatal(err)
	}
	overflow := 3*schemaNodeOverflowEntryBytes + schemaNodeOverflowHeaderBytes
	if s.retained != 4*schemaNodeSlotBytes+overflow {
		t.Fatalf("overflow of 3 IDs charged %d", s.retained-4*schemaNodeSlotBytes)
	}
	check := func() {
		t.Helper()
		for id, want := range map[NodeID]Code{0: "a", 2: "c", 6: "again", 9: "j"} {
			if got, ok := table.get(id); !ok || got.reason != want {
				t.Fatalf("ID %d: %+v %v", id, got, ok)
			}
		}
		for _, id := range []NodeID{1, 3, 7, 100} {
			if _, ok := table.get(id); ok {
				t.Fatalf("ID %d present", id)
			}
		}
	}
	check()
	peak := s.retained
	if err := table.grow(s, 7); err != nil {
		t.Fatal(err)
	}
	check()
	if len(table.dense) != 7 || len(table.overflow) != 2 {
		t.Fatalf("grown table: %d dense, %d in the map", len(table.dense), len(table.overflow))
	}
	if s.retained != peak+3*schemaNodeSlotBytes {
		t.Fatalf("growing to 7 left %d charged, want %d", s.retained, peak+3*schemaNodeSlotBytes)
	}
	// Growth by small steps adds at least a quarter.
	if err := table.grow(s, 8); err != nil {
		t.Fatal(err)
	}
	if len(table.dense) != 8 {
		t.Fatalf("grown to %d", len(table.dense))
	}
	if err := table.grow(s, 9); err != nil {
		t.Fatal(err)
	}
	if len(table.dense) != 10 {
		t.Fatalf("grown to %d, want 10", len(table.dense))
	}
	check()
}
