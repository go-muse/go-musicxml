package validation

import "context"

// stateReader consumes shared assessed scalar and declaration evidence. The
// bindings are copied during plan compilation; runtime roles never use names.
type stateReader struct {
	ctx context.Context
	s   *Session
	b   *timelineBindings
	err error
}

func (r *stateReader) step(n int64) bool {
	if r.err == nil {
		r.err = r.s.step(r.ctx, n)
	}
	return r.err == nil
}
func (r *stateReader) matches(n *factNode, role timelineRole) bool {
	if n == nil || !r.step(int64(len(r.b.Roles[role]))+1) || n.header.Declaration.State != EvidenceKnown {
		return false
	}
	return r.b.matches(role, n.header.Declaration.Value)
}
func (r *stateReader) complete(n *factNode) Evidence[bool] {
	if n == nil || !n.closed {
		return stateUnknown[bool]("timeline-node-open")
	}
	if !r.step(1) {
		return stateUnknown[bool]("timeline-interrupted")
	}
	e := n.end.ChildrenComplete
	if e.State != EvidenceKnown || !e.Value {
		if e.State == EvidenceKnown || e.State == EvidenceAbsent {
			return stateUnknown[bool]("timeline-children-incomplete")
		}
		return e
	}
	if n.end.Order.State != EvidenceKnown {
		return stateUnavailable[OrderKind, bool](n.end.Order)
	}
	if n.end.Order.Value != OrderSource && n.end.Order.Value != OrderModelContent {
		return stateUnknown[bool]("timeline-content-order-unavailable")
	}
	if r.s.schema != nil {
		cause, err := r.s.schemaResultCause(r.ctx, n, XSDCheckContent)
		if err != nil {
			r.err = err
			return stateUnknown[bool]("timeline-interrupted")
		}
		if cause != nil {
			return Evidence[bool]{State: EvidenceInvalid, Reason: "timeline-content-invalid", Cause: cause}
		}
		result := r.s.schemaResult(n, XSDCheckContent)
		if !result.Valid {
			v, err := r.s.schemaInvalidValue(r.ctx, n, XSDCheckContent)
			if err != nil {
				r.err = err
			}
			return stateUnavailable[Atomic, bool](v)
		}
		if !result.Complete {
			return stateUnknown[bool]("timeline-content-incomplete")
		}
	}
	return Evidence[bool]{State: EvidenceKnown, Value: true}
}
func (r *stateReader) children(n *factNode, role timelineRole) ([]*factNode, Evidence[bool]) {
	complete := r.complete(n)
	if complete.State != EvidenceKnown || !complete.Value {
		return nil, complete
	}
	if !r.step(int64(len(n.children))) {
		return nil, stateUnknown[bool]("timeline-interrupted")
	}
	cost, e := rationalCheckedMul(BudgetBufferedBytes, int64(len(n.children)), 16)
	if e != nil {
		r.err = e
		return nil, stateUnknown[bool]("timeline-interrupted")
	}
	if e = r.s.retain(cost); e != nil {
		r.err = e
		return nil, stateUnknown[bool]("timeline-interrupted")
	}
	out := make([]*factNode, 0, len(n.children))
	for _, ch := range n.children {
		if ch.header.Declaration.State != EvidenceKnown {
			return nil, stateUnavailable[DeclarationID, bool](ch.header.Declaration)
		}
		if r.matches(ch, role) {
			out = append(out, ch)
		}
	}
	return out, complete
}
func (r *stateReader) child(n *factNode, role timelineRole) (*factNode, Evidence[bool]) {
	found, e := r.children(n, role)
	if e.State != EvidenceKnown {
		return nil, e
	}
	if len(found) == 0 {
		return nil, Evidence[bool]{State: EvidenceAbsent}
	}
	if len(found) != 1 {
		return nil, stateUnknown[bool]("timeline-multiple-children")
	}
	return found[0], e
}
func (r *stateReader) scalar(n *factNode) Evidence[Atomic] {
	if n == nil {
		return Evidence[Atomic]{State: EvidenceAbsent}
	}
	if r.err != nil {
		return stateUnknown[Atomic]("timeline-interrupted")
	}
	v, e := r.s.localScalar(r.ctx, n, nil)
	if e != nil {
		r.err = e
	}
	return v
}
func (r *stateReader) childScalar(n *factNode, role timelineRole) Evidence[Atomic] {
	ch, e := r.child(n, role)
	if e.State != EvidenceKnown {
		return stateUnavailable[bool, Atomic](e)
	}
	return r.scalar(ch)
}
func (r *stateReader) attribute(n *factNode, role timelineRole) Evidence[Atomic] {
	a, ok := r.b.attribute(role)
	if !ok {
		return Evidence[Atomic]{State: EvidenceUnsupported, Reason: "timeline-attribute-binding-unavailable"}
	}
	if r.err != nil {
		return stateUnknown[Atomic]("timeline-interrupted")
	}
	v, e := r.s.localScalar(r.ctx, n, &a)
	if e != nil {
		r.err = e
	}
	return v
}
func stateInteger(v Evidence[Atomic]) Evidence[Integer] {
	if v.State != EvidenceKnown {
		return stateUnavailable[Atomic, Integer](v)
	}
	if v.Value.Kind != AtomicInteger || !v.Value.Integer.Valid() {
		return Evidence[Integer]{State: EvidenceUnsupported, Reason: "timeline-integer-family-unavailable"}
	}
	return Evidence[Integer]{State: EvidenceKnown, Value: v.Value.Integer}
}
func stateDecimalValue(v Evidence[Atomic]) Evidence[Decimal] {
	if v.State != EvidenceKnown {
		return stateUnavailable[Atomic, Decimal](v)
	}
	if v.Value.Kind != AtomicDecimal || !v.Value.Decimal.Valid() {
		return Evidence[Decimal]{State: EvidenceUnsupported, Reason: "timeline-decimal-family-unavailable"}
	}
	return Evidence[Decimal]{State: EvidenceKnown, Value: v.Value.Decimal}
}
func stateText(v Evidence[Atomic]) Evidence[string] {
	if v.State != EvidenceKnown {
		return stateUnavailable[Atomic, string](v)
	}
	if v.Value.Kind != AtomicString {
		return Evidence[string]{State: EvidenceUnsupported, Reason: "timeline-string-family-unavailable"}
	}
	return Evidence[string]{State: EvidenceKnown, Value: v.Value.Text}
}
func stateYesNo(v Evidence[Atomic]) Evidence[bool] {
	if v.State != EvidenceKnown {
		return stateUnavailable[Atomic, bool](v)
	}
	if v.Value.Kind != AtomicString || (v.Value.Text != "yes" && v.Value.Text != "no") {
		return Evidence[bool]{State: EvidenceUnsupported, Reason: "timeline-yes-no-family-unavailable"}
	}
	return Evidence[bool]{State: EvidenceKnown, Value: v.Value.Text == "yes"}
}
func (r *stateReader) scope(n *factNode, role timelineRole, clef bool) Evidence[stateScope] {
	number := stateInteger(r.attribute(n, role))
	if number.State == EvidenceAbsent {
		scope := stateScope{AllStaves: true}
		if clef {
			scope = stateScope{Staff: stateOne()}
		}
		return Evidence[stateScope]{State: EvidenceKnown, Value: scope}
	}
	if number.State != EvidenceKnown {
		return stateUnavailable[Integer, stateScope](number)
	}
	return Evidence[stateScope]{State: EvidenceKnown, Value: stateScope{Staff: number.Value}}
}
func stateEmit[T any](r *stateReader, h *stateHistory[T], n *factNode, p musicalPosition, scope Evidence[stateScope], v Evidence[T]) {
	if r.err != nil {
		return
	}
	if scope.State != EvidenceKnown {
		v = stateUnavailable[stateScope, T](scope)
		scope = Evidence[stateScope]{State: EvidenceKnown, Value: stateScope{AllStaves: true}}
	}
	r.err = publishState(r.ctx, r.s, h, statePublication[T]{Position: p, Scope: scope.Value, Value: v, Provenance: []stateProvenance{{Node: NodeRef{DocumentID: r.s.documentID, Node: n.header.ID}, Kind: stateExplicit}}})
}
func (r *stateReader) key(n *factNode) Evidence[keyState] {
	complete := r.complete(n)
	if complete.State != EvidenceKnown {
		return stateUnavailable[bool, keyState](complete)
	}
	fifths := stateInteger(r.childScalar(n, timelineRoleKeyFifths))
	value := keyState{Traditional: fifths.State != EvidenceAbsent, Fifths: fifths, Mode: stateText(r.childScalar(n, timelineRoleKeyMode)), Cancel: stateInteger(r.childScalar(n, timelineRoleKeyCancel)), CancelLocation: Evidence[string]{State: EvidenceAbsent}}
	if cancel, e := r.child(n, timelineRoleKeyCancel); e.State == EvidenceKnown {
		value.CancelLocation = stateText(r.attribute(cancel, timelineRoleCancelLocation))
	}
	if !r.step(int64(len(n.children))) {
		return stateUnknown[keyState]("timeline-interrupted")
	}
	if _, e := stateReserve(r.ctx, r.s, struct{ Nodes int }{len(n.children)}); e != nil {
		r.err = e
		return stateUnknown[keyState]("timeline-interrupted")
	}
	// Precharge variable compound records independently of their source nodes.
	if e := r.s.retain(int64(len(n.children)) * 1024); e != nil {
		r.err = e
		return stateUnknown[keyState]("timeline-interrupted")
	}
	for _, ch := range n.children {
		if ch.header.Declaration.State != EvidenceKnown {
			return stateUnavailable[DeclarationID, keyState](ch.header.Declaration)
		}
		switch {
		case r.matches(ch, timelineRoleKeyStep):
			value.Alterations = append(value.Alterations, keyAlteration{Step: stateText(r.scalar(ch)), Alter: stateUnknown[Decimal]("timeline-key-alter-missing"), Accidental: Evidence[string]{State: EvidenceAbsent}, SMuFL: Evidence[string]{State: EvidenceAbsent}})
		case r.matches(ch, timelineRoleKeyAlter):
			if len(value.Alterations) == 0 {
				return stateUnknown[keyState]("timeline-key-pair-unavailable")
			}
			value.Alterations[len(value.Alterations)-1].Alter = stateDecimalValue(r.scalar(ch))
		case r.matches(ch, timelineRoleKeyAccidental):
			if len(value.Alterations) == 0 {
				return stateUnknown[keyState]("timeline-key-pair-unavailable")
			}
			value.Alterations[len(value.Alterations)-1].Accidental = stateText(r.scalar(ch))
			value.Alterations[len(value.Alterations)-1].SMuFL = stateText(r.attribute(ch, timelineRoleKeyAccidentalSMuFL))
		case r.matches(ch, timelineRoleKeyOctave):
			value.Octaves = append(value.Octaves, keyOctave{Number: stateInteger(r.attribute(ch, timelineRoleKeyOctaveNumber)), Octave: stateInteger(r.scalar(ch)), Cancel: stateYesNo(r.attribute(ch, timelineRoleKeyOctaveCancel))})
		}
	}
	return Evidence[keyState]{State: EvidenceKnown, Value: value}
}
func (r *stateReader) timePairs(n *factNode, beats, beatType timelineRole) []timePair {
	if !r.step(int64(len(n.children))) {
		return nil
	}
	if e := r.s.retain(int64(len(n.children)) * 512); e != nil {
		r.err = e
		return nil
	}
	var out []timePair
	for _, ch := range n.children {
		if r.matches(ch, beats) {
			out = append(out, timePair{Beats: stateText(r.scalar(ch)), BeatType: stateUnknown[string]("timeline-beat-type-missing")})
		} else if r.matches(ch, beatType) {
			if len(out) == 0 {
				r.err = contract("timeline time", "validated time pair missing numerator")
				return nil
			}
			out[len(out)-1].BeatType = stateText(r.scalar(ch))
		}
	}
	return out
}
func (r *stateReader) time(n *factNode) Evidence[timeState] {
	complete := r.complete(n)
	if complete.State != EvidenceKnown {
		return stateUnavailable[bool, timeState](complete)
	}
	v := timeState{SenzaMisura: stateText(r.childScalar(n, timelineRoleTimeSenzaMisura)), Symbol: stateText(r.attribute(n, timelineRoleTimeSymbol)), Separator: stateText(r.attribute(n, timelineRoleTimeSeparator)), Interchangeable: Evidence[interchangeableState]{State: EvidenceAbsent}}
	v.Pairs = r.timePairs(n, timelineRoleTimeBeats, timelineRoleTimeBeatType)
	ch, e := r.child(n, timelineRoleTimeInterchangeable)
	if e.State == EvidenceKnown {
		complete := r.complete(ch)
		if complete.State != EvidenceKnown {
			v.Interchangeable = stateUnavailable[bool, interchangeableState](complete)
		} else {
			v.Interchangeable = Evidence[interchangeableState]{State: EvidenceKnown, Value: interchangeableState{Pairs: r.timePairs(ch, timelineRoleInterchangeableBeats, timelineRoleInterchangeableBeatType), Relation: stateText(r.childScalar(ch, timelineRoleInterchangeableRelation)), Symbol: stateText(r.attribute(ch, timelineRoleInterchangeableSymbol)), Separator: stateText(r.attribute(ch, timelineRoleInterchangeableSeparator))}}
		}
	} else {
		v.Interchangeable = stateUnavailable[bool, interchangeableState](e)
	}
	return Evidence[timeState]{State: EvidenceKnown, Value: v}
}
func (r *stateReader) clef(n *factNode, created bool) Evidence[clefState] {
	complete := r.complete(n)
	if complete.State != EvidenceKnown {
		return stateUnavailable[bool, clefState](complete)
	}
	sign, line, oct := timelineRoleClefSign, timelineRoleClefLine, timelineRoleClefClefOctaveChange
	v := clefState{Additional: Evidence[bool]{State: EvidenceAbsent}, Size: Evidence[string]{State: EvidenceAbsent}, AfterBarline: Evidence[bool]{State: EvidenceAbsent}}
	if created {
		sign, line, oct = timelineRolePartClefSign, timelineRolePartClefLine, timelineRolePartClefClefOctaveChange
	} else {
		v.Additional = stateYesNo(r.attribute(n, timelineRoleClefAdditional))
		v.Size = stateText(r.attribute(n, timelineRoleClefSize))
		v.AfterBarline = stateYesNo(r.attribute(n, timelineRoleClefAfterBarline))
	}
	v.Sign = stateText(r.childScalar(n, sign))
	v.Line = stateInteger(r.childScalar(n, line))
	v.OctaveChange = stateInteger(r.childScalar(n, oct))
	return Evidence[clefState]{State: EvidenceKnown, Value: v}
}
func (r *stateReader) transpose(n *factNode, created bool) Evidence[transposeState] {
	complete := r.complete(n)
	if complete.State != EvidenceKnown {
		return stateUnavailable[bool, transposeState](complete)
	}
	diat, chrom, oct, double := timelineRoleTransposeDiatonic, timelineRoleTransposeChromatic, timelineRoleTransposeOctaveChange, timelineRoleTransposeDouble
	if created {
		diat, chrom, oct, double = timelineRolePartTransposeDiatonic, timelineRolePartTransposeChromatic, timelineRolePartTransposeOctaveChange, timelineRolePartTransposeDouble
	}
	v := transposeState{Diatonic: stateInteger(r.childScalar(n, diat)), Chromatic: stateDecimalValue(r.childScalar(n, chrom)), OctaveChange: stateInteger(r.childScalar(n, oct))}
	ch, e := r.child(n, double)
	if e.State == EvidenceKnown {
		v.Double = Evidence[doublingState]{State: EvidenceKnown, Value: doublingState{Above: stateYesNo(r.attribute(ch, timelineRoleDoubleAbove))}}
	} else {
		v.Double = stateUnavailable[bool, doublingState](e)
	}
	return Evidence[transposeState]{State: EvidenceKnown, Value: v}
}
func (h *timelineState) attributeGap(r *stateReader, n *factNode, p musicalPosition, gap Evidence[bool]) {
	all := Evidence[stateScope]{State: EvidenceKnown, Value: stateScope{AllStaves: true}}
	stateEmit(r, &h.Divisions, n, p, all, stateUnavailable[bool, Decimal](gap))
	stateEmit(r, &h.Keys, n, p, all, stateUnavailable[bool, keyState](gap))
	stateEmit(r, &h.Times, n, p, all, stateUnavailable[bool, timeState](gap))
	stateEmit(r, &h.Staves, n, p, all, stateUnavailable[bool, Integer](gap))
	stateEmit(r, &h.Instruments, n, p, all, stateUnavailable[bool, Integer](gap))
	stateEmit(r, &h.Clefs, n, p, all, stateUnavailable[bool, clefState](gap))
	stateEmit(r, &h.AdditionalClefs, n, p, all, stateUnavailable[bool, clefState](gap))
	stateEmit(r, &h.Transpose, n, p, all, stateUnavailable[bool, transposeState](gap))
	stateEmit(r, &h.CreatedPartClefs, n, p, all, stateUnavailable[bool, clefState](gap))
	stateEmit(r, &h.CreatedPartTranspose, n, p, all, stateUnavailable[bool, transposeState](gap))
}
func (h *timelineState) readAttributes(ctx context.Context, s *Session, b *timelineBindings, n *factNode, pos musicalPosition) error {
	r := &stateReader{ctx: ctx, s: s, b: b}
	if !r.matches(n, timelineRoleAttributes) {
		if r.err != nil {
			return r.err
		}
		return contract("timeline attributes", "exact attributes declaration required")
	}
	complete := r.complete(n)
	if complete.State != EvidenceKnown {
		h.attributeGap(r, n, pos, complete)
		return r.err
	}
	all := Evidence[stateScope]{State: EvidenceKnown, Value: stateScope{AllStaves: true}}
	for _, ch := range n.children {
		if ch.header.Declaration.State != EvidenceKnown {
			h.attributeGap(r, n, pos, stateUnavailable[DeclarationID, bool](ch.header.Declaration))
			continue
		}
		switch {
		case r.matches(ch, timelineRoleAttributesDivisions):
			stateEmit(r, &h.Divisions, ch, pos, all, stateDecimalValue(r.scalar(ch)))
		case r.matches(ch, timelineRoleAttributesStaves):
			stateEmit(r, &h.Staves, ch, pos, all, stateInteger(r.scalar(ch)))
		case r.matches(ch, timelineRoleAttributesInstruments):
			stateEmit(r, &h.Instruments, ch, pos, all, stateInteger(r.scalar(ch)))
		case r.matches(ch, timelineRoleAttributesKey):
			stateEmit(r, &h.Keys, ch, pos, r.scope(ch, timelineRoleKeyNumber, false), r.key(ch))
		case r.matches(ch, timelineRoleAttributesTime):
			stateEmit(r, &h.Times, ch, pos, r.scope(ch, timelineRoleTimeNumber, false), r.time(ch))
		case r.matches(ch, timelineRoleAttributesTranspose):
			stateEmit(r, &h.Transpose, ch, pos, r.scope(ch, timelineRoleTransposeNumber, false), r.transpose(ch, false))
		case r.matches(ch, timelineRoleAttributesClef):
			v := r.clef(ch, false)
			scope := r.scope(ch, timelineRoleClefNumber, true)
			if v.State != EvidenceKnown {
				stateEmit(r, &h.Clefs, ch, pos, scope, v)
				stateEmit(r, &h.AdditionalClefs, ch, pos, scope, v)
			} else if v.Value.Additional.State == EvidenceKnown && v.Value.Additional.Value {
				stateEmit(r, &h.AdditionalClefs, ch, pos, scope, v)
			} else if v.Value.Additional.State == EvidenceAbsent || (v.Value.Additional.State == EvidenceKnown && !v.Value.Additional.Value) {
				stateEmit(r, &h.Clefs, ch, pos, scope, v)
			} else {
				gap := stateUnavailable[bool, clefState](v.Value.Additional)
				stateEmit(r, &h.Clefs, ch, pos, scope, gap)
				stateEmit(r, &h.AdditionalClefs, ch, pos, scope, gap)
			}
		case r.matches(ch, timelineRoleAttributesForPart):
			scope := r.scope(ch, timelineRoleForPartNumber, false)
			clef, ce := r.child(ch, timelineRoleForPartPartClef)
			if ce.State == EvidenceKnown {
				stateEmit(r, &h.CreatedPartClefs, clef, pos, scope, r.clef(clef, true))
			} else if ce.State != EvidenceAbsent {
				stateEmit(r, &h.CreatedPartClefs, ch, pos, scope, stateUnavailable[bool, clefState](ce))
			}
			trans, te := r.child(ch, timelineRoleForPartPartTranspose)
			if te.State == EvidenceKnown {
				stateEmit(r, &h.CreatedPartTranspose, trans, pos, scope, r.transpose(trans, true))
			} else {
				stateEmit(r, &h.CreatedPartTranspose, ch, pos, scope, stateUnavailable[bool, transposeState](te))
			}
		}
		if r.err != nil {
			return r.err
		}
	}
	return r.err
}
