package musicxml

import (
	"context"
	"math"
	"strings"
	"unicode/utf8"
	"unsafe"

	"github.com/go-muse/go-musicxml/validation"
)

type sourceDTDStandalone string

const (
	sourceDTDStandaloneAbsent sourceDTDStandalone = ""
	sourceDTDStandaloneNo     sourceDTDStandalone = "no"
	sourceDTDStandaloneYes    sourceDTDStandalone = "yes"
)

type sourceDTDDeclarationKind string

const (
	sourceDTDElementDeclaration         sourceDTDDeclarationKind = "element"
	sourceDTDAttributeDeclaration       sourceDTDDeclarationKind = "attribute"
	sourceDTDGeneralEntityDeclaration   sourceDTDDeclarationKind = "general-entity"
	sourceDTDParameterEntityDeclaration sourceDTDDeclarationKind = "parameter-entity"
	sourceDTDNotationDeclaration        sourceDTDDeclarationKind = "notation"
)

type sourceDTDDefaultKind string

const (
	sourceDTDDefaultValue    sourceDTDDefaultKind = "value"
	sourceDTDDefaultFixed    sourceDTDDefaultKind = "fixed"
	sourceDTDDefaultRequired sourceDTDDefaultKind = "required"
	sourceDTDDefaultImplied  sourceDTDDefaultKind = "implied"
)

type sourceDTDIgnoredReason string

const (
	sourceDTDIgnoredDuplicate sourceDTDIgnoredReason = "duplicate"
	sourceDTDIgnoredCutoff    sourceDTDIgnoredReason = "unread-parameter-entity"
)

type sourceDTDRegion string

const (
	sourceDTDInternalSubset        sourceDTDRegion = "internal-subset"
	sourceDTDParameterEntityRegion sourceDTDRegion = "parameter-entity"
)

// An ancestry entry distinguishes a parameter entity's declaration from the
// particular reference which entered its replacement. DeclarationRecord retains
// provenance for a declaration itself generated in a no-longer-active entity.
// All records and name strings are borrowed from this same store.
type sourceDTDOriginStep struct {
	Entity                             string
	DeclarationAvailable, UseAvailable bool
	Declaration, Use                   sourceXMLSpan
	ReplacementOffset                  int64
	DeclarationRecord                  *sourceDTDDeclaration
}

type sourceDTDOrigin struct {
	Sequence int64
	Region   sourceDTDRegion
	// Entity, when nonempty, is the system identifier of the external DTD
	// entity whose normalized text Span is relative to; otherwise Span is in
	// the document's original bytes.
	Entity           string
	Span             sourceXMLSpan
	SpanAvailable    bool
	ExternalMarkup   bool
	Ancestry         []sourceDTDOriginStep
	AncestryComplete bool
}
type sourceDTDDeclaration struct {
	Kind                                       sourceDTDDeclarationKind
	Name, Element                              string
	AttributeType, RawType                     string
	DefaultKind                                sourceDTDDefaultKind
	RawValue, Replacement                      string
	PublicID, SystemID, Notation               string
	Parameter, External, ActuallyRead, Binding bool
	IgnoredReason                              sourceDTDIgnoredReason
	Origin                                     sourceDTDOrigin
	HasGeneralReferences                       bool
	RequiresContentCheck                       bool
}
type sourceDTDStoreState struct {
	Name                                                                              string
	EntityWellFormednessIncomplete, DefaultWellFormednessIncomplete                   bool
	PredefinedEntityWellFormednessIncomplete, ParameterEntityWellFormednessIncomplete bool
	Standalone                                                                        sourceDTDStandalone
	PublicID, SystemID                                                                string
	GrammarComplete, DeclarationsComplete                                             bool
	AttributeDeclarationsComplete, UnparsedEntitiesComplete                           bool
	WellFormednessIncomplete                                                          bool
	ExternalSubsetUnread, ParameterEntityUnread, BindingCutoff                        bool
	ParameterEntityReferences                                                         bool
	FirstUnreadParameter                                                              string
}

// The store owns all returned strings, records, pointer slices and indexes.
// Borrowed records are immutable and valid until Close. It is deliberately not
// concurrency-safe: like the source scanner it has one owner. Logical map-entry
// allowances include key/value headers and amortized table storage, not a claim
// to measure the runtime heap. Strings in indexes borrow retained record keys.
const (
	sourceDTDStoreIndexBytes int64 = 128
	sourceDTDStoreMapBytes   int64 = 64
)

