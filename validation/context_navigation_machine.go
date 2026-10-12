package validation

import (
	"context"
	"math/big"
	"strconv"
	"strings"
)

// NavigationKind identifies a closed encoded navigation or occurrence role.
type NavigationKind string

// Navigation context kinds do not imply that their execution is established.
const (
	NavigationGraphicSegno NavigationKind = "graphic-segno"
	NavigationGraphicCoda  NavigationKind = "graphic-coda"
	NavigationNote         NavigationKind = "note"
	NavigationSound        NavigationKind = "sound"
	NavigationListening    NavigationKind = "listening"
	NavigationForward      NavigationKind = "forward-repeat"
	NavigationBackward     NavigationKind = "backward-repeat"
	NavigationSegno        NavigationKind = "segno"
	NavigationCoda         NavigationKind = "coda"
	NavigationDaCapo       NavigationKind = "dacapo"
	NavigationDalSegno     NavigationKind = "dalsegno"
	NavigationToCoda       NavigationKind = "tocoda"
	NavigationFine         NavigationKind = "fine"
	NavigationEnding       NavigationKind = "ending"
	NavigationTraversal    NavigationKind = "traversal"
)

// NavigationPassFrame identifies one established repeat region and its local pass.
type NavigationPassFrame struct {
	Region NodeRef
	Pass   Integer
}

// NavigationOccurrence is one bounded execution occurrence, distinct from its
// score event. Pass is known only for the proved selector domain; Frames retain
// nested identities without flattening them into a global pass number.
type NavigationOccurrence struct {
	NoteListening   []NavigationNoteListening
	Play            []NavigationPlayInstruction
	Instruments     []NavigationInstrumentState
	EndDynamics     Evidence[Decimal]
	Listening       []NavigationListeningStatement
	Source          NodeRef
	Pass            Evidence[Integer]
	Epoch           uint64
	Frames          []NavigationPassFrame
	Played          Evidence[bool]
	Tempo, Dynamics Evidence[Decimal]
	Pizzicato       Evidence[bool]
}

// NavigationContextResult preserves encoded values, applicability, target and
// bounded occurrence evidence. Complete describes traversal extent only.
type NavigationContextResult struct {
	Directives []NavigationDirective
	// PlaybackSoundNeeded is advice for a graphic; LocalPlaybackSound is its labelled sibling, if established.
	PlaybackSoundNeeded                Evidence[bool]
	LocalPlaybackSound                 Evidence[NodeRef]
	NoteListening                      []NavigationNoteListening
	RecommendedAssess, TargetsDistinct Evidence[bool]
	InitialSyncMode                    Evidence[string]
	Play                               []TimingPlayInstruction
	Listening                          []NavigationListeningStatement
	Position                           Evidence[ContextPosition]
	Kind                               NavigationKind
	Part                               Evidence[NodeRef]
	Direction                          Evidence[string]
	Times                              Evidence[Integer]
	AfterJump                          Evidence[bool]
	AttributesActive                   Evidence[bool]
	Label                              Evidence[string]
	Target                             Evidence[NodeRef]
	TimeOnly                           Evidence[string]
	// MarkerActive reports barline marker activation; Guard is pass selection.
	MarkerActive      Evidence[bool]
	Guard             Evidence[bool]
	Defaulted         bool
	EndingNumber      Evidence[string]
	DurationUnchanged Evidence[bool]
	FineDuration      Evidence[Decimal]
	Traversal         []NavigationOccurrence
	Complete          Evidence[bool]
}

type navigationPassDomain uint8

const (
	navigationUnknownPass navigationPassDomain = iota
	navigationMovementPass
	navigationSingleRepeatPass
)

