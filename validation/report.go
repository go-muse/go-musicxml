package validation

import (
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
	"unsafe"
)

const (
	reportValueBytes        = 4096
	reportTerminalBytes     = 256
	reportFormatFieldBytes  = 4096
	reportFormatBytes       = 1 << 20
	reportFormatItems       = 1000
	reportFormatNestedItems = 64
	reportTruncatedMarker   = "\n[report output truncated]\n"
)

// Satisfied reports whether every independent acceptance axis succeeded. An
// unrequested, incomplete, contradictory, or zero report is never satisfied.
// Diagnostic storage truncation alone does not change acceptance. Report is a
// caller-editable value, not a cryptographic certificate of assessment.
func (r Report) Satisfied() bool {
	return r.Requested && r.Profile.ID != "" && r.Target == r.Profile.Target &&
		r.InputComplete && reportSyntaxComplete(r.Target, r.Syntax) &&
		r.Conformance == ConformanceConformant && r.AssessmentComplete &&
		r.BlockingProblems == 0 && r.Terminal == nil && r.Counts.Fail == 0 &&
		r.Counts.Unknown == 0 && r.Counts.Unsupported == 0 && r.Counts.Blocked == 0
}

func reportSyntaxComplete(target Target, syntax SyntaxState) bool {
	switch target {
	case TargetSource:
		return syntax == SyntaxWellFormed
	case TargetModel, TargetPackage:
		return syntax == SyntaxNotAssessed
	default:
		return false
	}
}

// Format writes a bounded offline English explanation of the report, including
// rule identities, stored source URLs, and qualified locations. Untrusted text
// is escaped, each field is limited to 4096 encoded bytes, and total output is
// limited to 1 MiB. At most 1000 findings/rule records and 64 nested source or
// related-location records per finding are rendered. Any rendering omission is
// explicitly marked. These independent limits also apply to fabricated reports.
// Format does not fetch URLs or invoke localization callbacks. Writer errors,
// including short writes without an error, are returned to the caller.
func (r Report) Format(w io.Writer) error {
	return r.FormatLocalized(w, nil)
}

// String returns the text that Format writes, within the same limits.
func (r Report) String() string {
	var out strings.Builder
	_ = r.Format(&out)
	return out.String()
}

