package validation

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// PlanBinding gives one immutable, constructor-produced plan its catalog-local
// ID. RuleDefinition.Predicate refers to this ID, never to executable source.
type PlanBinding struct {
	ID   string
	Plan Plan
}

// CompiledCatalog owns validated rule metadata, typed plans and exact profile
// dependency closures. It is immutable and safe for simultaneous readers.
// A catalog makes only its profiles' explicit claims; no MusicXML profile is
// implicitly supplied by this package.
type CompiledCatalog struct {
	fragmentModes          map[ProfileID]map[Assembly]fragmentMode
	fragmentDTDs           map[ProfileID]map[Assembly]bool
	sourceDeclarations     map[ProfileID]map[Assembly]xsdPlanMode
	sourceOrigins          map[ProfileID]map[Assembly]RuleKey
	sourceAttributeLexical map[ProfileID]map[Assembly]RuleKey
	schemas                map[ProfileID]map[Assembly]*CompiledSchema
	rules                  map[RuleKey]RuleDefinition
	profiles               map[ProfileID]Profile
	plans                  map[string]Plan
	orders                 map[ProfileID][]RuleKey
	modelOrders            map[ProfileID][]RuleKey
	phases                 map[ProfileID][]catalogPhase
	supports               map[ProfileID]map[RuleKey]RuleSupport
}

// A declaration's registration phase collects every eligible subject before
// any same-index uniqueness verdict. Evaluation remains a separate causal result.
type catalogPhase struct {
	rule         RuleKey
	registration bool
}

// Compilation is bounded independently from per-document session resources.
// Collection slots are charged conservatively at 128 bytes, plus string bytes;
// this measures configuration input, not a promise about Go allocator overhead.
const (
	catalogMaxRules          = 65536
	catalogMaxProfiles       = 4096
	catalogMaxBytes    int64 = 64 << 20
	catalogSlotBytes   int64 = 128
)

