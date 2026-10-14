package musicxml

import (
	"context"
	"encoding/xml"
	"errors"
	"github.com/go-muse/go-musicxml/validation"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestDTDAuditIntegratedDefaultNamespaceMatrix(t *testing.T) {
	for _, tc := range []struct {
		name, source, want string
		syntax             bool
	}{
		{"default-on-own-qname", `<!DOCTYPE p:r [<!ATTLIST p:r xmlns:p CDATA 'urn:p' a CDATA 'v'>]><p:r/>`, "urn:p:r", false},
		{"default-does-not-qualify-attribute", `<!DOCTYPE r [<!ATTLIST r xmlns CDATA 'urn:r' a CDATA 'v'>]><r/>`, "urn:r:r", false},
		{"lexical-prefix-distinct", `<!DOCTYPE r [<!ATTLIST p:r a CDATA 'bad'>]><q:r xmlns:q='u' xmlns:p='u'/>`, "u:r", false},
		{"fixed-mismatch-is-validity", `<!DOCTYPE r [<!ATTLIST r a CDATA #FIXED 'one'>]><r a='two'/>`, ":r", false},
		{"required-missing-is-validity", `<!DOCTYPE r [<!ATTLIST r a CDATA #REQUIRED>]><r/>`, ":r", false},
		{"default-collision", `<!DOCTYPE r [<!ATTLIST r p:a CDATA 'x' q:a CDATA 'y'>]><r xmlns:p='u' xmlns:q='u'/>`, "", true},
		{"reserved-default-xml", `<!DOCTYPE r [<!ATTLIST r xmlns:xml CDATA 'u'>]><r/>`, "", true},
		{"duplicate-physical-stays-error", `<!DOCTYPE r [<!ATTLIST r a CDATA 'x'>]><r a='a' a='b'/>`, "", true},
		{"explicit-suppresses-invalid-namespace-default", `<!DOCTYPE r [<!ATTLIST r xmlns:xml CDATA 'u'>]><r xmlns:xml='http://www.w3.org/XML/1998/namespace'/>`, ":r", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := adapterAccount(t, validation.Budgets{})
			r, err := newSourceXMLTokenReader(strings.NewReader(tc.source), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true})
			if err != nil {
				t.Fatal(err)
			}
			dec := xml.NewTokenDecoder(r)
			var start xml.StartElement
			for err == nil {
				var tok xml.Token
				tok, err = dec.Token()
				if e, ok := tok.(xml.StartElement); ok {
					start = e
				}
			}
			r.Close()
			adapterClean(t, a)
			var syntax *xml.SyntaxError
			if tc.syntax {
				if !errors.As(err, &syntax) {
					t.Fatalf("want syntax got %v", err)
				}
				return
			}
			if err != io.EOF {
				t.Fatal(err)
			}
			if start.Name.Space+":"+start.Name.Local != tc.want {
				t.Fatalf("root=%+v", start.Name)
			}
			for _, at := range start.Attr {
				if at.Name.Local == "a" && at.Name.Space != "" {
					t.Fatalf("unprefixed attribute qualified: %+v", at)
				}
			}
			if tc.name == "lexical-prefix-distinct" && len(start.Attr) != 2 {
				t.Fatalf("wrong lexical default: %+v", start.Attr)
			}
		})
	}
}

const dtdAuditPublicRepeated = `<!DOCTYPE opus [<!ATTLIST score xlink:href CDATA 'a.xml'>]><opus xmlns:xlink='http://www.w3.org/1999/xlink'><score/><score/><score/></opus>`

func TestDTDAuditPublicDefaultMultiplicityAndOwnership(t *testing.T) {
	a := adapterAccount(t, validation.Budgets{})
	c := adapterSourceConfig(t, a, "audit", "", ProfileSourceInternalDTD, sourceBytesPrimary)
	report, err := adapterInspect(t, strings.NewReader(dtdAuditPublicRepeated), c)
	if err != nil || !report.Satisfied() {
		t.Fatalf("error=%v syntax=%s findings=%+v", err, report.Syntax, report.Findings)
	}
	adapterClean(t, a)
	if used := adapterUsage(t, a, validation.BudgetAttributes); used != 4 {
		t.Fatalf("effective attributes charged %d want4", used)
	}
	if used := adapterUsage(t, a, validation.BudgetNodes); used != 4 {
		t.Fatalf("nodes charged %d want4", used)
	}
	if used := adapterUsage(t, a, validation.BudgetInputBytes); used != int64(len(dtdAuditPublicRepeated)) {
		t.Fatalf("bytes=%d", used)
	}
	var defaults []*validation.SourceOriginContextResult
	for _, row := range report.Context {
		o := row.SourceOrigin
		if o != nil && o.Kind == validation.SourceOriginDefault {
			defaults = append(defaults, o)
		}
	}
	if len(defaults) != 3 {
		t.Fatalf("default origins=%d", len(defaults))
	}
	for _, o := range defaults {
		if o.Specified.Value || o.Physical.State != validation.EvidenceAbsent || o.Declaration.State != validation.EvidenceKnown || o.Use.State != validation.EvidenceKnown || o.Declaration.Value.Position == nil || o.Use.Value.Position == nil {
			t.Fatalf("origin %+v", o)
		}
	}
	before := defaults[1].Declaration.Value.Position.ByteOffset
	defaults[0].Declaration.Value.Position.ByteOffset = 999
	if defaults[1].Declaration.Value.Position.ByteOffset != before {
		t.Fatal("different default occurrences alias public owned sites")
	}
	// Report mutation cannot corrupt later assessments through the cached engine.
	report2, err := adapterInspect(t, strings.NewReader(dtdAuditPublicRepeated), c)
	if err != nil || !report2.Satisfied() {
		t.Fatalf("repeat=%v", err)
	}
	adapterClean(t, a)
	for _, row := range report2.Context {
		if o := row.SourceOrigin; o != nil && o.Declaration.State == validation.EvidenceKnown && o.Declaration.Value.Position.ByteOffset == 999 {
			t.Fatal("report mutation reached cached state")
		}
	}
}

