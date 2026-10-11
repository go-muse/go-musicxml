package xsd

import (
	"context"
	"errors"
	"testing"
)

func TestRunIdentityResultsAreMonotonicAcrossPreviewAndFinalize(t *testing.T) {
	c := testRunSchema(t)
	r, err := NewRun(c, &testMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	r.current = context.Background()
	// Seed absence before a duplicate, so final algorithm traversal visits the
	// newly provable absence first. Publication must still retain the old prefix.
	r.context.RecordIdentity(&TypeRef{Name: QName{Space: XSDNamespace, Local: "IDREF"}}, "missing", "/ref")
	ref := &TypeRef{Name: QName{Space: XSDNamespace, Local: "ID"}}
	r.context.RecordIdentity(ref, "duplicate", "/a")
	r.context.RecordIdentityBinding(ref, "duplicate", "/a", &Node{})
	r.context.RecordIdentityBinding(ref, "duplicate", "/b", &Node{})
	preview, err := r.IdentityPreview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if preview.Valid || preview.Complete || len(preview.Issues) != 1 || preview.Issues[0].Constraint != "ID" {
		t.Fatalf("preview=%+v", preview)
	}
	final, err := r.Finalize(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if final.Valid || !final.Complete || len(final.Issues) != 2 || final.Issues[0].Issue != preview.Issues[0].Issue || final.Issues[1].Constraint != "IDREF" {
		t.Fatalf("final=%+v", final)
	}
}

type identityDetailStopMeter struct {
	testMeter
	stop bool
}

func (m *identityDetailStopMeter) Reserve(n int64) error {
	if m.stop && n >= 400 {
		return errors.New("detail retention exhausted")
	}
	return m.testMeter.Reserve(n)
}
func TestRunIdentityFailureSurvivesDetailReservationStop(t *testing.T) {
	m := &identityDetailStopMeter{}
	r, err := NewRun(testRunSchema(t), m)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	r.current = context.Background()
	ref := &TypeRef{Name: QName{Space: XSDNamespace, Local: "ID"}}
	r.context.RecordIdentity(ref, "same", "/a")
	r.context.RecordIdentityBinding(ref, "same", "/a", &Node{})
	r.context.RecordIdentityBinding(ref, "same", "/b", &Node{})
	m.stop = true
	result, err := r.IdentityPreview(context.Background())
	if err == nil || result.Valid || result.Complete {
		t.Fatalf("known failure erased on stop: %+v, %v", result, err)
	}
}
