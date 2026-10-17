package validation

import (
	"context"
	"sort"
)

type layoutNumberingSnapshot struct {
	Origin  NodeRef
	Ordinal uint64
	Value   Evidence[string]
}
type layoutStateGlobalGap struct {
	Ordinal      uint64
	Availability Evidence[bool]
}
type layoutStateIndex struct {
	Numbering                        map[timelineDomain]layoutNumberingSnapshot
	Symbols                          stateHistory[NodeRef]
	UnknownNumbering, UnknownSymbols []layoutStateGlobalGap
	Availability                     Evidence[bool]
}

func layoutGetStateIndex(r *contextReader, key RuleKey, p *contextPlan) (*layoutStateIndex, error) {
	if r.err != nil {
		return nil, r.err
	}
	service, e := r.s.newContextService(r.ctx, key)
	if e != nil {
		return nil, e
	}
	if service.Layout != nil && service.Layout.State != nil {
		return service.Layout.State, nil
	}
	if e = r.s.retain(2048); e != nil {
		return nil, e
	}
	index := &layoutStateIndex{Numbering: make(map[timelineDomain]layoutNumberingSnapshot), Availability: Evidence[bool]{State: EvidenceKnown, Value: true}}
	if service.Layout == nil {
		service.Layout = &contextLayoutService{}
	}
	blocks, available, e := layoutGetBlocks(r, key, p)
	if e != nil {
		return nil, e
	}
	if available.State != EvidenceKnown {
		index.Availability = available
		service.Layout.State = index
		return index, nil
	}
	current := make(map[NodeID]layoutNumberingSnapshot)
	for _, b := range blocks {
		if !r.step(1) {
			return nil, r.err
		}
		snapshot := layoutNumberingSnapshot{Value: stateUnknown[string]("measure-numbering-policy-not-encoded")}
		if b.part.State == EvidenceKnown {
			if previous, ok := current[b.part.Value.Node]; ok {
				snapshot = previous
			}
		}
		selected := false
		if b.complete.State != EvidenceKnown {
			snapshot = layoutNumberingSnapshot{Ordinal: b.ordinal, Value: stateUnavailable[bool, string](b.complete)}
		}
		if b.complete.State != EvidenceKnown {
			if e = index.symbolGap(r, b, b.complete); e != nil {
				return nil, e
			}
		}

		for _, event := range b.content.children {
			if !r.step(1) {
				return nil, r.err
			}
			if event.header.Declaration.State != EvidenceKnown {
				unavailable := timelineGapAs[DeclarationID, bool](*event.header.Declaration, "layout-statement-declaration-unavailable")
				snapshot = layoutNumberingSnapshot{Ordinal: b.ordinal, Value: stateUnavailable[bool, string](unavailable)}
				if e = index.symbolGap(r, b, unavailable, layoutStateFact(r, p, event)); e != nil {
					return nil, e
				}
				continue
			}
			if r.matches(event, layoutStateRole("print")) {
				child, presence := r.child(event, layoutStateRole("measure-numbering"))
				if presence.State == EvidenceAbsent {
					continue
				}
				value := stateUnavailable[bool, string](presence)
				origin := NodeRef{}
				if presence.State == EvidenceKnown {
					value = stateText(r.scalar(child))
					origin = NodeRef{r.s.documentID, child.header.ID}
				}
				if selected {
					value = stateUnknown[string]("competing-measure-numbering-publishers")
				}
				selected = true
				snapshot = layoutNumberingSnapshot{Origin: origin, Ordinal: b.ordinal, Value: value}
				if b.part.State != EvidenceKnown {
					gap := timelineGapAs[NodeRef, bool](b.part, "layout-part-unavailable")
					if e = r.s.retain(256); e != nil {
						return nil, e
					}
					index.UnknownNumbering = append(index.UnknownNumbering, layoutStateGlobalGap{b.ordinal, gap})
				}
			}
			if r.matches(event, layoutStateRole("attributes")) {
				child, presence := r.child(event, layoutStateRole("part-symbol"))
				if presence.State == EvidenceAbsent {
					continue
				}
				if presence.State != EvidenceKnown {
					if e = index.symbolGap(r, b, presence, layoutStateFact(r, p, event)); e != nil {
						return nil, e
					}
					continue
				}
				fact := layoutStateFact(r, p, event)
				if b.part.State != EvidenceKnown || fact.Position.State != EvidenceKnown {
					gap := stateUnavailable[musicalPosition, bool](fact.Position)
					if b.part.State != EvidenceKnown {
						gap = stateUnavailable[NodeRef, bool](b.part)
					}
					if e = index.symbolGap(r, b, gap, fact); e != nil {
						return nil, e
					}
					continue
				}
				ref := NodeRef{r.s.documentID, child.header.ID}
				if e = publishState(r.ctx, r.s, &index.Symbols, statePublication[NodeRef]{Position: fact.Position.Value, Scope: stateScope{AllStaves: true}, Value: Evidence[NodeRef]{State: EvidenceKnown, Value: ref}, Provenance: []stateProvenance{{Node: ref, Kind: stateExplicit}}}); e != nil {
					return nil, e
				}
			}
		}
		if b.complete.State != EvidenceKnown {
			snapshot = layoutNumberingSnapshot{Ordinal: b.ordinal, Value: stateUnavailable[bool, string](b.complete)}
		}
		if b.part.State == EvidenceKnown {
			domain := timelineDomain{b.part.Value.Node, b.ordinal}
			if _, duplicate := index.Numbering[domain]; duplicate {
				snapshot = layoutNumberingSnapshot{Ordinal: b.ordinal, Value: stateUnknown[string]("duplicate-part-measure-numbering-domain")}
			}
			if e = r.s.retain(1024); e != nil {
				return nil, e
			}
			index.Numbering[domain] = snapshot
			current[b.part.Value.Node] = snapshot
		}
	}
	var sortErr error
	for _, gaps := range [][]layoutStateGlobalGap{index.UnknownNumbering, index.UnknownSymbols} {
		sort.SliceStable(gaps, func(i, j int) bool {
			if sortErr != nil {
				return false
			}
			sortErr = r.s.step(r.ctx, 1)
			return gaps[i].Ordinal < gaps[j].Ordinal
		})
	}
	if sortErr != nil {
		return nil, sortErr
	}
	if r.err != nil {
		return nil, r.err
	}
	service.Layout.State = index
	return index, nil
}
func (index *layoutStateIndex) symbolGap(r *contextReader, b timelineMeasureBlock, availability Evidence[bool], facts ...timelineEventFact) error {
	availability = timelineGapAs[bool, bool](availability, "layout-publisher-evidence-unavailable")
	if b.part.State != EvidenceKnown {
		if e := r.s.retain(256); e != nil {
			return e
		}
		index.UnknownSymbols = append(index.UnknownSymbols, layoutStateGlobalGap{b.ordinal, availability})
		return nil
	}
	gap := stateGap{Part: b.part.Value, FirstMeasure: b.ordinal, LastMeasure: b.ordinal, Scope: stateScope{AllStaves: true}, Availability: availability, Placement: stateGapPlacement{Displaced: true}}
	if len(facts) > 0 {
		gap.Placement = facts[0].Placement
		if facts[0].Position.State == EvidenceKnown {
			position := facts[0].Position.Value
			gap.Position = &position
		}
	}
	return addStateGap(r.ctx, r.s, &index.Symbols, gap)
}

