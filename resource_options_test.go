package musicxml

import (
	"context"
	"errors"
	"github.com/go-muse/go-musicxml/validation"
	"testing"
)

type optionNilResolver struct{}

func (*optionNilResolver) Resolve(context.Context, validation.ResourceRequest) (validation.ResourceResult, error) {
	panic("configuration invoked resolver")
}
func TestResourceOptionsRejectBeforeAnyIO(t *testing.T) {
	profile := validation.ProfileID("test-resource")
	for _, tc := range []LinkedAssessmentOptions{
		{}, {Graph: AssessmentOptions{Profile: "wrong"}},
		{Graph: AssessmentOptions{Profile: profile}, Dependencies: []ResourceDependency{"bogus"}},
		{Graph: AssessmentOptions{Profile: profile}, Dependencies: []ResourceDependency{ResourceDocuments, ResourceDocuments}},
		{Graph: AssessmentOptions{Profile: profile}, Identity: ResourceIdentity{RetrievalURI: "relative/base"}},
		{Graph: AssessmentOptions{Profile: profile}, Identity: ResourceIdentity{CanonicalID: "a\x00b"}},
		{Graph: AssessmentOptions{Profile: profile, Budgets: validation.Budgets{MaxRuleSteps: -1}}},
	} {
		if _, err := validateLinkedAssessmentOptions(context.Background(), tc, profile); !errors.Is(err, validation.ErrInvalidConfiguration) {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
}
func TestResourceOptionsEmptyAndTypedNilResolver(t *testing.T) {
	profile := validation.ProfileID("test-resource")
	for _, resolver := range []validation.ResourceResolver{nil, (*optionNilResolver)(nil)} {
		cfg, err := validateLinkedAssessmentOptions(context.Background(), LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: profile}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}, Identity: ResourceIdentity{RetrievalURI: "urn:example:score"}}, profile)
		if err != nil || cfg.resolver != nil || !cfg.selected[ResourceDocuments] {
			t.Fatal(cfg, err)
		}
	}
	cfg, err := validateLinkedAssessmentOptions(context.Background(), LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: profile}, Resolver: &optionNilResolver{}}, profile)
	if err != nil || len(cfg.selected) != 0 {
		t.Fatal(cfg, err)
	}
	first, err := newResourceRunIdentity()
	if err != nil {
		t.Fatal(err)
	}
	second, err := newResourceRunIdentity()
	if err != nil || first == second || first == "" {
		t.Fatal(first, second, err)
	}
}
