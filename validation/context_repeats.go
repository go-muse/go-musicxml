package validation

import (
	"context"
	"math/big"
	"sort"
)

const (
	contextRepeatStyle      contextRole = "repeat-style"
	contextRepeatStaff      contextRole = "repeat-staff"
	contextBeatRepeat       contextRole = "beat-repeat"
	contextMeasureRepeat    contextRole = "measure-repeat"
	contextSlashRepeat      contextRole = "slash-repeat"
	contextBeatType         contextRole = "beat-repeat-type"
	contextMeasureType      contextRole = "measure-repeat-type"
	contextSlashType        contextRole = "slash-repeat-type"
	contextBeatSlashes      contextRole = "beat-repeat-slashes"
	contextMeasureSlashes   contextRole = "measure-repeat-slashes"
	contextBeatDots         contextRole = "beat-repeat-dots"
	contextSlashDots        contextRole = "slash-repeat-dots"
	contextSlashStems       contextRole = "slash-repeat-stems"
	contextBeatDisplayType  contextRole = "beat-display-type"
	contextSlashDisplayType contextRole = "slash-display-type"
	contextBeatDisplayDot   contextRole = "beat-display-dot"
	contextSlashDisplayDot  contextRole = "slash-display-dot"
	contextBeatExcept       contextRole = "beat-except-voice"
	contextSlashExcept      contextRole = "slash-except-voice"
	contextBarlineRepeat    contextRole = "barline-repeat"
	contextBarlineEnding    contextRole = "barline-ending"
	contextRepeatDirection  contextRole = "repeat-direction"
	contextRepeatTimes      contextRole = "repeat-times"
	contextRepeatAfterJump  contextRole = "repeat-after-jump"
	contextEndingType       contextRole = "ending-type"
	contextEndingNumber     contextRole = "ending-number"
)

func contextRepeatBindingSpecs() []contextBindingSpec {
	return append([]contextBindingSpec{
		{Role: contextRepeatStyle, Owner: "attributes", Name: "measure-style", Type: "measure-style", Count: 1},
		{Role: contextRepeatStaff, Owner: "measure-style", Name: "number", Type: "staff-number", Attribute: true, Count: 1},
		{Role: contextBeatRepeat, Owner: "measure-style", Name: "beat-repeat", Type: "beat-repeat", Count: 1},
		{Role: contextMeasureRepeat, Owner: "measure-style", Name: "measure-repeat", Type: "measure-repeat", Count: 1},
		{Role: contextSlashRepeat, Owner: "measure-style", Name: "slash", Type: "slash", Count: 1},
		{Role: contextBeatType, Owner: "beat-repeat", Name: "type", Type: "start-stop", Attribute: true, Count: 1},
		{Role: contextMeasureType, Owner: "measure-repeat", Name: "type", Type: "start-stop", Attribute: true, Count: 1},
		{Role: contextSlashType, Owner: "slash", Name: "type", Type: "start-stop", Attribute: true, Count: 1},
		{Role: contextBeatSlashes, Owner: "beat-repeat", Name: "slashes", Type: "xs:positiveInteger", Attribute: true, Count: 1},
		{Role: contextMeasureSlashes, Owner: "measure-repeat", Name: "slashes", Type: "xs:positiveInteger", Attribute: true, Count: 1},
		{Role: contextBeatDots, Owner: "beat-repeat", Name: "use-dots", Type: "yes-no", Attribute: true, Count: 1},
		{Role: contextSlashDots, Owner: "slash", Name: "use-dots", Type: "yes-no", Attribute: true, Count: 1},
		{Role: contextSlashStems, Owner: "slash", Name: "use-stems", Type: "yes-no", Attribute: true, Count: 1},
		{Role: contextBeatDisplayType, Owner: "beat-repeat", Name: "slash-type", Type: "note-type-value", Count: 1},
		{Role: contextSlashDisplayType, Owner: "slash", Name: "slash-type", Type: "note-type-value", Count: 1},
		{Role: contextBeatDisplayDot, Owner: "beat-repeat", Name: "slash-dot", Type: "empty", Count: 1},
		{Role: contextSlashDisplayDot, Owner: "slash", Name: "slash-dot", Type: "empty", Count: 1},
		{Role: contextBeatExcept, Owner: "beat-repeat", Name: "except-voice", Type: "xs:string", Count: 1},
		{Role: contextSlashExcept, Owner: "slash", Name: "except-voice", Type: "xs:string", Count: 1},
		{Role: contextBarlineRepeat, Owner: "barline", Name: "repeat", Type: "repeat", Count: 1},
		{Role: contextBarlineEnding, Owner: "barline", Name: "ending", Type: "ending", Count: 1},
		{Role: contextRepeatDirection, Owner: "repeat", Name: "direction", Type: "backward-forward", Attribute: true, Count: 1},
		{Role: contextRepeatTimes, Owner: "repeat", Name: "times", Type: "xs:nonNegativeInteger", Attribute: true, Count: 1},
		{Role: contextRepeatAfterJump, Owner: "repeat", Name: "after-jump", Type: "yes-no", Attribute: true, Count: 1},
		{Role: contextEndingType, Owner: "ending", Name: "type", Type: "start-stop-discontinue", Attribute: true, Count: 1},
		{Role: contextEndingNumber, Owner: "ending", Name: "number", Type: "ending-number", Attribute: true, Count: 1},
	}, append(contextRepeatContentBindingSpecs(), append(navigationListeningBindingSpecs(), append(navigationNoteListeningBindings(), append(navigationGraphicBindings(), navigationRawBindings()...)...)...)...)...)
}

