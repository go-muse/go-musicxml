package validation

import "context"

// Proof indexes borrow admitted values and live only for one provider run.
// Keys preserve canonical document/part identity and exact source order.
type contextRepeatMeasureKey struct {
	part    NodeRef
	ordinal uint64
}
type contextRepeatProofIndex struct {
	events        map[NodeRef][]NodeID
	rows          map[NodeRef]int
	ends          map[contextRepeatMeasureKey]Evidence[musicalPosition]
	closures      map[NodeRef]stateClosure
	coverage      map[NodeRef]Evidence[bool]
	unknown       Evidence[bool]
	all           Evidence[bool]
	parts         map[NodeRef][]contextRepeatNote
	scopes        map[contextRepeatProofScopeKey]*contextRepeatProofScope
	releases      []func()
	endGaps       map[NodeRef]Evidence[bool]
	coverageOrder map[NodeRef]int
	unknownOrder  int
}

func contextRepeatScratch(ctx context.Context, s *Session, count int, size int64) (func(), error) {
	if err := s.step(ctx, 1); err != nil {
		return nil, err
	}
	bytes, err := rationalCheckedMul(BudgetBufferedBytes, int64(count), size)
	if err != nil {
		return nil, err
	}
	if _, err = AllocationSize(bytes); err != nil {
		return nil, err
	}
	if err = s.account.Charge(BudgetBufferedBytes, bytes); err != nil {
		return nil, err
	}
	return func() { _ = s.account.Release(BudgetBufferedBytes, bytes) }, nil
}
func contextRepeatBuildProofIndex(r *contextReader, timeline *timelineService, rows []contextRepeatRecord, plans ...*timelinePlan) (*contextRepeatProofIndex, func(), error) {
	free, err := contextRepeatScratch(r.ctx, r.s, len(r.s.sequence)+len(rows)+len(timeline.State.Closures), 640)
	if err != nil {
		return nil, nil, err
	}
	ok := false
	freeEvents := func() {}
	defer func() {
		if !ok {
			freeEvents()
			free()
		}
	}()
	x := &contextRepeatProofIndex{coverageOrder: make(map[NodeRef]int), unknownOrder: -1, endGaps: make(map[NodeRef]Evidence[bool]), parts: make(map[NodeRef][]contextRepeatNote), scopes: make(map[contextRepeatProofScopeKey]*contextRepeatProofScope), rows: make(map[NodeRef]int), ends: make(map[contextRepeatMeasureKey]Evidence[musicalPosition]), closures: make(map[NodeRef]stateClosure), coverage: make(map[NodeRef]Evidence[bool]), unknown: navigationKnownBool(true), all: navigationKnownBool(true)}
	type scope struct {
		part  Evidence[NodeRef]
		note  bool
		level uint8
	}
	scopes := make(map[NodeID]scope)
	type ancestorKey struct {
		part NodeRef
		node NodeID
	}
	checked := make(map[ancestorKey]bool)
	order := 0
	merge := func(part Evidence[NodeRef], gap Evidence[bool]) {
		if gap.State == EvidenceKnown && gap.Value {
			return
		}
		// Unknown-part rows enumerate every note; aggregate once in source order.
		x.all = stateMergeGap(x.all, gap)
		if part.State != EvidenceKnown {
			if stateAvailabilityRank(gap.State) > stateAvailabilityRank(x.unknown.State) {
				x.unknownOrder = order
			}
			x.unknown = stateMergeGap(x.unknown, gap)
			return
		}
		old, found := x.coverage[part.Value]
		if !found {
			old = navigationKnownBool(true)
		}
		if stateAvailabilityRank(gap.State) > stateAvailabilityRank(old.State) {
			x.coverageOrder[part.Value] = order
		}
		x.coverage[part.Value] = stateMergeGap(old, gap)
	}
	// Borrow the selected timeline's exact hierarchy and canonical part resolver.
	// Events do not enumerate containers that have no representable children.
	var plan *timelinePlan
	var hierarchy timelineHierarchy
	hierarchyIndex := -1
	if len(plans) != 0 && plans[0] != nil && plans[0].Bindings != nil && r.s.root != nil {
		plan = plans[0]
		for i, h := range plan.Bindings.Hierarchies {
			if r.s.root.header.Declaration.State == EvidenceKnown && r.sameDeclaration(r.s.root.header.Declaration.Value, h.Root.Declaration) {
				hierarchy, hierarchyIndex = h, i
				break
			}
		}
	}
	for ordinal, id := range r.s.sequence {
		order = ordinal
		if !r.step(1) {
			return nil, nil, r.err
		}
		n := r.s.nodes[id]
		v := scopes[n.header.Parent]
		parentLevel := v.level
		v.level = 0
		if hierarchyIndex >= 0 {
			container, partContainer := false, false
			var declaration DeclarationID
			var potential timelineRole
			switch {
			case n == r.s.root:
				v.level, container = 1, true
			case parentLevel == 1:
				declaration, potential = hierarchy.Part.Declaration, timelineRolePart
				if hierarchyIndex == 1 {
					declaration, potential = hierarchy.Measure.Declaration, timelineRoleMeasure
				}
			case parentLevel == 2:
				declaration, potential = hierarchy.Measure.Declaration, timelineRoleMeasure
				if hierarchyIndex == 1 {
					declaration, potential = hierarchy.Part.Declaration, timelineRolePart
				}
			}
			if declaration != (DeclarationID{}) {
				if n.header.Declaration.State == EvidenceKnown {
					container = r.sameDeclaration(n.header.Declaration.Value, declaration)
				} else {
					reader := stateReader{ctx: r.ctx, s: r.s, b: plan.Bindings}
					container = reader.potential(n, potential)
					r.err = reader.err
				}
				if container {
					v.level = parentLevel + 1
					partContainer = potential == timelineRolePart
				}
			}
			if r.err != nil {
				return nil, nil, r.err
			}
			if container {
				if partContainer {
					var err error
					v.part, err = r.s.timelinePart(r.ctx, n, plan)
					if err != nil {
						return nil, nil, err
					}
				}
				if !r.step(1 + 4*int64(len(v.part.Value.DocumentID))) {
					return nil, nil, r.err
				}
				merge(v.part, r.enumerated(n))
				if n.header.Declaration.State != EvidenceKnown {
					merge(v.part, stateUnavailable[DeclarationID, bool](*n.header.Declaration))
				}
				checked[ancestorKey{v.part.Value, id}] = true
			}
		}
		// Enumerate every direct music-data child, including an event whose
		// timeline fact is unavailable. The singleton theorem must not infer
		// absence from the ordinary positive note roster.
		if parentLevel == 3 && v.part.State == EvidenceKnown {
			if !r.step(1 + int64(len(v.part.Value.DocumentID))) {
				return nil, nil, r.err
			}
			if x.events == nil {
				releaseEvents, allocationErr := contextRepeatScratch(r.ctx, r.s, len(r.s.sequence), 128)
				if allocationErr != nil {
					return nil, nil, allocationErr
				}
				freeEvents = releaseEvents
				x.events = make(map[NodeRef][]NodeID)
			}
			x.events[v.part.Value] = append(x.events[v.part.Value], id)
		}
		fact, isEvent := timeline.Facts[id]
		if isEvent {
			if !r.step(1 + 4*int64(len(fact.Part.Value.DocumentID))) {
				return nil, nil, r.err
			}
			if fact.Part.State == EvidenceKnown {
				gap := navigationKnownBool(true)
				if fact.Origin.State != EvidenceKnown {
					gap = stateMergeGap(gap, stateUnavailable[musicalPosition, bool](fact.Origin))
				}
				if fact.Duration.State != EvidenceKnown && fact.Duration.State != EvidenceAbsent && fact.Duration.State != "" {
					gap = stateMergeGap(gap, stateUnavailable[Rational, bool](fact.Duration))
				}
				if gap.State != EvidenceKnown {
					old, ok := x.endGaps[fact.Part.Value]
					if !ok {
						old = navigationKnownBool(true)
					}
					x.endGaps[fact.Part.Value] = stateMergeGap(old, gap)
				}
			}
			v.part = fact.Part
			v.note = r.matches(n, contextRepeatAnyNote)
			// Only structural ancestors and the note universe are prerequisites.
			// Unrelated metadata/part-list and other parts' descendants are independent.
			for parent := r.s.nodes[n.header.Parent]; parent != nil; parent = r.s.nodes[parent.header.Parent] {
				if !r.step(1 + int64(len(v.part.Value.DocumentID))) {
					return nil, nil, r.err
				}
				// Recheck shared ancestors per part: their own enumeration is relevant,
				// but descendant scalar validity is never merged here.
				if checked[ancestorKey{v.part.Value, parent.header.ID}] {
					break
				}
				checked[ancestorKey{v.part.Value, parent.header.ID}] = true
				merge(v.part, r.enumerated(parent))
			}
			if v.note {
				if _, found := timeline.Playback.Notes[id]; !found {
					merge(v.part, stateUnknown[bool]("context-repeat-note-universe-incomplete"))
				}
			}
			if n.header.Declaration.State != EvidenceKnown {
				merge(v.part, stateUnavailable[DeclarationID, bool](*n.header.Declaration))
			}
			// A displaced display/effect position cannot extend encoded music.
			// Closure uses the unshifted cursor and its actual movement only.
			if fact.Origin.State == EvidenceKnown && fact.Origin.Value.Offset.Valid() {
				pos := fact.Origin.Value
				k := contextRepeatMeasureKey{pos.Part, pos.MeasureOrdinal}
				end := fact.Origin
				backup := r.matches(n, contextRepeatCursorBackup)
				if r.err != nil {
					return nil, nil, r.err
				}
				if !backup && fact.Duration.State == EvidenceKnown && fact.Duration.Value.Valid() && fact.Duration.Value.value.Sign() > 0 {
					end.Value.Offset, err = r.s.rationalAdd(r.ctx, pos.Offset, fact.Duration.Value)
					// This is encoded same-measure extent, not a contained endpoint.
					// Notes and forwards preserve the same timeline uncertainty.
					end.Value.UnprovedFrame = true
					if err != nil {
						return nil, nil, err
					}
				}
				old, found := x.ends[k]
				if !found {
					x.ends[k] = end
				} else if old.State == EvidenceKnown {
					cmp, e := r.s.rationalCmp(r.ctx, old.Value.Offset, end.Value.Offset)
					if e != nil {
						return nil, nil, e
					}
					if cmp < 0 {
						x.ends[k] = end
					}
				}
			}
		}
		if v.note {
			merge(v.part, r.complete(n))
			if n.header.Declaration.State != EvidenceKnown {
				merge(v.part, stateUnavailable[DeclarationID, bool](*n.header.Declaration))
			}
		}
		if r.err != nil {
			return nil, nil, r.err
		}
		scopes[id] = v
	}
	for i, row := range rows {
		if !r.step(1 + int64(len(row.source.DocumentID))) {
			return nil, nil, r.err
		}
		x.rows[row.source] = i
	}
	order = len(r.s.sequence)
	for _, c := range timeline.State.Closures {
		if !r.step(1 + int64(len(c.Part.DocumentID))) {
			return nil, nil, r.err
		}
		x.closures[c.Part] = c
		merge(Evidence[NodeRef]{State: EvidenceKnown, Value: c.Part}, c.Complete)
	}
	ok = true
	return x, func() {
		for _, release := range x.releases {
			release()
		}
		freeEvents()
		free()
	}, nil
}
func (x *contextRepeatProofIndex) partCoverage(part Evidence[NodeRef]) Evidence[bool] {
	if part.State != EvidenceKnown {
		return x.all
	}
	out := x.unknown
	if local, ok := x.coverage[part.Value]; ok {
		if x.unknownOrder < 0 || x.coverageOrder[part.Value] < x.unknownOrder {
			out = stateMergeGap(local, out)
		} else {
			out = stateMergeGap(out, local)
		}
	}
	return out
}
func (x *contextRepeatProofIndex) stopRows(rows []contextRepeatRecord, stop NodeRef) []contextRepeatRecord {
	if i, ok := x.rows[stop]; ok {
		return rows[i : i+1]
	}
	return nil
}

