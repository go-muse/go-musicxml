package musicxml

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

type sourceDeclarationTestSink struct {
	recordingSourceFacts
	declarations []validation.SourceEntityDeclarations
	events       []string
	fail         error
	inspect      func(validation.SourceEntityDeclarations) error
}

func (s *sourceDeclarationTestSink) OpenNode(ctx context.Context, h validation.NodeHeader) error {
	s.events = append(s.events, "open")
	return s.recordingSourceFacts.OpenNode(ctx, h)
}
func (s *sourceDeclarationTestSink) Attribute(ctx context.Context, a validation.AttributeFact) error {
	s.events = append(s.events, "attribute")
	return s.recordingSourceFacts.Attribute(ctx, a)
}
func (s *sourceDeclarationTestSink) CloseNode(ctx context.Context, e validation.NodeEnd) error {
	s.events = append(s.events, "close")
	return s.recordingSourceFacts.CloseNode(ctx, e)
}
func (s *sourceDeclarationTestSink) SourceEntityDeclarations(_ context.Context, f validation.SourceEntityDeclarations) error {
	s.events = append(s.events, "declarations")
	if s.inspect != nil {
		if err := s.inspect(f); err != nil {
			return err
		}
	}
	if s.fail != nil {
		return s.fail
	}
	f.Declarations = append([]validation.SourceEntityDeclaration(nil), f.Declarations...)
	for i := range f.Declarations {
		if p := f.Declarations[i].Origin.Value.Position; p != nil {
			owned := *p
			f.Declarations[i].Origin.Value.Position = &owned
		}
	}
	s.declarations = append(s.declarations, f)
	return nil
}
func sourceDeclarationReader(t *testing.T, ctx context.Context, input string, internal bool, budgets validation.Budgets) (*sourceXMLTokenReader, *validation.Account) {
	t.Helper()
	account := dtdStoreAccount(t, budgets)
	r, err := newSourceXMLTokenReader(strings.NewReader(input), sourceXMLConfig{Context: ctx, Account: account, InternalDTD: internal})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = r.Close()
		if used, _ := account.Usage(validation.BudgetBufferedBytes); used != 0 {
			t.Errorf("live buffered charge=%d", used)
		}
	})
	return r, account
}
func sourceDeclarationReadRoot(t *testing.T, r *sourceXMLTokenReader) {
	t.Helper()
	for {
		token, err := r.Token()
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := token.(xml.StartElement); ok {
			return
		}
	}
}
func TestSourceDeclarationExportEffectiveFirstBindings(t *testing.T) {
	input := `<!DOCTYPE r [<!ENTITY parsed 'text'><!ENTITY parsed SYSTEM 'ignored' NDATA n><!ENTITY image SYSTEM 'image' NDATA missing><!ENTITY image 'ignored'><!ENTITY % image 'parameter'><!ENTITY external SYSTEM 'unread'><!ENTITY % p "<!ENTITY generated SYSTEM 'g' NDATA n>">%p;]><r/>`
	r, _ := sourceDeclarationReader(t, context.Background(), input, true, validation.Budgets{})
	sourceDeclarationReadRoot(t, r)
	sink := &sourceDeclarationTestSink{}
	if err := r.exportEntityDeclarations(context.Background(), "document", "entry.xml", sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.declarations) != 1 {
		t.Fatalf("snapshot count=%d", len(sink.declarations))
	}
	got := sink.declarations[0]
	if got.DocumentID != "document" || got.Complete.State != validation.EvidenceKnown || !got.Complete.Value {
		t.Fatalf("snapshot=%+v", got)
	}
	if len(got.Declarations) != 4 {
		t.Fatalf("declarations=%+v", got.Declarations)
	}
	for i, want := range []struct {
		name     string
		unparsed bool
		sequence uint64
	}{{"parsed", false, 1}, {"image", true, 3}, {"external", false, 6}, {"generated", true, 8}} {
		d := got.Declarations[i]
		if d.Name != want.name || d.Unparsed != want.unparsed || d.Sequence != want.sequence {
			t.Errorf("declaration %d=%+v want=%+v", i, d, want)
		}
		if i == 3 {
			if d.Origin.State != validation.EvidenceUnknown || d.Origin.Value.Position != nil {
				t.Errorf("generated origin=%+v", d.Origin)
			}
			continue
		}
		p := d.Origin.Value.Position
		if d.Origin.State != validation.EvidenceKnown || p == nil || p.ByteOffset != int64(strings.Index(input, "<!ENTITY "+want.name+" ")) || p.CoordinateSpace != "original-bytes" || d.Origin.Value.DocumentID != "document" || d.Origin.Value.Entry != "entry.xml" {
			t.Errorf("physical origin=%+v", d.Origin)
		}
	}
	if r.Summary().InputComplete {
		t.Error("test should export before document EOF")
	}
	_ = r.Close()
	if got.Declarations[0].Name != "parsed" || got.Declarations[0].Origin.Value.Position == nil {
		t.Error("owned snapshot lost on scanner close")
	}
}
func TestSourceDeclarationExportStandaloneCutoffAndClosure(t *testing.T) {
	for _, standalone := range []string{"", "no", "yes"} {
		t.Run("standalone-"+standalone, func(t *testing.T) {
			prefix := ""
			if standalone != "" {
				prefix = `<?xml version="1.0" standalone="` + standalone + `"?>`
			}
			input := prefix + `<!DOCTYPE r [<!ENTITY early SYSTEM 'e' NDATA n><!ENTITY % unread SYSTEM 'u'>%unread;<!ENTITY late SYSTEM 'l' NDATA n>]><r/>`
			r, _ := sourceDeclarationReader(t, context.Background(), input, true, validation.Budgets{})
			sourceDeclarationReadRoot(t, r)
			sink := &sourceDeclarationTestSink{}
			if err := r.exportEntityDeclarations(context.Background(), "source", "", sink); err != nil {
				t.Fatal(err)
			}
			f := sink.declarations[0]
			expected := 1
			if standalone == "yes" {
				expected = 2
			}
			if len(f.Declarations) != expected || f.Declarations[0].Name != "early" || f.Complete.State != validation.EvidenceKnown || f.Complete.Value || f.Complete.Reason == "" {
				t.Fatalf("facts=%+v", f)
			}
		})
	}
	for _, input := range []string{`<r/>`, `<!DOCTYPE r [<!ENTITY unused '&unused;'>]><r/>`, `<!DOCTYPE r [<!ENTITY amp 'wrong'>]><r/>`} {
		t.Run(input, func(t *testing.T) {
			r, _ := sourceDeclarationReader(t, context.Background(), input, true, validation.Budgets{})
			sourceDeclarationReadRoot(t, r)
			sink := &sourceDeclarationTestSink{}
			if err := r.exportEntityDeclarations(context.Background(), "source", "", sink); err != nil {
				t.Fatal(err)
			}
			if f := sink.declarations[0]; f.Complete.State != validation.EvidenceKnown || !f.Complete.Value {
				t.Errorf("declaration closure tied to replacement/EOF: %+v", f)
			}
		})
	}
}
func TestSourceDeclarationExportNeverInventsCapability(t *testing.T) {
	for _, internal := range []bool{false, true} {
		t.Run(map[bool]string{false: "not-elected", true: "store-missing"}[internal], func(t *testing.T) {
			r, _ := sourceDeclarationReader(t, context.Background(), `<r/>`, internal, validation.Budgets{})
			sourceDeclarationReadRoot(t, r)
			if internal {
				r.doctype = true
			}
			sink := &sourceDeclarationTestSink{}
			err := r.exportEntityDeclarations(context.Background(), "source", "", sink)
			if !internal {
				if !errors.Is(err, validation.ErrInvalidConfiguration) {
					t.Errorf("missing scanner capability=%v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if f := sink.declarations[0]; f.Complete.State == validation.EvidenceKnown && f.Complete.Value {
				t.Errorf("missing store proved empty: %+v", f)
			}
		})
	}
}
func TestSourceDeclarationObserverPublishesOnceBeforeRootAttributes(t *testing.T) {
	for _, input := range []string{`<!DOCTYPE r [<!ENTITY image SYSTEM 'image' NDATA n>]><r a="value"><child/></r>`, `<!DOCTYPE r [<!ENTITY image SYSTEM 'image' NDATA n>]><r a="value"><child/>`} {
		t.Run(input, func(t *testing.T) {
			r, account := sourceDeclarationReader(t, context.Background(), input, true, validation.Budgets{})
			sink := &sourceDeclarationTestSink{}
			observer, err := newSourceFactObserver(context.Background(), sink, account)
			if err != nil {
				t.Fatal(err)
			}
			observer.sourceDeclarations = true
			defer observer.close()
			source := &observingXMLSource{ctx: context.Background(), lexical: r, decoder: xml.NewTokenDecoder(r), observer: observer}
			for {
				_, err = source.Token()
				if err != nil {
					break
				}
			}
			if len(sink.events) < 3 || !reflect.DeepEqual(sink.events[:3], []string{"open", "declarations", "attribute"}) || len(sink.declarations) != 1 {
				t.Errorf("events=%v declarations=%+v", sink.events, sink.declarations)
			}
			if !strings.HasSuffix(input, "</r>") && errors.Is(err, io.EOF) {
				t.Error("malformed suffix lost")
			}
		})
	}
}
func TestSourceDeclarationObserverMissingIngressAndMalformedPrefix(t *testing.T) {
	r, account := sourceDeclarationReader(t, context.Background(), `<r/>`, true, validation.Budgets{})
	observer, err := newSourceFactObserver(context.Background(), &recordingSourceFacts{}, account)
	if err != nil {
		t.Fatal(err)
	}
	observer.sourceDeclarations = true
	defer observer.close()
	source := &observingXMLSource{ctx: context.Background(), lexical: r, decoder: xml.NewTokenDecoder(r), observer: observer}
	if _, err := source.Token(); !errors.Is(err, validation.ErrInvalidConfiguration) {
		t.Errorf("missing elected ingress=%v", err)
	}
	r2, account2 := sourceDeclarationReader(t, context.Background(), `<!DOCTYPE r [<!ENTITY image SYSTEM 'image' NDATA n><!BROKEN>]><r/>`, true, validation.Budgets{})
	sink := &sourceDeclarationTestSink{}
	observer2, err := newSourceFactObserver(context.Background(), sink, account2)
	if err != nil {
		t.Fatal(err)
	}
	observer2.sourceDeclarations = true
	defer observer2.close()
	source2 := &observingXMLSource{ctx: context.Background(), lexical: r2, decoder: xml.NewTokenDecoder(r2), observer: observer2}
	if _, err := source2.Token(); err == nil || len(sink.declarations) != 0 {
		t.Errorf("malformed prefix err=%v snapshots=%+v", err, sink.declarations)
	}
}
func TestSourceDeclarationExportFailureAndCancellationRelease(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r, account := sourceDeclarationReader(t, ctx, `<!DOCTYPE r [<!ENTITY image SYSTEM 'image' NDATA n>]><r/>`, true, validation.Budgets{})
	sourceDeclarationReadRoot(t, r)
	before, _ := account.Usage(validation.BudgetBufferedBytes)
	sentinel := errors.New("sink rejected snapshot")
	for _, sink := range []*sourceDeclarationTestSink{{fail: sentinel}, {inspect: func(validation.SourceEntityDeclarations) error { cancel(); return ctx.Err() }}} {
		err := r.exportEntityDeclarations(ctx, "source", "", sink)
		if !errors.Is(err, sentinel) && !errors.Is(err, context.Canceled) {
			t.Errorf("error=%v", err)
		}
		if after, _ := account.Usage(validation.BudgetBufferedBytes); after != before {
			t.Errorf("temporary charge leaked: before=%d after=%d", before, after)
		}
	}
	sink := &sourceDeclarationTestSink{}
	if err := r.exportEntityDeclarations(ctx, "source", "", sink); !errors.Is(err, context.Canceled) || len(sink.events) != 0 {
		t.Errorf("pre-cancel err=%v events=%v", err, sink.events)
	}
}
func TestSourceDeclarationExportExactAndOneShortBudgets(t *testing.T) {
	const input = `<!DOCTYPE r [<!ENTITY parsed 'text'><!ENTITY image SYSTEM 'image' NDATA n>]><r/>`
	r, account := sourceDeclarationReader(t, context.Background(), input, true, validation.Budgets{})
	sourceDeclarationReadRoot(t, r)
	before, _ := account.Usage(validation.BudgetBufferedBytes)
	stepsBefore, _ := account.Usage(validation.BudgetRuleSteps)
	var temporary int64
	sink := &sourceDeclarationTestSink{inspect: func(validation.SourceEntityDeclarations) error {
		during, _ := account.Usage(validation.BudgetBufferedBytes)
		temporary = during - before
		return nil
	}}
	if err := r.exportEntityDeclarations(context.Background(), "source", "", sink); err != nil {
		t.Fatal(err)
	}
	stepsAfter, _ := account.Usage(validation.BudgetRuleSteps)
	if temporary <= 0 || stepsAfter <= stepsBefore {
		t.Fatalf("unmetered snapshot temporary=%d steps=%d", temporary, stepsAfter-stepsBefore)
	}
	for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetRuleSteps} {
		for _, short := range []int64{0, 1} {
			t.Run(string(kind)+map[int64]string{0: "-exact", 1: "-one-short"}[short], func(t *testing.T) {
				r, a := sourceDeclarationReader(t, context.Background(), input, true, validation.Budgets{})
				sourceDeclarationReadRoot(t, r)
				need := temporary
				limit := a.Limits().MaxBufferedBytes
				if kind == validation.BudgetRuleSteps {
					need = stepsAfter - stepsBefore
					limit = a.Limits().MaxRuleSteps
				}
				used, _ := a.Usage(kind)
				filler := limit - used - need + short
				if err := a.Charge(kind, filler); err != nil {
					t.Fatal(err)
				}
				if kind == validation.BudgetBufferedBytes {
					defer a.Release(kind, filler)
				}
				sink := &sourceDeclarationTestSink{}
				err := r.exportEntityDeclarations(context.Background(), "source", "", sink)
				if short == 0 {
					if err != nil || len(sink.declarations) != 1 {
						t.Errorf("exact err=%v snapshots=%d", err, len(sink.declarations))
					}
				} else if !errors.Is(err, validation.ErrResourceLimit) || len(sink.declarations) != 0 {
					t.Errorf("one-short err=%v snapshots=%d", err, len(sink.declarations))
				}
			})
		}
	}
}

func TestSourceDeclarationExportCancellationSweep(t *testing.T) {
	for at := 1; at <= 16; at++ {
		r, account := sourceDeclarationReader(t, context.Background(), `<!DOCTYPE r [<!ENTITY a 'parsed'><!ENTITY a 'ignored'><!ENTITY b SYSTEM 'b' NDATA n><!ENTITY % p 'parameter'>]><r/>`, true, validation.Budgets{})
		sourceDeclarationReadRoot(t, r)
		before, _ := account.Usage(validation.BudgetBufferedBytes)
		ctx, cancel := context.WithCancel(context.Background())
		counted := &dtdAuditCountingContext{Context: ctx, cancel: cancel, at: at}
		sink := &sourceDeclarationTestSink{}
		err := r.exportEntityDeclarations(counted, "source", "", sink)
		cancel()
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("at=%d err=%v", at, err)
		}
		if err != nil && len(sink.declarations) != 0 {
			t.Errorf("at=%d partial snapshot published", at)
		}
		if after, _ := account.Usage(validation.BudgetBufferedBytes); after != before {
			t.Errorf("at=%d leaked temporary buffers: before=%d after=%d", at, before, after)
		}
	}
}

