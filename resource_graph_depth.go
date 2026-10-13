package musicxml

import "github.com/go-muse/go-musicxml/validation"

type graphDocumentTarget struct {
	resource *graphResource
	base     resourceURIBase
	pkg      validation.PackageID
}

func (g *resourceGraphRun) admittedDocumentTarget(target graphDocumentTarget) bool {
	return target.resource != nil && g.expanded[graphExpansionKey{target.resource.id, target.pkg, target.base}]
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
