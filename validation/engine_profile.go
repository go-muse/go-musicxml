package validation

// Profile returns an independently owned compiled profile without beginning a
// session or consuming any run budget. It supports fixing intrinsic profile
// identity in a package catalog, never mutates the engine, and is concurrency safe.
func (e *Engine) Profile(id ProfileID) (Profile, bool) {
	if e == nil || e.catalog == nil {
		return Profile{}, false
	}
	return e.catalog.Profile(id)
}
