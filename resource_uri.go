package musicxml

import (
	"context"
	"math"
	"net/url"
	"strings"

	"github.com/go-muse/go-musicxml/validation"
)

// resourceURIBase keeps retrieval context separate from canonical cache identity.
// ArchiveRoot is selected only by the exact score link/part-link source family;
// the existing Opus helper retains containing-document-relative behavior.
type resourceURIBase struct {
	RetrievalURI string
	Archive      bool
	Entry        string
	ArchiveRoot  bool
}

type resolvedResourceURI struct {
	RetrievalURI    string
	ArchivePath     string
	Fragment        string
	FragmentPresent bool
	SameEntity      bool
}

type resourceURIUnavailable struct{ reason validation.Code }

func (e *resourceURIUnavailable) Error() string {
	return "musicxml: resource URI unavailable: " + string(e.reason)
}
func (e *resourceURIUnavailable) Unwrap() error { return validation.ErrIncompleteAssessment }

// resolveAssessmentResourceURI performs no retrieval. URI processing policy is
// independent of the XSD anyURI domain and never yields a normative violation.
// Returned strings are caller-owned values. This function admits its parsing
// scratch; orchestration must separately admit any returned data it retains.
func resolveAssessmentResourceURI(ctx context.Context, account *validation.Account, base resourceURIBase, href string) (resolvedResourceURI, error) {
	var out resolvedResourceURI
	if ctx == nil || account == nil {
		return out, &validation.ConfigurationError{Field: "resource URI", Reason: "context and root account required"}
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	var n int64
	for _, value := range []string{href, base.RetrievalURI, base.Entry} {
		if err := account.Check(validation.BudgetScalarBytes, int64(len(value))); err != nil {
			return out, err
		}
		if int64(len(value)) > math.MaxInt64-n {
			return out, &validation.ConfigurationError{Field: "resource URI", Reason: "size overflow"}
		}
		n += int64(len(value))
	}
	if n > (math.MaxInt64-1024)/16 {
		return out, &validation.ConfigurationError{Field: "resource URI", Reason: "scratch size overflow"}
	}
	if err := account.Charge(validation.BudgetRuleSteps, n+1); err != nil {
		return out, err
	}
	retained := 16*n + 1024
	if _, err := validation.AllocationSize(retained); err != nil {
		return out, err
	}
	if err := account.Charge(validation.BudgetBufferedBytes, retained); err != nil {
		return out, err
	}
	defer account.Release(validation.BudgetBufferedBytes, retained)
	unavailable := func(reason validation.Code) (resolvedResourceURI, error) {
		return resolvedResourceURI{}, &resourceURIUnavailable{reason: reason}
	}
	// xs:anyURI whitespace collapse precedes component parsing. Escaped spaces
	// remain significant, and URI paths are never filesystem-cleaned here.
	normalized := collapseValidationWhitespace(href)
	ref, err := url.Parse(normalized)
	if err != nil {
		return unavailable("uri-processing-syntax")
	}
	out.FragmentPresent = strings.Contains(normalized, "#")
	out.Fragment = ref.Fragment
	ref.Fragment, ref.RawFragment = "", ""
	if err := ctx.Err(); err != nil {
		return resolvedResourceURI{}, err
	}
	// Empty and fragment-only references retain the current entity (RFC3986
	// section4.4); the fragment's interpretation remains a separate graph fact.
	current := ref.Scheme == "" && ref.Host == "" && ref.User == nil && ref.Opaque == "" && ref.Path == "" && ref.RawQuery == "" && !ref.ForceQuery
	if base.Archive && current {
		if !validMXLContentPath(base.Entry) {
			return unavailable("archive-source-path")
		}
		out.ArchivePath, out.SameEntity = base.Entry, true
		return out, nil
	}
	if base.Archive && ref.Scheme == "" && ref.Host == "" && ref.User == nil && ref.Opaque == "" {
		source := base.Entry
		if base.ArchiveRoot {
			source = ""
		}
		target, _, err := resolveMXLLinkPath(source, normalized)
		if err != nil {
			return unavailable("archive-uri-policy")
		}
		out.ArchivePath, out.SameEntity = target, target == base.Entry
		if err := ctx.Err(); err != nil {
			return resolvedResourceURI{}, err
		}
		return out, nil
	}
	if ref.IsAbs() {
		out.RetrievalURI = ref.String()
		if !base.Archive && base.RetrievalURI != "" {
			b, e := url.Parse(base.RetrievalURI)
			if e == nil {
				b.Fragment, b.RawFragment = "", ""
				out.SameEntity = out.RetrievalURI == b.String()
			}
		}
		if err := ctx.Err(); err != nil {
			return resolvedResourceURI{}, err
		}
		return out, nil
	}
	if base.RetrievalURI == "" {
		if current {
			out.SameEntity = true
			return out, nil
		}
		return unavailable("retrieval-base-unavailable")
	}
	b, err := url.Parse(base.RetrievalURI)
	if err != nil || !b.IsAbs() {
		return unavailable("retrieval-base-unavailable")
	}
	b.Fragment, b.RawFragment = "", ""
	if b.Opaque != "" && !current {
		return unavailable("opaque-relative-resolution")
	}
	resolved := b.ResolveReference(ref)
	out.RetrievalURI = resolved.String()
	out.SameEntity = !base.Archive && (current || out.RetrievalURI == b.String())
	if err := ctx.Err(); err != nil {
		return resolvedResourceURI{}, err
	}
	return out, nil
}
