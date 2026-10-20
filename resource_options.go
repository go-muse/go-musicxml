package musicxml

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/go-muse/go-musicxml/validation"
	"net/url"
	"strings"
	"unicode/utf8"
)

// ResourceDependency selects one closed class of caller-resolver requests.
// Selection grants no built-in network or filesystem transport. The supplied
// resolver owns transport, custom-scheme and redirect authorization.
type ResourceDependency string

// Explicit external dependency classes; each selection is independent.
const (
	ResourceDocuments    ResourceDependency = "documents"
	ResourceImages       ResourceDependency = "image-metadata"
	ResourceAssociations ResourceDependency = "instrument-associations"
	ResourceRoles        ResourceDependency = "package-roles"
)

// ResourceIdentity supplies primary canonical naming and its retrieval base.
// Empty canonical identity becomes run-local. RetrievalURI, when present, is an
// absolute URI; it is independent of canonical identity and grants no I/O.
// Source digests are measured, and current snapshots receive fresh opaque IDs.
type ResourceIdentity struct{ CanonicalID, Revision, RetrievalURI string }

// linkedAssessmentOptions is the normalized form of LinkedAssessmentOptions.
type linkedAssessmentOptions struct {
	Graph        assessmentOptions
	Identity     ResourceIdentity
	Resolver     validation.ResourceResolver
	Dependencies []ResourceDependency
}

// mxlAssessmentOptions is the normalized form of MXLAssessmentOptions.
type mxlAssessmentOptions struct {
	Graph        assessmentOptions
	Identity     ResourceIdentity
	Resolver     validation.ResourceResolver
	Dependencies []ResourceDependency
	MXL          MXLOptions
}

func (o mxlAssessmentOptions) linked() linkedAssessmentOptions {
	linked := linkedAssessmentOptions{Graph: o.Graph, Identity: o.Identity, Resolver: o.Resolver, Dependencies: o.Dependencies}
	if linked.Graph.DTD == nil {
		linked.Graph.DTD = o.MXL.DTD
	}
	return linked
}

type resourceRunOptions struct {
	account  *validation.Account
	identity ResourceIdentity
	resolver validation.ResourceResolver
	selected map[ResourceDependency]bool
	dtd      *DTDOptions
}

func validateLinkedAssessmentOptions(ctx context.Context, o linkedAssessmentOptions, expected validation.ProfileID) (resourceRunOptions, error) {
	return validateLinkedAssessmentOptionsAccount(ctx, o, expected, nil)
}

// validateLinkedAssessmentOptionsAccount uses account when it is non-nil, so
// that several graph runs share one operation's budgets.
func validateLinkedAssessmentOptionsAccount(ctx context.Context, o linkedAssessmentOptions, expected validation.ProfileID, account *validation.Account) (resourceRunOptions, error) {
	var out resourceRunOptions
	invalid := func(field, reason string) (resourceRunOptions, error) {
		return out, &validation.ConfigurationError{Field: field, Reason: reason}
	}
	if err := assessmentContext(ctx, "canceled-before-resource"); err != nil {
		return out, err
	}
	if expected == "" || o.Graph.Profile != expected {
		return invalid("graph profile", "explicit entry-point compatible graph profile required")
	}
	if len(o.Dependencies) > 4 {
		return invalid("resource dependencies", "bounded closed selection required")
	}
	var selected [4]bool
	for _, kind := range o.Dependencies {
		index := -1
		switch kind {
		case ResourceDocuments:
			index = 0
		case ResourceImages:
			index = 1
		case ResourceAssociations:
			index = 2
		case ResourceRoles:
			index = 3
		}
		if index < 0 {
			return invalid("resource dependencies", "unknown dependency kind")
		}
		if selected[index] {
			return invalid("resource dependencies", "duplicate dependency kind")
		}
		selected[index] = true
	}
	for _, v := range []string{o.Identity.CanonicalID, o.Identity.Revision, o.Identity.RetrievalURI} {
		if len(v) > 4096 || !utf8.ValidString(v) || strings.ContainsRune(v, 0) {
			return invalid("resource identity", "invalid or excessive identity text")
		}
	}
	if strings.ContainsAny(o.Identity.CanonicalID, "\r\n\t") || len(o.Identity.Revision) > 1024 {
		return invalid("resource identity", "canonical identity and revision must satisfy the pinned core identity contract")
	}
	if o.Identity.RetrievalURI != "" {
		u, err := url.Parse(o.Identity.RetrievalURI)
		if err != nil || !u.IsAbs() {
			return invalid("retrieval URI", "absolute usable URI required")
		}
	}
	if account == nil {
		var err error
		if account, err = validation.NewBudgetAccount(o.Graph.Budgets); err != nil {
			return out, err
		}
	}
	out.account, out.identity, out.resolver, out.dtd = account, o.Identity, o.Resolver, o.Graph.DTD
	if nilValue(out.resolver) {
		out.resolver = nil
	} else {
		out.resolver = absentTypedNilBodies{out.resolver}
	}
	out.selected = make(map[ResourceDependency]bool, len(o.Dependencies))
	for _, kind := range o.Dependencies {
		out.selected[kind] = true
	}
	return out, nil
}
func newResourceRunIdentity() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return "resource-run-" + hex.EncodeToString(value[:]), nil
}

// absentTypedNilBodies treats a typed-nil ResourceResult.Body, such as a nil
// *os.File, as no body, so that no method is called on it.
type absentTypedNilBodies struct{ resolver validation.ResourceResolver }

func (r absentTypedNilBodies) Resolve(ctx context.Context, request validation.ResourceRequest) (validation.ResourceResult, error) {
	result, err := r.resolver.Resolve(ctx, request)
	if result.Body != nil && nilValue(result.Body) {
		result.Body = nil
	}
	return result, err
}
