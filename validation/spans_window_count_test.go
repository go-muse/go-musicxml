package validation

import (
	"context"
	"errors"
	"math/rand"
	"testing"
)

// tiedUnit describes one random tied endpoint.
type tiedUnit struct {
	note     int
	role     Evidence[string]
	domain   Evidence[NodeRef]
	anchor   bool
	number   Evidence[Atomic]
	implied  bool
	pitch    string
	complete Evidence[bool]
}

func (u tiedUnit) endpoint(documentID string, index int) *spanEndpoint {
	e := &spanEndpoint{ref: NodeRef{documentID, NodeID(1000 + index)}, order: uint64(index), complete: u.complete, pitch: u.pitch}
	p := &e.payload
	p.Family = SpanTied
	p.Role = u.role
	p.Domain = u.domain
	p.Anchor = stateUnknown[NodeRef]("span-anchor-unavailable")
	if u.anchor {
		p.Anchor = Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{documentID, NodeID(1 + u.note)}}
	}
	p.Number = u.number
	p.Identity = SpanIdentityNumber
	if u.implied {
		p.Identity = SpanIdentityImpliedTie
	}
	p.Position = stateUnknown[SpanPosition]("span-timeline-position-unavailable")
	p.Association = stateUnknown[bool]("span-correspondence-unresolved")
	return e
}

// The windows' count of tied pairs on two notes is the buffered provider's:
// on random runs of tied endpoints, split into windows at random notes, the
// streaming scan reaches the number of pairs the document-wide passes
// establish and the relational pass would publish a recommendation for.
func TestWindowSpansTiedCountMatchesAssociation(t *testing.T) {
	ctx := context.Background()
	known := func(v string) Evidence[string] { return Evidence[string]{State: EvidenceKnown, Value: v} }
	paired, withheld := 0, 0
	for seed := int64(0); seed < 4000; seed++ {
		rng := rand.New(rand.NewSource(seed))
		r, k := spanWindowUnit(t)
		s := r.s
		p1, p2 := NodeRef{s.documentID, 501}, NodeRef{s.documentID, 502}
		var units []tiedUnit
		notes := 1 + rng.Intn(7)
		// Half of the documents are plain chains, where pairs stand; the
		// rest take every irregularity at its own rate.
		plain := seed%2 == 0
		odd := func(n int) bool { return !plain && rng.Intn(n) == 0 }
		for note := 0; note < notes; note++ {
			domain := Evidence[NodeRef]{State: EvidenceKnown, Value: p1}
			switch rng.Intn(12) {
			case 0, 1, 2:
				domain.Value = p2
			case 3:
				if odd(4) {
					domain = stateUnknown[NodeRef]("span-part-unavailable")
				}
			}
			pitch := []string{"60", "62", "62", "64"}[rng.Intn(4)]
			if odd(15) {
				pitch = ""
			}
			count := 1
			if !plain || rng.Intn(3) == 0 {
				count += rng.Intn(3)
			}
			for n := count; n > 0; n-- {
				u := tiedUnit{note: note, domain: domain, anchor: !odd(25), implied: rng.Intn(4) != 0, pitch: pitch, complete: Evidence[bool]{State: EvidenceKnown, Value: true}}
				roll := rng.Intn(20)
				if plain {
					roll = 19
				}
				switch roll {
				case 0:
					u.role = known("continue")
				case 1:
					u.role = known("let-ring")
				case 2:
					u.role = stateUnknown[string]("role-unavailable")
				case 3:
					u.role = known("odd")
				default:
					u.role = known([]string{"start", "stop"}[rng.Intn(2)])
				}
				u.number = Evidence[Atomic]{State: EvidenceAbsent}
				if !u.implied {
					u.number = spanInteger(1 + rng.Intn(2))
					if odd(20) {
						u.number = stateUnknown[Atomic]("number-unavailable")
					}
				}
				if odd(30) {
					u.complete = stateUnknown[bool]("incomplete")
				}
				units = append(units, u)
			}
		}
		// A gap of a domain, or of none, as an incomplete node leaves it.
		var gaps []NodeRef
		if !plain {
			switch rng.Intn(25) {
			case 0:
				gaps = append(gaps, p1)
			case 1:
				gaps = append(gaps, NodeRef{})
			}
		}
		build := func() []*spanEndpoint {
			es := make([]*spanEndpoint, len(units))
			for i, u := range units {
				es[i] = u.endpoint(s.documentID, i)
			}
			return es
		}

		batch := build()
		for _, e := range batch {
			for _, gap := range gaps {
				if gap == (NodeRef{}) || e.payload.Domain.State == EvidenceKnown && e.payload.Domain.Value == gap {
					e.complete = timelineMergeAvailability(e.complete, stateUnknown[bool]("gap"))
				}
			}
		}
		if err := spanLocalTiePairs(ctx, s, batch); err != nil {
			t.Fatal(err)
		}
		if err := associateSpanEndpoints(ctx, s, batch, nil); err != nil {
			t.Fatal(err)
		}
		byRef := map[NodeRef]*spanEndpoint{}
		for _, e := range batch {
			byRef[e.ref] = e
		}
		want := int64(0)
		for _, e := range batch {
			if e.payload.Association.State != EvidenceKnown || len(e.payload.Members) != 2 || e.payload.Members[0] != e.ref {
				continue
			}
			stop := byRef[e.payload.Members[1]]
			if e.payload.Anchor.State == EvidenceKnown && stop.payload.Anchor.State == EvidenceKnown && e.payload.Anchor.Value != stop.payload.Anchor.Value {
				want++
			}
		}

		streamed := build()
		for _, gap := range gaps {
			k.collector.windowGap(r, gap, stateUnknown[bool]("gap"))
		}
		for from := 0; from < len(streamed); {
			to := from + 1
			// A window ends between two notes.
			for to < len(streamed) && (units[to].note == units[to-1].note || rng.Intn(3) != 0) {
				to++
			}
			k.countTied(r, streamed[from:to])
			if r.err != nil {
				t.Fatal(r.err)
			}
			from = to
		}
		got := k.tiedPairs()
		if got != want {
			t.Fatalf("seed %d: windows count %d pairs, association establishes %d; endpoints %+v, gaps %v", seed, got, want, units, gaps)
		}
		if want != 0 {
			paired++
		} else if len(k.tied) != 0 {
			withheld++
		}
		if k.tiedBytes == 0 && (len(k.tied) != 0 || len(k.tiedPoisoned) != 0) {
			t.Fatalf("seed %d: tied runs are not charged", seed)
		}
	}
	if paired < 500 || withheld < 500 {
		t.Fatalf("%d documents with pairs, %d with runs but no pair", paired, withheld)
	}
}

