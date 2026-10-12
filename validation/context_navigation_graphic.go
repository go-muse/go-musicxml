package validation

import "context"

const (
	contextNavGraphicSegno      contextRole = "navigation-graphic-segno"
	contextNavGraphicCoda       contextRole = "navigation-graphic-coda"
	contextNavGraphicDirection  contextRole = "navigation-graphic-direction"
	contextNavGraphicSound      contextRole = "navigation-graphic-sound"
	contextNavGraphicSoundSegno contextRole = "navigation-graphic-sound-segno"
	contextNavGraphicSoundCoda  contextRole = "navigation-graphic-sound-coda"
)

func navigationGraphicBindings() []contextBindingSpec {
	return []contextBindingSpec{
		{Role: contextNavGraphicSegno, Owner: "direction-type", Name: "segno", Type: "segno", Count: 1},
		{Role: contextNavGraphicCoda, Owner: "direction-type", Name: "coda", Type: "coda", Count: 1},
		{Role: contextNavGraphicDirection, Name: "direction", Type: "direction", Count: -1},
		{Role: contextNavGraphicSound, Owner: "direction", Name: "sound", Type: "sound", Count: 1},
		{Role: contextNavGraphicSoundSegno, Owner: "sound", Name: "segno", Type: "xs:token", Attribute: true, Count: 1},
		{Role: contextNavGraphicSoundCoda, Owner: "sound", Name: "coda", Type: "xs:token", Attribute: true, Count: 1},
	}
}

// A graphic creates advice, never a control node. LocalPlaybackSound reports
// only an explicitly labelled sibling sound in its enclosing direction. The
// pinned prose does not authorize guessing a distant association by proximity.
func navigationPublishGraphics(ctx context.Context, s *Session, p *contextPlan, key RuleKey, timeline *timelineService) error {
	r := &contextReader{ctx: ctx, s: s, b: p.Bindings}
	for _, id := range s.sequence {
		n := s.nodes[id]
		kind := NavigationKind("")
		role := contextRole("")
		if r.matches(n, contextNavGraphicSegno) {
			kind = NavigationGraphicSegno
			role = contextNavGraphicSoundSegno
		}
		if r.matches(n, contextNavGraphicCoda) {
			kind = NavigationGraphicCoda
			role = contextNavGraphicSoundCoda
		}
		if kind == "" {
			continue
		}
		origin, err := contextRepeatOrigin(ctx, s, timeline, n)
		if err != nil {
			return err
		}
		v := NavigationContextResult{Kind: kind, Part: origin.Part, PlaybackSoundNeeded: navigationKnownBool(true), LocalPlaybackSound: stateUnknown[NodeRef]("navigation-graphic-association-outside-direction-unresolved")}
		for a := s.nodes[n.header.Parent]; a != nil; a = s.nodes[a.header.Parent] {
			if !r.step(1) {
				return r.err
			}
			if !r.matches(a, contextNavGraphicDirection) {
				continue
			}
			sound, present := r.child(a, contextNavGraphicSound)
			v.LocalPlaybackSound = stateUnavailable[bool, NodeRef](present)
			if present.State == EvidenceAbsent {
				v.LocalPlaybackSound = Evidence[NodeRef]{State: EvidenceAbsent}
			}
			if present.State == EvidenceKnown {
				label := stateText(r.attribute(sound, role))
				v.Label = label
				if label.State == EvidenceKnown {
					v.LocalPlaybackSound = Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{s.documentID, sound.header.ID}}
				} else if label.State == EvidenceAbsent {
					v.LocalPlaybackSound = Evidence[NodeRef]{State: EvidenceAbsent}
				} else {
					v.LocalPlaybackSound = stateUnavailable[string, NodeRef](label)
				}
			}
			break
		}
		if r.err != nil {
			return r.err
		}
		if err = s.publishContextResult(ctx, ContextResult{Provider: key, Subject: NodeRef{s.documentID, id}, Property: ContextPropertyNavigation, Navigation: &v}); err != nil {
			return err
		}
	}
	return r.err
}
