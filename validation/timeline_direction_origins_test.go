package validation

import (
	"context"
	"errors"
	"testing"
)

func directionOriginFixture(t *testing.T, order OrderKind, budget Budgets) (*Session, *timelineService, *timelineBindings, []timelineMeasureBlock) {
	t.Helper()
	s := rationalSession(t, budget)
	s.documentID = "d"
	t.Cleanup(s.cleanup)
	b := stateBindings()
	direction := stateNode(b, timelineRoleDirection, 10, stateAbsent[Atomic]())
	// Deliberately disagree with musical order: note 30 is later than note 40.
	block := runtimeBlock(b, 0, direction, runtimeNote(t, b, 30, 1, false), runtimeNote(t, b, 20, 1, false), runtimeNote(t, b, 40, 1, false))
	block.content.end.Order = testEvidence(stateKnown(order))
	block.coverage = stateKnown(true)
	service := &timelineService{Facts: map[NodeID]timelineEventFact{}}
	for id, offset := range map[NodeID]int64{10: 1, 20: 0, 30: 3, 40: 2} {
		at := timelineAt(block, stateKnown(mustRational(t, offset, 1)))
		service.Facts[id] = timelineEventFact{Origin: at, Position: at, Part: block.part, Staff: stateAbsent[Integer](), Voice: stateAbsent[string]()}
	}
	stateClose(t, s, &service.State, 0)
	return s, service, b, []timelineMeasureBlock{block}
}

func TestDirectionOriginsExactScoreOrderAndIndependentDisplayBothRepresentations(t *testing.T) {
	for _, order := range []OrderKind{OrderSource, OrderModelContent} {
		t.Run(string(order), func(t *testing.T) {
			s, service, b, blocks := directionOriginFixture(t, order, Budgets{})
			f := service.Facts[10]
			f.Position.Value.Offset = mustRational(t, 3, 1)
			service.Facts[10] = f
			if err := service.resolveDirectionOrigins(context.Background(), s, b, blocks); err != nil {
				t.Fatal(err)
			}
			got := service.DirectionOrigins[10]
			if got.Anchor.State != EvidenceKnown || got.Anchor.Value.Node != 40 {
				t.Fatalf("score-order association = %+v", got)
			}
			noteOriginCheckPosition(t, got.CursorOrigin, "1")
			noteOriginCheckPosition(t, got.DisplayPosition, "3")
			noteOriginCheckPosition(t, got.AnchorPosition, "2")
			if got.Voice.State != EvidenceAbsent || got.Staff.State != EvidenceAbsent {
				t.Fatalf("invented omitted selector: %+v", got)
			}
		})
	}
}

func TestDirectionOriginsVoiceExactRestrictionAndStaffIndependence(t *testing.T) {
	for _, staffState := range []EvidenceState{EvidenceKnown, EvidenceAbsent, EvidenceUnknown, EvidenceInvalid} {
		t.Run(string(staffState), func(t *testing.T) {
			s, service, b, blocks := directionOriginFixture(t, OrderSource, Budgets{})
			f := service.Facts[10]
			f.Voice = stateKnown("upper")
			f.Staff = Evidence[Integer]{State: staffState, Value: stateInt(t, 1), Reason: "staff-unavailable"}
			service.Facts[10] = f
			f = service.Facts[40]
			f.Voice, f.Staff = stateKnown("lower"), stateKnown(stateInt(t, 1))
			service.Facts[40] = f
			f = service.Facts[30]
			f.Voice, f.Staff = stateKnown("upper"), stateKnown(stateInt(t, 2))
			service.Facts[30] = f
			if err := service.resolveDirectionOrigins(context.Background(), s, b, blocks); err != nil {
				t.Fatal(err)
			}
			got := service.DirectionOrigins[10]
			if got.Anchor.State != EvidenceKnown || got.Anchor.Value.Node != 30 || got.Staff.State != staffState || got.AnchorStaff.Value.String() != "2" {
				t.Fatalf("voice selection or cross-staff association = %+v", got)
			}
		})
	}
}

func TestDirectionOriginsOmittedVoiceIsNotInventedVoiceOne(t *testing.T) {
	s, service, b, blocks := directionOriginFixture(t, OrderModelContent, Budgets{})
	f := service.Facts[40]
	f.Voice = stateKnown("bass")
	service.Facts[40] = f
	if err := service.resolveDirectionOrigins(context.Background(), s, b, blocks); err != nil {
		t.Fatal(err)
	}
	if got := service.DirectionOrigins[10]; got.Anchor.Value.Node != 40 || got.AnchorVoice.Value != "bass" || got.Voice.State != EvidenceAbsent {
		t.Fatalf("omitted voice restricted candidates: %+v", got)
	}
}

