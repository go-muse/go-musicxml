package validation

import "unsafe"

// These constant-size arrays make the independent closed-shape margins part of
// 386 cross-compilation too. Compilation does not establish runtime behavior;
// execution and cancellation on that architecture remain separate CI gates.
var (
	_ [timelineFactReserveBytes - int64(unsafe.Sizeof(timelineEventFact{})) - 512]struct{}
	_ [timelineNoteReserveBytes - int64(unsafe.Sizeof(noteTimingOrigin{})) - 512]struct{}
	_ [timelineDirectionReserveBytes - int64(unsafe.Sizeof(directionTimingOrigin{})) - 512]struct{}
	_ [timelineFigureReserveBytes - int64(unsafe.Sizeof(figuredBassTimingOrigin{})) - 512]struct{}
	_ [timelineBendReserveBytes - 3*int64(unsafe.Sizeof(bendTimingOrigin{}))]struct{}
	_ [timelineNoteInstrumentReserveBytes - 3*int64(unsafe.Sizeof(noteInstrumentSet{}))]struct{}
	_ [stateIndexPublicationBytes - 3*int64(unsafe.Sizeof(stateIndexScope{})+unsafe.Sizeof(stateIndexBucket{})) - 4*int64(unsafe.Sizeof(int(0)))]struct{}
	_ [timelineFrameReserveBytes - 4*int64(unsafe.Sizeof(timelineFrame{})+unsafe.Sizeof(Rational{})) - 128]struct{}
	_ [statePossiblePublicationBytes - int64(unsafe.Sizeof(statePossiblePublication{}))]struct{}
)

// Grace's admitted variable scratch owns streams, lookup maps, sequence IDs,
// two note-local gaps, and up to three additional structural gaps per block.
// Keep these constraints active even when a target architecture is compile-only.
const (
	independentGraceNoteShape = 4*unsafe.Sizeof(graceStream{}) +
		4*(unsafe.Sizeof(graceStreamKey{})+unsafe.Sizeof(int(0))) +
		4*(unsafe.Sizeof(NodeID(0))+unsafe.Sizeof(int(0))) +
		4*unsafe.Sizeof(NodeID(0)) +
		4*(unsafe.Sizeof(NodeRef{})+unsafe.Sizeof((*gracePartDomain)(nil))) +
		unsafe.Sizeof(gracePartDomain{}) + 4*unsafe.Sizeof(graceDomainNote{}) +
		8*unsafe.Sizeof(graceScopedGap{}) + 4*unsafe.Sizeof(NodeID(0)) +
		4*(unsafe.Sizeof(NodeID(0))+unsafe.Sizeof(Evidence[NodeRef]{}))
	independentGraceBlockShape = 12*unsafe.Sizeof(graceScopedGap{}) +
		4*(unsafe.Sizeof(NodeRef{})+unsafe.Sizeof((*gracePartDomain)(nil))) +
		unsafe.Sizeof(gracePartDomain{})
)

var (
	_ [timelineGraceScratchPerNote - int64(independentGraceNoteShape)]struct{}
	_ [timelineGraceScratchPerBlock - int64(independentGraceBlockShape)]struct{}
)

var (
	_ [timelineExcursionReserveBytes - 3*int64(unsafe.Sizeof(timelineUnitExcursion{}))]struct{}
	_ [timelineUnitPublisherBytes - int64(unsafe.Sizeof(timelineUnitPublisher{}))]struct{}
	_ [timelineUnitGateBytes - 4*int64(unsafe.Sizeof(NodeID(0))+unsafe.Sizeof(timelineExcursionGate{}))]struct{}
	_ [timelineUnitGateBytes - 4*int64(unsafe.Sizeof(NodeRef{})+unsafe.Sizeof(bool(false)))]struct{}
	_ [timelineUnitStageBytes - int64(unsafe.Sizeof(timelineService{})) - 512]struct{}
)
