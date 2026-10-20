package validation

import (
	"context"
	"reflect"
	"strings"
	"unsafe"
)

// ContextProperty identifies one reviewed scalar fact or typed context payload.
// Values are a closed source-backed vocabulary, not runtime expression keys.
type ContextProperty string

// Compound context properties have explicit typed payloads.
const (
	ContextPropertyContainerMetadata ContextProperty = "container.metadata"
	ContextPropertySoundMapping      ContextProperty = "sounds.mapping"
	ContextPropertySpan              ContextProperty = "span"
	ContextPropertyRepeat            ContextProperty = "graphical-repeat"
	ContextPropertyNavigation        ContextProperty = "navigation"
	ContextPropertyTiming            ContextProperty = "timing"
)

// ContextPosition is an established encoded-score coordinate, not a playback
// time. UnprovedFrame preserves the timeline's bounded frame uncertainty.
//
// ContextPosition is not comparable with ==; compare Offset with
// Rational.Equal or Rational.Cmp and the other fields directly.
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
	// New payload families omit nil to preserve the eight original profiles
	// byte-for-byte; the historical payload fields retain explicit JSON nulls.
	SoundMapping *SoundMappingContextResult `json:",omitempty"`
	Resource     *ResourceContextResult     `json:",omitempty"`
	Container    *ContainerContextResult    `json:",omitempty"`
	SourceOrigin *SourceOriginContextResult `json:",omitempty"`
	Dictionary   *DictionaryContextResult   `json:",omitempty"`
}

func validContextProperty(p ContextProperty) bool {
	switch p {
	case ContextPropertySourceOrigin, ContextPropertyContainerMetadata, ContextPropertySoundMapping, ContextPropertySpan, ContextPropertyRepeat, ContextPropertyNavigation, ContextPropertyTiming:
		return true
	}
	return dictionaryPropertyValid(p) || resourcePropertyValid(p) || contextLocalPropertyValid(p) || contextSpanPropertyValid(p) || contextRepeatPropertyValid(p) || contextPerformancePropertyValid(p)
}

// contextResultBytes is one ContextResult record in Report.Context. The
// backing array is accounted exactly by growContextResults.
var contextResultBytes = int64(unsafe.Sizeof(ContextResult{}))

// publishContextResult is transactional with respect to visibility and live
// retention. Cumulative work/fact counters are never refunded on interruption.
func (s *Session) publishContextResult(ctx context.Context, in ContextResult) error {
	if err := s.validateContextResult(ctx, in); err != nil {
		return err
	}
	return s.publishValidatedContextResult(ctx, in)
}

// validateContextResult performs all payload validation before any publication.
// SourceOrigin also uses this gate before checking the admitted occurrence site.
func (s *Session) validateContextResult(ctx context.Context, in ContextResult) (err error) {
	if err = s.step(ctx, 1); err != nil {
		return err
	}
	if !catalogKey(in.Provider) || in.Subject.Node == 0 || in.Subject.DocumentID == "" || !validContextProperty(in.Property) {
		return contract("context projection", "exact provider, subject and closed property required")
	}
	payloads := 0
	for _, present := range []bool{in.Span != nil, in.Repeat != nil, in.Navigation != nil, in.Layout != nil, in.Timing != nil, in.SoundMapping != nil, in.Resource != nil, in.Container != nil, in.SourceOrigin != nil, in.Dictionary != nil} {
		if present {
			payloads++
		}
	}
	if payloads > 1 {
		return contract("context projection", "one typed payload per result required")
	}
	expected := "scalar"
	switch in.Property {
	case ContextPropertySourceOrigin:
		expected = "source-origin"
	case ContextPropertyContainerMetadata:
		expected = "container-metadata"
	case ContextPropertySoundMapping:
		expected = "sound-mapping"
	case ContextPropertySpan, ContextPropertySpanLyricSyllable:
		expected = "span"
	case ContextPropertyRepeat:
		expected = "repeat"
	case ContextPropertyNavigation, ContextPropertyNavigationTraversal:
		expected = "navigation"
	case ContextPropertyTiming:
		expected = "timing"
	default:
		if dictionaryPropertyValid(in.Property) {
			expected = "dictionary"
		} else if resourcePropertyValid(in.Property) {
			expected = "resource"
		} else if contextLocalPropertyValid(in.Property) {
			expected = "layout"
		}
	}
	matched := expected == "dictionary" && in.Dictionary != nil || expected == "source-origin" && in.SourceOrigin != nil || expected == "container-metadata" && in.Container != nil || expected == "sound-mapping" && in.SoundMapping != nil || expected == "scalar" && payloads == 0 || expected == "span" && in.Span != nil || expected == "repeat" && in.Repeat != nil || expected == "navigation" && in.Navigation != nil || expected == "timing" && in.Timing != nil || expected == "layout" && in.Layout != nil || expected == "resource" && resourcePayloadValid(in.Property, in.Resource)
	if !matched {
		return contract("context projection", "property and typed payload must match")
	}

	if in.SourceOrigin != nil {
		if err = validateSourceOrigin(ctx, s, in.SourceOrigin, in.Subject); err != nil {
			return err
		}
	}
	return nil
}

