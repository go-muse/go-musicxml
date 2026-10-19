package validationlink

import (
	"strings"
	"testing"
)

// register adds a test hook and removes it when the test ends, so that the
// tests leave the package-global registry as they found it and can repeat.
// The tests touch global state, so none of them runs in parallel.
func register(t *testing.T, name string, hook any) {
	t.Helper()
	Register(name, hook)
	t.Cleanup(func() { delete(hooks, name) })
}

func TestHookReturnsTheRegisteredFunction(t *testing.T) {
	register(t, "test.double", func(n int) int { return 2 * n })
	if got := Hook[func(int) int]("test.double")(21); got != 42 {
		t.Fatalf("got %d", got)
	}
}

func TestHookPanicsOnMissingOrMistypedHooks(t *testing.T) {
	register(t, "test.typed", func() string { return "" })
	for name, call := range map[string]func(){
		"missing":   func() { Hook[func()]("test.missing") },
		"mistyped":  func() { Hook[func() int]("test.typed") },
		"duplicate": func() { Register("test.typed", func() string { return "" }) },
	} {
		func() {
			defer func() {
				if r, ok := recover().(string); !ok || !strings.HasPrefix(r, "validationlink: ") {
					t.Errorf("%s: recovered %v", name, r)
				}
			}()
			call()
		}()
	}
}
