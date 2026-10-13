package musicxml

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/go-muse/go-musicxml/validation"
)

type sourceXMLConfig struct {
	Context     context.Context
	Account     *validation.Account
	MaxDepth    int
	ByteOwner   sourceByteOwner
	InternalDTD bool
}
type sourceXMLAttribute struct {
	NameSpan, ValueSpan  sourceXMLSpan
	NamespaceDeclaration bool
	Origin               *sourceXMLOrigin
}
type sourceXMLTokenMeta struct {
	Span                       sourceXMLSpan
	Attributes                 []sourceXMLAttribute
	CDATA, Reference, EmptyEnd bool
	Origin                     *sourceXMLOrigin
}
type sourceXMLSummary struct {
	BytesRead, BytesConsumed                      int64
	ReadPrefixSHA256                              [32]byte
	DigestComplete, InputComplete, SyntaxComplete bool
	Encoding                                      string
	DTD                                           sourceDTDInfo
}
type sourceXMLBinding struct {
	uri   string
	bound bool
}
type sourceXMLUndo struct {
	prefix   string
	previous sourceXMLBinding
}
type sourceXMLFrame struct {
	name     xml.Name
	undo     []sourceXMLUndo
	retained int64
}

const (
	sourceXMLNameScratchBytes  = 64
	sourceXMLValueScratchBytes = 256
	sourceXMLReaderBytes       = 512 + sourceXMLNameScratchBytes + sourceXMLValueScratchBytes
)

type sourceXMLTokenReader struct {
	// Names become owned strings before another name is scanned. Value tokens
	// remain borrowed until Token advances, and entity names use separate storage.
	nameScratch                                                [sourceXMLNameScratchBytes]byte
	valueScratch                                               [sourceXMLValueScratchBytes]byte
	cursor                                                     *sourceXMLCursor
	entity                                                     *sourceXMLEntityFrame
	account                                                    *validation.Account
	internalDTD                                                bool
	maxDepth                                                   int64
	peakDepth                                                  int
	metadata                                                   sourceXMLTokenMeta
	tokenOwned, retained, mapOwned                             int64
	frames                                                     []sourceXMLFrame
	namespaces                                                 map[string]sourceXMLBinding
	needEnd                                                    bool
	seenToken, seenRoot, rootClosed, doctype, complete, closed bool
	dtd                                                        sourceDTDInfo
	dtdStore                                                   *sourceDTDStore
	standalone                                                 sourceDTDStandalone
	dtdBytes                                                   int64
	dtdOriginScratch                                           []sourceDTDOriginStep
	err                                                        error
}

func newSourceXMLTokenReader(input io.Reader, config sourceXMLConfig) (*sourceXMLTokenReader, error) {
	if config.InternalDTD && config.Account == nil {
		return nil, &validation.ConfigurationError{Field: "Account", Reason: "internal DTD mode requires the shared account"}
	}
	if config.MaxDepth < 0 {
		return nil, ErrInvalidDecodeOptions
	}
	cursor, err := newSourceXMLCursorWithByteOwner(input, config.Context, config.Account, config.ByteOwner)
	if err != nil {
		return nil, err
	}
	if config.Account != nil {
		if err := config.Account.Charge(validation.BudgetBufferedBytes, sourceXMLReaderBytes); err != nil {
			cursor.close()
			return nil, err
		}
	}
	maximum := int64(^uint64(0) >> 1)
	if config.MaxDepth != 0 {
		maximum = int64(config.MaxDepth)
	}
	if config.Account != nil && config.Account.Limits().MaxDepth < maximum {
		maximum = config.Account.Limits().MaxDepth
	}
	return &sourceXMLTokenReader{cursor: cursor, account: config.Account, internalDTD: config.InternalDTD, maxDepth: maximum, retained: sourceXMLReaderBytes}, nil
}
func (r *sourceXMLTokenReader) Metadata() sourceXMLTokenMeta { return r.metadata }
func (r *sourceXMLTokenReader) Summary() sourceXMLSummary {
	return sourceXMLSummary{BytesRead: r.cursor.reads, BytesConsumed: r.cursor.position.ByteOffset, ReadPrefixSHA256: r.cursor.readDigest(), DigestComplete: r.complete, InputComplete: r.complete, SyntaxComplete: r.complete && !r.dtd.WellFormednessIncomplete, Encoding: r.cursor.encoding, DTD: r.dtd}
}
func (r *sourceXMLTokenReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	r.cursor.close()
	if r.dtdStore != nil {
		_ = r.dtdStore.Close()
	}
	if r.account != nil {
		_ = r.account.Release(validation.BudgetBufferedBytes, r.tokenOwned+r.retained+r.mapOwned)
	}
	r.tokenOwned, r.retained, r.mapOwned = 0, 0, 0
	r.frames = nil
	r.entity = nil
	r.dtdOriginScratch = nil
	r.namespaces = nil
	return nil
}
func (r *sourceXMLTokenReader) reserve(n int64) error {
	if n < 0 {
		return &validation.ResourceError{Kind: validation.BudgetBufferedBytes, Requested: n}
	}
	if r.account != nil {
		return r.account.Charge(validation.BudgetBufferedBytes, n)
	}
	return nil
}
func (r *sourceXMLTokenReader) tokenReserve(n int64) error {
	if err := r.reserve(n); err != nil {
		return err
	}
	r.tokenOwned += n
	return nil
}
func (r *sourceXMLTokenReader) checkScalar(n int64) error {
	if r.account != nil {
		return r.account.Check(validation.BudgetScalarBytes, n)
	}
	return nil
}
func (r *sourceXMLTokenReader) releaseToken() {
	if r.account != nil && r.tokenOwned > 0 {
		_ = r.account.Release(validation.BudgetBufferedBytes, r.tokenOwned)
	}
	r.tokenOwned = 0
	r.metadata = sourceXMLTokenMeta{}
}
func (r *sourceXMLTokenReader) syntax(message string) error { return r.cursor.syntax(message) }
func (r *sourceXMLTokenReader) must() (sourceXMLUnit, error) {
	u, err := r.take()
	if err == io.EOF {
		return u, r.syntax("unexpected EOF")
	}
	if errors.Is(err, errSourceEntityBoundary) {
		return u, r.syntax("entity boundary splits XML token")
	}
	return u, err
}
func (r *sourceXMLTokenReader) literal(want string) error {
	for _, c := range want {
		u, err := r.must()
		if err != nil {
			return err
		}
		if u.Rune != c {
			return r.syntax("expected " + want)
		}
	}
	return nil
}
func sourceXMLSpace(c rune) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }
func (r *sourceXMLTokenReader) spaces() (bool, error) {
	found := false
	for {
		if err := r.cursor.contextErr(); err != nil {
			return found, err
		}
		if span := r.asciiSpan(sourceASCIISpace, 0, sourceXMLCursorBytes); len(span) > 0 {
			r.cursor.consumeASCII(span)
			found = true
			continue
		}
		u, err := r.peek()
		if err == io.EOF {
			return found, nil
		}
		if err != nil {
			return found, err
		}
		if !sourceXMLSpace(u.Rune) {
			return found, nil
		}
		if err = r.discard(); err != nil {
			return found, err
		}
		found = true
	}
}

