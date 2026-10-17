package validation

import "context"

// NavigationNoteListenKind identifies a note-local listening/assessment carrier.
type NavigationNoteListenKind string

// Supported NavigationNoteListenKind values.
const (
	// NavigationAssess is an explicit per-note assessment override.
	NavigationAssess NavigationNoteListenKind = "assess"
	// NavigationWait is a performer wait point.
	NavigationWait NavigationNoteListenKind = "wait"
	// NavigationOtherListen preserves an extension-defined note interaction.
	NavigationOtherListen NavigationNoteListenKind = "other-listen"
)

// NavigationNoteListening retains target scopes without choosing an application's
// assessment policy. RecommendedAssess remains advice, separate from Override.
type NavigationNoteListening struct {
	PlayerTarget                        Evidence[NodeRef]
	Source                              NodeRef
	Kind                                NavigationNoteListenKind
	Player, TimeOnly                    Evidence[string]
	AllPlayers, AllPasses               Evidence[bool]
	Override, RecommendedAssess, Active Evidence[bool]
	OtherType, Value                    Evidence[string]
}

const (
	contextNavNoteListen        contextRole = "navigation-note-listen"
	contextNavNoteCue           contextRole = "navigation-note-cue"
	contextNavAssess            contextRole = "navigation-assess"
	contextNavWait              contextRole = "navigation-wait"
	contextNavOtherListen       contextRole = "navigation-other-listen"
	contextNavAssessType        contextRole = "navigation-assess-type"
	contextNavAssessPlayer      contextRole = "navigation-assess-player"
	contextNavAssessTime        contextRole = "navigation-assess-time"
	contextNavWaitPlayer        contextRole = "navigation-wait-player"
	contextNavWaitTime          contextRole = "navigation-wait-time"
	contextNavOtherListenType   contextRole = "navigation-other-listen-type"
	contextNavOtherListenPlayer contextRole = "navigation-other-listen-player"
	contextNavOtherListenTime   contextRole = "navigation-other-listen-time"
)

