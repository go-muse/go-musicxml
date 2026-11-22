package musicxml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

const (
	equalHeader = `<?xml version="1.0" encoding="UTF-8"?><score-partwise version="4.0"><part-list>` +
		`<score-part id="P1"><part-name>A</part-name></score-part></part-list>`
	equalAttributes = `<attributes><divisions>1</divisions><time><beats>2</beats><beat-type>4</beat-type></time></attributes>`
	equalNote       = `<note><pitch><step>C</step><octave>4</octave></pitch><duration>4</duration><type>whole</type></note>`
)

func equalScore(measure string) string {
	return equalHeader + `<part id="P1"><measure number="1">` + measure + `</measure></part></score-partwise>`
}

func equalDecode(t *testing.T, text string) Document {
	t.Helper()
	document, err := Decode(strings.NewReader(text))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return document
}

// equalDivisions returns the divisions value of an equalScore document.
func equalDivisions(t *testing.T, document Document) *PositiveDivisions {
	t.Helper()
	score, ok := AsScorePartwise(document)
	if !ok {
		t.Fatalf("%T is not a partwise score", document)
	}
	attributes := score.Part[0].Measure[0].Content[0].Attributes
	if attributes == nil || attributes.Divisions == nil {
		t.Fatal("no divisions")
	}
	return attributes.Divisions
}