type navigationEvent struct {
	NoteListening             []NavigationNoteListening
	Play                      []TimingPlayInstruction
	Instruments               []navigationInstrumentUpdate
	NoteDynamics, EndDynamics Evidence[Decimal]
	NotePizzicato             Evidence[bool]
	Listening                 []NavigationListeningStatement
	Source                    NodeRef
	Kind                      NavigationKind
	Guard                     Evidence[string]
	Times                     Evidence[Integer]
	AfterJump                 Evidence[bool]
	Target                    Evidence[int]
	PassDomain                navigationPassDomain
	Tempo, Dynamics           Evidence[Decimal]
	Pizzicato                 Evidence[bool]
	Control                   Evidence[bool]
}

type navigationFrame struct {
	start int
	pass  uint64
}

// Membership is independent of the ascending/duplicate conformance policy.
// The complete positive-decimal item syntax is checked without allocation.
func navigationPassApplies(ctx context.Context, s *Session, set Evidence[string], pass uint64) (Evidence[bool], error) {
	if err := s.step(ctx, 1); err != nil {
		return Evidence[bool]{}, err
	}
	if set.State == EvidenceAbsent {
		return Evidence[bool]{State: EvidenceKnown, Value: true}, nil
	}
	if set.State != EvidenceKnown {
		return stateUnavailable[string, bool](set), nil
	}
	if pass == 0 {
		return stateUnknown[bool]("navigation-pass-unavailable"), nil
	}
	if err := s.account.Charge(BudgetBufferedBytes, 64); err != nil {
		return Evidence[bool]{}, err
	}
	defer func() { _ = s.account.Release(BudgetBufferedBytes, 64) }()
	target := strconv.FormatUint(pass, 10)
	matched := false
	valid, err := contextScanPassSet(ctx, s, set.Value, func(item string) error { matched = matched || item == target; return nil })
	if err != nil {
		return Evidence[bool]{}, err
	}
	if !valid {
		return stateUnknown[bool]("navigation-pass-set-unavailable"), nil
	}
	return navigationKnownBool(matched), nil
}

func navigationConditionalValue[T any](v Evidence[T], guard Evidence[bool]) Evidence[T] {
	if guard.State == EvidenceKnown && !guard.Value {
		return Evidence[T]{State: EvidenceAbsent}
	}
	if v.State == EvidenceAbsent || v.State == "" {
		return Evidence[T]{State: EvidenceAbsent}
	}
	if guard.State == EvidenceKnown {
		return v
	}
	return stateUnavailable[bool, T](stateMergeGap(stateUnavailable[T, bool](v), guard))
}

func navigationInteger(ctx context.Context, s *Session, n uint64) (Integer, error) {
	if err := s.step(ctx, 1); err != nil {
		return Integer{}, err
	}
	if err := s.retain(128); err != nil {
		return Integer{}, err
	}
	if err := s.account.Charge(BudgetBufferedBytes, 128); err != nil {
		return Integer{}, err
	}
	defer func() { _ = s.account.Release(BudgetBufferedBytes, 128) }()
	return NewInteger(new(big.Int).SetUint64(n))
}

func navigationCountCompare(ctx context.Context, s *Session, count Integer, pass uint64) (int, error) {
	if !count.Valid() {
		return 0, contract("navigation count", "uninitialized value")
	}
	if err := s.step(ctx, int64(len(count.text))+1); err != nil {
		return 0, err
	}
	if err := s.account.Charge(BudgetBufferedBytes, 64); err != nil {
		return 0, err
	}
	defer func() { _ = s.account.Release(BudgetBufferedBytes, 64) }()
	p := strconv.FormatUint(pass, 10)
	if count.value.Sign() < 0 {
		return -1, nil
	}
	if len(count.text) < len(p) {
		return -1, nil
	}
	if len(count.text) > len(p) {
		return 1, nil
	}
	return strings.Compare(count.text, p), nil
}

