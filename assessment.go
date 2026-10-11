package musicxml

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/go-muse/go-musicxml/validation"
)

// AssessmentOptions selects an explicit source profile and finite shared limits.
// An empty or unknown Profile is invalid; zero budget fields select defaults.
// Profiles describe their bounded scope and are not interchangeable with model
// or package assessment. Source facts also have fixed identity limits: 1024
// UTF-8 bytes per local name or prefix, 4096 per namespace URI, and 65536 in-scope
// namespace bindings. Exceeding these is incomplete, never malformed XML.
// Buffered-byte accounting bounds admitted retained representations and logical
// model-candidate storage, not exact Go heap use or garbage-collector overhead.
type AssessmentOptions struct {
	Profile validation.ProfileID
	Budgets validation.Budgets
}

// DecodeAssessmentOptions keeps assessment opt-in without changing DecodeOptions.
// A nil Assessment performs permissive model conversion and mandatory XML checks
// without loading a catalog. A nonnil Assessment requires an explicit profile.
type DecodeAssessmentOptions struct {
	Decode     DecodeOptions
	Assessment *AssessmentOptions
}

// DecodeResult separates original-source assessment from model conversion.
// Document is nonnil only after conversion, source EOF, and (when requested)
// source acceptance all succeed. Converted means successful mapping under the
// existing Go model contract, not exact numeric or lexical round-trip equality.
// Conversion.Findings is optional bounded detail; State and the returned typed
// error remain authoritative if the diagnostic quota omits that detail.
type DecodeResult struct {
	Document   Document
	Source     validation.Report
	Conversion validation.ConversionResult
}

// ConversionError preserves a model-mapping failure independently of source
// conformance. A machine-range failure is Unrepresentable and does not itself
// match ErrInvalidDocument. Cause retains the original mapper or transport error.
type ConversionError struct {
	State validation.ConversionState
	Cause error
}

// Error describes the conversion state. Inspect Cause for the mapper's details.
func (e *ConversionError) Error() string {
	if e == nil {
		return "musicxml: nil conversion error"
	}
	return "musicxml: model conversion " + string(e.State)
}

