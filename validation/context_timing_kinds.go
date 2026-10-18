package validation

// TimingContextResult.Kind values. Each names the encoded role a timing row
// describes and selects its applicable fields.
const (
	// TimingKindEvent is a projected encoded event without a more specific role.
	TimingKindEvent Code = "event"
	// TimingKindNote is a note.
	TimingKindNote Code = "note"
	// TimingKindFiguredBass is a figured-bass element.
	TimingKindFiguredBass Code = "figured-bass"
	// TimingKindDirection is a direction.
	TimingKindDirection Code = "direction"
	// TimingKindSound is a sound element.
	TimingKindSound Code = "sound"
	// TimingKindForward is a forward cursor move.
	TimingKindForward Code = "forward"
	// TimingKindBackup is a backup cursor move.
	TimingKindBackup Code = "backup"
	// TimingKindFret is a fret technical mark.
	TimingKindFret Code = "fret"
	// TimingKindEditorialLevel is an editorial level.
	TimingKindEditorialLevel Code = "editorial-level"
	// TimingKindRootAlter is a harmony root alteration.
	TimingKindRootAlter Code = "root-alter"
	// TimingKindTrill is a trill mark.
	TimingKindTrill Code = "trill"
	// TimingKindOrnamentAccidental is an accidental mark within ornaments.
	TimingKindOrnamentAccidental Code = "ornament-accidental"
	// TimingKindMeasureLabel is a measure label.
	TimingKindMeasureLabel Code = "measure-label"
	// TimingKindPartOrder is a part order.
	TimingKindPartOrder Code = "part-order"
	// TimingKindKey is a key signature.
	TimingKindKey Code = "key"
	// TimingKindTime is a time signature.
	TimingKindTime Code = "time"
	// TimingKindTranspose is a transposition.
	TimingKindTranspose Code = "transpose"
	// TimingKindStaffDetails is a staff-details element.
	TimingKindStaffDetails Code = "staff-details"
	// TimingKindMIDIUnpitched is a MIDI unpitched instrument setting.
	TimingKindMIDIUnpitched Code = "midi-unpitched"
	// TimingKindPan is a MIDI pan setting.
	TimingKindPan Code = "pan"
	// TimingKindElevation is a MIDI elevation setting.
	TimingKindElevation Code = "elevation"
	// TimingKindSwing is a swing setting.
	TimingKindSwing Code = "swing"
	// TimingKindBend is a bend technical mark.
	TimingKindBend Code = "bend"
)

// timingKindPerformance starts a performance row; every published row
// replaces it with a role.
const timingKindPerformance Code = "performance"
