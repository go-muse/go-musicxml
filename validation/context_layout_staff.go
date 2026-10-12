package validation

import (
	"math/big"
	"sort"
)

// Sparse observed staff selectors. The encoded staff count is never expanded.
type layoutStaffBucket struct {
	Number Evidence[Atomic]
	Nodes  []*factNode
	Epoch  uint64
}

func (l *layoutScopes) readStaffLayouts(owner *factNode, buckets map[string]layoutStaffBucket, scratch *int64) (Evidence[bool], error) {
	gap := Evidence[bool]{State: EvidenceAbsent}
	if owner == nil {
		return gap, nil
	}
	r := l.r
	complete := r.enumerated(owner)
	if complete.State != EvidenceKnown || !complete.Value {
		gap = timelineGapAs[bool, bool](complete, "staff-layout-selector-unavailable")
	}
	for _, n := range owner.children {
		if !r.step(1) {
			return gap, r.err
		}
		if !r.matches(n, layoutRole("staff-layout")) {
			continue
		}
		number := localRawAttribute(r, n, layoutRole("staff-layout.@number"))
		if number.State == EvidenceAbsent {
			number = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger, Integer: localOne}}
		}
		if number.State != EvidenceKnown {
			gap = layoutMergeSelection(gap, stateUnavailable[Atomic, bool](number))
			continue
		}
		if number.Value.Kind != AtomicInteger || !number.Value.Integer.Valid() {
			gap = layoutMergeSelection(gap, stateUnknown[bool]("staff-layout-selector-unavailable"))
			continue
		}
		if !r.step(1 + number.Value.Integer.ByteSize()) {
			return gap, r.err
		}
		// Reserve the selector's decimal string before materializing it. Three
		// bytes per integer byte conservatively cover base-10 digits.
		bytes, err := rationalCheckedMul(BudgetBufferedBytes, number.Value.Integer.ByteSize(), 3)
		if err != nil {
			return gap, err
		}
		bytes, err = rationalCheckedAdd(BudgetBufferedBytes, 768, bytes)
		if err != nil {
			return gap, err
		}
		if err := layoutStaffReserve(r.s, scratch, bytes); err != nil {
			return gap, err
		}
		k := number.Value.Integer.String()
		bucket := buckets[k]
		bucket.Number = number
		// Fresh explicit publishers establish current-system applicability, but do
		// not resolve competing statements retained for that same observed selector.
		if bucket.Epoch != l.staffEpoch {
			bucket.Nodes = nil
		}
		bucket.Epoch = l.staffEpoch
		bucket.Nodes = append(bucket.Nodes, n)
		buckets[k] = bucket
	}
	return gap, r.err
}

