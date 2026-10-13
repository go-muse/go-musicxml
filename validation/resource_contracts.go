package validation

import (
	"context"
	"io"
	"slices"
)

// ResourceRepresentation distinguishes immutable original bytes from current
// caller state. A current model/package has no invented source-byte digest.
type ResourceRepresentation string

// Supported representation identities.
const (
	ResourceSourceXML    ResourceRepresentation = "source-xml"
	ResourceCurrentModel ResourceRepresentation = "current-model"
	ResourceRawMXL       ResourceRepresentation = "raw-mxl"
	ResourceCurrentMXL   ResourceRepresentation = "current-mxl"
	ResourceBinary       ResourceRepresentation = "binary"
)

// ResourcePin identifies an immutable representation independently of retrieval
// base or adoption identity. Source pins use verified lowercase SHA-256 Digest;
// current-state pins use an opaque root-run SnapshotID and no Digest.
type ResourcePin struct {
	CanonicalID, Revision, Digest string
	SnapshotID                    string
	Representation                ResourceRepresentation
}

// ResourceAssessmentID identifies one canonical intrinsic assessment in a bundle.
type ResourceAssessmentID string

// ResourceAdoptionID identifies a fresh occurrence view of a canonical assessment.
type ResourceAdoptionID string

// ResourceRequestID binds one bounded request to its exact provider response.
type ResourceRequestID string

// PackageID identifies an encapsulation boundary, not a filesystem directory.
type PackageID string

// AssociationKind is a closed relation vocabulary, not an executable predicate.
type AssociationKind string

// AssociationInstrumentPartFile names the exact local instrument's membership
// in the selected linked individual-part resource, without cross-file ID equality.
const AssociationInstrumentPartFile AssociationKind = "instrument-part-file"

// AssociationRequest binds exact source subjects and normalized local IDs to
// both immutable resource pins. Optional target IDs are not assumed here.
type AssociationRequest struct {
	ID                           ResourceRequestID
	Kind                         AssociationKind
	Source, Target               ResourcePin
	SourcePart, SourceInstrument NodeRef
	PartID, InstrumentID         string
	PartLink, InstrumentLink     NodeRef
}

// EvidenceAuthority declares a bounded assertion's provenance and exact-scope
// completeness. Missing entries in a partial mapping never assert nonmembership.
type EvidenceAuthority struct {
	ID, Revision          string
	Provenance            []SourceRef
	ExactRelationComplete bool
}

// AssociationResult echoes the complete exact request. Target IDs are optional;
// supplied IDs must resolve uniquely to the correct target kind/owner in core.
type AssociationResult struct {
	Request                          AssociationRequest
	Membership                       Evidence[bool]
	Authority                        EvidenceAuthority
	TargetPartID, TargetInstrumentID Evidence[string]
}

// ResourceRole describes a resource within an explicitly bounded package scope.
type ResourceRole string

// Authoritative package role categories; filenames are not role evidence.
const (
	ResourceRoleScore          ResourceRole = "score"
	ResourceRoleIndividualPart ResourceRole = "individual-part"
	ResourceRoleOther          ResourceRole = "other"
)

// ResourceRoleMember identifies one literal member in Request.Package. A pin
// proves content identity, not membership. BaseURI is exact retrieval context,
// not transport authority; absent differs from a known empty string.
type ResourceRoleMember struct {
	Resource     ResourcePin
	ArchiveEntry string
	BaseURI      Evidence[string]
}
type resourceRoleMemberKey struct {
	pin         ResourcePin
	entry, base string
	baseState   EvidenceState
}

func roleMemberKey(m ResourceRoleMember) resourceRoleMemberKey {
	return resourceRoleMemberKey{m.Resource, m.ArchiveEntry, m.BaseURI.Value, m.BaseURI.State}
}
func validateRoleMember(m ResourceRoleMember) error {
	if err := validateResourcePin(m.Resource); err != nil {
		return err
	}
	if m.ArchiveEntry == "" || !validText(m.ArchiveEntry, 4096) {
		return contract("role member", "exact bounded archive member required")
	}
	if err := validateEvidence(m.BaseURI, "role member base"); err != nil {
		return err
	}
	if m.BaseURI.State != EvidenceKnown && m.BaseURI.State != EvidenceAbsent {
		return contract("role member base", "exact presence or absence required")
	}
	if !validText(m.BaseURI.Value, 16384) {
		return contract("role member base", "bounded retrieval context required")
	}
	return nil
}

