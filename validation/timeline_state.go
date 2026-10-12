package validation

import (
	"context"
	"math/big"
	"reflect"
	"strings"
)

// Positions distinguish canonical part identity, occurrence identity and score
// order. A measure's printed number is never a coordinate. This layer accepts
// already established positions; it does not choose duration units or reconstruct
// positions after a rewind.
type musicalPosition struct {
	Part, Measure  NodeRef
	MeasureOrdinal uint64
	Offset         Rational
}

type stateScope struct {
	AllStaves bool
	Staff     Integer
}
type stateProvenanceKind uint8

const (
	stateExplicit stateProvenanceKind = iota + 1
	stateInitialDefault
)

type stateProvenance struct {
	Node NodeRef
	Kind stateProvenanceKind
}
type statePublication[T any] struct {
	Position   musicalPosition
	Scope      stateScope
	Value      Evidence[T]
	Provenance []stateProvenance
	bytes      int64
}
type stateGap struct {
	Part                      NodeRef
	FirstMeasure, LastMeasure uint64
	Scope                     stateScope
	Availability              Evidence[bool]
}
type stateHistory[T any] struct {
	Publications []statePublication[T]
	Gaps         []stateGap
}
type stateResult[T any] struct {
	Value      Evidence[T]
	Provenance []stateProvenance
	Inherited  bool
}
type stateClosure struct {
	Part     NodeRef
	Through  uint64
	Complete Evidence[bool]
}

// Compound encodings remain typed. Individual component availability is retained
// even when another component is missing, malformed, or unavailable.
type keyState struct {
	Traditional    bool
	Fifths         Evidence[Integer]
	Mode           Evidence[string]
	Cancel         Evidence[Integer]
	CancelLocation Evidence[string]
	Alterations    []keyAlteration
	Octaves        []keyOctave
}
type keyAlteration struct {
	Step              Evidence[string]
	Alter             Evidence[Decimal]
	Accidental, SMuFL Evidence[string]
}
type keyOctave struct {
	Number, Octave Evidence[Integer]
	Cancel         Evidence[bool]
}
type timePair struct{ Beats, BeatType Evidence[string] }
type interchangeableState struct {
	Pairs                       []timePair
	Relation, Symbol, Separator Evidence[string]
}
type timeState struct {
	Pairs                          []timePair
	Interchangeable                Evidence[interchangeableState]
	SenzaMisura, Symbol, Separator Evidence[string]
}
type clefState struct {
	Sign         Evidence[string]
	Line         Evidence[Integer]
	OctaveChange Evidence[Integer]
	Additional   Evidence[bool]
	Size         Evidence[string]
	AfterBarline Evidence[bool]
}
type doublingState struct{ Above Evidence[bool] }
type transposeState struct {
	Diatonic, OctaveChange Evidence[Integer]
	Chromatic              Evidence[Decimal]
	Double                 Evidence[doublingState]
}
type virtualInstrumentState struct{ Library, Name Evidence[string] }
type ensembleState struct {
	Empty  bool
	Number Integer
}
type instrumentState struct {
	Sound    Evidence[string]
	Solo     Evidence[bool]
	Ensemble Evidence[ensembleState]
	Virtual  Evidence[virtualInstrumentState]
}
type instrumentDefinition struct {
	Part, Instrument NodeRef
	Value            Evidence[instrumentState]
	Provenance       []stateProvenance
}
type noteInstrumentSet struct {
	Note, Part NodeRef
	References []Evidence[NodeRef]
	Complete   Evidence[bool]
}
type instrumentChanges struct {
	Instrument NodeRef
	History    stateHistory[instrumentState]
}

type timelineState struct {
	Divisions                                stateHistory[Decimal]
	Keys                                     stateHistory[keyState]
	Times                                    stateHistory[timeState]
	Staves, Instruments                      stateHistory[Integer]
	Clefs, AdditionalClefs, CreatedPartClefs stateHistory[clefState]
	Transpose, CreatedPartTranspose          stateHistory[transposeState]
	Definitions                              []instrumentDefinition
	NoteInstruments                          []noteInstrumentSet
	InstrumentChanges                        []instrumentChanges
	InstrumentGaps                           []stateGap
	MIDI                                     []instrumentRecord[midiState]
	MIDIDevices                              []instrumentRecord[midiDeviceState]
	Play                                     []instrumentRecord[playState]
	Closures                                 []stateClosure
}