func (l *layoutScopes) staffConsensus(bucket layoutStaffBucket) Evidence[Atomic] {
	result := Evidence[Atomic]{State: EvidenceAbsent}
	for _, n := range bucket.Nodes {
		if !l.r.step(1) {
			return stateUnknown[Atomic]("context-interrupted")
		}
		value := l.scalar(n, "staff-distance")
		if !l.r.step(atomicLogicalBytes(value.Value) + atomicLogicalBytes(result.Value)) {
			return stateUnknown[Atomic]("context-interrupted")
		}
		result = layoutConsensusPair(result, value)
	}
	return result
}
func (l *layoutScopes) publishStaffLayouts(b timelineMeasureBlock, first Evidence[layoutStaffRef], frame layoutFrame) error {
	r := l.r
	if !r.step(1) {
		return r.err
	}
	var scratch int64
	defer func() { _ = r.s.account.Release(BudgetBufferedBytes, scratch) }()
	var localScratch *int64
	if b.part.State != EvidenceKnown {
		localScratch = &scratch
	}
	print, selected := l.child(b.content, "print")
	part := b.part.Value.Node
	local, ok := l.staffOverrides[part]
	if b.part.State != EvidenceKnown {
		local, ok = nil, false
	}
	if !ok {
		if err := layoutStaffReserve(r.s, localScratch, 512); err != nil {
			return err
		}
		local = make(map[string]layoutStaffBucket)
		if b.part.State == EvidenceKnown {
			l.staffOverrides[part] = local
		}
	}
	gap, err := l.readStaffLayouts(print, local, localScratch)
	if err != nil {
		return err
	}
	if selected.State != EvidenceKnown && selected.State != EvidenceAbsent {
		gap = layoutMergeSelection(gap, selected)
		// A repeated print selection is unresolved, but its observed sparse
		// staff identities are still useful subjects for unavailable rows.
		for _, candidate := range b.content.children {
			if !r.step(1) {
				return r.err
			}
			if r.matches(candidate, layoutRole("print")) {
				more, err := l.readStaffLayouts(candidate, local, localScratch)
				if err != nil {
					return err
				}
				gap = layoutMergeSelection(gap, more)
			}
		}
	}
	if gap.State != EvidenceAbsent && b.part.State == EvidenceKnown {
		if l.staffGaps == nil {
			if err := r.s.retain(512); err != nil {
				return err
			}
			l.staffGaps = make(map[NodeID]Evidence[bool])
		}
		if _, found := l.staffGaps[part]; !found {
			if err := r.s.retain(256); err != nil {
				return err
			}
		}
		previous := l.staffGaps[part]
		if previous.State == "" {
			previous.State = EvidenceAbsent
		}
		l.staffGaps[part] = layoutMergeSelection(previous, gap)
	}
	partGap := gap
	if b.part.State == EvidenceKnown {
		partGap = l.staffGaps[part]
	} else {
		partGap = layoutMergeSelection(partGap, stateUnavailable[NodeRef, bool](b.part))
	}
	keyCount, err := rationalCheckedAdd(BudgetBufferedBytes, int64(len(l.staffDefaults)), int64(len(local)))
	if err != nil {
		return err
	}
	keyBytes, err := rationalCheckedMul(BudgetBufferedBytes, keyCount, 128)
	if err != nil {
		return err
	}
	keyBytes, err = rationalCheckedAdd(BudgetBufferedBytes, 512, keyBytes)
	if err != nil {
		return err
	}
	if err := layoutStaffReserve(r.s, &scratch, keyBytes); err != nil {
		return err
	}
	keys := make(map[string]bool)
	for k := range l.staffDefaults {
		if !r.step(1 + int64(len(k))) {
			return r.err
		}
		keys[k] = true
	}
	for k := range local {
		if !r.step(1 + int64(len(k))) {
			return r.err
		}
		keys[k] = true
	}
	// An unresolved selector still denotes an observed layout statement. Do
	// not make it disappear merely because no valid selector/default exists.
	selectorGap := layoutMergeSelection(partGap, l.staffDefaultGap)
	if len(keys) == 0 && selectorGap.State != "" && selectorGap.State != EvidenceAbsent && selectorGap.State != EvidenceKnown {
		if r.err != nil {
			return r.err
		}
		ref := NodeRef{r.s.documentID, b.measure.header.ID}
		from := ref
		if print != nil {
			from.Node = print.header.ID
		}
		return r.s.publishContextResult(r.ctx, ContextResult{Provider: l.key, Subject: ref, Property: "layout.staff-distance", Value: stateUnavailable[bool, Atomic](selectorGap), Layout: &LayoutContextResult{Conversion: Evidence[Rational]{State: EvidenceAbsent}, Part: b.part, Scope: ref, Origin: from, Raw: stateUnavailable[bool, Atomic](selectorGap), Applicability: stateUnavailable[bool, bool](selectorGap), Page: frame.page, System: frame.system, Staff: stateUnavailable[bool, Integer](selectorGap), Unit: ContextUnitTenths, Provenance: ContextExplicit, Basis: "previous-system-staff-bottom-to-current-staff-top"}})
	}
	// Deterministic publication order is numeric-independent and does not expand
	// gaps between sparse identifiers; score semantics never rely on map order.
	order := make([]string, 0, len(keys))
	for k := range keys {
		if !r.step(1) {
			return r.err
		}
		order = append(order, k)
	}
	if err := layoutSortStrings(r, order); err != nil {
		return err
	}
	for _, k := range order {
		bucket := local[k]
		def := l.staffDefaults[k]
		raw := l.staffConsensus(bucket)
		value := raw
		number := bucket.Number
		if number.State == "" {
			number = def.Number
		}
		origin := (*factNode)(nil)
		if len(bucket.Nodes) > 0 {
			origin = bucket.Nodes[0]
		}
		if value.State == EvidenceAbsent {
			value = l.staffConsensus(def)
			if number.State == "" {
				number = def.Number
			}
			if len(def.Nodes) > 0 {
				origin = def.Nodes[0]
			}
		}
		if bucket.Epoch != l.staffEpoch && len(bucket.Nodes) > 0 && raw.State != EvidenceAbsent {
			value = layoutLifetimeValue[Atomic](frame.system, "staff-override-lifetime-unavailable")
		}
		if raw.State == EvidenceAbsent {
			value = layoutSelectionValue(value, l.staffDefaultGap)
		}
		value = layoutSelectionValue(value, partGap)
		if number.State == "" {
			number = Evidence[Atomic]{State: EvidenceAbsent}
		}
		active := Evidence[bool]{State: EvidenceKnown, Value: true}
		for _, prerequisite := range []Evidence[bool]{stateUnavailable[layoutStaffRef, bool](first), stateUnavailable[Atomic, bool](number), stateUnavailable[NodeRef, bool](frame.system), stateUnavailable[NodeRef, bool](b.part)} {
			if prerequisite.State == EvidenceKnown {
				continue
			}
			if prerequisite.State == EvidenceAbsent {
				prerequisite = stateUnknown[bool]("first-visible-staff-unavailable")
			}
			active = timelineMergeAvailability(active, prerequisite)
		}
		if active.State == EvidenceKnown {
			active.Value = !(first.Value.Part == b.part.Value && k == first.Value.Staff.String())
		}
		ref := NodeRef{r.s.documentID, b.measure.header.ID}
		from := ref
		if origin != nil {
			from.Node = origin.header.ID
		}
		if r.err != nil {
			return r.err
		}
		payload := &LayoutContextResult{Conversion: Evidence[Rational]{State: EvidenceAbsent}, Part: b.part, Scope: ref, Origin: from, Raw: raw, Applicability: active, Unit: ContextUnitTenths, Page: frame.page, System: frame.system, Staff: stateInteger(number), Provenance: ContextInherited, Basis: "previous-system-staff-bottom-to-current-staff-top"}
		if err := r.s.publishContextResult(r.ctx, ContextResult{Provider: l.key, Subject: ref, Property: "layout.staff-distance", Value: value, Layout: payload}); err != nil {
			return err
		}
	}
	return r.err
}

