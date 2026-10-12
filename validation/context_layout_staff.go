package validation

import (
	"math/big"
	"sort"
)

// Sparse observed staff selectors. The encoded staff count is never expanded.
type layoutStaffBucket struct {
	Number  Evidence[Atomic]
	Nodes   []*factNode
	Unknown bool
}

func (l *layoutScopes) readStaffLayouts(owner *factNode, buckets map[string]layoutStaffBucket) error {
	if owner == nil {
		return nil
	}
	r := l.r
	complete := r.enumerated(owner)
	if complete.State != EvidenceKnown || !complete.Value {
		l.staffUnknown = true
	}
	for _, n := range owner.children {
		if !r.step(1) {
			return r.err
		}
		if !r.matches(n, layoutRole("staff-layout")) {
			continue
		}
		number := localRawAttribute(r, n, layoutRole("staff-layout.@number"))
		if number.State == EvidenceAbsent {
			number = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger, Integer: localOne}}
		}
		if number.State != EvidenceKnown {
			l.staffUnknown = true
			continue
		}
		if !r.step(1 + number.Value.Integer.ByteSize()) {
			return r.err
		}
		k := number.Value.Integer.String()
		bucket := buckets[k]
		if err := r.s.retain(768 + int64(len(k))); err != nil {
			return err
		}
		bucket.Number = number
		bucket.Nodes = append(bucket.Nodes, n)
		buckets[k] = bucket
	}
	return r.err
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
	print, _ := l.child(b.content, "print")
	if b.part.State != EvidenceKnown {
		return nil
	}
	part := b.part.Value.Node
	local, ok := l.staffOverrides[part]
	if !ok {
		if err := r.s.retain(512); err != nil {
			return err
		}
		local = make(map[string]layoutStaffBucket)
		l.staffOverrides[part] = local
	}
	if err := l.readStaffLayouts(print, local); err != nil {
		return err
	}
	if err := r.s.retain(512 + int64(len(l.staffDefaults)+len(local))*128); err != nil {
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
	// Deterministic publication order is numeric-independent and does not expand
	// gaps between sparse identifiers; score semantics never rely on map order.
	order := make([]string, 0, len(keys))
	for k := range keys {
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
		origin := (*factNode)(nil)
		if len(bucket.Nodes) > 0 {
			origin = bucket.Nodes[0]
		}
		if value.State == EvidenceAbsent {
			value = l.staffConsensus(def)
			number = def.Number
			if len(def.Nodes) > 0 {
				origin = def.Nodes[0]
			}
		}
		if !frame.systemCertain && len(bucket.Nodes) > 0 && raw.State != EvidenceAbsent {
			value = stateUnknown[Atomic]("staff-override-lifetime-unavailable")
		}
		if l.staffUnknown {
			value = stateUnknown[Atomic]("staff-layout-selector-unavailable")
		}
		active := stateUnknown[bool]("first-visible-staff-unavailable")
		if first.State == EvidenceKnown && number.State == EvidenceKnown && frame.system.State == EvidenceKnown {
			active = Evidence[bool]{State: EvidenceKnown, Value: !(first.Value.Part == b.part.Value && k == first.Value.Staff.String())}
		}
		ref := NodeRef{r.s.documentID, b.measure.header.ID}
		from := ref
		if origin != nil {
			from.Node = origin.header.ID
		}
		payload := &LayoutContextResult{Scope: ref, Origin: from, Raw: raw, Applicability: active, Unit: ContextUnitTenths, Page: frame.page, System: frame.system, Staff: stateInteger(number), Provenance: ContextInherited, Basis: "previous-system-staff-bottom-to-current-staff-top"}
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
			if spacing.State != EvidenceKnown {
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
