package musicxml

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/go-muse/go-musicxml/validation"
)

// External DTD subsets are read only from bundled official MusicXML DTDs or
// from a caller DTDSource; nothing is fetched. A resolved subset is parsed by
// the same declaration grammar as the internal subset, extended with the
// external-subset productions: parameter-entity references inside markup
// declarations and entity values, conditional sections and external parameter
// entities. Parsing happens once per closure into an immutable
// sourceDTDPrepared, whose declarations a document's store imports after its
// internal subset, so first-binding precedence matches XML 1.0 section 2.8.

const (
	sourceDTDExternalSubsetRegion sourceDTDRegion = "external-subset"
	// sourceDTDExternalEnd is returned by Peek at the end of the external
	// subset itself. Read never crosses it.
	sourceDTDExternalEnd rune = -3
	// sourceDTDExternalTextBytes caps one caller-supplied DTD entity.
	sourceDTDExternalTextBytes = 16 << 20
)

// sourceDTDExternalBudgets bound the private account that parses one external
// closure. They are independent of the document's assessment budgets: bundled
// DTDs are trusted static data, and caller bytes are additionally charged to the
// document account when an assessment imports them.
func sourceDTDExternalBudgets() validation.Budgets {
	budgets := validation.DefaultBudgets()
	budgets.MaxEvents = 64_000_000
	return budgets
}

type sourceDTDExternalFrameKind uint8

const (
	sourceDTDExternalRoot sourceDTDExternalFrameKind = iota
	// A declaration-level frame holds a parameter entity referenced between
	// declarations; its end is the parser-visible parameter boundary.
	sourceDTDExternalDeclaration
	// An inline frame holds a parameter entity referenced inside a markup
	// declaration; its end is transparent.
	sourceDTDExternalInline
)

type sourceDTDExternalFrame struct {
	kind        sourceDTDExternalFrameKind
	name        string
	systemID    string
	text        string
	offset      int
	position    sourceXMLPosition
	declaration *sourceDTDDeclaration
}

// sourceDTDExternalInput feeds normalized external DTD text to the
// declaration parser. Replacement text is never re-decoded.
type sourceDTDExternalInput struct {
	ctx     context.Context
	account *validation.Account
	frames  []sourceDTDExternalFrame
}

func newSourceDTDExternalInput(ctx context.Context, account *validation.Account, name, systemID, text string) *sourceDTDExternalInput {
	return &sourceDTDExternalInput{ctx: ctx, account: account, frames: []sourceDTDExternalFrame{{kind: sourceDTDExternalRoot, name: name, systemID: systemID, text: text, position: sourceXMLPosition{Line: 1, Column: 1}}}}
}

func (in *sourceDTDExternalInput) unit() (rune, int, error) {
	for {
		top := &in.frames[len(in.frames)-1]
		if top.offset < len(top.text) {
			r, size := utf8.DecodeRuneInString(top.text[top.offset:])
			if r == utf8.RuneError && size <= 1 || !sourceXMLChar(r) {
				return 0, 0, in.SyntaxError("illegal character in external DTD")
			}
			return r, size, nil
		}
		switch top.kind {
		case sourceDTDExternalInline:
			in.frames = in.frames[:len(in.frames)-1]
		case sourceDTDExternalDeclaration:
			return sourceDTDParameterBoundary, 0, nil
		default:
			return sourceDTDExternalEnd, 0, nil
		}
	}
}

func (in *sourceDTDExternalInput) Peek() (rune, error) {
	if err := in.ctx.Err(); err != nil {
		return 0, err
	}
	r, _, err := in.unit()
	return r, err
}

