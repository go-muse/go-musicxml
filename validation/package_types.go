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
	// BaseURI and ArchiveEntry retain exact admitted transport provenance.
	BaseURI, ArchiveEntry string
	// PinUnavailable admits only a raw MXL prefix with no verified whole-body
	// digest. Such a boundary is never canonical and can never certify completion.
	PinUnavailable bool
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
	ID      PackageFactID
	Package PackageID
	Name    string
	// RawName retains an exact non-UTF8 byte name only for original-header
	// observation. It is exclusive with Name and grants no path capability.
	RawName           []byte
	Location          Location
	Local, Central    ZIPHeaderObservation
	HeaderAssociation Evidence[bool]
	// UTF8Name reports observed byte validity, not a declared encoding.
	// The EFS encoding flag remains separate in Local.Flags and Central.Flags.
	UTF8Name      Evidence[bool]
	PhysicalOrder Evidence[uint64]
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
// absent media-type versus present-empty and explicit nested indirection. Source
// is an adapter-asserted well-formed provenance reference, not a checked adopted
// container membership capability. Raw adapters must project original container
// occurrences; current MXL may use zero Source with an explicit Location.Path.
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
// adoption. Fragment is the once URI-decoded component: absent and present-empty
// are distinct; empty addresses the whole document. Only schema-ID shorthand
// (the schema kernel NCName grammar) is implemented; other grammars stay unsupported.
// The core validates
// supplied fragment targets against the adopted schema-qualified identity index.
// Indirection is required adapter evidence: Known false asserts the target is
// direct; Known true records traversal through a nested package and does not
// establish that package's primary document as the encoded link destination.
type PackageEdgeObservation struct {
	ID             PackageFactID
	Source         ResourceAdoptionID
	Template       uint64
	Target         Evidence[ResourceAdoptionID]
	Indirection    Evidence[bool]
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
	// BaseURI and ArchiveEntry are exact effective retrieval context, never inferred
	// from canonical resource identity. They participate in expansion equivalence.
	BaseURI, ArchiveEntry string
	Package               PackageID
	DocumentID            string
	Root                  bool
}

// ResourceFailureSummary preserves known child truth when ordinary full-report
// retention is refused or the package session is already terminal. It is
// explicitly incomplete, not a synthetic Report.
type ResourceFailureSummary struct {
	Pin           ResourcePin
	PinComplete   Evidence[bool]
	Target        Target
	Profile       ProfileID
	CatalogDigest string
	Counts        Counts
	// ModelFailures is the failed report's Report.ModelFailures; a nonzero
	// count rejects the bundle like a violation.
	ModelFailures                           uint64
	Conformance                             ConformanceState
	Terminal                                TerminalSummary
	DiagnosticsTruncated, MetadataTruncated bool
}