// NewCatalog validates and snapshots reviewed rules, profiles and sealed plans.
// Every profile must name at least one required rule and root, explicitly state
// its version, source pin and normative closure, and carry exactly one support
// row for every required or transitive rule. Unselected support rows are errors.
// Unavailable scopes/selectors require unsupported support; unavailable runtime
// dependencies remain present and can produce unsupported assessment outcomes.
// Selected context value/default writers precede readers of the same key and
// overlapping local/document scope. Identity declaration collection from every
// selected declaring plan precedes all same-index uniqueness/reference verdicts;
// collection itself waits for that declaration's semantic prerequisites. These
// typed-plan phase edges are cycle-checked alongside Rule dependencies;
// Schedule exposes final-evaluation order. They neither select additional rules
// nor change semantic Requires or turn provider non-applicability into a
// failure. Missing/unselected providers remain runtime evidence limitations.
//
// Compilation rejects more than 65,536 rules or plans, 4,096 profiles, or 64 MiB
// of configuration accounting (128 bytes per collection slot plus string/plan
// parameter bytes). IDs are bounded to 1,024 bytes, namespace URIs and rule text
// to 4,096 bytes, and profile closure text to 16,384 bytes. Bounds are checked
// before snapshots are allocated. Compiled profile schedules additionally allow
// at most 524,288 unique prerequisite phase edges across the catalog (64 MiB,
// at 128 accounting bytes per edge), checked before retaining each edge. This bounds
// expansion of matching context/identity writers and readers. Invalid input
// returns ConfigurationError.
//
// All profiles receive the same SHA-256 digest of the canonical catalog input,
// including every typed plan parameter and profile claim. Set-like collections
// are sorted for this digest; supplied CatalogDigest fields are excluded. A
// nonempty supplied digest must exactly match the computed lowercase hex value.
func NewCatalog(rules []RuleDefinition, profiles []Profile, plans []PlanBinding) (*CompiledCatalog, error) {
	if err := catalogCheckBounds(rules, profiles, plans); err != nil {
		return nil, err
	}
	c := &CompiledCatalog{
		schemas:     make(map[ProfileID]map[Assembly]*CompiledSchema, len(profiles)),
		rules:       make(map[RuleKey]RuleDefinition, len(rules)),
		profiles:    make(map[ProfileID]Profile, len(profiles)),
		plans:       make(map[string]Plan, len(plans)),
		orders:      make(map[ProfileID][]RuleKey, len(profiles)),
		modelOrders: make(map[ProfileID][]RuleKey, len(profiles)),
		phases:      make(map[ProfileID][]catalogPhase, len(profiles)),
		supports:    make(map[ProfileID]map[RuleKey]RuleSupport, len(profiles)),
	}
	internedSchemas := make(map[string]*CompiledSchema)
	for _, binding := range plans {
		if schema := binding.Plan.boundSchema(); schema != nil {
			key := string(schema.assembly) + ":" + schema.Digest()
			if existing := internedSchemas[key]; existing != nil {
				binding.Plan = binding.Plan.withBoundSchema(existing)
			} else {
				internedSchemas[key] = schema
			}
		}
		if !catalogID(binding.ID) {
			return nil, contract("plan ID", "missing or malformed identity")
		}
		if _, exists := c.plans[binding.ID]; exists {
			return nil, contract("plan ID", "duplicate identity")
		}
		// catalogCheckBounds validated this plan before any retained copy.
		c.plans[strings.Clone(binding.ID)] = binding.Plan.clone()
	}
	for _, rule := range rules {
		if err := catalogValidateRule(rule); err != nil {
			return nil, err
		}
		if _, exists := c.rules[rule.Key]; exists {
			return nil, contract("rule key", "duplicate exact rule revision")
		}
		owned := cloneRule(rule)
		c.rules[owned.Key] = owned
	}
	order, err := catalogOrder(c.rules)
	if err != nil {
		return nil, err
	}
	for _, key := range order {
		if err := catalogValidateRelations(c.rules[key], c.rules); err != nil {
			return nil, err
		}
	}
	scheduleBudget := catalogBudget{remaining: catalogMaxBytes}
	for _, profile := range profiles {
		if err := catalogValidateProfile(profile); err != nil {
			return nil, err
		}
		if _, exists := c.profiles[profile.ID]; exists {
			return nil, contract("profile ID", "duplicate identity")
		}
		owned := cloneProfile(profile)
		supports := make(map[RuleKey]RuleSupport, len(owned.Support))
		for _, support := range owned.Support {
			if _, exists := supports[support.Rule]; exists {
				return nil, contract("profile support", "duplicate exact rule revision")
			}
			supports[support.Rule] = support
		}
		closure, err := catalogClosure(owned.Required, c.rules)
		if err != nil {
			return nil, err
		}
		if len(closure) != len(supports) {
			return nil, contract("profile support", "must exactly cover required and transitive rules")
		}
		for key := range closure {
			rule := c.rules[key]
			support, exists := supports[key]
			if !exists {
				return nil, contract("profile support", "missing required or transitive rule")
			}
			if !slices.Contains(rule.Targets, profile.Target) {
				return nil, contract("profile target", "required or transitive rule does not support target")
			}
			if plan, exists := c.plans[rule.Predicate]; exists && plan.isTimeline() {
				if err := plan.validateTimelineRules(rule, c, closure); err != nil {
					return nil, err
				}
			}
			if plan, exists := c.plans[rule.Predicate]; exists && plan.isContext() {
				if err := plan.validateContextRules(rule, c, closure); err != nil {
					return nil, err
				}
			}
			if support.State == ImplementationAvailable {
				plan, exists := c.plans[rule.Predicate]
				if !exists {
					return nil, contract("rule predicate", "available support requires a resolved plan")
				}
				if rule.Scope != ScopeNode && rule.Scope != ScopeDocument {
					return nil, contract("rule scope", "unimplemented scope requires unsupported support")
				}
				if rule.Selector.Component != nil && rule.Selector.Component.Kind != ComponentType {
					return nil, contract("rule selector", "only exact type components or declarations are executable")
				}
				if err := plan.validateReferenceRules(c, closure); err != nil {
					return nil, err
				}
				if err := plan.validateFor(rule); err != nil {
					return nil, fmt.Errorf("catalog rule %q revision %d: %w", key.ID, key.Revision, err)
				}
			}
		}
		schemas := make(map[Assembly]*CompiledSchema)
		modes := make(map[Assembly]xsdPlanMode)
		availableModes := make(map[Assembly]xsdPlanMode)
		for key := range closure {
			rule := c.rules[key]
			plan, found := c.plans[rule.Predicate]
			if !found || plan.kind != planXSD {
				continue
			}
			if err := plan.validateFor(rule); err != nil {
				return nil, err
			}
			if profile.Target != TargetSource && profile.Target != TargetModel {
				return nil, contract("schema profile", "only source or model targets are implemented")
			}
			if previous := schemas[plan.schema.assembly]; previous != nil && previous.Digest() != plan.schema.Digest() {
				return nil, contract("schema profile", "conflicting schemas in one assembly")
			}
			if previous, present := modes[plan.schema.assembly]; present && previous != plan.xsdMode {
				return nil, contract("schema profile", "conflicting source-declaration modes in one assembly")
			}
			modes[plan.schema.assembly] = plan.xsdMode
			if supports[key].State == ImplementationAvailable && plan.xsdMode == xsdSourceDeclarationsV1 {
				availableModes[plan.schema.assembly] = plan.xsdMode
			}
			schemas[plan.schema.assembly] = plan.schema
		}
		if len(availableModes) != 0 {
			if c.sourceDeclarations == nil {
				c.sourceDeclarations = make(map[ProfileID]map[Assembly]xsdPlanMode)
			}
			c.sourceDeclarations[profile.ID] = availableModes
		}
		if err := validateSourceLocalPresenceProfile(c, owned, closure, supports); err != nil {
			return nil, err
		}
		if err := validateSourceOriginProfile(c, owned, closure, supports, schemas); err != nil {
			return nil, err
		}
		if err := validateDictionaryProfile(c, closure, schemas); err != nil {
			return nil, err
		}
		if err := validateSourceResourceProfile(c, owned, closure, supports); err != nil {
			return nil, err
		}
		if err := validateFragmentProfile(c, owned, closure, supports); err != nil {
			return nil, err
		}
		if err := validateContextProfile(c, closure, schemas); err != nil {
			return nil, err
		}
		if err := validateTimelineProfile(c, closure, schemas); err != nil {
			return nil, err
		}
		if len(schemas) != 0 {
			for _, name := range profile.SupportedRoots {
				matches := 0
				for _, schema := range schemas {
					if _, ok := schema.Root(name); ok {
						matches++
					}
				}
				if matches != 1 {
					return nil, contract("schema profile roots", "each root must identify exactly one selected assembly")
				}
			}
			c.schemas[profile.ID] = schemas
		}
		phases, err := catalogProfileOrder(c, closure, &scheduleBudget)
		if err != nil {
			return nil, err
		}
		keys := make([]RuleKey, 0, len(closure))
		for _, phase := range phases {
			if !phase.registration {
				keys = append(keys, phase.rule)
			}
		}
		// Cache only the ordered preparation subset. Its exact capacity is
		// bounded by this profile's already checked support inventory, and
		// keys borrow immutable strings from the compiled rules. Source
		// profiles have no model-representation preparation work.
		modelCount := 0
		for _, key := range keys {
			if c.plans[c.rules[key].Predicate].kind == planModelRepresentation {
				modelCount++
			}
		}
		if modelCount != 0 {
			modelKeys := make([]RuleKey, 0, modelCount)
			for _, key := range keys {
				if c.plans[c.rules[key].Predicate].kind == planModelRepresentation {
					modelKeys = append(modelKeys, key)
				}
			}
			c.modelOrders[owned.ID] = modelKeys
		}
		c.profiles[owned.ID] = owned
		c.orders[owned.ID] = keys
		c.phases[owned.ID] = phases
		c.supports[owned.ID] = supports
	}
	digest, err := catalogDigest(c)
	if err != nil {
		return nil, err
	}
	for id, profile := range c.profiles {
		if profile.CatalogDigest != "" && profile.CatalogDigest != digest {
			return nil, contract("profile catalog digest", "supplied digest does not match canonical catalog")
		}
		profile.CatalogDigest = digest
		c.profiles[id] = profile
	}
	return c, nil
}

