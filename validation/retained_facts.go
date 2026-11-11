package validation

import (
	"context"
	"math"
	"strings"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

type retainedLocationMeta struct {
	target            Target
	documentID, entry string
	coordSpace        string
	bindings          []Binding
	xmlBindings       [1]Binding
	posLine, posCol   int
	customPos         bool
}

func (m *retainedLocationMeta) matches(target Target, docID, entry, coordSpace string, bindings []Binding) bool {
	if m == nil || m.customPos || m.target != target || m.documentID != docID || m.entry != entry || m.coordSpace != coordSpace || len(m.bindings) != len(bindings) {
		return false
	}
	for i := range bindings {
		if m.bindings[i] != bindings[i] {
			return false
		}
	}
	return true
}

const (
	locFlagHasPos uint32 = 1 << iota
	locFlagXMLBind
	locCoordShift = 2
	locCoordMask  = 3 << locCoordShift
	locColShift   = 4
)

const (
	locCoordEmpty uint32 = iota << locCoordShift
	locCoordOriginalBytes
	locCoordExternalDTD
	locCoordMeta
)

type retainedLocation struct {
	meta                   *retainedLocationMeta
	pathPrefix, pathSuffix string
	byteOffset             int64
	line                   int32
	columnAndKind          uint32
}

func (v retainedLocation) Path() string {
	if v.pathSuffix == "" {
		return v.pathPrefix
	}
	if v.pathPrefix == "" {
		return v.pathSuffix
	}
	return v.pathPrefix + v.pathSuffix
}

func (v retainedLocation) bindings() []Binding {
	if v.meta == nil {
		return nil
	}
	if v.columnAndKind&locFlagXMLBind != 0 {
		return v.meta.xmlBindings[:]
	}
	return v.meta.bindings
}

func (v retainedLocation) coordSpace() string {
	switch v.columnAndKind & locCoordMask {
	case locCoordEmpty:
		return ""
	case locCoordOriginalBytes:
		return CoordinateSpaceOriginalBytes
	case locCoordExternalDTD:
		return CoordinateSpaceExternalDTD
	default:
		if v.meta != nil {
			return v.meta.coordSpace
		}
		return ""
	}
}

func (v retainedLocation) value() Location {
	var out Location
	if v.meta != nil {
		out.Target = v.meta.target
		out.DocumentID = v.meta.documentID
		out.Entry = v.meta.entry
	}
	out.Path = v.Path()
	if v.columnAndKind&locFlagHasPos != 0 {
		pos := SourcePosition{
			ByteOffset:      v.byteOffset,
			Line:            int(v.line),
			Column:          int(v.columnAndKind >> locColShift),
			CoordinateSpace: v.coordSpace(),
		}
		if v.meta != nil && v.meta.customPos {
			pos.Line = v.meta.posLine
			pos.Column = v.meta.posCol
		}
		out.Position = &pos
	}
	return out
}

func (v retainedLocation) logicalBytes() int64 {
	var docLen, entryLen int
	if v.meta != nil {
		docLen = len(v.meta.documentID)
		entryLen = len(v.meta.entry)
	}
	n := int64(96 + docLen + entryLen + len(v.pathPrefix) + len(v.pathSuffix))
	if v.columnAndKind&locFlagHasPos != 0 {
		n += int64(64 + len(v.coordSpace()))
	}
	return n
}

// These private records borrow only immutable, session-owned evidence. Readers
// may project public values synchronously; exporters must retain their own copy.
type retainedHeader struct {
	ID, Parent  NodeID
	Name        QName
	Type        *Evidence[ComponentID]
	Declaration *Evidence[DeclarationID]
	loc         retainedLocation
}
type retainedScalar struct {
	Lexeme *Evidence[string]
	Value  *Evidence[Atomic]
}
type retainedEnd struct {
	Scalar                               retainedScalar
	ChildrenComplete, AttributesComplete *Evidence[bool]
	Order                                *Evidence[OrderKind]
}
type retainedAttribute struct {
	Name                QName
	Declaration         *Evidence[DeclarationID]
	Presence            *Evidence[bool]
	Scalar              retainedScalar
	loc                 retainedLocation
	lexemeRetainedBytes int32
	atomicRetainedBytes int32
}

func (v retainedHeader) value() NodeHeader {
	var bindings []Binding
	if b := v.loc.bindings(); len(b) > 0 {
		bindings = append([]Binding(nil), b...)
	}
	return NodeHeader{v.ID, v.Parent, v.Name, *v.Type, *v.Declaration, v.loc.value(), bindings}
}
func (v retainedScalar) value() Scalar { return Scalar{*v.Lexeme, *v.Value} }

func (s *Session) readLexeme(v *Evidence[string]) Evidence[string] {
	if s.facts != nil && v == &s.facts.releasedLexeme && s.schema != nil {
		s.schema.typedReleased = true
	}
	return *v
}

func (s *Session) readAtomic(v *Evidence[Atomic]) Evidence[Atomic] {
	if s.facts != nil && v == &s.facts.releasedAtomic && s.schema != nil {
		s.schema.typedReleased = true
	}
	return *v
}
func (v retainedEnd) value(id NodeID) NodeEnd {
	return NodeEnd{id, v.Scalar.value(), *v.ChildrenComplete, *v.AttributesComplete, *v.Order}
}
func (v retainedAttribute) location() Location { return v.loc.value() }
func (v retainedAttribute) value(owner NodeID) AttributeFact {
	return AttributeFact{
		Owner:       owner,
		Name:        v.Name,
		Declaration: *v.Declaration,
		Presence:    *v.Presence,
		Scalar:      v.Scalar.value(),
		Location:    v.loc.value(),
	}
}

const (
	retainedFactNodeBytes              int64 = 256
	retainedFactPoolBytes              int64 = 2048
	retainedComponentBytes             int64 = 128
	retainedDeclarationBytes           int64 = 96
	retainedLexemeBytes                int64 = 64
	retainedAtomicBytes                int64 = 192
	retainedBoolBytes                  int64 = 64
	retainedOrderBytes                 int64 = 64
	retainedNodeMapHeaderBytes         int64 = 0
	retainedSchemaCacheHeaderBytes     int64 = 256
	retainedSchemaTypeSlotBytes        int64 = 64
	retainedSchemaDeclarationSlotBytes int64 = 64
	retainedNodeMapSlotBytes           int64 = 8
	retainedFactPointerBytes           int64 = 8
)

// No process-global mutable canonical records: each pool has one live account.
// Canonical eligibility compares state, value, reason AND cause exactly.
type retainedFactPool struct {
	zeroLexeme                            Evidence[string]
	zeroAtomic                            Evidence[Atomic]
	zeroBool                              Evidence[bool]
	zeroOrder                             Evidence[OrderKind]
	pendingType                           Evidence[ComponentID]
	pendingDeclaration                    Evidence[DeclarationID]
	pendingAtomic                         Evidence[Atomic]
	modelType                             Evidence[ComponentID]
	modelDeclaration                      Evidence[DeclarationID]
	modelLexeme                           Evidence[string]
	absentLexeme                          Evidence[string]
	absentAtomic                          Evidence[Atomic]
	absentBool                            Evidence[bool]
	unavailableAtomic                     *Evidence[Atomic]
	unavailableAttrDeclaration            Evidence[DeclarationID]
	emptyKnownLexeme                      *Evidence[string]
	releasedLexeme                        Evidence[string]
	releasedAtomic                        Evidence[Atomic]
	complete, incomplete                  Evidence[bool]
	sourceOrder, contentOrder, fieldOrder Evidence[OrderKind]
	defaultMeta                           retainedLocationMeta
	lastNoBindMeta, lastBindMeta          *retainedLocationMeta
	attrDeclarations                      map[xsd.CacheIdentity]*Evidence[DeclarationID]
	types                                 map[xsd.CacheIdentity]*Evidence[ComponentID]
	typeCapacity, declarationCapacity     int
	declarations                          map[xsd.CacheIdentity]*Evidence[DeclarationID]
}

func (s *Session) ensureFactPool() error {
	if s.facts != nil {
		return nil
	}
	if err := s.retain(retainedFactPoolBytes); err != nil {
		return err
	}
	s.facts = &retainedFactPool{
		pendingType:                Evidence[ComponentID]{State: EvidenceUnknown, Reason: "schema-binding-pending"},
		pendingDeclaration:         Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "schema-binding-pending"},
		pendingAtomic:              Evidence[Atomic]{State: EvidenceUnknown, Reason: "schema-binding-pending"},
		modelType:                  Evidence[ComponentID]{State: EvidenceUnsupported, Reason: "model-schema-witness-required"},
		modelDeclaration:           Evidence[DeclarationID]{State: EvidenceUnsupported, Reason: "model-schema-witness-required"},
		modelLexeme:                Evidence[string]{State: EvidenceUnsupported, Reason: "model-source-lexeme-unavailable"},
		absentLexeme:               Evidence[string]{State: EvidenceAbsent},
		absentAtomic:               Evidence[Atomic]{State: EvidenceAbsent},
		absentBool:                 Evidence[bool]{State: EvidenceAbsent},
		unavailableAtomic:          &Evidence[Atomic]{State: EvidenceUnsupported, Reason: "typed-scalar-unavailable"},
		unavailableAttrDeclaration: Evidence[DeclarationID]{State: EvidenceUnsupported, Reason: "attribute-use-unavailable"},
		emptyKnownLexeme:           &Evidence[string]{State: EvidenceKnown},
		releasedLexeme:             Evidence[string]{State: EvidenceUnsupported, Reason: "source-lexeme-released"},
		releasedAtomic:             Evidence[Atomic]{State: EvidenceUnsupported, Reason: "schema-typed-scalar-released"},
		complete:                   Evidence[bool]{State: EvidenceKnown, Value: true},
		incomplete:                 Evidence[bool]{State: EvidenceKnown},
		sourceOrder:                Evidence[OrderKind]{State: EvidenceKnown, Value: OrderSource},
		contentOrder:               Evidence[OrderKind]{State: EvidenceKnown, Value: OrderModelContent},
		fieldOrder:                 Evidence[OrderKind]{State: EvidenceKnown, Value: OrderModelFields},
		defaultMeta: retainedLocationMeta{
			target:      s.profile.Target,
			documentID:  s.documentID,
			xmlBindings: [1]Binding{{Prefix: "xml", URI: "http://www.w3.org/XML/1998/namespace"}},
		},
	}
	return nil
}
func (s *Session) zeroEnd() retainedEnd {
	p := s.facts
	return retainedEnd{retainedScalar{&p.zeroLexeme, &p.zeroAtomic}, &p.zeroBool, &p.zeroBool, &p.zeroOrder}
}
func (s *Session) retainType(v Evidence[ComponentID]) (*Evidence[ComponentID], error) {
	if v == s.facts.pendingType {
		return &s.facts.pendingType, nil
	}
	if v == s.facts.modelType {
		return &s.facts.modelType, nil
	}
	n, err := retainedFactSize(retainedComponentBytes, evidenceLogicalBytes(v), int64(len(v.Value.Assembly)), int64(len(v.Value.Kind)), int64(len(v.Value.Name.Space)), int64(len(v.Value.Name.Local)), int64(len(v.Value.Occurrence)))
	if err != nil {
		return nil, err
	}
	if err := s.retain(n); err != nil {
		return nil, err
	}
	owned := cloneEvidence(v)
	owned.Value = cloneComponent(v.Value)
	return &owned, nil
}
func (s *Session) retainDeclaration(v Evidence[DeclarationID]) (*Evidence[DeclarationID], error) {
	if v == s.facts.pendingDeclaration {
		return &s.facts.pendingDeclaration, nil
	}
	if v == s.facts.modelDeclaration {
		return &s.facts.modelDeclaration, nil
	}
	n, err := retainedFactSize(retainedDeclarationBytes, evidenceLogicalBytes(v), int64(len(v.Value.Assembly)), int64(len(v.Value.OwnerOccurrence)), int64(len(v.Value.Occurrence)))
	if err != nil {
		return nil, err
	}
	if err := s.retain(n); err != nil {
		return nil, err
	}
	owned := cloneEvidence(v)
	owned.Value = cloneDeclaration(v.Value)
	return &owned, nil
}
func (s *Session) retainLexeme(v Evidence[string]) (*Evidence[string], error) {
	out, _, err := s.retainLexemeCounted(v)
	return out, err
}
func (s *Session) retainLexemeCounted(v Evidence[string]) (*Evidence[string], int64, error) {
	if v == s.facts.absentLexeme {
		return &s.facts.absentLexeme, 0, nil
	}
	if v == s.facts.modelLexeme {
		return &s.facts.modelLexeme, 0, nil
	}
	n, err := retainedFactSize(retainedLexemeBytes, evidenceLogicalBytes(v), int64(len(v.Value)))
	if err != nil {
		return nil, 0, err
	}
	if err := s.retain(n); err != nil {
		return nil, 0, err
	}
	if s.facts.emptyKnownLexeme != nil && v == *s.facts.emptyKnownLexeme {
		return s.facts.emptyKnownLexeme, n, nil
	}
	owned := cloneEvidence(v)
	owned.Value = strings.Clone(v.Value)
	return &owned, n, nil
}
func (s *Session) retainAtomic(v Evidence[Atomic]) (*Evidence[Atomic], error) {
	out, _, err := s.retainAtomicCounted(v)
	return out, err
}
func (s *Session) retainAtomicCounted(v Evidence[Atomic]) (*Evidence[Atomic], int64, error) {
	if atomicEvidenceIdentical(v, s.facts.pendingAtomic) {
		return &s.facts.pendingAtomic, 0, nil
	}
	if atomicEvidenceIdentical(v, s.facts.absentAtomic) {
		return &s.facts.absentAtomic, 0, nil
	}
	n, err := retainedFactSize(retainedAtomicBytes, evidenceLogicalBytes(v), int64(len(v.Value.Text)), int64(len(v.Value.Name.Space)), int64(len(v.Value.Name.Local)), v.Value.Integer.ByteSize(), v.Value.Decimal.ByteSize())
	if err != nil {
		return nil, 0, err
	}
	if err := s.retain(n); err != nil {
		return nil, 0, err
	}
	owned := cloneEvidence(v)
	owned.Value = cloneAtomic(v.Value)
	return &owned, n, nil
}
func (s *Session) retainBool(v Evidence[bool]) (*Evidence[bool], error) {
	if v == s.facts.complete {
		return &s.facts.complete, nil
	}
	if v == s.facts.incomplete {
		return &s.facts.incomplete, nil
	}
	n, err := retainedFactSize(retainedBoolBytes, evidenceLogicalBytes(v))
	if err != nil {
		return nil, err
	}
	if err := s.retain(n); err != nil {
		return nil, err
	}
	owned := cloneEvidence(v)
	return &owned, nil
}
func (s *Session) retainOrder(v Evidence[OrderKind]) (*Evidence[OrderKind], error) {
	p := s.facts
	if v == p.sourceOrder {
		return &p.sourceOrder, nil
	}
	if v == p.contentOrder {
		return &p.contentOrder, nil
	}
	if v == p.fieldOrder {
		return &p.fieldOrder, nil
	}
	n, err := retainedFactSize(retainedOrderBytes, evidenceLogicalBytes(v), int64(len(v.Value)))
	if err != nil {
		return nil, err
	}
	if err := s.retain(n); err != nil {
		return nil, err
	}
	owned := cloneEvidence(v)
	owned.Value = OrderKind(strings.Clone(string(v.Value)))
	return &owned, nil
}

