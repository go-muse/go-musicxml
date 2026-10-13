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

const sourceDTDInternalSubset sourceDTDRegion = "internal-subset"

type sourceDTDOrigin struct {
	Sequence       int64
	Region         sourceDTDRegion
	Span           sourceXMLSpan
	SpanAvailable  bool
	ExternalMarkup bool
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
	context        context.Context
	account        *validation.Account
	state          sourceDTDStoreState
	declarations   []*sourceDTDDeclaration
	bindings       map[sourceDTDStoreKey]*sourceDTDDeclaration
	attributes     map[string][]*sourceDTDDeclaration
	owned          int64
	parsed, closed bool
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
	if _, err := validation.AllocationSize(n); err != nil {
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
	p.doctype()
	grammarComplete := p.err == nil
	if p.err == nil {
		p.checkDefaultReferences()
	}
	store.state.Name = p.info.Name
	store.state.GrammarComplete = grammarComplete
	store.state.WellFormednessIncomplete = store.state.EntityWellFormednessIncomplete || store.state.DefaultWellFormednessIncomplete || store.state.PredefinedEntityWellFormednessIncomplete || store.state.ParameterEntityWellFormednessIncomplete || store.state.ParameterEntityUnread || p.err != nil
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
	count, err := validation.AllocationSize(n)
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
	// Residual proof obligations stay separate from declaration-domain closure.
	// In particular a literal GE is closed without pretending a lexical graph
	// proves arbitrary unused replacement/reference contexts free of recursion.
	switch d.Kind {
	case sourceDTDGeneralEntityDeclaration:
		if !d.External && strings.ContainsRune(d.Replacement, '&') {
			s.state.EntityWellFormednessIncomplete = true
		}
		switch d.Name {
		case "amp", "lt", "gt", "apos", "quot":
			s.state.PredefinedEntityWellFormednessIncomplete = true
		}
	case sourceDTDParameterEntityDeclaration:
		s.state.ParameterEntityWellFormednessIncomplete = true
	case sourceDTDAttributeDeclaration:
		if d.HasGeneralReferences {
			s.state.DefaultWellFormednessIncomplete = true
		}
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
		count, err := validation.AllocationSize(capacity)
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
