package validation

import "slices"

const (
	planContextContainerMetadata planKind = "container-context-metadata"
	planContextSoundsMapping     planKind = "sounds-context-mapping"
	planContextOpusLayout        planKind = "opus-context-layout"
	planFigureNumberNumeric      planKind = "musicxml-figure-number-numeric"
	planGraphicalRepeatContent   planKind = "musicxml-graphical-repeat-content"
	planBendReleaseSign          planKind = "musicxml-bend-release-sign"
	planContextLocal             planKind = "musicxml-context-local"
	planContextSpans             planKind = "musicxml-context-spans"
	planContextRepeats           planKind = "musicxml-context-repeats"
	planContextNavigation        planKind = "musicxml-context-navigation"
	planContextLayout            planKind = "musicxml-context-layout"
	planContextPerformance       planKind = "musicxml-context-performance"
	planTimeOnlyOrder            planKind = "musicxml-time-only-order"
	contextSemanticRevision      uint32   = 1
)

// ContextPlanSpec binds a sealed context family to immutable MusicXML schema
// roles and, when needed, an exact selected timeline service revision. An empty
// TimelineProvider is permitted for independent span/layout/performance facts.
type ContextPlanSpec struct {
	Schema           *CompiledSchema
	TimelineProvider RuleKey
}
type contextPlan struct {
	sourceInfoset bool
	Schema        *CompiledSchema
	Timeline      RuleKey
	Provider      RuleKey
	Bindings      *contextBindings
	revision      uint32
}

// NewLocalContextPlan constructs the source-supported local default/fact
// provider. It has no timeline or navigation dependency.
func NewLocalContextPlan(schema *CompiledSchema) (Plan, error) {
	return newContextPlan(planContextLocal, ContextPlanSpec{Schema: schema})
}

// NewSpanContextPlan constructs a type-specific span context provider.
func NewSpanContextPlan(spec ContextPlanSpec) (Plan, error) {
	return newContextPlan(planContextSpans, spec)
}

// NewRepeatContextPlan constructs graphical repeat context, not playback expansion.
func NewRepeatContextPlan(spec ContextPlanSpec) (Plan, error) {
	return newContextPlan(planContextRepeats, spec)
}

// NewNavigationContextPlan constructs bounded symbolic navigation/pass context.
func NewNavigationContextPlan(spec ContextPlanSpec) (Plan, error) {
	return newContextPlan(planContextNavigation, spec)
}

// NewLayoutContextPlan constructs layout applicability/context without an engraver.
func NewLayoutContextPlan(spec ContextPlanSpec) (Plan, error) {
	return newContextPlan(planContextLayout, spec)
}

// NewPerformanceContextPlan constructs source-supported performance context,
// preserving unresolved intent and physical-realization dependencies.
func NewPerformanceContextPlan(spec ContextPlanSpec) (Plan, error) {
	return newContextPlan(planContextPerformance, spec)
}

// NewTimeOnlyOrderPlan compares exact time-only pass values without requiring
// musical time or expanding repeat traversals.
func NewTimeOnlyOrderPlan(schema *CompiledSchema) (Plan, error) {
	return newContextPlan(planTimeOnlyOrder, ContextPlanSpec{Schema: schema})
}

// NewFigureNumberNumericPlan accepts the sufficient nonempty ASCII-digit subset
// for the exact figure-number declaration. Other spellings retain the unreviewed
// numeric-grammar gate rather than asserting a complete lexical grammar.
func NewFigureNumberNumericPlan(schema *CompiledSchema) (Plan, error) {
	return newContextPlan(planFigureNumberNumeric, ContextPlanSpec{Schema: schema})
}

// NewOpusLayoutContextPlan projects local Opus version defaults and pagination hints.
// It never resolves score links or supplies application-dependent page choices.
func NewOpusLayoutContextPlan(schema *CompiledSchema) (Plan, error) {
	return newContextPlan(planContextOpusLayout, ContextPlanSpec{Schema: schema})
}

// NewSoundsMappingContextPlan projects the four directional sounds mapping roles.
// Only the exact sounds assembly and source target are supported.
func NewSoundsMappingContextPlan(schema *CompiledSchema) (Plan, error) {
	return newContextPlan(planContextSoundsMapping, ContextPlanSpec{Schema: schema})
}

