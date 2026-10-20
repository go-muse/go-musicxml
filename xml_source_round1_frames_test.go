package musicxml

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestSourceRound1LegacyEOFDiagnostics(t *testing.T) {
	for _, input := range []string{`<r>&`, `<!-- x`, `<![CDATA[x`, `<`, `<?pi x`, `<r a="&`, `<r a="x`} {
		t.Run(input, func(t *testing.T) {
			r, err := newSourceXMLTokenReader(strings.NewReader(input), sourceXMLConfig{Context: context.Background()})
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			for err == nil {
				_, err = r.Token()
			}
			var syntax *xml.SyntaxError
			if !errors.As(err, &syntax) || syntax.Msg != "unexpected EOF" {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestSourceRound1NoExpansionEmptyText(t *testing.T) {
	for _, replacement := range []string{"", "<x/>", "&e;"} {
		input := `<!DOCTYPE r [<!ENTITY e ""><!ENTITY x "` + replacement + `">]><r>&x;&x;</r>`
		tokens, _, _, err := sourceEntityScan(t, input)
		if err != nil {
			t.Fatal(err)
		}
		for _, tok := range tokens {
			if text, ok := tok.(xml.CharData); ok && len(text) == 0 {
				t.Errorf("replacement %q emitted empty text", replacement)
			}
		}
	}
	// Empty CDATA is a physical token, not an expansion artifact.
	tokens, _, _, err := sourceEntityScan(t, `<r><![CDATA[]]></r>`)
	if err != nil || len(tokens) != 3 {
		t.Fatalf("CDATA tokens=%v err=%v", tokens, err)
	}
}

func TestSourceRound1UnresolvedAttributeDeclaration(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		want        validation.EvidenceState
	}{
		{"external", `<!DOCTYPE opus SYSTEM "opus.dtd"><opus version="4.0"/>`, validation.EvidenceUnknown},
		{"cutoff", `<!DOCTYPE opus [<!ENTITY % e SYSTEM "e.ent">%e;<!ATTLIST opus version CDATA #IMPLIED>]><opus version="4.0"/>`, validation.EvidenceUnknown},
		{"standalone-direct", `<?xml version="1.0" standalone="yes"?><!DOCTYPE opus [<!ENTITY % e SYSTEM "e.ent">%e;<!ATTLIST opus version CDATA #IMPLIED>]><opus version="4.0"/>`, validation.EvidenceKnown},
		{"closed-absence", `<opus version="4.0"/>`, validation.EvidenceAbsent},
		{"known-despite-external", `<!DOCTYPE opus SYSTEM "opus.dtd" [<!ATTLIST opus version CDATA #IMPLIED>]><opus version="4.0"/>`, validation.EvidenceKnown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report, err := InspectXML(context.Background(), strings.NewReader(tc.input), SourceOptions{Context: validation.ContextFamilyAll, Profile: ProfileSourceInternalDTD})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, row := range report.Context {
				o := row.SourceOrigin
				if o == nil || o.Attribute == nil || o.Attribute.Local != "version" {
					continue
				}
				found = true
				if o.Declaration.State != tc.want {
					t.Errorf("declaration=%+v want=%v", o.Declaration, tc.want)
				}
				if tc.want == validation.EvidenceUnknown && o.Declaration.Reason != "dtd-infoset-unavailable" {
					t.Errorf("reason=%+v", o.Declaration)
				}
				if o.Kind != validation.SourceOriginPhysical || !o.Specified.Value || o.Physical.State != validation.EvidenceKnown || o.Use.State != validation.EvidenceAbsent {
					t.Errorf("independent physical provenance changed: %+v", o)
				}
			}
			if !found {
				t.Fatal("missing version origin")
			}
		})
	}
}

func TestSourceRound1UnresolvedOriginEnumeration(t *testing.T) {
	report, err := InspectXML(context.Background(), strings.NewReader(`<!DOCTYPE opus SYSTEM "opus.dtd"><opus version="4.0"/>`), SourceOptions{Context: validation.ContextFamilyAll, Profile: ProfileSourceInternalDTD})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range report.Findings {
		if strings.HasPrefix(string(f.Rule.ID), "MX40-SOURCE-ORIGIN-") && f.Outcome == validation.OutcomeUnsupported && f.Reason == "dtd-infoset-unavailable" {
			return
		}
	}
	t.Fatalf("unavailable origin enumeration not reported: %+v", report.Findings)
}

func TestSourceRound1DefaultDepthSymmetry(t *testing.T) {
	for _, depth := range []int{64, 65} {
		var dtd strings.Builder
		for i := 0; i < depth; i++ {
			value := "v"
			if i+1 < depth {
				value = fmt.Sprintf("&e%d;", i+1)
			}
			fmt.Fprintf(&dtd, `<!ENTITY e%d "%s">`, i, value)
		}
		for _, form := range []string{"content", "physical", "default"} {
			t.Run(fmt.Sprintf("%s-%d", form, depth), func(t *testing.T) {
				body := `<r>&e0;</r>`
				decl := dtd.String()
				if form == "physical" {
					body = `<r a="&e0;"/>`
				}
				if form == "default" {
					decl += `<!ATTLIST r a CDATA "&e0;">`
					body = `<r/>`
				}
				_, _, _, err := sourceEntityScan(t, `<!DOCTYPE r [`+decl+`]>`+body)
				if depth == 64 && err != nil {
					t.Fatalf("64 named entities: %v", err)
				}
				if depth == 65 {
					var resource *validation.ResourceError
					if !errors.Is(err, ErrEntityNestingLimit) || !errors.As(err, &resource) || resource.Kind != validation.BudgetDepth || resource.Limit != 64 {
						t.Fatalf("65 named entities: %v", err)
					}
				}
			})
		}
	}
}

func TestSourceRound1ParameterPhysicalBytes(t *testing.T) {
	doctype := `<!DOCTYPE r [<!ENTITY % p "` + strings.Repeat(`<!---->`, 10) + `">` + strings.Repeat(`%p;`, 30) + `]>`
	a, _ := validation.NewBudgetAccount(validation.Budgets{MaxScalarBytes: 512})
	r, err := newSourceXMLTokenReader(strings.NewReader(doctype+`<r/>`), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, err = r.Token(); err != nil {
		t.Fatal(err)
	}
	if r.dtdBytes != int64(len(doctype)-len(`<!DOCTYPE`)) {
		t.Fatalf("physical DTD bytes=%d want=%d", r.dtdBytes, len(doctype)-len(`<!DOCTYPE`))
	}
}

func TestSourceRound1ParameterScratchLifetime(t *testing.T) {
	doctype := `<!DOCTYPE r [<!ENTITY % p "` + strings.Repeat(`<!---->`, 100) + `">` + strings.Repeat(`%p;`, 40) + `]>`
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	r, err := newSourceXMLTokenReader(strings.NewReader(doctype+`<r/>`), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Token(); err != nil {
		t.Fatal(err)
	}
	if r.tokenOwned > 1024 {
		t.Errorf("temporary PE origin accumulated: tokenOwned=%d", r.tokenOwned)
	}
	r.Close()
	used, _ := a.Usage(validation.BudgetBufferedBytes)
	if used != 0 {
		t.Fatalf("retained=%d", used)
	}
}

func TestSourceRound1PaddedReplacementOffset(t *testing.T) {
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	r, err := newSourceXMLTokenReader(strings.NewReader(`<!DOCTYPE r [<!ENTITY % p "<!ENTITY e 'v'>">%p;]><r/>`), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, err = r.Token(); err != nil {
		t.Fatal(err)
	}
	d, ok := r.dtdStore.Entity("e", false)
	if !ok {
		t.Fatal("missing e")
	}
	if len(d.Origin.Ancestry) != 1 || d.Origin.Ancestry[0].ReplacementOffset != 0 {
		t.Fatalf("origin=%+v", d.Origin)
	}
}

func TestSourceRound1TextOriginSingleCapture(t *testing.T) {
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	r, err := newSourceXMLTokenReader(strings.NewReader(`<!DOCTYPE r [<!ENTITY a "&b;"><!ENTITY b "v">]><r>&a;</r>`), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for {
		token, err := r.Token()
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := token.(xml.CharData); !ok {
			continue
		}
		// One origin and its two-step ancestry, plus short reference-name strings.
		if r.tokenOwned > 700 {
			t.Fatalf("duplicate origin reservations=%d", r.tokenOwned)
		}
		if o := r.Metadata().Origin; o == nil || len(o.Ancestry) != 2 || o.Ancestry[0].Entity != "a" || o.Ancestry[1].Entity != "b" {
			t.Fatalf("origin=%+v", o)
		}
		break
	}
}

// This evidence benchmark measures allocations independently of budget gauges.
func BenchmarkSourceRound1ExpansionChurn(b *testing.B) {
	input := `<!DOCTYPE r [<!ENTITY a "&b;&b;&b;&b;&b;&b;&b;&b;&b;&b;"><!ENTITY b "lol">]><r>` + strings.Repeat(`&a;`, 1000) + `</r>`
	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	for b.Loop() {
		a, _ := validation.NewBudgetAccount(validation.Budgets{})
		r, err := newSourceXMLTokenReader(strings.NewReader(input), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true})
		if err != nil {
			b.Fatal(err)
		}
		for err == nil {
			_, err = r.Token()
		}
		if err != io.EOF {
			b.Fatal(err)
		}
		r.Close()
	}
	runtime.KeepAlive(input)
}

func TestSourceRound1OriginOccurrenceBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		kind        validation.SourceOriginKind
		specified   bool
		ancestry    int
	}{
		{"default-value-entity", `<!DOCTYPE opus [<!ENTITY version "4.0"><!ATTLIST opus version CDATA "&version;">]><opus/>`, validation.SourceOriginDefault, false, 0},
		{"entity-generated-attribute", `<!DOCTYPE opus [<!ENTITY score "<score xmlns:xlink='http://www.w3.org/1999/xlink' xlink:href='a.xml'/>">]><opus version="4.0">&score;</opus>`, validation.SourceOriginEntity, false, 1},
		{"physical-value-entity", `<!DOCTYPE opus [<!ENTITY version "4.0">]><opus version="&version;"/>`, validation.SourceOriginPhysical, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report, err := InspectXML(context.Background(), strings.NewReader(tc.input), SourceOptions{Context: validation.ContextFamilyAll, Profile: ProfileSourceInternalDTD})
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, row := range report.Context {
				o := row.SourceOrigin
				if o == nil || o.Attribute == nil {
					continue
				}
				name := "version"
				if tc.ancestry != 0 {
					name = "href"
				}
				if o.Attribute.Local != name {
					continue
				}
				found = true
				if o.Kind != tc.kind || o.Specified.State != validation.EvidenceKnown || o.Specified.Value != tc.specified || len(o.Ancestry) != tc.ancestry {
					t.Errorf("occurrence=%+v", o)
				}
			}
			if !found {
				t.Fatal("missing origin")
			}
		})
	}
}

func TestSourceRound1ParameterScratchOwnsLiveCapacity(t *testing.T) {
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	r, err := newSourceXMLTokenReader(strings.NewReader(""), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true})
	if err != nil {
		t.Fatal(err)
	}
	base, _ := a.Usage(validation.BudgetBufferedBytes)
	for depth := 1; depth <= 6; depth++ {
		d := &sourceDTDDeclaration{Name: fmt.Sprintf("p%d", depth), Parameter: true, Replacement: "x"}
		if err = r.PushDTDParameter(d, sourceDTDOrigin{}); err != nil {
			t.Fatal(err)
		}
		origin, err := r.DTDParameterOrigin()
		if err != nil {
			t.Fatal(err)
		}
		if len(origin.Ancestry) != depth {
			t.Fatal("ancestry depth")
		}
		want := base + int64(depth)*sourceXMLEntityFrameBytes + int64(cap(r.dtdOriginScratch))*160
		got, _ := a.Usage(validation.BudgetBufferedBytes)
		if got != want {
			t.Fatalf("depth=%d live=%d want=%d", depth, got, want)
		}
		if r.tokenOwned != 0 {
			t.Fatalf("scratch charged as token retention: %d", r.tokenOwned)
		}
		for range 100 {
			if _, err = r.DTDParameterOrigin(); err != nil {
				t.Fatal(err)
			}
		}
		got, _ = a.Usage(validation.BudgetBufferedBytes)
		if got != want {
			t.Fatalf("snapshot copies accumulated=%d want=%d", got, want)
		}
	}
	r.Close()
	got, _ := a.Usage(validation.BudgetBufferedBytes)
	if got != 0 {
		t.Fatalf("retained=%d", got)
	}
}

func TestSourceRound1ParameterScratchAdmissionAndCancellation(t *testing.T) {
	// Derive the initial reader allowance using an independent account, avoiding
	// a duplicate assumed cursor allowance in the actual exact-boundary checks.
	probe, _ := validation.NewBudgetAccount(validation.Budgets{})
	reader, err := newSourceXMLTokenReader(strings.NewReader(""), sourceXMLConfig{Context: context.Background(), Account: probe, InternalDTD: true})
	if err != nil {
		t.Fatal(err)
	}
	initial, _ := probe.Usage(validation.BudgetBufferedBytes)
	reader.Close()
	for _, short := range []int64{0, 1} {
		// At depth two both the old 160-byte slice and the new 320-byte slice live.
		limit := initial + 2*sourceXMLEntityFrameBytes + 160 + 320 - short
		a, _ := validation.NewBudgetAccount(validation.Budgets{MaxBufferedBytes: limit})
		ctx, cancel := context.WithCancel(context.Background())
		r, err := newSourceXMLTokenReader(strings.NewReader(""), sourceXMLConfig{Context: ctx, Account: a, InternalDTD: true})
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			if err = r.PushDTDParameter(&sourceDTDDeclaration{Name: fmt.Sprintf("p%d", i), Replacement: "x"}, sourceDTDOrigin{}); err != nil {
				t.Fatal(err)
			}
			_, err = r.DTDParameterOrigin()
			if i == 0 && err != nil {
				t.Fatal(err)
			}
		}
		if short == 0 && err != nil || short == 1 && !errors.Is(err, validation.ErrResourceLimit) {
			t.Errorf("short=%d error=%v", short, err)
		}
		if short == 1 && cap(r.dtdOriginScratch) != 1 {
			t.Fatal("failed growth changed scratch")
		}
		cancel()
		if _, err = r.DTDParameterOrigin(); !errors.Is(err, context.Canceled) {
			t.Errorf("canceled snapshot=%v", err)
		}
		r.Close()
		used, _ := a.Usage(validation.BudgetBufferedBytes)
		if used != 0 {
			t.Errorf("retained=%d", used)
		}
	}
}

func BenchmarkSourceRound1DefaultBudgetBomb(b *testing.B) {
	var dtd strings.Builder
	dtd.WriteString(`<!DOCTYPE score-partwise [<!ENTITY e0 "lol">`)
	for i := 1; i < 10; i++ {
		fmt.Fprintf(&dtd, `<!ENTITY e%d "%s">`, i, strings.Repeat(fmt.Sprintf("&e%d;", i-1), 10))
	}
	dtd.WriteString(`]>`)
	input := dtd.String() + strings.Replace(assessedScore("1", ""), "<part-list>", "<movement-title>&e9;</movement-title><part-list>", 1)
	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	for b.Loop() {
		report, err := InspectXML(context.Background(), strings.NewReader(input), SourceOptions{Context: validation.ContextFamilyAll, Profile: ProfileSourceInternalDTD})
		var resource *validation.ResourceError
		if !errors.As(err, &resource) || resource.Kind != validation.BudgetScalarBytes || report.InputComplete {
			b.Fatalf("refusal=%v complete=%t", err, report.InputComplete)
		}
	}
}

func TestSourceRound1ParameterScratchDoesNotAliasDeclarations(t *testing.T) {
	input := `<!DOCTYPE r [<!ENTITY % p "<!ENTITY a 'a'>"><!ENTITY % q "<!ENTITY b 'b'>">%p;%q;]><r/>`
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	r, err := newSourceXMLTokenReader(strings.NewReader(input), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, err = r.Token(); err != nil {
		t.Fatal(err)
	}
	if r.dtdOriginScratch != nil {
		t.Fatal("DOCTYPE retained temporary ancestry")
	}
	for name, want := range map[string]string{"a": "p", "b": "q"} {
		d, ok := r.dtdStore.Entity(name, false)
		if !ok || len(d.Origin.Ancestry) != 1 || d.Origin.Ancestry[0].Entity != want {
			t.Fatalf("declaration %s=%+v", name, d)
		}
	}
	if _, err = r.Token(); err != nil {
		t.Fatal(err)
	}
	d, _ := r.dtdStore.Entity("a", false)
	if d.Origin.Ancestry[0].Entity != "p" {
		t.Fatal("token advance changed stored ancestry")
	}
}

func TestSourceRound1ContentVerificationFrameBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		complete    bool
	}{
		{"content-read", `<!DOCTYPE r [<!ENTITY e "<x/>">]><r>&e;</r>`, true},
		{"attribute-read-only", `<!DOCTYPE r [<!ENTITY e "]]>">]><r a="&e;"/>`, false},
		{"unused-independent", `<!DOCTYPE r [<!ENTITY e "<x/>"><!ENTITY unused "&unused;">]><r>&e;</r>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, summary, err := sourceEntityScan(t, tc.input)
			if err != nil || summary.SyntaxComplete != tc.complete {
				t.Fatalf("syntaxComplete=%t error=%v", summary.SyntaxComplete, err)
			}
		})
	}
}

func TestSourceRound1ContentVerificationFailedAdmissionClosesOwnership(t *testing.T) {
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	r, err := newSourceXMLTokenReader(strings.NewReader(`<!DOCTYPE r [<!ENTITY e "<x/>">]><r>&e;</r>`), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true})
	if err != nil {
		t.Fatal(err)
	}
	for {
		tok, e := r.Token()
		if e != nil {
			t.Fatal(e)
		}
		if end, ok := tok.(xml.EndElement); ok && end.Name.Local == "x" {
			break
		}
	}
	r.releaseToken()
	used, _ := a.Usage(validation.BudgetBufferedBytes)
	held := a.Limits().MaxBufferedBytes - used
	if err = a.Charge(validation.BudgetBufferedBytes, held); err != nil {
		t.Fatal(err)
	}
	_, err = r.Token()
	if !errors.Is(err, validation.ErrResourceLimit) {
		t.Fatalf("admission=%v", err)
	}
	if r.entity == nil || !r.dtdStore.State().EntityWellFormednessIncomplete {
		t.Fatal("failed verification committed frame/proof")
	}
	r.Close()
	used, _ = a.Usage(validation.BudgetBufferedBytes)
	if used != held {
		t.Fatalf("foreign ownership: got=%d want=%d", used, held)
	}
	if err = a.Release(validation.BudgetBufferedBytes, held); err != nil {
		t.Fatal(err)
	}
}
