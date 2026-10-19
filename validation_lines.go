package musicxml

import (
	"sync"

	"github.com/go-muse/go-musicxml/validation"
)

// ValidationLine names one of the three validation lines of the loading
// scenarios. Scenario 1 (Decode) runs line 1; scenario 2 (DecodeAssessed,
// DecodeMXLAssessed) runs lines 1, 2 and 3; scenario 3 (AssessModel,
// AssessMXLPackage) runs line 3. Lines 2 and 3 do not check the same thing,
// except for the overlap that AllowedLineOverlap lists.
type ValidationLine uint8

const (
	// LineDecoder is XML and document correctness inside the decoder:
	// well-formedness, encoding, entities and the DOCTYPE. It has no rules.
	LineDecoder ValidationLine = 1
	// LineSource is the MusicXML 4.0 and XSD requirements that the loaded
	// structure cannot show, because decoding and conversion lose or change
	// the data they read: ProfileSourceLine and ProfileMXLSourceLine.
	LineSource ValidationLine = 2
	// LineModel is the checks on the loaded structure: ProfileModelSemantics
	// and ProfileMXLModelSemantics.
	LineModel ValidationLine = 3
)

// String returns "decoder", "source" or "model".
func (l ValidationLine) String() string {
	switch l {
	case LineDecoder:
		return "decoder"
	case LineSource:
		return "source"
	case LineModel:
		return "model"
	}
	return "unknown"
}

// RuleLine reports the validation line that owns a rule revision of a public
// profile. The second result is false for a rule that no public profile
// requires.
//
// A rule that reads data only the original bytes carry belongs to line 2;
// every other rule belongs to line 3. The diagnostic source profiles, such as
// ProfileSourceSemantics, also evaluate line-3 rules on the source; that does
// not move them to line 2. ProfileMXLModelSemantics assesses the files that a
// converted MXLPackage keeps as bytes with source profiles, because for those
// files the bytes are the structure.
func RuleLine(rule validation.RuleKey) (ValidationLine, bool) {
	entry, ok := ruleLineIndex()[rule]
	return entry.line, ok
}

// AllowedLineOverlap returns the rule IDs that both line 2 and line 3 check,
// each with the reason the overlap is accepted. The line profiles count with
// the profiles they bind for package resources, including the source
// profiles that line 3 binds for the files a converted MXLPackage keeps as
// bytes. Any other rule ID that both lines require is a defect.
func AllowedLineOverlap() map[validation.RuleID]string {
	out := make(map[validation.RuleID]string, len(allowedLineOverlap))
	for id, reason := range allowedLineOverlap {
		out[id] = reason
	}
	return out
}

type ruleLineGroup struct {
	line   ValidationLine
	reason string
	rules  []validation.RuleKey
}

type ruleLineEntry struct {
	line   ValidationLine
	reason string
}

var ruleLineIndex = sync.OnceValue(func() map[validation.RuleKey]ruleLineEntry {
	index := make(map[validation.RuleKey]ruleLineEntry)
	for _, g := range ruleLineGroups {
		for _, key := range g.rules {
			index[key] = ruleLineEntry{g.line, g.reason}
		}
	}
	return index
})

const (
	overlapXSD      = "The six XSD operations run whole on line 2, on the original bytes. Cutting out the subchecks that the converted structure shows with the same result would need the core to know which XSD particles the generator keeps in Contents slices, for a small saving."
	overlapArchive  = "ProfileMXLModelSemantics evaluates the archive-byte and schema-hint checks only on the files a converted MXLPackage retains as bytes, such as a nested raw MXL; for its current-model resources they do not apply."
	overlapRetained = "Line 3 assesses the files a converted MXLPackage keeps as bytes, and the container of a nested raw MXL, with source profiles, because for those files the bytes are the structure. Line 2 assesses only what conversion consumes, the container and the first rootfile document of the original archive, so no file gets these checks on both lines."
)

var allowedLineOverlap = map[validation.RuleID]string{
	"MX40-XSD-musicxml-binding": overlapXSD, "MX40-XSD-musicxml-declaration": overlapXSD, "MX40-XSD-musicxml-attributes": overlapXSD,
	"MX40-XSD-musicxml-scalar": overlapXSD, "MX40-XSD-musicxml-content": overlapXSD, "MX40-XSD-musicxml-identity": overlapXSD,
	"MX40-XSD-opus-binding": overlapXSD, "MX40-XSD-opus-declaration": overlapXSD, "MX40-XSD-opus-attributes": overlapXSD,
	"MX40-XSD-opus-scalar": overlapXSD, "MX40-XSD-opus-content": overlapXSD, "MX40-XSD-opus-identity": overlapXSD,
	"MX40-PACKAGE-container-path": overlapArchive, "MX40-PACKAGE-mimetype-stored": overlapArchive, "MX40-PACKAGE-mimetype-no-extra": overlapArchive,
	"MX40-PACKAGE-mimetype-no-prefix": overlapArchive, "MX40-PACKAGE-mimetype-advice": overlapArchive, "MX40-PACKAGE-entry-name-advice": overlapArchive,
	"MX40-PACKAGE-compression-observation": overlapArchive, "MX40-PACKAGE-schema-hint-pairs": overlapArchive,
	"MX40-SOURCE-ORIGIN-musicxml": overlapRetained, "MX40-SOURCE-ORIGIN-opus": overlapRetained, "MX40-SOURCE-ORIGIN-container": overlapRetained,
	"MX40-RESOURCE-schema-hints/musicxml": overlapRetained, "MX40-RESOURCE-schema-hints/opus": overlapRetained, "MX40-RESOURCE-schema-hints/container": overlapRetained,
	"MX40-CONTEXT-container-metadata":   overlapRetained,
	"MX40-SOURCE-XSD-container-binding": overlapRetained, "MX40-SOURCE-XSD-container-declaration": overlapRetained, "MX40-SOURCE-XSD-container-attributes": overlapRetained,
	"MX40-SOURCE-XSD-container-scalar": overlapRetained, "MX40-SOURCE-XSD-container-content": overlapRetained, "MX40-SOURCE-XSD-container-identity": overlapRetained,
}