func (in *sourceDTDExternalInput) Read() (rune, error) {
	if err := in.ctx.Err(); err != nil {
		return 0, err
	}
	r, size, err := in.unit()
	if err != nil {
		return 0, err
	}
	switch r {
	case sourceDTDExternalEnd:
		return 0, io.EOF
	case sourceDTDParameterBoundary:
		return 0, in.SyntaxError("markup crosses a parameter entity boundary")
	}
	top := &in.frames[len(in.frames)-1]
	top.offset += size
	top.position.ByteOffset += int64(size)
	top.position.advanceCoordinates(r)
	return r, nil
}

// next reports the rune after the next one in the same frame, for recognizing a
// parameter-entity reference. A reference never spans frames.
func (in *sourceDTDExternalInput) next() rune {
	top := in.frames[len(in.frames)-1]
	if top.offset >= len(top.text) {
		return 0
	}
	_, size := utf8.DecodeRuneInString(top.text[top.offset:])
	if top.offset+size >= len(top.text) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(top.text[top.offset+size:])
	return r
}

func (in *sourceDTDExternalInput) SyntaxError(message string) error {
	top := in.frames[len(in.frames)-1]
	root := in.frames[0]
	where := root.name
	if top.name != root.name {
		where += " (" + top.name + ")"
	}
	return &xml.SyntaxError{Msg: "external DTD " + where + ": " + message, Line: top.position.Line}
}

func (in *sourceDTDExternalInput) systemID() string {
	for i := len(in.frames) - 1; i >= 0; i-- {
		if in.frames[i].systemID != "" {
			return in.frames[i].systemID
		}
	}
	return ""
}

// push enlarges replacement text by one leading and one following space, as
// XML 4.4.8 requires for a parameter entity included in the DTD.
func (in *sourceDTDExternalInput) push(frame sourceDTDExternalFrame) error {
	entities := 0
	for _, f := range in.frames {
		if f.declaration != nil && f.declaration == frame.declaration {
			return in.SyntaxError("recursive parameter entity " + sourceXMLDiagnosticValue(frame.name))
		}
		if f.kind != sourceDTDExternalRoot {
			entities++
		}
	}
	if entities >= sourceXMLEntityMaxDepth {
		return fmt.Errorf("%w: %w", ErrEntityNestingLimit, &validation.ResourceError{Kind: validation.BudgetDepth, Limit: sourceXMLEntityMaxDepth, Used: int64(entities), Requested: 1})
	}
	if err := in.account.Charge(validation.BudgetEvents, int64(len(frame.text))+3); err != nil {
		return err
	}
	frame.text = " " + frame.text + " "
	if frame.systemID != "" {
		// The padding space is not part of the external entity's text.
		frame.position = sourceXMLPosition{ByteOffset: -1, Line: 1}
	} else {
		frame.position = sourceXMLPosition{Line: 1, Column: 1}
	}
	in.frames = append(in.frames, frame)
	return nil
}

func (in *sourceDTDExternalInput) PushDTDParameter(declaration *sourceDTDDeclaration, _ sourceDTDOrigin) error {
	return in.push(sourceDTDExternalFrame{kind: sourceDTDExternalDeclaration, name: "%" + declaration.Name + ";", text: declaration.Replacement, declaration: declaration})
}

func (in *sourceDTDExternalInput) FinishDTDParameter() error {
	top := in.frames[len(in.frames)-1]
	if top.kind != sourceDTDExternalDeclaration || top.offset != len(top.text) {
		return in.SyntaxError("unbalanced parameter entity boundary")
	}
	in.frames = in.frames[:len(in.frames)-1]
	return nil
}

func (in *sourceDTDExternalInput) DTDInParameter() bool { return true }

// DTDParameterOrigin locates a declaration at the current position of the
// innermost frame that holds the text of an external entity. Inside parameter
// entity replacement text that is the position just after the reference.
func (in *sourceDTDExternalInput) DTDParameterOrigin() (sourceDTDOrigin, error) {
	if err := in.ctx.Err(); err != nil {
		return sourceDTDOrigin{}, err
	}
	origin := sourceDTDOrigin{Region: sourceDTDExternalSubsetRegion, ExternalMarkup: true}
	for i := len(in.frames) - 1; i >= 0; i-- {
		frame := in.frames[i]
		if frame.systemID == "" {
			continue
		}
		if frame.position.Line > 0 && frame.position.ByteOffset >= 0 {
			origin.Entity = frame.systemID
			origin.SpanAvailable = true
			origin.Span.Start = frame.position
		}
		break
	}
	return origin, nil
}

