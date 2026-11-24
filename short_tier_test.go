package musicxml

import "testing"

// skipInShortTier keeps a test that takes a second or more out of the short
// tier, `go test -short`, which is meant for the edit loop. The full tier,
// plain `go test`, runs every test: scripts/check-all.sh, CI and the release
// workflow never pass -short.
func skipInShortTier(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("takes a second or more: not in the short tier")
	}
}