func (s *Session) buildRetainedLocation(target Target, docID string, v Location, bindings []Binding, anchor string) retainedLocation {
	var out retainedLocation
	entry := s.own(v.Entry)
	var coordSpace string
	var customPos bool
	var coordBits uint32
	if v.Position != nil {
		coordSpace = s.own(v.Position.CoordinateSpace)
		switch coordSpace {
		case "":
			coordBits = locCoordEmpty
		case CoordinateSpaceOriginalBytes:
			coordBits = locCoordOriginalBytes
		case CoordinateSpaceExternalDTD:
			coordBits = locCoordExternalDTD
		default:
			coordBits = locCoordMeta
		}
		out.byteOffset = v.Position.ByteOffset
		if v.Position.Line >= 0 && v.Position.Line <= math.MaxInt32 && v.Position.Column >= 0 && v.Position.Column <= (1<<27)-1 {
			out.line = int32(v.Position.Line)
			out.columnAndKind = (uint32(v.Position.Column) << locColShift) | coordBits | locFlagHasPos
		} else {
			out.columnAndKind = coordBits | locFlagHasPos
			customPos = true
		}
	}
	isXMLBind := s.facts != nil && len(bindings) == 1 && bindings[0] == s.facts.defaultMeta.xmlBindings[0]
	if isXMLBind {
		out.columnAndKind |= locFlagXMLBind
		bindings = nil
	}
	if s.facts != nil && target == s.profile.Target && docID == s.documentID && entry == "" && coordBits != locCoordMeta && len(bindings) == 0 && !customPos {
		out.meta = &s.facts.defaultMeta
	}
	var metaCoord string
	if coordBits == locCoordMeta {
		metaCoord = coordSpace
	}
	if out.meta == nil && s.facts != nil && !customPos {
		if len(bindings) == 0 && s.facts.lastNoBindMeta.matches(target, docID, entry, metaCoord, nil) {
			out.meta = s.facts.lastNoBindMeta
		} else if len(bindings) > 0 && s.facts.lastBindMeta.matches(target, docID, entry, metaCoord, bindings) {
			out.meta = s.facts.lastBindMeta
		}
	}
	if out.meta == nil {
		meta := &retainedLocationMeta{
			target:     target,
			documentID: docID,
			entry:      entry,
			coordSpace: metaCoord,
			bindings:   bindings,
			customPos:  customPos,
		}
		if s.facts != nil {
			meta.xmlBindings = s.facts.defaultMeta.xmlBindings
		}
		if v.Position != nil && customPos {
			meta.posLine = v.Position.Line
			meta.posCol = v.Position.Column
		}
		out.meta = meta
		if s.facts != nil && !customPos {
			if len(bindings) == 0 {
				s.facts.lastNoBindMeta = meta
			} else {
				s.facts.lastBindMeta = meta
			}
		}
	}
	if anchor != "" && len(v.Path) > len(anchor) && v.Path[len(anchor)] == '/' && strings.HasPrefix(v.Path, anchor) {
		out.pathPrefix = anchor
		suffix := v.Path[len(anchor):]
		if len(suffix) <= 128 {
			out.pathSuffix = s.own(suffix)
		} else {
			out.pathSuffix = strings.Clone(suffix)
		}
	} else {
		out.pathPrefix = strings.Clone(v.Path)
	}
	return out
}

