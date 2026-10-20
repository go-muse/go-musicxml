package validation

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// RunOptions binds one occurrence to an explicit profile. Account nil creates
// an independent finite account. A supplied Account must be core-created and
// requires zero Budgets; child sessions cannot reset or widen its limits.
// DocumentID must be unique among occurrences in the caller's root assessment.
type RunOptions struct {
	DocumentID string
	Profile    ProfileID
	Budgets    Budgets
	Account    *Account
	// OmitContext leaves Report.Context empty. Providers compute and check
	// their context exactly as otherwise, and every decision, count and
	// finding is the same; the session only keeps no copy of the projection
	// for the report, which saves the storage it would hold. A resource
	// snapshot reads that projection, so FinishWithResourceSnapshot refuses
	// a session that omits it.
	OmitContext bool
}

// Engine shares only an immutable compiled catalog. Independent sessions may run
// concurrently; each Session itself is single-goroutine and must be finalized.
type Engine struct{ catalog *CompiledCatalog }

// NewEngine accepts only a successfully compiled, nonempty catalog.
func NewEngine(catalog *CompiledCatalog) (*Engine, error) {
	if catalog == nil || len(catalog.profiles) == 0 {
		return nil, contract("engine", "compiled catalog required")
	}
	return &Engine{catalog: catalog}, nil
}

// SessionError identifies an invalid fact event or lifecycle transition. It
// preserves a more specific contract or prior terminal cause when available.
type SessionError struct {
	Operation, Reason string
	Cause             error
}

// Error describes the rejected event without interpreting its cause.
func (e *SessionError) Error() string {
	if e == nil {
		return "validation: nil session error"
	}
	return joinErrorText(ErrInvalidSession.Error(), e.Operation, e.Reason, causeText(e.Cause))
}

// Unwrap preserves the lifecycle sentinel and the original cause.
func (e *SessionError) Unwrap() []error {
	if e == nil {
		return nil
	}
	if e.Cause == nil {
		return []error{ErrInvalidSession}
	}
	return []error{ErrInvalidSession, e.Cause}
}

// ExcludedRootError describes a known root excluded by the selected profile.
// It denotes a capability boundary, not malformed XML or normative invalidity.
// It is unrelated to musicxml.UnsupportedRootError, which reports an XML root
// element that the decoder cannot represent.
type ExcludedRootError struct {
	Name    QName
	Profile ProfileID
}

// Error identifies the excluded root and selected profile.
func (e *ExcludedRootError) Error() string {
	if e == nil {
		return "validation: nil excluded root error"
	}
	return fmt.Sprintf("validation: root {%s}%s is excluded by profile %s", e.Name.Space, e.Name.Local, e.Profile)
}

// Unwrap classifies unsupported roots as incomplete assessment.
func (e *ExcludedRootError) Unwrap() error { return ErrIncompleteAssessment }

type factNode struct {
	modelRepresentation *modelRepresentationRecord
	header              retainedHeader
	end                 retainedEnd
	closed              bool
	children            []*factNode
	attributes          []AttributeFact
	attributeSeal       *Evidence[bool]
}
type decisionID struct {
	rule RuleKey
	node NodeID
}

// Successful outcomes do not need diagnostic payloads. Zero is deliberately not
// a result: an absent entry must never be confused with proven non-applicability.
type successfulOutcome uint8

const (
	successfulPass successfulOutcome = iota + 1
	successfulNotApplicable
)

// A document prerequisite can be read only after its final evaluation phase.
// The first actual consumer computes the aggregate; excluded consumers do not
// cause a document scan merely because their catalog lists the prerequisite.
type documentPrerequisite struct {
	finalized, ready bool
	decision         Decision
}

