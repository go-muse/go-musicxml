package validation

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
func (l *layoutScopes) publishStaffLayouts(b timelineMeasureBlock, first Evidence[NodeRef], frame layoutFrame) error {
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
			active = Evidence[bool]{State: EvidenceKnown, Value: !(first.Value == b.part.Value && k == "1")}
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
