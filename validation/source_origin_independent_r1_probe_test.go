package validation

import (
	"context"
	"errors"
	"testing"
)

// Independently exercise the public ingress rather than only the publication helper.
func TestIndependentR1PhysicalEntityAttribute(t *testing.T) {
	_, s, key := sourceOriginFixture(t, Budgets{})
	defer s.cleanup()
	ctx := context.Background()
	h := schemaPendingHeader()
	h.Location = sourceOriginPhysical().Physical.Value
	if err := s.Open(ctx, h); err != nil {
		t.Fatal(err)
	}
	name := QName{Local: "a"}
	in := sourceOriginPhysical()
	in.Attribute = &name
	in.Kind = SourceOriginEntity
	in.Declaration = stateKnown(cloneLocation(in.Physical.Value))
	in.Use = stateKnown(cloneLocation(in.Physical.Value))
	in.Ancestry = []SourceOriginStep{{Entity: "e", Declaration: in.Declaration, Use: in.Use}}
	if err := s.Attribute(ctx, sourceOriginAttribute(name, in.Physical.Value)); err != nil {
		t.Fatal(err)
	}
	if err := s.SourceOrigin(ctx, key, in); err != nil {
		t.Fatal(err)
	}
	got := s.contextResults[0].SourceOrigin
	if got.Kind != SourceOriginEntity || got.Specified.State != EvidenceKnown || !got.Specified.Value || got.Physical.State != EvidenceKnown {
		t.Fatalf("lost legitimate shape: %+v", got)
	}
}

func TestIndependentR1OriginCanceledBeforeMalformed(t *testing.T) {
	_, s, key := sourceOriginFixture(t, Budgets{})
	defer s.cleanup()
	h := schemaPendingHeader()
	h.Location = sourceOriginPhysical().Physical.Value
	if err := s.Open(context.Background(), h); err != nil {
		t.Fatal(err)
	}
	in := sourceOriginPhysical()
	in.Kind = "invalid"
	before := s.retained
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := s.SourceOrigin(ctx, key, in)
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrInvalidConfiguration) || len(s.contextResults) != 0 || len(s.sourceOriginSeen) != 0 || s.retained != before {
		t.Fatalf("canceled invalid publication: err=%v retained=%d/%d", err, s.retained, before)
	}
}