// Session consumes validated facts synchronously. It retains a bounded fact set
// until Finish (not a streaming constant-memory promise). Every event error is
// terminal; call Finish exactly once even on cancellation to release live gauges.
// Reports own their contents after cleanup. The caller must serialize methods.
type Session struct {
	facts                   *retainedFactPool
	nodeCapacity            int
	commonModelSeal         *modelRepresentationRecord
	commonAttributeSeal     *Evidence[bool]
	dtdIDs                  *sourceDTDIDs
	sourceEntities          *sourceEntityDeclarations
	dictionaries            *dictionaryLease
	sourceOriginSeen        map[sourceOriginSubject]*SourceOriginContextResult
	sourceAttributeLexical  map[sourceOriginSubject]Evidence[string]
	sourceOriginExpected    uint64
	sourceOriginUncertain   bool
	sourceOriginUnavailable bool
	sourceOriginSeal        *Evidence[bool]
	contextServices         map[RuleKey]*contextService
	contextElementRoles     map[contextBoundElementKey][]contextRole
	contextAttributeTables  map[contextAttributeTableKey]*contextAttributeTable
	contextAttributeIntern  map[contextSharedInternKey]*contextAttributeTable
	// contextSharedTableBytes sums the charges of the shared attribute tables.
	contextSharedTableBytes int64
	contextResults          []ContextResult
	omitContext             bool
	// contextBackingHold keeps replaced Report.Context arrays charged while
	// a rollback snapshot may restore one; contextBackingHeld sums them.
	contextBackingHold      bool
	contextBackingHeld      int64
	contextReportRetained   int64
	timeline                map[RuleKey]*timelineService
	localDocuments          map[*localPlan]localDocumentResult
	schema                  *schemaSession
	catalog                 *CompiledCatalog
	documentID              string
	profile                 Profile
	account                 *Account
	accumulator             *reportAccumulator
	nodes                   map[NodeID]*factNode
	sequence                []NodeID
	stack                   []*factNode
	decisions               map[decisionID]Decision
	successful              map[RuleKey]map[NodeID]successfulOutcome
	documentPrerequisites   map[RuleKey]*documentPrerequisite
	contexts                map[ContextKey]ContextFact
	identities              map[string]*identityIndex
	registered              map[decisionID]bool
	terminalCause           error
	inputComplete, finished bool
	enumerationIncomplete   bool
	collectingIdentity      bool
	root                    *factNode
	rejectedRootName        QName
	retained, depth         int64

	attributeEnumerationIncomplete bool
}

