package validation

import (
	"context"
	"strings"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

func resourceAbsentAttribute() ResourceAttribute {
	return ResourceAttribute{Presence: Evidence[bool]{State: EvidenceAbsent}, Raw: Evidence[string]{State: EvidenceAbsent}, Value: Evidence[Atomic]{State: EvidenceAbsent}}
}
func resourceAttribute(r *contextReader, n *factNode, role contextRole) ResourceAttribute {
	out := resourceAbsentAttribute()
	out.Value = r.attribute(n, role)
	selector, ok := r.attributeSelector(n, role)
	if !ok {
		out.Presence = stateUnavailable[Atomic, bool](out.Value)
		out.Raw = stateUnavailable[Atomic, string](out.Value)
		return out
	}
	presence, attribute, err := r.s.attributeEvidence(r.ctx, n, selector)
	if err != nil {
		r.err = err
		return out
	}
	// Atomic's existing scalar bridge deliberately does not erase primitive
	// anyURI identity globally. This exact resource slot exports the shared
	// kernel's already assessed normalized URI text without revalidating it.
	if out.Value.State == EvidenceUnsupported && out.Value.Reason == "typed-scalar-unavailable" && r.s.schema != nil && r.s.schema.run != nil {
		bound := r.s.schema.nodes[n.header.ID]
		for result := range r.s.schema.run.Attributes(bound.binding) {
			if !r.step(1) {
				return out
			}
			if result.Name != (xsd.QName{Space: selector.Name.Space, Local: selector.Name.Local}) || r.s.schema.active.declaration(result.Use) != selector.Declaration {
				continue
			}
			scalar := result.Scalar
			if !result.Violation && result.TypeValid && result.DeclarationValid && result.Complete && scalar.Known && scalar.Complete && scalar.Value.Domain == xsd.LexicalDomain && scalar.Value.Primitive == "anyURI" {
				if !r.step(1 + int64(len(scalar.Value.Text))) {
					return out
				}
				out.Value = Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: scalar.Value.Text}}
			}
		}
	}
	if r.sourceInfoset {
		out.Presence, out.Raw, r.err = r.s.sourcePhysicalAttribute(r.ctx, n, attribute, presence)
		return out
	}
	out.Presence = presence
	if attribute != nil {
		out.Raw = attribute.Scalar.Lexeme
	} else {
		out.Raw = stateUnavailable[bool, string](presence)
	}
	return out
}
func resourcePublish(r *contextReader, key RuleKey, n *factNode, property ContextProperty, value *ResourceContextResult) error {
	if r.err != nil {
		return r.err
	}
	return r.s.publishContextResult(r.ctx, ContextResult{Provider: key, Subject: NodeRef{r.s.documentID, n.header.ID}, Property: property, Value: Evidence[Atomic]{State: EvidenceAbsent}, Origins: []Location{n.header.Location}, Resource: value})
}
func resourceShell(s *Session, n *factNode, part Evidence[NodeRef]) *ResourceContextResult {
	owner := n.header.Parent
	if owner == 0 {
		owner = n.header.ID
	}
	return &ResourceContextResult{SubjectDeclaration: n.header.Declaration, Owner: NodeRef{s.documentID, owner}, Part: part}
}
func resourceString(e Evidence[Atomic]) Evidence[string] {
	if e.State != EvidenceKnown {
		return stateUnavailable[Atomic, string](e)
	}
	if e.Value.Kind != AtomicString {
		return Evidence[string]{State: EvidenceUnsupported, Reason: "resource-string-domain-unavailable"}
	}
	return Evidence[string]{State: EvidenceKnown, Value: e.Value.Text}
}
func resourceDecisionNode(s *Session, n *factNode, d Decision) Evidence[NodeRef] {
	if d.Outcome == OutcomePass && n != nil {
		return Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{s.documentID, n.header.ID}}
	}
	out := Evidence[NodeRef]{State: EvidenceUnknown, Reason: d.Reason, Cause: d.Cause}
	switch d.Outcome {
	case OutcomeBlocked, OutcomeFail:
		out.State = EvidenceInvalid
	case OutcomeUnsupported:
		out.State = EvidenceUnsupported
	case OutcomeNotApplicable:
		out.State = EvidenceAbsent
	}
	return out
}
func resourceInstrumentTarget(r *contextReader, n *factNode) Evidence[NodeRef] {
	key := RuleKey{ID: "MX40-LOCAL-instrument-link-target/local", Revision: 1}
	d, ok := r.s.lookupDecision(decisionID{key, n.header.ID})
	if !ok {
		return Evidence[NodeRef]{State: EvidenceUnknown, Reason: "local-instrument-target-provider-unavailable"}
	}
	if d.Outcome != OutcomePass {
		if d.Cause == nil && d.Outcome == OutcomeFail {
			d.Cause = &DecisionRef{Rule: key, Subject: NodeRef{r.s.documentID, n.header.ID}}
		}
		return resourceDecisionNode(r.s, nil, d)
	}
	selector, ok := r.attributeSelector(n, resourceRole("instrument-link.@id"))
	if !ok {
		return Evidence[NodeRef]{State: EvidenceUnsupported, Reason: "instrument-link-slot-unavailable"}
	}
	target, decision, err := r.s.referenceResolution(r.ctx, n, selector)
	if err != nil {
		r.err = err
	}
	return resourceDecisionNode(r.s, target, decision)
}
func evaluateResourceLinks(ctx context.Context, s *Session, _ *factNode, key RuleKey, p *contextPlan, final bool) (Decision, bool, error) {
	if !final {
		return Decision{}, true, nil
	}
	r := &contextReader{ctx: ctx, s: s, b: p.Bindings, sourceInfoset: p.sourceInfoset}
	index := newResourceLinkIndex(r)
	defer index.release(s)
	if r.err != nil {
		return Decision{}, false, r.err
	}
	for _, id := range s.sequence {
		if !r.step(1) {
			return Decision{}, false, r.err
		}
		n := s.nodes[id]
		if n == nil {
			continue
		}
		roles := r.roles(n)
		for _, role := range roles {
			shell := resourceShell(s, n, index.parts[id])
			var kind ResourceLinkKind
			owner := ""
			switch role {
			case resourceRole("link"):
				kind = ResourceScoreLink
				owner = "link"
			case resourceRole("bookmark"):
				kind = ResourceBookmark
				owner = "bookmark"
			case resourceRole("part-link"):
				kind = ResourcePartLink
				owner = "part-link"
			case resourceRole("opus"):
				kind = ResourceScoreWorkOpus
				owner = "opus"
			case resourceRole("score"):
				kind = ResourceOpusScore
				owner = "score"
			case resourceRole("opus-link"):
				kind = ResourceOpusLink
				owner = "opus-link"
			case resourceRole("image"), resourceRole("credit-image"):
				image := &ResourceImageContext{Credit: role == resourceRole("credit-image"), Source: resourceAttribute(r, n, resourceRole("image.@source")), MediaType: resourceAttribute(r, n, resourceRole("image.@type")), Width: resourceAttribute(r, n, resourceRole("image.@width")), Height: resourceAttribute(r, n, resourceRole("image.@height"))}
				image.Scaling = resourceImageScaling(image.Width.Value, image.Height.Value)
				shell.Image = image
				if err := resourcePublish(r, key, n, ContextResourceImage, shell); err != nil {
					return Decision{}, false, err
				}
			case resourceRole("group"), resourceRole("group-link"):
				shell.Group = &ResourceGroupContext{Linked: role == resourceRole("group-link"), Value: resourceString(r.scalar(n))}
				if err := resourcePublish(r, key, n, ContextResourceGroup, shell); err != nil {
					return Decision{}, false, err
				}
			case resourceRole("instrument-link"):
				shell.Instrument = &ResourceInstrumentContext{ID: resourceAttribute(r, n, resourceRole("instrument-link.@id")), Target: resourceInstrumentTarget(r, n)}
				if err := resourcePublish(r, key, n, ContextResourceInstrument, shell); err != nil {
					return Decision{}, false, err
				}
			}
			if kind == "" {
				continue
			}
			link := &ResourceLinkContext{PresentationIntent: Evidence[string]{State: EvidenceAbsent}, ActuationIntent: Evidence[string]{State: EvidenceAbsent}, Kind: kind, Href: resourceAbsentAttribute(), Show: resourceAbsentAttribute(), Actuate: resourceAbsentAttribute(), Element: resourceAbsentAttribute(), Position: resourceAbsentAttribute(), Attachment: Evidence[NodeRef]{State: EvidenceAbsent}, Fragment: Evidence[string]{State: EvidenceAbsent}}
			if kind != ResourceBookmark {
				link.Href = resourceAttribute(r, n, resourceRole(owner+".@href"))
				link.Show = resourceAttribute(r, n, resourceRole(owner+".@show"))
				link.Actuate = resourceAttribute(r, n, resourceRole(owner+".@actuate"))
				link.PresentationIntent = resourceXLinkIntent(link.Show.Value, true)
				link.ActuationIntent = resourceXLinkIntent(link.Actuate.Value, false)
				href := resourceString(link.Href.Value)
				if href.State != EvidenceKnown {
					link.Fragment = stateUnavailable[string, string](href)
				} else {
					if !r.step(int64(len(href.Value)) + 1) {
						return Decision{}, false, r.err
					}
					if split := strings.IndexByte(href.Value, '#'); split >= 0 {
						link.Fragment = Evidence[string]{State: EvidenceKnown, Value: href.Value[split+1:]}
					}
				}
			}
			if kind == ResourceBookmark || kind == ResourceScoreLink {
				link.Element = resourceAttribute(r, n, resourceRole(owner+".@element"))
				link.Position = resourceAttribute(r, n, resourceRole(owner+".@position"))
				link.Attachment = index.attachment(r, n, link.Element.Value, link.Position.Value)
			}
			shell.Link = link
			if err := resourcePublish(r, key, n, ContextResourceLink, shell); err != nil {
				return Decision{}, false, err
			}
		}
	}
	return Decision{Outcome: OutcomePass}, false, r.err
}
func resourceImageScaling(width, height Evidence[Atomic]) Evidence[string] {
	for _, e := range []Evidence[Atomic]{width, height} {
		if e.State != EvidenceKnown && e.State != EvidenceAbsent {
			return stateUnavailable[Atomic, string](e)
		}
	}
	switch {
	case width.State == EvidenceKnown && height.State == EvidenceKnown:
		return Evidence[string]{State: EvidenceKnown, Value: "independent-x-y"}
	case width.State == EvidenceKnown || height.State == EvidenceKnown:
		return Evidence[string]{State: EvidenceKnown, Value: "proportional"}
	default:
		return Evidence[string]{State: EvidenceAbsent}
	}
}

func resourceXLinkIntent(e Evidence[Atomic], presentation bool) Evidence[string] {
	v := resourceString(e)
	if v.State != EvidenceKnown {
		return v
	}
	switch v.Value {
	case "other", "none":
		return Evidence[string]{State: EvidenceUnknown, Reason: "application-dependent-xlink-behavior"}
	case "new", "replace", "embed":
		if presentation {
			return v
		}
	case "onRequest", "onLoad":
		if !presentation {
			return v
		}
	}
	return Evidence[string]{State: EvidenceUnknown, Reason: "xlink-behavior-domain-unavailable"}
}