type contextRepeatRecord struct {
	source   NodeRef
	position Evidence[musicalPosition]
	value    RepeatContextResult
}
type contextRepeatService struct {
	Records []contextRepeatRecord
	ByNode  map[NodeID]int
	Final   bool
}

func contextRepeatOrigin(ctx context.Context, s *Session, timeline *timelineService, n *factNode) (timelineEventFact, error) {
	for n != nil {
		if err := s.step(ctx, 1); err != nil {
			return timelineEventFact{}, err
		}
		if fact, ok := timeline.Facts[n.header.ID]; ok {
			return fact, nil
		}
		n = s.nodes[n.header.Parent]
	}
	gap := stateUnknown[musicalPosition]("context-origin-unavailable")
	return timelineEventFact{Origin: gap, Position: gap, Part: stateUnknown[NodeRef]("context-part-unavailable")}, nil
}

func contextRepeatSameScope(ctx context.Context, s *Session, a, b RepeatContextResult) (Evidence[bool], error) {
	if err := s.step(ctx, 1); err != nil {
		return Evidence[bool]{}, err
	}
	if a.Part.State != EvidenceKnown {
		return stateUnavailable[NodeRef, bool](a.Part), nil
	}
	if b.Part.State != EvidenceKnown {
		return stateUnavailable[NodeRef, bool](b.Part), nil
	}
	same, err := stateSameRef(ctx, s, a.Part.Value, b.Part.Value)
	if err != nil || !same {
		return Evidence[bool]{State: EvidenceKnown, Value: false}, err
	}
	if a.Kind != b.Kind {
		return Evidence[bool]{State: EvidenceKnown, Value: false}, nil
	}
	if a.AllStaves.State != EvidenceKnown {
		return a.AllStaves, nil
	}
	if b.AllStaves.State != EvidenceKnown {
		return b.AllStaves, nil
	}
	if a.AllStaves.Value && b.AllStaves.Value {
		return Evidence[bool]{State: EvidenceKnown, Value: true}, nil
	}
	if a.AllStaves.Value != b.AllStaves.Value {
		return stateUnknown[bool]("context-repeat-overlapping-staff-scope"), nil
	}
	if a.Staff.State != EvidenceKnown {
		return stateUnavailable[Integer, bool](a.Staff), nil
	}
	if b.Staff.State != EvidenceKnown {
		return stateUnavailable[Integer, bool](b.Staff), nil
	}
	if err = s.step(ctx, a.Staff.Value.ByteSize()+b.Staff.Value.ByteSize()); err != nil {
		return Evidence[bool]{}, err
	}
	cmp, err := a.Staff.Value.Cmp(b.Staff.Value)
	return Evidence[bool]{State: EvidenceKnown, Value: cmp == 0}, err
}