// Rule returns an independent deep copy of one exact semantic revision.
// Missing keys and a nil catalog return false.
func (c *CompiledCatalog) Rule(key RuleKey) (RuleDefinition, bool) {
	if c == nil {
		return RuleDefinition{}, false
	}
	rule, ok := c.rules[key]
	if !ok {
		return RuleDefinition{}, false
	}
	return cloneRule(rule), true
}

// Profile returns an independent deep copy, including its complete support
// inventory and computed catalog digest. A nil catalog returns false.
func (c *CompiledCatalog) Profile(id ProfileID) (Profile, bool) {
	if c == nil {
		return Profile{}, false
	}
	profile, ok := c.profiles[id]
	if !ok {
		return Profile{}, false
	}
	return cloneProfile(profile), true
}

// Schedule returns an independent copy of a profile's final-evaluation rule order.
// The order includes declared rule prerequisites and ordering-only edges derived
// from the selected sealed context/identity plans. Each rule occurs exactly
// once; identity registration phases precede final evaluation internally. It
// never selects extra rules or adds semantic Requires prerequisites. Nil and unknown catalogs return false.
func (c *CompiledCatalog) Schedule(id ProfileID) ([]RuleKey, bool) {
	if c == nil {
		return nil, false
	}
	order, ok := c.orders[id]
	if !ok {
		return nil, false
	}
	owned := make([]RuleKey, len(order))
	for i, key := range order {
		owned[i] = cloneKey(key)
	}
	return owned, true
}

