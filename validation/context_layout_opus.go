package validation

import "context"

const (
	layoutOpusVersion contextRole = "layout.opus.version"
	layoutOpusScore   contextRole = "layout.opus.score"
	layoutOpusNewPage contextRole = "layout.opus.new-page"
)

// These roles compile against AssemblyOpus, never the same-spelled MusicXML
// link declaration. No linked score is opened or needed for this local hint.
func contextOpusLayoutBindingSpecs() []contextBindingSpec {
	return []contextBindingSpec{
		{Role: layoutOpusVersion, Owner: "opus", Name: "version", Type: "xs:token", Attribute: true, Count: 1},
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
		if r.attributeOwner(n, layoutOpusVersion) {
			raw := localRawAttribute(r, n, layoutOpusVersion)
			value := localDefault(raw, Atomic{Kind: AtomicString, Text: "1.0"})
			if r.err != nil {
				return Decision{}, false, r.err
			}
			if err := localPublishDefaulted(ctx, s, key, n, "layout.opus-version", raw, value, Evidence[bool]{State: EvidenceKnown, Value: true}, NodeRef{s.documentID, n.header.ID}, ContextUnitNone); err != nil {
				return Decision{}, false, err
			}
		}
		if !r.matches(n, layoutOpusScore) {
			continue
		}
		raw := localRawAttribute(r, n, layoutOpusNewPage)
		value := layoutUnknownDefault(raw, "application-opus-page-choice")
		if r.err != nil {
			return Decision{}, false, r.err
		}
		provenance := ContextExplicit
		if raw.State == EvidenceAbsent {
			provenance = ContextApplicationChoice
		}
		if err := localPublishWithProvenance(ctx, s, key, n, "layout.opus-new-page", raw, value, Evidence[bool]{State: EvidenceKnown, Value: true}, NodeRef{s.documentID, n.header.Parent}, ContextUnitNone, provenance); err != nil {
			return Decision{}, false, err
		}
	}
	return Decision{Outcome: OutcomePass}, false, r.err
}