func TestDirectionOriginsUnavailabilityIsCausalAndLocal(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*timelineService, []timelineMeasureBlock, *DecisionRef)
		want   EvidenceState
		cause  bool
	}{
		{"bad-display-only", func(v *timelineService, _ []timelineMeasureBlock, c *DecisionRef) {
			f := v.Facts[10]
			f.Position = Evidence[musicalPosition]{State: EvidenceInvalid, Reason: "bad-offset", Cause: c}
			v.Facts[10] = f
		}, EvidenceKnown, false},
		{"bad-origin", func(v *timelineService, _ []timelineMeasureBlock, c *DecisionRef) {
			f := v.Facts[10]
			f.Origin = Evidence[musicalPosition]{State: EvidenceInvalid, Reason: "bad-origin", Cause: c}
			v.Facts[10] = f
		}, EvidenceInvalid, true},
		{"bad-selected-voice", func(v *timelineService, _ []timelineMeasureBlock, c *DecisionRef) {
			f := v.Facts[10]
			f.Voice = Evidence[string]{State: EvidenceInvalid, Reason: "bad-voice", Cause: c}
			v.Facts[10] = f
		}, EvidenceInvalid, true},
		{"candidate-missing-voice", func(v *timelineService, _ []timelineMeasureBlock, c *DecisionRef) {
			f := v.Facts[10]
			f.Voice = stateKnown("upper")
			v.Facts[10] = f
			f = v.Facts[40]
			f.Voice = Evidence[string]{State: EvidenceInvalid, Reason: "bad-voice", Cause: c}
			v.Facts[40] = f
			f = v.Facts[30]
			f.Voice = stateKnown("upper")
			v.Facts[30] = f
		}, EvidenceInvalid, true},
		{"later-unknown-position", func(v *timelineService, _ []timelineMeasureBlock, c *DecisionRef) {
			f := v.Facts[30]
			f.Position = Evidence[musicalPosition]{State: EvidenceInvalid, Reason: "unplaced", Cause: c}
			v.Facts[30] = f
		}, EvidenceInvalid, true},
		{"later-unknown-voice", func(v *timelineService, _ []timelineMeasureBlock, c *DecisionRef) {
			f := v.Facts[10]
			f.Voice = stateKnown("upper")
			v.Facts[10] = f
			f = v.Facts[40]
			f.Voice = stateKnown("upper")
			v.Facts[40] = f
			f = v.Facts[30]
			f.Voice = Evidence[string]{State: EvidenceInvalid, Reason: "bad-later-voice", Cause: c}
			v.Facts[30] = f
		}, EvidenceKnown, false},
		{"incomplete-domain", func(_ *timelineService, blocks []timelineMeasureBlock, c *DecisionRef) {
			blocks[0].complete = Evidence[bool]{State: EvidenceInvalid, Reason: "incomplete-domain", Cause: c}
		}, EvidenceInvalid, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, service, b, blocks := directionOriginFixture(t, OrderSource, Budgets{})
			cause := &DecisionRef{Rule: RuleKey{"input", 1}, Subject: NodeRef{"d", 77}}
			tc.change(service, blocks, cause)
			if err := service.resolveDirectionOrigins(context.Background(), s, b, blocks); err != nil {
				t.Fatal(err)
			}
			got := service.DirectionOrigins[10]
			if got.Anchor.State != tc.want {
				t.Fatalf("anchor = %+v; want %s", got, tc.want)
			}
			cause.Subject.Node = 999
			if tc.cause && (got.Anchor.Cause == nil || got.Anchor.Cause.Subject.Node != 77 || got.AnchorPosition.Cause == nil || got.AnchorPosition.Cause.Subject.Node != 77) {
				t.Fatalf("borrowed or lost causal evidence: %+v", got)
			}
		})
	}
}

func TestDirectionOriginsEqualTimeGraceChordAndTrueTies(t *testing.T) {
	for _, role := range []timelineRole{timelineRoleNoteGrace, timelineRoleNoteChord} {
		s, service, b, blocks := directionOriginFixture(t, OrderSource, Budgets{})
		note := blocks[0].content.children[3]
		note.children = append(note.children, stateNode(b, role, 45, stateAbsent[Atomic]()))
		f := service.Facts[40]
		f.Position.Value.Offset = mustRational(t, 1, 1)
		service.Facts[40] = f
		if err := service.resolveDirectionOrigins(context.Background(), s, b, blocks); err != nil {
			t.Fatal(err)
		}
		if got := service.DirectionOrigins[10].Anchor; got.State != EvidenceKnown || got.Value.Node != 40 {
			t.Fatalf("unique equal-onset note excluded for role %d: %+v", role, got)
		}
	}
	s, service, b, blocks := directionOriginFixture(t, OrderSource, Budgets{})
	f := service.Facts[30]
	f.Position = service.Facts[40].Position
	service.Facts[30] = f
	if err := service.resolveDirectionOrigins(context.Background(), s, b, blocks); err != nil {
		t.Fatal(err)
	}
	if got := service.DirectionOrigins[10].Anchor; got.State != EvidenceUnknown || got.Reason != "direction-simultaneous-notes-unresolved" {
		t.Fatalf("lexical tie-break fabricated: %+v", got)
	}
}

