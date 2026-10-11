package validation

import (
	"context"
	"math/big"
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
}
type schemaNode struct {
	binding   xsd.Binding
	available bool
	reason    Code
	cause     *DecisionRef
}
type schemaMeter struct{ session *Session }

func (m schemaMeter) Step(ctx context.Context, n int64) error { return m.session.step(ctx, n) }
func (m schemaMeter) Reserve(n int64) error                   { return m.session.account.Charge(BudgetBufferedBytes, n) }
func (m schemaMeter) Release(n int64) error                   { return m.session.account.Release(BudgetBufferedBytes, n) }
func (m schemaMeter) Scalar(n int64) error                    { return m.session.account.Check(BudgetScalarBytes, n) }
func (m schemaMeter) Identity(n int64) error                  { return m.session.account.Charge(BudgetIDs, n) }

func (s *Session) schemaSubject(ctx context.Context, n *factNode) (xsd.Subject, error) {
	bytes := int64(256) + int64(len(n.header.Bindings))*64 + int64(len(n.attributes))*96 + int64(len(n.children))*64
	if err := s.retain(bytes); err != nil {
		return xsd.Subject{}, err
	}
	out := xsd.Subject{ID: xsd.SubjectID(n.header.ID), Parent: xsd.SubjectID(n.header.Parent), Name: xsd.QName{Space: n.header.Name.Space, Local: n.header.Name.Local}, Path: n.header.Location.Path, Text: n.end.Scalar.Lexeme.Value, TextKnown: n.closed && n.end.Scalar.Lexeme.State == EvidenceKnown, AttributesComplete: n.closed && n.end.AttributesComplete.State == EvidenceKnown && n.end.AttributesComplete.Value, ChildrenComplete: n.closed && n.end.ChildrenComplete.State == EvidenceKnown && n.end.ChildrenComplete.Value}
	out.Namespaces = make(map[string]string, len(n.header.Bindings))
	for _, b := range n.header.Bindings {
		if err := s.step(ctx, 1); err != nil {
			return out, err
		}
		out.Namespaces[b.Prefix] = b.URI
	}
	out.Attributes = make([]xsd.Attribute, len(n.attributes))
	for i, a := range n.attributes {
		if err := s.step(ctx, 1); err != nil {
			return out, err
		}
		out.Attributes[i] = xsd.Attribute{Attr: xsd.Attr{Name: xsd.QName{Space: a.Name.Space, Local: a.Name.Local}, Value: a.Scalar.Lexeme.Value}, ValueKnown: a.Scalar.Lexeme.State == EvidenceKnown, NamespaceDeclaration: a.Name.Space == "http://www.w3.org/2000/xmlns/"}
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
	if s.root.header.Declaration.State == EvidenceUnsupported || s.root.header.Type.State == EvidenceUnsupported {
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
	state.run, err = xsd.NewRun(state.active.kernel, schemaMeter{s})
	if err != nil {
		return err
	}
	rootUse, ok := state.active.kernel.Root(xsd.QName{Space: s.root.header.Name.Space, Local: s.root.header.Name.Local})
	if !ok {
		return contract("source schema root", "no compiled root use")
	}
	type frame struct {
		node      *factNode
		use       xsd.Use
		binding   xsd.Binding
		witnesses []xsd.Witness
		next      int
		children  []xsd.Completion
		entered   bool
	}
	if err := s.retain(256 * int64(len(s.nodes))); err != nil {
		return err
	}
	stack := []frame{{node: s.root, use: rootUse}}
	for len(stack) > 0 {
		if err := s.step(ctx, 1); err != nil {
			return err
		}
		f := &stack[len(stack)-1]
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
			if err := s.retain(int64(len(f.witnesses)) * 64); err != nil {
				return err
			}
			f.children = make([]xsd.Completion, 0, len(f.witnesses))
		}
		if f.next < len(f.witnesses) {
			witness := f.witnesses[f.next]
			f.next++
			child := s.nodes[NodeID(witness.Child)]
			if child == nil {
				return contract("schema witness", "unknown child occurrence")
			}
			stack = append(stack, frame{node: child, use: witness.Use})
			continue
		}
		completion, completeErr := state.run.Complete(ctx, f.binding, f.children)
		if err := s.recordXSDProvedFailures(ctx, f.node); err != nil {
			return err
		}
		if completeErr != nil {
			return completeErr
		}
		stack = stack[:len(stack)-1]
		if len(stack) > 0 {
			parent := &stack[len(stack)-1]
			parent.children = append(parent.children, completion)
		}
	}
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
		n.header.Type = Evidence[ComponentID]{State: EvidenceUnsupported, Reason: item.reason}
		n.header.Declaration = Evidence[DeclarationID]{State: EvidenceUnsupported, Reason: item.reason}
		if item.cause != nil {
			n.header.Type.State = EvidenceInvalid
			n.header.Type.Cause = item.cause
			n.header.Declaration.State = EvidenceInvalid
			n.header.Declaration.Cause = item.cause
		}
	}
	identity, identityErr := state.run.IdentityPreview(ctx)
	state.identity = identity
	if err := s.recordXSDIdentityFailure(ctx); err != nil {
		return err
	}
	return identityErr
}

