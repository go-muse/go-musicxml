package validation

import (
	"context"
	"reflect"
	"strings"
)

// ResourceSnapshotHeader identifies an intrinsic assessment without confusing
// occurrence identity, schema assembly, root kind and assessment completeness.
type ResourceSnapshotHeader struct {
	DocumentID      string
	Root            NodeRef
	Name            Evidence[QName]
	Assembly        Evidence[Assembly]
	Profile         ProfileID
	CatalogDigest   string
	Target          Target
	InputComplete   bool
	Syntax          SyntaxState
	ScopeComplete   Evidence[bool]
	IDScopeComplete Evidence[bool]
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
	header   ResourceSnapshotHeader
	report   Report
	contexts []ContextResult
	ids      []ResourceIDFact
	account  *Account
	retained int64
	closed   bool
	owner    *resourceLeaseOwner
}

// FinishWithResourceSnapshot is an alternate terminal finalizer. It evaluates
// all intrinsic phases before admitting and copying the selected owned facts,
// then cleans the session exactly once. The best Report is always returned even
// if snapshot admission fails. A nonnil snapshot must be closed or transferred
// to a package session sharing the exact same Account.
func (s *Session) FinishWithResourceSnapshot(ctx context.Context, end Completion) (Report, *ResourceSnapshot, error) {
	return s.finishResource(ctx, end, true)
}

func (s *Session) makeResourceSnapshot(ctx context.Context, report Report) (*ResourceSnapshot, error) {
	if err := s.step(ctx, 1); err != nil {
		return nil, err
	}
	h := ResourceSnapshotHeader{DocumentID: s.documentID, Root: NodeRef{DocumentID: s.documentID}, Profile: s.profile.ID, CatalogDigest: s.profile.CatalogDigest, Target: s.profile.Target, InputComplete: s.inputComplete, Syntax: report.Syntax, Name: Evidence[QName]{State: EvidenceUnknown, Reason: "resource-root-unavailable"}, Assembly: Evidence[Assembly]{State: EvidenceUnknown, Reason: "resource-assembly-unavailable"}, ScopeComplete: Evidence[bool]{State: EvidenceUnknown, Reason: "resource-scope-incomplete"}}
	if s.root != nil {
		h.Root.Node = s.root.header.ID
		h.Name = Evidence[QName]{State: EvidenceKnown, Value: s.root.header.Name}
		if s.schema != nil && s.schema.active != nil {
			h.Assembly = Evidence[Assembly]{State: EvidenceKnown, Value: s.schema.active.assembly}
		}
	}
	if s.inputComplete && !s.enumerationIncomplete && s.terminalCause == nil {
		h.ScopeComplete = Evidence[bool]{State: EvidenceKnown, Value: true}
	}
	h.IDScopeComplete = Evidence[bool]{State: EvidenceUnknown, Reason: "resource-id-scope-incomplete"}
	if s.schema != nil && s.schema.identityReady && s.schema.identity.Complete {
		h.IDScopeComplete = Evidence[bool]{State: EvidenceKnown, Value: true}
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
	if _, err = AllocationSize(size); err != nil {
		return nil, err
	}
	if err = s.account.Charge(BudgetBufferedBytes, size); err != nil {
		return nil, err
	}
	out := &ResourceSnapshot{header: stateClone(h), report: stateClone(report), ids: stateClone(ids), account: s.account, retained: size, contexts: make([]ContextResult, 0, count)}
	for _, v := range report.Context {
		if resourceSelectedContext(v.Property) {
			out.contexts = append(out.contexts, stateClone(v))
		}
	}
	return out, nil
}

func resourceSelectedContext(p ContextProperty) bool {
	switch string(p) {
	case "container.metadata", "resource.link", "resource.image", "resource.schema-hints", "resource.group", "resource.instrument", "sounds.mapping", "layout.opus-version", "layout.opus-new-page":
		return true
	}
	return false
}

// resourceMeasure is limited to closed library-defined transport/result values.
// It is not a public reflection or predicate hook. Numeric wrappers are immutable;
// all other pointers in these fixed acyclic record graphs are measured deeply.
func resourceMeasure(ctx context.Context, a *Account, v reflect.Value) (int64, error) {
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
	if v.CanInterface() {
		switch n := v.Interface().(type) {
		case Integer:
			return n.ByteSize() + 64, nil
		case Decimal:
			return n.ByteSize() + 64, nil
		case Rational:
			return n.ByteSize() + 64, nil
		}
	}
	total := int64(32)
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
			if err := add(resourceMeasure(ctx, a, v.Field(i))); err != nil {
				return 0, err
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if err := add(resourceMeasure(ctx, a, v.Index(i))); err != nil {
				return 0, err
			}
		}
	case reflect.Pointer:
		if !v.IsNil() {
			if err := add(resourceMeasure(ctx, a, v.Elem())); err != nil {
				return 0, err
			}
		}
	case reflect.String:
		if err := a.Charge(BudgetRuleSteps, int64(v.Len())); err != nil {
			return 0, err
		}
		if err := add(int64(v.Len()), nil); err != nil {
			return 0, err
		}
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
	default:
		return 0, contract("resource payload", "unsupported private payload")
	}
	if _, err := AllocationSize(total); err != nil {
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
	if _, err = AllocationSize(bytes); err != nil {
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
			fact, e := s.resourceID(ctx, n, part, scalar.Value.Text, n.header.Declaration.Value)
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
			fact, e := s.resourceID(ctx, n, part, a.Scalar.Value.Text, s.schema.active.declaration(a.Use))
			if e != nil {
				return nil, e
			}
			out = append(out, fact)
		}
	}
	return out, nil
}
func (s *Session) resourceID(ctx context.Context, n *factNode, part NodeRef, value string, declaration DeclarationID) (ResourceIDFact, error) {
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
	actualOwner := NodeID(resolved.Owner)
	if actualOwner == 0 {
		if NodeID(resolved.FirstOwner) == n.header.ID || NodeID(resolved.FirstOwner) == n.header.Parent {
			actualOwner = NodeID(resolved.FirstOwner)
		} else if NodeID(resolved.SecondOwner) == n.header.ID || NodeID(resolved.SecondOwner) == n.header.Parent {
			actualOwner = NodeID(resolved.SecondOwner)
		}
	}
	if actualOwner == 0 {
		actualOwner = n.header.ID
		unique = Evidence[bool]{State: EvidenceUnknown, Reason: "resource-id-has-no-containing-owner"}
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
	if s == nil || s.closed || s.account == nil || s.owner != nil {
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

// ContextAt returns an owned API copy; these caller-owned getter copies do not
// consume or retain root Account storage. Retaining work adapters admit their
// own storage. Content access after Close is rejected.
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
	return stateClone(s.contexts[i]), true, nil
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
	return stateClone(s.ids[i]), true, nil
}

// Report returns a defensive caller-owned API copy of the canonical intrinsic
// report. It never consults a released Account.
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
	return stateClone(s.report), nil
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
	if value.CanInterface() {
		switch value.Interface().(type) {
		case Integer, Decimal, Rational:
			return
		case NodeRef:
			if value.FieldByName("DocumentID").String() == from {
				value.FieldByName("DocumentID").SetString(strings.Clone(to))
			}
			return
		case Location:
			if value.FieldByName("DocumentID").String() == from {
				value.FieldByName("DocumentID").SetString(strings.Clone(to))
			}
			return
		}
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