// FormatLocalized formats a report using optional per-rule replacement text.
// Only Title, Explanation, and Message are translated; semantic metadata and
// source URLs are unchanged. Missing, empty, oversized, invalid UTF-8, or control-
// containing translations fall back to the report's bounded English text.
// Output and writer-error guarantees are identical to Format.
func (r Report) FormatLocalized(w io.Writer, translations map[RuleKey]RuleText) error {
	if nilWriter(w) {
		return &ConfigurationError{Field: "writer", Reason: "must not be nil"}
	}
	out := reportRenderer{}
	out.add("Validation report\n")
	out.add("profile=" + reportField(string(r.Profile.ID)) + " target=" + reportField(string(r.Target)) + " requested=" + strconv.FormatBool(r.Requested) + "\n")
	out.add("syntax=" + reportField(string(r.Syntax)) + " conformance=" + reportField(string(r.Conformance)) + " assessment-complete=" + strconv.FormatBool(r.AssessmentComplete) + " input-complete=" + strconv.FormatBool(r.InputComplete) + "\n")
	out.add(fmt.Sprintf("counts: pass=%d fail=%d not-applicable=%d unknown=%d unsupported=%d blocked=%d\nblocking-problems=%d model-failures=%d diagnostics-truncated=%t\n", r.Counts.Pass, r.Counts.Fail, r.Counts.NotApplicable, r.Counts.Unknown, r.Counts.Unsupported, r.Counts.Blocked, r.BlockingProblems, r.ModelFailures, r.DiagnosticsTruncated))
	if r.Terminal != nil {
		out.add("terminal: category=" + reportField(string(r.Terminal.Category)) + " reason=" + reportField(string(r.Terminal.Reason)) + "\n")
	}
	// Bound both indexing work and hashing, even when Report was fabricated by a
	// caller rather than produced by a budgeted session.
	rules := make(map[RuleKey]RuleDefinition, min(len(r.Rules), reportFormatItems))
	for i := 0; i < len(r.Rules) && i < reportFormatItems; i++ {
		if len(r.Rules[i].Key.ID) <= reportFormatFieldBytes {
			if _, exists := rules[r.Rules[i].Key]; !exists {
				rules[r.Rules[i].Key] = r.Rules[i]
			}
		}
	}
	if len(r.Rules) > reportFormatItems {
		out.truncated = true
	}
	for i := 0; i < len(r.Findings) && i < reportFormatItems && !out.full; i++ {
		finding := r.Findings[i]
		out.add(fmt.Sprintf("\nFinding %d: ", i+1) + "rule=" + reportField(string(finding.Rule.ID)) + " revision=" + strconv.FormatUint(uint64(finding.Rule.Revision), 10) + " outcome=" + reportField(string(finding.Outcome)) + " category=" + reportField(string(finding.Category)) + " reason=" + reportField(string(finding.Reason)) + "\n")
		out.location("location", finding.Location)
		out.add("subject: document=" + reportField(finding.Subject.DocumentID) + " node=" + strconv.FormatUint(uint64(finding.Subject.Node), 10) + "\n")
		if finding.Cause != nil {
			cause := finding.Cause
			out.add("cause: rule=" + reportField(string(cause.Rule.ID)) + " revision=" + strconv.FormatUint(uint64(cause.Rule.Revision), 10) + " document=" + reportField(cause.Subject.DocumentID) + " node=" + strconv.FormatUint(uint64(cause.Subject.Node), 10) + "\n")
		}
		out.add("expected=" + reportField(finding.Expected) + " actual=" + reportField(finding.Actual) + "\n")
		for j := 0; j < len(finding.Related) && j < reportFormatNestedItems && !out.full; j++ {
			out.location("related", finding.Related[j])
		}
		if len(finding.Related) > reportFormatNestedItems {
			out.truncated = true
		}
		var rule RuleDefinition
		var found bool
		if len(finding.Rule.ID) <= reportFormatFieldBytes {
			rule, found = rules[finding.Rule]
		}
		if !found {
			out.add("explanation=\"Rule metadata unavailable.\"\n")
			continue
		}
		text := rule.Text
		if translation, ok := translations[rule.Key]; ok && validReportTranslation(translation) {
			text = translation
		}
		out.add("title=" + reportField(text.Title) + "\nexplanation=" + reportField(text.Explanation) + "\nmessage=" + reportField(text.Message) + "\n")
		for j := 0; j < len(rule.Sources) && j < reportFormatNestedItems && !out.full; j++ {
			source := rule.Sources[j]
			out.add("source: URL=" + reportField(source.URL) + " locator=" + reportField(source.Locator) + "\n")
			for k := 0; k < len(source.IDs) && k < reportFormatNestedItems && !out.full; k++ {
				out.add("source-id=" + reportField(source.IDs[k]) + "\n")
			}
			if len(source.IDs) > reportFormatNestedItems {
				out.truncated = true
			}
		}
		if len(rule.Sources) > reportFormatNestedItems {
			out.truncated = true
		}
	}
	if len(r.Findings) > reportFormatItems {
		out.truncated = true
	}
	if out.truncated {
		out.text.WriteString(reportTruncatedMarker)
	}
	output := out.text.String()
	n, err := io.WriteString(w, output)
	if err != nil {
		return err
	}
	if n != len(output) {
		return io.ErrShortWrite
	}
	return nil
}

type reportRenderer struct {
	text            strings.Builder
	full, truncated bool
}

func (r *reportRenderer) add(text string) {
	if r.full {
		return
	}
	remaining := reportFormatBytes - len(reportTruncatedMarker) - r.text.Len()
	if len(text) > remaining {
		text = reportUTF8Prefix(text, remaining)
		r.full, r.truncated = true, true
	}
	r.text.WriteString(text)
}

func (r *reportRenderer) location(label string, location Location) {
	r.add(label + ": target=" + reportField(string(location.Target)) + " document=" + reportField(location.DocumentID) + " entry=" + reportField(location.Entry) + " path=" + reportField(location.Path))
	// A model/package position cannot reconstruct original XML coordinates. A
	// source position without an identified coordinate stream is also omitted.
	if p := location.Position; location.Target == TargetSource && p != nil && p.CoordinateSpace != "" {
		r.add(" coordinates=" + reportField(p.CoordinateSpace))
		if p.Line > 0 {
			r.add(" line=" + strconv.Itoa(p.Line))
		}
		if p.Column > 0 {
			r.add(" column=" + strconv.Itoa(p.Column))
		}
		if p.ByteOffset >= 0 {
			r.add(" byte=" + strconv.FormatInt(p.ByteOffset, 10))
		}
	}
	r.add("\n")
}

