package musicxml

import (
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"
)

type sourceDTDTestInput struct {
	text                  string
	offset                int
	depthLimit, nameLimit int64
	maxDepth, maxName     int64
	faultAt               int
	fault                 error
}

func (in *sourceDTDTestInput) Peek() (rune, error) {
	if in.fault != nil && in.offset >= in.faultAt {
		return 0, in.fault
	}
	if in.offset == len(in.text) {
		return 0, io.EOF
	}
	for _, r := range in.text[in.offset:] {
		return r, nil
	}
	panic("unreachable")
}
func (in *sourceDTDTestInput) Read() (rune, error) {
	r, err := in.Peek()
	if err == nil {
		in.offset += len(string(r))
	}
	return r, err
}
func (in *sourceDTDTestInput) SyntaxError(message string) error {
	return &xml.SyntaxError{Msg: message, Line: 1}
}
func (in *sourceDTDTestInput) CheckDTDDepth(n int64) error {
	if n > in.maxDepth {
		in.maxDepth = n
	}
	if in.depthLimit > 0 && n > in.depthLimit {
		return errSourceDTDTestLimit
	}
	return nil
}
func (in *sourceDTDTestInput) CheckDTDNameBytes(n int64) error {
	if n > in.maxName {
		in.maxName = n
	}
	if in.nameLimit > 0 && n > in.nameLimit {
		return errSourceDTDTestLimit
	}
	return nil
}

var errSourceDTDTestLimit = errors.New("test DTD resource limit")

func TestSourceDTDValidGrammar(t *testing.T) {
	cases := []struct{ name, input string }{
		{"name only", " root>"},
		{"qname", " p:root >"},
		{"fifth edition name", " \U00010000\U000EFFFF>"},
		{"empty subset", " root[]>"},
		{"whitespace subset", " root [ \n\t\r ] >"},
		{"system", ` root SYSTEM 'missing.dtd'>`},
		{"public", ` root PUBLIC '-//Example//DTD Music 1.0//EN' "https://invalid.example/d.dtd">`},
		{"system literal opaque", ` root SYSTEM '<&%[]>'[]>`},
		{"comments and instructions", ` root [<!-- --><?p?><?target data ? x?>]>`},
		{"empty", ` root [<!ELEMENT root EMPTY>]>`},
		{"any", ` root [<!ELEMENT root ANY >]>`},
		{"pcdata", ` root [<!ELEMENT root (#PCDATA)>]>`},
		{"pcdata star", ` root [<!ELEMENT root (#PCDATA)*>]>`},
		{"mixed", ` root [<!ELEMENT root (#PCDATA | p:a | b)*>]>`},
		{"single sequence", ` root [<!ELEMENT root (a)>]>`},
		{"nested children", ` root [<!ELEMENT root ((a|b)+,(c?,(d,e)*),f?)>]>`},
		{"nested single sequence", ` root [<!ELEMENT root (((a)))+>]>`},
		{"empty attlist", ` root [<!ATTLIST root>]>`},
		{"string attribute", ` root [<!ATTLIST root a CDATA #IMPLIED>]>`},
		{"all attribute types", ` root [<!ATTLIST root a ID #REQUIRED b IDREF #IMPLIED c IDREFS #IMPLIED d ENTITY #IMPLIED e ENTITIES #IMPLIED f NMTOKEN #IMPLIED g NMTOKENS #IMPLIED>]>`},
		{"attribute defaults", ` root [<!ATTLIST root a CDATA 'x &lt; &#9;' b CDATA #FIXED "x" c CDATA #REQUIRED>]>`},
		{"literal attribute percent", ` root [<!ATTLIST root a CDATA '%uninterpreted;'>]>`},
		{"enumeration", ` root [<!ATTLIST root a (1|two|p:three) '1'>]>`},
		{"notation attribute", ` root [<!ATTLIST root a NOTATION (gif | png) #IMPLIED>]>`},
		{"internal general entity", ` root [<!ENTITY unused 'literal < > &amp; &#x41;'>]>`},
		{"internal parameter entity", ` root [<!ENTITY % unused '<!ELEMENT root EMPTY>'>]>`},
		{"external general entity", ` root [<!ENTITY unused SYSTEM 'never-fetch.xml'>]>`},
		{"external parameter entity", ` root [<!ENTITY % unused PUBLIC 'identifier' 'never-fetch.dtd'>]>`},
		{"unparsed entity", ` root [<!ENTITY image SYSTEM 'image.gif' NDATA gif>]>`},
		{"notation system", ` root [<!NOTATION gif SYSTEM 'viewer'>]>`},
		{"notation public only", ` root [<!NOTATION gif PUBLIC 'image/gif'>]>`},
		{"notation public system", ` root [<!NOTATION gif PUBLIC 'image/gif' 'viewer'>]>`},
		{"adjacent declarations", ` root[<!ELEMENT root EMPTY><!ATTLIST root a CDATA #IMPLIED><!ENTITY a 'x'><!NOTATION n PUBLIC 'n'>]>`},
		{"duplicate declaration is validity", ` root[<!ELEMENT root EMPTY><!ELEMENT root ANY>]>`},
		{"mixed duplicate is validity", ` root[<!ELEMENT root (#PCDATA|a|a)*>]>`},
		{"zero padded char reference", " root[<!ENTITY x '&#" + strings.Repeat("0", 1000) + "65;'>]>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := &sourceDTDTestInput{text: tc.input + "SENTINEL"}
			info, err := parseSourceDTD(in)
			if err != nil {
				t.Fatalf("parseSourceDTD: %v", err)
			}
			if info.Name == "" {
				t.Error("missing DOCTYPE name")
			}
			if in.offset != len(tc.input) {
				t.Errorf("consumed %d bytes, want %d", in.offset, len(tc.input))
			}
		})
	}
}

