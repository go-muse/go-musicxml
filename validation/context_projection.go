package validation

import (
	"context"
	"strings"
)

// ContextProperty identifies one reviewed scalar fact or typed context payload.
// Values are a closed source-backed vocabulary, not runtime expression keys.
type ContextProperty string

// Compound context properties have explicit typed payloads.
const (
	ContextPropertySpan       ContextProperty = "span"
	ContextPropertyRepeat     ContextProperty = "graphical-repeat"
	ContextPropertyNavigation ContextProperty = "navigation"
	ContextPropertyTiming     ContextProperty = "timing"
)

// ContextPosition is an established encoded-score coordinate, not a playback
// time. UnprovedFrame preserves the timeline's bounded frame uncertainty.
type ContextPosition struct {
	Part, Measure  NodeRef
	MeasureOrdinal uint64
	Offset         Rational
	UnprovedFrame  bool
}

// ContextResult is a caller-owned projection of one selected context fact.
// At most one compound payload is populated. Scalar facts use Value; compound
// facts retain independent availability on their typed fields. Unavailable
// optional context does not itself assert a failed mandatory predicate.
// Like Report, these are editable snapshots, not tamper-proof certificates.
type ContextResult struct {
	Provider   RuleKey
	Subject    NodeRef
	Property   ContextProperty
	Value      Evidence[Atomic]
	Origins    []Location
	Span       *SpanContextResult
	Repeat     *RepeatContextResult
	Navigation *NavigationContextResult
	Layout     *LayoutContextResult
	Timing     *TimingContextResult
}

func validContextProperty(p ContextProperty) bool {
	switch p {
	case ContextPropertySpan, ContextPropertyRepeat, ContextPropertyNavigation, ContextPropertyTiming:
		return true
	}
	return contextLocalPropertyValid(p) || contextSpanPropertyValid(p) || contextRepeatPropertyValid(p) || contextPerformancePropertyValid(p)
}

const contextResultReserveBytes int64 = 1536

// publishContextResult is transactional with respect to visibility and live
// retention. Cumulative work/fact counters are never refunded on interruption.
func (s *Session) publishContextResult(ctx context.Context, in ContextResult) (err error) {
	if err = s.step(ctx, 1); err != nil {
		return err
	}
	if !catalogKey(in.Provider) || in.Subject.Node == 0 || in.Subject.DocumentID == "" || !validContextProperty(in.Property) {
		return contract("context projection", "exact provider, subject and closed property required")
	}
	payloads := 0
	for _, present := range []bool{in.Span != nil, in.Repeat != nil, in.Navigation != nil, in.Layout != nil, in.Timing != nil} {
		if present {
			payloads++
		}
	}
	if payloads > 1 {
		return contract("context projection", "one typed payload per result required")
	}
	expected := "scalar"
	switch in.Property {
	case ContextPropertySpan, ContextPropertySpanLyricSyllable:
		expected = "span"
	case ContextPropertyRepeat, ContextProperty("repeat-style"):
		expected = "repeat"
	case ContextPropertyNavigation, ContextProperty("navigation-traversal"):
		expected = "navigation"
	case ContextPropertyTiming:
		expected = "timing"
	default:
		if contextLocalPropertyValid(in.Property) {
			expected = "layout"
		}
	}
	matched := expected == "scalar" && payloads == 0 || expected == "span" && in.Span != nil || expected == "repeat" && in.Repeat != nil || expected == "navigation" && in.Navigation != nil || expected == "timing" && in.Timing != nil || expected == "layout" && in.Layout != nil
	if !matched {
		return contract("context projection", "property and typed payload must match")
	}

	if err = s.account.Charge(BudgetContextFacts, 1); err != nil {
		return err
	}
	before := s.retained
	defer func() {
		if err != nil {
			delta := s.retained - before
			if delta > 0 {
				_ = s.account.Release(BudgetBufferedBytes, delta)
				s.retained = before
			}
		}
	}()
	cost := contextResultReserveBytes + int64(len(in.Provider.ID)+len(in.Subject.DocumentID)+len(in.Property)) + contextAtomicBytes(in.Value.Value) + evidenceLogicalBytes(in.Value)
	if err = s.account.Check(BudgetScalarBytes, atomicLogicalBytes(in.Value.Value)); err != nil {
		return err
	}
	work := int64(16+len(in.Provider.ID)+len(in.Subject.DocumentID)+len(in.Property)) + atomicLogicalBytes(in.Value.Value) + evidenceLogicalBytes(in.Value)
	if err = s.step(ctx, work); err != nil {
		return err
	}
	if err = s.retain(cost); err != nil {
		return err
	}
	out := in
	out.Provider = cloneKey(in.Provider)
	out.Subject.DocumentID = strings.Clone(in.Subject.DocumentID)
	out.Property = ContextProperty(strings.Clone(string(in.Property)))
	out.Value = cloneEvidence(in.Value)
	out.Value.Value = cloneAtomic(in.Value.Value)
	originBytes, e := rationalCheckedMul(BudgetBufferedBytes, int64(len(in.Origins)), 384)
	if e != nil {
		return e
	}
	if err = s.retain(originBytes); err != nil {
		return err
	}
	for _, origin := range in.Origins {
		n := locationLogicalBytes(origin)
		if err = s.step(ctx, n); err != nil {
			return err
		}
		if err = s.retain(n); err != nil {
			return err
		}
	}
	if len(in.Origins) != 0 {
		out.Origins = make([]Location, len(in.Origins))
		for i, origin := range in.Origins {
			out.Origins[i] = cloneLocation(origin)
		}
	}
	if out.Span, err = cloneSpanContext(ctx, s, in.Span); err != nil {
		return err
	}
	if out.Repeat, err = cloneRepeatContext(ctx, s, in.Repeat); err != nil {
		return err
	}
	if out.Navigation, err = cloneNavigationContext(ctx, s, in.Navigation); err != nil {
		return err
	}
	if out.Layout, err = cloneLayoutContext(ctx, s, in.Layout); err != nil {
		return err
	}
	if out.Timing, err = cloneTimingContext(ctx, s, in.Timing); err != nil {
		return err
	}
	if err = s.step(ctx, 1); err != nil {
		return err
	}
	// The per-record reserve covers geometric slice backing growth and the
	// simultaneous old/new backing arrays; payload allocations were precharged.
	s.contextResults = append(s.contextResults, out)
	return nil
}

