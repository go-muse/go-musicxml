package musicxml

import (
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/go-muse/go-musicxml/validation"
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
	input                    sourceDTDInput
	info                     sourceDTDInfo
	err                      error
	store                    *sourceDTDStore
	capture                  *sourceDTDText
	origin                   sourceDTDOrigin
	literalGeneralReferences bool
	parameters               []*sourceDTDDeclaration
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
		publicID, systemID := p.externalID(false)
		if p.store != nil {
			p.store.state.ExternalSubsetUnread = true
			p.store.state.PublicID = publicID
			p.store.state.SystemID = systemID
		}
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
		case sourceDTDParameterBoundary:
			input, ok := p.input.(sourceDTDParameterInput)
			if p.store == nil || !ok || len(p.parameters) == 0 || !input.DTDInParameter() {
				p.fail("unexpected parameter entity boundary in DTD")
				return
			}
			d := p.parameters[len(p.parameters)-1]
			if p.err = input.FinishDTDParameter(); p.err != nil {
				return
			}
			p.parameters = p.parameters[:len(p.parameters)-1]
			p.err = p.store.parameterRead(d)
		case ']':
			if input, ok := p.input.(sourceDTDParameterInput); ok && input.DTDInParameter() {
				p.fail("parameter entity cannot close the caller DTD subset")
				return
			}
			return
		case '%':
			origin := p.declarationOrigin()
			p.read()
			name := p.name(sourceDTDNCName, p.store != nil)
			p.expect(';')
			if p.err != nil {
				return
			}
			p.info.HasParameterReferences = true
			if p.store == nil {
				p.unsupported("parameter-entity replacement in an internal DTD subset")
				return
			}
			if position, ok := p.input.(interface{ DTDPosition() sourceXMLPosition }); ok && origin.SpanAvailable {
				origin.Span.End = position.DTDPosition()
			}
			declaration, known := p.store.Entity(name, true)
			if input, ok := p.input.(sourceDTDParameterInput); ok && known && !declaration.External {
				next, err := p.store.growPointers(p.parameters)
				if err != nil {
					p.err = err
					return
				}
				p.parameters = next
				p.err = input.PushDTDParameter(declaration, origin)
				p.releaseBytes(int64(len(name)))
				if p.err != nil {
					return
				}
				p.parameters = append(p.parameters, declaration)
				continue
			}
			p.info.Unsupported = true
			p.info.WellFormednessIncomplete = true
			p.store.state.ParameterEntityUnread = true
			if p.store.state.FirstUnreadParameter == "" {
				p.store.state.FirstUnreadParameter = name
			} else {
				p.releaseBytes(int64(len(name)))
			}
			p.store.state.BindingCutoff = p.store.state.Standalone != sourceDTDStandaloneYes
		case '<':
			p.origin = p.declarationOrigin()
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
		d := sourceDTDDeclaration{Kind: sourceDTDNotationDeclaration, Origin: p.origin}
		d.Name = p.name(sourceDTDNCName, p.store != nil)
		p.requireSpace()
		d.PublicID, d.SystemID = p.externalID(true)
		p.endDeclaration()
		p.collect(d)
	default:
		p.fail("unknown markup declaration in internal DTD subset")
	}
}

func (p *sourceDTDParser) element() {
	d := sourceDTDDeclaration{Kind: sourceDTDElementDeclaration, Origin: p.origin}
	d.Name = p.name(sourceDTDQName, p.store != nil)
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
	p.collect(d)
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
	element := p.name(sourceDTDQName, p.store != nil)
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
		// An attribute's origin spans its own definition, not a fabricated physical
		// attribute on an instance element or the entire enclosing ATTLIST.
		d := sourceDTDDeclaration{Kind: sourceDTDAttributeDeclaration, Element: element, Origin: p.declarationOrigin()}
		d.Name = p.name(sourceDTDQName, p.store != nil)
		p.requireSpace()
		d.AttributeType, d.RawType = p.attributeType()
		p.requireSpace()
		d.DefaultKind, d.RawValue = p.attributeDefault()
		d.HasGeneralReferences = p.literalGeneralReferences
		p.collect(d)
	}
}

