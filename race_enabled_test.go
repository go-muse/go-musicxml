//go:build race

package musicxml

// raceEnabled reports a -race build, whose slower tests sample their corpus.
const raceEnabled = true
