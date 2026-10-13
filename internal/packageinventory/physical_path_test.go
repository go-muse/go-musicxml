package packageinventory

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func physicalTraversalFixture(t *testing.T) (root, nested, traversed string) {
	t.Helper()
	root = t.TempDir()
	nested = filepath.Join(root, "nested")
	if err := os.MkdirAll(filepath.Join(nested, "child"), 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(filepath.Join(nested, "child"), alias); err != nil {
		t.Fatal(err)
	}
	return root, nested, alias + string(filepath.Separator) + ".."
}

func TestRepositoryPathSurvivesNativeLexicalNormalization(t *testing.T) {
	t.Parallel()
	root, nested, traversed := physicalTraversalFixture(t)
	maintenanceWrite(t, filepath.Join(root, "input"), []byte("wrong"))
	maintenanceWrite(t, filepath.Join(nested, "input"), []byte("correct"))
	// Emulate native Windows normalization at the final I/O boundary.
	path := filepath.Clean(repositoryPath(traversed, "input"))
	maintenanceEqual(t, string(maintenanceRead(t, path)), "correct")
}

func TestReadBytesResolvesPhysicalTraversal(t *testing.T) {
	t.Parallel()
	root, nested, traversed := physicalTraversalFixture(t)
	maintenanceWrite(t, filepath.Join(root, "input"), []byte("wrong"))
	maintenanceWrite(t, filepath.Join(nested, "input"), []byte("correct"))
	maintenanceEqual(t, string(readBytes(traversed+string(filepath.Separator)+"input")), "correct")
}

func TestResolveStatsOnlyPhysicalPaths(t *testing.T) {
	t.Parallel()
	_, nested, traversed := physicalTraversalFixture(t)
	ops := defaultFileOps()
	ops.stat = func(path string) (os.FileInfo, error) {
		for _, component := range strings.Split(filepath.ToSlash(path), "/") {
			if component == ".." {
				return nil, errors.New("unresolved parent traversal reached native stat")
			}
		}
		return os.Stat(filepath.Clean(path))
	}
	got, err := checkedResolve(traversed, ops)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(nested)
	if err != nil {
		t.Fatal(err)
	}
	if !samePhysicalDirectory(t, got, want) {
		t.Fatalf("resolved=%q want=%q err=%v", got, want, err)
	}
}

func TestHardlinkAliasSurvivesNativeLexicalNormalization(t *testing.T) {
	t.Parallel()
	root, nested, traversed := physicalTraversalFixture(t)
	protected := filepath.Join(root, "protected")
	maintenanceWrite(t, protected, []byte("protected"))
	maintenanceWrite(t, filepath.Join(root, "output"), []byte("unrelated"))
	if err := os.Link(protected, filepath.Join(nested, "output")); err != nil {
		t.Fatal(err)
	}
	ops := defaultFileOps()
	ops.stat = func(path string) (os.FileInfo, error) { return os.Stat(filepath.Clean(path)) }
	output := traversed + string(filepath.Separator) + "output"
	if err := rejectAlias(output, []string{protected}, ops); err == nil || !strings.Contains(err.Error(), "protected input") {
		t.Fatalf("physical hardlink alias not rejected: %v", err)
	}
}

func TestAtomicWriteSurvivesNativeLexicalNormalization(t *testing.T) {
	t.Parallel()
	root, nested, traversed := physicalTraversalFixture(t)
	maintenanceWrite(t, filepath.Join(root, "output"), []byte("untouched"))
	ops := defaultFileOps()
	create, rename := ops.createTemp, ops.rename
	ops.createTemp = func(dir, pattern string) (atomicFile, error) {
		return create(filepath.Clean(dir), pattern)
	}
	ops.rename = func(old, new string) error { return rename(filepath.Clean(old), filepath.Clean(new)) }
	if err := atomicWrite(traversed+string(filepath.Separator)+"output", []byte("correct"), ops); err != nil {
		t.Fatal(err)
	}
	maintenanceEqual(t, string(maintenanceRead(t, filepath.Join(nested, "output"))), "correct")
	maintenanceEqual(t, string(maintenanceRead(t, filepath.Join(root, "output"))), "untouched")
}

func TestAtomicWriteReplacesFinalSymlinkWithoutChangingReferent(t *testing.T) {
	t.Parallel()
	root, nested, traversed := physicalTraversalFixture(t)
	referent := filepath.Join(root, "referent")
	maintenanceWrite(t, referent, []byte("untouched"))
	destination := filepath.Join(nested, "output")
	if err := os.Symlink(referent, destination); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(traversed+string(filepath.Separator)+"output", []byte("replacement"), defaultFileOps()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(destination)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("destination is still a symlink: %v", err)
	}
	maintenanceEqual(t, string(maintenanceRead(t, destination)), "replacement")
	maintenanceEqual(t, string(maintenanceRead(t, referent)), "untouched")
}

func TestResolveRejectsNonDirectoryIntermediateComponents(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	file := filepath.Join(root, "file")
	maintenanceWrite(t, file, []byte("not a directory"))
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(file, alias); err != nil {
		t.Fatal(err)
	}
	separator := string(filepath.Separator)
	for _, base := range []string{file, alias} {
		for _, suffix := range []string{separator + ".." + separator + "output", separator + ".", separator, separator + "child"} {
			path := base + suffix
			if got, err := checkedResolve(path, defaultFileOps()); err == nil {
				t.Errorf("non-directory traversal %q resolved to %q", path, got)
			}
		}
	}
}

func samePhysicalDirectory(t *testing.T, first, second string) bool {
	t.Helper()
	firstInfo, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	secondInfo, err := os.Stat(second)
	if err != nil {
		t.Fatal(err)
	}
	if !firstInfo.IsDir() || !secondInfo.IsDir() {
		t.Fatalf("physical directory comparison requires directories: %q, %q", first, second)
	}
	return os.SameFile(firstInfo, secondInfo)
}

func TestPhysicalDirectoryIdentityDistinguishesAliasesAndWrongParents(t *testing.T) {
	t.Parallel()
	root, nested, _ := physicalTraversalFixture(t)
	alias := filepath.Join(root, "directory-alias")
	if err := os.Symlink(nested, alias); err != nil {
		t.Fatal(err)
	}
	if nested == alias {
		t.Fatal("fixture must use distinct directory spellings")
	}
	if !samePhysicalDirectory(t, nested, alias) {
		t.Error("different spellings of the same physical directory were rejected")
	}
	if samePhysicalDirectory(t, root, alias) {
		t.Error("different physical parent directories were treated as identical")
	}
}

// Inject just native spelling expansion, after real component traversal. Linux
// has no Windows 8.3 aliases; the Windows test below exercises the real API.
func TestMissingOutputAliasCanonicalizesExistingPrefix(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	long := filepath.Join(root, "LongDirectoryName")
	short := filepath.Join(root, "LONGDI~1")
	other := filepath.Join(root, "OtherDirectory")
	for _, path := range []string{long, short, other} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	ops := defaultFileOps()
	calls := 0
	ops.evalSymlinks = func(path string) (string, error) {
		calls++
		if path == short {
			return long, nil
		}
		return filepath.EvalSymlinks(path)
	}
	for _, suffix := range []string{"report.json", filepath.Join("missing", "nested", "report.json")} {
		output := filepath.Join(short, suffix)
		protected := filepath.Join(long, suffix)
		if err := rejectAlias(output, []string{protected}, ops); err == nil || !strings.Contains(err.Error(), "protected input") {
			t.Fatalf("missing-final spelling alias accepted: %v", err)
		}
		if err := rejectAlias(output, []string{filepath.Join(other, suffix)}, ops); err != nil {
			t.Fatalf("different directory rejected: %v", err)
		}
		if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("output changed: %v", err)
		}
	}
	if calls == 0 {
		t.Fatal("native spelling normalizer was not used")
	}
}