// Pair only an established next transition in the same style scope. No stack
// discipline or same-time lexical winner is inferred from serialization order.
func contextRepeatPairRanges(ctx context.Context, s *Session, timeline *timelineService, rows []contextRepeatRecord) error {
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	release, err := contextRepeatIndexReserve(ctx, s, len(rows), len(timeline.State.Closures))
	if err != nil {
		return err
	}
	defer release()
	groups, unknown, all, err := contextRepeatRowGroups(ctx, s, rows)
	if err != nil {
		return err
	}
	fast, err := contextRepeatFastRanges(ctx, s, timeline, rows, groups, unknown)
	if err != nil {
		return err
	}
	closures := make(map[NodeRef]Evidence[bool])
	for _, closure := range timeline.State.Closures {
		if err = s.step(ctx, 1+int64(len(closure.Part.DocumentID))); err != nil {
			return err
		}
		closures[closure.Part] = closure.Complete
	}

	for i := range rows {
		if err = s.step(ctx, 1); err != nil {
			return err
		}
		a := &rows[i]
		if a.value.Type.State != EvidenceKnown || a.value.Type.Value != "start" {
			continue
		}
		a.value.Start = Evidence[NodeRef]{State: EvidenceKnown, Value: a.source}
		if a.position.State != EvidenceKnown {
			a.value.Complete = stateUnavailable[musicalPosition, bool](a.position)
			continue
		}
		best := -1
		gap := Evidence[bool]{State: EvidenceKnown, Value: true}
		if err = s.step(ctx, 1+int64(len(a.value.Part.Value.DocumentID)+len(a.value.Kind))); err != nil {
			return err
		}
		local := groups[contextRepeatPartKind{a.value.Part.Value, a.value.Kind}]
		extra := unknown
		if a.value.Part.State != EvidenceKnown {
			local = all
			extra = nil
		}
		if candidate, ok := fast[i]; ok {
			if candidate.ambiguous {
				gap = stateUnknown[bool]("context-repeat-transition-order-unresolved")
				local = nil
				extra = nil
			} else if candidate.next < 0 {
				local = nil
				extra = nil
			} else {
				local = []int{candidate.next}
				extra = nil
			}
		}
		for left, right := 0, 0; left < len(local) || right < len(extra); {
			j := 0
			if right == len(extra) || left < len(local) && local[left] < extra[right] {
				j = local[left]
				left++
			} else {
				j = extra[right]
				right++
			}
			if i == j {
				continue
			}
			b := &rows[j]
			scope, err := contextRepeatSameScope(ctx, s, a.value, b.value)
			if err != nil {
				return err
			}
			if scope.State == EvidenceKnown && !scope.Value {
				continue
			}
			if b.position.State != EvidenceKnown {
				gap = stateMergeGap(gap, stateUnavailable[musicalPosition, bool](b.position))
				continue
			}
			if b.value.Part.State != EvidenceKnown || b.value.Part.Value != a.value.Part.Value {
				gap = stateMergeGap(gap, scope)
				continue
			}
			order, err := timeline.State.positionOrder(ctx, s, a.position.Value, b.position.Value)
			if err != nil {
				return err
			}
			if order == stateAfter {
				continue
			}
			if scope.State != EvidenceKnown {
				gap = stateMergeGap(gap, scope)
				continue
			}
			if order != stateBefore {
				gap = stateMergeGap(gap, stateUnknown[bool]("context-repeat-transition-order-unresolved"))
				continue
			}
			if best < 0 {
				best = j
				continue
			}
			order, err = timeline.State.positionOrder(ctx, s, b.position.Value, rows[best].position.Value)
			if err != nil {
				return err
			}
			if order == stateBefore {
				best = j
			} else if order != stateAfter {
				gap = stateMergeGap(gap, stateUnknown[bool]("context-repeat-next-transition-unresolved"))
			}
		}
		if gap.State != EvidenceKnown {
			a.value.Complete = gap
			a.value.Stop = stateUnavailable[bool, NodeRef](gap)
			continue
		}
		if best >= 0 {
			stop := &rows[best]
			if stop.value.Type.State != EvidenceKnown {
				a.value.Complete = stateUnavailable[string, bool](stop.value.Type)
				continue
			}
			if stop.value.Type.Value != "stop" {
				a.value.Complete = stateUnknown[bool]("context-repeat-restarted-range-unresolved")
				continue
			}
			a.value.Stop = Evidence[NodeRef]{State: EvidenceKnown, Value: stop.source}
			stop.value.Start = Evidence[NodeRef]{State: EvidenceKnown, Value: a.source}
			stop.value.Stop = Evidence[NodeRef]{State: EvidenceKnown, Value: stop.source}
			a.value.Complete = Evidence[bool]{State: EvidenceKnown, Value: true}
			stop.value.Complete = a.value.Complete
		} else {
			a.value.Complete = stateUnknown[bool]("context-repeat-part-end-unavailable")
			if err = s.step(ctx, 1+int64(len(a.value.Part.Value.DocumentID))); err != nil {
				return err
			}
			if complete, ok := closures[a.value.Part.Value]; ok {
				a.value.Complete = complete
			}
			if a.value.Complete.State == EvidenceKnown && a.value.Complete.Value {
				a.value.Stop = Evidence[NodeRef]{State: EvidenceAbsent}
			}
		}
	}
	return nil
}

