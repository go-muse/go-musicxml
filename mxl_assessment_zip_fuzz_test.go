package musicxml

import (
	"bytes"
	"context"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func FuzzMXLAssessmentZIPObserver(f *testing.F) {
	for _, seed := range [][]byte{
		{}, []byte("PK\x03\x04"), assessmentZIPBytes(nil, nil, ""),
		assessmentZIPBytes([]assessmentZIPFixture{{name: "mimetype", body: mxlMIMEType, localExtra: "\xfe\xca\x00\x00"}}, nil, ""),
		assessmentZIPBytes([]assessmentZIPFixture{{name: "a", body: "deflate", method: 8, flags: 8, descriptor: 12}}, nil, "comment"),
		assessmentZIPBytes([]assessmentZIPFixture{{name: "a", body: "signed", flags: 8, descriptor: 16}, {name: "b", body: "other"}}, []int{1, 0}, ""),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		account := assessmentZIPAccount(t, validation.Budgets{MaxInputBytes: 1 << 20, MaxExternalBytes: 128 << 10, MaxBufferedBytes: 4 << 20, MaxNodes: 256, MaxScalarBytes: 65535, MaxRuleSteps: 1 << 20})
		a, e := openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), MXLOptions{MaxArchiveBytes: 1 << 20, MaxResourceBytes: 128 << 10, MaxResourcesBytes: 128 << 10}, account)
		defer assessmentZIPClose(t, a, account)
		if e != nil {
			return
		}
		if !a.ready || !a.inventoryComplete || !a.countConsistent || len(a.entries) != len(a.reader.File) {
			t.Fatal("successful archive lacks complete inventory")
		}
		var end int64
		for _, i := range a.physical {
			entry := a.entries[i]
			if !entry.trustedIdentity || !entry.associated || entry.localOffset != end {
				t.Fatal("unproved physical range")
			}
			end = entry.end
		}
		for i := 0; i < len(a.entries) && i < 3; i++ {
			_, _ = a.readEntry(i, mxlAssessmentResource)
		}
	})
}