func TestDirectionOriginsCrossMeasureAndProvenAbsence(t *testing.T) {
	for _, order := range []OrderKind{OrderSource, OrderModelContent} {
		t.Run(string(order), func(t *testing.T) {
			s, service, b, blocks := directionOriginFixture(t, order, Budgets{})
			f := service.Facts[10]
			f.Origin.Value.Offset = mustRational(t, 4, 1)
			service.Facts[10] = f
			next := runtimeBlock(b, 1, runtimeNote(t, b, 70, 1, false))
			next.coverage = stateKnown(true)
			next.content.end.Order = testEvidence(stateKnown(order))
			service.Facts[70] = timelineEventFact{Position: timelineAt(next, stateKnown(mustRational(t, 0, 1))), Part: next.part, Staff: stateAbsent[Integer](), Voice: stateAbsent[string]()}
			blocks = append(blocks, next)
			stateClose(t, s, &service.State, 1)
			if err := service.resolveDirectionOrigins(context.Background(), s, b, blocks); err != nil {
				t.Fatal(err)
			}
			if got := service.DirectionOrigins[10]; got.Anchor.State != EvidenceKnown || got.Anchor.Value.Node != 70 || got.AnchorPosition.Value.MeasureOrdinal != 1 {
				t.Fatalf("cross-measure association lost: %+v", got)
			}
		})
	}
	s, service, b, blocks := directionOriginFixture(t, OrderSource, Budgets{})
	f := service.Facts[10]
	f.Origin.Value.Offset = mustRational(t, 4, 1)
	service.Facts[10] = f
	if err := service.resolveDirectionOrigins(context.Background(), s, b, blocks); err != nil {
		t.Fatal(err)
	}
	if got := service.DirectionOrigins[10]; got.Anchor.State != EvidenceAbsent || got.AnchorPosition.State != EvidenceAbsent {
		t.Fatalf("complete no-following-note search is not absence: %+v", got)
	}
}

func TestDirectionOriginsBudgetsCancellationAndDuplicatePublication(t *testing.T) {
	for _, tc := range []struct {
		name   string
		budget Budgets
		cancel bool
	}{
		{"work", Budgets{MaxRuleSteps: 5}, false},
		{"retention", Budgets{MaxBufferedBytes: 1024}, false},
		{"cancellation", Budgets{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Fixture ownership itself needs unrestricted admission.
			_, service, b, blocks := directionOriginFixture(t, OrderSource, Budgets{})
			s := rationalSession(t, tc.budget)
			s.documentID = "d"
			defer s.cleanup()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			err := service.resolveDirectionOrigins(ctx, s, b, blocks)
			want := ErrResourceLimit
			if tc.cancel {
				want = context.Canceled
			}
			if !errors.Is(err, want) || len(service.DirectionOrigins) != 0 {
				t.Fatalf("failed publication escaped: err=%v records=%d", err, len(service.DirectionOrigins))
			}
		})
	}
	_, service, b, blocks := directionOriginFixture(t, OrderSource, Budgets{})
	s := rationalSession(t, Budgets{MaxContextFacts: 1})
	s.documentID = "d"
	defer s.cleanup()
	blocks[0].content.children = append(blocks[0].content.children, stateNode(b, timelineRoleDirection, 90, stateAbsent[Atomic]()))
	service.Facts[90] = service.Facts[10]
	if err := service.resolveDirectionOrigins(context.Background(), s, b, blocks); !errors.Is(err, ErrResourceLimit) || len(service.DirectionOrigins) != 1 || rationalUsage(t, s, BudgetContextFacts) != 1 {
		t.Fatalf("per-record context quota: err=%v records=%d", err, len(service.DirectionOrigins))
	}
	if err := service.resolveDirectionOrigins(context.Background(), s, b, blocks); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("duplicate accepted: %v", err)
	}
}