func validReportTranslation(text RuleText) bool {
	for _, field := range []string{text.Title, text.Explanation, text.Message} {
		if len(field) == 0 || len(field) > reportFormatFieldBytes || !utf8.ValidString(field) || strings.TrimSpace(field) == "" {
			return false
		}
		for _, ch := range field {
			if unicode.IsControl(ch) || unicode.Is(unicode.Cf, ch) || ch == '\u2028' || ch == '\u2029' {
				return false
			}
		}
	}
	return true
}

func reportField(value string) string {
	const marker = "[truncated]"
	var out strings.Builder
	out.WriteByte('"')
	for len(value) != 0 {
		ch, size := utf8.DecodeRuneInString(value)
		var encoded string
		switch {
		case ch == utf8.RuneError && size == 1:
			encoded = fmt.Sprintf("\\x%02x", value[0])
		case ch == '\\':
			encoded = "\\\\"
		case ch == '"':
			encoded = "\\\""
		case unicode.IsControl(ch) || unicode.Is(unicode.Cf, ch) || ch == '\u2028' || ch == '\u2029':
			quoted := strconv.QuoteRuneToASCII(ch)
			encoded = quoted[1 : len(quoted)-1]
		default:
			encoded = value[:size]
		}
		if out.Len()+len(encoded)+len(marker)+1 > reportFormatFieldBytes {
			out.WriteString(marker)
			break
		}
		out.WriteString(encoded)
		value = value[size:]
	}
	out.WriteByte('"')
	return out.String()
}

func reportUTF8Prefix(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	for limit > 0 && !utf8.RuneStart(value[limit]) {
		limit--
	}
	return value[:limit]
}

func reportBoundedValue(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	const marker = "[truncated]"
	return reportUTF8Prefix(value, limit-len(marker)) + marker
}

// findingStage names the part of a session that produced a finding, in the
// order the buffered engine runs them.
type findingStage uint8

const (
	stageInput findingStage = iota
	stageSchema
	stageSchemaFinal
	stagePhases
	stageSummaries
)

// findingKey orders a stored finding by where the buffered engine produces
// it, independently of when an engine evaluates it: the stage, the evaluation
// phase, the trigger (the input event, or the node in document order) and the
// order within that trigger. The windowed engine sorts by it to reproduce the
// buffered report.
type findingKey struct {
	stage   findingStage
	phase   int
	trigger int64
	order   int
}

func (k findingKey) less(other findingKey) bool {
	if k.stage != other.stage {
		return k.stage < other.stage
	}
	if k.phase != other.phase {
		return k.phase < other.phase
	}
	if k.trigger != other.trigger {
		return k.trigger < other.trigger
	}
	return k.order < other.order
}

// findingKeyObserver receives every finished report's finding keys. It is an
// unsynchronized package variable: only serial tests set it.
var findingKeyObserver func(keys []findingKey)

// setFindingKeyObserverOrdered observes, for every finished report, whether its
// findings were stored in strictly increasing key order and how many there
// were; nil stops observing.
func setFindingKeyObserverOrdered(observer func(ordered bool, findings int)) {
	if observer == nil {
		findingKeyObserver = nil
		return
	}
	findingKeyObserver = func(keys []findingKey) {
		ordered := true
		for i := 1; i < len(keys); i++ {
			if !keys[i-1].less(keys[i]) {
				ordered = false
			}
		}
		observer(ordered, len(keys))
	}
}

// reportAccumulator keeps authoritative assessment axes independently of the
// diagnostic storage budget. It belongs to one single-goroutine session.
type reportAccumulator struct {
	report           Report
	account          *Account
	rules            map[RuleKey]bool
	normativeUnknown bool
	unfinished       bool
	// at is the position the session is evaluating; keys holds one key per
	// stored finding. The keys are not charged separately: each stored finding
	// is charged to the diagnostic byte quota for more than its key.
	at   findingKey
	keys []findingKey
	// deferred keeps findings by key and stores them in Finish (see
	// deferDiagnostic). pending holds the candidates in key order, charged to
	// the buffer for pendingBytes while they wait; cutoff is the first key the
	// buffered accumulator would not store, once one is known.
	deferred     bool
	pending      []pendingFinding
	pendingBytes int64
	cutoff       *findingKey
	// lastArrival is the key of the latest finding, which numbers findings at
	// one position in the order they arrive.
	lastArrival findingKey
	arrived     bool
	// simulated counts the leading candidates whose buffered storage simSize,
	// simCount and simRules already account for. stale means a candidate
	// arrived before that prefix: the cutoff is then only an upper bound until
	// the next full simulation.
	simulated int
	simSize   int64
	simCount  int64
	simRules  map[RuleKey]bool
	stale     bool
	// capacityBytes is the charge for the pending array's capacity.
	capacityBytes int64
	// holding keeps outcomes and findings in held instead of recording
	// them, charged to the buffer for heldBytes, until commitHeld records
	// them or dropHeld discards them. Binding a window holds its proved
	// failures until binding the whole document confirms them.
	holding   bool
	held      []heldRecord
	heldBytes int64
}

