package validation

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPackageReviewMediaParametersAndScratch(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  Outcome
	}{
		{"application/vnd.recordare.musicxml+xml", OutcomePass},
		{"Application/VND.Recordare.MusicXML+XML; Charset=utf-8", OutcomePass},
		{"application/vnd.recordare.musicxml+xml; foo=bar", OutcomePass},
		{"application/vnd.recordare.musicxml; charset=utf-8", OutcomePass},
		{"application/vnd.recordare.musicxml; foo=bar", OutcomePass},
		{"Application/VND.Recordare.MusicXML", OutcomePass},
		{"application/pdf", OutcomeFail},
		{"application/x-unreviewed", OutcomeUnknown},
		{"", OutcomeUnknown},
	} {
		t.Run(tc.value, func(t *testing.T) {
			s := beginPackageFixture(t, PackageCheckFirstMediaType)
			before, _ := s.account.Usage(BudgetBufferedBytes)
			got := s.mediaDecision(context.Background(), Decision{}, Evidence[string]{State: EvidenceKnown, Value: tc.value})
			if got.Outcome != tc.want {
				t.Errorf("got %+v want %s", got, tc.want)
			}
			after, _ := s.account.Usage(BudgetBufferedBytes)
			if after != before {
				t.Errorf("transient MIME parse retained %d bytes", after-before)
			}
			_, _ = s.finish(context.Background(), PackageCompletion{})
		})
	}
}

type reviewCancelAfterParse struct{ calls int }

func (*reviewCancelAfterParse) Deadline() (time.Time, bool) { return time.Time{}, false }
func (*reviewCancelAfterParse) Done() <-chan struct{}       { return nil }
func (c *reviewCancelAfterParse) Err() error {
	c.calls++
	if c.calls >= 2 {
		return context.Canceled
	}
	return nil
}
func (*reviewCancelAfterParse) Value(any) any { return nil }
func TestPackageReviewMediaRechecksCancellation(t *testing.T) {
	s := beginPackageFixture(t, PackageCheckFirstMediaType)
	ctx := &reviewCancelAfterParse{}
	before, _ := s.account.Usage(BudgetBufferedBytes)
	got := s.mediaDecision(ctx, Decision{}, Evidence[string]{State: EvidenceKnown, Value: "application/vnd.recordare.musicxml"})
	if !errors.Is(s.terminal, context.Canceled) || got.Outcome == OutcomePass {
		t.Errorf("post-parse cancellation ignored: decision=%+v error=%v", got, s.terminal)
	}
	after, _ := s.account.Usage(BudgetBufferedBytes)
	if after != before {
		t.Errorf("canceled parse retained %d bytes", after-before)
	}
	_, _ = s.finish(context.Background(), PackageCompletion{})
}

func TestPackageReviewFileAdviceSharesMIMEPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, media string
		compressed  bool
		want        Outcome
	}{
		{"score.musicxml", "Application/VND.Recordare.MusicXML+XML", false, OutcomePass},
		{"score.musicxml", "application/vnd.recordare.musicxml+xml; charset=utf-8", false, OutcomePass},
		{"score.musicxml", "application/vnd.recordare.musicxml+xml; foo=bar", false, OutcomePass},
		{"score.mxl", "application/vnd.recordare.musicxml; foo=bar", true, OutcomePass},
		{"score.musicxml", "application/x-unreviewed", false, OutcomeUnknown},
		{"score.musicxml", "application/pdf", false, OutcomeFail},
		{"score.musicxml", "application/vnd.recordare.musicxml", false, OutcomeFail},
	} {
		t.Run(tc.name+"/"+tc.media, func(t *testing.T) {
			s, _, pin := packageAdviceFixtureCheck(t, PackageCheckFileMetadataAdvice)
			if tc.compressed {
				pin = s.boundaries["outer"].identity.Pin
			}
			v := PackageFileMetadataObservation{ID: 11, Package: "outer", Pin: pin, Name: Evidence[string]{State: EvidenceKnown, Value: tc.name}, MediaType: Evidence[string]{State: EvidenceKnown, Value: tc.media}}
			if err := s.observeFileMetadata(context.Background(), v); err != nil {
				t.Fatal(err)
			}
			b, err := s.finish(context.Background(), PackageCompletion{})
			if err != nil {
				t.Fatal(err)
			}
			findings := b.PackageReport().Findings
			if tc.want == OutcomePass {
				if len(findings) != 0 {
					t.Errorf("registered media advice=%+v", findings)
				}
			} else if len(findings) != 1 || findings[0].Outcome != tc.want {
				t.Errorf("findings=%+v want %s", findings, tc.want)
			}
		})
	}
}
func TestPackageReviewBinaryMetadataOutsideMusicXMLScope(t *testing.T) {
	s, _, _ := packageAdviceFixtureCheck(t, PackageCheckFileMetadataAdvice)
	if err := s.observeEntry(context.Background(), packageTestEntry(10, "cover.png")); err != nil {
		t.Fatal(err)
	}
	pin := interruptionPin("cover")
	pin.Representation = ResourceBinary
	v := PackageFileMetadataObservation{ID: 11, Package: "outer", Pin: pin, Entry: 10, ArchiveEntry: "cover.png", Location: Location{Target: TargetPackage, DocumentID: "package", Entry: "cover.png"}, Name: Evidence[string]{State: EvidenceKnown, Value: "cover.png"}, MediaType: Evidence[string]{State: EvidenceKnown, Value: "image/png"}}
	if err := s.observeFileMetadata(context.Background(), v); err != nil {
		t.Fatal(err)
	}
	b, err := s.finish(context.Background(), PackageCompletion{})
	if err != nil {
		t.Fatal(err)
	}
	if findings := b.PackageReport().Findings; len(findings) != 0 {
		t.Errorf("binary member received MusicXML advice: %+v", findings)
	}
}

func TestPackageReviewMimetypePartialPrefixIsUnknown(t *testing.T) {
	rules, profiles := packageFixture(t, PackageCheckMimetypeAdvice)
	rules[0].Rule.Class = ClassRecommendation
	rules[0].Rule.Role = RoleAdvisory
	rules[0].Rule.Category = CategoryAdvisory
	catalog, err := NewPackageCatalog(rules, profiles)
	if err != nil {
		t.Fatal(err)
	}
	s, err := catalog.begin(context.Background(), RunOptions{Context: ContextFamilyAll, DocumentID: "package", Profile: "test.package"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.openPackage(context.Background(), packageTestIdentity()); err != nil {
		t.Fatal(err)
	}
	if err = s.observeEntry(context.Background(), packageTestEntry(1, "mimetype")); err != nil {
		t.Fatal(err)
	}
	m := PackageMimetypeObservation{Entry: 1, Prefix: []byte("application/vnd.r"), ByteLength: Evidence[uint64]{State: EvidenceKnown, Value: uint64(len("application/vnd.recordare.musicxml"))}, ContentComplete: Evidence[bool]{State: EvidenceKnown, Value: true}}
	if err = s.observeMimetype(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	b, err := s.finish(context.Background(), PackageCompletion{})
	if err != nil {
		t.Fatal(err)
	}
	f := b.PackageReport().Findings
	if len(f) != 1 || f[0].Outcome != OutcomeUnknown || f[0].Reason != "mimetype-prefix-family-unresolved" {
		t.Errorf("unobserved bytes classified: %+v", f)
	}
}
