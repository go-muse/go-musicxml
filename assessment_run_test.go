package musicxml

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func adapterAccount(t *testing.T, budgets validation.Budgets) *validation.Account {
	t.Helper()
	a, err := validation.NewBudgetAccount(budgets)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func adapterSourceConfig(t *testing.T, a *validation.Account, id, entry string, profile validation.ProfileID, owner sourceByteOwner) sourceAssessmentConfig {
	t.Helper()
	engine, err := sourceAssessmentEngineForProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	return sourceAssessmentConfig{assessmentRunConfig: assessmentRunConfig{Account: a, DocumentID: id, Entry: entry, Profile: profile, Engine: engine}, ByteOwner: owner}
}
func adapterModelConfig(t *testing.T, a *validation.Account, id, entry string, profile validation.ProfileID) assessmentRunConfig {
	t.Helper()
	engine, err := modelAssessmentEngineForProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	return assessmentRunConfig{Account: a, DocumentID: id, Entry: entry, Profile: profile, Engine: engine}
}
func adapterInspect(t *testing.T, input io.Reader, config sourceAssessmentConfig) (validation.Report, error) {
	t.Helper()
	run, err := newSourceAssessmentRunWithConfig(context.Background(), input, DecodeOptions{}, &config, false)
	if err != nil {
		if run != nil {
			return run.startupReport, err
		}
		return validation.Report{}, err
	}
	drainErr := run.drain()
	report, finishErr := run.finish(drainErr)
	return report, errors.Join(drainErr, finishErr)
}
func adapterUsage(t *testing.T, a *validation.Account, kind validation.BudgetKind) int64 {
	t.Helper()
	used, err := a.Usage(kind)
	if err != nil {
		t.Fatal(err)
	}
	return used
}
func adapterClean(t *testing.T, a *validation.Account) {
	t.Helper()
	for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth} {
		if n := adapterUsage(t, a, kind); n != 0 {
			t.Errorf("leaked %s=%d", kind, n)
		}
	}
}

func TestAssessmentRunSharedAggregateSourceBytes(t *testing.T) {
	const input = `<opus version="4.0"/>`
	for _, owner := range []sourceByteOwner{sourceBytesPrimary, sourceBytesExternal} {
		t.Run(fmt.Sprint(owner), func(t *testing.T) {
			a := adapterAccount(t, validation.Budgets{MaxInputBytes: int64(len(input)), MaxExternalBytes: int64(len(input))})
			for i := 0; i < 2; i++ {
				c := adapterSourceConfig(t, a, fmt.Sprintf("package/child-%d", i), "scores/opus.xml", ProfileSourceXSDPreview, owner)
				report, err := adapterInspect(t, strings.NewReader(input), c)
				if i == 0 && (err != nil || !report.Satisfied()) {
					t.Fatalf("first child: %v %s", err, localFindings(report))
				}
				if i == 1 && (!errors.Is(err, validation.ErrResourceLimit) || report.InputComplete) {
					t.Fatalf("child reset aggregate bytes: %v %+v", err, report)
				}
				adapterClean(t, a)
			}
			kind, other := validation.BudgetInputBytes, validation.BudgetExternalBytes
			if owner == sourceBytesExternal {
				kind, other = other, kind
			}
			if used := adapterUsage(t, a, kind); used != int64(len(input)) {
				t.Errorf("%s=%d", kind, used)
			}
			if used := adapterUsage(t, a, other); used != 0 {
				t.Errorf("charged unrelated %s=%d", other, used)
			}
		})
	}
}

func TestAssessmentRunAccountedBytesStillChargeParsing(t *testing.T) {
	const input = `<opus version="4.0"/>`
	a := adapterAccount(t, validation.Budgets{MaxInputBytes: 1, MaxExternalBytes: int64(len(input))})
	if err := a.Charge(validation.BudgetExternalBytes, int64(len(input))); err != nil {
		t.Fatal(err)
	}
	c := adapterSourceConfig(t, a, "archive/opus", "scores/opus.xml", ProfileSourceXSDPreview, sourceBytesAccounted)
	report, err := adapterInspect(t, strings.NewReader(input), c)
	if err != nil || !report.Satisfied() {
		t.Fatalf("accounted bytes: %v %+v", err, report)
	}
	if adapterUsage(t, a, validation.BudgetInputBytes) != 0 || adapterUsage(t, a, validation.BudgetExternalBytes) != int64(len(input)) {
		t.Fatal("second cumulative byte charge")
	}
	for _, kind := range []validation.BudgetKind{validation.BudgetNodes, validation.BudgetAttributes, validation.BudgetEvents, validation.BudgetRuleSteps} {
		if adapterUsage(t, a, kind) == 0 {
			t.Errorf("accounted bytes bypassed %s", kind)
		}
	}
	adapterClean(t, a)
}

