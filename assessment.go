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

// AssessmentOptions selects a target-specific profile and finite shared limits.
// An empty Profile selects the full semantic profile of each entry point, with
// the bundled required dictionaries unless Dictionaries is set; a non-empty
// Profile must be known and fit the entry point. Zero budget fields select
// defaults. DecodeAssessed fixes its required stages and accepts only an empty
// Profile or ProfileSourceSemantics.
// Source and model profiles describe their bounded target-specific scope and
// cannot be interchanged. Standalone source/model entry points reject the
// separate package/resource-graph profiles. Source
// facts also have fixed identity limits: 1024
// UTF-8 bytes per local name or prefix, 4096 per namespace URI, and 65536 in-scope
// namespace bindings. Exceeding these is incomplete, never malformed XML.
// Buffered-byte accounting bounds admitted retained representations and logical
// model-candidate storage, not exact Go heap use or garbage-collector overhead.
type AssessmentOptions struct {
	Profile validation.ProfileID
	Budgets validation.Budgets
	// DTD supplies caller DTD bytes ahead of the bundled official MusicXML
	// DTDs to source profiles that read DTD declarations. Nil uses only the
	// bundled DTDs. Model assessment ignores it.
	DTD *DTDOptions
	// Dictionaries selects pinned dictionary capabilities, which the
	// dictionary and semantic profiles consult. With an empty Profile, nil or
	// a value with a nil Source and no selections selects the bundled data
	// those profiles require; with an explicit profile, nil keeps the
	// profile's dictionary-free behaviour.
	Dictionaries *DictionaryOptions
}

// DecodeAssessmentOptions configures mandatory assessed loading. Nil Assessment
// and an empty Profile select the required source and model semantic stages.
// A nonempty Profile must be ProfileSourceSemantics; it cannot narrow validation.
// Budgets are shared by dictionary import and both stages. Decode remains the
// separate conversion-only API.
type DecodeAssessmentOptions struct {
	Decode     DecodeOptions
	Assessment *AssessmentOptions
	// Lenient returns the converted document even when an assessment finds
	// violations or is incomplete, with err == nil and DecodeResult.Accepted
	// false. Operational failures still return an error and no document:
	// malformed XML, I/O, cancellation, resource limits, conversion failure,
	// dictionary import failure and invalid configuration. The model is
	// assessed even after source findings, so both reports are complete.
	Lenient bool
}

