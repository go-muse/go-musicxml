package musicxml

import (
	"errors"
	"github.com/go-muse/go-musicxml/validation"
)

func (g *resourceGraphRun) walkImage(item graphExpansion, index uint64, v validation.ContextResult) error {
	image := v.Resource.Image
	// Invalid local source evidence is already independently retained by the
	// intrinsic report. It never creates a guessed provider request.
	if image.Source.Value.State != validation.EvidenceKnown || image.Source.Value.Value.Kind != validation.AtomicString {
		return nil
	}
	release, err := resourceWorkingLease(g.ctx, g.options.account, v)
	if err != nil {
		return err
	}
	defer release()
	q := validation.ImageMetadataRequest{ID: validation.ResourceRequestID(g.next("image")), Source: item.resource.pin, Subject: v.Subject, Href: image.Source.Value.Value.Text, BaseURI: item.base.RetrievalURI, ArchiveEntry: item.base.Entry, Package: item.pkg}
	validMedia := image.MediaType.Value.State == validation.EvidenceKnown && image.MediaType.Value.Value.Kind == validation.AtomicString
	if validMedia {
		q.MediaType = image.MediaType.Value.Value.Text
	}
	request := validation.ResourceRequest{ID: q.ID, Image: &q}
	result := validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceUnknown, Reason: "image-metadata-not-selected"}
	if !validMedia {
		result.Reason = "local-image-media-type-unavailable"
	} else if g.options.selected[ResourceImages] && g.options.resolver != nil {
		result, err = g.resolveMetadata(request)
		if err != nil {
			return err
		}
	}
	resultRelease, err := resourceWorkingLease(g.ctx, g.options.account, result)
	if err != nil {
		return err
	}
	defer resultRelease()
	return g.session.ObserveImageMetadata(g.ctx, item.adoption, index, q, result)
}

// resolveMetadata closes any unexpected transferred stream even when the
// response is rejected. Successful metadata is copied/accounted before ingress;
// resolver-owned transport remains cooperative and outside library authority.
func (g *resourceGraphRun) resolveMetadata(request validation.ResourceRequest) (validation.ResourceResult, error) {
	if err := validation.ValidateResourceRequest(request); err != nil {
		return validation.ResourceResult{}, err
	}
	release, err := resourceWorkingLease(g.ctx, g.options.account, [2]validation.ResourceRequest{request, request})
	if err != nil {
		return validation.ResourceResult{}, err
	}
	defer release()
	if err = g.ctx.Err(); err != nil {
		return validation.ResourceResult{}, err
	}
	callback := request
	if request.Image != nil {
		value := *request.Image
		callback.Image = &value
	}
	if request.Association != nil {
		value := *request.Association
		callback.Association = &value
	}
	result, err := g.options.resolver.Resolve(g.ctx, callback)
	if result.Body != nil {
		err = errors.Join(err, result.Body.Close())
		result.Body = nil
		if err == nil {
			err = &validation.ConfigurationError{Field: "metadata response", Reason: "unexpected document body"}
		}
	}
	if err != nil {
		return validation.ResourceResult{}, err
	}
	if err = validation.ValidateResourceResult(request, result); err != nil {
		return validation.ResourceResult{}, err
	}
	if err = g.ctx.Err(); err != nil {
		return validation.ResourceResult{}, err
	}
	return result, nil
}

type graphLinkKey struct {
	source validation.ResourceAdoptionID
	node   validation.NodeID
}
type graphLinkedTarget struct {
	resource *graphResource
	adoption validation.ResourceAdoptionID
}

func (g *resourceGraphRun) walkAssociation(item graphExpansion, v validation.ContextResult) error {
	release, err := resourceWorkingLease(g.ctx, g.options.account, v)
	if err != nil {
		return err
	}
	defer release()
	p := v.Resource
	if p.Part.State != validation.EvidenceKnown || p.Instrument.Target.State != validation.EvidenceKnown {
		return nil
	}
	target, ok := g.linked[graphLinkKey{item.adoption, p.Owner.Node}]
	if !ok {
		return nil
	}
	a, err := g.session.Adoption(g.ctx, item.adoption)
	if err != nil {
		return err
	}
	child, err := g.session.Adoption(g.ctx, target.adoption)
	if err != nil {
		return err
	}
	header := child.Header()
	headerRelease, err := resourceWorkingLease(g.ctx, g.options.account, header)
	if err != nil {
		return err
	}
	defer headerRelease()
	root := header.Name
	if root.State != validation.EvidenceKnown || root.Value.Space != "" || (root.Value.Local != "score-partwise" && root.Value.Local != "score-timewise") {
		return nil
	}
	partID, instrumentID := "", ""
	for i := 0; i < a.IDCount(); i++ {
		id, ok, e := a.IDAt(g.ctx, i)
		if e != nil {
			return e
		}
		idRelease, e := resourceWorkingLease(g.ctx, g.options.account, id)
		if e != nil {
			return e
		}
		if !ok || id.Unique.State != validation.EvidenceKnown || !id.Unique.Value || id.Kind.State != validation.EvidenceKnown {
			idRelease()
			continue
		}
		if id.Subject == p.Part.Value && id.Kind.Value.Local == "score-part" {
			selectedRelease, e := resourceWorkingLease(g.ctx, g.options.account, id.Value)
			if e != nil {
				idRelease()
				return e
			}
			defer selectedRelease()
			partID = id.Value
		}
		if id.Subject == p.Instrument.Target.Value && id.Kind.Value.Local == "score-instrument" && id.Part.State == validation.EvidenceKnown && id.Part.Value == p.Part.Value {
			selectedRelease, e := resourceWorkingLease(g.ctx, g.options.account, id.Value)
			if e != nil {
				idRelease()
				return e
			}
			defer selectedRelease()
			instrumentID = id.Value
		}
		idRelease()
	}
	if partID == "" || instrumentID == "" {
		return nil
	}
	q := validation.AssociationRequest{ID: validation.ResourceRequestID(g.next("association")), Kind: validation.AssociationInstrumentPartFile, Source: item.resource.pin, Target: target.resource.pin, SourcePart: p.Part.Value, SourceInstrument: p.Instrument.Target.Value, PartID: partID, InstrumentID: instrumentID, PartLink: p.Owner, InstrumentLink: v.Subject}
	result, err := g.resolveMetadata(validation.ResourceRequest{ID: q.ID, Association: &q})
	if err != nil {
		return err
	}
	var association validation.AssociationResult
	if result.State == validation.EvidenceKnown {
		association = *result.Association
	} else {
		state, reason := result.State, result.Reason
		if state == validation.EvidenceAbsent {
			state, reason = validation.EvidenceUnknown, "association-metadata-absent"
		}
		association = validation.AssociationResult{Request: q, Membership: validation.Evidence[bool]{State: state, Reason: reason}, TargetPartID: validation.Evidence[string]{State: validation.EvidenceAbsent}, TargetInstrumentID: validation.Evidence[string]{State: validation.EvidenceAbsent}}
	}
	resultRelease, err := resourceWorkingLease(g.ctx, g.options.account, association)
	if err != nil {
		return err
	}
	defer resultRelease()
	return g.session.ObserveAssociation(g.ctx, q, association)
}
