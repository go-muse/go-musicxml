package validation

import "context"

type staffTuningState struct {
	Line, Octave Evidence[Integer]
	Step         Evidence[string]
	Alter        Evidence[Decimal]
}
type lineDetailState struct {
	Line            Evidence[Integer]
	Width           Evidence[Decimal]
	Color, LineType Evidence[string]
	PrintObject     Evidence[bool]
}
type staffSizeState struct{ Size, Scaling Evidence[Decimal] }
type staffDetailsState struct {
	Type                      Evidence[string]
	Lines                     Evidence[Integer]
	LineDetails               []lineDetailState
	Tunings                   []staffTuningState
	Capo                      Evidence[Integer]
	Size                      Evidence[staffSizeState]
	ShowFrets                 Evidence[string]
	PrintObject, PrintSpacing Evidence[bool]
}

// Documented semantic defaults are projections over proven local absence. Raw
// encodings remain unchanged, and neither entire-state absence nor unavailable
// scalar evidence is promoted to a guessed initial signature, clef or line count.
type stateDefault[T any] struct {
	Raw, Effective Evidence[T]
	Provenance     stateProvenanceKind
	Clause         Code
}

func stateWithDefault[T any](raw Evidence[T], fallback T, clause Code) stateDefault[T] {
	v := stateDefault[T]{Raw: raw, Effective: raw}
	if raw.State == EvidenceAbsent {
		v.Effective = Evidence[T]{State: EvidenceKnown, Value: fallback}
		v.Provenance = stateDocumentedDefault
		v.Clause = clause
	} else if raw.State == EvidenceKnown {
		v.Provenance = stateExplicit
	}
	return v
}
func (v timeState) displaySymbol() stateDefault[string] {
	out := stateWithDefault(v.Symbol, "normal", "musicxml-time-symbol-normal")
	if v.SenzaMisura.State == EvidenceKnown {
		out.Effective = Evidence[string]{State: EvidenceAbsent}
		out.Provenance = stateExplicit
		out.Clause = "musicxml-senza-misura-symbol-inactive"
	} else if v.SenzaMisura.State != EvidenceAbsent {
		out.Effective = stateUnavailable[string, string](v.SenzaMisura)
		out.Provenance = 0
		out.Clause = ""
	}
	return out
}
func (v timeState) displaySeparator() stateDefault[string] {
	return stateWithDefault(v.Separator, "none", "musicxml-time-separator-none")
}
func (v interchangeableState) displaySymbol() stateDefault[string] {
	return stateWithDefault(v.Symbol, "normal", "musicxml-time-symbol-normal")
}
func (v interchangeableState) displaySeparator() stateDefault[string] {
	return stateWithDefault(v.Separator, "none", "musicxml-time-separator-none")
}
func (v doublingState) effectiveAbove() stateDefault[bool] {
	return stateWithDefault(v.Above, false, "musicxml-double-below")
}
func (v staffDetailsState) effectiveType() stateDefault[string] {
	return stateWithDefault(v.Type, "regular", "musicxml-staff-type-regular")
}

