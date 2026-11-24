package musicxml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// differs is a difference at one path: of both documents, or of the one that
// has the item.
func differs(path string, kind DifferenceKind, a, b string) Difference {
	switch kind {
	case DifferenceOnlyInA:
		return Difference{PathA: path, Kind: kind}
	case DifferenceOnlyInB:
		return Difference{PathB: path, Kind: kind}
	}
	return Difference{PathA: path, PathB: path, Kind: kind, A: a, B: b}
}

// Compare says where documents differ: the paths are those of the XML the
// documents stand for, and the values are the two documents' own.
func TestComparePaths(t *testing.T) {
	base := equalScore(equalAttributes + equalNote)
	note := func(step string) string {
		return strings.Replace(equalNote, "<step>C</step>", "<step>"+step+"</step>", 1)
	}
	withTitle := strings.Replace(base, "<part-list>", "<movement-title>T</movement-title><part-list>", 1)
	const measure = "/score-partwise/part[1]/measure[1]"
	for _, tc := range []struct {
		name string
		a, b string
		want []Difference
	}{
		{"a pitch", base, strings.Replace(base, "<step>C</step>", "<step>D</step>", 1),
			[]Difference{differs(measure+"/note[1]/pitch/step", DifferenceValue, "C", "D")}},
		{"a duration", base, strings.Replace(base, "<duration>4</duration>", "<duration>8</duration>", 1),
			[]Difference{differs(measure+"/note[1]/duration", DifferenceValue, "4", "8")}},
		// Numbers are written as Encode writes them, not in exponent form.
		{"a large and a small number", strings.Replace(base, "<duration>4</duration>", "<duration>1000000</duration>", 1), strings.Replace(base, "<duration>4</duration>", "<duration>0.00001</duration>", 1),
			[]Difference{differs(measure+"/note[1]/duration", DifferenceValue, "1000000", "0.00001")}},
		{"2/4 and 4/8", base, strings.Replace(strings.Replace(base, "<beats>2</beats>", "<beats>4</beats>", 1), "<beat-type>4</beat-type>", "<beat-type>8</beat-type>", 1),
			[]Difference{
				differs(measure+"/attributes[1]/time[1]/beats[1]", DifferenceValue, "2", "4"),
				differs(measure+"/attributes[1]/time[1]/beat-type[1]", DifferenceValue, "4", "8"),
			}},
		{"the version attribute", base, strings.Replace(base, `version="4.0"`, `version="3.1"`, 1),
			[]Difference{differs("/score-partwise/@version", DifferenceValue, "4.0", "3.1")}},
		{"an attribute of a measure", base, strings.Replace(base, `number="1"`, `number="7"`, 1),
			[]Difference{differs(measure+"/@number", DifferenceValue, "1", "7")}},
		// An element's attributes come before its content.
		{"attributes and content", base, strings.Replace(strings.Replace(strings.Replace(base, `version="4.0"`, `version="3.1"`, 1), `number="1"`, `number="7"`, 1), "<step>C</step>", "<step>D</step>", 1),
			[]Difference{
				differs("/score-partwise/@version", DifferenceValue, "4.0", "3.1"),
				differs(measure+"/@number", DifferenceValue, "1", "7"),
				differs(measure+"/note[1]/pitch/step", DifferenceValue, "C", "D"),
			}},
		{"the part list", base, strings.Replace(base, "<part-name>A</part-name>", "<part-name>B</part-name>", 1),
			[]Difference{differs("/score-partwise/part-list/score-part[1]/part-name", DifferenceValue, "A", "B")}},
		{"an optional element only in the second", base, withTitle,
			[]Difference{differs("/score-partwise/movement-title", DifferenceOnlyInB, "", "")}},
		{"an optional element only in the first", withTitle, base,
			[]Difference{differs("/score-partwise/movement-title", DifferenceOnlyInA, "", "")}},
		{"an optional attribute only in the second", base, strings.Replace(base, `<measure number="1">`, `<measure number="1" width="100">`, 1),
			[]Difference{differs(measure+"/@width", DifferenceOnlyInB, "", "")}},
		{"a note more", equalScore(equalAttributes + equalNote), equalScore(equalAttributes + equalNote + note("D")),
			[]Difference{differs(measure+"/note[2]", DifferenceOnlyInB, "", "")}},
		{"a note fewer", equalScore(equalAttributes + equalNote + note("D")), equalScore(equalAttributes + equalNote),
			[]Difference{differs(measure+"/note[2]", DifferenceOnlyInA, "", "")}},
		{"the second note", equalScore(equalAttributes + equalNote + equalNote), equalScore(equalAttributes + equalNote + note("D")),
			[]Difference{differs(measure+"/note[2]/pitch/step", DifferenceValue, "C", "D")}},
		// The children are compared by position: with the attributes gone the
		// note takes their place, and the second child exists only in a.
		{"an element dropped from the front", base, equalScore(equalNote),
			[]Difference{
				{PathA: measure + "/attributes[1]", PathB: measure + "/note[1]", Kind: DifferenceElement, A: "attributes", B: "note"},
				differs(measure+"/note[1]", DifferenceOnlyInA, "", ""),
			}},
		{"an element put in front", equalScore(equalNote), base,
			[]Difference{
				{PathA: measure + "/note[1]", PathB: measure + "/attributes[1]", Kind: DifferenceElement, A: "note", B: "attributes"},
				differs(measure+"/note[1]", DifferenceOnlyInB, "", ""),
			}},
		// After children that differ in name, an element has another number
		// in the second document, and each value is at its own path there.
		{"children shifted", equalScore(equalAttributes + note("C")), equalScore(note("E") + note("D") + note("C")),
			[]Difference{
				{PathA: measure + "/attributes[1]", PathB: measure + "/note[1]", Kind: DifferenceElement, A: "attributes", B: "note"},
				{PathA: measure + "/note[1]/pitch/step", PathB: measure + "/note[2]/pitch/step", Kind: DifferenceValue, A: "C", B: "D"},
				differs(measure+"/note[3]", DifferenceOnlyInB, "", ""),
			}},
		// Under a parent with another number in the second document, an item
		// only one of them has is at that document's own path.
		{"an element only in the second, after a shift", equalScore(equalAttributes + note("C")), equalScore(note("C") + strings.Replace(note("C"), "<duration>", "<voice>1</voice><duration>", 1)),
			[]Difference{
				{PathA: measure + "/attributes[1]", PathB: measure + "/note[1]", Kind: DifferenceElement, A: "attributes", B: "note"},
				differs(measure+"/note[2]/voice", DifferenceOnlyInB, "", ""),
			}},
		{"a partwise score and a timewise one", base, `<?xml version="1.0"?><score-timewise version="4.0"><part-list><score-part id="P1"><part-name>A</part-name></score-part></part-list></score-timewise>`,
			[]Difference{differs("/", DifferenceElement, "score-partwise", "score-timewise")}},
		{"equal", base, strings.ReplaceAll(base, "><", ">\n<"), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := equalDecode(t, tc.a), equalDecode(t, tc.b)
			got := Compare(a, b, CompareOptions{MaxDifferences: -1})
			if !reflect.DeepEqual(got.Differences, tc.want) || got.Truncated {
				t.Errorf("Compare = %+v, want %+v", got.Differences, tc.want)
			}
			// The other way round, every difference is the mirror of one.
			var mirrored []Difference
			for _, d := range tc.want {
				mirrored = append(mirrored, mirror(d))
			}
			if got := Compare(b, a, CompareOptions{MaxDifferences: -1}); !reflect.DeepEqual(got.Differences, mirrored) {
				t.Errorf("reversed: Compare = %+v, want %+v", got.Differences, mirrored)
			}
			if equal := Equal(a, b); equal != (len(tc.want) == 0) {
				t.Errorf("Equal = %v with %d differences", equal, len(tc.want))
			}
		})
	}
}

