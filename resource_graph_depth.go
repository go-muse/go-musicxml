package musicxml

import "github.com/go-muse/go-musicxml/validation"

// This cache records already proved request contexts, never URI-to-canonical
// equivalence. Fixed resolver/catalog/profile configuration belongs to the run;
// source pin, template, href, package and effective base remain exact keys.
type graphDocumentKey struct {
	source   validation.ResourcePin
	profile  validation.ProfileID
	pkg      validation.PackageID
	base     resourceURIBase
	template uint64
	href     string
}
type graphDocumentTarget struct {
	resource *graphResource
	base     resourceURIBase
	pkg      validation.PackageID
}

func documentRequestKey(item graphExpansion, index uint64, href string) graphDocumentKey {
	return graphDocumentKey{item.resource.pin, item.resource.profile, item.pkg, item.base, index, href}
}
func (g *resourceGraphRun) admittedDocumentTarget(target graphDocumentTarget) bool {
	return target.resource != nil && g.expanded[graphExpansionKey{target.resource.id, target.pkg, target.base}]
}
func (g *resourceGraphRun) knownDocumentTarget(item graphExpansion, index uint64, href string) (graphDocumentTarget, bool) {
	target, ok := g.documentTargets[documentRequestKey(item, index, href)]
	return target, ok && g.admittedDocumentTarget(target)
}
func (g *resourceGraphRun) rememberDocumentTarget(item graphExpansion, index uint64, href string, target graphDocumentTarget) error {
	if !g.admittedDocumentTarget(target) {
		return nil
	}
	key := documentRequestKey(item, index, href)
	if _, ok := g.documentTargets[key]; ok {
		return nil
	}
	if err := g.reserve(1024 + 2*int64(len(href))); err != nil {
		return err
	}
	g.documentTargets[key] = target
	return nil
}
func (g *resourceGraphRun) graphDepthError(parent int64) error {
	return &validation.ResourceError{Kind: validation.BudgetDepth, Limit: g.options.account.Limits().MaxDepth, Used: parent, Requested: 1}
}

// An admitted archive path is identity evidence only inside its exact immutable
// package and retrieval context. The queue contains successful expansion owners,
// including cycles still in progress; cached canonical bytes alone do not suffice.
func (g *resourceGraphRun) knownArchiveTarget(item graphExpansion, path string) (graphDocumentTarget, bool, error) {
	if p := g.packages[item.pkg]; p != nil && p.primaryPath == path {
		// Canonical membership identifies the primary bytes; the incoming
		// expansion supplies its effective retrieval base. A different base
		// must already be admitted independently before it can bypass depth.
		base := p.primaryBase
		base.RetrievalURI = item.base.RetrievalURI
		target := graphDocumentTarget{p.primary, base, p.primaryPackage}
		if g.admittedDocumentTarget(target) {
			return target, true, nil
		}
	}
	for _, known := range g.queue {
		if err := g.ctx.Err(); err != nil {
			return graphDocumentTarget{}, false, err
		}
		if err := g.options.account.Charge(validation.BudgetRuleSteps, 1); err != nil {
			return graphDocumentTarget{}, false, err
		}
		if known.pkg == item.pkg && known.base.Archive && known.base.Entry == path && known.base.RetrievalURI == item.base.RetrievalURI {
			target := graphDocumentTarget{known.resource, known.base, known.pkg}
			if g.admittedDocumentTarget(target) {
				return target, true, nil
			}
		}
	}
	return graphDocumentTarget{}, false, nil
}
