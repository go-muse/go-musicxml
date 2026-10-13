package musicxml

import (
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/go-muse/go-musicxml/validation"
)

// Replacement text has already passed physical decoding/EOL normalization and
// EntityValue construction. It must never enter the physical cursor again.
type sourceXMLEntityContext uint8

const (
	sourceEntityContent sourceXMLEntityContext = iota
	sourceEntityAttribute
	sourceEntityDTD
)
const sourceXMLEntityMaxDepth = 64
const sourceXMLEntityFrameBytes = 512

var errSourceEntityBoundary = errors.New("XML entity replacement boundary")

type sourceXMLEntityInput struct {
	Name, Replacement    string
	Parameter            bool
	Padded               bool
	ExternalMarkup       bool
	Declaration          sourceXMLSpan
	DeclarationAvailable bool
	DeclarationRecord    *sourceDTDDeclaration
}
type sourceXMLEntityFrame struct {
	parent                    *sourceXMLEntityFrame
	input                     sourceXMLEntityInput
	context                   sourceXMLEntityContext
	use                       sourceXMLSpan
	useAvailable              bool
	offset, entryDepth, depth int
	cached                    sourceXMLUnit
	hasCached                 bool
}

// Each invocation charges one frame operation and all replacement UTF-8 bytes
// before any replacement scanning. Reference recognition is charged separately.
// The immutable replacement storage remains owned by the declaration store.
func (r *sourceXMLTokenReader) pushEntity(input sourceXMLEntityInput, context sourceXMLEntityContext, use sourceXMLSpan) error {
	if r.account == nil {
		return &validation.ConfigurationError{Field: "Account", Reason: "entity replacement requires the shared account"}
	}
	if err := r.cursor.contextErr(); err != nil {
		return err
	}
	depth := 1
	for frame := r.entity; frame != nil; frame = frame.parent {
		if frame.input.Name == input.Name && frame.input.Parameter == input.Parameter {
			return r.syntax("recursive XML entity " + sourceXMLDiagnosticValue(input.Name))
		}
		depth++
	}
	if depth > sourceXMLEntityMaxDepth {
		return fmt.Errorf("musicxml: entity replacement nesting limit: %w", &validation.ResourceError{Kind: validation.BudgetDepth, Limit: sourceXMLEntityMaxDepth, Used: int64(depth - 1), Requested: 1})
	}
	replacementBytes := int64(len(input.Replacement))
	if input.Padded {
		if len(input.Replacement) > int(^uint(0)>>1)-2 {
			return &validation.ResourceError{Kind: validation.BudgetEvents, Requested: replacementBytes}
		}
		replacementBytes += 2
	}
	if err := r.account.Charge(validation.BudgetEvents, replacementBytes+1); err != nil {
		return err
	}
	if err := r.reserve(sourceXMLEntityFrameBytes); err != nil {
		return err
	}
	r.retained += sourceXMLEntityFrameBytes
	r.entity = &sourceXMLEntityFrame{parent: r.entity, input: input, context: context, use: use, useAvailable: r.entity == nil, entryDepth: len(r.frames), depth: depth}
	return nil
}
func (r *sourceXMLTokenReader) finishEntity(context sourceXMLEntityContext) error {
	frame := r.entity
	if frame == nil || frame.context != context || frame.offset != frame.length() || frame.hasCached {
		return r.syntax("entity boundary splits XML token")
	}
	if len(r.frames) != frame.entryDepth || r.needEnd {
		return r.syntax("entity replacement is not balanced XML content")
	}
	r.entity = frame.parent
	if r.account != nil {
		_ = r.account.Release(validation.BudgetBufferedBytes, sourceXMLEntityFrameBytes)
	}
	r.retained -= sourceXMLEntityFrameBytes
	return nil
}
func (r *sourceXMLTokenReader) peek() (sourceXMLUnit, error) {
	frame := r.entity
	if frame == nil {
		return r.cursor.peek()
	}
	if err := r.cursor.contextErr(); err != nil {
		return sourceXMLUnit{}, err
	}
	if frame.hasCached {
		return frame.cached, nil
	}
	if frame.offset == frame.length() {
		return sourceXMLUnit{}, errSourceEntityBoundary
	}
	at := frame.offset
	if frame.input.Padded {
		if at == 0 || at == frame.length()-1 {
			frame.cached = sourceXMLUnit{Rune: ' '}
			frame.hasCached = true
			return frame.cached, nil
		}
		at--
	}
	c, size := utf8.DecodeRuneInString(frame.input.Replacement[at:])
	if c == utf8.RuneError && size == 1 || !sourceXMLChar(c) {
		return sourceXMLUnit{}, r.syntax("invalid XML character in entity replacement")
	}
	frame.cached = sourceXMLUnit{Rune: c}
	frame.hasCached = true
	return frame.cached, nil
}
func (r *sourceXMLTokenReader) take() (sourceXMLUnit, error) {
	if r.entity == nil {
		return r.cursor.take()
	}
	u, err := r.peek()
	if err == nil {
		r.entity.offset += utf8.RuneLen(u.Rune)
		r.entity.hasCached = false
	}
	return u, err
}
func (r *sourceXMLTokenReader) discard() error {
	if r.entity == nil {
		return r.cursor.discard()
	}
	_, err := r.take()
	return err
}
func (r *sourceXMLTokenReader) unread(u sourceXMLUnit) {
	if r.entity == nil {
		r.cursor.cached = u
		r.cursor.hasCached = true
		r.cursor.position = u.Start
		return
	}
	r.entity.offset -= utf8.RuneLen(u.Rune)
	r.entity.cached = u
	r.entity.hasCached = true
}
func (r *sourceXMLTokenReader) position() sourceXMLPosition {
	if r.entity != nil {
		return sourceXMLPosition{}
	}
	return r.cursor.position
}
func (r *sourceXMLTokenReader) DTDPosition() sourceXMLPosition { return r.position() }

func (f *sourceXMLEntityFrame) length() int {
	n := len(f.input.Replacement)
	if f.input.Padded {
		n += 2
	}
	return n
}