// mirror is the difference Compare reports with its arguments exchanged.
func mirror(d Difference) Difference {
	d.PathA, d.PathB, d.A, d.B = d.PathB, d.PathA, d.B, d.A
	switch d.Kind {
	case DifferenceOnlyInA:
		d.Kind = DifferenceOnlyInB
	case DifferenceOnlyInB:
		d.Kind = DifferenceOnlyInA
	}
	return d
}

func TestComparePathsOfConstructedDocuments(t *testing.T) {
	all := CompareOptions{MaxDifferences: -1}
	one := NewScorePartwise(PartDefinition{ID: "P1", Name: "A"})
	two := NewScorePartwise(PartDefinition{ID: "P1", Name: "A"}, PartDefinition{ID: "P2", Name: "B"})
	got := Compare(one, two, all).Differences
	want := []Difference{
		differs("/score-partwise/part-list/score-part[2]", DifferenceOnlyInB, "", ""),
		differs("/score-partwise/part[2]", DifferenceOnlyInB, "", ""),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Compare = %+v, want %+v", got, want)
	}
	// Names carry the namespace prefixes the schema uses, and the elements
	// an opus holds are told apart by name.
	link := func(hrefs ...string) *OpusDocument {
		opus := NewOpusDocument()
		for _, href := range hrefs {
			opus.AddOpusLink(&OpusLink{Href: href})
		}
		return opus
	}
	got = Compare(link("a.musicxml"), link("b.musicxml"), all).Differences
	want = []Difference{differs("/opus/opus-link[1]/@xlink:href", DifferenceValue, "a.musicxml", "b.musicxml")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Compare = %+v, want %+v", got, want)
	}
	score := NewOpusDocument()
	score.Content = append(score.Content, OpusDocumentContent{Score: &OpusScore{Href: "a.musicxml"}})
	got = Compare(link("a.musicxml"), score, all).Differences
	want = []Difference{{PathA: "/opus/opus-link[1]", PathB: "/opus/score[1]", Kind: DifferenceElement, A: "opus-link", B: "score"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Compare = %+v, want %+v", got, want)
	}
	// A child that holds no element, which only code can build, is told by
	// its position among the ordered children, not among the nameless ones.
	holed := link("a.musicxml")
	holed.Content = append(holed.Content, OpusDocumentContent{}, OpusDocumentContent{})
	got = Compare(holed, link("a.musicxml", "b.musicxml"), all).Differences
	want = []Difference{
		{PathA: "/opus/*[2]", PathB: "/opus/opus-link[2]", Kind: DifferenceElement, A: "", B: "opus-link"},
		differs("/opus/*[3]", DifferenceOnlyInA, "", ""),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Compare = %+v, want %+v", got, want)
	}
	// Children that hold no element, or two, are compared as what they hold.
	both := func(href string) *OpusDocument {
		opus := NewOpusDocument()
		opus.Content = append(opus.Content, OpusDocumentContent{OpusLink: &OpusLink{Href: href}, Score: &OpusScore{Href: href}})
		return opus
	}
	none := NewOpusDocument()
	none.Content = append(none.Content, OpusDocumentContent{})
	if !Equal(both("a"), both("a")) || Equal(both("a"), both("b")) || Equal(none, both("a")) || Equal(both("a"), none) {
		t.Error("children with no element or with two are not compared by what they hold")
	}
	// An opus nested in an opus is compared like any element, and its
	// XMLName is no more content than a root's.
	nested := func(title string) *OpusDocument {
		outer := NewOpusDocument()
		outer.AddOpus(&OpusDocument{Title: Ptr(title), XMLName: xml.Name{Local: title}})
		return outer
	}
	got = Compare(nested("a"), nested("b"), all).Differences
	want = []Difference{differs("/opus/opus[1]/title", DifferenceValue, "a", "b")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Compare = %+v, want %+v", got, want)
	}
}

func TestComparePathNames(t *testing.T) {
	for tag, want := range map[string]string{
		"id": "id",
		"http://www.w3.org/XML/1998/namespace lang": "xml:lang",
		"http://www.w3.org/1999/xlink href":         "xlink:href",
		"urn:example name":                          "{urn:example}name",
		"score-partwise":                            "score-partwise",
	} {
		if got := pathName(tag); got != want {
			t.Errorf("pathName(%q) = %q, want %q", tag, got, want)
		}
	}
	// A list's items are the nth of the name of the field that holds it.
	type group struct {
		Items []string `xml:"item"`
	}
	c := &comparison{limit: -1, path: []segment{{nameA: "root", nameB: "root"}}}
	c.walk(reflect.ValueOf(group{Items: []string{"a", "b"}}), reflect.ValueOf(group{Items: []string{"a", "c", "d"}}))
	want := []Difference{differs("/root/item[2]", DifferenceValue, "b", "c"), differs("/root/item[3]", DifferenceOnlyInB, "", "")}
	if !reflect.DeepEqual(c.found, want) {
		t.Errorf("found %+v, want %+v", c.found, want)
	}
}

func TestDifferenceString(t *testing.T) {
	for want, d := range map[string]Difference{
		`/a/b: "C" != "D"`:                            differs("/a/b", DifferenceValue, "C", "D"),
		`/a/b[1] (in b: /a/b[2]): "C" != "D"`:         {PathA: "/a/b[1]", PathB: "/a/b[2]", Kind: DifferenceValue, A: "C", B: "D"},
		`/a/x[1] (in b: /a/y[1]): element <x> != <y>`: {PathA: "/a/x[1]", PathB: "/a/y[1]", Kind: DifferenceElement, A: "x", B: "y"},
		`/a/b: only in a`:                             differs("/a/b", DifferenceOnlyInA, "", ""),
		`/a/b: only in b`:                             differs("/a/b", DifferenceOnlyInB, "", ""),
		`/: not comparable: a is cyclic`:              differs("/", DifferenceNotComparable, "cyclic", ""),
		`/: not comparable: a is cyclic, b is cyclic`: differs("/", DifferenceNotComparable, "cyclic", "cyclic"),
		`/: not comparable: b is nested too deeply`:   differs("/", DifferenceNotComparable, "", "nested too deeply"),
		`/a: other`: {PathA: "/a", PathB: "/a", Kind: "other"},
	} {
		if got := d.String(); got != want {
			t.Errorf("%+v is written %q, want %q", d, got, want)
		}
	}
}

// The limit is the caller's: Compare reports that many differences, says
// whether there were more, and stops looking once it knows.
func TestCompareLimit(t *testing.T) {
	const notes = 150
	secondNote := strings.Replace(equalNote, "<step>C</step>", "<step>D</step>", 1)
	a := equalDecode(t, equalScore(equalAttributes+strings.Repeat(equalNote, notes)))
	b := equalDecode(t, equalScore(equalAttributes+strings.Repeat(secondNote, notes)))
	for _, tc := range []struct {
		name      string
		limit     int
		want      int
		truncated bool
	}{
		{"the default", 0, DefaultMaxDifferences, true},
		{"one", 1, 1, true},
		{"three", 3, 3, true},
		{"one fewer than there are", notes - 1, notes - 1, true},
		{"as many as there are", notes, notes, false},
		{"more than there are", notes + 1, notes, false},
		{"the largest", math.MaxInt, notes, false},
		{"none", -1, notes, false},
		{"none, however negative", math.MinInt, notes, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Compare(a, b, CompareOptions{MaxDifferences: tc.limit})
			if len(got.Differences) != tc.want || got.Truncated != tc.truncated {
				t.Fatalf("%d differences, truncated %v; want %d, %v", len(got.Differences), got.Truncated, tc.want, tc.truncated)
			}
			// The first ones in order, however many are asked for.
			for i, difference := range got.Differences {
				if want := "/score-partwise/part[1]/measure[1]/note[" + strconv.Itoa(i+1) + "]/pitch/step"; difference.PathA != want {
					t.Fatalf("difference %d is at %s, want %s", i, difference.PathA, want)
				}
			}
			if cap(got.Differences) != len(got.Differences) {
				t.Errorf("the list has room for %d more, which an append would overwrite", cap(got.Differences)-len(got.Differences))
			}
		})
	}
	// Items that only one document has stop at the limit too, in a list
	// and among ordered children.
	parts := make([]PartDefinition, 10)
	for i := range parts {
		parts[i] = PartDefinition{ID: "P" + strconv.Itoa(i), Name: "N"}
	}
	withParts, withoutBodies, withoutList := NewScorePartwise(parts...), NewScorePartwise(parts...), NewScorePartwise(parts...)
	withoutBodies.Part = nil
	withoutList.PartList.Content = nil
	for name, other := range map[string]Document{"a list": withoutBodies, "ordered children": withoutList} {
		for _, pair := range [][2]Document{{withParts, other}, {other, withParts}} {
			if c := compareDocuments(pair[0], pair[1], 2); len(c.found) != 3 {
				t.Errorf("%s: %d differences found at a limit of 2", name, len(c.found))
			}
			if c := compareDocuments(pair[0], pair[1], -1); len(c.found) != 10 {
				t.Errorf("%s: %d differences, want 10", name, len(c.found))
			}
		}
	}
	// So do the values of one element: its attributes and its content.
	plain := equalScore(equalAttributes + equalNote)
	other := strings.Replace(strings.Replace(plain, `<measure number="1">`, `<measure number="2" width="5">`, 1), "<duration>4</duration><type>whole</type>", "<duration>2</duration><type>half</type>", 1)
	for limit, want := range map[int]int{0: 1, 1: 2, 2: 3, 3: 4, 4: 4, -1: 4} {
		if c := compareDocuments(equalDecode(t, plain), equalDecode(t, other), limit); len(c.found) != want {
			t.Errorf("values of one element: %d differences found at a limit of %d, want %d", len(c.found), limit, want)
		}
	}
	for _, limit := range []int{0, 1, 5, -1, notes} {
		if got := Compare(a, a, CompareOptions{MaxDifferences: limit}); len(got.Differences) != 0 || got.Truncated {
			t.Errorf("limit %d: equal documents differ: %+v", limit, got)
		}
	}
	if Equal(a, b) || !Equal(a, a) {
		t.Error("Equal disagrees with Compare")
	}
}

