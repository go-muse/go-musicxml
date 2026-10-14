package validation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestDictionaryLengthDelimitedEncodingMatchesIndependentVector(t *testing.T) {
	a := dictionaryCoreAccount(t)
	h := sha256.New()
	w := dictionaryEncoder{ctx: context.Background(), account: a, hash: h}
	w.write([]byte("go-musicxml.dictionary\x00"))
	w.count(1)
	for _, s := range []string{"vector", "test", "encoding", "test-v1", "test-v1"} {
		w.text(s)
	}
	w.count(4)
	w.row("item", "z", "")
	w.row("item", "é", "a\x00b")
	w.row("pair", "a", "bc")
	w.row("pair", "ab", "c")
	if w.err != nil {
		t.Fatal(w.err)
	}
	if w.bytes != 175 || hex.EncodeToString(h.Sum(nil)) != "6c3d536ec738c361d6178c38dedcf335fe3a9bcd90a8c49b0d91ac42eb0d340c" {
		t.Fatalf("independent codec mismatch: %d %x", w.bytes, h.Sum(nil))
	}
	for _, tc := range []struct {
		cp   uint32
		want string
	}{{0, "U+0000"}, {0xe050, "U+E050"}, {0x10ffff, "U+10FFFF"}} {
		if got := dictionaryCodepointString(tc.cp); got != tc.want {
			t.Fatalf("codepoint %x = %s", tc.cp, got)
		}
	}
}
