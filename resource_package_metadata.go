package musicxml

import (
	"errors"
	"fmt"

	"github.com/go-muse/go-musicxml/validation"
)

// observePackageMetadata runs after the selected graph walk and before package
// enumeration seals. Roles cover only exact admitted archive-member contexts.
// Unread/unpinned members keep scope incomplete; names and XML roots never
// classify a resource as a score or individual part.
func (g *resourceGraphRun) observePackageMetadata() error {
	if err := g.ctx.Err(); err != nil {
		return err
	}
	return g.observePackageRoles()
}
func (g *resourceGraphRun) packageMetadataStep() error {
	if err := g.ctx.Err(); err != nil {
		return err
	}
	return g.options.account.Charge(validation.BudgetRuleSteps, 1)
}
func (g *resourceGraphRun) observePackageRoles() error {
	if !g.options.selected[ResourceRoles] || g.options.resolver == nil {
		return nil
	}
	for _, p := range g.packageList {
		if err := g.packageMetadataStep(); err != nil {
			return err
		}
		// Package role I/O requires a complete usable local package inventory
		// and at least one selected musical-document context. Plain XML roots
		// are outside packageList and never dispatch this capability. Container
		// metadata alone does not create a musical-role request. Once dispatched,
		// the all-member contract includes its admitted container context too.
		if p.entriesComplete.State != validation.EvidenceKnown || !p.entriesComplete.Value || p.rootsComplete.State != validation.EvidenceKnown || !p.rootsComplete.Value {
			continue
		}
		relevant := false
		for _, item := range g.queue {
			if err := g.packageMetadataStep(); err != nil {
				return err
			}
			if item.pkg == p.id && item.base.Archive && item.base.Entry != "" && (item.resource.profile == g.profiles.source || item.resource.profile == g.profiles.model) {
				relevant = true
			}
		}
		if !relevant {
			continue
		}
		if err := validationPackageSessionSealResourceMembers(g.session, g.ctx, p.id, packageKnown(true)); err != nil {
			return err
		}
		if err := g.observePackageRoleScope(p); err != nil {
			return err
		}
	}
	return nil
}
func (g *resourceGraphRun) observePackageRoleScope(p *graphPackage) error {
	// Admit both the retained original request and outgoing slice copies before
	// building any provider-visible arrays or maps. Their strings borrow already
	// admitted immutable graph storage. The provider may mutate only its copies.
	count := int64(0)
	for _, item := range g.queue {
		if err := g.packageMetadataStep(); err != nil {
			return err
		}
		if item.pkg == p.id && item.base.Archive && item.base.Entry != "" {
			count++
		}
	}
	if count > 65536 {
		return &validation.IncompleteError{Reason: "package-role-scope-too-large", Cause: fmt.Errorf("%w: fixed limit of 65536 exact member contexts (observed %d)", validation.ErrResourceLimit, count)}
	}
	size := 1024 + count*4096
	if _, err := validationAllocationSize(size); err != nil {
		return err
	}
	if err := g.options.account.Charge(validation.BudgetBufferedBytes, size); err != nil {
		return err
	}
	defer g.options.account.Release(validation.BudgetBufferedBytes, size)
	q := validation.ResourceRoleRequest{ID: validation.ResourceRequestID(g.next("roles")), Package: p.id, Resources: make([]validation.ResourcePin, 0, int(count)), Members: make([]validation.ResourceRoleMember, 0, int(count)), ScopeComplete: packageUnknown[bool]("package-role-member-scope-incomplete")}
	pins := make(map[validation.ResourcePin]bool, int(count))
	paths := make(map[string]bool, int(count))
	for _, item := range g.queue {
		if err := g.packageMetadataStep(); err != nil {
			return err
		}
		if item.pkg != p.id || !item.base.Archive || item.base.Entry == "" {
			continue
		}
		base := validation.Evidence[string]{State: validation.EvidenceAbsent}
		if item.base.RetrievalURI != "" {
			base = packageKnown(item.base.RetrievalURI)
		}
		q.Members = append(q.Members, validation.ResourceRoleMember{Resource: item.resource.pin, ArchiveEntry: item.base.Entry, BaseURI: base})
		paths[item.base.Entry] = true
		if !pins[item.resource.pin] {
			pins[item.resource.pin] = true
			q.Resources = append(q.Resources, item.resource.pin)
		}
	}
	complete := p.entriesComplete.State == validation.EvidenceKnown && p.entriesComplete.Value && p.rootsComplete.State == validation.EvidenceKnown && p.rootsComplete.Value
	for path := range p.entries {
		if err := g.packageMetadataStep(); err != nil {
			return err
		}
		// The specification's literal mimetype record is package metadata, not an
		// XML member. Every other unassessed entry leaves the role universe open.
		if path != mxlMIMETypePath && !paths[path] {
			complete = false
		}
	}
	if complete {
		q.ScopeComplete = packageKnown(true)
	}
	outgoing := q
	outgoing.Resources = make([]validation.ResourcePin, len(q.Resources))
	copy(outgoing.Resources, q.Resources)
	outgoing.Members = make([]validation.ResourceRoleMember, len(q.Members))
	copy(outgoing.Members, q.Members)
	request := validation.ResourceRequest{ID: q.ID, Roles: &q}
	if err := validation.ValidateResourceRequest(request); err != nil {
		return err
	}
	if err := g.ctx.Err(); err != nil {
		return err
	}
	result, err := g.options.resolver.Resolve(g.ctx, validation.ResourceRequest{ID: outgoing.ID, Roles: &outgoing})
	if result.Body != nil {
		err = errors.Join(err, result.Body.Close())
		result.Body = nil
		if err == nil {
			err = &validation.ConfigurationError{Field: "package role response", Reason: "unexpected document body"}
		}
	}
	if err != nil {
		return err
	}
	if err = g.ctx.Err(); err != nil {
		return err
	}
	// Provider output is admitted before validation allocates bounded lookup maps
	// and before core retains any owned reply. No callback can replace q's arrays.
	release, err := resourceWorkingLease(g.ctx, g.options.account, result)
	if err != nil {
		return err
	}
	defer release()
	if err = validation.ValidateResourceResult(request, result); err != nil {
		return err
	}
	if result.State != validation.EvidenceKnown {
		state := result.State
		if state == validation.EvidenceAbsent {
			state = validation.EvidenceUnknown
		}
		return validationPackageSessionObserveRoles(g.session, g.ctx, validation.ResourceRoleResult{Request: q, ScopeComplete: validation.Evidence[bool]{State: state, Reason: result.Reason}})
	}
	return validationPackageSessionObserveRoles(g.session, g.ctx, *result.Roles)
}
