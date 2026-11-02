package validation

import "testing"

// ruleStepLimit returns the account's current rule-step maximum, which grows
// with charged nodes under the default limit.
func ruleStepLimit(t testing.TB, a *Account) int64 {
	t.Helper()
	limit, err := a.Limit(BudgetRuleSteps)
	if err != nil {
		t.Fatal(err)
	}
	return limit
}