// Compare stops looking at one difference more than the limit, and Equal at
// the first: with the documents differing from their first note on, what is
// compared is a small part of what comparing them whole takes. What the two
// documents share is not compared at all.
func TestCompareStopsEarly(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "official", "MozartTrio.musicxml"))
	if err != nil {
		t.Fatal(err)
	}
	a := equalDecode(t, string(data))
	same := equalDecode(t, string(data))
	b := equalDecode(t, strings.ReplaceAll(string(data), "<octave>4</octave>", "<octave>5</octave>"))
	whole := compareDocuments(a, same, -1)
	if len(whole.found) != 0 || whole.visited < 5000 {
		t.Fatalf("comparing equal documents found %d differences over %d values", len(whole.found), whole.visited)
	}
	every := compareDocuments(a, b, -1)
	if len(every.found) < 60 || every.visited < whole.visited/2 {
		t.Fatalf("%d differences over %d values", len(every.found), every.visited)
	}
	previous := 0
	for _, limit := range []int{0, 1, 10, 50} {
		c := compareDocuments(a, b, limit)
		if len(c.found) != limit+1 {
			t.Errorf("limit %d: %d differences found, want %d", limit, len(c.found), limit+1)
		}
		if c.visited <= previous || c.visited >= every.visited {
			t.Errorf("limit %d: %d values compared, %d for the limit before and %d for every difference", limit, c.visited, previous, every.visited)
		}
		previous = c.visited
	}
	if first := compareDocuments(a, b, 0); first.visited > whole.visited/10 {
		t.Errorf("Equal compared %d of %d values to find the first difference", first.visited, whole.visited)
	}
	// A value both documents point at is the same value.
	shared := *same.(*ScorePartwise)
	shared.Identification = a.(*ScorePartwise).Identification
	if c := compareDocuments(a, &shared, -1); len(c.found) != 0 || c.visited >= whole.visited {
		t.Errorf("with a shared element: %d differences over %d values, %d without", len(c.found), c.visited, whole.visited)
	}
}

