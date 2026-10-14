package musicxml

//go:generate go run ./internal/xsdgen/cmd/xsdgen -kind validation -schema-dir testdata/validation -schema attribute-wildcard-union-contract.xsd -catalog catalog.xml -package musicxml -validation-name validationAttributeWildcardUnionGenerated -output zz_generated_attribute_wildcard_union_validation_test.go

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

type attributeWildcardUnionCase struct {
	name, source string
	valid        bool
}

func attributeWildcardUnionCases() []attributeWildcardUnionCase {
	var cases []attributeWildcardUnionCase
	for _, simple := range []string{"", "simple-"} {
		for _, typ := range []string{"extended", "inherited", "restricted", "closed", "reextended"} {
			for _, attribute := range []struct{ prefix, namespace string }{{"", ""}, {"b:", "urn:base"}, {"d:", "urn:derived"}, {"l:", "urn:last"}, {"o:", "urn:outside"}} {
				valid := false
				switch typ {
				case "extended", "inherited":
					valid = attribute.namespace == "" || attribute.namespace == "urn:base" || attribute.namespace == "urn:derived"
				case "restricted":
					valid = attribute.namespace == "urn:derived"
				case "reextended":
					valid = attribute.namespace == "urn:derived" || attribute.namespace == "urn:last"
				}
				element := simple + typ
				source := `<` + element + ` xmlns:b="urn:base" xmlns:d="urn:derived" xmlns:l="urn:last" xmlns:o="urn:outside" ` + attribute.prefix + `a="x"/>`
				cases = append(cases, attributeWildcardUnionCase{element + "/" + attribute.namespace, source, valid})
			}
		}
	}
	for _, attr := range []string{`a="x"`, `b:a="x"`, `o:a="x"`} {
		cases = append(cases, attributeWildcardUnionCase{"any/" + attr, `<any-extended xmlns:b="urn:base" xmlns:o="urn:outside" ` + attr + `/>`, true})
	}
	cases = append(cases, attributeWildcardUnionCase{"derived-skip-uses-base-namespace", `<derived-skip xmlns:b="urn:base" b:a="x"/>`, true})
	return cases
}
func attributeWildcardUnionBind(t *testing.T, compiled *xsd.Compiled, source string) xsd.Result {
	t.Helper()
	node, err := parseValidationDocument([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	run, err := xsd.NewRun(compiled, kernelTestMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer run.Close()
	use, ok := compiled.Root(node.Name)
	if !ok {
		t.Fatal("missing declaration")
	}
	attrs := append([]xsd.Attribute(nil), node.Attrs...)
	for i := range attrs {
		attrs[i].ValueKnown = true
	}
	b, err := run.Bind(context.Background(), use, xsd.Subject{ID: 1, Name: node.Name, Text: node.Text.String(), TextKnown: true, Attributes: attrs, AttributesComplete: true, ChildrenComplete: true})
	if err != nil {
		t.Fatal(err)
	}
	return run.Results(b).Attributes
}
func TestGeneratedAttributeWildcardUnionSourceAndCompiled(t *testing.T) {
	compiled, err := xsd.Compile(validationAttributeWildcardUnionGenerated, xsd.CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	digest := compiled.Digest()
	for _, tc := range attributeWildcardUnionCases() {
		t.Run(tc.name, func(t *testing.T) {
			legacy := assessAttributeSource(t, &validationAttributeWildcardUnionGenerated, tc.source)
			if legacy.AssessmentErr != nil || (len(legacy.Issues) == 0) != tc.valid {
				t.Fatalf("source validity=%v err=%v issues=%+v", tc.valid, legacy.AssessmentErr, legacy.Issues)
			}
			got := attributeWildcardUnionBind(t, compiled, tc.source)
			if got.Valid != tc.valid || !got.Complete {
				t.Fatalf("compiled validity=%v: %+v", tc.valid, got)
			}
		})
	}
	if compiled.Digest() != digest {
		t.Fatal("effective union mutated compiled identity")
	}
}
func TestGeneratedAttributeWildcardUnionModesRemainUnavailable(t *testing.T) {
	compiled, err := xsd.Compile(validationAttributeWildcardUnionGenerated, xsd.CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"lax", "strict"} {
		for _, attr := range []string{`unknown="x"`, `known="1"`, `known="bad"`, `xmlns:b="urn:base" b:a="x"`} {
			got := attributeWildcardUnionBind(t, compiled, `<derived-`+mode+` `+attr+`/>`)
			if !got.Valid || got.Complete || len(got.Issues) != 0 {
				t.Fatalf("%s %s: %+v", mode, attr, got)
			}
			found := false
			for _, limitation := range got.Limitations {
				found = found || limitation.Reason == "wildcard-"+mode+"-unavailable"
			}
			if !found {
				t.Fatalf("lost %s mode: %+v", mode, got)
			}
		}
	}
}
func TestAttributeWildcardUnionAgainstXMLLint(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal(err)
		}
		t.Skip("xmllint not installed")
	}
	schema := filepath.Join("testdata", "validation", "attribute-wildcard-union-contract.xsd")
	catalog := filepath.Join("testdata", "validation", "catalog.xml")
	for _, tc := range attributeWildcardUnionCases() {
		t.Run(tc.name, func(t *testing.T) { assertXMLLintOutcome(t, xmllint, schema, catalog, tc.source, tc.valid) })
	}
	// The actual oracle can check global declarations. Our unavailable result
	// above must not be confused with either these valid or invalid outcomes.
	for _, tc := range []attributeWildcardUnionCase{
		{"lax-unknown", `<derived-lax unknown="x"/>`, true},
		{"lax-known-valid", `<derived-lax known="1"/>`, true},
		{"lax-known-invalid", `<derived-lax known="bad"/>`, false},
		{"strict-unknown", `<derived-strict unknown="x"/>`, false},
		{"strict-known-valid", `<derived-strict known="1"/>`, true},
		{"strict-known-invalid", `<derived-strict known="bad"/>`, false},
		{"derived-strict-base-namespace", `<derived-strict xmlns:b="urn:base" b:a="x"/>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) { assertXMLLintOutcome(t, xmllint, schema, catalog, tc.source, tc.valid) })
	}
}
func TestAttributeWildcardUnexpressibleUnionXMLLintSchemaError(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal(err)
		}
		t.Skip("xmllint not installed")
	}
	// XSD 1.0 3.10.6 union 5.3: not(urn:owner) union {absent} is
	// inexpressible. This is schema invalidity, never an instance violation.
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:owner" targetNamespace="urn:owner"><xs:complexType name="base"><xs:anyAttribute namespace="##other" processContents="skip"/></xs:complexType><xs:complexType name="derived"><xs:complexContent><xs:extension base="t:base"><xs:anyAttribute namespace="##local" processContents="skip"/></xs:extension></xs:complexContent></xs:complexType><xs:element name="n" type="t:derived"/></xs:schema>`
	file := filepath.Join(t.TempDir(), "unexpressible.xsd")
	if err := os.WriteFile(file, []byte(schema), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(xmllint, "--nonet", "--noout", "--schema", file, "-")
	command.Stdin = strings.NewReader(`<n xmlns="urn:owner" a="x"/>`)
	output, err := command.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 5 || !strings.Contains(string(output), "not expressible") {
		t.Fatalf("expected schema compilation error, got %v: %s", err, output)
	}
}