func stateUnavailable[T, U any](v Evidence[T]) Evidence[U] {
	return Evidence[U]{State: v.State, Reason: v.Reason, Cause: v.Cause}
}
func stateUnknown[T any](reason Code) Evidence[T] {
	return Evidence[T]{State: EvidenceUnknown, Reason: reason}
}
func stateValidPosition(p musicalPosition) bool {
	return p.Part.Node != 0 && p.Measure.Node != 0 && p.Part.DocumentID != "" && p.Part.DocumentID == p.Measure.DocumentID && p.Offset.Valid()
}
func stateValidScope(v stateScope) bool {
	return (v.AllStaves && !v.Staff.Valid()) || (!v.AllStaves && v.Staff.Valid() && v.Staff.value.Sign() > 0)
}

// stateMeasure walks only the closed, shallow set of private typed state records.
// Numeric wrappers stop recursion: their immutable magnitude/text is accounted
// directly. This is neither a scalar parser nor a composite serialization path.
// Every visited item and string byte is charged before copying or comparison.
func stateMeasure(ctx context.Context, s *Session, v reflect.Value) (int64, error) {
	if e := s.step(ctx, 1); e != nil {
		return 0, e
	}
	if !v.IsValid() {
		return 0, nil
	}
	if v.CanInterface() {
		switch n := v.Interface().(type) {
		case Integer:
			return n.ByteSize() + 64, nil
		case Decimal:
			return n.ByteSize() + 64, nil
		case Rational:
			return n.ByteSize() + 64, nil
		}
	}
	n := int64(32)
	add := func(m int64, e error) error {
		if e != nil {
			return e
		}
		var x error
		n, x = rationalCheckedAdd(BudgetBufferedBytes, n, m)
		return x
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if e := add(stateMeasure(ctx, s, v.Field(i))); e != nil {
				return 0, e
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if e := add(stateMeasure(ctx, s, v.Index(i))); e != nil {
				return 0, e
			}
		}
	case reflect.Pointer:
		if !v.IsNil() {
			if v.Type() != reflect.TypeFor[*DecisionRef]() {
				return 0, contract("timeline state", "unexpected mutable pointer")
			}
			if e := add(stateMeasure(ctx, s, v.Elem())); e != nil {
				return 0, e
			}
		}
	case reflect.String:
		if e := s.step(ctx, int64(v.Len())); e != nil {
			return 0, e
		}
		if e := add(int64(v.Len()), nil); e != nil {
			return 0, e
		}
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
	default:
		return 0, contract("timeline state", "unsupported private payload")
	}
	return n, nil
}
func stateCopy(v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return v
	}
	if v.CanInterface() {
		switch v.Interface().(type) {
		case Integer, Decimal, Rational:
			return v
		}
	}
	switch v.Kind() {
	case reflect.Struct:
		out := reflect.New(v.Type()).Elem()
		for i := 0; i < v.NumField(); i++ {
			out.Field(i).Set(stateCopy(v.Field(i)))
		}
		return out
	case reflect.Slice:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			out.Index(i).Set(stateCopy(v.Index(i)))
		}
		return out
	case reflect.Pointer:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.New(v.Type().Elem())
		out.Elem().Set(stateCopy(v.Elem()))
		return out
	case reflect.String:
		out := reflect.New(v.Type()).Elem()
		out.SetString(strings.Clone(v.String()))
		return out
	default:
		return v
	}
}
func stateClone[T any](v T) T { return stateCopy(reflect.ValueOf(v)).Interface().(T) }

