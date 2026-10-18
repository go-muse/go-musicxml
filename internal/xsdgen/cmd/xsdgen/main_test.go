package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testCatalog = `<?xml version="1.0"?>
<catalog xmlns="urn:oasis:names:tc:entity:xmlns:xml:catalog"/>`

func writeSchema(t *testing.T, directory string, enumerations ...string) {
	t.Helper()
	var values strings.Builder
	for _, value := range enumerations {
		values.WriteString(`<xs:enumeration value="` + value + `"/>`)
	}
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:simpleType name="state"><xs:restriction base="xs:token">` +
		values.String() + `</xs:restriction></xs:simpleType></xs:schema>`
	if err := os.WriteFile(filepath.Join(directory, "types.xsd"), []byte(schema), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "catalog.xml"), []byte(testCatalog), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunNameManifest(t *testing.T) {
	directory := t.TempDir()
	writeSchema(t, directory, "open")
	manifest := filepath.Join(directory, "names.txt")
	output := filepath.Join(directory, "out.go")
	arguments := func(extra ...string) []string {
		return append([]string{"-kind", "simple", "-schema-dir", directory, "-schema", "types.xsd", "-catalog", "catalog.xml", "-package", "example", "-names", manifest, "-output", output}, extra...)
	}

	// The first generation creates the manifest.
	if err := run(arguments("-update-names"), io.Discard); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if want := "types.xsd type state\tState\ntypes.xsd type state enumeration \"open\"\tStateOpen\n"; string(data) != want {
		t.Fatalf("manifest:\n%s\nwant:\n%s", data, want)
	}

	// A complete manifest checks without changes.
	if err := run(arguments(), io.Discard); err != nil {
		t.Fatal(err)
	}

	// A new component fails the check and leaves the output and the
	// manifest alone.
	before, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	writeSchema(t, directory, "open", "closed")
	err = run(arguments(), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "-update-names") || !strings.Contains(err.Error(), "StateClosed") {
		t.Fatalf("incomplete manifest accepted: %v", err)
	}
	after, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("a failed check wrote the output")
	}
	if unchanged, _ := os.ReadFile(manifest); string(unchanged) != string(data) {
		t.Fatal("a failed check wrote the manifest")
	}

	if err := run(append(arguments()[:len(arguments())-4], "-update-names", "-output", output), io.Discard); err == nil {
		t.Fatal("-update-names without -names accepted")
	}
}
