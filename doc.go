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
// # Scenarios and validation lines
//
// Three scenarios cover loading and checking MusicXML:
//
//  1. Loading without validation: Decode, DecodeMXL, DecodeMXLPackage and the
//     typed decoders. Only the XML decoder checks the input.
//  2. Strict loading: DecodeAssessed and DecodeMXLAssessed return a valid
//     MusicXML 4.0 structure or an error.
//  3. Checking a structure built or edited in Go: AssessModel and
//     AssessMXLPackage.
//
// They share three validation lines, named by ValidationLine. Line 1
// (LineDecoder) is XML and document correctness inside the decoder. Line 2
// (LineSource, ProfileSourceLine and ProfileMXLSourceLine) checks the
// MusicXML 4.0 and XSD requirements that the loaded structure cannot show,
// because decoding and conversion lose or change the data they read; strict
// loading runs it while it reads. Line 3 (LineModel, ProfileModelSemantics
// and ProfileMXLModelSemantics) checks the loaded structure; strict loading
// runs it after conversion, and scenario 3 runs only it. Lines 2 and 3 do not
// check the same thing, except for the overlap AllowedLineOverlap lists;
// RuleLine reports the line of each rule.
//
// Validation is explicit. Decode and Encode preserve transport semantics and
// do not call Validate automatically. Validate, the fast XSD-only check of a
// model, and the Inspect functions are diagnostic tools outside the
// scenarios: InspectXML and InspectMXL assess original bytes against an
// explicitly selected profile, one of the Profile constants, and
// InspectXMLWithResources and AssessModelWithResources assess graphs of
// linked documents. Reports are validation.Report values; resource graphs and
// MXL packages return a validation.AssessmentBundle.
//
// Each kind of entry point takes its own options type, and the Profile field
// of each has its own profile type: SourceOptions and SourceProfile (the
// ProfileSource* constants), ModelOptions and ModelProfile (ProfileModel*),
// GraphOptions and GraphProfile (ProfileResourceGraph*), MXLGraphOptions and
// MXLProfile (ProfileMXL*). A profile of another kind does not compile; ID
// gives the engine identity that reports carry.
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