var contextRepeatBeatValues = map[string]int64{"maxima": 32, "long": 16, "breve": 8, "whole": 4, "half": 2, "quarter": 1, "eighth": -2, "16th": -4, "32nd": -8, "64th": -16, "128th": -32, "256th": -64, "512th": -128, "1024th": -256}

func contextRepeatBeat(r *contextReader, timeline *timelineService, n *factNode, value RepeatContextResult, pos Evidence[musicalPosition], typeRole, dotRole contextRole, roster *contextRepeatMeterRoster) Evidence[Rational] {
	child, available := r.child(n, typeRole)
	if available.State != EvidenceKnown && available.State != EvidenceAbsent {
		return stateUnavailable[bool, Rational](available)
	}
	kind := stateText(r.scalar(child))
	if kind.State == EvidenceAbsent {
		// The source does not define a unique beat for every compound/additive meter.
		if pos.State != EvidenceKnown {
			return stateUnavailable[musicalPosition, Rational](pos)
		}
		if value.AllStaves.State == EvidenceKnown && value.AllStaves.Value {
			return contextRepeatUniformBeat(r, timeline, pos.Value, roster)
		}
		meter, err := queryState(r.ctx, r.s, &timeline.State, &timeline.State.Times, pos.Value, value.Staff, func(v timeState) Evidence[Rational] { return contextRepeatSimpleMeterBeat(r, v) })
		if err != nil {
			r.err = err
			return stateUnknown[Rational]("context-interrupted")
		}
		if r.err != nil {
			return stateUnknown[Rational]("context-interrupted")
		}
		return meter.Value
	}
	if kind.State != EvidenceKnown {
		return stateUnavailable[string, Rational](kind)
	}

	size, ok := contextRepeatBeatValues[kind.Value]
	if !ok {
		return stateUnknown[Rational]("context-repeat-beat-type-unavailable")
	}
	if err := r.s.retain(256); err != nil {
		r.err = err
		return stateUnknown[Rational]("context-interrupted")
	}
	numerator, denominator := size, int64(1)
	if size < 0 {
		numerator = 1
		denominator = -size
	}
	beat, err := NewRational(big.NewRat(numerator, denominator))
	if err != nil {
		r.err = err
		return stateUnknown[Rational]("context-interrupted")
	}
	part := beat
	// child already established the parent enumeration above.
	two, _ := NewRational(big.NewRat(2, 1))
	for _, ch := range n.children {
		if r.matches(ch, dotRole) {
			part, err = r.s.rationalQuo(r.ctx, part, two)
			if err == nil {
				beat, err = r.s.rationalAdd(r.ctx, beat, part)
			}
			if err != nil {
				r.err = err
				return stateUnknown[Rational]("context-interrupted")
			}
		}
	}
	return Evidence[Rational]{State: EvidenceKnown, Value: beat}
}