func (s *Session) publishSchemaFacts(ctx context.Context, n *factNode, use xsd.Use, binding xsd.Binding) error {
	state := s.schema
	declaration := state.active.declaration(use)
	n.header.Declaration = Evidence[DeclarationID]{State: EvidenceKnown, Value: declaration}
	if kind, known := state.run.ActualType(binding); known {
		n.header.Type = Evidence[ComponentID]{State: EvidenceKnown, Value: state.active.component(kind)}
	} else {
		n.header.Type = Evidence[ComponentID]{State: EvidenceUnsupported, Reason: "actual-type-unavailable"}
	}
	scalar, err := s.schemaAtomic(ctx, state.run.Scalar(binding))
	if err != nil {
		return err
	}
	n.end.Scalar.Value = scalar
	for _, result := range state.run.Attributes(binding) {
		for i := range n.attributes {
			a := &n.attributes[i]
			if a.Name != (QName{Space: result.Name.Space, Local: result.Name.Local}) {
				continue
			}
			if result.Use.Occurrence() != "" {
				a.Declaration = Evidence[DeclarationID]{State: EvidenceKnown, Value: state.active.declaration(result.Use)}
			} else {
				a.Declaration = Evidence[DeclarationID]{State: EvidenceUnsupported, Reason: "attribute-use-unavailable"}
			}
			value, err := s.schemaAtomic(ctx, result.Scalar)
			if err != nil {
				return err
			}
			a.Scalar.Value = value
			break
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
		size := int64(len(number.Whole)) + int64(len(number.Fraction)) + 3
		if err := checkExactSize(size); err != nil {
			return unavailable, err
		}
		if err := s.account.Check(BudgetScalarBytes, size); err != nil {
			return unavailable, err
		}
		if err := s.step(ctx, 1+size); err != nil {
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
	case XSDCheckContent:
		return results.Content
	}
	return xsd.Result{}
}
func (s *Session) recordXSDProvedFailures(ctx context.Context, n *factNode) error {
	for _, key := range s.catalog.orders[s.profile.ID] {
		r := s.catalog.rules[key]
		p := s.catalog.plans[r.Predicate]
		if p.kind != planXSD || p.xsdCheck == XSDCheckIdentity || p.schema.assembly != s.schema.active.assembly || s.catalog.supports[s.profile.ID][key].State != ImplementationAvailable {
			continue
		}
		result := s.schemaResult(n, p.xsdCheck)
		if result.Applicable && !result.Valid {
			if err := s.recordXSDResult(ctx, n, r, result); err != nil {
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
		result := xsd.Result{Applicable: true, Valid: true, Complete: s.schema.identityReady && s.schema.identity.Complete}
		if !result.Complete {
			result.Limitations = []xsd.Limitation{{Reason: "identity-scope-incomplete"}}
		}
		for _, id := range s.sequence {
			if local, ok := s.schema.identity.BySubject[xsd.SubjectID(id)]; ok {
				result.Valid = result.Valid && local.Valid
				result.Issues = append(result.Issues, local.Issues...)
			}
		}
		return s.recordXSDResult(ctx, n, r, result)
	}
	return s.recordXSDResult(ctx, n, r, s.schemaResult(n, p.xsdCheck))
}
func (s *Session) recordXSDResult(ctx context.Context, n *factNode, r RuleDefinition, result xsd.Result) error {
	if _, done := s.lookupDecision(decisionID{r.Key, n.header.ID}); done {
		return nil
	}
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
		if bound := s.schema.nodes[n.header.ID]; bound.cause != nil {
			d.Outcome = OutcomeBlocked
			d.Cause = bound.cause
		}
	}
	if err := s.retain(int64(len(result.Issues)) * 384); err != nil {
		return err
	}
	details := make([]Finding, 0, len(result.Issues))
	for _, issue := range result.Issues {
		if err := s.step(ctx, 1); err != nil {
			return err
		}
		location := n.header.Location
		owner := s.nodes[NodeID(issue.Subject)]
		if owner != nil {
			location = owner.header.Location
			if issue.Attribute != (xsd.QName{}) {
				for _, a := range owner.attributes {
					if a.Name == (QName{Space: issue.Attribute.Space, Local: issue.Attribute.Local}) {
						location = a.Location
						break
					}
				}
			}
		}
		detail := Finding{Decision: d, Category: r.Category, Location: location}
		detail.Reason = Code(issue.Constraint)
		detail.Actual = issue.Message
		for _, id := range issue.RelatedSubjects {
			if related := s.nodes[NodeID(id)]; related != nil {
				detail.Related = append(detail.Related, related.header.Location)
			}
		}
		details = append(details, detail)
	}
	if err := s.accumulator.recordComposite(r, d, n.header.Location, details, result.Complete); err != nil {
		return err
	}
	return s.cacheDecision(r, n, d)
}
func (s *Session) recordXSDIdentityFailure(ctx context.Context) error {
	hasFailure := false
	for _, result := range s.schema.identity.BySubject {
		hasFailure = hasFailure || !result.Valid
	}
	if !hasFailure {
		return nil
	}
	for _, key := range s.catalog.orders[s.profile.ID] {
		r := s.catalog.rules[key]
		p := s.catalog.plans[r.Predicate]
		if p.kind == planXSD && p.xsdCheck == XSDCheckIdentity && p.schema.assembly == s.schema.active.assembly && s.catalog.supports[s.profile.ID][key].State == ImplementationAvailable {
			if err := s.evaluateXSD(ctx, s.root, r, p, true); err != nil {
				return err
			}
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
	return err
}
