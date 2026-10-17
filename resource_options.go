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

// LinkedAssessmentOptions selects an explicit graph profile and one shared set
// of finite limits for primary input, children, metadata and graph operations.
// Empty Dependencies invokes no resolver. Nil (including typed-nil) Resolver
// leaves selected external dependencies unavailable. No schema hint is fetched.
// The caller must not mutate options or input models during assessment.
type LinkedAssessmentOptions struct {
	Graph        AssessmentOptions
	Identity     ResourceIdentity
	Resolver     validation.ResourceResolver
	Dependencies []ResourceDependency
}

// MXLAssessmentOptions adds the existing independent archive processing caps.
// MXL does not create another account; Graph.Budgets bounds the complete run.
// MXL.MaxXMLDepth independently caps each represented XML document in an MXL
// context (zero uses 256), without restricting graph hops between shallow XML.
// Original raw ZIP evidence is available only to InspectMXL, never reconstructed
// from the current object passed to AssessMXLPackage.
type MXLAssessmentOptions struct {
	Graph        AssessmentOptions
	Identity     ResourceIdentity
	Resolver     validation.ResourceResolver
	Dependencies []ResourceDependency
	MXL          MXLOptions
}

func (o MXLAssessmentOptions) linked() LinkedAssessmentOptions {
	linked := LinkedAssessmentOptions{Graph: o.Graph, Identity: o.Identity, Resolver: o.Resolver, Dependencies: o.Dependencies}
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

func validateLinkedAssessmentOptions(ctx context.Context, o LinkedAssessmentOptions, expected validation.ProfileID) (resourceRunOptions, error) {
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
	account, err := validation.NewBudgetAccount(o.Graph.Budgets)
	if err != nil {
		return out, err
	}
	out.account, out.identity, out.resolver, out.dtd = account, o.Identity, o.Resolver, o.Graph.DTD
	if dictionaryNil(out.resolver) {
		out.resolver = nil
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
