package validation

import (
	"context"
	"reflect"
)

// ResourceLinkKind identifies the exact encoded link family, independently of
// retrieval policy, URI base, file suffix or target resource availability.
type ResourceLinkKind string

// Local link families preserve the differing MusicXML and Opus source clauses.
const (
	ResourceScoreLink     ResourceLinkKind = "score-link"
	ResourcePartLink      ResourceLinkKind = "part-link"
	ResourceOpusScore     ResourceLinkKind = "opus-score"
	ResourceOpusLink      ResourceLinkKind = "opus-link"
	ResourceScoreWorkOpus ResourceLinkKind = "score-work-opus"
	ResourceBookmark      ResourceLinkKind = "bookmark"
)

// Resource context properties contain exactly one corresponding typed payload.
const (
	ContextResourceLink        ContextProperty = "resource.link"
	ContextResourceImage       ContextProperty = "resource.image"
	ContextResourceSchemaHints ContextProperty = "resource.schema-hints"
	ContextResourceGroup       ContextProperty = "resource.group"
	ContextResourceInstrument  ContextProperty = "resource.instrument"
)

// ResourceAttribute keeps physical presence, original lexical evidence and the
// shared XSD's effective value independent. A model need not retain raw spelling.
type ResourceAttribute struct {
	Presence Evidence[bool]
	Raw      Evidence[string]
	Value    Evidence[Atomic]
}

// ResourceLinkContext describes an encoded, nonresolving outgoing link or local
// bookmark. Fragment splits only the literal '#' delimiter; it is not XPointer
// assessment. Known empty and absent fragments remain distinct. Actuate never
// grants permission to load anything.
type ResourceLinkContext struct {
	// Intended XLink behavior, not rendering or retrieval authority. Other/none
	// remain application-dependent; effective default fields stay separate.
	PresentationIntent, ActuationIntent Evidence[string]
	Kind                                ResourceLinkKind
	Href, Show, Actuate                 ResourceAttribute
	Element, Position                   ResourceAttribute
	Attachment                          Evidence[NodeRef]
	Fragment                            Evidence[string]
}

// ResourceImageContext preserves required source/MIME-type facts and dimension
// advice. It does not infer media type from a suffix or intrinsic image geometry.
type ResourceImageContext struct {
	Credit                           bool
	Source, MediaType, Width, Height ResourceAttribute
	Scaling                          Evidence[string]
}

// SchemaHintPair is one complete discovery pair, with exact source ownership.
// It neither selects a schema nor authorizes retrieval.
type SchemaHintPair struct {
	Namespace, Location Evidence[string]
	Origin              Location
}

// ResourceSchemaHintsContext separates discovery pairing from the unchanged XSD
// list(anyURI) datatype. A trailing token is incomplete discovery, not a new XSD
// datatype violation. Complete is Known(false) for an observed unmatched token
// and Known(true) for zero or evenly paired tokens. Raw distinguishes absent
// schemaLocation from an explicitly empty value; NoNamespace retains its own
// presence. In provenance-aware source plans, DTD-defaulted and entity-generated
// hints can have effective pairs despite physically absent Raw. Discarded model
// xsi attributes are never reconstructed.
type ResourceSchemaHintsContext struct {
	Raw         Evidence[string]
	Pairs       []SchemaHintPair
	Trailing    Evidence[string]
	Complete    Evidence[bool]
	NoNamespace ResourceAttribute
}

// ResourceGroupContext preserves exact string content of group/group-link;
// it is unrelated to the part-group numbering domain.
type ResourceGroupContext struct {
	Linked bool
	Value  Evidence[string]
}

// ResourceInstrumentContext reuses the selected typed same-part IDREF result.
// Target does not establish any association with a different document.
type ResourceInstrumentContext struct {
	ID     ResourceAttribute
	Target Evidence[NodeRef]
}

// ResourceContextResult is a caller-owned local resource projection. Exactly
// one named payload is populated. Owner is the containing occurrence (the root
// owns itself); Part is
// the local owning score-part when established. SubjectDeclaration identifies the
// projected subject's exact assembly-qualified declaration use or its original
// unavailability evidence, reason and cause.
type ResourceContextResult struct {
	SubjectDeclaration Evidence[DeclarationID]
	Owner              NodeRef
	Part               Evidence[NodeRef]
	Link               *ResourceLinkContext
	Image              *ResourceImageContext
	SchemaHints        *ResourceSchemaHintsContext
	Group              *ResourceGroupContext
	Instrument         *ResourceInstrumentContext
}

