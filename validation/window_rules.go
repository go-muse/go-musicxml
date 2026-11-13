package validation

import (
	"context"
	"math"
	"sort"
)

// Under the windowed strategy the rules of a measure window's nodes whose
// plans read only the node and its bound subtree are evaluated when the
// window closes and is bound, instead of when the root closes. Each is
// evaluated as the root-close pass evaluates it: not final, in document
// order, with the finding key of its node. An evaluation that would reach a
// node outside the window, or a rule of another plan kind, waits for the
// root to close. The root can still never close (truncated input, a
// rejected event, a limit), and then no rule of the root-close pass is
// evaluated: the window's outcomes and findings are kept aside and recorded
// only when the root closes, and the decisions it cached are forgotten when
// it does not.
func windowRuleKind(kind planKind) bool {
	switch kind {
	case planChildren, planLocal, planDictionary, planTimeOnlyOrder, planFigureNumberNumeric, planBendReleaseSign:
		return true
	}
	return false
}

// windowEvaluation is the window whose rules are being evaluated, as the
// range of node identities it covers.
type windowEvaluation struct {
	first, last NodeID
}

func (w *windowEvaluation) contains(id NodeID) bool {
	return id >= w.first && id <= w.last
}

// evaluateWindowRules evaluates the window's node rules, after the window
// was bound.
func (s *Session) evaluateWindowRules(ctx context.Context, w evaluationWindow) error {
	if s.strategy != strategyWindowed || windowRulesOff || s.catalog == nil || w.end <= w.first {
		return nil
	}
	ids := s.sequence[w.first:w.end]
	// Node identities follow document order; a window is one contiguous run.
	if !sort.SliceIsSorted(ids, func(i, j int) bool { return ids[i] < ids[j] }) {
		return nil
	}
	if len(s.windowKept.windows) == cap(s.windowKept.windows) {
		grown := max(16, 2*cap(s.windowKept.windows))
		if err := s.retain(int64(grown) * windowRangeBytes); err != nil {
			return err
		}
		next := make([][2]int, len(s.windowKept.windows), grown)
		copy(next, s.windowKept.windows)
		s.releaseFactBytes(int64(cap(s.windowKept.windows)) * windowRangeBytes)
		s.windowKept.windows = next
	}
	s.windowKept.windows = append(s.windowKept.windows, [2]int{w.first, w.end})
	s.windowEvaluating = &windowEvaluation{first: ids[0], last: ids[len(ids)-1]}
	defer func() { s.windowEvaluating = nil }()
	keys := s.catalog.orders[s.profile.ID]
	for i, id := range ids {
		n := s.node(id)
		for k, key := range keys {
			if !windowRuleKind(s.catalog.plans[s.catalog.rules[key].Predicate].kind) {
				continue
			}
			s.accumulator.at = nodeRuleKey(w.first+i, k, len(keys))
			if err := s.evaluate(ctx, n, key, false); err != nil {
				return err
			}
		}
	}
	return nil
}

// nodeRuleKey is the finding key of the k-th rule of the profile's order
// evaluated on the node at index i of the sequence: by node, then by rule,
// as the root-close pass evaluates them.
func nodeRuleKey(i, k, rules int) findingKey {
	return findingKey{stage: stageNodeRules, trigger: int64(i)*int64(rules) + int64(k)}
}

// windowRuleDeferred reports whether a rule evaluation must wait for the
// root to close: during a window's evaluation, a rule of another plan kind
// or a node outside the window.
func (s *Session) windowRuleDeferred(n *factNode, kind planKind) bool {
	w := s.windowEvaluating
	return w != nil && (!windowRuleKind(kind) || !w.contains(n.header.ID))
}

// windowKeptCount is the number of outcomes windows kept aside so far.
func (s *Session) windowKeptCount() int {
	return int(s.windowKept.pass+s.windowKept.notApplicable) + len(s.windowKept.records)
}

// windowRecord is an outcome other than pass or not applicable that a
// window's evaluation recorded, kept aside with its finding key until the
// root closes.
type windowRecord struct {
	rule     RuleKey
	node     NodeID
	decision Decision
	at       findingKey
	charged  int64
}

