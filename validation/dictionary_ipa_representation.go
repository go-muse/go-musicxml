package validation

import (
	"context"
	"slices"
	"unicode/utf8"
)

// This index is derived only after the exact manifest's typed-data digest passes.
// Its finite bounds describe the reviewed data, never a limit on user text.
type ipaRepresentationScalar struct {
	scalar rune
	leaves [3]rune
	count  uint8
	ccc    uint8 // Set only on identity leaves; decomposition keys have no leaf CCC.
	mark   bool
}
type ipaRepresentationAtom struct {
	leaves [3]rune
	count  uint8
	base   bool
}
type ipaRepresentationData struct {
	scalars                [436]ipaRepresentationScalar
	atoms                  [248]ipaRepresentationAtom
	scalarCount, atomCount int
}

const ipaRepresentationIndexBytes int64 = 32768

func (d *DictionarySet) buildIPARepresentation(ctx context.Context, e *dictionaryEntry) error {
	if err := d.retain(ipaRepresentationIndexBytes); err != nil {
		return err
	}
	p := &ipaRepresentationData{}
	add := func(cp rune, leaves [3]rune, count uint8) error {
		for i := 0; i < p.scalarCount; i++ {
			if err := dictionaryStep(ctx, d.account, 1); err != nil {
				return err
			}
			if p.scalars[i].scalar == cp {
				return nil
			}
		}
		if p.scalarCount == len(p.scalars) {
			return dictionaryConfiguration("excessive representation support")
		}
		p.scalars[p.scalarCount] = ipaRepresentationScalar{scalar: cp, leaves: leaves, count: count}
		p.scalarCount++
		return nil
	}
	for _, row := range e.ipa.Equivalences {
		cp, w := utf8.DecodeRuneInString(row.Encoding)
		if w != len(row.Encoding) {
			return dictionaryConfiguration("representation decomposition key is not scalar")
		}
		var leaves [3]rune
		count := 0
		for _, r := range row.EquivalentToEncoding {
			if err := dictionaryStep(ctx, d.account, 1); err != nil {
				return err
			}
			if count == len(leaves) {
				return dictionaryConfiguration("excessive representation decomposition")
			}
			leaves[count] = r
			count++
		}
		if count == 0 || (count == 1 && leaves[0] == cp) {
			return dictionaryConfiguration("identity representation decomposition")
		}
		if err := add(cp, leaves, uint8(count)); err != nil {
			return err
		}
	}
	// Full recursive decompositions contain identity leaves only. A key that
	// occurs in any RHS would violate the sealed one-step NFD representation.
	for _, row := range e.ipa.Equivalences {
		for _, cp := range row.EquivalentToEncoding {
			if err := add(cp, [3]rune{cp}, 1); err != nil {
				return err
			}
		}
	}
	for _, row := range e.ipa.Symbols {
		for _, cp := range row.Encoding {
			if err := add(cp, [3]rune{cp}, 1); err != nil {
				return err
			}
		}
	}
	for _, pair := range ipaRepresentationHalfPairs {
		for _, cp := range [...]rune{pair.left, pair.right} {
			if err := add(cp, [3]rune{cp}, 1); err != nil {
				return err
			}
		}
	}
	if p.scalarCount != len(p.scalars) {
		return dictionaryConfiguration("incomplete representation support")
	}
	var sortErr error
	slices.SortFunc(p.scalars[:], func(x, y ipaRepresentationScalar) int {
		if sortErr != nil {
			return 0
		}
		sortErr = dictionaryStep(ctx, d.account, 1)
		if x.scalar < y.scalar {
			return -1
		}
		if x.scalar > y.scalar {
			return 1
		}
		return 0
	})
	if sortErr != nil {
		return sortErr
	}
	for i := range p.scalars {
		row := &p.scalars[i]
		// The finite CCC policy describes normalized leaves, not raw keys such
		// as U+0340/U+0341/U+0344. A key's own Unicode CCC is never consumed.
		if row.count == 1 && row.leaves[0] == row.scalar {
			ccc, err := ipaRepresentationCCC(ctx, d.account, row.scalar)
			if err != nil {
				return err
			}
			row.ccc = ccc
		}
		for j := uint8(0); j < row.count; j++ {
			leaf, err := p.lookup(ctx, d.account, row.leaves[j])
			if err != nil {
				return err
			}
			if leaf == nil || leaf.count != 1 || leaf.leaves[0] != leaf.scalar {
				return dictionaryConfiguration("nonrecursive representation decomposition required")
			}
		}
	}
	for _, row := range e.ipa.Symbols {
		var atom ipaRepresentationAtom
		for _, cp := range row.Encoding {
			scalar, err := p.lookup(ctx, d.account, cp)
			if err != nil {
				return err
			}
			if scalar == nil {
				return dictionaryConfiguration("missing representation scalar")
			}
			for j := uint8(0); j < scalar.count; j++ {
				if err := dictionaryStep(ctx, d.account, 1); err != nil {
					return err
				}
				if int(atom.count) == len(atom.leaves) {
					return dictionaryConfiguration("excessive representation atom")
				}
				atom.leaves[atom.count] = scalar.leaves[j]
				atom.count++
			}
		}
		// Reviewed atoms have at most two normalized leaves (one mark). The
		// three-leaf structural bound also admits a synthetic multi-mark control;
		// insertion sorting has a finite comparison bound and charged moves.
		for i := 1; i < int(atom.count); i++ {
			for j := i; j > 0; j-- {
				left, err := p.lookup(ctx, d.account, atom.leaves[j-1])
				if err != nil {
					return err
				}
				right, err := p.lookup(ctx, d.account, atom.leaves[j])
				if err != nil {
					return err
				}
				if left.ccc == 0 || right.ccc == 0 || left.ccc <= right.ccc {
					break
				}
				if err := dictionaryStep(ctx, d.account, 1); err != nil {
					return err
				}
				atom.leaves[j-1], atom.leaves[j] = atom.leaves[j], atom.leaves[j-1]
			}
		}
		first, err := p.lookup(ctx, d.account, atom.leaves[0])
		if err != nil {
			return err
		}
		switch row.Category {
		case "mark":
			if atom.count != 1 || first.ccc == 0 {
				return dictionaryConfiguration("invalid independent representation mark")
			}
			first.mark = true
			continue
		case "letter", "modifier-letter", "constructed-letter":
			atom.base = true
		case "sign":
		default:
			return dictionaryConfiguration("invalid representation role")
		}
		if first.ccc != 0 || p.atomCount == len(p.atoms) {
			return dictionaryConfiguration("invalid representation starter")
		}
		for j := uint8(1); j < atom.count; j++ {
			v, err := p.lookup(ctx, d.account, atom.leaves[j])
			if err != nil {
				return err
			}
			if v.ccc == 0 {
				return dictionaryConfiguration("multiple representation starters")
			}
		}
		p.atoms[p.atomCount] = atom
		p.atomCount++
	}
	if p.atomCount != len(p.atoms) {
		return dictionaryConfiguration("incomplete representation atoms")
	}
	slices.SortFunc(p.atoms[:], func(x, y ipaRepresentationAtom) int {
		if sortErr != nil {
			return 0
		}
		sortErr = dictionaryStep(ctx, d.account, 1)
		if x.leaves[0] < y.leaves[0] {
			return -1
		}
		if x.leaves[0] > y.leaves[0] {
			return 1
		}
		return 0
	})
	if sortErr != nil {
		return sortErr
	}
	e.representation = p
	return nil
}
func (p *ipaRepresentationData) lookup(ctx context.Context, a *Account, cp rune) (*ipaRepresentationScalar, error) {
	lo, hi := 0, p.scalarCount
	for lo < hi {
		if err := dictionaryStep(ctx, a, 1); err != nil {
			return nil, err
		}
		mid := lo + (hi-lo)/2
		v := &p.scalars[mid]
		if v.scalar < cp {
			lo = mid + 1
		} else if v.scalar > cp {
			hi = mid
		} else {
			return v, nil
		}
	}
	return nil, nil
}