func TestAssessmentRunMixedChildrenShareNodesAndWork(t *testing.T) {
	a := adapterAccount(t, validation.Budgets{MaxNodes: 2})
	source := adapterSourceConfig(t, a, "root/source", "a.xml", ProfileSourceXSDPreview, sourceBytesExternal)
	report, err := adapterInspect(t, strings.NewReader(`<opus/>`), source)
	if err != nil || !report.Satisfied() {
		t.Fatalf("source: %v %+v", err, report)
	}
	events, steps := adapterUsage(t, a, validation.BudgetEvents), adapterUsage(t, a, validation.BudgetRuleSteps)
	model := adapterModelConfig(t, a, "root/model", "b.xml", ProfileModelXSD)
	report, _, err = assessModelWithConfig(context.Background(), NewOpusDocument(), model, modelWalkOptions{})
	if err != nil || !report.Satisfied() {
		t.Fatalf("model: %v %+v", err, report)
	}
	if adapterUsage(t, a, validation.BudgetEvents) <= events || adapterUsage(t, a, validation.BudgetRuleSteps) <= steps {
		t.Fatal("child work reset")
	}
	source.DocumentID = "root/source-2"
	report, err = adapterInspect(t, strings.NewReader(`<opus/>`), source)
	if !errors.Is(err, validation.ErrResourceLimit) || report.Satisfied() {
		t.Fatalf("aggregate nodes reset: %v %+v", err, report)
	}
	if adapterUsage(t, a, validation.BudgetNodes) != 2 {
		t.Fatal("incorrect aggregate node usage")
	}
	adapterClean(t, a)
}

// Traverse all exported report payloads so new compound context references also
// remain covered. Zero optional references are absent, not fabricated identities.
func adapterAssertQualified(t *testing.T, report validation.Report, id, entry string, target validation.Target) {
	t.Helper()
	nodeType, locationType := reflect.TypeOf(validation.NodeRef{}), reflect.TypeOf(validation.Location{})
	var walk func(reflect.Value)
	walk = func(v reflect.Value) {
		if !v.IsValid() {
			return
		}
		if v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
			if !v.IsNil() {
				walk(v.Elem())
			}
			return
		}
		if v.Type() == nodeType {
			n := v.Interface().(validation.NodeRef)
			if n != (validation.NodeRef{}) && n.DocumentID != id {
				t.Errorf("unqualified reference %+v want %s", n, id)
			}
			return
		}
		if v.Type() == locationType {
			loc := v.Interface().(validation.Location)
			if loc == (validation.Location{}) {
				return
			}
			if loc.DocumentID != id || loc.Entry != entry || loc.Target != target {
				t.Errorf("unqualified location %+v", loc)
			}
			if target == validation.TargetModel && loc.Position != nil {
				t.Errorf("model fabricated original coordinates %+v", loc)
			}
			if target == validation.TargetSource && (loc.Position == nil || loc.Position.CoordinateSpace != "original-bytes") {
				t.Errorf("source lost coordinates %+v", loc)
			}
			return
		}
		switch v.Kind() {
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() {
					walk(v.Field(i))
				}
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i))
			}
		}
	}
	walk(reflect.ValueOf(report))
}