type contextService struct {
	Final       bool
	Timeline    Evidence[bool]
	Local       *contextLocalService
	Layout      *contextLayoutService
	Spans       *contextSpanService
	Repeats     *contextRepeatService
	Navigation  *contextNavigationService
	Performance *contextPerformanceService
}

func (s *Session) newContextService(ctx context.Context, key RuleKey) (*contextService, error) {
	if err := s.step(ctx, 1+int64(len(key.ID))); err != nil {
		return nil, err
	}
	if existing := s.contextServices[key]; existing != nil {
		return existing, nil
	}
	if err := s.retain(2048 + int64(len(key.ID))); err != nil {
		return nil, err
	}
	if s.contextServices == nil {
		s.contextServices = make(map[RuleKey]*contextService)
	}
	service := &contextService{Timeline: Evidence[bool]{State: EvidenceAbsent}}
	s.contextServices[cloneKey(key)] = service
	return service, nil
}
func (s *Session) evaluateContextPlan(ctx context.Context, n *factNode, key RuleKey, p Plan, final bool) (Decision, bool, error) {
	if p.kind == planGraphicalRepeatContent {
		return evaluateGraphicalRepeatContent(ctx, s, n, p.context, final)
	}
	if p.kind == planBendReleaseSign {
		return evaluateBendReleaseSign(ctx, s, n, p.context, final)
	}
	if p.kind == planFigureNumberNumeric {
		return evaluateFigureNumberNumeric(ctx, s, n)
	}
	if p.kind == planTimeOnlyOrder {
		return evaluateTimeOnlyOrder(ctx, s, n, p.context.Bindings)
	}
	if !final {
		return Decision{}, true, nil
	}
	service, err := s.newContextService(ctx, key)
	if err != nil {
		return Decision{}, false, err
	}
	if service.Final {
		return Decision{Outcome: OutcomePass}, false, nil
	}
	var d Decision
	var pending bool
	switch p.kind {
	case planContextLocal:
		d, pending, err = evaluateContextLocal(ctx, s, n, key, p.context, final)
	case planContextSpans:
		d, pending, err = evaluateContextSpans(ctx, s, n, key, p.context, final)
	case planContextRepeats:
		d, pending, err = evaluateContextRepeats(ctx, s, n, key, p.context, final)
	case planContextNavigation:
		d, pending, err = evaluateContextNavigation(ctx, s, n, key, p.context, final)
	case planContextLayout:
		d, pending, err = evaluateContextLayout(ctx, s, n, key, p.context, final)
	case planContextPerformance:
		d, pending, err = evaluateContextPerformance(ctx, s, n, key, p.context, final)
	default:
		return Decision{}, false, contract("context plan", "unknown sealed operation")
	}
	if err == nil && !pending && d.Outcome == OutcomePass {
		service.Final = true
	}
	return d, pending, err
}

// contextDependencies keeps the exact timeline ordering/closure edge but
// records unavailable timeline evidence for field-specific consumers. Only that
// sealed dependency is non-blocking; every unrelated prerequisite is unchanged.
func (s *Session) contextDependencies(ctx context.Context, n *factNode, r RuleDefinition, p Plan, final bool) (Decision, bool, error) {
	result := Decision{Outcome: OutcomePass}
	pending := false
	for _, dependency := range r.Requires {
		d, wait, err := s.dependency(ctx, n, dependency, final)
		if err != nil {
			return Decision{}, false, err
		}
		pending = pending || wait
		if dependency == (Dependency{Kind: DependencyRule, Rule: p.context.Timeline, Relation: RelationDocument}) && p.context.Timeline != (RuleKey{}) {
			if !wait {
				service, err := s.newContextService(ctx, r.Key)
				if err != nil {
					return Decision{}, false, err
				}
				e := Evidence[bool]{State: EvidenceKnown, Value: true, Reason: d.Reason, Cause: d.Cause}
				switch d.Outcome {
				case OutcomeUnknown, OutcomeNotApplicable:
					e.State = EvidenceUnknown
					e.Value = false
				case OutcomeUnsupported:
					e.State = EvidenceUnsupported
					e.Value = false
				case OutcomeBlocked, OutcomeFail:
					e.State = EvidenceInvalid
					e.Value = false
				}
				if err := s.step(ctx, 1+evidenceLogicalBytes(e)); err != nil {
					return Decision{}, false, err
				}
				if err := s.retain(128 + evidenceLogicalBytes(e)); err != nil {
					return Decision{}, false, err
				}
				service.Timeline = cloneEvidence(e)
			}
			continue
		}
		if dependencyOutcomeRank(d) > dependencyOutcomeRank(result) || (dependencyOutcomeRank(d) == dependencyOutcomeRank(result) && d.Outcome != OutcomePass && dependencyCauseLess(d, result)) {
			result = d
		}
	}
	if result.Outcome == OutcomeBlocked {
		return result, false, nil
	}
	if pending {
		return Decision{}, true, nil
	}
	return result, false, nil
}
