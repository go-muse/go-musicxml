package musicxml

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The bundle iterators yield exactly what the indexed accessors return, in
// order, and stop when the loop breaks.
func TestAssessmentBundleIterators(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "corpus", "musicxml-test-suite", "90a-Compressed-MusicXML.mxl"))
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := InspectMXL(context.Background(), bytes.NewReader(data), MXLAssessmentOptions{})
	if err != nil || bundle.ResourceCount() == 0 || bundle.AdoptionCount() == 0 {
		t.Fatalf("fixture: resources=%d adoptions=%d err=%v", bundle.ResourceCount(), bundle.AdoptionCount(), err)
	}
	i := 0
	for id, report := range bundle.Resources() {
		wantID, want, _ := bundle.ResourceReport(i)
		if id != wantID || !reflect.DeepEqual(report, want) {
			t.Errorf("resource %d differs", i)
		}
		i++
	}
	if i != bundle.ResourceCount() {
		t.Fatalf("yielded %d of %d resources", i, bundle.ResourceCount())
	}
	i = 0
	for id, document := range bundle.Adoptions() {
		wantID, wantDocument, _ := bundle.Adoption(i)
		if id != wantID || document != wantDocument {
			t.Errorf("adoption %d differs", i)
		}
		i++
	}
	j := 0
	for context := range bundle.AdoptionContexts() {
		want, _ := bundle.AdoptionContext(j)
		if !reflect.DeepEqual(context, want) {
			t.Errorf("adoption context %d differs", j)
		}
		j++
	}
	if i != bundle.AdoptionCount() || j != i {
		t.Fatalf("yielded %d adoptions and %d contexts of %d", i, j, bundle.AdoptionCount())
	}
	broke := 0
	for range bundle.Adoptions() {
		broke++
		break
	}
	if broke != 1 {
		t.Fatal("break did not stop the sequence")
	}
}
