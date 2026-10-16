package validation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestIPARepresentationCompiledPolicyMatchesPinnedData(t *testing.T) {
	raw, err := os.ReadFile("../testdata/dictionaries/ipa-representation/representation-policy.json")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != ipaRepresentationPolicySHA256 {
		t.Fatal("policy file no longer matches compiled/transformed pin")
	}
	var policy struct {
		Revision string            `json:"policy_revision"`
		CCC      []json.RawMessage `json:"nonzero_ccc"`
		Pairs    [][3]string       `json:"half_pairs"`
		Roles    []string          `json:"base_roles"`
		Mark     string            `json:"independent_mark_role"`
	}
	if err := json.Unmarshal(raw, &policy); err != nil {
		t.Fatal(err)
	}
	manifest, _ := LookupDictionaryManifest(DictionaryManifestIPARepresentation2015Unicode13)
	if policy.Revision != manifest.PolicyRevision || len(policy.CCC) != 53 || len(ipaRepresentationCCCValues) != 53 || len(policy.Pairs) != len(ipaRepresentationHalfPairs) || len(ipaRepresentationHalfPairs) != 2 || policy.Mark != "mark" || !slices.Equal(policy.Roles, []string{"letter", "modifier-letter", "constructed-letter"}) {
		t.Fatal("sealed policy schema changed")
	}
	cp := func(s string) rune {
		v, e := strconv.ParseInt(strings.TrimPrefix(s, "U+"), 16, 32)
		if e != nil {
			t.Fatal(e)
		}
		return rune(v)
	}
	for i, row := range policy.CCC {
		var pair []json.RawMessage
		if err := json.Unmarshal(row, &pair); err != nil {
			t.Fatal(err)
		}
		var code string
		var ccc uint8
		json.Unmarshal(pair[0], &code)
		json.Unmarshal(pair[1], &ccc)
		if ipaRepresentationCCCValues[i].scalar != cp(code) || ipaRepresentationCCCValues[i].ccc != ccc {
			t.Fatalf("compiled CCC differs from bound policy row%d", i)
		}
	}
	d, a := ipaRepresentationSet(t)
	p := d.entries[0].representation
	for i, pair := range policy.Pairs {
		left, right, whole := cp(pair[0]), cp(pair[1]), cp(pair[2])
		compiled := ipaRepresentationHalfPairs[i]
		if compiled.left != left || compiled.right != right || compiled.whole != whole {
			t.Fatalf("compiled half pair differs from bound policy row%d", i)
		}
		if !ipaRepresentationHalf(left) || !ipaRepresentationHalf(right) || ipaRepresentationHalf(whole) {
			t.Fatal("half production identity differs from pinned policy")
		}
		for _, text := range []string{"t" + string(left) + "s" + string(right), "t\u0301" + string(left) + "s\u0300" + string(right)} {
			if ok, err := ipaRepresentationWitness(context.Background(), a, p, text); err != nil || !ok {
				t.Fatalf("pinned pair %q: %v %v", text, ok, err)
			}
		}
		for _, text := range []string{"t" + string(right) + "s" + string(left), "t" + string(left) + string(left) + "s" + string(right) + string(right), "t" + string(whole) + string(left) + "s" + string(right), "t" + string(left) + "s" + string(right) + string(whole), "a" + string(whole) + "t" + string(left) + "s" + string(right)} {
			if ok, err := ipaRepresentationWitness(context.Background(), a, p, text); err != nil || ok {
				t.Fatalf("unproved pair %q: %v %v", text, ok, err)
			}
		}
	}
	decomposable := 0
	for _, row := range p.scalars {
		if row.count != 1 || row.leaves[0] != row.scalar {
			decomposable++
			if row.ccc != 0 {
				t.Fatalf("raw decomposition key U+%04X incorrectly stores a leaf CCC", row.scalar)
			}
			continue
		}
		want, err := ipaRepresentationCCC(context.Background(), a, row.scalar)
		if err != nil || row.ccc != want {
			t.Fatalf("leaf U+%04X class=%d want%d: %v", row.scalar, row.ccc, want, err)
		}
	}
	if decomposable != 158 {
		t.Fatalf("decomposition support=%d", decomposable)
	}
	marks, bases, signs := 0, 0, 0
	for _, row := range d.entries[0].ipa.Symbols {
		switch row.Category {
		case "mark":
			marks++
		case "letter", "modifier-letter", "constructed-letter":
			bases++
		case "sign":
			signs++
		default:
			t.Fatal("unbound role")
		}
	}
	if marks != 47 || bases != 221 || signs != 27 || p.scalarCount != 436 || p.atomCount != 248 {
		t.Fatalf("roles/index=%d %d %d/%d %d", marks, bases, signs, p.scalarCount, p.atomCount)
	}
	for _, atom := range p.atoms {
		if atom.count > 2 {
			t.Fatalf("reviewed atom exceeds one normalized mark: %+v", atom)
		}
	}
	maxCandidates, starters := 0, 0
	for i := 0; i < len(p.atoms); {
		j := i + 1
		for j < len(p.atoms) && p.atoms[j].leaves[0] == p.atoms[i].leaves[0] {
			j++
		}
		maxCandidates = max(maxCandidates, j-i)
		starters++
		i = j
	}
	if maxCandidates != 2 || starters != 225 {
		t.Fatalf("derived candidates=%d starters=%d", maxCandidates, starters)
	}
}

