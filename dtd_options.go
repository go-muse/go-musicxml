package musicxml

import (
	"context"
	"errors"
	"fmt"
)

var (
	// ErrDTDNotFound is returned by a DTDSource that does not provide the
	// requested identifier. The decoder then uses the bundled official MusicXML
	// DTDs, if one of them matches the identifier.
	ErrDTDNotFound = errors.New("musicxml: DTD not provided by source")

	// ErrDTDUnavailable identifies a DTD that could not be supplied or read:
	// a DTDSource error other than ErrDTDNotFound, or caller bytes that are not
	// a well-formed external DTD subset. *DTDError matches it.
	ErrDTDUnavailable = errors.New("musicxml: DTD unavailable")
)

// DTDRequest identifies one external DTD entity that a document needs: the
// external subset named by its DOCTYPE or an external parameter entity
// referenced from a caller-supplied DTD.
type DTDRequest struct {
	// PublicID is the public identifier as written, or empty.
	PublicID string
	// SystemID is the system identifier as written, or empty.
	SystemID string
	// BaseSystemID is the system identifier of the DTD entity that contains
	// the declaration of the requested parameter entity, for resolving a
	// relative SystemID (XML 4.2.2). It is empty for the DOCTYPE external
	// subset and for parameter entities declared in the internal subset.
	BaseSystemID string
}

// DTDSource supplies caller-authorized DTD bytes. It is consulted before the
// bundled official MusicXML DTDs and never causes network access by itself.
//
// For a DOCTYPE external subset, returning ErrDTDNotFound selects the bundled
// DTD for that identifier, including its modules. When the source supplies the
// external subset, every external parameter entity it references is requested
// from the source first and from the bundled DTDs second. Any other error
// fails the operation that needs the DTD with a *DTDError.
//
// The returned bytes must be UTF-8, optionally with a byte order mark and a
// text declaration, and are not retained after the call that requested them.
// One entity may not exceed 16 MiB.
//
// A bundled closure is parsed once per process, but an external subset that
// the source supplies is parsed for every document that names it, under the
// assessment's budgets when one reads it. A source should therefore return
// ErrDTDNotFound for identifiers it does not own.
// Ordinary decoding resolves the external subset of every document with an
// external DOCTYPE; it ignores identifiers longer than 4096 bytes.
//
// DTDSource is implemented by callers, so its method set is frozen.
type DTDSource interface {
	OpenDTD(ctx context.Context, request DTDRequest) ([]byte, error)
}

// DTDSourceFunc adapts a function to DTDSource.
type DTDSourceFunc func(ctx context.Context, request DTDRequest) ([]byte, error)

// OpenDTD calls f(ctx, request).
func (f DTDSourceFunc) OpenDTD(ctx context.Context, request DTDRequest) ([]byte, error) {
	return f(ctx, request)
}

// DTDOptions configures how external DTD subsets are obtained. The official
// MusicXML 4.0 DTDs are bundled and resolve the public identifiers of
// MusicXML 1.0 through 4.0 as the official MusicXML catalog does; a nil
// *DTDOptions uses only them.
type DTDOptions struct {
	// Source, when non-nil, is consulted before the bundled DTDs. A nil or
	// typed-nil Source is ignored.
	Source DTDSource
}

// DTDError reports a DTD that could not be supplied or read. It matches
// ErrDTDUnavailable and unwraps to the source error or to a description of the
// defect in the DTD bytes, which never matches *xml.SyntaxError.
type DTDError struct {
	PublicID string
	SystemID string
	Err      error
}

// Error describes the DTD and its cause.
func (e *DTDError) Error() string {
	if e == nil {
		return "musicxml: nil DTD error"
	}
	return fmt.Sprintf(
		"%v: PUBLIC %q SYSTEM %q: %v",
		ErrDTDUnavailable,
		e.PublicID,
		e.SystemID,
		e.Err,
	)
}

// Is makes DTDError match ErrDTDUnavailable.
func (e *DTDError) Is(target error) bool {
	return e != nil && target == ErrDTDUnavailable
}

// Unwrap returns the source or syntax error.
func (e *DTDError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