func TestDTDAuditPublicBudgetAndCancellationCleanup(t *testing.T) {
	for _, limits := range []validation.Budgets{{MaxAttributes: 3}, {MaxNodes: 3}, {MaxContextFacts: 1}, {MaxEvents: 1}, {MaxBufferedBytes: 1}, {MaxScalarBytes: 1}} {
		a := adapterAccount(t, limits)
		c := adapterSourceConfig(t, a, "audit", "", ProfileSourceInternalDTD, sourceBytesPrimary)
		report, err := adapterInspect(t, strings.NewReader(dtdAuditPublicRepeated), c)
		if !errors.Is(err, validation.ErrResourceLimit) || report.Satisfied() {
			t.Fatalf("limits=%+v err=%v", limits, err)
		}
		adapterClean(t, a)
	}
	for _, at := range []int{1, 10, 100, 500, 1000, 2000, 4000} {
		ctx, cancel := context.WithCancel(context.Background())
		counted := &dtdAuditCountingContext{Context: ctx, cancel: cancel, at: at}
		a := adapterAccount(t, validation.Budgets{})
		c := adapterSourceConfig(t, a, "audit", "", ProfileSourceInternalDTD, sourceBytesPrimary)
		run, err := newSourceAssessmentRunWithConfig(counted, strings.NewReader(dtdAuditPublicRepeated), DecodeOptions{}, &c, false)
		if err == nil {
			cause := run.drain()
			_, finish := run.finish(cause)
			err = errors.Join(cause, finish)
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel %d: %v", at, err)
		}
		adapterClean(t, a)
		cancel()
	}
}

func TestDTDAuditPublicPartialProvenanceIsNotComplete(t *testing.T) {
	source := `<!DOCTYPE opus [<!ENTITY % p "<!ATTLIST opus version CDATA '4.0'>">%p;]><opus/>`
	report, err := InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceInternalDTD})
	if err != nil || report.Satisfied() || report.AssessmentComplete {
		t.Fatalf("err=%v satisfied=%v complete=%v", err, report.Satisfied(), report.AssessmentComplete)
	}
	found := false
	for _, row := range report.Context {
		o := row.SourceOrigin
		if o != nil && o.Kind == validation.SourceOriginDefault {
			found = true
			if o.Specified.Value || o.Physical.State == validation.EvidenceKnown || o.Declaration.State != validation.EvidenceUnknown {
				t.Fatalf("fabricated PE default origin %+v", o)
			}
		}
	}
	if !found {
		t.Fatal("missing effective default occurrence")
	}
}

func TestDTDAuditPublicSnapshotExportsOwnedOrigins(t *testing.T) {
	ctx := context.Background()
	a := adapterAccount(t, validation.Budgets{})
	c := adapterSourceConfig(t, a, "audit", "", ProfileSourceInternalDTD, sourceBytesPrimary)
	run, err := newSourceAssessmentRunWithConfig(ctx, strings.NewReader(dtdAuditPublicRepeated), DecodeOptions{}, &c, false)
	if err != nil {
		t.Fatal(err)
	}
	cause := run.drain()
	completion, err := run.finishInput(cause)
	if err != nil || cause != nil {
		t.Fatalf("input=%v %v", cause, err)
	}
	report, snapshot, err := run.session.FinishWithResourceSnapshot(ctx, completion)
	if err != nil || snapshot == nil || !report.AssessmentComplete || !report.Satisfied() {
		t.Fatalf("snapshot=%v err=%v complete=%v", snapshot, err, report.AssessmentComplete)
	}
	origins := 0
	for _, row := range report.Context {
		if row.SourceOrigin != nil {
			origins++
		}
	}
	if origins != 8 || snapshot.ContextCount() != origins {
		t.Fatalf("report/snapshot lost origins: report=%d snapshot=%d want8", origins, snapshot.ContextCount())
	}
	canonical, err := snapshot.Report(ctx)
	if err != nil || !reflect.DeepEqual(canonical, report) {
		t.Fatalf("snapshot changed original report: %v", err)
	}
	for i, row := range report.Context {
		got, ok, err := snapshot.ContextAt(ctx, i)
		if err != nil || !ok || !reflect.DeepEqual(got, row) {
			t.Fatalf("snapshot origin %d changed: %+v err=%v", i, got, err)
		}
		if got.SourceOrigin.Physical.Value.Position != nil {
			got.SourceOrigin.Physical.Value.Position.ByteOffset = -1
		}
		if got.SourceOrigin.Declaration.Value.Position != nil {
			got.SourceOrigin.Declaration.Value.Position.ByteOffset = -1
		}
		again, _, err := snapshot.ContextAt(ctx, i)
		if err != nil || !reflect.DeepEqual(again, row) {
			t.Fatalf("origin %d aliases a returned copy: %v", i, err)
		}
	}
	if err = snapshot.Close(); err != nil {
		t.Fatal(err)
	}
	adapterClean(t, a)
}

