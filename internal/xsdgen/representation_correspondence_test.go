package xsdgen

import (
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

// The source representability rule must judge each numeric built-in type by
// the Go type the generator gives it: a mismatch would silently accept values
// the model changes, or reject values it holds.
func TestModelRepresentationMatchesGeneratedGoTypes(t *testing.T) {
	checked := 0
	for name, goType := range builtinGoTypes {
		representation, ok := xsd.ModelRepresentationOf(name)
		switch goType.Expression {
		case "float64":
			if !ok || !representation.Binary64 {
				t.Errorf("%s is float64 but not a binary64 representation", name)
			}
		case "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64":
			bits := uint(0)
			for _, width := range []uint{8, 16, 32, 64} {
				if strings.HasSuffix(goType.Expression, big.NewInt(int64(width)).String()) {
					bits = width
				}
			}
			min, max := new(big.Int), new(big.Int).Lsh(big.NewInt(1), bits)
			if strings.HasPrefix(goType.Expression, "int") {
				max.Rsh(max, 1)
				min.Neg(max)
			}
			max.Sub(max, big.NewInt(1))
			if !ok || representation.Binary64 || representation.Min.Cmp(min) != 0 || representation.Max.Cmp(max) != 0 {
				t.Errorf("%s is %s but represented as %+v", name, goType.Expression, representation)
			}
		default:
			if ok {
				t.Errorf("%s is %s but has a numeric representation", name, goType.Expression)
			}
			continue
		}
		checked++
	}
	if checked != 15 {
		t.Errorf("checked %d numeric built-in types, want 15", checked)
	}
}

// xs:float has no representation in the rule because the model would hold it
// as a binary32; no bundled MusicXML schema uses it.
func TestBundledSchemasUseNoFloat(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "schema", "musicxml-4.0", "*.xsd"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("schemas: %v %v", paths, err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), `"xs:float"`) {
			t.Errorf("%s uses xs:float, which the representability rule does not check", path)
		}
	}
}
