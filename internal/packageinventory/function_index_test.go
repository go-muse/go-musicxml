package packageinventory

import (
	"testing"
)

func TestFunctionIndexMatchesDirectMatcher(t *testing.T) {
	t.Parallel()
	samples := []string{"func TestOne(t *testing.T) {}\nfunc TestTwo() {}\n", "func\n TestSplit \n(\n){}", "func TestGeneric[T any](){}\nfunc (r Receiver) TestMethod(){}", "// func TestComment() {}\n func TestIndented() {}\nfunc TestUnicodeé(){}", "func \tTestSpace() {}\nfunc A$B() {}\nfunc TestSuffixLong() {}"}
	names := []string{"TestOne", "TestTwo", "TestSplit", "TestGeneric", "TestMethod", "TestComment", "TestIndented", "TestUnicodeé", "TestSpace", " TestSpace", "A$B", "TestSuffix", "TestSuffixLong", "", "TestSpace() {}\nfunc A$B"}
	for _, source := range samples {
		index := inspectTestFunctions([]byte(source))
		for _, name := range names {
			want := hasFunction([]byte(source), name)
			if got := index.contains(name); got != want {
				t.Errorf("source=%q name=%q indexed=%t direct=%t", source, name, got, want)
			}
		}
	}
}

func TestFunctionIndexHashIsFreshAndCRLFOnly(t *testing.T) {
	t.Parallel()
	lf := []byte("func TestOne() {}\n")
	crlf := []byte("func TestOne() {}\r\n")
	first := inspectTestFunctions(lf)
	if first.hash != inspectTestFunctions(crlf).hash {
		t.Fatal("CRLF changed indexed hash")
	}
	lf[5] = 'X'
	next := inspectTestFunctions(lf)
	if next.hash == first.hash || next.contains("TestOne") {
		t.Fatal("new invocation reused stale file observations")
	}
	if inspectTestFunctions(append(crlf, '\r')).hash == first.hash {
		t.Fatal("lone CR normalized")
	}
}
