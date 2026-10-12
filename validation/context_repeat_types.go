package validation

import "context"

// RepeatStyleKind identifies a closed graphical repeat/style family. These
// styles never synthesize playback events or navigation edges.
type RepeatStyleKind string

// Graphical repeat and slash families.
const (
	RepeatStyleBeat    RepeatStyleKind = "beat-repeat"
	RepeatStyleMeasure RepeatStyleKind = "measure-repeat"
	RepeatStyleSlash   RepeatStyleKind = "slash"
)

// RepeatContextResult is an owned projection of one encoded style transition.
// Start/Stop are established half-open range endpoints; absent Stop may mean
// continuation to part end only when Complete is known. AllStaves is symbolic.
type RepeatContextResult struct {
	Notes                     []RepeatNoteScope
	Kind                      RepeatStyleKind
	Part                      Evidence[NodeRef]
	AllStaves                 Evidence[bool]
	Staff                     Evidence[Integer]
	Type                      Evidence[string]
	Slashes                   Evidence[Integer]
	UseDots, UseStems, Active Evidence[bool]
	PatternMeasures           Evidence[Integer]
	Beat                      Evidence[Rational]
	Start, Stop               Evidence[NodeRef]
	ExceptVoices              []Evidence[string]
	ContentEquivalent         Evidence[bool]
	Complete                  Evidence[bool]
}

func cloneRepeatContext(ctx context.Context, s *Session, in *RepeatContextResult) (*RepeatContextResult, error) {
	if in == nil {
		return nil, nil
	}
	if _, err := stateReserve(ctx, s, *in); err != nil {
		return nil, err
	}
	out := stateClone(*in)
	if err := s.step(ctx, 1); err != nil {
		return nil, err
	}
	return &out, nil
}

func contextRepeatDefaults(result *RepeatContextResult) {
	if result.Kind == RepeatStyleBeat || result.Kind == RepeatStyleMeasure {
		if result.Slashes.State == EvidenceAbsent {
			result.Slashes = Evidence[Integer]{State: EvidenceKnown, Value: stateOne()}
		}
	}
	if result.Kind == RepeatStyleBeat || result.Kind == RepeatStyleSlash {
		if result.UseDots.State == EvidenceAbsent {
			result.UseDots = Evidence[bool]{State: EvidenceKnown, Value: false}
		}
	}
	if result.Kind == RepeatStyleSlash {
		if result.UseStems.State == EvidenceAbsent {
			result.UseStems = Evidence[bool]{State: EvidenceKnown, Value: false}
		}
		if result.UseStems.State == EvidenceKnown && result.UseStems.Value {
			result.UseDots = Evidence[bool]{State: EvidenceAbsent}
		}
	}
	if result.Staff.State == EvidenceAbsent {
		result.AllStaves = Evidence[bool]{State: EvidenceKnown, Value: true}
	} else if result.Staff.State == EvidenceKnown {
		result.AllStaves = Evidence[bool]{State: EvidenceKnown, Value: false}
	} else {
		result.AllStaves = stateUnavailable[Integer, bool](result.Staff)
	}
	if result.Type.State == EvidenceKnown {
		result.Active = Evidence[bool]{State: EvidenceKnown, Value: result.Type.Value == "start"}
	} else {
		result.Active = stateUnavailable[string, bool](result.Type)
	}
}

func contextRepeatPropertyValid(property ContextProperty) bool {
	return property == ContextPropertyNavigationTraversal
}

// ContextPropertyNavigationTraversal identifies a bounded part traversal snapshot.
const ContextPropertyNavigationTraversal ContextProperty = "navigation-traversal"
