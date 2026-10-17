package musicxml

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"sync"
	"unicode/utf8"
	"unsafe"

	"github.com/go-muse/go-musicxml/validation"
)

// External DTD subsets are read only from bundled official MusicXML DTDs or
// from a caller DTDSource; nothing is fetched. A resolved subset is parsed by
// the same declaration grammar as the internal subset, extended with the
// external-subset productions: parameter-entity references inside markup
// declarations and entity values, conditional sections and external parameter
// entities.
//
// XML 1.0 section 2.8 processes the internal subset first, so its declarations,
// including parameter entities that customize the external subset, bind first.
// When the internal subset declares no parameter entity, the external subset
// cannot depend on it: its closure is parsed once into an immutable
// sourceDTDPrepared, which documents import after their internal subset.
// Otherwise the external subset is parsed into the document's own store.

const (
	sourceDTDExternalSubsetRegion sourceDTDRegion = "external-subset"
	// sourceDTDExternalEnd is returned by Peek at the end of the external
	// subset itself. Read never crosses it.
	sourceDTDExternalEnd rune = -3
	// sourceDTDExternalTextBytes caps one caller-supplied DTD entity.
	sourceDTDExternalTextBytes = 16 << 20
	// sourceDTDMaxEntityIdentifierBytes bounds the system identifier that
	// source origins report in validation.Location.Entry.
	sourceDTDMaxEntityIdentifierBytes = 4096
)

type sourceDTDExternalFrameKind uint8

const (
	sourceDTDExternalRoot sourceDTDExternalFrameKind = iota
	// A declaration-level frame holds a parameter entity referenced between
	// declarations; its end is the parser-visible parameter boundary.
	sourceDTDExternalDeclaration
	// An inline frame holds a parameter entity referenced inside a markup
	// declaration or an entity value; its end is transparent.
	sourceDTDExternalInline
)

