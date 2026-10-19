package validation

import (
	"context"
	"errors"
	"testing"
)

func packageAssociationFixture(t *testing.T, indirection ...Evidence[bool]) (*PackageSession, AssociationRequest) {
	t.Helper()
	s, source, sourcePin := packageAdviceFixtureCheck(t, PackageCheckInstrumentAssociation)
	sourceSnapshot := s.adoptions[source].resource.snapshot
	part := NodeRef{"source", 1}
	instrument := NodeRef{"source", 2}
	id := func(value string, subject NodeRef, kind string, owner NodeRef) ResourceIDFact {
		return ResourceIDFact{Value: value, Subject: subject, Source: subject, Kind: Evidence[QName]{State: EvidenceKnown, Value: QName{Local: kind}}, Part: Evidence[NodeRef]{State: EvidenceKnown, Value: owner}, Unique: Evidence[bool]{State: EvidenceKnown, Value: true}}
	}
	sourceSnapshot.ids = []ResourceIDFact{id("P1", part, "score-part", part), id("I1", instrument, "score-instrument", part)}
	sourceSnapshot.contexts = []ContextResult{{Subject: NodeRef{"source", 3}, Resource: &ResourceContextResult{Part: Evidence[NodeRef]{State: EvidenceKnown, Value: part}, Link: &ResourceLinkContext{Kind: ResourcePartLink, Href: ResourceAttribute{Value: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "part.xml"}}}}}}, {Subject: NodeRef{"source", 4}, Resource: &ResourceContextResult{Owner: NodeRef{"source", 3}, Part: Evidence[NodeRef]{State: EvidenceKnown, Value: part}, Instrument: &ResourceInstrumentContext{Target: Evidence[NodeRef]{State: EvidenceKnown, Value: instrument}}}}}
	child, err := sourceSchemaEngine(t, XSDCheckScalar).Begin(context.Background(), RunOptions{DocumentID: "target", Profile: "source-schema-test", Account: s.account})
	if err != nil {
		t.Fatal(err)
	}
	h := schemaPendingHeader()
	h.Location.DocumentID = "target"
	if err = child.OpenNode(context.Background(), h); err != nil {
		t.Fatal(err)
	}
	if err = child.CloseNode(context.Background(), schemaLexemeEnd("1")); err != nil {
		t.Fatal(err)
	}
	_, targetSnapshot, err := child.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	targetSnapshot.header.Name = Evidence[QName]{State: EvidenceKnown, Value: QName{Local: "score-partwise"}}
	targetPart := NodeRef{"target", 1}
	targetInstrument := NodeRef{"target", 2}
	targetSnapshot.ids = []ResourceIDFact{id("Q9", targetPart, "score-part", targetPart), id("T7", targetInstrument, "score-instrument", targetPart)}
	targetPin := testResourcePin("target")
	resource, err := s.AddResource(context.Background(), targetPin, targetSnapshot, true)
	if err != nil {
		t.Fatal(err)
	}
	target, err := s.AdoptResource(context.Background(), resource, ResourceAdoptionOptions{Package: "outer", DocumentID: "target-view"})
	if err != nil {
		t.Fatal(err)
	}
	edge := PackageEdgeObservation{ID: 1, Source: source, Target: Evidence[ResourceAdoptionID]{State: EvidenceKnown, Value: target}, Indirection: Evidence[bool]{State: EvidenceKnown, Value: false}, Fragment: Evidence[string]{State: EvidenceAbsent}, FragmentTarget: Evidence[NodeRef]{State: EvidenceAbsent}}
	if len(indirection) != 0 {
		edge.Indirection = indirection[0]
	}
	if err = s.ObserveEdge(context.Background(), edge); err != nil {
		t.Fatal(err)
	}
	return s, AssociationRequest{ID: "association", Kind: AssociationInstrumentPartFile, Source: sourcePin, Target: targetPin, SourcePart: part, SourceInstrument: instrument, PartLink: NodeRef{"source", 3}, InstrumentLink: NodeRef{"source", 4}, PartID: "P1", InstrumentID: "I1"}
}
func packageAssociationReply(q AssociationRequest, membership Evidence[bool]) AssociationResult {
	return AssociationResult{Request: q, Membership: membership, TargetPartID: Evidence[string]{State: EvidenceKnown, Value: "Q9"}, TargetInstrumentID: Evidence[string]{State: EvidenceAbsent}, Authority: EvidenceAuthority{ID: "mapping", ExactRelationComplete: true, Provenance: []SourceRef{{IDs: []string{"membership"}, URL: "https://example.test/mapping", Locator: "row"}}}}
}
func TestPackageAssociationUsesMembershipWithoutCrossFileIDEquality(t *testing.T) {
	for _, targetInstrument := range []bool{false, true} {
		s, q := packageAssociationFixture(t)
		r := packageAssociationReply(q, Evidence[bool]{State: EvidenceKnown, Value: true})
		if targetInstrument {
			r.TargetInstrumentID = Evidence[string]{State: EvidenceKnown, Value: "T7"}
		}
		if err := s.ObserveAssociation(context.Background(), q, r); err != nil {
			t.Fatal(err)
		}
		sealFixtureExpansions(t, s)
		_ = s.SealEnumeration(context.Background(), completePackage("outer"))
		b, err := s.Finish(context.Background(), PackageCompletion{InputComplete: true})
		if err != nil || !b.Satisfied() || b.PackageReport().Counts.Pass != 1 {
			t.Fatalf("exact association=%v %+v", err, b.PackageReport())
		}
	}
}
func TestPackageAssociationKnownNegativeSurvivesCancellation(t *testing.T) {
	s, q := packageAssociationFixture(t)
	if err := s.ObserveAssociation(context.Background(), q, packageAssociationReply(q, Evidence[bool]{State: EvidenceKnown, Value: false})); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b, err := s.Finish(ctx, PackageCompletion{})
	if !errors.Is(err, context.Canceled) || b.PackageReport().Counts.Fail != 1 {
		t.Fatalf("known membership failure disappeared: %v %+v", err, b.PackageReport())
	}
}
func TestPackageAssociationPartialMissIsUnknown(t *testing.T) {
	s, q := packageAssociationFixture(t)
	r := packageAssociationReply(q, Evidence[bool]{State: EvidenceUnknown, Reason: "partial-table"})
	r.Authority = EvidenceAuthority{}
	if err := s.ObserveAssociation(context.Background(), q, r); err != nil {
		t.Fatal(err)
	}
	sealFixtureExpansions(t, s)
	_ = s.SealEnumeration(context.Background(), completePackage("outer"))
	b, err := s.Finish(context.Background(), PackageCompletion{InputComplete: true})
	if err != nil || b.Satisfied() || b.PackageReport().Counts.Unknown != 1 {
		t.Fatalf("partial relation=%v %+v", err, b.PackageReport())
	}
}
