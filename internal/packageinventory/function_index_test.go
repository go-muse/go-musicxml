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

func TestFunctionMatchersUseDocumentedASCIIWhitespace(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, whitespace string
		want             bool
	}{
		{"space", " ", true},
		{"tab", "\t", true},
		{"newline", "\n", true},
		{"form-feed", "\f", true},
		{"carriage-return", "\r", true},
		{"vertical-tab", "\v", false},
		{"file-separator", "\u001c", false},
		{"next-line", "\u0085", false},
		{"no-break-space", "\u00a0", false},
		{"em-space", "\u2003", false},
		{"line-separator", "\u2028", false},
		{"ideographic-space", "\u3000", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, source := range []string{"func" + tc.whitespace + "TestWhitespace() {}", "func TestWhitespace" + tc.whitespace + "() {}"} {
				data := []byte(source)
				if got := hasFunction(data, "TestWhitespace"); got != tc.want {
					t.Errorf("direct matcher source=%q matched=%t want=%t", source, got, tc.want)
				}
				if got := inspectTestFunctions(data).contains("TestWhitespace"); got != tc.want {
					t.Errorf("indexed matcher source=%q matched=%t want=%t", source, got, tc.want)
				}
			}
		})
	}
}
