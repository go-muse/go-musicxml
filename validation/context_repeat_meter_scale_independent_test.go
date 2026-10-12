package validation

import (
	"context"
	"testing"
)

// Subtract identical authoritative timeline queries so this guard isolates the
// added uniform-meter selector collection, not already accepted state costs.
func TestIndependentContextUniformMeterUnrelatedPartsLinearWork(t *testing.T) {
	measure := func(count int, uniform bool) int64 {
		s := rationalSession(t, Budgets{MaxRuleSteps: 1 << 34, MaxBufferedBytes: 512 << 20})
		defer s.cleanup()
		service := &timelineService{}
		positions := make([]musicalPosition, count)
		for i := 0; i < count; i++ {
			part := NodeRef{"d", NodeID(10000 + i)}
			pos := statePosition(t, 0, 0, 1)
			pos.Part = part
			service.State.Staves.Publications = append(service.State.Staves.Publications, statePublication[Integer]{Position: pos, Scope: stateScope{AllStaves: true}, Value: stateKnown(stateInt(t, 1))})
			service.State.Times.Publications = append(service.State.Times.Publications, statePublication[timeState]{Position: pos, Scope: stateScope{AllStaves: true}, Value: stateKnown(repeatMeterValue("4", "4"))})
			service.State.Closures = append(service.State.Closures, stateClosure{Part: part, Through: 2, Complete: stateKnown(true)})
			positions[i] = statePosition(t, 0, 2, 1)
			positions[i].Part = part
		}
		r := &contextReader{ctx: context.Background(), s: s}
		roster := &contextRepeatMeterRoster{}
		if uniform {
			if err := roster.prepare(r.ctx, s, service); err != nil {
				t.Fatal(err)
			}
			defer roster.close()
		}
		for _, pos := range positions {
			var got Evidence[Rational]
			if uniform {
				got = contextRepeatUniformBeat(r, service, pos, roster)
			} else {
				count, e := service.State.effectiveStaves(r.ctx, s, pos)
				if e != nil || count.Value.State != EvidenceKnown {
					t.Fatalf("baseline count %+v %v", count, e)
				}
				meter, e := queryStateAtFull(r.ctx, s, &service.State, &service.State.Times, pos, stateKnown(stateInt(t, 1)), func(v timeState) Evidence[Rational] { return contextRepeatSimpleMeterBeat(r, v) }, true, true, nil, nil)
				if e != nil {
					t.Fatal(e)
				}
				got = meter.Value
			}
			if r.err != nil || got.State != EvidenceKnown || got.Value.String() != "1" {
				t.Fatalf("uniform=%t result %+v %v", uniform, got, r.err)
			}
		}
		return rationalUsage(t, s, BudgetRuleSteps)
	}
	smallFull, smallBase := measure(100, true), measure(100, false)
	largeFull, largeBase := measure(1000, true), measure(1000, false)
	small, large := smallFull-smallBase, largeFull-largeBase
	t.Logf("100 parts full/base/extra=%d/%d/%d;1000=%d/%d/%d;extra ratio %.2f", smallFull, smallBase, small, largeFull, largeBase, large, float64(large)/float64(small))
	if small <= 0 || large > small*15 {
		t.Fatalf("uniform selector collection rescanned unrelated parts: %d > 15*%d", large, small)
	}
}
