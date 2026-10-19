package validationlink

import (
	"strings"
	"testing"
)

func TestHookReturnsTheRegisteredFunction(t *testing.T) {
	Register("test.double", func(n int) int { return 2 * n })
	if got := Hook[func(int) int]("test.double")(21); got != 42 {
		t.Fatalf("got %d", got)
	}
}

func TestHookPanicsOnMissingOrMistypedHooks(t *testing.T) {
	Register("test.typed", func() string { return "" })
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
