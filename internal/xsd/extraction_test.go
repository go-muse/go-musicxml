package xsd

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestKernelExtractionBoundary(t *testing.T) {
	packages, err := parser.ParseDir(token.NewFileSet(), ".", func(f os.FileInfo) bool { return !strings.HasSuffix(f.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	p := packages["xsd"]
	if p == nil {
		t.Fatal("XML-neutral kernel is absent")
	}
	for name, f := range p.Files {
		for _, i := range f.Imports {
			path, _ := strconv.Unquote(i.Path.Value)
			if strings.Contains(path, "go-musicxml") || path == "encoding/xml" || path == "net" || (strings.HasPrefix(path, "net/") && path != "net/url" && path != "net/netip") {
				t.Errorf("forbidden kernel dependency %s in %s", path, name)
			}
		}
	}
}
