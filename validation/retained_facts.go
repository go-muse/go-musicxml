package validation

import (
	"context"
	"strings"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

// These private records borrow only immutable, session-owned evidence. Readers
// may project public values synchronously; exporters must retain their own copy.
type retainedHeader struct {
	ID, Parent  NodeID
	Name        QName
	Type        *Evidence[ComponentID]
	Declaration *Evidence[DeclarationID]
	Location    Location
	Bindings    []Binding
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

func (v retainedHeader) value() NodeHeader {
	return NodeHeader{v.ID, v.Parent, v.Name, *v.Type, *v.Declaration, v.Location, v.Bindings}
}
func (v retainedScalar) value() Scalar { return Scalar{*v.Lexeme, *v.Value} }
func (v retainedEnd) value(id NodeID) NodeEnd {
	return NodeEnd{id, v.Scalar.value(), *v.ChildrenComplete, *v.AttributesComplete, *v.Order}
}

const (
	retainedFactNodeBytes              int64 = 320
	retainedFactPoolBytes              int64 = 2048
	retainedComponentBytes             int64 = 128
	retainedDeclarationBytes           int64 = 96
	retainedLexemeBytes                int64 = 64
	retainedAtomicBytes                int64 = 192
	retainedBoolBytes                  int64 = 64
	retainedOrderBytes                 int64 = 64
	retainedNodeMapHeaderBytes         int64 = 256
	retainedSchemaCacheHeaderBytes     int64 = 256
	retainedSchemaTypeSlotBytes        int64 = 64
	retainedSchemaDeclarationSlotBytes int64 = 64
	retainedNodeMapSlotBytes           int64 = 48
	retainedFactPointerBytes           int64 = 8
)

// No process-global mutable canonical records: each pool has one live account.
// Canonical eligibility compares state, value, reason AND cause exactly.
type retainedFactPool struct {
	zeroType                              Evidence[ComponentID]
	zeroDeclaration                       Evidence[DeclarationID]
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
	complete, incomplete                  Evidence[bool]
	sourceOrder, contentOrder, fieldOrder Evidence[OrderKind]
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
		pendingType:        Evidence[ComponentID]{State: EvidenceUnknown, Reason: "schema-binding-pending"},
		pendingDeclaration: Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "schema-binding-pending"},
		pendingAtomic:      Evidence[Atomic]{State: EvidenceUnknown, Reason: "schema-binding-pending"},
		modelType:          Evidence[ComponentID]{State: EvidenceUnsupported, Reason: "model-schema-witness-required"},
		modelDeclaration:   Evidence[DeclarationID]{State: EvidenceUnsupported, Reason: "model-schema-witness-required"},
		modelLexeme:        Evidence[string]{State: EvidenceUnsupported, Reason: "model-source-lexeme-unavailable"},
		absentLexeme:       Evidence[string]{State: EvidenceAbsent}, absentAtomic: Evidence[Atomic]{State: EvidenceAbsent},
		complete: Evidence[bool]{State: EvidenceKnown, Value: true}, incomplete: Evidence[bool]{State: EvidenceKnown},
		sourceOrder:  Evidence[OrderKind]{State: EvidenceKnown, Value: OrderSource},
		contentOrder: Evidence[OrderKind]{State: EvidenceKnown, Value: OrderModelContent},
		fieldOrder:   Evidence[OrderKind]{State: EvidenceKnown, Value: OrderModelFields},
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
	if v == s.facts.absentLexeme {
		return &s.facts.absentLexeme, nil
	}
	if v == s.facts.modelLexeme {
		return &s.facts.modelLexeme, nil
	}
	n, err := retainedFactSize(retainedLexemeBytes, evidenceLogicalBytes(v), int64(len(v.Value)))
	if err != nil {
		return nil, err
	}
	if err := s.retain(n); err != nil {
		return nil, err
	}
	owned := cloneEvidence(v)
	owned.Value = strings.Clone(v.Value)
	return &owned, nil
}
func (s *Session) retainAtomic(v Evidence[Atomic]) (*Evidence[Atomic], error) {
	if v == s.facts.pendingAtomic {
		return &s.facts.pendingAtomic, nil
	}
	if v == s.facts.absentAtomic {
		return &s.facts.absentAtomic, nil
	}
	n, err := retainedFactSize(retainedAtomicBytes, evidenceLogicalBytes(v), int64(len(v.Value.Text)), int64(len(v.Value.Name.Space)), int64(len(v.Value.Name.Local)), v.Value.Integer.ByteSize(), v.Value.Decimal.ByteSize())
	if err != nil {
		return nil, err
	}
	if err := s.retain(n); err != nil {
		return nil, err
	}
	owned := cloneEvidence(v)
	owned.Value = cloneAtomic(v.Value)
	return &owned, nil
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
	out.Name = QName{strings.Clone(v.Name.Space), strings.Clone(v.Name.Local)}
	out.Location = v.Location
	out.Location.Target = s.profile.Target
	out.Location.DocumentID = s.documentID
	out.Location.Entry = strings.Clone(v.Location.Entry)
	out.Location.Path = strings.Clone(v.Location.Path)
	if v.Location.Position != nil {
		p := *v.Location.Position
		p.CoordinateSpace = strings.Clone(p.CoordinateSpace)
		out.Location.Position = &p
	}
	out.Bindings = make([]Binding, len(v.Bindings))
	for i, b := range v.Bindings {
		if err := s.step(ctx, 1); err != nil {
			return out, err
		}
		out.Bindings[i] = Binding{strings.Clone(b.Prefix), strings.Clone(b.URI)}
	}
	return out, nil
}
func (s *Session) retainFactEnd(v NodeEnd) (retainedEnd, error) {
	var out retainedEnd
	var err error
	out.Scalar.Lexeme, err = s.retainLexeme(v.Scalar.Lexeme)
	if err != nil {
		return out, err
	}
	out.Scalar.Value, err = s.retainAtomic(v.Scalar.Value)
	if err != nil {
		return out, err
	}
	out.ChildrenComplete, err = s.retainBool(v.ChildrenComplete)
	if err != nil {
		return out, err
	}
	out.AttributesComplete, err = s.retainBool(v.AttributesComplete)
	if err != nil {
		return out, err
	}
	out.Order, err = s.retainOrder(v.Order)
	return out, err
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
	if _, err = AllocationSize(bytes); err != nil {
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
	if len(s.nodes) < s.nodeCapacity {
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
	if _, err = AllocationSize(bytes); err != nil {
		return err
	}
	if err = s.retain(bytes); err != nil {
		return err
	}
	if err = s.step(ctx, int64(len(s.nodes))+1); err != nil {
		s.releaseFactBytes(bytes)
		return err
	}
	out := make(map[NodeID]*factNode, int(capacity))
	for id, n := range s.nodes {
		out[id] = n
	}
	if s.nodeCapacity != 0 {
		s.releaseFactBytes(retainedNodeMapHeaderBytes + int64(s.nodeCapacity)*retainedNodeMapSlotBytes)
	}
	s.nodes = out
	s.nodeCapacity = int(capacity)
	return nil
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
	owned := v
	return &owned, nil
}

// Sizes are checked after converting each individual length to the account's
// int64 domain, including on 32-bit hosts. No native-int aggregate is formed.
func retainedFactSize(parts ...int64) (int64, error) {
	var n int64
	for _, part := range parts {
		var err error
		n, err = rationalCheckedAdd(BudgetBufferedBytes, n, part)
		if err != nil {
			return 0, err
		}
	}
	if _, err := AllocationSize(n); err != nil {
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