func (in *sourceDTDExternalInput) CheckDTDDepth(depth int64) error {
	if depth > sourceDTDMaxDepth {
		return &validation.ResourceError{Kind: validation.BudgetDepth, Limit: sourceDTDMaxDepth, Requested: depth}
	}
	return nil
}

// sourceDTDExternalText applies the XML 4.3.1 text-declaration and 2.11
// end-of-line rules to the bytes of one external parsed entity. Only UTF-8 is
// accepted.
func sourceDTDExternalText(data []byte) (string, error) {
	if len(data) > sourceDTDExternalTextBytes {
		return "", &validation.ResourceError{Kind: validation.BudgetExternalBytes, Limit: sourceDTDExternalTextBytes, Requested: int64(len(data))}
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if !utf8.Valid(data) {
		return "", errors.New("external DTD is not valid UTF-8")
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if rest, found := strings.CutPrefix(text, "<?xml"); found && rest != "" && sourceXMLSpace(rune(rest[0])) {
		end := strings.Index(rest, "?>")
		if end < 0 {
			return "", errors.New("unterminated text declaration in external DTD")
		}
		declaration := rest[:end]
		encoding := ""
		if i := strings.Index(declaration, "encoding"); i >= 0 {
			value := strings.TrimLeft(declaration[i+len("encoding"):], " \t\n")
			if value, found = strings.CutPrefix(value, "="); found {
				value = strings.TrimLeft(value, " \t\n")
				if value != "" && (value[0] == '"' || value[0] == '\'') {
					if close := strings.IndexByte(value[1:], value[0]); close >= 0 {
						encoding = value[1 : close+1]
					}
				}
			}
		}
		if encoding == "" {
			return "", errors.New("text declaration in external DTD requires an encoding")
		}
		if normalizeCharset(encoding) != "utf8" {
			return "", fmt.Errorf("external DTD encoding %q is not supported; only UTF-8 is", encoding)
		}
		text = rest[end+len("?>"):]
	}
	return text, nil
}

// sourceDTDPrepared is the immutable result of reading one external subset.
// Records are values whose strings are no longer owned by any store; importing
// stores copy them under their own accounting.
type sourceDTDPrepared struct {
	declarations []sourceDTDDeclaration
	// characters holds internal general entities whose replacement text is
	// exactly one character, such as the ISO Latin entities of MusicXML.
	characters map[string]rune
	// namespaces holds defaulted xmlns attributes by lexical element name.
	namespaces map[string][]xml.Attr
	bytes      int64
}

// sourceDTDResolver locates external DTD entities. A nil resolver disables
// external subsets, preserving the historical unread behaviour.
type sourceDTDResolver struct {
	source DTDSource
}

func newSourceDTDResolver(options *DTDOptions) *sourceDTDResolver {
	resolver := &sourceDTDResolver{}
	if options != nil {
		resolver.source = options.Source
	}
	return resolver
}

var bundledDTDPrepared sync.Map // file name -> *bundledDTDPreparation

type bundledDTDPreparation struct {
	once     sync.Once
	prepared *sourceDTDPrepared
	err      error
}

// prepare returns the external subset named by a DOCTYPE, or nil when no
// source or bundled DTD provides it. Errors are DTD source or syntax failures.
func (r *sourceDTDResolver) prepare(ctx context.Context, publicID, systemID string) (*sourceDTDPrepared, error) {
	if r == nil || publicID == "" && systemID == "" {
		return nil, nil
	}
	if r.source != nil {
		data, err := r.source.OpenDTD(ctx, DTDRequest{PublicID: publicID, SystemID: systemID})
		switch {
		case err == nil:
			text, textErr := sourceDTDExternalText(data)
			if textErr != nil {
				return nil, &DTDError{PublicID: publicID, SystemID: systemID, Err: textErr}
			}
			prepared, err := prepareSourceDTD(ctx, r, systemID, systemID, text, true)
			if err != nil {
				return nil, &DTDError{PublicID: publicID, SystemID: systemID, Err: err}
			}
			return prepared, nil
		case errors.Is(err, ErrDTDNotFound):
		case ctx.Err() != nil:
			return nil, ctx.Err()
		default:
			return nil, &DTDError{PublicID: publicID, SystemID: systemID, Err: err}
		}
	}
	name, text, ok := bundledDTD(publicID, systemID, "")
	if !ok {
		return nil, nil
	}
	entry, _ := bundledDTDPrepared.LoadOrStore(name, &bundledDTDPreparation{})
	preparation := entry.(*bundledDTDPreparation)
	preparation.once.Do(func() {
		// The bundled closure is static, so its one preparation is not tied to
		// any caller's cancellation.
		preparation.prepared, preparation.err = prepareSourceDTD(context.Background(), &sourceDTDResolver{}, name, bundledDTDSystemPrefixes[0]+name, text, false)
	})
	if preparation.err != nil {
		return nil, fmt.Errorf("musicxml: bundled DTD %s: %w", name, preparation.err)
	}
	return preparation.prepared, nil
}

// parameter resolves an external parameter entity. Entities referenced from a
// caller-supplied subset ask the caller first; bundled closures never do.
func (r *sourceDTDResolver) parameter(ctx context.Context, d *sourceDTDDeclaration, base string, caller bool) (text, systemID string, found bool, size int64, err error) {
	if caller && r.source != nil {
		data, sourceErr := r.source.OpenDTD(ctx, DTDRequest{PublicID: d.PublicID, SystemID: d.SystemID, BaseSystemID: base})
		switch {
		case sourceErr == nil:
			text, err = sourceDTDExternalText(data)
			if err != nil {
				return "", "", false, 0, &DTDError{PublicID: d.PublicID, SystemID: d.SystemID, Err: err}
			}
			return text, sourceDTDJoinSystemID(base, d.SystemID), true, int64(len(data)), nil
		case errors.Is(sourceErr, ErrDTDNotFound):
		case ctx.Err() != nil:
			return "", "", false, 0, ctx.Err()
		default:
			return "", "", false, 0, &DTDError{PublicID: d.PublicID, SystemID: d.SystemID, Err: sourceErr}
		}
	}
	name, text, ok := bundledDTD(d.PublicID, d.SystemID, base)
	if !ok {
		return "", "", false, 0, nil
	}
	return text, bundledDTDSystemPrefixes[0] + name, true, int64(len(text)), nil
}

func sourceDTDJoinSystemID(base, systemID string) string {
	if base == "" || systemID == "" || strings.Contains(systemID, ":") || strings.HasPrefix(systemID, "/") {
		return systemID
	}
	if i := strings.LastIndexByte(base, '/'); i >= 0 {
		return base[:i+1] + path.Clean(systemID)
	}
	return systemID
}

// prepareSourceDTD parses one external subset with a private account and
// returns its binding declarations. An unresolved external parameter entity
// leaves the closure incomplete, which is reported as unavailable (nil, nil
// for bundled data would hide a packaging defect, so it is an error there).
func prepareSourceDTD(ctx context.Context, resolver *sourceDTDResolver, name, systemID, text string, caller bool) (*sourceDTDPrepared, error) {
	account, err := validation.NewBudgetAccount(sourceDTDExternalBudgets())
	if err != nil {
		return nil, err
	}
	store, err := newSourceDTDStore(ctx, account, sourceDTDStandaloneAbsent)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	store.parsed = true
	prepared := &sourceDTDPrepared{bytes: int64(len(text))}
	input := newSourceDTDExternalInput(ctx, account, name, systemID, text)
	p := sourceDTDParser{input: input, store: store, external: true}
	p.resolveParameter = func(d *sourceDTDDeclaration) (string, string, bool, error) {
		text, system, found, size, err := resolver.parameter(ctx, d, input.systemID(), caller)
		if err == nil && found {
			if chargeErr := account.Charge(validation.BudgetExternalBytes, size); chargeErr != nil {
				return "", "", false, chargeErr
			}
			prepared.bytes += size
		}
		return text, system, found, err
	}
	p.subset()
	if p.err == nil && (p.conditionalDepth != 0 || len(p.parameters) != 0) {
		p.fail("unterminated conditional section or parameter entity")
	}
	if p.err != nil {
		return nil, p.err
	}
	if store.state.ParameterEntityUnread {
		return nil, fmt.Errorf("external DTD references the unavailable parameter entity %q", store.state.FirstUnreadParameter)
	}
	for _, d := range store.Declarations() {
		if !d.Binding || d.Kind == sourceDTDParameterEntityDeclaration {
			continue
		}
		record := *d
		record.Origin.Ancestry = nil
		prepared.declarations = append(prepared.declarations, record)
		switch {
		case record.Kind == sourceDTDGeneralEntityDeclaration && !record.External && record.Notation == "":
			if r, size := utf8.DecodeRuneInString(record.Replacement); size == len(record.Replacement) && size > 0 && r != '<' && r != '&' && sourceXMLChar(r) {
				if prepared.characters == nil {
					prepared.characters = make(map[string]rune)
				}
				prepared.characters[record.Name] = r
			}
		case record.Kind == sourceDTDAttributeDeclaration && (record.Name == "xmlns" || strings.HasPrefix(record.Name, "xmlns:")) &&
			(record.DefaultKind == sourceDTDDefaultValue || record.DefaultKind == sourceDTDDefaultFixed) && !strings.Contains(record.RawValue, "&"):
			attrName, valid := sourceXMLQName(record.Name)
			if !valid {
				continue
			}
			value := strings.Map(func(r rune) rune {
				if sourceXMLSpace(r) {
					return ' '
				}
				return r
			}, record.RawValue)
			if prepared.namespaces == nil {
				prepared.namespaces = make(map[string][]xml.Attr)
			}
			prepared.namespaces[record.Element] = append(prepared.namespaces[record.Element], xml.Attr{Name: attrName, Value: value})
		}
	}
	return prepared, nil
}

// importExternalSubset appends a prepared external subset after the internal
// subset. The store's ordinary first-binding rule keeps internal declarations,
// and an earlier unread parameter entity still cuts off later bindings.
func (s *sourceDTDStore) importExternalSubset(prepared *sourceDTDPrepared) error {
	if err := s.account.Charge(validation.BudgetExternalBytes, prepared.bytes); err != nil {
		return err
	}
	for _, d := range prepared.declarations {
		d.Origin.Region, d.Origin.ExternalMarkup = sourceDTDExternalSubsetRegion, true
		if err := s.add(d); err != nil {
			return err
		}
	}
	s.state.ExternalSubsetUnread = false
	complete := !s.failed && s.state.GrammarComplete && !s.state.ParameterEntityUnread
	s.state.DeclarationsComplete = complete
	s.state.AttributeDeclarationsComplete = complete
	s.state.UnparsedEntitiesComplete = complete
	s.refreshWellFormedness()
	return nil
}

// conditionalSection parses the remainder of <![ after the opening bracket.
// Its keyword may come from a parameter-entity reference.
func (p *sourceDTDParser) conditionalSection() {
	p.inDeclaration = true
	p.space()
	keyword := p.keyword()
	p.space()
	p.inDeclaration = false
	p.expect('[')
	if p.err != nil {
		return
	}
	switch keyword {
	case "INCLUDE":
		p.conditionalDepth++
		p.checkDepth(int64(p.conditionalDepth))
		if p.err == nil {
			p.subset()
		}
		p.conditionalDepth--
	case "IGNORE":
		p.ignoredSection()
	default:
		p.fail("expected INCLUDE or IGNORE in a conditional section")
	}
}

// ignoredSection skips an IGNORE section, including nested sections, without
// recognizing references or declarations (XML 3.4).
func (p *sourceDTDParser) ignoredSection() {
	depth := 1
	var previous2, previous1 rune
	for p.err == nil {
		r := p.read()
		switch {
		case previous2 == '<' && previous1 == '!' && r == '[':
			depth++
			r = 0
		case previous2 == ']' && previous1 == ']' && r == '>':
			depth--
			if depth == 0 {
				return
			}
			r = 0
		}
		previous2, previous1 = previous1, r
	}
}

// declarationReference replaces a parameter-entity reference recognized inside
// a markup declaration of the external subset with its padded replacement.
func (p *sourceDTDParser) declarationReference(input *sourceDTDExternalInput) {
	saved := p.capture
	p.capture = nil
	defer func() { p.capture = saved }()
	if _, err := input.Read(); err != nil {
		p.err = err
		return
	}
	name := p.name(sourceDTDNCName, true)
	if p.err != nil {
		return
	}
	defer p.releaseBytes(int64(len(name)))
	if r, err := input.Read(); err != nil || r != ';' {
		if err != nil {
			p.err = err
		} else {
			p.fail("parameter entity reference requires ;")
		}
		return
	}
	d, known := p.store.Entity(name, true)
	if !known {
		p.unreadParameter(name)
		p.fail("undeclared parameter entity in external DTD declaration")
		return
	}
	text := d.Replacement
	if d.External {
		resolved, _, found, err := p.resolveParameter(d)
		if err != nil {
			p.err = err
			return
		}
		if !found {
			p.unreadParameter(name)
			p.fail("unavailable external parameter entity in DTD declaration")
			return
		}
		text = resolved
	}
	p.err = input.push(sourceDTDExternalFrame{kind: sourceDTDExternalInline, name: "%" + name + ";", text: text, declaration: d})
}

// literalParameter includes a parameter entity's replacement text in an entity
// value literal without padding (XML 4.4.5).
func (p *sourceDTDParser) literalParameter(replacement *sourceDTDText) {
	name := p.name(sourceDTDNCName, true)
	p.expect(';')
	if p.err != nil {
		return
	}
	defer p.releaseBytes(int64(len(name)))
	d, known := p.store.Entity(name, true)
	if !known {
		p.unreadParameter(name)
		return
	}
	text := d.Replacement
	if d.External {
		resolved, _, found, err := p.resolveParameter(d)
		if err != nil {
			p.err = err
			return
		}
		if !found {
			p.unreadParameter(name)
			return
		}
		text = resolved
	}
	if replacement.store != nil {
		p.err = replacement.append([]byte(text))
	}
}

func (p *sourceDTDParser) unreadParameter(name string) {
	p.info.Unsupported = true
	p.info.WellFormednessIncomplete = true
	p.store.state.ParameterEntityUnread = true
	if p.store.state.FirstUnreadParameter == "" {
		p.store.state.FirstUnreadParameter = strings.Clone(name)
	}
	p.store.state.BindingCutoff = true
}

// pushExternalParameter enters an external parameter entity referenced between
// declarations.
func (p *sourceDTDParser) pushExternalParameter(d *sourceDTDDeclaration) bool {
	input, ok := p.input.(*sourceDTDExternalInput)
	if !ok || p.resolveParameter == nil {
		return false
	}
	text, systemID, found, err := p.resolveParameter(d)
	if err != nil {
		p.err = err
		return true
	}
	if !found {
		return false
	}
	next, err := p.store.growPointers(p.parameters)
	if err != nil {
		p.err = err
		return true
	}
	p.parameters = next
	if p.err = input.push(sourceDTDExternalFrame{kind: sourceDTDExternalDeclaration, name: "%" + d.Name + ";", systemID: systemID, text: text, declaration: d}); p.err != nil {
		return true
	}
	p.parameters = append(p.parameters, d)
	return true
}

// importExternalDTD reads the DOCTYPE external subset into the document's
// declaration store after its internal subset. An identifier that no source
// or bundled DTD provides stays unread, as before.
func (r *sourceXMLTokenReader) importExternalDTD() error {
	state := r.dtdStore.State()
	if r.dtdResolver == nil || !state.ExternalSubsetUnread {
		return nil
	}
	prepared, err := r.dtdResolver.prepare(r.cursor.ctx, state.PublicID, state.SystemID)
	if err != nil || prepared == nil {
		return err
	}
	return r.dtdStore.importExternalSubset(prepared)
}

// legacyExternalDTDApplies reports whether the grammar-only path may use its
// external subset. An internal subset that the grammar-only path does not
// interpret (declarations of the given kind or parameter-entity references)
// could override it, so it keeps the historical unavailable result.
func (r *sourceXMLTokenReader) legacyExternalDTDApplies(internalDeclarations bool) bool {
	return !r.internalDTD && r.dtdResolver != nil && r.dtd.ExternalSubset && !internalDeclarations && !r.dtd.HasParameterReferences
}

func (r *sourceXMLTokenReader) legacyExternalDTD() (*sourceDTDPrepared, error) {
	if !r.externalDTDResolved {
		r.externalDTDResolved = true
		r.externalDTD, r.externalDTDErr = r.dtdResolver.prepare(r.cursor.ctx, r.dtd.PublicID, r.dtd.SystemID)
	}
	return r.externalDTD, r.externalDTDErr
}

// externalCharacterEntity expands a single-character general entity declared
// by the external subset on the grammar-only path. Attribute normalization
// turns a whitespace replacement into a space (XML 3.3.3).
func (r *sourceXMLTokenReader) externalCharacterEntity(name string, context sourceXMLEntityContext) (rune, bool, error) {
	if !r.legacyExternalDTDApplies(r.dtd.HasEntityDeclarations) {
		return 0, false, nil
	}
	prepared, err := r.legacyExternalDTD()
	if err != nil || prepared == nil {
		return 0, false, err
	}
	c, ok := prepared.characters[name]
	if !ok {
		return 0, false, nil
	}
	r.metadata.Reference = true
	if context == sourceEntityAttribute && sourceXMLSpace(c) {
		c = ' '
	}
	return c, true, nil
}

// applyExternalNamespaceDefaults adds namespace declarations that the
// external subset defaults for an element and the start tag omits. Other
// defaulted attributes are deliberately not added on the grammar-only path. A
// DTD resolution error is kept for unboundPrefix, which reports it only if a
// binding is actually missing.
func (r *sourceXMLTokenReader) applyExternalNamespaceDefaults(element string, attributes []xml.Attr, metadata []sourceXMLAttribute) ([]xml.Attr, []sourceXMLAttribute) {
	if !r.legacyExternalDTDApplies(r.dtd.HasAttributeDeclarations) {
		return attributes, metadata
	}
	prepared, _ := r.legacyExternalDTD()
	if prepared == nil {
		return attributes, metadata
	}
	for _, def := range prepared.namespaces[element] {
		specified := false
		for _, a := range attributes {
			if a.Name == def.Name {
				specified = true
				break
			}
		}
		if !specified {
			attributes = append(attributes, def)
			metadata = append(metadata, sourceXMLAttribute{NamespaceDeclaration: true})
		}
	}
	return attributes, metadata
}