func catalogID(s string) bool {
	if !validID(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func catalogText(s string, max int) bool { return validText(s, max) && strings.TrimSpace(s) != "" }
func catalogKey(k RuleKey) bool          { return validRuleKey(k) && catalogID(string(k.ID)) }
func catalogQName(q QName) bool {
	return validQName(q) && catalogID(q.Local) && !strings.ContainsRune(q.Local, ':')
}
func catalogDeclaration(d DeclarationID) bool {
	return validDeclaration(d) && catalogID(d.OwnerOccurrence) && catalogID(d.Occurrence)
}
func catalogComponent(c ComponentID) bool {
	return validComponent(c) && ((c.Occurrence != "" && catalogID(c.Occurrence)) || (c.Occurrence == "" && catalogQName(c.Name)))
}
func catalogCompareKeys(a, b RuleKey) int {
	if n := cmp.Compare(a.ID, b.ID); n != 0 {
		return n
	}
	return cmp.Compare(a.Revision, b.Revision)
}
func catalogCompareNames(a, b QName) int {
	if n := cmp.Compare(a.Space, b.Space); n != 0 {
		return n
	}
	return cmp.Compare(a.Local, b.Local)
}
func catalogCompareDeps(a, b Dependency) int {
	if n := cmp.Compare(a.Kind, b.Kind); n != 0 {
		return n
	}
	if n := catalogCompareKeys(a.Rule, b.Rule); n != 0 {
		return n
	}
	if n := cmp.Compare(a.Key, b.Key); n != 0 {
		return n
	}
	return cmp.Compare(a.Relation, b.Relation)
}

func catalogValidateRule(r RuleDefinition) error { return catalogValidateRuleMode(r, true) }

func catalogValidateRuleMode(r RuleDefinition, xmlSelector bool) error {
	if !catalogKey(r.Key) {
		return contract("rule key", "nonempty stable ID and positive revision required")
	}
	switch r.Class {
	case ClassMandatory, ClassRecommendation, ClassInterpretation, ClassDefault, ClassPolicy:
	default:
		return contract("rule class", "unknown class")
	}
	switch r.Role {
	case RolePredicate, RoleContextProvider, RoleAdvisory:
	default:
		return contract("rule role", "unknown role")
	}
	if !validCategory(r.Category) {
		return contract("rule category", "unknown category")
	}
	switch r.Scope {
	case ScopeNode, ScopeSiblings, ScopeMeasure, ScopePart, ScopeDocument, ScopePackage:
	default:
		return contract("rule scope", "unknown scope")
	}
	if len(r.Targets) == 0 {
		return contract("rule targets", "at least one target required")
	}
	targets := make(map[Target]bool, len(r.Targets))
	for _, target := range r.Targets {
		if !validTarget(target) || targets[target] {
			return contract("rule targets", "unknown or duplicate target")
		}
		targets[target] = true
	}
	branches := 0
	if r.Selector.Component != nil {
		branches++
	}
	if r.Selector.Declaration != nil {
		branches++
	}
	if r.Selector.Assembly != nil {
		branches++
	}
	if xmlSelector && branches != 1 {
		return contract("rule selector", "exactly one branch required")
	}
	if r.Selector.Assembly != nil && !validAssembly(*r.Selector.Assembly) {
		return contract("rule selector", "invalid schema assembly")
	}
	if r.Selector.Component != nil && !catalogComponent(*r.Selector.Component) {
		return contract("rule selector", "invalid component identity")
	}
	if r.Selector.Declaration != nil && !catalogDeclaration(*r.Selector.Declaration) {
		return contract("rule selector", "invalid declaration identity")
	}
	if !catalogID(r.Predicate) {
		return contract("rule predicate", "stable plan identity required even when unsupported")
	}
	if !catalogText(r.Text.Title, 4096) || !catalogText(r.Text.Explanation, 4096) || !catalogText(r.Text.Message, 4096) {
		return contract("rule text", "bounded nonempty title, explanation and message required")
	}
	if len(r.Sources) == 0 {
		return contract("rule sources", "provenance required")
	}
	for _, source := range r.Sources {
		if len(source.IDs) == 0 || !catalogText(source.URL, 4096) || !catalogText(source.Locator, 4096) {
			return contract("rule source", "IDs, URL and locator required")
		}
		ids := make(map[string]bool, len(source.IDs))
		for _, id := range source.IDs {
			if !catalogID(id) || ids[id] {
				return contract("rule source IDs", "invalid or duplicate provenance identity")
			}
			ids[id] = true
		}
	}
	deps := make(map[Dependency]bool, len(r.Requires))
	for _, dep := range r.Requires {
		switch dep.Relation {
		case RelationSameNode, RelationParent, RelationDocument, RelationPart, RelationMusicalPosition:
		default:
			return contract("dependency relation", "unknown relation")
		}
		switch dep.Kind {
		case DependencyRule:
			if !catalogKey(dep.Rule) || dep.Key != "" {
				return contract("rule dependency", "exact rule key and no capability key required")
			}
		case DependencyFact, DependencyExternal:
			if !catalogID(dep.Key) || dep.Rule != (RuleKey{}) {
				return contract("capability dependency", "capability key and no rule key required")
			}
		default:
			return contract("dependency kind", "unknown kind")
		}
		if deps[dep] {
			return contract("rule dependencies", "duplicate dependency")
		}
		deps[dep] = true
	}
	return nil
}

func catalogValidateProfile(p Profile) error { return catalogValidateProfileMode(p, true) }

func catalogValidateProfileMode(p Profile, xmlRoots bool) error {
	if !catalogID(string(p.ID)) || !catalogID(p.MusicXMLVersion) || !catalogID(p.RegistryCommit) {
		return contract("profile identity", "ID, version and registry pin required")
	}
	if !validText(p.CatalogDigest, 64) {
		return contract("profile catalog digest", "invalid digest")
	}
	if !validTarget(p.Target) {
		return contract("profile target", "unknown target")
	}
	if !catalogText(p.NormativeClosure, 16384) {
		return contract("profile closure", "bounded explicit closure required")
	}
	if len(p.Required) == 0 {
		return contract("profile required", "at least one obligation required")
	}
	required := make(map[RuleKey]bool, len(p.Required))
	for _, key := range p.Required {
		if !catalogKey(key) || required[key] {
			return contract("profile required", "invalid or duplicate exact rule revision")
		}
		required[key] = true
	}
	if xmlRoots && len(p.SupportedRoots) == 0 {
		return contract("profile roots", "at least one explicit root required")
	}
	roots := make(map[QName]bool, len(p.SupportedRoots))
	for _, root := range p.SupportedRoots {
		if !catalogQName(root) || roots[root] {
			return contract("profile roots", "invalid or duplicate root")
		}
		roots[root] = true
	}
	for _, support := range p.Support {
		if !catalogKey(support.Rule) {
			return contract("profile support", "invalid rule identity")
		}
		switch support.State {
		case ImplementationAvailable:
			if support.Reason != "" {
				return contract("profile support", "available support cannot carry an unavailable reason")
			}
		case ImplementationUnsupported:
			if !catalogID(string(support.Reason)) {
				return contract("profile support", "unsupported support requires stable reason")
			}
		default:
			return contract("profile support", "unknown implementation state")
		}
	}
	return nil
}

func catalogValidateRelations(rule RuleDefinition, rules map[RuleKey]RuleDefinition) error {
	for _, dep := range rule.Requires {
		if dep.Kind != DependencyRule {
			continue
		}
		prerequisite := rules[dep.Rule]
		// Relations whose runtime service is unavailable remain explicit. Their
		// scopes cannot be projected onto node/document relations silently.
		if dep.Relation == RelationPart || dep.Relation == RelationMusicalPosition {
			continue
		}
		if prerequisite.Scope != ScopeNode && prerequisite.Scope != ScopeDocument {
			continue
		}
		switch dep.Relation {
		case RelationSameNode:
			if rule.Scope != prerequisite.Scope {
				return contract("dependency scope", "same-node relation requires equal node/document scopes")
			}
		case RelationParent:
			if rule.Scope != ScopeNode || prerequisite.Scope != ScopeNode {
				return contract("dependency scope", "parent relation requires node scopes")
			}
		}
	}
	return nil
}

// An iterative DFS bounds call-stack use even for the maximum-length DAG.
func catalogOrder(rules map[RuleKey]RuleDefinition) ([]RuleKey, error) {
	keys := make([]RuleKey, 0, len(rules))
	dependencies := make(map[RuleKey][]RuleKey, len(rules))
	for key, rule := range rules {
		keys = append(keys, key)
		for _, dep := range rule.Requires {
			if dep.Kind != DependencyRule {
				continue
			}
			if _, ok := rules[dep.Rule]; !ok {
				return nil, contract("rule dependency", "unresolved exact rule revision")
			}
			dependencies[key] = append(dependencies[key], dep.Rule)
		}
	}
	return catalogSortGraph(keys, dependencies, catalogCompareKeys)
}

func catalogSortGraph[T comparable](keys []T, dependencies map[T][]T, compare func(T, T) int) ([]T, error) {
	for _, key := range keys {
		slices.SortFunc(dependencies[key], compare)
	}
	slices.SortFunc(keys, compare)
	state := make(map[T]uint8, len(keys))
	order := make([]T, 0, len(keys))
	type frame struct {
		key  T
		next int
	}
	stack := make([]frame, 0)
	for _, key := range keys {
		if state[key] != 0 {
			continue
		}
		state[key] = 1
		stack = append(stack, frame{key: key})
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			deps := dependencies[top.key]
			if top.next == len(deps) {
				state[top.key] = 2
				order = append(order, top.key)
				stack = stack[:len(stack)-1]
				continue
			}
			dep := deps[top.next]
			top.next++
			switch state[dep] {
			case 1:
				return nil, contract("rule dependencies", "runtime dependency cycle")
			case 0:
				state[dep] = 1
				stack = append(stack, frame{key: dep})
			}
		}
	}
	return order, nil
}

// Context node-zero publication and lookup are disjoint from real node scopes.
// Same-node and parent publication/lookup can overlap, so both belong to the
// conservative local ordering domain. This does not infer instance applicability.
type catalogFactBinding struct {
	key      string
	document bool
}

func catalogComparePhases(a, b catalogPhase) int {
	if n := catalogCompareKeys(a.rule, b.rule); n != 0 {
		return n
	}
	if a.registration == b.registration {
		return 0
	}
	if a.registration {
		return -1
	}
	return 1
}

func catalogProfileOrder(c *CompiledCatalog, closure map[RuleKey]bool, budget *catalogBudget) ([]catalogPhase, error) {
	contextProviders := make(map[catalogFactBinding][]RuleKey)
	identityProviders := make(map[string][]RuleKey)
	declaring := make(map[RuleKey]bool)
	phases := make([]catalogPhase, 0, len(closure))
	for key := range closure {
		phases = append(phases, catalogPhase{rule: key})
		rule := c.rules[key]
		plan, ok := c.plans[rule.Predicate]
		if !ok {
			continue
		}
		switch plan.kind {
		case planContextValue, planContextDefault:
			binding := catalogFactBinding{key: plan.key, document: plan.relation == RelationDocument}
			contextProviders[binding] = append(contextProviders[binding], key)
		case planIdentityDeclare:
			identityProviders[plan.key] = append(identityProviders[plan.key], key)
			declaring[key] = true
			phases = append(phases, catalogPhase{rule: key, registration: true})
		}
	}
	dependencies := make(map[catalogPhase][]catalogPhase, len(phases))
	seen := make(map[catalogPhase]map[catalogPhase]bool)
	add := func(consumer, provider catalogPhase) error {
		if seen[consumer][provider] {
			return nil
		}
		budget.slots(1)
		if !budget.ok() {
			return contract("catalog schedule", "total profile ordering phase-edge limit exceeded")
		}
		if seen[consumer] == nil {
			seen[consumer] = make(map[catalogPhase]bool)
		}
		seen[consumer][provider] = true
		dependencies[consumer] = append(dependencies[consumer], provider)
		return nil
	}
	for key := range closure {
		rule := c.rules[key]
		// A declaration must meet its original prerequisites before collecting
		// any keys. Other rule families have only their evaluation phase.
		consumer := catalogPhase{rule: key, registration: declaring[key]}
		reads := make(map[catalogFactBinding]bool)
		for _, dep := range rule.Requires {
			if dep.Kind == DependencyRule {
				if err := add(consumer, catalogPhase{rule: dep.Rule}); err != nil {
					return nil, err
				}
			}
			if dep.Kind == DependencyFact && validLocalRelation(dep.Relation) {
				reads[catalogFactBinding{key: dep.Key, document: dep.Relation == RelationDocument}] = true
			}
		}
		if plan, ok := c.plans[rule.Predicate]; ok {
			if plan.kind == planDictionary {
				for prerequisite := range closure {
					provider := c.plans[c.rules[prerequisite].Predicate]
					if provider.kind == planXSD && provider.schema.assembly == plan.dictionary.Schema.assembly {
						if err := add(consumer, catalogPhase{rule: prerequisite}); err != nil {
							return nil, err
						}
					}
				}
			}
			if resourceLinkPlan(plan.kind) || resourceHintPlan(plan.kind) {
				for prerequisite := range closure {
					provider := c.plans[c.rules[prerequisite].Predicate]
					if (resourceLinkPlan(plan.kind) && provider.kind == planTypedReference) || (provider.kind == planXSD && provider.schema.assembly == plan.context.Schema.assembly) {
						if err := add(consumer, catalogPhase{rule: prerequisite}); err != nil {
							return nil, err
						}
					}
				}
			}
			if plan.kind == planTimeline {
				for _, owner := range plan.timeline.PartOwners {
					if err := add(consumer, catalogPhase{rule: owner.Rule}); err != nil {
						return nil, err
					}
				}
			}
			if plan.kind == planContextCompare {
				reads[catalogFactBinding{key: plan.key, document: plan.relation == RelationDocument}] = true
			}
			if plan.kind == planIdentityDeclare || plan.kind == planIdentityReference {
				verdict := catalogPhase{rule: key}
				for _, provider := range identityProviders[plan.key] {
					// Every declaration verdict must observe collection from
					// every declaring plan, not merely from its own subjects.
					if err := add(verdict, catalogPhase{rule: provider, registration: true}); err != nil {
						return nil, err
					}
					if plan.kind == planIdentityReference {
						if err := add(verdict, catalogPhase{rule: provider}); err != nil {
							return nil, err
						}
					}
				}
			}
		}
		for read := range reads {
			for _, provider := range contextProviders[read] {
				if err := add(consumer, catalogPhase{rule: provider}); err != nil {
					return nil, err
				}
			}
		}
	}
	return catalogSortGraph(phases, dependencies, catalogComparePhases)
}

func catalogClosure(required []RuleKey, rules map[RuleKey]RuleDefinition) (map[RuleKey]bool, error) {
	closure := make(map[RuleKey]bool)
	stack := append([]RuleKey(nil), required...)
	for len(stack) > 0 {
		key := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if closure[key] {
			continue
		}
		rule, ok := rules[key]
		if !ok {
			return nil, contract("profile required", "unresolved exact rule revision")
		}
		closure[key] = true
		for _, dep := range rule.Requires {
			if dep.Kind == DependencyRule && !closure[dep.Rule] {
				stack = append(stack, dep.Rule)
			}
		}
	}
	return closure, nil
}

func catalogDigest(c *CompiledCatalog) (string, error) {
	type digestPlan struct {
		ID    string
		Value any
	}
	artifact := struct {
		Format   string
		Rules    []RuleDefinition
		Profiles []Profile
		Plans    []digestPlan
	}{Format: "validation-catalog-v1"}
	for _, rule := range c.rules {
		owned := cloneRule(rule)
		slices.Sort(owned.Targets)
		slices.SortFunc(owned.Requires, catalogCompareDeps)
		for i := range owned.Sources {
			slices.Sort(owned.Sources[i].IDs)
		}
		slices.SortFunc(owned.Sources, func(a, b SourceRef) int {
			if n := cmp.Compare(a.URL, b.URL); n != 0 {
				return n
			}
			if n := cmp.Compare(a.Locator, b.Locator); n != 0 {
				return n
			}
			return slices.Compare(a.IDs, b.IDs)
		})
		artifact.Rules = append(artifact.Rules, owned)
	}
	slices.SortFunc(artifact.Rules, func(a, b RuleDefinition) int { return catalogCompareKeys(a.Key, b.Key) })
	for _, profile := range c.profiles {
		owned := cloneProfile(profile)
		owned.CatalogDigest = ""
		slices.SortFunc(owned.Required, catalogCompareKeys)
		slices.SortFunc(owned.Support, func(a, b RuleSupport) int { return catalogCompareKeys(a.Rule, b.Rule) })
		slices.SortFunc(owned.SupportedRoots, catalogCompareNames)
		artifact.Profiles = append(artifact.Profiles, owned)
	}
	slices.SortFunc(artifact.Profiles, func(a, b Profile) int { return cmp.Compare(a.ID, b.ID) })
	for id, plan := range c.plans {
		artifact.Plans = append(artifact.Plans, digestPlan{ID: id, Value: plan.digestValue()})
	}
	slices.SortFunc(artifact.Plans, func(a, b digestPlan) int { return cmp.Compare(a.ID, b.ID) })
	encoded, err := json.Marshal(artifact)
	if err != nil {
		return "", contract("catalog digest", "plan cannot be canonically encoded")
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

// Check collection lengths before traversing or copying them; the budget also
// bounds the validation/digest work attributable to untrusted configuration.
func catalogCheckBounds(rules []RuleDefinition, profiles []Profile, plans []PlanBinding) error {
	return catalogCheckBoundsRemaining(rules, profiles, plans, catalogMaxBytes)
}
func catalogCheckBoundsRemaining(rules []RuleDefinition, profiles []Profile, plans []PlanBinding, remaining int64) error {
	if len(rules) > catalogMaxRules || len(plans) > catalogMaxRules || len(profiles) > catalogMaxProfiles {
		return contract("catalog size", "top-level collection limit exceeded")
	}
	b := catalogBudget{remaining: remaining}
	b.slots(len(rules))
	b.slots(len(profiles))
	b.slots(len(plans))
	for _, r := range rules {
		if !b.ok() {
			break
		}
		b.identity(string(r.Key.ID))
		b.identity(string(r.Class))
		b.identity(string(r.Role))
		b.identity(string(r.Category))
		b.identity(string(r.Scope))
		b.identity(r.Predicate)
		b.text(r.Text.Title, 4096)
		b.text(r.Text.Explanation, 4096)
		b.text(r.Text.Message, 4096)
		b.slots(len(r.Targets))
		b.slots(len(r.Requires))
		b.slots(len(r.Sources))
		if !b.ok() {
			break
		}
		for _, target := range r.Targets {
			b.identity(string(target))
		}
		if a := r.Selector.Assembly; a != nil {
			b.identity(string(*a))
		}
		if d := r.Selector.Declaration; d != nil {
			b.declaration(*d)
		}
		if c := r.Selector.Component; c != nil {
			b.identity(string(c.Assembly))
			b.identity(string(c.Kind))
			b.name(c.Name)
			b.identity(c.Occurrence)
		}
		for _, dep := range r.Requires {
			if !b.ok() {
				break
			}
			b.identity(string(dep.Kind))
			b.identity(string(dep.Relation))
			b.identity(string(dep.Rule.ID))
			b.identity(dep.Key)
		}
		for _, source := range r.Sources {
			if !b.ok() {
				break
			}
			b.slots(len(source.IDs))
			b.text(source.URL, 4096)
			b.text(source.Locator, 4096)
			if !b.ok() {
				break
			}
			for _, id := range source.IDs {
				if !b.ok() {
					break
				}
				b.identity(id)
			}
		}
	}
	for _, p := range profiles {
		if !b.ok() {
			break
		}
		b.identity(string(p.ID))
		b.identity(p.MusicXMLVersion)
		b.text(p.CatalogDigest, 64)
		b.identity(p.RegistryCommit)
		b.identity(string(p.Target))
		b.text(p.NormativeClosure, 16384)
		b.slots(len(p.Required))
		b.slots(len(p.Support))
		b.slots(len(p.SupportedRoots))
		if !b.ok() {
			break
		}
		for _, key := range p.Required {
			if !b.ok() {
				break
			}
			b.identity(string(key.ID))
		}
		for _, support := range p.Support {
			if !b.ok() {
				break
			}
			b.identity(string(support.Rule.ID))
			b.identity(string(support.State))
			b.identity(string(support.Reason))
		}
		for _, root := range p.SupportedRoots {
			if !b.ok() {
				break
			}
			b.name(root)
		}
	}
	seenSchemas := make(map[string]bool)
	// Context plans really share this immutable owned table through snapshots.
	// Count pointer identity only: equal hashes on distinct retained tables do
	// not permit either allocation to escape the finite configuration budget.
	var seenContextBindings map[*contextBindings]bool
	for _, binding := range plans {
		if !b.ok() {
			break
		}
		b.identity(binding.ID)
		if err := binding.Plan.valid(); err != nil {
			return err
		}
		if schema := binding.Plan.boundSchema(); schema != nil {
			key := string(schema.assembly) + ":" + schema.Digest()
			if !seenSchemas[key] {
				b.take(schema.kernel.LogicalBytes())
				if !b.ok() {
					break
				}
				seenSchemas[key] = true
			}
		}
		size := binding.Plan.configSize()
		var table *contextBindings
		if p := binding.Plan.context; p != nil {
			table = p.Bindings
		}
		if p := binding.Plan.dictionary; p != nil {
			table = p.Bindings
		}
		if table != nil {
			if seenContextBindings[table] {
				size -= table.size()
			} else {
				if seenContextBindings == nil {
					b.take(128)
				}
				b.slots(1)
				if !b.ok() {
					break
				}
				if seenContextBindings == nil {
					seenContextBindings = make(map[*contextBindings]bool)
				}
				seenContextBindings[table] = true
			}
		}
		b.take(size)
	}
	if !b.ok() {
		return contract("catalog size", "bounded string, collection or total configuration limit exceeded")
	}
	return nil
}

type catalogBudget struct{ remaining int64 }

func (b *catalogBudget) ok() bool { return b.remaining >= 0 }
func (b *catalogBudget) take(n int64) {
	if n < 0 || n > b.remaining {
		b.remaining = -1
		return
	}
	b.remaining -= n
}
func (b *catalogBudget) slots(n int) {
	if !b.ok() || int64(n) > b.remaining/catalogSlotBytes {
		b.remaining = -1
		return
	}
	b.take(int64(n) * catalogSlotBytes)
}
func (b *catalogBudget) text(s string, max int) {
	if len(s) > max {
		b.remaining = -1
		return
	}
	b.take(int64(len(s)))
}
func (b *catalogBudget) identity(s string) { b.text(s, 1024) }
func (b *catalogBudget) name(q QName)      { b.text(q.Space, 4096); b.identity(q.Local) }
func (b *catalogBudget) declaration(d DeclarationID) {
	b.identity(string(d.Assembly))
	b.identity(d.OwnerOccurrence)
	b.identity(d.Occurrence)
}
