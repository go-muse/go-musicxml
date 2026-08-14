// Package musicxml provides a typed Go model for MusicXML 4.0.
//
// Decode and Encode read and write score-partwise, score-timewise, and opus
// root documents. DecodeScorePartwise, DecodeScoreTimewise, and
// DecodeOpusDocument provide concrete result types when the expected root is
// known. DecodeMXL and EncodeMXL provide the equivalent transport for
// compressed MusicXML archives. DecodeMXLPackage preserves related resources,
// while ResolveOpus and SyncResolvedOpus support linked documents inside an
// archive.
//
// Validation is explicit. Decode and Encode preserve transport semantics and
// do not call Validate automatically. Constructors and ordered-content Add
// methods provide a smaller API for creating documents without manually
// assembling generated choice wrappers.
//
// The generated public types mirror the bundled MusicXML 4.0 XSD. Optional
// attributes are represented by pointers so omission remains distinguishable
// from an explicit zero value. Effective methods expose XSD defaults without
// changing the decoded representation.
package musicxml
