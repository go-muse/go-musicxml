package musicxml

import (
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

// sourceDTDInput is the existing source cursor, never a second XML parser.
// Read and Peek must check XML characters, input/token budgets and cancellation.
// They must not fetch external identifiers. SyntaxError preserves source position.
type sourceDTDInput interface {
	Peek() (rune, error)
	Read() (rune, error)
	SyntaxError(string) error
}

// sourceDTDInfo distinguishes checked local declaration grammar from unavailable
// DTD semantics. Unsupported does not imply that the declaration is malformed.
// Even when parsing succeeds, declared defaults/entities are not interpreted.
type sourceDTDInfo struct {
	// WellFormednessIncomplete distinguishes unassessed declaration WFCs
	// (for example entity declaration order/recursion) from known token grammar.
	WellFormednessIncomplete bool
	Name                     string
	ExternalSubset           bool
	HasDeclarations          bool
	HasElementDeclarations   bool
	HasAttributeDeclarations bool
	HasEntityDeclarations    bool
	HasNotationDeclarations  bool
	HasParameterReferences   bool
	Unsupported              bool
}

// These hard ceilings also bound callers without the optional budget methods.
// The parent reader may impose lower limits and return its typed resource error.
const (
	sourceDTDMaxNameBytes = 1 << 20
	sourceDTDMaxDepth     = 1024
)

type sourceDTDParser struct {
	input sourceDTDInput
	info  sourceDTDInfo
	err   error
}

// parseSourceDTD starts immediately after the literal <!DOCTYPE. It consumes
// exactly through the closing >, retaining only the document type name and a
// fixed-size content-model stack. Grammar follows XML 1.0 Fifth Edition, sections
// 2.8, 3.2, 3.3, 4.1, 4.2 and 4.7; namespace name rules follow Namespaces 1.0 §7.
// https://www.w3.org/TR/REC-xml/ and https://www.w3.org/TR/xml-names/#Conformance
func parseSourceDTD(input sourceDTDInput) (sourceDTDInfo, error) {
	p := sourceDTDParser{input: input}
	p.doctype()
	return p.info, p.err
}

func (p *sourceDTDParser) doctype() {
	p.requireSpace()
	p.info.Name = p.name(sourceDTDQName, true)
	space := p.space()
	if r := p.peek(); r == 'S' || r == 'P' {
		if !space {
			p.fail("DOCTYPE external identifier requires whitespace")
			return
		}
		p.info.ExternalSubset = true
		p.info.Unsupported = true
		p.externalID(false)
		p.space()
	}
	if p.peek() == '[' {
		p.read()
		p.subset()
		p.expect(']')
		p.space()
	}
	p.expect('>')
}

func (p *sourceDTDParser) subset() {
	for p.err == nil {
		p.space()
		switch p.peek() {
		case ']':
			return
		case '%':
			p.read()
			p.name(sourceDTDNCName, false)
			p.expect(';')
			if p.err == nil {
				p.info.HasParameterReferences = true
				p.unsupported("parameter-entity replacement in an internal DTD subset")
			}
		case '<':
			p.read()
			switch p.peek() {
			case '?':
				p.read()
				p.processingInstruction()
			case '!':
				p.read()
				if p.peek() == '-' {
					p.literal("--")
					p.comment()
					continue
				}
				p.declaration()
			default:
				p.fail("expected a declaration, comment or processing instruction in DTD")
			}
		default:
			p.fail("unexpected character in internal DTD subset")
		}
	}
}

func (p *sourceDTDParser) declaration() {
	keyword := p.keyword()
	p.requireSpace()
	p.info.HasDeclarations = true
	switch keyword {
	case "ELEMENT":
		p.info.HasElementDeclarations = true
		p.element()
	case "ATTLIST":
		p.info.HasAttributeDeclarations = true
		p.info.WellFormednessIncomplete = true
		p.info.Unsupported = true
		p.attlist()
	case "ENTITY":
		p.info.HasEntityDeclarations = true
		p.info.WellFormednessIncomplete = true
		p.info.Unsupported = true
		p.entity()
	case "NOTATION":
		p.info.HasNotationDeclarations = true
		p.name(sourceDTDNCName, false)
		p.requireSpace()
		p.externalID(true)
		p.endDeclaration()
	default:
		p.fail("unknown markup declaration in internal DTD subset")
	}
}

func (p *sourceDTDParser) element() {
	p.name(sourceDTDQName, false)
	p.requireSpace()
	if p.peek() != '(' {
		keyword := p.keyword()
		if keyword != "EMPTY" && keyword != "ANY" {
			p.fail("expected EMPTY, ANY or an element content model")
		}
	} else {
		p.read()
		p.space()
		if p.peek() == '#' {
			p.checkDepth(1)
			p.mixed()
		} else {
			p.children()
		}
	}
	p.endDeclaration()
}

// children recognizes nested sequence/choice groups without recursive calls.
// Each live group stores only its separator; one state bit belongs to the top.
func (p *sourceDTDParser) children() {
	depth := 1
	p.checkDepth(int64(depth))
	if !p.reserveBytes(sourceDTDMaxDepth) {
		return
	}
	defer p.releaseBytes(sourceDTDMaxDepth)
	var separators [sourceDTDMaxDepth]byte
	wantParticle := true
	for p.err == nil {
		p.space()
		if wantParticle {
			if p.peek() == '(' {
				p.read()
				p.checkDepth(int64(depth + 1))
				if p.err != nil {
					return
				}
				separators[depth] = 0
				depth++
				continue
			}
			p.name(sourceDTDQName, false)
			p.occurrence()
			wantParticle = false
			continue
		}
		switch r := p.peek(); r {
		case '|', ',':
			if separators[depth-1] != 0 && separators[depth-1] != byte(r) {
				p.fail("cannot mix choice and sequence separators in one DTD group")
				return
			}
			separators[depth-1] = byte(r)
			p.read()
			wantParticle = true
		case ')':
			p.read()
			p.occurrence()
			depth--
			if depth == 0 {
				return
			}
		default:
			p.fail("expected a separator or closing parenthesis in DTD content model")
		}
	}
}

func (p *sourceDTDParser) mixed() {
	p.literal("#PCDATA")
	p.space()
	hasNames := false
	for p.err == nil && p.peek() == '|' {
		p.read()
		p.space()
		p.name(sourceDTDQName, false)
		hasNames = true
		p.space()
	}
	p.expect(')')
	if hasNames {
		p.expect('*')
	} else if p.peek() == '*' {
		p.read()
	}
}

func (p *sourceDTDParser) occurrence() {
	if r := p.peek(); r == '?' || r == '*' || r == '+' {
		p.read()
	}
}

func (p *sourceDTDParser) attlist() {
	p.name(sourceDTDQName, false)
	for p.err == nil {
		space := p.space()
		if p.peek() == '>' {
			p.read()
			return
		}
		if !space {
			p.fail("attribute definitions in ATTLIST require whitespace")
			return
		}
		p.name(sourceDTDQName, false)
		p.requireSpace()
		p.attributeType()
		p.requireSpace()
		p.attributeDefault()
	}
}

func (p *sourceDTDParser) attributeType() {
	if p.peek() == '(' {
		p.enumeration(sourceDTDNmtoken)
		return
	}
	switch p.keyword() {
	case "CDATA", "ID", "IDREF", "IDREFS", "ENTITY", "ENTITIES", "NMTOKEN", "NMTOKENS":
	case "NOTATION":
		p.requireSpace()
		p.enumeration(sourceDTDNCName)
	default:
		p.fail("unknown DTD attribute type")
	}
}

func (p *sourceDTDParser) enumeration(kind sourceDTDNameKind) {
	p.expect('(')
	p.space()
	p.name(kind, false)
	p.space()
	for p.err == nil && p.peek() == '|' {
		p.read()
		p.space()
		p.name(kind, false)
		p.space()
	}
	p.expect(')')
}

func (p *sourceDTDParser) attributeDefault() {
	if p.peek() == '#' {
		p.read()
		switch p.keyword() {
		case "REQUIRED", "IMPLIED":
			return
		case "FIXED":
			p.requireSpace()
		default:
			p.fail("unknown DTD attribute default declaration")
			return
		}
	}
	p.quoted(sourceDTDAttributeLiteral)
}

func (p *sourceDTDParser) entity() {
	parameter := p.peek() == '%'
	if parameter {
		p.read()
		p.requireSpace()
	}
	p.name(sourceDTDNCName, false)
	p.requireSpace()
	if r := p.peek(); r == '\'' || r == '"' {
		p.quoted(sourceDTDEntityLiteral)
		p.endDeclaration()
		return
	}
	p.externalID(false)
	space := p.space()
	if p.peek() == 'N' && !parameter {
		if !space {
			p.fail("NDATA requires preceding whitespace")
			return
		}
		p.literal("NDATA")
		p.requireSpace()
		p.name(sourceDTDNCName, false)
	}
	p.endDeclaration()
}

// Public identifiers may omit their system literal only in NOTATION.
func (p *sourceDTDParser) externalID(publicOnly bool) {
	switch p.keyword() {
	case "SYSTEM":
		p.requireSpace()
		p.quoted(sourceDTDSystemLiteral)
	case "PUBLIC":
		p.requireSpace()
		p.quoted(sourceDTDPublicLiteral)
		space := p.space()
		if r := p.peek(); publicOnly && r != '\'' && r != '"' {
			return
		}
		if !space {
			p.fail("PUBLIC system literal requires whitespace")
			return
		}
		p.quoted(sourceDTDSystemLiteral)
	default:
		p.fail("expected SYSTEM or PUBLIC external identifier")
	}
}

type sourceDTDQuotedKind uint8

const (
	sourceDTDSystemLiteral sourceDTDQuotedKind = iota
	sourceDTDPublicLiteral
	sourceDTDEntityLiteral
	sourceDTDAttributeLiteral
)

func (p *sourceDTDParser) quoted(kind sourceDTDQuotedKind) {
	quote := p.read()
	if quote != '\'' && quote != '"' {
		p.fail("expected a quoted DTD literal")
		return
	}
	for p.err == nil {
		r := p.read()
		if p.err != nil || r == quote {
			return
		}
		switch kind {
		case sourceDTDPublicLiteral:
			if !sourceDTDPublicChar(r) {
				p.fail("invalid character in public identifier")
			}
		case sourceDTDEntityLiteral, sourceDTDAttributeLiteral:
			if r == '&' {
				p.reference()
			} else if kind == sourceDTDEntityLiteral && r == '%' {
				p.fail("parameter-entity reference inside internal DTD declaration")
			} else if kind == sourceDTDAttributeLiteral && r == '<' {
				p.fail("literal < in DTD attribute default")
			}
		}
	}
}

// reference checks syntax and character legality without expanding entities or
// retaining arbitrarily many leading zeroes in a numeric character reference.
func (p *sourceDTDParser) reference() {
	if p.peek() != '#' {
		p.name(sourceDTDNCName, false)
		p.expect(';')
		return
	}
	p.read()
	base := uint32(10)
	if p.peek() == 'x' {
		base = 16
		p.read()
	}
	value := uint32(0)
	hasDigit := false
	for p.err == nil {
		r := p.peek()
		if r == ';' {
			p.read()
			if !hasDigit || !sourceXMLChar(rune(value)) {
				p.fail("invalid XML character reference in DTD")
			}
			return
		}
		var digit uint32
		switch {
		case r >= '0' && r <= '9':
			digit = uint32(r - '0')
		case base == 16 && r >= 'a' && r <= 'f':
			digit = uint32(r - 'a' + 10)
		case base == 16 && r >= 'A' && r <= 'F':
			digit = uint32(r - 'A' + 10)
		default:
			p.fail("invalid numeric character reference in DTD")
			return
		}
		if value > (0x10FFFF-digit)/base {
			p.fail("XML character reference exceeds Unicode range")
			return
		}
		value = value*base + digit
		hasDigit = true
		p.read()
	}
}

func (p *sourceDTDParser) processingInstruction() {
	target := p.name(sourceDTDNCName, true)
	reserved := strings.EqualFold(target, "xml")
	p.releaseBytes(int64(len(target)))
	if reserved {
		p.fail("reserved XML processing instruction target in DTD")
		return
	}
	if p.peek() == '?' {
		p.read()
		p.expect('>')
		return
	}
	p.requireSpace()
	for p.err == nil {
		if p.read() == '?' && p.peek() == '>' {
			p.read()
			return
		}
	}
}

func (p *sourceDTDParser) comment() {
	for p.err == nil {
		if p.read() == '-' && p.peek() == '-' {
			p.read()
			p.expect('>')
			return
		}
	}
}

type sourceDTDNameKind uint8

const (
	sourceDTDQName sourceDTDNameKind = iota
	sourceDTDNCName
	sourceDTDNmtoken
)

// name checks an entire name even when its value is not retained. Names in DTD
// declarations are lexical; the DTD itself cannot establish prefix bindings.
func (p *sourceDTDParser) name(kind sourceDTDNameKind, retain bool) string {
	var value []byte
	defer func() { p.releaseBytes(int64(cap(value))) }()
	var bytes int64
	needStart := kind != sourceDTDNmtoken
	colon := false
	for p.err == nil {
		r := p.peek()
		if needStart {
			if !sourceDTDNameStart(r) {
				p.fail("expected XML name in DTD")
				break
			}
		} else if r == ':' && kind == sourceDTDQName {
			if colon {
				p.fail("multiple colons in DTD qualified name")
				break
			}
			colon = true
		} else if !sourceDTDNameChar(r) && !(r == ':' && kind == sourceDTDNmtoken) {
			break
		}
		bytes += int64(utf8.RuneLen(r))
		p.checkNameBytes(bytes)
		if p.err != nil {
			break
		}
		p.read()
		if retain {
			if bytes > int64(cap(value)) {
				capacity := cap(value) * 2
				if capacity < 32 {
					capacity = 32
				}
				if capacity > sourceDTDMaxNameBytes {
					capacity = sourceDTDMaxNameBytes
				}
				if !p.reserveBytes(int64(capacity)) {
					break
				}
				next := make([]byte, len(value), capacity)
				copy(next, value)
				p.releaseBytes(int64(cap(value)))
				value = next
			}
			value = utf8.AppendRune(value, r)
		}
		needStart = r == ':' && kind == sourceDTDQName
	}
	if bytes == 0 {
		p.fail("expected XML name or name token in DTD")
	}
	if !retain || p.err != nil || !p.reserveBytes(int64(len(value))) {
		return ""
	}
	return string(value)
}

// keyword is a bounded ASCII grammar dispatch token, not an arbitrary XML name.
func (p *sourceDTDParser) keyword() string {
	var word [9]byte
	n := 0
	for p.err == nil {
		r := p.peek()
		if r < 'A' || r > 'Z' {
			break
		}
		if n == len(word) {
			p.fail("unknown DTD grammar keyword")
			return ""
		}
		word[n] = byte(p.read())
		n++
	}
	if n == 0 {
		p.fail("expected DTD grammar keyword")
	}
	return string(word[:n])
}

// The optional pair tracks temporary name storage before each allocation.
// Returned names stay charged to the caller's current token until it releases
// that token or transfers the name into persistent declaration metadata.
type sourceDTDBufferBudget interface {
	ReserveDTDBytes(int64) error
	ReleaseDTDBytes(int64)
}

func (p *sourceDTDParser) reserveBytes(n int64) bool {
	if p.err != nil {
		return false
	}
	if budget, ok := p.input.(sourceDTDBufferBudget); ok && n != 0 {
		p.err = budget.ReserveDTDBytes(n)
	}
	return p.err == nil
}

func (p *sourceDTDParser) releaseBytes(n int64) {
	if budget, ok := p.input.(sourceDTDBufferBudget); ok && n != 0 {
		budget.ReleaseDTDBytes(n)
	}
}

func (p *sourceDTDParser) checkNameBytes(n int64) {
	if p.err != nil {
		return
	}
	if check, ok := p.input.(interface{ CheckDTDNameBytes(int64) error }); ok {
		p.err = check.CheckDTDNameBytes(n)
	}
	if p.err == nil && n > sourceDTDMaxNameBytes {
		p.unsupported("DTD name exceeds the bounded reader capacity")
	}
}

func (p *sourceDTDParser) checkDepth(n int64) {
	if p.err != nil {
		return
	}
	if check, ok := p.input.(interface{ CheckDTDDepth(int64) error }); ok {
		p.err = check.CheckDTDDepth(n)
	}
	if p.err == nil && n > sourceDTDMaxDepth {
		p.unsupported("DTD content model exceeds the bounded nesting capacity")
	}
}

func (p *sourceDTDParser) endDeclaration() {
	p.space()
	p.expect('>')
}

func (p *sourceDTDParser) space() bool {
	found := false
	for p.err == nil {
		switch p.peek() {
		case ' ', '\t', '\r', '\n':
			found = true
			p.read()
		default:
			return found
		}
	}
	return found
}

func (p *sourceDTDParser) requireSpace() {
	if !p.space() {
		p.fail("expected whitespace in DTD declaration")
	}
}

func (p *sourceDTDParser) literal(s string) {
	for _, r := range s {
		p.expect(r)
	}
}

func (p *sourceDTDParser) expect(want rune) {
	if got := p.read(); got != want {
		p.fail("unexpected delimiter in DTD declaration")
	}
}

func (p *sourceDTDParser) peek() rune {
	if p.err != nil {
		return 0
	}
	r, err := p.input.Peek()
	p.inputError(err)
	return r
}

func (p *sourceDTDParser) read() rune {
	if p.err != nil {
		return 0
	}
	r, err := p.input.Read()
	p.inputError(err)
	return r
}

func (p *sourceDTDParser) inputError(err error) {
	if errors.Is(err, io.EOF) {
		p.fail("unexpected EOF in DOCTYPE declaration")
	} else if err != nil {
		p.err = err
	}
}

func (p *sourceDTDParser) fail(message string) {
	if p.err == nil {
		p.err = p.input.SyntaxError(message)
	}
}

func (p *sourceDTDParser) unsupported(feature string) {
	if p.err == nil {
		p.info.Unsupported = true
		p.err = &sourceXMLUnsupportedError{Feature: feature, Reason: "xml-dtd-unavailable"}
	}
}

func sourceDTDNameStart(r rune) bool { return r != ':' && sourceXMLNameStart(r) }
func sourceDTDNameChar(r rune) bool  { return r != ':' && sourceXMLNameChar(r) }

func sourceDTDPublicChar(r rune) bool {
	return r == ' ' || r == '\r' || r == '\n' ||
		r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
		strings.ContainsRune("-'()+,./:=?;!*#@$_%", r)
}
