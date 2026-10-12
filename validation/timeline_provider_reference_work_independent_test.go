package validation

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestIndependentTimelineProviderReferenceGuardWork(t *testing.T) {
	doc := strings.Repeat("canonical-reference-", 1024)
	a, b := NodeRef{doc + "a", 1}, NodeRef{doc + "b", 1}
	at := statePosition(t, 0, 0, 1)
	at.Part = a
	at.Measure.DocumentID = a.DocumentID
	origin := timelineEventFact{Part: stateKnown(b), Origin: stateKnown(at), Position: stateKnown(at), Voice: stateAbsent[string]()}
	tests := []struct {
		name string
		run  func(*Session) error
	}{
		{"direction-origin", func(s *Session) error {
			r := &stateReader{ctx: context.Background(), s: s, b: stateBindings()}
			(&timelineService{}).directionAnchor(r, origin, nil)
			return r.err
		}},
		{"direction-index", func(s *Session) error {
			_, _, _, err := directionNoteIndex{}.anchor(context.Background(), s, &timelineService{}, origin)
			return err
		}},
		{"figure-index", func(s *Session) error {
			_, _, _, err := figuredBassNoteIndex{}.anchor(context.Background(), s, &timelineService{}, origin)
			return err
		}},
		{"figure-scan", func(s *Session) error {
			r := &stateReader{ctx: context.Background(), s: s, b: stateBindings()}
			block := timelineMeasureBlock{part: stateKnown(b)}
			(&timelineService{}).figuredBassAnchor(r, []figuredBassDomain{{block: block}}, origin, stateKnown(true))
			return r.err
		}},
		{"grace-gap", func(s *Session) error {
			r := &stateReader{ctx: context.Background(), s: s}
			_, err := graceGapRelevant(r, &timelineService{}, graceScopedGap{Position: stateKnown(at)}, graceStreamKey{part: b}, stateAbsent[musicalPosition](), stateAbsent[musicalPosition]())
			return err
		}},
		{"instrument", func(s *Session) error {
			_, err := instrumentStatement(context.Background(), s, &timelineState{}, at, b, 0, instrumentSoundStatement)
			return err
		}},
		{"midi", func(s *Session) error {
			_, err := recordStatement(context.Background(), s, &timelineState{}, at, b, (*recordStatementStore[midiState])(nil), nil, 0, false, func(v midiState) Evidence[Integer] { return v.Program })
			return err
		}},
		{"spatial", func(s *Session) error {
			sound := soundPlaybackContext{Origin: playbackOrigin{Part: stateKnown(a)}}
			record := instrumentRecord[midiState]{Part: b, Instrument: stateKnown(a)}
			_, err := sound.spatial(context.Background(), s, stateKnown(a), stateKnown(record))
			return err
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := rationalSession(t, Budgets{MaxRuleSteps: 32})
			defer s.cleanup()
			err := tc.run(s)
			if !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("compared%d-byte IDs before work admission: %v (steps%d)", len(doc), err, rationalUsage(t, s, BudgetRuleSteps))
			}
		})
	}
}
