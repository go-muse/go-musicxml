package musicxml

import (
	"context"
	"io"

	"github.com/go-muse/go-musicxml/validation"
)

// DictionaryOptions selects fixed dictionary components supplied by the caller.
// Standalone Inspect/Assess calls with empty Selections invoke no source.
// Assessed loaders use bundled required data when Source is nil and Selections
// is empty; explicit partial selections are rejected before I/O.
// Otherwise nil (including typed-nil) Source leaves
// selected capabilities unavailable. A component is requested at most once per
// root assessment. Selection never authorizes network, file or DTD access.
// The caller must not mutate options or source bytes during assessment.
type DictionaryOptions struct {
	Selections []validation.DictionarySelection
	Source     DictionarySource
}

// dictionaryAssessmentOptions pairs standalone assessment options with the
// dictionary capabilities that AssessmentOptions.Dictionaries selected.
type dictionaryAssessmentOptions struct {
	Assessment   AssessmentOptions
	Dictionaries DictionaryOptions
}

// linkedDictionaryAssessmentOptions shares one dictionary set and resource
// account across the explicitly selected linked assessment graph.
type linkedDictionaryAssessmentOptions struct {
	Resources    LinkedAssessmentOptions
	Dictionaries DictionaryOptions
}

// mxlDictionaryAssessmentOptions adds pinned capabilities to an MXL assessment.
// Existing independent archive limits continue to apply.
type mxlDictionaryAssessmentOptions struct {
	Resources    MXLAssessmentOptions
	Dictionaries DictionaryOptions
}

// DictionarySource supplies caller-authorized transport for exact pinned bytes.
// Open must honor its own cancellation of blocked I/O. Every usable nonnil Body
// transfers ownership, even alongside an error; the assessment closes it once.
// Metadata URLs do not confer transport or redirect authorization.
//
// DictionarySource is implemented by callers, so its method set is frozen.
// New capabilities will be offered as separate optional interfaces that the
// importer discovers with type assertions.
type DictionarySource interface {
	Open(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error)
}

// DictionarySourceFunc adapts a function to DictionarySource. A nil
// DictionarySourceFunc is a typed-nil source: the importer supplies nothing
// from it, as for a nil Source, so its selections stay unavailable. Like any
// typed-nil source it is an explicit override, not a request for the bundled
// defaults.
type DictionarySourceFunc func(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error)

// Open calls f(ctx, request).
func (f DictionarySourceFunc) Open(ctx context.Context, request DictionaryComponentRequest) (DictionaryComponentResult, error) {
	return f(ctx, request)
}

// DictionaryComponentRequest identifies one compiled manifest and exact raw pin.
// No arbitrary URI, latest-version lookup, or implicit dependency is requested.
type DictionaryComponentRequest struct {
	Manifest  validation.DictionaryManifestID
	Component validation.DictionaryComponentPin
}

// DictionaryComponentResult must echo Request exactly. Known requires a usable
// Body and no Reason; Unknown or Unsupported requires a bounded nonempty Reason
// and no Body. Other states are invalid. Byte verification precedes parsing.
type DictionaryComponentResult struct {
	Request DictionaryComponentRequest
	State   validation.EvidenceState
	Reason  validation.Code
	Body    io.ReadCloser
}

func validateDictionaryOptions(o DictionaryOptions) error {
	bad := func(reason string) error {
		return &validation.ConfigurationError{Field: "dictionary selections", Reason: reason}
	}
	if len(o.Selections) > 5 {
		return bad("at most five fixed components may be selected")
	}
	for i, s := range o.Selections {
		m, ok := validation.LookupDictionaryManifest(s.Manifest)
		if !ok {
			return bad("unsupported manifest")
		}
		if len(s.Relations) == 0 || len(s.Relations) > len(m.Relations) {
			return bad("nonempty bounded relation selection required")
		}
		for j := 0; j < i; j++ {
			if o.Selections[j].Manifest == s.Manifest {
				return bad("duplicate manifest selection")
			}
		}
		for j, r := range s.Relations {
			known := false
			for _, allowed := range m.Relations {
				if r == allowed {
					known = true
					break
				}
			}
			if !known {
				return bad("unsupported manifest relation")
			}
			for k := 0; k < j; k++ {
				if s.Relations[k] == r {
					return bad("duplicate relation selection")
				}
			}
		}
	}
	return nil
}
