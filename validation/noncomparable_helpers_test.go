package validation

import (
	"strings"
	"testing"
)

// The helpers that replaced == on structs holding exact values keep its
// meaning: every field takes part.

func TestResourceAttributeAbsentChecksEveryField(t *testing.T) {
	if !resourceAbsentAttribute().absent() {
		t.Fatal("the absent attribute is not absent")
	}
	for name, mutate := range map[string]func(*ResourceAttribute){
		"presence": func(a *ResourceAttribute) { a.Presence = Known(false) },
		"raw":      func(a *ResourceAttribute) { a.Raw.Value = "x" },
		"state":    func(a *ResourceAttribute) { a.Value.State = EvidenceKnown },
		"reason":   func(a *ResourceAttribute) { a.Value.Reason = "r" },
		"cause":    func(a *ResourceAttribute) { a.Value.Cause = &DecisionRef{} },
		"text":     func(a *ResourceAttribute) { a.Value.Value.Text = "x" },
		"name":     func(a *ResourceAttribute) { a.Value.Value.Name.Local = "x" },
		"integer":  func(a *ResourceAttribute) { a.Value.Value.Integer = Integer{text: "0"} },
		"decimal":  func(a *ResourceAttribute) { a.Value.Value.Decimal = Decimal{bytes: 1} },
	} {
		a := resourceAbsentAttribute()
		mutate(&a)
		if a.absent() {
			t.Errorf("%s: a changed attribute is still absent", name)
		}
	}
}

func TestAtomicZeroAndIdentity(t *testing.T) {
	checkUnavailableEvidenceWith(t, Atomic{Name: QName{Local: "x"}}, validateAtomicEvidence)
	checkUnavailableEvidenceWith(t, Atomic{Text: "x"}, validateAtomicEvidence)
	one := Atomic{Kind: AtomicDecimal, Decimal: localDecimal(t, "1")}
	if !atomicEvidenceIdentical(Known(one), Known(one)) {
		t.Fatal("a value is not identical to itself")
	}
	if atomicEvidenceIdentical(Known(one), Known(Atomic{Kind: AtomicDecimal, Decimal: localDecimal(t, "1")})) {
		t.Fatal("identity compared numeric values instead of representations")
	}
}

func TestImageDimensionRejectsValueOnUnavailableEvidence(t *testing.T) {
	pin := func(id string, r ResourceRepresentation) ResourcePin {
		return ResourcePin{CanonicalID: id, Digest: strings.Repeat("a", 64), Representation: r}
	}
	request := ImageMetadataRequest{ID: "image", Source: pin("source", ResourceSourceXML), Subject: NodeRef{DocumentID: "source", Node: 1}, Href: "plot.png", MediaType: "image/png"}
	q := ResourceRequest{ID: "image", Image: &request}
	result := func(width Evidence[Decimal]) ResourceResult {
		return ResourceResult{RequestID: "image", State: EvidenceKnown, Image: &ImageMetadataResult{
			Request: request, Resource: pin("plot", ResourceBinary), Unit: ImageUnitPixel,
			Width: width, Height: Known(localDecimal(t, "5")), PixelAspectRatio: Evidence[Decimal]{State: EvidenceAbsent},
			Authority: EvidenceAuthority{ID: "authority", Provenance: []SourceRef{{IDs: []string{"source"}, URL: "https://example.test/image", Locator: "metadata"}}},
		}}
	}
	if err := ValidateResourceResult(q, result(Known(localDecimal(t, "10")))); err != nil {
		t.Fatalf("valid image result: %v", err)
	}
	if err := ValidateResourceResult(q, result(Evidence[Decimal]{State: EvidenceUnknown, Reason: "unavailable", Value: localDecimal(t, "10")})); err == nil {
		t.Fatal("unknown width carried a value")
	}
}
