package musicxml

import (
	"context"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"hash"
	"io"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/go-muse/go-musicxml/validation"
)

// The cursor's positions describe original bytes, including any encoding
// signature. Columns count decoded source characters; CRLF is one line break.
type sourceXMLPosition struct {
	ByteOffset   int64
	Line, Column int
}
type sourceXMLSpan struct{ Start, End sourceXMLPosition }
type sourceXMLUnit struct {
	Rune       rune
	Start, End sourceXMLPosition
}

type sourceXMLUnsupportedError struct {
	Feature string
	Reason  validation.Code
}

func (e *sourceXMLUnsupportedError) Error() string {
	return "musicxml: unsupported XML feature: " + e.Feature
}
func (e *sourceXMLUnsupportedError) Unwrap() error { return validation.ErrIncompleteAssessment }

const sourceXMLCursorBytes = 4096

type sourceXMLCursor struct {
	source             io.Reader
	ctx                context.Context
	cancelable         bool
	account            *validation.Account
	data               [sourceXMLCursorBytes]byte
	at, n              int
	pendingErr         error
	reads              int64
	digest             hash.Hash
	prefix             [4]byte
	prefixAt, prefixN  int
	initialized        bool
	encoding           string
	wide, littleEndian bool
	bom                bool
	rawOffset          int64
	position           sourceXMLPosition
	cached             sourceXMLUnit
	hasCached          bool
	rawCached          sourceXMLUnit
	hasRawCached       bool
	rawErr             error
	closed             bool
}