func evaluateContextRepeats(ctx context.Context, s *Session, n *factNode, key RuleKey, p *contextPlan, final bool) (Decision, bool, error) {
	if !final {
		return Decision{}, true, nil
	}
	service, err := s.newContextService(ctx, key)
	if err != nil {
		return Decision{}, false, err
	}
	if service.Repeats != nil && service.Repeats.Final {
		return Decision{Outcome: OutcomePass}, false, nil
	}
	timeline := s.timeline[p.Timeline]
	if timeline == nil {
		timeline = &timelineService{}
	}
	if err = s.retain(256); err != nil {
		return Decision{}, false, err
	}
	result := &contextRepeatService{ByNode: make(map[NodeID]int)}
	r := &contextReader{ctx: ctx, s: s, b: p.Bindings}
	meterRoster := &contextRepeatMeterRoster{}
	defer meterRoster.close()
	for _, id := range s.sequence {
		node := s.nodes[id]
		kind := RepeatStyleKind("")
		typeRole, slashRole, dotsRole, displayRole, dotRole, exceptRole := contextRole(""), contextRole(""), contextRole(""), contextRole(""), contextRole(""), contextRole("")
		switch {
		case r.matches(node, contextBeatRepeat):
			kind = RepeatStyleBeat
			typeRole = contextBeatType
			slashRole = contextBeatSlashes
			dotsRole = contextBeatDots
			displayRole = contextBeatDisplayType
			dotRole = contextBeatDisplayDot
			exceptRole = contextBeatExcept
		case r.matches(node, contextMeasureRepeat):
			kind = RepeatStyleMeasure
			typeRole = contextMeasureType
			slashRole = contextMeasureSlashes
		case r.matches(node, contextSlashRepeat):
			kind = RepeatStyleSlash
			typeRole = contextSlashType
			dotsRole = contextSlashDots
			displayRole = contextSlashDisplayType
			dotRole = contextSlashDisplayDot
			exceptRole = contextSlashExcept
		}
		if r.err != nil {
			return Decision{}, false, r.err
		}
		if kind == "" {
			continue
		}
		fact, e := contextRepeatOrigin(ctx, s, timeline, node)
		if e != nil {
			return Decision{}, false, e
		}
		value := RepeatContextResult{Beat: Evidence[Rational]{State: EvidenceAbsent}, PatternMeasures: Evidence[Integer]{State: EvidenceAbsent}, Kind: kind, Part: fact.Part, Type: stateText(r.attribute(node, typeRole)), Staff: stateInteger(r.attribute(s.nodes[node.header.Parent], contextRepeatStaff)), Slashes: Evidence[Integer]{State: EvidenceAbsent}, UseDots: Evidence[bool]{State: EvidenceAbsent}, UseStems: Evidence[bool]{State: EvidenceAbsent}, Start: Evidence[NodeRef]{State: EvidenceAbsent}, Stop: Evidence[NodeRef]{State: EvidenceAbsent}, ContentEquivalent: stateUnknown[bool]("context-repeat-content-equivalence-unresolved"), Complete: r.complete(node)}
		if slashRole != "" {
			value.Slashes = stateInteger(r.attribute(node, slashRole))
		}
		if dotsRole != "" {
			value.UseDots = stateYesNo(r.attribute(node, dotsRole))
		}
		if kind == RepeatStyleSlash {
			value.UseStems = stateYesNo(r.attribute(node, contextSlashStems))
			value.ContentEquivalent = Evidence[bool]{State: EvidenceAbsent}
		}
		if kind == RepeatStyleMeasure {
			if value.Type.State == EvidenceKnown && value.Type.Value == "stop" {
				value.PatternMeasures = Evidence[Integer]{State: EvidenceAbsent}
			} else {
				raw := r.scalar(node)
				if raw.State == EvidenceKnown && raw.Value.Kind == AtomicString && raw.Value.Text == "" {
					value.PatternMeasures = stateUnknown[Integer]("context-repeat-pattern-size-unavailable")
				} else {
					value.PatternMeasures = stateInteger(raw)
				}
			}
		}
		if value.Type.State == EvidenceKnown && value.Type.Value == "stop" {
			value.ContentEquivalent = Evidence[bool]{State: EvidenceAbsent}
		}
		contextRepeatDefaults(&value)
		if displayRole != "" {
			value.Beat = contextRepeatBeat(r, timeline, node, value, fact.Position, displayRole, dotRole, meterRoster)
		}
		for _, ch := range node.children {
			if !r.step(1) {
				return Decision{}, false, r.err
			}
			if exceptRole != "" && r.matches(ch, exceptRole) {
				if err = s.retain(256); err != nil {
					return Decision{}, false, err
				}
				value.ExceptVoices = append(value.ExceptVoices, stateText(r.scalar(ch)))
			}
		}
		if r.err != nil {
			return Decision{}, false, r.err
		}
		if err = s.retain(4096); err != nil {
			return Decision{}, false, err
		}
		result.ByNode[id] = len(result.Records)
		result.Records = append(result.Records, contextRepeatRecord{source: NodeRef{s.documentID, id}, position: fact.Position, value: value})
	}
	if err = contextRepeatPairRanges(ctx, s, timeline, result.Records); err != nil {
		return Decision{}, false, err
	}
	if err = contextRepeatNoteScopes(ctx, s, timeline, result.Records); err != nil {
		return Decision{}, false, err
	}
	if err = contextRepeatProveContent(ctx, s, p, timeline, result.Records); err != nil {
		return Decision{}, false, err
	}
	for i := range result.Records {
		row := &result.Records[i]
		if err = s.publishContextResult(ctx, ContextResult{Provider: key, Subject: row.source, Property: ContextPropertyRepeat, Repeat: &row.value}); err != nil {
			return Decision{}, false, err
		}
	}
	result.Final = true
	service.Repeats = result
	return Decision{Outcome: OutcomePass}, false, nil
}

