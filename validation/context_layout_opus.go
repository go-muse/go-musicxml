package validation

import "context"

const (
	layoutOpusScore   contextRole = "layout.opus.score"
	layoutOpusNewPage contextRole = "layout.opus.new-page"
)

// These roles compile against AssemblyOpus, never the same-spelled MusicXML
// link declaration. No linked score is opened or needed for this local hint.
func contextOpusLayoutBindingSpecs() []contextBindingSpec {
	return []contextBindingSpec{
		{Role: layoutOpusScore, Owner: "opus", Name: "score", Type: "score", Count: 1},
		{Role: layoutOpusNewPage, Owner: "score", Name: "new-page", Type: "yes-no", Attribute: true, Count: 1},
	}
}
func evaluateContextOpusLayout(ctx context.Context, s *Session, _ *factNode, key RuleKey, p *contextPlan, final bool) (Decision, bool, error) {
	if !final {
		return Decision{}, true, nil
	}
	r := &contextReader{ctx: ctx, s: s, b: p.Bindings}
	for _, id := range s.sequence {
		if !r.step(1) {
			return Decision{}, false, r.err
		}
		n := s.nodes[id]
		if !r.matches(n, layoutOpusScore) {
			continue
		}
		raw := localRawAttribute(r, n, layoutOpusNewPage)
		value := layoutUnknownDefault(raw, "application-opus-page-choice")
		if r.err != nil {
			return Decision{}, false, r.err
		}
		if err := localPublish(ctx, s, key, n, "layout.opus-new-page", raw, value, Evidence[bool]{State: EvidenceKnown, Value: true}, NodeRef{s.documentID, n.header.Parent}, ContextUnitNone); err != nil {
			return Decision{}, false, err
		}
	}
	return Decision{Outcome: OutcomePass}, false, r.err
}
