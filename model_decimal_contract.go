package musicxml

import (
	"strings"

	"github.com/go-muse/go-musicxml/validation"
)

// modelDecimalContract names how model assessment reads float64 decimal
// fields. It is part of every current-model profile's closure, so that the
// published profile identity (catalog digest and Profile JSON) distinguishes
// this numeric contract from earlier ones.
const modelDecimalContract = "Finite float64 decimal fields mean the shortest decimal that reads back to the same float64."

// newCatalog builds a catalog after naming the model decimal contract in the
// closure of every current-model profile, including those whose closure a
// composition replaced. It edits profiles in place: callers pass definitions
// they built fresh for this catalog.
func newCatalog(rules []validation.RuleDefinition, profiles []validation.Profile, plans []validation.PlanBinding) (*validation.CompiledCatalog, error) {
	for i := range profiles {
		if profiles[i].Target == validation.TargetModel && !strings.Contains(profiles[i].NormativeClosure, modelDecimalContract) {
			profiles[i].NormativeClosure = strings.TrimSpace(profiles[i].NormativeClosure + " " + modelDecimalContract)
		}
	}
	return validation.NewCatalog(rules, profiles, plans)
}