// The first visible publisher and the first occupied layout slot are different
// facts: an invisible staff can retain space without supplying system-layout.
type layoutStaffRef struct {
	Part  NodeRef
	Staff Integer
}

func layoutStaffPart(v Evidence[layoutStaffRef]) Evidence[NodeRef] {
	if v.State != EvidenceKnown {
		return stateUnavailable[layoutStaffRef, NodeRef](v)
	}
	return Evidence[NodeRef]{State: EvidenceKnown, Value: v.Value.Part}
}
func (l *layoutScopes) firstSystemStaff(blocks []timelineMeasureBlock, slots bool) (Evidence[layoutStaffRef], error) {
	r := l.r
	if l.timeline == nil || !l.timeline.Final {
		return stateUnknown[layoutStaffRef]("staff-visibility-state-unavailable"), nil
	}
	for _, b := range blocks {
		if !r.step(1) {
			return Evidence[layoutStaffRef]{}, r.err
		}
		if b.part.State != EvidenceKnown {
			return stateUnavailable[NodeRef, layoutStaffRef](b.part), nil
		}
		pos := musicalPosition{Part: b.part.Value, Measure: NodeRef{r.s.documentID, b.measure.header.ID}, MeasureOrdinal: b.ordinal, Offset: timelineZero}
		count, err := l.timeline.State.effectiveStaves(r.ctx, r.s, pos)
		if err != nil {
			return Evidence[layoutStaffRef]{}, err
		}
		if count.Value.State != EvidenceKnown {
			return stateUnavailable[Integer, layoutStaffRef](count.Value), nil
		}
		candidates, err := l.staffCandidates(b.part.Value, count.Value.Value)
		if err != nil {
			return Evidence[layoutStaffRef]{}, err
		}
		for _, staff := range candidates {
			details, err := l.timeline.State.effectiveStaffDetails(r.ctx, r.s, pos, Evidence[Integer]{State: EvidenceKnown, Value: staff})
			if err != nil {
				return Evidence[layoutStaffRef]{}, err
			}
			result := Evidence[layoutStaffRef]{State: EvidenceKnown, Value: layoutStaffRef{b.part.Value, staff}}
			if details.Value.State == EvidenceAbsent {
				return result, nil
			}
			if details.Value.State != EvidenceKnown {
				return stateUnavailable[staffDetailsState, layoutStaffRef](details.Value), nil
			}
			visible := details.Value.Value.effectivePrintObject().Effective
			spacing := details.Value.Value.effectivePrintSpacing().Effective
			if visible.State != EvidenceKnown {
				return stateUnavailable[bool, layoutStaffRef](visible), nil
			}
			if visible.Value {
				return result, nil
			}
			if slots && spacing.State != EvidenceKnown {
				return stateUnavailable[bool, layoutStaffRef](spacing), nil
			}
			if slots && spacing.Value {
				return result, nil
			}
		}
	}
	return Evidence[layoutStaffRef]{State: EvidenceAbsent}, nil
}
func (l *layoutScopes) staffCandidates(part NodeRef, count Integer) ([]Integer, error) {
	r := l.r
	if !r.step(1 + count.ByteSize()) {
		return nil, r.err
	}
	cmp, err := count.Cmp(localOne)
	if err != nil {
		return nil, err
	}
	if cmp < 0 {
		return nil, nil
	}
	if l.candidateCache == nil {
		if err := r.s.retain(1024); err != nil {
			return nil, err
		}
		l.candidateCache = make(map[NodeID][]Integer)
		buckets := make(map[NodeID]map[string]Integer)
		for _, p := range l.timeline.State.StaffDetails.Publications {
			if !r.step(1 + p.Scope.Staff.ByteSize()) {
				return nil, r.err
			}
			if p.Scope.AllStaves || !p.Scope.Staff.Valid() {
				continue
			}
			part := p.Position.Part.Node
			bucket := buckets[part]
			if bucket == nil {
				if err := r.s.retain(512); err != nil {
					return nil, err
				}
				bucket = map[string]Integer{"1": localOne}
				buckets[part] = bucket
			}
			if err := r.s.retain(768 + p.Scope.Staff.ByteSize()*3); err != nil {
				return nil, err
			}
			bucket[p.Scope.Staff.String()] = p.Scope.Staff
			next := new(big.Int).Add(p.Scope.Staff.value, big.NewInt(1))
			n, err := NewInteger(next)
			if err != nil {
				return nil, err
			}
			bucket[n.String()] = n
		}
		for part, bucket := range buckets {
			if !r.step(1) {
				return nil, r.err
			}
			if err := r.s.retain(256 + int64(len(bucket))*64); err != nil {
				return nil, err
			}
			values := make([]Integer, 0, len(bucket))
			for _, v := range bucket {
				if !r.step(1) {
					return nil, r.err
				}
				values = append(values, v)
			}
			var err error
			sort.Slice(values, func(i, j int) bool {
				if err != nil {
					return false
				}
				err = r.s.step(r.ctx, 1+values[i].ByteSize()+values[j].ByteSize())
				if err != nil {
					return false
				}
				cmp, e := values[i].Cmp(values[j])
				err = e
				return cmp < 0
			})
			if err != nil {
				return nil, err
			}
			l.candidateCache[part] = values
		}
	}
	values := l.candidateCache[part.Node]
	if len(values) == 0 {
		return layoutStaffOneCandidates, nil
	}
	// Sorted sparse identities are sliced at the current count; a later encoded
	// staff increase reuses the same index without pre-expanding that count.
	end := 0
	for end < len(values) {
		if !r.step(1 + values[end].ByteSize() + count.ByteSize()) {
			return nil, r.err
		}
		cmp, err := values[end].Cmp(count)
		if err != nil {
			return nil, err
		}
		if cmp > 0 {
			break
		}
		end++
	}
	return values[:end], nil
}

var layoutStaffOneCandidates = []Integer{localOne}

// Known-part buckets survive this call. Unknown-part buckets and the selection
// maps are scratch, held until all publications have copied their payloads.
func layoutStaffReserve(s *Session, scratch *int64, bytes int64) error {
	if scratch == nil {
		return s.retain(bytes)
	}
	if err := s.account.Charge(BudgetBufferedBytes, bytes); err != nil {
		return err
	}
	*scratch += bytes
	return nil
}
