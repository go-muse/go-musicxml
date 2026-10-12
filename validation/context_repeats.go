package validation

import (
	"context"
	"math/big"
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
	for i := range rows {
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
		for j := range rows {
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
			for _, closure := range timeline.State.Closures {
				if err := s.step(ctx, 1); err != nil {
					return err
				}
				if closure.Part == a.value.Part.Value {
					a.value.Complete = closure.Complete
				}
			}
			if a.value.Complete.State == EvidenceKnown && a.value.Complete.Value {
				a.value.Stop = Evidence[NodeRef]{State: EvidenceAbsent}
			}
		}
	}
	return nil
}

var contextRepeatBeatValues = map[string]int64{"maxima": 32, "long": 16, "breve": 8, "whole": 4, "half": 2, "quarter": 1, "eighth": -2, "16th": -4, "32nd": -8, "64th": -16, "128th": -32, "256th": -64, "512th": -128, "1024th": -256}

func contextRepeatBeat(r *contextReader, timeline *timelineService, n *factNode, value RepeatContextResult, pos Evidence[musicalPosition], typeRole, dotRole contextRole) Evidence[Rational] {
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
		staff := value.Staff
		if value.AllStaves.State == EvidenceKnown && value.AllStaves.Value {
			staff = Evidence[Integer]{State: EvidenceAbsent}
		}
		meter, err := queryState(r.ctx, r.s, &timeline.State, &timeline.State.Times, pos.Value, staff, func(v timeState) Evidence[timeState] { return Evidence[timeState]{State: EvidenceKnown, Value: v} })
		if err != nil {
			r.err = err
			return stateUnknown[Rational]("context-interrupted")
		}
		if meter.Value.State != EvidenceKnown {
			return stateUnavailable[timeState, Rational](meter.Value)
		}
		v := meter.Value.Value
		if v.SenzaMisura.State != EvidenceAbsent || len(v.Pairs) != 1 || v.Pairs[0].Beats.State != EvidenceKnown || v.Pairs[0].BeatType.State != EvidenceKnown {
			return stateUnknown[Rational]("context-repeat-meter-beat-derivation-unresolved")
		}
		beats, denominator := v.Pairs[0].Beats.Value, v.Pairs[0].BeatType.Value
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
	complete := r.complete(n)
	if complete.State != EvidenceKnown {
		return stateUnavailable[bool, Rational](complete)
	}
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
		if kind == "" {
			continue
		}
		fact, e := contextRepeatOrigin(ctx, s, timeline, node)
		if e != nil {
			return Decision{}, false, e
		}
		value := RepeatContextResult{Kind: kind, Part: fact.Part, Type: stateText(r.attribute(node, typeRole)), Staff: stateInteger(r.attribute(s.nodes[node.header.Parent], contextRepeatStaff)), Slashes: Evidence[Integer]{State: EvidenceAbsent}, UseDots: Evidence[bool]{State: EvidenceAbsent}, UseStems: Evidence[bool]{State: EvidenceAbsent}, Start: Evidence[NodeRef]{State: EvidenceAbsent}, Stop: Evidence[NodeRef]{State: EvidenceAbsent}, ContentEquivalent: stateUnknown[bool]("context-repeat-content-equivalence-unresolved"), Complete: r.complete(node)}
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
		contextRepeatDefaults(&value)
		if displayRole != "" {
			value.Beat = contextRepeatBeat(r, timeline, node, value, fact.Position, displayRole, dotRole)
		}
		for _, ch := range node.children {
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
