package musicxml

import (
	"bytes"
	"os"
	"testing"
)

// BenchmarkValidate measures explicit validation of an already decoded official
// score. Decoding and fixture I/O are outside the measured operation.
func BenchmarkValidate(b *testing.B) {
	input, err := os.ReadFile("testdata/official/MozaChloSample.musicxml")
	if err != nil {
		b.Fatal(err)
	}
	document, err := Decode(bytes.NewReader(input))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if err := Validate(document); err != nil {
			b.Fatal(err)
		}
	}
}