func TestSourceDTDInvalidGrammar(t *testing.T) {
	cases := []string{
		`>`, `root>`, ` >`, ` 1root>`, ` :root>`, ` root:>`, ` a:b:c>`, " \U000F0000>",
		` root junk>`, ` root PUBLIC 'identifier'>`, ` root SYSTEM>`, ` root SYSTEM "x" "y">`,
		` root PUBLIC "bad|public" 'x'>`, " root PUBLIC 'bad\tpublic' 'x'>", ` root SYSTEM'x'>`,
		` root[nonsense]>`, ` root[<!UNKNOWN x>]>`, ` root[<![IGNORE[x]]>]>`, ` root[&x;]>`,
		` root[<!-- bad--comment -->]>`, ` root[<!-- bad--->]>`, ` root[<?xml?>]>`, ` root[<?XmL data?>]>`,
		` root[<?p?x?>]>`, ` root[<?p:target?>]>`, ` root[<??>]>`,
		` root[<!ELEMENT root>]>`, ` root[<!ELEMENTroot EMPTY>]>`, ` root[<!ELEMENT rootEMPTY>]>`,
		` root[<!ELEMENT root EMPTYx>]>`, ` root[<!ELEMENT root EMPTY*>]>`, ` root[<!ELEMENT root ()>]>`,
		` root[<!ELEMENT root (a|)>]>`, ` root[<!ELEMENT root (|a)>]>`, ` root[<!ELEMENT root (a,)>]>`,
		` root[<!ELEMENT root (a,b|c)>]>`, ` root[<!ELEMENT root (a b)>]>`, ` root[<!ELEMENT root (a)**>]>`,
		` root[<!ELEMENT root (a ?)>]>`, ` root[<!ELEMENT root (a)?+>]>`, ` root[<!ELEMENT root (#PCDATA|a)>]>`,
		` root[<!ELEMENT root (#PCDATA, a)*>]>`, ` root[<!ELEMENT root (#PCDATA)? >]>`,
		` root[<!ELEMENT root ((#PCDATA))>]>`, ` root[<!ELEMENT root (a:b:c)>]>`,
		` root[<!ATTLIST>]>`, ` root[<!ATTLIST root a>]>`, ` root[<!ATTLIST root a UNKNOWN #IMPLIED>]>`,
		` root[<!ATTLIST root a CDATA>]>`, ` root[<!ATTLIST root a CDATA #UNKNOWN>]>`,
		` root[<!ATTLIST root a CDATA #FIXED>]>`, ` root[<!ATTLIST root a CDATA #FIXED'x'>]>`,
		` root[<!ATTLIST root a CDATA 'x'b CDATA 'y'>]>`, ` root[<!ATTLIST root a CDATA '<'>]>`,
		` root[<!ATTLIST root a () #IMPLIED>]>`, ` root[<!ATTLIST root a (a,) #IMPLIED>]>`,
		` root[<!ATTLIST root a NOTATION(a) #IMPLIED>]>`, ` root[<!ATTLIST root a NOTATION (1) #IMPLIED>]>`,
		` root[<!ENTITY>]>`, ` root[<!ENTITY a>]>`, ` root[<!ENTITY %x 'a'>]>`, ` root[<!ENTITY a 'unterminated>]>`,
		` root[<!ENTITY a 'bare &'>]>`, ` root[<!ENTITY a '&name'>]>`, ` root[<!ENTITY a '&#;'>]>`,
		` root[<!ENTITY a '&#x;'>]>`, ` root[<!ENTITY a '&#X41;'>]>`, ` root[<!ENTITY a '&#0;'>]>`,
		` root[<!ENTITY a '&#xD800;'>]>`, ` root[<!ENTITY a '&#1114112;'>]>`, ` root[<!ENTITY a '&#999999999999999999999;'>]>`,
		` root[<!ENTITY a '&#-1;'>]>`, ` root[<!ENTITY a '&p:name;'>]>`, ` root[<!ENTITY p:name 'x'>]>`,
		` root[<!ENTITY a '%parameter;'>]>`, ` root[<!ENTITY a 'x' NDATA n>]>`,
		` root[<!ENTITY a PUBLIC 'id'>]>`, ` root[<!ENTITY % a SYSTEM 'x' NDATA n>]>`,
		` root[<!ENTITY a SYSTEM 'x'NDATA n>]>`, ` root[<!ENTITY a SYSTEM 'x' NDATA>]>`,
		` root[<!NOTATION>]>`, ` root[<!NOTATION n>]>`, ` root[<!NOTATION n 'id'>]>`,
		` root[<!NOTATION p:n PUBLIC 'id'>]>`, ` root[<!NOTATION n SYSTEM>]>`, ` root[<!NOTATION n PUBLIC 'id''uri'>]>`,
		` root[%;]>`, ` root[%1bad;]>`, ` root[%name]>`, ` root[%p:name;]>`,
	}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			_, err := parseSourceDTD(&sourceDTDTestInput{text: input})
			var syntax *xml.SyntaxError
			if !errors.As(err, &syntax) {
				t.Fatalf("got %T %v, want XML syntax error", err, err)
			}
		})
	}
}