type sourceDTDStoreKey struct {
	kind          sourceDTDDeclarationKind
	element, name string
}
type sourceDTDStore struct {
	parametersRead      map[*sourceDTDDeclaration]bool
	entitiesContentRead map[*sourceDTDDeclaration]bool
	// segments maps the replacement text of parameter entities declared in
	// an external subset to where it is written.
	segments map[*sourceDTDDeclaration][]sourceDTDTextSegment
	// bases records, for an external parameter entity declared in an
	// external subset, the external entity being read when its declaration
	// was parsed: the base of a relative system identifier (XML 4.2.2).
	bases map[*sourceDTDDeclaration]string
	// externalParameterIncomplete carries an imported closure's unprocessed
	// parameter-entity replacement.
	externalParameterIncomplete bool
	entitiesUnchecked           int64
	context                     context.Context
	account                     *validation.Account
	state                       sourceDTDStoreState
	declarations                []*sourceDTDDeclaration
	bindings                    map[sourceDTDStoreKey]*sourceDTDDeclaration
	attributes                  map[string][]*sourceDTDDeclaration
	owned                       int64
	parsed, closed, failed      bool
}

func newSourceDTDStore(ctx context.Context, account *validation.Account, standalone sourceDTDStandalone) (*sourceDTDStore, error) {
	if ctx == nil {
		return nil, &validation.ConfigurationError{Field: "Context", Reason: "DTD collection requires a nonnil context"}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := account.Charge(validation.BudgetBufferedBytes, 0); err != nil {
		return nil, err
	}
	switch standalone {
	case sourceDTDStandaloneAbsent, sourceDTDStandaloneNo, sourceDTDStandaloneYes:
	default:
		return nil, &validation.ConfigurationError{Field: "Standalone", Reason: "must be absent, no or yes"}
	}
	n := int64(unsafe.Sizeof(sourceDTDStore{})) + 2*sourceDTDStoreMapBytes
	if err := account.Charge(validation.BudgetBufferedBytes, n); err != nil {
		return nil, err
	}
	return &sourceDTDStore{context: ctx, account: account, state: sourceDTDStoreState{Standalone: standalone}, bindings: make(map[sourceDTDStoreKey]*sourceDTDDeclaration), attributes: make(map[string][]*sourceDTDDeclaration), owned: n}, nil
}
func (s *sourceDTDStore) Close() error {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true
	err := s.account.Release(validation.BudgetBufferedBytes, s.owned)
	s.owned = 0
	s.declarations, s.bindings, s.attributes = nil, nil, nil
	s.parametersRead, s.entitiesContentRead, s.segments, s.bases = nil, nil, nil, nil
	s.state = sourceDTDStoreState{}
	return err
}
func (s *sourceDTDStore) check() error {
	if s == nil || s.closed {
		return &validation.ConfigurationError{Field: "DTDStore", Reason: "must be open"}
	}
	return s.context.Err()
}
func (s *sourceDTDStore) reserve(n int64) error {
	if err := s.check(); err != nil {
		return err
	}
	if _, err := validationAllocationSize(n); err != nil {
		return err
	}
	if err := s.account.Charge(validation.BudgetBufferedBytes, n); err != nil {
		return err
	}
	s.owned += n
	return nil
}
func (s *sourceDTDStore) release(n int64) {
	_ = s.account.Release(validation.BudgetBufferedBytes, n)
	s.owned -= n
}
func (s *sourceDTDStore) Attribute(element, name string) (*sourceDTDDeclaration, bool) {
	d, ok := s.bindings[sourceDTDStoreKey{kind: sourceDTDAttributeDeclaration, element: element, name: name}]
	return d, ok
}
func (s *sourceDTDStore) Attributes(element string) []*sourceDTDDeclaration {
	return s.attributes[element]
}
func (s *sourceDTDStore) Entity(name string, parameter bool) (*sourceDTDDeclaration, bool) {
	kind := sourceDTDGeneralEntityDeclaration
	if parameter {
		kind = sourceDTDParameterEntityDeclaration
	}
	d, ok := s.bindings[sourceDTDStoreKey{kind: kind, name: name}]
	return d, ok
}
func (s *sourceDTDStore) Notation(name string) (*sourceDTDDeclaration, bool) {
	d, ok := s.bindings[sourceDTDStoreKey{kind: sourceDTDNotationDeclaration, name: name}]
	return d, ok
}
func (s *sourceDTDStore) Declarations() []*sourceDTDDeclaration { return s.declarations }
func (s *sourceDTDStore) State() sourceDTDStoreState            { return s.state }

// parseSourceDTDWithStore elects declaration collection without changing the
// legacy grammar-only entry point. On failure the caller still owns the store
// and its reliable earlier bindings; Close releases them on every terminal path.
// The returned info.Name is borrowed from the store, unlike the legacy input's
// token-owned name. This function never retrieves an external identifier.
func parseSourceDTDWithStore(input sourceDTDInput, store *sourceDTDStore) (sourceDTDInfo, error) {
	if err := store.check(); err != nil {
		return sourceDTDInfo{}, err
	}
	if input == nil {
		return sourceDTDInfo{}, &validation.ConfigurationError{Field: "DTDInput", Reason: "must not be nil"}
	}
	if store.parsed {
		return sourceDTDInfo{}, &validation.ConfigurationError{Field: "DTDStore", Reason: "already parsed"}
	}
	store.parsed = true
	p := sourceDTDParser{input: input, store: store}
	defer func() { store.release(int64(cap(p.parameters)) * int64(unsafe.Sizeof((*sourceDTDDeclaration)(nil)))) }()
	p.doctype()
	if p.err == nil {
		p.err = store.check()
	}
	grammarComplete := p.err == nil
	if p.err == nil {
		p.checkDefaultReferences()
	}
	store.state.Name = p.info.Name
	store.failed = p.err != nil
	store.state.ParameterEntityReferences = p.info.HasParameterReferences
	store.state.GrammarComplete = grammarComplete
	store.state.ParameterEntityWellFormednessIncomplete = false
	for _, d := range store.declarations {
		if d.Parameter && d.Binding && strings.Contains(d.Replacement, "%") && !store.parametersRead[d] {
			store.state.ParameterEntityWellFormednessIncomplete = true
			break
		}
	}
	store.refreshWellFormedness()
	p.info.WellFormednessIncomplete = store.state.WellFormednessIncomplete
	complete := p.err == nil && !store.state.ExternalSubsetUnread && !store.state.ParameterEntityUnread
	store.state.DeclarationsComplete = complete
	store.state.AttributeDeclarationsComplete = complete
	store.state.UnparsedEntitiesComplete = complete
	return p.info, p.err
}

func (s *sourceDTDStore) growPointers(old []*sourceDTDDeclaration) ([]*sourceDTDDeclaration, error) {
	if len(old) < cap(old) {
		return old, nil
	}
	capacity := int64(cap(old))
	if capacity > math.MaxInt64/2 {
		return nil, &validation.ResourceError{Kind: validation.BudgetBufferedBytes, Requested: capacity}
	}
	capacity *= 2
	if capacity < 4 {
		capacity = 4
	}
	size := int64(unsafe.Sizeof((*sourceDTDDeclaration)(nil)))
	if capacity > math.MaxInt64/size {
		return nil, &validation.ResourceError{Kind: validation.BudgetBufferedBytes, Requested: capacity}
	}
	n := capacity * size
	count, err := validationAllocationSize(n)
	if err != nil {
		return nil, err
	}
	if err := s.reserve(n); err != nil {
		return nil, err
	}
	next := make([]*sourceDTDDeclaration, len(old), count/int(size))
	copy(next, old)
	s.release(int64(cap(old)) * size)
	return next, nil
}
func (s *sourceDTDStore) add(d sourceDTDDeclaration) error {
	if err := s.check(); err != nil {
		return err
	}
	// Every read declaration record is raw declaration-event work, including an
	// ignored duplicate or a binding suppressed after an unread parameter entity.
	if err := s.account.Charge(validation.BudgetEvents, 1); err != nil {
		return err
	}
	key := sourceDTDStoreKey{kind: d.Kind, element: d.Element, name: d.Name}
	_, duplicate := s.bindings[key]
	cutoff := s.state.BindingCutoff && (d.Kind == sourceDTDAttributeDeclaration || d.Kind == sourceDTDGeneralEntityDeclaration || d.Kind == sourceDTDParameterEntityDeclaration)
	d.ActuallyRead = true
	d.Binding = !duplicate && !cutoff
	if cutoff {
		d.IgnoredReason = sourceDTDIgnoredCutoff
	} else if duplicate {
		d.IgnoredReason = sourceDTDIgnoredDuplicate
	}
	d.Origin.Sequence = int64(len(s.declarations)) + 1
	next, err := s.growPointers(s.declarations)
	if err != nil {
		return err
	}
	s.declarations = next
	if err := s.reserve(int64(unsafe.Sizeof(sourceDTDDeclaration{}))); err != nil {
		return err
	}
	if d.Binding {
		if err := s.reserve(sourceDTDStoreIndexBytes); err != nil {
			return err
		}
		if d.Kind == sourceDTDAttributeDeclaration {
			group, exists := s.attributes[d.Element]
			if !exists {
				if err := s.reserve(sourceDTDStoreIndexBytes); err != nil {
					return err
				}
			}
			group, err = s.growPointers(group)
			if err != nil {
				return err
			}
			s.attributes[d.Element] = group
		}
	}
	// A declaration's immutable syntax facts are distinct from verification of
	// one actual replacement context. Simple replacement text has no arbitrary
	// reference/markup obligation; a successful content frame may prove the
	// remaining content obligation later without bypassing any subsequent use.
	if d.Kind == sourceDTDGeneralEntityDeclaration {
		simple, predefined, err := s.entityReplacementClosure(d)
		if err != nil {
			return err
		}
		d.RequiresContentCheck = d.Binding && !d.External && !simple
		if d.RequiresContentCheck {
			s.entitiesUnchecked++
			s.state.EntityWellFormednessIncomplete = true
		}
		if !predefined {
			s.state.PredefinedEntityWellFormednessIncomplete = true
		}
	} else if d.Kind == sourceDTDAttributeDeclaration && d.HasGeneralReferences {
		s.state.DefaultWellFormednessIncomplete = true
	}
	var originErr error
	d.Origin, originErr = s.copyOrigin(d.Origin)
	if originErr != nil {
		return originErr
	}
	record := new(sourceDTDDeclaration)
	*record = d
	s.declarations = append(s.declarations, record)
	if d.Binding {
		s.bindings[key] = record
		if d.Kind == sourceDTDAttributeDeclaration {
			s.attributes[d.Element] = append(s.attributes[d.Element], record)
		}
	}
	return nil
}

// sourceDTDText admits both old and new capacity before copying and the final
// immutable string before conversion. Callers release only scratch capacity;
// the final string remains owned by the store until Close, including failures.
type sourceDTDText struct {
	store *sourceDTDStore
	value []byte
}

func (b *sourceDTDText) appendRune(r rune) error {
	var encoded [utf8.UTFMax]byte
	n := utf8.EncodeRune(encoded[:], r)
	return b.append(encoded[:n])
}
func (b *sourceDTDText) append(value []byte) error {
	if err := b.store.check(); err != nil {
		return err
	}
	if int64(len(value)) > math.MaxInt64-int64(len(b.value)) {
		return &validation.ResourceError{Kind: validation.BudgetBufferedBytes, Requested: math.MaxInt64}
	}
	n := int64(len(b.value)) + int64(len(value))
	if err := b.store.account.Check(validation.BudgetScalarBytes, n); err != nil {
		return err
	}
	if n > int64(cap(b.value)) {
		capacity := int64(cap(b.value))
		if capacity <= math.MaxInt64/2 {
			capacity *= 2
		} else {
			capacity = math.MaxInt64
		}
		if capacity < 32 {
			capacity = 32
		}
		if capacity < n {
			capacity = n
		}
		// Never reserve geometric slack past the per-scalar limit.
		if limit := b.store.account.Limits().MaxScalarBytes; capacity > limit {
			capacity = limit
		}
		count, err := validationAllocationSize(capacity)
		if err != nil {
			return err
		}
		if err := b.store.reserve(capacity); err != nil {
			return err
		}
		next := make([]byte, len(b.value), count)
		copy(next, b.value)
		b.store.release(int64(cap(b.value)))
		b.value = next
	}
	b.value = append(b.value, value...)
	return nil
}
func (b *sourceDTDText) string() (string, error) {
	if err := b.store.reserve(int64(len(b.value))); err != nil {
		return "", err
	}
	return string(b.value), nil
}
func (b *sourceDTDText) close() { b.store.release(int64(cap(b.value))); b.value = nil }

func (s *sourceDTDStore) copyOrigin(origin sourceDTDOrigin) (sourceDTDOrigin, error) {
	if len(origin.Ancestry) == 0 {
		return origin, nil
	}
	const stepBytes = int64(160)
	if int64(len(origin.Ancestry)) > math.MaxInt64/stepBytes {
		return sourceDTDOrigin{}, &validation.ResourceError{Kind: validation.BudgetBufferedBytes, Requested: math.MaxInt64}
	}
	if err := s.reserve(int64(len(origin.Ancestry)) * stepBytes); err != nil {
		return sourceDTDOrigin{}, err
	}
	ancestry := make([]sourceDTDOriginStep, len(origin.Ancestry))
	copy(ancestry, origin.Ancestry)
	origin.Ancestry = ancestry
	return origin, nil
}

// replacementSegments returns the replacement map of a parameter entity
// declared in an external subset, or nil.
func (s *sourceDTDStore) replacementSegments(d *sourceDTDDeclaration) []sourceDTDTextSegment {
	return s.segments[d]
}

// setReplacementSegments retains a copy of a parameter entity's replacement
// map. The entity strings are borrowed from the declaring frames.
func (s *sourceDTDStore) setReplacementSegments(d *sourceDTDDeclaration, segments []sourceDTDTextSegment) error {
	if len(segments) == 0 {
		return nil
	}
	if s.segments == nil {
		if err := s.reserve(sourceDTDStoreMapBytes); err != nil {
			return err
		}
		s.segments = make(map[*sourceDTDDeclaration][]sourceDTDTextSegment)
	}
	if int64(len(segments)) > (math.MaxInt64-sourceDTDStoreIndexBytes)/sourceDTDTextSegmentBytes {
		return &validation.ResourceError{Kind: validation.BudgetBufferedBytes, Requested: math.MaxInt64}
	}
	if err := s.reserve(sourceDTDStoreIndexBytes + int64(len(segments))*sourceDTDTextSegmentBytes); err != nil {
		return err
	}
	s.segments[d] = append([]sourceDTDTextSegment(nil), segments...)
	return nil
}

func (s *sourceDTDStore) declarationBase(d *sourceDTDDeclaration) (string, bool) {
	base, ok := s.bases[d]
	return base, ok
}

// setDeclarationBase records a declaration's base. The string is borrowed
// from the declaring frame.
func (s *sourceDTDStore) setDeclarationBase(d *sourceDTDDeclaration, base string) error {
	if s.bases == nil {
		if err := s.reserve(sourceDTDStoreMapBytes); err != nil {
			return err
		}
		s.bases = make(map[*sourceDTDDeclaration]string)
	}
	if err := s.reserve(sourceDTDStoreIndexBytes); err != nil {
		return err
	}
	s.bases[d] = base
	return nil
}

func (s *sourceDTDStore) parameterRead(d *sourceDTDDeclaration) error {
	if s.parametersRead[d] {
		return nil
	}
	if s.parametersRead == nil {
		if err := s.reserve(sourceDTDStoreMapBytes); err != nil {
			return err
		}
		s.parametersRead = make(map[*sourceDTDDeclaration]bool)
	}
	if err := s.reserve(sourceDTDStoreIndexBytes); err != nil {
		return err
	}
	s.parametersRead[d] = true
	return nil
}

// completeDefaultWellFormedness records the caller's successful context-correct
// validation of every processable read default, including unused and duplicate
// definitions. XML 5.1 cutoff definitions must not be replacement-processed;
// they retain their own uncertainty. Other closure cells are never discharged.
func (s *sourceDTDStore) completeDefaultWellFormedness() error {
	if err := s.check(); err != nil {
		return err
	}
	if !s.parsed || s.failed || !s.state.GrammarComplete {
		return &validation.ConfigurationError{Field: "DTDStore", Reason: "default verification requires a successful parse"}
	}
	incomplete := false
	for _, d := range s.declarations {
		if err := s.check(); err != nil {
			return err
		}
		if d.Kind == sourceDTDAttributeDeclaration && d.IgnoredReason == sourceDTDIgnoredCutoff && d.HasGeneralReferences {
			incomplete = true
		}
	}
	s.state.DefaultWellFormednessIncomplete = incomplete
	s.refreshWellFormedness()
	return nil
}
func (s *sourceDTDStore) refreshWellFormedness() {
	s.state.WellFormednessIncomplete = s.state.EntityWellFormednessIncomplete || s.state.DefaultWellFormednessIncomplete || s.state.PredefinedEntityWellFormednessIncomplete || s.state.ParameterEntityWellFormednessIncomplete || s.state.ParameterEntityUnread || s.failed || !s.state.GrammarComplete
}

// entityContentRead discharges only the content-production obligation for this
// bound declaration. Attribute-only expansion does not prove content syntax
// (for example, literal ]]> is legal in an attribute), and every later reference
// still passes all context-sensitive checks. Declaration records remain immutable.
func (s *sourceDTDStore) entityContentRead(d *sourceDTDDeclaration) error {
	if err := s.check(); err != nil {
		return err
	}
	if d == nil || !d.RequiresContentCheck || s.entitiesContentRead[d] {
		return nil
	}
	if s.entitiesContentRead == nil {
		if err := s.reserve(sourceDTDStoreMapBytes); err != nil {
			return err
		}
		s.entitiesContentRead = make(map[*sourceDTDDeclaration]bool)
	}
	if err := s.reserve(sourceDTDStoreIndexBytes); err != nil {
		return err
	}
	s.entitiesContentRead[d] = true
	s.entitiesUnchecked--
	s.state.EntityWellFormednessIncomplete = s.entitiesUnchecked != 0
	s.refreshWellFormedness()
	return nil
}

// entityReplacementClosure checks only context-independent closed forms. XML
// 4.6 describes replacement text, after 4.5 character-reference construction:
// lt/amp require a character reference; gt/apos/quot also allow the character.
// An unrecognized redeclaration remains incomplete, not a fabricated WFC error.
func (s *sourceDTDStore) entityReplacementClosure(d sourceDTDDeclaration) (simple, predefined bool, err error) {
	predefined = true
	want := rune(0)
	switch d.Name {
	case "lt":
		want = '<'
	case "amp":
		want = '&'
	case "gt":
		want = '>'
	case "apos":
		want = '\''
	case "quot":
		want = '"'
	}
	value := d.Replacement
	if err = s.account.Charge(validation.BudgetEvents, int64(len(value))); err != nil {
		return
	}
	if want != 0 {
		predefined = !d.External && (want != '<' && want != '&' && value == string(want))
		if !predefined && !d.External {
			decoded, count, valid, e := s.replacementCharacterReference(value)
			if e != nil {
				return false, false, e
			}
			predefined = valid && count == len(value) && decoded == want
		}
		// A conforming predefined binding is interpreted by the scanner's built-in
		// reference path, so it does not carry an ordinary GE content-frame obligation.
		if predefined {
			return true, true, nil
		}
	}
	if d.External {
		return true, predefined, nil
	}
	for i := 0; i < len(value); i++ {
		if err = s.check(); err != nil {
			return false, predefined, err
		}
		switch value[i] {
		case '<':
			return false, predefined, nil
		case ']':
			if strings.HasPrefix(value[i:], "]]>") {
				return false, predefined, nil
			}
		case '&':
			matched := false
			for _, spelling := range []string{"&amp;", "&lt;", "&gt;", "&apos;", "&quot;"} {
				if strings.HasPrefix(value[i:], spelling) {
					i += len(spelling) - 1
					matched = true
					break
				}
			}
			if !matched {
				_, count, valid, e := s.replacementCharacterReference(value[i:])
				if e != nil {
					return false, predefined, e
				}
				if !valid {
					return false, predefined, nil
				}
				i += count - 1
			}
		}
	}
	return true, predefined, nil
}

// The replacement can introduce a new numeric reference not present in the
// EntityValue spelling. Check it without allocating or overflowing, even with
// arbitrarily many leading zeroes. The caller has admitted its inspection work.
func (s *sourceDTDStore) replacementCharacterReference(value string) (rune, int, bool, error) {
	if !strings.HasPrefix(value, "&#") {
		return 0, 0, false, nil
	}
	i, base := 2, uint32(10)
	if i < len(value) && value[i] == 'x' {
		i++
		base = 16
	}
	start, n := i, uint32(0)
	for ; i < len(value); i++ {
		if err := s.check(); err != nil {
			return 0, 0, false, err
		}
		c := value[i]
		if c == ';' {
			return rune(n), i + 1, i > start && sourceXMLChar(rune(n)), nil
		}
		digit := uint32(16)
		switch {
		case c >= '0' && c <= '9':
			digit = uint32(c - '0')
		case base == 16 && c >= 'a' && c <= 'f':
			digit = uint32(c-'a') + 10
		case base == 16 && c >= 'A' && c <= 'F':
			digit = uint32(c-'A') + 10
		}
		if digit >= base || n > (0x10ffff-digit)/base {
			return 0, 0, false, nil
		}
		n = n*base + digit
	}
	return 0, 0, false, nil
}