// Unwrap preserves the original conversion cause for errors.Is and errors.As.
func (e *ConversionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// InspectXML assesses original XML once without building a Go document. Ordinary
// normative findings have a nil operation error; inspect Report.Satisfied for
// acceptance. Fatal syntax, I/O, capability, cancellation and resource errors
// return the best partial report with their causes. The input is never closed.
//
// Positions use original, zero-based byte offsets including any BOM, with
// one-based decoded-character line/column coordinates. CRLF is one line break.
// If either coordinate exceeds the host integer range, both are omitted as 0/0
// for the rest of the source; original-byte offsets remain available.
// Overlong display paths are omitted without dropping occurrence IDs/positions.
// Context is checked during parsing and evaluation; it cannot interrupt a Read
// blocked inside an arbitrary reader, which must provide its own cancellation.
func InspectXML(ctx context.Context, reader io.Reader, options AssessmentOptions) (report validation.Report, err error) {
	run, err := newSourceAssessmentRun(ctx, reader, DecodeOptions{}, &options, false)
	if err != nil {
		if run != nil {
			return run.startupReport, err
		}
		return validation.Report{}, err
	}
	err = run.drain()
	report, finishErr := run.finish(err)
	return report, errors.Join(err, finishErr)
}

// DecodeAssessed maps one MusicXML document while optionally assessing all of its
// original source. Observation precedes model namespace filtering and skipping.
// After a recoverable model-conversion failure the same healthy source parser is
// drained, so a later source violation or malformed tail remains visible. No
// input replay, model encoding or second XML parse is performed.
//
// Strict assessment rejects a violated or incomplete report and always discards
// a partial model. Nil Assessment retains permissive conversion, legacy depth
// limits and context checks, and leaves Source.Requested false. The input is
// never closed. InspectXML documents source coordinates and cancellation limits.
// A returned ValidationError contains only stored normative failure details;
// Issues can be empty when the diagnostic budget excludes them. The operation
// error explains truncation without adding synthetic issues. Source.Counts
// counts rule evaluations, which need not equal the number of stored details.
func DecodeAssessed(ctx context.Context, reader io.Reader, options DecodeAssessmentOptions) (result DecodeResult, err error) {
	result.Conversion.State = validation.ConversionIncomplete
	run, err := newSourceAssessmentRun(ctx, reader, options.Decode, options.Assessment, true)
	if err != nil {
		if run != nil {
			result.Source = run.startupReport
		}
		return result, err
	}
	// The mapping decoder only receives namespace-neutral projected elements and
	// the existing allowlisted expanded attributes. Its namespace table is empty.
	projected := &namespaceXMLTokenReader{source: run.source, allowQualifiedAttribute: musicXMLQualifiedAttribute}
	modelTokens := &assessedModelTokenReader{source: projected, account: run.account, ctx: ctx, observer: run.source.observer}
	decoder := xml.NewTokenDecoder(modelTokens)
	start, mapErr := readRoot(decoder)
	var document Document
	if mapErr == nil {
		if start.Name.Space != "" {
			mapErr = &UnsupportedRootError{Name: start.Name}
		} else {
			switch start.Name.Local {
			case "score-partwise":
				document = &ScorePartwise{}
			case "score-timewise":
				document = &ScoreTimewise{}
			case "opus":
				document = &OpusDocument{}
			default:
				mapErr = &UnsupportedRootError{Name: start.Name}
			}
		}
	}
	if mapErr == nil {
		mapErr = decoder.DecodeElement(document, &start)
	}
	var conversionErr error
	conversionLocation := validation.Location{Target: validation.TargetSource, DocumentID: sourceDocumentID}
	var conversionNode validation.NodeID
	if mapErr != nil {
		state := validation.ConversionFailed
		switch {
		case errors.Is(mapErr, strconv.ErrRange):
			state = validation.ConversionUnrepresentable
		case run.source.failure != nil || sourceFatalError(mapErr):
			state = validation.ConversionIncomplete
		}
		result.Conversion.State = state
		conversionErr = &ConversionError{State: state, Cause: mapErr}
		if run.source.observer != nil {
			conversionLocation = modelTokens.last.location
			conversionNode = modelTokens.last.node
		}
	} else {
		result.Conversion.State = validation.ConversionConverted
	}
	// Never ask the failed reflection decoder to resume. Only the independent,
	// authoritative source owner can safely continue after UnmarshalXML unwinds.
	sourceErr := run.source.failure
	if sourceErr == nil && !sourceFatalError(mapErr) {
		sourceErr = run.drain()
	} else if sourceErr == nil {
		sourceErr = mapErr
	}
	// The candidate remains live through root/identity/report finalization and
	// source acceptance. Releasing it earlier would admit overlapping retained
	// data against a budget that no longer represented the live operation.
	report, finishErr := run.finish(sourceErr)
	result.Source = report
	if conversionErr != nil && result.Conversion.State != validation.ConversionIncomplete {
		result.Conversion.Findings = run.conversionFinding(result.Conversion.State, conversionLocation, conversionNode)
	}
	acceptanceErr := error(nil)
	if options.Assessment != nil {
		acceptanceErr = sourceAcceptanceError(report)
	}
	err = errors.Join(conversionErr, sourceErr, finishErr, acceptanceErr)
	if err != nil {
		document = nil
	}
	cleanupErr := modelTokens.close()
	err = errors.Join(err, cleanupErr)
	if err == nil {
		result.Document = document
	}
	return result, err
}

type sourceAssessmentRun struct {
	ctx           context.Context
	account       *validation.Account
	session       *validation.Session
	source        *observingXMLSource
	startupReport validation.Report
}

func newSourceAssessmentRun(ctx context.Context, reader io.Reader, decode DecodeOptions, assessment *AssessmentOptions, converting bool) (*sourceAssessmentRun, error) {
	if ctx == nil {
		return nil, &validation.ConfigurationError{Field: "context", Reason: "must not be nil"}
	}
	if err := ctx.Err(); err != nil {
		return nil, &validation.IncompleteError{Reason: "canceled-before-source", Cause: err}
	}
	if reader == nil {
		return nil, ErrNilReader
	}
	maxDepth, err := decode.xmlDepth()
	if err != nil {
		return nil, err
	}
	run := &sourceAssessmentRun{ctx: ctx}
	if assessment != nil {
		if assessment.Profile != ProfileSourceXSDPreview {
			return nil, &validation.ConfigurationError{Field: "profile", Reason: "unknown or empty explicit source profile"}
		}
		run.account, err = validation.NewBudgetAccount(assessment.Budgets)
		if err != nil {
			return nil, err
		}
		// Inspection has no model depth contract. Decode enforces its explicit or
		// default maximum in addition to the assessment account's scope limit.
		if !converting {
			maxDepth = 0
		}
		engine, engineErr := newSourceAssessmentEngine()
		if engineErr != nil {
			return nil, engineErr
		}
		run.session, err = engine.Begin(ctx, validation.RunOptions{DocumentID: sourceDocumentID, Profile: assessment.Profile, Account: run.account})
		if err != nil {
			return nil, err
		}
	}
	fail := func(cause error) (*sourceAssessmentRun, error) {
		if run.source != nil {
			report, finishErr := run.finish(cause)
			run.startupReport = report
			return run, errors.Join(cause, finishErr)
		}
		if run.session != nil {
			report, finishErr := run.session.Finish(ctx, validation.Completion{Syntax: validation.SyntaxIncomplete, Cause: cause})
			run.startupReport = report
			return run, errors.Join(cause, finishErr)
		}
		return nil, cause
	}
	lexical, err := newSourceXMLTokenReader(reader, sourceXMLConfig{Context: ctx, Account: run.account, MaxDepth: maxDepth})
	if err != nil {
		return fail(err)
	}
	run.source = &observingXMLSource{ctx: ctx, lexical: lexical, decoder: xml.NewTokenDecoder(lexical), musicXMLRoot: converting}
	if run.session != nil {
		run.source.observer, err = newSourceFactObserver(ctx, run.session, run.account)
		if err != nil {
			return fail(err)
		}
	}
	return run, nil
}

func (r *sourceAssessmentRun) drain() error {
	for {
		_, err := r.source.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
func (r *sourceAssessmentRun) finish(cause error) (validation.Report, error) {
	summary := r.source.lexical.Summary()
	observerErr := r.source.observer.close()
	parserErr := r.source.lexical.Close()
	cause = errors.Join(cause, observerErr, parserErr)
	if r.session == nil {
		return validation.Report{}, cause
	}
	syntax := validation.SyntaxIncomplete
	if summary.SyntaxComplete {
		syntax = validation.SyntaxWellFormed
	} else {
		var syntaxErr *xml.SyntaxError
		if errors.As(cause, &syntaxErr) || errors.Is(cause, ErrEmptyDocument) {
			syntax = validation.SyntaxMalformed
		}
	}
	return r.session.Finish(r.ctx, validation.Completion{InputComplete: r.source.eof && summary.InputComplete, Syntax: syntax, Cause: cause})
}
func sourceFatalError(err error) bool {
	return err != nil && (errors.Is(err, validation.ErrResourceLimit) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, validation.ErrInvalidSession))
}
func sourceAcceptanceError(report validation.Report) error {
	if report.Satisfied() {
		return nil
	}
	var invalid error
	if report.Conformance == validation.ConformanceViolated {
		issues := make([]ValidationIssue, 0)
		for _, finding := range report.Findings {
			if finding.Category == validation.CategoryNormative && finding.Outcome == validation.OutcomeFail {
				issues = append(issues, ValidationIssue{Path: finding.Location.Path, Constraint: string(finding.Rule.ID), Message: string(finding.Reason)})
			}
		}
		invalid = &ValidationError{Issues: issues}
		if report.DiagnosticsTruncated {
			// Counts measure rule evaluations, not individual details. Preserve
			// only stored details in Issues; a bounded operation summary neither
			// invents missing locations nor consumes another diagnostic slot.
			invalid = fmt.Errorf("%w; source assessment: failed rule evaluations: %d; diagnostic details were truncated", invalid, report.Counts.Fail)
		}
	}
	if !report.AssessmentComplete || !report.InputComplete || report.Syntax != validation.SyntaxWellFormed || report.BlockingProblems != 0 || report.Terminal != nil {
		return errors.Join(invalid, &validation.IncompleteError{Reason: "source-profile-not-satisfied"})
	}
	return invalid
}
func (r *sourceAssessmentRun) conversionFinding(state validation.ConversionState, location validation.Location, node validation.NodeID) []validation.Finding {
	if r.account == nil {
		return nil
	}
	size := int64(256 + len(location.Path))
	// Diagnostic exhaustion is storage truncation; conversion State and the typed
	// operation error remain authoritative even when this optional detail is lost.
	used, _ := r.account.Usage(validation.BudgetDiagnostics)
	byteUsage, _ := r.account.Usage(validation.BudgetDiagnosticBytes)
	limits := r.account.Limits()
	if used >= limits.MaxDiagnostics || size > limits.MaxDiagnosticBytes-byteUsage {
		return nil
	}
	if err := r.account.Charge(validation.BudgetDiagnosticBytes, size); err != nil {
		return nil
	}
	if err := r.account.Charge(validation.BudgetDiagnostics, 1); err != nil {
		return nil
	}
	return []validation.Finding{{Decision: validation.Decision{Rule: validation.RuleKey{ID: "musicxml.model-conversion", Revision: 1}, Subject: validation.NodeRef{DocumentID: sourceDocumentID, Node: node}, Outcome: validation.OutcomeFail, Reason: validation.Code(state)}, Category: validation.CategoryModel, Location: location}}
}

// assessedModelTokenReader reserves logical model-candidate retention before
// handing each projected token to reflection/generated unmarshalling. The
// allowance is deliberately separate from parser/session copies, not a claim to
// measure Go allocator overhead. The caller owns a successful model after return.
type assessedModelTokenReader struct {
	source   xml.TokenReader
	account  *validation.Account
	ctx      context.Context
	retained int64
	observer *sourceFactObserver
	scopes   []assessedModelScope
	last     assessedModelScope
}

type assessedModelScope struct {
	node     validation.NodeID
	location validation.Location
}

func (r *assessedModelTokenReader) Token() (xml.Token, error) {
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	token, err := r.source.Token()
	if err != nil || r.account == nil {
		return token, err
	}
	var size int64
	var scope assessedModelScope
	switch value := token.(type) {
	case xml.StartElement:
		size = 256 + int64(len(value.Name.Local)) + int64(len(value.Name.Space))
		if r.observer != nil {
			scope = assessedModelScope{node: r.observer.next, location: r.observer.last}
			size += 160 + int64(len(scope.location.Path))
		}
		for _, attr := range value.Attr {
			size += 96 + int64(len(attr.Name.Local)) + int64(len(attr.Name.Space)) + int64(len(attr.Value))
		}
	case xml.CharData:
		size = int64(len(value))
	}
	if err := r.account.Charge(validation.BudgetBufferedBytes, size); err != nil {
		return nil, err
	}
	r.retained += size
	// Track the projected mapping owner's start, not the latest source fact:
	// skipped nested content may be the last observed node when a scalar fails.
	switch token.(type) {
	case xml.StartElement:
		scope.location.Path = strings.Clone(scope.location.Path)
		if scope.location.Position != nil {
			position := *scope.location.Position
			scope.location.Position = &position
		}
		r.scopes = append(r.scopes, scope)
		r.last = scope
	case xml.EndElement:
		if len(r.scopes) > 0 {
			r.last = r.scopes[len(r.scopes)-1]
			r.scopes = r.scopes[:len(r.scopes)-1]
		}
	case xml.CharData:
		if len(r.scopes) > 0 {
			r.last = r.scopes[len(r.scopes)-1]
		}
	}
	return token, nil
}
func (r *assessedModelTokenReader) close() error {
	if r.account == nil {
		return nil
	}
	if err := r.account.Release(validation.BudgetBufferedBytes, r.retained); err != nil {
		return fmt.Errorf("musicxml: release candidate model: %w", err)
	}
	r.retained = 0
	r.scopes = nil
	r.last = assessedModelScope{}
	return nil
}
