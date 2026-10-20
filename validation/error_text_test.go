package validation

import (
	"errors"
	"testing"
)

// Error texts name every nonempty part and the cause, and never print empty
// separators.
func TestErrorTextsIncludeCausesWithoutEmptyParts(t *testing.T) {
	cause := errors.New("reader failed")
	for _, tc := range []struct {
		err  error
		want string
	}{
		{&ConfigurationError{Field: "Profile", Reason: "unknown"}, "validation: invalid configuration: Profile: unknown"},
		{&ConfigurationError{Reason: "unknown"}, "validation: invalid configuration: unknown"},
		{&ConfigurationError{Field: "Profile"}, "validation: invalid configuration: Profile"},
		{&ConfigurationError{}, "validation: invalid configuration"},
		{&SessionError{Operation: "Finish", Reason: "already finished"}, "validation: invalid session: Finish: already finished"},
		{&SessionError{Operation: "Finish", Reason: "terminated", Cause: cause}, "validation: invalid session: Finish: terminated: reader failed"},
		{&SessionError{Cause: cause}, "validation: invalid session: reader failed"},
		{&SessionError{Operation: "Finish", Reason: "terminated", Cause: &SessionError{Operation: "OpenNode", Reason: "late"}}, "validation: invalid session: Finish: terminated: OpenNode: late"},
	} {
		if got := tc.err.Error(); got != tc.want {
			t.Errorf("%#v: %q, want %q", tc.err, got, tc.want)
		}
	}
	if !errors.Is(&SessionError{Cause: cause}, cause) || !errors.Is(&SessionError{Cause: cause}, ErrInvalidSession) || !errors.Is(&ConfigurationError{}, ErrInvalidConfiguration) {
		t.Fatal("error classes changed")
	}
}