func layoutLastGlobalGap(ctx context.Context, s *Session, gaps []layoutStateGlobalGap, ordinal uint64) (*layoutStateGlobalGap, error) {
	lo, hi := 0, len(gaps)
	for lo < hi {
		if e := s.step(ctx, 1); e != nil {
			return nil, e
		}
		mid := lo + (hi-lo)/2
		if gaps[mid].Ordinal <= ordinal {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo == 0 {
		return nil, nil
	}
	return &gaps[lo-1], nil
}

// layoutGetBlocks shares the accepted hierarchy projection between visual
// scopes and the state index. timelineBlocks already owns and accounts for the
// immutable slice; caching it does not copy or retain every block a second time.
func layoutGetBlocks(r *contextReader, key RuleKey, p *contextPlan) ([]timelineMeasureBlock, Evidence[bool], error) {
	unavailable := stateUnknown[bool]("layout-timeline-unavailable")
	if r.err != nil {
		return nil, unavailable, r.err
	}
	if p.Timeline == (RuleKey{}) || r.s.catalog == nil {
		return nil, unavailable, nil
	}
	rule, ok := r.s.catalog.rules[p.Timeline]
	if !ok {
		return nil, unavailable, nil
	}
	plan := r.s.catalog.plans[rule.Predicate].timeline
	if plan == nil {
		return nil, unavailable, nil
	}
	service, err := r.s.newContextService(r.ctx, key)
	if err != nil {
		return nil, unavailable, err
	}
	if service.Layout == nil {
		if err = r.s.retain(256); err != nil {
			return nil, unavailable, err
		}
		service.Layout = &contextLayoutService{}
	}
	if !service.Layout.BlocksReady {
		blocks, err := r.s.timelineBlocks(r.ctx, plan)
		if err != nil {
			return nil, unavailable, err
		}
		service.Layout.Blocks = blocks
		service.Layout.BlocksReady = true
	}
	return service.Layout.Blocks, Evidence[bool]{State: EvidenceKnown, Value: true}, nil
}