type sourceDTDExternalFrame struct {
	kind sourceDTDExternalFrameKind
	name string
	// systemID is set for frames that hold the text of an external entity;
	// their positions are source-origin coordinates in that entity.
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

func (in *sourceDTDExternalInput) depth() int { return len(in.frames) }

// file returns the innermost frame holding an external entity's text. Inside
// parameter-entity replacement text its position is just after the reference.
func (in *sourceDTDExternalInput) file() *sourceDTDExternalFrame {
	for i := len(in.frames) - 1; i > 0; i-- {
		if in.frames[i].systemID != "" {
			return &in.frames[i]
		}
	}
	return &in.frames[0]
}

func (in *sourceDTDExternalInput) SyntaxError(message string) error {
	file := in.file()
	where := in.frames[0].name
	if file.name != where {
		where += " (" + file.name + ")"
	}
	return &xml.SyntaxError{Msg: "external DTD " + where + ": " + message, Line: max(file.position.Line, 0)}
}

func (in *sourceDTDExternalInput) systemID() string { return in.file().systemID }

// DTDPosition reports the current coordinates in the innermost external
// entity, for the end of a declaration span.
func (in *sourceDTDExternalInput) DTDPosition() sourceXMLPosition { return in.file().position }

// push enters replacement text. A padded frame is enlarged by one leading and
// one following space, as XML 4.4.8 requires for a parameter entity included
// in the DTD; a frame included in an entity value is not (XML 4.4.5).
func (in *sourceDTDExternalInput) push(frame sourceDTDExternalFrame, padded bool) error {
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
	frame.position = sourceXMLPosition{Line: 1, Column: 1}
	if padded {
		frame.text = " " + frame.text + " "
		// The padding space is not part of the entity's text.
		frame.position = sourceXMLPosition{ByteOffset: -1, Line: 1}
	}
	in.frames = append(in.frames, frame)
	return nil
}

func (in *sourceDTDExternalInput) PushDTDParameter(declaration *sourceDTDDeclaration, _ sourceDTDOrigin) error {
	return in.push(sourceDTDExternalFrame{kind: sourceDTDExternalDeclaration, name: "%" + declaration.Name + ";", text: declaration.Replacement, declaration: declaration}, true)
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

// DTDParameterOrigin locates a declaration at the current position in the
// innermost external entity.
func (in *sourceDTDExternalInput) DTDParameterOrigin() (sourceDTDOrigin, error) {
	if err := in.ctx.Err(); err != nil {
		return sourceDTDOrigin{}, err
	}
	origin := sourceDTDOrigin{Region: sourceDTDExternalSubsetRegion, ExternalMarkup: true}
	if file := in.file(); file.systemID != "" && file.position.Line > 0 && file.position.ByteOffset >= 0 {
		origin.Entity = file.systemID
		origin.SpanAvailable = true
		origin.Span.Start = file.position
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
		body, err := sourceDTDTextDeclaration(rest)
		if err != nil {
			return "", err
		}
		text = body
	}
	return text, nil
}

// sourceDTDTextDeclaration consumes the remainder of a text declaration:
// TextDecl ::= '<?xml' VersionInfo? EncodingDecl S? '?>'. It returns the text
// after it and requires UTF-8.
func sourceDTDTextDeclaration(rest string) (string, error) {
	space := func() bool {
		n := len(rest)
		rest = strings.TrimLeft(rest, " \t\n")
		return len(rest) != n
	}
	pseudo := func(name string) (string, bool) {
		saved := rest
		if !space() || !strings.HasPrefix(rest, name) {
			rest = saved
			return "", false
		}
		rest = rest[len(name):]
		space()
		if !strings.HasPrefix(rest, "=") {
			rest = saved
			return "", false
		}
		rest = rest[1:]
		space()
		if rest == "" || rest[0] != '"' && rest[0] != '\'' {
			rest = saved
			return "", false
		}
		end := strings.IndexByte(rest[1:], rest[0])
		if end < 0 {
			rest = saved
			return "", false
		}
		value := rest[1 : end+1]
		rest = rest[end+2:]
		return value, true
	}
	if version, ok := pseudo("version"); ok && !strings.HasPrefix(version, "1.") {
		return "", fmt.Errorf("unsupported XML version %q in external DTD text declaration", version)
	}
	encoding, ok := pseudo("encoding")
	if !ok {
		return "", errors.New("text declaration in external DTD requires an encoding")
	}
	if normalizeCharset(encoding) != "utf8" {
		return "", fmt.Errorf("external DTD encoding %q is not supported; only UTF-8 is", encoding)
	}
	space()
	if !strings.HasPrefix(rest, "?>") {
		return "", errors.New("malformed text declaration in external DTD")
	}
	return rest[2:], nil
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

// newSourceDTDResolver ignores a nil or typed-nil source, like the dictionary
// sources, rather than calling a method on it.
func newSourceDTDResolver(options *DTDOptions) *sourceDTDResolver {
	resolver := &sourceDTDResolver{}
	if options != nil && !dictionaryNil(options.Source) {
		resolver.source = options.Source
	}
	return resolver
}

// sourceDTDEntity is one resolved external entity.
type sourceDTDEntity struct {
	name, systemID, text string
	bundled              bool
	size                 int64
}

var bundledDTDPrepared sync.Map // file name -> *bundledDTDPreparation

type bundledDTDPreparation struct {
	once     sync.Once
	prepared *sourceDTDPrepared
	err      error
}

// root resolves the DOCTYPE external subset: the caller source first, then
// the bundled DTDs. found is false when neither provides it.
func (r *sourceDTDResolver) root(ctx context.Context, publicID, systemID string) (sourceDTDEntity, bool, error) {
	if r == nil || publicID == "" && systemID == "" {
		return sourceDTDEntity{}, false, nil
	}
	if r.source != nil {
		data, err := r.source.OpenDTD(ctx, DTDRequest{PublicID: publicID, SystemID: systemID})
		switch {
		case err == nil:
			text, textErr := sourceDTDExternalText(data)
			if textErr != nil {
				return sourceDTDEntity{}, false, &DTDError{PublicID: publicID, SystemID: systemID, Err: textErr}
			}
			return sourceDTDEntity{name: systemID, systemID: systemID, text: text, size: int64(len(data))}, true, nil
		case errors.Is(err, ErrDTDNotFound):
		case ctx.Err() != nil:
			return sourceDTDEntity{}, false, ctx.Err()
		default:
			return sourceDTDEntity{}, false, &DTDError{PublicID: publicID, SystemID: systemID, Err: err}
		}
	}
	file, ok := bundledDTDLookup(publicID, systemID, "")
	if !ok {
		return sourceDTDEntity{}, false, nil
	}
	return sourceDTDEntity{name: file, systemID: bundledDTDSystemPrefixes[0] + file, bundled: true}, true, nil
}

// prepare returns the immutable external subset named by a DOCTYPE, or nil
// when no source or bundled DTD provides it. A bundled closure is prepared
// once per process; a caller-supplied one for every call.
func (r *sourceDTDResolver) prepare(ctx context.Context, publicID, systemID string) (*sourceDTDPrepared, error) {
	entity, found, err := r.root(ctx, publicID, systemID)
	if err != nil || !found {
		return nil, err
	}
	if !entity.bundled {
		prepared, err := prepareSourceDTD(ctx, r, entity, false)
		if err != nil {
			return nil, &DTDError{PublicID: publicID, SystemID: systemID, Err: err}
		}
		return prepared, nil
	}
	value, _ := bundledDTDPrepared.LoadOrStore(entity.name, &bundledDTDPreparation{})
	preparation := value.(*bundledDTDPreparation)
	preparation.once.Do(func() {
		// The bundled closure is static, so its one preparation is not tied to
		// any caller's cancellation.
		text, err := bundledDTDEntityText(entity.name)
		if err != nil {
			preparation.err = err
			return
		}
		entity.text, entity.size = text, int64(len(text))
		preparation.prepared, preparation.err = prepareSourceDTD(context.Background(), &sourceDTDResolver{}, entity, true)
	})
	if preparation.err != nil {
		return nil, &DTDError{PublicID: publicID, SystemID: systemID, Err: preparation.err}
	}
	return preparation.prepared, nil
}

// parameter resolves an external parameter entity relative to base, the
// system identifier of the entity containing its declaration (XML 4.2.2).
// Entities of a caller-supplied subset ask the caller first; bundled
// closures never do.
func (r *sourceDTDResolver) parameter(ctx context.Context, d *sourceDTDDeclaration, base string, bundledOnly bool) (sourceDTDEntity, bool, error) {
	if !bundledOnly && r.source != nil {
		data, err := r.source.OpenDTD(ctx, DTDRequest{PublicID: d.PublicID, SystemID: d.SystemID, BaseSystemID: base})
		switch {
		case err == nil:
			text, textErr := sourceDTDExternalText(data)
			if textErr != nil {
				return sourceDTDEntity{}, false, &DTDError{PublicID: d.PublicID, SystemID: d.SystemID, Err: textErr}
			}
			systemID := sourceDTDJoinSystemID(base, d.SystemID)
			return sourceDTDEntity{name: systemID, systemID: systemID, text: text, size: int64(len(data))}, true, nil
		case errors.Is(err, ErrDTDNotFound):
		case ctx.Err() != nil:
			return sourceDTDEntity{}, false, ctx.Err()
		default:
			return sourceDTDEntity{}, false, &DTDError{PublicID: d.PublicID, SystemID: d.SystemID, Err: err}
		}
	}
	file, ok := bundledDTDLookup(d.PublicID, d.SystemID, base)
	if !ok {
		return sourceDTDEntity{}, false, nil
	}
	text, err := bundledDTDEntityText(file)
	if err != nil {
		return sourceDTDEntity{}, false, err
	}
	return sourceDTDEntity{name: file, systemID: bundledDTDSystemPrefixes[0] + file, text: text, bundled: true, size: int64(len(text))}, true, nil
}

// sourceDTDJoinSystemID resolves a relative system identifier against an
// absolute URI base (RFC 3986), or against a relative base path.
func sourceDTDJoinSystemID(base, systemID string) string {
	if base == "" || systemID == "" {
		return systemID
	}
	if reference, err := url.Parse(systemID); err == nil {
		if reference.IsAbs() {
			return systemID
		}
		if parsed, err := url.Parse(base); err == nil && parsed.IsAbs() {
			return parsed.ResolveReference(reference).String()
		}
	}
	if strings.HasPrefix(systemID, "/") {
		return systemID
	}
	return path.Clean(path.Join(path.Dir(base), systemID))
}

// sourceDTDExternalError keeps an external DTD's syntax error from matching
// *xml.SyntaxError, which identifies a defect in the document itself.
func sourceDTDExternalError(err error) error {
	var syntax *xml.SyntaxError
	if errors.As(err, &syntax) {
		if syntax.Line > 0 {
			return fmt.Errorf("%s (line %d)", syntax.Msg, syntax.Line)
		}
		return errors.New(syntax.Msg)
	}
	return err
}

// parseExternalSubset parses one external subset into store, whose account
// also charges the parameter-entity replacement work. bundledOnly closures
// never consult the caller source.
func parseExternalSubset(ctx context.Context, store *sourceDTDStore, resolver *sourceDTDResolver, root sourceDTDEntity, bundledOnly bool) (int64, error) {
	size := root.size
	input := newSourceDTDExternalInput(ctx, store.account, root.name, root.systemID, root.text)
	p := sourceDTDParser{input: input, store: store, external: true}
	p.resolveParameter = func(d *sourceDTDDeclaration) (sourceDTDEntity, bool, error) {
		base := d.Origin.Entity
		if base == "" && d.Origin.Region == sourceDTDExternalSubsetRegion {
			base = input.systemID()
		}
		entity, found, err := resolver.parameter(ctx, d, base, bundledOnly)
		if err == nil && found {
			if err = store.account.Charge(validation.BudgetExternalBytes, entity.size); err != nil {
				return sourceDTDEntity{}, false, err
			}
			size += entity.size
		}
		return entity, found, err
	}
	defer func() { store.release(int64(cap(p.parameters)) * int64(unsafe.Sizeof((*sourceDTDDeclaration)(nil)))) }()
	p.subset()
	if p.err == nil && (p.conditionalDepth != 0 || len(p.parameters) != 0) {
		p.fail("unterminated conditional section or parameter entity")
	}
	return size, p.err
}

// prepareSourceDTD parses one external subset with a private account and
// returns its binding declarations. Its bounded events also limit parameter
// entity amplification in caller DTDs.
func prepareSourceDTD(ctx context.Context, resolver *sourceDTDResolver, root sourceDTDEntity, bundledOnly bool) (*sourceDTDPrepared, error) {
	account, err := validation.NewBudgetAccount(validation.Budgets{})
	if err != nil {
		return nil, err
	}
	store, err := newSourceDTDStore(ctx, account, sourceDTDStandaloneAbsent)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	store.parsed = true
	size, err := parseExternalSubset(ctx, store, resolver, root, bundledOnly)
	if err != nil {
		return nil, sourceDTDExternalError(err)
	}
	if store.state.ParameterEntityUnread {
		return nil, fmt.Errorf("external DTD references the unavailable parameter entity %q", store.state.FirstUnreadParameter)
	}
	prepared := &sourceDTDPrepared{bytes: size}
	for _, d := range store.Declarations() {
		if !d.Binding || d.Kind == sourceDTDParameterEntityDeclaration {
			continue
		}
		record := *d
		record.Origin.Ancestry = nil
		prepared.declarations = append(prepared.declarations, record)
		switch {
		case record.Kind == sourceDTDGeneralEntityDeclaration && !record.External && record.Notation == "":
			if r, n := utf8.DecodeRuneInString(record.Replacement); n == len(record.Replacement) && n > 0 && r != '<' && r != '&' && sourceXMLChar(r) {
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

// sourceDTDRecordBytes is the retained string storage of one declaration.
func sourceDTDRecordBytes(d *sourceDTDDeclaration) int64 {
	return int64(len(d.Name) + len(d.Element) + len(d.AttributeType) + len(d.RawType) + len(d.RawValue) + len(d.Replacement) + len(d.PublicID) + len(d.SystemID) + len(d.Notation) + len(d.Origin.Entity))
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
		if err := s.reserve(sourceDTDRecordBytes(&d)); err != nil {
			return err
		}
		if err := s.add(d); err != nil {
			return err
		}
	}
	s.completeExternalSubset()
	return nil
}

// completeExternalSubset recomputes declaration completeness once the
// external subset has been read.
func (s *sourceDTDStore) completeExternalSubset() {
	s.state.ExternalSubsetUnread = false
	complete := !s.failed && s.state.GrammarComplete && !s.state.ParameterEntityUnread
	s.state.DeclarationsComplete = complete
	s.state.AttributeDeclarationsComplete = complete
	s.state.UnparsedEntitiesComplete = complete
	s.refreshWellFormedness()
}

// declaresParameterEntity reports whether the internal subset bound a
// parameter entity that the external subset could reference.
func (s *sourceDTDStore) declaresParameterEntity() bool {
	for _, d := range s.declarations {
		if d.Kind == sourceDTDParameterEntityDeclaration && d.Binding {
			return true
		}
	}
	return false
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

// closeConditionalSection consumes ]]>. A section must close in the parameter
// entity replacement text that opened it (XML 2.8, PE Between Declarations).
func (p *sourceDTDParser) closeConditionalSection() {
	if n := len(p.parameterConditionals); n != 0 && p.conditionalDepth <= p.parameterConditionals[n-1] {
		p.fail("conditional section closes outside the parameter entity that opened it")
		return
	}
	p.read()
	p.expect(']')
	p.expect('>')
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

// parameterText resolves the replacement text of a parameter entity
// referenced in the external subset.
func (p *sourceDTDParser) parameterText(name string) (*sourceDTDDeclaration, sourceDTDEntity, bool) {
	d, known := p.store.Entity(name, true)
	if !known {
		p.unreadParameter(name)
		return nil, sourceDTDEntity{}, false
	}
	if !d.External {
		return d, sourceDTDEntity{name: "%" + name + ";", text: d.Replacement}, true
	}
	entity, found, err := p.resolveParameter(d)
	if err != nil {
		p.err = err
		return nil, sourceDTDEntity{}, false
	}
	if !found {
		p.unreadParameter(name)
		return nil, sourceDTDEntity{}, false
	}
	return d, entity, true
}

// referenceName consumes name; after the percent sign of a parameter-entity
// reference. Inside a markup declaration the reference is replaced, so it is
// not captured; inside a literal it is part of the raw spelling.
func (p *sourceDTDParser) referenceName(capture bool) string {
	if !capture {
		saved := p.capture
		p.capture = nil
		defer func() { p.capture = saved }()
	}
	name := p.name(sourceDTDNCName, true)
	if p.err != nil {
		return ""
	}
	if p.read() != ';' {
		p.releaseBytes(int64(len(name)))
		p.fail("parameter entity reference requires ;")
		return ""
	}
	return name
}

// declarationReference replaces a parameter-entity reference recognized inside
// a markup declaration of the external subset with its padded replacement.
func (p *sourceDTDParser) declarationReference(input *sourceDTDExternalInput) {
	if _, err := input.Read(); err != nil {
		p.err = err
		return
	}
	name := p.referenceName(false)
	if p.err != nil {
		return
	}
	defer p.releaseBytes(int64(len(name)))
	d, entity, ok := p.parameterText(name)
	if !ok {
		p.fail("undeclared or unavailable parameter entity in external DTD declaration")
		return
	}
	p.err = input.push(sourceDTDExternalFrame{kind: sourceDTDExternalInline, name: "%" + name + ";", systemID: entity.externalSystemID(d), text: entity.text, declaration: d}, true)
}

// literalParameter includes a parameter entity's replacement text in an entity
// value literal: it is processed in place of the reference, unpadded, so its
// own references are recognized and its quotes do not end the literal (XML
// 4.4.5). The percent sign has already been read.
func (p *sourceDTDParser) literalParameter(input *sourceDTDExternalInput) {
	name := p.referenceName(true)
	if p.err != nil {
		return
	}
	defer p.releaseBytes(int64(len(name)))
	d, entity, ok := p.parameterText(name)
	if !ok {
		return
	}
	p.err = input.push(sourceDTDExternalFrame{kind: sourceDTDExternalInline, name: "%" + name + ";", systemID: entity.externalSystemID(d), text: entity.text, declaration: d}, false)
}

// externalSystemID is the frame identity for an external parameter entity's
// text; internal replacement text has none.
func (e sourceDTDEntity) externalSystemID(d *sourceDTDDeclaration) string {
	if d != nil && d.External {
		return e.systemID
	}
	return ""
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
	entity, found, err := p.resolveParameter(d)
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
	if p.err = input.push(sourceDTDExternalFrame{kind: sourceDTDExternalDeclaration, name: "%" + d.Name + ";", systemID: entity.systemID, text: entity.text, declaration: d}, true); p.err != nil {
		return true
	}
	p.parameters = append(p.parameters, d)
	p.parameterConditionals = append(p.parameterConditionals, p.conditionalDepth)
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
	if !r.dtdStore.declaresParameterEntity() {
		prepared, err := r.dtdResolver.prepare(r.cursor.ctx, state.PublicID, state.SystemID)
		if err != nil || prepared == nil {
			return err
		}
		return r.dtdStore.importExternalSubset(prepared)
	}
	// Internal parameter entities can customize the external subset, so it is
	// parsed here with them in scope, charged to the document account.
	root, found, err := r.dtdResolver.root(r.cursor.ctx, state.PublicID, state.SystemID)
	if err != nil || !found {
		return err
	}
	if root.bundled {
		if root.text, err = bundledDTDEntityText(root.name); err != nil {
			return &DTDError{PublicID: state.PublicID, SystemID: state.SystemID, Err: err}
		}
		root.size = int64(len(root.text))
	}
	if err := r.account.Charge(validation.BudgetExternalBytes, root.size); err != nil {
		return err
	}
	if _, err := parseExternalSubset(r.cursor.ctx, r.dtdStore, r.dtdResolver, root, root.bundled); err != nil {
		var resource *validation.ResourceError
		if errors.As(err, &resource) || r.cursor.ctx.Err() != nil {
			return err
		}
		return &DTDError{PublicID: state.PublicID, SystemID: state.SystemID, Err: sourceDTDExternalError(err)}
	}
	r.dtdStore.completeExternalSubset()
	return nil
}

// legacyExternalDTDApplies reports whether the grammar-only path may use its
// external subset. That path does not interpret an internal subset, so any
// internal entity or attribute-list declaration or parameter-entity reference,
// which could override the external subset, keeps the historical result.
func (r *sourceXMLTokenReader) legacyExternalDTDApplies() bool {
	return !r.internalDTD && r.dtdResolver != nil && r.dtd.ExternalSubset &&
		!r.dtd.HasEntityDeclarations && !r.dtd.HasAttributeDeclarations && !r.dtd.HasParameterReferences
}

func (r *sourceXMLTokenReader) legacyExternalDTD() (*sourceDTDPrepared, error) {
	if !r.externalDTDResolved {
		r.externalDTDResolved = true
		r.externalDTD, r.externalDTDErr = r.dtdResolver.prepare(r.cursor.ctx, r.dtd.PublicID, r.dtd.SystemID)
	}
	return r.externalDTD, r.externalDTDErr
}

// externalCharacterEntity expands a single-character general entity declared
// by the external subset on the grammar-only path. A standalone document may
// not use it (XML 4.1, Entity Declared). Attribute normalization turns a
// whitespace replacement into a space (XML 3.3.3).
func (r *sourceXMLTokenReader) externalCharacterEntity(name string, context sourceXMLEntityContext) (rune, bool, error) {
	if !r.legacyExternalDTDApplies() || r.standalone == sourceDTDStandaloneYes {
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
	if !r.legacyExternalDTDApplies() {
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