func resourcePropertyValid(p ContextProperty) bool {
	switch p {
	case ContextResourceLink, ContextResourceImage, ContextResourceSchemaHints, ContextResourceGroup, ContextResourceInstrument:
		return true
	}
	return false
}
func resourcePayloadValid(p ContextProperty, v *ResourceContextResult) bool {
	if v == nil {
		return false
	}
	count := 0
	for _, ok := range []bool{v.Link != nil, v.Image != nil, v.SchemaHints != nil, v.Group != nil, v.Instrument != nil} {
		if ok {
			count++
		}
	}
	if count != 1 {
		return false
	}
	switch p {
	case ContextResourceLink:
		if v.Link == nil {
			return false
		}
		switch v.Link.Kind {
		case ResourceBookmark:
			return v.Link.Href.absent() && v.Link.Show.absent() && v.Link.Actuate.absent() && v.Link.Fragment.State == EvidenceAbsent
		case ResourceScoreLink:
			return true
		case ResourcePartLink, ResourceOpusScore, ResourceOpusLink, ResourceScoreWorkOpus:
			return v.Link.Element.absent() && v.Link.Position.absent() && v.Link.Attachment.State == EvidenceAbsent
		}
	case ContextResourceImage:
		return v.Image != nil
	case ContextResourceSchemaHints:
		return v.SchemaHints != nil
	case ContextResourceGroup:
		return v.Group != nil
	case ContextResourceInstrument:
		return v.Instrument != nil
	}
	return false
}
func resourceContextSize(ctx context.Context, s *Session, p *ResourceContextResult) (int64, error) {
	if p == nil {
		return 0, nil
	}
	base := *p
	base.Link, base.Image, base.SchemaHints, base.Group, base.Instrument = nil, nil, nil, nil, nil
	size, err := stateMeasure(ctx, s, reflect.ValueOf(base))
	if err != nil {
		return 0, err
	}
	add := func(value any) error {
		n, e := stateMeasure(ctx, s, reflect.ValueOf(value))
		if e != nil {
			return e
		}
		size, e = rationalCheckedAdd(BudgetBufferedBytes, size, n)
		return e
	}
	if p.Link != nil {
		if err = add(*p.Link); err != nil {
			return 0, err
		}
	}
	if p.Image != nil {
		if err = add(*p.Image); err != nil {
			return 0, err
		}
	}
	if p.Group != nil {
		if err = add(*p.Group); err != nil {
			return 0, err
		}
	}
	if p.Instrument != nil {
		if err = add(*p.Instrument); err != nil {
			return 0, err
		}
	}
	if p.SchemaHints != nil {
		hints := *p.SchemaHints
		hints.Pairs = nil
		if err = add(hints); err != nil {
			return 0, err
		}
		for _, pair := range p.SchemaHints.Pairs {
			if err = add(struct{ Namespace, Location Evidence[string] }{pair.Namespace, pair.Location}); err != nil {
				return 0, err
			}
			n := locationLogicalBytes(pair.Origin) + 384
			if err = s.step(ctx, n); err != nil {
				return 0, err
			}
			size, err = rationalCheckedAdd(BudgetBufferedBytes, size, n)
			if err != nil {
				return 0, err
			}
		}
	}
	return size, nil
}
func cloneResourceContextValue(p *ResourceContextResult) *ResourceContextResult {
	if p == nil {
		return nil
	}
	q := stateClone(*p)
	return &q
}
func cloneResourceContext(ctx context.Context, s *Session, p *ResourceContextResult) (*ResourceContextResult, error) {
	if p == nil {
		return nil, nil
	}
	size, err := resourceContextSize(ctx, s, p)
	if err != nil {
		return nil, err
	}
	if _, err = allocationSize(size); err != nil {
		return nil, err
	}
	if err = s.retain(size); err != nil {
		return nil, err
	}
	return cloneResourceContextValue(p), nil
}
func remapResourceEvidence[T any](v *Evidence[T], from, to string) {
	if v.Cause != nil && v.Cause.Subject.DocumentID == from {
		v.Cause.Subject.DocumentID = to
	}
}
func remapResourceNode(v *NodeRef, from, to string) {
	if v.DocumentID == from {
		v.DocumentID = to
	}
}
func remapResourceNodeEvidence(v *Evidence[NodeRef], from, to string) {
	remapResourceEvidence(v, from, to)
	remapResourceNode(&v.Value, from, to)
}
func remapResourceAttribute(v *ResourceAttribute, from, to string) {
	remapResourceEvidence(&v.Presence, from, to)
	remapResourceEvidence(&v.Raw, from, to)
	remapResourceEvidence(&v.Value, from, to)
}

// remapResourceContextValue changes occurrence identity only on an owned clone.
func remapResourceContextValue(v *ResourceContextResult, from, to string) {
	if v == nil {
		return
	}
	remapResourceEvidence(&v.SubjectDeclaration, from, to)
	remapResourceNode(&v.Owner, from, to)
	remapResourceNodeEvidence(&v.Part, from, to)
	if p := v.Link; p != nil {
		for _, a := range []*ResourceAttribute{&p.Href, &p.Show, &p.Actuate, &p.Element, &p.Position} {
			remapResourceAttribute(a, from, to)
		}
		remapResourceNodeEvidence(&p.Attachment, from, to)
		remapResourceEvidence(&p.Fragment, from, to)
	}
	if p := v.Image; p != nil {
		for _, a := range []*ResourceAttribute{&p.Source, &p.MediaType, &p.Width, &p.Height} {
			remapResourceAttribute(a, from, to)
		}
		remapResourceEvidence(&p.Scaling, from, to)
	}
	if p := v.SchemaHints; p != nil {
		remapResourceEvidence(&p.Raw, from, to)
		remapResourceEvidence(&p.Trailing, from, to)
		remapResourceEvidence(&p.Complete, from, to)
		remapResourceAttribute(&p.NoNamespace, from, to)
		for i := range p.Pairs {
			q := &p.Pairs[i]
			remapResourceEvidence(&q.Namespace, from, to)
			remapResourceEvidence(&q.Location, from, to)
			if q.Origin.DocumentID == from {
				q.Origin.DocumentID = to
			}
		}
	}
	if p := v.Group; p != nil {
		remapResourceEvidence(&p.Value, from, to)
	}
	if p := v.Instrument; p != nil {
		remapResourceAttribute(&p.ID, from, to)
		remapResourceNodeEvidence(&p.Target, from, to)
	}
}