func (p *sourceDTDParser) attributeType() (kind, raw string) {
	var text sourceDTDText
	if p.store != nil {
		text.store = p.store
		p.capture = &text
		defer text.close()
		defer func() { p.capture = nil }()
	}
	if p.peek() == '(' {
		kind = "ENUMERATION"
		p.enumeration(sourceDTDNmtoken)
	} else {
		switch p.keyword() {
		case "CDATA":
			kind = "CDATA"
		case "ID":
			kind = "ID"
		case "IDREF":
			kind = "IDREF"
		case "IDREFS":
			kind = "IDREFS"
		case "ENTITY":
			kind = "ENTITY"
		case "ENTITIES":
			kind = "ENTITIES"
		case "NMTOKEN":
			kind = "NMTOKEN"
		case "NMTOKENS":
			kind = "NMTOKENS"
		case "NOTATION":
			kind = "NOTATION"
			p.requireSpace()
			p.enumeration(sourceDTDNCName)
		default:
			p.fail("unknown DTD attribute type")
		}
	}
	if p.store != nil && p.err == nil {
		raw, p.err = text.string()
	}
	return kind, raw
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

func (p *sourceDTDParser) attributeDefault() (sourceDTDDefaultKind, string) {
	kind := sourceDTDDefaultValue
	p.literalGeneralReferences = false
	if p.peek() == '#' {
		p.read()
		switch p.keyword() {
		case "REQUIRED":
			return sourceDTDDefaultRequired, ""
		case "IMPLIED":
			return sourceDTDDefaultImplied, ""
		case "FIXED":
			kind = sourceDTDDefaultFixed
			p.requireSpace()
		default:
			p.fail("unknown DTD attribute default declaration")
			return "", ""
		}
	}
	raw, _ := p.quoted(sourceDTDAttributeLiteral)
	return kind, raw
}

func (p *sourceDTDParser) entity() {
	d := sourceDTDDeclaration{Kind: sourceDTDGeneralEntityDeclaration, Origin: p.origin}
	d.Parameter = p.peek() == '%'
	if d.Parameter {
		d.Kind = sourceDTDParameterEntityDeclaration
		p.read()
		p.requireSpace()
	}
	d.Name = p.name(sourceDTDNCName, p.store != nil)
	p.requireSpace()
	if r := p.peek(); r == '\'' || r == '"' {
		d.RawValue, d.Replacement = p.quoted(sourceDTDEntityLiteral)
		d.HasGeneralReferences = p.literalGeneralReferences
		p.endDeclaration()
		p.collect(d)
		return
	}
	d.External = true
	d.PublicID, d.SystemID = p.externalID(false)
	space := p.space()
	if p.peek() == 'N' && !d.Parameter {
		if !space {
			p.fail("NDATA requires preceding whitespace")
			return
		}
		p.literal("NDATA")
		p.requireSpace()
		d.Notation = p.name(sourceDTDNCName, p.store != nil)
	}
	p.endDeclaration()
	p.collect(d)
}

// Public identifiers may omit their system literal only in NOTATION.
func (p *sourceDTDParser) externalID(publicOnly bool) (publicID, systemID string) {
	switch p.keyword() {
	case "SYSTEM":
		p.requireSpace()
		systemID, _ = p.quoted(sourceDTDSystemLiteral)
	case "PUBLIC":
		p.requireSpace()
		publicID, _ = p.quoted(sourceDTDPublicLiteral)
		space := p.space()
		if r := p.peek(); publicOnly && r != '\'' && r != '"' {
			return publicID, systemID
		}
		if !space {
			p.fail("PUBLIC system literal requires whitespace")
			return publicID, systemID
		}
		systemID, _ = p.quoted(sourceDTDSystemLiteral)
	default:
		p.fail("expected SYSTEM or PUBLIC external identifier")
	}
	return publicID, systemID
}

type sourceDTDQuotedKind uint8

const (
	sourceDTDSystemLiteral sourceDTDQuotedKind = iota
	sourceDTDPublicLiteral
	sourceDTDEntityLiteral
	sourceDTDAttributeLiteral
)

func (p *sourceDTDParser) quoted(kind sourceDTDQuotedKind) (rawValue, replacementValue string) {
	p.literalGeneralReferences = false
	quote := p.read()
	if quote != '\'' && quote != '"' {
		p.fail("expected a quoted DTD literal")
		return "", ""
	}
	var raw, replacement sourceDTDText
	if p.store != nil {
		raw.store, replacement.store = p.store, p.store
		p.capture = &raw
		defer raw.close()
		defer replacement.close()
		defer func() { p.capture = nil }()
	}
	for p.err == nil {
		r := p.peek()
		if r == quote {
			p.capture = nil
			p.read()
			break
		}
		start := len(raw.value)
		r = p.read()
		if p.err != nil {
			break
		}
		numeric := false
		decoded := rune(0)
		switch kind {
		case sourceDTDPublicLiteral:
			if !sourceDTDPublicChar(r) {
				p.fail("invalid character in public identifier")
			}
		case sourceDTDEntityLiteral, sourceDTDAttributeLiteral:
			if r == '&' {
				decoded, numeric = p.reference()
				if !numeric {
					p.literalGeneralReferences = true
				}
			} else if kind == sourceDTDEntityLiteral && r == '%' {
				p.fail("parameter-entity reference inside internal DTD declaration")
			} else if kind == sourceDTDAttributeLiteral && r == '<' {
				p.fail("literal < in DTD attribute default")
			}
		}
		if p.store != nil && kind == sourceDTDEntityLiteral && p.err == nil {
			// XML 4.5 constructs entity replacement text now: decode numeric character
			// references once, but retain general-reference spellings for their eventual
			// recognition context. RawValue separately preserves the lexical spelling.
			if numeric {
				p.err = replacement.appendRune(decoded)
			} else {
				p.err = replacement.append(raw.value[start:])
			}
		}
	}
	if p.store != nil && p.err == nil {
		rawValue, p.err = raw.string()
		if p.err == nil && kind == sourceDTDEntityLiteral {
			replacementValue, p.err = replacement.string()
		}
	}
	return rawValue, replacementValue
}

// reference checks syntax and character legality without expanding entities or
// retaining arbitrarily many leading zeroes in a numeric character reference.
func (p *sourceDTDParser) reference() (rune, bool) {
	if p.peek() != '#' {
		p.name(sourceDTDNCName, false)
		p.expect(';')
		return 0, false
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
			return rune(value), true
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
			return 0, false
		}
		if value > (0x10FFFF-digit)/base {
			p.fail("XML character reference exceeds Unicode range")
			return 0, false
		}
		value = value*base + digit
		hasDigit = true
		p.read()
	}
	return 0, false
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
	if p.store != nil {
		p.err = p.store.reserve(n)
	} else if budget, ok := p.input.(sourceDTDBufferBudget); ok && n != 0 {
		p.err = budget.ReserveDTDBytes(n)
	}
	return p.err == nil
}