type heldKind uint8

const (
	heldComposite heldKind = iota
	heldBlocking
	heldFinding
)

// heldRecord is one held outcome (recordCompositeOutcome), blocking
// incompleteness (blockingIncomplete) or finding (storeDiagnostic, with the
// key it arrived with).
type heldRecord struct {
	kind     heldKind
	rule     RuleDefinition
	decision Decision
	location Location
	complete bool
	key      findingKey
}

// heldRecordBytes is the charge of one held record before its strings.
var heldRecordBytes = int64(unsafe.Sizeof(heldRecord{})) + pendingRuleBytes

// hold keeps one record, charging it first.
func (a *reportAccumulator) hold(record heldRecord) error {
	size := reportStorageSize{limit: math.MaxInt64}
	size.decision(record.decision)
	size.location(record.location)
	charge := heldRecordBytes + size.total
	if err := a.account.Charge(BudgetBufferedBytes, charge); err != nil {
		return err
	}
	a.heldBytes += charge
	record.decision = cloneDecision(record.decision)
	record.location = cloneLocation(record.location)
	a.held = append(a.held, record)
	return nil
}

// commitHeld records the held outcomes and findings in the order they
// arrived, each finding at its own key.
func (a *reportAccumulator) commitHeld() error {
	held := a.held
	a.held, a.holding = nil, false
	defer a.releaseHeld()
	for _, record := range held {
		switch record.kind {
		case heldComposite:
			if err := a.recordCompositeOutcome(record.rule, record.decision, record.complete); err != nil {
				return err
			}
		case heldBlocking:
			a.blockingIncomplete()
		case heldFinding:
			if err := a.deferAt(record.rule, record.decision, record.location, record.key); err != nil {
				return err
			}
		}
	}
	return nil
}

// dropHeld discards the held outcomes and findings.
func (a *reportAccumulator) dropHeld() {
	a.held, a.holding = nil, false
	a.releaseHeld()
}

func (a *reportAccumulator) releaseHeld() {
	if a.heldBytes > 0 {
		_ = a.account.Release(BudgetBufferedBytes, a.heldBytes)
		a.heldBytes = 0
	}
}

// blockingIncomplete records that a proved failure turned out incomplete.
func (a *reportAccumulator) blockingIncomplete() error {
	if a.holding {
		return a.hold(heldRecord{kind: heldBlocking})
	}
	a.incomplete()
	a.report.BlockingProblems++
	return nil
}

// pendingFinding is a finding that the deferred accumulator will store if no
// finding before it in key order truncates the report.
type pendingFinding struct {
	key     findingKey
	finding Finding
	rule    *RuleDefinition
	charged int64
}

// pendingFindingSlotBytes is one slot of the pending array; a candidate's rule
// copy is charged at pendingRuleBytes, the allocator class holding it.
var pendingFindingSlotBytes = int64(unsafe.Sizeof(pendingFinding{}))

const pendingRuleBytes = 256

func newReportAccumulator(profile Profile, account *Account) *reportAccumulator {
	return &reportAccumulator{report: Report{Requested: true, Profile: cloneProfile(profile), Target: profile.Target, AssessmentComplete: true}, account: account, rules: make(map[RuleKey]bool)}
}

func (a *reportAccumulator) record(rule RuleDefinition, decision Decision, location Location) error {
	if err := a.recordOutcome(rule, decision); err != nil {
		return err
	}
	return a.storeDiagnostic(rule, decision, location)
}

// recordCompositeOutcome counts one sealed shared-schema predicate instance.
// Detail findings are stored separately so early proofs precede budgeted detail
// work and later identity details can append without recounting the predicate.
func (a *reportAccumulator) recordCompositeOutcome(rule RuleDefinition, summary Decision, complete bool) error {
	if !complete && (summary.Outcome == OutcomePass || summary.Outcome == OutcomeNotApplicable) {
		return contract("composite result", "incomplete work cannot pass or prove exclusion")
	}
	if complete && (summary.Outcome == OutcomeUnknown || summary.Outcome == OutcomeUnsupported || summary.Outcome == OutcomeBlocked) {
		return contract("composite result", "unresolved work cannot be complete")
	}
	if a.holding {
		return a.hold(heldRecord{kind: heldComposite, rule: rule, decision: summary, complete: complete})
	}
	if err := a.recordOutcome(rule, summary); err != nil {
		return err
	}
	if !complete {
		a.incomplete()
		a.report.BlockingProblems++
	}
	return nil
}

