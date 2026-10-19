package validation

import (
	"context"
	"reflect"
	"strings"
)

// ResourceSnapshotHeader identifies an intrinsic assessment without confusing
// occurrence identity, schema assembly, root kind and assessment completeness.
type ResourceSnapshotHeader struct {
	// LinkProjectionSelected records selection of an available sealed local-link
	// plan for the established assembly. It does not assert projection or scope
	// completion.
	LinkProjectionSelected bool
	DocumentID             string
	Root                   NodeRef
	Name                   Evidence[QName]
	Assembly               Evidence[Assembly]
	Profile                ProfileID
	CatalogDigest          string
	Target                 Target
	InputComplete          bool
	Syntax                 SyntaxState
	ScopeComplete          Evidence[bool]
	IDScopeComplete        Evidence[bool]
}

// ResourceIDFact is one schema-determined normalized ID and its exact owner.
// Unique remains unavailable in an open identity scope; ambiguity retains its
// causal identity decision. Part is an exact score-part owner when established.
type ResourceIDFact struct {
	Value       string
	Declaration DeclarationID
	Subject     NodeRef
	Source      NodeRef
	Part        Evidence[NodeRef]
	Kind        Evidence[QName]
	Unique      Evidence[bool]
	Location    Location
}

type resourceLeaseOwner struct{ marker byte }

// ResourceSnapshot is a core-created immutable intrinsic result with a live
// retention owner. It is single-goroutine: callers must serialize reads, transfer
// and Close. Getters return caller-owned copies, never mutable backing storage.
// After Close, header/count getters are zero and content getters reject access.
type ResourceSnapshot struct {
	// Private cache identity is bound to the admitted session, not an editable report.
	dictionary    [32]byte
	fragments     *resourceFragmentSnapshot
	fragmentOrder map[NodeRef]uint64
	header        ResourceSnapshotHeader
	report        Report
	contexts      []ContextResult
	ids           []ResourceIDFact
	account       *Account
	retained      int64
	closed        bool
	owner         *resourceLeaseOwner
}

// FinishWithResourceSnapshot is an alternate terminal finalizer. It evaluates
// all intrinsic phases before admitting and copying the selected owned facts,
// then cleans the session exactly once. Selected source-origin contexts retain
// their component availability and intrinsic provider outcome; export does not
// establish source-origin completeness. Snapshot admission failure preserves
// intrinsic findings but marks the returned Report incomplete/terminal; without
// a known violation its conformance becomes undetermined. A nonnil snapshot must
// be closed or transferred
// to a package session sharing the exact same Account.
//
// Adapter API: the root musicxml package uses this to stream facts into an
// engine; application code is not expected to call it.
func (s *Session) FinishWithResourceSnapshot(ctx context.Context, end Completion) (Report, *ResourceSnapshot, error) {
	return s.finishResource(ctx, end, true)
}