// The window's index of tied starts is scratch: charged while the window is
// counted, so a budget refuses a window of too many, and released after it.
func TestWindowSpansTiedStartsAreCharged(t *testing.T) {
	known := func(v string) Evidence[string] { return Evidence[string]{State: EvidenceKnown, Value: v} }
	build := func(r *contextReader, notes int) []*spanEndpoint {
		domain := Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{r.s.documentID, 501}}
		var es []*spanEndpoint
		for _, role := range []string{"start", "stop"} {
			for note := 0; note < notes; note++ {
				u := tiedUnit{note: note + len(es), role: known(role), domain: domain, anchor: true, implied: true, pitch: "60", complete: Evidence[bool]{State: EvidenceKnown, Value: true}}
				es = append(es, u.endpoint(r.s.documentID, len(es)))
			}
		}
		return es
	}
	r, k := spanWindowUnit(t)
	s := r.s
	before, _ := s.account.Usage(BudgetBufferedBytes)
	k.countTied(r, build(r, 2000))
	if r.err != nil {
		t.Fatal(r.err)
	}
	after, _ := s.account.Usage(BudgetBufferedBytes)
	if left := after - before - k.tiedBytes; left != 0 || k.tiedBytes == 0 {
		t.Fatalf("a counted window leaves %d bytes of scratch charged beside %d of tied runs", left, k.tiedBytes)
	}

	r, k = spanWindowUnit(t)
	s = r.s
	limit, _ := s.account.Limit(BudgetBufferedBytes)
	used, _ := s.account.Usage(BudgetBufferedBytes)
	// Room for the run and a few hundred starts, not for two thousand.
	if err := s.account.Charge(BudgetBufferedBytes, limit-used-100_000); err != nil {
		t.Fatal(err)
	}
	k.countTied(r, build(r, 2000))
	var refused *ResourceError
	if !errors.As(r.err, &refused) || refused.Kind != BudgetBufferedBytes {
		t.Fatalf("two thousand tied starts within 100 KB: %v", r.err)
	}
	if now, _ := s.account.Usage(BudgetBufferedBytes); now != limit-100_000+k.tiedBytes {
		t.Fatalf("a refused window leaves %d bytes of scratch charged", now-(limit-100_000)-k.tiedBytes)
	}
}

// A counting window lets its endpoints go with their list: nothing of a
// window full of endpoints is left for the windows after it.
func TestWindowSpansCountingDiscardsTheList(t *testing.T) {
	r, k := spanWindowUnit(t)
	s := r.s
	s.contextFamilies = 0
	s.spansStash = k
	defer func() { s.spansStash = nil }()
	c := k.collector
	note := spanUnitNode(r, spanRole_note, nil)
	for i := 0; i < 100; i++ {
		slur := spanUnitNode(r, spanRole_slur, note)
		e := &spanEndpoint{ref: NodeRef{s.documentID, slur.header.ID}, order: uint64(i), complete: Evidence[bool]{State: EvidenceKnown, Value: true}}
		e.payload.Family = SpanSlur
		e.payload.Identity = SpanIdentityNumber
		e.payload.Role = Evidence[string]{State: EvidenceKnown, Value: "start"}
		e.payload.Number = spanInteger(1)
		e.payload.Anchor = Evidence[NodeRef]{State: EvidenceKnown, Value: NodeRef{s.documentID, note.header.ID}}
		e.payload.Domain = stateUnknown[NodeRef]("span-part-unavailable")
		e.payload.Position = stateUnknown[SpanPosition]("span-timeline-position-unavailable")
		e.payload.Association = stateUnknown[bool]("span-correspondence-unresolved")
		c.endpoints = append(c.endpoints, e)
	}
	k.countAndDiscard(r)
	if r.err != nil {
		t.Fatal(r.err)
	}
	// One result each, and the order on one note for all but the first.
	if c.endpoints != nil || k.counted != 100 || k.pendingFacts != 199 || len(k.records) != 0 {
		t.Fatalf("list of %d (capacity %d), %d counted, %d facts, %d records kept", len(c.endpoints), cap(c.endpoints), k.counted, k.pendingFacts, len(k.records))
	}
}