func (s *Session) retainFactHeader(ctx context.Context, v NodeHeader) (retainedHeader, error) {
	var out retainedHeader
	if err := s.ensureFactPool(); err != nil {
		return out, err
	}
	// Includes all inline node/header/location/end/slice fields exactly once.
	n, err := retainedFactSize(retainedFactNodeBytes, int64(len(v.Name.Space)), int64(len(v.Name.Local)), int64(len(v.Location.Entry)), int64(len(v.Location.Path)))
	if err != nil {
		return out, err
	}
	if v.Location.Position != nil {
		n, err = retainedFactSize(n, 64, int64(len(v.Location.Position.CoordinateSpace)))
		if err != nil {
			return out, err
		}
	}
	for _, b := range v.Bindings {
		if err := s.step(ctx, 1); err != nil {
			return out, err
		}
		n, err = retainedFactSize(n, 64, int64(len(b.Prefix)), int64(len(b.URI)))
		if err != nil {
			return out, err
		}
	}
	if err := s.retain(n); err != nil {
		return out, err
	}
	out.Type, err = s.retainType(v.Type)
	if err != nil {
		return out, err
	}
	out.Declaration, err = s.retainDeclaration(v.Declaration)
	if err != nil {
		return out, err
	}
	out.ID, out.Parent = v.ID, v.Parent
	out.Name = QName{s.own(v.Name.Space), s.own(v.Name.Local)}
	var bindings []Binding
	if len(v.Bindings) == 1 && v.Bindings[0] == s.facts.defaultMeta.xmlBindings[0] {
		if err := s.step(ctx, 1); err != nil {
			return out, err
		}
		_ = s.own(v.Bindings[0].Prefix)
		_ = s.own(v.Bindings[0].URI)
		bindings = s.facts.defaultMeta.xmlBindings[:]
	} else if len(v.Bindings) > 0 {
		bindings = make([]Binding, len(v.Bindings))
		for i, b := range v.Bindings {
			if err := s.step(ctx, 1); err != nil {
				return out, err
			}
			bindings[i] = Binding{s.own(b.Prefix), s.own(b.URI)}
		}
	}
	var anchor string
	if len(s.stack) >= 3 {
		anchor = s.stack[2].header.loc.pathPrefix
	}
	out.loc = s.buildRetainedLocation(s.profile.Target, s.documentID, v.Location, bindings, anchor)
	return out, nil
}
func (s *Session) retainFactEnd(v NodeEnd) (retainedEnd, error) {
	out, _, _, err := s.retainFactEndCounted(v)
	return out, err
}
func (s *Session) retainFactEndCounted(v NodeEnd) (retainedEnd, int64, int64, error) {
	var out retainedEnd
	var lexBytes, atomicBytes int64
	var err error
	out.Scalar.Lexeme, lexBytes, err = s.retainLexemeCounted(v.Scalar.Lexeme)
	if err != nil {
		return out, 0, 0, err
	}
	out.Scalar.Value, atomicBytes, err = s.retainAtomicCounted(v.Scalar.Value)
	if err != nil {
		return out, 0, 0, err
	}
	out.ChildrenComplete, err = s.retainBool(v.ChildrenComplete)
	if err != nil {
		return out, 0, 0, err
	}
	out.AttributesComplete, err = s.retainBool(v.AttributesComplete)
	if err != nil {
		return out, 0, 0, err
	}
	out.Order, err = s.retainOrder(v.Order)
	return out, lexBytes, atomicBytes, err
}