// NewContainerMetadataContextPlan projects ordered source container metadata.
// Only the exact container assembly and source target are supported.
func NewContainerMetadataContextPlan(schema *CompiledSchema) (Plan, error) {
	return newContextPlan(planContextContainerMetadata, ContextPlanSpec{Schema: schema})
}

// NewSourceContainerMetadataContextPlan projects effective ordered container
// metadata separately from original XML-normalized attribute spellings. DTD
// defaults and entity-generated attributes have no physical Raw spelling. It
// requires a selected source-origin provider and supports only source input.
func NewSourceContainerMetadataContextPlan(schema *CompiledSchema) (Plan, error) {
	p, err := NewContainerMetadataContextPlan(schema)
	if err != nil {
		return Plan{}, err
	}
	p.context.sourceInfoset = true
	return checkedPlan(p)
}

func contextPlanAssembly(kind planKind) Assembly {
	if kind == planContextContainerMetadata {
		return AssemblyContainer
	}
	if kind == planContextSoundsMapping {
		return AssemblySounds
	}
	if kind == planContextOpusLayout || kind == planContextOpusLinks || kind == planContextOpusHints {
		return AssemblyOpus
	}
	if kind == planContextContainerHints {
		return AssemblyContainer
	}
	if kind == planContextSoundsHints {
		return AssemblySounds
	}
	return AssemblyMusicXML
}