func TestSourceDTDCapabilityFlags(t *testing.T) {
	cases := []struct {
		input                                          string
		external, decl, attribute, entity, unsupported bool
	}{
		{input: ` root[]>`},
		{input: ` root[<!--comment--><?p?>]>`},
		{input: ` root SYSTEM 'no-fetch.dtd'>`, external: true, unsupported: true},
		{input: ` root[<!ELEMENT root EMPTY>]>`, decl: true},
		{input: ` root[<!ATTLIST root a CDATA 'default'>]>`, decl: true, attribute: true, unsupported: true},
		{input: ` root[<!ENTITY unused 'value'>]>`, decl: true, entity: true, unsupported: true},
		{input: ` root[<!NOTATION n PUBLIC 'n'>]>`, decl: true},
	}
	for _, tc := range cases {
		info, err := parseSourceDTD(&sourceDTDTestInput{text: tc.input})
		if err != nil {
			t.Errorf("%s: %v", tc.input, err)
			continue
		}
		if info.ExternalSubset != tc.external || info.HasDeclarations != tc.decl || info.HasAttributeDeclarations != tc.attribute || info.HasEntityDeclarations != tc.entity || info.Unsupported != tc.unsupported || info.HasParameterReferences {
			t.Errorf("%s: flags = %+v", tc.input, info)
		}
	}
}

func TestSourceDTDParameterEntityStopsBeforeDependentGrammar(t *testing.T) {
	input := ` root[<!ENTITY % p '<!ELEMENT root EMPTY>'>%p;`
	in := &sourceDTDTestInput{text: input + ` not-yet-known ]>`}
	info, err := parseSourceDTD(in)
	var unsupported *sourceXMLUnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("got %T %v, want unsupported", err, err)
	}
	var syntax *xml.SyntaxError
	if errors.As(err, &syntax) {
		t.Error("unsupported parameter replacement mislabeled malformed")
	}
	if !info.HasParameterReferences || !info.Unsupported {
		t.Errorf("missing incomplete flags: %+v", info)
	}
	if in.offset != len(input) {
		t.Errorf("consumed dependent grammar: %d, want %d", in.offset, len(input))
	}
}