// partEnd supplies only a same-measure encoded-content bound to the beat
// comparison. It never observes a frame origin or establishes measure capacity.
func (x *contextRepeatProofIndex) partEnd(start musicalPosition) Evidence[musicalPosition] {
	if gap, ok := x.endGaps[start.Part]; ok {
		return stateUnavailable[bool, musicalPosition](gap)
	}
	c, ok := x.closures[start.Part]
	if !ok || c.Complete.State != EvidenceKnown || !c.Complete.Value || c.Through != start.MeasureOrdinal {
		return stateUnknown[musicalPosition]("context-repeat-beat-measure-local-boundary-unresolved")
	}
	if end, ok := x.ends[contextRepeatMeasureKey{start.Part, start.MeasureOrdinal}]; ok {
		return end
	}
	return stateUnknown[musicalPosition]("context-repeat-part-end-position-unavailable")
}

// The common unqualified/all-staff and exact single-staff scopes share their
// content roster across all style starts. Unknown/voice-qualified scopes keep
// the charged conservative path; output membership itself can be range-sized.
type contextRepeatProofScopeKey struct {
	part  NodeRef
	all   bool
	staff string
}
type contextRepeatProofScope struct {
	onsetComplete Evidence[bool]
	sourceOnsets  map[uint64]bool
	releaseOnsets func()
	notes         []contextRepeatNote
	buckets       map[uint64][]contextRepeatNote
	complete      Evidence[bool]
	windows       map[uint64]*contextRepeatWindowIndex
}