type ipaRepresentationLeaf struct {
	scalar rune
	ccc    uint8
	mark   bool
	// Internal, test-observable provenance through decomposition and sorting.
	// Public projection uses only the unchanged whole-input span, not these
	// intervals, which can be discontiguous for a normalized atom.
	start, end uint64
}
type ipaRepresentationNormalization struct {
	leaves   []ipaRepresentationLeaf
	spare    []ipaRepresentationLeaf
	account  *Account
	retained int64
}

func (n *ipaRepresentationNormalization) close() {
	if n != nil && n.account != nil {
		_ = n.account.Release(BudgetBufferedBytes, n.retained)
		n.account = nil
		n.leaves = nil
		n.spare = nil
		n.retained = 0
	}
}

func ipaRepresentationNormalize(ctx context.Context, a *Account, p *ipaRepresentationData, text string) (_ *ipaRepresentationNormalization, known bool, err error) {
	if err = dictionaryStep(ctx, a, 1); err != nil {
		return nil, false, err
	}
	if err = a.Check(BudgetScalarBytes, int64(len(text))); err != nil {
		return nil, false, err
	}
	if p == nil {
		return nil, false, dictionaryConfiguration("missing representation index")
	}
	count := int64(0)
	// Admission scans raw scalars, not host-normalized text. Unsupported or invalid
	// UTF-8 cannot enter via a newer Unicode implementation.
	for start := 0; start < len(text); {
		if err = dictionaryStep(ctx, a, 1); err != nil {
			return nil, false, err
		}
		cp, w := utf8.DecodeRuneInString(text[start:])
		if cp == utf8.RuneError && w == 1 {
			return nil, false, nil
		}
		row, e := p.lookup(ctx, a, cp)
		if e != nil {
			return nil, false, e
		}
		if row == nil {
			return nil, false, nil
		}
		count, err = rationalCheckedAdd(BudgetBufferedBytes, count, int64(row.count))
		if err != nil {
			return nil, false, err
		}
		start += w
	}
	// Two independent buffers, bounded slice/control headers and the 256-entry
	// counting workspace. Charge their simultaneous lifetime before allocating.
	size, e := rationalCheckedMul(BudgetBufferedBytes, count, 64)
	if e != nil {
		return nil, false, e
	}
	size, e = rationalCheckedAdd(BudgetBufferedBytes, size, 4096)
	if e != nil {
		return nil, false, e
	}
	if _, err = allocationSize(size); err != nil {
		return nil, false, err
	}
	length, e := allocationSize(count)
	if e != nil {
		return nil, false, e
	}
	if err = a.Charge(BudgetBufferedBytes, size); err != nil {
		return nil, false, err
	}
	n := &ipaRepresentationNormalization{leaves: make([]ipaRepresentationLeaf, length), spare: make([]ipaRepresentationLeaf, length), account: a, retained: size}
	defer func() {
		if err != nil {
			n.close()
		}
	}()
	at := 0
	for start := 0; start < len(text); {
		if err = dictionaryStep(ctx, a, 1); err != nil {
			return nil, false, err
		}
		cp, w := utf8.DecodeRuneInString(text[start:])
		row, e := p.lookup(ctx, a, cp)
		if e != nil {
			err = e
			return nil, false, err
		}
		for i := uint8(0); i < row.count; i++ {
			if err = dictionaryStep(ctx, a, 1); err != nil {
				return nil, false, err
			}
			leaf, e := p.lookup(ctx, a, row.leaves[i])
			if e != nil {
				err = e
				return nil, false, err
			}
			n.leaves[at] = ipaRepresentationLeaf{scalar: leaf.scalar, ccc: leaf.ccc, mark: leaf.mark, start: uint64(start), end: uint64(start + w)}
			at++
		}
		start += w
	}
	for start := 0; start < len(n.leaves); {
		if err = dictionaryStep(ctx, a, 1); err != nil {
			return nil, false, err
		}
		marks := start
		if n.leaves[start].ccc == 0 {
			n.spare[start] = n.leaves[start]
			marks++
		}
		end := marks
		for end < len(n.leaves) && n.leaves[end].ccc != 0 {
			if err = dictionaryStep(ctx, a, 1); err != nil {
				return nil, false, err
			}
			end++
		}
		if end-marks <= 8 {
			// Short runs use stable insertion sorting, avoiding the fixed 256-bucket
			// cost for ordinary single-mark atoms. Copies, comparisons and moves
			// are charged; the fixed eight-mark bound keeps this branch linear in
			// the number of segments. Equal-CCC marks never exchange places.
			for i := marks; i < end; i++ {
				if err = dictionaryStep(ctx, a, 1); err != nil {
					return nil, false, err
				}
				n.spare[i] = n.leaves[i]
				for j := i; j > marks; j-- {
					if err = dictionaryStep(ctx, a, 1); err != nil {
						return nil, false, err
					}
					if n.spare[j-1].ccc <= n.spare[j].ccc {
						break
					}
					if err = dictionaryStep(ctx, a, 1); err != nil {
						return nil, false, err
					}
					n.spare[j-1], n.spare[j] = n.spare[j], n.spare[j-1]
				}
			}
		} else {
			// Long runs retain a linear counting sort with charged clearing and
			// prefix summation, independent of the input's original order.
			if err = dictionaryStep(ctx, a, 256); err != nil {
				return nil, false, err
			}
			var offsets [256]int
			for i := marks; i < end; i++ {
				if err = dictionaryStep(ctx, a, 1); err != nil {
					return nil, false, err
				}
				offsets[n.leaves[i].ccc]++
			}
			pos := marks
			for c := range offsets {
				if err = dictionaryStep(ctx, a, 1); err != nil {
					return nil, false, err
				}
				v := offsets[c]
				offsets[c] = pos
				pos += v
			}
			for i := marks; i < end; i++ {
				if err = dictionaryStep(ctx, a, 1); err != nil {
					return nil, false, err
				}
				leaf := n.leaves[i]
				n.spare[offsets[leaf.ccc]] = leaf
				offsets[leaf.ccc]++
			}
		}
		start = end
	}
	n.leaves, n.spare = n.spare, n.leaves
	return n, true, nil
}

