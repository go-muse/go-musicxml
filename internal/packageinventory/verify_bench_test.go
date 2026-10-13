package packageinventory

import (
	"path/filepath"
	"testing"
)

func BenchmarkVerifyCommittedInventory(b *testing.B) {
	root, err := filepath.Abs("../..")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := runner{fs: defaultFileOps(), opts: options{root: root, manifest: defaultManifest, evidence: defaultEvidence, strict: true}}
		result := r.verify()
		if failures := resultFailures(result); len(failures) > 0 {
			b.Fatal(failures)
		}
	}
}
