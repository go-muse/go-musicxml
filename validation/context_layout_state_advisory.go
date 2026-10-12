package validation

import "context"

// This recommendation never changes conformance. Its applicability uses the
// accepted effective clef, not a document-order or visual-coordinate guess.
func layoutStatePercussionAdvice(ctx context.Context, s *Session, key RuleKey, n *factNode, r *contextReader, p *contextPlan) error {
	fact := layoutStateFact(r, p, n)
	app := timelineGapAs[musicalPosition, bool](fact.Position, "percussion-clef-position-unavailable")
	_, rest := r.child(n, layoutStateRole("rest"))
	if rest.State == EvidenceKnown {
		app = Evidence[bool]{State: EvidenceKnown, Value: false}
	} else if rest.State != EvidenceAbsent {
		app = timelineGapAs[bool, bool](rest, "note-kind-unavailable")
	} else if service := s.timeline[p.Timeline]; service != nil && fact.Position.State == EvidenceKnown {
		staff, e := service.State.resolveStaff(ctx, s, fact.Position.Value, fact.Staff)
		if e != nil {
			return e
		}
		clef, e := queryState(ctx, s, &service.State, &service.State.Clefs, fact.Position.Value, staff, stateWhole[clefState])
		if e != nil {
			return e
		}
		if clef.Value.State == EvidenceKnown {
			sign := clef.Value.Value.Sign
			app = timelineGapAs[string, bool](sign, "percussion-clef-sign-unavailable")
			if sign.State == EvidenceKnown {
				app = Evidence[bool]{State: EvidenceKnown, Value: sign.Value == "percussion"}
			}
		} else {
			app = timelineGapAs[clefState, bool](clef.Value, "percussion-effective-clef-not-encoded")
		}
	}
	return localPublish(ctx, s, key, n, "layout.advisory-percussion-unpitched", Evidence[Atomic]{State: EvidenceAbsent}, Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "use-unpitched-for-notes-in-percussion-clef"}}, app, NodeRef{}, ContextUnitNone)
}
