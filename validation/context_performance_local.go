package validation

func performanceLocalRoles(r *contextReader, key RuleKey, n *factNode, out *TimingContextResult, roles performanceNodeRoles) bool {
	if r.err != nil {
		return false
	}
	publish := false
	// This conditional role does not assert that the current instrument is fretted.
	if roles.has(performanceFingering) {
		publishPerformanceScalar(r, key, n, "performance-fretted-instrument-role", Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "stopping-finger"}})
	}
	if roles.has(performancePluck) {
		publishPerformanceScalar(r, key, n, "performance-fretted-instrument-role", Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "plucking-finger"}})
	}
	if roles.has(performanceFret) {
		publish = true
		out.Kind = "fret"
		out.FretNumber = stateInteger(r.scalar(n))
		out.FretOpenString = stateUnavailable[Integer, bool](out.FretNumber)
		if out.FretNumber.State == EvidenceKnown {
			if !r.step(out.FretNumber.Value.ByteSize()) {
				return publish
			}
			out.FretOpenString = Evidence[bool]{State: EvidenceKnown, Value: out.FretNumber.Value.value.Sign() == 0}
		}
	}
	if roles.has(performanceLevel) {
		publish = true
		out.Kind = "editorial-level"
		out.EditorialPlaybackSuppressed = stateWithDefault(stateYesNo(r.attribute(n, "perf-level-reference")), false, "editorial-reference-no").Effective
	}
	if roles.has(performanceRootAlter) {
		publish = true
		out.Kind = "root-alter"
		out.RootAlterPrintObject = stateWithDefault(stateYesNo(r.attribute(n, "perf-root-print")), true, "print-object-yes").Effective
	}
	if roles.has(performanceTrill) {
		publish = true
		out.Kind = "trill"
		out.TrillIncludesInitialSound = Evidence[bool]{State: EvidenceKnown, Value: true}
		out.TrillExcludesClosingTurn = Evidence[bool]{State: EvidenceKnown, Value: true}
	}
	if roles.has(performancePrefix) {
		publishPerformanceScalar(r, key, n, "performance-figure-prefix", r.scalar(n))
	}
	if roles.has(performanceSuffix) {
		publishPerformanceScalar(r, key, n, "performance-figure-suffix", r.scalar(n))
	}
	if roles.has(performanceAccidental) {
		parent := r.s.nodes[n.header.Parent]
		if r.matches(parent, "perf-ornaments") {
			publish = true
			out.Kind = "ornament-accidental"
			out.LayoutOrigin = stateUnknown[NodeRef]("ornament-origin-unavailable")
			complete := r.complete(parent)
			if complete.State != EvidenceKnown {
				out.LayoutOrigin = stateUnavailable[bool, NodeRef](complete)
			} else {
				var prior *factNode
				for _, ch := range parent.children {
					if !r.step(1) {
						return publish
					}
					if ch == n {
						if prior != nil {
							out.LayoutOrigin = Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{r.s.documentID, prior.header.ID}}
						}
						break
					}
					if ch.header.Declaration.State != EvidenceKnown {
						out.LayoutOrigin = stateUnavailable[DeclarationID, NodeRef](*ch.header.Declaration)
						break
					}
					if !r.matches(ch, "perf-accidental-mark") {
						prior = ch
					}
				}
			}
		}
	}
	return publish
}