func newContextPlan(kind planKind, spec ContextPlanSpec) (Plan, error) {
	if spec.Schema == nil || spec.Schema.assembly != contextPlanAssembly(kind) {
		return Plan{}, contract("context schema", "exact plan assembly required")
	}
	if (kind == planContextRepeats || kind == planContextNavigation) && !catalogKey(spec.TimelineProvider) {
		return Plan{}, contract("context timeline", "exact provider revision required")
	}
	b, err := spec.Schema.sharedContextBindings()
	if err != nil {
		return Plan{}, err
	}
	return checkedPlan(Plan{kind: kind, context: &contextPlan{Schema: spec.Schema, Timeline: spec.TimelineProvider, Bindings: b, revision: contextSemanticRevision}})
}
func (p Plan) isContext() bool {
	switch p.kind {
	case planContextContainerMetadata, planContextSoundsMapping, planContextMusicXMLLinks, planContextOpusLinks, planContextMusicXMLHints, planContextOpusHints, planContextContainerHints, planContextSoundsHints, planContextOpusLayout, planContextLocal, planContextSpans, planContextRepeats, planContextNavigation, planContextLayout, planContextPerformance, planTimeOnlyOrder, planFigureNumberNumeric, planBendReleaseSign, planGraphicalRepeatContent, planBarlineLocation:
		return true
	}
	return false
}
func (p *contextPlan) valid(kind planKind) error {
	if p == nil || p.Schema == nil || p.Schema.kernel == nil || p.Schema.assembly != contextPlanAssembly(kind) || p.Bindings == nil || p.revision != contextSemanticRevision {
		return contract("context plan", "sealed MusicXML schema and exact context revision required")
	}
	if p.sourceInfoset && kind != planContextMusicXMLLinks && kind != planContextOpusLinks && kind != planContextMusicXMLHints && kind != planContextOpusHints && kind != planContextContainerHints && kind != planContextSoundsHints && kind != planContextContainerMetadata {
		return contract("source resource context", "supported source resource family required")
	}
	if p.Timeline != (RuleKey{}) && !catalogKey(p.Timeline) {
		return contract("context timeline", "invalid provider revision")
	}
	return nil
}
func (p *contextPlan) clone() *contextPlan {
	if p == nil {
		return nil
	}
	q := *p
	q.Schema = p.Schema.snapshot()
	q.Timeline = cloneKey(p.Timeline)
	q.Provider = cloneKey(p.Provider)
	return &q
}
func (p *contextPlan) size() int64 {
	if p == nil {
		return 0
	}
	return 256 + int64(len(p.Timeline.ID)+len(p.Provider.ID)) + p.Bindings.size()
}
func (p *contextPlan) digest(kind planKind) any {
	if p == nil {
		return nil
	}
	out := struct {
		Kind         planKind
		Revision     uint32
		SchemaDigest string
		Assembly     Assembly
		Timeline     RuleKey
		Provider     RuleKey
		Bindings     string
	}{kind, p.revision, p.Schema.Digest(), p.Schema.Assembly(), p.Timeline, p.Provider, p.Bindings.digest()}
	if p.sourceInfoset {
		return struct {
			Provenance string
			Context    any
		}{"source-infoset", out}
	}
	return out
}
func (p Plan) validateContextFor(r RuleDefinition) error {
	if !p.isContext() {
		return nil
	}
	if err := p.context.valid(p.kind); err != nil {
		return err
	}
	if r.Category != CategoryNormative || r.Class != ClassMandatory {
		return contract("context rule", "normative mandatory family contract required")
	}
	for _, target := range r.Targets {
		if p.context.sourceInfoset && target != TargetSource {
			return contract("source resource target", "source-only target required")
		}
		if p.kind == planContextContainerMetadata && target != TargetSource {
			return contract("container context target", "source-only assembly required")
		}
		if p.kind == planContextSoundsMapping && target != TargetSource {
			return contract("sounds context target", "source-only assembly required")
		}
		if target != TargetSource && target != TargetModel {
			return contract("context target", "only source/current-model targets supported")
		}
	}
	if p.kind == planBendReleaseSign || p.kind == planGraphicalRepeatContent {
		selectorOK := r.Selector.Component != nil && *r.Selector.Component == contextNamedType("bend")
		if p.kind == planGraphicalRepeatContent {
			selectorOK = r.Selector.Component != nil && (*r.Selector.Component == contextNamedType("beat-repeat") || *r.Selector.Component == contextNamedType("measure-repeat"))
		}
		if r.Scope != ScopeNode || r.Role != RolePredicate || !selectorOK || r.Selector.Assembly != nil || r.Selector.Declaration != nil {
			return contract("context consumer selector", "exact type-specific node predicate required")
		}
		if !catalogKey(p.context.Provider) || !slices.Contains(r.Requires, Dependency{Kind: DependencyRule, Rule: p.context.Provider, Relation: RelationDocument}) {
			return contract("context consumer dependency", "exact type-specific provider required")
		}
	} else if p.kind == planBarlineLocation {
		if r.Scope != ScopeNode || r.Role != RolePredicate || r.Selector.Component == nil || *r.Selector.Component != contextNamedType("barline") || r.Selector.Assembly != nil || r.Selector.Declaration != nil || !catalogKey(p.context.Timeline) {
			return contract("barline selector", "exact barline node predicate and timeline required")
		}
	} else if p.kind == planFigureNumberNumeric {
		uses := p.context.Bindings.Roles[contextRoleFigureNumber]
		if r.Scope != ScopeNode || r.Role != RolePredicate || r.Selector.Assembly != nil || r.Selector.Component != nil || r.Selector.Declaration == nil || len(uses) != 1 || *r.Selector.Declaration != uses[0].Declaration {
			return contract("figure-number selector", "exact figure-number declaration predicate required")
		}
	} else if p.kind == planTimeOnlyOrder {
		if r.Scope != ScopeNode || r.Role != RolePredicate || r.Selector.Assembly != nil || r.Selector.Declaration != nil {
			return contract("time-only selector", "exact node predicate selector required")
		}
		matched := false
		for _, u := range p.context.Bindings.Roles[contextRoleTimeOnly] {
			if r.Selector.Component != nil && *r.Selector.Component == u.OwnerType {
				matched = true
			}
		}
		if !matched {
			return contract("time-only selector", "exact type owning a bound time-only use required")
		}
	} else if r.Scope != ScopeDocument || r.Role != RoleContextProvider || r.Selector.Assembly == nil || *r.Selector.Assembly != contextPlanAssembly(p.kind) || r.Selector.Component != nil || r.Selector.Declaration != nil {
		return contract("context provider", "document provider with exact plan assembly selector required")
	}
	if p.context.Timeline != (RuleKey{}) {
		dep := Dependency{Kind: DependencyRule, Rule: p.context.Timeline, Relation: RelationDocument}
		if !slices.Contains(r.Requires, dep) {
			return contract("context dependency", "exact document timeline dependency required")
		}
	} else if p.kind == planContextRepeats || p.kind == planContextNavigation {
		return contract("context dependency", "timeline service required")
	}
	return nil
}
func (p Plan) validateContextRules(r RuleDefinition, c *CompiledCatalog, closure map[RuleKey]bool) error {
	if !p.isContext() {
		return nil
	}
	if err := p.validateContextFor(r); err != nil {
		return err
	}
	if p.kind == planBendReleaseSign || p.kind == planGraphicalRepeatContent {
		expected := planContextPerformance
		if p.kind == planGraphicalRepeatContent {
			expected = planContextRepeats
		}
		key := p.context.Provider
		rule, ok := c.rules[key]
		if !ok || !closure[key] {
			return contract("context consumer dependency", "selected type-specific provider required")
		}
		provider, ok := c.plans[rule.Predicate]
		if !ok || provider.kind != expected || provider.context == nil || provider.context.revision != p.context.revision || provider.context.Schema.Digest() != p.context.Schema.Digest() {
			return contract("context consumer dependency", "matching sealed type-specific provider required")
		}
		return nil
	}
	key := p.context.Timeline
	if key == (RuleKey{}) {
		return nil
	}
	rule, ok := c.rules[key]
	if !ok || !closure[key] {
		return contract("context dependency", "exact selected timeline provider required")
	}
	provider, ok := c.plans[rule.Predicate]
	if !ok || provider.kind != planTimeline || provider.timeline == nil || provider.timeline.revision != timelineSemanticRevision || provider.timeline.Schema.Digest() != p.context.Schema.Digest() {
		return contract("context dependency", "matching sealed timeline schema/revision required")
	}
	return nil
}
func validateContextProfile(c *CompiledCatalog, closure map[RuleKey]bool, schemas map[Assembly]*CompiledSchema) error {
	seen := map[planKind]bool{}
	for key := range closure {
		r := c.rules[key]
		p, ok := c.plans[r.Predicate]
		if !ok || !p.isContext() {
			continue
		}
		schema := schemas[contextPlanAssembly(p.kind)]
		if schema == nil || schema.Digest() != p.context.Schema.Digest() {
			return contract("context schema", "matching selected XSD context assembly schema required")
		}
		if p.kind != planTimeOnlyOrder && p.kind != planFigureNumberNumeric && p.kind != planBendReleaseSign && p.kind != planGraphicalRepeatContent && p.kind != planBarlineLocation {
			if seen[p.kind] {
				return contract("context profile", "only one provider per sealed family is supported")
			}
			seen[p.kind] = true
		}
	}
	return nil
}

