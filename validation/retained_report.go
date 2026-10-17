package validation

import "context"

// reportLeaseControlBytes admits the small ownership handle before allocation.
// Context payload and backing-array allowances have already been admitted by
// publishValidatedContextResult and are transferred, never charged twice.
const reportLeaseControlBytes int64 = 64

// ReportLease keeps a finished report's context projection charged to its root
// account while another assessment stage runs. It owns accounting only: the
// returned Report remains caller-owned and is not invalidated by Close. Its
// diagnostic quotas remain cumulative independently of this lease.
//
// Do not copy a lease or use it concurrently. Caller mutations or additional
// copies of the report are outside the admitted projection's storage contract.
type ReportLease struct {
	account  *Account
	retained int64
	closed   bool
}

// FinishWithRetainedReport finalizes like Finish, but transfers the already
// admitted context projection to an optional lease before cleaning intrinsic
// state. It does not copy the report or create a resource snapshot. A nonnil
// lease must be closed even when a partial report accompanies an error.
//
// Reports without context need no lease. If its fixed ownership handle cannot
// be admitted, the best report is returned as incomplete with no lease; callers
// must not continue dependent assessment work. Close the lease when its storage
// is discarded or transferred out of the root operation to its caller.
func (s *Session) FinishWithRetainedReport(ctx context.Context, end Completion) (report Report, lease *ReportLease, err error) {
	report, _, err = s.finishWithOwnership(ctx, end, false, &lease)
	return report, lease, err
}

func (s *Session) retainFinishedReport() (*ReportLease, error) {
	if s.contextReportRetained == 0 {
		return nil, nil
	}
	if s.contextReportRetained < 0 || s.contextReportRetained > s.retained {
		return nil, &SessionError{Operation: "retain report", Reason: "projection ownership exceeds session retention"}
	}
	if err := s.account.Charge(BudgetBufferedBytes, reportLeaseControlBytes); err != nil {
		return nil, err
	}
	lease := &ReportLease{account: s.account, retained: s.contextReportRetained + reportLeaseControlBytes}
	// Transfer ownership before deferred cleanup. Account usage is unchanged
	// except for the separately admitted handle, including on partial failures.
	s.retained -= s.contextReportRetained
	s.contextReportRetained = 0
	return lease, nil
}

// Close relinquishes the live context/control allowance exactly once. It needs
// no context and is safe for a nil or already-closed lease. The report's data
// remain usable; closing does not refund cumulative work or diagnostic quotas.
func (l *ReportLease) Close() error {
	if l == nil || l.closed {
		return nil
	}
	if l.retained != 0 {
		if err := l.account.Release(BudgetBufferedBytes, l.retained); err != nil {
			return err
		}
	}
	l.retained = 0
	l.account = nil
	l.closed = true
	return nil
}
