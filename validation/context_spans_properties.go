package validation

// Span property identifiers describe independent typed projections; advisory
// values do not change document validity.
const (
	ContextPropertySpanTremoloSMuFL               ContextProperty = "span.tremolo-smufl"
	ContextPropertySpanTremoloSMuFLActive         ContextProperty = "span.tremolo-smufl-active"
	ContextPropertySpanEndpointMeaning            ContextProperty = "span.endpoint-meaning"
	ContextPropertySpanEndLength                  ContextProperty = "span.end-length"
	ContextPropertySpanEndLengthApplicable        ContextProperty = "span.end-length-applicable"
	ContextPropertySpanDisplayOctaves             ContextProperty = "span.display-octaves"
	ContextPropertySpanTremoloDurationRecommended ContextProperty = "span.tremolo-duration-recommended"

	ContextPropertySpanLineType                   ContextProperty = "span.line-type"
	ContextPropertySpanLineShape                  ContextProperty = "span.line-shape"
	ContextPropertySpanSize                       ContextProperty = "span.size"
	ContextPropertySpanFan                        ContextProperty = "span.fan"
	ContextPropertySpanFanApplicable              ContextProperty = "span.fan-applicable"
	ContextPropertySpanRepeater                   ContextProperty = "span.repeater"
	ContextPropertySpanTupletBracket              ContextProperty = "span.tuplet-bracket"
	ContextPropertySpanTupletShowNumber           ContextProperty = "span.tuplet-show-number"
	ContextPropertySpanTupletShowType             ContextProperty = "span.tuplet-show-type"
	ContextPropertySpanTupletActualNumber         ContextProperty = "span.tuplet-actual-number"
	ContextPropertySpanTupletActualType           ContextProperty = "span.tuplet-actual-type"
	ContextPropertySpanTupletActualDots           ContextProperty = "span.tuplet-actual-dots"
	ContextPropertySpanTupletNormalNumber         ContextProperty = "span.tuplet-normal-number"
	ContextPropertySpanTupletNormalType           ContextProperty = "span.tuplet-normal-type"
	ContextPropertySpanTupletNormalDots           ContextProperty = "span.tuplet-normal-dots"
	ContextPropertySpanBeamGrace                  ContextProperty = "span.beam-grace"
	ContextPropertySpanBeamCue                    ContextProperty = "span.beam-cue"
	ContextPropertySpanGroupStopContentActive     ContextProperty = "span.group-stop-content-active"
	ContextPropertySpanGroupSymbol                ContextProperty = "span.group-symbol"
	ContextPropertySpanPedalLine                  ContextProperty = "span.pedal-line"
	ContextPropertySpanPedalSign                  ContextProperty = "span.pedal-sign"
	ContextPropertySpanPedalAbbreviated           ContextProperty = "span.pedal-abbreviated"
	ContextPropertySpanPedalAbbreviatedActive     ContextProperty = "span.pedal-abbreviated-active"
	ContextPropertySpanPedalAlignmentActive       ContextProperty = "span.pedal-alignment-active"
	ContextPropertySpanPedalLineRoleActive        ContextProperty = "span.pedal-line-role-active"
	ContextPropertySpanWedgeSpreadActive          ContextProperty = "span.wedge-spread-active"
	ContextPropertySpanWedgeNiente                ContextProperty = "span.wedge-niente"
	ContextPropertySpanWedgeNienteActive          ContextProperty = "span.wedge-niente-active"
	ContextPropertySpanHammerMarking              ContextProperty = "span.hammer-marking"
	ContextPropertySpanArpeggioDirection          ContextProperty = "span.arpeggio-direction"
	ContextPropertySpanPlaybackEffect             ContextProperty = "span.playback-effect"
	ContextPropertySpanTimeOnly                   ContextProperty = "span.time-only"
	ContextPropertySpanTimeModificationActual     ContextProperty = "span.time-modification-actual"
	ContextPropertySpanTimeModificationNormal     ContextProperty = "span.time-modification-normal"
	ContextPropertySpanTimeModificationMeaning    ContextProperty = "span.time-modification-meaning"
	ContextPropertySpanCurrentType                ContextProperty = "span.current-type"
	ContextPropertySpanCurrentDots                ContextProperty = "span.current-dots"
	ContextPropertySpanNormalType                 ContextProperty = "span.normal-type"
	ContextPropertySpanNormalDots                 ContextProperty = "span.normal-dots"
	ContextPropertySpanLyricSyllable              ContextProperty = "span.lyric-syllable"
	ContextPropertySpanTieAttackRecommended       ContextProperty = "span.tie-attack-recommended"
	ContextPropertySpanTieReleaseRecommended      ContextProperty = "span.tie-release-recommended"
	ContextPropertySpanSameNoteOrderRecommended   ContextProperty = "span.same-note-order-recommended"
	ContextPropertySpanPitchEquivalentRecommended ContextProperty = "span.pitch-equivalent-recommended"
	ContextPropertySpanBarlineContinueRecommended ContextProperty = "span.barline-continue-recommended"
	ContextPropertySpanTremoloMarksRecommended    ContextProperty = "span.tremolo-marks-recommended"
	ContextPropertySpanTremoloRatioRecommended    ContextProperty = "span.tremolo-ratio-recommended"
)

