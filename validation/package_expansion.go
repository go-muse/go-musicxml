package validation

import (
	"context"
	"strings"
)

type resourceExpansionKey struct {
	resource              ResourceAssessmentID
	packageID             PackageID
	baseURI, archiveEntry string
}
type resourceExpansion struct {
	key      resourceExpansionKey
	owner    ResourceAdoptionID
	sealed   bool
	complete Evidence[bool]
	edges    map[uint64]PackageFactID
}

// ResourceAdoptionContext identifies an occurrence and its exact expansion owner.
// One canonical intrinsic assessment can have different base/package contexts.
type ResourceAdoptionContext struct {
	Adoption              ResourceAdoptionID
	Assessment            ResourceAssessmentID
	DocumentID            string
	Package               PackageID
	BaseURI, ArchiveEntry string
	ExpansionOwner        ResourceAdoptionID
	ExpansionComplete     Evidence[bool]
}

// expansionOwner returns the first adoption in an exact assessment, package,
// effective base URI and archive-entry context. An alias retains its own document
// identity and quota charge; it cannot certify until this owner is sealed.
func (s *PackageSession) expansionOwner(ctx context.Context, id ResourceAdoptionID) (ResourceAdoptionID, error) {
	if err := s.event(ctx, "ExpansionOwner"); err != nil {
		return "", err
	}
	a := s.adoptions[id]
	if a == nil {
		return "", s.fail(contract("expansion owner", "unknown adoption"))
	}
	return a.expansion.owner, nil
}

// sealResourceExpansion closes only the owner after its template/metadata walk.
// Complete assertion requires intrinsic scope completeness and exactly one
// selected edge for every outgoing non-bookmark template. Partial scope remains
// incomplete independently of package inventory or aliases pointing back to it.
func (s *PackageSession) sealResourceExpansion(ctx context.Context, id ResourceAdoptionID, complete Evidence[bool]) error {
	if err := s.event(ctx, "SealResourceExpansion"); err != nil {
		return err
	}
	a := s.adoptions[id]
	if a == nil || a.expansion.owner != id || a.expansion.sealed || s.boundaries[a.packageID].sealed {
		return s.fail(contract("resource expansion", "unsealed owner in open package required"))
	}
	if err := validateEvidence(complete, "resource expansion completeness"); err != nil {
		return s.fail(err)
	}
	if complete.State == EvidenceAbsent {
		return s.fail(contract("resource expansion", "absence is not completeness"))
	}
	if knownComplete(complete) {
		if !knownComplete(a.resource.snapshot.header.ScopeComplete) {
			return s.fail(contract("resource expansion", "intrinsic scope cannot prove complete expansion"))
		}
		for i, v := range a.resource.snapshot.contexts {
			if err := s.scan(ctx, 1); err != nil {
				return err
			}
			if v.Resource == nil || v.Resource.Link == nil || v.Resource.Link.Kind == ResourceBookmark {
				continue
			}
			if _, ok := a.expansion.edges[uint64(i)]; !ok {
				return s.fail(contract("resource expansion", "complete assertion omitted an outgoing template"))
			}
		}
	}
	if err := s.admit(ctx, complete, 128); err != nil {
		return err
	}
	a.expansion.complete = stateClone(complete)
	a.expansion.sealed = true
	return nil
}

// AdoptionContext returns an owned immutable context after package completion.
func (b AssessmentBundle) AdoptionContext(i int) (ResourceAdoptionContext, bool) {
	if b.data == nil || i < 0 || i >= len(b.data.adoptions) {
		return ResourceAdoptionContext{}, false
	}
	return stateClone(b.data.adoptions[i].context), true
}

func (s *PackageSession) newExpansion(id ResourceAdoptionID, r ResourceAssessmentID, opts ResourceAdoptionOptions) *resourceExpansion {
	key := resourceExpansionKey{r, opts.Package, opts.BaseURI, opts.ArchiveEntry}
	if old := s.expansions[key]; old != nil {
		return old
	}
	key.baseURI = strings.Clone(key.baseURI)
	key.archiveEntry = strings.Clone(key.archiveEntry)
	expansion := &resourceExpansion{key: key, owner: id, complete: Evidence[bool]{State: EvidenceUnknown, Reason: "resource-expansion-in-progress"}, edges: map[uint64]PackageFactID{}}
	s.expansions[key] = expansion
	return expansion
}

// sealResourceMembers closes the package's admitted expansion-context inventory
// after graph discovery and before complete role proofs. It is distinct from
// raw entry enumeration: an unread physical member still prevents role-universe
// exclusion. Exact aliases remain permitted; new contexts are rejected.
// A complete seal requires every existing expansion owner to be complete. Failed
// or canceled admission never changes the seal; partial seals cannot prove roles.
func (s *PackageSession) sealResourceMembers(ctx context.Context, pkg PackageID, complete Evidence[bool]) error {
	if err := s.event(ctx, "SealResourceMembers"); err != nil {
		return err
	}
	b := s.boundaries[pkg]
	if b == nil || b.sealed || b.membersSealed {
		return s.fail(contract("resource members", "open unsealed package inventory required"))
	}
	if err := validateEvidence(complete, "member enumeration"); err != nil {
		return s.fail(err)
	}
	if complete.State == EvidenceAbsent {
		return s.fail(contract("member enumeration", "absence is not completeness"))
	}
	if knownComplete(complete) {
		for _, id := range s.adoptionOrder {
			if err := s.scan(ctx, 1); err != nil {
				return err
			}
			a := s.adoptions[id]
			if a.packageID == pkg && a.expansion.owner == id && (!a.expansion.sealed || !knownComplete(a.expansion.complete)) {
				return s.fail(contract("resource members", "complete expansion owners required"))
			}
		}
	}
	if err := s.admit(ctx, complete, 128); err != nil {
		return err
	}
	b.membersComplete = stateClone(complete)
	b.membersSealed = true
	return s.evaluateBoundary(ctx, b)
}
