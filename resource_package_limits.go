package musicxml

import "fmt"

// This fixed adapter evidence bound is separate from configurable transport and
// scalar budgets. Reject untrusted overlong input before the core contract;
// truncation would alter exact member/token identity.
func packageEvidenceTextLimit(field, value string) error {
	if len(value) > 4096 {
		return fmt.Errorf("%w: %s exceeds fixed package evidence limit of 4096 bytes (observed %d)", ErrMXLTooLarge, field, len(value))
	}
	return nil
}
