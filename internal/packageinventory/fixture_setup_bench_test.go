package packageinventory

import (
	"os"
	"path/filepath"
	"testing"
)

// These benchmarks isolate fixture construction from the verifier and mutation
// commands, all of which still read and validate each fixture normally.
func BenchmarkMaintenanceFixtureSetup(b *testing.B) {
	// Include first-use initialization outside the steady-state measurement.
	warm := newMaintenanceFixture(b)
	if err := os.RemoveAll(warm.root); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f := newMaintenanceFixture(b)
		b.StopTimer()
		if err := os.RemoveAll(f.root); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
	}
}

func BenchmarkMaintenanceClone(b *testing.B) {
	for _, tc := range []struct{ name, path string }{
		{"Manifest", maintenanceManifest},
		{"Evidence", maintenanceEvidence},
	} {
		b.Run(tc.name, func(b *testing.B) {
			value := maintenanceJSON(b, maintenanceRead(b, filepath.Join(maintenanceRepo(b), tc.path)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				clone := maintenanceClone(b, value)
				if len(clone) != len(value) {
					b.Fatal("clone changed the top-level field count")
				}
			}
		})
	}
}
