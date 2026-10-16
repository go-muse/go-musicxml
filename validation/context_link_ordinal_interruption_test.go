package validation

import (
	"context"
	"errors"
	"testing"
)

func TestResourceAttachmentOrdinalInterruptedSearch(t *testing.T) {
	setup := func(t *testing.T) (*contextReader, *resourceLinkIndex, *factNode) {
		root := resourceTestNode(1, 0, "measure")
		earlier := resourceTestNode(2, 1, "direction")
		old := resourceTestNode(3, 2, "words")
		link := resourceTestNode(4, 1, "link")
		anchor := resourceTestNode(5, 1, "direction")
		word := resourceTestNode(6, 5, "words")
		r, index := resourceTestReader(t, root, earlier, old, link, anchor, word)
		return r, index, link
	}
	r, index, link := setup(t)
	counter := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	r.ctx = counter
	got := index.attachment(r, link, resourceTestText("words"), resourceTestPosition("2"))
	if r.err != nil || got.State != EvidenceAbsent {
		t.Fatalf("baseline: %+v %v", got, r.err)
	}
	for at := 1; at <= counter.calls; at++ {
		t.Run(string(rune('A'+at)), func(t *testing.T) {
			r, index, link := setup(t)
			r.ctx = &rationalCancelContext{Context: context.Background(), at: at}
			got := index.attachment(r, link, resourceTestText("words"), resourceTestPosition("2"))
			if !errors.Is(r.err, context.Canceled) || got.State != EvidenceUnknown {
				t.Fatalf("poll %d: fabricated selection %+v, error %v", at, got, r.err)
			}
		})
	}
}
