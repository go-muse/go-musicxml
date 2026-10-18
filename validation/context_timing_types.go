package validation

import "context"

// TimingAdjustment retains exact encoded-score evidence; it does not assert physical playback.
type TimingAdjustment struct {
	Raw      Evidence[Decimal]
	Units    Evidence[Decimal]
	Delta    Evidence[Rational]
	Origin   Evidence[ContextPosition]
	Position Evidence[ContextPosition]
}

// TimingPercent retains exact encoded-score evidence; it does not assert physical playback.
type TimingPercent struct {
	Raw      Evidence[Decimal]
	Percent  Evidence[Decimal]
	Origin   Evidence[ContextPosition]
	Position Evidence[ContextPosition]
}

// TimingDonor retains exact encoded-score evidence; it does not assert physical playback.
type TimingDonor struct {
	Note           Evidence[NodeRef]
	Raw            Evidence[Decimal]
	DonorDuration  Evidence[Rational]
	Fraction       Evidence[Rational]
	StolenDuration Evidence[Rational]
}

// TimingBend retains exact encoded-score evidence; it does not assert physical playback.
type TimingBend struct {
	Bend                NodeRef
	Release             TimingAdjustment
	FirstBeat, LastBeat TimingPercent
	OriginalNote        Evidence[ContextPosition]
	ReleasePresent      Evidence[bool]
	RawBeats            Evidence[Decimal]
	Beats               Evidence[Decimal]
	RawAccelerate       Evidence[bool]
	Accelerate          Evidence[bool]
}

// TimingTuning retains exact encoded-score evidence; it does not assert physical playback.
type TimingTuning struct {
	Line          Evidence[Integer]
	Octave        Evidence[Integer]
	Step          Evidence[string]
	Alter         Evidence[Decimal]
	Capo          Evidence[Integer]
	OpenPitch     Evidence[Rational]
	AdjustedPitch Evidence[Rational]
}