type embeddedDocument struct {
	*ScorePartwise
	notComparable []int
}

func TestCompareNilAndRefusedDocuments(t *testing.T) {
	document := equalDecode(t, equalScore(equalAttributes+equalNote))
	score := document.(*ScorePartwise)
	var nilPartwise *ScorePartwise
	var nilOpus *OpusDocument
	cyclic := NewOpusDocument()
	cyclic.Content = append(cyclic.Content, OpusDocumentContent{Opus: cyclic})
	deep := NewOpusDocument()
	for current, i := deep, 0; i < maximumDocumentDepth+8; i++ {
		current = current.AddOpus(NewOpusDocument())
	}
	// A type that embeds a root is a Document, and no root: Encode refuses it.
	embedded := embeddedDocument{ScorePartwise: score}
	embeddedPointer := &embedded
	if err := Encode(io.Discard, embedded); !errors.Is(err, ErrUnsupportedDocument) {
		t.Fatalf("Encode of an embedding type: %v", err)
	}
	const unsupported = "unsupported document type"
	for name, tc := range map[string]struct {
		a, b Document
		want []Difference
	}{
		"nil and nil":                         {nil, nilPartwise, nil},
		"nil pointers of two root types":      {nilOpus, nilPartwise, nil},
		"a document and none":                 {document, nil, []Difference{differs("/", DifferenceOnlyInA, "", "")}},
		"none and a document":                 {nilPartwise, document, []Difference{differs("/", DifferenceOnlyInB, "", "")}},
		"the same document":                   {document, document, nil},
		"a cyclic opus":                       {cyclic, NewOpusDocument(), []Difference{differs("/", DifferenceNotComparable, "cyclic", "")}},
		"a cyclic opus, reversed":             {NewOpusDocument(), cyclic, []Difference{differs("/", DifferenceNotComparable, "", "cyclic")}},
		"an opus that is too deep":            {deep, NewOpusDocument(), []Difference{differs("/", DifferenceNotComparable, "nested too deeply", "")}},
		"a cyclic opus and a deep one":        {cyclic, deep, []Difference{differs("/", DifferenceNotComparable, "cyclic", "nested too deeply")}},
		"a cyclic opus with itself":           {cyclic, cyclic, nil},
		"a deep opus with itself":             {deep, deep, nil},
		"a cyclic opus and none":              {cyclic, nil, []Difference{differs("/", DifferenceOnlyInA, "", "")}},
		"none and a cyclic opus":              {nilOpus, cyclic, []Difference{differs("/", DifferenceOnlyInB, "", "")}},
		"a cyclic opus and a score":           {cyclic, document, []Difference{differs("/", DifferenceNotComparable, "cyclic", "")}},
		"a score and a cyclic opus":           {document, cyclic, []Difference{differs("/", DifferenceNotComparable, "", "cyclic")}},
		"an embedding type":                   {embedded, document, []Difference{differs("/", DifferenceNotComparable, unsupported, "")}},
		"an embedding type, reversed":         {document, embeddedPointer, []Difference{differs("/", DifferenceNotComparable, "", unsupported)}},
		"an embedding type with itself":       {embedded, embedded, []Difference{differs("/", DifferenceNotComparable, unsupported, unsupported)}},
		"an embedding type and none":          {embeddedPointer, nil, []Difference{differs("/", DifferenceOnlyInA, "", "")}},
		"a nil pointer to an embedding type":  {(*embeddedDocument)(nil), nil, []Difference{differs("/", DifferenceOnlyInA, "", "")}},
		"an embedding type and a cyclic opus": {embedded, cyclic, []Difference{differs("/", DifferenceNotComparable, unsupported, "cyclic")}},
	} {
		t.Run(name, func(t *testing.T) {
			got := Compare(tc.a, tc.b, CompareOptions{}).Differences
			if len(got) != len(tc.want) || len(got) != 0 && !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Compare = %+v, want %+v", got, tc.want)
			}
			if Equal(tc.a, tc.b) != (len(tc.want) == 0) {
				t.Errorf("Equal = %v with %d differences", Equal(tc.a, tc.b), len(tc.want))
			}
		})
	}
}

