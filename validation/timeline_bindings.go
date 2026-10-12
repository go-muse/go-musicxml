package validation

import (
	"fmt"
	"slices"
	"strings"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

// timelineRole names a fixed MusicXML role, never a runtime string-key fact.
// A role may have several exact declaration uses (notably note choice branches).
type timelineRole uint16

const (
	timelineRoleInvalid timelineRole = iota
	timelineRoleRoot
	timelineRolePart
	timelineRoleMeasure
	timelineRolePartList
	timelineRoleNote
	timelineRoleBackup
	timelineRoleForward
	timelineRoleAttributes
	timelineRoleHarmony
	timelineRoleDirection
	timelineRoleSound
	timelineRoleNoteChord
	timelineRoleNoteDuration
	timelineRoleNoteGrace
	timelineRoleNoteCue
	timelineRoleNoteVoice
	timelineRoleNoteStaff
	timelineRoleNoteInstrument
	timelineRoleNotePlay
	timelineRoleBackupDuration
	timelineRoleForwardDuration
	timelineRoleForwardVoice
	timelineRoleForwardStaff
	timelineRoleAttributesDivisions
	timelineRoleAttributesKey
	timelineRoleAttributesTime
	timelineRoleAttributesStaves
	timelineRoleAttributesInstruments
	timelineRoleAttributesClef
	timelineRoleAttributesTranspose
	timelineRoleAttributesForPart
	timelineRoleAttributesStaffDetails
	timelineRoleKeyFifths
	timelineRoleKeyMode
	timelineRoleKeyCancel
	timelineRoleKeyStep
	timelineRoleKeyAlter
	timelineRoleKeyAccidental
	timelineRoleKeyOctave
	timelineRoleTimeBeats
	timelineRoleTimeBeatType
	timelineRoleTimeSymbol
	timelineRoleTimeSeparator
	timelineRoleInterchangeableBeats
	timelineRoleInterchangeableBeatType
	timelineRoleInterchangeableSymbol
	timelineRoleInterchangeableSeparator
	timelineRoleTimeSenzaMisura
	timelineRoleTimeInterchangeable
	timelineRoleInterchangeableRelation
	timelineRoleClefSign
	timelineRoleClefLine
	timelineRoleClefClefOctaveChange
	timelineRolePartClefSign
	timelineRolePartClefLine
	timelineRolePartClefClefOctaveChange
	timelineRoleClefAdditional
	timelineRoleClefSize
	timelineRoleClefAfterBarline
	timelineRoleTransposeDiatonic
	timelineRoleTransposeChromatic
	timelineRoleTransposeOctaveChange
	timelineRoleTransposeDouble
	timelineRolePartTransposeDiatonic
	timelineRolePartTransposeChromatic
	timelineRolePartTransposeOctaveChange
	timelineRolePartTransposeDouble
	timelineRoleDoubleAbove
	timelineRoleForPartPartClef
	timelineRoleForPartPartTranspose
	timelineRoleKeyNumber
	timelineRoleTimeNumber
	timelineRoleClefNumber
	timelineRoleTransposeNumber
	timelineRoleForPartNumber
	timelineRoleStaffDetailsNumber
	timelineRoleKeyOctaveNumber
	timelineRoleKeyOctaveCancel
	timelineRoleCancelLocation
	timelineRoleKeyAccidentalSMuFL
	timelineRoleStaffDetailsStaffType
	timelineRoleStaffDetailsStaffLines
	timelineRoleStaffDetailsLineDetail
	timelineRoleStaffDetailsStaffTuning
	timelineRoleStaffDetailsCapo
	timelineRoleStaffDetailsStaffSize
	timelineRoleStaffTuningTuningStep
	timelineRoleStaffTuningTuningAlter
	timelineRoleStaffTuningTuningOctave
	timelineRoleStaffTuningLine
	timelineRoleHarmonyOffset
	timelineRoleHarmonyStaff
	timelineRoleDirectionOffset
	timelineRoleDirectionStaff
	timelineRoleHarmonyNumeral
	timelineRoleDirectionVoice
	timelineRoleDirectionSound
	timelineRoleNumeralKey
	timelineRoleNumeralKeyFifths
	timelineRoleNumeralKeyMode
	timelineRoleSoundOffset
	timelineRoleOffsetSound
	timelineRoleScorePartMidiDevice
	timelineRoleScorePartMidiInstrument
	timelineRoleSoundMidiDevice
	timelineRoleSoundMidiInstrument
	timelineRoleSoundInstrumentChange
	timelineRoleSoundPlay
	timelineRoleSoundSwing
	timelineRoleInstrumentID
	timelineRoleMidiDeviceID
	timelineRoleMidiInstrumentID
	timelineRoleInstrumentChangeID
	timelineRolePlayID
	timelineRoleScorePartID
	timelineRoleScoreInstrumentID
	timelineRoleScoreInstrument
	timelineRoleScorePart
	timelineRoleMidiInstrumentMidiChannel
	timelineRoleMidiInstrumentMidiName
	timelineRoleMidiInstrumentMidiBank
	timelineRoleMidiInstrumentMidiProgram
	timelineRoleMidiInstrumentMidiUnpitched
	timelineRoleMidiInstrumentVolume
	timelineRoleMidiInstrumentPan
	timelineRoleMidiInstrumentElevation
	timelineRoleMidiDevicePort
	timelineRoleScoreInstrumentInstrumentSound
	timelineRoleScoreInstrumentSolo
	timelineRoleScoreInstrumentEnsemble
	timelineRoleScoreInstrumentVirtualInstrument
	timelineRoleInstrumentChangeInstrumentSound
	timelineRoleInstrumentChangeSolo
	timelineRoleInstrumentChangeEnsemble
	timelineRoleInstrumentChangeVirtualInstrument
	timelineRoleVirtualInstrumentVirtualLibrary
	timelineRoleVirtualInstrumentVirtualName
	timelineRolePlayIpa
	timelineRolePlayMute
	timelineRolePlaySemiPitched
	timelineRolePlayOtherPlay
	timelineRoleSoundTempo
	timelineRoleSoundDynamics
	timelineRoleSoundDivisions
	timelineRoleSoundPizzicato
	timelineRoleSoundPan
	timelineRoleSoundElevation
	timelineRoleSoundDamperPedal
	timelineRoleSoundSoftPedal
	timelineRoleSoundSostenutoPedal
	timelineRoleOtherPlayType
	timelineRoleScoreInstrumentInstrumentName
	timelineRoleScoreInstrumentInstrumentAbbreviation
	timelineRoleLineDetailLine
	timelineRoleStaffSizeScaling
	timelineRoleStaffDetailsShowFrets
	timelineRoleStaffDetailsPrintObject
	timelineRoleStaffDetailsPrintSpacing
	timelineRoleLineDetailWidth
	timelineRoleLineDetailColor
	timelineRoleLineDetailLineType
	timelineRoleLineDetailPrintObject
	timelineRoleListening
	timelineRoleDirectionListening
	timelineRoleListeningOffset
	timelineRoleNoteAttack
	timelineRoleNoteRelease
	timelineRoleNoteNotations
	timelineRoleNotationsTechnical
	timelineRoleTechnicalBend
	timelineRoleBendRelease
	timelineRoleBendReleaseOffset
	timelineRoleBendFirstBeat
	timelineRoleBendLastBeat
	timelineRoleBendBeats
	timelineRoleBendAccelerate
	timelineRoleGraceStealPrevious
	timelineRoleGraceStealFollowing
	timelineRoleGraceMakeTime
	timelineRoleFiguredBass
	timelineRoleFiguredBassFigure
	timelineRoleFiguredBassDuration
	timelineRoleKeyPrintObject
	timelineRoleTimePrintObject
	timelineRoleClefPrintObject
	timelineRoleLimit
)

// timelineHierarchy retains the two distinct anonymous hierarchy chains. The
// content owner is the partwise measure or the timewise part, respectively.
type timelineHierarchy struct {
	Root, Part, Measure SchemaUse
	Content             ComponentID
	PartID              AttributeSelector
}

type timelineBindings struct {
	Roles       [timelineRoleLimit][]SchemaUse
	Hierarchies [2]timelineHierarchy
}

type timelineRoleShape struct {
	role             timelineRole
	owner, name, typ string
	count            int
	attribute        bool
}

// This finite table is a configuration-time binding contract against MusicXML
// 4.0 metadata. Runtime readers use only the copied exact descriptors.
var timelineRoleShapes = [...]timelineRoleShape{
	{timelineRoleKeyPrintObject, "key", "print-object", "yes-no", 1, true},
	{timelineRoleTimePrintObject, "time", "print-object", "yes-no", 1, true},
	{timelineRoleClefPrintObject, "clef", "print-object", "yes-no", 1, true},
	{timelineRoleStaffDetailsShowFrets, "staff-details", "show-frets", "show-frets", 1, true},
	{timelineRoleStaffDetailsPrintObject, "staff-details", "print-object", "yes-no", 1, true},
	{timelineRoleStaffDetailsPrintSpacing, "staff-details", "print-spacing", "yes-no", 1, true},
	{timelineRoleLineDetailWidth, "line-detail", "width", "tenths", 1, true},
	{timelineRoleLineDetailColor, "line-detail", "color", "color", 1, true},
	{timelineRoleLineDetailLineType, "line-detail", "line-type", "line-type", 1, true},
	{timelineRoleLineDetailPrintObject, "line-detail", "print-object", "yes-no", 1, true},
	{timelineRoleFiguredBassFigure, "figured-bass", "figure", "figure", 1, false},
	{timelineRoleFiguredBassDuration, "figured-bass", "duration", "positive-divisions", 1, false},
	{timelineRoleNoteAttack, "note", "attack", "divisions", 1, true},
	{timelineRoleNoteRelease, "note", "release", "divisions", 1, true},
	{timelineRoleNoteNotations, "note", "notations", "notations", 1, false},
	{timelineRoleNotationsTechnical, "notations", "technical", "technical", 1, false},
	{timelineRoleTechnicalBend, "technical", "bend", "bend", 1, false},
	{timelineRoleBendRelease, "bend", "release", "release", 1, false},
	{timelineRoleBendReleaseOffset, "release", "offset", "divisions", 1, true},
	{timelineRoleBendFirstBeat, "bend", "first-beat", "percent", 1, true},
	{timelineRoleBendLastBeat, "bend", "last-beat", "percent", 1, true},
	{timelineRoleBendBeats, "bend", "beats", "trill-beats", 1, true},
	{timelineRoleBendAccelerate, "bend", "accelerate", "yes-no", 1, true},
	{timelineRoleGraceStealPrevious, "grace", "steal-time-previous", "percent", 1, true},
	{timelineRoleGraceStealFollowing, "grace", "steal-time-following", "percent", 1, true},
	{timelineRoleGraceMakeTime, "grace", "make-time", "divisions", 1, true},

	{timelineRoleLineDetailLine, "line-detail", "line", "staff-line", 1, true},
	{timelineRoleStaffSizeScaling, "staff-size", "scaling", "non-negative-decimal", 1, true},
	{timelineRoleOtherPlayType, "other-play", "type", "xs:token", 1, true},
	{timelineRoleScoreInstrumentInstrumentName, "score-instrument", "instrument-name", "xs:string", 1, false},
	{timelineRoleScoreInstrumentInstrumentAbbreviation, "score-instrument", "instrument-abbreviation", "xs:string", 1, false},
	{timelineRoleNoteChord, "note", "chord", "empty", 4, false},
	{timelineRoleNoteDuration, "note", "duration", "positive-divisions", 2, false},
	{timelineRoleNoteGrace, "note", "grace", "grace", 1, false},
	{timelineRoleNoteCue, "note", "cue", "empty", 2, false},
	{timelineRoleNoteVoice, "note", "voice", "xs:string", 1, false},
	{timelineRoleNoteStaff, "note", "staff", "xs:positiveInteger", 1, false},
	{timelineRoleNoteInstrument, "note", "instrument", "instrument", 1, false},
	{timelineRoleNotePlay, "note", "play", "play", 1, false},
	{timelineRoleBackupDuration, "backup", "duration", "positive-divisions", 1, false},
	{timelineRoleForwardDuration, "forward", "duration", "positive-divisions", 1, false},
	{timelineRoleForwardVoice, "forward", "voice", "xs:string", 1, false},
	{timelineRoleForwardStaff, "forward", "staff", "xs:positiveInteger", 1, false},
	{timelineRoleAttributesDivisions, "attributes", "divisions", "positive-divisions", 1, false},
	{timelineRoleAttributesKey, "attributes", "key", "key", 1, false},
	{timelineRoleAttributesTime, "attributes", "time", "time", 1, false},
	{timelineRoleAttributesStaves, "attributes", "staves", "xs:nonNegativeInteger", 1, false},
	{timelineRoleAttributesInstruments, "attributes", "instruments", "xs:nonNegativeInteger", 1, false},
	{timelineRoleAttributesClef, "attributes", "clef", "clef", 1, false},
	{timelineRoleAttributesTranspose, "attributes", "transpose", "transpose", 1, false},
	{timelineRoleAttributesForPart, "attributes", "for-part", "for-part", 1, false},
	{timelineRoleAttributesStaffDetails, "attributes", "staff-details", "staff-details", 1, false},
	{timelineRoleKeyFifths, "key", "fifths", "fifths", 1, false},
	{timelineRoleKeyMode, "key", "mode", "mode", 1, false},
	{timelineRoleKeyCancel, "key", "cancel", "cancel", 1, false},
	{timelineRoleKeyStep, "key", "key-step", "step", 1, false},
	{timelineRoleKeyAlter, "key", "key-alter", "semitones", 1, false},
	{timelineRoleKeyAccidental, "key", "key-accidental", "key-accidental", 1, false},
	{timelineRoleKeyOctave, "key", "key-octave", "key-octave", 1, false},
	{timelineRoleTimeBeats, "time", "beats", "xs:string", 1, false},
	{timelineRoleTimeBeatType, "time", "beat-type", "xs:string", 1, false},
	{timelineRoleTimeSymbol, "time", "symbol", "time-symbol", 1, true},
	{timelineRoleTimeSeparator, "time", "separator", "time-separator", 1, true},
	{timelineRoleInterchangeableBeats, "interchangeable", "beats", "xs:string", 1, false},
	{timelineRoleInterchangeableBeatType, "interchangeable", "beat-type", "xs:string", 1, false},
	{timelineRoleInterchangeableSymbol, "interchangeable", "symbol", "time-symbol", 1, true},
	{timelineRoleInterchangeableSeparator, "interchangeable", "separator", "time-separator", 1, true},
	{timelineRoleTimeSenzaMisura, "time", "senza-misura", "xs:string", 1, false},
	{timelineRoleTimeInterchangeable, "time", "interchangeable", "interchangeable", 1, false},
	{timelineRoleInterchangeableRelation, "interchangeable", "time-relation", "time-relation", 1, false},
	{timelineRoleClefSign, "clef", "sign", "clef-sign", 1, false},
	{timelineRoleClefLine, "clef", "line", "staff-line-position", 1, false},
	{timelineRoleClefClefOctaveChange, "clef", "clef-octave-change", "xs:integer", 1, false},
	{timelineRolePartClefSign, "part-clef", "sign", "clef-sign", 1, false},
	{timelineRolePartClefLine, "part-clef", "line", "staff-line-position", 1, false},
	{timelineRolePartClefClefOctaveChange, "part-clef", "clef-octave-change", "xs:integer", 1, false},
	{timelineRoleClefAdditional, "clef", "additional", "yes-no", 1, true},
	{timelineRoleClefSize, "clef", "size", "symbol-size", 1, true},
	{timelineRoleClefAfterBarline, "clef", "after-barline", "yes-no", 1, true},
	{timelineRoleTransposeDiatonic, "transpose", "diatonic", "xs:integer", 1, false},
	{timelineRoleTransposeChromatic, "transpose", "chromatic", "semitones", 1, false},
	{timelineRoleTransposeOctaveChange, "transpose", "octave-change", "xs:integer", 1, false},
	{timelineRoleTransposeDouble, "transpose", "double", "double", 1, false},
	{timelineRolePartTransposeDiatonic, "part-transpose", "diatonic", "xs:integer", 1, false},
	{timelineRolePartTransposeChromatic, "part-transpose", "chromatic", "semitones", 1, false},
	{timelineRolePartTransposeOctaveChange, "part-transpose", "octave-change", "xs:integer", 1, false},
	{timelineRolePartTransposeDouble, "part-transpose", "double", "double", 1, false},
	{timelineRoleDoubleAbove, "double", "above", "yes-no", 1, true},
	{timelineRoleForPartPartClef, "for-part", "part-clef", "part-clef", 1, false},
	{timelineRoleForPartPartTranspose, "for-part", "part-transpose", "part-transpose", 1, false},
	{timelineRoleKeyNumber, "key", "number", "staff-number", 1, true},
	{timelineRoleTimeNumber, "time", "number", "staff-number", 1, true},
	{timelineRoleClefNumber, "clef", "number", "staff-number", 1, true},
	{timelineRoleTransposeNumber, "transpose", "number", "staff-number", 1, true},
	{timelineRoleForPartNumber, "for-part", "number", "staff-number", 1, true},
	{timelineRoleStaffDetailsNumber, "staff-details", "number", "staff-number", 1, true},
	{timelineRoleKeyOctaveNumber, "key-octave", "number", "xs:positiveInteger", 1, true},
	{timelineRoleKeyOctaveCancel, "key-octave", "cancel", "yes-no", 1, true},
	{timelineRoleCancelLocation, "cancel", "location", "cancel-location", 1, true},
	{timelineRoleKeyAccidentalSMuFL, "key-accidental", "smufl", "smufl-accidental-glyph-name", 1, true},
	{timelineRoleStaffDetailsStaffType, "staff-details", "staff-type", "staff-type", 1, false},
	{timelineRoleStaffDetailsStaffLines, "staff-details", "staff-lines", "xs:nonNegativeInteger", 1, false},
	{timelineRoleStaffDetailsLineDetail, "staff-details", "line-detail", "line-detail", 1, false},
	{timelineRoleStaffDetailsStaffTuning, "staff-details", "staff-tuning", "staff-tuning", 1, false},
	{timelineRoleStaffDetailsCapo, "staff-details", "capo", "xs:nonNegativeInteger", 1, false},
	{timelineRoleStaffDetailsStaffSize, "staff-details", "staff-size", "staff-size", 1, false},
	{timelineRoleStaffTuningTuningStep, "staff-tuning", "tuning-step", "step", 1, false},
	{timelineRoleStaffTuningTuningAlter, "staff-tuning", "tuning-alter", "semitones", 1, false},
	{timelineRoleStaffTuningTuningOctave, "staff-tuning", "tuning-octave", "octave", 1, false},
	{timelineRoleStaffTuningLine, "staff-tuning", "line", "staff-line", 1, true},
	{timelineRoleHarmonyOffset, "harmony", "offset", "offset", 1, false},
	{timelineRoleHarmonyStaff, "harmony", "staff", "xs:positiveInteger", 1, false},
	{timelineRoleDirectionOffset, "direction", "offset", "offset", 1, false},
	{timelineRoleDirectionStaff, "direction", "staff", "xs:positiveInteger", 1, false},
	{timelineRoleHarmonyNumeral, "harmony", "numeral", "numeral", 1, false},
	{timelineRoleDirectionVoice, "direction", "voice", "xs:string", 1, false},
	{timelineRoleDirectionSound, "direction", "sound", "sound", 1, false},
	{timelineRoleDirectionListening, "direction", "listening", "listening", 1, false},
	{timelineRoleListeningOffset, "listening", "offset", "offset", 1, false},
	{timelineRoleNumeralKey, "numeral", "numeral-key", "numeral-key", 1, false},
	{timelineRoleNumeralKeyFifths, "numeral-key", "numeral-fifths", "fifths", 1, false},
	{timelineRoleNumeralKeyMode, "numeral-key", "numeral-mode", "numeral-mode", 1, false},
	{timelineRoleSoundOffset, "sound", "offset", "offset", 1, false},
	{timelineRoleOffsetSound, "offset", "sound", "yes-no", 1, true},
	{timelineRoleScorePartMidiDevice, "score-part", "midi-device", "midi-device", 1, false},
	{timelineRoleScorePartMidiInstrument, "score-part", "midi-instrument", "midi-instrument", 1, false},
	{timelineRoleSoundMidiDevice, "sound", "midi-device", "midi-device", 1, false},
	{timelineRoleSoundMidiInstrument, "sound", "midi-instrument", "midi-instrument", 1, false},
	{timelineRoleSoundInstrumentChange, "sound", "instrument-change", "instrument-change", 1, false},
	{timelineRoleSoundPlay, "sound", "play", "play", 1, false},
	{timelineRoleSoundSwing, "sound", "swing", "swing", 1, false},
	{timelineRoleInstrumentID, "instrument", "id", "xs:IDREF", 1, true},
	{timelineRoleMidiDeviceID, "midi-device", "id", "xs:IDREF", 1, true},
	{timelineRoleMidiInstrumentID, "midi-instrument", "id", "xs:IDREF", 1, true},
	{timelineRoleInstrumentChangeID, "instrument-change", "id", "xs:IDREF", 1, true},
	{timelineRolePlayID, "play", "id", "xs:IDREF", 1, true},
	{timelineRoleScorePartID, "score-part", "id", "xs:ID", 1, true},
	{timelineRoleScoreInstrumentID, "score-instrument", "id", "xs:ID", 1, true},
	{timelineRoleScoreInstrument, "score-part", "score-instrument", "score-instrument", 1, false},
	{timelineRoleScorePart, "part-list", "score-part", "score-part", 2, false},
	{timelineRoleMidiInstrumentMidiChannel, "midi-instrument", "midi-channel", "midi-16", 1, false},
	{timelineRoleMidiInstrumentMidiName, "midi-instrument", "midi-name", "xs:string", 1, false},
	{timelineRoleMidiInstrumentMidiBank, "midi-instrument", "midi-bank", "midi-16384", 1, false},
	{timelineRoleMidiInstrumentMidiProgram, "midi-instrument", "midi-program", "midi-128", 1, false},
	{timelineRoleMidiInstrumentMidiUnpitched, "midi-instrument", "midi-unpitched", "midi-128", 1, false},
	{timelineRoleMidiInstrumentVolume, "midi-instrument", "volume", "percent", 1, false},
	{timelineRoleMidiInstrumentPan, "midi-instrument", "pan", "rotation-degrees", 1, false},
	{timelineRoleMidiInstrumentElevation, "midi-instrument", "elevation", "rotation-degrees", 1, false},
	{timelineRoleMidiDevicePort, "midi-device", "port", "midi-16", 1, true},
	{timelineRoleScoreInstrumentInstrumentSound, "score-instrument", "instrument-sound", "xs:string", 1, false},
	{timelineRoleScoreInstrumentSolo, "score-instrument", "solo", "empty", 1, false},
	{timelineRoleScoreInstrumentEnsemble, "score-instrument", "ensemble", "positive-integer-or-empty", 1, false},
	{timelineRoleScoreInstrumentVirtualInstrument, "score-instrument", "virtual-instrument", "virtual-instrument", 1, false},
	{timelineRoleInstrumentChangeInstrumentSound, "instrument-change", "instrument-sound", "xs:string", 1, false},
	{timelineRoleInstrumentChangeSolo, "instrument-change", "solo", "empty", 1, false},
	{timelineRoleInstrumentChangeEnsemble, "instrument-change", "ensemble", "positive-integer-or-empty", 1, false},
	{timelineRoleInstrumentChangeVirtualInstrument, "instrument-change", "virtual-instrument", "virtual-instrument", 1, false},
	{timelineRoleVirtualInstrumentVirtualLibrary, "virtual-instrument", "virtual-library", "xs:string", 1, false},
	{timelineRoleVirtualInstrumentVirtualName, "virtual-instrument", "virtual-name", "xs:string", 1, false},
	{timelineRolePlayIpa, "play", "ipa", "xs:string", 1, false},
	{timelineRolePlayMute, "play", "mute", "mute", 1, false},
	{timelineRolePlaySemiPitched, "play", "semi-pitched", "semi-pitched", 1, false},
	{timelineRolePlayOtherPlay, "play", "other-play", "other-play", 1, false},
	{timelineRoleSoundTempo, "sound", "tempo", "non-negative-decimal", 1, true},
	{timelineRoleSoundDynamics, "sound", "dynamics", "non-negative-decimal", 1, true},
	{timelineRoleSoundDivisions, "sound", "divisions", "divisions", 1, true},
	{timelineRoleSoundPizzicato, "sound", "pizzicato", "yes-no", 1, true},
	{timelineRoleSoundPan, "sound", "pan", "rotation-degrees", 1, true},
	{timelineRoleSoundElevation, "sound", "elevation", "rotation-degrees", 1, true},
	{timelineRoleSoundDamperPedal, "sound", "damper-pedal", "yes-no-number", 1, true},
	{timelineRoleSoundSoftPedal, "sound", "soft-pedal", "yes-no-number", 1, true},
	{timelineRoleSoundSostenutoPedal, "sound", "sostenuto-pedal", "yes-no-number", 1, true},
}

func timelineNamedType(name string) ComponentID {
	space := ""
	if strings.HasPrefix(name, "xs:") {
		space, name = "http://www.w3.org/2001/XMLSchema", strings.TrimPrefix(name, "xs:")
	}
	return ComponentID{Assembly: AssemblyMusicXML, Kind: ComponentType, Name: QName{Space: space, Local: name}}
}
func timelineShapeError(owner, name string) error {
	return contract("timeline schema roles", fmt.Sprintf("missing, ambiguous or incompatible exact %s/%s declaration shape", owner, name))
}
func compileTimelineBindings(schema *CompiledSchema) (*timelineBindings, error) {
	if schema == nil || schema.kernel == nil || schema.assembly != AssemblyMusicXML {
		return nil, contract("timeline schema", "compiled MusicXML assembly required")
	}
	// Uses is already bounded by schema compilation. Build one configuration-time
	// index, then bind the finite role table without repeated whole-schema scans.
	uses := schema.Uses()
	types := make(map[ComponentID]xsd.TypeID)
	for _, typ := range schema.kernel.Components() {
		types[schema.component(typ)] = typ
	}
	type inspectedShape struct {
		shape xsd.TypeContentShape
		known bool
	}
	shapeCache := make(map[ComponentID]inspectedShape)
	shapeMatches := func(typ ComponentID, name string) bool {
		expected, known := timelineExpectedShape(name)
		inspected, found := shapeCache[typ]
		if !found {
			inspected.shape, inspected.known = types[typ].ContentShape()
			shapeCache[typ] = inspected
		}
		return known && inspected.known && inspected.shape == expected
	}
	type useKey struct {
		owner     ComponentID
		name      QName
		attribute bool
	}
	owned := make(map[useKey][]SchemaUse)
	roots := make(map[DeclarationID]SchemaUse)
	for _, u := range uses {
		if u.OwnerTypeKnown {
			k := useKey{u.OwnerType, u.Name, u.Attribute}
			owned[k] = append(owned[k], u)
		} else if !u.Attribute {
			roots[u.Declaration] = u
		}
	}
	out := &timelineBindings{}
	find := func(owner ComponentID, name, typ string, count int, attribute bool) ([]SchemaUse, error) {
		us := owned[useKey{owner, QName{Local: name}, attribute}]
		if len(us) != count {
			return nil, timelineShapeError(owner.Name.Local, name)
		}
		for _, u := range us {
			if !validDeclaration(u.Declaration) || u.Declaration.Assembly != AssemblyMusicXML || !u.DeclaredTypeKnown || !validComponent(u.DeclaredType) || u.DeclaredType.Assembly != AssemblyMusicXML {
				return nil, timelineShapeError(owner.Name.Local, name)
			}
			if !shapeMatches(u.DeclaredType, typ) {
				return nil, timelineShapeError(owner.Name.Local, name)
			}
			if typ == "" {
				if u.DeclaredType.Name != (QName{}) || u.DeclaredType.Occurrence == "" {
					return nil, timelineShapeError(owner.Name.Local, name)
				}
			} else if u.DeclaredType != timelineNamedType(typ) {
				return nil, timelineShapeError(owner.Name.Local, name)
			}
		}
		return us, nil
	}
	for i, name := range []string{"score-partwise", "score-timewise"} {
		d, ok := schema.Root(QName{Local: name})
		u, exists := roots[d]
		if !ok || !exists || u.Name != (QName{Local: name}) || !u.DeclaredTypeKnown || u.DeclaredType.Name != (QName{}) || u.DeclaredType.Occurrence == "" {
			return nil, timelineShapeError("root", name)
		}
		if !shapeMatches(u.DeclaredType, "") {
			return nil, timelineShapeError("root", name)
		}
		h := timelineHierarchy{Root: u}
		first, second := "part", "measure"
		if i == 1 {
			first, second = second, first
		}
		a, e := find(u.DeclaredType, first, "", 1, false)
		if e != nil {
			return nil, e
		}
		b, e := find(a[0].DeclaredType, second, "", 1, false)
		if e != nil {
			return nil, e
		}
		if i == 0 {
			h.Part, h.Measure = a[0], b[0]
		} else {
			h.Measure, h.Part = a[0], b[0]
		}
		h.Content = b[0].DeclaredType
		ids, e := find(h.Part.DeclaredType, "id", "xs:IDREF", 1, true)
		if e != nil {
			return nil, e
		}
		h.PartID = AttributeSelector{Name: ids[0].Name, Declaration: ids[0].Declaration}
		lists, e := find(u.DeclaredType, "part-list", "part-list", 1, false)
		if e != nil {
			return nil, e
		}
		out.Roles[timelineRolePartList] = append(out.Roles[timelineRolePartList], lists...)
		out.Hierarchies[i] = h
		out.Roles[timelineRoleRoot] = append(out.Roles[timelineRoleRoot], h.Root)
		out.Roles[timelineRolePart] = append(out.Roles[timelineRolePart], h.Part)
		out.Roles[timelineRoleMeasure] = append(out.Roles[timelineRoleMeasure], h.Measure)
		for _, event := range []struct {
			role timelineRole
			name string
		}{{timelineRoleNote, "note"}, {timelineRoleBackup, "backup"}, {timelineRoleForward, "forward"}, {timelineRoleAttributes, "attributes"}, {timelineRoleHarmony, "harmony"}, {timelineRoleDirection, "direction"}, {timelineRoleSound, "sound"}, {timelineRoleListening, "listening"}, {timelineRoleFiguredBass, "figured-bass"}} {
			events, e := find(h.Content, event.name, event.name, 1, false)
			if e != nil {
				return nil, e
			}
			out.Roles[event.role] = append(out.Roles[event.role], events...)
		}
	}
	for _, shape := range timelineRoleShapes {
		us, e := find(timelineNamedType(shape.owner), shape.name, shape.typ, shape.count, shape.attribute)
		if e != nil {
			return nil, e
		}
		out.Roles[shape.role] = append(out.Roles[shape.role], us...)
	}
	for i := range out.Roles {
		slices.SortFunc(out.Roles[i], func(a, b SchemaUse) int { return compareLocalDeclaration(a.Declaration, b.Declaration) })
	}
	return out, nil
}

// matches examines a fixed small role set. Runtime callers must charge their
// bounded role examination work before calling it.
func (b *timelineBindings) matches(role timelineRole, d DeclarationID) bool {
	if b == nil || role <= timelineRoleInvalid || role >= timelineRoleLimit {
		return false
	}
	for _, u := range b.Roles[role] {
		if u.Declaration == d {
			return true
		}
	}
	return false
}
func (b *timelineBindings) attribute(role timelineRole) (AttributeSelector, bool) {
	if b == nil || role <= timelineRoleInvalid || role >= timelineRoleLimit || len(b.Roles[role]) != 1 || !b.Roles[role][0].Attribute {
		return AttributeSelector{}, false
	}
	u := b.Roles[role][0]
	return AttributeSelector{Name: u.Name, Declaration: u.Declaration}, true
}

// declarations is configuration-only; it materializes a sorted identity set.
// Runtime callers borrow Roles and account any retained collection themselves.
func (b *timelineBindings) declarations(role timelineRole) []DeclarationID {
	if b == nil || role <= timelineRoleInvalid || role >= timelineRoleLimit {
		return nil
	}
	out := make([]DeclarationID, len(b.Roles[role]))
	for i, u := range b.Roles[role] {
		out[i] = u.Declaration
	}
	return out
}
func cloneTimelineUse(u SchemaUse) SchemaUse {
	u.Declaration = cloneDeclaration(u.Declaration)
	u.Name = QName{strings.Clone(u.Name.Space), strings.Clone(u.Name.Local)}
	u.OwnerType = cloneComponent(u.OwnerType)
	u.DeclaredType = cloneComponent(u.DeclaredType)
	return u
}
func (b *timelineBindings) clone() *timelineBindings {
	if b == nil {
		return nil
	}
	out := *b
	for i, us := range b.Roles {
		out.Roles[i] = make([]SchemaUse, len(us))
		for j, u := range us {
			out.Roles[i][j] = cloneTimelineUse(u)
		}
	}
	for i, h := range b.Hierarchies {
		out.Hierarchies[i] = timelineHierarchy{Root: cloneTimelineUse(h.Root), Part: cloneTimelineUse(h.Part), Measure: cloneTimelineUse(h.Measure), Content: cloneComponent(h.Content), PartID: cloneLocalAttribute(h.PartID)}
	}
	return &out
}
func timelineUseSize(u SchemaUse) int64 {
	return int64(512 + len(u.Name.Space) + len(u.Name.Local) + len(u.Declaration.OwnerOccurrence) + len(u.Declaration.Occurrence) + len(u.OwnerType.Name.Space) + len(u.OwnerType.Name.Local) + len(u.OwnerType.Occurrence) + len(u.DeclaredType.Name.Space) + len(u.DeclaredType.Name.Local) + len(u.DeclaredType.Occurrence))
}
func (b *timelineBindings) size() int64 {
	if b == nil {
		return 0
	}
	n := int64(4096)
	for _, us := range b.Roles {
		for _, u := range us {
			n += timelineUseSize(u)
		}
	}
	for _, h := range b.Hierarchies {
		n += timelineUseSize(h.Root) + timelineUseSize(h.Part) + timelineUseSize(h.Measure) + localAttributeSize(h.PartID) + int64(128+len(h.Content.Occurrence))
	}
	return n
}

// Expected scalar families are explicit; a familiar named type is insufficient
// when its underlying graph has been changed to another value family.
func timelineExpectedShape(name string) (xsd.TypeContentShape, bool) {
	switch name {
	case "", "attributes", "note", "backup", "forward", "direction", "harmony", "clef", "double", "empty", "for-part", "grace", "instrument", "instrument-change", "interchangeable", "key", "line-detail", "listening", "midi-instrument", "numeral", "numeral-key", "part-clef", "part-list", "part-transpose", "play", "score-instrument", "score-part", "sound", "staff-details", "staff-tuning", "swing", "time", "transpose", "virtual-instrument", "bend", "notations", "technical", "release", "figured-bass", "figure":
		return xsd.ShapeComplex | xsd.ShapeElements, true
	case "cancel", "key-octave":
		return xsd.ShapeComplex | xsd.ShapeInteger, true
	case "offset", "staff-size":
		return xsd.ShapeComplex | xsd.ShapeDecimal, true
	case "key-accidental", "midi-device", "other-play":
		return xsd.ShapeComplex | xsd.ShapeString, true
	case "fifths", "midi-128", "midi-16", "midi-16384", "octave", "staff-line", "staff-line-position", "staff-number", "xs:integer", "xs:positiveInteger", "xs:nonNegativeInteger":
		return xsd.ShapeInteger, true
	case "tenths", "divisions", "non-negative-decimal", "percent", "positive-divisions", "rotation-degrees", "semitones", "xs:decimal", "trill-beats":
		return xsd.ShapeDecimal, true
	case "positive-integer-or-empty":
		return xsd.ShapeUnion | xsd.ShapeInteger | xsd.ShapeString, true
	case "yes-no-number":
		return xsd.ShapeUnion | xsd.ShapeDecimal | xsd.ShapeString, true
	case "color", "line-type", "show-frets", "cancel-location", "clef-sign", "mode", "mute", "numeral-mode", "semi-pitched", "smufl-accidental-glyph-name", "staff-type", "step", "symbol-size", "time-relation", "time-separator", "time-symbol", "yes-no", "xs:string", "xs:token", "xs:ID", "xs:IDREF":
		return xsd.ShapeString, true
	}
	return 0, false
}