// Schema-published identities borrow immutable compiled payloads. The exact
// compiled owner/ordinal token contains no strings: lookup/insertion and each
// growth-copy consume one admitted work unit. Unknown/causal evidence and
// zero/foreign tokens never take this cache path.
type retainedSchemaIdentity interface{ ComponentID | DeclarationID }

// retainedSchemaCacheBytes is the preallocation guard for the complete new
// hinted map. Keep multiplication, header addition and host-width admission on
// the same production path, before converting capacity to an allocation hint.
func retainedSchemaCacheBytes(capacity, slotBytes int64) (int64, error) {
	bytes, err := rationalCheckedMul(BudgetBufferedBytes, capacity, slotBytes)
	if err != nil {
		return 0, err
	}
	return retainedFactSize(retainedSchemaCacheHeaderBytes, bytes)
}

func growRetainedSchemaCache[K retainedSchemaIdentity](ctx context.Context, s *Session, old map[xsd.CacheIdentity]*Evidence[K], capacity int, slotBytes int64) (map[xsd.CacheIdentity]*Evidence[K], int, error) {
	if len(old) < capacity {
		return old, capacity, nil
	}
	next, err := rationalCheckedMul(BudgetBufferedBytes, int64(capacity), 2)
	if err != nil {
		return old, capacity, err
	}
	next = max(8, next)
	bytes, err := retainedSchemaCacheBytes(next, slotBytes)
	if err != nil {
		return old, capacity, err
	}
	if err = s.retain(bytes); err != nil {
		return old, capacity, err
	}
	if err = s.step(ctx, 1); err != nil {
		s.releaseFactBytes(bytes)
		return old, capacity, err
	}
	out := make(map[xsd.CacheIdentity]*Evidence[K], int(next))
	for key, value := range old {
		err := s.step(ctx, 1)
		if err != nil {
			out = nil
			s.releaseFactBytes(bytes)
			return old, capacity, err
		}
		out[key] = value
	}
	if capacity != 0 {
		s.releaseFactBytes(retainedSchemaCacheHeaderBytes + int64(capacity)*slotBytes)
	}
	return out, int(next), nil
}