func sourceDeclarationLifecycleEngine(t *testing.T) *validation.Engine {
	t.Helper()
	assembly := validation.AssemblyMusicXML
	schema, err := validation.CompileSchema(assembly, validation.SchemaData{Elements: map[validation.SchemaName]*validation.SchemaElement{{Local: "r"}: {Name: validation.SchemaName{Local: "r"}, Type: validation.SchemaTypeRef{Name: validation.SchemaName{Space: "http://www.w3.org/2001/XMLSchema", Local: "ENTITY"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := validation.NewXSDPlanWithSourceDeclarations(schema, validation.XSDCheckScalar)
	if err != nil {
		t.Fatal(err)
	}
	key := validation.RuleKey{ID: "test.source-declaration-lifecycle", Revision: 1}
	rule := validation.RuleDefinition{Key: key, Targets: []validation.Target{validation.TargetSource}, Scope: validation.ScopeNode, Category: validation.CategoryNormative, Class: validation.ClassMandatory, Role: validation.RolePredicate, Selector: validation.Selector{Assembly: &assembly}, Predicate: string(key.ID), Text: validation.RuleText{Title: "Source declaration lifecycle", Explanation: "Retain admitted declaration facts through source cleanup.", Message: "Selected scalar assessment."}, Sources: []validation.SourceRef{{IDs: []string{"XSD-1.0"}, URL: "https://www.w3.org/TR/xmlschema-1/", Locator: "cvc-datatype-valid"}}}
	profile := validation.Profile{ID: "source-declaration-lifecycle", Target: validation.TargetSource, MusicXMLVersion: "4.0", RegistryCommit: "3e33e4c80aa2a46ec323a5c2ea2c473e9f47905c", SupportedRoots: []validation.QName{{Local: "r"}}, NormativeClosure: "Source declaration lifetime regression.", Required: []validation.RuleKey{key}, Support: []validation.RuleSupport{{Rule: key, State: validation.ImplementationAvailable}}}
	catalog, err := validation.NewCatalog([]validation.RuleDefinition{rule}, []validation.Profile{profile}, []validation.PlanBinding{{ID: rule.Predicate, Plan: plan}})
	if err != nil {
		t.Fatal(err)
	}
	engine, err := validation.NewEngine(catalog)
	if err != nil {
		t.Fatal(err)
	}
	return engine
}
func TestSourceDeclarationSessionFinalizesAfterScannerCleanup(t *testing.T) {
	engine := sourceDeclarationLifecycleEngine(t)
	for _, tc := range []struct {
		name, input string
		cancel      bool
	}{
		{"complete", `<!DOCTYPE r [<!ENTITY image SYSTEM 'i' NDATA n>]><r>image</r>`, false},
		{"malformed-suffix", `<!DOCTYPE r [<!ENTITY image SYSTEM 'i' NDATA n>]><r>image</r><`, false},
		{"malformed-prefix", `<!DOCTYPE r [<!ENTITY image SYSTEM 'i' NDATA n><!BROKEN>]><r>image</r>`, false},
		{"canceled-prefix", `<!DOCTYPE r [<!ENTITY image SYSTEM 'i' NDATA n>]><r>image</r>`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reader, account := sourceDeclarationReader(t, ctx, tc.input, true, validation.Budgets{})
			session, err := engine.Begin(ctx, validation.RunOptions{DocumentID: "source", Profile: "source-declaration-lifecycle", Account: account})
			if err != nil {
				t.Fatal(err)
			}
			observer, err := newSourceFactObserver(ctx, session, account)
			if err != nil {
				t.Fatal(err)
			}
			observer.sourceDeclarations = true
			source := &observingXMLSource{ctx: ctx, lexical: reader, decoder: xml.NewTokenDecoder(reader), observer: observer}
			run := &sourceAssessmentRun{ctx: ctx, documentID: "source", account: account, session: session, source: source}
			for {
				token, e := source.Token()
				err = e
				if err != nil {
					break
				}
				if tc.cancel {
					if _, ok := token.(xml.StartElement); ok {
						cancel()
					}
				}
			}
			cause := err
			if errors.Is(cause, io.EOF) {
				cause = nil
			}
			completion, err := run.finishInput(cause)
			if err != nil {
				t.Fatal(err)
			}
			if !reader.closed || observer.retained != 0 {
				t.Fatal("scanner/observer not cleaned before fallback Finish")
			}
			if used, _ := account.Usage(validation.BudgetBufferedBytes); used <= 0 {
				t.Fatal("session failed to retain owned snapshot/facts")
			}
			_, finishErr := session.Finish(ctx, completion)
			if tc.cancel && !errors.Is(finishErr, context.Canceled) {
				t.Errorf("cancellation cause lost: %v", finishErr)
			}
			if used, _ := account.Usage(validation.BudgetBufferedBytes); used != 0 {
				t.Errorf("final live buffers=%d", used)
			}
			if depth, _ := account.Usage(validation.BudgetDepth); depth != 0 {
				t.Errorf("final live depth=%d", depth)
			}
		})
	}
}

type sourceDeclarationRejectingSink struct {
	sourceDeclarationTestSink
	rejection error
}

func (s *sourceDeclarationRejectingSink) OpenNode(context.Context, validation.NodeHeader) error {
	return s.rejection
}
func TestSourceDeclarationObserverRequiresRootAdmissionAndSharedAccount(t *testing.T) {
	sentinel := errors.New("root assembly rejected")
	reader, account := sourceDeclarationReader(t, context.Background(), `<r/>`, true, validation.Budgets{})
	sink := &sourceDeclarationRejectingSink{rejection: sentinel}
	observer, err := newSourceFactObserver(context.Background(), sink, account)
	if err != nil {
		t.Fatal(err)
	}
	defer observer.close()
	observer.sourceDeclarations = true
	source := &observingXMLSource{ctx: context.Background(), lexical: reader, decoder: xml.NewTokenDecoder(reader), observer: observer}
	if _, err := source.Token(); !errors.Is(err, sentinel) || len(sink.declarations) != 0 {
		t.Errorf("root rejection err=%v declarations=%+v", err, sink.declarations)
	}
	other := dtdStoreAccount(t, validation.Budgets{})
	mismatched, err := newSourceFactObserver(context.Background(), &sourceDeclarationTestSink{}, other)
	if err != nil {
		t.Fatal(err)
	}
	defer mismatched.close()
	if err := mismatched.publishSourceEntityDeclarations(reader); !errors.Is(err, validation.ErrInvalidConfiguration) {
		t.Errorf("mismatched account=%v", err)
	}
}
