package musicxml

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestDictionaryJSONFailedEscapeDoesNotWriteScratch(t *testing.T) {
	for _, tc := range []struct {
		name, raw, reason string
		maximum           int
	}{
		{"truncated hex", `"A\u12`, "truncated JSON escape", 40},
		{"invalid hex", `"A\u12xz"`, "invalid JSON escape", 40},
		{"missing low surrogate", `"A\uD800"`, "unpaired JSON surrogate", 40},
		{"invalid low surrogate", `"A\uD800\u0061"`, "unpaired JSON surrogate", 40},
		{"invalid low hex", `"A\uD800\uDCxz"`, "invalid JSON escape", 40},
		{"low surrogate alone", `"A\uDC00"`, "unpaired JSON surrogate", 40},
		{"escaped scalar exceeds maximum", `"A\u00E9"`, "excessive corpus string", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := dictionaryJSON{ctx: context.Background(), raw: []byte(tc.raw)}
			for i := range c.scratch {
				c.scratch[i] = 0xA5
			}
			got := c.str(tc.maximum)
			if got != nil || c.err == nil || !strings.Contains(c.err.Error(), tc.reason) {
				t.Fatalf("got=%q err=%v", got, c.err)
			}
			if c.scratch[0] != 'A' || !bytes.Equal(c.scratch[1:], bytes.Repeat([]byte{0xA5}, len(c.scratch)-1)) {
				t.Fatalf("failed escape changed scratch: % x", c.scratch[:8])
			}
		})
	}
}

func TestDictionaryDecoderFailedEscapeReleasesActualLease(t *testing.T) {
	id := validation.DictionaryManifestSMuFL14Canonical
	raw := dictionaryImportFixture(t, id)
	// Match the private decoder's exact-length contract while forcing its string
	// scanner down a malformed escape path after the retained lease is acquired.
	prefix := []byte(`{"A\u12xz":{"codepoint":"U+E050","description":"x"}}`)
	copy(raw, prefix)
	for i := len(prefix); i < len(raw); i++ {
		raw[i] = ' '
	}
	a := dictionaryImportAccount(t, validation.Budgets{})
	if err := a.Charge(validation.BudgetBufferedBytes, 17); err != nil {
		t.Fatal(err)
	}
	out, err := decodeDictionaryComponent(context.Background(), a, id, raw)
	if out != nil || err == nil || !strings.Contains(err.Error(), "invalid JSON escape") {
		t.Fatalf("out=%v err=%v", out, err)
	}
	if live, _ := a.Usage(validation.BudgetBufferedBytes); live != 17 {
		t.Fatalf("lost caller lease or leaked decoder lease: %d", live)
	}
	if work, _ := a.Usage(validation.BudgetRuleSteps); work != 6*int64(len(raw))+1025 {
		t.Fatalf("failure never reached the admitted decoder: work=%d", work)
	}
	if err := a.Release(validation.BudgetBufferedBytes, 17); err != nil {
		t.Fatal(err)
	}
	dictionaryImportNoLive(t, a)
}