func (s *Session) retainSchemaType(ctx context.Context, key xsd.CacheIdentity, v Evidence[ComponentID]) (*Evidence[ComponentID], error) {
	if v.State != EvidenceKnown || v.Reason != "" || v.Cause != nil || s.schema == nil || s.schema.active == nil || !key.BelongsTo(s.schema.active.kernel) {
		return s.retainType(v)
	}
	var err error
	if s.facts.types != nil {
		if err = s.step(ctx, 1); err != nil {
			return nil, err
		}
		if p := s.facts.types[key]; p != nil {
			return p, nil
		}
	}
	if err = s.retain(retainedComponentBytes); err != nil {
		return nil, err
	}
	s.facts.types, s.facts.typeCapacity, err = growRetainedSchemaCache(ctx, s, s.facts.types, s.facts.typeCapacity, retainedSchemaTypeSlotBytes)
	if err != nil {
		return nil, err
	}
	if err = s.step(ctx, 1); err != nil {
		return nil, err
	}
	owned := v
	s.facts.types[key] = &owned
	return &owned, nil
}
func (s *Session) retainSchemaDeclaration(ctx context.Context, key xsd.CacheIdentity, v Evidence[DeclarationID]) (*Evidence[DeclarationID], error) {
	if v.State != EvidenceKnown || v.Reason != "" || v.Cause != nil || s.schema == nil || s.schema.active == nil || !key.BelongsTo(s.schema.active.kernel) {
		return s.retainDeclaration(v)
	}
	var err error
	if s.facts.declarations != nil {
		if err = s.step(ctx, 1); err != nil {
			return nil, err
		}
		if p := s.facts.declarations[key]; p != nil {
			return p, nil
		}
	}
	if err = s.retain(retainedDeclarationBytes); err != nil {
		return nil, err
	}
	s.facts.declarations, s.facts.declarationCapacity, err = growRetainedSchemaCache(ctx, s, s.facts.declarations, s.facts.declarationCapacity, retainedSchemaDeclarationSlotBytes)
	if err != nil {
		return nil, err
	}
	if err = s.step(ctx, 1); err != nil {
		return nil, err
	}
	owned := v
	s.facts.declarations[key] = &owned
	return &owned, nil
}

