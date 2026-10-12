package validation

import (
	"context"
	"math/big"
)

// This layer moves a measure-relative cursor using already-established exact
// durations. Selecting duration units across disputed divisions boundaries is
// deliberately a separate responsibility; it cannot be smuggled into arithmetic.
type timelineMovementKind uint8

const (
	timelineMovementOther timelineMovementKind = iota + 1
	timelineMovementNote
	timelineMovementBackup
	timelineMovementForward
	timelineMovementGap
)

type timelineMovement struct {
	Subject        NodeRef
	Kind           timelineMovementKind
	Duration       Evidence[Rational]
	Chord, Grace   Evidence[bool]
	Unavailability Evidence[bool]
}

type timelineCursorFact struct {
	Position, Duration, PrincipalDuration Evidence[Rational]
	Principal                             Evidence[NodeRef]
}

type timelineCursor struct {
	Position, principalPosition, principalDuration Evidence[Rational]
	principal                                      Evidence[NodeRef]
}

// This immutable, bounded literal is a coordinate origin, not an implicit
// divisions value or an invented duration for a grace note.
var timelineZero = func() Rational {
	v, err := NewRational(big.NewRat(0, 1))
	if err != nil {
		panic(err)
	}
	return v
}()

func newTimelineCursor() timelineCursor {
	return timelineCursor{
		Position:          Evidence[Rational]{State: EvidenceKnown, Value: timelineZero},
		principal:         Evidence[NodeRef]{State: EvidenceUnknown, Reason: "chord-principal-unavailable"},
		principalPosition: Evidence[Rational]{State: EvidenceUnknown, Reason: "chord-principal-unavailable"},
		principalDuration: Evidence[Rational]{State: EvidenceUnknown, Reason: "chord-principal-unavailable"},
	}
}

func timelineGapAs[T, U any](e Evidence[T], absent Code) Evidence[U] {
	if e.State == EvidenceAbsent {
		return Evidence[U]{State: EvidenceUnknown, Reason: absent}
	}
	if e.State == EvidenceKnown || e.State == "" {
		return Evidence[U]{State: EvidenceUnknown, Reason: "timeline-evidence-unavailable"}
	}
	return Evidence[U]{State: e.State, Reason: e.Reason, Cause: e.Cause}
}

func (s *Session) normalizeTimelineDuration(ctx context.Context, raw, units Evidence[Decimal]) (Evidence[Rational], error) {
	if err := rationalPoll(ctx); err != nil {
		return Evidence[Rational]{}, err
	}
	if err := s.step(ctx, 1); err != nil {
		return Evidence[Rational]{}, err
	}
	if raw.State != EvidenceKnown {
		return timelineGapAs[Decimal, Rational](raw, "duration-unavailable"), nil
	}
	if units.State != EvidenceKnown {
		return timelineGapAs[Decimal, Rational](units, "initial-divisions-unavailable"), nil
	}
	if !raw.Value.Valid() || !units.Value.Valid() {
		return Evidence[Rational]{}, contract("timeline normalization", "initialized exact decimal inputs required")
	}
	if units.Value.value.Sign() == 0 {
		return Evidence[Rational]{State: EvidenceUnsupported, Reason: "zero-division-units"}, nil
	}
	numerator, err := s.rationalFromDecimal(ctx, raw.Value)
	if err != nil {
		return Evidence[Rational]{}, err
	}
	denominator, err := s.rationalFromDecimal(ctx, units.Value)
	if err != nil {
		return Evidence[Rational]{}, err
	}
	value, err := s.rationalQuo(ctx, numerator, denominator)
	if err != nil {
		return Evidence[Rational]{}, err
	}
	return Evidence[Rational]{State: EvidenceKnown, Value: value}, nil
}

func (c *timelineCursor) interrupt(e Evidence[bool]) {
	c.Position = timelineGapAs[bool, Rational](e, "timeline-event-unavailable")
	c.principal = timelineGapAs[bool, NodeRef](e, "chord-principal-unavailable")
	c.principalPosition = timelineGapAs[bool, Rational](e, "chord-principal-unavailable")
	c.principalDuration = timelineGapAs[bool, Rational](e, "chord-principal-unavailable")
}

