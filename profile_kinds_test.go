package musicxml

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

// Each profile kind is its own type, so passing a profile to an entry point
// of another kind, or OmitContext to a graph, fails to compile; the matching
// kinds compile. An external package is type-checked against the public API,
// one statement per line.
func TestProfileKindsAreDistinctTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks the package from source")
	}
	valid := []string{
		`_ = m.SourceOptions{Profile: m.ProfileSourceLine}`,
		`_ = m.ModelOptions{Profile: m.ProfileModelSemantics}`,
		`_ = m.GraphOptions{Profile: m.ProfileResourceGraph}`,
		`_ = m.MXLGraphOptions{Profile: m.ProfileMXLSourceLine}`,
		`_ = m.MXLGraphOptions{Profile: m.ProfileMXLModelSemantics}`,
		`_ = m.SourceOptions{OmitContext: true}`,
		`_ = m.ModelOptions{OmitContext: true}`,
		`_ = m.DecodeAssessmentOptions{Assessment: &m.SourceOptions{Profile: m.ProfileSourceLine}}`,
		`_ = m.ProfileSourceLine.ID() == m.ProfileModelSemantics.ID()`,
	}
	invalid := []string{
		`_ = m.SourceOptions{Profile: m.ProfileModelXSD}`,
		`_ = m.SourceOptions{Profile: m.ProfileResourceGraph}`,
		`_ = m.SourceOptions{Profile: m.ProfileMXLSource}`,
		`_ = m.ModelOptions{Profile: m.ProfileSourceXSDPreview}`,
		`_ = m.ModelOptions{Profile: m.ProfileResourceGraph}`,
		`_ = m.ModelOptions{Profile: m.ProfileMXLModel}`,
		`_ = m.GraphOptions{Profile: m.ProfileSourceLine}`,
		`_ = m.GraphOptions{Profile: m.ProfileModelSemantics}`,
		`_ = m.GraphOptions{Profile: m.ProfileMXLSource}`,
		`_ = m.MXLGraphOptions{Profile: m.ProfileResourceGraph}`,
		`_ = m.MXLGraphOptions{Profile: m.ProfileSourceLine}`,
		`_ = m.MXLGraphOptions{Profile: m.ProfileModelSemantics}`,
		`_ = m.GraphOptions{OmitContext: true}`,
		`_ = m.MXLGraphOptions{OmitContext: true}`,
		`_ = m.ModelOptions{DTD: nil}`,
	}
	var b strings.Builder
	b.WriteString("package probe\n\nimport m \"github.com/go-muse/go-musicxml\"\n\nfunc _() {\n")
	first := 6
	for _, line := range append(append([]string{}, valid...), invalid...) {
		b.WriteString("\t" + line + "\n")
	}
	b.WriteString("}\n")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "probe.go", b.String(), 0)
	if err != nil {
		t.Fatal(err)
	}
	// Only a type mismatch or a missing field counts: an unknown identifier
	// would fail for the wrong reason.
	failed := map[int]bool{}
	config := types.Config{Importer: importer.ForCompiler(fset, "source", nil), Error: func(err error) {
		e, ok := err.(types.Error)
		if !ok || e.Fset.Position(e.Pos).Filename != "probe.go" {
			return
		}
		if strings.Contains(e.Msg, "cannot use") || strings.Contains(e.Msg, "unknown field") {
			failed[e.Fset.Position(e.Pos).Line] = true
		} else {
			t.Errorf("unexpected type error: %v", e)
		}
	}}
	_, _ = config.Check("probe", fset, []*ast.File{file}, nil)
	for i, line := range valid {
		if failed[first+i] {
			t.Errorf("does not compile: %s", line)
		}
	}
	for i, line := range invalid {
		if !failed[first+len(valid)+i] {
			t.Errorf("compiles: %s", line)
		}
	}
}
