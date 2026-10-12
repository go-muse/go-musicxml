package validation

import "context"

// NavigationListeningKind is the closed continuing-listening instruction family.
type NavigationListeningKind string

const (
	// NavigationListeningSync selects synchronization behavior.
	NavigationListeningSync NavigationListeningKind = "sync"
	// NavigationListeningOther preserves extension-defined listening behavior.
	NavigationListeningOther NavigationListeningKind = "other-listening"
)

// NavigationListeningStatement is a player-qualified instruction. Absent Player
// is symbolic all-players scope. Active and fields preserve independent causes.
type NavigationListeningStatement struct {
	PlayerTarget Evidence[NodeRef]
	Source       NodeRef
	Kind         NavigationListeningKind
	Player       Evidence[string]
	Type         Evidence[string]
	Latency      Evidence[Decimal]
	Value        Evidence[string]
	TimeOnly     Evidence[string]
	Active       Evidence[bool]
}

const (
	contextNavListening      contextRole = "navigation-listening"
	contextNavSync           contextRole = "navigation-sync"
	contextNavOtherListening contextRole = "navigation-other-listening"
	contextNavSyncType       contextRole = "navigation-sync-type"
	contextNavSyncPlayer     contextRole = "navigation-sync-player"
	contextNavSyncTime       contextRole = "navigation-sync-time"
	contextNavSyncLatency    contextRole = "navigation-sync-latency"
	contextNavOtherType      contextRole = "navigation-other-type"
	contextNavOtherPlayer    contextRole = "navigation-other-player"
	contextNavOtherTime      contextRole = "navigation-other-time"
)

func navigationListeningBindingSpecs() []contextBindingSpec {
	return append([]contextBindingSpec{
		{Role: contextNavListening, Name: "listening", Type: "listening", Count: -1},
		{Role: contextNavSync, Owner: "listening", Name: "sync", Type: "sync", Count: 1},
		{Role: contextNavOtherListening, Owner: "listening", Name: "other-listening", Type: "other-listening", Count: 1},
		{Role: contextNavSyncType, Owner: "sync", Name: "type", Type: "sync-type", Attribute: true, Count: 1},
		{Role: contextNavSyncPlayer, Owner: "sync", Name: "player", Type: "xs:IDREF", Attribute: true, Count: 1},
		{Role: contextNavSyncTime, Owner: "sync", Name: "time-only", Type: "time-only", Attribute: true, Count: 1},
		{Role: contextNavSyncLatency, Owner: "sync", Name: "latency", Type: "milliseconds", Attribute: true, Count: 1},
		{Role: contextNavOtherType, Owner: "other-listening", Name: "type", Type: "xs:token", Attribute: true, Count: 1},
		{Role: contextNavOtherPlayer, Owner: "other-listening", Name: "player", Type: "xs:IDREF", Attribute: true, Count: 1},
		{Role: contextNavOtherTime, Owner: "other-listening", Name: "time-only", Type: "time-only", Attribute: true, Count: 1},
	}, navigationPlayerBindingSpecs()...)
}

func navigationReadListening(r *contextReader, n *factNode, part Evidence[NodeRef]) ([]NavigationListeningStatement, error) {
	var out []NavigationListeningStatement
	for _, ch := range n.children {
		kind := NavigationListeningKind("")
		typeRole, playerRole, timeRole := contextRole(""), contextRole(""), contextRole("")
		switch {
		case r.matches(ch, contextNavSync):
			kind = NavigationListeningSync
			typeRole = contextNavSyncType
			playerRole = contextNavSyncPlayer
			timeRole = contextNavSyncTime
		case r.matches(ch, contextNavOtherListening):
			kind = NavigationListeningOther
			typeRole = contextNavOtherType
			playerRole = contextNavOtherPlayer
			timeRole = contextNavOtherTime
		}
		if kind == "" {
			continue
		}
		v := NavigationListeningStatement{Source: NodeRef{r.s.documentID, ch.header.ID}, Kind: kind, Player: stateText(r.attribute(ch, playerRole)), Type: stateText(r.attribute(ch, typeRole)), TimeOnly: stateText(r.attribute(ch, timeRole)), Latency: Evidence[Decimal]{State: EvidenceAbsent}, Value: Evidence[string]{State: EvidenceAbsent}, Active: stateUnknown[bool]("navigation-pass-domain-unavailable")}
		v.PlayerTarget = navigationResolvePlayer(r, part, v.Player)
		if v.TimeOnly.State == EvidenceAbsent {
			v.Active = navigationKnownBool(true)
		}
		if kind == NavigationListeningSync {
			v.Latency = stateDecimalValue(r.attribute(ch, contextNavSyncLatency))
		} else {
			v.Value = stateText(r.scalar(ch))
		}
		if _, err := stateReserve(r.ctx, r.s, v); err != nil {
			return nil, err
		}
		out = append(out, stateClone(v))
	}
	return out, r.err
}