type sourceXMLBuilder struct {
	r        *sourceXMLTokenReader
	data     []byte
	borrowed bool
}

func (b *sourceXMLBuilder) append(c rune) error {
	size := utf8.RuneLen(c)
	if size < 0 {
		return b.r.syntax("invalid character")
	}
	if err := b.grow(int64(len(b.data)) + int64(size)); err != nil {
		return err
	}
	b.data = utf8.AppendRune(b.data, c)
	return nil
}
func (b *sourceXMLBuilder) grow(n int64) error {
	if err := b.r.checkScalar(n); err != nil {
		return err
	}
	if n > int64(cap(b.data)) {
		capacity := int64(cap(b.data)) * 2
		if capacity < 32 {
			capacity = 32
		}
		if capacity < n {
			capacity = n
		}
		if b.r.account != nil && capacity > b.r.account.Limits().MaxScalarBytes {
			capacity = b.r.account.Limits().MaxScalarBytes
		}
		count, err := validation.AllocationSize(capacity)
		if err != nil {
			return err
		}
		if err = b.r.tokenReserve(capacity); err != nil {
			return err
		}
		next := make([]byte, len(b.data), count)
		copy(next, b.data)
		old := int64(cap(b.data))
		b.data = next
		if !b.borrowed {
			if b.r.account != nil && old > 0 {
				_ = b.r.account.Release(validation.BudgetBufferedBytes, old)
			}
			b.r.tokenOwned -= old
		}
		b.borrowed = false
	}
	return nil
}
func (b *sourceXMLBuilder) string() (string, error) {
	if err := b.r.tokenReserve(int64(len(b.data))); err != nil {
		return "", err
	}
	return string(b.data), nil
}
func (r *sourceXMLTokenReader) name() (string, error) {
	b := sourceXMLBuilder{r: r, data: r.nameScratch[:0], borrowed: true}
	u, err := r.peek()
	if err != nil {
		if err == io.EOF {
			return "", r.syntax("unexpected EOF in XML name")
		}
		return "", err
	}
	if !sourceXMLNameStart(u.Rune) {
		return "", r.syntax("expected XML name")
	}
	for {
		if consumed, err := r.appendASCII(&b, sourceASCIIName, 0); err != nil {
			return "", err
		} else if consumed {
			continue
		}
		u, err = r.peek()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if !sourceXMLNameChar(u.Rune) {
			break
		}
		_ = r.discard()
		if err = b.append(u.Rune); err != nil {
			return "", err
		}
	}
	return b.string()
}
func sourceXMLQName(name string) (xml.Name, bool) {
	prefix, local, qualified := strings.Cut(name, ":")
	if !qualified {
		return xml.Name{Local: name}, sourceXMLNCName(name)
	}
	return xml.Name{Space: prefix, Local: local}, sourceXMLNCName(prefix) && sourceXMLNCName(local)
}
func sourceXMLNCName(name string) bool {
	first := true
	for _, c := range name {
		if c == ':' || first && !sourceXMLNameStart(c) || !first && !sourceXMLNameChar(c) {
			return false
		}
		first = false
	}
	return !first
}
func sourceXMLNameStart(c rune) bool {
	return c == ':' || c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= 0xc0 && c <= 0xd6 || c >= 0xd8 && c <= 0xf6 || c >= 0xf8 && c <= 0x2ff || c >= 0x370 && c <= 0x37d || c >= 0x37f && c <= 0x1fff || c >= 0x200c && c <= 0x200d || c >= 0x2070 && c <= 0x218f || c >= 0x2c00 && c <= 0x2fef || c >= 0x3001 && c <= 0xd7ff || c >= 0xf900 && c <= 0xfdcf || c >= 0xfdf0 && c <= 0xfffd || c >= 0x10000 && c <= 0xeffff
}
func sourceXMLNameChar(c rune) bool {
	return sourceXMLNameStart(c) || c == '-' || c == '.' || c >= '0' && c <= '9' || c == 0xb7 || c >= 0x300 && c <= 0x36f || c >= 0x203f && c <= 0x2040
}