// ResourceRoleFact supplies one exact resource-to-role membership assertion.
// Member is required when Request.Members is present; nil preserves the legacy
// pin-only protocol. Result facts may be reordered independently of the request.
type ResourceRoleFact struct {
	Resource   ResourcePin
	Member     *ResourceRoleMember
	Role       ResourceRole
	Membership Evidence[bool]
}

// ResourceRoleRequest names all requested resources and scope completeness.
type ResourceRoleRequest struct {
	ID        ResourceRequestID
	Package   PackageID
	Resources []ResourcePin
	// Members preserves distinct package contexts for duplicate content. Resources
	// is the deduplicated pin inventory; the complete ordered request is echoed.
	Members       []ResourceRoleMember
	ScopeComplete Evidence[bool]
}

// ResourceRoleResult echoes the full requested scope and supplies independent
// role memberships. ScopeComplete cannot silently broaden the requested scope.
type ResourceRoleResult struct {
	Request       ResourceRoleRequest
	Resources     []ResourceRoleFact
	ScopeComplete Evidence[bool]
	Authority     EvidenceAuthority
}

// ImageMetadataRequest selects intrinsic image metadata without rendering or
// executing media. Media type is explicit and never inferred from a suffix.
type ImageMetadataRequest struct {
	// These are the exact retrieval context, separate from canonical identity.
	BaseURI, ArchiveEntry string
	Package               PackageID
	ID                    ResourceRequestID
	Source                ResourcePin
	Subject               NodeRef
	Href, MediaType       string
}

// ImageDimensionUnit identifies the common unit used by intrinsic dimensions.
type ImageDimensionUnit string

// Supported intrinsic dimension units. No unit implies MusicXML tenths or DPI.
const (
	ImageUnitPixel      ImageDimensionUnit = "pixel"
	ImageUnitPoint      ImageDimensionUnit = "point"
	ImageUnitMillimeter ImageDimensionUnit = "millimeter"
)

// ImageMetadataResult is optional authoritative metadata. PixelAspectRatio is
// meaningful only for pixels; absent physical density is never manufactured.
type ImageMetadataResult struct {
	Request          ImageMetadataRequest
	Resource         ResourcePin
	Width, Height    Evidence[Decimal]
	Unit             ImageDimensionUnit
	PixelAspectRatio Evidence[Decimal]
	Authority        EvidenceAuthority
}

// LinkedResourceKind selects the transport representation a request admits.
type LinkedResourceKind string

// XML-or-MXL allows bounded technical nested packages, not normative indirection.
const (
	LinkedResourceXML  LinkedResourceKind = "xml"
	LinkedResourceMXL  LinkedResourceKind = "mxl"
	LinkedResourceAuto LinkedResourceKind = "xml-or-mxl"
)

// ResourceDocumentRequest names a selected intrinsic interpretation and exact
// source context. Caller resolver code owns transport/redirect authorization;
// these metadata fields cannot grant authority or control already-performed I/O.
type ResourceDocumentRequest struct {
	Source                           ResourcePin
	Subject                          NodeRef
	Kind                             LinkedResourceKind
	RequiredProfile                  ProfileID
	CatalogDigest                    string
	BaseURI, Href, RequestedRevision string
	Package                          PackageID
}

// ResourceRequest is a closed union: exactly one named branch must be populated.
type ResourceRequest struct {
	ID          ResourceRequestID
	Document    *ResourceDocumentRequest
	Image       *ImageMetadataRequest
	Association *AssociationRequest
	Roles       *ResourceRoleRequest
}

