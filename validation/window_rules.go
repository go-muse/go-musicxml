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
// it does not. Dictionary and bend-release-sign rules are not evaluated
// there: they wait for the document whatever the window holds.
func windowRuleKind(kind planKind) bool {
	switch kind {
	case planChildren, planLocal, planTimeOnlyOrder, planFigureNumberNumeric:
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
	rules, count := s.windowRules()
	for i, id := range ids {
		n := s.node(id)
		for _, rule := range rules {
			s.accumulator.at = nodeRuleKey(w.first+i, rule.order, count)
			if err := s.evaluate(ctx, n, rule.key, false); err != nil {
				return err
			}
		}
	}
	return nil
}

// windowRule is a rule of the profile's order that windows evaluate, with
// its place in that order.
type windowRule struct {
	key   RuleKey
	order int
}

// windowRules are the profile's rules that windows evaluate, found once
// per session, and the number of rules in the profile's order.
func (s *Session) windowRules() ([]windowRule, int) {
	keys := s.catalog.orders[s.profile.ID]
	if !s.windowRulesIndexed {
		s.windowRulesIndexed = true
		for k, key := range keys {
			if windowRuleKind(s.catalog.plans[s.catalog.rules[key].Predicate].kind) {
				s.windowRuleIndex = append(s.windowRuleIndex, windowRule{key: key, order: k})
			}
		}
	}
	return s.windowRuleIndex, len(keys)
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
	// Every reader bounds the expected and actual previews; keep them bounded.
	d.Expected = reportBoundedValue(d.Expected, reportValueBytes)
	d.Actual = reportBoundedValue(d.Actual, reportValueBytes)
	size := reportStorageSize{limit: math.MaxInt64}
	size.decision(d)
	if err := s.retain(size.total); err != nil {
		return err
	}
	records := s.windowKept.records
	if len(records) == cap(records) {
		// The new array is charged whole before it is made; the old one is
		// released once replaced.
		grown := max(16, 2*cap(records))
		if err := s.retain(int64(grown) * windowRecordBytes); err != nil {
			s.releaseFactBytes(size.total)
			return err
		}
		next := make([]windowRecord, len(records), grown)
		copy(next, records)
		s.releaseFactBytes(int64(cap(records)) * windowRecordBytes)
		records = next
	}
	s.windowKept.records = append(records, windowRecord{rule: r.Key, node: n.header.ID, decision: cloneDecision(d), at: s.accumulator.at, charged: size.total})
	return nil
}

// windowRecordBytes charges one slot of the kept records' array, at least
// the record's size (TestWindowRecordWidth).
const windowRecordBytes int64 = 224

// confirmWindowDecisions records the outcomes and findings windows kept
// aside, with their finding keys, once the root closed and bound.
func (s *Session) confirmWindowDecisions() error {
	kept := s.windowKept
	s.windowKept = windowOutcomes{}
	s.accumulator.report.Counts.Pass += kept.pass
	s.accumulator.report.Counts.NotApplicable += kept.notApplicable
	// The kept records stay charged until they are all recorded: they are
	// live while the accumulator stores their findings.
	defer s.releaseWindowKept(kept)
	for _, rec := range kept.records {
		r := s.catalog.rules[rec.rule]
		s.accumulator.at = rec.at
		if err := s.accumulator.recordOutcome(r, rec.decision); err != nil {
			return err
		}
		if err := s.accumulator.storeDiagnostic(r, rec.decision, s.node(rec.node).location()); err != nil {
			return err
		}
	}
	return nil
}

// releaseWindowKept releases what kept's records, their array and the
// window ranges charged.
func (s *Session) releaseWindowKept(kept windowOutcomes) {
	for _, rec := range kept.records {
		s.releaseFactBytes(rec.charged)
	}
	s.releaseFactBytes(int64(cap(kept.records))*windowRecordBytes + int64(cap(kept.windows))*windowRangeBytes)
}

// windowRangeBytes charges one evaluated window's range.
const windowRangeBytes int64 = 32

// forgetWindowDecisions drops what windows kept aside, releasing its
// charge, and the decisions of their rules when the root never closed, so
// that Finish evaluates them as the buffered engine does. What caching those
// decisions charged stays charged.
func (s *Session) forgetWindowDecisions() {
	kept := s.windowKept
	s.windowKept = windowOutcomes{}
	rules, _ := s.windowRules()
	for _, w := range kept.windows {
		for _, id := range s.sequence[w[0]:w[1]] {
			for _, rule := range rules {
				if bits := s.successful[rule.key]; bits != nil && bits.get(id) != 0 {
					_ = bits.set(s, id, 0)
				}
				delete(s.decisions, decisionID{rule.key, id})
			}
		}
	}
	s.releaseWindowKept(kept)
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