func TestSourceDTDTruncationAndReadError(t *testing.T) {
	input := ` root PUBLIC 'pubid' 'system'[<!ELEMENT root (a,(b|c)*)><!ATTLIST root a CDATA 'x'><!ENTITY e '&#x41;'><!NOTATION n PUBLIC 'n'><!--c--><?p x?>]>`
	for i := range len(input) {
		_, err := parseSourceDTD(&sourceDTDTestInput{text: input[:i]})
		var syntax *xml.SyntaxError
		if !errors.As(err, &syntax) {
			t.Errorf("truncated at %d: got %T %v", i, err, err)
		}
		cause := errors.New("input failure")
		_, err = parseSourceDTD(&sourceDTDTestInput{text: input, faultAt: i, fault: cause})
		if !errors.Is(err, cause) {
			t.Errorf("fault at %d: got %v", i, err)
		}
	}
}

func TestSourceDTDBounds(t *testing.T) {
	for _, tc := range []struct {
		input       string
		depth, name int64
	}{
		{input: ` root[<!ELEMENT root (((a)))>]>`, depth: 2},
		{input: ` abcde>`, name: 4},
		{input: ` root[<!ELEMENT root (abcde)>]>`, name: 4},
	} {
		in := &sourceDTDTestInput{text: tc.input, depthLimit: tc.depth, nameLimit: tc.name}
		_, err := parseSourceDTD(in)
		if !errors.Is(err, errSourceDTDTestLimit) {
			t.Errorf("%s: got %v, want limit error", tc.input, err)
		}
	}
	in := &sourceDTDTestInput{text: " \U00010000>", nameLimit: 3}
	if _, err := parseSourceDTD(in); !errors.Is(err, errSourceDTDTestLimit) {
		t.Errorf("UTF-8 byte limit: %v", err)
	}
	in = &sourceDTDTestInput{text: ` root[<!ELEMENT root (((a)))>]>`, depthLimit: 3, nameLimit: 4}
	if _, err := parseSourceDTD(in); err != nil {
		t.Errorf("exact limits: %v", err)
	}
}

func TestSourceDTDLiteralsAreNotRetained(t *testing.T) {
	short := ` root[<!ENTITY unused 'short'>]>`
	long := ` root[<!ENTITY unused '` + strings.Repeat("long < > text ", 100000) + `'>]>`
	allocs := func(input string) float64 {
		return testing.AllocsPerRun(2, func() {
			if _, err := parseSourceDTD(&sourceDTDTestInput{text: input}); err != nil {
				panic(err)
			}
		})
	}
	if small, big := allocs(short), allocs(long); big > small+2 {
		t.Errorf("literal-sized allocations: short=%v long=%v", small, big)
	}
}

func TestSourceDTDHardCapacityIsNotMalformed(t *testing.T) {
	for _, input := range []string{
		" " + strings.Repeat("a", sourceDTDMaxNameBytes+1) + ">",
		" root[<!ELEMENT root " + strings.Repeat("(", sourceDTDMaxDepth+1) + "a" + strings.Repeat(")", sourceDTDMaxDepth+1) + ">]>",
	} {
		// Hide the optional callbacks to exercise the component's own ceilings.
		in := struct{ sourceDTDInput }{&sourceDTDTestInput{text: input}}
		info, err := parseSourceDTD(in)
		var unsupported *sourceXMLUnsupportedError
		if !errors.As(err, &unsupported) || !info.Unsupported {
			t.Errorf("capacity exceeded: info=%+v error=%T %v", info, err, err)
		}
	}
}

func TestSourceDTDNameRanges(t *testing.T) {
	starts := []rune{'A', 'Z', 'a', 'z', '_', 0xC0, 0xD6, 0xD8, 0xF6, 0xF8, 0x2FF, 0x370, 0x37D, 0x37F, 0x1FFF, 0x200C, 0x200D, 0x2070, 0x218F, 0x2C00, 0x2FEF, 0x3001, 0xD7FF, 0xF900, 0xFDCF, 0xFDF0, 0xFFFD, 0x10000, 0xEFFFF}
	for _, r := range starts {
		input := " " + string(r) + ":" + string(r) + ">"
		info, err := parseSourceDTD(&sourceDTDTestInput{text: input})
		if err != nil || info.Name != string(r)+":"+string(r) {
			t.Errorf("name start U+%04X: %+v %v", r, info, err)
		}
	}
	for _, r := range []rune{'-', '.', '0', '9', 0xB7, 0x300, 0x36F, 0x203F, 0x2040} {
		if _, err := parseSourceDTD(&sourceDTDTestInput{text: " a" + string(r) + ">"}); err != nil {
			t.Errorf("name continuation U+%04X: %v", r, err)
		}
		var syntax *xml.SyntaxError
		if _, err := parseSourceDTD(&sourceDTDTestInput{text: " " + string(r) + ">"}); !errors.As(err, &syntax) {
			t.Errorf("invalid initial U+%04X: %v", r, err)
		}
	}
	for _, r := range []rune{0xB6, 0xD7, 0xF7, 0x37E, 0x200B, 0x200E, 0x206F, 0x2190, 0x2BFF, 0x2FF0, 0x3000, 0xF8FF, 0xFDD0, 0xFDEF, 0xF0000} {
		var syntax *xml.SyntaxError
		if _, err := parseSourceDTD(&sourceDTDTestInput{text: " a" + string(r) + ">"}); !errors.As(err, &syntax) {
			t.Errorf("invalid continuation U+%04X: %v", r, err)
		}
	}
}

