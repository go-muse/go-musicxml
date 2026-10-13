package musicxml

import (
	"errors"
	"fmt"
	"github.com/go-muse/go-musicxml/validation"
)

// intrinsicXMLDepth deliberately applies MXLOptions to each XML representation
// inside an actual MXL boundary, including its outgoing XML dependencies. A plain
// XML root or sibling remains governed by Graph.Budgets only, even if another
// branch of that run has entered a nested MXL. Package/graph hop depth is separate.
func (g *resourceGraphRun) intrinsicXMLDepth(pkg validation.PackageID) int {
	if g.packages[pkg] == nil {
		return 0
	}
	if g.mxlOptions.MaxXMLDepth != 0 {
		return g.mxlOptions.MaxXMLDepth
	}
	return defaultMaxXMLDepth
}

// checkIntrinsicXMLDepth checks contextual applicability without parsing the
// canonical bytes again or rewriting their complete intrinsic assessment. The
// witness is stored only for reusable successful parses. Refusal belongs to the
// receiving graph context and occurs before its target adoption.
func (g *resourceGraphRun) checkIntrinsicXMLDepth(r *graphResource, pkg validation.PackageID) error {
	if err := g.ctx.Err(); err != nil {
		return err
	}
	if err := g.options.account.Charge(validation.BudgetRuleSteps, 1); err != nil {
		return err
	}
	limit := g.intrinsicXMLDepth(pkg)
	if limit == 0 || r.xmlDepth <= limit {
		return nil
	}
	return errors.Join(fmt.Errorf("%w: maximum is %d elements", ErrXMLTooDeep, limit), &validation.ResourceError{Kind: validation.BudgetDepth, Limit: int64(limit), Used: int64(r.xmlDepth), Requested: 0})
}
