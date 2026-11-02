//go:build !race

package validation

// raceEnabled reports a -race build, whose heap measurements differ.
const raceEnabled = false
