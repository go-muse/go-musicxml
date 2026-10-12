package validation

import (
	"context"
	"sort"
)

type layoutFrame struct {
	page, system                   Evidence[NodeRef]
	first                          Evidence[bool]
	parity                         Evidence[bool]
	pageLayout, systemLayout       *factNode
	pagePeers                      []*factNode
	pageCertain, systemCertain     bool
	pageSelection, systemSelection Evidence[bool]
}
type layoutScopes struct {
	r                            *contextReader
	key                          RuleKey
	timeline                     *timelineService
	defaults                     *factNode
	pageComplete, systemComplete Evidence[bool]
	staffDefaults                map[string]layoutStaffBucket
	staffOverrides               map[NodeID]map[string]layoutStaffBucket
	staffDefaultGap              Evidence[bool]
	staffGaps                    map[NodeID]Evidence[bool]
	staffEpoch                   uint64
	openWidth                    NodeID
	widths                       map[NodeID]layoutSystemWidth
	names                        map[NodeID][]layoutPendingName
	candidateCache               map[NodeID][]Integer
}

func (l *layoutScopes) supports(attribute string) Evidence[bool] {
	r := l.r
	result := Evidence[bool]{State: EvidenceAbsent}
	uncertain := Evidence[bool]{State: EvidenceAbsent}
	for _, id := range r.s.sequence {
		if !r.step(1) {
			return stateUnknown[bool]("context-interrupted")
		}
		n := r.s.nodes[id]
		if !r.matches(n, layoutRole("supports")) {
			continue
		}
		element := stateText(r.attribute(n, layoutRole("supports.@element")))
		attr := stateText(r.attribute(n, layoutRole("supports.@attribute")))
		if element.State == EvidenceKnown && element.Value != "print" || attr.State == EvidenceAbsent || attr.State == EvidenceKnown && attr.Value != attribute {
			continue
		}
		if element.State != EvidenceKnown || attr.State != EvidenceKnown {
			uncertain = stateUnknown[bool]("potential-break-support-unavailable")
			continue
		}
		if element.Value != "print" || attr.Value != attribute {
			continue
		}
		typ := stateYesNo(r.attribute(n, layoutRole("supports.@type")))
		value := stateText(r.attribute(n, layoutRole("supports.@value")))
		if typ.State == EvidenceKnown && typ.Value && value.State == EvidenceKnown && value.Value == "yes" {
			if result.State == EvidenceKnown && !result.Value {
				return stateUnknown[bool]("conflicting-break-support")
			}
			result = Evidence[bool]{State: EvidenceKnown, Value: true}
		} else if typ.State == EvidenceKnown && !typ.Value {
			if result.State == EvidenceKnown && result.Value {
				return stateUnknown[bool]("conflicting-break-support")
			}
			result = Evidence[bool]{State: EvidenceKnown, Value: false}
		}
	}
	if uncertain.State != EvidenceAbsent {
		return uncertain
	}
	if result.State == EvidenceAbsent {
		return stateUnknown[bool]("encoded-break-completeness-unavailable")
	}
	return result
}
func (l *layoutScopes) publish(n *factNode, property string, raw, value Evidence[Atomic], active Evidence[bool], scope NodeRef, frame layoutFrame, origin *factNode, part Evidence[NodeRef]) error {
	if l.r.err != nil {
		return l.r.err
	}
	ref := NodeRef{l.r.s.documentID, n.header.ID}
	from := ref
	if origin != nil {
		from.Node = origin.header.ID
	}
	provenance := ContextExplicit
	if raw.State == EvidenceAbsent {
		provenance = ContextInherited
		if value.State == EvidenceUnknown {
			provenance = ContextApplicationChoice
		}
	}
	return l.r.s.publishContextResult(l.r.ctx, ContextResult{Provider: l.key, Subject: ref, Property: ContextProperty("layout." + property), Value: value, Layout: &LayoutContextResult{Staff: Evidence[Integer]{State: EvidenceAbsent}, Conversion: Evidence[Rational]{State: EvidenceAbsent}, Part: part, Scope: scope, Origin: from, Raw: raw, Applicability: active, Unit: ContextUnitTenths, Page: frame.page, System: frame.system, Provenance: provenance}})
}
func (l *layoutScopes) child(n *factNode, name string) (*factNode, Evidence[bool]) {
	if n == nil {
		return nil, Evidence[bool]{State: EvidenceAbsent}
	}
	return l.r.child(n, layoutRole(name))
}