// ResourceResult is the corresponding closed result union. Every nonnil Body
// transfers ownership to root orchestration, even on invalid payloads or errors.
// The core validators never read or close it. Claimed pins require root byte
// verification; newly returned alias bodies still consume external byte quotas.
type ResourceResult struct {
	RequestID               ResourceRequestID
	State                   EvidenceState
	Reason                  Code
	Pin                     ResourcePin
	RetrievalURI, MediaType string
	Body                    io.ReadCloser
	Image                   *ImageMetadataResult
	Association             *AssociationResult
	Roles                   *ResourceRoleResult
	Authority               EvidenceAuthority
}

// ResourceResolver supplies explicitly selected resources or typed assertions.
// No built-in network/filesystem fallback is provided by validation.
type ResourceResolver interface {
	Resolve(context.Context, ResourceRequest) (ResourceResult, error)
}

func validateResourcePin(p ResourcePin) error {
	if !packageIdentityText(p.CanonicalID) || !validText(p.Revision, maxIdentityBytes) {
		return contract("resource pin", "canonical identity and bounded revision required")
	}
	switch p.Representation {
	case ResourceSourceXML, ResourceRawMXL, ResourceBinary:
		if !validResourceDigest(p.Digest) || p.SnapshotID != "" {
			return contract("resource pin", "verified byte digest and no current snapshot ID required")
		}
	case ResourceCurrentModel, ResourceCurrentMXL:
		if !validID(p.SnapshotID) || p.Digest != "" {
			return contract("resource pin", "opaque current snapshot identity and no source digest required")
		}
	default:
		return contract("resource pin", "unknown representation")
	}
	return nil
}
func validResourceNode(n NodeRef) bool { return validID(n.DocumentID) && n.Node != 0 }
func validateAuthority(a EvidenceAuthority, required bool) error {
	if !required && a.ID == "" && a.Revision == "" && len(a.Provenance) == 0 && !a.ExactRelationComplete {
		return nil
	}
	if !validID(a.ID) || !validText(a.Revision, maxIdentityBytes) || len(a.Provenance) == 0 || len(a.Provenance) > 64 {
		return contract("evidence authority", "bounded identity and provenance required")
	}
	for _, s := range a.Provenance {
		if len(s.IDs) == 0 || len(s.IDs) > 64 || !catalogText(s.URL, 4096) || !catalogText(s.Locator, 4096) {
			return contract("evidence authority", "bounded complete source reference required")
		}
		for _, id := range s.IDs {
			if !validID(id) {
				return contract("evidence authority", "invalid source identity")
			}
		}
	}
	return nil
}
func validateAssociationRequest(q AssociationRequest) error {
	if !validID(string(q.ID)) || q.Kind != AssociationInstrumentPartFile {
		return contract("association request", "exact request identity and closed relation required")
	}
	if err := validateResourcePin(q.Source); err != nil {
		return err
	}
	if err := validateResourcePin(q.Target); err != nil {
		return err
	}
	for _, r := range []NodeRef{q.SourcePart, q.SourceInstrument, q.PartLink, q.InstrumentLink} {
		if !validResourceNode(r) || r.DocumentID != q.SourcePart.DocumentID {
			return contract("association request", "exact same-source subjects required")
		}
	}
	if !validID(q.PartID) || !validID(q.InstrumentID) {
		return contract("association request", "normalized local IDs required")
	}
	return nil
}
func validateRoleRequest(q ResourceRoleRequest) error {
	if !validID(string(q.ID)) || !validID(string(q.Package)) || len(q.Resources) > 65536 || len(q.Members) > 65536 {
		return contract("role request", "bounded exact package scope required")
	}
	if err := validateEvidence(q.ScopeComplete, "role scope"); err != nil {
		return err
	}
	if q.ScopeComplete.State == EvidenceAbsent {
		return contract("role scope", "absence is not completeness")
	}
	for _, p := range q.Resources {
		if err := validateResourcePin(p); err != nil {
			return err
		}
	}
	// Admission of the small fixed key map precedes allocation; hard collection
	// bounds also apply without a running Account to these pure transport checks.
	seen := make(map[ResourcePin]bool, len(q.Resources))
	for _, p := range q.Resources {
		if seen[p] {
			return contract("role request", "duplicate resource pin")
		}
		seen[p] = true
	}

	if len(q.Members) > 0 {
		members := make(map[resourceRoleMemberKey]bool, len(q.Members))
		covered := make(map[ResourcePin]bool, len(q.Resources))
		for _, m := range q.Members {
			if err := validateRoleMember(m); err != nil {
				return err
			}
			key := roleMemberKey(m)
			if members[key] || !seen[m.Resource] {
				return contract("role member", "duplicate member or missing inventory pin")
			}
			members[key] = true
			covered[m.Resource] = true
		}
		if len(covered) != len(seen) {
			return contract("role member", "pin inventory differs from member scope")
		}
	}
	return nil
}
func validateImageRequest(q ImageMetadataRequest) error {
	if !validID(string(q.ID)) || !validResourceNode(q.Subject) || !validText(q.Href, 16384) || !validText(q.MediaType, 4096) || !validText(q.BaseURI, 16384) || !validText(q.ArchiveEntry, 4096) || (q.Package != "" && !validID(string(q.Package))) {
		return contract("image request", "invalid identity or bounded source fields")
	}
	return validateResourcePin(q.Source)
}