// Slice leases cover the complete old/new capacity overlap. The only append
// calls at ingress occur after these helpers have made room, so no hidden
// backing allocation remains covered by the old heavyweight header envelope.
type retainedFactSlot interface{ NodeID | *factNode }

func growFactSlice[T retainedFactSlot](ctx context.Context, s *Session, v []T) ([]T, error) {
	if len(v) < cap(v) {
		return v, nil
	}
	capacity, err := rationalCheckedMul(BudgetBufferedBytes, int64(cap(v)), 2)
	if err != nil {
		return v, err
	}
	capacity = max(1, capacity)
	bytes, err := rationalCheckedMul(BudgetBufferedBytes, capacity, retainedFactPointerBytes)
	if err != nil {
		return v, err
	}
	if _, err = allocationSize(bytes); err != nil {
		return v, err
	}
	if err = s.retain(bytes); err != nil {
		return v, err
	}
	if err = s.step(ctx, int64(len(v))+1); err != nil {
		s.releaseFactBytes(bytes)
		return v, err
	}
	out := make([]T, len(v), int(capacity))
	copy(out, v)
	s.releaseFactBytes(int64(cap(v)) * retainedFactPointerBytes)
	return out, nil
}
func (s *Session) releaseFactBytes(n int64) {
	if n != 0 {
		_ = s.account.Release(BudgetBufferedBytes, n)
		s.retained -= n
	}
}
func (s *Session) growFactNodes(ctx context.Context) error {
	count := s.totalNodes()
	if count < s.nodeCapacity {
		return nil
	}
	capacity, err := rationalCheckedMul(BudgetBufferedBytes, int64(s.nodeCapacity), 2)
	if err != nil {
		return err
	}
	capacity = max(8, capacity)
	bytes, err := rationalCheckedMul(BudgetBufferedBytes, capacity, retainedNodeMapSlotBytes)
	if err != nil {
		return err
	}
	bytes, err = rationalCheckedAdd(BudgetBufferedBytes, bytes, retainedNodeMapHeaderBytes)
	if err != nil {
		return err
	}
	if _, err = allocationSize(bytes); err != nil {
		return err
	}
	if err = s.retain(bytes); err != nil {
		return err
	}
	if err = s.step(ctx, int64(count)+1); err != nil {
		s.releaseFactBytes(bytes)
		return err
	}
	out := make([]*factNode, int(capacity))
	copy(out, s.denseNodes)
	if s.nodeCapacity != 0 {
		s.releaseFactBytes(retainedNodeMapHeaderBytes + int64(s.nodeCapacity)*retainedNodeMapSlotBytes)
	}
	s.denseNodes = out
	s.nodeCapacity = int(capacity)
	for id, n := range s.nodes {
		if id != 0 && int(id) <= len(s.denseNodes) && s.denseNodes[id-1] == nil {
			s.denseNodes[id-1] = n
			s.nodeCount++
			delete(s.nodes, id)
		}
	}
	return nil
}