func navigationNoteListeningBindings() []contextBindingSpec {
	return []contextBindingSpec{
		{Role: contextNavNoteListen, Owner: "note", Name: "listen", Type: "listen", Count: 1},
		{Role: contextNavNoteCue, Owner: "note", Name: "cue", Type: "empty", Count: 2},
		{Role: contextNavAssess, Owner: "listen", Name: "assess", Type: "assess", Count: 1},
		{Role: contextNavWait, Owner: "listen", Name: "wait", Type: "wait", Count: 1},
		{Role: contextNavOtherListen, Owner: "listen", Name: "other-listen", Type: "other-listening", Count: 1},
		{Role: contextNavAssessType, Owner: "assess", Name: "type", Type: "yes-no", Attribute: true, Count: 1},
		{Role: contextNavAssessPlayer, Owner: "assess", Name: "player", Type: "xs:IDREF", Attribute: true, Count: 1},
		{Role: contextNavAssessTime, Owner: "assess", Name: "time-only", Type: "time-only", Attribute: true, Count: 1},
		{Role: contextNavWaitPlayer, Owner: "wait", Name: "player", Type: "xs:IDREF", Attribute: true, Count: 1},
		{Role: contextNavWaitTime, Owner: "wait", Name: "time-only", Type: "time-only", Attribute: true, Count: 1},
		{Role: contextNavOtherListenType, Owner: "other-listening", Name: "type", Type: "xs:token", Attribute: true, Count: 1},
		{Role: contextNavOtherListenPlayer, Owner: "other-listening", Name: "player", Type: "xs:IDREF", Attribute: true, Count: 1},
		{Role: contextNavOtherListenTime, Owner: "other-listening", Name: "time-only", Type: "time-only", Attribute: true, Count: 1},
	}
}
func navigationScopeAll(v Evidence[string]) Evidence[bool] {
	if v.State == EvidenceAbsent {
		return navigationKnownBool(true)
	}
	if v.State == EvidenceKnown {
		return navigationKnownBool(false)
	}
	return stateUnavailable[string, bool](v)
}
func navigationReadNoteListening(r *contextReader, n *factNode, part Evidence[NodeRef]) ([]NavigationNoteListening, Evidence[bool], Evidence[bool], error) {
	_, cue := r.child(n, contextNavNoteCue)
	recommended := stateUnavailable[bool, bool](cue)
	if cue.State == EvidenceAbsent {
		recommended = navigationKnownBool(true)
	} else if cue.State == EvidenceKnown {
		recommended = navigationKnownBool(false)
	}
	listen, present := r.child(n, contextNavNoteListen)
	if present.State == EvidenceAbsent {
		return nil, recommended, navigationKnownBool(true), r.err
	}
	if present.State != EvidenceKnown {
		return nil, recommended, present, r.err
	}
	var out []NavigationNoteListening
	distinct := r.complete(listen)
	for _, ch := range listen.children {
		kind := NavigationNoteListenKind("")
		player, time := contextRole(""), contextRole("")
		switch {
		case r.matches(ch, contextNavAssess):
			kind = NavigationAssess
			player = contextNavAssessPlayer
			time = contextNavAssessTime
		case r.matches(ch, contextNavWait):
			kind = NavigationWait
			player = contextNavWaitPlayer
			time = contextNavWaitTime
		case r.matches(ch, contextNavOtherListen):
			kind = NavigationOtherListen
			player = contextNavOtherListenPlayer
			time = contextNavOtherListenTime
		}
		if kind == "" {
			continue
		}
		v := NavigationNoteListening{Source: NodeRef{r.s.documentID, ch.header.ID}, Kind: kind, Player: stateText(r.attribute(ch, player)), TimeOnly: stateText(r.attribute(ch, time)), Override: Evidence[bool]{State: EvidenceAbsent}, RecommendedAssess: recommended, OtherType: Evidence[string]{State: EvidenceAbsent}, Value: Evidence[string]{State: EvidenceAbsent}, Active: stateUnknown[bool]("navigation-pass-domain-unavailable")}
		v.PlayerTarget = navigationResolvePlayer(r, part, v.Player)
		v.AllPlayers = navigationScopeAll(v.Player)
		v.AllPasses = navigationScopeAll(v.TimeOnly)
		if v.TimeOnly.State == EvidenceAbsent {
			v.Active = navigationKnownBool(true)
		}
		if kind == NavigationAssess {
			v.Override = stateYesNo(r.attribute(ch, contextNavAssessType))
		}
		if kind == NavigationOtherListen {
			v.OtherType = stateText(r.attribute(ch, contextNavOtherListenType))
			v.Value = stateText(r.scalar(ch))
		}
		if _, err := stateReserve(r.ctx, r.s, v); err != nil {
			return nil, recommended, distinct, err
		}
		out = append(out, stateClone(v))
	}
	if r.err != nil {
		return nil, recommended, distinct, r.err
	}
	distinct, err := navigationDistinctTargets(r.ctx, r.s, len(out), distinct, func(i int) (string, Evidence[string], Evidence[string]) {
		v := out[i]
		return string(v.Kind), v.Player, v.TimeOnly
	})
	return out, recommended, distinct, err
}
func navigationSameTarget(ctx context.Context, s *Session, ap, at, bp, bt Evidence[string]) (Evidence[bool], error) {
	if err := s.step(ctx, 1); err != nil {
		return Evidence[bool]{}, err
	}
	result := navigationKnownBool(true)
	for _, pair := range [][2]Evidence[string]{{ap, bp}, {at, bt}} {
		for _, v := range pair {
			if v.State != EvidenceKnown && v.State != EvidenceAbsent {
				result = stateMergeGap(result, stateUnavailable[string, bool](v))
			}
		}
		if result.State != EvidenceKnown {
			continue
		}
		if pair[0].State != pair[1].State {
			return navigationKnownBool(false), nil
		}
		if err := s.step(ctx, int64(len(pair[0].Value)+len(pair[1].Value))+1); err != nil {
			return Evidence[bool]{}, err
		}
		if pair[0].State == EvidenceKnown && pair[0].Value != pair[1].Value {
			return navigationKnownBool(false), nil
		}
	}
	return result, nil
}
func navigationNoteListeningAt(ctx context.Context, s *Session, raw []NavigationNoteListening, pass uint64, parent Evidence[bool]) ([]NavigationNoteListening, error) {
	if _, err := stateReserve(ctx, s, raw); err != nil {
		return nil, err
	}
	out := stateClone(raw)
	for i := range out {
		guard, err := navigationPassApplies(ctx, s, out[i].TimeOnly, pass)
		if err != nil {
			return nil, err
		}
		if parent.State == EvidenceKnown && !parent.Value || guard.State == EvidenceKnown && !guard.Value {
			out[i].Active = navigationKnownBool(false)
		} else {
			out[i].Active = stateMergeGap(parent, guard)
		}
	}
	return out, nil
}