func TestDTDAuditPublicExactAttributeBudgetStaysAggregate(t *testing.T) {
	a := adapterAccount(t, validation.Budgets{MaxAttributes: 4})
	c := adapterSourceConfig(t, a, "audit", "", ProfileSourceInternalDTD, sourceBytesPrimary)
	first, err := adapterInspect(t, strings.NewReader(dtdAuditPublicRepeated), c)
	if err != nil || !first.Satisfied() {
		t.Fatalf("exact four: %v", err)
	}
	adapterClean(t, a)
	second, err := adapterInspect(t, strings.NewReader(dtdAuditPublicRepeated), c)
	if !errors.Is(err, validation.ErrResourceLimit) || second.Satisfied() {
		t.Fatalf("reused account reset defaults: %v", err)
	}
	adapterClean(t, a)
}

func TestDTDAuditPublicSnapshotKindsPreserveOriginalReport(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		kind         validation.SourceOriginKind
		attribute    bool
	}{
		{"physical", `<opus/>`, validation.SourceOriginPhysical, false},
		{"default", `<!DOCTYPE opus [<!ATTLIST opus version CDATA '4.0'>]><opus/>`, validation.SourceOriginDefault, true},
		{"entity-value-physical-attribute", `<!DOCTYPE opus [<!ENTITY version '4.0'>]><opus version='&version;'/>`, validation.SourceOriginPhysical, true},
		{"entity-element", `<!DOCTYPE opus [<!ENTITY score "<score xlink:href='a.xml'/>">]><opus xmlns:xlink='http://www.w3.org/1999/xlink'>&score;</opus>`, validation.SourceOriginEntity, false},
		{"unavailable-parameter-entity", `<!DOCTYPE opus [<!ENTITY % p "<!ATTLIST opus version CDATA '4.0'>">%p;]><opus/>`, validation.SourceOriginDefault, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			baselineAccount := adapterAccount(t, validation.Budgets{})
			baselineConfig := adapterSourceConfig(t, baselineAccount, "source", "", ProfileSourceInternalDTD, sourceBytesPrimary)
			want, err := adapterInspect(t, strings.NewReader(tc.source), baselineConfig)
			if err != nil {
				t.Fatal(err)
			}
			adapterClean(t, baselineAccount)
			a := adapterAccount(t, validation.Budgets{})
			config := adapterSourceConfig(t, a, "source", "", ProfileSourceInternalDTD, sourceBytesPrimary)
			run, err := newSourceAssessmentRunWithConfig(ctx, strings.NewReader(tc.source), DecodeOptions{}, &config, false)
			if err != nil {
				t.Fatal(err)
			}
			cause := run.drain()
			completion, err := run.finishInput(cause)
			if cause != nil || err != nil {
				t.Fatalf("input: %v %v", cause, err)
			}
			report, snapshot, err := run.session.FinishWithResourceSnapshot(ctx, completion)
			if err != nil || snapshot == nil || !reflect.DeepEqual(report, want) {
				t.Fatalf("export changed original intrinsic report: snapshot=%v err=%v", snapshot, err)
			}
			found := false
			for i := 0; i < snapshot.ContextCount(); i++ {
				row, ok, err := snapshot.ContextAt(ctx, i)
				if err != nil || !ok || row.SourceOrigin == nil {
					t.Fatalf("origin %d: %v", i, err)
				}
				origin := row.SourceOrigin
				if origin.Kind == tc.kind && (origin.Attribute != nil) == tc.attribute {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing real %s origin", tc.name)
			}
			if used := adapterUsage(t, a, validation.BudgetInputBytes); used != int64(len(tc.source)) {
				t.Fatalf("snapshot reread original source: got %d want %d", used, len(tc.source))
			}
			if err = snapshot.Close(); err != nil {
				t.Fatal(err)
			}
			adapterClean(t, a)
		})
	}
}