// Begin validates configuration and cancellation before accepting input. It
// creates no parser, resolver or source/model adapter and chooses no default.
func (e *Engine) Begin(ctx context.Context, opts RunOptions) (*Session, error) {
	if ctx == nil {
		return nil, contract("context", "must not be nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, &IncompleteError{Reason: "canceled-before-begin", Cause: err}
	}
	if e == nil || e.catalog == nil {
		return nil, contract("engine", "uninitialized engine")
	}
	if !validID(opts.DocumentID) {
		return nil, contract("document ID", "explicit nonempty occurrence required")
	}
	profile, ok := e.catalog.profiles[opts.Profile]
	if !ok {
		return nil, contract("profile", "unknown or empty explicit profile")
	}
	account := opts.Account
	if account != nil {
		if opts.Budgets != (Budgets{}) {
			return nil, contract("budgets", "nonzero limits cannot accompany shared account")
		}
		if _, err := account.Usage(BudgetNodes); err != nil {
			return nil, err
		}
	} else {
		var err error
		account, err = NewBudgetAccount(opts.Budgets)
		if err != nil {
			return nil, err
		}
	}
	s := &Session{catalog: e.catalog, documentID: strings.Clone(opts.DocumentID), profile: profile, account: account, nodes: make(map[NodeID]*factNode), decisions: make(map[decisionID]Decision), contexts: make(map[ContextKey]ContextFact), identities: make(map[string]*identityIndex), registered: make(map[decisionID]bool), omitContext: opts.OmitContext}
	// Profile/report fixed metadata is finite at compile time and its per-run
	// copied payload is also reserved before the report snapshot is allocated.
	if err := s.retain(int64(1024+len(opts.DocumentID)) + profileLogicalBytes(profile)); err != nil {
		return nil, err
	}
	s.accumulator = newReportAccumulator(profile, account)
	if len(e.catalog.schemas[profile.ID]) != 0 {
		if err := s.retain(512); err != nil {
			s.cleanup()
			return nil, err
		}
		s.schema = &schemaSession{schemas: e.catalog.schemas[profile.ID], proofRules: make(map[Assembly]map[XSDCheck][]RuleKey)}
	}

	for _, key := range e.catalog.orders[profile.ID] {
		if err := s.step(ctx, 1); err != nil {
			s.cleanup()
			return nil, &IncompleteError{Reason: "session-initialization", Cause: err}
		}
		rule := e.catalog.rules[key]
		if p := e.catalog.plans[rule.Predicate]; s.schema != nil && p.kind == planXSD && len(rule.Requires) == 0 && e.catalog.supports[profile.ID][key].State == ImplementationAvailable {
			if err := s.retain(384); err != nil {
				s.cleanup()
				return nil, err
			}
			groups := s.schema.proofRules[p.schema.assembly]
			if groups == nil {
				groups = make(map[XSDCheck][]RuleKey)
				s.schema.proofRules[p.schema.assembly] = groups
			}
			groups[p.xsdCheck] = append(groups[p.xsdCheck], key)
		}
		for _, dependency := range rule.Requires {
			if err := s.step(ctx, 1); err != nil {
				s.cleanup()
				return nil, &IncompleteError{Reason: "session-initialization", Cause: err}
			}
			if dependency.Kind != DependencyRule || dependency.Relation != RelationDocument || s.documentPrerequisites[dependency.Rule] != nil {
				continue
			}
			// Includes the map slot, aggregate Decision and optional causal ref.
			// Aggregates borrow immutable strings from the owned decision cache.
			if err := s.retain(int64(512 + len(dependency.Rule.ID))); err != nil {
				s.cleanup()
				return nil, err
			}
			if s.documentPrerequisites == nil {
				s.documentPrerequisites = make(map[RuleKey]*documentPrerequisite)
			}
			s.documentPrerequisites[dependency.Rule] = &documentPrerequisite{}
		}
		if plan, ok := e.catalog.plans[rule.Predicate]; ok && plan.kind == planIdentityDeclare {
			if err := s.markIdentityProvider(plan.key); err != nil {
				s.cleanup()
				return nil, err
			}
			if e.catalog.supports[profile.ID][key].State != ImplementationAvailable {
				s.markIdentityIncomplete(plan.key)
			}
		}
	}
	return s, nil
}
func (s *Session) event(ctx context.Context, op string) error {
	if s == nil {
		return &SessionError{Operation: op, Reason: "nil session"}
	}
	if s.finished {
		return &SessionError{Operation: op, Reason: "session already finished"}
	}
	if s.catalog == nil || s.account == nil || s.accumulator == nil {
		return &SessionError{Operation: op, Reason: "uninitialized session"}
	}
	if s.terminalCause != nil {
		return &SessionError{Operation: op, Reason: "session has a terminal failure", Cause: s.terminalCause}
	}
	if ctx == nil {
		return s.fail(contract("context", "must not be nil"))
	}
	if err := ctx.Err(); err != nil {
		return s.fail(err)
	}
	if err := s.account.Charge(BudgetEvents, 1); err != nil {
		return s.fail(err)
	}
	return nil
}
func (s *Session) fail(err error) error {
	if err == nil {
		return nil
	}
	if s.terminalCause == nil {
		s.terminalCause = err
	}
	return err
}
func (s *Session) step(ctx context.Context, amount int64) error {
	if ctx == nil {
		return contract("context", "must not be nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.account.Charge(BudgetRuleSteps, amount)
}
func (s *Session) retain(amount int64) error {
	if err := s.account.Charge(BudgetBufferedBytes, amount); err != nil {
		return err
	}
	s.retained += amount
	return nil
}

// OpenNode starts one balanced occurrence and copies retained headers and
// bindings. It is an XML node event paired with CloseNode.
func (s *Session) OpenNode(ctx context.Context, node NodeHeader) error {
	if err := s.event(ctx, "OpenNode"); err != nil {
		return err
	}
	if len(s.stack) != 0 && s.stack[len(s.stack)-1].modelRepresentation != nil {
		return s.fail(&SessionError{Operation: "OpenNode", Reason: "child after model representation seal"})
	}
	if len(node.Bindings) > 65536 {
		return s.fail(contract("bindings", "too many bindings"))
	}
	if err := s.account.Charge(BudgetNodes, 1); err != nil {
		return s.fail(err)
	}
	if err := s.account.Charge(BudgetDepth, 1); err != nil {
		return s.fail(err)
	}
	s.depth++
	if err := s.validateRetainedHeader(node); err != nil {
		if errors.Is(err, ErrResourceLimit) {
			return s.fail(err)
		}
		return s.fail(&SessionError{Operation: "OpenNode", Reason: "invalid node header", Cause: err})
	}
	if _, exists := s.nodes[node.ID]; exists {
		return s.fail(&SessionError{Operation: "OpenNode", Reason: "duplicate occurrence ID"})
	}
	if len(s.stack) == 0 {
		if s.root != nil || node.Parent != 0 {
			return s.fail(&SessionError{Operation: "OpenNode", Reason: "duplicate root or nonzero root parent"})
		}
		found := false
		for _, name := range s.profile.SupportedRoots {
			if name == node.Name {
				found = true
				break
			}
		}
		if !found {
			// A rejected root has no accepted node lease. Admit its independently
			// retained name before copying, without inventing schema evidence.
			size, err := retainedFactSize(32, int64(len(node.Name.Space)), int64(len(node.Name.Local)))
			if err != nil {
				return s.fail(err)
			}
			if err := s.retain(size); err != nil {
				return s.fail(err)
			}
			s.rejectedRootName = QName{Space: strings.Clone(node.Name.Space), Local: strings.Clone(node.Name.Local)}
			return s.fail(&ExcludedRootError{Name: node.Name, Profile: s.profile.ID})
		}
	} else if node.Parent != s.stack[len(s.stack)-1].header.ID {
		return s.fail(&SessionError{Operation: "OpenNode", Reason: "parent is not current open occurrence"})
	}
	if len(s.stack) == 0 && s.schema != nil {
		for _, schema := range s.schema.schemas {
			if _, ok := schema.Root(node.Name); ok {
				s.schema.active = schema
				break
			}
		}
		if s.schema.active == nil {
			return s.fail(contract("source schema root", "no selected compiled schema"))
		}
	}
	header, err := s.retainFactHeader(ctx, node)
	if err != nil {
		return s.fail(err)
	}
	if err = s.growFactNodes(ctx); err != nil {
		return s.fail(err)
	}
	if s.sequence, err = growFactSlice(ctx, s, s.sequence); err != nil {
		return s.fail(err)
	}
	if s.stack, err = growFactSlice(ctx, s, s.stack); err != nil {
		return s.fail(err)
	}
	if len(s.stack) > 0 {
		parent := s.stack[len(s.stack)-1]
		if parent.children, err = growFactSlice(ctx, s, parent.children); err != nil {
			return s.fail(err)
		}
	}
	retained := &factNode{header: header, end: s.zeroEnd()}
	if len(s.stack) == 0 {
		s.root = retained
	} else {
		parent := s.stack[len(s.stack)-1]
		parent.children = append(parent.children, retained)
	}
	s.nodes[node.ID] = retained
	s.sequence = append(s.sequence, node.ID)
	s.stack = append(s.stack, retained)
	if s.sourceOriginKey() != (RuleKey{}) {
		s.sourceOriginExpected++
	}
	return nil
}

// Attribute supplies one exact slot before any child on its currently open owner.
func (s *Session) Attribute(ctx context.Context, attribute AttributeFact) error {
	if err := s.event(ctx, "Attribute"); err != nil {
		return err
	}
	if len(s.stack) == 0 {
		return s.fail(&SessionError{Operation: "Attribute", Reason: "no open owner"})
	}
	n := s.stack[len(s.stack)-1]
	if attribute.Owner != n.header.ID || len(n.children) > 0 || n.attributeSeal != nil || n.modelRepresentation != nil {
		return s.fail(&SessionError{Operation: "Attribute", Reason: "wrong owner or attribute after children"})
	}
	if err := s.account.Charge(BudgetAttributes, 1); err != nil {
		return s.fail(err)
	}
	if err := s.checkScalar(attribute.Scalar); err != nil {
		return s.fail(err)
	}
	if err := s.retain(attributeLogicalBytes(attribute)); err != nil {
		return s.fail(err)
	}
	if err := validateAttribute(attribute, s.profile.Target, s.documentID); err != nil {
		return s.fail(&SessionError{Operation: "Attribute", Reason: "invalid attribute", Cause: err})
	}
	for _, existing := range n.attributes {
		if err := s.step(ctx, 1); err != nil {
			return s.fail(err)
		}
		if existing.Name == attribute.Name {
			return s.fail(&SessionError{Operation: "Attribute", Reason: "duplicate expanded attribute slot"})
		}
	}
	n.attributes = append(n.attributes, cloneAttribute(attribute))
	if s.sourceOriginKey() != (RuleKey{}) {
		if attribute.Presence.State == EvidenceKnown && attribute.Presence.Value {
			s.sourceOriginExpected++
		} else if attribute.Presence.State != EvidenceAbsent {
			s.sourceOriginUncertain = true
		}
	}
	return nil
}

// CompleteAttributes optionally seals one start-tag attribute phase before
// children or node closure. It preserves explicit enumeration evidence for an
// interrupted parent; unknown evidence never establishes absence. Once called,
// no further Attribute event is allowed on this occurrence. Existing streams
// may omit this event and supply completeness only in NodeEnd.
func (s *Session) CompleteAttributes(ctx context.Context, id NodeID, complete Evidence[bool]) error {
	if err := s.event(ctx, "CompleteAttributes"); err != nil {
		return err
	}
	if len(s.stack) == 0 {
		return s.fail(&SessionError{Operation: "CompleteAttributes", Reason: "no open owner"})
	}
	n := s.stack[len(s.stack)-1]
	if n.header.ID != id || len(n.children) != 0 || n.attributeSeal != nil || n.modelRepresentation != nil {
		return s.fail(&SessionError{Operation: "CompleteAttributes", Reason: "wrong owner, repeated closure, or children already started"})
	}
	if err := validateEvidence(complete, "attribute completeness"); err != nil {
		return s.fail(err)
	}
	if canonicalComplete(complete) && s.commonAttributeSeal != nil {
		n.attributeSeal = s.commonAttributeSeal
		return nil
	}
	if err := s.retain(64 + evidenceLogicalBytes(complete)); err != nil {
		return s.fail(err)
	}
	owned := cloneEvidence(complete)
	n.attributeSeal = &owned
	if canonicalComplete(complete) {
		s.commonAttributeSeal = n.attributeSeal
	}
	return nil
}

// CloseNode seals one occurrence. Ordinary predicate failures are retained
// findings, not method errors. Independent local decisions are established
// before EOF. Finish (or one of its variants) ends the session and releases its
// storage.
func (s *Session) CloseNode(ctx context.Context, end NodeEnd) error {
	if err := s.event(ctx, "CloseNode"); err != nil {
		return err
	}
	if len(s.stack) == 0 || s.stack[len(s.stack)-1].header.ID != end.ID {
		return s.fail(&SessionError{Operation: "CloseNode", Reason: "unbalanced close"})
	}
	n := s.stack[len(s.stack)-1]
	if err := s.checkScalar(end.Scalar); err != nil {
		return s.fail(err)
	}
	if err := validateEnd(end, s.profile.Target); err != nil {
		return s.fail(&SessionError{Operation: "CloseNode", Reason: "invalid closure facts", Cause: err})
	}
	if err := s.validateModelRepresentationEnd(ctx, n, end); err != nil {
		return s.fail(err)
	}
	if end.ChildrenComplete.State != EvidenceKnown || !end.ChildrenComplete.Value {
		s.enumerationIncomplete = true
		for name := range s.identities {
			s.markIdentityIncomplete(name)
		}
	}
	if n.attributeSeal != nil && n.attributeSeal.State == EvidenceKnown && n.attributeSeal.Value && (end.AttributesComplete.State != EvidenceKnown || !end.AttributesComplete.Value) {
		return s.fail(&SessionError{Operation: "CloseNode", Reason: "attribute completeness contradicts sealed start tag"})
	}
	if end.AttributesComplete.State != EvidenceKnown || !end.AttributesComplete.Value {
		s.attributeEnumerationIncomplete = true
	}
	ownedEnd, err := s.retainFactEnd(end)
	if err != nil {
		return s.fail(err)
	}
	n.end = ownedEnd
	n.closed = true
	s.stack = s.stack[:len(s.stack)-1]
	if err := s.account.Release(BudgetDepth, 1); err != nil {
		return s.fail(err)
	}
	s.depth--
	// Model evidence establishes its real verdict before schema binding can
	// consume affected scalar/attribute/order facts or cache dependent results.
	for _, key := range s.catalog.modelOrders[s.profile.ID] {
		if err := s.evaluate(ctx, n, key, false); err != nil {
			return s.fail(err)
		}
	}
	if s.schema != nil && !s.schema.bound {
		if n != s.root {
			return nil
		}
		if err := s.bindSourceSchema(ctx); err != nil {
			return s.fail(err)
		}
		// Binding traverses every closed source node before any exact selector
		// can cache an unavailable result. Evaluate in schema witness order.
		for _, id := range s.sequence {
			for _, key := range s.catalog.orders[s.profile.ID] {
				if err := s.evaluate(ctx, s.nodes[id], key, false); err != nil {
					return s.fail(err)
				}
			}
		}
		return nil
	}
	for _, key := range s.catalog.orders[s.profile.ID] {
		if err := s.evaluate(ctx, n, key, false); err != nil {
			return s.fail(err)
		}
	}
	return nil
}

// Finish finalizes pending document obligations, returns the best report and
// always frees owned live gauges. Evidence-only unknown/unsupported outcomes and
// normative violations have nil operation errors; terminal failures return an
// IncompleteError preserving actual causes. A second Finish returns no report.
func (s *Session) Finish(ctx context.Context, end Completion) (Report, error) {
	report, _, err := s.finishResource(ctx, end, false)
	return report, err
}

func (s *Session) finishResource(ctx context.Context, end Completion, snapshot bool) (Report, *ResourceSnapshot, error) {
	return s.finishWithOwnership(ctx, end, snapshot, nil)
}

func (s *Session) finishWithOwnership(ctx context.Context, end Completion, snapshot bool, reportLease **ReportLease) (Report, *ResourceSnapshot, error) {
	if s == nil {
		return Report{}, nil, &SessionError{Operation: "Finish", Reason: "nil session"}
	}
	if s.finished {
		return Report{}, nil, &SessionError{Operation: "Finish", Reason: "session already finished"}
	}
	if s.catalog == nil || s.account == nil || s.accumulator == nil {
		return Report{}, nil, &SessionError{Operation: "Finish", Reason: "uninitialized session"}
	}
	s.finished = true
	eventsValid := s.terminalCause == nil
	defer s.cleanup()
	addCause := func(err error) {
		if err == nil {
			return
		}
		if s.terminalCause == nil {
			s.terminalCause = err
		} else if !errors.Is(s.terminalCause, err) {
			s.terminalCause = errors.Join(s.terminalCause, err)
		}
	}
	if ctx == nil {
		addCause(contract("context", "must not be nil"))
	} else {
		addCause(ctx.Err())
	}
	addCause(end.Cause)
	completionErr := s.validateCompletion(end)
	addCause(completionErr)
	malformedSource := completionErr == nil && s.profile.Target == TargetSource && end.Syntax == SyntaxMalformed && end.Cause != nil
	// A valid retained prefix can still prove independent violations after an
	// external parse or I/O failure. Never resume work after cancellation, a
	// rejected fact event, or exhausted resources.
	if eventsValid && completionErr == nil && ctx != nil && ctx.Err() == nil && s.schema != nil && !s.schema.bound && s.root != nil && !errors.Is(end.Cause, ErrResourceLimit) {
		addCause(s.bindSourceSchema(ctx))
	}
	s.inputComplete = end.InputComplete && len(s.stack) == 0 && s.root != nil && end.Syntax != SyntaxMalformed
	if s.terminalCause == nil && s.schema != nil && s.schema.bound {
		addCause(s.finalizeSourceSchema(ctx))
	}
	if s.terminalCause == nil {
		for _, phase := range s.catalog.phases[s.profile.ID] {
			s.collectingIdentity = phase.registration
			for _, id := range s.sequence {
				n := s.nodes[id]
				if !n.closed {
					continue
				}
				if err := s.evaluate(ctx, n, phase.rule, true); err != nil {
					addCause(err)
					break
				}
			}
			if s.terminalCause != nil {
				break
			}
			if !phase.registration {
				if prerequisite := s.documentPrerequisites[phase.rule]; prerequisite != nil {
					prerequisite.finalized = true
				}
			}
		}
	}
	addCause(s.flushSourceDTDSummaries())
	if !s.inputComplete || !reportSyntaxComplete(s.profile.Target, end.Syntax) || s.terminalCause != nil || s.enumerationIncomplete {
		s.accumulator.incomplete()
	}
	if s.terminalCause != nil {
		category, reason := terminalClassification(s.terminalCause, malformedSource)
		s.accumulator.terminal(s.terminalCause, category, reason)
	}
	report := s.accumulator.finish(s.inputComplete, end.Syntax)
	report.Context = s.contextResults
	var resource *ResourceSnapshot
	var ownershipErr error
	if snapshot {
		resource, ownershipErr = s.makeResourceSnapshot(ctx, report)
	} else if reportLease != nil {
		*reportLease, ownershipErr = s.retainFinishedReport()
	}
	if ownershipErr != nil {
		addCause(ownershipErr)
		report.AssessmentComplete = false
		if report.Counts.Fail == 0 {
			report.Conformance = ConformanceUndetermined
		}
		if report.Terminal == nil {
			category, reason := terminalClassification(ownershipErr, malformedSource)
			report.Terminal = &TerminalSummary{Category: category, Reason: reason}
			report.BlockingProblems++
		}
	}
	if s.terminalCause != nil {
		return report, resource, &IncompleteError{Reason: "session-terminated", Cause: s.terminalCause}
	}
	return report, resource, nil
}
func (s *Session) validateCompletion(end Completion) error {
	if s.profile.Target == TargetSource {
		if end.Syntax != SyntaxWellFormed && end.Syntax != SyntaxMalformed && end.Syntax != SyntaxIncomplete {
			return contract("completion syntax", "source syntax assertion required")
		}
		if end.InputComplete && end.Syntax == SyntaxMalformed {
			return contract("completion syntax", "malformed source cannot assert complete input")
		}
	} else if end.Syntax != SyntaxNotAssessed {
		return contract("completion syntax", "model/package syntax must be not-assessed")
	}
	if end.InputComplete && (s.root == nil || len(s.stack) != 0) {
		return &SessionError{Operation: "Finish", Reason: "complete input requires one closed root"}
	}
	if end.Syntax == SyntaxMalformed && end.Cause == nil && s.terminalCause == nil {
		return &SessionError{Operation: "Finish", Reason: "malformed source requires a syntax cause"}
	}
	return nil
}

// Classify independently of error wrapping order. Resource interruption dominates
// malformed syntax; lifecycle wrappers dominate their invalid-argument causes.
// The report is a bounded summary; Finish retains the entire actual error chain.
func terminalClassification(err error, malformedSource bool) (DiagnosticCategory, Code) {
	switch {
	case errors.Is(err, ErrResourceLimit):
		return CategoryResource, "resource-limit"
	case errors.Is(err, context.Canceled):
		return CategoryResource, "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return CategoryResource, "deadline-exceeded"
	case malformedSource:
		return CategorySyntax, "malformed-source"
	case errors.Is(err, ErrProcessingPolicy):
		return CategoryPolicy, "processing-policy"
	}
	var root *ExcludedRootError
	if errors.As(err, &root) {
		return CategoryCapability, "unsupported-root"
	}
	if errors.Is(err, ErrInvalidSession) {
		return CategoryCapability, "invalid-session"
	}
	if errors.Is(err, ErrInvalidConfiguration) {
		return CategoryCapability, "invalid-configuration"
	}
	return CategoryCapability, "assessment-interrupted"
}
func (s *Session) cleanup() {
	if lease := s.dictionaries; lease != nil {
		s.dictionaries = nil
		lease.release()
	}
	s.sourceOriginSeen = nil
	s.sourceAttributeLexical = nil
	s.sourceOriginSeal = nil
	s.sourceOriginExpected = 0
	s.sourceOriginUncertain = false
	s.sourceOriginUnavailable = false
	// Drop borrowed schema bindings before closing their run and releasing the
	// session-owned cache allowance below.
	s.contextElementRoles = nil
	s.contextAttributeTables = nil
	s.contextAttributeIntern = nil
	s.contextSharedTableBytes = 0
	if s.schema != nil && s.schema.run != nil {
		_ = s.schema.run.Close()
	}
	s.schema = nil
	s.sourceEntities = nil
	s.dtdIDs = nil
	s.localDocuments = nil
	s.timeline = nil
	s.contextServices = nil
	s.contextResults = nil
	s.contextReportRetained = 0
	if s.depth > 0 {
		_ = s.account.Release(BudgetDepth, s.depth)
		s.depth = 0
	}
	if s.retained > 0 {
		_ = s.account.Release(BudgetBufferedBytes, s.retained)
		s.retained = 0
	}
	s.nodes = nil
	s.facts = nil
	s.nodeCapacity = 0
	s.sequence = nil
	s.stack = nil
	s.root = nil
	s.commonModelSeal = nil
	s.commonAttributeSeal = nil
	s.rejectedRootName = QName{}
	s.decisions = nil
	s.successful = nil
	s.documentPrerequisites = nil
	s.contexts = nil
	s.identities = nil
	s.registered = nil
	s.accumulator = nil
}
func (s *Session) checkScalar(v Scalar) error {
	if err := s.account.Check(BudgetScalarBytes, int64(len(v.Lexeme.Value))); err != nil {
		return err
	}
	return s.account.Check(BudgetScalarBytes, atomicLogicalBytes(v.Value.Value))
}
func atomicLogicalBytes(v Atomic) int64 {
	return int64(len(v.Text)+len(v.Name.Space)+len(v.Name.Local)) + v.Integer.ByteSize() + v.Decimal.ByteSize()
}
func evidenceLogicalBytes[T any](v Evidence[T]) int64 {
	n := int64(len(v.Reason))
	if v.Cause != nil {
		n += int64(128 + len(v.Cause.Rule.ID) + len(v.Cause.Subject.DocumentID))
	}
	return n
}
func locationLogicalBytes(v Location) int64 {
	n := int64(96 + len(v.DocumentID) + len(v.Entry) + len(v.Path))
	if v.Position != nil {
		n += int64(64 + len(v.Position.CoordinateSpace))
	}
	return n
}
func scalarLogicalBytes(v Scalar) int64 {
	return 192 + int64(len(v.Lexeme.Value)) + atomicLogicalBytes(v.Value.Value) + evidenceLogicalBytes(v.Lexeme) + evidenceLogicalBytes(v.Value)
}
func attributeLogicalBytes(v AttributeFact) int64 {
	return 256 + int64(len(v.Name.Space)+len(v.Name.Local)+len(v.Declaration.Value.OwnerOccurrence)+len(v.Declaration.Value.Occurrence)) + locationLogicalBytes(v.Location) + scalarLogicalBytes(v.Scalar) + evidenceLogicalBytes(v.Declaration) + evidenceLogicalBytes(v.Presence)
}
func profileLogicalBytes(v Profile) int64 {
	n := int64(512 + len(v.ID) + len(v.MusicXMLVersion) + len(v.CatalogDigest) + len(v.RegistryCommit) + len(v.NormativeClosure))
	for _, k := range v.Required {
		n += int64(64 + len(k.ID))
	}
	for _, s := range v.Support {
		n += int64(96 + len(s.Rule.ID) + len(s.Reason))
	}
	for _, r := range v.SupportedRoots {
		n += int64(64 + len(r.Local) + len(r.Space))
	}
	return n
}