// selectedChild keeps an unavailable parent selection distinct from absence.
func (l *layoutScopes) selectedChild(n *factNode, selected Evidence[bool], name string) (*factNode, Evidence[bool]) {
	if selected.State != "" && selected.State != EvidenceKnown && selected.State != EvidenceAbsent {
		return nil, selected
	}
	return l.child(n, name)
}
func layoutMergeSelection(a, b Evidence[bool]) Evidence[bool] {
	if a.State == "" || a.State == EvidenceAbsent {
		return b
	}
	if b.State == "" || b.State == EvidenceAbsent {
		return a
	}
	return timelineMergeAvailability(a, b)
}
func layoutSelectionValue(value Evidence[Atomic], selected Evidence[bool]) Evidence[Atomic] {
	if selected.State != "" && selected.State != EvidenceKnown && selected.State != EvidenceAbsent {
		return stateUnavailable[bool, Atomic](selected)
	}
	return value
}
func (l *layoutScopes) effectiveSystem(frame layoutFrame, defaults *factNode, name string) (Evidence[Atomic], Evidence[Atomic], *factNode) {
	raw, value, origin := l.effective(frame.systemLayout, defaults, name, frame.systemCertain)
	if !frame.systemCertain && raw.State != EvidenceAbsent {
		value = layoutLifetimeValue[Atomic](frame.system, "layout-scope-lifetime-unavailable")
	}
	if frame.systemSelection.State != "" && frame.systemSelection.State != EvidenceKnown && frame.systemSelection.State != EvidenceAbsent {
		origin = nil
	}
	return layoutSelectionValue(raw, frame.systemSelection), layoutSelectionValue(value, frame.systemSelection), origin
}
func (l *layoutScopes) effectiveSystemMargin(frame layoutFrame, defaults *factNode, name string) (Evidence[Atomic], Evidence[Atomic], *factNode) {
	override, e := l.selectedChild(frame.systemLayout, frame.systemSelection, "system-margins")
	fallback, fe := l.child(defaults, "system-margins")
	raw, value, origin := l.effective(override, fallback, name, frame.systemCertain)
	if !frame.systemCertain && raw.State != EvidenceAbsent {
		value = layoutLifetimeValue[Atomic](frame.system, "layout-scope-lifetime-unavailable")
	}
	raw = layoutSelectionValue(raw, e)
	value = layoutSelectionValue(value, e)
	if e.State != EvidenceKnown && e.State != EvidenceAbsent {
		origin = nil
	}
	if raw.State == EvidenceAbsent {
		value = layoutSelectionValue(value, fe)
	}
	return raw, value, origin
}
func (l *layoutScopes) scalar(n *factNode, name string) Evidence[Atomic] {
	if n == nil {
		return Evidence[Atomic]{State: EvidenceAbsent}
	}
	return l.r.childScalar(n, layoutRole(name))
}
func (l *layoutScopes) effective(override, defaults *factNode, name string, certain bool) (Evidence[Atomic], Evidence[Atomic], *factNode) {
	raw := l.scalar(override, name)
	value := raw
	origin := override
	if !certain && override != nil && raw.State != EvidenceAbsent {
		return raw, stateUnknown[Atomic]("layout-scope-lifetime-unavailable"), override
	}
	if value.State == EvidenceAbsent {
		value = l.scalar(defaults, name)
		origin = defaults
	}
	return raw, layoutUnknownDefault(value, "application-layout-choice"), origin
}
func layoutBreak(r *contextReader, print *factNode, name string, complete Evidence[bool]) Evidence[bool] {
	raw := layoutAttribute(r, print, name)
	if raw.State == EvidenceAbsent {
		if complete.State == EvidenceKnown && complete.Value {
			return Evidence[bool]{State: EvidenceKnown, Value: false}
		}
		return stateUnknown[bool]("encoded-break-completeness-unavailable")
	}
	return stateYesNo(raw)
}
func evaluateLayoutScopes(ctx context.Context, s *Session, key RuleKey, p *contextPlan, defaults *factNode) error {
	r := &contextReader{ctx: ctx, s: s, b: p.Bindings}
	blocks, available, err := layoutGetBlocks(r, key, p)
	if err != nil {
		return err
	}
	if available.State != EvidenceKnown {
		return nil
	}
	if err = s.retain(1024 + int64(len(blocks))*768); err != nil {
		return err
	}
	ordered := append([]timelineMeasureBlock(nil), blocks...)
	var sortError error
	sort.SliceStable(ordered, func(i, j int) bool {
		if sortError != nil {
			return false
		}
		sortError = s.step(ctx, 1)
		if ordered[i].ordinal != ordered[j].ordinal {
			return ordered[i].ordinal < ordered[j].ordinal
		}
		return ordered[i].part.Value.Node < ordered[j].part.Value.Node
	})
	if sortError != nil {
		return sortError
	}
	l := layoutScopes{r: r, key: key, timeline: s.timeline[p.Timeline], defaults: defaults, staffDefaults: make(map[string]layoutStaffBucket), staffOverrides: make(map[NodeID]map[string]layoutStaffBucket), widths: make(map[NodeID]layoutSystemWidth), names: make(map[NodeID][]layoutPendingName), staffGaps: make(map[NodeID]Evidence[bool])}
	if l.staffDefaultGap, err = l.readStaffLayouts(defaults, l.staffDefaults); err != nil {
		return err
	}
	l.pageComplete = l.supports("new-page")
	l.systemComplete = l.supports("new-system")
	pageDefaults, _ := l.child(defaults, "page-layout")
	systemDefaults, _ := l.child(defaults, "system-layout")
	frame := layoutFrame{pageCertain: true, systemCertain: true, first: Evidence[bool]{State: EvidenceKnown, Value: true}, parity: Evidence[bool]{State: EvidenceKnown, Value: true}}
	for start := 0; start < len(ordered); {
		end := start + 1
		for end < len(ordered) && ordered[end].ordinal == ordered[start].ordinal {
			if !r.step(1) {
				return r.err
			}
			end++
		}
		group := ordered[start:end]
		visible, err := l.firstSystemStaff(group, false)
		first := layoutStaffPart(visible)
		if err != nil {
			return err
		}
		firstSlot, err := l.firstSystemStaff(group, true)
		if err != nil {
			return err
		}
		var pageBreak, systemBreak Evidence[bool]
		blanks := Evidence[Atomic]{State: EvidenceAbsent}
		breakPosition := Evidence[Rational]{State: EvidenceAbsent}
		breakPositionUnknown := false
		breakDeclarations := 0
		breakSelection := Evidence[bool]{State: EvidenceAbsent}
		var pagePublisher, systemPublisher *factNode
		pageSelection := Evidence[bool]{State: EvidenceAbsent}
		systemSelection := Evidence[bool]{State: EvidenceAbsent}
		var pagePeers []*factNode
		conflict := false
		for _, b := range group {
			if !r.step(1) {
				return r.err
			}
			print, e := r.child(b.content, layoutRole("print"))
			if e.State != EvidenceKnown && e.State != EvidenceAbsent {
				pageBreak = stateUnavailable[bool, bool](e)
				systemBreak = pageBreak
				breakSelection = layoutMergeSelection(breakSelection, e)
				pageSelection = layoutMergeSelection(pageSelection, e)
				if first.State != EvidenceKnown || b.part.State != EvidenceKnown || first.Value == b.part.Value {
					systemSelection = layoutMergeSelection(systemSelection, e)
				}
				continue
			}
			pb, sb := layoutBreak(r, print, "new-page", l.pageComplete), layoutBreak(r, print, "new-system", l.systemComplete)
			if pb.State == EvidenceKnown && pb.Value {
				bv := layoutAttribute(r, print, "blank-page")
				if !r.step(1 + atomicLogicalBytes(bv.Value) + atomicLogicalBytes(blanks.Value)) {
					return r.err
				}
				blanks = layoutConsensusPair(blanks, bv)
				sb = pb
			} else if pb.State != EvidenceKnown && !(sb.State == EvidenceKnown && sb.Value) {
				// Every page boundary starts a system; an unavailable page
				// attribute can therefore invalidate a negative system break.
				sb, _ = layoutMergeBreak(sb, pb)
			}
			if print != nil && ((pb.State == EvidenceKnown && pb.Value) || (sb.State == EvidenceKnown && sb.Value)) {
				breakDeclarations++
				var fact timelineEventFact
				ok := false
				if l.timeline != nil {
					fact, ok = l.timeline.Facts[print.header.ID]
				}
				if !ok || fact.Origin.State != EvidenceKnown || fact.Origin.Value.UnprovedFrame {
					breakPositionUnknown = true
				} else if breakPosition.State == EvidenceAbsent {
					breakPosition = Evidence[Rational]{State: EvidenceKnown, Value: fact.Origin.Value.Offset}
				} else {
					cmp, e := s.rationalCmp(ctx, breakPosition.Value, fact.Origin.Value.Offset)
					if e != nil {
						return e
					}
					if cmp != 0 {
						conflict = true
					}
				}
			}
			var disagrees bool
			pageBreak, disagrees = layoutMergeBreak(pageBreak, pb)
			conflict = conflict || disagrees
			systemBreak, disagrees = layoutMergeBreak(systemBreak, sb)
			conflict = conflict || disagrees
			if print != nil {
				pg, pe := l.child(print, "page-layout")
				if pe.State != EvidenceKnown && pe.State != EvidenceAbsent {
					pageSelection = layoutMergeSelection(pageSelection, pe)
				}
				if pg != nil {
					if pagePublisher == nil {
						pagePublisher = pg
					}
					if err = s.retain(32); err != nil {
						return err
					}
					pagePeers = append(pagePeers, pg)
				}
				system, se := l.child(print, "system-layout")
				if first.State == EvidenceKnown && b.part.State == EvidenceKnown && first.Value == b.part.Value {
					systemPublisher = system
					systemSelection = layoutMergeSelection(systemSelection, se)
				} else if (first.State != EvidenceKnown && first.State != EvidenceAbsent || b.part.State != EvidenceKnown) && se.State != EvidenceAbsent {
					gap := stateUnavailable[NodeRef, bool](first)
					if b.part.State != EvidenceKnown {
						gap = stateUnavailable[NodeRef, bool](b.part)
					}
					systemSelection = layoutMergeSelection(systemSelection, gap)
				}
			}
		}
		// Consensus is transactional: disputed boundaries never clear live
		// overrides or advance page parity as though a break had been proved.
		if breakSelection.State != EvidenceAbsent {
			pageBreak = breakSelection
			systemBreak = breakSelection
		}
		conflict = conflict || (breakDeclarations > 1 && breakPositionUnknown)
		if conflict {
			pageBreak = stateUnknown[bool]("parallel-print-break-disagreement")
			systemBreak = pageBreak
			blanks = Evidence[Atomic]{State: EvidenceAbsent}
		}
		initial := start == 0
		if initial {
			frame.page = Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{s.documentID, group[0].measure.header.ID}}
			frame.system = frame.page
		}
		if !initial && systemBreak.State == EvidenceKnown && systemBreak.Value {
			if err = s.retain(512); err != nil {
				return err
			}
			l.staffOverrides = make(map[NodeID]map[string]layoutStaffBucket)
			l.staffGaps = make(map[NodeID]Evidence[bool])
		} else if !initial && systemBreak.State != EvidenceKnown {
			l.staffEpoch++
		}
		if !initial {
			if pageBreak.State == EvidenceKnown && pageBreak.Value {
				frame.pageLayout = nil
				frame.pagePeers = nil
				frame.systemLayout = nil
				frame.pageSelection = Evidence[bool]{State: EvidenceAbsent}
				frame.systemSelection = Evidence[bool]{State: EvidenceAbsent}
				frame.pageCertain = true
				frame.systemCertain = true
				frame.page = Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{s.documentID, group[0].measure.header.ID}}
				frame.system = frame.page
				if frame.parity.State == EvidenceKnown {
					frame.parity.Value = !frame.parity.Value
				}
				frame.first = Evidence[bool]{State: EvidenceKnown, Value: true}
			} else {
				if pageBreak.State != EvidenceKnown {
					frame.pageCertain = false
					frame.parity = layoutBoundaryGap[bool](pageBreak, "physical-page-parity-unavailable")
					frame.page = layoutBoundaryGap[NodeRef](pageBreak, "page-membership-unavailable")
				}
				if systemBreak.State == EvidenceKnown && systemBreak.Value {
					frame.systemLayout = nil
					frame.systemSelection = Evidence[bool]{State: EvidenceAbsent}
					frame.systemCertain = true
					frame.system = Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{s.documentID, group[0].measure.header.ID}}
					frame.first = Evidence[bool]{State: EvidenceKnown, Value: false}
					if pageBreak.State != EvidenceKnown {
						frame.first = layoutBoundaryGap[bool](pageBreak, "page-break-unavailable")
					}
				} else if systemBreak.State != EvidenceKnown {
					frame.systemCertain = false
					frame.system = layoutBoundaryGap[NodeRef](systemBreak, "system-membership-unavailable")
					frame.first = layoutBoundaryGap[bool](systemBreak, "system-membership-unavailable")
				}
			}
		}

		frame.parity = layoutBlankPageParity(frame.parity, blanks)
		if conflict {
			frame.pageCertain = false
			frame.systemCertain = false
			frame.parity = stateUnknown[bool]("parallel-print-break-disagreement")
			frame.page = stateUnknown[NodeRef]("parallel-print-break-disagreement")
			frame.system = frame.page
			frame.first = stateUnknown[bool]("parallel-print-break-disagreement")
		}
		if pagePublisher != nil {
			frame.pageLayout = pagePublisher
			frame.pagePeers = pagePeers
			frame.pageCertain = true
			frame.pageSelection = Evidence[bool]{State: EvidenceAbsent}
		}
		if systemPublisher != nil {
			frame.systemLayout = systemPublisher
			frame.systemCertain = true
			frame.systemSelection = Evidence[bool]{State: EvidenceAbsent}
		}
		if pageSelection.State != EvidenceKnown && pageSelection.State != EvidenceAbsent {
			frame.pageSelection = pageSelection
		}
		if systemSelection.State != EvidenceKnown && systemSelection.State != EvidenceAbsent {
			frame.systemSelection = systemSelection
		}
		systemStart := systemBreak
		if initial {
			systemStart = Evidence[bool]{State: EvidenceKnown, Value: true}
		}
		if conflict || (breakDeclarations > 1 && breakPositionUnknown) {
			systemStart = stateUnknown[bool]("parallel-print-break-disagreement")
		}
		for _, b := range group {
			if err = layoutStateMeasure(ctx, s, key, b, r, p, systemStart); err != nil {
				return err
			}
			if err = l.nameUpdates(b, frame, systemStart); err != nil {
				return err
			}
			ref := NodeRef{s.documentID, b.measure.header.ID}
			active := Evidence[bool]{State: EvidenceKnown, Value: true}
			for _, name := range []string{"page-height", "page-width"} {
				raw, value, origin := l.effectivePage(frame, pageDefaults, name)
				if err = l.publish(b.measure, name, raw, value, active, ref, frame, origin, b.part); err != nil {
					return err
				}
			}
			for _, name := range []string{"system-distance", "top-system-distance"} {
				raw, value, origin := l.effectiveSystem(frame, systemDefaults, name)
				app := frame.first
				if name == "system-distance" && app.State == EvidenceKnown {
					app.Value = !app.Value
				}
				if err = l.publish(b.measure, name, raw, value, app, ref, frame, origin, b.part); err != nil {
					return err
				}
			}
			print, pe := l.child(b.content, "print")
			measureLayout, me := l.selectedChild(print, pe, "measure-layout")
			raw := layoutSelectionValue(l.scalar(measureLayout, "measure-distance"), me)
			if err = l.publish(b.measure, "measure-distance", raw, raw, stateUnknown[bool]("internal-system-whitespace-unavailable"), ref, frame, measureLayout, b.part); err != nil {
				return err
			}

			if conflict {
				if err = l.publish(b.measure, "width-outcome", Evidence[Atomic]{State: EvidenceAbsent}, stateUnknown[Atomic]("parallel-print-rendering-unspecified"), active, ref, frame, nil, b.part); err != nil {
					return err
				}
			}
			if err = l.publishDividers(b, frame, systemDefaults); err != nil {
				return err
			}
			if err = l.addWidth(b, frame, first, pageDefaults, systemDefaults); err != nil {
				return err
			}
			if err = l.publishMargins(b, frame, pageDefaults, systemDefaults); err != nil {
				return err
			}
			if err = l.publishStaffLayouts(b, firstSlot, frame); err != nil {
				return err
			}
		}
		start = end
	}
	if r.err != nil {
		return r.err
	}
	if err = l.closeNames(); err != nil {
		return err
	}
	return l.publishWidths()
}