func (s *Session) makeResourceSnapshot(ctx context.Context, report Report) (*ResourceSnapshot, error) {
	if err := s.step(ctx, 1); err != nil {
		return nil, err
	}
	h := ResourceSnapshotHeader{DocumentID: s.documentID, Root: NodeRef{DocumentID: s.documentID}, Profile: s.profile.ID, CatalogDigest: s.profile.CatalogDigest, Target: s.profile.Target, InputComplete: s.inputComplete, Syntax: report.Syntax, Name: Evidence[QName]{State: EvidenceUnknown, Reason: "resource-root-unavailable"}, Assembly: Evidence[Assembly]{State: EvidenceUnknown, Reason: "resource-assembly-unavailable"}, ScopeComplete: Evidence[bool]{State: EvidenceUnknown, Reason: "resource-scope-incomplete"}}
	if s.rejectedRootName.Local != "" {
		h.Name = Evidence[QName]{State: EvidenceKnown, Value: s.rejectedRootName}
	}
	if s.root != nil {
		h.Root.Node = s.root.header.ID
		h.Name = Evidence[QName]{State: EvidenceKnown, Value: s.root.header.Name}
		if s.schema != nil && s.schema.active != nil {
			h.Assembly = Evidence[Assembly]{State: EvidenceKnown, Value: s.schema.active.assembly}
		}
	}
	if h.Assembly.State == EvidenceKnown {
		for _, key := range s.catalog.orders[s.profile.ID] {
			if err := s.step(ctx, 1); err != nil {
				return nil, err
			}
			// Match evaluate's support gate: an unsupported or missing entry
			// returns before running the plan and cannot witness link absence.
			if s.catalog.supports[s.profile.ID][key].State != ImplementationAvailable {
				continue
			}
			kind := s.catalog.plans[s.catalog.rules[key].Predicate].kind
			if (kind == planContextMusicXMLLinks && h.Assembly.Value == AssemblyMusicXML) || (kind == planContextOpusLinks && h.Assembly.Value == AssemblyOpus) {
				h.LinkProjectionSelected = true
				break
			}
		}
	}
	if s.inputComplete && !s.enumerationIncomplete && s.terminalCause == nil {
		h.ScopeComplete = Evidence[bool]{State: EvidenceKnown, Value: true}
	}
	h.IDScopeComplete = Evidence[bool]{State: EvidenceUnknown, Reason: "resource-id-scope-incomplete"}
	if s.schema != nil && s.schema.identityReady && s.schema.identity.Complete {
		h.IDScopeComplete = Evidence[bool]{State: EvidenceKnown, Value: true}
	}
	fragments, err := s.resourceFragments(ctx)
	if err != nil {
		return nil, err
	}
	ids, err := s.resourceIDs(ctx)
	if err != nil {
		return nil, err
	}
	count := 0
	for _, v := range report.Context {
		if resourceSelectedContext(v.Property) {
			count++
		}
	}
	// Account the private canonical report, selected fact copies, and their
	// exact-length backing arrays while session/output representations coexist.
	size, err := resourceMeasure(ctx, s.account, reflect.ValueOf(report))
	if err != nil {
		return nil, err
	}
	for _, v := range []any{h, ids} {
		n, e := resourceMeasure(ctx, s.account, reflect.ValueOf(v))
		if e != nil {
			return nil, e
		}
		size, e = rationalCheckedAdd(BudgetBufferedBytes, size, n)
		if e != nil {
			return nil, e
		}
	}
	if fragments != nil {
		n, e := resourceMeasure(ctx, s.account, reflect.ValueOf(fragments))
		if e != nil {
			return nil, e
		}
		size, e = rationalCheckedAdd(BudgetBufferedBytes, size, n)
		if e != nil {
			return nil, e
		}
		indexBytes, e := rationalCheckedMul(BudgetBufferedBytes, int64(len(fragments.Elements)), resourceFragmentOrderEntryBytes)
		if e != nil {
			return nil, e
		}
		indexBytes, e = rationalCheckedAdd(BudgetBufferedBytes, indexBytes, resourceFragmentOrderHeaderBytes)
		if e != nil {
			return nil, e
		}
		size, e = rationalCheckedAdd(BudgetBufferedBytes, size, indexBytes)
		if e != nil {
			return nil, e
		}
	}
	n, e := rationalCheckedMul(BudgetBufferedBytes, int64(count), 256)
	if e != nil {
		return nil, e
	}
	size, e = rationalCheckedAdd(BudgetBufferedBytes, size, n+512)
	if e != nil {
		return nil, e
	}
	for _, v := range report.Context {
		if !resourceSelectedContext(v.Property) {
			continue
		}
		n, e := resourceMeasure(ctx, s.account, reflect.ValueOf(v))
		if e != nil {
			return nil, e
		}
		size, e = rationalCheckedAdd(BudgetBufferedBytes, size, n)
		if e != nil {
			return nil, e
		}
	}
	if _, err = allocationSize(size); err != nil {
		return nil, err
	}
	if err = s.account.Charge(BudgetBufferedBytes, size); err != nil {
		return nil, err
	}
	// The single admitted lease covers the canonical report and the separately
	// owned selected contexts while the session/report input still exists. Keep
	// it live throughout copying and release it on every interrupted copy.
	out := &ResourceSnapshot{dictionary: s.dictionaryFingerprint(), account: s.account, retained: size}
	committed := false
	defer func() {
		if !committed {
			out.release()
		}
	}()
	if err = resourceCopyInto(ctx, reflect.ValueOf(&out.header).Elem(), reflect.ValueOf(h), "", ""); err != nil {
		return nil, err
	}
	if err = resourceCopyInto(ctx, reflect.ValueOf(&out.report).Elem(), reflect.ValueOf(report), "", ""); err != nil {
		return nil, err
	}
	if err = resourceCopyInto(ctx, reflect.ValueOf(&out.ids).Elem(), reflect.ValueOf(ids), "", ""); err != nil {
		return nil, err
	}
	if fragments != nil {
		if err = resourceCopyInto(ctx, reflect.ValueOf(&out.fragments).Elem(), reflect.ValueOf(fragments), "", ""); err != nil {
			return nil, err
		}
		if err = out.indexFragmentOrder(ctx); err != nil {
			return nil, err
		}
	}
	out.contexts = make([]ContextResult, count)
	at := 0
	for _, v := range report.Context {
		if resourceSelectedContext(v.Property) {
			if err = resourceCopyInto(ctx, reflect.ValueOf(&out.contexts[at]).Elem(), reflect.ValueOf(v), "", ""); err != nil {
				return nil, err
			}
			at++
		}
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	committed = true
	return out, nil
}

func resourceSelectedContext(p ContextProperty) bool {
	if p == ContextPropertySourceOrigin || dictionaryPropertyValid(p) {
		return true
	}
	switch string(p) {
	case "container.metadata", "resource.link", "resource.image", "resource.schema-hints", "resource.group", "resource.instrument", "sounds.mapping", "layout.opus-version", "layout.opus-new-page":
		return true
	}
	return false
}

// Numeric wrappers are immutable and cache their retained-byte size. Type
// identity is checked before reading that private field, avoiding Interface
// boxing for every traversed result field. Cache field indexes once as well.
var resourceNumericFields = func() map[reflect.Type]int {
	fields := make(map[reflect.Type]int, 3)
	for _, typ := range []reflect.Type{reflect.TypeFor[Integer](), reflect.TypeFor[Decimal](), reflect.TypeFor[Rational]()} {
		field, _ := typ.FieldByName("bytes")
		fields[typ] = field.Index[0]
	}
	return fields
}()

const resourceCopyChunk = 4096

// resourceMeasure is limited to closed library-defined transport/result values.
// It is not a public reflection or predicate hook. Numeric wrappers are immutable;
// all other pointers in these fixed acyclic record graphs are measured deeply.
func resourceMeasure(ctx context.Context, a *Account, v reflect.Value) (int64, error) {
	return resourceMeasureAs(ctx, a, v, "")
}
func resourceMeasureAs(ctx context.Context, a *Account, v reflect.Value, to string) (int64, error) {
	if ctx == nil {
		return 0, contract("context", "must not be nil")
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if err := a.Charge(BudgetRuleSteps, 1); err != nil {
		return 0, err
	}
	if !v.IsValid() {
		return 0, nil
	}
	if v.Type() == reflect.TypeFor[SourceOriginContextResult]() {
		return resourceRecordMeasureAs(ctx, a, v, to, true)
	}
	if field, ok := resourceNumericFields[v.Type()]; ok {
		return v.Field(field).Int() + 64, nil
	}
	total := int64(32)
	if v.Type() == reflect.TypeFor[NodeRef]() || v.Type() == reflect.TypeFor[Location]() {
		total += int64(len(to))
	}
	add := func(x int64, e error) error {
		if e != nil {
			return e
		}
		var err error
		total, err = rationalCheckedAdd(BudgetBufferedBytes, total, x)
		return err
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if err := add(resourceMeasureAs(ctx, a, v.Field(i), to)); err != nil {
				return 0, err
			}
		}
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			if err := a.Charge(BudgetRuleSteps, int64(v.Len())); err != nil {
				return 0, err
			}
			if err := add(int64(v.Len()), nil); err != nil {
				return 0, err
			}
			break
		}
		for i := 0; i < v.Len(); i++ {
			if err := add(resourceMeasureAs(ctx, a, v.Index(i), to)); err != nil {
				return 0, err
			}
		}
	case reflect.Pointer:
		if !v.IsNil() {
			if err := add(resourceMeasureAs(ctx, a, v.Elem(), to)); err != nil {
				return 0, err
			}
		}
	case reflect.String:
		// String length is already available without scanning. Reserve copy work
		// per bounded block rather than pretending measurement scans each byte.
		blocks := int64(v.Len()) / resourceCopyChunk
		if v.Len()%resourceCopyChunk != 0 {
			blocks++
		}
		if err := a.Charge(BudgetRuleSteps, blocks); err != nil {
			return 0, err
		}
		if err := add(int64(v.Len()), nil); err != nil {
			return 0, err
		}
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
	default:
		return 0, contract("resource payload", "unsupported private payload")
	}
	if _, err := allocationSize(total); err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Session) resourceIDs(ctx context.Context) ([]ResourceIDFact, error) {
	if s.schema == nil || s.schema.run == nil || s.schema.active == nil {
		return nil, nil
	}
	var out []ResourceIDFact
	bytes, err := rationalCheckedMul(BudgetBufferedBytes, int64(len(s.sequence)), 128)
	if err != nil {
		return nil, err
	}
	if _, err = allocationSize(bytes); err != nil {
		return nil, err
	}
	if err = s.retain(bytes); err != nil {
		return nil, err
	}
	parts := make(map[NodeID]NodeRef, len(s.sequence))
	for _, id := range s.sequence {
		if err = s.step(ctx, 1); err != nil {
			return nil, err
		}
		n := s.nodes[id]
		part := parts[n.header.Parent]
		if n.header.Type.State == EvidenceKnown && n.header.Type.Value.Assembly == AssemblyMusicXML && n.header.Type.Value.Name == (QName{Local: "score-part"}) {
			part = NodeRef{s.documentID, id}
		}
		parts[id] = part
		bound := s.schema.nodes[id]
		if !bound.available {
			continue
		}
		scalar := s.schema.run.Scalar(bound.binding)
		results := s.schema.run.Results(bound.binding)
		if n.header.Parent != 0 && scalar.Builtin == "ID" && scalar.Known && results.Scalar.Valid && results.Declaration.Valid && n.header.Declaration.State == EvidenceKnown {
			fact, e := s.resourceID(ctx, n, n.header.Parent, part, scalar.Value.Text, n.header.Declaration.Value)
			if e != nil {
				return nil, e
			}
			out = append(out, fact)
		}
		for a := range s.schema.run.Attributes(bound.binding) {
			if err = s.step(ctx, 1); err != nil {
				return nil, err
			}
			if a.Scalar.Builtin != "ID" || !a.Present || !a.TypeValid || !a.DeclarationValid || !a.Scalar.Known {
				continue
			}
			fact, e := s.resourceID(ctx, n, n.header.ID, part, a.Scalar.Value.Text, s.schema.active.declaration(a.Use))
			if e != nil {
				return nil, e
			}
			out = append(out, fact)
		}
	}
	return out, nil
}
func (s *Session) resourceID(ctx context.Context, n *factNode, actualOwner NodeID, part NodeRef, value string, declaration DeclarationID) (ResourceIDFact, error) {
	resolved, e := s.schema.run.ResolveIdentity(ctx, value)
	if e != nil {
		return ResourceIDFact{}, e
	}
	unique := Evidence[bool]{State: EvidenceUnknown, Reason: "identity-scope-incomplete"}
	switch string(resolved.Status) {
	case "unique":
		unique = Evidence[bool]{State: EvidenceKnown, Value: true}
	case "ambiguous":
		bad, e := s.schemaInvalidValue(ctx, s.root, XSDCheckIdentity)
		if e != nil {
			return ResourceIDFact{}, e
		}
		if bad.Cause != nil {
			unique = Evidence[bool]{State: EvidenceInvalid, Reason: "ambiguous-resource-id", Cause: bad.Cause}
		} else {
			unique = Evidence[bool]{State: EvidenceUnknown, Reason: "ambiguous-resource-id"}
		}
	}
	owner := Evidence[NodeRef]{State: EvidenceAbsent}
	if part.Node != 0 {
		owner = Evidence[NodeRef]{State: EvidenceKnown, Value: part}
	}
	kind := n.header.Name
	if actual := s.nodes[actualOwner]; actual != nil {
		kind = actual.header.Name
	}
	fact := ResourceIDFact{Value: value, Declaration: declaration, Subject: NodeRef{s.documentID, actualOwner}, Source: NodeRef{s.documentID, n.header.ID}, Part: owner, Kind: Evidence[QName]{State: EvidenceKnown, Value: kind}, Unique: unique, Location: n.header.Location}
	cost, e := resourceMeasure(ctx, s.account, reflect.ValueOf(fact))
	if e != nil {
		return ResourceIDFact{}, e
	}
	if e = s.retain(cost + 512); e != nil {
		return ResourceIDFact{}, e
	}
	return fact, nil
}