// opusRefusal refuses exactly the documents Encode refuses, and for the same
// reason, unless the opus is both cyclic and too deep: then the nesting
// check, which looks at depth first, may say too deep where opusRefusal has
// met the cycle.
func TestOpusRefusalMatchesTheNestingCheck(t *testing.T) {
	chain := func(depth int, leaf func(*OpusDocument)) *OpusDocument {
		root := NewOpusDocument()
		current := root
		for range depth - 1 {
			current = current.AddOpus(NewOpusDocument())
		}
		if leaf != nil {
			leaf(current)
		}
		return root
	}
	leaves := map[string]func(*OpusDocument){
		"nothing": nil,
		"a title": func(o *OpusDocument) { o.Title = Ptr("t") },
		"a link":  func(o *OpusDocument) { o.AddOpusLink(&OpusLink{Href: "x"}) },
		"a score": func(o *OpusDocument) {
			o.Content = append(o.Content, OpusDocumentContent{Score: &OpusScore{Href: "x"}})
		},
		"a hole": func(o *OpusDocument) { o.Content = append(o.Content, OpusDocumentContent{}) },
	}
	cases := map[string]*OpusDocument{}
	for _, depth := range []int{1, 2, maximumDocumentDepth - 1, maximumDocumentDepth, maximumDocumentDepth + 1, maximumDocumentDepth + 2} {
		for name, leaf := range leaves {
			cases["a chain of "+strconv.Itoa(depth)+" ending in "+name] = chain(depth, leaf)
		}
	}
	self := NewOpusDocument()
	self.AddOpus(self)
	cases["an opus in itself"] = self
	far := chain(40, nil)
	end := far
	for len(end.Content) != 0 {
		end = end.Content[0].Opus
	}
	end.AddOpus(far.Content[0].Opus)
	cases["a cycle below the root"] = far
	// A diamond is not a cycle: the shared opus is nested twice.
	diamond := NewOpusDocument()
	shared := chain(3, leaves["a title"])
	diamond.AddOpus(NewOpusDocument()).AddOpus(shared)
	diamond.AddOpus(shared)
	cases["a diamond"] = diamond
	deepDiamond := NewOpusDocument()
	deepShared := chain(maximumDocumentDepth-1, leaves["a title"])
	deepDiamond.AddOpus(NewOpusDocument())
	deepDiamond.AddOpus(deepShared)
	cases["a diamond whose second branch is too deep"] = deepDiamond
	// Too deep is found on the way down, before a cycle further down.
	endless := chain(maximumDocumentDepth+40, nil)
	end = endless
	for len(end.Content) != 0 {
		end = end.Content[0].Opus
	}
	end.AddOpus(endless)
	cases["a chain too deep that ends in a cycle"] = endless
	// An opus nested twice is as deep as the deeper of its two places.
	twice := NewOpusDocument()
	below := chain(maximumDocumentDepth-2, leaves["a title"])
	twice.AddOpus(below)
	twice.AddOpus(NewOpusDocument()).AddOpus(below)
	cases["a shared opus too deep in its second place"] = twice
	allowed := NewOpusDocument()
	allowedBelow := chain(maximumDocumentDepth-3, leaves["a title"])
	allowed.AddOpus(allowedBelow)
	allowed.AddOpus(NewOpusDocument()).AddOpus(allowedBelow)
	cases["a shared opus as deep as allowed in its second place"] = allowed
	// A title counts as a level whatever else the opus holds.
	cases["a title and a hole at the last level"] = chain(maximumDocumentDepth, func(o *OpusDocument) {
		o.Title = Ptr("t")
		o.Content = append(o.Content, OpusDocumentContent{})
	})
	titled := chain(maximumDocumentDepth, leaves["a link"])
	titled.Title = Ptr("t")
	cases["a titled root over a chain too deep by a link"] = titled
	// Both cyclic and too deep: a ring as long as the limit, and a chain
	// too deep by a leaf beside an opus in itself.
	both := map[string]bool{}
	ring := chain(maximumDocumentDepth, nil)
	end = ring
	for len(end.Content) != 0 {
		end = end.Content[0].Opus
	}
	end.AddOpus(ring)
	cases["a ring as long as the limit"], both["a ring as long as the limit"] = ring, true
	beside := NewOpusDocument()
	beside.AddOpus(chain(maximumDocumentDepth-1, leaves["a title"]))
	beside.AddOpus(self)
	cases["a chain too deep by a leaf beside a cycle"], both["a chain too deep by a leaf beside a cycle"] = beside, true
	// Small graphs of every shape: each opus nests up to three of the
	// others, at random, and may have a title, a link or a hole.
	random := rand.New(rand.NewPCG(1, 2))
	for i := range 400 {
		nodes := make([]*OpusDocument, 2+random.IntN(6))
		for j := range nodes {
			nodes[j] = NewOpusDocument()
		}
		for _, node := range nodes {
			for range random.IntN(4) {
				switch k := random.IntN(len(nodes) + 3); {
				case k == len(nodes):
					node.Title = Ptr("t")
				case k == len(nodes)+1:
					node.AddOpusLink(&OpusLink{Href: "x"})
				case k == len(nodes)+2:
					node.Content = append(node.Content, OpusDocumentContent{})
				default:
					node.AddOpus(nodes[k])
				}
			}
		}
		name := "random graph " + strconv.Itoa(i)
		cases[name] = nodes[0]
	}
	refused := 0
	for name, opus := range cases {
		want := ""
		switch err := checkDocumentNesting(opus); {
		case errors.Is(err, ErrDocumentCycle):
			want = "cyclic"
		case errors.Is(err, ErrDocumentTooDeep):
			want = "nested too deeply"
		case err != nil:
			t.Fatalf("%s: %v", name, err)
		}
		if got := opusRefusal(opus); got != want && !(both[name] && got == "cyclic" && want == "nested too deeply") {
			t.Errorf("%s: refused as %q, the nesting check says %q", name, got, want)
		}
		if want != "" {
			refused++
		}
	}
	if refused < 8 || refused > len(cases)-8 {
		t.Errorf("%d of %d documents refused", refused, len(cases))
	}
}