func (a *reportAccumulator) recordOutcome(rule RuleDefinition, decision Decision) error {
	if a.account.Limits().MaxDiagnosticBytes <= 0 {
		return &ConfigurationError{Field: "Account", Reason: "must be created by NewBudgetAccount"}
	}
	switch decision.Outcome {
	case OutcomePass, OutcomeFail, OutcomeNotApplicable, OutcomeUnknown, OutcomeUnsupported, OutcomeBlocked:
	default:
		return &ConfigurationError{Field: "outcome", Reason: "unknown report outcome"}
	}
	normative := rule.Category == CategoryNormative && rule.Class == ClassMandatory && rule.Role == RolePredicate
	optional := rule.Category == CategoryAdvisory || rule.Class == ClassRecommendation || rule.Role == RoleAdvisory
	unresolved := decision.Outcome == OutcomeUnknown || decision.Outcome == OutcomeUnsupported || decision.Outcome == OutcomeBlocked
	if normative {
		switch decision.Outcome {
		case OutcomePass:
			a.report.Counts.Pass++
		case OutcomeFail:
			a.report.Counts.Fail++
		case OutcomeNotApplicable:
			a.report.Counts.NotApplicable++
		case OutcomeUnknown:
			a.report.Counts.Unknown++
		case OutcomeUnsupported:
			a.report.Counts.Unsupported++
		case OutcomeBlocked:
			a.report.Counts.Blocked++
		}
		if unresolved {
			a.normativeUnknown = true
			a.report.AssessmentComplete = false
		}
	} else if !optional {
		if decision.Outcome == OutcomeFail || unresolved {
			a.report.BlockingProblems++
		}
		if decision.Outcome == OutcomeFail && rule.Category == CategoryModel {
			a.report.ModelFailures++
		}
		if unresolved {
			a.report.AssessmentComplete = false
		}
	}
	return nil
}

// nextKey is the key of the next finding at the current position: findings
// at one position keep the order they arrive in.
func (a *reportAccumulator) nextKey() findingKey {
	key := a.at
	var last *findingKey
	switch {
	case a.deferred && a.arrived:
		last = &a.lastArrival
	case !a.deferred && len(a.keys) > 0:
		last = &a.keys[len(a.keys)-1]
	}
	if last != nil && last.stage == key.stage && last.phase == key.phase && last.trigger == key.trigger {
		key.order = last.order + 1
	}
	return key
}

// diagnosticsTruncated reports whether a finding produced now would be
// dropped: work that only adds diagnostic detail may stop once it is.
func (a *reportAccumulator) diagnosticsTruncated() bool {
	if !a.deferred {
		return a.report.DiagnosticsTruncated
	}
	return a.cutoff != nil && !a.nextKey().less(*a.cutoff)
}

// truncateDiagnostics truncates diagnostic storage at the current position:
// findings stored before it are kept, later ones are dropped.
func (a *reportAccumulator) truncateDiagnostics() {
	if !a.deferred {
		a.report.DiagnosticsTruncated = true
		return
	}
	a.setCutoff(a.nextKey())
}

func (a *reportAccumulator) storeDiagnostic(rule RuleDefinition, decision Decision, location Location) error {
	// Pass and proven exclusion are assessment results, not diagnostics. Keep
	// their authoritative counts above, without measuring or storing payloads,
	// consuming diagnostic quotas, or marking diagnostic storage truncated.
	if decision.Outcome == OutcomePass || decision.Outcome == OutcomeNotApplicable {
		return nil
	}
	if a.holding {
		key := a.nextKey()
		a.lastArrival, a.arrived = key, true
		return a.hold(heldRecord{kind: heldFinding, rule: rule, decision: decision, location: location, key: key})
	}
	if a.deferred {
		return a.deferDiagnostic(rule, decision, location)
	}
	if a.report.DiagnosticsTruncated {
		return nil
	}
	// The expected/actual preview is bounded before measuring or cloning. Other
	// retained fields and all nested metadata must fit the diagnostic byte quota.
	decision.Expected = reportBoundedValue(decision.Expected, reportValueBytes)
	decision.Actual = reportBoundedValue(decision.Actual, reportValueBytes)
	includeRule := !a.rules[rule.Key]
	size := reportStorageSize{limit: a.account.Limits().MaxDiagnosticBytes}
	size.decision(decision)
	size.location(location)
	if includeRule {
		size.rule(rule)
	}
	if size.exceeded {
		a.report.DiagnosticsTruncated = true
		return nil
	}
	if err := a.account.chargeDiagnostic(size.total); err != nil {
		if errors.Is(err, ErrResourceLimit) {
			a.report.DiagnosticsTruncated = true
			return nil
		}
		return err
	}
	a.report.Findings = append(a.report.Findings, Finding{Decision: cloneDecision(decision), Category: rule.Category, Location: cloneLocation(location)})
	a.keys = append(a.keys, a.nextKey())
	if includeRule {
		a.report.Rules = append(a.report.Rules, cloneRule(rule))
		a.rules[rule.Key] = true
	}
	return nil
}

