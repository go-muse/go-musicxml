package validation

import (
	"slices"
)

const (
	planTimeline      planKind = "musicxml-timeline"
	planChordDuration planKind = "musicxml-chord-duration"
	planNumeralKey    planKind = "musicxml-numeral-key"
	// Changing the sealed role set or reviewed semantics requires a new revision.
	timelineSemanticRevision uint32 = 2
)

// TimelinePlanSpec binds the sealed MusicXML timeline service to one immutable
// compiled schema and the exact selected part-reference leaf rules. Both score
// hierarchy forms are required. A provider Pass certifies completed indexing and
// finalization, not known positions or complete effective state for every query.
type TimelinePlanSpec struct {
	Schema     *CompiledSchema
	PartOwners []ReferenceOwner
}

// TimelineConsumerSpec identifies an exact timeline provider rule revision and
// matching schema. Consumers additionally require that exact rule with
// DependencyRule and RelationDocument in their RuleDefinition. A valid local
// numeral-key is an independent alternative to querying the timeline service.
type TimelineConsumerSpec struct {
	Schema   *CompiledSchema
	Provider RuleKey
}

type timelinePlan struct {
	Schema     *CompiledSchema
	PartOwners []ReferenceOwner
	Bindings   *timelineBindings
	revision   uint32
}
type timelineConsumerPlan struct {
	Schema   *CompiledSchema
	Provider RuleKey
	Bindings *timelineBindings
	revision uint32
}

// NewTimelinePlan creates the sealed document-scoped MusicXML timeline service.
// It compiles fixed typed roles from exact schema identities once; it does not
// add generic musical semantics to context keys or prerequisite relations.
func NewTimelinePlan(spec TimelinePlanSpec) (Plan, error) {
	if len(spec.PartOwners) != 2 {
		return Plan{}, contract("timeline part owners", "both exact score hierarchy owner references required")
	}
	bindings, err := spec.Schema.sharedTimelineBindings()
	if err != nil {
		return Plan{}, err
	}
	return checkedPlan(Plan{kind: planTimeline, timeline: &timelinePlan{Schema: spec.Schema, PartOwners: spec.PartOwners, Bindings: bindings, revision: timelineSemanticRevision}})
}

// NewChordDurationPlan compares each duration-bearing chord tone against its
// established principal non-chord note through the exact selected service.
func NewChordDurationPlan(spec TimelineConsumerSpec) (Plan, error) {
	return newTimelineConsumer(planChordDuration, spec)
}

