package musicxml

import (
	"context"
	"errors"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

type dictionaryNilMapResolver map[string]string

func (dictionaryNilMapResolver) Resolve(context.Context, validation.ResourceRequest) (validation.ResourceResult, error) {
	panic("nil map resolver invoked")
}

type dictionaryNilSliceResolver []string

func (dictionaryNilSliceResolver) Resolve(context.Context, validation.ResourceRequest) (validation.ResourceResult, error) {
	panic("nil slice resolver invoked")
}

type dictionaryNilChanResolver chan string

func (dictionaryNilChanResolver) Resolve(context.Context, validation.ResourceRequest) (validation.ResourceResult, error) {
	panic("nil channel resolver invoked")
}

func TestDictionarySharedNilCheckPreservesResourceProfiles(t *testing.T) {
	for _, profile := range []validation.ProfileID{ProfileResourceGraph, ProfileResourceGraphDictionaries} {
		for _, resolver := range []validation.ResourceResolver{nil, (*optionNilResolver)(nil), graphResolverFunc(nil), dictionaryNilMapResolver(nil), dictionaryNilSliceResolver(nil), dictionaryNilChanResolver(nil)} {
			cfg, err := validateLinkedAssessmentOptions(context.Background(), LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: profile}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}}, profile)
			if err != nil || cfg.resolver != nil || !cfg.selected[ResourceDocuments] {
				t.Fatalf("profile=%s resolver=%T cfg=%+v err=%v", profile, resolver, cfg, err)
			}
		}
		nonNil := &optionNilResolver{}
		cfg, err := validateLinkedAssessmentOptions(context.Background(), LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: profile}, Resolver: nonNil}, profile)
		if err != nil || cfg.resolver != nonNil {
			t.Fatalf("profile=%s resolver=%v err=%v", profile, cfg.resolver, err)
		}
	}
}

func TestDictionarySingleValidationStillRejectsBeforeProviderAndInput(t *testing.T) {
	calls := 0
	dictionaries := DictionaryOptions{Selections: []validation.DictionarySelection{{Manifest: "unknown", Relations: []validation.DictionaryRelation{validation.DictionaryRelationCanonicalName}}}, Source: dictionaryImportSourceFunc(func(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error) {
		calls++
		return DictionaryComponentResult{}, errors.New("unexpected provider call")
	})}
	for _, run := range []func() error{
		func() error {
			_, err := inspectXMLWithDictionaries(context.Background(), &dictionaryUnexpectedRead{t}, dictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: ProfileSourceDictionaries}, Dictionaries: dictionaries})
			return err
		},
		func() error {
			_, err := assessModelWithDictionaries(context.Background(), &OpusDocument{}, dictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: ProfileModelDictionaries}, Dictionaries: dictionaries})
			return err
		},
		func() error {
			_, err := decodeAssessedWithDictionaries(context.Background(), &dictionaryUnexpectedRead{t}, decodeDictionaryTestOptions{Assessment: &dictionaryAssessmentOptions{Assessment: AssessmentOptions{Profile: ProfileSourceSemantics}, Dictionaries: dictionaries}})
			return err
		},
		func() error {
			_, err := inspectXMLWithResourcesAndDictionaries(context.Background(), &dictionaryUnexpectedRead{t}, linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraphDictionaries}}, Dictionaries: dictionaries})
			return err
		},
		func() error {
			_, err := inspectMXLWithDictionaries(context.Background(), &dictionaryUnexpectedRead{t}, mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileMXLSourceDictionaries}}, Dictionaries: dictionaries})
			return err
		},
	} {
		if err := run(); !errors.Is(err, validation.ErrInvalidConfiguration) || errors.Is(err, ErrDictionaryUnavailable) || calls != 0 {
			t.Fatalf("err=%v calls=%d", err, calls)
		}
	}
}

func TestDictionaryBodyRejectsOvercountOfLimitedSlice(t *testing.T) {
	cause := errors.New("underlying read failure")
	body := &resourceTestBody{Reader: dictionaryInvalidCountReader{n: 4, err: cause}}
	a := dictionaryImportAccount(t, validation.Budgets{})
	// The shared reader's buffer is 4096 bytes, but the transferred body receives
	// only the expected two bytes plus its one-byte overflow probe.
	out, err := readDictionaryComponentBody(context.Background(), a, body, 2)
	if out != nil || !errors.Is(err, cause) || errors.Is(err, validation.ErrInvalidConfiguration) || body.closed != 1 {
		t.Fatalf("out=%v err=%v closes=%d", out, err, body.closed)
	}
	if external, _ := a.Usage(validation.BudgetExternalBytes); external != 0 {
		t.Fatalf("invalid read count admitted %d bytes", external)
	}
	dictionaryImportNoLive(t, a)
}

func TestDictionaryValidatedImportSnapshotsSelectionsBeforeCallback(t *testing.T) {
	ctx := context.Background()
	first, second := validation.DictionaryManifestStandardSounds40, validation.DictionaryManifestSMuFL14Canonical
	selections := []validation.DictionarySelection{dictionaryImportSelection(first), dictionaryImportSelection(second)}
	a := dictionaryImportAccount(t, validation.Budgets{})
	var calls []DictionaryComponentRequest
	fixture := dictionaryImportFixtureSource(t, &calls)
	opts := DictionaryOptions{Selections: selections, Source: dictionaryImportSourceFunc(func(ctx context.Context, r DictionaryComponentRequest) (DictionaryComponentResult, error) {
		if len(calls) == 0 {
			// Even callback-time mutation cannot change requests or relations
			// after preflight accepted and the root copied the full selection.
			selections[0].Relations[0] = "unknown-first-relation"
			selections[1].Manifest = "unknown-second-manifest"
			selections[1].Relations[0] = "unknown-second-relation"
		}
		return fixture.Open(ctx, r)
	})}
	if err := validateDictionaryOptions(opts); err != nil {
		t.Fatal(err)
	}
	set, err := loadValidatedDictionarySet(ctx, a, opts)
	if err != nil || set == nil || len(calls) != 2 || calls[0].Manifest != first || calls[1].Manifest != second {
		t.Fatalf("set=%v calls=%v err=%v", set, calls, err)
	}
	actual, err := set.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	set.Close()
	dictionaryImportNoLive(t, a)
	calls = nil
	want, err := loadDictionarySet(ctx, a, DictionaryOptions{Selections: []validation.DictionarySelection{dictionaryImportSelection(first), dictionaryImportSelection(second)}, Source: dictionaryImportFixtureSource(t, &calls)})
	if err != nil {
		t.Fatal(err)
	}
	expected, err := want.Fingerprint()
	if err != nil || actual != expected {
		t.Fatalf("snapshot changed fingerprint: got=%x want=%x err=%v", actual, expected, err)
	}
	want.Close()
	dictionaryImportNoLive(t, a)
}