// Each copied record reserves its own growth allowance; unrelated history never
// inflates query retention. Traversal work is charged before the actual copy.
func stateRetainCopy(ctx context.Context, s *Session, v any) error {
	n, e := stateMeasure(ctx, s, reflect.ValueOf(v))
	if e != nil {
		return e
	}
	n, e = rationalCheckedMul(BudgetBufferedBytes, n, 2)
	if e != nil {
		return e
	}
	if _, e = AllocationSize(n); e != nil {
		return e
	}
	return s.retain(n)
}
func stateRetainCount(s *Session, count int, width int64) error {
	n, e := rationalCheckedMul(BudgetBufferedBytes, int64(count), width)
	if e != nil {
		return e
	}
	if _, e = AllocationSize(n); e != nil {
		return e
	}
	return s.retain(n)
}
func stateReserve(ctx context.Context, s *Session, value any) (int64, error) {
	n, e := stateMeasure(ctx, s, reflect.ValueOf(value))
	if e != nil {
		return 0, e
	}
	if _, e = AllocationSize(n); e != nil {
		return 0, e
	}
	// Append growth may coexist with the old slice. Reserve a conservative second
	// record allowance; the session owns it until cleanup, with no cumulative refund.
	n, e = rationalCheckedMul(BudgetBufferedBytes, n, 2)
	if e != nil {
		return 0, e
	}
	if e = s.account.Charge(BudgetContextFacts, 1); e != nil {
		return 0, e
	}
	if e = s.retain(n); e != nil {
		return 0, e
	}
	return n, nil
}
func publishState[T any](ctx context.Context, s *Session, h *stateHistory[T], p statePublication[T]) error {
	if !stateValidPosition(p.Position) || !stateValidScope(p.Scope) {
		return contract("timeline state", "known position and exact scope required")
	}
	// Measure the public fields only; the private accounting cache is not copied.
	v := struct {
		Position   musicalPosition
		Scope      stateScope
		Value      Evidence[T]
		Provenance []stateProvenance
	}{p.Position, p.Scope, p.Value, p.Provenance}
	n, e := stateReserve(ctx, s, v)
	if e != nil {
		return e
	}
	p.Position = stateClone(p.Position)
	p.Scope = stateClone(p.Scope)
	p.Value = stateClone(p.Value)
	p.Provenance = stateClone(p.Provenance)
	p.bytes = n
	h.Publications = append(h.Publications, p)
	return nil
}
func addStateGap[T any](ctx context.Context, s *Session, h *stateHistory[T], g stateGap) error {
	if g.Part.Node == 0 || g.FirstMeasure > g.LastMeasure || !stateValidScope(g.Scope) || g.Availability.State == EvidenceKnown || g.Availability.State == EvidenceAbsent {
		return contract("timeline gap", "bounded affected interval and causal unavailable evidence required")
	}
	if _, e := stateReserve(ctx, s, g); e != nil {
		return e
	}
	h.Gaps = append(h.Gaps, stateClone(g))
	return nil
}
func (h *timelineState) closePart(ctx context.Context, s *Session, part NodeRef, through uint64, complete Evidence[bool]) error {
	c := stateClosure{part, through, complete}
	if _, e := stateReserve(ctx, s, c); e != nil {
		return e
	}
	h.Closures = append(h.Closures, stateClone(c))
	return nil
}
func (h *timelineState) closure(ctx context.Context, s *Session, p musicalPosition) (Evidence[bool], error) {
	best := stateUnknown[bool]("timeline-scope-open")
	for _, c := range h.Closures {
		if e := s.step(ctx, 1); e != nil {
			return Evidence[bool]{}, e
		}
		if c.Part != p.Part || c.Through < p.MeasureOrdinal {
			continue
		}
		if c.Complete.State == EvidenceKnown && c.Complete.Value {
			return c.Complete, nil
		}
		best = c.Complete
	}
	if best.State == EvidenceKnown || best.State == EvidenceAbsent {
		best = stateUnknown[bool]("timeline-scope-incomplete")
	}
	return best, nil
}
func stateComparePosition(ctx context.Context, s *Session, a, b musicalPosition) (int, error) {
	if e := s.step(ctx, 1); e != nil {
		return 0, e
	}
	if a.Part != b.Part {
		return 0, contract("timeline position", "different canonical parts")
	}
	if a.MeasureOrdinal < b.MeasureOrdinal {
		return -1, nil
	}
	if a.MeasureOrdinal > b.MeasureOrdinal {
		return 1, nil
	}
	if a.Measure != b.Measure {
		return 0, contract("timeline position", "same ordinal has different measure occurrences")
	}
	return s.rationalCmp(ctx, a.Offset, b.Offset)
}
func stateApplies(ctx context.Context, s *Session, scope stateScope, staff Evidence[Integer]) (bool, error) {
	if e := s.step(ctx, 1); e != nil {
		return false, e
	}
	if scope.AllStaves {
		return true, nil
	}
	if staff.State != EvidenceKnown || !staff.Value.Valid() {
		return false, nil
	}
	if e := s.step(ctx, scope.Staff.ByteSize()+staff.Value.ByteSize()); e != nil {
		return false, e
	}
	return scope.Staff.String() == staff.Value.String(), nil
}