func (s *Session) allocFactNode(header retainedHeader, end retainedEnd) *factNode {
	return &factNode{header: header, end: end}
}

func (s *Session) appendRetainedAttribute(n *factNode, attr retainedAttribute) {
	if s.attrOwner == n && len(s.attrChunk) > 0 && len(n.attributes) > 0 {
		s.attrChunk[0] = attr
		s.attrChunk = s.attrChunk[1:]
		count := len(n.attributes) + 1
		n.attributes = s.attrOwnerBase[:count:count]
		s.attrCount++
		return
	}
	if len(n.attributes) == 0 {
		if len(s.attrChunk) == 0 || (len(s.attrChunk) < 4 && s.attrCount >= 4) {
			size := min(8, max(1, s.attrCount))
			s.attrChunk = make([]retainedAttribute, size)
		}
		s.attrChunk[0] = attr
		s.attrOwner = n
		s.attrOwnerBase = s.attrChunk
		s.attrChunk = s.attrChunk[1:]
		n.attributes = s.attrOwnerBase[:1:1]
		s.attrCount++
		return
	}
	count := len(n.attributes) + 1
	s.attrChunk = make([]retainedAttribute, max(8, count+3))
	copy(s.attrChunk, n.attributes)
	s.attrChunk[len(n.attributes)] = attr
	s.attrOwner = n
	s.attrOwnerBase = s.attrChunk
	s.attrChunk = s.attrChunk[count:]
	n.attributes = s.attrOwnerBase[:count:count]
	s.attrCount++
}

