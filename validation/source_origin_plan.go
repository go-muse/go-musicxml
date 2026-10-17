package validation

import "strings"

const planSourceOrigin planKind = "source-origin"

// NewSourceOriginPlan selects the bounded source-origin contract for one exact
// schema assembly. It consumes typed adapter facts, never XML or callbacks.
func NewSourceOriginPlan(assembly Assembly) (Plan, error) {
	return checkedPlan(Plan{kind: planSourceOrigin, sourceAssembly: assembly})
}
func (p Plan) validateSourceOriginFor(r RuleDefinition) error {
	if err := p.valid(); err != nil {
		return err
	}
	if r.Scope != ScopeDocument || r.Role != RoleContextProvider || r.Category != CategoryNormative || r.Class != ClassMandatory || len(r.Requires) != 0 || r.Selector.Assembly == nil || *r.Selector.Assembly != p.sourceAssembly || len(r.Targets) != 1 || r.Targets[0] != TargetSource {
		return contract("source origin plan", "source-only mandatory document provider with exact assembly and no prerequisites required")
	}
	return nil
}
func validateSourceOriginProfile(c *CompiledCatalog, profile Profile, closure map[RuleKey]bool, supports map[RuleKey]RuleSupport, schemas map[Assembly]*CompiledSchema) error {
	for key := range closure {
		p := c.plans[c.rules[key].Predicate]
		if p.kind != planSourceOrigin || supports[key].State != ImplementationAvailable {
			continue
		}
		if schemas[p.sourceAssembly] == nil {
			return contract("source origin profile", "matching selected compiled schema required")
		}
		if c.sourceOrigins[profile.ID][p.sourceAssembly] != (RuleKey{}) {
			return contract("source origin profile", "one available provider per assembly required")
		}
		if c.sourceOrigins == nil {
			c.sourceOrigins = make(map[ProfileID]map[Assembly]RuleKey)
		}
		if c.sourceOrigins[profile.ID] == nil {
			c.sourceOrigins[profile.ID] = make(map[Assembly]RuleKey)
		}
		c.sourceOrigins[profile.ID][p.sourceAssembly] = key
	}
	return nil
}

// SourceOriginProvider returns the exact available sealed provider selected for
// this source profile and assembly. Like Profile, false includes unknown IDs.
// Catalog selection is independent of the caller's editable profile snapshots.
//
// Adapter API: the root musicxml package uses this to stream facts into an
// engine; application code is not expected to call it.
func (e *Engine) SourceOriginProvider(profile ProfileID, assembly Assembly) (RuleKey, bool) {
	if e == nil || e.catalog == nil {
		return RuleKey{}, false
	}
	key := e.catalog.sourceOrigins[profile][assembly]
	if key == (RuleKey{}) {
		return RuleKey{}, false
	}
	return RuleKey{ID: RuleID(strings.Clone(string(key.ID))), Revision: key.Revision}, true
}