// An opus nested in several places is compared once: two documents that each
// nest one opus twice on every level take as long as their elements, not as
// their paths, of which there are 2 to the power of the depth.
func TestCompareSharedOpus(t *testing.T) {
	build := func(title string) *OpusDocument {
		bottom := &OpusDocument{Title: Ptr(title)}
		current := bottom
		for range 40 {
			parent := NewOpusDocument()
			parent.AddOpus(current)
			parent.AddOpus(current)
			current = parent
		}
		return current
	}
	a, b := build("t"), build("t")
	c := compareDocuments(a, b, -1)
	if len(c.found) != 0 || c.visited > 2000 {
		t.Fatalf("%d differences over %d values", len(c.found), c.visited)
	}
	if !Equal(a, b) {
		t.Error("equal documents differ")
	}
	// An opus found equal to one counterpart is still compared with another:
	// x twice against y, equal to x, and z, which differs.
	x, y, z := &OpusDocument{Title: Ptr("t")}, &OpusDocument{Title: Ptr("t")}, &OpusDocument{Title: Ptr("u")}
	pair := func(first, second *OpusDocument) *OpusDocument {
		outer := NewOpusDocument()
		outer.AddOpus(first)
		outer.AddOpus(second)
		return outer
	}
	want := []Difference{differs("/opus/opus[2]/title", DifferenceValue, "t", "u")}
	if got := Compare(pair(x, x), pair(y, z), CompareOptions{}).Differences; !reflect.DeepEqual(got, want) {
		t.Errorf("x, x against y, z: %+v", got)
	}
	if got := Compare(pair(y, z), pair(x, x), CompareOptions{}).Differences; len(got) != 1 || got[0] != mirror(want[0]) {
		t.Errorf("y, z against x, x: %+v", got)
	}
	// Where they differ, each of the paths is a place, up to the limit.
	other := build("u")
	got := Compare(a, other, CompareOptions{})
	if len(got.Differences) != DefaultMaxDifferences || !got.Truncated || Equal(a, other) {
		t.Fatalf("%d differences, truncated %v", len(got.Differences), got.Truncated)
	}
	if want := "/opus" + strings.Repeat("/opus[1]", 40) + "/title"; got.Differences[0].PathA != want || got.Differences[0].A != "t" || got.Differences[0].B != "u" {
		t.Errorf("the first difference is %v", got.Differences[0])
	}
	if want := "/opus" + strings.Repeat("/opus[1]", 39) + "/opus[2]/title"; got.Differences[1].PathA != want {
		t.Errorf("the second difference is %v", got.Differences[1])
	}
}

// Compare and Equal read their arguments and leave them as they were.
func TestCompareDoesNotModify(t *testing.T) {
	build := func() (Document, Document) {
		a := equalDecode(t, equalScore(equalAttributes+equalNote+equalNote)).(*ScorePartwise)
		b := equalDecode(t, equalScore(equalNote+equalAttributes)).(*ScorePartwise)
		a.Credit, b.Credit = nil, []Credit{}
		a.Part[0].Measure[0].Content = append(a.Part[0].Measure[0].Content, ScorePartwisePartMeasureContent{})
		return a, b
	}
	a, b := build()
	wantA, wantB := build()
	for _, limit := range []int{0, 1, -1} {
		if got := Compare(a, b, CompareOptions{MaxDifferences: limit}); len(got.Differences) == 0 {
			t.Fatal("the documents do not differ")
		}
	}
	if Equal(a, b) || !Equal(a, a) {
		t.Fatal("Equal is wrong")
	}
	if !reflect.DeepEqual(a, wantA) || !reflect.DeepEqual(b, wantB) {
		t.Error("a document was modified")
	}
}

// xmlTree is the XML a document encodes to, read back as elements, their
// attributes and their text, to look paths up in.
type xmlTree struct {
	name     string
	attrs    map[string]string
	children []*xmlTree
	text     strings.Builder
}

func parseXMLTree(t *testing.T, data []byte) *xmlTree {
	t.Helper()
	root := &xmlTree{}
	stack := []*xmlTree{root}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return root
		}
		if err != nil {
			t.Fatal(err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			node := &xmlTree{name: token.Name.Local, attrs: map[string]string{}}
			for _, attribute := range token.Attr {
				name := attribute.Name.Local
				if attribute.Name.Space != "" {
					name = pathName(attribute.Name.Space + " " + attribute.Name.Local)
				}
				node.attrs[name] = attribute.Value
			}
			stack[len(stack)-1].children = append(stack[len(stack)-1].children, node)
			stack = append(stack, node)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			stack[len(stack)-1].text.Write(token)
		}
	}
}

var pathStep = regexp.MustCompile(`^(@?[^\[]+)(?:\[([0-9]+)\])?$`)

// lookup evaluates path as an XPath location path and returns the text of
// the one element or attribute it selects. A step with no number selects
// every child of its name, as in XPath, so it finds nothing unless there is
// exactly one.
func (root *xmlTree) lookup(t *testing.T, path string) (string, bool) {
	t.Helper()
	node := root
	for _, step := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		parts := pathStep.FindStringSubmatch(step)
		if parts == nil {
			t.Fatalf("path %q has a step %q that is not a name and a number", path, step)
		}
		name := parts[1]
		if strings.HasPrefix(name, "@") {
			value, ok := node.attrs[name[1:]]
			return value, ok
		}
		var named []*xmlTree
		for _, child := range node.children {
			if name == "*" || child.name == name {
				named = append(named, child)
			}
		}
		switch index, _ := strconv.Atoi(parts[2]); {
		case parts[2] == "" && len(named) == 1:
			node = named[0]
		case parts[2] != "" && index >= 1 && index <= len(named):
			node = named[index-1]
		default:
			return "", false
		}
	}
	return node.text.String(), true
}

