package validation

import (
	"context"
	"math/big"
)

// Bounds are observed in-measure current locations, not actual measure spans.
// They can prove containment and lower bounds on later measure origins only.
const timelineFrameReserveBytes int64 = 512

type timelineFrame struct {
	Part, Measure NodeRef
	Ordinal       uint64
}

func frameOf(p musicalPosition) timelineFrame {
	return timelineFrame{p.Part, p.Measure, p.MeasureOrdinal}
}

func (h *timelineState) observeFrameOrigin(ctx context.Context, s *Session, p musicalPosition, monotone bool) error {
	if err := s.step(ctx, 1+int64(len(p.Part.DocumentID))+int64(len(p.Measure.DocumentID))); err != nil {
		return err
	}
	if p.UnprovedFrame || p.Offset.value.Sign() < 0 {
		return nil
	}
	key := frameOf(p)
	previous, found := h.frameBounds[key]
	if found && !monotone && p.Offset.value != previous.value {
		cmp, err := s.rationalCmp(ctx, p.Offset, previous)
		if err != nil || cmp <= 0 {
			return err
		}
	} else if !found {
		cost, err := rationalSum(BudgetBufferedBytes, timelineFrameReserveBytes, int64(len(p.Part.DocumentID)), int64(len(p.Measure.DocumentID)))
		if err != nil {
			return err
		}
		if err = s.retain(cost); err != nil {
			return err
		}
		if h.frameBounds == nil {
			h.frameBounds = make(map[timelineFrame]Rational)
		}
	}
	h.frameBounds[key] = p.Offset
	return nil
}
func (h *timelineState) frameMember(ctx context.Context, s *Session, p musicalPosition) (bool, error) {
	if err := s.step(ctx, 1); err != nil {
		return false, err
	}
	if !p.UnprovedFrame {
		return true, nil
	}
	if p.Offset.value.Sign() < 0 {
		return false, nil
	}
	if p.Offset.value.Sign() == 0 {
		return true, nil
	}
	if err := s.step(ctx, int64(len(p.Part.DocumentID))+int64(len(p.Measure.DocumentID))); err != nil {
		return false, err
	}
	bound, ok := h.frameBounds[frameOf(p)]
	if !ok {
		return false, nil
	}
	cmp, err := s.rationalCmp(ctx, p.Offset, bound)
	return cmp <= 0, err
}
func (h *timelineState) proveFramePosition(ctx context.Context, s *Session, p *Evidence[musicalPosition]) error {
	if p.State != EvidenceKnown || !p.Value.UnprovedFrame {
		return nil
	}
	member, err := h.frameMember(ctx, s, p.Value)
	if err == nil && member {
		p.Value.UnprovedFrame = false
	}
	return err
}

// A relation is a set of remaining chronological possibilities. In particular
// <= is not strict dominance. Canonical score precedence remains established
// for two contained positions, including before/after-barline boundary scope.
type statePositionOrder uint8

const (
	stateBefore statePositionOrder = 1 << iota
	stateEqual
	stateAfter
	stateUnordered = stateBefore | stateEqual | stateAfter
)

func (o statePositionOrder) reversed() statePositionOrder {
	return o&stateEqual | (o&stateBefore)<<2 | (o&stateAfter)>>2
}
func (o statePositionOrder) possiblyApplies(inclusive bool) bool {
	return o&stateBefore != 0 || inclusive && o&stateEqual != 0
}
func (o statePositionOrder) definitelyApplies(inclusive bool) bool {
	return o == stateBefore || inclusive && o&stateAfter == 0
}

// Compare a with b+c without retaining temporary arithmetic as session-lifetime
// outputs. Both exact operations reserve bounded scratch/work before execution.
func stateCompareSum(ctx context.Context, s *Session, a, b, c Rational) (int, error) {
	release, err := s.rationalReserve(ctx, rationalAddOp, b.value, c.value, b.ByteSize(), c.ByteSize())
	if err != nil {
		return 0, err
	}
	defer release()
	sum := new(big.Rat).Add(b.value, c.value)
	if err = rationalPoll(ctx); err != nil {
		return 0, err
	}
	bytes := magnitudeBytes(sum.Num()) + magnitudeBytes(sum.Denom())
	releaseCompare, err := s.rationalReserve(ctx, rationalCmpOp, a.value, sum, a.ByteSize(), bytes)
	if err != nil {
		return 0, err
	}
	defer releaseCompare()
	cmp := a.value.Cmp(sum)
	if err = rationalPoll(ctx); err != nil {
		return 0, err
	}
	return cmp, nil
}
func (h *timelineState) positionOrder(ctx context.Context, s *Session, a, b musicalPosition) (statePositionOrder, error) {
	same, err := stateSameRef(ctx, s, a.Part, b.Part)
	if err != nil {
		return 0, err
	}
	if !same {
		return 0, contract("timeline position", "different canonical parts")
	}
	if a.MeasureOrdinal == b.MeasureOrdinal {
		cmp, err := stateComparePosition(ctx, s, a, b)
		if err != nil {
			return 0, err
		}
		if cmp < 0 {
			return stateBefore, nil
		}
		if cmp > 0 {
			return stateAfter, nil
		}
		return stateEqual, nil
	}
	if a.MeasureOrdinal > b.MeasureOrdinal {
		order, err := h.positionOrder(ctx, s, b, a)
		return order.reversed(), err
	}
	am, err := h.frameMember(ctx, s, a)
	if err != nil {
		return 0, err
	}
	bm, err := h.frameMember(ctx, s, b)
	if err != nil {
		return 0, err
	}
	if am && bm {
		return stateBefore, nil
	}
	// Later origin distance is at least a witnessed location in the earlier
	// frame. No upper bound or actual span follows from this witness.
	if err := s.step(ctx, int64(len(a.Part.DocumentID))+int64(len(a.Measure.DocumentID))); err != nil {
		return 0, err
	}
	lower, ok := h.frameBounds[frameOf(a)]
	if !ok {
		lower = timelineZero
	}
	cmp, err := stateCompareSum(ctx, s, a.Offset, lower, b.Offset)
	if err != nil {
		return 0, err
	}
	if cmp < 0 {
		return stateBefore, nil
	}
	if cmp == 0 {
		return stateBefore | stateEqual, nil
	}
	// A contained earlier position is at/before the later structural start,
	// independently of the numerical lower bound currently retained above.
	if am && b.Offset.value.Sign() >= 0 {
		if b.Offset.value.Sign() > 0 {
			return stateBefore, nil
		}
		return stateBefore | stateEqual, nil
	}
	return stateUnordered, nil
}