// ContextConsumerSpec binds a typed contextual predicate to one exact provider.
type ContextConsumerSpec struct {
	Schema   *CompiledSchema
	Provider RuleKey
}

// NewBendReleaseSignPlan checks the source-supported sign relation only when
// the selected performance service establishes the corresponding bend gesture.
func NewBendReleaseSignPlan(spec ContextConsumerSpec) (Plan, error) {
	if !catalogKey(spec.Provider) {
		return Plan{}, contract("bend consumer", "exact provider revision required")
	}
	b, err := spec.Schema.sharedContextBindings()
	if err != nil {
		return Plan{}, err
	}
	return checkedPlan(Plan{kind: planBendReleaseSign, context: &contextPlan{Schema: spec.Schema, Provider: spec.Provider, Bindings: b, revision: contextSemanticRevision}})
}

// NewGraphicalRepeatContentPlan consumes proved graphical repeat content
// correspondence. It does not equate XML strings, infer missing musical events,
// or declare unresolved event-equivalence policies to be violations.
func NewGraphicalRepeatContentPlan(spec ContextConsumerSpec) (Plan, error) {
	if !catalogKey(spec.Provider) {
		return Plan{}, contract("repeat content consumer", "exact provider revision required")
	}
	b, err := spec.Schema.sharedContextBindings()
	if err != nil {
		return Plan{}, err
	}
	return checkedPlan(Plan{kind: planGraphicalRepeatContent, context: &contextPlan{Schema: spec.Schema, Provider: spec.Provider, Bindings: b, revision: contextSemanticRevision}})
}