func (s *ResourceSnapshot) live() error {
	if s != nil && s.owner != nil {
		return &SessionError{Operation: "resource snapshot", Reason: "ownership transferred to package"}
	}
	if s == nil || s.closed || s.account == nil {
		return &SessionError{Operation: "resource snapshot", Reason: "closed or uninitialized snapshot"}
	}
	return nil
}

// Header returns an owned fixed-size identity copy, or zero after Close.
func (s *ResourceSnapshot) Header() ResourceSnapshotHeader {
	if s.live() != nil {
		return ResourceSnapshotHeader{}
	}
	return stateClone(s.header)
}

// ContextCount returns the selected intrinsic projection count, or zero after Close.
func (s *ResourceSnapshot) ContextCount() int {
	if s.live() != nil {
		return 0
	}
	return len(s.contexts)
}

// IDCount returns schema-qualified ID occurrences, or zero after Close.
func (s *ResourceSnapshot) IDCount() int {
	if s.live() != nil {
		return 0
	}
	return len(s.ids)
}

// ContextAt admits temporary copy storage and work against the live Account.
// The returned copy is caller-owned; adapters separately admit lasting storage.
// Content access after Close is rejected.
func (s *ResourceSnapshot) ContextAt(ctx context.Context, i int) (ContextResult, bool, error) {
	if e := s.live(); e != nil {
		return ContextResult{}, false, e
	}
	if ctx == nil {
		return ContextResult{}, false, contract("context", "must not be nil")
	}
	if e := ctx.Err(); e != nil {
		return ContextResult{}, false, e
	}
	if i < 0 || i >= len(s.contexts) {
		return ContextResult{}, false, nil
	}
	out, err := resourceLiveCopy(ctx, s.account, s.contexts[i], "", "")
	return out, err == nil, err
}