func TestSourceDTDAllOpaqueTextIsStreaming(t *testing.T) {
	for _, wrap := range []struct{ before, after string }{
		{` root SYSTEM '`, `'>`},
		{` root PUBLIC '`, `' 'none'>`},
		{` root[<!--`, `-->]>`},
		{` root[<?p `, `?>]>`},
		{` root[<!ATTLIST root a CDATA '`, `'>]>`},
	} {
		short := wrap.before + "abcd " + wrap.after
		long := wrap.before + strings.Repeat("abcd ", 100000) + wrap.after
		allocs := func(input string) float64 {
			return testing.AllocsPerRun(2, func() {
				if _, err := parseSourceDTD(&sourceDTDTestInput{text: input}); err != nil {
					panic(err)
				}
			})
		}
		if small, big := allocs(short), allocs(long); big > small+2 {
			t.Errorf("%s: short=%v long=%v", wrap.before, small, big)
		}
	}
}

type sourceDTDBufferTestInput struct {
	*sourceDTDTestInput
	limit, used, peak int64
}

func (in *sourceDTDBufferTestInput) ReserveDTDBytes(n int64) error {
	if n < 0 || in.limit > 0 && n > in.limit-in.used {
		return errSourceDTDTestLimit
	}
	in.used += n
	if in.used > in.peak {
		in.peak = in.used
	}
	return nil
}

func (in *sourceDTDBufferTestInput) ReleaseDTDBytes(n int64) {
	in.used -= n
	if in.used < 0 {
		panic("DTD buffer accounting underflow")
	}
}

func TestSourceDTDBufferReservations(t *testing.T) {
	for _, input := range []string{
		` root>`,
		` root[<?long_instruction_target data?><!ELEMENT root ((a,b)|c)>]>`,
		` root[<?xml?>]>`,
		` root[<!ELEMENT root ((a,b)|)>]>`,
		` root[<!ENTITY % p 'declaration'>%p;]>`,
	} {
		in := &sourceDTDBufferTestInput{sourceDTDTestInput: &sourceDTDTestInput{text: input}}
		info, _ := parseSourceDTD(in)
		if in.used != int64(len(info.Name)) {
			t.Errorf("%s: retained %d bytes, name needs %d", input, in.used, len(info.Name))
		}
		if in.peak == 0 {
			t.Errorf("%s: name storage was not reserved", input)
		}
	}
	for _, tc := range []struct {
		input string
		limit int64
	}{
		{` root>`, 31},                        // The initial capacity allocation must be admitted first.
		{` root>`, 35},                        // The final string overlaps its temporary 32-byte buffer.
		{` root[<!ELEMENT root (a)>]>`, 1027}, // The content stack overlaps the name.
	} {
		in := &sourceDTDBufferTestInput{sourceDTDTestInput: &sourceDTDTestInput{text: tc.input}, limit: tc.limit}
		info, err := parseSourceDTD(in)
		if !errors.Is(err, errSourceDTDTestLimit) {
			t.Errorf("limit %d: got %v", tc.limit, err)
		}
		if in.used != int64(len(info.Name)) {
			t.Errorf("limit %d: temporary buffer leaked (%d bytes)", tc.limit, in.used)
		}
		if in.peak > tc.limit {
			t.Errorf("limit %d: reserved %d bytes", tc.limit, in.peak)
		}
	}
	in := &sourceDTDBufferTestInput{sourceDTDTestInput: &sourceDTDTestInput{text: ` root[<!ELEMENT root (a)>]>`}, limit: 1028}
	if _, err := parseSourceDTD(in); err != nil {
		t.Errorf("exact buffer limit: %v", err)
	}
}