// NewNumeralKeyPlan requires a valid local numeral-key or an explicitly encoded
// applicable effective key at the enclosing harmony's musical position/staff.
func NewNumeralKeyPlan(spec TimelineConsumerSpec) (Plan, error) {
	return newTimelineConsumer(planNumeralKey, spec)
}
func newTimelineConsumer(kind planKind, spec TimelineConsumerSpec) (Plan, error) {
	if !catalogKey(spec.Provider) {
		return Plan{}, contract("timeline consumer", "exact provider rule revision required")
	}
	bindings, err := spec.Schema.sharedTimelineBindings()
	if err != nil {
		return Plan{}, err
	}
	return checkedPlan(Plan{kind: kind, timelineConsumer: &timelineConsumerPlan{Schema: spec.Schema, Provider: spec.Provider, Bindings: bindings, revision: timelineSemanticRevision}})
}
func (p *timelinePlan) valid() error {
	if p == nil || p.Schema == nil || p.Schema.kernel == nil || p.Schema.assembly != AssemblyMusicXML || p.Bindings == nil || p.revision != timelineSemanticRevision || len(p.PartOwners) != 2 {
		return contract("timeline plan", "sealed MusicXML schema roles and both exact part owners required")
	}
	seen := map[DeclarationID]bool{}
	for _, owner := range p.PartOwners {
		if !catalogKey(owner.Rule) || seen[owner.Declaration] {
			return contract("timeline part owners", "exact distinct owner declarations and rule revisions required")
		}
		seen[owner.Declaration] = true
		matched := false
		for _, h := range p.Bindings.Hierarchies {
			if owner.Declaration == h.Part.Declaration && owner.Attribute == h.PartID {
				matched = true
			}
		}
		if !matched {
			return contract("timeline part owners", "reference declaration and ID slot must match exact hierarchy")
		}
	}
	return nil
}
func (p *timelineConsumerPlan) valid() error {
	if p == nil || p.Schema == nil || p.Schema.kernel == nil || p.Schema.assembly != AssemblyMusicXML || p.Bindings == nil || p.revision != timelineSemanticRevision || !catalogKey(p.Provider) {
		return contract("timeline consumer", "sealed matching MusicXML schema roles and exact provider required")
	}
	return nil
}
func (p *timelinePlan) clone() *timelinePlan {
	if p == nil {
		return nil
	}
	q := *p
	q.Schema = p.Schema.snapshot()
	// Bindings are compiled, private and immutable; only caller-supplied owner
	// references need their own retained copy.
	q.PartOwners = make([]ReferenceOwner, len(p.PartOwners))
	for i, r := range p.PartOwners {
		q.PartOwners[i] = ReferenceOwner{Declaration: cloneDeclaration(r.Declaration), Attribute: cloneLocalAttribute(r.Attribute), Rule: cloneKey(r.Rule)}
	}
	slices.SortFunc(q.PartOwners, func(a, b ReferenceOwner) int { return compareLocalDeclaration(a.Declaration, b.Declaration) })
	return &q
}
func (p *timelineConsumerPlan) clone() *timelineConsumerPlan {
	if p == nil {
		return nil
	}
	q := *p
	q.Schema = p.Schema.snapshot()
	q.Provider = cloneKey(p.Provider)
	return &q
}
func (p *timelinePlan) size() int64 {
	if p == nil {
		return 0
	}
	n := int64(256) + p.Bindings.size()
	for _, r := range p.PartOwners {
		n += int64(192+len(r.Declaration.OwnerOccurrence)+len(r.Declaration.Occurrence)+len(r.Rule.ID)) + localAttributeSize(r.Attribute)
	}
	return n
}
func (p *timelineConsumerPlan) size() int64 {
	if p == nil {
		return 0
	}
	return int64(256+len(p.Provider.ID)) + p.Bindings.size()
}
func (p *timelinePlan) digest() any {
	if p == nil {
		return nil
	}
	return struct {
		Kind         planKind
		Revision     uint32
		SchemaDigest string
		Assembly     Assembly
		PartOwners   []ReferenceOwner
		Bindings     *timelineBindings
	}{planTimeline, p.revision, p.Schema.Digest(), p.Schema.Assembly(), p.PartOwners, p.Bindings}
}
func (p *timelineConsumerPlan) digest(kind planKind) any {
	if p == nil {
		return nil
	}
	return struct {
		Kind         planKind
		Revision     uint32
		SchemaDigest string
		Assembly     Assembly
		Provider     RuleKey
		Bindings     *timelineBindings
	}{kind, p.revision, p.Schema.Digest(), p.Schema.Assembly(), p.Provider, p.Bindings}
}
func (p Plan) isTimeline() bool {
	return p.kind == planTimeline || p.kind == planChordDuration || p.kind == planNumeralKey
}
func (p Plan) validateTimelineFor(r RuleDefinition) error {
	if !p.isTimeline() {
		return nil
	}
	if err := p.valid(); err != nil {
		return err
	}
	for _, target := range r.Targets {
		if target != TargetSource && target != TargetModel {
			return contract("timeline targets", "only source/current-model targets are supported")
		}
	}
	if r.Category != CategoryNormative || r.Class != ClassMandatory {
		return contract("timeline rule", "required normative rule required")
	}
	if p.kind == planTimeline {
		if r.Scope != ScopeDocument || r.Role != RoleContextProvider || r.Selector.Assembly == nil || *r.Selector.Assembly != AssemblyMusicXML || r.Selector.Declaration != nil || r.Selector.Component != nil {
			return contract("timeline provider", "document context provider with exact MusicXML assembly selector required")
		}
		return nil
	}
	if r.Scope != ScopeNode || r.Role != RolePredicate || r.Selector.Assembly != nil {
		return contract("timeline consumer", "mandatory normative node predicate required")
	}
	expected := timelineNamedType("note")
	role := timelineRoleNote
	if p.kind == planNumeralKey {
		expected = timelineNamedType("numeral")
		role = timelineRoleHarmonyNumeral
	}
	if (r.Selector.Component == nil || *r.Selector.Component != expected) && (r.Selector.Declaration == nil || !p.timelineConsumer.Bindings.matches(role, *r.Selector.Declaration)) {
		return contract("timeline consumer selector", "exact note or numeral declaration/type for the selected family required")
	}
	dep := Dependency{Kind: DependencyRule, Rule: p.timelineConsumer.Provider, Relation: RelationDocument}
	if !slices.Contains(r.Requires, dep) {
		return contract("timeline consumer dependency", "exact provider DependencyRule with RelationDocument required")
	}
	return nil
}
func (p Plan) validateTimelineRules(r RuleDefinition, c *CompiledCatalog, closure map[RuleKey]bool) error {
	if !p.isTimeline() {
		return nil
	}
	if err := p.validateTimelineFor(r); err != nil {
		return err
	}
	if p.kind == planTimeline {
		for _, owner := range p.timeline.PartOwners {
			rule, ok := c.rules[owner.Rule]
			if !ok || !closure[owner.Rule] {
				return contract("timeline part owners", "exact selected owner rule required")
			}
			ref, ok := c.plans[rule.Predicate]
			if !ok || ref.kind != planTypedReference || ref.reference == nil {
				return contract("timeline part owners", "selected typed-reference leaf required")
			}
			if err := ref.validateFor(rule); err != nil {
				return err
			}
			if rule.Selector.Declaration == nil || *rule.Selector.Declaration != owner.Declaration || ref.reference.Attribute != owner.Attribute || len(ref.reference.Owners) != 0 || len(ref.reference.OwnerReferences) != 0 || len(ref.reference.TriggerChildren) != 0 || !slices.Equal(ref.reference.Targets, p.timeline.Bindings.declarations(timelineRoleScorePart)) || !slices.Equal(ref.reference.TargetAncestors, p.timeline.Bindings.declarations(timelineRolePartList)) {
				return contract("timeline part owners", "leaf must match exact part ID slot, canonical score-part targets and part-list ancestors")
			}
			for _, target := range r.Targets {
				if !slices.Contains(rule.Targets, target) {
					return contract("timeline part owners", "owner provider must support every service target")
				}
			}
		}
		return nil
	}
	key := p.timelineConsumer.Provider
	provider, ok := c.rules[key]
	if !ok || !closure[key] {
		return contract("timeline provider", "exact selected provider rule required")
	}
	plan, ok := c.plans[provider.Predicate]
	if !ok || plan.kind != planTimeline || plan.timeline == nil {
		return contract("timeline provider", "sealed timeline provider required")
	}
	if err := plan.validateTimelineFor(provider); err != nil {
		return err
	}
	if plan.timeline.Schema.Digest() != p.timelineConsumer.Schema.Digest() || plan.timeline.revision != p.timelineConsumer.revision {
		return contract("timeline provider", "matching schema and semantic revision required")
	}
	for _, target := range r.Targets {
		if !slices.Contains(provider.Targets, target) {
			return contract("timeline provider", "provider must support every consumer target")
		}
	}
	return nil
}
func validateTimelineProfile(c *CompiledCatalog, closure map[RuleKey]bool, schemas map[Assembly]*CompiledSchema) error {
	providers := 0
	for key := range closure {
		rule := c.rules[key]
		p, ok := c.plans[rule.Predicate]
		if !ok || !p.isTimeline() {
			continue
		}
		schema := p.timelineConsumerSchema()
		if schema == nil || schemas[AssemblyMusicXML] == nil || schemas[AssemblyMusicXML].Digest() != schema.Digest() {
			return contract("timeline profile", "exact matching selected XSD schema required")
		}
		if p.kind == planTimeline {
			providers++
		}
	}
	if providers > 1 {
		return contract("timeline profile", "only one sealed document timeline provider may be selected")
	}
	return nil
}
func (p Plan) timelineConsumerSchema() *CompiledSchema {
	if p.kind == planTimeline && p.timeline != nil {
		return p.timeline.Schema
	}
	if p.timelineConsumer != nil {
		return p.timelineConsumer.Schema
	}
	return nil
}

// boundSchema accounts and interns every retained schema handle, including plans
// that are not selected by a profile. Immutable timeline bindings are shared,
// but remain conservatively accounted for by every plan that retains them.
func (p Plan) boundSchema() *CompiledSchema {
	if p.kind == planXSD {
		return p.schema
	}
	return p.timelineConsumerSchema()
}
func (p Plan) withBoundSchema(schema *CompiledSchema) Plan {
	switch p.kind {
	case planXSD:
		p.schema = schema
	case planTimeline:
		copy := *p.timeline
		copy.Schema = schema
		p.timeline = &copy
	case planChordDuration, planNumeralKey:
		copy := *p.timelineConsumer
		copy.Schema = schema
		p.timelineConsumer = &copy
	}
	return p
}