// IDAt returns an independently owned schema ID occurrence.
func (s *ResourceSnapshot) IDAt(ctx context.Context, i int) (ResourceIDFact, bool, error) {
	if e := s.live(); e != nil {
		return ResourceIDFact{}, false, e
	}
	if ctx == nil {
		return ResourceIDFact{}, false, contract("context", "must not be nil")
	}
	if e := ctx.Err(); e != nil {
		return ResourceIDFact{}, false, e
	}
	if i < 0 || i >= len(s.ids) {
		return ResourceIDFact{}, false, nil
	}
	out, err := resourceLiveCopy(ctx, s.account, s.ids[i], "", "")
	return out, err == nil, err
}

// Report returns a defensive caller-owned API copy of the canonical intrinsic
// report, admitting temporary copy storage and work while its Account is live.
func (s *ResourceSnapshot) Report(ctx context.Context) (Report, error) {
	if e := s.live(); e != nil {
		return Report{}, e
	}
	if ctx == nil {
		return Report{}, contract("context", "must not be nil")
	}
	if e := ctx.Err(); e != nil {
		return Report{}, e
	}
	return resourceLiveCopy(ctx, s.account, s.report, "", "")
}

// Close releases standalone snapshot ownership once. After successful package
// transfer only the owning package may release it. Close does not need context.
func (s *ResourceSnapshot) Close() error {
	if s == nil || s.closed {
		return nil
	}
	if s.owner != nil {
		return &SessionError{Operation: "Close resource", Reason: "ownership transferred to package"}
	}
	s.release()
	return nil
}
func (s *ResourceSnapshot) release() {
	if s == nil || s.closed {
		return
	}
	if s.retained > 0 {
		_ = s.account.Release(BudgetBufferedBytes, s.retained)
	}
	s.retained = 0
	s.closed = true
	s.report = Report{}
	s.contexts = nil
	s.fragments = nil
	s.fragmentOrder = nil
	s.ids = nil
	s.header = ResourceSnapshotHeader{}
	s.owner = nil
}

