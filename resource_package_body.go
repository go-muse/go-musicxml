package musicxml

import (
	"errors"
	"io"

	"github.com/go-muse/go-musicxml/validation"
)

// readExternalPackageBody preserves the transferred-body owner and applies
// legacy MXL caps before retention. Every newly returned body counts, including
// canonical aliases; inner ZIP expansion is a distinct later representation.
func (g *resourceGraphRun) readExternalPackageBody(item graphExpansion, pin validation.ResourcePin, body io.ReadCloser) (*assessmentResourceBody, error) {
	p := g.packages[item.pkg]
	if pin.Representation != validation.ResourceRawMXL && p == nil {
		return readAssessmentResourceBody(g.ctx, g.options.account, body)
	}
	limits, err := g.mxlOptions.limits()
	if err != nil {
		if body != nil {
			err = errors.Join(err, body.Close())
		}
		return nil, err
	}
	limit := min(limits.documentSize, limits.resourceSize)
	if pin.Representation == validation.ResourceRawMXL {
		limit = min(limits.archiveSize, limits.resourceSize)
	}
	if body == nil {
		return readAssessmentResourceBody(g.ctx, g.options.account, nil)
	}
	return readAssessmentResourceBody(g.ctx, g.options.account, &packageCappedBody{body: body, run: g, limit: limit, totalLimit: limits.resourcesSize})
}

type packageCappedBody struct {
	body                    io.ReadCloser
	run                     *resourceGraphRun
	limit, totalLimit, seen int64
}

func (r *packageCappedBody) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	remaining := min(r.limit-r.seen, r.totalLimit-r.run.mxlExpanded)
	if remaining < 0 {
		return 0, ErrMXLTooLarge
	}
	// At most one byte beyond admission is requested to distinguish exact EOF.
	want := len(p)
	if int64(want) > remaining {
		want = int(remaining) + 1
	}
	n, err := r.body.Read(p[:want])
	if n < 0 || n > want {
		return 0, errors.Join(&validation.ConfigurationError{Field: "resource reader", Reason: "invalid capped read count"}, err)
	}
	admitted := min(int64(n), remaining)
	r.seen += admitted
	r.run.mxlExpanded += admitted
	if int64(n) > remaining {
		// The rejected EOF-probe byte is read but never retained. The body
		// owner accounts the returned prefix; this wrapper alone accounts the
		// withheld probe, provided that does not steal the prefix's remaining
		// shared budget. At shared exhaustion the owner still admits its full
		// allowed prefix, and we preserve the extra byte's limit cause.
		a := r.run.options.account
		used, usageErr := a.Usage(validation.BudgetExternalBytes)
		limit := a.Limits().MaxExternalBytes
		var probeErr error
		if usageErr != nil {
			probeErr = usageErr
		} else if int64(n) <= limit-used {
			probeErr = errors.Join(a.Charge(validation.BudgetExternalBytes, int64(n)-admitted), a.Charge(validation.BudgetRuleSteps, int64(n)-admitted))
		} else {
			probeErr = &validation.ResourceError{Kind: validation.BudgetExternalBytes, Limit: limit, Used: used + min(admitted, limit-used), Requested: int64(n) - min(admitted, limit-used)}
		}
		return int(admitted), errors.Join(ErrMXLTooLarge, probeErr, err)
	}
	return n, err
}
func (r *packageCappedBody) Close() error { return r.body.Close() }
