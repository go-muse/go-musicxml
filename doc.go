// Package musicxml provides a typed Go model for MusicXML 4.0.
//
// # Reading and writing
//
// Decode and Encode read and write score-partwise, score-timewise, and opus
// root documents. DecodeScorePartwise, DecodeScoreTimewise, and
// DecodeOpusDocument provide concrete result types when the expected root is
// known; AsScorePartwise, AsScoreTimewise, and AsOpusDocument narrow a
// polymorphic Document. DecodeMXL and EncodeMXL provide the equivalent
// transport for compressed MusicXML archives, with typed DecodeMXLScorePartwise,
// DecodeMXLScoreTimewise, and DecodeMXLOpusDocument variants.
// DecodeMXLPackage preserves related resources, while ResolveOpus and
// SyncResolvedOpus support linked documents inside an archive.
//
// DecodeContext, DecodeMXLContext, and DecodeMXLPackageContext accept a
// context.Context and stop at their next cancellation check after it is done;
// a Read call that blocks is not interrupted. EncodeWithOptions adds
// an XML declaration or indentation while keeping Encode's checks. The
// WithOptions variants set resource limits; zero option values select the
// package defaults.
//
// # Validation and assessment
//
// Validation is explicit. Decode and Encode preserve transport semantics and
// do not call Validate automatically. Validate checks a model against the
// bundled MusicXML 4.0 XSD and reports failures as a *ValidationError.
//
// InspectXML assesses original document bytes and AssessModel assesses a
// current model against an explicitly selected validation profile, one of the
// Profile constants, and return a validation.Report. DecodeAssessed returns a
// document only after both its source and its model assessment succeed. The
// WithDictionaries variants add pinned dictionary evidence, for example from
// BundledDictionarySource, and InspectXMLWithResources, InspectMXL,
// AssessMXLPackage and their variants also assess linked resources and MXL
// packages, returning a validation.AssessmentBundle.
//
// # Constructing documents
//
// Constructors and ordered-content Add methods provide a smaller API for
// creating documents without manually assembling generated choice wrappers.
// Directly assigning generated fields is equally supported.
//
// The generated public types mirror the bundled MusicXML 4.0 XSD. Optional
// attributes are represented by pointers so omission remains distinguishable
// from an explicit zero value. Effective methods expose XSD defaults without
// changing the decoded representation.
//
// # Errors
//
// Match the Err sentinel values with errors.Is and the typed errors, such as
// *UnsupportedRootError, *ValidationError, *MXLLinkError, *ConversionError,
// and *DictionaryImportError, with errors.As. Error message text is not part
// of the API.
//
// # Compatibility
//
// API.md in the module repository describes the compatibility policy,
// including which changes to the generated model are additive and which
// properties of generated types, such as comparability and their
// encoding/xml method sets, are not guaranteed.
package musicxml
