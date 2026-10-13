package validation

import (
	"context"
	"strings"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

func resourceHintAttribute(r *contextReader, n *factNode, name string, scratch *int64) ResourceAttribute {
	out := resourceAbsentAttribute()
	q := QName{Space: xsd.XSINamespace, Local: name}
	var a *AttributeFact
	for i := range n.attributes {
		if !r.step(1) {
			return out
		}
		if n.attributes[i].Name == q {
			a = &n.attributes[i]
			break
		}
	}
	if a == nil {
		if n.end.AttributesComplete.State != EvidenceKnown || !n.end.AttributesComplete.Value {
			out.Presence = stateUnavailable[bool, bool](n.end.AttributesComplete)
			if out.Presence.State == EvidenceKnown || out.Presence.State == EvidenceAbsent {
				out.Presence = stateUnknown[bool]("schema-hint-attributes-incomplete")
			}
			out.Raw = stateUnavailable[bool, string](out.Presence)
			out.Value = stateUnavailable[bool, Atomic](out.Presence)
		}
		return out
	}
	out.Presence = a.Presence
	out.Raw = a.Scalar.Lexeme
	if out.Presence.State != EvidenceKnown || !out.Presence.Value {
		out.Value = stateUnavailable[bool, Atomic](out.Presence)
		if out.Presence.State == EvidenceKnown {
			out.Value = Evidence[Atomic]{State: EvidenceAbsent}
		}
		return out
	}
	if out.Raw.State != EvidenceKnown {
		out.Value = stateUnavailable[string, Atomic](out.Raw)
		return out
	}
	// The kernel owns URI datatype validity. Discovery preserves any independent
	// invalid hint cause even when another attribute or declaration is unavailable.
	complete := true
	for _, check := range []XSDCheck{XSDCheckDeclaration, XSDCheckAttributes} {
		result := r.s.schemaResult(n, check)
		complete = complete && result.Complete
		for _, issue := range result.Issues {
			if !r.step(1) {
				return out
			}
			exactAttribute := issue.Attribute == (xsd.QName{Space: q.Space, Local: q.Local})
			if issue.Attribute == (xsd.QName{}) {
				if !r.step(int64(len(issue.Path)+len(n.header.Location.Path)+len(q.Space)+len(q.Local)) + 4) {
					return out
				}
				exactAttribute = resourceHintPathMatches(issue.Path, n.header.Location.Path, name)
			}
			if exactAttribute && issue.Subject == xsd.SubjectID(n.header.ID) {
				out.Value, r.err = r.s.schemaInvalidValue(r.ctx, n, check)
				return out
			}
		}
	}
	if !complete {
		out.Value = Evidence[Atomic]{State: EvidenceUnknown, Reason: "schema-hint-datatype-evidence-unavailable"}
		return out
	}
	if !r.step(1 + int64(len(out.Raw.Value))*2) {
		return out
	}
	if err := r.s.account.Charge(BudgetBufferedBytes, int64(len(out.Raw.Value))+32); err != nil {
		r.err = err
		return out
	}
	*scratch += int64(len(out.Raw.Value)) + 32
	normalized := xsd.CollapseWhitespace(out.Raw.Value)
	out.Value = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: normalized}}
	return out
}
func evaluateResourceHints(ctx context.Context, s *Session, _ *factNode, key RuleKey, p *contextPlan, final bool) (Decision, bool, error) {
	if !final {
		return Decision{}, true, nil
	}
	r := &contextReader{ctx: ctx, s: s, b: p.Bindings}
	if s.profile.Target == TargetModel {
		// The current model has no xsi input history. Do not copy a stale source
		// snapshot, claim known omission, or serialize it to fabricate hints.
		if s.root != nil {
			unavailable := Evidence[string]{State: EvidenceUnsupported, Reason: "model-schema-hints-not-retained"}
			value := &ResourceSchemaHintsContext{Raw: unavailable, Trailing: stateUnavailable[string, string](unavailable), Complete: stateUnavailable[string, bool](unavailable), NoNamespace: ResourceAttribute{Presence: stateUnavailable[string, bool](unavailable), Raw: unavailable, Value: stateUnavailable[string, Atomic](unavailable)}}
			shell := resourceShell(s, s.root, Evidence[NodeRef]{State: EvidenceAbsent})
			shell.SchemaHints = value
			if err := resourcePublish(r, key, s.root, ContextResourceSchemaHints, shell); err != nil {
				return Decision{}, false, err
			}
		}
		return Decision{Outcome: OutcomePass}, false, r.err
	}
	for _, id := range s.sequence {
		if !r.step(1) {
			return Decision{}, false, r.err
		}
		n := s.nodes[id]
		if n == nil {
			continue
		}
		if err := projectResourceHintNode(r, key, n); err != nil {
			return Decision{}, false, err
		}

	}
	return Decision{Outcome: OutcomePass}, false, r.err
}