// runNavigation unfolds only established edges. The shared cumulative account
// bounds traversal and output, including huge exact counts. Source-node revisits
// are normal repeat behavior and are not classified as cycles.
func runNavigation(ctx context.Context, s *Session, events []navigationEvent, initial ...[]navigationInstrumentUpdate) (NavigationContextResult, error) {
	out := NavigationContextResult{Kind: NavigationTraversal, InitialSyncMode: stateUnknown[string]("navigation-sync-application-default"), Complete: Evidence[bool]{State: EvidenceKnown, Value: true}}
	if err := s.step(ctx, 1); err != nil {
		return out, err
	}
	bytes, err := rationalCheckedMul(BudgetBufferedBytes, int64(len(events)), 32)
	if err != nil {
		return out, err
	}
	if err = s.account.Charge(BudgetBufferedBytes, bytes); err != nil {
		return out, err
	}
	defer func() { _ = s.account.Release(BudgetBufferedBytes, bytes) }()
	frames := make([]navigationFrame, 0, len(events))
	epoch := uint64(0)
	var listening []NavigationListeningStatement
	var play []NavigationPlayInstruction
	var instruments []NavigationInstrumentState
	for _, updates := range initial {
		instruments, err = navigationApplyInstruments(ctx, s, instruments, updates, navigationKnownBool(true))
		if err != nil {
			return out, err
		}
	}
	tempo, dynamics := Evidence[Decimal]{State: EvidenceAbsent}, Evidence[Decimal]{State: EvidenceAbsent}
	pizzicato := Evidence[bool]{State: EvidenceAbsent}
	invalidateReentry := func() error {
		// Carrying performance state across a jump is source-sensitive. Preserve
		// exact source facts separately; no absent instruction becomes a reset.
		if tempo.State != EvidenceAbsent {
			tempo = stateUnknown[Decimal]("navigation-reentry-state-unresolved")
		}
		if dynamics.State != EvidenceAbsent {
			dynamics = stateUnknown[Decimal]("navigation-reentry-state-unresolved")
		}
		if pizzicato.State != EvidenceAbsent {
			pizzicato = stateUnknown[bool]("navigation-reentry-state-unresolved")
		}
		for i := range play {
			if err := s.step(ctx, 1); err != nil {
				return err
			}
			play[i].Active = stateUnknown[bool]("navigation-reentry-play-unresolved")
		}
		for i := range instruments {
			if err := navigationInvalidateInstrument(ctx, s, &instruments[i], initial); err != nil {
				return err
			}
		}
		for i := range listening {
			if err := s.step(ctx, 1); err != nil {
				return err
			}
			listening[i].Type = stateUnknown[string]("navigation-reentry-listening-unresolved")
			listening[i].Latency = stateUnknown[Decimal]("navigation-reentry-listening-unresolved")
			listening[i].Value = stateUnknown[string]("navigation-reentry-listening-unresolved")
		}
		return nil
	}
	for pc := 0; pc < len(events); {
		if err = s.step(ctx, 1); err != nil {
			return out, err
		}
		event := events[pc]
		if event.Kind == NavigationForward && (len(frames) == 0 || frames[len(frames)-1].start != pc) {
			frames = append(frames, navigationFrame{start: pc, pass: 1})
		}
		pass := uint64(0)
		if event.PassDomain == navigationMovementPass && len(frames) == 0 {
			pass = epoch + 1
		}
		if event.PassDomain == navigationSingleRepeatPass && len(frames) == 1 {
			pass = frames[0].pass
		}
		guard := Evidence[bool]{State: EvidenceKnown, Value: true}
		if event.Guard.State != "" && event.Guard.State != EvidenceAbsent {
			guard, err = navigationPassApplies(ctx, s, event.Guard, pass)
			if err != nil {
				return out, err
			}
		}
		frameBytes, e := rationalCheckedMul(BudgetBufferedBytes, int64(len(frames)), 128)
		if e != nil {
			return out, e
		}
		if e = s.retain(1024 + frameBytes); e != nil {
			return out, e
		}
		if e = s.account.Charge(BudgetContextFacts, 1); e != nil {
			return out, e
		}
		occurrence := NavigationOccurrence{EndDynamics: Evidence[Decimal]{State: EvidenceAbsent}, Source: event.Source, Epoch: epoch, Played: guard, Pass: stateUnknown[Integer]("navigation-pass-domain-unavailable"), Frames: make([]NavigationPassFrame, len(frames))}
		if pass != 0 {
			v, e := navigationInteger(ctx, s, pass)
			if e != nil {
				return out, e
			}
			occurrence.Pass = Evidence[Integer]{State: EvidenceKnown, Value: v}
		}
		for i, f := range frames {
			v, e := navigationInteger(ctx, s, f.pass)
			if e != nil {
				return out, e
			}
			occurrence.Frames[i] = NavigationPassFrame{Region: events[f.start].Source, Pass: v}
		}
		for _, v := range []struct {
			source Evidence[Decimal]
			dest   *Evidence[Decimal]
		}{{event.Tempo, &tempo}, {event.Dynamics, &dynamics}} {
			value := navigationConditionalValue(v.source, guard)
			if value.State != EvidenceAbsent {
				*v.dest = value
			}
		}
		if value := navigationConditionalValue(event.Pizzicato, guard); value.State != EvidenceAbsent {
			pizzicato = value
		}
		occurrence.NoteListening, err = navigationNoteListeningAt(ctx, s, event.NoteListening, pass, guard)
		if err != nil {
			return out, err
		}
		var localPlay []NavigationPlayInstruction
		play, localPlay, err = navigationApplyPlay(ctx, s, play, event.Play, guard)
		if err != nil {
			return out, err
		}
		if _, err = stateReserve(ctx, s, play); err != nil {
			return out, err
		}
		if _, err = stateReserve(ctx, s, localPlay); err != nil {
			return out, err
		}
		occurrence.Play = make([]NavigationPlayInstruction, 0, len(play)+len(localPlay))
		occurrence.Play = append(occurrence.Play, stateClone(play)...)
		occurrence.Play = append(occurrence.Play, stateClone(localPlay)...)
		instruments, err = navigationApplyInstruments(ctx, s, instruments, event.Instruments, guard)
		if err != nil {
			return out, err
		}
		if _, err = stateReserve(ctx, s, instruments); err != nil {
			return out, err
		}
		occurrence.Instruments = stateClone(instruments)
		listening, err = navigationApplyListening(ctx, s, listening, event.Listening, pass)
		if err != nil {
			return out, err
		}
		if _, err = stateReserve(ctx, s, listening); err != nil {
			return out, err
		}
		occurrence.Listening = stateClone(listening)
		occurrence.Tempo, occurrence.Dynamics, occurrence.Pizzicato = tempo, dynamics, pizzicato
		if event.Kind == NavigationNote {
			if v := navigationConditionalValue(event.NoteDynamics, guard); v.State != EvidenceAbsent {
				occurrence.Dynamics = v
			}
			if v := navigationConditionalValue(event.NotePizzicato, guard); v.State != EvidenceAbsent {
				occurrence.Pizzicato = v
			}
			occurrence.EndDynamics = navigationConditionalValue(event.EndDynamics, guard)
		}
		out.Traversal = append(out.Traversal, occurrence)
		if event.Control.State != "" && event.Control.State != EvidenceKnown {
			out.Complete = event.Control
			return out, nil
		}
		switch event.Kind {
		case NavigationBackward:
			if epoch > 0 {
				if event.AfterJump.State != EvidenceKnown {
					out.Complete = stateUnknown[bool]("navigation-after-jump-unavailable")
					return out, nil
				}
				if !event.AfterJump.Value {
					if len(frames) > 0 {
						frames = frames[:len(frames)-1]
					}
					pc++
					continue
				}
			}
			if event.Target.State != EvidenceKnown {
				out.Complete = stateUnavailable[int, bool](event.Target)
				return out, nil
			}
			if len(frames) == 0 || frames[len(frames)-1].start != event.Target.Value {
				out.Complete = stateUnknown[bool]("navigation-repeat-region-unavailable")
				return out, nil
			}
			if event.Times.State != EvidenceKnown {
				out.Complete = stateUnavailable[Integer, bool](event.Times)
				if event.Times.State == EvidenceAbsent {
					out.Complete = stateUnknown[bool]("navigation-repeat-times-unavailable")
				}
				return out, nil
			}
			if event.Times.Value.value.Sign() == 0 {
				out.Complete = stateUnknown[bool]("navigation-zero-repeat-times-unresolved")
				return out, nil
			}
			cmp, e := navigationCountCompare(ctx, s, event.Times.Value, frames[len(frames)-1].pass)
			if e != nil {
				return out, e
			}
			if cmp > 0 {
				if err = invalidateReentry(); err != nil {
					return out, err
				}
				frames[len(frames)-1].pass++
				pc = event.Target.Value
				continue
			}
			frames = frames[:len(frames)-1]
		case NavigationDaCapo, NavigationDalSegno, NavigationToCoda, NavigationFine:
			if guard.State == EvidenceKnown && !guard.Value {
				pc++
				continue
			}
			if event.Kind != NavigationFine && event.Target.State != EvidenceKnown {
				// Resolving a selector cannot establish a missing destination. Keep
				// its evidence without inventing a pass domain; stronger guard
				// defects still win, while a proved false guard above skips the edge.
				out.Complete = stateUnavailable[int, bool](event.Target)
				if guard.State != EvidenceKnown {
					out.Complete = stateMergeGap(out.Complete, guard)
				}
				return out, nil
			}
			if guard.State != EvidenceKnown {
				out.Complete = guard
				return out, nil
			}
			if event.Kind == NavigationFine {
				out.Complete = stateUnknown[bool]("navigation-fine-activation-unresolved")
				return out, nil
			}
			if event.Target.Value < 0 || event.Target.Value >= len(events) {
				return out, contract("navigation target", "outside indexed events")
			}
			if event.Kind == NavigationDaCapo || event.Kind == NavigationDalSegno {
				epoch++
			}
			if err = invalidateReentry(); err != nil {
				return out, err
			}
			frames = frames[:0]
			pc = event.Target.Value
			continue
		case NavigationEnding:
			out.Complete = stateUnknown[bool]("navigation-ending-pass-binding-unresolved")
			return out, nil
		}
		pc++
	}
	return out, nil
}