func newSourceXMLCursor(r io.Reader, ctx context.Context, a *validation.Account) (*sourceXMLCursor, error) {
	if r == nil {
		return nil, ErrNilReader
	}
	if ctx == nil {
		return nil, &validation.ConfigurationError{Field: "context", Reason: "must not be nil"}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if a != nil {
		if err := a.Charge(validation.BudgetBufferedBytes, sourceXMLCursorBytes+512); err != nil {
			return nil, err
		}
	}
	return &sourceXMLCursor{source: r, ctx: ctx, cancelable: ctx.Done() != nil, account: a, digest: sha256.New(), encoding: "utf8", position: sourceXMLPosition{Line: 1, Column: 1}}, nil
}
func (c *sourceXMLCursor) close() {
	if !c.closed {
		c.closed = true
		if c.account != nil {
			_ = c.account.Release(validation.BudgetBufferedBytes, sourceXMLCursorBytes+512)
		}
	}
}
func (c *sourceXMLCursor) readDigest() (d [32]byte) { copy(d[:], c.digest.Sum(nil)); return }
func (c *sourceXMLCursor) syntax(msg string) error {
	return &xml.SyntaxError{Msg: msg, Line: c.position.Line}
}

// A nil Done channel means the context can never be canceled. Avoid repeated
// interface calls for the ordinary Background transport without weakening checks
// on cancellable contexts, including already cached lookahead.
func (c *sourceXMLCursor) contextErr() error {
	if c.cancelable {
		return c.ctx.Err()
	}
	return nil
}

func (c *sourceXMLCursor) byte() (byte, error) {
	if err := c.contextErr(); err != nil {
		return 0, err
	}
	for c.at == c.n {
		if c.pendingErr != nil {
			return 0, c.pendingErr
		}
		size := len(c.data)
		var remaining int64 = -1
		if c.account != nil {
			used, err := c.account.Usage(validation.BudgetInputBytes)
			if err != nil {
				return 0, err
			}
			remaining = c.account.Limits().MaxInputBytes - used
			if remaining < int64(size) {
				size = int(remaining) + 1
			}
		}
		n, err := c.source.Read(c.data[:size])
		if n < 0 || n > size {
			return 0, fmt.Errorf("musicxml: invalid XML reader count %d", n)
		}
		c.at, c.n = 0, n
		if n > 0 {
			c.reads += int64(n)
			_, _ = c.digest.Write(c.data[:n])
			if c.account != nil {
				charged := int64(n)
				if charged > remaining {
					charged = remaining
				}
				if chargeErr := c.account.Charge(validation.BudgetInputBytes, charged); chargeErr != nil {
					return 0, chargeErr
				}
				if int64(n) > remaining {
					c.pendingErr = &validation.ResourceError{Kind: validation.BudgetInputBytes, Limit: c.account.Limits().MaxInputBytes, Used: c.account.Limits().MaxInputBytes, Requested: 1}
					c.n = 0
					return 0, c.pendingErr
				}
			}
		}
		if err != nil {
			c.pendingErr = err
		}
		if contextErr := c.contextErr(); contextErr != nil {
			c.pendingErr = contextErr
			c.n = 0
			return 0, contextErr
		}
		if n == 0 && err == nil { // A bounded no-progress loop does not spin forever.
			for attempts := 0; attempts < 99; attempts++ {
				n, err = c.source.Read(c.data[:size])
				if n != 0 || err != nil {
					break
				}
				if contextErr := c.contextErr(); contextErr != nil {
					return 0, contextErr
				}
			}
			if n == 0 && err == nil {
				c.pendingErr = io.ErrNoProgress
				return 0, c.pendingErr
			}
			// Account this eventual read through the same invariant as the first.
			if n < 0 || n > size {
				return 0, fmt.Errorf("musicxml: invalid XML reader count %d", n)
			}
			c.at, c.n = 0, n
			if n > 0 {
				c.reads += int64(n)
				_, _ = c.digest.Write(c.data[:n])
				if c.account != nil {
					charged := int64(n)
					if charged > remaining {
						charged = remaining
					}
					if e := c.account.Charge(validation.BudgetInputBytes, charged); e != nil {
						return 0, e
					}
					if int64(n) > remaining {
						c.pendingErr = &validation.ResourceError{Kind: validation.BudgetInputBytes, Limit: c.account.Limits().MaxInputBytes, Used: c.account.Limits().MaxInputBytes, Requested: 1}
						c.n = 0
						return 0, c.pendingErr
					}
				}
			}
			if err != nil {
				c.pendingErr = err
			}
		}
	}
	b := c.data[c.at]
	c.at++
	return b, nil
}
func (c *sourceXMLCursor) initialize() error {
	if c.initialized {
		return nil
	}
	c.initialized = true
	for c.prefixN < 4 {
		b, err := c.byte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		c.prefix[c.prefixN] = b
		c.prefixN++
	}
	p := c.prefix[:c.prefixN]
	switch {
	case len(p) >= 3 && p[0] == 239 && p[1] == 187 && p[2] == 191:
		c.bom = true
		c.prefixAt = 3
	case len(p) >= 2 && p[0] == 254 && p[1] == 255:
		c.bom = true
		c.encoding = "utf16be"
		c.prefixAt = 2
	case len(p) >= 2 && p[0] == 255 && p[1] == 254:
		c.bom = true
		c.encoding = "utf16le"
		c.prefixAt = 2
	case len(p) >= 4 && p[0] == 0 && p[1] == '<' && p[2] == 0 && p[3] == '?':
		c.encoding = "utf16be"
	case len(p) >= 4 && p[0] == '<' && p[1] == 0 && p[2] == '?' && p[3] == 0:
		c.encoding = "utf16le"
	}
	c.wide = c.encoding == "utf16be" || c.encoding == "utf16le"
	c.littleEndian = c.encoding == "utf16le"
	c.rawOffset = int64(c.prefixAt)
	c.position.ByteOffset = c.rawOffset
	return nil
}
func (c *sourceXMLCursor) encodedByte() (byte, error) {
	var b byte
	var err error
	if c.prefixAt < c.prefixN {
		b = c.prefix[c.prefixAt]
		c.prefixAt++
	} else {
		b, err = c.byte()
	}
	if err == nil {
		c.rawOffset++
	}
	return b, err
}
func (c *sourceXMLCursor) rawRune() (sourceXMLUnit, error) {
	if c.hasRawCached {
		u := c.rawCached
		c.hasRawCached = false
		return u, nil
	}
	if c.rawErr != nil {
		return sourceXMLUnit{}, c.rawErr
	}
	start := c.rawOffset
	b, err := c.encodedByte()
	if err != nil {
		return sourceXMLUnit{}, err
	}
	var r rune
	switch c.encoding {
	case "latin1":
		r = rune(b)
	case "utf16be", "utf16le":
		second, e := c.encodedByte()
		if e != nil {
			if e == io.EOF {
				return sourceXMLUnit{}, c.syntax("truncated UTF-16 code unit")
			}
			return sourceXMLUnit{}, e
		}
		v := uint16(b)<<8 | uint16(second)
		if c.encoding == "utf16le" {
			v = uint16(second)<<8 | uint16(b)
		}
		if v >= 0xd800 && v <= 0xdbff {
			a, e := c.encodedByte()
			if e != nil {
				if e == io.EOF {
					return sourceXMLUnit{}, c.syntax("truncated UTF-16 surrogate pair")
				}
				return sourceXMLUnit{}, e
			}
			z, e := c.encodedByte()
			if e != nil {
				if e == io.EOF {
					return sourceXMLUnit{}, c.syntax("truncated UTF-16 surrogate pair")
				}
				return sourceXMLUnit{}, e
			}
			low := uint16(a)<<8 | uint16(z)
			if c.encoding == "utf16le" {
				low = uint16(z)<<8 | uint16(a)
			}
			if low < 0xdc00 || low > 0xdfff {
				return sourceXMLUnit{}, c.syntax("invalid UTF-16 surrogate pair")
			}
			r = utf16.DecodeRune(rune(v), rune(low))
		} else if v >= 0xdc00 && v <= 0xdfff {
			return sourceXMLUnit{}, c.syntax("unexpected UTF-16 low surrogate")
		} else {
			r = rune(v)
		}
	default:
		if b < 128 {
			r = rune(b)
		} else {
			var data [4]byte
			data[0] = b
			n := 1
			for !utf8.FullRune(data[:n]) && n < 4 {
				v, e := c.encodedByte()
				if e != nil {
					if e == io.EOF {
						return sourceXMLUnit{}, c.syntax("truncated UTF-8 character")
					}
					return sourceXMLUnit{}, e
				}
				data[n] = v
				n++
			}
			var size int
			r, size = utf8.DecodeRune(data[:n])
			if r == utf8.RuneError && size == 1 {
				return sourceXMLUnit{}, c.syntax("invalid UTF-8")
			}
		}
	}
	if !sourceXMLChar(r) {
		return sourceXMLUnit{}, c.syntax(fmt.Sprintf("illegal XML character %U", r))
	}
	return sourceXMLUnit{Rune: r, Start: sourceXMLPosition{ByteOffset: start}, End: sourceXMLPosition{ByteOffset: c.rawOffset}}, nil
}
func (c *sourceXMLCursor) peek() (sourceXMLUnit, error) {
	if err := c.contextErr(); err != nil {
		return sourceXMLUnit{}, err
	}
	if c.hasCached {
		return c.cached, nil
	}
	// Buffered UTF-16 ASCII needs no generic decoding or CR lookahead.
	// Every refill boundary, control and non-ASCII unit stays on rawRune.
	if c.wide && !c.hasRawCached && c.rawErr == nil && c.prefixAt == c.prefixN && c.at+1 < c.n {
		value, padding := c.data[c.at], c.data[c.at+1]
		if !c.littleEndian {
			value, padding = padding, value
		}
		if padding == 0 && value >= 32 && value < 128 {
			end := c.position
			end.ByteOffset += 2
			end.Column++
			c.cached = sourceXMLUnit{Rune: rune(value), Start: c.position, End: end}
			c.hasCached = true
			c.at += 2
			c.rawOffset += 2
			return c.cached, nil
		}
	}
	if err := c.initialize(); err != nil {
		return sourceXMLUnit{}, err
	}
	u, err := c.rawRune()
	if err != nil {
		return u, err
	}
	if u.Rune == '\r' {
		next, e := c.rawRune()
		if e == nil {
			if next.Rune == '\n' {
				u.End.ByteOffset = next.End.ByteOffset
			} else {
				c.rawCached = next
				c.hasRawCached = true
			}
		} else {
			if e != io.EOF {
				return sourceXMLUnit{}, e
			}
			c.rawErr = e
		}
		u.Rune = '\n'
	}
	u.Start.Line, u.Start.Column = c.position.Line, c.position.Column
	u.End.Line, u.End.Column = c.position.Line, c.position.Column+1
	if u.Rune == '\n' {
		u.End.Line++
		u.End.Column = 1
	}
	c.cached = u
	c.hasCached = true
	return u, nil
}
func (c *sourceXMLCursor) take() (sourceXMLUnit, error) {
	if c.hasCached {
		if err := c.contextErr(); err != nil {
			return sourceXMLUnit{}, err
		}
		c.hasCached = false
		c.position = c.cached.End
		return c.cached, nil
	}
	u, err := c.peek()
	if err == nil {
		c.hasCached = false
		c.position = u.End
	}
	return u, err
}
func sourceXMLChar(r rune) bool {
	return r == 9 || r == 10 || r == 13 || r >= 32 && r <= 0xd7ff || r >= 0xe000 && r <= 0xfffd || r >= 0x10000 && r <= 0x10ffff
}

// discard consumes cached lookahead when the scanner needs only its position.
// Avoid returning/copying two complete coordinates for an ignored rune value.
func (c *sourceXMLCursor) discard() error {
	if c.hasCached {
		if err := c.contextErr(); err != nil {
			return err
		}
		c.hasCached = false
		c.position = c.cached.End
		return nil
	}
	_, err := c.take()
	return err
}