// queryState chooses the latest applicable musical position, then compares all
// overlapping publications there. Neither lexical order nor specificity breaks
// a tie. Projection happens before comparison, so an independent known component
// can survive a conflict elsewhere in the compound state.
func queryState[T, V any](ctx context.Context, s *Session, h *timelineState, history *stateHistory[T], pos musicalPosition, staff Evidence[Integer], project func(T) Evidence[V]) (stateResult[V], error) {
	out := stateResult[V]{}
	if !stateValidPosition(pos) {
		return out, contract("timeline query", "known musical position required")
	}
	closure, e := h.closure(ctx, s, pos)
	if e != nil {
		return out, e
	}
	if closure.State != EvidenceKnown || !closure.Value {
		out.Value = stateUnavailable[bool, V](closure)
		return out, nil
	}
	if staff.State != EvidenceKnown || !staff.Value.Valid() {
		out.Value = stateUnknown[V]("timeline-staff-unresolved")
		if staff.State != EvidenceAbsent && staff.State != EvidenceKnown {
			out.Value = stateUnavailable[Integer, V](staff)
		}
		return out, nil
	}
	best := -1
	for i, p := range history.Publications {
		if e = s.step(ctx, 1); e != nil {
			return out, e
		}
		if p.Position.Part != pos.Part {
			continue
		}
		applies, x := stateApplies(ctx, s, p.Scope, staff)
		if x != nil {
			return out, x
		}
		if !applies {
			continue
		}
		cmp, x := stateComparePosition(ctx, s, p.Position, pos)
		if x != nil {
			return out, x
		}
		if cmp > 0 {
			continue
		}
		if best < 0 {
			best = i
			continue
		}
		cmp, x = stateComparePosition(ctx, s, p.Position, history.Publications[best].Position)
		if x != nil {
			return out, x
		}
		if cmp > 0 {
			best = i
		}
	}
	for _, g := range history.Gaps {
		if e = s.step(ctx, 1); e != nil {
			return out, e
		}
		if g.Part != pos.Part || g.FirstMeasure > pos.MeasureOrdinal {
			continue
		}
		applies, x := stateApplies(ctx, s, g.Scope, staff)
		if x != nil {
			return out, x
		}
		if !applies {
			continue
		}
		if best < 0 || history.Publications[best].Position.MeasureOrdinal <= g.LastMeasure {
			out.Value = stateUnavailable[bool, V](g.Availability)
			return out, nil
		}
	}
	if best < 0 {
		out.Value = Evidence[V]{State: EvidenceAbsent}
		return out, nil
	}
	selected := history.Publications[best]
	projectValue := func(p statePublication[T]) Evidence[V] {
		if p.Value.State != EvidenceKnown {
			return stateUnavailable[T, V](p.Value)
		}
		return project(p.Value.Value)
	}
	out.Value = projectValue(selected)
	initial := out.Value
	conflict := false
	for _, p := range history.Publications {
		if e = s.step(ctx, 1); e != nil {
			return stateResult[V]{}, e
		}
		if p.Position.Part != pos.Part {
			continue
		}
		applies, x := stateApplies(ctx, s, p.Scope, staff)
		if x != nil {
			return stateResult[V]{}, x
		}
		if !applies {
			continue
		}
		cmp, x := stateComparePosition(ctx, s, p.Position, selected.Position)
		if x != nil {
			return stateResult[V]{}, x
		}
		if cmp != 0 {
			continue
		}
		compareCost, x := rationalCheckedAdd(BudgetRuleSteps, p.bytes, selected.bytes)
		if x != nil {
			return stateResult[V]{}, x
		}
		if e = s.step(ctx, compareCost); e != nil {
			return stateResult[V]{}, e
		}
		candidate := projectValue(p)
		if !reflect.DeepEqual(initial, candidate) {
			conflict = true
		}
		// Preserve an actual invalid/unsupported publication's causal evidence rather
		// than replacing it with an invented conflict cause.
		if candidate.State == EvidenceInvalid || (candidate.State == EvidenceUnsupported && out.Value.State != EvidenceInvalid) || (candidate.State == EvidenceUnknown && out.Value.State == EvidenceKnown) {
			out.Value = candidate
		}
		for _, pr := range p.Provenance {
			duplicate := false
			for _, prior := range out.Provenance {
				if e = s.step(ctx, 1); e != nil {
					return stateResult[V]{}, e
				}
				if prior == pr {
					duplicate = true
					break
				}
			}
			if !duplicate {
				if e = stateRetainCopy(ctx, s, pr); e != nil {
					return stateResult[V]{}, e
				}
				out.Provenance = append(out.Provenance, stateClone(pr))
			}
		}
	}
	if conflict && (out.Value.State == EvidenceKnown || out.Value.State == EvidenceAbsent) {
		out.Value = stateUnknown[V]("timeline-state-conflict")
	}
	if e = stateRetainCopy(ctx, s, out.Value); e != nil {
		return stateResult[V]{}, e
	}
	out.Value = stateClone(out.Value)
	cmp, e := stateComparePosition(ctx, s, selected.Position, pos)
	if e != nil {
		return stateResult[V]{}, e
	}
	out.Inherited = cmp < 0
	return out, nil
}
func stateWhole[T any](v T) Evidence[T] { return Evidence[T]{State: EvidenceKnown, Value: v} }