// Internal producers already own the immutable payload and cause. Reserve the
// new record only, without cloning or charging that payload a second time.
func (s *Session) retainInternalAtomic(v Evidence[Atomic]) (*Evidence[Atomic], error) {
	if err := s.retain(retainedAtomicBytes); err != nil {
		return nil, err
	}
	owned := v
	return &owned, nil
}
func (s *Session) retainSchemaAtomic(v Evidence[Atomic]) (*Evidence[Atomic], error) {
	n := retainedAtomicBytes
	// schemaAtomic already admits the 128-byte Atomic body for known values.
	if v.State == EvidenceKnown {
		n -= 128
	}
	if err := s.retain(n); err != nil {
		return nil, err
	}
	if s.facts != nil && s.facts.unavailableAtomic != nil && atomicEvidenceIdentical(v, *s.facts.unavailableAtomic) {
		return s.facts.unavailableAtomic, nil
	}
	owned := v
	return &owned, nil
}

// Sizes are checked after converting each individual length to the account's
// int64 domain. No native-int aggregate is formed.
func retainedFactSize(parts ...int64) (int64, error) {
	var n int64
	for _, part := range parts {
		var err error
		n, err = rationalCheckedAdd(BudgetBufferedBytes, n, part)
		if err != nil {
			return 0, err
		}
	}
	if _, err := allocationSize(n); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Session) validateRetainedHeader(v NodeHeader) error {
	// validateHeader owns a temporary duplicate-binding map before retained facts
	// exist. Its fixed first group and conservative per-binding slots are live
	// only during this call and cannot borrow a future node's lease.
	slots, err := rationalCheckedMul(BudgetBufferedBytes, int64(len(v.Bindings)), 96)
	if err != nil {
		return err
	}
	size, err := retainedFactSize(256, slots)
	if err != nil {
		return err
	}
	if err = s.account.Charge(BudgetBufferedBytes, size); err != nil {
		return err
	}
	defer s.account.Release(BudgetBufferedBytes, size)
	return validateHeader(v, s.profile.Target, s.documentID)
}
