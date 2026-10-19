package validation

import (
	"context"
	"strings"
	"testing"
)

func TestPackageObservedFileDoesNotAdmitParsedCapabilities(t *testing.T) {
	s, _, _ := packageAdviceFixtureCheck(t, PackageCheckFileMetadataAdvice)
	if err := s.observeEntry(context.Background(), packageTestEntry(10, "failed.bad")); err != nil {
		t.Fatal(err)
	}
	v := PackageFileMetadataObservation{ID: 11, Package: "outer", Pin: ResourcePin{CanonicalID: "unparsed", Representation: ResourceSourceXML, Digest: strings.Repeat("e", 64)}, Entry: 10, ArchiveEntry: "failed.bad", Location: Location{Target: TargetPackage, DocumentID: "package", Entry: "failed.bad", Path: "rootfile[1]"}, Name: Evidence[string]{State: EvidenceKnown, Value: "failed.bad"}, MediaType: Evidence[string]{State: EvidenceKnown, Value: ""}}
	resources, adoptions := len(s.resources), len(s.adoptions)
	if err := s.observeFileMetadata(context.Background(), v); err != nil {
		t.Fatal(err)
	}
	if len(s.resources) != resources || len(s.adoptions) != adoptions || len(s.rootfiles) != 0 || len(s.roles) != 0 {
		t.Fatal("metadata granted parsed capabilities")
	}
	v.Name.Value = "mutated"
	v.Location.Path = "mutated"
	b, _ := s.finish(context.Background(), PackageCompletion{})
	got, ok := b.FileMetadataAt(0)
	if !ok || got.Name.Value != "failed.bad" || got.Location.Path != "rootfile[1]" || got.MediaType.State != EvidenceKnown || got.MediaType.Value != "" {
		t.Fatal(got)
	}
	if b.EnumerationComplete() || b.Satisfied() {
		t.Fatal("metadata certified incomplete package")
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatal("leaked", used)
	}
}
func TestPackageObservedFileRejectsForgedScope(t *testing.T) {
	for _, mutate := range []func(*PackageFileMetadataObservation){
		func(v *PackageFileMetadataObservation) { v.Entry = 999 },
		func(v *PackageFileMetadataObservation) { v.ArchiveEntry = "other.xml" },
		func(v *PackageFileMetadataObservation) { v.Name.Value = "other.xml" },
		func(v *PackageFileMetadataObservation) { v.Source = "other" },
		func(v *PackageFileMetadataObservation) { v.Location = Location{} },
		func(v *PackageFileMetadataObservation) { v.Pin.Digest = "" },
		func(v *PackageFileMetadataObservation) { v.BaseURI = strings.Repeat("x", 16385) },
	} {
		s, _, _ := packageAdviceFixtureCheck(t, PackageCheckFileMetadataAdvice)
		if err := s.observeEntry(context.Background(), packageTestEntry(10, "failed.xml")); err != nil {
			t.Fatal(err)
		}
		v := PackageFileMetadataObservation{ID: 11, Package: "outer", Pin: ResourcePin{CanonicalID: "unparsed", Representation: ResourceSourceXML, Digest: strings.Repeat("e", 64)}, Entry: 10, ArchiveEntry: "failed.xml", Location: Location{Target: TargetPackage, DocumentID: "package", Entry: "failed.xml"}, Name: Evidence[string]{State: EvidenceKnown, Value: "failed.xml"}, MediaType: Evidence[string]{State: EvidenceAbsent}}
		mutate(&v)
		if err := s.observeFileMetadata(context.Background(), v); err == nil {
			t.Fatal("accepted forged scope", v)
		}
		b, _ := s.finish(context.Background(), PackageCompletion{})
		if b.FileMetadataCount() != 0 {
			t.Fatal("retained refused metadata")
		}
		if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatal("leaked", used)
		}
	}
}

func TestPackageObservedFileRejectsUnadmittedBase(t *testing.T) {
	s, _, _ := packageAdviceFixtureCheck(t, PackageCheckFileMetadataAdvice)
	if err := s.observeEntry(context.Background(), packageTestEntry(10, "member.xml")); err != nil {
		t.Fatal(err)
	}
	v := PackageFileMetadataObservation{ID: 11, Package: "outer", Pin: ResourcePin{CanonicalID: "member", Representation: ResourceSourceXML, Digest: strings.Repeat("e", 64)}, Entry: 10, ArchiveEntry: "member.xml", BaseURI: "https://unadmitted.test/", Location: Location{Target: TargetPackage, DocumentID: "package", Entry: "member.xml"}, Name: Evidence[string]{State: EvidenceKnown, Value: "member.xml"}, MediaType: Evidence[string]{State: EvidenceAbsent}}
	if err := s.observeFileMetadata(context.Background(), v); err == nil {
		t.Fatal("accepted member under unadmitted effective base")
	}
	_, _ = s.finish(context.Background(), PackageCompletion{})
}
func TestPackageObservedExternalRejectsForgedOriginEntry(t *testing.T) {
	s, source, _ := packageAdviceFixtureCheck(t, PackageCheckFileMetadataAdvice)
	s.adoptions[source].resource.snapshot.contexts[0].Resource = &ResourceContextResult{Link: &ResourceLinkContext{Kind: ResourceOpusLink, Href: ResourceAttribute{Value: Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "https://e.test/file.xml"}}}}}
	v := PackageFileMetadataObservation{ID: 11, Package: "outer", Pin: ResourcePin{CanonicalID: "external", Representation: ResourceSourceXML, Digest: strings.Repeat("e", 64)}, Source: source, Href: "https://e.test/file.xml", BaseURI: "https://e.test/file.xml", Location: Location{Target: TargetPackage, DocumentID: "package", Entry: "forged-source.xml"}, Name: Evidence[string]{State: EvidenceKnown, Value: "file.xml"}, MediaType: Evidence[string]{State: EvidenceAbsent}}
	if err := s.observeFileMetadata(context.Background(), v); err == nil {
		t.Fatal("accepted forged source member location")
	}
	_, _ = s.finish(context.Background(), PackageCompletion{})
}
