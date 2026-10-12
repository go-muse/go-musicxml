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
