package musicxml

import (
	"errors"
	"testing"

	v "github.com/go-muse/go-musicxml/validation"
)

func TestIncompleteAssessmentPublicBridge(t *testing.T) {
	if ErrIncompleteAssessment != v.ErrIncompleteAssessment {
		t.Fatal("root and core incomplete sentinels must share identity")
	}
	for _, invalid := range []bool{false, true} {
		t.Run(map[bool]string{false: "pure", true: "joined"}[invalid], func(t *testing.T) {
			c := validationContext{}
			c.incomplete("unsupported exact-domain metadata")
			if invalid {
				c.issues = append(c.issues, ValidationIssue{Path: "/score-partwise", Constraint: "required", Message: "missing independently proven required attribute"})
			}
			err := c.result()
			if !errors.Is(err, ErrIncompleteAssessment) {
				t.Fatalf("missing public incomplete identity: %v", err)
			}
			if errors.Is(err, ErrInvalidDocument) != invalid {
				t.Fatalf("invented or lost normative identity: %v", err)
			}
			var technical *v.IncompleteError
			if !errors.As(err, &technical) {
				t.Fatalf("missing typed technical result: %v", err)
			}
			// Characterize the disclosed migration; callers should inspect the
			// public identities above rather than depend on a textual prefix.
			if got, want := technical.Error(), "validation: incomplete assessment: incomplete XSD assessment: unsupported exact-domain metadata"; got != want {
				t.Fatalf("documented technical text changed: %q, want %q", got, want)
			}
			var legacy *ValidationError
			if errors.As(err, &legacy) != invalid {
				t.Fatalf("legacy issue recognition = %v", err)
			}
			if invalid && len(legacy.Issues) != 1 {
				t.Fatalf("issues = %#v", legacy.Issues)
			}
		})
	}
}
