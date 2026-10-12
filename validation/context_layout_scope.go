package validation

import (
	"context"
	"sort"
)

type layoutFrame struct {
	page, system               Evidence[NodeRef]
	first                      Evidence[bool]
	parity                     Evidence[bool]
	pageLayout, systemLayout   *factNode
	pagePeers                  []*factNode
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
		if element.State == EvidenceKnown && element.Value != "print" || attr.State == EvidenceKnown && attr.Value != attribute {
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
	if p.Timeline == (RuleKey{}) || s.catalog == nil {
		return nil
	}
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
	l := layoutScopes{r: r, key: key, timeline: s.timeline[p.Timeline], defaults: defaults, staffDefaults: make(map[string]layoutStaffBucket), staffOverrides: make(map[NodeID]map[string]layoutStaffBucket), widths: make(map[NodeID]layoutSystemWidth), names: make(map[NodeID][]layoutPendingName)}
	if err = l.readStaffLayouts(defaults, l.staffDefaults); err != nil {
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
		var pagePublisher, systemPublisher *factNode
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
			}
			if print != nil && ((pb.State == EvidenceKnown && pb.Value) || (sb.State == EvidenceKnown && sb.Value)) {
				breakDeclarations++
				fact, ok := l.timeline.Facts[print.header.ID]
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
					}
					if err = s.retain(32); err != nil {
						return err
					}
					pagePeers = append(pagePeers, pg)
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
				frame.pagePeers = nil
				frame.systemLayout = nil
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

		if blanks.State == EvidenceKnown && frame.parity.State == EvidenceKnown {
			text := blanks.Value.Integer.String()
			if len(text) > 0 && (text[len(text)-1]-'0')%2 == 1 {
				frame.parity.Value = !frame.parity.Value
			}
		} else if blanks.State != EvidenceAbsent && blanks.State != EvidenceKnown {
			frame.parity = stateUnavailable[Atomic, bool](blanks)
		}
		if conflict || (breakDeclarations > 1 && breakPositionUnknown) {
			frame.pageCertain = false
			frame.systemCertain = false
			frame.page = stateUnknown[NodeRef]("parallel-print-break-disagreement")
			frame.system = frame.page
			frame.first = stateUnknown[bool]("parallel-print-break-disagreement")
		}
		if pagePublisher != nil {
			frame.pageLayout = pagePublisher
			frame.pagePeers = pagePeers
			frame.pageCertain = true
		}
		if systemPublisher != nil {
			frame.systemLayout = systemPublisher
			frame.systemCertain = true
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
