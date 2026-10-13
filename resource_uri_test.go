package musicxml

import (
	"context"
	"errors"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestResourceURIReferenceContexts(t *testing.T) {
	tests := []struct {
		name                      string
		base                      resourceURIBase
		href, uri, path, fragment string
		present, same             bool
	}{
		{name: "external relative", base: resourceURIBase{RetrievalURI: "https://example.test/sets/catalog.opus?edition=1"}, href: "movement.musicxml", uri: "https://example.test/sets/movement.musicxml"},
		{name: "external empty", base: resourceURIBase{RetrievalURI: "https://example.test/sets/catalog.opus?edition=1"}, uri: "https://example.test/sets/catalog.opus?edition=1", same: true},
		{name: "empty fragment", base: resourceURIBase{RetrievalURI: "https://example.test/sets/catalog.opus?edition=1"}, href: "#", uri: "https://example.test/sets/catalog.opus?edition=1", present: true, same: true},
		{name: "fragment", base: resourceURIBase{RetrievalURI: "https://example.test/sets/catalog.opus?edition=1"}, href: "#movement", uri: "https://example.test/sets/catalog.opus?edition=1", fragment: "movement", present: true, same: true},
		{name: "empty query preserved", base: resourceURIBase{RetrievalURI: "https://example.test/sets/catalog.opus?edition=1"}, href: "?#", uri: "https://example.test/sets/catalog.opus?", present: true},
		{name: "query changed", base: resourceURIBase{RetrievalURI: "https://example.test/sets/catalog.opus?edition=1"}, href: "?edition=2#movement", uri: "https://example.test/sets/catalog.opus?edition=2", fragment: "movement", present: true},
		{name: "detached current entity", href: "#here", fragment: "here", present: true, same: true},
		{name: "archive opus base", base: resourceURIBase{Archive: true, Entry: "sets/catalog.opus"}, href: "parts/one.musicxml", path: "sets/parts/one.musicxml"},
		{name: "archive score link base", base: resourceURIBase{Archive: true, ArchiveRoot: true, Entry: "sets/score.musicxml"}, href: "parts/one.musicxml", path: "parts/one.musicxml"},
		{name: "archive fragment current entity", base: resourceURIBase{Archive: true, ArchiveRoot: true, Entry: "sets/score.musicxml"}, href: "#mark", path: "sets/score.musicxml", fragment: "mark", present: true, same: true},
		{name: "literal base percent", base: resourceURIBase{Archive: true, Entry: "sets%20/catalog.opus"}, href: "one%20two.musicxml", path: "sets%20/one two.musicxml"},
		{name: "decode once", base: resourceURIBase{Archive: true, ArchiveRoot: true, Entry: "score.musicxml"}, href: "part%252Fone.musicxml", path: "part%2Fone.musicxml"},
		{name: "empty URI segments", base: resourceURIBase{Archive: true, ArchiveRoot: true, Entry: "score.musicxml"}, href: "/scores//../first.musicxml", path: "scores/first.musicxml"},
		{name: "custom absolute resolver URI", href: "urn:example:score", uri: "urn:example:score"},
		{name: "opaque current entity", base: resourceURIBase{RetrievalURI: "urn:example:score"}, href: "#id", uri: "urn:example:score", fragment: "id", present: true, same: true},
		{name: "archive transport URI is not contained XML", base: resourceURIBase{Archive: true, Entry: "main.opus", RetrievalURI: "https://e.test/inner.mxl"}, href: "https://e.test/inner.mxl#id", uri: "https://e.test/inner.mxl", fragment: "id", present: true},
		{name: "archive network-path URI is not contained XML", base: resourceURIBase{Archive: true, Entry: "main.opus", RetrievalURI: "https://e.test/inner.mxl"}, href: "//e.test/inner.mxl", uri: "https://e.test/inner.mxl"},
		{name: "plain absolute same entity", base: resourceURIBase{RetrievalURI: "https://e.test/score.musicxml"}, href: "https://e.test/score.musicxml", uri: "https://e.test/score.musicxml", same: true},
		{name: "absolute external in archive", base: resourceURIBase{Archive: true, Entry: "score.musicxml"}, href: "https://elsewhere.test/s.musicxml#id", uri: "https://elsewhere.test/s.musicxml", fragment: "id", present: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, _ := validation.NewBudgetAccount(validation.Budgets{})
			got, err := resolveAssessmentResourceURI(context.Background(), a, tt.base, tt.href)
			if err != nil {
				t.Fatal(err)
			}
			if got.RetrievalURI != tt.uri || got.ArchivePath != tt.path || got.Fragment != tt.fragment || got.FragmentPresent != tt.present || got.SameEntity != tt.same {
				t.Fatalf("got %#v", got)
			}
			live, _ := a.Usage(validation.BudgetBufferedBytes)
			if live != 0 {
				t.Fatalf("retained URI scratch %d", live)
			}
		})
	}
}

func TestResourceURIBoundedUnsupported(t *testing.T) {
	for _, tt := range []struct {
		base resourceURIBase
		href string
	}{
		{resourceURIBase{}, "relative.musicxml"},
		{resourceURIBase{Archive: true, Entry: "a.musicxml"}, "../outside.musicxml"},
		{resourceURIBase{Archive: true, Entry: "a.musicxml"}, "?query"},
		{resourceURIBase{Archive: true, Entry: "a.musicxml"}, "folder/"},
		{resourceURIBase{Archive: true, Entry: "a.musicxml"}, "META-INF/container.xml"},
		{resourceURIBase{RetrievalURI: "relative-base"}, "b"},
		{resourceURIBase{}, "bad%GG"},
	} {
		a, _ := validation.NewBudgetAccount(validation.Budgets{})
		_, err := resolveAssessmentResourceURI(context.Background(), a, tt.base, tt.href)
		if !errors.Is(err, validation.ErrIncompleteAssessment) {
			t.Fatalf("%#v %q: %v", tt.base, tt.href, err)
		}
		if errors.Is(err, ErrInvalidDocument) {
			t.Fatalf("URI processing became normative: %v", err)
		}
		live, _ := a.Usage(validation.BudgetBufferedBytes)
		if live != 0 {
			t.Fatalf("scratch leak %d", live)
		}
	}
}

func TestResourceURIBudgetAndCancellation(t *testing.T) {
	base := resourceURIBase{RetrievalURI: "https://example.test/sets/catalog.opus"}
	href := "movement.musicxml#id"
	for _, b := range []validation.Budgets{{MaxBufferedBytes: 1}, {MaxRuleSteps: 1}, {MaxScalarBytes: 1}} {
		a, _ := validation.NewBudgetAccount(b)
		_, err := resolveAssessmentResourceURI(context.Background(), a, base, href)
		if !errors.Is(err, validation.ErrResourceLimit) {
			t.Fatalf("%+v: %v", b, err)
		}
		live, _ := a.Usage(validation.BudgetBufferedBytes)
		if live != 0 {
			t.Fatalf("scratch leak %d", live)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	_, err := resolveAssessmentResourceURI(ctx, a, base, href)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := resolveAssessmentResourceURI(context.Background(), nil, base, href); !errors.Is(err, validation.ErrInvalidConfiguration) {
		t.Fatal(err)
	}
}
