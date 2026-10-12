package validation

import (
	"math/big"
)

var spanSmallIntegers = func() [23]Integer {
	var out [23]Integer
	for i := range out {
		out[i], _ = NewInteger(big.NewInt(int64(i)))
	}
	return out
}()

func spanString(v string) Evidence[Atomic] {
	return Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: v}}
}
func spanBool(v bool) Evidence[Atomic] {
	s := "false"
	if v {
		s = "true"
	}
	return Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicBoolean, Text: s}}
}
func spanInteger(v int) Evidence[Atomic] {
	return Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger, Integer: spanSmallIntegers[v]}}
}
func spanDefault(v, d Evidence[Atomic]) Evidence[Atomic] {
	if v.State == EvidenceAbsent {
		return d
	}
	return v
}
func spanText(v Evidence[Atomic]) Evidence[string] {
	if v.State != EvidenceKnown {
		return stateUnavailable[Atomic, string](v)
	}
	if v.Value.Kind != AtomicString {
		return Evidence[string]{State: EvidenceUnsupported, Reason: "span-string-domain"}
	}
	return Evidence[string]{State: EvidenceKnown, Value: v.Value.Text}
}
func spanIs(v Evidence[Atomic], text string) bool {
	return v.State == EvidenceKnown && v.Value.Kind == AtomicString && v.Value.Text == text
}
func spanPublish(r *contextReader, key RuleKey, n *factNode, property ContextProperty, v Evidence[Atomic]) {
	if r.err != nil {
		return
	}
	r.err = r.s.publishContextResult(r.ctx, ContextResult{Provider: key, Subject: NodeRef{r.s.documentID, n.header.ID}, Property: property, Value: v, Origins: []Location{n.header.Location}})
}
func spanAttrDefault(r *contextReader, key RuleKey, n *factNode, role contextRole, property ContextProperty, d Evidence[Atomic]) Evidence[Atomic] {
	v := spanDefault(r.attribute(n, role), d)
	spanPublish(r, key, n, property, v)
	return v
}
func spanCountChildren(r *contextReader, n *factNode, role contextRole) Evidence[Atomic] {
	complete := r.complete(n)
	if complete.State != EvidenceKnown || !complete.Value {
		return stateUnavailable[bool, Atomic](complete)
	}
	count := uint64(0)
	for _, ch := range n.children {
		if !r.step(1) {
			return stateUnknown[Atomic]("context-interrupted")
		}
		if ch.header.Declaration.State != EvidenceKnown {
			return stateUnavailable[DeclarationID, Atomic](ch.header.Declaration)
		}
		if r.matches(ch, role) {
			if count == ^uint64(0) {
				return stateUnknown[Atomic]("span-count-overflow")
			}
			count++
		}
	}
	if err := r.s.retain(256); err != nil {
		r.err = err
		return stateUnknown[Atomic]("context-interrupted")
	}
	value, err := NewInteger(new(big.Int).SetUint64(count))
	if err != nil {
		r.err = err
		return stateUnknown[Atomic]("context-interrupted")
	}
	return Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger, Integer: value}}
}
func spanFallbackChild(r *contextReader, container *factNode, role contextRole, fallback Evidence[Atomic]) Evidence[Atomic] {
	if container == nil {
		return fallback
	}
	v := r.childScalar(container, role)
	return spanDefault(v, fallback)
}
func spanTupletFacts(r *contextReader, key RuleKey, n, note *factNode) {
	spanAttrDefault(r, key, n, spanRole_tuplet_line_shape, ContextPropertySpanLineShape, spanString("straight"))
	spanAttrDefault(r, key, n, spanRole_tuplet_show_number, ContextPropertySpanTupletShowNumber, spanString("actual"))
	spanAttrDefault(r, key, n, spanRole_tuplet_show_type, ContextPropertySpanTupletShowType, spanString("none"))
	spanAttrDefault(r, key, n, spanRole_tuplet_bracket, ContextPropertySpanTupletBracket, stateUnknown[Atomic]("span-tuplet-bracket-application-dependent"))
	mod, me := r.child(note, spanRole_noteModification)
	missing := stateUnavailable[bool, Atomic](me)
	if me.State == EvidenceAbsent {
		missing = stateUnknown[Atomic]("span-time-modification-absent")
	}
	actual, normal, normalType := missing, missing, missing
	currentType := r.childScalar(note, spanRole_noteType)
	currentDots := spanCountChildren(r, note, spanRole_noteDot)
	normalDots := missing
	if me.State == EvidenceKnown {
		actual = r.childScalar(mod, spanRole_actualNotes)
		normal = r.childScalar(mod, spanRole_normalNotes)
		normalType = spanDefault(r.childScalar(mod, spanRole_normalType), currentType)
		normalDots = spanEffectiveNormalDots(r, mod, currentDots)
	}
	for _, side := range []struct {
		role              contextRole
		number, typ, dots Evidence[Atomic]
		np, tp, dp        ContextProperty
	}{{spanRole_tupletActual, actual, currentType, currentDots, ContextPropertySpanTupletActualNumber, ContextPropertySpanTupletActualType, ContextPropertySpanTupletActualDots}, {spanRole_tupletNormal, normal, normalType, normalDots, ContextPropertySpanTupletNormalNumber, ContextPropertySpanTupletNormalType, ContextPropertySpanTupletNormalDots}} {
		portion, pe := r.child(n, side.role)
		num, typ, dots := side.number, side.typ, side.dots
		if pe.State == EvidenceKnown {
			num = spanFallbackChild(r, portion, spanRole_tupletNumber, num)
			typ = spanFallbackChild(r, portion, spanRole_tupletType, typ)
			d := spanCountChildren(r, portion, spanRole_tupletDot)
			if d.State != EvidenceKnown {
				dots = d
			} else if d.Value.Integer.value.Sign() != 0 {
				dots = d
			}
		}
		if pe.State != EvidenceKnown && pe.State != EvidenceAbsent {
			num = stateUnavailable[bool, Atomic](pe)
			typ = num
			dots = num
		}
		spanPublish(r, key, n, side.np, num)
		spanPublish(r, key, n, side.tp, typ)
		spanPublish(r, key, n, side.dp, dots)
	}
}
func spanLocalFacts(r *contextReader, key RuleKey, n, note *factNode, e *spanEndpoint) {
	p := e.payload
	role := p.Role
	switch p.Family {
	case SpanSlur:
		spanPublish(r, key, n, ContextPropertySpanPlaybackEffect, spanString("notation"))
	case SpanTied:
		spanPublish(r, key, n, ContextPropertySpanPlaybackEffect, spanString("notation"))
	case SpanTie:
		spanPublish(r, key, n, ContextPropertySpanPlaybackEffect, spanString("sound"))
		spanPublish(r, key, n, ContextPropertySpanTimeOnly, r.attribute(n, spanRole_tie_time_only))
	case SpanBeam:
		spanAttrDefault(r, key, n, spanRole_beam_fan, ContextPropertySpanFan, spanString("none"))
		spanPublish(r, key, n, ContextPropertySpanRepeater, r.attribute(n, spanRole_beam_repeater))
		applicable := stateUnavailable[string, Atomic](role)
		if role.State == EvidenceKnown {
			switch role.Value {
			case "begin":
				applicable = spanBool(true)
			case "continue":
				applicable = stateUnknown[Atomic]("span-fan-prior-direction-required")
			default:
				applicable = spanBool(false)
			}
		}
		if role.State != EvidenceKnown || role.Value != "continue" {
			spanPublish(r, key, n, ContextPropertySpanFanApplicable, applicable)
		}
		if e.categoryKnown {
			spanPublish(r, key, n, ContextPropertySpanBeamGrace, spanBool(e.grace))
			spanPublish(r, key, n, ContextPropertySpanBeamCue, spanBool(e.cue))
		}
	case SpanTuplet:
		spanTupletFacts(r, key, n, note)
	case SpanGlissando:
		spanAttrDefault(r, key, n, spanRole_glissando_line_type, ContextPropertySpanLineType, spanString("wavy"))
		spanPublish(r, key, n, ContextPropertySpanPlaybackEffect, spanString("distinct-intermediate-pitches"))
	case SpanSlide:
		spanAttrDefault(r, key, n, spanRole_slide_line_type, ContextPropertySpanLineType, spanString("solid"))
		spanPublish(r, key, n, ContextPropertySpanPlaybackEffect, spanString("continuous-pitch"))
	case SpanOctaveShift:
		meaning := stateUnavailable[string, Atomic](role)
		if role.State == EvidenceKnown {
			meaning = stateUnknown[Atomic]("span-octave-direction-start-required")
			if role.Value == "up" || role.Value == "down" {
				meaning = spanString("display-" + role.Value + "-from-note-pitch")
			}
		}
		spanPublish(r, key, n, ContextPropertySpanEndpointMeaning, meaning)
		size := spanAttrDefault(r, key, n, spanRole_octave_shift_size, ContextPropertySpanSize, spanInteger(8))
		oct := stateUnavailable[Atomic, Atomic](size)
		if size.State == EvidenceKnown {
			oct = stateUnknown[Atomic]("span-octave-size-interpretation")
			if size.Value.Kind == AtomicInteger {
				switch size.Value.Integer.String() {
				case "8":
					oct = spanInteger(1)
				case "15":
					oct = spanInteger(2)
				case "22":
					oct = spanInteger(3)
				}
			}
		}
		spanPublish(r, key, n, ContextPropertySpanDisplayOctaves, oct)
		spanPublish(r, key, n, ContextPropertySpanPlaybackEffect, spanString("display-relative-to-performed-pitch"))
	case SpanBarre:
		v := stateUnavailable[string, Atomic](role)
		if role.State == EvidenceKnown {
			if role.Value == "start" {
				v = spanString("lower-pitched-higher-numbered-string")
			} else {
				v = spanString("higher-pitched-lower-numbered-string")
			}
		}
		spanPublish(r, key, n, ContextPropertySpanEndpointMeaning, v)
	case SpanWavyLine:
		if r.matches(n, spanRole_barlineWavy) {
			v := stateUnavailable[string, Atomic](role)
			if role.State == EvidenceKnown {
				v = spanBool(role.Value == "continue")
			}
			spanPublish(r, key, n, ContextPropertySpanBarlineContinueRecommended, v)
		}
	case SpanTremolo:
		glyph := stateUnavailable[string, Atomic](role)
		active := stateUnavailable[string, Atomic](role)
		if role.State == EvidenceKnown {
			active = spanBool(role.Value == "unmeasured")
			glyph = Evidence[Atomic]{State: EvidenceAbsent}
			if role.Value == "unmeasured" {
				glyph = spanDefault(r.attribute(n, spanRole_tremoloSMuFL), spanString("buzzRoll"))
			}
		}
		spanPublish(r, key, n, ContextPropertySpanTremoloSMuFL, glyph)
		spanPublish(r, key, n, ContextPropertySpanTremoloSMuFLActive, active)
		if role.State == EvidenceKnown && role.Value == "unmeasured" {
			v := r.scalar(n)
			if v.State == EvidenceKnown && v.Value.Kind == AtomicInteger {
				v = spanBool(v.Value.Integer.value.Sign() == 0)
			}
			spanPublish(r, key, n, ContextPropertySpanTremoloMarksRecommended, v)
		}
	case SpanBracket:
		lineEnd := r.attribute(n, spanRole_bracketLineEnd)
		active := stateUnavailable[Atomic, Atomic](lineEnd)
		if lineEnd.State == EvidenceKnown {
			active = spanBool(spanIs(lineEnd, "up") || spanIs(lineEnd, "down"))
		}
		spanPublish(r, key, n, ContextPropertySpanEndLengthApplicable, active)
		spanPublish(r, key, n, ContextPropertySpanEndLength, r.attribute(n, spanRole_bracketEndLength))
		spanAttrDefault(r, key, n, spanRole_bracket_line_type, ContextPropertySpanLineType, spanString("solid"))
	case SpanHammerOn, SpanPullOff:
		v := r.scalar(n)
		if spanIs(v, "") {
			v = stateUnknown[Atomic]("span-hammer-marking-application-dependent")
		}
		spanPublish(r, key, n, ContextPropertySpanHammerMarking, v)
	case SpanArpeggiate:
		spanAttrDefault(r, key, n, spanRole_arpeggiate_arpeggio_direction, ContextPropertySpanArpeggioDirection, spanString("up"))
	case SpanNonArpeggiate:
		spanPublish(r, key, n, ContextPropertySpanPlaybackEffect, spanBool(false))
		v := stateUnavailable[string, Atomic](role)
		if role.State == EvidenceKnown {
			v = spanString(role.Value + "-note-of-graphical-bracket")
		}
		spanPublish(r, key, n, ContextPropertySpanEndpointMeaning, v)
	case SpanPartGroup:
		active := stateUnavailable[string, Atomic](role)
		if role.State == EvidenceKnown {
			active = spanBool(role.Value != "stop")
		}
		spanPublish(r, key, n, ContextPropertySpanGroupStopContentActive, active)
		spanPublish(r, key, n, ContextPropertySpanGroupSymbol, spanDefault(r.childScalar(n, spanRole_groupSymbol), spanString("none")))
	case SpanGrouping:
		active := stateUnavailable[string, Atomic](role)
		if role.State == EvidenceKnown {
			if role.Value == "stop" {
				active = stateUnknown[Atomic]("span-grouping-stop-content-may-be-ignored")
			} else {
				active = spanBool(true)
			}
		}
		spanPublish(r, key, n, ContextPropertySpanGroupStopContentActive, active)
	case SpanPedal:
		line := spanAttrDefault(r, key, n, spanRole_pedal_line, ContextPropertySpanPedalLine, spanString("no"))
		sd := stateUnavailable[Atomic, Atomic](line)
		if spanIs(line, "yes") {
			sd = spanString("no")
		} else if spanIs(line, "no") {
			sd = spanString("yes")
		}
		sign := spanAttrDefault(r, key, n, spanRole_pedal_sign, ContextPropertySpanPedalSign, sd)
		spanAttrDefault(r, key, n, spanRole_pedal_abbreviated, ContextPropertySpanPedalAbbreviated, spanString("no"))
		active := stateUnavailable[Atomic, Atomic](sign)
		if sign.State == EvidenceKnown {
			active = spanBool(spanIs(sign, "yes"))
		}
		spanPublish(r, key, n, ContextPropertySpanPedalAlignmentActive, active)
		if sign.State == EvidenceKnown && role.State == EvidenceKnown {
			active = spanBool(spanIs(sign, "yes") && (role.Value == "start" || role.Value == "sostenuto"))
		} else if role.State != EvidenceKnown {
			active = stateUnavailable[string, Atomic](role)
		}
		spanPublish(r, key, n, ContextPropertySpanPedalAbbreviatedActive, active)
		if role.State == EvidenceKnown {
			switch role.Value {
			case "change", "continue", "discontinue", "resume":
				if line.State == EvidenceKnown {
					active = spanBool(spanIs(line, "yes"))
				} else {
					active = stateUnavailable[Atomic, Atomic](line)
				}
			default:
				active = spanBool(false)
			}
		} else {
			active = stateUnavailable[string, Atomic](role)
		}
		spanPublish(r, key, n, ContextPropertySpanPedalLineRoleActive, active)
	case SpanWedge:
		spanAttrDefault(r, key, n, spanRole_wedge_line_type, ContextPropertySpanLineType, spanString("solid"))
		spanAttrDefault(r, key, n, spanRole_wedge_niente, ContextPropertySpanWedgeNiente, spanString("no"))
		active := stateUnavailable[string, Atomic](role)
		if role.State == EvidenceKnown {
			switch role.Value {
			case "crescendo":
				active = spanBool(false)
			case "diminuendo":
				active = spanBool(true)
			case "stop":
				active = stateUnknown[Atomic]("span-wedge-start-required")
			default:
				active = spanBool(false)
			}
		}
		if role.State != EvidenceKnown || role.Value != "stop" {
			spanPublish(r, key, n, ContextPropertySpanWedgeSpreadActive, active)
		}
		if role.State == EvidenceKnown {
			switch role.Value {
			case "crescendo":
				active = spanBool(true)
			case "stop":
				active = stateUnknown[Atomic]("span-wedge-start-required")
			default:
				active = spanBool(false)
			}
		}
		if role.State != EvidenceKnown || role.Value != "stop" {
			spanPublish(r, key, n, ContextPropertySpanWedgeNienteActive, active)
		}
	}
}

// spanEffectiveNormalDots does not invent a dotted-note inheritance rule. The
// normal-type documentation describes a group with normal-dot but specifies
// only the type fallback. Explicit normal-type disambiguates an undotted normal
// side (see the official tuplet-dot example); an omitted group on a dotted note
// leaves its effective normal-dot interpretation unresolved.
func spanEffectiveNormalDots(r *contextReader, mod *factNode, current Evidence[Atomic]) Evidence[Atomic] {
	typ := r.childScalar(mod, spanRole_normalType)
	dots := spanCountChildren(r, mod, spanRole_normalDot)
	if typ.State == EvidenceKnown {
		return dots
	}
	if typ.State != EvidenceAbsent {
		return stateUnavailable[Atomic, Atomic](typ)
	}
	if dots.State != EvidenceKnown {
		return dots
	}
	if current.State != EvidenceKnown {
		return stateUnavailable[Atomic, Atomic](current)
	}
	if current.Value.Kind == AtomicInteger && current.Value.Integer.value.Sign() == 0 {
		return dots
	}
	return stateUnknown[Atomic]("span-normal-dots-omitted-group-interpretation")
}
