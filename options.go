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
}

// DefaultDecodeOptions returns the limits used by Decode and the typed decode
// helpers.
func DefaultDecodeOptions() DecodeOptions {
	return DecodeOptions{MaxXMLDepth: defaultMaxXMLDepth}
}

// MXLOptions controls resource limits for compressed MusicXML decoding.
//
// Zero values select the package defaults. Negative values and XML depths
// above the documented maximum are invalid.
type MXLOptions struct {
	MaxArchiveBytes   int64
	MaxMetadataBytes  int64
	MaxDocumentBytes  int64
	MaxResourceBytes  int64
	MaxResourcesBytes int64
	// MaxXMLDepth has the same zero default and 4096 maximum as DecodeOptions.
	MaxXMLDepth int
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