func ipaRepresentationWitness(ctx context.Context, a *Account, p *ipaRepresentationData, text string) (bool, error) {
	n, known, err := ipaRepresentationNormalize(ctx, a, p, text)
	if err != nil || !known {
		return false, err
	}
	defer n.close()
	pending := rune(0)
	previousWholeTie := false
	for start := 0; start < len(n.leaves); {
		if err := dictionaryStep(ctx, a, 1); err != nil {
			return false, err
		}
		marks := start
		starter := rune(0)
		if n.leaves[start].ccc == 0 {
			starter = n.leaves[start].scalar
			marks++
		}
		end := marks
		for end < len(n.leaves) && n.leaves[end].ccc != 0 {
			if err := dictionaryStep(ctx, a, 1); err != nil {
				return false, err
			}
			end++
		}
		half := rune(0)
		wholeTie := false
		for i := marks; i < end; i++ {
			if err := dictionaryStep(ctx, a, 1); err != nil {
				return false, err
			}
			cp := n.leaves[i].scalar
			if ipaRepresentationWholeTie(cp) {
				wholeTie = true
			}
			if ipaRepresentationHalf(cp) {
				if half != 0 {
					return false, nil
				}
				half = cp
			}
		}
		base := false
		matched := false
		if starter == 0 {
			matched = true
			for i := marks; i < end; i++ {
				if err := dictionaryStep(ctx, a, 1); err != nil {
					return false, err
				}
				if !n.leaves[i].mark {
					matched = false
					break
				}
			}
		} else {
			lo, hi := 0, p.atomCount
			for lo < hi {
				if err := dictionaryStep(ctx, a, 1); err != nil {
					return false, err
				}
				mid := lo + (hi-lo)/2
				if p.atoms[mid].leaves[0] < starter {
					lo = mid + 1
				} else {
					hi = mid
				}
			}
			for i := lo; i < p.atomCount && p.atoms[i].leaves[0] == starter; i++ {
				if err := dictionaryStep(ctx, a, 1); err != nil {
					return false, err
				}
				ok, err := ipaRepresentationCandidate(ctx, a, p, p.atoms[i], n.leaves[marks:end])
				if err != nil {
					return false, err
				}
				if ok {
					matched = true
					base = base || p.atoms[i].base
				}
			}
		}
		if !matched {
			return false, nil
		}
		if pending != 0 {
			if !base || half != pending || wholeTie {
				return false, nil
			}
			pending = 0
		} else if half != 0 {
			if !base || wholeTie || previousWholeTie {
				return false, nil
			}
			for _, pair := range ipaRepresentationHalfPairs {
				if half == pair.left {
					pending = pair.right
					break
				}
			}
			if pending == 0 {
				return false, nil
			}
		}
		previousWholeTie = wholeTie
		start = end
	}
	return pending == 0, nil
}
func ipaRepresentationHalf(cp rune) bool {
	for _, pair := range ipaRepresentationHalfPairs {
		if cp == pair.left || cp == pair.right {
			return true
		}
	}
	return false
}
func ipaRepresentationWholeTie(cp rune) bool {
	for _, pair := range ipaRepresentationHalfPairs {
		if cp == pair.whole {
			return true
		}
	}
	return false
}
func ipaRepresentationCandidate(ctx context.Context, a *Account, p *ipaRepresentationData, atom ipaRepresentationAtom, marks []ipaRepresentationLeaf) (bool, error) {
	required := uint8(1)
	for _, mark := range marks {
		if err := dictionaryStep(ctx, a, 1); err != nil {
			return false, err
		}
		if ipaRepresentationHalf(mark.scalar) {
			continue
		}
		if required < atom.count {
			need, err := p.lookup(ctx, a, atom.leaves[required])
			if err != nil {
				return false, err
			}
			if need.ccc < mark.ccc {
				return false, nil
			}
			if need.ccc == mark.ccc {
				if need.scalar != mark.scalar {
					return false, nil
				}
				required++
				continue
			}
		}
		if !mark.mark {
			return false, nil
		}
	}
	return required == atom.count, nil
}
