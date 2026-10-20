package validation

// outcomeBits stores the compact Pass/NotApplicable outcomes of one rule.
// Adapters number nodes densely from one, so the outcome of node ID i lives in
// two bits at index i-1 of words. An ID far beyond the number of admitted
// nodes (a sparse caller numbering) goes to the overflow map instead, so the
// dense table never exceeds a constant factor of the node count.
type outcomeBits struct {
	words    []uint64
	overflow map[NodeID]successfulOutcome
}

// outcomeDenseSlack bounds how far beyond the admitted node count a dense
// index may reach.
const outcomeDenseSlack = 4096

// get reads the dense table first. An ID stored in the overflow map while it
// was beyond the table keeps its entry there after the table grows over it,
// so an empty dense slot falls back to the map.
func (b *outcomeBits) get(id NodeID) successfulOutcome {
	if b == nil || id == 0 {
		return 0
	}
	i := uint64(id - 1)
	if w := i / 32; w < uint64(len(b.words)) {
		if v := successfulOutcome(b.words[w] >> (2 * (i % 32)) & 3); v != 0 {
			return v
		}
	}
	return b.overflow[id]
}

func (b *outcomeBits) set(s *Session, id NodeID, v successfulOutcome) error {
	i := uint64(id - 1)
	w := i / 32
	if id != 0 && w >= uint64(len(b.words)) {
		if w < uint64(cap(b.words)) {
			b.words = b.words[:w+1]
		} else if i < 2*uint64(len(s.sequence))+outcomeDenseSlack {
			grown := max(uint64(16), 2*uint64(cap(b.words)), w+1)
			if err := s.retain(int64(8 * (grown - uint64(cap(b.words))))); err != nil {
				return err
			}
			next := make([]uint64, w+1, grown)
			copy(next, b.words)
			b.words = next
		}
	}
	if id != 0 && w < uint64(len(b.words)) {
		shift := 2 * (i % 32)
		b.words[w] = b.words[w]&^(3<<shift) | uint64(v)<<shift
		return nil
	}
	if err := s.retain(32); err != nil { // NodeID, outcome byte and map allowance.
		return err
	}
	if b.overflow == nil {
		b.overflow = make(map[NodeID]successfulOutcome)
	}
	b.overflow[id] = v
	return nil
}