// Project only the beat-bearing fields. Display/print metadata is independent.
func contextRepeatSimpleMeterBeat(r *contextReader, v timeState) Evidence[Rational] {
	if !r.step(1) {
		return stateUnknown[Rational]("context-interrupted")
	}
	if v.Interchangeable.State != EvidenceAbsent && v.Interchangeable.State != "" {
		if v.Interchangeable.State != EvidenceKnown {
			return stateUnavailable[interchangeableState, Rational](v.Interchangeable)
		}
		return stateUnknown[Rational]("context-repeat-interchangeable-meter-unresolved")
	}
	if v.SenzaMisura.State != EvidenceAbsent {
		if v.SenzaMisura.State != EvidenceKnown {
			return stateUnavailable[string, Rational](v.SenzaMisura)
		}
		return stateUnknown[Rational]("context-repeat-meter-beat-derivation-unresolved")
	}
	if len(v.Pairs) != 1 {
		return stateUnknown[Rational]("context-repeat-meter-beat-derivation-unresolved")
	}
	pair := v.Pairs[0]
	if pair.Beats.State != EvidenceKnown {
		return stateUnavailable[string, Rational](pair.Beats)
	}
	if pair.BeatType.State != EvidenceKnown {
		return stateUnavailable[string, Rational](pair.BeatType)
	}
	if !r.step(int64(len(pair.Beats.Value) + len(pair.BeatType.Value))) {
		return stateUnknown[Rational]("context-interrupted")
	}
	beats, denominator := pair.Beats.Value, pair.BeatType.Value
	if beats != "1" && beats != "2" && beats != "3" && beats != "4" {
		return stateUnknown[Rational]("context-repeat-meter-beat-derivation-unresolved")
	}
	denom := int64(0)
	switch denominator {
	case "1":
		denom = 1
	case "2":
		denom = 2
	case "4":
		denom = 4
	case "8":
		denom = 8
	case "16":
		denom = 16
	case "32":
		denom = 32
	case "64":
		denom = 64
	}
	if denom == 0 {
		return stateUnknown[Rational]("context-repeat-meter-beat-derivation-unresolved")
	}
	if err := r.s.retain(256); err != nil {
		r.err = err
		return stateUnknown[Rational]("context-interrupted")
	}
	beat, err := NewRational(big.NewRat(4, denom))
	if err != nil {
		r.err = err
		return stateUnknown[Rational]("context-interrupted")
	}
	return Evidence[Rational]{State: EvidenceKnown, Value: beat}
}