// Documents are equal when their content is the same, and unequal when any
// content differs, however the difference could be read as the same music.
func TestEqualContent(t *testing.T) {
	base := equalScore(equalAttributes + equalNote)
	secondNote := strings.Replace(equalNote, "<step>C</step>", "<step>D</step>", 1)
	for _, tc := range []struct {
		name string
		a, b string
		want bool
	}{
		{"same text", base, base, true},
		{"whitespace and a comment", base, strings.ReplaceAll(strings.Replace(base, "<part-list>", "<!-- c --><part-list>", 1), "><", ">\n  <"), true},
		{"spacing inside a tag", base, strings.Replace(base, `<score-part id="P1">`, `<score-part   id="P1" >`, 1), true},
		{"attribute order", strings.Replace(base, `<measure number="1">`, `<measure number="1" width="100">`, 1), strings.Replace(base, `<measure number="1">`, `<measure width="100" number="1">`, 1), true},
		{"4 and 4.0", base, strings.Replace(base, "<duration>4</duration>", "<duration>4.0</duration>", 1), true},
		{"divisions 1 and 1.0", base, strings.Replace(base, "<divisions>1</divisions>", "<divisions>1.00</divisions>", 1), true},
		// Text is content as it is written, and so is a number the model
		// holds as text.
		{"spaces in text", base, strings.Replace(base, "<part-name>A</part-name>", "<part-name> A </part-name>", 1), false},
		{"beats 2 and 2.0", base, strings.Replace(base, "<beats>2</beats>", "<beats>2.0</beats>", 1), false},
		{"versions 4.0 and 4.00", base, strings.Replace(base, `version="4.0"`, `version="4.00"`, 1), false},
		{"2/4 and 4/8", base, strings.Replace(strings.Replace(base, "<beats>2</beats>", "<beats>4</beats>", 1), "<beat-type>4</beat-type>", "<beat-type>8</beat-type>", 1), false},
		{"divisions and durations scaled", base, strings.Replace(strings.Replace(base, "<divisions>1</divisions>", "<divisions>2</divisions>", 1), "<duration>4</duration>", "<duration>8</duration>", 1), false},
		{"another pitch", base, strings.Replace(base, "<step>C</step>", "<step>D</step>", 1), false},
		{"sharp and flat", equalScore(equalAttributes + strings.Replace(equalNote, "<step>C</step>", "<step>C</step><alter>1</alter>", 1)), equalScore(equalAttributes + strings.Replace(secondNote, "<step>D</step>", "<step>D</step><alter>-1</alter>", 1)), false},
		{"another version", base, strings.Replace(base, `version="4.0"`, `version="3.1"`, 1), false},
		{"an element more", base, equalScore(equalNote), false},
		{"children reordered", equalScore(equalAttributes + equalNote + secondNote), equalScore(equalAttributes + secondNote + equalNote), false},
		{"a child more", equalScore(equalAttributes + equalNote), equalScore(equalAttributes + equalNote + equalNote), false},
		{"partwise and timewise", base, `<?xml version="1.0"?><score-timewise version="4.0"><part-list><score-part id="P1"><part-name>A</part-name></score-part></part-list></score-timewise>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := equalDecode(t, tc.a), equalDecode(t, tc.b)
			if got := Equal(a, b); got != tc.want {
				t.Errorf("Equal(a, b) = %v, want %v", got, tc.want)
			}
			if got := Equal(b, a); got != tc.want {
				t.Errorf("Equal(b, a) = %v, want %v", got, tc.want)
			}
			if !Equal(a, a) || !Equal(b, b) {
				t.Error("a document is not equal to itself")
			}
		})
	}
}

// What a model can hold that decoding never produces: an absent field and an
// empty one are different content, an absent list and an empty one are not.
func TestEqualModelValues(t *testing.T) {
	score := func() *ScorePartwise {
		return equalDecode(t, equalScore(equalAttributes+equalNote)).(*ScorePartwise)
	}
	t.Run("absent and empty field", func(t *testing.T) {
		a, b := score(), score()
		b.MovementTitle = Ptr("")
		if Equal(a, b) {
			t.Error("an absent element equals an empty one")
		}
		a.MovementTitle = Ptr("")
		if !Equal(a, b) {
			t.Error("equal empty elements differ")
		}
	})
	t.Run("nil and empty list", func(t *testing.T) {
		a, b := score(), score()
		a.Credit, b.Credit = nil, []Credit{}
		if !Equal(a, b) || !Equal(b, a) {
			t.Error("a nil list differs from an empty one")
		}
		b.Credit = []Credit{{}}
		if Equal(a, b) {
			t.Error("a list with an entry equals an empty one")
		}
	})
	t.Run("a pointer's target is compared, not its address", func(t *testing.T) {
		a, b := score(), score()
		if a.Part[0].Measure[0].Content[1].Note == b.Part[0].Measure[0].Content[1].Note {
			t.Fatal("decodes share a note")
		}
		if !Equal(a, b) {
			t.Error("equal copies differ")
		}
	})
	t.Run("the root's XMLName is not content", func(t *testing.T) {
		a, b := score(), score()
		b.XMLName.Local, b.XMLName.Space = "other", "urn:other"
		if !Equal(a, b) {
			t.Error("XMLName makes documents unequal")
		}
	})
	t.Run("a shared child", func(t *testing.T) {
		a := score()
		b := score()
		b.Part = append(b.Part, b.Part[0])
		a.Part = append(a.Part, a.Part[0])
		if !Equal(a, b) {
			t.Error("equal documents with a repeated part differ")
		}
	})
}

// A decoded document can hold NaN, infinities and a negative zero, and
// reflect.DeepEqual calls a document with NaN unequal to its own copy.
func TestEqualNumbers(t *testing.T) {
	decode := func(divisions string) Document {
		return equalDecode(t, strings.Replace(equalScore(equalAttributes+equalNote), "<divisions>1</divisions>", "<divisions>"+divisions+"</divisions>", 1))
	}
	set := func(value float64) Document {
		document := decode("1")
		*equalDivisions(t, document) = PositiveDivisions(value)
		return document
	}
	negativeZero := math.Copysign(0, -1)
	for _, tc := range []struct {
		name string
		a, b float64
		want bool
	}{
		{"NaN and NaN", math.NaN(), math.NaN(), true},
		{"NaN and a number", math.NaN(), 1, false},
		{"zero and negative zero", 0, negativeZero, true},
		{"infinity and infinity", math.Inf(1), math.Inf(1), true},
		{"infinity and negative infinity", math.Inf(1), math.Inf(-1), false},
		{"infinity and a number", math.Inf(1), 1, false},
		{"1 and 2", 1, 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := set(tc.a), set(tc.b)
			if got := Equal(a, b); got != tc.want {
				t.Errorf("Equal = %v, want %v", got, tc.want)
			}
			if got := Equal(b, a); got != tc.want {
				t.Errorf("reversed: Equal = %v, want %v", got, tc.want)
			}
		})
	}
	t.Run("a copy of a document with NaN", func(t *testing.T) {
		a, b := set(math.NaN()), set(math.NaN())
		if reflect.DeepEqual(a, b) {
			t.Fatal("reflect.DeepEqual calls NaN equal: Equal's own comparison is no longer needed")
		}
		if !Equal(a, b) || !Equal(a, a) {
			t.Error("a document with NaN differs from its copy")
		}
	})
	t.Run("decoded", func(t *testing.T) {
		for _, text := range []string{"NaN", "-0", "INF", "-INF"} {
			a, err := Decode(strings.NewReader(strings.Replace(equalScore(equalAttributes+equalNote), "<divisions>1</divisions>", "<divisions>"+text+"</divisions>", 1)))
			if err != nil {
				t.Logf("%s is not decoded: %v", text, err)
				continue
			}
			b := equalDecode(t, strings.Replace(equalScore(equalAttributes+equalNote), "<divisions>1</divisions>", "<divisions>"+text+"</divisions>", 1))
			if !Equal(a, b) {
				t.Errorf("two decodes of %s differ", text)
			}
		}
	})
}

func TestEqualNilDocuments(t *testing.T) {
	var (
		nilPartwise *ScorePartwise
		nilTimewise *ScoreTimewise
		nilOpus     *OpusDocument
	)
	document := equalDecode(t, equalScore(equalAttributes+equalNote))
	nils := []Document{nil, nilPartwise, nilTimewise, nilOpus}
	for i, a := range nils {
		for j, b := range nils {
			if !Equal(a, b) {
				t.Errorf("nil documents %d and %d differ", i, j)
			}
		}
		if Equal(a, document) || Equal(document, a) {
			t.Errorf("nil document %d equals a document", i)
		}
	}
	if Equal(nilPartwise, NewScorePartwise()) || Equal(NewOpusDocument(), nil) {
		t.Error("an empty document equals no document")
	}
}

// A constructed document equals the decoded one it encodes to; the
// difference between them is the XMLName Decode fills.
func TestEqualConstructedAndDecoded(t *testing.T) {
	for name, built := range map[string]Document{
		"partwise": NewScorePartwise(PartDefinition{ID: "P1", Name: "A"}, PartDefinition{ID: "P2", Name: "B"}),
		"timewise": NewScoreTimewise(PartDefinition{ID: "P1", Name: "A"}),
		"opus":     NewOpusDocument(),
		"empty":    NewScorePartwise(),
	} {
		t.Run(name, func(t *testing.T) {
			var encoded bytes.Buffer
			if err := Encode(&encoded, built); err != nil {
				t.Fatal(err)
			}
			decoded, err := Decode(bytes.NewReader(encoded.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			if !Equal(built, decoded) || !Equal(decoded, built) {
				t.Error("a constructed document differs from its decoded encoding")
			}
		})
	}
	if Equal(NewScorePartwise(PartDefinition{ID: "P1", Name: "A"}), NewScorePartwise(PartDefinition{ID: "P1", Name: "B"})) {
		t.Error("parts named differently are equal")
	}
}

// A document that Encode refuses as cyclic or too deep equals only itself,
// and Equal returns instead of recursing without end.
func TestEqualRefusedNesting(t *testing.T) {
	cyclic := func() *OpusDocument {
		opus := NewOpusDocument()
		opus.Content = append(opus.Content, OpusDocumentContent{Opus: opus})
		return opus
	}
	deep := func() *OpusDocument {
		root := NewOpusDocument()
		current := root
		for range maximumDocumentDepth + 8 {
			child := NewOpusDocument()
			current.Content = append(current.Content, OpusDocumentContent{Opus: child})
			current = child
		}
		return root
	}
	for name, build := range map[string]func() *OpusDocument{"cyclic": cyclic, "too deep": deep} {
		t.Run(name, func(t *testing.T) {
			a, b := build(), build()
			if !Equal(a, a) {
				t.Error("a document is not equal to itself")
			}
			if Equal(a, b) || Equal(b, a) {
				t.Error("equal refused documents")
			}
			if Equal(a, NewOpusDocument()) || Equal(NewOpusDocument(), a) {
				t.Error("a refused document equals an empty one")
			}
		})
	}
	// Documents nested up to the limit compare as any other.
	nested := func() *OpusDocument {
		root := NewOpusDocument()
		current := root
		for range maximumDocumentDepth - 2 {
			child := NewOpusDocument()
			current.Content = append(current.Content, OpusDocumentContent{Opus: child})
			current = child
		}
		return root
	}
	if a, b := nested(), nested(); !Equal(a, b) {
		t.Error("documents nested to the limit differ")
	}
}

// Equal and Compare read and never write, so they may run on shared
// documents at once, also when no comparison has yet read how the model's
// types appear in paths.
func TestEqualConcurrent(t *testing.T) {
	a := equalDecode(t, equalScore(equalAttributes+equalNote))
	b := equalDecode(t, equalScore(equalAttributes+equalNote))
	other := equalDecode(t, equalScore(equalNote+equalAttributes+equalNote))
	want := Compare(a, other, CompareOptions{MaxDifferences: -1})
	if len(want.Differences) < 2 {
		t.Fatal("the documents do not differ")
	}
	structInfos.Clear()
	var group sync.WaitGroup
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			for range 50 {
				if got := Compare(a, other, CompareOptions{MaxDifferences: -1}); !reflect.DeepEqual(got, want) {
					t.Error("concurrent comparisons differ")
					return
				}
			}
		}()
	}
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			for range 200 {
				if !Equal(a, b) {
					t.Error("equal documents differ")
					return
				}
			}
		}()
	}
	group.Wait()
}

// modelShape lists what, in the types reachable from roots, Compare is not
// built for, the struct types that contain themselves, and the numbers of
// struct types, of parents with ordered children and of fields beside those.
func modelShape(roots ...reflect.Type) (problems, cycles []string, structs, parents, beside int) {
	problem := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }
	handled := map[reflect.Kind]bool{
		reflect.Pointer: true, reflect.Struct: true, reflect.Slice: true, reflect.String: true, reflect.Bool: true,
		reflect.Float32: true, reflect.Float64: true,
		reflect.Int: true, reflect.Int8: true, reflect.Int16: true, reflect.Int32: true, reflect.Int64: true,
		reflect.Uint: true, reflect.Uint8: true, reflect.Uint16: true, reflect.Uint32: true, reflect.Uint64: true, reflect.Uintptr: true,
	}
	state := map[reflect.Type]int{}
	ordered := map[reflect.Type]reflect.Type{}
	// names lists the elements and the attributes a struct holds in fields
	// of their own, those of the groups it embeds included.
	var names func(tp reflect.Type) (elements, attributes []string)
	names = func(tp reflect.Type) (elements, attributes []string) {
		for i := range tp.NumField() {
			field := tp.Field(i)
			tag, tagged := field.Tag.Lookup("xml")
			name, options, _ := strings.Cut(tag, ",")
			switch {
			case field.Name == "XMLName":
			case !tagged && field.Anonymous && field.Type.Kind() == reflect.Struct:
				inner, innerAttributes := names(field.Type)
				elements, attributes = append(elements, inner...), append(attributes, innerAttributes...)
			case name != "" && strings.Contains(","+options+",", ",attr,"):
				attributes = append(attributes, pathName(name))
			case name != "":
				elements = append(elements, pathName(name))
			}
		}
		return elements, attributes
	}
	var walk func(tp reflect.Type)
	walk = func(tp reflect.Type) {
		if !handled[tp.Kind()] {
			problem("%s has kind %s, which Compare does not handle", tp, tp.Kind())
			return
		}
		switch tp.Kind() {
		case reflect.Pointer, reflect.Slice:
			walk(tp.Elem())
		case reflect.Struct:
			switch state[tp] {
			case 1:
				cycles = append(cycles, tp.Name())
				return
			case 2:
				return
			}
			state[tp] = 1
			_, union := tp.MethodByName("ElementName")
			for i := range tp.NumField() {
				field := tp.Field(i)
				if field.Name == "XMLName" {
					if field.Type != reflect.TypeFor[xml.Name]() {
						problem("%s.XMLName is a %s, which Compare would leave out as the element's name", tp.Name(), field.Type)
					}
					continue
				}
				tag, tagged := field.Tag.Lookup("xml")
				name, options, _ := strings.Cut(tag, ",")
				form := "tag " + tag
				switch {
				case !tagged && (union || field.Anonymous):
					form = "variant or embedded group"
				case !tagged:
					form = "no tag"
				case strings.Contains(name, ">") || tag == "-":
				case name == "" && options == "chardata", name == "" && options == "any":
					form = "," + options
				case name != "" && (options == "" || options == "omitempty"):
					form = "element"
				case name != "" && (options == "attr" || options == "attr,omitempty"):
					form = "attribute"
				}
				switch form {
				case "variant or embedded group", ",chardata", ",any", "element", "attribute":
				default:
					problem("%s.%s has %s, which Compare's paths do not understand", tp.Name(), field.Name, form)
				}
				// The elements a parent keeps in order are a list of its
				// own, tagged ",any", of a type that names its element.
				inner := field.Type
				for inner.Kind() == reflect.Pointer {
					inner = inner.Elem()
				}
				_, named := inner.MethodByName("ElementName")
				listed := false
				if inner.Kind() == reflect.Slice && inner.Elem().Kind() == reflect.Struct {
					_, listed = inner.Elem().MethodByName("ElementName")
				}
				if named || listed != (form == ",any") {
					problem("%s.%s: ordered children are not a ,any list of a type with ElementName", tp.Name(), field.Name)
				}
				// A list of pointers could hold a nil, which the XML would
				// not have: the numbers in a path would run ahead of it.
				if inner.Kind() == reflect.Slice && inner.Elem().Kind() == reflect.Pointer {
					problem("%s.%s is a list of pointers", tp.Name(), field.Name)
				}
				if listed {
					ordered[tp] = inner.Elem()
				}
				walk(field.Type)
			}
			// An element or attribute has one field: a path names it once.
			elements, attributes := names(tp)
			for _, list := range [][]string{elements, attributes} {
				seen := map[string]bool{}
				for _, name := range list {
					if seen[name] {
						problem("%s holds %s in two fields", tp.Name(), name)
					}
					seen[name] = true
				}
			}
			state[tp] = 2
		}
	}
	for _, root := range roots {
		walk(root)
	}
	// An element held by a field of its own has no number in a path, so no
	// ordered child of the same parent may have its name.
	for parent, child := range ordered {
		childNames := map[string]bool{}
		for i := range child.NumField() {
			item := reflect.New(child).Elem()
			if item.Field(i).Kind() != reflect.Pointer {
				problem("%s.%s is not a pointer to the element it stands for", child.Name(), child.Field(i).Name)
				continue
			}
			item.Field(i).Set(reflect.New(child.Field(i).Type.Elem()))
			childNames[item.Interface().(interface{ ElementName() string }).ElementName()] = true
		}
		if childNames[""] || len(childNames) != child.NumField() {
			problem("%s: the ordered children of %s are named %v", child.Name(), parent.Name(), childNames)
		}
		elements, _ := names(parent)
		for _, name := range elements {
			beside++
			if childNames[name] {
				problem("%s has %s both as a field and as an ordered child", parent.Name(), name)
			}
		}
	}
	sort.Strings(problems)
	return problems, cycles, len(state), len(ordered), beside
}

// Compare walks what the model is made of. The kinds it handles, the forms of
// xml tag its paths understand and the one cycle in the model, an opus inside
// an opus, which it refuses before it compares, are what it is built on: a
// model that gains another kind, another form of tag or another way to nest
// needs Compare changed with it.
func TestCompareModelShape(t *testing.T) {
	problems, cycles, structs, parents, beside := modelShape(reflect.TypeFor[ScorePartwise](), reflect.TypeFor[ScoreTimewise](), reflect.TypeFor[OpusDocument]())
	for _, problem := range problems {
		t.Error(problem)
	}
	if len(cycles) != 1 || cycles[0] != "OpusDocument" {
		t.Errorf("the model's cycles are %v, want only OpusDocument: make Compare refuse them as it refuses an opus inside an opus", cycles)
	}
	if structs < 250 || parents < 20 || beside < 20 {
		t.Errorf("%d model structs reached, %d parents with ordered children and %d fields beside them", structs, parents, beside)
	}
}

type (
	shapeChild struct{ X, Y *string }
	shapeGroup struct {
		Inner *string `xml:"x"`
	}
	shapeMap struct {
		M map[string]string `xml:"m"`
	}
	shapePointers struct {
		L []*string `xml:"l"`
	}
	shapeNested struct {
		A string `xml:"a>b"`
	}
	shapeSkipped struct {
		A string `xml:"-"`
	}
	shapeUntagged struct{ A string }
	shapeName     struct {
		XMLName string `xml:"x"`
	}
	shapeEmbedded struct {
		shapeGroup
		A *string `xml:"x"`
	}
	shapeClash struct {
		A       *string      `xml:"x"`
		Content []shapeChild `xml:",any"`
	}
	shapeLoose struct {
		Child shapeChild `xml:"child"`
	}
	shapeNoList struct {
		Content []shapeChild `xml:"child"`
	}
	shapeCycle struct {
		Next *shapeCycle `xml:"next"`
	}
	shapeFine struct {
		shapeFineGroup
		A       *string      `xml:"a"`
		B       []string     `xml:"b"`
		C       float64      `xml:"c,attr,omitempty"`
		Text    string       `xml:",chardata"`
		Content []shapeChild `xml:",any"`
	}
)

type shapeFineGroup struct {
	G *string `xml:"g"`
}

// shapeTwice is a struct with two fields of one tag, which go vet does not
// let a declared type have.
func shapeTwice(tag string) reflect.Type {
	field := func(name string) reflect.StructField {
		return reflect.StructField{Name: name, Type: reflect.TypeFor[*string](), Tag: reflect.StructTag(`xml:"` + tag + `"`)}
	}
	return reflect.StructOf([]reflect.StructField{field("A"), field("B")})
}

func (c shapeChild) ElementName() string {
	switch {
	case c.X != nil && c.Y == nil:
		return "x"
	case c.Y != nil && c.X == nil:
		return "y"
	}
	return ""
}

// The shape check finds each thing Compare is not built for.
func TestCompareModelShapeFindsProblems(t *testing.T) {
	for name, tc := range map[string]struct {
		root reflect.Type
		want string
	}{
		"a map":                           {reflect.TypeFor[shapeMap](), "kind map"},
		"a list of pointers":              {reflect.TypeFor[shapePointers](), "list of pointers"},
		"a nested tag":                    {reflect.TypeFor[shapeNested](), "tag a>b"},
		"a skipped field":                 {reflect.TypeFor[shapeSkipped](), "tag -"},
		"a field with no tag":             {reflect.TypeFor[shapeUntagged](), "no tag"},
		"an XMLName of another type":      {reflect.TypeFor[shapeName](), "XMLName is a string"},
		"an element in two fields":        {shapeTwice("x"), "holds x in two fields"},
		"an attribute in two fields":      {shapeTwice("x,attr"), "holds x in two fields"},
		"an element of an embedded group": {reflect.TypeFor[shapeEmbedded](), "holds x in two fields"},
		"a field named as a child":        {reflect.TypeFor[shapeClash](), "both as a field and as an ordered child"},
		"an ordered child out of a list":  {reflect.TypeFor[shapeLoose](), "not a ,any list"},
		"ordered children under a name":   {reflect.TypeFor[shapeNoList](), "not a ,any list"},
	} {
		problems, _, _, _, _ := modelShape(tc.root)
		if len(problems) == 0 || !strings.Contains(strings.Join(problems, "\n"), tc.want) {
			t.Errorf("%s: problems %q, want one with %q", name, problems, tc.want)
		}
	}
	if _, cycles, _, _, _ := modelShape(reflect.TypeFor[shapeCycle]()); len(cycles) != 1 || cycles[0] != "shapeCycle" {
		t.Errorf("a type in itself: cycles %v", cycles)
	}
	if problems, cycles, _, parents, beside := modelShape(reflect.TypeFor[shapeFine]()); len(problems) != 0 || len(cycles) != 0 || parents != 1 || beside != 3 {
		t.Errorf("a shape Compare is built for: problems %q, cycles %v, %d parents, %d fields beside", problems, cycles, parents, beside)
	}
}

// The kinds beyond the model's own that Compare handles, and the ones it
// refuses: nothing it does not use may be compared wrongly, and nothing it
// cannot compare may be compared quietly.
func TestCompareKinds(t *testing.T) {
	type shape struct {
		Flag   bool
		Small  int8
		Count  uint16
		Ratio  float32
		Text   string
		hidden int
		Wide   int64
		Long   uint64
	}
	for _, tc := range []struct {
		name string
		a, b shape
		want int
	}{
		{"same", shape{Flag: true, Small: -2, Count: 3, Ratio: 1.5, Text: "x", hidden: 1}, shape{Flag: true, Small: -2, Count: 3, Ratio: 1.5, Text: "x", hidden: 1}, 0},
		{"bool", shape{Flag: true}, shape{}, 1},
		{"bool, the other way", shape{}, shape{Flag: true}, 1},
		{"small integer, the other way", shape{Small: 1}, shape{Small: -1}, 1},
		{"unsigned, the other way", shape{Count: 2}, shape{Count: 1}, 1},
		{"float32, the other way", shape{Ratio: 2}, shape{Ratio: 1}, 1},
		{"small integer", shape{Small: -1}, shape{Small: 1}, 1},
		{"unsigned", shape{Count: 1}, shape{Count: 2}, 1},
		{"float32", shape{Ratio: 1}, shape{Ratio: 2}, 1},
		{"float32 NaN", shape{Ratio: float32(math.NaN())}, shape{Ratio: float32(math.NaN())}, 0},
		{"unexported field", shape{hidden: 1}, shape{hidden: 2}, 1},
		{"integers beyond 32 bits", shape{Wide: 1}, shape{Wide: 1 + 1<<32}, 1},
		{"unsigned integers beyond 32 bits", shape{Long: 1}, shape{Long: 1 + 1<<32}, 1},
		{"text", shape{Text: "a"}, shape{Text: "b"}, 1},
		{"every field", shape{true, 1, 1, 1, "a", 1, 1, 1}, shape{false, 2, 2, 2, "b", 2, 2, 2}, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &comparison{limit: -1}
			c.walk(reflect.ValueOf(tc.a), reflect.ValueOf(tc.b))
			if len(c.found) != tc.want {
				t.Errorf("%d differences %v, want %d", len(c.found), c.found, tc.want)
			}
		})
	}
	t.Run("the values as text", func(t *testing.T) {
		c := &comparison{limit: -1}
		c.walk(reflect.ValueOf(shape{Flag: true, Small: -3, Count: 7, Wide: 1 << 40, Long: 1 << 63}), reflect.ValueOf(shape{Small: 4, Count: 8, Wide: -1, Long: 5}))
		var got []string
		for _, d := range c.found {
			got = append(got, d.A+" "+d.B)
		}
		if want := []string{"true false", "-3 4", "7 8", "1099511627776 -1", "9223372036854775808 5"}; !reflect.DeepEqual(got, want) {
			t.Errorf("values are written %q, want %q", got, want)
		}
	})
	t.Run("float32 text", func(t *testing.T) {
		c := &comparison{limit: -1}
		c.walk(reflect.ValueOf(shape{Ratio: 0.1}), reflect.ValueOf(shape{Ratio: 0.2}))
		if len(c.found) != 1 || c.found[0].A != "0.1" || c.found[0].B != "0.2" {
			t.Errorf("float32 values are written as %v", c.found)
		}
	})
	type withMap struct{ M map[string]int }
	type withInterface struct{ I any }
	type withFunc struct{ F func() }
	type withArray struct{ A [2]int }
	for name, value := range map[string]any{"map": withMap{}, "interface": withInterface{}, "func": withFunc{}, "array": withArray{}} {
		t.Run("refuses a "+name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("a %s was compared quietly", name)
				}
			}()
			(&comparison{limit: -1}).walk(reflect.ValueOf(value), reflect.ValueOf(value))
		})
	}
}

// equalPerturbation changes one value in a document and puts it back.
type equalPerturbation struct {
	kind        string
	path        string
	apply, undo func()
}

// collectEqualPerturbations lists, for every exported, settable value under v,
// a change of that value: a string extended, a number moved, a pointer
// cleared, a list shortened. The root's XMLName is not content and is left out.
func collectEqualPerturbations(v reflect.Value, path string, out *[]equalPerturbation) {
	add := func(kind string, apply, undo func()) {
		*out = append(*out, equalPerturbation{kind: kind, path: path, apply: apply, undo: undo})
	}
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return
		}
		if v.CanSet() {
			old := reflect.New(v.Type()).Elem()
			old.Set(v)
			add("pointer", func() { v.SetZero() }, func() { v.Set(old) })
		}
		collectEqualPerturbations(v.Elem(), path, out)
	case reflect.Struct:
		for i := range v.NumField() {
			field := v.Type().Field(i)
			if !field.IsExported() || field.Name == "XMLName" {
				continue
			}
			collectEqualPerturbations(v.Field(i), path+"."+field.Name, out)
		}
	case reflect.Slice:
		if v.Len() > 0 && v.CanSet() {
			// The slice header is put back, not a copy: the values collected
			// below it point into its array.
			old := reflect.New(v.Type()).Elem()
			old.Set(v)
			add("slice", func() { v.Set(v.Slice(0, v.Len()-1)) }, func() { v.Set(old) })
		}
		for i := range v.Len() {
			collectEqualPerturbations(v.Index(i), path+"[]", out)
		}
	case reflect.String:
		if v.CanSet() {
			old := v.String()
			add("string", func() { v.SetString(old + "x") }, func() { v.SetString(old) })
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if v.CanSet() {
			old := v.Uint()
			add("unsigned", func() { v.SetUint(old + 1) }, func() { v.SetUint(old) })
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.CanSet() {
			old := v.Int()
			add("integer", func() { v.SetInt(old + 1) }, func() { v.SetInt(old) })
		}
	case reflect.Float32, reflect.Float64:
		if v.CanSet() {
			old := v.Float()
			add("float", func() {
				if math.IsNaN(old) {
					v.SetFloat(0)
				} else {
					v.SetFloat(old + 1)
				}
			}, func() { v.SetFloat(old) })
		}
	case reflect.Bool:
		if v.CanSet() {
			old := v.Bool()
			add("bool", func() { v.SetBool(!old) }, func() { v.SetBool(old) })
		}
	}
}

// Every value in a real score matters: changing any single one of them,
// whatever its kind and wherever it sits, makes the document unequal to a
// copy of the original, and putting it back makes them equal again.
func TestEqualDetectsEveryValue(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "official", "MozartTrio.musicxml"))
	if err != nil {
		t.Fatal(err)
	}
	original, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	changed, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	var all []equalPerturbation
	collectEqualPerturbations(reflect.ValueOf(changed), "", &all)
	perKind := map[string][]equalPerturbation{}
	for _, p := range all {
		perKind[p.kind] = append(perKind[p.kind], p)
	}
	// A sample of each kind, all of them when asked for.
	limit := 150
	if os.Getenv("MUSICXML_WINDOWED_FULL") != "" {
		limit = len(all)
	}
	checked := map[string]int{}
	for kind, list := range perKind {
		step := max(1, len(list)/limit)
		for i := 0; i < len(list); i += step {
			p := list[i]
			p.apply()
			if Equal(original, changed) {
				t.Errorf("a changed %s at %s is not noticed", kind, p.path)
			}
			p.undo()
			if !Equal(original, changed) {
				t.Fatalf("the document differs after the %s at %s is put back", kind, p.path)
			}
			checked[kind]++
		}
	}
	t.Logf("%d values in the score; values changed by kind: %v", len(all), checked)
	for _, kind := range []string{"string", "unsigned", "float", "pointer", "slice"} {
		if checked[kind] == 0 {
			t.Errorf("no %s in the score to change", kind)
		}
	}
	// The root's XMLName, and only it, is not content.
	score := changed.(*ScorePartwise)
	score.XMLName.Local = "changed"
	if !Equal(original, changed) {
		t.Error("the root's XMLName is content")
	}
}

// On the corpus, a document equals a second decode of its file and the
// decode of its own encoding, and differs from the document of a file that
// encodes differently.
func TestEqualOnCorpus(t *testing.T) {
	if raceEnabled {
		t.Skip("slow under the race detector")
	}
	var files []string
	for _, dir := range []string{"testdata/official", "testdata/corpus"} {
		if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && (strings.HasSuffix(path, ".xml") || strings.HasSuffix(path, ".musicxml")) {
				files = append(files, path)
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	var previous Document
	var previousEncoded []byte
	compared, different := 0, 0
	for i := 0; i < len(files); i += windowedStride() {
		data, err := os.ReadFile(files[i])
		if err != nil {
			t.Fatal(err)
		}
		first, err := Decode(bytes.NewReader(data))
		if err != nil {
			continue
		}
		second, err := Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s: second decode: %v", files[i], err)
		}
		var encoded bytes.Buffer
		if err := Encode(&encoded, first); err != nil {
			continue
		}
		again, err := Decode(bytes.NewReader(encoded.Bytes()))
		if err != nil {
			t.Fatalf("%s: decode of the encoding: %v", files[i], err)
		}
		if !Equal(first, first) || !Equal(first, second) || !Equal(second, first) {
			t.Errorf("%s: two decodes of the file differ", files[i])
		}
		if !Equal(first, again) {
			t.Errorf("%s: the document differs from the decode of its encoding", files[i])
		}
		if previous != nil && !bytes.Equal(previousEncoded, encoded.Bytes()) {
			if Equal(first, previous) || Equal(previous, first) {
				t.Errorf("%s: equal to the previous file's document, which encodes differently", files[i])
			}
			different++
		}
		previous, previousEncoded = first, encoded.Bytes()
		compared++
	}
	t.Logf("%d files compared, %d against a different file", compared, different)
	if compared < 20 || different < 20 {
		t.Errorf("%d files compared, %d against a different file", compared, different)
	}
}