func (x *contextRepeatProofIndex) cachedPart(ctx context.Context, s *Session, roster *contextRepeatContentRoster, part NodeRef) ([]contextRepeatNote, func(), error) {
	if err := s.step(ctx, 1+int64(len(part.DocumentID))); err != nil {
		return nil, nil, err
	}
	if notes, ok := x.parts[part]; ok {
		return notes, func() {}, nil
	}
	notes, free, err := roster.partNotes(ctx, s, part)
	if err != nil {
		return nil, nil, err
	}
	x.parts[part] = notes
	x.releases = append(x.releases, free)
	return notes, func() {}, nil
}
func (x *contextRepeatProofIndex) scope(ctx context.Context, s *Session, notes []contextRepeatNote, row RepeatContextResult) (*contextRepeatProofScope, func(), error) {
	cache := len(row.ExceptVoices) == 0 && row.Part.State == EvidenceKnown && row.AllStaves.State == EvidenceKnown && (row.AllStaves.Value || row.Staff.State == EvidenceKnown)
	key := contextRepeatProofScopeKey{part: row.Part.Value, all: row.AllStaves.Value}
	if !key.all && row.Staff.State == EvidenceKnown {
		key.staff = row.Staff.Value.String()
	}
	if err := s.step(ctx, 1+int64(len(key.part.DocumentID)+len(key.staff))); err != nil {
		return nil, nil, err
	}
	if cache {
		if scope, ok := x.scopes[key]; ok {
			return scope, func() {}, nil
		}
	}
	selected, status, freeSelected, err := contextRepeatScopedNotes(ctx, s, notes, row)
	if err != nil {
		return nil, nil, err
	}
	scope := &contextRepeatProofScope{notes: selected, complete: status, windows: make(map[uint64]*contextRepeatWindowIndex)}
	var freeBuckets func() = func() {}
	if status.State == EvidenceKnown && status.Value {
		scope.buckets, scope.complete, freeBuckets, err = contextRepeatBuildMeasureRoster(ctx, s, selected, row.Part.Value)
		if err != nil {
			freeSelected()
			return nil, nil, err
		}
	}
	free := func() { scope.closeOnsets(); freeBuckets(); freeSelected() }
	if cache {
		x.scopes[key] = scope
		x.releases = append(x.releases, free)
		return scope, func() {}, nil
	}
	return scope, free, nil
}