// This oracle enumerates complete-atom concatenations, then independently
// normalizes them using adjacent swaps and the sealed finite source facts. It
// does not share the segment candidate algorithm or a host Unicode version.
func TestIPARepresentationFiniteConcatenationOracle(t *testing.T) {
	d, _ := ipaRepresentationSet(t)
	p := d.entries[0].representation
	_, input, _ := ipaRepresentationFixture(t)
	mapping := map[rune][]rune{}
	for _, row := range input.IPA.Equivalences {
		mapping[[]rune(row.Encoding)[0]] = []rune(row.EquivalentToEncoding)
	}
	classes := map[rune]uint8{}
	for _, row := range ipaRepresentationCCCValues {
		classes[row.scalar] = row.ccc
	}
	nfd := func(text string) string {
		var rs []rune
		for _, r := range text {
			if ds, ok := mapping[r]; ok {
				rs = append(rs, ds...)
			} else {
				rs = append(rs, r)
			}
		}
		for i := 1; i < len(rs); i++ {
			for j := i; j > 0 && classes[rs[j]] != 0 && classes[rs[j-1]] > classes[rs[j]]; j-- {
				rs[j-1], rs[j] = rs[j], rs[j-1]
			}
		}
		return string(rs)
	}
	positive := map[string]bool{}
	atoms := []string{"a", "c", "ç", "d", "d\u0322", "\u0334", "\u0301"}
	var build func(string, int)
	build = func(s string, depth int) {
		positive[nfd(s)] = true
		if depth == 4 {
			return
		}
		for _, atom := range atoms {
			build(s+atom, depth+1)
		}
	}
	build("", 0)
	alphabet := []rune{'a', 'c', 'd', '\u0327', '\u0322', '\u0334', '\u0301'}
	count := 0
	var check func(string, int)
	check = func(s string, depth int) {
		a := dictionaryCoreAccount(t)
		got, err := ipaRepresentationWitness(context.Background(), a, p, s)
		if err != nil || got != positive[nfd(s)] {
			t.Fatalf("oracle %q: got%v err%v want%v", s, got, err, positive[nfd(s)])
		}
		if dictionaryCoreUsage(t, a) != 17 {
			t.Fatal("oracle scratch leak")
		}
		count++
		if depth == 4 {
			return
		}
		for _, r := range alphabet {
			check(s+string(r), depth+1)
		}
	}
	check("", 0)
	if count != 2801 {
		t.Fatalf("oracle cases=%d", count)
	}
}