func contextSpanPropertyValid(p ContextProperty) bool {
	switch p {
	case ContextPropertySpanTremoloSMuFL, ContextPropertySpanTremoloSMuFLActive:
		return true
	case ContextPropertySpanEndpointMeaning, ContextPropertySpanEndLength, ContextPropertySpanEndLengthApplicable, ContextPropertySpanDisplayOctaves, ContextPropertySpanTremoloDurationRecommended:
		return true
	case ContextPropertySpanLineType, ContextPropertySpanLineShape, ContextPropertySpanSize, ContextPropertySpanFan, ContextPropertySpanFanApplicable, ContextPropertySpanRepeater, ContextPropertySpanTupletBracket, ContextPropertySpanTupletShowNumber, ContextPropertySpanTupletShowType, ContextPropertySpanTupletActualNumber, ContextPropertySpanTupletActualType, ContextPropertySpanTupletActualDots, ContextPropertySpanTupletNormalNumber, ContextPropertySpanTupletNormalType, ContextPropertySpanTupletNormalDots, ContextPropertySpanBeamGrace, ContextPropertySpanBeamCue, ContextPropertySpanGroupStopContentActive, ContextPropertySpanGroupSymbol, ContextPropertySpanPedalLine, ContextPropertySpanPedalSign, ContextPropertySpanPedalAbbreviated, ContextPropertySpanPedalAbbreviatedActive, ContextPropertySpanPedalAlignmentActive, ContextPropertySpanPedalLineRoleActive, ContextPropertySpanWedgeSpreadActive, ContextPropertySpanWedgeNiente, ContextPropertySpanWedgeNienteActive, ContextPropertySpanHammerMarking, ContextPropertySpanArpeggioDirection, ContextPropertySpanPlaybackEffect, ContextPropertySpanTimeOnly, ContextPropertySpanTimeModificationActual, ContextPropertySpanTimeModificationNormal, ContextPropertySpanTimeModificationMeaning, ContextPropertySpanCurrentType, ContextPropertySpanCurrentDots, ContextPropertySpanNormalType, ContextPropertySpanNormalDots, ContextPropertySpanLyricSyllable, ContextPropertySpanTieAttackRecommended, ContextPropertySpanTieReleaseRecommended, ContextPropertySpanSameNoteOrderRecommended, ContextPropertySpanPitchEquivalentRecommended, ContextPropertySpanBarlineContinueRecommended, ContextPropertySpanTremoloMarksRecommended, ContextPropertySpanTremoloRatioRecommended:
		return true
	}
	return false
}
