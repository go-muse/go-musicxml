package validation

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
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

// FormatLocalized formats a report using optional per-rule replacement text.
// Only Title, Explanation, and Message are translated; semantic metadata and
// source URLs are unchanged. Missing, empty, oversized, invalid UTF-8, or control-
// containing translations fall back to the report's bounded English text.
// Output and writer-error guarantees are identical to Format.
func (r Report) FormatLocalized(w io.Writer, translations map[RuleKey]RuleText) error {
	if w == nil {
		return &ConfigurationError{Field: "writer", Reason: "must not be nil"}
	}
	out := reportRenderer{}
	out.add("Validation report\n")
	out.add("profile=" + reportField(string(r.Profile.ID)) + " target=" + reportField(string(r.Target)) + " requested=" + strconv.FormatBool(r.Requested) + "\n")
	out.add("syntax=" + reportField(string(r.Syntax)) + " conformance=" + reportField(string(r.Conformance)) + " assessment-complete=" + strconv.FormatBool(r.AssessmentComplete) + " input-complete=" + strconv.FormatBool(r.InputComplete) + "\n")
	out.add(fmt.Sprintf("counts: pass=%d fail=%d not-applicable=%d unknown=%d unsupported=%d blocked=%d\nblocking-problems=%d diagnostics-truncated=%t\n", r.Counts.Pass, r.Counts.Fail, r.Counts.NotApplicable, r.Counts.Unknown, r.Counts.Unsupported, r.Counts.Blocked, r.BlockingProblems, r.DiagnosticsTruncated))
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

// reportAccumulator keeps authoritative assessment axes independently of the
// diagnostic storage budget. It belongs to one single-goroutine session.
type reportAccumulator struct {
	report           Report
	account          *Account
	rules            map[RuleKey]bool
	normativeUnknown bool
	unfinished       bool
}

func newReportAccumulator(profile Profile, account *Account) *reportAccumulator {
	return &reportAccumulator{report: Report{Requested: true, Profile: cloneProfile(profile), Target: profile.Target, AssessmentComplete: true}, account: account, rules: make(map[RuleKey]bool)}
}

func (a *reportAccumulator) record(rule RuleDefinition, decision Decision, location Location) error {
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
		if unresolved {
			a.report.AssessmentComplete = false
		}
	}
	// Pass and proven exclusion are assessment results, not diagnostics. Keep
	// their authoritative counts above, without measuring or storing payloads,
	// consuming diagnostic quotas, or marking diagnostic storage truncated.
	if decision.Outcome == OutcomePass || decision.Outcome == OutcomeNotApplicable || a.report.DiagnosticsTruncated {
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
