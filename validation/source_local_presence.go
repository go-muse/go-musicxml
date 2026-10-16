package validation

import "fmt"

// New declaration-aware assemblies cannot silently reinterpret an old local
// constructor's physical-presence contract. This guard is intentionally limited
// to elected executable modes and rules; historical profiles and unsupported
// metadata retain their existing behavior.
func validateSourceLocalPresenceProfile(c *CompiledCatalog, profile Profile, closure map[RuleKey]bool, supports map[RuleKey]RuleSupport) error {
	if profile.Target != TargetSource {
		return nil
	}
	for key := range closure {
		rule := c.rules[key]
		plan := c.plans[rule.Predicate]
		if supports[key].State != ImplementationAvailable || plan.kind != planLocal || plan.local.sourceInfoset {
			continue
		}
		replacement := ""
		switch plan.local.Operation {
		case localAttributeSet:
			replacement = "NewSourceAttributeValuePlan"
		case localPresenceAttribute:
			replacement = "NewSourcePresenceImpliesAttributePlan"
		default:
			continue
		}
		assembly := Assembly("")
		if rule.Selector.Declaration != nil {
			assembly = rule.Selector.Declaration.Assembly
		}
		if rule.Selector.Component != nil {
			assembly = rule.Selector.Component.Assembly
		}
		if c.sourceDeclarations[profile.ID][assembly] != xsdSourceDeclarationsV1 {
			continue
		}
		return fmt.Errorf("catalog rule %q revision %d: %w", key.ID, key.Revision, contract("source local presence", "legacy physical-presence plan is incompatible with declaration-aware source mode; use "+replacement))
	}
	return nil
}
