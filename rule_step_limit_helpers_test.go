package musicxml

import (
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// ruleStepLimit returns the account's current rule-step maximum, which grows
// with charged nodes under the default limit.
func ruleStepLimit(t testing.TB, a *validation.Account) int64 {
	t.Helper()
	limit, err := a.Limit(validation.BudgetRuleSteps)
	if err != nil {
		t.Fatal(err)
	}
	return limit
}
