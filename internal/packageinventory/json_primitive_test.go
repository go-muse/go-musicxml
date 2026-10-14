package packageinventory

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func inventoryFailure(t *testing.T, f func()) (message string) {
	t.Helper()
	defer func() {
		recovered := recover()
		failure, ok := recovered.(inventoryError)
		if !ok {
			t.Fatalf("expected inventoryError, got %T: %v", recovered, recovered)
		}
		message = failure.Error()
	}()
	f()
	t.Fatal("expected inventory failure")
	return ""
}

func TestInventoryTypeErrorsUseJSONVocabulary(t *testing.T) {
	t.Parallel()
	for _, actual := range []struct{ raw, name string }{
		{`null`, "null"}, {`true`, "boolean"}, {`"x"`, "string"}, {`[]`, "array"}, {`{}`, "object"}, {`1`, "integer"}, {`1.0`, "number"}, {`1e0`, "number"},
	} {
		for _, expected := range []string{"object", "array", "string", "integer"} {
			if expected == actual.name {
				continue
			}
			t.Run(actual.name+"_as_"+expected+"_"+actual.raw, func(t *testing.T) {
				n := node{parseJSON([]byte(`{"field":`+actual.raw+`}`), "manifest.json"), "manifest.json"}
				got := inventoryFailure(t, func() { n.fields(map[string]string{"field": expected}) })
				want := fmt.Sprintf("invalid inventory type: manifest.json/field: expected %s, got %s", expected, actual.name)
				if got != want {
					t.Fatalf("got %q, want %q", got, want)
				}
			})
		}
	}
}

func TestInventoryIntegerFieldsAcceptArbitraryPrecision(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"9223372036854775808", "-9223372036854775809", strings.Repeat("9", 100), "-" + strings.Repeat("9", 100)} {
		t.Run(raw, func(t *testing.T) {
			n := node{parseJSON([]byte(`{"count":`+raw+`,"index":`+raw+`}`), "manifest.json"), "manifest.json"}
			n.fields(map[string]string{"count": "integer", "index": "integer"})
			if got := string(canonicalJSON(n.value)); got != `{"count":`+raw+`,"index":`+raw+`}` {
				t.Fatalf("lost integer precision: %s", got)
			}
		})
	}
}

func TestJSONNonFiniteAndOverflowBoundaries(t *testing.T) {
	t.Parallel()
	for _, literal := range []string{"NaN", "Infinity", "-Infinity"} {
		t.Run(literal, func(t *testing.T) {
			message := inventoryFailure(t, func() { parseJSON([]byte(literal), "input.json") })
			if !strings.Contains(message, "cannot parse JSON: input.json:") {
				t.Fatal(message)
			}
		})
	}
	for _, literal := range []string{"1e400", "-1e400"} {
		t.Run(literal, func(t *testing.T) {
			value := parseJSON([]byte(literal), "input.json")
			if value != json.Number(literal) {
				t.Fatalf("token was changed: %v", value)
			}
			message := inventoryFailure(t, func() { canonicalJSON(value) })
			if message != "invalid finite JSON number: "+literal {
				t.Fatal(message)
			}
		})
	}
}

func TestInventoryIntegerValuesRetainPrecision(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"0", "-0", "9223372036854775808", "-9223372036854775809", strings.Repeat("9", 100)} {
		n := node{parseJSON([]byte(raw), "index"), "index"}
		want := raw
		if raw == "-0" {
			want = "0"
		}
		got := n.integerValue()
		if got.String() != want {
			t.Fatalf("got %s want %s", got, want)
		}
		got.SetInt64(1)
		if n.integerValue().String() != want {
			t.Fatal("integer conversion shared mutable state")
		}
	}
}

func TestNamedInputDiagnosticsPreserveDisplayLabels(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := filepath.Join(root, "source.json")
	label := "archive/source.json"
	missing := inventoryFailure(t, func() { readBytesNamed(source, label) })
	if strings.Contains(missing, root) || !strings.HasPrefix(missing, "cannot read input: "+label+":") {
		t.Fatal(missing)
	}
	maintenanceWrite(t, source, []byte(`{"bad":`))
	malformed := inventoryFailure(t, func() { readJSONNamed(source, label) })
	if !strings.HasPrefix(malformed, "cannot parse JSON: "+label+":") {
		t.Fatal(malformed)
	}
	maintenanceWrite(t, source, []byte(`{"count":true}`))
	parsed := readJSONNamed(source, label)
	typed := inventoryFailure(t, func() { parsed.fields(map[string]string{"count": "integer"}) })
	if typed != "invalid inventory type: archive/source.json/count: expected integer, got boolean" {
		t.Fatal(typed)
	}
	defaultLabel := inventoryFailure(t, func() { readJSON(source).fields(map[string]string{"count": "integer"}) })
	if !strings.Contains(defaultLabel, source+"/count:") {
		t.Fatal(defaultLabel)
	}
}