// TimingContextResult retains exact encoded-score evidence; it does not assert physical playback.
// Kind, one of the TimingKind constants, selects the applicable fields;
// unrelated fields are absent. Positions and
// durations use exact quarter-note units. WrittenPitch and tuning pitches count
// semitones from C0; SoundingPitch adds only established encoded transposition.
// ReferenceVelocity is dynamics percent times 90/100, without MIDI quantization.
// Pan uses 0 front, -90 left, +90 right and +/-180 rear; elevation uses 0 level,
// +90 above and -90 below. FretOpenString identifies encoded fret zero.
// PitchIntent describes separately unavailable external intended pitch, never a
// guessed obligation to agree with the encoded written or sounding projection.
// Advisory values are optional guidance, not mandatory conformance decisions.
type TimingContextResult struct {
	AllStaves    Evidence[bool]
	ScopeStaff   Evidence[Integer]
	PartOrder    Evidence[Integer]
	MeasureLabel Evidence[string]

	Advisories  []TimingAdvisory
	CursorAfter Evidence[ContextPosition]
	SwingRatio  Evidence[Rational]

	StateOrigins []TimingStateOrigin
	Spatial      []TimingSpatial
	Meter        []TimingMeterPair

	FretNumber                  Evidence[Integer]
	FretOpenString              Evidence[bool]
	EditorialPlaybackSuppressed Evidence[bool]
	TrillIncludesInitialSound   Evidence[bool]
	TrillExcludesClosingTurn    Evidence[bool]
	LayoutOrigin                Evidence[NodeRef]
	TuningBottomUp              Evidence[bool]
	StaffTopDown                Evidence[bool]
	RootAlterPrintObject        Evidence[bool]
	EffectiveTempo              Evidence[Decimal]
	EffectiveDynamics           Evidence[Decimal]
	EffectivePizzicato          Evidence[bool]
	EffectiveVelocity           Evidence[Rational]
	PitchIntent                 Evidence[Rational]
	Kind                        Code
	Play                        []TimingPlayInstruction
	PlayComplete                Evidence[bool]
	Attack, Release, MakeTime   TimingAdjustment
	Previous, Following         TimingDonor
	Bends                       []TimingBend
	Figures                     []NodeRef
	Tunings                     []TimingTuning
	ChainInterpretation         Code
	Origin                      Evidence[ContextPosition]
	Position                    Evidence[ContextPosition]
	End                         Evidence[ContextPosition]
	Duration                    Evidence[Rational]
	PrincipalDuration           Evidence[Rational]
	Principal                   Evidence[NodeRef]
	Units                       Evidence[Decimal]
	Staff                       Evidence[Integer]
	Voice                       Evidence[string]
	Part                        Evidence[NodeRef]
	ScoreOnset                  Evidence[ContextPosition]
	ScoreEnd                    Evidence[ContextPosition]
	Chord                       Evidence[bool]
	Grace                       Evidence[bool]
	Cue                         Evidence[bool]
	BendsComplete               Evidence[bool]
	GraceNode                   Evidence[NodeRef]
	StealPrevious               Evidence[Decimal]
	StealFollowing              Evidence[Decimal]
	Anchor                      Evidence[NodeRef]
	AnchorPosition              Evidence[ContextPosition]
	AnchorStaff                 Evidence[Integer]
	AnchorVoice                 Evidence[string]
	ChainPosition               Evidence[ContextPosition]
	ChainEnd                    Evidence[ContextPosition]
	GroupStart                  Evidence[NodeRef]
	FiguresComplete             Evidence[bool]
	DurationRaw                 Evidence[Decimal]
	AnchorUnits                 Evidence[Decimal]
	DurationUnits               Evidence[Decimal]
	Tempo                       Evidence[Decimal]
	Dynamics                    Evidence[Decimal]
	ReferenceVelocity           Evidence[Rational]
	EndDynamics                 Evidence[Decimal]
	EndReferenceVelocity        Evidence[Rational]
	Pizzicato                   Evidence[bool]
	TimeOnly                    Evidence[string]
	Pan                         Evidence[Decimal]
	Elevation                   Evidence[Decimal]
	MIDIUnpitched               Evidence[Integer]
	MIDINote                    Evidence[Rational]
	SwingType                   Evidence[string]
	SwingFirst                  Evidence[Integer]
	SwingSecond                 Evidence[Integer]
	Straight                    Evidence[bool]
	// SwingEligible is bounded to an encoded note type. Known absence of type
	// excludes this projection, even if duration could describe a swingable note.
	SwingEligible          Evidence[bool]
	SwingRealized          Evidence[bool]
	SwungOnset             Evidence[ContextPosition]
	SwungEnd               Evidence[ContextPosition]
	BendAlter              Evidence[Decimal]
	PreBend                Evidence[bool]
	BendRelease            Evidence[bool]
	BendNegativeApplicable Evidence[bool]
	BendNegativeSatisfied  Evidence[bool]
	Staves                 Evidence[Integer]
	Instruments            Evidence[Integer]
	StaffExists            Evidence[bool]
	TranspositionChromatic Evidence[Decimal]
	TranspositionDiatonic  Evidence[Integer]
	TranspositionOctave    Evidence[Integer]
	ChromaticDisplacement  Evidence[Rational]
	DiatonicDisplacement   Evidence[Rational]
	DoublingAbove          Evidence[bool]
	WrittenStep            Evidence[string]
	WrittenAlter           Evidence[Decimal]
	WrittenOctave          Evidence[Integer]
	WrittenPitch           Evidence[Rational]
	SoundingPitch          Evidence[Rational]
	StaffType              Evidence[string]
	Capo                   Evidence[Integer]
	KeyFifths              Evidence[Integer]
	KeyMode                Evidence[string]
	KeyCancel              Evidence[Integer]
	CancelLocation         Evidence[string]
	TimeSymbol             Evidence[string]
	TimeSeparator          Evidence[string]
	SenzaMisura            Evidence[string]
	TimePrintObject        Evidence[bool]
	ClefSign               Evidence[string]
	ClefLine               Evidence[Integer]
	ClefOctave             Evidence[Integer]
}