// DecodeResult preserves original-source and current-model reports separately.
// Document is nonnil only after conversion, source EOF, and both required
// assessments succeed. Model may be unrequested after an earlier failure or
// model-session startup failure; it does not distinguish those cases. Converted
// means successful mapping under the existing Go model contract, not exact
// numeric or lexical round-trip equality.
// Conversion.Findings is optional bounded detail; State and the returned typed
// error remain authoritative if the diagnostic quota omits that detail.
type DecodeResult struct {
	Document   Document
	Source     validation.Report
	Model      validation.Report
	Conversion validation.ConversionResult
	// Accepted reports that conversion succeeded and both the source and the
	// model assessment were accepted. In the default strict mode Document is
	// non-nil exactly when Accepted. In the lenient mode Document is also
	// returned when Accepted is false.
	Accepted bool
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
//
// An empty Profile selects ProfileSourceSemantics with the bundled required
// dictionaries, or options.Dictionaries: the source assessment of
// DecodeAssessed. Otherwise Profile must be a source-target profile, and a
// non-nil options.Dictionaries requires a dictionary or semantic one.
func InspectXML(ctx context.Context, reader io.Reader, options AssessmentOptions) (report validation.Report, err error) {
	if assessment, dictionaries, ok := options.scenario(ProfileSourceSemantics); ok {
		return inspectXMLWithDictionaries(ctx, reader, DictionaryAssessmentOptions{Assessment: assessment, Dictionaries: dictionaries})
	}
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

// DecodeAssessed reads and maps one MusicXML document, always assessing its
// original source and then its current Go model with the built-in semantic
// profiles. Source observation precedes namespace filtering and skipped fields;
// model assessment walks the converted graph directly, without encoding/replay.
// After recoverable conversion failure, the healthy parser is drained so later
// source violations remain visible. Model assessment runs only after source and
// conversion acceptance. Both stages share one finite budget account and the
// bundled pinned canonical SMuFL and IPA representation data.
//
// Nil/empty assessment options enable the required stages. A narrower profile is
// a configuration error. options.Assessment.Dictionaries replaces the bundled
// required dictionary data and must select canonical SMuFL and IPA
// representation relations. By default, violated, incomplete or unavailable
// assessment never returns a document; nil operation error alone is not
// acceptance. options.Lenient returns the document with its reports instead,
// and DecodeResult.Accepted tells the two outcomes apart. The profiles
// retain their bounded coverage: unsupported cases remain incomplete. No DTD,
// schema, dictionary or linked resource is fetched. The input is never closed.
// A DOCTYPE external subset is read from the bundled official MusicXML DTDs
// or options.Decode.DTD; an unrecognized one cannot satisfy this loader.
// Ordinary Decode provides conversion only, not validation.
// InspectXML documents source coordinates and cancellation limits.
//
// Any binary referencing DecodeAssessed embeds all five bundled dictionary
// components, even with custom dictionary options; API.md lists the other
// entry points that do.
// Distribute their notice/license files; see THIRD_PARTY_NOTICES.md.
//
// ValidationError contains stored normative/model failure details, possibly
// empty when diagnostic quotas omit them. Source and Model preserve authoritative
// counts and completion separately; Conversion describes mapping, not acceptance.
func DecodeAssessed(ctx context.Context, reader io.Reader, options DecodeAssessmentOptions) (DecodeResult, error) {
	var assessment *DictionaryAssessmentOptions
	if options.Assessment != nil {
		assessment = &DictionaryAssessmentOptions{Assessment: *options.Assessment}
		if dictionaries := options.Assessment.Dictionaries; dictionaries != nil {
			assessment.Dictionaries = *dictionaries
		}
		assessment.Assessment.Dictionaries = nil
	}
	return decodeValidated(ctx, reader, options.Decode, assessment, options.Lenient)
}

// decodeAssessedRun preserves the single source/parser owner for both the
// original public API and additive capability wrappers.
func decodeAssessedRun(ctx context.Context, run *sourceAssessmentRun, modelConfig assessmentRunConfig, priorErr error, lenient bool) (result DecodeResult, err error) {
	result.Conversion.State = validation.ConversionIncomplete
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
	conversionLocation := validation.Location{Target: validation.TargetSource, DocumentID: run.documentID, Entry: run.entry}
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
		conversionLocation = modelTokens.last.location
		conversionNode = modelTokens.last.node
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
	report, reportLease, finishErr := run.finishRetained(sourceErr)
	result.Source = report
	if conversionErr != nil && result.Conversion.State != validation.ConversionIncomplete {
		result.Conversion.Findings = run.conversionFinding(result.Conversion.State, conversionLocation, conversionNode)
	}
	// Acceptance errors describe the assessments; every other error is an
	// operational failure, which no mode tolerates. The lenient mode still
	// assesses the model after source findings, so that both reports exist.
	acceptanceErr := sourceAcceptanceError(report)
	operationErr := errors.Join(priorErr, conversionErr, sourceErr, finishErr)
	modelAssessed := false
	if operationErr == nil && (acceptanceErr == nil || lenient) {
		var modelErr error
		var modelStatus modelWalkStatus
		result.Model, modelStatus, modelErr = assessModelWithConfig(ctx, document, modelConfig, modelWalkOptions{})
		operationErr = modelErr
		acceptanceErr = errors.Join(acceptanceErr, modelAcceptanceError(result.Model, modelStatus))
		modelAssessed = true
	}
	accepted := operationErr == nil && acceptanceErr == nil && modelAssessed
	err = operationErr
	if !lenient {
		err = errors.Join(operationErr, acceptanceErr)
	}
	// Both reports and the candidate coexist throughout the model phase.
	// Closing the lease releases accounting, never the caller-owned report.
	err = errors.Join(err, reportLease.Close())
	if err != nil {
		document = nil
	}
	cleanupErr := modelTokens.close()
	err = errors.Join(err, cleanupErr)
	if err == nil {
		result.Document = document
		result.Accepted = accepted
	}
	return result, err
}

type sourceAssessmentConfig struct {
	assessmentRunConfig
	ByteOwner sourceByteOwner
}

type sourceAssessmentRun struct {
	ctx               context.Context
	account           *validation.Account
	session           *validation.Session
	source            *observingXMLSource
	documentID, entry string
	inputFinished     bool
	startupReport     validation.Report
}

// Public wrappers alone create the root account. Resource orchestration supplies
// the same account to every child via newSourceAssessmentRunWithConfig.
func newSourceAssessmentRun(ctx context.Context, reader io.Reader, decode DecodeOptions, assessment *AssessmentOptions, converting bool) (*sourceAssessmentRun, error) {
	if err := assessmentContext(ctx, "canceled-before-source"); err != nil {
		return nil, err
	}
	if reader == nil {
		return nil, ErrNilReader
	}
	if _, err := decode.xmlDepth(); err != nil {
		return nil, err
	}
	if assessment == nil {
		return nil, &validation.ConfigurationError{Field: "assessment", Reason: "source assessment requires an explicit configuration"}
	}
	engine, err := sourceAssessmentEngineForProfile(assessment.Profile)
	if err != nil {
		return nil, err
	}
	account, err := validation.NewBudgetAccount(assessment.Budgets)
	if err != nil {
		return nil, err
	}
	config := &sourceAssessmentConfig{assessmentRunConfig: assessmentRunConfig{Account: account, DocumentID: sourceDocumentID, Profile: assessment.Profile, Engine: engine, DTD: assessment.DTD}, ByteOwner: sourceBytesPrimary}
	return newSourceAssessmentRunWithConfig(ctx, reader, decode, config, converting)
}

// Source assessment always requires an existing account and an explicit selected
// profile. Ordinary conversion does not use this adapter.
func newSourceAssessmentRunWithConfig(ctx context.Context, reader io.Reader, decode DecodeOptions, config *sourceAssessmentConfig, converting bool) (*sourceAssessmentRun, error) {
	if err := assessmentContext(ctx, "canceled-before-source"); err != nil {
		return nil, err
	}
	if reader == nil {
		return nil, ErrNilReader
	}
	maxDepth, err := decode.xmlDepth()
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, &validation.ConfigurationError{Field: "source assessment config", Reason: "must not be nil"}
	}
	if err := config.validate(validation.TargetSource); err != nil {
		return nil, err
	}
	if err := config.ByteOwner.validate(); err != nil {
		return nil, err
	}
	run := &sourceAssessmentRun{ctx: ctx, account: config.Account, documentID: config.DocumentID, entry: config.Entry}
	// Ordinary inspection has no decode/model depth contract. A resource
	// adapter can deliberately supply its separate MXL XML-depth cap.
	if !converting {
		maxDepth = config.XMLDepth
	} else if config.XMLDepth > 0 && config.XMLDepth < maxDepth {
		maxDepth = config.XMLDepth
	}
	run.session, err = config.assessmentRunConfig.begin(ctx)
	if err != nil {
		return nil, err
	}
	fail := func(cause error) (*sourceAssessmentRun, error) {
		report, finishErr := run.finish(cause)
		run.startupReport = report
		return run, errors.Join(cause, finishErr)
	}
	internalDTD, sourceDeclarations, captureAttributeLexical := sourceDTDPolicy(config.Profile)
	sourceDTDIDs := config.Engine.SourceDTDIDsSelected(config.Profile)
	// Profiles that read DTD declarations also read a bundled or caller
	// external subset; the others keep reporting it as unread.
	var dtd *sourceDTDResolver
	if internalDTD {
		options := decode.DTD
		if options == nil {
			options = config.DTD
		}
		dtd = newSourceDTDResolver(options)
	}
	lexical, err := newSourceXMLTokenReader(reader, sourceXMLConfig{Context: ctx, Account: run.account, MaxDepth: maxDepth, ByteOwner: config.ByteOwner, InternalDTD: internalDTD, CaptureAttributeLexical: captureAttributeLexical, CaptureDTDIDs: sourceDTDIDs, DTD: dtd})
	if err != nil {
		return fail(err)
	}
	run.source = &observingXMLSource{ctx: ctx, lexical: lexical, decoder: xml.NewTokenDecoder(lexical)}
	run.source.observer, err = newSourceFactObserverWithIdentity(ctx, run.session, run.account, run.documentID, run.entry)
	if err != nil {
		return fail(err)
	}
	run.source.observer.sourceDeclarations = sourceDeclarations
	run.source.observer.sourceDTDIDs = sourceDTDIDs
	run.source.observer.captureAttributeLexical = captureAttributeLexical
	if internalDTD {
		const originBridgeBytes = 512
		if err := run.account.Charge(validation.BudgetBufferedBytes, originBridgeBytes); err != nil {
			return fail(err)
		}
		run.source.observer.retained += originBridgeBytes
		run.source.observer.origins = &sourceOriginObserver{engine: config.Engine, session: run.session, profile: config.Profile}
	}
	return run, nil
}

func (r *sourceAssessmentRun) drain() error {
	if r.inputFinished || r.source == nil {
		return &validation.SessionError{Operation: "source drain", Reason: "input unavailable or finalized"}
	}
	for {
		_, err := r.source.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// finishInput closes only adapter-owned parsing state and returns the exact
// completion for one terminal core finalizer. A future resource finalizer uses
// this same seam before exporting its owned snapshot and cleaning the session.
func (r *sourceAssessmentRun) finishInput(cause error) (validation.Completion, error) {
	if r.inputFinished {
		return validation.Completion{}, &validation.SessionError{Operation: "source finish", Reason: "input already finalized"}
	}
	r.inputFinished = true
	var summary sourceXMLSummary
	inputComplete := false
	if r.source != nil {
		summary = r.source.lexical.Summary()
		inputComplete = r.source.eof && summary.InputComplete
		if r.source.failure != nil && !errors.Is(cause, r.source.failure) {
			cause = errors.Join(r.source.failure, cause)
		}
		if inputComplete && cause == nil {
			cause = r.source.observer.sealSourceOrigins(r.ctx)
			if cause == nil {
				cause = r.source.observer.sealSourceDTDIDs(r.ctx)
			}
		}
		cause = errors.Join(cause, r.source.observer.close(), r.source.lexical.Close())
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
	return validation.Completion{InputComplete: inputComplete, Syntax: syntax, Cause: cause}, nil
}

func (r *sourceAssessmentRun) finish(cause error) (validation.Report, error) {
	completion, err := r.finishInput(cause)
	if err != nil {
		return validation.Report{}, err
	}
	return r.session.Finish(r.ctx, completion)
}

func (r *sourceAssessmentRun) finishRetained(cause error) (validation.Report, *validation.ReportLease, error) {
	completion, err := r.finishInput(cause)
	if err != nil {
		return validation.Report{}, nil, err
	}
	return r.session.FinishWithRetainedReport(r.ctx, completion)
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
				issues = append(issues, assessmentIssue(finding))
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
	if !report.AssessmentComplete || !report.InputComplete || report.Syntax != validation.SyntaxWellFormed || report.BlockingProblems != 0 || report.Terminal != nil || report.Counts.Unknown != 0 || report.Counts.Unsupported != 0 || report.Counts.Blocked != 0 || invalid == nil {
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
	return []validation.Finding{{Decision: validation.Decision{Rule: validation.RuleKey{ID: "musicxml.model-conversion", Revision: 1}, Subject: validation.NodeRef{DocumentID: r.documentID, Node: node}, Outcome: validation.OutcomeFail, Reason: validation.Code(state)}, Category: validation.CategoryModel, Location: location}}
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
