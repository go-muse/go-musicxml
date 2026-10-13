package musicxml

import (
	"bytes"
	"context"
	"errors"

	"github.com/go-muse/go-musicxml/validation"
)

type graphCanonicalKey struct {
	canonical      string
	representation validation.ResourceRepresentation
}
type graphResourceKey struct {
	pin     validation.ResourcePin
	profile validation.ProfileID
}
type graphResource struct {
	pin      validation.ResourcePin
	profile  validation.ProfileID
	id       validation.ResourceAssessmentID
	xmlDepth int
}
type graphExpansionKey struct {
	resource validation.ResourceAssessmentID
	pkg      validation.PackageID
	base     resourceURIBase
}
type graphExpansion struct {
	resource *graphResource
	adoption validation.ResourceAdoptionID
	pkg      validation.PackageID
	base     resourceURIBase
	depth    int64
}

func (g *resourceGraphRun) cacheResource(pin validation.ResourcePin, profile validation.ProfileID, id validation.ResourceAssessmentID, xmlDepth int, reusable bool) (*graphResource, error) {
	key := graphResourceKey{pin, profile}
	if old := g.cache[key]; old != nil {
		return old, nil
	}
	canonical := graphCanonicalKey{pin.CanonicalID, pin.Representation}
	if old, ok := g.pins[canonical]; ok && old != pin {
		return nil, &validation.ConfigurationError{Field: "resource pin", Reason: "canonical identity changed revision or content"}
	}
	if err := g.reserve(1040 + 2*int64(len(pin.CanonicalID)+len(pin.Revision)+len(pin.Digest)+len(pin.SnapshotID)+len(profile))); err != nil {
		return nil, err
	}
	r := &graphResource{pin, profile, id, xmlDepth}
	// A partial intrinsic assessment remains available for known proofs, but
	// cannot become a successful cache hit in a later, less restrictive context.
	if reusable {
		g.cache[key] = r
	}
	g.pins[canonical] = pin
	return r, nil
}
func (g *resourceGraphRun) enqueue(r *graphResource, adoption validation.ResourceAdoptionID, pkg validation.PackageID, base resourceURIBase, depth int64) error {
	if err := g.ctx.Err(); err != nil {
		return err
	}
	owner, ownerErr := g.session.ExpansionOwner(g.ctx, adoption)
	if ownerErr != nil {
		return ownerErr
	}
	adoption = owner
	key := graphExpansionKey{r.id, pkg, base}
	if g.expanded[key] {
		return nil
	}
	if depth > g.options.account.Limits().MaxDepth {
		return &validation.ResourceError{Kind: validation.BudgetDepth, Limit: g.options.account.Limits().MaxDepth, Used: depth - 1, Requested: 1}
	}
	if err := g.reserve(1024 + 2*int64(len(base.RetrievalURI)+len(base.Entry)+len(pkg))); err != nil {
		return err
	}
	g.expanded[key] = true
	g.queue = append(g.queue, graphExpansion{r, adoption, pkg, base, depth})
	return nil
}
func (g *resourceGraphRun) walkResources() error {
	for at := 0; at < len(g.queue); at++ {
		if err := g.ctx.Err(); err != nil {
			return err
		}
		item := g.queue[at]
		if err := g.options.account.Charge(validation.BudgetRuleSteps, 1); err != nil {
			return err
		}
		a, err := g.session.Adoption(g.ctx, item.adoption)
		if err != nil {
			return err
		}
		for i := 0; i < a.ContextCount(); i++ {
			v, ok, err := a.ContextAt(g.ctx, i)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			if v.Resource == nil {
				continue
			}
			if v.Resource.Image != nil {
				if err = g.walkImage(item, uint64(i), v); err != nil {
					return err
				}
			}
			if v.Resource.Link != nil && v.Resource.Link.Kind != validation.ResourceBookmark {
				if err = g.walkLink(item, uint64(i), v); err != nil {
					return err
				}
			}
		}
		if g.options.selected[ResourceAssociations] && g.options.resolver != nil {
			for i := 0; i < a.ContextCount(); i++ {
				v, ok, e := a.ContextAt(g.ctx, i)
				if e != nil {
					return e
				}
				if ok && v.Resource != nil && v.Resource.Instrument != nil {
					if e = g.walkAssociation(item, v); e != nil {
						return e
					}
				}
			}
		}
		completeness := a.Header().ScopeComplete
		if err = g.session.SealResourceExpansion(g.ctx, item.adoption, completeness); err != nil {
			return err
		}
	}
	return nil
}
func unavailableTarget(state validation.EvidenceState, reason validation.Code, cause *validation.DecisionRef) validation.Evidence[validation.ResourceAdoptionID] {
	return validation.Evidence[validation.ResourceAdoptionID]{State: state, Reason: reason, Cause: cause}
}
func (g *resourceGraphRun) walkLink(item graphExpansion, index uint64, v validation.ContextResult) error {
	release, err := resourceWorkingLease(g.ctx, g.options.account, v)
	if err != nil {
		return err
	}
	defer release()
	// Every incoming edge requires a linked occurrence below; importing a new
	// package may additionally admit its container and rootfile resources.
	// Refuse a known-exhausted admission before invoking caller I/O or parsing a
	// local target. Core keeps the actual cumulative charge; this is no recharge.
	used, err := g.options.account.Usage(validation.BudgetLinkedDocuments)
	if err != nil {
		return err
	}
	limit := g.options.account.Limits().MaxLinkedDocuments
	if used >= limit {
		return &validation.ResourceError{Kind: validation.BudgetLinkedDocuments, Limit: limit, Used: used, Requested: 1}
	}
	link := v.Resource.Link
	target := unavailableTarget(validation.EvidenceUnknown, "resolver-not-selected", nil)
	if g.options.selected[ResourceDocuments] && g.options.resolver == nil {
		target.Reason = "resolver-unavailable"
	}
	fragment := link.Fragment
	fragmentTarget := validation.Evidence[validation.NodeRef]{State: validation.EvidenceUnknown, Reason: "target-unavailable"}
	href := link.Href.Value
	var child *graphResource
	var childBase resourceURIBase
	childPackage := item.pkg
	var loadErr error
	if href.State != validation.EvidenceKnown {
		switch href.State {
		case validation.EvidenceInvalid, validation.EvidenceUnsupported, validation.EvidenceUnknown:
			target = unavailableTarget(href.State, href.Reason, href.Cause)
		default:
			target = unavailableTarget(validation.EvidenceUnknown, "local-href-unavailable", nil)
		}
	} else if href.Value.Kind != validation.AtomicString {
		target = unavailableTarget(validation.EvidenceUnknown, "local-href-type-unavailable", nil)
	} else {
		base := item.base
		base.ArchiveRoot = link.Kind == validation.ResourceScoreLink || link.Kind == validation.ResourcePartLink
		resolved, uriErr := resolveAssessmentResourceURI(g.ctx, g.options.account, base, href.Value.Text)
		if uriErr != nil {
			var unavailable *resourceURIUnavailable
			if errors.As(uriErr, &unavailable) {
				target = unavailableTarget(validation.EvidenceUnsupported, unavailable.reason, nil)
			} else {
				loadErr = uriErr
			}
		} else {
			fragment = validation.Evidence[string]{State: validation.EvidenceAbsent}
			if resolved.FragmentPresent {
				fragment = validation.Evidence[string]{State: validation.EvidenceKnown, Value: resolved.Fragment}
			}
			if resolved.SameEntity {
				child, childBase = item.resource, item.base
			} else if resolved.ArchivePath != "" {
				if item.depth >= g.options.account.Limits().MaxDepth {
					known, ok, err := g.knownArchiveTarget(item, resolved.ArchivePath)
					if err != nil {
						return err
					}
					if !ok {
						return g.graphDepthError(item.depth)
					}
					child, childBase, childPackage = known.resource, known.base, known.pkg
				} else {
					child, childBase, childPackage, target, loadErr = g.loadArchiveDocument(item, resolved.ArchivePath)
				}
			} else if g.options.selected[ResourceDocuments] && g.options.resolver != nil {
				if item.depth >= g.options.account.Limits().MaxDepth {
					known, ok := g.knownDocumentTarget(item, index, href.Value.Text)
					if !ok {
						return g.graphDepthError(item.depth)
					}
					child, childBase, childPackage = known.resource, known.base, known.pkg
				} else {
					child, childBase, childPackage, target, loadErr = g.loadExternalDocument(item, index, v, href.Value.Text, resolved)
				}
			}
		}
	}
	if terminal := g.session.TerminalCause(); terminal != nil {
		if errors.Is(loadErr, terminal) {
			return loadErr
		}
		return errors.Join(loadErr, terminal)
	}
	// A retained intrinsic snapshot can prove its observed root even when that
	// child failed syntax or root support. Preserve that evidence, but never
	// expand more work after interruption or a child operation error.
	if child != nil && g.ctx.Err() == nil && !errors.Is(loadErr, validation.ErrResourceLimit) && !errors.Is(loadErr, ErrMXLTooLarge) && !errors.Is(loadErr, context.Canceled) && !errors.Is(loadErr, context.DeadlineExceeded) {
		aid, e := g.session.AdoptResource(g.ctx, child.id, validation.ResourceAdoptionOptions{DocumentID: g.next("adoption"), Package: childPackage, BaseURI: childBase.RetrievalURI, ArchiveEntry: childBase.Entry})
		if e != nil {
			return errors.Join(loadErr, e)
		}
		target = validation.Evidence[validation.ResourceAdoptionID]{State: validation.EvidenceKnown, Value: aid}
		fragmentTarget, e = g.resolveFragment(aid, fragment)
		if e != nil {
			return errors.Join(loadErr, e)
		}
		fragmentRelease, leaseErr := resourceWorkingLease(g.ctx, g.options.account, fragmentTarget)
		if leaseErr != nil {
			return errors.Join(loadErr, leaseErr)
		}
		defer fragmentRelease()
		if loadErr == nil {
			if e = g.enqueue(child, aid, childPackage, childBase, item.depth+1); e != nil {
				return errors.Join(loadErr, e)
			}
		}
	}
	g.sequence++
	edgeErr := g.session.ObserveEdge(g.ctx, validation.PackageEdgeObservation{ID: validation.PackageFactID(g.sequence), Source: item.adoption, Template: index, Target: target, Fragment: fragment, FragmentTarget: fragmentTarget})
	if edgeErr == nil && target.State == validation.EvidenceKnown && child != nil {
		if e := g.reserve(512); e != nil {
			return errors.Join(loadErr, e)
		}
		g.linked[graphLinkKey{item.adoption, v.Subject.Node}] = graphLinkedTarget{child, target.Value}
		if loadErr == nil {
			if e := g.rememberDocumentTarget(item, index, href.Value.Text, graphDocumentTarget{child, childBase, childPackage}); e != nil {
				return e
			}
		}
	}
	return errors.Join(loadErr, edgeErr)
}
func (g *resourceGraphRun) loadExternalDocument(item graphExpansion, index uint64, v validation.ContextResult, href string, resolved resolvedResourceURI) (*graphResource, resourceURIBase, validation.PackageID, validation.Evidence[validation.ResourceAdoptionID], error) {
	unknown := unavailableTarget(validation.EvidenceUnknown, "resource-unavailable", nil)
	engine, err := sourceAssessmentEngineForProfile(ProfileSourceLinks)
	if err != nil {
		return nil, resourceURIBase{}, item.pkg, unknown, err
	}
	profile, _ := engine.Profile(ProfileSourceLinks)
	request := validation.ResourceRequest{ID: validation.ResourceRequestID(g.next("request")), Document: &validation.ResourceDocumentRequest{Source: item.resource.pin, Subject: v.Subject, Kind: validation.LinkedResourceAuto, RequiredProfile: ProfileSourceLinks, CatalogDigest: profile.CatalogDigest, BaseURI: item.base.RetrievalURI, Href: href, Package: item.pkg}}
	if err = validation.ValidateResourceRequest(request); err != nil {
		return nil, resourceURIBase{}, item.pkg, unknown, err
	}
	release, err := resourceWorkingLease(g.ctx, g.options.account, [2]validation.ResourceRequest{request, request})
	if err != nil {
		return nil, resourceURIBase{}, item.pkg, unknown, err
	}
	defer release()
	if err = g.ctx.Err(); err != nil {
		return nil, resourceURIBase{}, item.pkg, unknown, err
	}
	callback := request
	document := *request.Document
	callback.Document = &document
	result, resolveErr := g.options.resolver.Resolve(g.ctx, callback)
	if resolveErr != nil {
		if result.Body != nil {
			resolveErr = errors.Join(resolveErr, result.Body.Close())
		}
		return nil, resourceURIBase{}, item.pkg, unknown, resolveErr
	}
	if err = validation.ValidateResourceResult(request, result); err != nil {
		if result.Body != nil {
			err = errors.Join(err, result.Body.Close())
		}
		return nil, resourceURIBase{}, item.pkg, unknown, err
	}
	if result.State != validation.EvidenceKnown {
		return nil, resourceURIBase{}, item.pkg, unavailableTarget(result.State, result.Reason, nil), nil
	}
	base := resourceURIBase{RetrievalURI: result.RetrievalURI}
	if base.RetrievalURI != "" {
		var resolvedBase resolvedResourceURI
		resolvedBase, err = resolveAssessmentResourceURI(g.ctx, g.options.account, base, "")
		if err != nil {
			var unavailable *resourceURIUnavailable
			if errors.As(err, &unavailable) {
				err = &validation.ConfigurationError{Field: "resource retrieval URI", Reason: string(unavailable.reason)}
			}
			return nil, base, item.pkg, unknown, errors.Join(err, result.Body.Close())
		}
		base.RetrievalURI = resolvedBase.RetrievalURI
		result.RetrievalURI = base.RetrievalURI
	}
	body, err := g.readExternalPackageBody(item, result.Pin, result.Body)
	if err != nil {
		return nil, resourceURIBase{}, item.pkg, unknown, err
	}
	bodyOwned := false
	defer func() {
		if !bodyOwned {
			body.close()
		}
	}()
	if body.digest != result.Pin.Digest {
		return nil, resourceURIBase{}, item.pkg, unknown, &validation.ConfigurationError{Field: "resource digest", Reason: "returned bytes do not match immutable pin"}
	}

	canonical := graphCanonicalKey{result.Pin.CanonicalID, result.Pin.Representation}
	if old, ok := g.pins[canonical]; ok && old != result.Pin {
		return nil, base, item.pkg, unknown, &validation.ConfigurationError{Field: "resource pin", Reason: "canonical identity changed revision or content"}
	}
	if err = g.observeExternalFileMetadata(item, index, result, base, href); err != nil {
		return nil, base, item.pkg, unknown, err
	}
	if result.Pin.Representation == validation.ResourceRawMXL {
		// Alias bodies were still read, charged and digest-verified above. Reuse
		// only the importer's exact pin/context proof, before borrowing another
		// archive or retaining a body that no new boundary will need.
		if resource, childBase, pkg, known, lookupErr := g.existingRawPackage(result.Pin, item.pkg, result.RetrievalURI); known || lookupErr != nil {
			return resource, childBase, pkg, unknown, lookupErr
		}
		if err = g.ownResourceBody(body); err != nil {
			return nil, base, item.pkg, unknown, err
		}
		bodyOwned = true
		archive, readErr := borrowMXLAssessmentArchive(g.ctx, body.bytes, g.mxlOptions, g.options.account)
		if archive == nil {
			return nil, base, item.pkg, unknown, readErr
		}
		if err = g.ownArchive(archive); err != nil {
			archive.Close()
			return nil, base, item.pkg, unknown, joinResourceErrors(readErr, err)
		}
		resource, childBase, pkg, importErr := g.importRawPackageAtDepth(archive, result.Pin, item.pkg, result.RetrievalURI, false, item.depth+1)
		return resource, childBase, pkg, unknown, joinResourceErrors(readErr, importErr)
	}
	key := graphResourceKey{result.Pin, ProfileSourceLinks}
	if cached := g.cache[key]; cached != nil {
		return cached, base, item.pkg, unknown, g.checkIntrinsicXMLDepth(cached, item.pkg)
	}
	intrinsic := assessResourceSource(g.ctx, bytes.NewReader(body.bytes), sourceAssessmentConfig{assessmentRunConfig: assessmentRunConfig{Account: g.options.account, DocumentID: g.next("source"), Profile: ProfileSourceLinks, XMLDepth: g.intrinsicXMLDepth(item.pkg), Engine: engine}, ByteOwner: sourceBytesAccounted})
	if intrinsic.Snapshot == nil {
		return nil, base, item.pkg, unknown, g.retainUntransferredIntrinsic(intrinsic, result.Pin, nil)
	}
	id, err := g.session.AddResource(g.ctx, result.Pin, intrinsic.Snapshot, true)
	if err != nil {
		return nil, base, item.pkg, unknown, g.retainUntransferredIntrinsic(intrinsic, result.Pin, err)
	}
	resource, err := g.cacheResource(result.Pin, ProfileSourceLinks, id, intrinsic.XMLDepth, intrinsic.Err == nil)
	return resource, base, item.pkg, unknown, errors.Join(intrinsic.Err, err)
}
func (g *resourceGraphRun) resolveFragment(aid validation.ResourceAdoptionID, fragment validation.Evidence[string]) (validation.Evidence[validation.NodeRef], error) {
	if fragment.State == validation.EvidenceAbsent {
		return validation.Evidence[validation.NodeRef]{State: validation.EvidenceAbsent}, nil
	}
	if fragment.State != validation.EvidenceKnown {
		return validation.Evidence[validation.NodeRef]{State: fragment.State, Reason: fragment.Reason, Cause: fragment.Cause}, nil
	}
	a, err := g.session.Adoption(g.ctx, aid)
	if err != nil {
		return validation.Evidence[validation.NodeRef]{}, err
	}
	header := a.Header()
	headerRelease, err := resourceWorkingLease(g.ctx, g.options.account, header)
	if err != nil {
		return validation.Evidence[validation.NodeRef]{}, err
	}
	defer headerRelease()
	if fragment.Value == "" {
		if header.Root.Node == 0 {
			return validation.Evidence[validation.NodeRef]{State: validation.EvidenceUnknown, Reason: "fragment-root-occurrence-unavailable"}, nil
		}
		return validation.Evidence[validation.NodeRef]{State: validation.EvidenceKnown, Value: header.Root}, nil
	}
	if !sourceXMLNCName(fragment.Value) {
		return validation.Evidence[validation.NodeRef]{State: validation.EvidenceUnsupported, Reason: "fragment-scheme-unavailable"}, nil
	}
	for i := 0; i < a.IDCount(); i++ {
		id, ok, err := a.IDAt(g.ctx, i)
		if err != nil {
			return validation.Evidence[validation.NodeRef]{}, err
		}
		if !ok || id.Value != fragment.Value {
			continue
		}
		if id.Unique.State == validation.EvidenceKnown && id.Unique.Value {
			return validation.Evidence[validation.NodeRef]{State: validation.EvidenceKnown, Value: id.Subject}, nil
		}
		return validation.Evidence[validation.NodeRef]{State: id.Unique.State, Reason: id.Unique.Reason, Cause: id.Unique.Cause}, nil
	}
	if header.IDScopeComplete.State == validation.EvidenceKnown && header.IDScopeComplete.Value {
		return validation.Evidence[validation.NodeRef]{State: validation.EvidenceAbsent}, nil
	}
	return validation.Evidence[validation.NodeRef]{State: validation.EvidenceUnknown, Reason: "fragment-id-scope-incomplete"}, nil
}
