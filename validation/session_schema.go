package validation

import (
	"context"
	"errors"
	"math/big"
	"strconv"
	"strings"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

type schemaSession struct {
	schemas       map[Assembly]*CompiledSchema
	active        *CompiledSchema
	run           *xsd.Run
	bound         bool
	nodes         map[NodeID]schemaNode
	identity      xsd.IdentityResults
	identityReady bool
	recorded      map[decisionID]schemaRecord
	proofRules    map[Assembly]map[XSDCheck][]RuleKey
	dtdSummaries  map[RuleKey]schemaDTDSummary
	dtdOrder      []RuleKey
	dtdOmitted    bool
}

// A session has one document. Only the fixed, cause-free DTD capability reason
// is eligible for this bounded representative-location summary.
type schemaDTDSummary struct {
	first NodeID
	count uint64
}

const schemaDTDSummaryBytes int64 = 320 // map, order capacity, record and bounded count text

type schemaRecord struct {
	details  int
	complete bool
}
type schemaNode struct {
	binding   xsd.Binding
	available bool
	skipped   bool
	reason    Code
	cause     *DecisionRef
	// representable is set once the completed node's numeric values have been
	// checked against the model representation; unrepresentable lists the
	// values that fail.
	representable   bool
	unrepresentable []xsd.Detail
}
type schemaMeter struct{ session *Session }

func (m schemaMeter) Step(ctx context.Context, n int64) error { return m.session.step(ctx, n) }
func (m schemaMeter) Reserve(n int64) error                   { return m.session.account.Charge(BudgetBufferedBytes, n) }
func (m schemaMeter) Release(n int64) error                   { return m.session.account.Release(BudgetBufferedBytes, n) }
func (m schemaMeter) Scalar(n int64) error                    { return m.session.account.Check(BudgetScalarBytes, n) }
func (m schemaMeter) Identity(n int64) error                  { return m.session.account.Charge(BudgetIDs, n) }

func (s *Session) schemaSubject(ctx context.Context, n *factNode) (xsd.Subject, error) {
	bytes := int64(256) + int64(len(n.header.Bindings))*64 + int64(len(n.attributes))*256 + int64(len(n.children))*64
	if err := s.retain(bytes); err != nil {
		return xsd.Subject{}, err
	}
	out := xsd.Subject{ID: xsd.SubjectID(n.header.ID), Parent: xsd.SubjectID(n.header.Parent), Name: xsd.QName{Space: n.header.Name.Space, Local: n.header.Name.Local}, Path: n.header.Location.Path, Text: n.end.Scalar.Lexeme.Value, TextKnown: n.closed && n.end.Scalar.Lexeme.State == EvidenceKnown, AttributesComplete: n.closed && n.end.AttributesComplete.State == EvidenceKnown && n.end.AttributesComplete.Value, ChildrenComplete: n.closed && n.end.ChildrenComplete.State == EvidenceKnown && n.end.ChildrenComplete.Value && n.end.Order.State == EvidenceKnown && n.end.Order.Value == OrderSource}
	if s.profile.Target == TargetModel {
		out.CurrentModel = true
		out.ChildrenOrderKnown = n.closed && n.end.Order.State == EvidenceKnown && (n.end.Order.Value == OrderModelFields || n.end.Order.Value == OrderModelContent)
		out.ChildrenComplete = n.closed && n.end.ChildrenComplete.State == EvidenceKnown && n.end.ChildrenComplete.Value && out.ChildrenOrderKnown
		out.Text, out.TextKnown = "", false
		var err error
		out.Current, err = s.schemaCurrent(ctx, *n.end.Scalar.Value)
		if err != nil {
			return out, err
		}
	}
	if !n.closed && n.attributeSeal != nil {
		out.AttributesComplete = n.attributeSeal.State == EvidenceKnown && n.attributeSeal.Value
	}
	out.Namespaces = make(map[string]string, len(n.header.Bindings))
	for _, b := range n.header.Bindings {
		if err := s.step(ctx, 1); err != nil {
			return out, err
		}
		out.Namespaces[b.Prefix] = b.URI
	}
	out.Attributes = make([]xsd.Attribute, 0, len(n.attributes))
	for _, a := range n.attributes {
		if err := s.step(ctx, 1); err != nil {
			return out, err
		}
		if a.Presence.State == EvidenceAbsent {
			continue
		}
		if a.Presence.State != EvidenceKnown || !a.Presence.Value {
			out.AttributesComplete = false
			if a.Name.Space != xsd.XSINamespace || (a.Name.Local != "type" && a.Name.Local != "nil") {
				continue
			}
		}
		attribute := xsd.Attribute{Attr: xsd.Attr{Name: xsd.QName{Space: a.Name.Space, Local: a.Name.Local}, Value: a.Scalar.Lexeme.Value}, ValueKnown: a.Presence.State == EvidenceKnown && a.Presence.Value && a.Scalar.Lexeme.State == EvidenceKnown, NamespaceDeclaration: a.Name.Space == "http://www.w3.org/2000/xmlns/"}
		if out.CurrentModel {
			attribute.Value, attribute.ValueKnown = "", false
			var err error
			attribute.Current, err = s.schemaCurrent(ctx, a.Scalar.Value)
			if err != nil {
				return out, err
			}
		}
		out.Attributes = append(out.Attributes, attribute)
	}
	out.Children = make([]xsd.Child, len(n.children))
	for i, ch := range n.children {
		if err := s.step(ctx, 1); err != nil {
			return out, err
		}
		out.Children[i] = xsd.Child{ID: xsd.SubjectID(ch.header.ID), Name: xsd.QName{Space: ch.header.Name.Space, Local: ch.header.Name.Local}, Path: ch.header.Location.Path}
	}
	return out, nil
}

// schemaBindFrame borrows witnesses from Run until Run.Close. Only children
// owns additional backing storage; Run.Complete consumes it synchronously and
// keeps a by-value Completion, never the caller's slice.
type schemaBindFrame struct {
	node      *factNode
	use       xsd.Use
	binding   xsd.Binding
	witnesses []xsd.Witness
	next      int
	children  []xsd.Completion
	entered   bool
}

// Fixed conservative slot envelopes cover both amd64 and 386 (size-guarded in
// tests). Charge allocated capacity, not the number of all document nodes.
const schemaBindFrameBytes int64 = 256
const schemaBindCompletionBytes int64 = 64

type schemaBindScratch struct {
	account *Account
	frames  []schemaBindFrame
	bytes   int64
}

func (w *schemaBindScratch) reserve(count, slot int64) error {
	n, err := rationalCheckedMul(BudgetBufferedBytes, count, slot)
	if err != nil {
		return err
	}
	if _, err := allocationSize(n); err != nil {
		return err
	}
	if err := w.account.Charge(BudgetBufferedBytes, n); err != nil {
		return err
	}
	w.bytes += n
	return nil
}

func (w *schemaBindScratch) releaseBytes(n int64) {
	_ = w.account.Release(BudgetBufferedBytes, n)
	w.bytes -= n
}

func (w *schemaBindScratch) push(ctx context.Context, frame schemaBindFrame) error {
	if ctx == nil {
		return contract("context", "must not be nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(w.frames) == cap(w.frames) {
		// The byte-size check also bounds capacity and its host-int conversion.
		capacity, err := rationalCheckedMul(BudgetBufferedBytes, int64(cap(w.frames)), 2)
		if err != nil {
			return err
		}
		capacity = max(1, capacity)
		if err := w.reserve(capacity, schemaBindFrameBytes); err != nil {
			return err
		}
		grown := make([]schemaBindFrame, len(w.frames), int(capacity))
		copy(grown, w.frames)
		oldBytes := int64(cap(w.frames)) * schemaBindFrameBytes
		clear(w.frames)
		w.frames = grown
		// Reserve the complete new allocation before dropping the old one:
		// old and new backing arrays coexist while copying.
		w.releaseBytes(oldBytes)
	}
	w.frames = append(w.frames, frame)
	return nil
}

func (w *schemaBindScratch) pop() {
	last := len(w.frames) - 1
	n := int64(cap(w.frames[last].children)) * schemaBindCompletionBytes
	w.frames[last] = schemaBindFrame{}
	w.frames = w.frames[:last]
	w.releaseBytes(n)
}

func (w *schemaBindScratch) release() {
	clear(w.frames)
	w.frames = nil
	w.releaseBytes(w.bytes)
}

// bindSourceSchema owns the pending source-fact transition. Exact child uses
// come only from the shared grammar witness; a path or QName never guesses one.
func (s *Session) bindSourceSchema(ctx context.Context) error {
	state := s.schema
	if state == nil || state.bound || s.root == nil {
		return nil
	}
	state.bound = true
	if err := s.retain(128 + int64(len(s.nodes))*128); err != nil {
		return err
	}
	state.nodes = make(map[NodeID]schemaNode, len(s.nodes))
	if s.profile.Target == TargetSource && (s.root.header.Declaration.State == EvidenceUnsupported || s.root.header.Type.State == EvidenceUnsupported) {
		reason := s.root.header.Declaration.Reason
		if reason == "" {
			reason = s.root.header.Type.Reason
		}
		for _, id := range s.sequence {
			state.nodes[id] = schemaNode{reason: reason}
		}
		return nil
	}
	var err error
	if s.catalog.sourceDeclarations[s.profile.ID][state.active.assembly] == xsdSourceDeclarationsV1 {
		var domain *xsd.EntityDeclarations
		if s.sourceEntities != nil {
			domain = &s.sourceEntities.domain
		}
		state.run, err = xsd.NewRunWithSourceDeclarations(state.active.kernel, schemaMeter{s}, domain)
	} else {
		state.run, err = xsd.NewRun(state.active.kernel, schemaMeter{s})
	}
	if err != nil {
		return err
	}
	rootUse, ok := state.active.kernel.Root(xsd.QName{Space: s.root.header.Name.Space, Local: s.root.header.Name.Local})
	if !ok {
		return contract("source schema root", "no compiled root use")
	}
	scratch := schemaBindScratch{account: s.account}
	defer scratch.release()
	if err := scratch.push(ctx, schemaBindFrame{node: s.root, use: rootUse}); err != nil {
		return err
	}
	for len(scratch.frames) > 0 {
		if err := s.step(ctx, 1); err != nil {
			return err
		}
		f := &scratch.frames[len(scratch.frames)-1]
		if !f.entered {
			subject, subjectErr := s.schemaSubject(ctx, f.node)
			if subjectErr != nil {
				return subjectErr
			}
			binding, bindErr := state.run.Bind(ctx, f.use, subject)
			f.binding = binding
			f.entered = true
			available := state.run.Results(binding).Binding.Applicable
			state.nodes[f.node.header.ID] = schemaNode{binding: binding, available: available, reason: "schema-binding-unavailable"}
			// Persist proved failures before later binding, exact-value or buffer work.
			if err := s.recordXSDProvedFailures(ctx, f.node); err != nil {
				return err
			}
			if bindErr != nil {
				return bindErr
			}
			if err := s.publishSchemaFacts(ctx, f.node, f.use, binding); err != nil {
				return err
			}
			f.witnesses = state.run.Witnesses(binding)
			if err := scratch.reserve(int64(len(f.witnesses)), schemaBindCompletionBytes); err != nil {
				return err
			}
			f.children = make([]xsd.Completion, 0, len(f.witnesses))
		}
		if f.next < len(f.witnesses) {
			witness := f.witnesses[f.next]
			f.next++
			if witness.Mode != xsd.WitnessDeclared {
				child := s.nodes[NodeID(witness.Child)]
				if child == nil {
					return contract("schema witness", "unknown child occurrence")
				}
				state.nodes[child.header.ID] = schemaNode{skipped: witness.Mode == xsd.WitnessSkip, reason: Code(witness.Mode)}
				continue
			}
			child := s.nodes[NodeID(witness.Child)]
			if child == nil {
				return contract("schema witness", "unknown child occurrence")
			}
			// No pointer into the old backing array survives a possible resize.
			f = nil
			if err := scratch.push(ctx, schemaBindFrame{node: child, use: witness.Use}); err != nil {
				return err
			}
			continue
		}
		completion, completeErr := state.run.Complete(ctx, f.binding, f.children)
		if err := s.checkRepresentable(ctx, f.node); err != nil {
			return err
		}
		if err := s.recordXSDProvedFailures(ctx, f.node); err != nil {
			return err
		}
		if completeErr != nil {
			return completeErr
		}
		f = nil
		scratch.pop()
		if len(scratch.frames) > 0 {
			parent := &scratch.frames[len(scratch.frames)-1]
			parent.children = append(parent.children, completion)
		}
	}
	// The remaining causal/identity work does not need traversal storage.
	scratch.release()
	// An unmatched subtree remains unavailable. Preserve its exact containing
	// content failure as the causal identity when that rule was selected.
	for _, id := range s.sequence {
		if _, ok := state.nodes[id]; ok {
			continue
		}
		n := s.nodes[id]
		item := schemaNode{reason: "exact-declaration-use-unavailable"}
		ancestor := s.nodes[n.header.Parent]
		for ancestor != nil {
			if bound, ok := state.nodes[ancestor.header.ID]; ok {
				if bound.skipped {
					item.skipped = true
					item.reason = "wildcard-skip"
					break
				}
				if ancestor.end.Order.State == EvidenceInvalid && ancestor.end.Order.Cause != nil {
					item.cause = ancestor.end.Order.Cause
					break
				}
				if bound.cause != nil {
					item.cause = bound.cause
					break
				}
				for _, key := range s.catalog.orders[s.profile.ID] {
					p := s.catalog.plans[s.catalog.rules[key].Predicate]
					if p.kind == planXSD && p.xsdCheck == XSDCheckContent && p.schema.assembly == state.active.assembly {
						if d, found := s.lookupDecision(decisionID{key, ancestor.header.ID}); found && d.Outcome == OutcomeFail {
							item.cause = &DecisionRef{Rule: key, Subject: NodeRef{s.documentID, ancestor.header.ID}}
							break
						}
					}
				}
				break
			}
			ancestor = s.nodes[ancestor.header.Parent]
		}
		state.nodes[id] = item
		typ := Evidence[ComponentID]{State: EvidenceUnsupported, Reason: item.reason}
		decl := Evidence[DeclarationID]{State: EvidenceUnsupported, Reason: item.reason}
		if item.cause != nil {
			typ.State, typ.Cause = EvidenceInvalid, item.cause
			decl.State, decl.Cause = EvidenceInvalid, item.cause
		}
		ownedType, err := s.retainType(typ)
		if err != nil {
			return err
		}
		ownedDeclaration, err := s.retainDeclaration(decl)
		if err != nil {
			return err
		}
		n.header.Type, n.header.Declaration = ownedType, ownedDeclaration
	}
	identity, identityErr := state.run.IdentityPreview(ctx)
	state.identity = identity
	if err := s.recordXSDIdentityFailure(ctx); err != nil {
		return err
	}
	return identityErr
}

// The local QName-to-slot map needs a header and an entire first group even
// for one attribute. Larger Go maps round hints up for load factor and table
// capacity: reserve three key/value/control slots per entry, plus fixed
// header/first-group space. Payload-size guards and independent measured heap
// tests cover allocator rounding and table/directory overhead as well.
const (
	schemaAttributeIndexHeaderBytes int64 = 512
	schemaAttributeIndexSlotBytes   int64 = 128
)

func schemaAttributeIndexBytes(count int64) (int64, error) {
	slots, err := rationalCheckedMul(BudgetBufferedBytes, count, schemaAttributeIndexSlotBytes)
	if err != nil {
		return 0, err
	}
	return retainedFactSize(schemaAttributeIndexHeaderBytes, slots)
}

func (s *Session) publishSchemaFacts(ctx context.Context, n *factNode, use xsd.Use, binding xsd.Binding) error {
	state := s.schema
	declaration := state.active.declaration(use)
	decl := Evidence[DeclarationID]{State: EvidenceUnsupported, Reason: "declaration-identity-unavailable"}
	if validDeclaration(declaration) {
		decl = Evidence[DeclarationID]{State: EvidenceKnown, Value: declaration}
	}
	typ := Evidence[ComponentID]{State: EvidenceUnsupported, Reason: "actual-type-unavailable"}
	var typeKey xsd.CacheIdentity
	if kind, known := state.run.ActualType(binding); known {
		component := state.active.component(kind)
		if validComponent(component) {
			typ = Evidence[ComponentID]{State: EvidenceKnown, Value: component}
			typeKey = kind.CacheIdentity()
		} else {
			typ = Evidence[ComponentID]{State: EvidenceUnsupported, Reason: "type-identity-unavailable"}
		}
	}
	ownedDeclaration, err := s.retainSchemaDeclaration(ctx, use.CacheIdentity(), decl)
	if err != nil {
		return err
	}
	ownedType, err := s.retainSchemaType(ctx, typeKey, typ)
	if err != nil {
		return err
	}
	n.header.Declaration, n.header.Type = ownedDeclaration, ownedType
	if s.profile.Target != TargetModel {
		scalar, err := s.schemaAtomic(ctx, state.run.Scalar(binding))
		if err != nil {
			return err
		}
		owned, err := s.retainSchemaAtomic(scalar)
		if err != nil {
			return err
		}
		n.end.Scalar.Value = owned
	}
	indexBytes, err := schemaAttributeIndexBytes(int64(len(n.attributes)))
	if err != nil {
		return err
	}
	if err := s.account.Charge(BudgetBufferedBytes, indexBytes); err != nil {
		return err
	}
	// The name-to-slot index is local to publication. Only the independently
	// retained declaration/scalar facts escape; drop the map on every exit.
	var attributes map[QName]int
	defer func() {
		attributes = nil
		_ = s.account.Release(BudgetBufferedBytes, indexBytes)
	}()
	attributes = make(map[QName]int, len(n.attributes))
	for i := range n.attributes {
		if n.attributes[i].Presence.State == EvidenceAbsent {
			continue
		}
		n.attributes[i].Declaration = Evidence[DeclarationID]{State: EvidenceUnsupported, Reason: "attribute-use-unavailable"}
		if s.profile.Target != TargetModel {
			n.attributes[i].Scalar.Value = Evidence[Atomic]{State: EvidenceUnsupported, Reason: "typed-scalar-unavailable"}
		}
	}
	for i, a := range n.attributes {
		if err := s.step(ctx, 1); err != nil {
			return err
		}
		attributes[a.Name] = i
	}
	for result := range state.run.Attributes(binding) {
		if err := s.step(ctx, 1); err != nil {
			return err
		}
		if !result.Present {
			continue
		}
		index, exists := attributes[QName{Space: result.Name.Space, Local: result.Name.Local}]
		if !exists {
			continue
		}
		a := &n.attributes[index]
		declaration := state.active.declaration(result.Use)
		if validDeclaration(declaration) {
			a.Declaration = Evidence[DeclarationID]{State: EvidenceKnown, Value: declaration}
		} else {
			a.Declaration = Evidence[DeclarationID]{State: EvidenceUnsupported, Reason: "attribute-use-unavailable"}
		}
		if s.profile.Target != TargetModel {
			value, err := s.schemaAtomic(ctx, result.Scalar)
			if err != nil {
				return err
			}
			a.Scalar.Value = value
		}
	}

	return nil
}

// schemaAtomic bridges already assessed digit views, never reparses XML and
// never routes an exact source number through a host integer or float.
func (s *Session) schemaAtomic(ctx context.Context, value xsd.ScalarResult) (Evidence[Atomic], error) {
	unavailable := Evidence[Atomic]{State: EvidenceUnsupported, Reason: "typed-scalar-unavailable"}
	if !value.Known || !value.Complete {
		return unavailable, nil
	}
	atomic := Atomic{}
	switch value.Value.Domain {
	case xsd.StringDomain:
		atomic = Atomic{Kind: AtomicString, Text: value.Value.Text}
	case xsd.BooleanDomain:
		atomic = Atomic{Kind: AtomicBoolean, Text: "false"}
		if value.Value.Boolean {
			atomic.Text = "true"
		}
	case xsd.NumberDomain:
		number := value.Value.Number
		size := int64(max(1, len(number.Whole)))
		if len(number.Fraction) > 0 {
			size += 1 + int64(len(number.Fraction))
		}
		if number.Negative {
			size++
		}
		if err := checkExactSize(size); err != nil {
			return unavailable, err
		}
		if err := s.account.Check(BudgetScalarBytes, size); err != nil {
			return unavailable, err
		}
		if err := s.step(ctx, 1+size+size*(size/64)); err != nil {
			return unavailable, err
		}
		// Includes bounded big-number temporaries and their independently owned copy.
		if err := s.retain(512 + size*12); err != nil {
			return unavailable, err
		}
		whole := number.Whole
		if whole == "" {
			whole = "0"
		}
		text := whole
		if len(number.Fraction) > 0 {
			text += "." + number.Fraction
		}
		if number.Negative {
			text = "-" + text
		}
		if _, _, integer := xsd.IntegerLimits(value.Builtin); integer {
			exact, ok := new(big.Int).SetString(text, 10)
			if !ok {
				return unavailable, contract("schema integer", "invalid assessed digit view")
			}
			out, err := NewInteger(exact)
			if err != nil {
				return unavailable, err
			}
			atomic = Atomic{Kind: AtomicInteger, Integer: out}
		} else {
			exact, ok := new(big.Rat).SetString(text)
			if !ok {
				return unavailable, contract("schema decimal", "invalid assessed digit view")
			}
			out, err := NewDecimal(exact)
			if err != nil {
				return unavailable, err
			}
			atomic = Atomic{Kind: AtomicDecimal, Decimal: out}
		}
	default:
		return unavailable, nil
	}
	if err := s.retain(128 + atomicLogicalBytes(atomic)); err != nil {
		return unavailable, err
	}
	if atomic.Kind == AtomicString {
		atomic.Text = strings.Clone(atomic.Text)
	}
	return Evidence[Atomic]{State: EvidenceKnown, Value: atomic}, nil
}

func (s *Session) schemaResult(n *factNode, check XSDCheck) xsd.Result {
	state := s.schema
	bound := state.nodes[n.header.ID]
	if bound.skipped {
		return xsd.Result{Applicable: false, Valid: true, Complete: true}
	}
	if !bound.available {
		return xsd.Result{Applicable: true, Valid: true, Complete: false, Limitations: []xsd.Limitation{{Reason: string(bound.reason)}}}
	}
	results := state.run.Results(bound.binding)
	switch check {
	case XSDCheckBinding:
		return results.Binding
	case XSDCheckDeclaration:
		return results.Declaration
	case XSDCheckAttributes:
		return results.Attributes
	case XSDCheckScalar:
		return results.Scalar
	case xsdCheckRepresentable:
		if !bound.representable {
			return xsd.Result{Applicable: true, Valid: true, Complete: false, Limitations: []xsd.Limitation{{Reason: "representability-pending"}}}
		}
		return xsd.Result{Applicable: true, Valid: len(bound.unrepresentable) == 0, Complete: true, Issues: bound.unrepresentable}
	case XSDCheckContent:
		if s.profile.Target == TargetSource && n.closed && (n.end.Order.State != EvidenceKnown || n.end.Order.Value != OrderSource) {
			results.Content.Complete = false
			results.Content.Limitations = append(results.Content.Limitations, xsd.Limitation{Reason: "original-order-unavailable"})
		}
		return results.Content
	}
	return xsd.Result{}
}
func (s *Session) recordXSDProvedFailures(ctx context.Context, n *factNode) error {
	// These bounded, dependency-free rule lists were admitted and charged at
	// Begin. Looking up an already proved result must precede a new cancellation
	// or exhausted-work check; diagnostic/cache work remains checked afterwards.
	for _, check := range []XSDCheck{XSDCheckBinding, XSDCheckDeclaration, XSDCheckAttributes, XSDCheckScalar, XSDCheckContent, xsdCheckRepresentable} {
		result := s.schemaResult(n, check)
		if !result.Applicable || result.Valid {
			continue
		}
		for _, key := range s.schema.proofRules[s.schema.active.assembly][check] {
			if err := s.recordXSDResult(ctx, n, s.catalog.rules[key], result); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Session) evaluateXSD(ctx context.Context, n *factNode, r RuleDefinition, p Plan, final bool) error {
	if p.xsdCheck == XSDCheckIdentity {
		if !final {
			return nil
		}
		result := xsd.Result{Applicable: true, Valid: s.schema.identity.Valid, Complete: s.schema.identityReady && s.schema.identity.Complete, Issues: s.schema.identity.Issues}
		if s.schema.run == nil {
			result.Valid = true
		}
		if !result.Complete {
			result.Limitations = []xsd.Limitation{{Reason: "identity-scope-incomplete"}}
		}
		if !s.schema.identityReady && !result.Valid {
			result.Complete = true
		}

		return s.recordXSDResult(ctx, n, r, result)
	}
	return s.recordXSDResult(ctx, n, r, s.schemaResult(n, p.xsdCheck))
}
func (s *Session) recordXSDResult(ctx context.Context, n *factNode, r RuleDefinition, result xsd.Result) error {
	id := decisionID{r.Key, n.header.ID}
	existing, done := s.lookupDecision(id)
	previous := s.schema.recorded[id]
	d := Decision{Rule: r.Key, Subject: NodeRef{s.documentID, n.header.ID}, Outcome: OutcomePass}
	if !result.Applicable {
		d.Outcome = OutcomeNotApplicable
		d.Reason = "schema-operation-not-applicable"
	} else if !result.Valid {
		d.Outcome = OutcomeFail
		d.Reason = "xsd-constraint-violation"
	} else if !result.Complete {
		d.Outcome = OutcomeUnsupported
		d.Reason = "xsd-assessment-unavailable"
		if len(result.Limitations) > 0 {
			d.Reason = Code(result.Limitations[0].Reason)
		}
		// Missing document evidence remains Unknown even when an earlier
		// attribute contributed an unrelated unsupported operation. Select the
		// first Unknown limitation without weakening normative failure or cause
		// precedence, and meter the scan before publishing its outcome.
		for _, limitation := range result.Limitations {
			if err := s.step(ctx, 1); err != nil {
				return err
			}
			if limitation.Unknown {
				d.Outcome = OutcomeUnknown
				d.Reason = Code(limitation.Reason)
				break
			}
		}
		cause, err := s.schemaResultCause(ctx, n, s.catalog.plans[r.Predicate].xsdCheck)
		if err != nil {
			return err
		}
		if cause != nil {
			d.Outcome = OutcomeBlocked
			d.Cause = cause
		}
	}
	if done {
		// A previously proved failure can acquire further details and a permanent
		// incompleteness blocker, without counting the predicate a second time.
		if existing.Outcome != OutcomeFail || d.Outcome != OutcomeFail {
			return nil
		}
		if previous.complete && !result.Complete {
			s.accumulator.incomplete()
			s.accumulator.report.BlockingProblems++
		}
	} else if err := s.accumulator.recordCompositeOutcome(r, d, result.Complete); err != nil {
		return err
	}
	// The verdict above precedes any diagnostic work, allocation, cancellation or
	// cache retention. Display quotas cannot erase a proved normative violation.
	for i := previous.details; i < len(result.Issues); i++ {
		if s.accumulator.report.DiagnosticsTruncated {
			break
		}
		if err := s.step(ctx, 1); err != nil {
			return err
		}
		issue := result.Issues[i]
		location := n.header.Location
		owner := s.nodes[NodeID(issue.Subject)]
		if owner != nil {
			location = owner.header.Location
			if issue.Attribute != (xsd.QName{}) {
				for _, a := range owner.attributes {
					if err := s.step(ctx, 1); err != nil {
						return err
					}
					if a.Name == (QName{Space: issue.Attribute.Space, Local: issue.Attribute.Local}) {
						location = a.Location
						break
					}
				}
			}
		}
		detail := d
		detail.Reason = Code(issue.Constraint)
		detail.Actual = issue.Message
		if err := s.retain(int64(len(issue.RelatedSubjects)) * 128); err != nil {
			return err
		}
		for _, relatedID := range issue.RelatedSubjects {
			if err := s.step(ctx, 1); err != nil {
				return err
			}
			if related := s.nodes[NodeID(relatedID)]; related != nil {
				detail.Related = append(detail.Related, related.header.Location)
			}
		}
		if issue.EntitySequence != 0 && s.sourceEntities != nil {
			if err := s.step(ctx, 1); err != nil {
				return err
			}
			if origin := s.sourceEntities.origins[issue.EntitySequence]; origin.State == EvidenceKnown {
				if err := s.retain(128); err != nil {
					return err
				}
				detail.Related = append(detail.Related, origin.Value)
			}
		}
		if err := s.accumulator.storeDiagnostic(r, detail, location); err != nil {
			return err
		}
	}
	deferDTD := !done && len(result.Issues) == 0 && d.Outcome == OutcomeUnsupported && d.Cause == nil && d.Reason == "dtd-infoset-unavailable"
	if !done && len(result.Issues) == 0 && !deferDTD {
		if err := s.accumulator.storeDiagnostic(r, d, n.header.Location); err != nil {
			return err
		}
	}
	if err := s.retain(64); err != nil {
		if deferDTD {
			s.schema.dtdOmitted = true
		}
		return err
	}
	if s.schema.recorded == nil {
		s.schema.recorded = make(map[decisionID]schemaRecord)
	}
	s.schema.recorded[id] = schemaRecord{details: len(result.Issues), complete: result.Complete && (!done || previous.complete)}
	if done {
		return nil
	}
	if err := s.cacheDecision(r, n, d); err != nil {
		if deferDTD {
			s.schema.dtdOmitted = true
		}
		return err
	}
	if deferDTD {
		return s.deferSourceDTDSummary(r.Key, n.header.ID)
	}
	return nil
}

func (s *Session) deferSourceDTDSummary(key RuleKey, subject NodeID) error {
	state := s.schema
	if summary, ok := state.dtdSummaries[key]; ok {
		summary.count++
		state.dtdSummaries[key] = summary
		return nil
	}
	// This is optional diagnostic storage. Do not spend a diagnostic slot yet,
	// or turn failed summary admission into a semantic assessment interruption.
	if err := s.retain(schemaDTDSummaryBytes); err != nil {
		if errors.Is(err, ErrResourceLimit) {
			state.dtdOmitted = true
			return nil
		}
		return err
	}
	if state.dtdSummaries == nil {
		state.dtdSummaries = make(map[RuleKey]schemaDTDSummary)
	}
	state.dtdSummaries[key] = schemaDTDSummary{first: subject, count: 1}
	state.dtdOrder = append(state.dtdOrder, key)
	return nil
}

func (s *Session) flushSourceDTDSummaries() error {
	state := s.schema
	if state == nil {
		return nil
	}
	// Flush after ordinary findings, even on an interrupted Finish. No healthy
	// context is needed for this already bounded diagnostic-only operation.
	for _, key := range state.dtdOrder {
		if s.accumulator.report.DiagnosticsTruncated {
			break
		}
		summary := state.dtdSummaries[key]
		n := s.nodes[summary.first]
		if n == nil {
			state.dtdOmitted = true
			continue
		}
		d := Decision{Rule: key, Subject: NodeRef{s.documentID, summary.first}, Outcome: OutcomeUnsupported, Reason: "dtd-infoset-unavailable",
			Actual: "Document-wide limitation summary covers " + strconv.FormatUint(summary.count, 10) + " selected rule evaluations; location is the first summarized occurrence."}
		if err := s.accumulator.storeDiagnostic(s.catalog.rules[key], d, n.header.Location); err != nil {
			return err
		}
	}
	// Setting this earlier would suppress later ordinary failure storage.
	if state.dtdOmitted {
		s.accumulator.report.DiagnosticsTruncated = true
	}
	return nil
}

func (s *Session) recordXSDIdentityFailure(ctx context.Context) error {
	if s.schema.identity.Valid {
		return nil
	}
	for _, key := range s.schema.proofRules[s.schema.active.assembly][XSDCheckIdentity] {
		r := s.catalog.rules[key]
		p := s.catalog.plans[r.Predicate]
		if err := s.evaluateXSD(ctx, s.root, r, p, true); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) finalizeSourceSchema(ctx context.Context) error {
	state := s.schema
	if state.run == nil {
		return nil
	}
	identity, err := state.run.Finalize(ctx, s.inputComplete)
	state.identity = identity
	state.identityReady = true
	if proofErr := s.recordXSDIdentityFailure(ctx); proofErr != nil {
		return errors.Join(err, proofErr)
	}
	return err
}

// schemaCurrent borrows immutable canonical number components as values. It
// never creates an XML lexeme or feeds canonical digits to lexical facets.
func (s *Session) schemaCurrent(ctx context.Context, evidence Evidence[Atomic]) (xsd.CurrentScalar, error) {
	if err := s.step(ctx, 1); err != nil {
		return xsd.CurrentScalar{}, err
	}
	if evidence.State == EvidenceAbsent {
		return xsd.CurrentScalar{Absent: true}, nil
	}
	if evidence.State != EvidenceKnown {
		return xsd.CurrentScalar{}, nil
	}
	value := evidence.Value
	out := xsd.CurrentScalar{Known: true}
	switch value.Kind {
	case AtomicString:
		out.Value = xsd.SimpleValue{Domain: xsd.StringDomain, Text: value.Text}
	case AtomicBoolean:
		out.Value = xsd.SimpleValue{Domain: xsd.BooleanDomain, Boolean: value.Text == "true"}
	case AtomicInteger, AtomicDecimal:
		text := value.Integer.String()
		if value.Kind == AtomicDecimal {
			text = value.Decimal.String()
		}
		if err := s.account.Check(BudgetScalarBytes, int64(len(text))); err != nil {
			return xsd.CurrentScalar{}, err
		}
		if err := s.step(ctx, 1+int64(len(text))*2); err != nil {
			return xsd.CurrentScalar{}, err
		}
		number := xsd.Decimal{}
		if len(text) > 0 && text[0] == '-' {
			number.Negative = true
			text = text[1:]
		}
		number.Whole = text
		if index := strings.IndexByte(text, '.'); index >= 0 {
			number.Whole, number.Fraction = text[:index], text[index+1:]
		}
		number.Whole = strings.TrimLeft(number.Whole, "0")
		number.Fraction = strings.TrimRight(number.Fraction, "0")
		number.Negative = number.Negative && (number.Whole != "" || number.Fraction != "")
		out.Value = xsd.SimpleValue{Domain: xsd.NumberDomain, Number: number}
	default:
		return xsd.CurrentScalar{}, nil
	}
	return out, nil
}

func (s *Session) schemaResultCause(ctx context.Context, n *factNode, check XSDCheck) (*DecisionRef, error) {
	if err := s.step(ctx, 1); err != nil {
		return nil, err
	}
	if bound := s.schema.nodes[n.header.ID]; bound.cause != nil {
		return bound.cause, nil
	}
	if s.profile.Target != TargetModel {
		return nil, nil
	}
	switch check {
	case XSDCheckScalar, XSDCheckDeclaration:
		if n.end.Scalar.Value.State == EvidenceInvalid {
			return n.end.Scalar.Value.Cause, nil
		}
	case XSDCheckContent:
		if n.end.Order.State == EvidenceInvalid {
			return n.end.Order.Cause, nil
		}
		if n.end.ChildrenComplete.State == EvidenceInvalid {
			return n.end.ChildrenComplete.Cause, nil
		}
		if n.end.Scalar.Value.State == EvidenceInvalid {
			return n.end.Scalar.Value.Cause, nil
		}
	case XSDCheckAttributes:
		for _, attribute := range n.attributes {
			if err := s.step(ctx, 1); err != nil {
				return nil, err
			}
			if attribute.Scalar.Value.State == EvidenceInvalid {
				return attribute.Scalar.Value.Cause, nil
			}
		}
	}
	return nil, nil
}

// schemaEffectiveScalar exposes only the shared kernel's validated effective
// value. Current input remains separately available in the original fact slot.
func (s *Session) schemaEffectiveScalar(ctx context.Context, node NodeID, attribute QName) (Evidence[Atomic], error) {
	unknown := Evidence[Atomic]{State: EvidenceUnsupported, Reason: "schema-effective-value-unavailable"}
	if err := s.step(ctx, 1); err != nil {
		return unknown, err
	}
	if s.schema == nil || s.schema.run == nil {
		return unknown, nil
	}
	n := s.nodes[node]
	if n == nil {
		return unknown, nil
	}
	bound, ok := s.schema.nodes[node]
	if !ok || !bound.available {
		if bound.cause != nil {
			unknown.State = EvidenceInvalid
			unknown.Cause = bound.cause
		}
		return unknown, nil
	}
	if attribute == (QName{}) {
		if n.end.Scalar.Value.State == EvidenceInvalid {
			return *n.end.Scalar.Value, nil
		}
		result := s.schema.run.Results(bound.binding)
		if !result.Scalar.Valid {
			return s.schemaInvalidValue(ctx, n, XSDCheckScalar)
		}
		if !result.Declaration.Valid {
			return s.schemaInvalidValue(ctx, n, XSDCheckDeclaration)
		}
		return s.schemaAtomic(ctx, s.schema.run.Scalar(bound.binding))
	}
	for _, input := range n.attributes {
		if err := s.step(ctx, 1); err != nil {
			return unknown, err
		}
		if input.Name == attribute && input.Scalar.Value.State == EvidenceInvalid {
			return input.Scalar.Value, nil
		}
	}
	for result := range s.schema.run.Attributes(bound.binding) {
		if err := s.step(ctx, 1); err != nil {
			return unknown, err
		}
		if result.Name != (xsd.QName{Space: attribute.Space, Local: attribute.Local}) {
			continue
		}
		if result.Violation {
			return s.schemaInvalidValue(ctx, n, XSDCheckAttributes)
		}
		if !result.Present && !result.Scalar.Defaulted && result.Complete {
			return Evidence[Atomic]{State: EvidenceAbsent}, nil
		}
		if !result.TypeValid || !result.DeclarationValid {
			if result.Complete {
				return s.schemaInvalidValue(ctx, n, XSDCheckAttributes)
			}
			return unknown, nil
		}
		return s.schemaAtomic(ctx, result.Scalar)
	}
	return unknown, nil
}
func (s *Session) schemaInvalidValue(ctx context.Context, n *factNode, check XSDCheck) (Evidence[Atomic], error) {
	out := Evidence[Atomic]{State: EvidenceUnsupported, Reason: "schema-value-invalid"}
	for _, key := range s.schema.proofRules[s.schema.active.assembly][check] {
		if err := s.step(ctx, 1); err != nil {
			return out, err
		}
		if decision, ok := s.lookupDecision(decisionID{key, n.header.ID}); ok && decision.Outcome == OutcomeFail {
			if err := s.retain(64); err != nil {
				return out, err
			}
			out.State = EvidenceInvalid
			out.Cause = &DecisionRef{Rule: key, Subject: NodeRef{s.documentID, n.header.ID}}
			break
		}
	}
	return out, nil
}
func (s *Session) schemaResolveIdentity(ctx context.Context, node NodeID, attribute QName) (xsd.IdentityResolution, *DecisionRef, error) {
	value, err := s.schemaEffectiveScalar(ctx, node, attribute)
	if err != nil {
		return xsd.IdentityResolution{Status: xsd.IdentityIncomplete, Reason: "schema-reference-value-unavailable"}, nil, err
	}
	if value.State != EvidenceKnown || value.Value.Kind != AtomicString {
		return xsd.IdentityResolution{Status: xsd.IdentityIncomplete, Reason: "schema-reference-value-unavailable"}, value.Cause, nil
	}
	result, err := s.schema.run.ResolveIdentity(ctx, value.Value.Text)
	if err != nil {
		return result, nil, err
	}
	if result.Status != xsd.IdentityMissing && result.Status != xsd.IdentityAmbiguous {
		return result, nil, nil
	}
	// Only a proved failure for this key uses the document identity decision as
	// its cause. An unrelated ID/IDREF failure cannot suppress a unique target.
	invalid, err := s.schemaInvalidValue(ctx, s.root, XSDCheckIdentity)
	return result, invalid.Cause, err
}
