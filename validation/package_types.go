package validation

// PackageFactID identifies a positive typed subject in one package session.
// Adapter IDs occupy the low 63 bits; core-generated package/resource subjects
// use the disjoint high range. No XML node is synthesized.
type PackageFactID uint64

// PackageIdentity opens an exact encapsulation boundary. Parent is empty only
// for the root boundary; linked cycles are independent of acyclic containment.
type PackageIdentity struct {
	ID             PackageID
	Parent         PackageID
	Representation ResourceRepresentation
	Pin            ResourcePin
}

// ZIPHeaderObservation preserves independent original-header evidence.
type ZIPHeaderObservation struct {
	Present       Evidence[bool]
	Method, Flags Evidence[uint16]
	ExtraLength   Evidence[uint64]
}

// PackageEntryObservation is one exact bounded archive entry association.
// Local proof remains usable independently of later central-directory failure.
type PackageEntryObservation struct {
	ID                PackageFactID
	Package           PackageID
	Name              string
	Location          Location
	Local, Central    ZIPHeaderObservation
	HeaderAssociation Evidence[bool]
	UTF8Name          Evidence[bool]
	PhysicalOrder     Evidence[uint64]
}

// PackageMimetypeObservation carries an original initial byte prefix (at most
// 64 bytes), measured total size, and independent content completion evidence.
type PackageMimetypeObservation struct {
	Entry           PackageFactID
	Prefix          []byte
	ByteLength      Evidence[uint64]
	ContentComplete Evidence[bool]
}

// PackageRootfileObservation keeps encoded order and token semantics, including
// absent media-type versus present-empty and explicit nested indirection.
type PackageRootfileObservation struct {
	ID                  PackageFactID
	Package             PackageID
	Source              NodeRef
	Location            Location
	Ordinal             uint64
	FullPath, MediaType Evidence[string]
	Entry               Evidence[PackageFactID]
	Resource            Evidence[ResourceAssessmentID]
	Indirection         Evidence[bool]
}

// PackageEdgeObservation binds an encoded intrinsic template to a fresh target
// adoption. Fragment absent and present-empty are distinct. The core validates
// supplied fragment targets against the adopted schema-qualified identity index.
type PackageEdgeObservation struct {
	ID             PackageFactID
	Source         ResourceAdoptionID
	Template       uint64
	Target         Evidence[ResourceAdoptionID]
	Fragment       Evidence[string]
	FragmentTarget Evidence[NodeRef]
}

// PackageEnumeration seals each independent package/graph scope explicitly.
// Missing sealing never proves empty scope, absent entries, or vacuity.
type PackageEnumeration struct {
	Package                          PackageID
	Entries, Rootfiles, Edges, Roles Evidence[bool]
}

// PackageCompletion records end of bounded root orchestration and its real cause.
type PackageCompletion struct {
	InputComplete bool
	Cause         error
}

// ResourceAdoptionOptions distinguishes the single primary root adoption from
// all linked occurrences, each of which charges LinkedDocuments even on reuse.
type ResourceAdoptionOptions struct {
	Package    PackageID
	DocumentID string
	Root       bool
}

// ResourceFailureSummary preserves known child truth when the quota refuses a
// complete retained report. It is explicitly incomplete, not a synthetic Report.
type ResourceFailureSummary struct {
	Pin                                     ResourcePin
	Target                                  Target
	Profile                                 ProfileID
	CatalogDigest                           string
	Counts                                  Counts
	Conformance                             ConformanceState
	Terminal                                TerminalSummary
	DiagnosticsTruncated, MetadataTruncated bool
}
