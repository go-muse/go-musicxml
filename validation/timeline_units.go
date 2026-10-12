package validation

import "context"

func (s *Session) sameTimelineUnits(ctx context.Context, a, b Evidence[Decimal]) (bool, error) {
	if a.State != EvidenceKnown || b.State != EvidenceKnown {
		return false, nil
	}
	if err := s.step(ctx, 1+int64(len(a.Value.text))/4+int64(len(b.Value.text))/4); err != nil {
		return false, err
	}
	return a.Value.text == b.Value.text, nil
}

// This detector does not choose a divisions/rewind reconstruction convention.
// The disputed class remains explicit unavailable evidence. It is deliberately
// separate from exact normalization and from known-position state publication.
func timelineUnitPlacementGap(r *stateReader, block timelineMeasureBlock, inherited Evidence[Decimal]) (Evidence[Decimal], error) {
	current := inherited
	moved, rewound, changed := false, false, false
	for _, event := range block.content.children {
		if !r.step(1) {
			return Evidence[Decimal]{}, r.err
		}
		if r.matches(event, timelineRoleBackup) {
			rewound = true
			moved = true
		}
		if r.matches(event, timelineRoleForward) {
			moved = true
		}
		if r.matches(event, timelineRoleNote) {
			_, chord := r.child(event, timelineRoleNoteChord)
			_, grace := r.child(event, timelineRoleNoteGrace)
			if chord.State == EvidenceAbsent && grace.State == EvidenceAbsent {
				moved = true
			}
		}
		if r.matches(event, timelineRoleAttributes) {
			next := stateDecimalValue(r.childScalar(event, timelineRoleAttributesDivisions))
			if next.State == EvidenceAbsent {
				continue
			}
			same, err := r.s.sameTimelineUnits(r.ctx, current, next)
			if err != nil {
				return Evidence[Decimal]{}, err
			}
			if moved && !same {
				changed = true
			}
			current = next
		}
	}
	if r.err != nil {
		return Evidence[Decimal]{}, r.err
	}
	if rewound && changed {
		return Evidence[Decimal]{State: EvidenceUnsupported, Reason: "divisions-rewind-placement-unresolved"}, nil
	}
	return Evidence[Decimal]{State: EvidenceAbsent}, nil
}
