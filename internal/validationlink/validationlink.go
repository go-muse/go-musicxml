// Package validationlink carries the adapter entry points of package
// validation to the root musicxml package without exporting them. Package
// validation registers typed functions in its init; the root package looks
// them up by name at its own initialization, which runs later because it
// imports validation. The package imports neither, so it creates no cycle.
package validationlink

import "fmt"

var hooks = map[string]any{}

// Register records hook under name. It panics on a duplicate name, so it is
// only called from package initialization.
func Register(name string, hook any) {
	if _, duplicate := hooks[name]; duplicate {
		panic("validationlink: duplicate hook " + name)
	}
	hooks[name] = hook
}

// Hook returns the hook registered under name as a T. It panics when the hook
// is missing or has another type, which fails initialization of the caller.
func Hook[T any](name string) T {
	hook, ok := hooks[name].(T)
	if !ok {
		var want T
		panic(fmt.Sprintf("validationlink: hook %s is %T, want %T", name, hooks[name], want))
	}
	return hook
}