// Updates are stored as sparse symbolic all-player baselines and specific
// overrides. An unknown player cannot silently become all players.
func navigationApplyListening(ctx context.Context, s *Session, current []NavigationListeningStatement, updates []NavigationListeningStatement, pass uint64) ([]NavigationListeningStatement, error) {
	for index, raw := range updates {
		guard, err := navigationPassApplies(ctx, s, raw.TimeOnly, pass)
		if err != nil {
			return nil, err
		}
		if guard.State == EvidenceKnown && !guard.Value {
			continue
		}
		for _, prior := range updates[:index] {
			if err = s.step(ctx, 1); err != nil {
				return nil, err
			}
			if prior.Kind != raw.Kind {
				continue
			}
			if raw.Kind == NavigationListeningOther && prior.Type.State == EvidenceKnown && raw.Type.State == EvidenceKnown {
				if err = s.step(ctx, int64(len(prior.Type.Value)+len(raw.Type.Value))); err != nil {
					return nil, err
				}
				if prior.Type.Value != raw.Type.Value {
					continue
				}
			}
			before, e := navigationPassApplies(ctx, s, prior.TimeOnly, pass)
			if e != nil {
				return nil, e
			}
			if before.State == EvidenceKnown && !before.Value {
				continue
			}
			if prior.PlayerTarget.State == EvidenceKnown && raw.PlayerTarget.State == EvidenceKnown {
				same, e := stateSameRef(ctx, s, prior.PlayerTarget.Value, raw.PlayerTarget.Value)
				if e != nil {
					return nil, e
				}
				if !same {
					continue
				}
			}
			guard = stateMergeGap(guard, stateUnknown[bool]("navigation-simultaneous-listening-precedence-unresolved"))
		}
		next := raw
		next.Active = guard
		next.Type = navigationConditionalValue(next.Type, guard)
		next.Latency = navigationConditionalValue(next.Latency, guard)
		next.Value = navigationConditionalValue(next.Value, guard)
		if next.Player.State != EvidenceAbsent && next.PlayerTarget.State != EvidenceKnown {
			next.Active = stateMergeGap(next.Active, stateUnavailable[NodeRef, bool](next.PlayerTarget))
			next.Type = stateUnavailable[bool, string](next.Active)
			next.Latency = stateUnavailable[bool, Decimal](next.Active)
			next.Value = stateUnavailable[bool, string](next.Active)
		}
		write := 0
		for _, old := range current {
			if err = s.step(ctx, 1+int64(len(old.Player.Value)+len(next.Player.Value))); err != nil {
				return nil, err
			}
			sameKind := old.Kind == next.Kind
			if sameKind && next.Kind == NavigationListeningOther {
				sameKind = old.Type.State == EvidenceKnown && raw.Type.State == EvidenceKnown && old.Type.Value == raw.Type.Value
			}
			replaces := sameKind && (next.Player.State == EvidenceAbsent || next.Player.State == EvidenceKnown && old.Player.State == EvidenceKnown && next.Player.Value == old.Player.Value)
			if !replaces {
				current[write] = old
				write++
			}
		}
		current = current[:write]
		if _, err = stateReserve(ctx, s, next); err != nil {
			return nil, err
		}
		current = append(current, stateClone(next))
	}
	return current, nil
}

// Distinctness is the pinned attribute-value advice, not an assertion that
// selector sets are disjoint or that conflicting updates have a precedence.
func navigationListeningDistinct(r *contextReader, n *factNode, rows []NavigationListeningStatement) Evidence[bool] {
	result, err := navigationDistinctTargets(r.ctx, r.s, len(rows), r.complete(n), func(i int) (string, Evidence[string], Evidence[string]) {
		v := rows[i]
		return string(v.Kind), v.Player, v.TimeOnly
	})
	if err != nil {
		r.err = err
		return stateUnknown[bool]("context-interrupted")
	}
	return result
}

// Known attribute targets use one hash lookup. Only unavailable targets need
// the conservative pairwise fallback; each pair, even differing presence/kind,
// is charged before comparison. A definite duplicate settles the advice.
func navigationDistinctTargets(ctx context.Context, s *Session, count int, result Evidence[bool], at func(int) (string, Evidence[string], Evidence[string])) (Evidence[bool], error) {
	free, err := contextRepeatScratch(ctx, s, count, 256)
	if err != nil {
		return Evidence[bool]{}, err
	}
	defer free()
	type key struct {
		kind   string
		ps, ts EvidenceState
		p, t   string
	}
	known := make(map[key]bool)
	var uncertain []int
	for i := 0; i < count; i++ {
		kind, p, t := at(i)
		if err = s.step(ctx, 1+int64(len(kind)+len(p.Value)+len(t.Value))); err != nil {
			return Evidence[bool]{}, err
		}
		exact := (p.State == EvidenceKnown || p.State == EvidenceAbsent) && (t.State == EvidenceKnown || t.State == EvidenceAbsent)
		candidates := uncertain
		if exact {
			k := key{kind, p.State, t.State, p.Value, t.Value}
			if known[k] {
				return navigationKnownBool(false), nil
			}
			known[k] = true
		}
		end := len(candidates)
		if !exact {
			end = i
		}
		for j := 0; j < end; j++ {
			if err = s.step(ctx, 1); err != nil {
				return Evidence[bool]{}, err
			}
			index := j
			if exact {
				index = candidates[j]
			}
			other, op, ot := at(index)
			if other != kind {
				continue
			}
			same, e := navigationSameTarget(ctx, s, op, ot, p, t)
			if e != nil {
				return Evidence[bool]{}, e
			}
			if same.State == EvidenceKnown && same.Value {
				return navigationKnownBool(false), nil
			}
			if same.State != EvidenceKnown {
				result = stateMergeGap(result, same)
			}
		}
		if !exact {
			uncertain = append(uncertain, i)
		}
	}
	return result, nil
}