func layoutSortStrings(r *contextReader, values []string) error {
	var err error
	sort.Slice(values, func(i, j int) bool {
		if err != nil {
			return false
		}
		err = r.s.step(r.ctx, 1+int64(len(values[i])+len(values[j])))
		return values[i] < values[j]
	})
	return err
}

// Blank pages change physical parity only for an initialized nonnegative integer.
func layoutBlankPageParity(parity Evidence[bool], blanks Evidence[Atomic]) Evidence[bool] {
	if blanks.State == EvidenceAbsent {
		return parity
	}
	if blanks.State != EvidenceKnown {
		return stateUnavailable[Atomic, bool](blanks)
	}
	if blanks.Value.Kind != AtomicInteger || !blanks.Value.Integer.Valid() || blanks.Value.Integer.value.Sign() < 0 {
		return stateUnknown[bool]("blank-page-integer-unavailable")
	}
	if parity.State == EvidenceKnown && blanks.Value.Integer.value.Bit(0) != 0 {
		parity.Value = !parity.Value
	}
	return parity
}

// A missing completeness statement is not a contradiction of another part's
// explicit positive break. Unavailable encoded attributes are different: they
// cannot be discarded in favor of either a positive or a negative statement.
func layoutMergeBreak(a, b Evidence[bool]) (Evidence[bool], bool) {
	if a.State == "" {
		return b, false
	}
	if a.State == EvidenceKnown && b.State == EvidenceKnown {
		if a.Value != b.Value {
			return stateUnknown[bool]("parallel-print-break-disagreement"), true
		}
		return a, false
	}
	incomplete := func(v Evidence[bool]) bool {
		return v.State == EvidenceUnknown && v.Reason == "encoded-break-completeness-unavailable"
	}
	if a.State == EvidenceKnown && a.Value && incomplete(b) {
		return a, false
	}
	if b.State == EvidenceKnown && b.Value && incomplete(a) {
		return b, false
	}
	if a.State == EvidenceKnown {
		return b, false
	}
	if b.State == EvidenceKnown {
		return a, false
	}
	if incomplete(a) && !incomplete(b) {
		return b, false
	}
	if incomplete(b) && !incomplete(a) {
		return a, false
	}
	return timelineMergeAvailability(a, b), false
}
func layoutBoundaryGap[T any](v Evidence[bool], reason string) Evidence[T] {
	if v.State == EvidenceInvalid || v.State == EvidenceUnsupported {
		return stateUnavailable[bool, T](v)
	}
	return Evidence[T]{State: EvidenceUnknown, Reason: Code(reason), Cause: v.Cause}
}

func layoutLifetimeValue[T any](scope Evidence[NodeRef], reason Code) Evidence[T] {
	if scope.State == EvidenceInvalid || scope.State == EvidenceUnsupported {
		return stateUnavailable[NodeRef, T](scope)
	}
	return Evidence[T]{State: EvidenceUnknown, Reason: reason, Cause: scope.Cause}
}
