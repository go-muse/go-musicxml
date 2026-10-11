package musicxml

import (
	"bytes"
	"os"
	"testing"
)

// BenchmarkDecode measures the ordinary transport API on the committed official
// UTF-8 and UTF-16 scores. Fixture I/O is outside the measured operation; model
// construction and all mandatory XML checks are included, with no assessment.
func BenchmarkDecode(b *testing.B) {
	for _, fixture := range []string{"MozartTrio.musicxml", "MozaChloSample.musicxml"} {
		b.Run(fixture, func(b *testing.B) {
			input, err := os.ReadFile("testdata/official/" + fixture)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(input)))
			for b.Loop() {
				if _, err := Decode(bytes.NewReader(input)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