// populate fills a value of the model with every element and attribute it
// can hold, each with a value of its own: every pointer set, one ordered
// child of every kind, one opus inside an opus, and two items in every list
// of the document's upper levels and one in the lists below, which keeps the
// document from growing with every level.
func populate(v reflect.Value, next *int, nestedOpus bool, depth int) {
	switch v.Kind() {
	case reflect.Pointer:
		if v.Type() == opusPointerType && nestedOpus {
			return
		}
		v.Set(reflect.New(v.Type().Elem()))
		populate(v.Elem(), next, nestedOpus || v.Type() == opusPointerType, depth)
	case reflect.Struct:
		for i := range v.NumField() {
			if v.Type().Field(i).Name != "XMLName" {
				populate(v.Field(i), next, nestedOpus, depth+1)
			}
		}
	case reflect.Slice:
		elem := v.Type().Elem()
		if _, union := elem.MethodByName("ElementName"); union {
			for i := range elem.NumField() {
				item := reflect.New(elem).Elem()
				populate(item.Field(i), next, nestedOpus, depth+1)
				if !item.Field(i).IsNil() {
					v.Set(reflect.Append(v, item))
				}
			}
			return
		}
		items := 1
		if depth <= 1 {
			items = 2
		}
		v.Set(reflect.MakeSlice(v.Type(), items, items))
		for i := range items {
			populate(v.Index(i), next, nestedOpus, depth)
		}
	case reflect.String:
		*next++
		v.SetString("s" + strconv.Itoa(*next))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		*next++
		v.SetUint(uint64(*next))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		*next++
		v.SetInt(int64(*next))
	case reflect.Float32, reflect.Float64:
		*next++
		v.SetFloat(float64(*next) + 0.5)
	case reflect.Bool:
		v.SetBool(true)
	}
}

// comparePerturbations changes values of changed one at a time, as listed by
// collectEqualPerturbations, and holds Compare to what each change is: one
// difference of the right kind, both ways round, at a place no other change
// is reported at, which is, in the XML the documents encode to, the element
// or attribute that was changed, with the values Compare gives.
func comparePerturbations(t *testing.T, original, changed Document, every, readBack int) (checked, resolved map[string]int, emptied int) {
	t.Helper()
	encode := func(document Document) (*xmlTree, bool) {
		var buffer bytes.Buffer
		if err := Encode(&buffer, document); err != nil {
			return nil, false
		}
		return parseXMLTree(t, buffer.Bytes()), true
	}
	originalTree, ok := encode(original)
	if !ok {
		t.Fatal("the original does not encode")
	}
	var all []equalPerturbation
	collectEqualPerturbations(reflect.ValueOf(changed), "", &all)
	if !Equal(original, changed) {
		t.Fatal("the two documents differ before any change")
	}
	places := map[string]string{}
	checked, resolved = map[string]int{}, map[string]int{}
	for i := 0; i < len(all); i += every {
		p := all[i]
		kind := p.kind
		p.apply()
		result := Compare(original, changed, CompareOptions{MaxDifferences: 3})
		if len(result.Differences) != 1 {
			t.Errorf("a changed %s in %s is reported as %+v", kind, p.path, result.Differences)
			p.undo()
			continue
		}
		difference := result.Differences[0]
		wantKind := DifferenceValue
		switch {
		case kind == "slice":
			wantKind = DifferenceOnlyInA
		case kind == "pointer" && difference.Kind == DifferenceElement && difference.A != "" && difference.B == "":
			// A cleared pointer that was the element of an ordered child
			// leaves a child with no element at that position.
			wantKind = DifferenceElement
			emptied++
		case kind == "pointer":
			wantKind = DifferenceOnlyInA
		}
		if difference.Kind != wantKind || !strings.HasPrefix(difference.PathA, "/") || strings.Contains(difference.PathA, "//") {
			t.Errorf("a changed %s in %s is reported as %+v", kind, p.path, difference)
		}
		// The other way round it is the same difference, mirrored.
		if reversed := Compare(changed, original, CompareOptions{MaxDifferences: 3}).Differences; len(reversed) != 1 || reversed[0] != mirror(difference) {
			t.Errorf("a changed %s in %s is reported the other way round as %+v, not the mirror of %+v", kind, p.path, reversed, difference)
		}
		// One place holds one value: a second change reported there is the
		// other kind of difference, the item gone instead of changed.
		place := difference.PathA + " " + string(difference.Kind)
		if other, twice := places[place]; twice {
			t.Errorf("%q names two values, a changed %s and a changed %s", place, other, kind)
		}
		places[place] = kind
		checked[kind]++
		if (checked[kind]-1)%readBack == 0 && wantKind != DifferenceElement {
			changedTree, encoded := encode(changed)
			if !encoded {
				t.Errorf("the document with a changed %s in %s does not encode", kind, p.path)
			} else {
				text, inA := originalTree.lookup(t, difference.PathA)
				switch {
				case !inA:
					t.Errorf("a changed %s: %s selects nothing in the first document", kind, difference.PathA)
				case wantKind == DifferenceOnlyInA:
					if _, inB := changedTree.lookup(t, difference.PathA); inB {
						t.Errorf("a removed %s: %s is in the second document too", kind, difference.PathA)
					}
				default:
					other, inB := changedTree.lookup(t, difference.PathB)
					same := text == difference.A && other == difference.B
					if kind != "string" {
						x, errX := strconv.ParseFloat(text, 64)
						y, errY := strconv.ParseFloat(other, 64)
						wantX, _ := strconv.ParseFloat(difference.A, 64)
						wantY, _ := strconv.ParseFloat(difference.B, 64)
						same = errX == nil && errY == nil && x == wantX && y == wantY
					}
					if !inB || !same {
						t.Errorf("a changed %s at %s: the XML has %q and %q (found %v), Compare says %q and %q", kind, difference.PathA, text, other, inB, difference.A, difference.B)
					}
				}
				resolved[kind]++
			}
		}
		p.undo()
		if !Equal(original, changed) {
			t.Fatalf("the documents differ after the %s in %s is put back", kind, p.path)
		}
	}
	return checked, resolved, emptied
}

