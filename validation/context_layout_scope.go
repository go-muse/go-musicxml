package validation

import (
	"context"
	"sort"
)

type layoutFrame struct {
	page, system               Evidence[NodeRef]
	first                      Evidence[bool]
	pageNumber                 uint64
	parity                     Evidence[bool]
	pageLayout, systemLayout   *factNode
	pageCertain, systemCertain bool
}
type layoutScopes struct {
	r                            *contextReader
	key                          RuleKey
	timeline                     *timelineService
	defaults                     *factNode
	pageComplete, systemComplete Evidence[bool]
	staffDefaults                map[string]layoutStaffBucket
	staffOverrides               map[NodeID]map[string]layoutStaffBucket
	staffUnknown                 bool
	widths                       map[NodeID]layoutSystemWidth
}

func (l *layoutScopes) supports(attribute string) Evidence[bool] {
	r := l.r
	result := Evidence[bool]{State: EvidenceAbsent}
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
		if element.State != EvidenceKnown || attr.State != EvidenceKnown {
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
	if result.State == EvidenceAbsent {
		return stateUnknown[bool]("encoded-break-completeness-unavailable")
	}
	return result
}
func (l *layoutScopes) publish(n *factNode, property string, raw, value Evidence[Atomic], active Evidence[bool], scope NodeRef, frame layoutFrame, origin *factNode) error {
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
	return l.r.s.publishContextResult(l.r.ctx, ContextResult{Provider: l.key, Subject: ref, Property: ContextProperty("layout." + property), Value: value, Layout: &LayoutContextResult{Scope: scope, Origin: from, Raw: raw, Applicability: active, Unit: ContextUnitTenths, Page: frame.page, System: frame.system, Provenance: provenance}})
}
func (l *layoutScopes) child(n *factNode, name string) (*factNode, Evidence[bool]) {
	if n == nil {
		return nil, Evidence[bool]{State: EvidenceAbsent}
	}
	return l.r.child(n, layoutRole(name))
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
func (l *layoutScopes) firstVisible(blocks []timelineMeasureBlock) (Evidence[NodeRef], error) {
	r := l.r
	for _, b := range blocks {
		if !r.step(1) {
			return Evidence[NodeRef]{}, r.err
		}
		if b.part.State != EvidenceKnown {
			return b.part, nil
		}
		if l.timeline == nil || !l.timeline.Final {
			return stateUnknown[NodeRef]("staff-visibility-state-unavailable"), nil
		}
		pos := musicalPosition{Part: b.part.Value, Measure: NodeRef{r.s.documentID, b.measure.header.ID}, MeasureOrdinal: b.ordinal, Offset: timelineZero}
		details, err := l.timeline.State.effectiveStaffDetails(r.ctx, r.s, pos, Evidence[Integer]{State: EvidenceKnown, Value: localOne})
		if err != nil {
			return Evidence[NodeRef]{}, err
		}
		if details.Value.State == EvidenceAbsent {
			return b.part, nil
		}
		if details.Value.State != EvidenceKnown {
			return stateUnavailable[staffDetailsState, NodeRef](details.Value), nil
		}
		visible := details.Value.Value.effectivePrintObject().Effective
		spacing := details.Value.Value.effectivePrintSpacing().Effective
		// Visibility selects the first visible system staff; spacing remains an
		// independent prerequisite for proving hidden-staff exclusion.
		if visible.State == EvidenceKnown && visible.Value {
			return b.part, nil
		}

		if visible.State != EvidenceKnown || spacing.State != EvidenceKnown {
			return stateUnknown[NodeRef]("first-visible-staff-unavailable"), nil
		}
		count, err := l.timeline.State.effectiveStaves(r.ctx, r.s, pos)
		if err != nil {
			return Evidence[NodeRef]{}, err
		}
		if count.Value.State != EvidenceKnown || count.Value.Value.String() != "1" {
			return stateUnknown[NodeRef]("first-visible-multistaff-unavailable"), nil
		}
	}
	return Evidence[NodeRef]{State: EvidenceAbsent}, nil
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
	rule, ok := s.catalog.rules[p.Timeline]
	if !ok {
		return nil
	}
	tp := s.catalog.plans[rule.Predicate].timeline
	if tp == nil {
		return nil
	}
	blocks, err := s.timelineBlocks(ctx, tp)
	if err != nil {
		return err
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
	l := layoutScopes{r: r, key: key, timeline: s.timeline[p.Timeline], defaults: defaults, staffDefaults: make(map[string]layoutStaffBucket), staffOverrides: make(map[NodeID]map[string]layoutStaffBucket), widths: make(map[NodeID]layoutSystemWidth)}
	if err = l.readStaffLayouts(defaults, l.staffDefaults); err != nil {
		return err
	}
	l.pageComplete = l.supports("new-page")
	l.systemComplete = l.supports("new-system")
	pageDefaults, _ := l.child(defaults, "page-layout")
	systemDefaults, _ := l.child(defaults, "system-layout")
	frame := layoutFrame{pageCertain: true, systemCertain: true, first: Evidence[bool]{State: EvidenceKnown, Value: true}, pageNumber: 1, parity: Evidence[bool]{State: EvidenceKnown, Value: true}}
	for start := 0; start < len(ordered); {
		end := start + 1
		for end < len(ordered) && ordered[end].ordinal == ordered[start].ordinal {
			if !r.step(1) {
				return r.err
			}
			end++
		}
		group := ordered[start:end]
		first, err := l.firstVisible(group)
		if err != nil {
			return err
		}
		var pageBreak, systemBreak Evidence[bool]
		var pagePublisher, systemPublisher *factNode
		conflict := false
		for _, b := range group {
			if !r.step(1) {
				return r.err
			}
			print, e := r.child(b.content, layoutRole("print"))
			if e.State != EvidenceKnown && e.State != EvidenceAbsent {
				pageBreak = stateUnavailable[bool, bool](e)
				systemBreak = pageBreak
				continue
			}
			pb, sb := layoutBreak(r, print, "new-page", l.pageComplete), layoutBreak(r, print, "new-system", l.systemComplete)
			if pb.State == EvidenceKnown && pb.Value {
				sb = pb
			}
			for _, pair := range [][2]Evidence[bool]{{pageBreak, pb}, {systemBreak, sb}} {
				if pair[0].State == EvidenceKnown && pair[1].State == EvidenceKnown && pair[0].Value != pair[1].Value {
					conflict = true
				}
			}
			if pageBreak.State == "" || pb.State == EvidenceKnown && pb.Value {
				pageBreak = pb
			}
			if systemBreak.State == "" || sb.State == EvidenceKnown && sb.Value {
				systemBreak = sb
			}
			if print != nil {
				pg, _ := l.child(print, "page-layout")
				if pg != nil {
					if pagePublisher == nil {
						pagePublisher = pg
					} else {
						conflict = true
					}
				}
				if first.State == EvidenceKnown && b.part.State == EvidenceKnown && first.Value == b.part.Value {
					systemPublisher, _ = l.child(print, "system-layout")
				}
			}
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
		}
		if !initial {
			if pageBreak.State == EvidenceKnown && pageBreak.Value {
				frame.pageLayout = nil
				frame.systemLayout = nil
				frame.pageCertain = true
				frame.systemCertain = true
				frame.page = Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{s.documentID, group[0].measure.header.ID}}
				frame.system = frame.page
				frame.pageNumber++
				if frame.parity.State == EvidenceKnown {
					frame.parity.Value = !frame.parity.Value
				}
				frame.first = Evidence[bool]{State: EvidenceKnown, Value: true}
			} else {
				if pageBreak.State != EvidenceKnown {
					frame.pageCertain = false
					frame.parity = stateUnknown[bool]("physical-page-parity-unavailable")
					frame.page = stateUnknown[NodeRef]("page-membership-unavailable")
				}
				if systemBreak.State == EvidenceKnown && systemBreak.Value {
					frame.systemLayout = nil
					frame.systemCertain = true
					frame.system = Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{s.documentID, group[0].measure.header.ID}}
					frame.first = Evidence[bool]{State: EvidenceKnown, Value: false}
					if pageBreak.State != EvidenceKnown {
						frame.first = stateUnknown[bool]("page-break-unavailable")
					}
				} else if systemBreak.State != EvidenceKnown {
					frame.systemCertain = false
					frame.system = stateUnknown[NodeRef]("system-membership-unavailable")
					frame.first = stateUnknown[bool]("system-membership-unavailable")
				}
			}
		}
		if conflict {
			frame.pageCertain = false
			frame.systemCertain = false
			frame.page = stateUnknown[NodeRef]("parallel-print-break-disagreement")
			frame.system = frame.page
			frame.first = stateUnknown[bool]("parallel-print-break-disagreement")
		}
		if pagePublisher != nil {
			frame.pageLayout = pagePublisher
			frame.pageCertain = true
		}
		if systemPublisher != nil {
			frame.systemLayout = systemPublisher
			frame.systemCertain = true
		}
		for _, b := range group {
			ref := NodeRef{s.documentID, b.measure.header.ID}
			active := Evidence[bool]{State: EvidenceKnown, Value: true}
			for _, name := range []string{"page-height", "page-width"} {
				raw, value, origin := l.effective(frame.pageLayout, pageDefaults, name, frame.pageCertain)
				if err = l.publish(b.measure, name, raw, value, active, ref, frame, origin); err != nil {
					return err
				}
			}
			for _, name := range []string{"system-distance", "top-system-distance"} {
				raw, value, origin := l.effective(frame.systemLayout, systemDefaults, name, frame.systemCertain)
				app := frame.first
				if name == "system-distance" && app.State == EvidenceKnown {
					app.Value = !app.Value
				}
				if err = l.publish(b.measure, name, raw, value, app, ref, frame, origin); err != nil {
					return err
				}
			}
			print, _ := l.child(b.content, "print")
			measureLayout, _ := l.child(print, "measure-layout")
			raw := l.scalar(measureLayout, "measure-distance")
			if err = l.publish(b.measure, "measure-distance", raw, raw, stateUnknown[bool]("internal-system-whitespace-unavailable"), ref, frame, measureLayout); err != nil {
				return err
			}
			if conflict {
				if err = l.publish(b.measure, "width-outcome", Evidence[Atomic]{State: EvidenceAbsent}, stateUnknown[Atomic]("parallel-print-rendering-unspecified"), active, ref, frame, nil); err != nil {
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
			if err = l.publishStaffLayouts(b, first, frame); err != nil {
				return err
			}
		}
		start = end
	}
	if r.err != nil {
		return r.err
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
