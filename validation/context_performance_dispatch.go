package validation

// This finite dispatch retains no per-node maps. The shared reader charges the
// exact declaration / owner lookup and role traversal before returning slices.
type performanceNodeRoles uint32

const (
	performanceNote performanceNodeRoles = 1 << iota
	performanceSound
	performanceMIDIUnpitched
	performancePan
	performanceElevation
	performanceSwingRole
	performanceBend
	performanceFret
	performanceLevel
	performanceRootAlter
	performanceAccidental
	performancePrefix
	performanceSuffix
	performancePluck
	performanceStateRole
	performanceMovementRole
	performanceTrill
	performanceFingering
)

var performanceElementDispatch = map[contextRole]performanceNodeRoles{
	"perf-note": performanceNote, "perf-sound": performanceSound, "perf-midi-unpitched": performanceMIDIUnpitched,
	"perf-pan": performancePan, "perf-elevation": performanceElevation, "perf-swing": performanceSwingRole,
	"perf-bend": performanceBend, "perf-fret": performanceFret, "perf-level": performanceLevel,
	"perf-root-alter": performanceRootAlter, "perf-accidental-mark": performanceAccidental,
	"perf-prefix": performancePrefix, "perf-suffix": performanceSuffix, "perf-pluck": performancePluck,
	"perf-score-part": performanceStateRole, "perf-key": performanceStateRole,
	"perf-time": performanceStateRole, "perf-transpose": performanceStateRole, "perf-staff-details": performanceStateRole,
	"perf-backup": performanceMovementRole, "perf-forward": performanceMovementRole,
}

func performanceRoles(elements, attributes []contextRole) performanceNodeRoles {
	var out performanceNodeRoles
	for _, role := range elements {
		out |= performanceElementDispatch[role]
	}
	for _, role := range attributes {
		switch role {
		case "perf-fingering-substitution":
			out |= performanceFingering
		case "perf-empty-trill-sound-beats", "perf-horizontal-turn-beats", "perf-wavy-line-beats":
			out |= performanceTrill
		}
	}
	return out
}
func (r performanceNodeRoles) has(role performanceNodeRoles) bool { return r&role != 0 }
func (r performanceNodeRoles) timing() bool {
	return r & ^(performancePrefix|performanceSuffix|performancePluck|performanceFingering) != 0
}
