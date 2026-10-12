package validation

func performanceLocal(r *contextReader, key RuleKey, n *factNode, out *TimingContextResult) bool {
	publish := false
	if r.matches(n, "perf-fret") {
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
	if r.matches(n, "perf-level") {
		publish = true
		out.Kind = "editorial-level"
		out.EditorialPlaybackSuppressed = stateWithDefault(stateYesNo(r.attribute(n, "perf-level-reference")), false, "editorial-reference-no").Effective
	}
	if r.matches(n, "perf-root-alter") {
		publish = true
		out.Kind = "root-alter"
		out.RootAlterPrintObject = stateWithDefault(stateYesNo(r.attribute(n, "perf-root-print")), true, "print-object-yes").Effective
	}
	for _, role := range []contextRole{"perf-empty-trill-sound-beats", "perf-horizontal-turn-beats", "perf-wavy-line-beats"} {
		if r.attributeOwner(n, role) {
			publish = true
			out.Kind = "trill"
			out.TrillIncludesInitialSound = Evidence[bool]{State: EvidenceKnown, Value: true}
			out.TrillExcludesClosingTurn = Evidence[bool]{State: EvidenceKnown, Value: true}
			break
		}
	}
	if r.matches(n, "perf-prefix") {
		publishPerformanceScalar(r, key, n, "performance-figure-prefix", r.scalar(n))
	}
	if r.matches(n, "perf-suffix") {
		publishPerformanceScalar(r, key, n, "performance-figure-suffix", r.scalar(n))
	}
	if r.matches(n, "perf-accidental-mark") {
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
						out.LayoutOrigin = stateUnavailable[DeclarationID, NodeRef](ch.header.Declaration)
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