// Remapping is a sealed structural ownership operation over library-defined
// value types. It remaps document references, never arbitrary string contents.
func resourceRemap(value reflect.Value, from, to string) {
	if !value.IsValid() {
		return
	}
	if _, ok := resourceNumericFields[value.Type()]; ok {
		return
	}
	if value.Type() == reflect.TypeFor[NodeRef]() || value.Type() == reflect.TypeFor[Location]() {
		if value.FieldByName("DocumentID").String() == from {
			value.FieldByName("DocumentID").SetString(strings.Clone(to))
		}
		return
	}
	switch value.Kind() {
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			resourceRemap(value.Field(i), from, to)
		}
	case reflect.Slice:
		for i := 0; i < value.Len(); i++ {
			resourceRemap(value.Index(i), from, to)
		}
	case reflect.Pointer:
		if !value.IsNil() {
			resourceRemap(value.Elem(), from, to)
		}
	}
}

// resourceLiveCopy meters the entire closed value before allocating. The scratch
// lease spans copying and remapping, with cancellation checks at every record.
// The caller owns a successful returned value outside the live Account.
func resourceLiveCopy[T any](ctx context.Context, a *Account, value T, from, to string) (T, error) {
	var zero T
	n, err := resourceMeasureAs(ctx, a, reflect.ValueOf(value), to)
	if err != nil {
		return zero, err
	}
	if err = a.Charge(BudgetBufferedBytes, n); err != nil {
		return zero, err
	}
	defer a.Release(BudgetBufferedBytes, n)
	out, err := resourceCopyAs(ctx, reflect.ValueOf(value), from, to)
	if err != nil {
		return zero, err
	}
	if err = ctx.Err(); err != nil {
		return zero, err
	}
	return out.Interface().(T), nil
}
func resourceCopyAs(ctx context.Context, v reflect.Value, from, to string) (reflect.Value, error) {
	if ctx == nil {
		return reflect.Value{}, contract("context", "must not be nil")
	}
	if err := ctx.Err(); err != nil {
		return reflect.Value{}, err
	}
	if !v.IsValid() {
		return v, ctx.Err()
	}
	out := reflect.New(v.Type()).Elem()
	if err := resourceCopyInto(ctx, out, v, from, to); err != nil {
		return reflect.Value{}, err
	}
	return out, nil
}