func (r *sourceXMLTokenReader) Token() (token xml.Token, err error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.complete {
		return nil, io.EOF
	}
	if r.closed {
		return nil, &sourceXMLUnsupportedError{Feature: "closed XML source", Reason: "source-closed"}
	}
	defer func() {
		if errors.Is(err, errSourceEntityBoundary) {
			err = r.syntax("entity boundary splits XML token")
		}
		if err != nil {
			r.err = err
		}
	}()
	for {
		r.releaseToken()
		if err = r.cursor.ctx.Err(); err != nil {
			return nil, err
		}
		if r.needEnd {
			r.needEnd = false
			r.metadata.EmptyEnd = true
			r.metadata.Span = sourceXMLSpan{Start: r.position(), End: r.position()}
			if err = r.captureOrigin(&r.metadata.Origin, true); err != nil {
				return nil, err
			}
			name := r.frames[len(r.frames)-1].name
			r.pop()
			if err = r.event(); err != nil {
				return nil, err
			}
			return xml.EndElement{Name: name}, nil
		}
		for r.entity != nil && r.entity.offset == r.entity.length() && !r.entity.hasCached {
			if err = r.finishEntity(sourceEntityContent); err != nil {
				return nil, err
			}
		}
		first := !r.seenToken
		u, e := r.peek()
		if e != nil {
			if e == io.EOF {
				if len(r.frames) > 0 {
					return nil, r.syntax("unexpected EOF")
				}
				if !r.seenRoot {
					return nil, ErrEmptyDocument
				}
				r.complete = true
				return nil, io.EOF
			}
			return nil, e
		}
		r.metadata.Span.Start = u.Start
		if u.Rune == '<' {
			if err = r.captureOrigin(&r.metadata.Origin, true); err != nil {
				return nil, err
			}
		}
		if first && strings.HasPrefix(r.cursor.encoding, "utf16") && !r.cursor.bom && u.Rune != '<' {
			return nil, r.syntax("UTF-16 without a byte order mark requires an encoding declaration")
		}
		if err = r.event(); err != nil {
			return nil, err
		}
		r.seenToken = true
		if u.Rune != '<' {
			token, err = r.text(false)
		} else {
			_ = r.discard()
			u, err = r.must()
			if err != nil {
				return nil, err
			}
			switch u.Rune {
			case '/':
				token, err = r.end()
			case '?':
				token, err = r.instruction(first)
			case '!':
				u, err = r.must()
				if err != nil {
					return nil, err
				}
				switch u.Rune {
				case '-':
					if err = r.literal("-"); err == nil {
						token, err = r.comment()
					}
				case '[':
					if err = r.literal("CDATA["); err == nil {
						r.metadata.CDATA = true
						token, err = r.text(true)
					}
				case 'D':
					if err = r.literal("OCTYPE"); err == nil {
						token, err = r.doctypeToken()
					}
				default:
					err = r.syntax("unknown XML markup declaration")
				}
			default:
				// The first name character is already consumed; preserve its exact unit.
				r.unread(u)
				token, err = r.start()
			}
		}
		if text, ok := token.(xml.CharData); err == nil && ok && len(text) == 0 && !r.metadata.CDATA && r.internalDTD {
			// Expansion can end without character data or begin with markup.
			// Advance iteratively, never exposing synthetic empty text tokens.
			continue
		}
		r.metadata.Span.End = r.position()
		if err == nil && first && strings.HasPrefix(r.cursor.encoding, "utf16") && !r.cursor.bom {
			if p, ok := token.(xml.ProcInst); !ok || p.Target != "xml" {
				return nil, r.syntax("UTF-16 without a byte order mark requires an encoding declaration")
			}
		}
		return token, err
	}
}
func (r *sourceXMLTokenReader) event() error {
	if r.account != nil {
		return r.account.Charge(validation.BudgetEvents, 1)
	}
	return nil
}
func (r *sourceXMLTokenReader) reference(context sourceXMLEntityContext, start sourceXMLPosition) (rune, error) {
	if r.internalDTD {
		if err := r.event(); err != nil {
			return 0, err
		}
	}
	r.metadata.Reference = true
	u, err := r.must()
	if err != nil {
		return 0, err
	}
	if u.Rune == '#' {
		base := int64(10)
		u, err = r.must()
		if err != nil {
			return 0, err
		}
		if u.Rune == 'x' {
			base = 16
			u, err = r.must()
			if err != nil {
				return 0, err
			}
		}
		var value, digits int64
		for {
			digit := int64(-1)
			if u.Rune >= '0' && u.Rune <= '9' {
				digit = int64(u.Rune - '0')
			} else if base == 16 && u.Rune >= 'a' && u.Rune <= 'f' {
				digit = int64(u.Rune-'a') + 10
			} else if base == 16 && u.Rune >= 'A' && u.Rune <= 'F' {
				digit = int64(u.Rune-'A') + 10
			}
			if digit < 0 {
				break
			}
			if value > (0x10ffff-digit)/base {
				return 0, r.syntax("character reference exceeds Unicode range")
			}
			value = value*base + digit
			digits++
			if err = r.checkScalar(digits); err != nil {
				return 0, err
			}
			u, err = r.must()
			if err != nil {
				return 0, err
			}
		}
		if digits == 0 || u.Rune != ';' || !sourceXMLChar(rune(value)) {
			return 0, r.syntax("invalid XML character reference")
		}
		return rune(value), nil
	}
	r.unread(u)
	name, err := r.name()
	if err != nil {
		return 0, err
	}
	if !sourceXMLNCName(name) {
		return 0, r.syntax("invalid entity name")
	}
	if err = r.literal(";"); err != nil {
		return 0, err
	}
	switch name {
	case "lt":
		return '<', nil
	case "gt":
		return '>', nil
	case "amp":
		return '&', nil
	case "apos":
		return '\'', nil
	case "quot":
		return '"', nil
	}
	if r.internalDTD {
		return r.entityReference(name, context, sourceXMLSpan{Start: start, End: r.position()})
	}
	if r.dtd.ExternalSubset || r.dtd.HasEntityDeclarations || r.dtd.HasParameterReferences {
		return 0, &sourceXMLUnsupportedError{Feature: "DTD entity expansion", Reason: "dtd-entity-expansion"}
	}
	return 0, r.syntax("undeclared XML entity " + sourceXMLDiagnosticValue(name))
}
func (r *sourceXMLTokenReader) text(cdata bool) (xml.Token, error) {
	if cdata {
		return r.cdata()
	}
	b := sourceXMLBuilder{r: r, data: r.valueScratch[:0], borrowed: true}
	var last1, last2 rune
	for {
		mode := sourceASCIIText
		if len(r.frames) == 0 {
			mode = sourceASCIISpace
		}
		if consumed, err := r.appendASCII(&b, mode, 0); err != nil {
			return nil, err
		} else if consumed {
			last1, last2 = 0, 0
			continue
		}
		u, err := r.peek()
		if err != nil {
			if errors.Is(err, errSourceEntityBoundary) {
				break
			}
			if err == io.EOF && !cdata {
				break
			}
			if err == io.EOF {
				return nil, r.syntax("unexpected EOF in CDATA section")
			}
			return nil, err
		}
		if !cdata && (u.Rune == '<' || r.internalDTD && u.Rune == '&' && len(b.data) > 0) {
			break
		}
		_ = r.discard()
		c := u.Rune
		if last2 == ']' && last1 == ']' && c == '>' {
			if !cdata {
				return nil, r.syntax("unescaped ]]> not in CDATA section")
			}
			b.data = b.data[:len(b.data)-2]
			break
		}
		if !cdata && c == '&' {
			if r.internalDTD && len(r.frames) == 0 {
				return nil, r.syntax("references are not allowed outside the root element")
			}
			c, err = r.reference(sourceEntityContent, u.Start)
			if err != nil {
				return nil, err
			}
			last1, last2 = 0, 0
			if c == sourceXMLExpandedRune {
				r.metadata.Span = sourceXMLSpan{}
				continue
			}
		} else {
			last2, last1 = last1, c
		}
		if len(r.frames) == 0 && !cdata && !sourceXMLSpace(c) {
			return nil, r.syntax("character data outside the root element")
		}
		if err = b.append(c); err != nil {
			return nil, err
		}
	}
	if len(r.frames) == 0 && (cdata || r.metadata.Reference) {
		return nil, r.syntax("references and CDATA are not allowed outside the root element")
	}
	if len(b.data) != 0 {
		if err := r.captureOrigin(&r.metadata.Origin, true); err != nil {
			return nil, err
		}
	}
	return xml.CharData(b.data), nil
}
func (r *sourceXMLTokenReader) cdata() (xml.Token, error) {
	b := sourceXMLBuilder{r: r, data: r.valueScratch[:0], borrowed: true}
	brackets := 0
	for {
		u, err := r.must()
		if err != nil {
			return nil, err
		}
		if u.Rune == ']' {
			if brackets == 2 {
				if err = b.append(']'); err != nil {
					return nil, err
				}
			} else {
				brackets++
			}
			continue
		}
		if u.Rune == '>' && brackets == 2 {
			if len(r.frames) == 0 {
				return nil, r.syntax("references and CDATA are not allowed outside the root element")
			}
			return xml.CharData(b.data), nil
		}
		for brackets > 0 {
			if err = b.append(']'); err != nil {
				return nil, err
			}
			brackets--
		}
		if err = b.append(u.Rune); err != nil {
			return nil, err
		}
	}
}
func (r *sourceXMLTokenReader) comment() (xml.Token, error) {
	b := sourceXMLBuilder{r: r, data: r.valueScratch[:0], borrowed: true}
	dashes := 0
	for {
		u, err := r.must()
		if err != nil {
			return nil, err
		}
		if dashes == 2 {
			if u.Rune != '>' {
				return nil, r.syntax(`invalid sequence "--" not allowed in comments`)
			}
			return xml.Comment(b.data), nil
		}
		if u.Rune == '-' {
			dashes++
			continue
		}
		if dashes != 0 {
			if err = b.append('-'); err != nil {
				return nil, err
			}
			dashes = 0
		}
		if err = b.append(u.Rune); err != nil {
			return nil, err
		}
	}
}
func (r *sourceXMLTokenReader) quoted(normalize, references bool) (string, sourceXMLSpan, error) {
	var span sourceXMLSpan
	u, err := r.must()
	if err != nil {
		return "", span, err
	}
	if u.Rune != '\'' && u.Rune != '"' {
		return "", span, r.syntax("unquoted or missing attribute value in element")
	}
	span.Start = r.position()
	value, end, err := r.attributeValue(u.Rune, r.entity, normalize, references)
	span.End = end
	return value, span, err
}