func cloneTimingAdjustment(p TimingAdjustment) TimingAdjustment {
	q := p
	q.Raw = cloneEvidence(p.Raw)
	q.Units = cloneEvidence(p.Units)
	q.Delta = cloneEvidence(p.Delta)
	q.Origin = cloneEvidence(p.Origin)
	q.Position = cloneEvidence(p.Position)
	return q
}
func cloneTimingPercent(p TimingPercent) TimingPercent {
	q := p
	q.Raw = cloneEvidence(p.Raw)
	q.Percent = cloneEvidence(p.Percent)
	q.Origin = cloneEvidence(p.Origin)
	q.Position = cloneEvidence(p.Position)
	return q
}
func cloneTimingDonor(p TimingDonor) TimingDonor {
	q := p
	q.Note = cloneEvidence(p.Note)
	q.Raw = cloneEvidence(p.Raw)
	q.DonorDuration = cloneEvidence(p.DonorDuration)
	q.Fraction = cloneEvidence(p.Fraction)
	q.StolenDuration = cloneEvidence(p.StolenDuration)
	return q
}
func cloneTimingBend(p TimingBend) TimingBend {
	q := p
	q.OriginalNote = cloneEvidence(p.OriginalNote)
	q.ReleasePresent = cloneEvidence(p.ReleasePresent)
	q.RawBeats = cloneEvidence(p.RawBeats)
	q.Beats = cloneEvidence(p.Beats)
	q.RawAccelerate = cloneEvidence(p.RawAccelerate)
	q.Accelerate = cloneEvidence(p.Accelerate)
	q.Release = cloneTimingAdjustment(p.Release)
	q.FirstBeat = cloneTimingPercent(p.FirstBeat)
	q.LastBeat = cloneTimingPercent(p.LastBeat)
	return q
}
func cloneTimingTuning(p TimingTuning) TimingTuning {
	q := p
	q.Line = cloneEvidence(p.Line)
	q.Octave = cloneEvidence(p.Octave)
	q.Step = cloneEvidence(p.Step)
	q.Alter = cloneEvidence(p.Alter)
	q.Capo = cloneEvidence(p.Capo)
	q.OpenPitch = cloneEvidence(p.OpenPitch)
	q.AdjustedPitch = cloneEvidence(p.AdjustedPitch)
	return q
}
func cloneTimingContextResult(p TimingContextResult) TimingContextResult {
	q := p
	q.AllStaves = cloneEvidence(p.AllStaves)
	q.ScopeStaff = cloneEvidence(p.ScopeStaff)
	q.PartOrder = cloneEvidence(p.PartOrder)
	q.MeasureLabel = cloneEvidence(p.MeasureLabel)
	q.Advisories = cloneTimingAdvice(p.Advisories)
	q.CursorAfter = cloneEvidence(p.CursorAfter)
	q.SwingRatio = cloneEvidence(p.SwingRatio)
	q.StateOrigins = cloneTimingStateOrigins(p.StateOrigins)
	q.Spatial = cloneTimingSpatial(p.Spatial)
	q.Meter = cloneTimingMeter(p.Meter)
	q.FretNumber = cloneEvidence(p.FretNumber)
	q.FretOpenString = cloneEvidence(p.FretOpenString)
	q.EditorialPlaybackSuppressed = cloneEvidence(p.EditorialPlaybackSuppressed)
	q.TrillIncludesInitialSound = cloneEvidence(p.TrillIncludesInitialSound)
	q.TrillExcludesClosingTurn = cloneEvidence(p.TrillExcludesClosingTurn)
	q.LayoutOrigin = cloneEvidence(p.LayoutOrigin)
	q.TuningBottomUp = cloneEvidence(p.TuningBottomUp)
	q.StaffTopDown = cloneEvidence(p.StaffTopDown)
	q.RootAlterPrintObject = cloneEvidence(p.RootAlterPrintObject)
	q.EffectiveTempo = cloneEvidence(p.EffectiveTempo)
	q.EffectiveDynamics = cloneEvidence(p.EffectiveDynamics)
	q.EffectivePizzicato = cloneEvidence(p.EffectivePizzicato)
	q.EffectiveVelocity = cloneEvidence(p.EffectiveVelocity)
	q.PitchIntent = cloneEvidence(p.PitchIntent)
	q.Origin = cloneEvidence(p.Origin)
	q.Position = cloneEvidence(p.Position)
	q.End = cloneEvidence(p.End)
	q.Duration = cloneEvidence(p.Duration)
	q.PrincipalDuration = cloneEvidence(p.PrincipalDuration)
	q.Principal = cloneEvidence(p.Principal)
	q.Units = cloneEvidence(p.Units)
	q.Staff = cloneEvidence(p.Staff)
	q.Voice = cloneEvidence(p.Voice)
	q.Part = cloneEvidence(p.Part)
	q.ScoreOnset = cloneEvidence(p.ScoreOnset)
	q.ScoreEnd = cloneEvidence(p.ScoreEnd)
	q.Chord = cloneEvidence(p.Chord)
	q.Grace = cloneEvidence(p.Grace)
	q.Cue = cloneEvidence(p.Cue)
	q.BendsComplete = cloneEvidence(p.BendsComplete)
	q.GraceNode = cloneEvidence(p.GraceNode)
	q.StealPrevious = cloneEvidence(p.StealPrevious)
	q.StealFollowing = cloneEvidence(p.StealFollowing)
	q.Anchor = cloneEvidence(p.Anchor)
	q.AnchorPosition = cloneEvidence(p.AnchorPosition)
	q.AnchorStaff = cloneEvidence(p.AnchorStaff)
	q.AnchorVoice = cloneEvidence(p.AnchorVoice)
	q.ChainPosition = cloneEvidence(p.ChainPosition)
	q.ChainEnd = cloneEvidence(p.ChainEnd)
	q.GroupStart = cloneEvidence(p.GroupStart)
	q.FiguresComplete = cloneEvidence(p.FiguresComplete)
	q.DurationRaw = cloneEvidence(p.DurationRaw)
	q.AnchorUnits = cloneEvidence(p.AnchorUnits)
	q.DurationUnits = cloneEvidence(p.DurationUnits)
	q.Tempo = cloneEvidence(p.Tempo)
	q.Dynamics = cloneEvidence(p.Dynamics)
	q.ReferenceVelocity = cloneEvidence(p.ReferenceVelocity)
	q.EndDynamics = cloneEvidence(p.EndDynamics)
	q.EndReferenceVelocity = cloneEvidence(p.EndReferenceVelocity)
	q.Pizzicato = cloneEvidence(p.Pizzicato)
	q.TimeOnly = cloneEvidence(p.TimeOnly)
	q.Pan = cloneEvidence(p.Pan)
	q.Elevation = cloneEvidence(p.Elevation)
	q.MIDIUnpitched = cloneEvidence(p.MIDIUnpitched)
	q.MIDINote = cloneEvidence(p.MIDINote)
	q.SwingType = cloneEvidence(p.SwingType)
	q.SwingFirst = cloneEvidence(p.SwingFirst)
	q.SwingSecond = cloneEvidence(p.SwingSecond)
	q.Straight = cloneEvidence(p.Straight)
	q.SwingEligible = cloneEvidence(p.SwingEligible)
	q.SwingRealized = cloneEvidence(p.SwingRealized)
	q.SwungOnset = cloneEvidence(p.SwungOnset)
	q.SwungEnd = cloneEvidence(p.SwungEnd)
	q.BendAlter = cloneEvidence(p.BendAlter)
	q.PreBend = cloneEvidence(p.PreBend)
	q.BendRelease = cloneEvidence(p.BendRelease)
	q.BendNegativeApplicable = cloneEvidence(p.BendNegativeApplicable)
	q.BendNegativeSatisfied = cloneEvidence(p.BendNegativeSatisfied)
	q.Staves = cloneEvidence(p.Staves)
	q.Instruments = cloneEvidence(p.Instruments)
	q.StaffExists = cloneEvidence(p.StaffExists)
	q.TranspositionChromatic = cloneEvidence(p.TranspositionChromatic)
	q.TranspositionDiatonic = cloneEvidence(p.TranspositionDiatonic)
	q.TranspositionOctave = cloneEvidence(p.TranspositionOctave)
	q.ChromaticDisplacement = cloneEvidence(p.ChromaticDisplacement)
	q.DiatonicDisplacement = cloneEvidence(p.DiatonicDisplacement)
	q.DoublingAbove = cloneEvidence(p.DoublingAbove)
	q.WrittenStep = cloneEvidence(p.WrittenStep)
	q.WrittenAlter = cloneEvidence(p.WrittenAlter)
	q.WrittenOctave = cloneEvidence(p.WrittenOctave)
	q.WrittenPitch = cloneEvidence(p.WrittenPitch)
	q.SoundingPitch = cloneEvidence(p.SoundingPitch)
	q.StaffType = cloneEvidence(p.StaffType)
	q.Capo = cloneEvidence(p.Capo)
	q.KeyFifths = cloneEvidence(p.KeyFifths)
	q.KeyMode = cloneEvidence(p.KeyMode)
	q.KeyCancel = cloneEvidence(p.KeyCancel)
	q.CancelLocation = cloneEvidence(p.CancelLocation)
	q.TimeSymbol = cloneEvidence(p.TimeSymbol)
	q.TimeSeparator = cloneEvidence(p.TimeSeparator)
	q.SenzaMisura = cloneEvidence(p.SenzaMisura)
	q.TimePrintObject = cloneEvidence(p.TimePrintObject)
	q.ClefSign = cloneEvidence(p.ClefSign)
	q.ClefLine = cloneEvidence(p.ClefLine)
	q.ClefOctave = cloneEvidence(p.ClefOctave)
	q.Play = cloneTimingPlayInstructions(p.Play)
	q.PlayComplete = cloneEvidence(p.PlayComplete)
	q.Attack = cloneTimingAdjustment(p.Attack)
	q.Release = cloneTimingAdjustment(p.Release)
	q.MakeTime = cloneTimingAdjustment(p.MakeTime)
	q.Previous = cloneTimingDonor(p.Previous)
	q.Following = cloneTimingDonor(p.Following)
	q.Bends = make([]TimingBend, len(p.Bends))
	for i, v := range p.Bends {
		q.Bends[i] = cloneTimingBend(v)
	}
	if p.Figures != nil {
		q.Figures = make([]NodeRef, len(p.Figures))
		copy(q.Figures, p.Figures)
	}
	q.Tunings = make([]TimingTuning, len(p.Tunings))
	for i, v := range p.Tunings {
		q.Tunings[i] = cloneTimingTuning(v)
	}
	return q
}
func cloneTimingContext(ctx context.Context, s *Session, p *TimingContextResult) (*TimingContextResult, error) {
	if p == nil {
		return nil, nil
	}
	if err := contextRetainProjection(ctx, s, *p); err != nil {
		return nil, err
	}
	q := cloneTimingContextResult(*p)
	return &q, nil
}
func performancePosition(v Evidence[musicalPosition]) Evidence[ContextPosition] {
	return contextPositionEvidence(v)
}