// The paths Compare gives locate what differs. In a real score, every change
// of a single value, of every kind and wherever it sits, is one difference
// at a path that leads, in the XML the documents encode to, to the element or
// attribute that was changed.
func TestComparePathsLeadToTheDifference(t *testing.T) {
	skipInShortTier(t)
	data, err := os.ReadFile(filepath.Join("testdata", "official", "MozartTrio.musicxml"))
	if err != nil {
		t.Fatal(err)
	}
	original, changed := equalDecode(t, string(data)), equalDecode(t, string(data))
	every, readBack := 5, 8
	if os.Getenv("MUSICXML_WINDOWED_FULL") != "" {
		every, readBack = 1, 4
	}
	checked, resolved, emptied := comparePerturbations(t, original, changed, every, readBack)
	t.Logf("values changed and compared by kind: %v, %d of the pointers the element of an ordered child; paths read back from the XML: %v", checked, emptied, resolved)
	for _, kind := range []string{"string", "unsigned", "integer", "float", "pointer", "slice"} {
		if checked[kind] < 20 || resolved[kind] == 0 {
			t.Errorf("%d changes of a %s checked, %d read back", checked[kind], kind, resolved[kind])
		}
	}
	if emptied == 0 {
		t.Error("no element of an ordered child was cleared")
	}
}

// The same holds for every element and attribute the model has, not only
// those a real score uses: in a document of each root type that holds them
// all, every change of a single value is one difference at the path of that
// value, in a partwise score, a timewise one and an opus.
func TestComparePathsOfEveryElement(t *testing.T) {
	skipInShortTier(t)
	if raceEnabled {
		t.Skip("slow under the race detector")
	}
	// A sample by default, every value when asked for; TestCompareSeesEveryField
	// holds every field of every type whatever the sample.
	every, readBack := 29, 4
	if os.Getenv("MUSICXML_WINDOWED_FULL") != "" {
		every, readBack = 1, 6
	}
	for name, build := range map[string]func() Document{
		"partwise": func() Document { return new(ScorePartwise) },
		"timewise": func() Document { return new(ScoreTimewise) },
		"opus":     func() Document { return new(OpusDocument) },
	} {
		t.Run(name, func(t *testing.T) {
			documents := [2]Document{build(), build()}
			for _, document := range documents {
				next := 0
				populate(reflect.ValueOf(document).Elem(), &next, false, 0)
			}
			whole := compareDocuments(documents[0], documents[1], -1)
			if len(whole.found) != 0 {
				t.Fatalf("two documents filled alike differ: %+v", whole.found[0])
			}
			step := every
			if name == "opus" {
				step = 1
			}
			checked, resolved, emptied := comparePerturbations(t, documents[0], documents[1], step, readBack)
			total := 0
			for kind, n := range checked {
				total += n
				if resolved[kind] == 0 {
					t.Errorf("no changed %s read back", kind)
				}
			}
			t.Logf("%d values compared in the whole document; changed and compared by kind: %v, %d of the pointers the element of an ordered child; read back: %v", whole.visited, checked, emptied, resolved)
			minimum := 200
			if name == "opus" {
				minimum = 40
			}
			if total < minimum || emptied == 0 {
				t.Errorf("%d changes checked, %d of them of an ordered child", total, emptied)
			}
		})
	}
	// Every struct of the model is in one of the three documents.
	reached := map[reflect.Type]bool{}
	var reach func(v reflect.Value)
	reach = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Pointer:
			if !v.IsNil() {
				reach(v.Elem())
			}
		case reflect.Struct:
			reached[v.Type()] = true
			for i := range v.NumField() {
				reach(v.Field(i))
			}
		case reflect.Slice:
			for i := range v.Len() {
				reach(v.Index(i))
			}
		}
	}
	for _, document := range []Document{new(ScorePartwise), new(ScoreTimewise), new(OpusDocument)} {
		next := 0
		populate(reflect.ValueOf(document).Elem(), &next, false, 0)
		reach(reflect.ValueOf(document))
	}
	if len(reached) < 250 {
		t.Errorf("the filled documents hold %d of the model's struct types", len(reached))
	}
}

// No field of any struct of the model is left out of a comparison: for each
// type, a change of each of its fields, whatever its kind, is seen, and
// putting it back leaves no difference.
func TestCompareSeesEveryField(t *testing.T) {
	types := map[reflect.Type]bool{}
	var gather func(tp reflect.Type)
	gather = func(tp reflect.Type) {
		switch tp.Kind() {
		case reflect.Pointer, reflect.Slice:
			gather(tp.Elem())
		case reflect.Struct:
			if types[tp] {
				return
			}
			types[tp] = true
			for i := range tp.NumField() {
				gather(tp.Field(i).Type)
			}
		}
	}
	for _, root := range []any{ScorePartwise{}, ScoreTimewise{}, OpusDocument{}} {
		gather(reflect.TypeOf(root))
	}
	delete(types, reflect.TypeFor[xml.Name]())
	fields := 0
	for tp := range types {
		a, b := reflect.New(tp).Elem(), reflect.New(tp).Elem()
		for _, value := range []reflect.Value{a, b} {
			next := 0
			populate(value, &next, false, 2)
		}
		differences := func() int {
			c := &comparison{limit: -1}
			c.walk(a, b)
			return len(c.found)
		}
		if differences() != 0 {
			t.Errorf("%s: two values filled alike differ", tp)
			continue
		}
		for i := range tp.NumField() {
			if tp.Field(i).Name == "XMLName" {
				continue
			}
			var changes []equalPerturbation
			collectEqualPerturbations(b.Field(i), tp.Name()+"."+tp.Field(i).Name, &changes)
			if len(changes) == 0 {
				// An element with no content and no attributes of its own.
				if held := tp.Field(i).Type; held.Kind() != reflect.Struct || held.NumField() != 0 {
					t.Errorf("%s.%s: nothing to change", tp, tp.Field(i).Name)
				}
				continue
			}
			// The field itself, and the last value below it.
			for _, change := range []equalPerturbation{changes[0], changes[len(changes)-1]} {
				change.apply()
				if differences() == 0 {
					t.Errorf("a changed %s in %s is not seen", change.kind, change.path)
				}
				change.undo()
				if differences() != 0 {
					t.Fatalf("%s differs after the %s is put back", change.path, change.kind)
				}
			}
			fields++
		}
	}
	if len(types) < 250 || fields < 1500 {
		t.Errorf("%d types and %d fields checked", len(types), fields)
	}
}