// publishValidatedContextResult accepts only a value checked by
// validateContextResult in the same call, with no intervening payload mutation.
// Keep this private gate separate so SourceOrigin can check ownership before
// appending without validating (and charging for) the payload a second time.
func (s *Session) publishValidatedContextResult(ctx context.Context, in ContextResult) (err error) {
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
	if err = s.account.Check(BudgetScalarBytes, atomicLogicalBytes(in.Value.Value)); err != nil {
		return err
	}
	// The record itself lives in the backing array. Its cloned strings, the
	// value's strings, cause and numbers, and the origins' backing array,
	// strings and positions are separate allocations, each rounded up as the
	// allocator does. The walks below copy nothing; work is charged next.
	value, err := measureRetained(reflect.ValueOf(in.Value), true)
	if err != nil {
		return err
	}
	origins, err := measureRetained(reflect.ValueOf(in.Origins), true)
	if err != nil {
		return err
	}
	cost := ownedStringBytes(string(in.Provider.ID)) + ownedStringBytes(in.Subject.DocumentID) + ownedStringBytes(string(in.Property)) + value.bytes + origins.bytes
	work := int64(16+len(in.Provider.ID)+len(in.Subject.DocumentID)+len(in.Property)) + atomicLogicalBytes(in.Value.Value) + evidenceLogicalBytes(in.Value)
	if err = s.step(ctx, work); err != nil {
		return err
	}
	if s.omitContext {
		// The result is counted and checked, but the report keeps no copy.
		return nil
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
	for _, origin := range in.Origins {
		if err = s.step(ctx, locationLogicalBytes(origin)); err != nil {
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
	if out.SoundMapping, err = cloneSoundMappingContext(ctx, s, in.SoundMapping); err != nil {
		return err
	}
	if out.Resource, err = cloneResourceContext(ctx, s, in.Resource); err != nil {
		return err
	}
	if out.Container, err = cloneContainerContext(ctx, s, in.Container); err != nil {
		return err
	}
	if out.SourceOrigin, err = cloneSourceOriginContext(ctx, s, in.SourceOrigin); err != nil {
		return err
	}
	if out.Dictionary, err = cloneDictionaryContext(ctx, s, in.Dictionary); err != nil {
		return err
	}
	if err = s.step(ctx, 1); err != nil {
		return err
	}
	if len(s.contextResults) == cap(s.contextResults) {
		if err = s.growContextResults(); err != nil {
			return err
		}
	}
	s.contextResults = append(s.contextResults, out)
	// The projection owns these charges independently of intrinsic facts and
	// provider scratch. A retained-report finalizer can transfer the admitted
	// subtotal without copying or temporarily releasing the live projection.
	s.contextReportRetained += s.retained - before
	return nil
}

// growContextResults doubles the Report.Context backing array. The charge for
// a backing array of capacity c is contextBackingBytes(c): the new array is
// admitted before allocation, both coexist during the copy, and the old
// array's charge is released afterward. A rollback that restores an earlier
// slice header thereby restores the charge for its capacity too.
func (s *Session) growContextResults() error {
	old := cap(s.contextResults)
	grown := max(8, 2*old)
	if err := s.retain(contextBackingBytes(grown)); err != nil {
		return err
	}
	next := make([]ContextResult, len(s.contextResults), grown)
	copy(next, s.contextResults)
	// The old array is not cleared: a span rollback may restore its header.
	s.contextResults = next
	released := contextBackingBytes(old)
	if s.contextBackingHold {
		// A rollback snapshot still references the old array; it stays
		// charged until releaseHeldContextBacking or the rollback.
		s.contextBackingHeld += released
		return nil
	}
	if released != 0 {
		_ = s.account.Release(BudgetBufferedBytes, released)
		s.retained -= released
	}
	return nil
}

// holdContextBacking keeps the charge of every backing array that growth
// replaces until releaseHeldContextBacking, for a caller whose rollback may
// restore an earlier slice header. It returns the previous hold state.
func (s *Session) holdContextBacking() bool {
	previous := s.contextBackingHold
	s.contextBackingHold = true
	return previous
}

// releaseHeldContextBacking ends a hold. On commit it releases the held
// charges of replaced arrays, which the projection subtotal also counts; a
// rollback restores the earlier header and its charge itself, so it only
// forgets them.
func (s *Session) releaseHeldContextBacking(previous, commit bool) {
	held := s.contextBackingHeld
	s.contextBackingHold, s.contextBackingHeld = previous, 0
	if commit && held != 0 {
		_ = s.account.Release(BudgetBufferedBytes, held)
		s.retained -= held
		s.contextReportRetained -= held
	}
}

func contextBackingBytes(capacity int) int64 {
	return allocationSlack(int64(capacity) * contextResultBytes)
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
	if p.kind == planBarlineLocation {
		return evaluateBarlineLocation(ctx, s, n, p.context, final)
	}
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
	case planContextContainerMetadata:
		d, pending, err = evaluateContextContainerMetadata(ctx, s, n, key, p.context, final)
	case planContextSoundsMapping:
		d, pending, err = evaluateContextSoundsMapping(ctx, s, n, key, p.context, final)
	case planContextMusicXMLLinks, planContextOpusLinks:
		d, pending, err = evaluateResourceLinks(ctx, s, n, key, p.context, final)
	case planContextMusicXMLHints, planContextOpusHints, planContextContainerHints, planContextSoundsHints:
		d, pending, err = evaluateResourceHints(ctx, s, n, key, p.context, final)
	case planContextOpusLayout:
		d, pending, err = evaluateContextOpusLayout(ctx, s, n, key, p.context, final)
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