var timelineOne = Integer{value: big.NewInt(1), text: "1", bytes: 2}

func stateOne() Integer { return timelineOne }
func (h *timelineState) effectiveCount(ctx context.Context, s *Session, hist *stateHistory[Integer], pos musicalPosition) (stateResult[Integer], error) {
	r, e := queryState(ctx, s, h, hist, pos, Evidence[Integer]{State: EvidenceKnown, Value: stateOne()}, stateWhole[Integer])
	if e != nil {
		return r, e
	}
	if r.Value.State == EvidenceAbsent {
		if e = s.retain(256); e != nil {
			return stateResult[Integer]{}, e
		}
		r.Value = Evidence[Integer]{State: EvidenceKnown, Value: stateOne()}
		r.Provenance = []stateProvenance{{Node: pos.Part, Kind: stateInitialDefault}}
	}
	return r, nil
}
func (h *timelineState) effectiveStaves(ctx context.Context, s *Session, pos musicalPosition) (stateResult[Integer], error) {
	return h.effectiveCount(ctx, s, &h.Staves, pos)
}
func (h *timelineState) effectiveInstruments(ctx context.Context, s *Session, pos musicalPosition) (stateResult[Integer], error) {
	return h.effectiveCount(ctx, s, &h.Instruments, pos)
}
func (h *timelineState) resolveStaff(ctx context.Context, s *Session, pos musicalPosition, staff Evidence[Integer]) (Evidence[Integer], error) {
	if staff.State != EvidenceAbsent {
		return staff, nil
	}
	count, e := h.effectiveStaves(ctx, s, pos)
	if e != nil {
		return Evidence[Integer]{}, e
	}
	if count.Value.State == EvidenceKnown && count.Value.Value.String() == "1" {
		return Evidence[Integer]{State: EvidenceKnown, Value: stateOne()}, nil
	}
	if count.Value.State != EvidenceKnown {
		return stateUnavailable[Integer, Integer](count.Value), nil
	}
	return stateUnknown[Integer]("timeline-staff-unresolved"), nil
}
func (h *timelineState) effectiveKey(ctx context.Context, s *Session, pos musicalPosition, staff Evidence[Integer]) (stateResult[keyState], error) {
	staff, e := h.resolveStaff(ctx, s, pos, staff)
	if e != nil {
		return stateResult[keyState]{}, e
	}
	return queryState(ctx, s, h, &h.Keys, pos, staff, stateWhole[keyState])
}
func (h *timelineState) effectiveDivisions(ctx context.Context, s *Session, pos musicalPosition) (stateResult[Decimal], error) {
	return queryState(ctx, s, h, &h.Divisions, pos, Evidence[Integer]{State: EvidenceKnown, Value: stateOne()}, stateWhole[Decimal])
}
func (v clefState) activeLine() Evidence[Integer] {
	if v.Additional.State == EvidenceKnown && v.Additional.Value {
		return Evidence[Integer]{State: EvidenceAbsent}
	}
	if v.Additional.State != EvidenceKnown && v.Additional.State != EvidenceAbsent {
		return stateUnavailable[bool, Integer](v.Additional)
	}
	return v.Line
}