// A finite set of explicit selectors partitions the effective staff domain.
// Query those selectors and, when needed, one representative of their complement.
// Staff count and selector values remain arbitrary-width; neither is expanded.
func contextRepeatUniformBeat(r *contextReader, timeline *timelineService, pos musicalPosition, rosters ...*contextRepeatMeterRoster) Evidence[Rational] {
	count, err := timeline.State.effectiveStaves(r.ctx, r.s, pos)
	if err != nil {
		r.err = err
		return stateUnknown[Rational]("context-interrupted")
	}
	if count.Value.State != EvidenceKnown {
		return stateUnavailable[Integer, Rational](count.Value)
	}
	if !count.Value.Value.Valid() || count.Value.Value.value.Sign() <= 0 {
		return stateUnknown[Rational]("context-repeat-staff-domain-unavailable")
	}
	roster := (*contextRepeatMeterRoster)(nil)
	if len(rosters) > 0 {
		roster = rosters[0]
	}
	if roster == nil {
		roster = &contextRepeatMeterRoster{}
		defer roster.close()
	}
	if err := roster.prepare(r.ctx, r.s, timeline); err != nil {
		r.err = err
		return stateUnknown[Rational]("context-interrupted")
	}
	if !r.step(1 + int64(len(pos.Part.DocumentID))) {
		return stateUnknown[Rational]("context-interrupted")
	}
	publications, gaps := roster.publications[pos.Part], roster.gaps[pos.Part]
	release, err := contextRepeatIndexReserve(r.ctx, r.s, len(publications), len(gaps))
	if err != nil {
		r.err = err
		return stateUnknown[Rational]("context-interrupted")
	}
	defer release()
	var selectors []Integer
	collect := func(part NodeRef, scope stateScope) Evidence[bool] {
		if !r.step(1 + int64(len(part.DocumentID)+len(pos.Part.DocumentID))) {
			return stateUnknown[bool]("context-interrupted")
		}
		if part != pos.Part || scope.AllStaves {
			return navigationKnownBool(true)
		}
		if !scope.Staff.Valid() || scope.Staff.value.Sign() <= 0 {
			return stateUnknown[bool]("context-repeat-meter-staff-scope-unavailable")
		}
		if !r.step(scope.Staff.ByteSize() + count.Value.Value.ByteSize()) {
			return stateUnknown[bool]("context-interrupted")
		}
		cmp, e := scope.Staff.Cmp(count.Value.Value)
		if e != nil {
			r.err = e
			return stateUnknown[bool]("context-interrupted")
		}
		if cmp <= 0 {
			selectors = append(selectors, scope.Staff)
		}
		return navigationKnownBool(true)
	}
	var possibleScopeGaps []stateGap
	for _, i := range publications {
		publication := &timeline.State.Times.Publications[i]
		v := collect(publication.Position.Part, publication.Scope)
		if r.err != nil {
			return stateUnknown[Rational]("context-interrupted")
		}
		if v.State != EvidenceKnown {
			if publication.Value.State != EvidenceKnown && publication.Value.State != EvidenceAbsent {
				v = stateMergeGap(v, stateUnavailable[timeState, bool](publication.Value))
			}
			possibleScopeGaps = append(possibleScopeGaps, stateGap{Part: publication.Position.Part, Position: &publication.Position, Scope: stateScope{AllStaves: true}, Availability: v})
		}
	}
	for _, i := range gaps {
		gap := timeline.State.Times.Gaps[i]
		if !r.step(1 + int64(len(gap.Part.DocumentID)+len(pos.Part.DocumentID))) {
			return stateUnknown[Rational]("context-interrupted")
		}
		if gap.Part == pos.Part {
			bound := gap.Position
			if bound == nil {
				bound = gap.Placement.Lower
			}
			if bound != nil {
				order, e := timeline.State.positionOrder(r.ctx, r.s, *bound, pos)
				if e != nil {
					r.err = e
					return stateUnknown[Rational]("context-interrupted")
				}
				if order == stateAfter {
					continue
				}
			} else if !gap.Placement.Displaced && gap.FirstMeasure > pos.MeasureOrdinal {
				continue
			}
		}
		v := collect(gap.Part, gap.Scope)
		if r.err != nil {
			return stateUnknown[Rational]("context-interrupted")
		}
		if v.State != EvidenceKnown {
			if gap.Availability.State != EvidenceKnown && gap.Availability.State != EvidenceAbsent {
				v = stateMergeGap(v, gap.Availability)
			}
			possible := gap
			possible.Scope = stateScope{AllStaves: true}
			possible.Availability = v
			possibleScopeGaps = append(possibleScopeGaps, possible)
		}
	}
	sort.Slice(selectors, func(i, j int) bool {
		if !r.step(selectors[i].ByteSize() + selectors[j].ByteSize()) {
			return false
		}
		cmp, e := selectors[i].Cmp(selectors[j])
		if e != nil {
			r.err = e
			return false
		}
		return cmp < 0
	})
	if r.err != nil {
		return stateUnknown[Rational]("context-interrupted")
	}
	result := Evidence[Rational]{State: EvidenceAbsent}
	availability := navigationKnownBool(true)
	query := func(staff Integer) bool {
		beat, e := queryStateAtFull(r.ctx, r.s, &timeline.State, &timeline.State.Times, pos, Evidence[Integer]{State: EvidenceKnown, Value: staff}, func(v timeState) Evidence[Rational] { return contextRepeatSimpleMeterBeat(r, v) }, true, true, nil, possibleScopeGaps)
		if e != nil {
			r.err = e
			return false
		}
		if r.err != nil {
			return false
		}
		if beat.Value.State != EvidenceKnown {
			gap := stateUnavailable[Rational, bool](beat.Value)
			if gap.State == EvidenceAbsent {
				gap = stateUnknown[bool]("context-repeat-meter-unavailable")
			}
			availability = stateMergeGap(availability, gap)
			return true
		}
		if result.State == EvidenceAbsent {
			result = beat.Value
			return true
		}
		cmp, e := r.s.rationalCmp(r.ctx, result.Value, beat.Value.Value)
		if e != nil {
			r.err = e
			return false
		}
		if cmp != 0 {
			availability = stateMergeGap(availability, stateUnknown[bool]("context-repeat-all-staff-meter-uniformity-unresolved"))
			return true
		}
		return true
	}
	missing := uint64(1)
	var previous Integer
	for _, staff := range selectors {
		if !r.step(1) {
			return stateUnknown[Rational]("context-interrupted")
		}
		if previous.Valid() {
			if !r.step(previous.ByteSize() + staff.ByteSize()) {
				return stateUnknown[Rational]("context-interrupted")
			}
			cmp, e := previous.Cmp(staff)
			if e != nil {
				r.err = e
				return stateUnknown[Rational]("context-interrupted")
			}
			if cmp == 0 {
				continue
			}
		}
		previous = staff
		cmp, e := navigationCountCompare(r.ctx, r.s, staff, missing)
		if e != nil {
			r.err = e
			return stateUnknown[Rational]("context-interrupted")
		}
		if cmp == 0 {
			missing++
		}
		if !query(staff) {
			return result
		}
	}
	cmp, err := navigationCountCompare(r.ctx, r.s, count.Value.Value, missing)
	if err != nil {
		r.err = err
		return stateUnknown[Rational]("context-interrupted")
	}
	if cmp >= 0 {
		representative, e := navigationInteger(r.ctx, r.s, missing)
		if e != nil {
			r.err = e
			return stateUnknown[Rational]("context-interrupted")
		}
		if !query(representative) {
			return result
		}
	}
	if availability.State != EvidenceKnown {
		return stateUnavailable[bool, Rational](availability)
	}
	return result
}