func TestDirectionOriginsScopeAndIndependentUnavailableNotes(t *testing.T) {
	for _, order := range []OrderKind{OrderSource, OrderModelContent} {
		for _, tc := range []struct {
			name   string
			change func(*testing.T, *timelineService, *timelineBindings, []timelineMeasureBlock)
		}{
			{"earlier-bad-voice", func(_ *testing.T, v *timelineService, _ *timelineBindings, _ []timelineMeasureBlock) {
				f := v.Facts[20]
				f.Voice = stateUnknown[string]("earlier-voice")
				v.Facts[20] = f
			}},
			{"other-voice-unplaced", func(_ *testing.T, v *timelineService, _ *timelineBindings, _ []timelineMeasureBlock) {
				f := v.Facts[30]
				f.Voice = stateKnown("lower")
				f.Position = stateUnknown[musicalPosition]("other-voice-unplaced")
				v.Facts[30] = f
			}},
			{"other-document", func(_ *testing.T, v *timelineService, _ *timelineBindings, _ []timelineMeasureBlock) {
				f := v.Facts[30]
				f.Position.Value.Part.DocumentID = "other"
				f.Part.Value.DocumentID = "other"
				v.Facts[30] = f
			}},
			{"foreign-known-declaration", func(_ *testing.T, v *timelineService, b *timelineBindings, blocks []timelineMeasureBlock) {
				foreign := stateNode(b, timelineRoleHarmony, 99, stateAbsent[Atomic]())
				blocks[0].content.children = append(blocks[0].content.children, foreign)
				v.Facts[99] = v.Facts[40]
			}},
		} {
			t.Run(string(order)+"/"+tc.name, func(t *testing.T) {
				s, service, b, blocks := directionOriginFixture(t, order, Budgets{})
				for _, id := range []NodeID{10, 20, 30, 40} {
					f := service.Facts[id]
					f.Voice = stateKnown("upper")
					service.Facts[id] = f
				}
				tc.change(t, service, b, blocks)
				if err := service.resolveDirectionOrigins(context.Background(), s, b, blocks); err != nil {
					t.Fatal(err)
				}
				if got := service.DirectionOrigins[10].Anchor; got.State != EvidenceKnown || got.Value.Node != 40 {
					t.Fatalf("irrelevant gap masked association: %+v", got)
				}
			})
		}
	}
}

func TestDirectionOriginsUnknownCandidateVoiceAndExactEmptyVoice(t *testing.T) {
	for _, candidateVoice := range []Evidence[string]{stateAbsent[string](), stateUnknown[string]("voice-unavailable"), stateKnown("")} {
		s, service, b, blocks := directionOriginFixture(t, OrderSource, Budgets{})
		f := service.Facts[10]
		f.Voice = stateKnown("")
		service.Facts[10] = f
		f = service.Facts[40]
		f.Voice = candidateVoice
		service.Facts[40] = f
		f = service.Facts[30]
		f.Voice = stateKnown("upper")
		service.Facts[30] = f
		if err := service.resolveDirectionOrigins(context.Background(), s, b, blocks); err != nil {
			t.Fatal(err)
		}
		got := service.DirectionOrigins[10].Anchor
		if candidateVoice.State == EvidenceKnown {
			if got.State != EvidenceKnown || got.Value.Node != 40 {
				t.Fatalf("empty xs:string voice lost: %+v", got)
			}
		} else if got.State != EvidenceUnknown {
			t.Fatalf("missing candidate voice invented: %+v", got)
		}
	}
}

func TestDirectionOriginsLaterMeasureGapDoesNotMaskEarlierCandidate(t *testing.T) {
	s, service, b, blocks := directionOriginFixture(t, OrderSource, Budgets{})
	late := runtimeBlock(b, 1, runtimeNote(t, b, 90, 1, false))
	late.complete = stateUnknown[bool]("later-measure-incomplete")
	blocks = append(blocks, late)
	stateClose(t, s, &service.State, 1)
	if err := service.resolveDirectionOrigins(context.Background(), s, b, blocks); err != nil {
		t.Fatal(err)
	}
	if got := service.DirectionOrigins[10].Anchor; got.State != EvidenceKnown || got.Value.Node != 40 {
		t.Fatalf("later domain gap leaked: %+v", got)
	}
}

func TestDirectionOriginsStoredCursorCauseOwned(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	service := &timelineService{Facts: map[NodeID]timelineEventFact{}}
	cause := &DecisionRef{Rule: RuleKey{"cursor", 1}, Subject: NodeRef{"d", 77}}
	fact := timelineEventFact{Origin: Evidence[musicalPosition]{State: EvidenceInvalid, Reason: "cursor-unavailable", Cause: cause}}
	if err := service.storeFact(context.Background(), s, 10, fact); err != nil {
		t.Fatal(err)
	}
	cause.Subject.Node = 999
	if got := service.Facts[10].Origin.Cause; got == nil || got.Subject.Node != 77 {
		t.Fatalf("origin cause borrowed: %+v", got)
	}
}