func (s *Session) stateRationalInteger(ctx context.Context, v Integer) (Rational, error) {
	if !v.Valid() {
		return Rational{}, contract("timeline integer", "uninitialized integer")
	}
	// Exact integer embedding, without parsing. Scratch is charged and released
	// separately from the immutable rational output retained by the arithmetic.
	cost, e := rationalCheckedAdd(BudgetBufferedBytes, v.ByteSize(), 128)
	if e != nil {
		return Rational{}, e
	}
	if e = s.account.Check(BudgetScalarBytes, v.ByteSize()); e != nil {
		return Rational{}, e
	}
	if e = s.step(ctx, cost); e != nil {
		return Rational{}, e
	}
	if _, e = AllocationSize(cost); e != nil {
		return Rational{}, e
	}
	if e = s.account.Charge(BudgetBufferedBytes, cost); e != nil {
		return Rational{}, e
	}
	defer func() { _ = s.account.Release(BudgetBufferedBytes, cost) }()
	return s.rationalFromDecimal(ctx, Decimal{value: new(big.Rat).SetInt(v.value), text: v.text, bytes: v.bytes + 1})
}

var timelineOctaveSemitones = Rational{value: big.NewRat(12, 1), text: "12", bytes: 4}
var timelineOctaveSteps = Rational{value: big.NewRat(7, 1), text: "7", bytes: 3}

func (v transposeState) contributions(ctx context.Context, s *Session) (Evidence[Rational], Evidence[Rational], error) {
	c, d := stateUnavailable[Decimal, Rational](v.Chromatic), stateUnavailable[Integer, Rational](v.Diatonic)
	if v.OctaveChange.State != EvidenceKnown && v.OctaveChange.State != EvidenceAbsent {
		return stateUnavailable[Integer, Rational](v.OctaveChange), stateUnavailable[Integer, Rational](v.OctaveChange), nil
	}
	oct := Rational{}
	var e error
	if v.OctaveChange.State == EvidenceKnown {
		oct, e = s.stateRationalInteger(ctx, v.OctaveChange.Value)
		if e != nil {
			return c, d, e
		}
	}
	addOctave := func(base Rational, multiple int64) (Rational, error) {
		if !oct.Valid() {
			return base, nil
		}
		constant := timelineOctaveSemitones
		if multiple == 7 {
			constant = timelineOctaveSteps
		}
		delta, e := s.rationalMul(ctx, oct, constant)
		if e != nil {
			return Rational{}, e
		}
		return s.rationalAdd(ctx, base, delta)
	}
	if v.Chromatic.State == EvidenceKnown {
		base, x := s.rationalFromDecimal(ctx, v.Chromatic.Value)
		if x != nil {
			return c, d, x
		}
		r, x := addOctave(base, 12)
		if x != nil {
			return c, d, x
		}
		c = Evidence[Rational]{State: EvidenceKnown, Value: r}
	}
	if v.Diatonic.State == EvidenceKnown {
		base, x := s.stateRationalInteger(ctx, v.Diatonic.Value)
		if x != nil {
			return c, d, x
		}
		r, x := addOctave(base, 7)
		if x != nil {
			return c, d, x
		}
		d = Evidence[Rational]{State: EvidenceKnown, Value: r}
	}
	// Doubling is independent notation and is deliberately not added to either
	// base transposition contribution. Absent diatonic remains absent spelling.
	return c, d, e
}
