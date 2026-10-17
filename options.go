package musicxml

import "fmt"

const (
	defaultMaxXMLDepth = 256
	maximumXMLDepth    = 4096
)

// DecodeOptions controls resource limits for uncompressed XML decoding.
//
// Zero values select the package defaults. Negative values and XML depths
// above the documented maximum are invalid.
type DecodeOptions struct {
	// MaxXMLDepth is the maximum number of simultaneously open XML elements.
	// Zero selects 256; values above 4096 are invalid.
	MaxXMLDepth int
	// DTD supplies caller DTD bytes ahead of the bundled official MusicXML
	// DTDs. Nil uses only the bundled DTDs. Decoding uses an external subset
	// only for character entities and defaulted namespace declarations; other
	// DTD defaults are not added to the decoded model.
	DTD *DTDOptions
}

// DefaultDecodeOptions returns the limits used by Decode and the typed decode
// helpers.
func DefaultDecodeOptions() DecodeOptions {
	return DecodeOptions{MaxXMLDepth: defaultMaxXMLDepth}
}

// MXLOptions controls resource limits for compressed MusicXML decoding and
// package assessment.
//
// Zero values select the package defaults. Negative values and XML depths
// above the documented maximum are invalid. Package assessment applies the
// archive and per-entry limits to the corresponding archive entries, with the
// additional accounting described in API.md.
type MXLOptions struct {
	// MaxArchiveBytes caps the compressed archive bytes read from the input.
	// Zero selects 64 MiB.
	MaxArchiveBytes int64
	// MaxMetadataBytes caps each uncompressed metadata entry, mimetype and
	// META-INF/container.xml. Zero selects 1 MiB.
	MaxMetadataBytes int64
	// MaxDocumentBytes caps the uncompressed primary MusicXML document.
	// Zero selects 256 MiB.
	MaxDocumentBytes int64
	// MaxResourceBytes caps each uncompressed non-primary entry on its own,
	// while MaxResourcesBytes caps their total. Zero selects 256 MiB.
	MaxResourceBytes int64
	// MaxResourcesBytes caps additional resource bytes in DecodeMXLPackage,
	// excluding mimetype, container and primary entries. Package assessment
	// instead counts all selected expanded entries (including those three),
	// current resource copies and bounded external bodies in package scope.
	// Zero selects 512 MiB.
	MaxResourcesBytes int64
	// MaxXMLDepth has the same zero default and 4096 maximum as DecodeOptions.
	MaxXMLDepth int
	// DTD has the same meaning as DecodeOptions.DTD for the root document.
	DTD *DTDOptions
}

// DefaultMXLOptions returns the limits used by DecodeMXL and
// DecodeMXLPackage.
func DefaultMXLOptions() MXLOptions {
	return MXLOptions{
		MaxArchiveBytes:   maxMXLArchiveSize,
		MaxMetadataBytes:  maxMXLMetadataSize,
		MaxDocumentBytes:  maxMXLDocumentSize,
		MaxResourceBytes:  maxMXLResourceSize,
		MaxResourcesBytes: maxMXLResourcesSize,
		MaxXMLDepth:       defaultMaxXMLDepth,
	}
}

func (o DecodeOptions) xmlDepth() (int, error) {
	if o.MaxXMLDepth < 0 || o.MaxXMLDepth > maximumXMLDepth {
		return 0, fmt.Errorf(
			"%w: MaxXMLDepth must be between 0 and %d",
			ErrInvalidDecodeOptions,
			maximumXMLDepth,
		)
	}
	if o.MaxXMLDepth == 0 {
		return defaultMaxXMLDepth, nil
	}

	return o.MaxXMLDepth, nil
}

func (o MXLOptions) limits() (mxlLimits, error) {
	defaults := DefaultMXLOptions()
	if o.MaxArchiveBytes < 0 ||
		o.MaxMetadataBytes < 0 ||
		o.MaxDocumentBytes < 0 ||
		o.MaxResourceBytes < 0 ||
		o.MaxResourcesBytes < 0 {
		return mxlLimits{}, fmt.Errorf(
			"%w: byte limits must not be negative",
			ErrInvalidMXLOptions,
		)
	}
	if o.MaxXMLDepth < 0 || o.MaxXMLDepth > maximumXMLDepth {
		return mxlLimits{}, fmt.Errorf(
			"%w: MaxXMLDepth must be between 0 and %d",
			ErrInvalidMXLOptions,
			maximumXMLDepth,
		)
	}

	return mxlLimits{
		archiveSize: valueOrDefault(
			o.MaxArchiveBytes,
			defaults.MaxArchiveBytes,
		),
		metadataSize: valueOrDefault(
			o.MaxMetadataBytes,
			defaults.MaxMetadataBytes,
		),
		documentSize: valueOrDefault(
			o.MaxDocumentBytes,
			defaults.MaxDocumentBytes,
		),
		resourceSize: valueOrDefault(
			o.MaxResourceBytes,
			defaults.MaxResourceBytes,
		),
		resourcesSize: valueOrDefault(
			o.MaxResourcesBytes,
			defaults.MaxResourcesBytes,
		),
		dtd: o.DTD,
		xmlDepth: int(valueOrDefault(
			int64(o.MaxXMLDepth),
			int64(defaults.MaxXMLDepth),
		)),
	}, nil
}

func valueOrDefault[T ~int | ~int64](value T, fallback T) T {
	if value == 0 {
		return fallback
	}

	return value
}
