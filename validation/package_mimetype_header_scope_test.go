package validation

import (
	"context"
	"errors"
	"testing"
)

func TestPackageMimetypeLocalHeaderExtraScope(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*PackageEntryObservation)
		want   Outcome
	}{
		{"neither", func(e *PackageEntryObservation) {}, OutcomePass},
		{"local-only", func(e *PackageEntryObservation) { e.Local.ExtraLength.Value = 4 }, OutcomeFail},
		{"central-only", func(e *PackageEntryObservation) { e.Central.ExtraLength.Value = 4 }, OutcomePass},
		{"both", func(e *PackageEntryObservation) { e.Local.ExtraLength.Value = 4; e.Central.ExtraLength.Value = 4 }, OutcomeFail},
		{"central-length-unavailable", func(e *PackageEntryObservation) {
			e.Central.ExtraLength = Evidence[uint64]{State: EvidenceUnknown, Reason: "unobserved-central-length"}
		}, OutcomePass},
		{"local-length-unavailable", func(e *PackageEntryObservation) {
			e.Local.ExtraLength = Evidence[uint64]{State: EvidenceUnknown, Reason: "unobserved-local-length"}
		}, OutcomeUnknown},
		{"association-unavailable", func(e *PackageEntryObservation) {
			e.HeaderAssociation = Evidence[bool]{State: EvidenceUnknown, Reason: "unproved-association"}
		}, OutcomeUnknown},
		{"central-framing-unavailable", func(e *PackageEntryObservation) {
			e.Central.Present = Evidence[bool]{State: EvidenceUnknown, Reason: "unproved-central-framing"}
		}, OutcomeUnknown},
		{"unrelated-entry", func(e *PackageEntryObservation) {
			e.Name = "other"
			e.Location.Entry = "other"
			e.Local.ExtraLength.Value = 4
			e.Central.ExtraLength.Value = 4
		}, OutcomeNotApplicable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := beginPackageFixture(t, PackageCheckMimetypeNoExtra)
			if err := s.OpenPackage(context.Background(), packageTestIdentity()); err != nil {
				t.Fatal(err)
			}
			e := packageTestEntry(1, "mimetype")
			tc.change(&e)
			if err := s.ObserveEntry(context.Background(), e); err != nil {
				t.Fatal(err)
			}
			if err := s.SealEnumeration(context.Background(), completePackage("outer")); err != nil {
				t.Fatal(err)
			}
			b, err := s.Finish(context.Background(), PackageCompletion{InputComplete: true})
			if err != nil {
				t.Fatal(err)
			}
			r := b.PackageReport()
			want := Counts{}
			switch tc.want {
			case OutcomePass:
				want.Pass = 1
			case OutcomeFail:
				want.Fail = 1
			case OutcomeUnknown:
				want.Unknown = 1
			case OutcomeNotApplicable:
				want.NotApplicable = 1
			}
			if r.Counts != want || b.Satisfied() != (tc.want == OutcomePass || tc.want == OutcomeNotApplicable) {
				t.Fatalf("wanted %s, got %+v", tc.want, r)
			}
			if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
				t.Fatalf("leaked %d buffered bytes", used)
			}
		})
	}
}

func TestPackageMimetypeLocalExtraFailureSurvivesUnavailableCentralAndCancellation(t *testing.T) {
	for _, cancelFinish := range []bool{false, true} {
		s := beginPackageFixture(t, PackageCheckMimetypeNoExtra)
		if err := s.OpenPackage(context.Background(), packageTestIdentity()); err != nil {
			t.Fatal(err)
		}
		e := packageTestEntry(1, "mimetype")
		e.Local.ExtraLength.Value = 4
		e.Central.Present = Evidence[bool]{State: EvidenceUnknown, Reason: "malformed-central-record"}
		e.Central.ExtraLength = Evidence[uint64]{State: EvidenceUnknown, Reason: "malformed-central-record"}
		e.HeaderAssociation = Evidence[bool]{State: EvidenceUnknown, Reason: "unavailable-association"}
		if err := s.ObserveEntry(context.Background(), e); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		if cancelFinish {
			cancel()
		}
		b, err := s.Finish(ctx, PackageCompletion{})
		cancel()
		if cancelFinish && !errors.Is(err, context.Canceled) || !cancelFinish && err != nil {
			t.Fatal(err)
		}
		r := b.PackageReport()
		if r.Counts.Fail != 1 || r.Conformance != ConformanceViolated || r.AssessmentComplete || b.Satisfied() || len(r.Findings) != 1 || r.Findings[0].Reason != "mimetype-local-extra-field" {
			t.Fatalf("local proof lost: %+v", r)
		}
		if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("leaked %d buffered bytes", used)
		}
	}
}
