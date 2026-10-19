package validation

import (
	"context"
	"errors"
	"reflect"
	"unsafe"
)

// finishWithRetainedBundle finalizes like Finish, then keeps the published
// bundle's live storage charged to the session's account in a lease, so that
// a dependent stage of the same root operation stays bounded by the combined
// live storage. Finish itself releases every package and resource reservation
// although the returned bundle still owns its reports and contexts.
//
// The bundle's reports are measured once after Finish. If the measurement or
// its admission is refused, the bundle is returned with that error and no
// lease; callers must not continue dependent work. A nonnil lease must be
// closed, even when an error accompanies it, when the bundle's storage leaves
// the root operation.
func (s *PackageSession) finishWithRetainedBundle(ctx context.Context, end PackageCompletion) (AssessmentBundle, *ReportLease, error) {
	var account *Account
	if s != nil {
		account = s.account
	}
	bundle, err := s.finish(ctx, end)
	if account == nil || bundle.data == nil {
		return bundle, nil, err
	}
	if ctx == nil {
		return bundle, nil, err
	}
	n, measureErr := bundleRetainedBytes(ctx, account, bundle.data)
	if measureErr == nil {
		n, measureErr = rationalCheckedAdd(BudgetBufferedBytes, n, reportLeaseControlBytes)
	}
	if measureErr == nil {
		measureErr = account.Charge(BudgetBufferedBytes, n)
	}
	if measureErr != nil {
		return bundle, nil, errors.Join(err, measureErr)
	}
	return bundle, &ReportLease{account: account, retained: n}, err
}

// bundleRetainedBytes measures the storage a published bundle keeps, as
// resource snapshots and failure reports are measured: every report and
// record with its strings, slices and pointed-to values.
func bundleRetainedBytes(ctx context.Context, a *Account, d *assessmentBundleData) (int64, error) {
	total := int64(unsafe.Sizeof(*d))
	add := func(n int64, err error) error {
		if err != nil {
			return err
		}
		total, err = rationalCheckedAdd(BudgetBufferedBytes, total, n)
		return err
	}
	measure := func(v any) error { return add(resourceMeasure(ctx, a, reflect.ValueOf(v))) }
	if err := measure(d.report); err != nil {
		return 0, err
	}
	for _, r := range d.resources {
		if err := add(int64(unsafe.Sizeof(r))+int64(len(r.id)), nil); err != nil {
			return 0, err
		}
		if err := measure(r.report); err != nil {
			return 0, err
		}
	}
	if d.failureReport != nil {
		if err := measure(*d.failureReport); err != nil {
			return 0, err
		}
	}
	if d.failure != nil {
		if err := measure(*d.failure); err != nil {
			return 0, err
		}
	}
	for _, v := range d.adoptions {
		if err := measure(v); err != nil {
			return 0, err
		}
	}
	for _, v := range d.files {
		if err := measure(v); err != nil {
			return 0, err
		}
	}
	for _, v := range d.mimetypes {
		if err := measure(v); err != nil {
			return 0, err
		}
	}
	for _, v := range d.images {
		if err := measure(v); err != nil {
			return 0, err
		}
	}
	if d.omittedDiagnostics != nil {
		if err := add(int64(unsafe.Sizeof(*d.omittedDiagnostics)), nil); err != nil {
			return 0, err
		}
	}
	return total, nil
}

// BundleFailures summarizes why a bundle is not satisfied, from the
// authoritative conformance and counts of its package report, its required
// resource reports and its failure summary, or the failed report the summary
// retained, without copying those reports.
type BundleFailures struct {
	// Violated reports a report or failure summary that is
	// ConformanceViolated or counts a failed rule evaluation, even when its
	// diagnostics were truncated.
	Violated bool
	// Incomplete reports anything else that keeps the bundle from being
	// satisfied: a report that is incomplete, terminated or undetermined
	// without a violation, incomplete enumeration, or a retained failure
	// summary.
	Incomplete bool
	// Failed counts failed rule evaluations across those reports and the
	// failure summary.
	Failed uint64
	// Truncated reports that a violated report or failure summary omitted
	// stored diagnostics, so Findings may not list all of its failures.
	Truncated bool
	// Findings are copies of the stored failed normative and model findings.
	Findings []Finding
}

// Failures summarizes the bundle's failures. A satisfied bundle has none.
func (b AssessmentBundle) Failures() BundleFailures {
	var out BundleFailures
	if b.Satisfied() {
		return out
	}
	if b.data == nil {
		out.Incomplete = true
		return out
	}
	out.Incomplete = !b.data.enumeration
	reports := []*Report{&b.data.report}
	if f := b.data.failure; f != nil {
		out.Incomplete = true
		switch {
		case b.data.failureReport != nil:
			// The retained failed report carries the summary's counts and its
			// diagnostics.
			reports = append(reports, b.data.failureReport)
		case f.Conformance == ConformanceViolated || f.Counts.Fail != 0:
			out.Violated = true
			out.Truncated = out.Truncated || f.DiagnosticsTruncated
			out.Failed += f.Counts.Fail
		}
	}
	for i := range b.data.resources {
		if b.data.resources[i].required {
			reports = append(reports, &b.data.resources[i].report)
		}
	}
	for _, r := range reports {
		violated := reportViolated(*r)
		if violated {
			out.Violated = true
			out.Truncated = out.Truncated || r.DiagnosticsTruncated
		}
		out.Failed += r.Counts.Fail
		if !r.Satisfied() && !(violated && reportOnlyViolated(*r)) {
			out.Incomplete = true
		}
		for _, f := range r.Findings {
			if f.Outcome == OutcomeFail && (f.Category == CategoryNormative || f.Category == CategoryModel) {
				out.Findings = append(out.Findings, stateClone(f))
			}
		}
	}
	return out
}

// reportViolated reports a proved violation, whether or not its diagnostics
// were kept.
func reportViolated(r Report) bool {
	return r.Conformance == ConformanceViolated || r.Counts.Fail != 0
}

// reportOnlyViolated reports that every acceptance axis except conformance is
// complete, so a violation is the only reason the report is not satisfied.
func reportOnlyViolated(r Report) bool {
	return r.Requested && r.Profile.ID != "" && r.Target == r.Profile.Target &&
		r.InputComplete && reportSyntaxComplete(r.Target, r.Syntax) && r.AssessmentComplete &&
		r.BlockingProblems == 0 && r.Terminal == nil &&
		r.Counts.Unknown == 0 && r.Counts.Unsupported == 0 && r.Counts.Blocked == 0
}