// windowOutcomes is what windows kept aside until the root closes: the
// normative passes and exclusions they counted, which carry nothing but
// their count, the other outcomes whole, and the windows themselves, whose
// decisions are forgotten if the root never closes.
type windowOutcomes struct {
	pass, notApplicable uint64
	records             []windowRecord
	windows             [][2]int
}

// stashWindowDecision keeps aside the outcome of a decision a window's
// evaluation recorded.
func (s *Session) stashWindowDecision(r RuleDefinition, n *factNode, d Decision) error {
	if d.Outcome == OutcomePass || d.Outcome == OutcomeNotApplicable {
		if r.Category == CategoryNormative && r.Class == ClassMandatory && r.Role == RolePredicate {
			if d.Outcome == OutcomePass {
				s.windowKept.pass++
			} else {
				s.windowKept.notApplicable++
			}
		}
		return nil
	}
	size := reportStorageSize{limit: math.MaxInt64}
	size.decision(d)
	charge := windowRecordBytes + size.total
	if err := s.retain(charge); err != nil {
		return err
	}
	s.windowKept.records = append(s.windowKept.records, windowRecord{rule: r.Key, node: n.header.ID, decision: cloneDecision(d), at: s.accumulator.at, charged: charge})
	return nil
}

// windowRecordBytes charges a kept record and its share of the slice's
// growth.
const windowRecordBytes int64 = 256

// confirmWindowDecisions records the outcomes and findings windows kept
// aside, with their finding keys, once the root closed and bound.
func (s *Session) confirmWindowDecisions() error {
	kept := s.windowKept
	s.windowKept = windowOutcomes{}
	s.accumulator.report.Counts.Pass += kept.pass
	s.accumulator.report.Counts.NotApplicable += kept.notApplicable
	for _, rec := range kept.records {
		r := s.catalog.rules[rec.rule]
		s.accumulator.at = rec.at
		if err := s.accumulator.recordOutcome(r, rec.decision); err != nil {
			return err
		}
		if err := s.accumulator.storeDiagnostic(r, rec.decision, s.node(rec.node).location()); err != nil {
			return err
		}
		s.releaseFactBytes(rec.charged)
	}
	s.releaseFactBytes(int64(cap(kept.windows)) * windowRangeBytes)
	return nil
}

// windowRangeBytes charges one evaluated window's range.
const windowRangeBytes int64 = 32

// forgetWindowDecisions drops what windows kept aside and the decisions of
// their rules when the root never closed, so that Finish evaluates them as
// the buffered engine does.
func (s *Session) forgetWindowDecisions() {
	kept := s.windowKept
	s.windowKept = windowOutcomes{}
	keys := s.catalog.orders[s.profile.ID]
	for _, w := range kept.windows {
		for _, id := range s.sequence[w[0]:w[1]] {
			for _, key := range keys {
				if !windowRuleKind(s.catalog.plans[s.catalog.rules[key].Predicate].kind) {
					continue
				}
				if bits := s.successful[key]; bits != nil && bits.get(id) != 0 {
					_ = bits.set(s, id, 0)
				}
				delete(s.decisions, decisionID{key, id})
			}
		}
	}
	for _, rec := range kept.records {
		s.releaseFactBytes(rec.charged)
	}
	s.releaseFactBytes(int64(cap(kept.windows)) * windowRangeBytes)
}

// windowRulesOff is a test switch only: it evaluates every rule when the root
// closes under the windowed strategy too.
var windowRulesOff bool

func setWindowRulesOff(off bool) bool {
	previous := windowRulesOff
	windowRulesOff = off
	return previous
}

// windowRulesObserver is a test observation point only: it receives, for
// each measure window that closed, whether its node rules were evaluated
// then (they wait for the root when the window was not bound or its binding
// proved a failure), and the decisions windows have kept aside so far.
var windowRulesObserver func(evaluated bool, decisions int)

func setWindowRulesObserver(observer func(bool, int)) func(bool, int) {
	previous := windowRulesObserver
	windowRulesObserver = observer
	return previous
}
