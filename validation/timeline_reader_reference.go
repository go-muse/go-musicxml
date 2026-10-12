package validation

// sameRef preserves the reader's first interruption while metering canonical
// identity strings before comparing them. Callers retain their existing causal
// fallback, and propagate r.err at the provider boundary.
func (r *stateReader) sameRef(a, b NodeRef) bool {
	if r.err != nil {
		return false
	}
	var same bool
	same, r.err = stateSameRef(r.ctx, r.s, a, b)
	return same
}

func (r *stateReader) validPosition(p musicalPosition) bool {
	if r.err != nil {
		return false
	}
	var valid bool
	valid, r.err = statePositionValid(r.ctx, r.s, p)
	return valid
}

func (r *stateReader) sameMeasure(a, b musicalPosition) bool {
	return a.MeasureOrdinal == b.MeasureOrdinal && r.sameRef(a.Part, b.Part) && r.sameRef(a.Measure, b.Measure)
}