// deferDiagnostic keeps a finding by its key instead of storing it now, so an
// engine that evaluates out of document order still reports exactly what the
// buffered engine stores. The candidates stay sorted, and the buffered
// storage of their key-order prefix is simulated against the quotas left on
// the account, which no other session charges while this one is open: the
// first candidate that would not be stored sets the cutoff, and every finding
// at or after it is dropped without being copied or charged. With findings
// arriving in key order the cutoff is exact as soon as truncation happens; an
// earlier finding arriving later can only move it earlier.
func (a *reportAccumulator) deferDiagnostic(rule RuleDefinition, decision Decision, location Location) error {
	key := a.nextKey()
	a.lastArrival, a.arrived = key, true
	return a.deferAt(rule, decision, location, key)
}

// deferAt keeps a finding that arrived with key.
func (a *reportAccumulator) deferAt(rule RuleDefinition, decision Decision, location Location, key findingKey) error {
	if a.cutoff != nil && !key.less(*a.cutoff) {
		return nil
	}
	decision.Expected = reportBoundedValue(decision.Expected, reportValueBytes)
	decision.Actual = reportBoundedValue(decision.Actual, reportValueBytes)
	size := reportStorageSize{limit: a.account.Limits().MaxDiagnosticBytes}
	size.decision(decision)
	size.location(location)
	if size.exceeded {
		a.setCutoff(key)
		return nil
	}
	if err := a.reservePendingSlot(); err != nil {
		return err
	}
	charge := size.total + pendingRuleBytes
	if err := a.account.Charge(BudgetBufferedBytes, charge); err != nil {
		return err
	}
	a.pendingBytes += charge
	owned := rule
	candidate := pendingFinding{key: key, rule: &owned, charged: charge, finding: Finding{Decision: cloneDecision(decision), Category: rule.Category, Location: cloneLocation(location)}}
	// After equal keys, so equal keys keep their arrival order.
	at, _ := slices.BinarySearchFunc(a.pending, key, func(p pendingFinding, k findingKey) int {
		if k.less(p.key) {
			return 1
		}
		return -1
	})
	a.pending = slices.Insert(a.pending, at, candidate)
	if at < a.simulated {
		a.stale = true
	}
	limits := a.account.Limits()
	switch {
	case !a.stale:
		a.simulate()
	case int64(len(a.pending)) > 2*limits.MaxDiagnostics || a.pendingBytes > 2*limits.MaxDiagnosticBytes:
		// Out-of-order arrivals are simulated in batches: until then the
		// cutoff is a valid upper bound and only keeps more candidates.
		a.simulated, a.simSize, a.simCount, a.simRules, a.stale = 0, 0, 0, nil, false
		a.simulate()
	}
	return nil
}

// reservePendingSlot grows the pending array before an insert that needs it,
// charging the new array before it is allocated and releasing the old one
// after the copy.
func (a *reportAccumulator) reservePendingSlot() error {
	if len(a.pending) < cap(a.pending) {
		return nil
	}
	grown := max(8, 2*cap(a.pending))
	charge := int64(grown) * pendingFindingSlotBytes
	if err := a.account.Charge(BudgetBufferedBytes, charge); err != nil {
		return err
	}
	next := make([]pendingFinding, len(a.pending), grown)
	copy(next, a.pending)
	a.pending = next
	_ = a.account.Release(BudgetBufferedBytes, a.capacityBytes)
	a.capacityBytes = charge
	return nil
}

