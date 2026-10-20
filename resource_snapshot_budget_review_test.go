package musicxml

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func resourceReviewSampleRun(data []byte, graph bool, budget int64) error {
	opts := SourceOptions{Profile: ProfileSourceLinks, Budgets: validation.Budgets{MaxBufferedBytes: budget}}
	if graph {
		_, err := InspectXMLWithResources(context.Background(), bytes.NewReader(data), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph, Budgets: opts.Budgets}})
		return err
	}
	_, err := InspectXML(context.Background(), bytes.NewReader(data), opts)
	return err
}

func TestResourceReviewOfficialSamplesFitDefaultSnapshotBudget(t *testing.T) {
	for _, file := range []string{"MozartTrio.musicxml", "MozaChloSample.musicxml"} {
		t.Run(file, func(t *testing.T) {
			data, err := os.ReadFile("testdata/official/" + file)
			if err != nil {
				t.Fatal(err)
			}
			bundle, err := InspectXMLWithResources(context.Background(), bytes.NewReader(data), LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}})
			if err != nil {
				t.Fatalf("official DTD-bearing source exceeds default snapshot budget: %v", err)
			}
			_, report, ok := bundle.ResourceReport(0)
			if !ok || report.AssessmentComplete || bundle.Satisfied() {
				t.Fatalf("budget optimization erased unread-DTD evidence: %+v", report)
			}
		})
	}
}

// This optional witness is an exact logical admission search, not a heap/RSS
// measurement. Keep it opt-in so routine tests do not re-run each sample 30 times.
func TestResourceReviewOfficialExactBufferWitness(t *testing.T) {
	if os.Getenv("MUSICXML_SNAPSHOT_BUFFER_WITNESS") != "1" {
		t.Skip("explicit resource witness")
	}
	for _, file := range []string{"MozartTrio.musicxml", "MozaChloSample.musicxml"} {
		data, err := os.ReadFile("testdata/official/" + file)
		if err != nil {
			t.Fatal(err)
		}
		for _, graph := range []bool{false, true} {
			low, high := int64(1), int64(256<<20)
			if err := resourceReviewSampleRun(data, graph, high); err != nil {
				t.Fatal(err)
			}
			for low < high {
				mid := low + (high-low)/2
				err := resourceReviewSampleRun(data, graph, mid)
				if err == nil {
					high = mid
				} else if errors.Is(err, validation.ErrResourceLimit) {
					low = mid + 1
				} else {
					t.Fatal(err)
				}
			}
			if err := resourceReviewSampleRun(data, graph, low); err != nil {
				t.Fatalf("exact minimum failed: %v", err)
			}
			if err := resourceReviewSampleRun(data, graph, low-1); !errors.Is(err, validation.ErrResourceLimit) {
				t.Fatalf("one under did not refuse: %v", err)
			}
			t.Logf("file=%s graph=%t minimum_buffer=%d", file, graph, low)
		}
	}
}

func BenchmarkResourceReviewOfficialSnapshots(b *testing.B) {
	for _, file := range []string{"MozartTrio.musicxml", "MozaChloSample.musicxml"} {
		data, err := os.ReadFile("testdata/official/" + file)
		if err != nil {
			b.Fatal(err)
		}
		for _, graph := range []bool{false, true} {
			name := file + "/links"
			if graph {
				name = file + "/graph"
			}
			b.Run(name, func(b *testing.B) {
				if err := resourceReviewSampleRun(data, graph, 256<<20); err != nil {
					b.Fatal(err)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if err := resourceReviewSampleRun(data, graph, 256<<20); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
