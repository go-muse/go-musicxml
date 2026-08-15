package musicxml

import (
	"encoding/xml"
	"errors"
	"fmt"
)

var (
	// ErrNilReader reports a nil input reader.
	ErrNilReader = errors.New("musicxml: nil reader")

	// ErrNilWriter reports a nil output writer.
	ErrNilWriter = errors.New("musicxml: nil writer")

	// ErrNilDocument reports a nil or typed-nil document.
	ErrNilDocument = errors.New("musicxml: nil document")

	// ErrNilMXLPackage reports a nil MXL package.
	ErrNilMXLPackage = errors.New("musicxml: nil MXL package")

	// ErrNilMXLResolvedOpus reports a nil resolved opus graph.
	ErrNilMXLResolvedOpus = errors.New("musicxml: nil resolved MXL opus")

	// ErrMXLNotOpus reports an attempt to resolve a non-opus package.
	ErrMXLNotOpus = errors.New("musicxml: MXL document is not an opus")

	// ErrMXLResolvedOpusMismatch reports a graph from another package.
	ErrMXLResolvedOpusMismatch = errors.New("musicxml: resolved opus does not belong to MXL package")

	// ErrMXLResolvedOpusStale reports package changes made after resolution.
	ErrMXLResolvedOpusStale = errors.New("musicxml: resolved opus no longer matches MXL package")

	// ErrMXLResolvedOpusInvalid reports an invalid resolved graph topology.
	ErrMXLResolvedOpusInvalid = errors.New("musicxml: invalid resolved opus graph")

	// ErrMXLInvalidLink reports an invalid link inside an MXL package.
	ErrMXLInvalidLink = errors.New("musicxml: invalid MXL link")

	// ErrMXLExternalLink reports a link that leaves the MXL package.
	ErrMXLExternalLink = errors.New("musicxml: external MXL link")

	// ErrMXLLinkedDocumentNotFound reports a missing linked document.
	ErrMXLLinkedDocumentNotFound = errors.New("musicxml: linked MXL document not found")

	// ErrMXLLinkedDocumentInvalid reports malformed linked MusicXML.
	ErrMXLLinkedDocumentInvalid = errors.New("musicxml: invalid linked MXL document")

	// ErrMXLLinkedDocumentType reports a link to the wrong root type.
	ErrMXLLinkedDocumentType = errors.New("musicxml: unexpected linked MXL document type")

	// ErrEmptyDocument reports input without an XML root element.
	ErrEmptyDocument = errors.New("musicxml: empty document")

	// ErrUnsupportedRoot reports an unsupported or namespaced root element.
	ErrUnsupportedRoot = errors.New("musicxml: unsupported root element")

	// ErrUnsupportedDocument reports a Document implementation that cannot be
	// encoded or validated.
	ErrUnsupportedDocument = errors.New("musicxml: unsupported document")

	// ErrInvalidDocument reports one or more MusicXML XSD violations.
	ErrInvalidDocument = errors.New("musicxml: invalid document")

	// ErrDocumentTooDeep reports a programmatically constructed document whose
	// element nesting exceeds the safe model traversal limit.
	ErrDocumentTooDeep = errors.New(
		"musicxml: document nesting depth limit exceeded",
	)

	// ErrDocumentCycle reports a cyclic programmatically constructed opus.
	ErrDocumentCycle = errors.New("musicxml: cyclic document model")

	// ErrXMLTooDeep reports an XML element nesting depth limit violation.
	ErrXMLTooDeep = errors.New("musicxml: XML nesting depth limit exceeded")

	// ErrInvalidDecodeOptions reports invalid uncompressed decode limits.
	ErrInvalidDecodeOptions = errors.New("musicxml: invalid decode options")

	// ErrInvalidMXLOptions reports invalid compressed decode limits.
	ErrInvalidMXLOptions = errors.New("musicxml: invalid MXL options")

	// ErrInvalidMXL reports a malformed or unsafe MXL archive.
	ErrInvalidMXL = errors.New("musicxml: invalid MXL archive")

	// ErrMXLTooLarge reports an MXL size limit violation.
	ErrMXLTooLarge = fmt.Errorf(
		"%w: size limit exceeded",
		ErrInvalidMXL,
	)

	// ErrMXLContainerNotFound reports a missing META-INF/container.xml.
	ErrMXLContainerNotFound = fmt.Errorf(
		"%w: META-INF/container.xml not found",
		ErrInvalidMXL,
	)

	// ErrMXLInvalidContainer reports malformed MXL container metadata.
	ErrMXLInvalidContainer = fmt.Errorf(
		"%w: invalid META-INF/container.xml",
		ErrInvalidMXL,
	)

	// ErrMXLRootFileNotFound reports a missing declared root file.
	ErrMXLRootFileNotFound = fmt.Errorf(
		"%w: root MusicXML file not found",
		ErrInvalidMXL,
	)

	// ErrMXLInvalidPath reports an unsafe or non-canonical ZIP path.
	ErrMXLInvalidPath = fmt.Errorf(
		"%w: invalid archive path",
		ErrInvalidMXL,
	)

	// ErrMXLDuplicateEntry reports a duplicate ZIP or rootfile entry.
	ErrMXLDuplicateEntry = fmt.Errorf(
		"%w: duplicate archive entry",
		ErrInvalidMXL,
	)

	// ErrMXLInvalidMIMEType reports a malformed MXL mimetype entry.
	ErrMXLInvalidMIMEType = fmt.Errorf(
		"%w: invalid mimetype entry",
		ErrInvalidMXL,
	)

	// ErrMXLUnsupportedMediaType reports an unsupported primary rootfile media
	// type.
	ErrMXLUnsupportedMediaType = fmt.Errorf(
		"%w: unsupported root file media type",
		ErrInvalidMXL,
	)
)

// UnsupportedRootError identifies an unsupported XML root element.
type UnsupportedRootError struct {
	Name xml.Name
}

// Error returns the unsupported root description.
func (e *UnsupportedRootError) Error() string {
	return fmt.Sprintf(
		"%s: {%s}%s",
		ErrUnsupportedRoot,
		e.Name.Space,
		e.Name.Local,
	)
}

// Unwrap makes UnsupportedRootError match ErrUnsupportedRoot.
func (e *UnsupportedRootError) Unwrap() error {
	return ErrUnsupportedRoot
}

// MXLLinkError reports a link that could not be resolved inside an MXL
// package.
type MXLLinkError struct {
	SourcePath string
	Href       string
	TargetPath string
	Err        error
}

// Error returns the link-resolution description.
func (e *MXLLinkError) Error() string {
	target := ""
	if e.TargetPath != "" {
		target = fmt.Sprintf(" to %q", e.TargetPath)
	}

	return fmt.Sprintf(
		"musicxml: resolve MXL link %q from %q%s: %v",
		e.Href,
		e.SourcePath,
		target,
		e.Err,
	)
}

// Unwrap returns the link-resolution cause.
func (e *MXLLinkError) Unwrap() error {
	return e.Err
}