func TestAssessmentRunQualifiedFindingsCausesRelatedAndContext(t *testing.T) {
	input := localContextFixture(`<attributes><divisions>1</divisions></attributes><note print-object="maybe"><rest/><duration>1</duration><type>quarter</type></note><note><rest/><duration>1</duration></note><note><chord/><rest/><duration>2</duration></note>`, false)
	// The oversized chord member retains its principal as a Related location;
	// the invalid print flag supplies causal context and blocked dependencies.
	document, err := Decode(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	a := adapterAccount(t, validation.Budgets{})
	for _, model := range []bool{false, true} {
		var report validation.Report
		id, entry, target := "root/child-source", "scores/source.xml", validation.TargetSource
		if model {
			id, entry, target = "root/child-model", "scores/current.xml", validation.TargetModel
			report, _, err = assessModelWithConfig(context.Background(), document, adapterModelConfig(t, a, id, entry, ProfileModelContext), modelWalkOptions{})
		} else {
			report, err = adapterInspect(t, strings.NewReader(input), adapterSourceConfig(t, a, id, entry, ProfileSourceContext, sourceBytesExternal))
		}
		if err != nil {
			t.Fatal(err)
		}
		adapterAssertQualified(t, report, id, entry, target)
		causes, related, origins := 0, 0, 0
		for _, f := range report.Findings {
			if f.Cause != nil {
				causes++
			}
			related += len(f.Related)
		}
		for _, c := range report.Context {
			if c.Value.Cause != nil {
				causes++
			}
			origins += len(c.Origins)
		}
		if len(report.Findings) == 0 || causes == 0 || related == 0 || origins == 0 {
			t.Fatalf("vacuous identity fixture: model=%v findings=%d causes=%d related=%d origins=%d", model, len(report.Findings), causes, related, origins)
		}
		// Starting/finalizing another child must not mutate the first child's IDs.
		_, childErr := adapterInspect(t, strings.NewReader(`<opus/>`), adapterSourceConfig(t, a, id+"/later", "later.xml", ProfileSourceXSDPreview, sourceBytesExternal))
		if childErr != nil {
			t.Fatal(childErr)
		}
		adapterAssertQualified(t, report, id, entry, target)
		adapterClean(t, a)
	}
}

func TestAssessmentRunOriginalSourcePositionsAndConversionIdentity(t *testing.T) {
	text := "<opus version=\"4.0\" xmlns:xlink=\"http://www.w3.org/1999/xlink\">\r\n<title>é</title><score xlink:href=\"x.xml\"/></opus>"
	for _, input := range [][]byte{append([]byte{239, 187, 191}, []byte(text)...), encodeUTF16(text, binary.LittleEndian), encodeUTF16(text, binary.BigEndian)} {
		a := adapterAccount(t, validation.Budgets{})
		c := adapterSourceConfig(t, a, "package/resource-2", "opus/book.xml", ProfileSourceXSDPreview, sourceBytesExternal)
		run, err := newSourceAssessmentRunWithConfig(context.Background(), bytes.NewReader(input), DecodeOptions{}, &c, false)
		if err != nil {
			t.Fatal(err)
		}
		facts := &recordingSourceFacts{}
		// Preserve the real session and record the exact observer ingress in parallel.
		run.source.observer.sink = &adapterRecordingSink{sourceFactSink: run.session, facts: facts}
		err = run.drain()
		location := run.source.observer.last
		report, finishErr := run.finish(err)
		if err != nil || finishErr != nil || !report.Satisfied() {
			t.Fatalf("source: %v %v %+v", err, finishErr, report)
		}
		if len(facts.headers) != 3 || len(facts.attributes) != 3 {
			t.Fatalf("source facts=%+v", facts)
		}
		scale, bom := 1, 3
		if input[0] != 239 {
			scale, bom = 2, 2
		}
		title := facts.headers[1].Location
		want := int64(bom + scale*strings.Index(text, "<title>"))
		if title.Position.ByteOffset != want || title.Position.Line != 2 || title.Position.Column != 1 || title.DocumentID != c.DocumentID || title.Entry != c.Entry {
			t.Errorf("original title position %+v want offset %d", title, want)
		}
		score := facts.headers[2].Location
		// Non-ASCII é uses two original UTF-8 bytes but one UTF-16 code unit.
		prefix := text[:strings.Index(text, "<score ")]
		want = int64(bom + len(prefix))
		if scale == 2 {
			want = int64(bom + 2*len([]rune(prefix)))
		}
		if score.Position.ByteOffset != want {
			t.Errorf("original score position %+v want %d", score, want)
		}
		finding := run.conversionFinding(validation.ConversionUnrepresentable, location, 3)
		if len(finding) != 1 || finding[0].Subject.DocumentID != c.DocumentID || finding[0].Location.Entry != c.Entry {
			t.Fatalf("conversion identity %+v", finding)
		}
		adapterClean(t, a)
	}
}

type adapterRecordingSink struct {
	sourceFactSink
	facts *recordingSourceFacts
}

func (s *adapterRecordingSink) Open(ctx context.Context, n validation.NodeHeader) error {
	if err := s.facts.Open(ctx, n); err != nil {
		return err
	}
	return s.sourceFactSink.Open(ctx, n)
}
func (s *adapterRecordingSink) Attribute(ctx context.Context, n validation.AttributeFact) error {
	if err := s.facts.Attribute(ctx, n); err != nil {
		return err
	}
	return s.sourceFactSink.Attribute(ctx, n)
}

type adapterReadCloser struct {
	io.Reader
	reads, closes int
}

func (r *adapterReadCloser) Read(p []byte) (int, error) { r.reads++; return r.Reader.Read(p) }
func (r *adapterReadCloser) Close() error               { r.closes++; return nil }

func TestAssessmentRunConfigurationBeforeInput(t *testing.T) {
	for _, mutate := range []func(*sourceAssessmentConfig){
		func(c *sourceAssessmentConfig) { c.Account = nil },
		func(c *sourceAssessmentConfig) { c.Account = &validation.Account{} },
		func(c *sourceAssessmentConfig) { c.DocumentID = "" },
		func(c *sourceAssessmentConfig) { c.DocumentID = "bad id" },
		func(c *sourceAssessmentConfig) { c.Entry = "bad\x00entry" },
		func(c *sourceAssessmentConfig) { c.Entry = string([]byte{255}) },
		func(c *sourceAssessmentConfig) { c.Entry = strings.Repeat("a", 4097) },
		func(c *sourceAssessmentConfig) { c.Profile = ProfileModelXSD },
		func(c *sourceAssessmentConfig) { c.Profile = "unknown" },
		func(c *sourceAssessmentConfig) { c.Engine = nil },
		func(c *sourceAssessmentConfig) { c.Engine, _ = newModelAssessmentEngine() },
		func(c *sourceAssessmentConfig) { c.ByteOwner = sourceByteOwner(255) },
	} {
		a := adapterAccount(t, validation.Budgets{})
		c := adapterSourceConfig(t, a, "resource", "a.xml", ProfileSourceXSDPreview, sourceBytesExternal)
		mutate(&c)
		reader := &adapterReadCloser{Reader: strings.NewReader(`<opus/>`)}
		run, err := newSourceAssessmentRunWithConfig(context.Background(), reader, DecodeOptions{}, &c, false)
		if !errors.Is(err, validation.ErrInvalidConfiguration) || run != nil || reader.reads != 0 || reader.closes != 0 {
			t.Errorf("invalid config consumed input: run=%v err=%v reads=%d closes=%d", run, err, reader.reads, reader.closes)
		}
		adapterClean(t, a)
	}
	a := adapterAccount(t, validation.Budgets{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	config := adapterSourceConfig(t, a, "resource", "a.xml", ProfileSourceXSDPreview, sourceBytesExternal)
	reader := &adapterReadCloser{Reader: strings.NewReader(`<opus/>`)}
	if _, err := newSourceAssessmentRunWithConfig(ctx, reader, DecodeOptions{}, &config, false); !errors.Is(err, context.Canceled) || reader.reads != 0 {
		t.Fatalf("canceled start: %v reads=%d", err, reader.reads)
	}
	adapterClean(t, a)
}

func TestAssessmentRunStartupAndTerminalCleanup(t *testing.T) {
	engine, err := newSourceAssessmentEngine()
	if err != nil {
		t.Fatal(err)
	}
	a := adapterAccount(t, validation.Budgets{})
	session, err := engine.Begin(context.Background(), validation.RunOptions{DocumentID: "resource", Profile: ProfileSourceXSDPreview, Account: a})
	if err != nil {
		t.Fatal(err)
	}
	setup := adapterUsage(t, a, validation.BudgetBufferedBytes)
	_, _ = session.Finish(context.Background(), validation.Completion{Syntax: validation.SyntaxIncomplete})
	// Session initialization succeeds, then cursor, parser and observer fail in turn.
	for _, limit := range []int64{setup + sourceXMLCursorBytes + 511, setup + sourceXMLCursorBytes + 512 + sourceXMLReaderBytes - 1, setup + sourceXMLCursorBytes + 512 + sourceXMLReaderBytes + 255} {
		a = adapterAccount(t, validation.Budgets{MaxBufferedBytes: limit})
		c := adapterSourceConfig(t, a, "resource", "a.xml", ProfileSourceXSDPreview, sourceBytesExternal)
		reader := &adapterReadCloser{Reader: strings.NewReader(`<opus/>`)}
		run, err := newSourceAssessmentRunWithConfig(context.Background(), reader, DecodeOptions{}, &c, false)
		if !errors.Is(err, validation.ErrResourceLimit) || run == nil || !run.startupReport.Requested || !run.inputFinished || reader.reads != 0 {
			t.Fatalf("startup limit %d: %+v %v reads=%d", limit, run, err, reader.reads)
		}
		if _, err := run.finish(nil); !errors.Is(err, validation.ErrInvalidSession) {
			t.Errorf("repeated finalization accepted: %v", err)
		}
		if reader.closes != 0 {
			t.Fatal("caller reader closed")
		}
		adapterClean(t, a)
	}
	for _, input := range []string{`<opus/>`, `<opus>`, `<opus><bad/></opus>`} {
		a = adapterAccount(t, validation.Budgets{})
		c := adapterSourceConfig(t, a, "resource", "a.xml", ProfileSourceXSDPreview, sourceBytesExternal)
		reader := &adapterReadCloser{Reader: strings.NewReader(input)}
		run, err := newSourceAssessmentRunWithConfig(context.Background(), reader, DecodeOptions{}, &c, false)
		if err != nil {
			t.Fatal(err)
		}
		err = run.drain()
		completion, err2 := run.finishInput(err)
		if err2 != nil {
			t.Fatal(err2)
		}
		// This is the same explicit terminal handoff used by the future core exporter.
		_, _ = run.session.Finish(context.Background(), completion)
		if _, err2 = run.finishInput(nil); !errors.Is(err2, validation.ErrInvalidSession) {
			t.Errorf("repeated preparation: %v", err2)
		}
		if reader.closes != 0 {
			t.Fatal("caller reader closed")
		}
		adapterClean(t, a)
	}
}

func TestAssessmentRunModelPreflightLifecycleAndNoEncoding(t *testing.T) {
	a := adapterAccount(t, validation.Budgets{MaxInputBytes: 1, MaxExternalBytes: 1})
	c := adapterModelConfig(t, a, "package/model", "score.xml", ProfileModelXSD)
	root := NewOpusDocument()
	root.XMLName = xml.Name{Space: "\x00", Local: "\xff"} // ignored by the direct model contract
	root.AddOpus(root)                                    // encoding this cycle cannot produce a finite document
	run, err := newModelAssessmentRun(context.Background(), root, c)
	if err != nil {
		t.Fatal(err)
	}
	status, err := run.walk(modelWalkOptions{})
	if err != nil || !status.HasCycle {
		t.Fatalf("direct bounded graph: %+v %v", status, err)
	}
	completion, err := run.finishInput(err)
	if err != nil {
		t.Fatal(err)
	}
	report, err := run.session.Finish(context.Background(), completion)
	if err != nil || report.Syntax != validation.SyntaxNotAssessed || !report.InputComplete || report.BlockingProblems == 0 {
		t.Fatalf("model lifecycle: %v %+v", err, report)
	}
	adapterAssertQualified(t, report, c.DocumentID, c.Entry, validation.TargetModel)
	if adapterUsage(t, a, validation.BudgetInputBytes) != 0 || adapterUsage(t, a, validation.BudgetExternalBytes) != 0 {
		t.Fatal("model fabricated bytes")
	}
	if _, err := run.finishInput(nil); !errors.Is(err, validation.ErrInvalidSession) {
		t.Errorf("model second finalization: %v", err)
	}
	if _, err := run.walk(modelWalkOptions{}); !errors.Is(err, validation.ErrInvalidSession) {
		t.Errorf("model second traversal: %v", err)
	}
	adapterClean(t, a)
	for _, mutate := range []func(*assessmentRunConfig){
		func(c *assessmentRunConfig) { c.Account = nil },
		func(c *assessmentRunConfig) { c.DocumentID = "" },
		func(c *assessmentRunConfig) { c.Entry = "\x00" },
		func(c *assessmentRunConfig) { c.Profile = ProfileSourceLocal },
		func(c *assessmentRunConfig) { c.Engine = nil },
	} {
		config := c
		mutate(&config)
		if _, err := newModelAssessmentRun(context.Background(), root, config); !errors.Is(err, validation.ErrInvalidConfiguration) {
			t.Errorf("model invalid config: %v", err)
		}
		adapterClean(t, a)
	}
}

type adapterFailReader struct{ cause error }

func (r adapterFailReader) Read([]byte) (int, error) { return 0, r.cause }

func TestAssessmentRunInterruptionsAndFinalizationBudgets(t *testing.T) {
	readErr := errors.New("distinct adapter reader failure")
	for _, tc := range []struct {
		name   string
		reader io.Reader
		cancel bool
		want   error
	}{
		{"cancel-before-drain", strings.NewReader(`<opus/>`), true, context.Canceled},
		{"reader-error", adapterFailReader{readErr}, false, readErr},
		{"no-progress", &byteOwnerPauseReader{Reader: strings.NewReader(`<opus/>`), pauses: 1000}, false, io.ErrNoProgress},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := adapterAccount(t, validation.Budgets{})
			config := adapterSourceConfig(t, a, "resource", "source.xml", ProfileSourceXSDPreview, sourceBytesExternal)
			reader := &adapterReadCloser{Reader: tc.reader}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			run, err := newSourceAssessmentRunWithConfig(ctx, reader, DecodeOptions{}, &config, false)
			if err != nil {
				t.Fatal(err)
			}
			if tc.cancel {
				cancel()
			}
			drainErr := run.drain()
			// The adapter retains its own source failure even if a future orchestration
			// layer omits the cause while preparing terminal export.
			report, finishErr := run.finish(nil)
			if !errors.Is(drainErr, tc.want) || !errors.Is(finishErr, tc.want) || report.InputComplete {
				t.Fatalf("lost cause: drain=%v finish=%v report=%+v", drainErr, finishErr, report)
			}
			if reader.closes != 0 {
				t.Fatal("closed caller reader")
			}
			if err = run.drain(); !errors.Is(err, validation.ErrInvalidSession) {
				t.Errorf("drain after finalization: %v", err)
			}
			adapterClean(t, a)
		})
	}
	// Derive the exact pre-finalization work from the real unchanged profile.
	a := adapterAccount(t, validation.Budgets{})
	config := adapterSourceConfig(t, a, "resource", "source.xml", ProfileSourceXSDPreview, sourceBytesExternal)
	run, err := newSourceAssessmentRunWithConfig(context.Background(), strings.NewReader(`<opus/>`), DecodeOptions{}, &config, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = run.drain(); err != nil {
		t.Fatal(err)
	}
	beforeFinish := adapterUsage(t, a, validation.BudgetRuleSteps)
	if _, err = run.finish(nil); err != nil {
		t.Fatal(err)
	}
	if adapterUsage(t, a, validation.BudgetRuleSteps) <= beforeFinish {
		t.Fatal("fixture has no final evaluation")
	}
	a = adapterAccount(t, validation.Budgets{MaxRuleSteps: beforeFinish})
	config = adapterSourceConfig(t, a, "resource", "source.xml", ProfileSourceXSDPreview, sourceBytesExternal)
	run, err = newSourceAssessmentRunWithConfig(context.Background(), strings.NewReader(`<opus/>`), DecodeOptions{}, &config, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = run.drain(); err != nil {
		t.Fatalf("budget failed before finish: %v", err)
	}
	report, err := run.finish(nil)
	if !errors.Is(err, validation.ErrResourceLimit) || report.AssessmentComplete || !report.InputComplete {
		t.Fatalf("finalization exhaustion: %v %+v", err, report)
	}
	if _, err = run.finish(nil); !errors.Is(err, validation.ErrInvalidSession) {
		t.Errorf("finished twice: %v", err)
	}
	adapterClean(t, a)

	for _, beforeWalk := range []bool{false, true} {
		a = adapterAccount(t, validation.Budgets{})
		config := adapterModelConfig(t, a, "model-resource", "current.xml", ProfileModelXSD)
		ctx, cancel := context.WithCancel(context.Background())
		model, err := newModelAssessmentRun(ctx, NewOpusDocument(), config)
		if err != nil {
			t.Fatal(err)
		}
		if beforeWalk {
			cancel()
		}
		_, walkErr := model.walk(modelWalkOptions{})
		cancel()
		report, err := model.finish(nil)
		if !errors.Is(err, context.Canceled) || report.AssessmentComplete || (beforeWalk && !errors.Is(walkErr, context.Canceled)) {
			t.Fatalf("model cancel: %v/%v %+v", walkErr, err, report)
		}
		adapterClean(t, a)
	}
}