func (p *sourceDTDParser) releaseBytes(n int64) {
	if p.store != nil {
		p.store.release(n)
	} else if budget, ok := p.input.(sourceDTDBufferBudget); ok && n != 0 {
		budget.ReleaseDTDBytes(n)
	}
}

func (p *sourceDTDParser) checkNameBytes(n int64) {
	if p.err != nil {
		return
	}
	if p.store != nil {
		p.err = p.store.account.Check(validation.BudgetScalarBytes, n)
		if p.err != nil {
			return
		}
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
	if p.store != nil {
		p.err = p.store.check()
		if p.err != nil {
			return 0
		}
	}
	r, err := p.input.Peek()
	p.inputError(err)
	return r
}

func (p *sourceDTDParser) read() rune {
	if p.err != nil {
		return 0
	}
	if p.store != nil {
		p.err = p.store.check()
		if p.err != nil {
			return 0
		}
	}
	r, err := p.input.Read()
	p.inputError(err)
	if p.capture != nil && p.err == nil {
		p.err = p.capture.appendRune(r)
	}
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

// DTDPosition is optional because grammar-only callers historically did not
// provide coordinates. SpanAvailable explicitly distinguishes absent evidence
// from a real declaration whose offset could happen to be zero.
func (p *sourceDTDParser) declarationOrigin() sourceDTDOrigin {
	origin := sourceDTDOrigin{Region: sourceDTDInternalSubset, AncestryComplete: true}
	if p.store != nil {
		if input, ok := p.input.(interface {
			DTDParameterOrigin() (sourceDTDOrigin, error)
		}); ok {
			origin, p.err = input.DTDParameterOrigin()
			return origin
		}
		if input, ok := p.input.(sourceDTDParameterInput); ok && input.DTDInParameter() {
			return sourceDTDOrigin{Region: sourceDTDParameterEntityRegion, ExternalMarkup: true}
		}
	}
	if position, ok := p.input.(interface{ DTDPosition() sourceXMLPosition }); ok {
		origin.SpanAvailable = true
		origin.Span.Start = position.DTDPosition()
	}
	return origin
}
func (p *sourceDTDParser) collect(d sourceDTDDeclaration) {
	if p.store == nil || p.err != nil {
		return
	}
	if position, ok := p.input.(interface{ DTDPosition() sourceXMLPosition }); ok && d.Origin.SpanAvailable {
		d.Origin.Span.End = position.DTDPosition()
	}
	p.err = p.store.add(d)
}

// XML 4.1's direct ATTLIST-default order rule is a WFC, distinct from
// the validity-only ordering of indirect references and ordinary EntityValue
// forward references. Check every read definition, including ignored bindings
// and defaults which no instance element ever uses. Unknown unread declarations
// remain unavailable evidence; an actually known later binding proves the order
// violation without fetching anything. Normalization/indirect WFCs remain for
// the effective-attribute layer, which must also inspect unused defaults.
func (p *sourceDTDParser) checkDefaultReferences() {
	for _, d := range p.store.declarations {
		if d.Kind != sourceDTDAttributeDeclaration || !d.HasGeneralReferences {
			continue
		}
		raw := d.RawValue
		for i := 0; i < len(raw); i++ {
			if p.err = p.store.check(); p.err != nil {
				return
			}
			if raw[i] != '&' || raw[i+1] == '#' {
				continue
			}
			start := i + 1
			i = start
			for raw[i] != ';' {
				if p.err = p.store.check(); p.err != nil {
					return
				}
				i++
			}
			if p.err = p.store.account.Charge(validation.BudgetEvents, 1); p.err != nil {
				return
			}
			name := raw[start:i]
			switch name {
			case "amp", "lt", "gt", "apos", "quot":
				continue
			}
			entity, known := p.store.Entity(name, false)
			if !known {
				if !d.Origin.ExternalMarkup && (p.store.state.Standalone == sourceDTDStandaloneYes || (!p.store.state.ExternalSubsetUnread && !p.info.HasParameterReferences)) {
					p.fail("undeclared general entity in DTD attribute default")
					return
				}
				continue
			}
			if p.store.state.Standalone == sourceDTDStandaloneYes && !d.Origin.ExternalMarkup && entity.Origin.ExternalMarkup {
				p.fail("standalone DTD default reference requires a declaration outside parameter entities")
				return
			}
			if entity.Origin.Sequence >= d.Origin.Sequence {
				p.fail("general entity declaration must precede its direct reference in a DTD attribute default")
				return
			}
			if entity.External {
				p.fail("external entity reference in DTD attribute default")
				return
			}
		}
	}
}

// The same scanner may elect whole-declaration internal PE frames. A boundary
// sentinel is legal only in subset's between-declarations dispatch; Read must
// reject it so names, literals, comments and declarations cannot borrow caller
// bytes. An unread entity uses the ordinary XML 5.1 binding cutoff instead.
type sourceDTDParameterInput interface {
	PushDTDParameter(*sourceDTDDeclaration, sourceDTDOrigin) error
	FinishDTDParameter() error
	DTDInParameter() bool
}