func (r *stateReader) staffDetails(n *factNode) Evidence[staffDetailsState] {
	c := r.complete(n)
	if c.State != EvidenceKnown {
		return stateUnavailable[bool, staffDetailsState](c)
	}
	v := staffDetailsState{Type: stateText(r.childScalar(n, timelineRoleStaffDetailsStaffType)), Lines: stateInteger(r.childScalar(n, timelineRoleStaffDetailsStaffLines)), Capo: stateInteger(r.childScalar(n, timelineRoleStaffDetailsCapo)), ShowFrets: stateText(r.attribute(n, timelineRoleStaffDetailsShowFrets)), PrintObject: stateYesNo(r.attribute(n, timelineRoleStaffDetailsPrintObject)), PrintSpacing: stateYesNo(r.attribute(n, timelineRoleStaffDetailsPrintSpacing))}
	children, c := r.children(n, timelineRoleStaffDetailsLineDetail)
	if c.State != EvidenceKnown {
		return stateUnavailable[bool, staffDetailsState](c)
	}
	if e := stateRetainCount(r.s, len(children), 1024); e != nil {
		r.err = e
		return stateUnknown[staffDetailsState]("timeline-interrupted")
	}
	for _, ch := range children {
		v.LineDetails = append(v.LineDetails, lineDetailState{Line: stateInteger(r.attribute(ch, timelineRoleLineDetailLine)), Width: stateDecimalValue(r.attribute(ch, timelineRoleLineDetailWidth)), Color: stateText(r.attribute(ch, timelineRoleLineDetailColor)), LineType: stateText(r.attribute(ch, timelineRoleLineDetailLineType)), PrintObject: stateYesNo(r.attribute(ch, timelineRoleLineDetailPrintObject))})
	}
	children, c = r.children(n, timelineRoleStaffDetailsStaffTuning)
	if c.State != EvidenceKnown {
		return stateUnavailable[bool, staffDetailsState](c)
	}
	if e := stateRetainCount(r.s, len(children), 1024); e != nil {
		r.err = e
		return stateUnknown[staffDetailsState]("timeline-interrupted")
	}
	for _, ch := range children {
		v.Tunings = append(v.Tunings, staffTuningState{Line: stateInteger(r.attribute(ch, timelineRoleStaffTuningLine)), Step: stateText(r.childScalar(ch, timelineRoleStaffTuningTuningStep)), Alter: stateDecimalValue(r.childScalar(ch, timelineRoleStaffTuningTuningAlter)), Octave: stateInteger(r.childScalar(ch, timelineRoleStaffTuningTuningOctave))})
	}
	size, e := r.child(n, timelineRoleStaffDetailsStaffSize)
	if e.State != EvidenceKnown {
		v.Size = stateUnavailable[bool, staffSizeState](e)
	} else {
		v.Size = Evidence[staffSizeState]{State: EvidenceKnown, Value: staffSizeState{Size: stateDecimalValue(r.scalar(size)), Scaling: stateDecimalValue(r.attribute(size, timelineRoleStaffSizeScaling))}}
	}
	return Evidence[staffDetailsState]{State: EvidenceKnown, Value: v}
}
func (h *timelineState) effectiveStaffDetails(ctx context.Context, s *Session, pos musicalPosition, staff Evidence[Integer]) (stateResult[staffDetailsState], error) {
	staff, e := h.resolveStaff(ctx, s, pos, staff)
	if e != nil {
		return stateResult[staffDetailsState]{}, e
	}
	return queryState(ctx, s, h, &h.StaffDetails, pos, staff, stateWhole[staffDetailsState])
}
func (h *timelineState) priorKey(ctx context.Context, s *Session, pos musicalPosition, staff Evidence[Integer]) (stateResult[keyState], error) {
	staff, e := h.resolveStaff(ctx, s, pos, staff)
	if e != nil {
		return stateResult[keyState]{}, e
	}
	return queryStateAt(ctx, s, h, &h.Keys, pos, staff, stateWhole[keyState], false)
}

func (v staffDetailsState) effectiveShowFrets() stateDefault[string] {
	return stateWithDefault(v.ShowFrets, "numbers", "musicxml-show-frets-numbers")
}
func (v staffDetailsState) effectivePrintObject() stateDefault[bool] {
	return stateWithDefault(v.PrintObject, true, "musicxml-print-object-yes")
}
func (v staffDetailsState) effectivePrintSpacing() stateDefault[bool] {
	return stateWithDefault(v.PrintSpacing, true, "musicxml-print-spacing-yes")
}
func (v lineDetailState) effectivePrintObject() stateDefault[bool] {
	return stateWithDefault(v.PrintObject, true, "musicxml-print-object-yes")
}