// simulate extends the simulated prefix over the pending candidates with the
// buffered accumulator's sizes and quota checks, and sets the cutoff at the
// first candidate it would not store.
func (a *reportAccumulator) simulate() {
	limits := a.account.Limits()
	usedCount, _ := a.account.Usage(BudgetDiagnostics)
	usedBytes, _ := a.account.Usage(BudgetDiagnosticBytes)
	for a.simulated < len(a.pending) {
		p := &a.pending[a.simulated]
		includeRule := !a.rules[p.rule.Key] && !a.simRules[p.rule.Key]
		size := reportStorageSize{limit: limits.MaxDiagnosticBytes}
		size.decision(p.finding.Decision)
		size.location(p.finding.Location)
		if includeRule {
			size.rule(*p.rule)
		}
		if size.exceeded || 1 > limits.MaxDiagnostics-usedCount-a.simCount || size.total > limits.MaxDiagnosticBytes-usedBytes-a.simSize {
			a.setCutoff(p.key)
			return
		}
		a.simSize += size.total
		a.simCount++
		if includeRule {
			if a.simRules == nil {
				a.simRules = make(map[RuleKey]bool)
			}
			a.simRules[p.rule.Key] = true
		}
		a.simulated++
	}
}

// setCutoff truncates at key, dropping and releasing the candidates at or
// after it.
func (a *reportAccumulator) setCutoff(key findingKey) {
	if a.cutoff != nil && !key.less(*a.cutoff) {
		return
	}
	a.cutoff = &key
	keep := len(a.pending)
	for keep > 0 && !a.pending[keep-1].key.less(key) {
		keep--
	}
	for _, p := range a.pending[keep:] {
		_ = a.account.Release(BudgetBufferedBytes, p.charged)
		a.pendingBytes -= p.charged
	}
	clear(a.pending[keep:])
	a.pending = a.pending[:keep]
	if keep < a.simulated {
		// Dropped candidates were part of the simulated prefix: simulate the
		// remaining ones again, which cannot drop more than the new cutoff.
		a.simulated, a.simSize, a.simCount, a.simRules, a.stale = 0, 0, 0, nil, false
		a.simulate()
	}
}

// storePending stores the deferred candidates in key order with the buffered
// accumulator's sizes, quotas and truncation, and releases their buffer charge.
func (a *reportAccumulator) storePending() error {
	pending := a.pending
	a.pending, a.deferred = nil, false
	_ = a.account.Release(BudgetBufferedBytes, a.pendingBytes+a.capacityBytes)
	a.pendingBytes, a.capacityBytes = 0, 0
	for _, p := range pending {
		if a.report.DiagnosticsTruncated {
			break
		}
		if err := a.storeDiagnosticAt(*p.rule, p.finding, p.key); err != nil {
			return err
		}
	}
	if a.cutoff != nil {
		a.report.DiagnosticsTruncated = true
	}
	return nil
}

// storeDiagnosticAt stores one already bounded and owned finding under its key.
func (a *reportAccumulator) storeDiagnosticAt(rule RuleDefinition, finding Finding, key findingKey) error {
	includeRule := !a.rules[rule.Key]
	size := reportStorageSize{limit: a.account.Limits().MaxDiagnosticBytes}
	size.decision(finding.Decision)
	size.location(finding.Location)
	if includeRule {
		size.rule(rule)
	}
	if size.exceeded {
		a.report.DiagnosticsTruncated = true
		return nil
	}
	if err := a.account.chargeDiagnostic(size.total); err != nil {
		if errors.Is(err, ErrResourceLimit) {
			a.report.DiagnosticsTruncated = true
			return nil
		}
		return err
	}
	a.report.Findings = append(a.report.Findings, finding)
	a.keys = append(a.keys, key)
	if includeRule {
		a.report.Rules = append(a.report.Rules, cloneRule(rule))
		a.rules[rule.Key] = true
	}
	return nil
}

func (a *reportAccumulator) incomplete() { a.unfinished = true; a.report.AssessmentComplete = false }

func (a *reportAccumulator) terminal(_ error, category DiagnosticCategory, reason Code) {
	a.incomplete()
	if a.report.Terminal != nil {
		return
	}
	a.report.BlockingProblems++
	a.report.Terminal = &TerminalSummary{Category: DiagnosticCategory(strings.Clone(reportBoundedValue(string(category), reportTerminalBytes))), Reason: Code(strings.Clone(reportBoundedValue(string(reason), reportTerminalBytes)))}
}