// Fill already allocated destinations instead of allocating a reflected box for
// every struct/string field. The root and mutable pointers/slices still own their
// storage; immutable numeric values may safely share their private magnitude.
func resourceCopyInto(ctx context.Context, out, v reflect.Value, from, to string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, ok := resourceNumericFields[v.Type()]; ok {
		out.Set(v)
		return nil
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if err := resourceCopyInto(ctx, out.Field(i), v.Field(i), from, to); err != nil {
				return err
			}
		}
		if from != "" && (v.Type() == reflect.TypeFor[NodeRef]() || v.Type() == reflect.TypeFor[Location]()) {
			if out.FieldByName("DocumentID").String() == from {
				text, err := resourceCopyString(ctx, to)
				if err != nil {
					return err
				}
				out.FieldByName("DocumentID").SetString(text)
			}
		}
	case reflect.Slice:
		if v.IsNil() {
			return nil
		}
		out.Set(reflect.MakeSlice(v.Type(), v.Len(), v.Len()))
		if v.Type().Elem().Kind() == reflect.Uint8 {
			for start := 0; start < v.Len(); {
				if err := ctx.Err(); err != nil {
					return err
				}
				end := start + min(resourceCopyChunk, v.Len()-start)
				reflect.Copy(out.Slice(start, end), v.Slice(start, end))
				start = end
			}
			return nil
		}
		for i := 0; i < v.Len(); i++ {
			if err := resourceCopyInto(ctx, out.Index(i), v.Index(i), from, to); err != nil {
				return err
			}
		}
	case reflect.Pointer:
		if !v.IsNil() {
			out.Set(reflect.New(v.Type().Elem()))
			return resourceCopyInto(ctx, out.Elem(), v.Elem(), from, to)
		}
	case reflect.String:
		text, err := resourceCopyString(ctx, v.String())
		if err != nil {
			return err
		}
		out.SetString(text)
	default:
		out.Set(v)
	}
	return nil
}

func resourceCopyString(ctx context.Context, value string) (string, error) {
	if len(value) <= resourceCopyChunk {
		return strings.Clone(value), ctx.Err()
	}
	var out strings.Builder
	out.Grow(len(value))
	for start := 0; start < len(value); {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		end := start + min(resourceCopyChunk, len(value)-start)
		out.WriteString(value[start:end])
		start = end
	}
	return out.String(), nil
}
