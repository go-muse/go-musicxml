package validation

import (
	"context"
	"errors"
	"testing"
)

func TestIndependentTimelineFigureIndexMatchesCausalScan(t *testing.T) {
	for _, mode := range []string{"known", "unknown-position", "unknown-kind", "unknown-owner", "duplicate-domain"} {
		t.Run(mode, func(t *testing.T) {
			s, v, b, blocks := independentDirectionIndexFixture(t)
			switch mode {
			case "unknown-position":
				f := v.Facts[104]
				f.Position = stateUnknown[musicalPosition]("unplaced")
				v.Facts[104] = f
			case "unknown-kind":
				blocks[0].content.children[5].end.ChildrenComplete = stateUnknown[bool]("incomplete")
			case "unknown-owner":
				blocks[0].part = stateUnknown[NodeRef]("owner")
			case "duplicate-domain":
				blocks = append(blocks, blocks[0])
			}
			domains := make([]figuredBassDomain, len(blocks))
			for i, block := range blocks {
				domains[i] = figuredBassDomain{block: block, complete: stateKnown(true)}
			}
			before := rationalUsage(t, s, BudgetBufferedBytes)
			index, release, err := v.buildFiguredBassNoteIndex(&stateReader{ctx: context.Background(), s: s, b: b}, domains)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { release() }()
			for numerator := int64(0); numerator < 31; numerator++ {
				origin := v.Facts[10]
				origin.Position = origin.Origin
				origin.Position.Value.Offset = mustRational(t, numerator, 3)
				got, fact, handled, err := index.anchor(context.Background(), s, v, origin)
				if err != nil {
					t.Fatal(err)
				}
				if mode == "known" && !handled {
					t.Fatal("complete known figure domain bypassed index")
				}
				if !handled {
					continue
				}
				r := &stateReader{ctx: context.Background(), s: s, b: b}
				want, wantFact := v.figuredBassAnchor(r, domains, origin, stateKnown(true))
				if r.err != nil {
					t.Fatal(r.err)
				}
				if got.State != want.State || got.Value != want.Value || got.Reason != want.Reason {
					t.Fatalf("q=%d/3 fast=%+v slow=%+v", numerator, got, want)
				}
				if got.State == EvidenceKnown && fact.Position.Value.Offset.String() != wantFact.Position.Value.Offset.String() {
					t.Fatal("selected fact differs")
				}
			}
			release()
			release = timelineNoScratch
			if rationalUsage(t, s, BudgetBufferedBytes) != before {
				t.Fatal("figure index scratch leaked")
			}
		})
	}
}

func TestIndependentTimelineFigureIndexEveryCancellationBoundary(t *testing.T) {
	s, v, b, blocks := independentDirectionIndexFixture(t)
	domains := []figuredBassDomain{{block: blocks[0], complete: stateKnown(true)}}
	probe := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	_, release, err := v.buildFiguredBassNoteIndex(&stateReader{ctx: probe, s: s, b: b}, domains)
	if err != nil {
		t.Fatal(err)
	}
	release()
	calls := probe.calls
	for at := 1; at <= calls; at++ {
		s, v, b, blocks := independentDirectionIndexFixture(t)
		domains := []figuredBassDomain{{block: blocks[0], complete: stateKnown(true)}}
		before := rationalUsage(t, s, BudgetBufferedBytes)
		ctx := &rationalCancelContext{Context: context.Background(), at: at}
		got, release, err := v.buildFiguredBassNoteIndex(&stateReader{ctx: ctx, s: s, b: b}, domains)
		release()
		if !errors.Is(err, context.Canceled) || got != nil {
			t.Fatalf("poll%d partial index or wrong error: %v", at, err)
		}
		if rationalUsage(t, s, BudgetBufferedBytes) != before {
			t.Fatalf("poll%d leaked figure-index scratch", at)
		}
		s.cleanup()
		if rationalUsage(t, s, BudgetBufferedBytes) != 0 {
			t.Fatal("cleanup leaked")
		}
	}
	t.Logf("interrupted all %d figure-index polls", calls)
}
