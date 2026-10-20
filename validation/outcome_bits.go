package validation

// outcomeBits stores the compact Pass/NotApplicable outcomes of one rule.
// Adapters number nodes densely from one, so the outcome of node ID i lives in
// two bits at index i-1 of words. An ID far beyond the number of admitted
// nodes (a sparse caller numbering) goes to the overflow map instead, so the
// dense table never exceeds a constant factor of the node count.
type outcomeBits struct {
	words []uint64
	// charged is the admitted size of the words backing array.
	charged  int64
	overflow map[NodeID]successfulOutcome
}

// outcomeDenseSlack bounds how far beyond the admitted node count a dense
// index may reach.
const outcomeDenseSlack = 4096

// The overflow map measures about 192 bytes with one entry and 36–42 bytes
// per entry at larger sizes; old and new tables coexist while it grows.
const (
	outcomeOverflowHeaderBytes int64 = 256
	outcomeOverflowEntryBytes  int64 = 64
)

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
			// The new array is admitted whole before allocation: it and
			// the old one coexist during the copy. The old array's charge
			// is released once it is replaced.
			grown := max(uint64(16), 2*uint64(cap(b.words)), w+1)
			size := allocationSlack(int64(8 * grown))
			if err := s.retain(size); err != nil {
				return err
			}
			next := make([]uint64, w+1, grown)
			copy(next, b.words)
			b.words = next
			if b.charged != 0 {
				_ = s.account.Release(BudgetBufferedBytes, b.charged)
				s.retained -= b.charged
			}
			b.charged = size
		}
	}
	if id != 0 && w < uint64(len(b.words)) {
		shift := 2 * (i % 32)
		b.words[w] = b.words[w]&^(3<<shift) | uint64(v)<<shift
		return nil
	}
	// A map entry with its share of growth, plus the map header and first
	// group on the first entry.
	charge := outcomeOverflowEntryBytes
	if b.overflow == nil {
		charge += outcomeOverflowHeaderBytes
	}
	if err := s.retain(charge); err != nil {
		return err
	}
	if b.overflow == nil {
		b.overflow = make(map[NodeID]successfulOutcome)
	}
	b.overflow[id] = v
	return nil
}