func (x *contextRepeatProofIndex) windowIndex(ctx context.Context, s *Session, notes []contextRepeatNote, row RepeatContextResult, origin musicalPosition) (*contextRepeatWindowIndex, func(), error) {
	key := contextRepeatProofScopeKey{part: row.Part.Value, all: row.AllStaves.Value}
	if !key.all && row.Staff.State == EvidenceKnown {
		key.staff = row.Staff.Value.String()
	}
	if err := s.step(ctx, 1+int64(len(key.part.DocumentID)+len(key.staff))); err != nil {
		return nil, nil, err
	}
	scope := x.scopes[key]
	if len(row.ExceptVoices) == 0 && scope != nil {
		if index, ok := scope.windows[origin.MeasureOrdinal]; ok {
			return index, func() {}, nil
		}
		notes = scope.buckets[origin.MeasureOrdinal]
		index, free, err := contextRepeatBuildWindowIndex(ctx, s, notes, origin.Part, origin.MeasureOrdinal)
		if err != nil {
			return nil, nil, err
		}
		scope.windows[origin.MeasureOrdinal] = index
		x.releases = append(x.releases, free)
		return index, func() {}, nil
	}
	return contextRepeatBuildWindowIndex(ctx, s, notes, origin.Part, origin.MeasureOrdinal)
}
