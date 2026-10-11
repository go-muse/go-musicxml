package musicxml_test

import (
	"reflect"
	"testing"

	musicxml "github.com/go-muse/go-musicxml"
)

func TestValidationIssueKeepsPublicNamedIdentity(t *testing.T) {
	typ := reflect.TypeOf(musicxml.ValidationIssue{})
	if typ.PkgPath() != "github.com/go-muse/go-musicxml" || typ.Name() != "ValidationIssue" {
		t.Fatalf("public named type moved: %v (%s)", typ, typ.PkgPath())
	}
	issue := musicxml.ValidationIssue{Path: "/score", Constraint: "required", Message: "missing"}
	if issue.String() != "/score: missing" {
		t.Fatal(issue.String())
	}
}