func (a *reportAccumulator) finish(inputComplete bool, syntax SyntaxState) Report {
	if a.deferred {
		// Only an invalid account fails a diagnostic charge other than by its
		// quota, and recordOutcome already rejected one.
		_ = a.storePending()
	}
	if findingKeyObserver != nil {
		findingKeyObserver(a.keys)
	}
	r := a.report
	r.InputComplete, r.Syntax = inputComplete, syntax
	unresolvedInput := !inputComplete || !reportSyntaxComplete(r.Target, syntax)
	r.AssessmentComplete = r.AssessmentComplete && !a.unfinished && !unresolvedInput
	switch {
	case r.Counts.Fail != 0:
		r.Conformance = ConformanceViolated
	case a.normativeUnknown || a.unfinished || unresolvedInput:
		r.Conformance = ConformanceUndetermined
	default:
		r.Conformance = ConformanceConformant
	}
	r.Profile = cloneProfile(r.Profile)
	r.Findings = make([]Finding, len(a.report.Findings))
	for i, finding := range a.report.Findings {
		r.Findings[i] = Finding{Decision: cloneDecision(finding.Decision), Category: finding.Category, Location: cloneLocation(finding.Location)}
	}
	r.Rules = make([]RuleDefinition, len(a.report.Rules))
	for i, rule := range a.report.Rules {
		r.Rules[i] = cloneRule(rule)
	}
	if r.Terminal != nil {
		terminal := *r.Terminal
		r.Terminal = &terminal
	}
	return r
}

// Diagnostic bytes account for logical retained payload plus conservative
// fixed per-record allowances. This is not a measurement of Go allocator or
// garbage-collector overhead. Measurement stops as soon as the cap is exceeded,
// before allocating copies or walking the rest of an oversized nested slice.
type reportStorageSize struct {
	total, limit int64
	exceeded     bool
}

func (s *reportStorageSize) add(n int64) {
	if s.exceeded {
		return
	}
	if n > s.limit-s.total {
		s.exceeded = true
		return
	}
	s.total += n
}
func (s *reportStorageSize) text(values ...string) {
	for _, value := range values {
		s.add(int64(len(value)))
	}
}
func (s *reportStorageSize) key(key RuleKey) { s.add(24); s.text(string(key.ID)) }
func (s *reportStorageSize) location(location Location) {
	s.add(128)
	s.text(string(location.Target), location.DocumentID, location.Entry, location.Path)
	if location.Position != nil {
		s.add(48)
		s.text(location.Position.CoordinateSpace)
	}
}
func (s *reportStorageSize) decision(decision Decision) {
	s.add(192)
	s.key(decision.Rule)
	s.text(decision.Subject.DocumentID, string(decision.Outcome), string(decision.Reason), decision.Expected, decision.Actual)
	if decision.Cause != nil {
		s.add(48)
		s.key(decision.Cause.Rule)
		s.text(decision.Cause.Subject.DocumentID)
	}
	for _, location := range decision.Related {
		if s.exceeded {
			break
		}
		s.location(location)
	}
}
func (s *reportStorageSize) rule(rule RuleDefinition) {
	s.add(384)
	s.key(rule.Key)
	s.text(string(rule.Class), string(rule.Role), string(rule.Category), string(rule.Scope), rule.Predicate, rule.Text.Title, rule.Text.Explanation, rule.Text.Message)
	for _, target := range rule.Targets {
		if s.exceeded {
			break
		}
		s.add(16)
		s.text(string(target))
	}
	if v := rule.Selector.Assembly; v != nil {
		s.add(16)
		s.text(string(*v))
	}
	if v := rule.Selector.Component; v != nil {
		s.add(80)
		s.text(string(v.Assembly), v.Name.Space, v.Name.Local, string(v.Kind), v.Occurrence)
	}
	if v := rule.Selector.Declaration; v != nil {
		s.add(48)
		s.text(string(v.Assembly), v.OwnerOccurrence, v.Occurrence)
	}
	for _, dependency := range rule.Requires {
		if s.exceeded {
			break
		}
		s.add(80)
		s.key(dependency.Rule)
		s.text(string(dependency.Kind), dependency.Key, string(dependency.Relation))
	}
	for _, source := range rule.Sources {
		if s.exceeded {
			break
		}
		s.add(56)
		s.text(source.URL, source.Locator)
		for _, id := range source.IDs {
			if s.exceeded {
				break
			}
			s.add(16)
			s.text(id)
		}
	}
}

// nilWriter reports whether w is nil or a typed nil, such as a nil
// *bytes.Buffer, whose Write would dereference the nil value.
func nilWriter(w io.Writer) bool {
	if w == nil {
		return true
	}
	v := reflect.ValueOf(w)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