func contextMusicalPosition(p ContextPosition) musicalPosition {
	return musicalPosition{Part: p.Part, Measure: p.Measure, MeasureOrdinal: p.MeasureOrdinal, Offset: p.Offset, UnprovedFrame: p.UnprovedFrame}
}

func contextPositionEvidence(v Evidence[musicalPosition]) Evidence[ContextPosition] {
	o := stateUnavailable[musicalPosition, ContextPosition](v)
	if v.State == EvidenceKnown {
		p := v.Value
		o.Value = ContextPosition{Part: p.Part, Measure: p.Measure, MeasureOrdinal: p.MeasureOrdinal, Offset: p.Offset, UnprovedFrame: p.UnprovedFrame}
	}
	return o
}
func performanceAdjustment(v noteTimingAdjustment) TimingAdjustment {
	return TimingAdjustment{Raw: v.Raw, Units: v.Units, Delta: v.Delta, Origin: performancePosition(v.Origin), Position: performancePosition(v.Position)}
}
func performancePercent(v noteTimingPercent) TimingPercent {
	return TimingPercent{Raw: v.Raw, Percent: v.Percent, Origin: performancePosition(v.Origin), Position: performancePosition(v.Position)}
}
func performanceDonor(v graceDonorTiming) TimingDonor {
	return TimingDonor{Note: v.Note, Raw: v.Raw, DonorDuration: v.DonorDuration, Fraction: v.Fraction, StolenDuration: v.StolenDuration}
}
func newTimingContext(kind Code) TimingContextResult {
	return TimingContextResult{
		Kind:         kind,
		AllStaves:    Evidence[bool]{State: EvidenceAbsent},
		ScopeStaff:   Evidence[Integer]{State: EvidenceAbsent},
		PartOrder:    Evidence[Integer]{State: EvidenceAbsent},
		MeasureLabel: Evidence[string]{State: EvidenceAbsent},
		CursorAfter:  Evidence[ContextPosition]{State: EvidenceAbsent},
		SwingRatio:   Evidence[Rational]{State: EvidenceAbsent},
		Attack:       absentTimingAdjustment(), Release: absentTimingAdjustment(), MakeTime: absentTimingAdjustment(), Previous: absentTimingDonor(), Following: absentTimingDonor(), PlayComplete: Evidence[bool]{State: EvidenceAbsent},
		FretNumber:                  Evidence[Integer]{State: EvidenceAbsent},
		FretOpenString:              Evidence[bool]{State: EvidenceAbsent},
		EditorialPlaybackSuppressed: Evidence[bool]{State: EvidenceAbsent},
		TrillIncludesInitialSound:   Evidence[bool]{State: EvidenceAbsent},
		TrillExcludesClosingTurn:    Evidence[bool]{State: EvidenceAbsent},
		LayoutOrigin:                Evidence[NodeRef]{State: EvidenceAbsent},
		TuningBottomUp:              Evidence[bool]{State: EvidenceAbsent},
		StaffTopDown:                Evidence[bool]{State: EvidenceAbsent},
		RootAlterPrintObject:        Evidence[bool]{State: EvidenceAbsent},
		EffectiveTempo:              Evidence[Decimal]{State: EvidenceAbsent},
		EffectiveDynamics:           Evidence[Decimal]{State: EvidenceAbsent},
		EffectivePizzicato:          Evidence[bool]{State: EvidenceAbsent},
		EffectiveVelocity:           Evidence[Rational]{State: EvidenceAbsent},
		PitchIntent:                 Evidence[Rational]{State: EvidenceAbsent},
		Origin:                      Evidence[ContextPosition]{State: EvidenceAbsent},
		Position:                    Evidence[ContextPosition]{State: EvidenceAbsent},
		End:                         Evidence[ContextPosition]{State: EvidenceAbsent},
		Duration:                    Evidence[Rational]{State: EvidenceAbsent},
		PrincipalDuration:           Evidence[Rational]{State: EvidenceAbsent},
		Principal:                   Evidence[NodeRef]{State: EvidenceAbsent},
		Units:                       Evidence[Decimal]{State: EvidenceAbsent},
		Staff:                       Evidence[Integer]{State: EvidenceAbsent},
		Voice:                       Evidence[string]{State: EvidenceAbsent},
		Part:                        Evidence[NodeRef]{State: EvidenceAbsent},
		ScoreOnset:                  Evidence[ContextPosition]{State: EvidenceAbsent},
		ScoreEnd:                    Evidence[ContextPosition]{State: EvidenceAbsent},
		Chord:                       Evidence[bool]{State: EvidenceAbsent},
		Grace:                       Evidence[bool]{State: EvidenceAbsent},
		Cue:                         Evidence[bool]{State: EvidenceAbsent},
		BendsComplete:               Evidence[bool]{State: EvidenceAbsent},
		GraceNode:                   Evidence[NodeRef]{State: EvidenceAbsent},
		StealPrevious:               Evidence[Decimal]{State: EvidenceAbsent},
		StealFollowing:              Evidence[Decimal]{State: EvidenceAbsent},
		Anchor:                      Evidence[NodeRef]{State: EvidenceAbsent},
		AnchorPosition:              Evidence[ContextPosition]{State: EvidenceAbsent},
		AnchorStaff:                 Evidence[Integer]{State: EvidenceAbsent},
		AnchorVoice:                 Evidence[string]{State: EvidenceAbsent},
		ChainPosition:               Evidence[ContextPosition]{State: EvidenceAbsent},
		ChainEnd:                    Evidence[ContextPosition]{State: EvidenceAbsent},
		GroupStart:                  Evidence[NodeRef]{State: EvidenceAbsent},
		FiguresComplete:             Evidence[bool]{State: EvidenceAbsent},
		DurationRaw:                 Evidence[Decimal]{State: EvidenceAbsent},
		AnchorUnits:                 Evidence[Decimal]{State: EvidenceAbsent},
		DurationUnits:               Evidence[Decimal]{State: EvidenceAbsent},
		Tempo:                       Evidence[Decimal]{State: EvidenceAbsent},
		Dynamics:                    Evidence[Decimal]{State: EvidenceAbsent},
		ReferenceVelocity:           Evidence[Rational]{State: EvidenceAbsent},
		EndDynamics:                 Evidence[Decimal]{State: EvidenceAbsent},
		EndReferenceVelocity:        Evidence[Rational]{State: EvidenceAbsent},
		Pizzicato:                   Evidence[bool]{State: EvidenceAbsent},
		TimeOnly:                    Evidence[string]{State: EvidenceAbsent},
		Pan:                         Evidence[Decimal]{State: EvidenceAbsent},
		Elevation:                   Evidence[Decimal]{State: EvidenceAbsent},
		MIDIUnpitched:               Evidence[Integer]{State: EvidenceAbsent},
		MIDINote:                    Evidence[Rational]{State: EvidenceAbsent},
		SwingType:                   Evidence[string]{State: EvidenceAbsent},
		SwingFirst:                  Evidence[Integer]{State: EvidenceAbsent},
		SwingSecond:                 Evidence[Integer]{State: EvidenceAbsent},
		Straight:                    Evidence[bool]{State: EvidenceAbsent},
		SwingEligible:               Evidence[bool]{State: EvidenceAbsent},
		SwingRealized:               Evidence[bool]{State: EvidenceAbsent},
		SwungOnset:                  Evidence[ContextPosition]{State: EvidenceAbsent},
		SwungEnd:                    Evidence[ContextPosition]{State: EvidenceAbsent},
		BendAlter:                   Evidence[Decimal]{State: EvidenceAbsent},
		PreBend:                     Evidence[bool]{State: EvidenceAbsent},
		BendRelease:                 Evidence[bool]{State: EvidenceAbsent},
		BendNegativeApplicable:      Evidence[bool]{State: EvidenceAbsent},
		BendNegativeSatisfied:       Evidence[bool]{State: EvidenceAbsent},
		Staves:                      Evidence[Integer]{State: EvidenceAbsent},
		Instruments:                 Evidence[Integer]{State: EvidenceAbsent},
		StaffExists:                 Evidence[bool]{State: EvidenceAbsent},
		TranspositionChromatic:      Evidence[Decimal]{State: EvidenceAbsent},
		TranspositionDiatonic:       Evidence[Integer]{State: EvidenceAbsent},
		TranspositionOctave:         Evidence[Integer]{State: EvidenceAbsent},
		ChromaticDisplacement:       Evidence[Rational]{State: EvidenceAbsent},
		DiatonicDisplacement:        Evidence[Rational]{State: EvidenceAbsent},
		DoublingAbove:               Evidence[bool]{State: EvidenceAbsent},
		WrittenStep:                 Evidence[string]{State: EvidenceAbsent},
		WrittenAlter:                Evidence[Decimal]{State: EvidenceAbsent},
		WrittenOctave:               Evidence[Integer]{State: EvidenceAbsent},
		WrittenPitch:                Evidence[Rational]{State: EvidenceAbsent},
		SoundingPitch:               Evidence[Rational]{State: EvidenceAbsent},
		StaffType:                   Evidence[string]{State: EvidenceAbsent},
		Capo:                        Evidence[Integer]{State: EvidenceAbsent},
		KeyFifths:                   Evidence[Integer]{State: EvidenceAbsent},
		KeyMode:                     Evidence[string]{State: EvidenceAbsent},
		KeyCancel:                   Evidence[Integer]{State: EvidenceAbsent},
		CancelLocation:              Evidence[string]{State: EvidenceAbsent},
		TimeSymbol:                  Evidence[string]{State: EvidenceAbsent},
		TimeSeparator:               Evidence[string]{State: EvidenceAbsent},
		SenzaMisura:                 Evidence[string]{State: EvidenceAbsent},
		TimePrintObject:             Evidence[bool]{State: EvidenceAbsent},
		ClefSign:                    Evidence[string]{State: EvidenceAbsent},
		ClefLine:                    Evidence[Integer]{State: EvidenceAbsent},
		ClefOctave:                  Evidence[Integer]{State: EvidenceAbsent}}
}

func absentTimingAdjustment() TimingAdjustment {
	return TimingAdjustment{Raw: Evidence[Decimal]{State: EvidenceAbsent}, Units: Evidence[Decimal]{State: EvidenceAbsent}, Delta: Evidence[Rational]{State: EvidenceAbsent}, Origin: Evidence[ContextPosition]{State: EvidenceAbsent}, Position: Evidence[ContextPosition]{State: EvidenceAbsent}}
}
func absentTimingDonor() TimingDonor {
	return TimingDonor{Note: Evidence[NodeRef]{State: EvidenceAbsent}, Raw: Evidence[Decimal]{State: EvidenceAbsent}, DonorDuration: Evidence[Rational]{State: EvidenceAbsent}, Fraction: Evidence[Rational]{State: EvidenceAbsent}, StolenDuration: Evidence[Rational]{State: EvidenceAbsent}}
}
