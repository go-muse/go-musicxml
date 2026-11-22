package musicxml

import (
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// DefaultMaxDifferences is the number of differences Compare reports when
// CompareOptions.MaxDifferences is zero.
const DefaultMaxDifferences = 100

// DifferenceKind says how two documents differ at a Difference's paths.
type DifferenceKind string

const (
	// DifferenceValue means both documents have the element or attribute
	// and its value is not the same.
	DifferenceValue DifferenceKind = "value"
	// DifferenceElement means the two documents have different elements at
	// one position of a parent's ordered children, or different root
	// elements.
	DifferenceElement DifferenceKind = "element"
	// DifferenceOnlyInA means the first document has the element or
	// attribute and the second has none.
	DifferenceOnlyInA DifferenceKind = "only-in-a"
	// DifferenceOnlyInB means the second document has the element or
	// attribute and the first has none.
	DifferenceOnlyInB DifferenceKind = "only-in-b"
	// DifferenceNotComparable means a document cannot be compared.
	// Difference.A and Difference.B hold the reason for each document, empty
	// for one that can be: "cyclic" and "nested too deeply" for an opus that
	// Encode refuses with ErrDocumentCycle or ErrDocumentTooDeep, and
	// "unsupported document type" for a Document that is none of the three
	// roots, which Encode refuses with ErrUnsupportedDocument. For an opus
	// that is both cyclic and too deep the reason is the one Compare meets
	// first, which need not be Encode's.
	DifferenceNotComparable DifferenceKind = "not-comparable"
)

// Difference is one place where two documents differ.
type Difference struct {
	// PathA and PathB locate the difference in the XML of the first and of
	// the second document, in the form of an XPath location path:
	// "/score-partwise/part[1]/measure[2]/note[5]/pitch/step" is the step of
	// the fifth note of the second measure of the first part, and
	// "/score-partwise/@version" the version attribute. "/" is the document
	// itself.
	//
	// A number after a name is the element's place among its parent's
	// children of that name, counted from 1. An element that the model keeps
	// in a list always has one, and so has every element of a parent's
	// ordered children, even where the schema allows only one; an element
	// held by a single field has none. Text is not a step: the difference of
	// an element's text is at the element's own path.
	//
	// The two paths are the same unless the difference is of two elements,
	// or earlier children of some parent differ in name, which gives the
	// element another number in the second document. PathB is empty for
	// DifferenceOnlyInA and PathA for
	// DifferenceOnlyInB: the path is that of the item in the one document
	// that has it.
	//
	// A document built in code can hold an ordered child with no element in
	// it, or more than one, which no XML has. Such a child has no name and
	// its step is "*[n]", its position among the ordered children.
	PathA, PathB string
	// Kind says how the documents differ there.
	Kind DifferenceKind
	// A and B hold what the first and the second document have: for
	// DifferenceValue the two values as text, numbers as Encode writes
	// them; for DifferenceElement the two elements' names; for
	// DifferenceNotComparable the reasons. They are empty for
	// DifferenceOnlyInA and DifferenceOnlyInB.
	A, B string
}

// String describes the difference in one line, such as
//
//	/score-partwise/part[1]/measure[2]/note[5]/pitch/step: "C" != "D"
func (d Difference) String() string {
	path := d.PathA
	switch {
	case path == "":
		path = d.PathB
	case d.PathB != "" && d.PathB != d.PathA:
		path += " (in b: " + d.PathB + ")"
	}
	switch d.Kind {
	case DifferenceValue:
		return path + ": " + strconv.Quote(d.A) + " != " + strconv.Quote(d.B)
	case DifferenceElement:
		return path + ": element <" + d.A + "> != <" + d.B + ">"
	case DifferenceOnlyInA:
		return path + ": only in a"
	case DifferenceOnlyInB:
		return path + ": only in b"
	case DifferenceNotComparable:
		var reasons []string
		if d.A != "" {
			reasons = append(reasons, "a is "+d.A)
		}
		if d.B != "" {
			reasons = append(reasons, "b is "+d.B)
		}
		return path + ": not comparable: " + strings.Join(reasons, ", ")
	}
	return path + ": " + string(d.Kind)
}

// CompareOptions selects how much Compare reports.
type CompareOptions struct {
	// MaxDifferences is the most differences Compare reports. Zero means
	// DefaultMaxDifferences and a negative value means every difference,
	// which for two very different documents is as many as the XML they
	// stand for has values: an opus nested in several places of an opus
	// differs in each of them, and a few levels of such nesting make that
	// many times the size of the documents. Compare stops looking as soon
	// as it has found one more than the limit.
	MaxDifferences int
}

// Comparison is the result of Compare.
type Comparison struct {
	// Differences lists where the documents differ, at most as many as
	// CompareOptions.MaxDifferences allows. It is empty when the documents
	// are equal. The order is that of the documents: an element's
	// attributes, then its content in the order of the model's fields.
	Differences []Difference
	// Truncated is true when the documents differ in more places than
	// Differences lists.
	Truncated bool
}

// Compare compares a and b strictly, as Equal does, and reports where they
// differ.
//
// The comparison is positional: the children of an element are compared in
// order, with no attempt to align them, so a child inserted into one document
// makes every later sibling differ from the one at its position in the other,
// and the limit is reached sooner. Each differing value is one difference,
// and a pair of different elements at one position is one difference, not one
// for every value in them.
func Compare(a, b Document, options CompareOptions) Comparison {
	limit := options.MaxDifferences
	if limit == 0 {
		limit = DefaultMaxDifferences
	}
	found := compareDocuments(a, b, limit).found
	if limit >= 0 && len(found) > limit {
		return Comparison{Differences: found[:limit:limit], Truncated: true}
	}
	// The list is the caller's: appending to it must not reach past it.
	return Comparison{Differences: found[:len(found):len(found)]}
}

// Equal reports whether a and b have identical content.
//
// The comparison is strict: it compares the two documents' data value by
// value and gives no meaning to it. Two documents are equal when they hold
// the same values in the same places, whatever text they were read from. A
// document that spells the same music differently is a different document: a
// 2/4 time signature and a 4/8 one, divisions of 1 and of 2 with durations
// scaled to match, C♯ and D♭, a partwise score and a timewise one, or a
// reordered pair of children all compare unequal. Compare says where.
//
// What decoding does not keep cannot make documents unequal: whitespace
// between elements, comments, attribute order and the spelling of a number
// in a numeric field (a duration of 4 and of 4.0 decode to the same value).
// Text is content as it is written: " A " and "A" differ, and so do "2" and
// "2.0" where the model holds text, as in beats. The rules for what decoding
// does keep are:
//
//   - A nil Document and a nil pointer of any root type are all "no
//     document" and equal each other. They are unequal to every document.
//   - Both documents must have the same root type.
//   - XMLName is ignored: Decode fills it, the constructors leave it empty
//     and Encode ignores it, so a decoded document equals a constructed one
//     with the same content.
//   - An absent optional field (a nil pointer) and a present one holding the
//     zero value are different content: an element that is not there differs
//     from an empty one.
//   - Children keep their order, and a nil slice equals an empty one.
//   - Numbers compare by value. NaN equals NaN, so that a document equals a
//     copy of itself, and +0 equals -0.
//
// An opus that Encode refuses as cyclic or nested too deeply
// (ErrDocumentCycle, ErrDocumentTooDeep) equals only itself, by identity. A
// Document that is none of the three roots, which only a type embedding one
// can be and which Encode refuses with ErrUnsupportedDocument, equals
// nothing, itself included.
//
// Equal does not modify its arguments. It is safe to call concurrently on
// documents that nothing modifies. It takes the time of reading the XML the
// documents stand for up to their first difference; an opus nested in several
// places is not compared again with the same counterpart once the two are
// found equal.
func Equal(a, b Document) bool {
	return len(compareDocuments(a, b, 0).found) == 0
}

// compareDocuments compares a and b, stopping once it has found more than
// limit differences; a negative limit never stops.
func compareDocuments(a, b Document, limit int) *comparison {
	c := &comparison{limit: limit}
	rootA, reasonA := documentRoot(a)
	rootB, reasonB := documentRoot(b)
	noneA, noneB := !rootA.IsValid() && reasonA == "", !rootB.IsValid() && reasonB == ""
	switch {
	case noneA && noneB:
	case noneA:
		c.emit(DifferenceOnlyInB, "", "")
	case noneB:
		c.emit(DifferenceOnlyInA, "", "")
	case rootA.IsValid() && rootB.IsValid() && rootA.Pointer() == rootB.Pointer():
		// The same document, whatever it holds.
	case reasonA != "" || reasonB != "":
		c.emit(DifferenceNotComparable, reasonA, reasonB)
	case rootA.Type() != rootB.Type():
		c.emit(DifferenceElement, infoFor(rootA.Type().Elem()).rootName, infoFor(rootB.Type().Elem()).rootName)
	default:
		name := infoFor(rootA.Type().Elem()).rootName
		c.path = append(c.path, segment{nameA: name, nameB: name})
		c.walk(rootA, rootB)
	}
	return c
}

// documentRoot returns the pointer to a document's root. The value is
// invalid for no document at all: a nil interface or a nil root pointer.
// The reason says why a document cannot be compared; a refused opus still
// has its root, to tell it from another one.
func documentRoot(document Document) (root reflect.Value, reason string) {
	switch value := document.(type) {
	case nil:
		return reflect.Value{}, ""
	case *ScorePartwise:
		if value != nil {
			root = reflect.ValueOf(value)
		}
	case *ScoreTimewise:
		if value != nil {
			root = reflect.ValueOf(value)
		}
	case *OpusDocument:
		if value != nil {
			root, reason = reflect.ValueOf(value), opusRefusal(value)
		}
	default:
		reason = "unsupported document type"
	}
	return root, reason
}

// opusRefusal says why an opus cannot be compared, "cyclic" or "nested too
// deeply", or "" if it can. It refuses exactly what checkDocumentNesting
// does, in time linear in the opus elements however often a nested opus is
// shared. For an opus that is both cyclic and too deep it may say "cyclic"
// where the nesting check, which looks at depth first, says too deep.
func opusRefusal(root *OpusDocument) string {
	type frame struct {
		opus *OpusDocument
		next int
		// levels is the number of XML levels the opus needs from itself
		// down, of the children seen so far.
		levels int
	}
	const visiting = 0
	levels := map[*OpusDocument]int{root: visiting}
	stack := []frame{{opus: root, levels: 1}}
	for len(stack) != 0 {
		if len(stack) > maximumDocumentDepth {
			return "nested too deeply"
		}
		top := &stack[len(stack)-1]
		if top.next == 0 && top.opus.Title != nil {
			top.levels = 2
		}
		if top.next < len(top.opus.Content) {
			child := top.opus.Content[top.next]
			top.next++
			switch known, seen := levels[child.Opus]; {
			case child.Opus == nil:
				if child.OpusLink != nil || child.Score != nil {
					top.levels = max(top.levels, 2)
				}
			case !seen:
				levels[child.Opus] = visiting
				stack = append(stack, frame{opus: child.Opus, levels: 1})
			case known == visiting:
				return "cyclic"
			default:
				top.levels = max(top.levels, known+1)
			}
			continue
		}
		done := *top
		stack = stack[:len(stack)-1]
		levels[done.opus] = done.levels
		if len(stack) != 0 {
			parent := &stack[len(stack)-1]
			parent.levels = max(parent.levels, done.levels+1)
		}
	}
	if levels[root] > maximumDocumentDepth {
		return "nested too deeply"
	}
	return ""
}

// segment is one step of the two paths: an element or an attribute with its
// place among the items of its name in each document, counted from 1, or
// 0 for none. A step with no name is the place alone, of the step before it.
// The two documents' names and places differ only for an ordered child.
type segment struct {
	nameA, nameB   string
	indexA, indexB int
	attr           bool
}

// comparison is the state of one Compare: what it has found, where in the
// documents it is, and when to stop.
type comparison struct {
	limit int
	found []Difference
	path  []segment
	full  bool
	// visited counts the values compared.
	visited int
	// equalOpus holds the pairs of nested opus documents found equal: an
	// opus nested in several places is compared once.
	equalOpus map[[2]uintptr]struct{}
}

// emit records a difference at the current path, and stops the walk once
// there are more of them than the limit.
func (c *comparison) emit(kind DifferenceKind, a, b string) {
	difference := Difference{Kind: kind, A: a, B: b}
	switch kind {
	case DifferenceOnlyInA:
		difference.PathA = c.render(false)
	case DifferenceOnlyInB:
		difference.PathB = c.render(true)
	default:
		difference.PathA = c.render(false)
		difference.PathB = difference.PathA
		for _, step := range c.path {
			if step.nameA != step.nameB || step.indexA != step.indexB {
				difference.PathB = c.render(true)
				break
			}
		}
	}
	c.found = append(c.found, difference)
	if c.limit >= 0 && len(c.found) > c.limit {
		c.full = true
	}
}

// render writes the current path in the first document, or in the second.
func (c *comparison) render(second bool) string {
	if len(c.path) == 0 {
		return "/"
	}
	var text strings.Builder
	for _, step := range c.path {
		name, index := step.nameA, step.indexA
		if second {
			name, index = step.nameB, step.indexB
		}
		if name != "" {
			text.WriteByte('/')
			if step.attr {
				text.WriteByte('@')
			}
			text.WriteString(name)
		}
		if index > 0 {
			text.WriteByte('[')
			text.WriteString(strconv.Itoa(index))
			text.WriteByte(']')
		}
	}
	return text.String()
}

var opusPointerType = reflect.TypeFor[*OpusDocument]()

// walk compares two values of one type, strictly, recording where they
// differ. The recursion ends: the model's only cycle is an opus that
// contains an opus, which compareDocuments refuses before it gets here
// (TestCompareModelShape holds the model to that). The kinds are the ones
// the model uses and their like; any other kind is a defect in the model's
// generation and stops the comparison loudly.
func (c *comparison) walk(a, b reflect.Value) {
	if c.full {
		return
	}
	c.visited++
	switch a.Kind() {
	case reflect.Pointer:
		switch {
		case a.IsNil() && b.IsNil():
		case a.IsNil():
			c.emit(DifferenceOnlyInB, "", "")
		case b.IsNil():
			c.emit(DifferenceOnlyInA, "", "")
		case a.Pointer() == b.Pointer():
			// Shared by the two documents.
		case a.Type() == opusPointerType:
			pair := [2]uintptr{a.Pointer(), b.Pointer()}
			if _, equal := c.equalOpus[pair]; equal {
				return
			}
			before := len(c.found)
			c.walk(a.Elem(), b.Elem())
			if len(c.found) == before {
				if c.equalOpus == nil {
					c.equalOpus = make(map[[2]uintptr]struct{})
				}
				c.equalOpus[pair] = struct{}{}
			}
		default:
			c.walk(a.Elem(), b.Elem())
		}
	case reflect.Struct:
		for _, field := range infoFor(a.Type()).fields {
			if field.name != "" {
				c.path = append(c.path, segment{nameA: field.name, nameB: field.name, attr: field.attr})
			}
			c.walk(a.Field(field.index), b.Field(field.index))
			if field.name != "" {
				c.path = c.path[:len(c.path)-1]
			}
		}
	case reflect.Slice:
		if elem := a.Type().Elem(); elem.Kind() == reflect.Struct && infoFor(elem).union {
			c.walkUnions(a, b)
		} else {
			c.walkSlice(a, b)
		}
	case reflect.String:
		if x, y := a.String(), b.String(); x != y {
			c.emit(DifferenceValue, x, y)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if x, y := a.Int(), b.Int(); x != y {
			c.emit(DifferenceValue, strconv.FormatInt(x, 10), strconv.FormatInt(y, 10))
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if x, y := a.Uint(), b.Uint(); x != y {
			c.emit(DifferenceValue, strconv.FormatUint(x, 10), strconv.FormatUint(y, 10))
		}
	case reflect.Float32, reflect.Float64:
		if x, y := a.Float(), b.Float(); x != y && !(math.IsNaN(x) && math.IsNaN(y)) {
			// Fixed-point, as Encode writes decimals.
			bits := a.Type().Bits()
			c.emit(DifferenceValue, strconv.FormatFloat(x, 'f', -1, bits), strconv.FormatFloat(y, 'f', -1, bits))
		}
	case reflect.Bool:
		if x, y := a.Bool(), b.Bool(); x != y {
			c.emit(DifferenceValue, strconv.FormatBool(x), strconv.FormatBool(y))
		}
	default:
		panic("musicxml: Compare: unsupported kind " + a.Kind().String() + " in " + a.Type().String())
	}
}

// walkSlice compares two lists of one kind of element, position by position;
// a nil list equals an empty one. The items are the nth of the field's name.
func (c *comparison) walkSlice(a, b reflect.Value) {
	common := min(a.Len(), b.Len())
	for i := 0; i < max(a.Len(), b.Len()) && !c.full; i++ {
		c.path = append(c.path, segment{indexA: i + 1, indexB: i + 1})
		switch {
		case i < common:
			c.walk(a.Index(i), b.Index(i))
		case i < a.Len():
			c.emit(DifferenceOnlyInA, "", "")
		default:
			c.emit(DifferenceOnlyInB, "", "")
		}
		c.path = c.path[:len(c.path)-1]
	}
}

// walkUnions compares two ordered lists of children, each of which is one of
// the elements its parent allows. A child is the nth of its name in its own
// document, so the two paths part where earlier children differ in name.
// Different elements at one position are one difference.
func (c *comparison) walkUnions(a, b reflect.Value) {
	var countsA, countsB nameCounts
	for i := 0; i < max(a.Len(), b.Len()) && !c.full; i++ {
		var step segment
		inA, inB := i < a.Len(), i < b.Len()
		if inA {
			step.nameA, step.indexA = unionStep(a.Index(i), &countsA, i)
		}
		if inB {
			step.nameB, step.indexB = unionStep(b.Index(i), &countsB, i)
		}
		c.path = append(c.path, step)
		switch {
		case !inB:
			c.emit(DifferenceOnlyInA, "", "")
		case !inA:
			c.emit(DifferenceOnlyInB, "", "")
		case step.nameA != step.nameB:
			c.emit(DifferenceElement, unionName(a.Index(i)), unionName(b.Index(i)))
		default:
			c.walk(a.Index(i), b.Index(i))
		}
		c.path = c.path[:len(c.path)-1]
	}
}

// unionName is the element name of an ordered child, "" when it holds no
// element or more than one.
func unionName(item reflect.Value) string {
	return item.Interface().(interface{ ElementName() string }).ElementName()
}

// unionStep is the step of an ordered child at a position of its list: its
// name and its place among the children of that name, or "*" and the
// position for a child with no name.
func unionStep(item reflect.Value, counts *nameCounts, position int) (string, int) {
	name := unionName(item)
	if name == "" {
		return "*", position + 1
	}
	return name, counts.next(name)
}

// nameCounts counts the items of each name in a list. The names in one list
// are as few as the elements its parent allows, so a slice is faster than a
// map.
type nameCounts []struct {
	name  string
	count int
}

func (counts *nameCounts) next(name string) int {
	for i := range *counts {
		if (*counts)[i].name == name {
			(*counts)[i].count++
			return (*counts)[i].count
		}
	}
	*counts = append(*counts, struct {
		name  string
		count int
	}{name, 1})
	return 1
}

// fieldInfo says how one field of a struct appears in a path: as an element
// or attribute of that name, or, with no name, not at all (an element's text,
// a group of fields that the XML places in the element itself, one variant
// of an ordered child).
type fieldInfo struct {
	index int
	name  string
	attr  bool
}

type structInfo struct {
	// fields are the attributes, then the rest in the model's order.
	fields []fieldInfo
	// rootName is the element a document root stands for.
	rootName string
	// union is true for the types that hold one of several elements as an
	// ordered child of their parent and name it with ElementName.
	union bool
}

var structInfos sync.Map

// infoFor reads, once for each struct type, how its fields appear in a path.
// XMLName is not content and is left out.
func infoFor(t reflect.Type) *structInfo {
	if known, ok := structInfos.Load(t); ok {
		return known.(*structInfo)
	}
	info := &structInfo{}
	_, info.union = t.MethodByName("ElementName")
	var content []fieldInfo
	for i := range t.NumField() {
		field := t.Field(i)
		name, options, _ := strings.Cut(field.Tag.Get("xml"), ",")
		if field.Name == "XMLName" {
			info.rootName = pathName(name)
			continue
		}
		entry := fieldInfo{index: i}
		if name != "" {
			entry.name = pathName(name)
			entry.attr = strings.Contains(","+options+",", ",attr,")
		}
		if entry.attr {
			info.fields = append(info.fields, entry)
		} else {
			content = append(content, entry)
		}
	}
	info.fields = append(info.fields, content...)
	structInfos.Store(t, info)
	return info
}

// pathName writes an element or attribute name from an xml tag, which holds a
// namespace and a name apart by a space, the way a path does: the prefix the
// MusicXML schema uses for the two namespaces it has, and the namespace in
// braces for any other.
func pathName(tag string) string {
	space, local, namespaced := strings.Cut(tag, " ")
	if !namespaced {
		return tag
	}
	switch space {
	case "http://www.w3.org/XML/1998/namespace":
		return "xml:" + local
	case "http://www.w3.org/1999/xlink":
		return "xlink:" + local
	}
	return "{" + space + "}" + local
}