func (r *sourceXMLTokenReader) start() (xml.Token, error) {
	nameString, err := r.name()
	if err != nil {
		return nil, err
	}
	name, valid := sourceXMLQName(nameString)
	if !valid {
		return nil, r.syntax("invalid qualified XML name")
	}
	if r.rootClosed {
		return nil, r.syntax("multiple root elements")
	}
	if int64(len(r.frames)) >= r.maxDepth {
		legacy := fmt.Errorf("%w: maximum is %d elements", ErrXMLTooDeep, r.maxDepth)
		if r.account != nil {
			resource := &validation.ResourceError{Kind: validation.BudgetDepth, Limit: r.maxDepth, Used: int64(len(r.frames)), Requested: 1}
			if r.maxDepth == r.account.Limits().MaxDepth {
				return nil, resource
			}
			// An explicit Decode depth limit remains a resource boundary when
			// tighter than the assessment account, with its legacy identity intact.
			return nil, errors.Join(legacy, resource)
		}
		return nil, legacy
	}
	var attributes []xml.Attr
	var metadata []sourceXMLAttribute
	empty := false
	for {
		separated, err := r.spaces()
		if err != nil {
			return nil, err
		}
		u, err := r.peek()
		if err != nil {
			if err == io.EOF {
				return nil, r.syntax("unexpected EOF in start tag")
			}
			return nil, err
		}
		if u.Rune == '>' {
			_ = r.discard()
			break
		}
		if u.Rune == '/' {
			_ = r.discard()
			if err = r.literal(">"); err != nil {
				return nil, err
			}
			empty = true
			break
		}
		if !separated {
			return nil, r.syntax("XML attributes must be separated by whitespace")
		}
		if r.account != nil {
			limit := r.account.Limits().MaxAttributes
			used, e := r.account.Usage(validation.BudgetAttributes)
			if e != nil {
				return nil, e
			}
			if int64(len(attributes)) >= limit-used {
				return nil, &validation.ResourceError{Kind: validation.BudgetAttributes, Limit: limit, Used: used, Requested: int64(len(attributes)) + 1}
			}
		}
		if err = r.event(); err != nil {
			return nil, err
		}
		m := sourceXMLAttribute{NameSpan: sourceXMLSpan{Start: u.Start}}
		raw, err := r.name()
		if err != nil {
			return nil, err
		}
		m.NameSpan.End = r.position()
		aName, valid := sourceXMLQName(raw)
		if !valid {
			return nil, r.syntax("invalid qualified XML attribute name")
		}
		if _, err = r.spaces(); err != nil {
			return nil, err
		}
		if err = r.literal("="); err != nil {
			return nil, err
		}
		if _, err = r.spaces(); err != nil {
			return nil, err
		}
		value, span, err := r.quoted(true, true)
		if err != nil {
			return nil, err
		}
		m.ValueSpan = span
		if err = r.captureOrigin(&m.Origin, true); err != nil {
			return nil, err
		}
		m.NamespaceDeclaration = aName.Space == "xmlns" || aName.Space == "" && aName.Local == "xmlns"
		if len(attributes) == cap(attributes) {
			capacity := cap(attributes) * 2
			if capacity < 4 {
				capacity = 4
			}
			if r.account != nil && int64(capacity) > r.account.Limits().MaxAttributes {
				capacity = int(r.account.Limits().MaxAttributes)
			}
			charge := int64(capacity) * 256
			if err = r.tokenReserve(charge); err != nil {
				return nil, err
			}
			next := make([]xml.Attr, len(attributes), capacity)
			nextMeta := make([]sourceXMLAttribute, len(metadata), capacity)
			copy(next, attributes)
			copy(nextMeta, metadata)
			old := int64(cap(attributes)) * 256
			attributes, metadata = next, nextMeta
			if r.account != nil && old > 0 {
				_ = r.account.Release(validation.BudgetBufferedBytes, old)
			}
			r.tokenOwned -= old
		}
		attributes = append(attributes, xml.Attr{Name: aName, Value: value})
		metadata = append(metadata, m)
	}
	if r.internalDTD {
		attributes, metadata, err = r.applyDTDAttributes(nameString, attributes, metadata)
		if err != nil {
			return nil, err
		}
	}
	r.metadata.Attributes = metadata
	if err = r.open(name, attributes); err != nil {
		return nil, err
	}
	r.seenRoot = true
	r.needEnd = empty
	return xml.StartElement{Name: name, Attr: attributes}, nil
}
func (r *sourceXMLTokenReader) lookup(prefix string) (string, bool) {
	if prefix == "xml" {
		return "http://www.w3.org/XML/1998/namespace", true
	}
	b := r.namespaces[prefix]
	return b.uri, b.bound
}
func (r *sourceXMLTokenReader) open(name xml.Name, attributes []xml.Attr) error {
	const xmlURI = "http://www.w3.org/XML/1998/namespace"
	const xmlnsURI = "http://www.w3.org/2000/xmlns/"
	frame := sourceXMLFrame{name: name}
	charge := int64(256 + len(name.Space) + len(name.Local))
	if err := r.reserve(charge); err != nil {
		return err
	}
	r.retained += charge
	frame.retained = charge
	// Temporary lookup tables and their bounded entries are reserved before make.
	if err := r.tokenReserve(128 + int64(len(attributes))*256); err != nil {
		return err
	}
	lexical := make(map[xml.Name]bool, len(attributes))
	for _, a := range attributes {
		if err := r.cursor.ctx.Err(); err != nil {
			return err
		}
		if lexical[a.Name] {
			return r.syntax(fmt.Sprintf("duplicate XML attribute {%s}%s", sourceXMLDiagnosticValue(a.Name.Space), sourceXMLDiagnosticValue(a.Name.Local)))
		}
		lexical[a.Name] = true
		if a.Name.Space != "xmlns" && !(a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		prefix := a.Name.Local
		if a.Name.Space == "" {
			prefix = ""
		}
		if prefix == "xmlns" || a.Value == xmlnsURI || prefix == "xml" && a.Value != xmlURI || prefix != "xml" && a.Value == xmlURI {
			return r.syntax("reserved XML namespace binding")
		}
		if prefix != "" && a.Value == "" {
			return r.syntax("namespace prefix undeclaring is not allowed")
		}
		charge = int64(256 + len(prefix) + len(a.Value))
		if err := r.reserve(charge); err != nil {
			return err
		}
		r.retained += charge
		frame.retained += charge
		if r.namespaces == nil {
			if err := r.reserve(256); err != nil {
				return err
			}
			r.mapOwned += 256
			r.namespaces = make(map[string]sourceXMLBinding)
		}
		previous, seen := r.namespaces[prefix]
		if !seen {
			n := int64(128 + len(prefix))
			if err := r.reserve(n); err != nil {
				return err
			}
			r.mapOwned += n
		}
		frame.undo = append(frame.undo, sourceXMLUndo{prefix: prefix, previous: previous})
		r.namespaces[prefix] = sourceXMLBinding{uri: a.Value, bound: true}
	}
	// All declarations apply to the whole start tag, independent of their order.
	if name.Space == "xmlns" {
		return r.syntax("xmlns prefix is not allowed on an element")
	}
	if name.Space != "" {
		if _, bound := r.lookup(name.Space); !bound {
			return r.unboundPrefix(name.Space)
		}
	}
	expanded := make(map[xml.Name]bool, len(attributes))
	for _, a := range attributes {
		if err := r.cursor.ctx.Err(); err != nil {
			return err
		}
		if a.Name.Space == "xmlns" || a.Name.Space == "" && a.Name.Local == "xmlns" {
			continue
		}
		expandedName := a.Name
		if a.Name.Space != "" {
			uri, bound := r.lookup(a.Name.Space)
			if !bound {
				return r.unboundPrefix(a.Name.Space)
			}
			expandedName.Space = uri
		}
		if expanded[expandedName] {
			return r.syntax(fmt.Sprintf("duplicate XML attribute {%s}%s", sourceXMLDiagnosticValue(expandedName.Space), sourceXMLDiagnosticValue(expandedName.Local)))
		}
		expanded[expandedName] = true
	}
	r.frames = append(r.frames, frame)
	if len(r.frames) > r.peakDepth {
		r.peakDepth = len(r.frames)
	}
	return nil
}
func (r *sourceXMLTokenReader) pop() {
	last := len(r.frames) - 1
	frame := r.frames[last]
	for i := len(frame.undo) - 1; i >= 0; i-- {
		u := frame.undo[i]
		r.namespaces[u.prefix] = u.previous
	}
	if r.account != nil {
		_ = r.account.Release(validation.BudgetBufferedBytes, frame.retained)
	}
	r.retained -= frame.retained
	r.frames[last] = sourceXMLFrame{}
	r.frames = r.frames[:last]
	if last == 0 {
		r.rootClosed = true
	}
}
func (r *sourceXMLTokenReader) end() (xml.Token, error) {
	raw, err := r.name()
	if err != nil {
		return nil, err
	}
	name, valid := sourceXMLQName(raw)
	if !valid {
		return nil, r.syntax("invalid qualified XML end name")
	}
	if _, err = r.spaces(); err != nil {
		return nil, err
	}
	if err = r.literal(">"); err != nil {
		return nil, err
	}
	if r.entity != nil && len(r.frames) <= r.entity.entryDepth {
		return nil, r.syntax("entity replacement closes an element outside its boundary")
	}
	if len(r.frames) == 0 || r.frames[len(r.frames)-1].name != name {
		return nil, r.syntax("element closed by a different end tag")
	}
	r.pop()
	return xml.EndElement{Name: name}, nil
}
func (r *sourceXMLTokenReader) instruction(first bool) (xml.Token, error) {
	target, err := r.name()
	if err != nil {
		return nil, err
	}
	if !sourceXMLNCName(target) {
		return nil, r.syntax("invalid processing instruction target")
	}
	if strings.EqualFold(target, "xml") {
		if target != "xml" {
			return nil, r.syntax(fmt.Sprintf("reserved XML processing instruction target %q", target))
		}
		return r.declaration(first)
	}
	separated, err := r.spaces()
	if err != nil {
		return nil, err
	}
	u, err := r.peek()
	if err != nil {
		return nil, err
	}
	if !separated && u.Rune != '?' {
		return nil, r.syntax("processing instruction data requires whitespace")
	}
	b := sourceXMLBuilder{r: r, data: r.valueScratch[:0], borrowed: true}
	question := false
	for {
		u, err = r.must()
		if err != nil {
			return nil, err
		}
		if question {
			if u.Rune == '>' {
				return xml.ProcInst{Target: target, Inst: b.data}, nil
			}
			if !separated {
				return nil, r.syntax("processing instruction data requires whitespace")
			}
			if err = b.append('?'); err != nil {
				return nil, err
			}
		}
		question = u.Rune == '?'
		if !question {
			if err = b.append(u.Rune); err != nil {
				return nil, err
			}
		}
	}
}

func (r *sourceXMLTokenReader) declaration(first bool) (xml.Token, error) {
	separated, err := r.spaces()
	if err != nil {
		return nil, err
	}
	if !separated {
		return nil, r.syntax("XML declaration requires version")
	}
	var version, encoding, standalone string
	var encodingSeen, standaloneSeen bool
	index := 0
	for {
		u, err := r.peek()
		if err != nil {
			return nil, err
		}
		if u.Rune == '?' {
			if err = r.literal("?>"); err != nil {
				return nil, err
			}
			break
		}
		key, err := r.name()
		if err != nil {
			return nil, err
		}
		if _, err = r.spaces(); err != nil {
			return nil, err
		}
		if err = r.literal("="); err != nil {
			return nil, err
		}
		if _, err = r.spaces(); err != nil {
			return nil, err
		}
		value, _, err := r.quoted(false, false)
		if err != nil {
			return nil, err
		}
		switch {
		case key == "version" && index == 0:
			version = value
			index = 1
		case key == "encoding" && index == 1:
			encoding = value
			encodingSeen = true
			index = 2
		case key == "standalone" && (index == 1 || index == 2):
			standalone = value
			standaloneSeen = true
			index = 3
		default:
			return nil, r.syntax("invalid XML declaration pseudo-attribute order or name")
		}
		separated, err = r.spaces()
		if err != nil {
			return nil, err
		}
		u, err = r.peek()
		if err != nil {
			return nil, err
		}
		if !separated && u.Rune != '?' {
			return nil, r.syntax("XML declaration pseudo-attributes require whitespace")
		}
	}
	if !first {
		return nil, r.syntax("XML declaration must be at the start of the document")
	}
	if len(version) < 3 || !strings.HasPrefix(version, "1.") {
		return nil, r.syntax("invalid XML version")
	}
	for _, c := range version[2:] {
		if c < '0' || c > '9' {
			return nil, r.syntax("invalid XML version")
		}
	}
	if standaloneSeen && standalone != "yes" && standalone != "no" {
		return nil, r.syntax("invalid standalone declaration")
	}
	if encodingSeen {
		if encoding == "" {
			return nil, r.syntax("invalid XML encoding name")
		}
		for i, c := range encoding {
			if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || i > 0 && (c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-')) {
				return nil, r.syntax("invalid XML encoding name")
			}
		}
	}
	r.standalone = sourceDTDStandalone(standalone)
	if err = r.selectEncoding(encoding); err != nil {
		return nil, err
	}
	text := "version=\"" + version + "\""
	if encoding != "" {
		text += " encoding=\"" + encoding + "\""
	}
	if standalone != "" {
		text += " standalone=\"" + standalone + "\""
	}
	if err = r.tokenReserve(int64(len(text)) * 2); err != nil {
		return nil, err
	}
	return xml.ProcInst{Target: "xml", Inst: []byte(text)}, nil
}
func (r *sourceXMLTokenReader) selectEncoding(declared string) error {
	normalized := normalizeCharset(declared)
	actual := r.cursor.encoding
	if strings.HasPrefix(actual, "utf16") {
		if !r.cursor.bom {
			if normalized != actual {
				return r.syntax("UTF-16 without a byte order mark requires a matching explicit encoding declaration")
			}
		} else if normalized != "" && normalized != "utf16" && normalized != actual {
			return r.syntax("XML encoding declaration does not match UTF-16 byte order mark")
		}
		return nil
	}
	if normalized == "" || normalized == "utf8" {
		return nil
	}
	if r.cursor.bom {
		return r.syntax("XML encoding declaration does not match UTF-8 byte order mark")
	}
	switch normalized {
	case "iso88591", "latin1":
		r.cursor.encoding = "latin1"
		return nil
	case "utf16", "utf16be", "utf16le":
		return r.syntax("XML encoding declaration does not match source bytes")
	}
	return &sourceXMLUnsupportedError{Feature: "encoding " + sourceXMLDiagnosticValue(declared), Reason: "xml-encoding-unavailable"}
}

// These methods are the streaming, no-fetch DTD reader boundary. The grammar
// component sees decoded characters once; it does not reparse a captured token.
func (r *sourceXMLTokenReader) Peek() (rune, error) {
	u, err := r.peek()
	if errors.Is(err, errSourceEntityBoundary) && r.entity != nil && r.entity.context == sourceEntityDTD {
		return sourceDTDParameterBoundary, nil
	}
	return u.Rune, err
}
func (r *sourceXMLTokenReader) Read() (rune, error) {
	physical := r.entity == nil
	u, err := r.must()
	if err == nil && physical {
		r.dtdBytes += int64(utf8.RuneLen(u.Rune))
		err = r.checkScalar(r.dtdBytes)
	}
	return u.Rune, err
}
func (r *sourceXMLTokenReader) SyntaxError(message string) error { return r.syntax(message) }
func (r *sourceXMLTokenReader) CheckDTDDepth(depth int64) error {
	if depth > int64(r.maxDepth) {
		return &validation.ResourceError{Kind: validation.BudgetDepth, Limit: int64(r.maxDepth), Requested: depth}
	}
	return nil
}
func (r *sourceXMLTokenReader) CheckDTDNameBytes(n int64) error { return r.checkScalar(n) }
func (r *sourceXMLTokenReader) doctypeToken() (xml.Token, error) {
	if r.internalDTD && (r.seenRoot || r.doctype) {
		return nil, r.syntax("DOCTYPE must appear once before the root element")
	}
	r.dtdBytes = 0
	var info sourceDTDInfo
	var err error
	if r.internalDTD {
		r.dtdStore, err = newSourceDTDStore(r.cursor.ctx, r.account, r.standalone)
		if err == nil {
			info, err = parseSourceDTDWithStore(r, r.dtdStore)
			r.releaseDTDOriginScratch()
		}
	} else {
		info, err = parseSourceDTD(r)
	}
	r.dtd = info
	if r.internalDTD {
		// Selected names belong to the store. Preserve a parsing failure before
		// any later admission, including default validation and token spelling.
		if err != nil {
			return nil, err
		}
		if err = r.validateDTDDefaults(); err != nil {
			return nil, err
		}
		state := r.dtdStore.State()
		r.dtd.WellFormednessIncomplete = state.WellFormednessIncomplete
		// Effective attributes and XML WFC closure are independent axes. An
		// unused entity declaration is not a missing attribute dependency.
		r.dtd.Unsupported = !state.AttributeDeclarationsComplete
	} else {
		if keepErr := r.reserve(int64(len(info.Name))); keepErr != nil {
			return nil, keepErr
		}
		r.retained += int64(len(info.Name))
		if err != nil {
			return nil, err
		}
	}
	if r.seenRoot {
		return nil, r.syntax("DOCTYPE must precede the root element")
	}
	if r.doctype {
		return nil, r.syntax("multiple DOCTYPE declarations")
	}
	r.doctype = true
	value := "DOCTYPE " + info.Name
	if err = r.tokenReserve(int64(len(value)) * 2); err != nil {
		return nil, err
	}
	return xml.Directive([]byte(value)), nil
}

func (r *sourceXMLTokenReader) ReserveDTDBytes(n int64) error { return r.tokenReserve(n) }
func (r *sourceXMLTokenReader) ReleaseDTDBytes(n int64) {
	if r.account != nil {
		_ = r.account.Release(validation.BudgetBufferedBytes, n)
	}
	r.tokenOwned -= n
}

func (r *sourceXMLTokenReader) unboundPrefix(prefix string) error {
	if r.internalDTD && r.dtdStore != nil && r.dtdStore.State().AttributeDeclarationsComplete {
		return r.syntax("undeclared XML namespace prefix " + sourceXMLDiagnosticValue(prefix))
	}
	if r.dtd.HasAttributeDeclarations || r.dtd.ExternalSubset || r.dtd.HasParameterReferences {
		return &sourceXMLUnsupportedError{Feature: "namespace bindings requiring DTD default attributes", Reason: "dtd-namespace-binding-unavailable"}
	}
	return r.syntax("undeclared XML namespace prefix " + sourceXMLDiagnosticValue(prefix))
}
func sourceXMLDiagnosticValue(value string) string {
	const maximum = 160
	if len(value) <= maximum {
		return value
	}
	end := maximum
	for end > 0 && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end] + "…"
}
