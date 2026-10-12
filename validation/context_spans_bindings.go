package validation

const (
	spanRole_metronome        contextRole = "span.metronome"
	spanRole_frame            contextRole = "span.frame"
	spanRole_frameNote        contextRole = "span.frameNote"
	spanRole_frameString      contextRole = "span.frameString"
	spanRole_frameFret        contextRole = "span.frameFret"
	spanRole_bracketLineEnd   contextRole = "span.bracketLineEnd"
	spanRole_bracketEndLength contextRole = "span.bracketEndLength"
	spanRole_noteAttack       contextRole = "span.noteAttack"
	spanRole_noteRelease      contextRole = "span.noteRelease"

	spanRole_noteTimeOnly                  contextRole = "span.noteTimeOnly"
	spanRole_arpeggiate_arpeggio_direction contextRole = "span.arpeggiate_direction"
	spanRole_slur                          contextRole = "span.slur"
	spanRole_tied                          contextRole = "span.tied"
	spanRole_tie                           contextRole = "span.tie"
	spanRole_beam                          contextRole = "span.beam"
	spanRole_tuplet                        contextRole = "span.tuplet"
	spanRole_glissando                     contextRole = "span.glissando"
	spanRole_slide                         contextRole = "span.slide"
	spanRole_octaveShift                   contextRole = "span.octaveShift"
	spanRole_bracket                       contextRole = "span.bracket"
	spanRole_dashes                        contextRole = "span.dashes"
	spanRole_wavyLine                      contextRole = "span.wavyLine"
	spanRole_barlineWavy                   contextRole = "span.barlineWavy"
	spanRole_hammerOn                      contextRole = "span.hammerOn"
	spanRole_pullOff                       contextRole = "span.pullOff"
	spanRole_lyricExtend                   contextRole = "span.lyricExtend"
	spanRole_figureExtend                  contextRole = "span.figureExtend"
	spanRole_partGroup                     contextRole = "span.partGroup"
	spanRole_barre                         contextRole = "span.barre"
	spanRole_arpeggiate                    contextRole = "span.arpeggiate"
	spanRole_nonArpeggiate                 contextRole = "span.nonArpeggiate"
	spanRole_pedal                         contextRole = "span.pedal"
	spanRole_wedge                         contextRole = "span.wedge"
	spanRole_otherNotation                 contextRole = "span.otherNotation"
	spanRole_principalVoice                contextRole = "span.principalVoice"
	spanRole_metronomeTied                 contextRole = "span.metronomeTied"
	spanRole_tremolo                       contextRole = "span.tremolo"
	spanRole_note                          contextRole = "span.note"
	spanRole_direction                     contextRole = "span.direction"
	spanRole_barline                       contextRole = "span.barline"
	spanRole_grouping                      contextRole = "span.grouping"
	spanRole_partList                      contextRole = "span.partList"
	spanRole_scorePart                     contextRole = "span.scorePart"
	spanRole_arpeggiate_number             contextRole = "span.arpeggiate_number"
	spanRole_barre_type                    contextRole = "span.barre_type"
	spanRole_beam_number                   contextRole = "span.beam_number"
	spanRole_beam_repeater                 contextRole = "span.beam_repeater"
	spanRole_beam_fan                      contextRole = "span.beam_fan"
	spanRole_bracket_type                  contextRole = "span.bracket_type"
	spanRole_bracket_number                contextRole = "span.bracket_number"
	spanRole_bracket_line_type             contextRole = "span.bracket_line_type"
	spanRole_dashes_type                   contextRole = "span.dashes_type"
	spanRole_dashes_number                 contextRole = "span.dashes_number"
	spanRole_extend_type                   contextRole = "span.extend_type"
	spanRole_glissando_type                contextRole = "span.glissando_type"
	spanRole_glissando_number              contextRole = "span.glissando_number"
	spanRole_glissando_line_type           contextRole = "span.glissando_line_type"
	spanRole_grouping_type                 contextRole = "span.grouping_type"
	spanRole_grouping_number               contextRole = "span.grouping_number"
	spanRole_grouping_member_of            contextRole = "span.grouping_member_of"
	spanRole_hammer_on_pull_off_type       contextRole = "span.hammer_on_pull_off_type"
	spanRole_hammer_on_pull_off_number     contextRole = "span.hammer_on_pull_off_number"
	spanRole_lyric_number                  contextRole = "span.lyric_number"
	spanRole_lyric_name                    contextRole = "span.lyric_name"
	spanRole_lyric_time_only               contextRole = "span.lyric_time_only"
	spanRole_metronome_tied_type           contextRole = "span.metronome_tied_type"
	spanRole_non_arpeggiate_type           contextRole = "span.non_arpeggiate_type"
	spanRole_non_arpeggiate_number         contextRole = "span.non_arpeggiate_number"
	spanRole_octave_shift_type             contextRole = "span.octave_shift_type"
	spanRole_octave_shift_number           contextRole = "span.octave_shift_number"
	spanRole_octave_shift_size             contextRole = "span.octave_shift_size"
	spanRole_other_notation_type           contextRole = "span.other_notation_type"
	spanRole_other_notation_number         contextRole = "span.other_notation_number"
	spanRole_part_group_type               contextRole = "span.part_group_type"
	spanRole_part_group_number             contextRole = "span.part_group_number"
	spanRole_pedal_type                    contextRole = "span.pedal_type"
	spanRole_pedal_number                  contextRole = "span.pedal_number"
	spanRole_pedal_line                    contextRole = "span.pedal_line"
	spanRole_pedal_sign                    contextRole = "span.pedal_sign"
	spanRole_pedal_abbreviated             contextRole = "span.pedal_abbreviated"
	spanRole_principal_voice_type          contextRole = "span.principal_voice_type"
	spanRole_slide_type                    contextRole = "span.slide_type"
	spanRole_slide_number                  contextRole = "span.slide_number"
	spanRole_slide_line_type               contextRole = "span.slide_line_type"
	spanRole_slur_type                     contextRole = "span.slur_type"
	spanRole_slur_number                   contextRole = "span.slur_number"
	spanRole_slur_line_type                contextRole = "span.slur_line_type"
	spanRole_tie_type                      contextRole = "span.tie_type"
	spanRole_tie_time_only                 contextRole = "span.tie_time_only"
	spanRole_tied_type                     contextRole = "span.tied_type"
	spanRole_tied_number                   contextRole = "span.tied_number"
	spanRole_tied_line_type                contextRole = "span.tied_line_type"
	spanRole_tremolo_type                  contextRole = "span.tremolo_type"
	spanRole_tuplet_type                   contextRole = "span.tuplet_type"
	spanRole_tuplet_number                 contextRole = "span.tuplet_number"
	spanRole_tuplet_bracket                contextRole = "span.tuplet_bracket"
	spanRole_tuplet_show_number            contextRole = "span.tuplet_show_number"
	spanRole_tuplet_show_type              contextRole = "span.tuplet_show_type"
	spanRole_tuplet_line_shape             contextRole = "span.tuplet_line_shape"
	spanRole_wavy_line_type                contextRole = "span.wavy_line_type"
	spanRole_wavy_line_number              contextRole = "span.wavy_line_number"
	spanRole_wedge_type                    contextRole = "span.wedge_type"
	spanRole_wedge_number                  contextRole = "span.wedge_number"
	spanRole_wedge_spread                  contextRole = "span.wedge_spread"
	spanRole_wedge_niente                  contextRole = "span.wedge_niente"
	spanRole_wedge_line_type               contextRole = "span.wedge_line_type"
	spanRole_noteGrace                     contextRole = "span.noteGrace"
	spanRole_noteCue                       contextRole = "span.noteCue"
	spanRole_noteVoice                     contextRole = "span.noteVoice"
	spanRole_noteStaff                     contextRole = "span.noteStaff"
	spanRole_notePitch                     contextRole = "span.notePitch"
	spanRole_noteType                      contextRole = "span.noteType"
	spanRole_noteDot                       contextRole = "span.noteDot"
	spanRole_noteModification              contextRole = "span.noteModification"
	spanRole_noteLyric                     contextRole = "span.noteLyric"
	spanRole_pitchStep                     contextRole = "span.pitchStep"
	spanRole_pitchAlter                    contextRole = "span.pitchAlter"
	spanRole_pitchOctave                   contextRole = "span.pitchOctave"
	spanRole_tupletActual                  contextRole = "span.tupletActual"
	spanRole_tupletNormal                  contextRole = "span.tupletNormal"
	spanRole_tupletNumber                  contextRole = "span.tupletNumber"
	spanRole_tupletType                    contextRole = "span.tupletType"
	spanRole_tupletDot                     contextRole = "span.tupletDot"
	spanRole_actualNotes                   contextRole = "span.actualNotes"
	spanRole_normalNotes                   contextRole = "span.normalNotes"
	spanRole_normalType                    contextRole = "span.normalType"
	spanRole_normalDot                     contextRole = "span.normalDot"
	spanRole_groupSymbol                   contextRole = "span.groupSymbol"
	spanRole_lyricText                     contextRole = "span.lyricText"
	spanRole_lyricElision                  contextRole = "span.lyricElision"
	spanRole_lyricSyllabic                 contextRole = "span.lyricSyllabic"
	spanRole_version                       contextRole = "span.version"
)