// ValidateResourceRequest checks bounded closed transport shape without I/O.
func ValidateResourceRequest(q ResourceRequest) error {
	if !validID(string(q.ID)) {
		return contract("resource request", "nonempty bounded request identity required")
	}
	n := 0
	for _, b := range []bool{q.Document != nil, q.Image != nil, q.Association != nil, q.Roles != nil} {
		if b {
			n++
		}
	}
	if n != 1 {
		return contract("resource request", "exactly one branch required")
	}
	switch {
	case q.Document != nil:
		d := q.Document
		if err := validateResourcePin(d.Source); err != nil {
			return err
		}
		if !validResourceNode(d.Subject) || !catalogID(string(d.RequiredProfile)) || !validResourceDigest(d.CatalogDigest) || !validText(d.BaseURI, 16384) || !validText(d.Href, 16384) || !validText(d.RequestedRevision, maxIdentityBytes) || (d.Package != "" && !validID(string(d.Package))) {
			return contract("resource document request", "invalid exact source/profile/base scope")
		}
		if d.Kind != LinkedResourceXML && d.Kind != LinkedResourceMXL && d.Kind != LinkedResourceAuto {
			return contract("resource document request", "unknown representation selection")
		}
	case q.Image != nil:
		if q.Image.ID != q.ID {
			return contract("image request", "request identity mismatch")
		}
		return validateImageRequest(*q.Image)
	case q.Association != nil:
		if q.Association.ID != q.ID {
			return contract("association request", "request identity mismatch")
		}
		return validateAssociationRequest(*q.Association)
	case q.Roles != nil:
		if q.Roles.ID != q.ID {
			return contract("roles request", "request identity mismatch")
		}
		return validateRoleRequest(*q.Roles)
	}
	return nil
}

