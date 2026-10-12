package validation

import "context"

func evaluateGraphicalRepeatContent(ctx context.Context, s *Session, n *factNode, p *contextPlan, final bool) (Decision, bool, error) {
	if !final {
		return Decision{}, true, nil
	}
	r := &contextReader{ctx: ctx, s: s, b: p.Bindings}
	if n.header.Declaration.State != EvidenceKnown {
		return localGap(n.header.Declaration), false, nil
	}
	role := contextRole("")
	switch {
	case r.matches(n, contextBeatRepeat):
		role = contextBeatType
	case r.matches(n, contextMeasureRepeat):
		role = contextMeasureType
	}
	if r.err != nil {
		return Decision{}, false, r.err
	}
	if role == "" {
		return Decision{Outcome: OutcomeNotApplicable}, false, nil
	}
	typ := stateText(r.attribute(n, role))
	if r.err != nil {
		return Decision{}, false, r.err
	}
	if typ.State != EvidenceKnown {
		return localGap(typ), false, nil
	}
	if typ.Value == "stop" {
		return Decision{Outcome: OutcomeNotApplicable}, false, nil
	}
	if typ.Value != "start" {
		return Decision{Outcome: OutcomeUnknown, Reason: "context-repeat-transition-unavailable"}, false, nil
	}
	service := s.contextServices[p.Provider]
	if service == nil || service.Repeats == nil || !service.Repeats.Final {
		return Decision{Outcome: OutcomeUnknown, Reason: "context-repeat-provider-unavailable"}, false, nil
	}
	if err := s.step(ctx, 1); err != nil {
		return Decision{}, false, err
	}
	index, ok := service.Repeats.ByNode[n.header.ID]
	if !ok {
		return Decision{Outcome: OutcomeUnknown, Reason: "context-repeat-content-unavailable"}, false, nil
	}
	value := service.Repeats.Records[index].value.ContentEquivalent
	if value.State != EvidenceKnown {
		return localGap(value), false, nil
	}
	if value.Value {
		return Decision{Outcome: OutcomePass}, false, nil
	}
	return Decision{Outcome: OutcomeFail, Reason: "context-repeat-encoded-content-missing"}, false, nil
}