func TestExistingPrefixNormalizationFollowsPhysicalTraversal(t *testing.T) {
	t.Parallel()
	_, nested, traversed := physicalTraversalFixture(t)
	ops := defaultFileOps()
	calls := 0
	ops.evalSymlinks = func(path string) (string, error) {
		calls++
		for _, component := range strings.Split(filepath.ToSlash(path), "/") {
			if component == ".." {
				return "", errors.New("raw parent traversal reached native normalization")
			}
		}
		return filepath.EvalSymlinks(path)
	}
	suffix := filepath.Join("missing", "nested", "report.json")
	got, err := checkedResolve(traversed+string(filepath.Separator)+suffix, ops)
	if err != nil {
		t.Fatal(err)
	}
	prefix, err := filepath.EvalSymlinks(nested)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(prefix, suffix); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if calls == 0 {
		t.Fatal("native spelling normalizer was not used")
	}
}

func TestExistingPrefixNormalizationFailsClosed(t *testing.T) {
	t.Parallel()
	failure := errors.New("cannot canonicalize existing prefix")
	ops := defaultFileOps()
	ops.evalSymlinks = func(string) (string, error) { return "", failure }
	path := filepath.Join(t.TempDir(), "missing", "report.json")
	if _, err := checkedResolve(path, ops); !errors.Is(err, failure) {
		t.Fatalf("canonicalization error ignored: %v", err)
	}
	if err := rejectAlias(path, nil, ops); !errors.Is(err, failure) {
		t.Fatalf("output accepted after canonicalization error: %v", err)
	}
}