func (c *timelineCursor) clearPrincipal() {
	c.principal = Evidence[NodeRef]{State: EvidenceUnknown, Reason: "chord-principal-interrupted"}
	c.principalPosition = Evidence[Rational]{State: EvidenceUnknown, Reason: "chord-principal-interrupted"}
	c.principalDuration = Evidence[Rational]{State: EvidenceUnknown, Reason: "chord-principal-interrupted"}
}

// advance commits cursor state only after all required arithmetic succeeds.
// Staff, voice and cue labels do not invent additional chord membership rules.
// A cursor-moving backup/forward makes the older principal relation unavailable;
// this is an evidence gap, not a normative orphan-chord error.
func (c *timelineCursor) advance(ctx context.Context, s *Session, e timelineMovement) (timelineCursorFact, error) {
	if err := rationalPoll(ctx); err != nil {
		return timelineCursorFact{}, err
	}
	if err := s.step(ctx, 1); err != nil {
		return timelineCursorFact{}, err
	}
	next := *c
	fact := timelineCursorFact{
		Position: c.Position, Duration: e.Duration,
		Principal:         Evidence[NodeRef]{State: EvidenceAbsent},
		PrincipalDuration: Evidence[Rational]{State: EvidenceAbsent},
	}
	switch e.Kind {
	case timelineMovementOther:
		fact.Duration = Evidence[Rational]{State: EvidenceAbsent}
	case timelineMovementGap:
		next.interrupt(e.Unavailability)
		fact.Position = next.Position
		fact.Duration = timelineGapAs[bool, Rational](e.Unavailability, "duration-unavailable")
	case timelineMovementBackup, timelineMovementForward:
		next.clearPrincipal()
		if err := next.move(ctx, s, e.Duration, e.Kind == timelineMovementBackup); err != nil {
			return fact, err
		}
	case timelineMovementNote:
		if e.Grace.State == EvidenceKnown && e.Grace.Value {
			fact.Duration = Evidence[Rational]{State: EvidenceAbsent}
		}
		if e.Chord.State != EvidenceKnown {
			next.interrupt(e.Chord)
			fact.Position = next.Position
			fact.Principal = next.principal
			fact.PrincipalDuration = next.principalDuration
			break
		}
		if e.Chord.Value {
			fact.Position, fact.Principal, fact.PrincipalDuration = c.principalPosition, c.principal, c.principalDuration
			break
		}
		next.principal = Evidence[NodeRef]{State: EvidenceKnown, Value: e.Subject}
		next.principalPosition, next.principalDuration = c.Position, fact.Duration
		fact.Principal, fact.PrincipalDuration = next.principal, next.principalDuration
		if e.Grace.State != EvidenceKnown {
			next.Position = timelineGapAs[bool, Rational](e.Grace, "note-branch-unavailable")
		} else if !e.Grace.Value {
			if err := next.move(ctx, s, fact.Duration, false); err != nil {
				return fact, err
			}
		}
	default:
		return fact, contract("timeline cursor", "unknown movement kind")
	}
	*c = next
	return fact, nil
}

func (c *timelineCursor) move(ctx context.Context, s *Session, duration Evidence[Rational], backwards bool) error {
	if c.Position.State != EvidenceKnown {
		return nil
	}
	if duration.State != EvidenceKnown {
		c.Position = timelineGapAs[Rational, Rational](duration, "duration-unavailable")
		return nil
	}
	var value Rational
	var err error
	if backwards {
		value, err = s.rationalSub(ctx, c.Position.Value, duration.Value)
	} else {
		value, err = s.rationalAdd(ctx, c.Position.Value, duration.Value)
	}
	if err != nil {
		return err
	}
	c.Position = Evidence[Rational]{State: EvidenceKnown, Value: value}
	return nil
}
