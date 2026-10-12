package validation

// spanScratch admits provider-local indexes separately from retained endpoint
// and public output storage. Failed, canceled and successful runs all release it.
type spanScratch struct {
	s     *Session
	bytes int64
}

func (w *spanScratch) reserve(n int64) error {
	if err := w.s.account.Charge(BudgetBufferedBytes, n); err != nil {
		return err
	}
	w.bytes += n
	return nil
}
func (w *spanScratch) release() {
	if w.bytes != 0 {
		_ = w.s.account.Release(BudgetBufferedBytes, w.bytes)
		w.bytes = 0
	}
}