func projectResourceHintNode(r *contextReader, key RuleKey, n *factNode) error {
	s := r.s
	var scratch int64
	defer func() {
		if scratch != 0 {
			_ = s.account.Release(BudgetBufferedBytes, scratch)
		}
	}()
	hint := resourceHintAttribute(r, n, "schemaLocation", &scratch)
	noNamespace := resourceHintAttribute(r, n, "noNamespaceSchemaLocation", &scratch)
	if n.header.Parent != 0 && hint.Presence.State == EvidenceAbsent && noNamespace.Presence.State == EvidenceAbsent {
		return r.err
	}
	value := &ResourceSchemaHintsContext{Raw: hint.Raw, Trailing: Evidence[string]{State: EvidenceAbsent}, Complete: Evidence[bool]{State: EvidenceKnown, Value: true}, NoNamespace: noNamespace}
	if hint.Value.State != EvidenceKnown && hint.Value.State != EvidenceAbsent {
		value.Complete = stateUnavailable[Atomic, bool](hint.Value)
		value.Trailing = stateUnavailable[Atomic, string](hint.Value)
	}
	if hint.Raw.State == EvidenceKnown {
		// Split only XML's four whitespace characters. Pre-admit the worst-case
		// pair headers before the bounded standard-library traversal/allocation.
		bytes := int64(len(hint.Raw.Value))
		if !r.step(1 + bytes*3) {
			return r.err
		}
		tokens := int64(0)
		inToken := false
		for _, ch := range hint.Raw.Value {
			white := ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
			if !white && !inToken {
				tokens++
			}
			inToken = !white
		}
		reserve, err := rationalCheckedMul(BudgetBufferedBytes, tokens+1, 512)
		if err != nil {
			return err
		}
		if err = s.account.Charge(BudgetBufferedBytes, reserve); err != nil {
			return err
		}
		scratch += reserve
		words := strings.FieldsFunc(hint.Raw.Value, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' })
		state := Evidence[string]{State: EvidenceKnown}
		if hint.Value.State != EvidenceKnown {
			state = stateUnavailable[Atomic, string](hint.Value)
		}
		origin := n.header.Location
		for _, attribute := range n.attributes {
			if !r.step(1) {
				return r.err
			}
			if attribute.Name == (QName{Space: xsd.XSINamespace, Local: "schemaLocation"}) {
				origin = attribute.Location
				break
			}
		}
		for i := 0; i+1 < len(words); i += 2 {
			if !r.step(1) {
				return r.err
			}
			ns, location := state, state
			if state.State == EvidenceKnown {
				ns.Value = words[i]
				location.Value = words[i+1]
			}
			value.Pairs = append(value.Pairs, SchemaHintPair{Namespace: ns, Location: location, Origin: origin})
		}
		if len(words)%2 != 0 {
			value.Trailing = state
			if state.State == EvidenceKnown {
				value.Trailing.Value = words[len(words)-1]
			}
			if hint.Value.State == EvidenceKnown {
				value.Complete = Evidence[bool]{State: EvidenceUnknown, Reason: "schema-location-unmatched-token"}
			}
		}
	}
	shell := resourceShell(s, n, Evidence[NodeRef]{State: EvidenceAbsent})
	shell.SchemaHints = value
	if err := resourcePublish(r, key, n, ContextResourceSchemaHints, shell); err != nil {
		return err
	}
	return r.err
}

// The old kernel hint diagnostic carries an exact slot path rather than a
// declared Attribute use. Compare its two fixed suffixes without constructing
// an unadmitted temporary path for every diagnostic.
func resourceHintPathMatches(path, owner, name string) bool {
	suffix := "/@{http://www.w3.org/2001/XMLSchema-instance}schemaLocation"
	if name == "noNamespaceSchemaLocation" {
		suffix = "/@{http://www.w3.org/2001/XMLSchema-instance}noNamespaceSchemaLocation"
	}
	return len(path) == len(owner)+len(suffix) && strings.HasPrefix(path, owner) && strings.HasSuffix(path, suffix)
}
