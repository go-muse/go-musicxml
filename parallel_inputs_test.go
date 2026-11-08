package musicxml

import "testing"

// inParallel runs body once for each of n inputs, each in its own parallel
// subtest named name(i) within a group named group, and returns when all of
// them have finished. The inputs run concurrently only with each other: a
// serial caller waits for them, so a process-wide switch it set before the
// call stays set for all of them and no other test runs meanwhile, and what
// the bodies wrote is visible to the caller afterwards.
func inParallel(t *testing.T, group string, n int, name func(i int) string, body func(t *testing.T, i int)) {
	t.Helper()
	t.Run(group, func(t *testing.T) {
		for i := range n {
			t.Run(name(i), func(t *testing.T) {
				t.Parallel()
				body(t, i)
			})
		}
	})
}
