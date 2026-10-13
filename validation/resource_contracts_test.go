package validation

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func testResourcePin(id string) ResourcePin {
	return ResourcePin{CanonicalID: id, Revision: "1", Digest: strings.Repeat("a", 64), Representation: ResourceSourceXML}
}
func testDocumentRequest() ResourceRequest {
	return ResourceRequest{ID: "request-1", Document: &ResourceDocumentRequest{Source: testResourcePin("source"), Subject: NodeRef{"source", 1}, Kind: LinkedResourceAuto, RequiredProfile: "intrinsic", CatalogDigest: strings.Repeat("b", 64), Href: "child.xml"}}
}
func TestResourceTransportUnionRejectsMixedOrUnboundResults(t *testing.T) {
	q := testDocumentRequest()
	if err := ValidateResourceRequest(q); err != nil {
		t.Fatal(err)
	}
	body := io.NopCloser(strings.NewReader("<opus/>"))
	good := ResourceResult{RequestID: q.ID, State: EvidenceKnown, Pin: testResourcePin("child"), Body: body}
	if err := ValidateResourceResult(q, good); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*ResourceResult){"wrong request": func(r *ResourceResult) { r.RequestID = "other" }, "missing body": func(r *ResourceResult) { r.Body = nil }, "mixed": func(r *ResourceResult) { r.Roles = &ResourceRoleResult{} }, "unavailable payload": func(r *ResourceResult) { r.State = EvidenceUnknown; r.Reason = "not-found" }, "bad digest": func(r *ResourceResult) { r.Pin.Digest = "ABC" }, "invented model digest": func(r *ResourceResult) { r.Pin.Representation = ResourceCurrentModel; r.Pin.SnapshotID = "snapshot" }} {
		t.Run(name, func(t *testing.T) {
			r := good
			mutate(&r)
			if err := ValidateResourceResult(q, r); !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("accepted %v", err)
			}
		})
	}
}
func TestResourceAbsenceNeedsExactAuthorityAndNoPayload(t *testing.T) {
	q := testDocumentRequest()
	r := ResourceResult{RequestID: q.ID, State: EvidenceAbsent, Reason: "absent"}
	if err := ValidateResourceResult(q, r); err == nil {
		t.Fatal("unproved absence accepted")
	}
	r.Authority = EvidenceAuthority{ID: "table", Revision: "1", ExactRelationComplete: true, Provenance: []SourceRef{{IDs: []string{"lookup"}, URL: "https://example.test/mapping", Locator: "exact-query"}}}
	if err := ValidateResourceResult(q, r); err != nil {
		t.Fatal(err)
	}
}
func TestAssociationResultPartialMissAndEchoBinding(t *testing.T) {
	q := AssociationRequest{ID: "a1", Kind: AssociationInstrumentPartFile, Source: testResourcePin("source"), Target: testResourcePin("target"), SourcePart: NodeRef{"source", 1}, SourceInstrument: NodeRef{"source", 2}, PartLink: NodeRef{"source", 3}, InstrumentLink: NodeRef{"source", 4}, PartID: "P1", InstrumentID: "I1"}
	request := ResourceRequest{ID: q.ID, Association: &q}
	r := ResourceResult{RequestID: q.ID, State: EvidenceKnown, Association: &AssociationResult{Request: q, Membership: Evidence[bool]{State: EvidenceUnknown, Reason: "partial-table-miss"}, TargetPartID: Evidence[string]{State: EvidenceAbsent}, TargetInstrumentID: Evidence[string]{State: EvidenceAbsent}}}
	if err := ValidateResourceResult(request, r); err != nil {
		t.Fatal(err)
	}
	r.Association.Membership = Evidence[bool]{State: EvidenceKnown, Value: false}
	if err := ValidateResourceResult(request, r); err == nil {
		t.Fatal("uncovered negative accepted")
	}
	r.Association.Authority = EvidenceAuthority{ID: "map", Revision: "1", ExactRelationComplete: true, Provenance: []SourceRef{{IDs: []string{"fact"}, URL: "https://example.test/map", Locator: "row"}}}
	if err := ValidateResourceResult(request, r); err != nil {
		t.Fatal(err)
	}
	r.Association.Request.Target.Digest = strings.Repeat("c", 64)
	if err := ValidateResourceResult(request, r); err == nil {
		t.Fatal("stale target accepted")
	}
}