func contextSpanBindingSpecs() []contextBindingSpec {
	return []contextBindingSpec{
		{Role: spanRole_metronome, Owner: "", Name: "metronome", Type: "metronome", Attribute: false, Count: -1},
		{Role: spanRole_frame, Owner: "", Name: "frame", Type: "frame", Attribute: false, Count: -1},
		{Role: spanRole_frameNote, Owner: "", Name: "frame-note", Type: "frame-note", Attribute: false, Count: -1},
		{Role: spanRole_frameString, Owner: "frame-note", Name: "string", Type: "string", Attribute: false, Count: 1},
		{Role: spanRole_frameFret, Owner: "frame-note", Name: "fret", Type: "fret", Attribute: false, Count: 1},
		{Role: spanRole_bracketLineEnd, Owner: "bracket", Name: "line-end", Type: "line-end", Attribute: true, Count: 1},
		{Role: spanRole_bracketEndLength, Owner: "bracket", Name: "end-length", Type: "tenths", Attribute: true, Count: 1},
		{Role: spanRole_noteAttack, Owner: "note", Name: "attack", Type: "divisions", Attribute: true, Count: 1},
		{Role: spanRole_noteRelease, Owner: "note", Name: "release", Type: "divisions", Attribute: true, Count: 1},

		{Role: spanRole_noteTimeOnly, Owner: "note", Name: "time-only", Type: "time-only", Attribute: true, Count: 1},
		{Role: spanRole_arpeggiate_arpeggio_direction, Owner: "arpeggiate", Name: "direction", Type: "up-down", Attribute: true, Count: 1},
		{Role: spanRole_slur, Owner: "notations", Name: "slur", Type: "slur", Attribute: false, Count: 1},
		{Role: spanRole_tied, Owner: "notations", Name: "tied", Type: "tied", Attribute: false, Count: 1},
		{Role: spanRole_tie, Owner: "note", Name: "tie", Type: "tie", Attribute: false, Count: 2},
		{Role: spanRole_beam, Owner: "note", Name: "beam", Type: "beam", Attribute: false, Count: 1},
		{Role: spanRole_tuplet, Owner: "notations", Name: "tuplet", Type: "tuplet", Attribute: false, Count: 1},
		{Role: spanRole_glissando, Owner: "notations", Name: "glissando", Type: "glissando", Attribute: false, Count: 1},
		{Role: spanRole_slide, Owner: "notations", Name: "slide", Type: "slide", Attribute: false, Count: 1},
		{Role: spanRole_octaveShift, Owner: "direction-type", Name: "octave-shift", Type: "octave-shift", Attribute: false, Count: 1},
		{Role: spanRole_bracket, Owner: "direction-type", Name: "bracket", Type: "bracket", Attribute: false, Count: 1},
		{Role: spanRole_dashes, Owner: "direction-type", Name: "dashes", Type: "dashes", Attribute: false, Count: 1},
		{Role: spanRole_wavyLine, Owner: "ornaments", Name: "wavy-line", Type: "wavy-line", Attribute: false, Count: 1},
		{Role: spanRole_barlineWavy, Owner: "barline", Name: "wavy-line", Type: "wavy-line", Attribute: false, Count: 1},
		{Role: spanRole_hammerOn, Owner: "technical", Name: "hammer-on", Type: "hammer-on-pull-off", Attribute: false, Count: 1},
		{Role: spanRole_pullOff, Owner: "technical", Name: "pull-off", Type: "hammer-on-pull-off", Attribute: false, Count: 1},
		{Role: spanRole_lyricExtend, Owner: "lyric", Name: "extend", Type: "extend", Attribute: false, Count: 2},
		{Role: spanRole_figureExtend, Owner: "figure", Name: "extend", Type: "extend", Attribute: false, Count: 1},
		{Role: spanRole_partGroup, Owner: "part-list", Name: "part-group", Type: "part-group", Attribute: false, Count: 2},
		{Role: spanRole_barre, Owner: "frame-note", Name: "barre", Type: "barre", Attribute: false, Count: 1},
		{Role: spanRole_arpeggiate, Owner: "notations", Name: "arpeggiate", Type: "arpeggiate", Attribute: false, Count: 1},
		{Role: spanRole_nonArpeggiate, Owner: "notations", Name: "non-arpeggiate", Type: "non-arpeggiate", Attribute: false, Count: 1},
		{Role: spanRole_pedal, Owner: "direction-type", Name: "pedal", Type: "pedal", Attribute: false, Count: 1},
		{Role: spanRole_wedge, Owner: "direction-type", Name: "wedge", Type: "wedge", Attribute: false, Count: 1},
		{Role: spanRole_otherNotation, Owner: "notations", Name: "other-notation", Type: "other-notation", Attribute: false, Count: 1},
		{Role: spanRole_principalVoice, Owner: "direction-type", Name: "principal-voice", Type: "principal-voice", Attribute: false, Count: 1},
		{Role: spanRole_metronomeTied, Owner: "metronome-note", Name: "metronome-tied", Type: "metronome-tied", Attribute: false, Count: 1},
		{Role: spanRole_tremolo, Owner: "ornaments", Name: "tremolo", Type: "tremolo", Attribute: false, Count: 1},
		{Role: spanRole_note, Owner: "", Name: "note", Type: "note", Attribute: false, Count: -1},
		{Role: spanRole_direction, Owner: "", Name: "direction", Type: "direction", Attribute: false, Count: -1},
		{Role: spanRole_barline, Owner: "", Name: "barline", Type: "barline", Attribute: false, Count: -1},
		{Role: spanRole_grouping, Owner: "", Name: "grouping", Type: "grouping", Attribute: false, Count: -1},
		{Role: spanRole_partList, Owner: "", Name: "part-list", Type: "part-list", Attribute: false, Count: -1},
		{Role: spanRole_scorePart, Owner: "", Name: "score-part", Type: "score-part", Attribute: false, Count: -1},
		{Role: spanRole_arpeggiate_number, Owner: "arpeggiate", Name: "number", Type: "number-level", Attribute: true, Count: 1},
		{Role: spanRole_barre_type, Owner: "barre", Name: "type", Type: "start-stop", Attribute: true, Count: 1},
		{Role: spanRole_beam_number, Owner: "beam", Name: "number", Type: "beam-level", Attribute: true, Count: 1},
		{Role: spanRole_beam_repeater, Owner: "beam", Name: "repeater", Type: "yes-no", Attribute: true, Count: 1},
		{Role: spanRole_beam_fan, Owner: "beam", Name: "fan", Type: "fan", Attribute: true, Count: 1},
		{Role: spanRole_bracket_type, Owner: "bracket", Name: "type", Type: "start-stop-continue", Attribute: true, Count: 1},
		{Role: spanRole_bracket_number, Owner: "bracket", Name: "number", Type: "number-level", Attribute: true, Count: 1},
		{Role: spanRole_bracket_line_type, Owner: "bracket", Name: "line-type", Type: "line-type", Attribute: true, Count: 1},
		{Role: spanRole_dashes_type, Owner: "dashes", Name: "type", Type: "start-stop-continue", Attribute: true, Count: 1},
		{Role: spanRole_dashes_number, Owner: "dashes", Name: "number", Type: "number-level", Attribute: true, Count: 1},
		{Role: spanRole_extend_type, Owner: "extend", Name: "type", Type: "start-stop-continue", Attribute: true, Count: 1},
		{Role: spanRole_glissando_type, Owner: "glissando", Name: "type", Type: "start-stop", Attribute: true, Count: 1},
		{Role: spanRole_glissando_number, Owner: "glissando", Name: "number", Type: "number-level", Attribute: true, Count: 1},
		{Role: spanRole_glissando_line_type, Owner: "glissando", Name: "line-type", Type: "line-type", Attribute: true, Count: 1},
		{Role: spanRole_grouping_type, Owner: "grouping", Name: "type", Type: "start-stop-single", Attribute: true, Count: 1},
		{Role: spanRole_grouping_number, Owner: "grouping", Name: "number", Type: "xs:token", Attribute: true, Count: 1},
		{Role: spanRole_grouping_member_of, Owner: "grouping", Name: "member-of", Type: "xs:token", Attribute: true, Count: 1},
		{Role: spanRole_hammer_on_pull_off_type, Owner: "hammer-on-pull-off", Name: "type", Type: "start-stop", Attribute: true, Count: 1},
		{Role: spanRole_hammer_on_pull_off_number, Owner: "hammer-on-pull-off", Name: "number", Type: "number-level", Attribute: true, Count: 1},
		{Role: spanRole_lyric_number, Owner: "lyric", Name: "number", Type: "xs:NMTOKEN", Attribute: true, Count: 1},
		{Role: spanRole_lyric_name, Owner: "lyric", Name: "name", Type: "xs:token", Attribute: true, Count: 1},
		{Role: spanRole_lyric_time_only, Owner: "lyric", Name: "time-only", Type: "time-only", Attribute: true, Count: 1},
		{Role: spanRole_metronome_tied_type, Owner: "metronome-tied", Name: "type", Type: "start-stop", Attribute: true, Count: 1},
		{Role: spanRole_non_arpeggiate_type, Owner: "non-arpeggiate", Name: "type", Type: "top-bottom", Attribute: true, Count: 1},
		{Role: spanRole_non_arpeggiate_number, Owner: "non-arpeggiate", Name: "number", Type: "number-level", Attribute: true, Count: 1},
		{Role: spanRole_octave_shift_type, Owner: "octave-shift", Name: "type", Type: "up-down-stop-continue", Attribute: true, Count: 1},
		{Role: spanRole_octave_shift_number, Owner: "octave-shift", Name: "number", Type: "number-level", Attribute: true, Count: 1},
		{Role: spanRole_octave_shift_size, Owner: "octave-shift", Name: "size", Type: "xs:positiveInteger", Attribute: true, Count: 1},
		{Role: spanRole_other_notation_type, Owner: "other-notation", Name: "type", Type: "start-stop-single", Attribute: true, Count: 1},
		{Role: spanRole_other_notation_number, Owner: "other-notation", Name: "number", Type: "number-level", Attribute: true, Count: 1},
		{Role: spanRole_part_group_type, Owner: "part-group", Name: "type", Type: "start-stop", Attribute: true, Count: 1},
		{Role: spanRole_part_group_number, Owner: "part-group", Name: "number", Type: "xs:token", Attribute: true, Count: 1},
		{Role: spanRole_pedal_type, Owner: "pedal", Name: "type", Type: "pedal-type", Attribute: true, Count: 1},
		{Role: spanRole_pedal_number, Owner: "pedal", Name: "number", Type: "number-level", Attribute: true, Count: 1},
		{Role: spanRole_pedal_line, Owner: "pedal", Name: "line", Type: "yes-no", Attribute: true, Count: 1},
		{Role: spanRole_pedal_sign, Owner: "pedal", Name: "sign", Type: "yes-no", Attribute: true, Count: 1},
		{Role: spanRole_pedal_abbreviated, Owner: "pedal", Name: "abbreviated", Type: "yes-no", Attribute: true, Count: 1},
		{Role: spanRole_principal_voice_type, Owner: "principal-voice", Name: "type", Type: "start-stop", Attribute: true, Count: 1},
		{Role: spanRole_slide_type, Owner: "slide", Name: "type", Type: "start-stop", Attribute: true, Count: 1},
		{Role: spanRole_slide_number, Owner: "slide", Name: "number", Type: "number-level", Attribute: true, Count: 1},
		{Role: spanRole_slide_line_type, Owner: "slide", Name: "line-type", Type: "line-type", Attribute: true, Count: 1},
		{Role: spanRole_slur_type, Owner: "slur", Name: "type", Type: "start-stop-continue", Attribute: true, Count: 1},
		{Role: spanRole_slur_number, Owner: "slur", Name: "number", Type: "number-level", Attribute: true, Count: 1},
		{Role: spanRole_slur_line_type, Owner: "slur", Name: "line-type", Type: "line-type", Attribute: true, Count: 1},
		{Role: spanRole_tie_type, Owner: "tie", Name: "type", Type: "start-stop", Attribute: true, Count: 1},
		{Role: spanRole_tie_time_only, Owner: "tie", Name: "time-only", Type: "time-only", Attribute: true, Count: 1},
		{Role: spanRole_tied_type, Owner: "tied", Name: "type", Type: "tied-type", Attribute: true, Count: 1},
		{Role: spanRole_tied_number, Owner: "tied", Name: "number", Type: "number-level", Attribute: true, Count: 1},
		{Role: spanRole_tied_line_type, Owner: "tied", Name: "line-type", Type: "line-type", Attribute: true, Count: 1},
		{Role: spanRole_tremolo_type, Owner: "tremolo", Name: "type", Type: "tremolo-type", Attribute: true, Count: 1},
		{Role: spanRole_tuplet_type, Owner: "tuplet", Name: "type", Type: "start-stop", Attribute: true, Count: 1},
		{Role: spanRole_tuplet_number, Owner: "tuplet", Name: "number", Type: "number-level", Attribute: true, Count: 1},
		{Role: spanRole_tuplet_bracket, Owner: "tuplet", Name: "bracket", Type: "yes-no", Attribute: true, Count: 1},
		{Role: spanRole_tuplet_show_number, Owner: "tuplet", Name: "show-number", Type: "show-tuplet", Attribute: true, Count: 1},
		{Role: spanRole_tuplet_show_type, Owner: "tuplet", Name: "show-type", Type: "show-tuplet", Attribute: true, Count: 1},
		{Role: spanRole_tuplet_line_shape, Owner: "tuplet", Name: "line-shape", Type: "line-shape", Attribute: true, Count: 1},
		{Role: spanRole_wavy_line_type, Owner: "wavy-line", Name: "type", Type: "start-stop-continue", Attribute: true, Count: 1},
		{Role: spanRole_wavy_line_number, Owner: "wavy-line", Name: "number", Type: "number-level", Attribute: true, Count: 1},
		{Role: spanRole_wedge_type, Owner: "wedge", Name: "type", Type: "wedge-type", Attribute: true, Count: 1},
		{Role: spanRole_wedge_number, Owner: "wedge", Name: "number", Type: "number-level", Attribute: true, Count: 1},
		{Role: spanRole_wedge_spread, Owner: "wedge", Name: "spread", Type: "tenths", Attribute: true, Count: 1},
		{Role: spanRole_wedge_niente, Owner: "wedge", Name: "niente", Type: "yes-no", Attribute: true, Count: 1},
		{Role: spanRole_wedge_line_type, Owner: "wedge", Name: "line-type", Type: "line-type", Attribute: true, Count: 1},
		{Role: spanRole_noteGrace, Owner: "note", Name: "grace", Type: "grace", Attribute: false, Count: 1},
		{Role: spanRole_noteCue, Owner: "note", Name: "cue", Type: "empty", Attribute: false, Count: 2},
		{Role: spanRole_noteVoice, Owner: "note", Name: "voice", Type: "xs:string", Attribute: false, Count: 1},
		{Role: spanRole_noteStaff, Owner: "note", Name: "staff", Type: "xs:positiveInteger", Attribute: false, Count: 1},
		{Role: spanRole_notePitch, Owner: "note", Name: "pitch", Type: "pitch", Attribute: false, Count: 4},
		{Role: spanRole_noteType, Owner: "note", Name: "type", Type: "note-type", Attribute: false, Count: 1},
		{Role: spanRole_noteDot, Owner: "note", Name: "dot", Type: "empty-placement", Attribute: false, Count: 1},
		{Role: spanRole_noteModification, Owner: "note", Name: "time-modification", Type: "time-modification", Attribute: false, Count: 1},
		{Role: spanRole_noteLyric, Owner: "note", Name: "lyric", Type: "lyric", Attribute: false, Count: 1},
		{Role: spanRole_pitchStep, Owner: "pitch", Name: "step", Type: "step", Attribute: false, Count: 1},
		{Role: spanRole_pitchAlter, Owner: "pitch", Name: "alter", Type: "semitones", Attribute: false, Count: 1},
		{Role: spanRole_pitchOctave, Owner: "pitch", Name: "octave", Type: "octave", Attribute: false, Count: 1},
		{Role: spanRole_tupletActual, Owner: "tuplet", Name: "tuplet-actual", Type: "tuplet-portion", Attribute: false, Count: 1},
		{Role: spanRole_tupletNormal, Owner: "tuplet", Name: "tuplet-normal", Type: "tuplet-portion", Attribute: false, Count: 1},
		{Role: spanRole_tupletNumber, Owner: "tuplet-portion", Name: "tuplet-number", Type: "tuplet-number", Attribute: false, Count: 1},
		{Role: spanRole_tupletType, Owner: "tuplet-portion", Name: "tuplet-type", Type: "tuplet-type", Attribute: false, Count: 1},
		{Role: spanRole_tupletDot, Owner: "tuplet-portion", Name: "tuplet-dot", Type: "tuplet-dot", Attribute: false, Count: 1},
		{Role: spanRole_actualNotes, Owner: "time-modification", Name: "actual-notes", Type: "xs:nonNegativeInteger", Attribute: false, Count: 1},
		{Role: spanRole_normalNotes, Owner: "time-modification", Name: "normal-notes", Type: "xs:nonNegativeInteger", Attribute: false, Count: 1},
		{Role: spanRole_normalType, Owner: "time-modification", Name: "normal-type", Type: "note-type-value", Attribute: false, Count: 1},
		{Role: spanRole_normalDot, Owner: "time-modification", Name: "normal-dot", Type: "empty", Attribute: false, Count: 1},
		{Role: spanRole_groupSymbol, Owner: "part-group", Name: "group-symbol", Type: "group-symbol", Attribute: false, Count: 1},
		{Role: spanRole_lyricText, Owner: "lyric", Name: "text", Type: "text-element-data", Attribute: false, Count: 2},
		{Role: spanRole_lyricElision, Owner: "lyric", Name: "elision", Type: "elision", Attribute: false, Count: 1},
		{Role: spanRole_lyricSyllabic, Owner: "lyric", Name: "syllabic", Type: "syllabic", Attribute: false, Count: 2},
		{Role: spanRole_version, Owner: "", Name: "version", Type: "xs:token", Attribute: true, Count: -1},
	}
}