// A roster borrows immutable timeline entries for one completed provider
// evaluation. It is prepared lazily once, prepaid before allocating any map or
// slice backing, and released even when a later query is interrupted.
type contextRepeatMeterRoster struct {
	timeline                   *timelineService
	publications               map[NodeRef][]int
	gaps                       map[NodeRef][]int
	publicationCount, gapCount int
	release                    func()
}

func (index *contextRepeatMeterRoster) close() {
	if index.release != nil {
		index.release()
		index.release = nil
	}
	index.timeline = nil
	index.publications = nil
	index.gaps = nil
}
func (index *contextRepeatMeterRoster) prepare(ctx context.Context, s *Session, timeline *timelineService) error {
	if err := s.step(ctx, 1); err != nil {
		return err
	}
	if index.release != nil {
		if index.timeline != timeline || index.publicationCount != len(timeline.State.Times.Publications) || index.gapCount != len(timeline.State.Times.Gaps) {
			return contract("repeat meter roster", "immutable provider timeline required")
		}
		return nil
	}
	release, err := contextRepeatIndexReserve(ctx, s, len(timeline.State.Times.Publications), len(timeline.State.Times.Gaps))
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			release()
		}
	}()
	publications := make(map[NodeRef][]int)
	gaps := make(map[NodeRef][]int)
	for i, p := range timeline.State.Times.Publications {
		if err = s.step(ctx, 1+int64(len(p.Position.Part.DocumentID))); err != nil {
			return err
		}
		publications[p.Position.Part] = append(publications[p.Position.Part], i)
	}
	for i, g := range timeline.State.Times.Gaps {
		if err = s.step(ctx, 1+int64(len(g.Part.DocumentID))); err != nil {
			return err
		}
		gaps[g.Part] = append(gaps[g.Part], i)
	}
	index.timeline = timeline
	index.publications = publications
	index.gaps = gaps
	index.publicationCount = len(timeline.State.Times.Publications)
	index.gapCount = len(timeline.State.Times.Gaps)
	index.release = release
	committed = true
	return nil
}
