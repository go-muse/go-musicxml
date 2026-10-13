package packageinventory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalPythonCompatible(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"z":"<&>\u2028\u2029é𝄞\b\f\n\r\t\u0001","a":[true,null,123456789012345678901234567890]}`)
	got := canonicalJSON(parseJSON(raw, "golden"))
	want := "{\"a\":[true,null,123456789012345678901234567890],\"z\":\"<&>\u2028\u2029é𝄞\\b\\f\\n\\r\\t\\u0001\"}"
	if string(got) != want {
		t.Fatalf("canonical = %q, want %q", got, want)
	}
	if digest(canonicalLF([]byte("a\r\nb\rc\n"))) != digest([]byte("a\nb\rc\n")) {
		t.Fatal("CRLF-only hashing changed")
	}
}

func TestStrictJSONRejectsAmbiguousInputs(t *testing.T) {
	t.Parallel()
	for _, raw := range [][]byte{[]byte(`{"a":1,"a":2}`), []byte(`{"a":{"x":0,"x":1}}`), []byte(`{} {}`), []byte(`{"a":NaN}`), {'"', 0xff, '"'}, []byte(`"\ud800"`)} {
		t.Run(string(raw), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("accepted %q", raw)
				}
			}()
			parseJSON(raw, "input")
		})
	}
	for _, raw := range []string{`true`, `0.0`, `"0"`, `null`} {
		t.Run("integer_"+raw, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("accepted non-integer")
				}
			}()
			node{parseJSON([]byte(raw), "index"), "index"}.integer()
		})
	}
	if nodeValue := (node{json.Number("0"), "index"}).integer(); nodeValue != 0 {
		t.Fatal(nodeValue)
	}
}

func TestAliasResolutionAndAtomicWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	if err := os.WriteFile(target, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"symlink", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			alias := filepath.Join(dir, kind)
			var err error
			if kind == "symlink" {
				err = os.Symlink(target, alias)
			} else {
				err = os.Link(target, alias)
			}
			if err != nil {
				t.Skip(err)
			}
			if err := rejectAlias(alias, []string{target}, defaultFileOps()); err == nil || !strings.Contains(err.Error(), "protected input") {
				t.Fatal(err)
			}
		})
	}
	output := filepath.Join(dir, "out.json")
	if err := atomicWrite(output, []byte("{\"ok\":true}\n"), defaultFileOps()); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(output)
	if err != nil || string(got) != "{\"ok\":true}\n" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestCanonicalFiniteNumberCompatibility(t *testing.T) {
	t.Parallel()
	cases := map[string]string{"1.0": "1.0", "1e0": "1.0", "1e15": "1000000000000000.0", "1e16": "1e+16", "1e-4": "0.0001", "1e-5": "1e-05", "-0.0": "-0.0", "-0": "0", "9007199254740993": "9007199254740993"}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			if got := string(canonicalJSON(parseJSON([]byte(input), input))); got != want {
				t.Fatalf("got %q want %q", got, want)
			}
		})
	}
}

func TestResolveSymlinkBeforeParentTraversal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(filepath.Join(nested, "child"), 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(filepath.Join(nested, "child"), alias); err != nil {
		t.Skip(err)
	}
	target := filepath.Join(nested, "protected.json")
	if err := os.WriteFile(target, []byte("preserved"), 0600); err != nil {
		t.Fatal(err)
	}
	// filepath.Join would erase the traversal before resolving the symlink.
	output := alias + string(filepath.Separator) + ".." + string(filepath.Separator) + "protected.json"
	if err := rejectAlias(output, []string{target}, defaultFileOps()); err == nil || !strings.Contains(err.Error(), "protected input") {
		t.Fatal(err)
	}
}

func TestAtomicTemporaryDirectoryPreservesSymlinkTraversal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(filepath.Join(nested, "child"), 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(filepath.Join(nested, "child"), alias); err != nil {
		t.Skip(err)
	}
	output := alias + string(filepath.Separator) + ".." + string(filepath.Separator) + "out.json"
	ops := defaultFileOps()
	create := ops.createTemp
	ops.createTemp = func(dir, pattern string) (atomicFile, error) {
		resolved, err := checkedResolve(dir, defaultFileOps())
		if err != nil {
			t.Fatal(err)
		}
		if resolved != nested {
			t.Fatalf("temporary directory = %s, want %s", resolved, nested)
		}
		return create(dir, pattern)
	}
	if err := atomicWrite(output, []byte("{}\n"), ops); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(nested, "out.json"))
	if err != nil || string(data) != "{}\n" {
		t.Fatalf("output=%q err=%v", data, err)
	}
}

func TestJoinUncleanUsesHostVolumeSemantics(t *testing.T) {
	t.Parallel()
	cases := []struct{ base, value, want string }{{"base", "child", "base" + string(filepath.Separator) + "child"}, {"", "child", "child"}}
	if filepath.Separator == '\\' {
		cases = append(cases, struct{ base, value, want string }{`C:`, `child`, `C:child`}, struct{ base, value, want string }{`\\server\share`, `child`, `\\server\share\child`})
	} else {
		cases = append(cases, struct{ base, value, want string }{"C:", "child", "C:/child"})
	}
	for _, c := range cases {
		if got := joinUnclean(c.base, c.value); got != c.want {
			t.Errorf("joinUnclean(%q,%q)=%q, want %q", c.base, c.value, got, c.want)
		}
	}
}