// ValidateResourceResult checks exact request binding and state/payload shape.
// It does not trust a claimed digest as verified content and never closes Body;
// root orchestration must close every nonnil Body even when this returns error.
func ValidateResourceResult(q ResourceRequest, r ResourceResult) error {
	if err := ValidateResourceRequest(q); err != nil {
		return err
	}
	if r.RequestID != q.ID || !validText(r.RetrievalURI, 16384) || !validText(r.MediaType, 4096) {
		return contract("resource result", "wrong request identity or excessive metadata")
	}
	payloads := 0
	for _, b := range []bool{r.Body != nil, r.Image != nil, r.Association != nil, r.Roles != nil} {
		if b {
			payloads++
		}
	}
	if r.State != EvidenceKnown {
		if r.State != EvidenceAbsent && r.State != EvidenceUnknown && r.State != EvidenceUnsupported {
			return contract("resource result", "invalid transport evidence state")
		}
		if payloads != 0 || r.Pin != (ResourcePin{}) || !validID(string(r.Reason)) || r.RetrievalURI != "" || r.MediaType != "" {
			return contract("resource result", "unavailable result cannot carry usable payload or identity")
		}
		if r.State == EvidenceAbsent {
			if !r.Authority.ExactRelationComplete {
				return contract("resource absence", "authoritative exact request coverage required")
			}
			return validateAuthority(r.Authority, true)
		}
		return validateAuthority(r.Authority, false)
	}
	if payloads != 1 || r.Reason != "" {
		return contract("resource result", "known result requires one matching payload and no unavailable reason")
	}
	switch {
	case q.Document != nil:
		if r.Body == nil {
			return contract("resource document result", "body required")
		}
		if err := validateResourcePin(r.Pin); err != nil {
			return err
		}
		if r.Pin.Representation != ResourceSourceXML && r.Pin.Representation != ResourceRawMXL {
			return contract("resource document result", "original XML/MXL bytes required")
		}
		if q.Document.Kind == LinkedResourceXML && r.Pin.Representation != ResourceSourceXML || q.Document.Kind == LinkedResourceMXL && r.Pin.Representation != ResourceRawMXL {
			return contract("resource document result", "representation differs from request")
		}
		if q.Document.RequestedRevision != "" && q.Document.RequestedRevision != r.Pin.Revision {
			return contract("resource document result", "revision differs from request")
		}
	case q.Association != nil:
		if r.Association == nil || r.Pin != (ResourcePin{}) || r.RetrievalURI != "" || r.MediaType != "" {
			return contract("association result", "typed relation payload only")
		}
		a := r.Association
		if err := validateAssociationRequest(a.Request); err != nil {
			return err
		}
		if a.Request != *q.Association {
			return contract("association result", "exact echoed request/pins required")
		}
		if err := validateEvidence(a.Membership, "association membership"); err != nil {
			return err
		}
		if a.Membership.State == EvidenceAbsent {
			return contract("association result", "partial miss must be unknown, not absent membership")
		}
		if err := validateAuthority(a.Authority, a.Membership.State == EvidenceKnown); err != nil {
			return err
		}
		if a.Membership.State == EvidenceKnown && !a.Membership.Value && !a.Authority.ExactRelationComplete {
			return contract("association result", "negative membership requires exact authoritative coverage")
		}
		for _, id := range []Evidence[string]{a.TargetPartID, a.TargetInstrumentID} {
			if err := validateEvidence(id, "association target ID"); err != nil {
				return err
			}
			if id.State == EvidenceKnown && !validID(id.Value) {
				return contract("association target ID", "normalized ID required")
			}
		}
	case q.Roles != nil:
		if r.Roles == nil || r.Pin != (ResourcePin{}) || r.RetrievalURI != "" || r.MediaType != "" {
			return contract("role result", "typed role payload only")
		}
		v := r.Roles
		if err := validateRoleRequest(v.Request); err != nil {
			return err
		}
		if !equalRoleRequest(v.Request, *q.Roles) {
			return contract("role result", "full exact request scope must be echoed")
		}
		if len(v.Resources) > 65536 {
			return contract("role result", "too many role assertions")
		}
		if err := validateEvidence(v.ScopeComplete, "role result scope"); err != nil {
			return err
		}
		if v.ScopeComplete.State == EvidenceAbsent {
			return contract("role result", "absence is not scope completeness")
		}
		if v.ScopeComplete.State == EvidenceKnown && v.ScopeComplete.Value && (q.Roles.ScopeComplete.State != EvidenceKnown || !q.Roles.ScopeComplete.Value) {
			return contract("role result", "cannot broaden incomplete requested scope")
		}
		if err := validateAuthority(v.Authority, len(v.Resources) > 0 || v.ScopeComplete.State == EvidenceKnown && v.ScopeComplete.Value); err != nil {
			return err
		}
		if v.ScopeComplete.State == EvidenceKnown && v.ScopeComplete.Value && !v.Authority.ExactRelationComplete {
			return contract("role result", "complete role scope requires exact authority")
		}
		allowed := make(map[ResourcePin]bool, len(q.Roles.Resources))
		for _, p := range q.Roles.Resources {
			allowed[p] = true
		}
		members := make(map[resourceRoleMemberKey]bool, len(q.Roles.Members))
		for _, m := range q.Roles.Members {
			members[roleMemberKey(m)] = true
		}
		type factKey struct {
			p      ResourcePin
			r      ResourceRole
			member resourceRoleMemberKey
		}
		seen := make(map[factKey]bool, len(v.Resources))
		for _, f := range v.Resources {
			if !allowed[f.Resource] || (f.Role != ResourceRoleScore && f.Role != ResourceRoleIndividualPart && f.Role != ResourceRoleOther) {
				return contract("role result", "unrequested resource or unknown role")
			}
			key := factKey{p: f.Resource, r: f.Role}
			if len(q.Roles.Members) > 0 {
				if f.Member == nil || f.Member.Resource != f.Resource {
					return contract("role result", "exact requested member required")
				}
				if err := validateRoleMember(*f.Member); err != nil {
					return err
				}
				key.member = roleMemberKey(*f.Member)
				if !members[key.member] {
					return contract("role result", "unrequested member")
				}
			} else if f.Member != nil {
				return contract("role result", "member outside legacy pin-only request")
			}
			if seen[key] {
				return contract("role result", "duplicate resource role")
			}
			seen[key] = true
			if err := validateEvidence(f.Membership, "role membership"); err != nil {
				return err
			}
			if f.Membership.State == EvidenceAbsent {
				return contract("role membership", "partial miss is unknown")
			}
			if f.Membership.State == EvidenceKnown && !f.Membership.Value && !v.Authority.ExactRelationComplete {
				return contract("role membership", "exact authority required for negative")
			}
		}
	case q.Image != nil:
		if r.Image == nil || r.Pin != (ResourcePin{}) || r.RetrievalURI != "" || r.MediaType != "" {
			return contract("image result", "typed image metadata only")
		}
		v := r.Image
		if err := validateImageRequest(v.Request); err != nil {
			return err
		}
		if v.Request != *q.Image {
			return contract("image result", "exact request must be echoed")
		}
		if err := validateResourcePin(v.Resource); err != nil {
			return err
		}
		if v.Resource.Representation != ResourceBinary {
			return contract("image result", "binary immutable resource pin required")
		}
		if v.Unit != ImageUnitPixel && v.Unit != ImageUnitPoint && v.Unit != ImageUnitMillimeter {
			return contract("image result", "explicit supported dimension unit required")
		}
		for _, d := range []Evidence[Decimal]{v.Width, v.Height, v.PixelAspectRatio} {
			if err := validateEvidence(d, "image dimension"); err != nil {
				return err
			}
			if d.State == EvidenceKnown && (!d.Value.Valid() || d.Value.value.Sign() <= 0) {
				return contract("image dimension", "known dimensions must be positive")
			}
		}
		if v.Unit != ImageUnitPixel && v.PixelAspectRatio.State == EvidenceKnown {
			return contract("image dimension", "pixel ratio on nonpixel units")
		}
		if err := validateAuthority(v.Authority, true); err != nil {
			return err
		}
	}
	return nil
}

// Empty slices represent the same ordered scope whether nil or allocated.
// Evidence still compares exact causal values, never pointer identity.
func equalRoleRequest(a, b ResourceRoleRequest) bool {
	return a.ID == b.ID && a.Package == b.Package &&
		equalRoleEvidence(a.ScopeComplete, b.ScopeComplete) &&
		slices.Equal(a.Resources, b.Resources) &&
		slices.EqualFunc(a.Members, b.Members, func(a, b ResourceRoleMember) bool {
			return a.Resource == b.Resource && a.ArchiveEntry == b.ArchiveEntry && equalRoleEvidence(a.BaseURI, b.BaseURI)
		})
}
func equalRoleEvidence[T comparable](a, b Evidence[T]) bool {
	if a.State != b.State || a.Value != b.Value || a.Reason != b.Reason {
		return false
	}
	if a.Cause == nil || b.Cause == nil {
		return a.Cause == b.Cause
	}
	return *a.Cause == *b.Cause
}
