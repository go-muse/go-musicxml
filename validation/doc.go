// Package validation provides a bounded fact-stream assessment foundation.
// It does not parse XML, traverse musicxml models, fetch external resources, or
// provide a ready MusicXML conformance profile. Callers supply reviewed catalogs
// and truthful facts; reports describe only the explicitly selected obligations.
//
// Most applications do not call this package directly. The root musicxml
// package's InspectXML, AssessModel, DecodeAssessed and related functions build
// the MusicXML catalogs, drive the sessions and return the types defined here.
//
// # API levels
//
// The exported identifiers serve three different audiences.
//
// Consumer API: the results and errors that the root package's assessment
// functions return. It includes Report, Finding and Decision, AssessmentBundle,
// the error types IncompleteError, ConfigurationError, ResourceError and
// SessionError, the sentinels ErrIncompleteAssessment,
// ErrInvalidConfiguration, ErrInvalidSession, ErrResourceLimit and
// ErrProcessingPolicy, Budgets, ProfileID, the identifier and value types
// (RuleKey, NodeRef, QName, Location, Evidence, Atomic, Integer, Decimal,
// Rational) and the *ContextResult projections.
//
// Catalog API: NewCatalog, the generic New*Plan constructors, CompileSchema
// and the Schema* metadata aliases, NewEngine, Engine.Begin and the Session
// fact events Open, Attribute, CompleteAttributes, Close and Finish. They let a
// caller assess its own fact stream; see the Engine example.
//
// Adapter API: hooks that let the root musicxml package stream source-parser,
// MXL-package and resource facts into an engine: the Session and Engine
// Source* methods, CompleteSourceDTDIDs, SourceEntityDeclarations,
// FinishWithRetainedReport, FinishWithResourceSnapshot, PackageSession and
// AllocationSize. Their doc comments carry an "Adapter API" paragraph. They
// are exported because the root package is a separate package; application
// code is not expected to need them. Similarly, MusicXML-specific plan
// constructors such as NewChordDurationPlan validate schema roles that only
// the root package's compiled MusicXML schemas provide.
//
// # Comparing values
//
// Integer, Decimal and Rational hold internal pointers. They and the structs
// that contain them, such as Atomic, Scalar and ContextPosition, compile with
// == but do not compare numeric values; use the Equal and Cmp methods. The
// identifier types RuleKey, NodeRef, QName, DeclarationID, ComponentID and
// ResourcePin contain only comparable scalar fields and are suitable map keys.
//
// # Implementing interfaces
//
// ResourceResolver is a single-method interface that callers implement. New
// resolver capabilities will be offered as separate optional interfaces that
// the engine discovers with type assertions, not as new methods. A resolver
// that receives a request branch it does not recognize should return a result
// carrying the request's ID, State EvidenceUnsupported, a Reason and no
// payload; ValidateResourceResult accepts that result for every request.
package validation

import "io"

// Exported handles that own releasable resources implement io.Closer.
var (
	_ io.Closer = (*DictionarySet)(nil)
	_ io.Closer = (*ResourceSnapshot)(nil)
	_ io.Closer = (*ReportLease)(nil)
)
