package validation

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"unsafe"
)

func dtdIDSession(t *testing.T) (*Session, RuleKey) {
	t.Helper()
	_, s, key := sourceOriginFixture(t, Budgets{})
	s.catalog.fragmentModes = map[ProfileID]map[Assembly]fragmentMode{s.profile.ID: {AssemblyMusicXML: fragmentJoinedShorthand}}
	s.catalog.fragmentDTDs = map[ProfileID]map[Assembly]bool{s.profile.ID: {AssemblyMusicXML: true}}
	s.catalog.sourceDeclarations = map[ProfileID]map[Assembly]xsdPlanMode{s.profile.ID: {AssemblyMusicXML: xsdSourceDeclarationsV1}}
	if e := s.Open(context.Background(), schemaPendingHeader()); e != nil {
		t.Fatal(e)
	}
	if e := s.SourceDTDIDDomain(context.Background(), SourceDTDIDDomain{"source", stateKnown(true)}); e != nil {
		t.Fatal(e)
	}
	return s, key
}
func dtdIDAdmittedAttribute(t *testing.T, s *Session, key RuleKey, name string, value string) SourceDTDIDAttribute {
	t.Helper()
	q := QName{Local: name}
	origin := sourceOriginPhysical()
	origin.Attribute = &q
	origin.Declaration = stateKnown(Location{Target: TargetSource, DocumentID: "source", Position: &SourcePosition{CoordinateSpace: "original-bytes", ByteOffset: 5}})
	a := sourceOriginAttribute(q, origin.Physical.Value)
	a.Scalar.Lexeme = stateKnown(value)
	if e := s.Attribute(context.Background(), a); e != nil {
		t.Fatal(e)
	}
	if e := s.SourceOrigin(context.Background(), key, origin); e != nil {
		t.Fatal(e)
	}
	return SourceDTDIDAttribute{Subject: NodeRef{"source", 1}, Attribute: q, Value: value, IsID: stateKnown(true), Declaration: &SourceDTDIDDeclaration{Sequence: 1, Element: "root", Attribute: name, DefaultKind: "implied", Origin: stateKnown(Location{Target: TargetSource, DocumentID: "source", Position: &SourcePosition{CoordinateSpace: "original-bytes", ByteOffset: 5}})}, Specified: stateKnown(true), Use: origin.Physical}
}
func TestSourceDTDIDIngressRejectsForgedOccurrence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*SourceDTDIDAttribute)
	}{
		{"document", func(f *SourceDTDIDAttribute) { f.Subject.DocumentID = "other" }},
		{"owner", func(f *SourceDTDIDAttribute) { f.Subject.Node = 2 }},
		{"value", func(f *SourceDTDIDAttribute) { f.Value = "another" }},
		{"slot", func(f *SourceDTDIDAttribute) { f.Attribute.Local = "other" }},
		{"lexical-owner", func(f *SourceDTDIDAttribute) { f.Declaration.Element = "other" }},
		{"lexical-attribute", func(f *SourceDTDIDAttribute) { f.Declaration.Attribute = "other" }},
		{"zero-sequence", func(f *SourceDTDIDAttribute) { f.Declaration.Sequence = 0 }},
		{"origin-document", func(f *SourceDTDIDAttribute) { f.Declaration.Origin.Value.DocumentID = "other" }},
		{"specifiedness", func(f *SourceDTDIDAttribute) { f.Specified.Value = false }},
		{"forged-use", func(f *SourceDTDIDAttribute) { f.Use.Value.Position.ByteOffset = 99999 }},
		{"forged-declaration", func(f *SourceDTDIDAttribute) { f.Declaration.Origin.Value.Position.ByteOffset = 99999 }},
		{"missing-declaration", func(f *SourceDTDIDAttribute) { f.Declaration = nil }},
		{"unknown-with-declaration", func(f *SourceDTDIDAttribute) { f.IsID = stateUnknown[bool]("unknown") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, key := dtdIDSession(t)
			f := dtdIDAdmittedAttribute(t, s, key, "a", "  exact  ")
			tc.mutate(&f)
			if e := s.SourceDTDIDAttribute(context.Background(), f); !errors.Is(e, ErrInvalidConfiguration) {
				t.Fatalf("forged event admitted: %v", e)
			}
			s.cleanup()
			if n, _ := s.account.Usage(BudgetBufferedBytes); n != 0 {
				t.Fatalf("leaked %d", n)
			}
		})
	}
}
func TestSourceDTDIDIngressOwnershipSealsAndDuplicateBinding(t *testing.T) {
	t.Run("owned", func(t *testing.T) {
		s, key := dtdIDSession(t)
		defer s.cleanup()
		f := dtdIDAdmittedAttribute(t, s, key, "a", "value")
		if e := s.SourceDTDIDAttribute(context.Background(), f); e != nil {
			t.Fatal(e)
		}
		f.Declaration.Origin.Value.Position.ByteOffset = 99
		f.Declaration.Element = "mutated"
		if s.dtdIDs.IDs[0].Declaration.Origin.Value.Position.ByteOffset != 5 || s.dtdIDs.IDs[0].Declaration.Element != "root" {
			t.Fatal("borrowed declaration")
		}
		if e := s.SourceDTDIDAttribute(context.Background(), s.dtdIDs.IDs[0]); e == nil {
			t.Fatal("duplicate admitted")
		}
	})
	t.Run("sequence-collision", func(t *testing.T) {
		s, key := dtdIDSession(t)
		defer s.cleanup()
		f := dtdIDAdmittedAttribute(t, s, key, "a", "value")
		if e := s.SourceDTDIDAttribute(context.Background(), f); e != nil {
			t.Fatal(e)
		}
		g := dtdIDAdmittedAttribute(t, s, key, "b", "other")
		if e := s.SourceDTDIDAttribute(context.Background(), g); e == nil {
			t.Fatal("sequence reused")
		}
	})
	t.Run("omission", func(t *testing.T) {
		s, key := dtdIDSession(t)
		defer s.cleanup()
		_ = dtdIDAdmittedAttribute(t, s, key, "a", "value")
		if e := s.Close(context.Background(), schemaLexemeEnd("1")); e != nil {
			t.Fatal(e)
		}
		if e := s.CompleteSourceDTDIDs(context.Background(), "source"); e == nil {
			t.Fatal("missing occurrence sealed")
		}
	})
}
func TestSourceDTDIDSnapshotOwnershipAndAccounting(t *testing.T) {
	s, key := dtdIDSession(t)
	f := dtdIDAdmittedAttribute(t, s, key, "a", "value")
	if e := s.SourceDTDIDAttribute(context.Background(), f); e != nil {
		t.Fatal(e)
	}
	if e := s.Close(context.Background(), schemaLexemeEnd("1")); e != nil {
		t.Fatal(e)
	}
	if e := s.CompleteSourceDTDIDs(context.Background(), "source"); e != nil {
		t.Fatal(e)
	}
	_, snap, e := s.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if e != nil || snap == nil {
		t.Fatalf("snapshot: %v", e)
	}
	if snap.fragments == nil || !knownComplete(snap.fragments.DTDIdentityComplete) || len(snap.fragments.DTDIDs) != 1 {
		t.Fatalf("missing selected facts: %+v", snap.fragments)
	}
	copy, e := resourceLiveCopy(context.Background(), snap.account, *snap.fragments, "source", "adopted")
	if e != nil {
		t.Fatal(e)
	}
	copy.DTDIDs[0].Declaration.Origin.Value.Position.ByteOffset = 77
	if snap.fragments.DTDIDs[0].Declaration.Origin.Value.Position.ByteOffset != 5 || copy.DTDIDs[0].Subject.DocumentID != "adopted" || copy.DTDIDs[0].Declaration.Origin.Value.DocumentID != "adopted" {
		t.Fatal("copy/remapping alias")
	}
	snap.Close()
	if n, _ := s.account.Usage(BudgetBufferedBytes); n != 0 {
		t.Fatalf("leaked %d", n)
	}
	if unsafe.Sizeof(resourceFragmentElement{}) > 256 || unsafe.Sizeof(resourceFragmentSnapshot{}) > 1024 {
		t.Fatal("snapshot envelope exceeded")
	}
}
func TestJoinedIDFirstOwnerAndIncompleteAuthorities(t *testing.T) {
	p := beginPackageFixture(t, PackageCheckLinkTarget)
	defer p.cleanup()
	snap := &ResourceSnapshot{header: ResourceSnapshotHeader{IDScopeComplete: stateKnown(true)}, ids: []ResourceIDFact{{Value: "ID", Subject: NodeRef{"doc", 2}, Unique: Evidence[bool]{State: EvidenceInvalid, Reason: "duplicate", Cause: &DecisionRef{Rule: RuleKey{"schema", 1}, Subject: NodeRef{"doc", 1}}}}}, fragments: &resourceFragmentSnapshot{Mode: fragmentJoinedShorthand, DTDRequired: true, DTDIdentityComplete: stateKnown(true), Complete: stateKnown(true), Elements: []resourceFragmentElement{{Subject: NodeRef{"doc", 99}, Order: 1}, {Subject: NodeRef{"doc", 2}, Order: 2}}, DTDIDs: []SourceDTDIDAttribute{{Subject: NodeRef{"doc", 99}, Value: "ID"}}}}
	got, e := p.resolveFragmentID(context.Background(), snap, "ID")
	if e != nil || got.State != EvidenceKnown || got.Value.Node != 99 {
		t.Fatalf("not document-first: %+v %v", got, e)
	}
	for _, field := range []string{"DTDIdentityComplete", "Complete"} {
		v := reflect.ValueOf(snap.fragments).Elem().FieldByName(field)
		old := v.Interface().(Evidence[bool])
		v.Set(reflect.ValueOf(stateUnknown[bool]("incomplete")))
		for _, id := range []string{"ID", "absent"} {
			got, e = p.resolveFragmentID(context.Background(), snap, id)
			if e != nil || got.State != EvidenceUnknown {
				t.Fatalf("partial %s proved %s: %+v %v", field, id, got, e)
			}
		}
		v.Set(reflect.ValueOf(old))
	}
	snap.fragments.DTDRequired = false
	snap.fragments.DTDIDs = nil
	got, e = p.resolveFragmentID(context.Background(), snap, "ID")
	if e != nil || got.State != EvidenceKnown || got.Value.Node != 2 {
		t.Fatalf("XSD validity erased typing: %+v %v", got, e)
	}
}