func cloneNavigationContext(ctx context.Context, s *Session, in *NavigationContextResult) (*NavigationContextResult, error) {
	if in == nil {
		return nil, nil
	}
	value := *in
	navigationNormalizeEvidence(&value)
	if _, err := stateReserve(ctx, s, value); err != nil {
		return nil, err
	}
	out := stateClone(value)
	if err := s.step(ctx, 1); err != nil {
		return nil, err
	}
	return &out, nil
}

// Irrelevant public fields are explicitly absent, never zero-state evidence.
func navigationAbsentIfUnset[T any](v *Evidence[T]) {
	if v.State == "" {
		*v = Evidence[T]{State: EvidenceAbsent}
	}
}
func navigationNormalizeEvidence(v *NavigationContextResult) {
	navigationAbsentIfUnset(&v.PlaybackSoundNeeded)
	navigationAbsentIfUnset(&v.LocalPlaybackSound)
	navigationAbsentIfUnset(&v.RecommendedAssess)
	navigationAbsentIfUnset(&v.TargetsDistinct)
	navigationAbsentIfUnset(&v.InitialSyncMode)
	navigationAbsentIfUnset(&v.Position)
	navigationAbsentIfUnset(&v.Part)
	navigationAbsentIfUnset(&v.Direction)
	navigationAbsentIfUnset(&v.Times)
	navigationAbsentIfUnset(&v.AfterJump)
	navigationAbsentIfUnset(&v.AttributesActive)
	navigationAbsentIfUnset(&v.Label)
	navigationAbsentIfUnset(&v.Target)
	navigationAbsentIfUnset(&v.TimeOnly)
	navigationAbsentIfUnset(&v.Guard)
	navigationAbsentIfUnset(&v.MarkerActive)
	navigationAbsentIfUnset(&v.EndingNumber)
	navigationAbsentIfUnset(&v.DurationUnchanged)
	navigationAbsentIfUnset(&v.FineDuration)
	navigationAbsentIfUnset(&v.Complete)
}
