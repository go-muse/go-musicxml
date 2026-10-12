package validation

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func timelineKnown[T any](value T) Evidence[T] {
	return Evidence[T]{State: EvidenceKnown, Value: value}
}

func timelineMovementFor(t *testing.T, id NodeID, kind timelineMovementKind, n, d int64) timelineMovement {
	t.Helper()
	return timelineMovement{Subject: NodeRef{"score", id}, Kind: kind, Duration: timelineKnown(mustRational(t, n, d)), Chord: timelineKnown(false), Grace: timelineKnown(false)}
}

func TestTimelineCursorExactNormalizedMovement(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	cursor := newTimelineCursor()
	events := []timelineMovement{
		timelineMovementFor(t, 1, timelineMovementNote, 1, 3),
		timelineMovementFor(t, 2, timelineMovementNote, 1, 3),
		timelineMovementFor(t, 3, timelineMovementBackup, 2, 3),
		timelineMovementFor(t, 4, timelineMovementForward, 1, 6),
		timelineMovementFor(t, 5, timelineMovementNote, 1, 2),
	}
	wantStart := []string{"0", "1/3", "2/3", "0", "1/6"}
	wantEnd := []string{"1/3", "2/3", "0", "1/6", "2/3"}
	for i, event := range events {
		fact, err := cursor.advance(context.Background(), s, event)
		if err != nil {
			t.Fatal(err)
		}
		if fact.Position.State != EvidenceKnown || fact.Position.Value.String() != wantStart[i] || cursor.Position.State != EvidenceKnown || cursor.Position.Value.String() != wantEnd[i] {
			t.Fatalf("event %d: start=%+v end=%+v", i, fact.Position, cursor.Position)
		}
	}
}

func TestTimelineCursorPrincipalNotAdjacentTone(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	cursor := newTimelineCursor()
	for i, duration := range []int64{4, 1, 3} {
		event := timelineMovementFor(t, NodeID(i+1), timelineMovementNote, duration, 1)
		event.Chord = timelineKnown(i != 0)
		fact, err := cursor.advance(context.Background(), s, event)
		if err != nil {
			t.Fatal(err)
		}
		if fact.Position.Value.String() != "0" || fact.Principal.State != EvidenceKnown || fact.Principal.Value.Node != 1 || fact.PrincipalDuration.Value.String() != "4" || cursor.Position.Value.String() != "4" {
			t.Fatalf("event %d lost principal: %+v cursor=%+v", i, fact, cursor.Position)
		}
	}
}

func TestTimelineCursorGraceAndInterruption(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	cursor := newTimelineCursor()
	grace := timelineMovementFor(t, 1, timelineMovementNote, 1, 1)
	grace.Grace = timelineKnown(true)
	grace.Duration = Evidence[Rational]{State: EvidenceAbsent}
	if _, err := cursor.advance(context.Background(), s, grace); err != nil {
		t.Fatal(err)
	}
	chord := timelineMovementFor(t, 2, timelineMovementNote, 1, 1)
	chord.Chord = timelineKnown(true)
	fact, err := cursor.advance(context.Background(), s, chord)
	if err != nil || fact.Principal.Value.Node != 1 || fact.PrincipalDuration.State != EvidenceAbsent || cursor.Position.Value.String() != "0" {
		t.Fatalf("grace anchor became invented duration: %+v %v", fact, err)
	}
	if _, err := cursor.advance(context.Background(), s, timelineMovementFor(t, 3, timelineMovementForward, 1, 1)); err != nil {
		t.Fatal(err)
	}
	fact, err = cursor.advance(context.Background(), s, chord)
	if err != nil || fact.Principal.State != EvidenceUnknown || fact.Position.State != EvidenceUnknown || cursor.Position.Value.String() != "1" {
		t.Fatalf("cursor interruption guessed a chord anchor: %+v %v", fact, err)
	}
}

func TestTimelineCursorGapsRemainScoped(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	cursor := newTimelineCursor()
	bad := timelineMovementFor(t, 1, timelineMovementNote, 1, 1)
	cause := &DecisionRef{Rule: RuleKey{"duration-invalid", 1}, Subject: bad.Subject}
	bad.Duration = Evidence[Rational]{State: EvidenceInvalid, Reason: "duration-invalid", Cause: cause}
	first, err := cursor.advance(context.Background(), s, bad)
	if err != nil || first.Position.State != EvidenceKnown || cursor.Position.State != EvidenceInvalid || cursor.Position.Cause != cause {
		t.Fatalf("invalid duration provenance: %+v %+v %v", first, cursor, err)
	}
	good := timelineMovementFor(t, 2, timelineMovementNote, 1, 3)
	next, err := cursor.advance(context.Background(), s, good)
	if err != nil || next.Position.State != EvidenceInvalid || next.Duration.State != EvidenceKnown || next.Duration.Value.String() != "1/3" {
		t.Fatalf("independent normalized duration lost: %+v %v", next, err)
	}
	fresh := newTimelineCursor()
	got, err := fresh.advance(context.Background(), s, good)
	if err != nil || got.Position.Value.String() != "0" {
		t.Fatalf("new measure did not recover relative origin: %+v %v", got, err)
	}
}

func TestTimelineCursorNegativeLocalPositionIsNotNormativeError(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	cursor := newTimelineCursor()
	_, err := cursor.advance(context.Background(), s, timelineMovementFor(t, 1, timelineMovementBackup, 1, 3))
	if err != nil || cursor.Position.State != EvidenceKnown || cursor.Position.Value.String() != "-1/3" {
		t.Fatalf("local displacement silently prohibited: %+v %v", cursor.Position, err)
	}
}

func TestTimelineNormalizationExactAndCausal(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	one, _ := NewDecimal(big.NewRat(1, 1))
	three, _ := NewDecimal(big.NewRat(3, 1))
	got, err := s.normalizeTimelineDuration(context.Background(), timelineKnown(one), timelineKnown(three))
	if err != nil || got.State != EvidenceKnown || got.Value.String() != "1/3" {
		t.Fatalf("nonterminating quotient: %+v %v", got, err)
	}
	missing := Evidence[Decimal]{State: EvidenceUnknown, Reason: "initial-divisions-unavailable"}
	got, err = s.normalizeTimelineDuration(context.Background(), timelineKnown(one), missing)
	if err != nil || got.State != EvidenceUnknown || got.Reason != missing.Reason || got.Value.Valid() {
		t.Fatalf("missing units became default: %+v %v", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.normalizeTimelineDuration(ctx, timelineKnown(one), timelineKnown(three)); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}
}
