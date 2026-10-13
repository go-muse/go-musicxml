package musicxml

import (
	"encoding/xml"
	"errors"
	"strings"
	"testing"
)

func TestSourceDTDReviewUndeclaredParameterDomain(t *testing.T) {
	for _, source := range []string{
		`<?xml version="1.0" standalone="yes"?><!DOCTYPE r [%p;]><r/>`,
		`<?xml version="1.0" standalone="yes"?><!DOCTYPE r [%p;<!ENTITY e "x">]><r>&e;</r>`,
		`<?xml version="1.0" standalone="yes"?><!DOCTYPE r SYSTEM "unread" [%p;]><r/>`,
		`<?xml version="1.0" standalone="yes"?><!DOCTYPE r [<!ENTITY % maker "<!ENTITY &#37; p ''>">%maker;%p;]><r/>`,
	} {
		_, _, _, err := sourceEntityScan(t, source)
		var syntax *xml.SyntaxError
		if !errors.As(err, &syntax) {
			t.Errorf("%s: want Entity Declared WFC, got %v", source, err)
		}
	}
	// The PE reference itself selects §4.1's Entity Declared validity domain.
	for _, source := range []string{`<!DOCTYPE r [%p;]><r/>`, `<!DOCTYPE r [%p;<!ENTITY % p "">]><r/>`} {
		_, _, summary, err := sourceEntityScan(t, source)
		if err != nil || summary.SyntaxComplete {
			t.Errorf("validity-only absence: err=%v summary=%+v", err, summary)
		}
	}
}

func TestSourceDTDReviewParameterDeclarationCapability(t *testing.T) {
	for _, source := range []string{
		`<!DOCTYPE r [<!ENTITY % pe "x"><!ENTITY % outer "<!ENTITY f &#39;&#37;pe;&#39;>">%outer;]><r>&f;</r>`,
		`<!DOCTYPE r [<!ENTITY % t "CDATA"><!ENTITY % outer "<!ATTLIST r a &#37;t; 'd'>">%outer;]><r/>`,
		`<!DOCTYPE r [<!ENTITY % outer "<![INCLUDE[<!ELEMENT r EMPTY>]]>">%outer;]><r/>`,
		`<!DOCTYPE r [<!ENTITY % outer "<![IGNORE[abc]]>">%outer;]><r/>`,
		`<!DOCTYPE r [<!ENTITY % outer "<!ELEMENT r (a&#37;t;)> ">%outer;]><r/>`,
	} {
		_, _, _, err := sourceEntityScan(t, source)
		var unsupported *sourceXMLUnsupportedError
		var syntax *xml.SyntaxError
		if !errors.As(err, &unsupported) || errors.As(err, &syntax) || unsupported.Reason != "dtd-parameter-reference-in-declaration-unread" {
			t.Errorf("%s: got %T %v", source, err, err)
		}
	}
	_, _, _, err := sourceEntityScan(t, `<!DOCTYPE r [<!ENTITY % nm "name"><!ENTITY %nm; "v">]><r/>`)
	var syntax *xml.SyntaxError
	if !errors.As(err, &syntax) || !strings.Contains(err.Error(), "PEs in Internal Subset") {
		t.Errorf("physical declaration WFC diagnostic: %v", err)
	}
}

func TestSourceDTDReviewDefaultDiagnosticOrigin(t *testing.T) {
	for _, tc := range []struct {
		source  string
		line    int
		message string
	}{
		{"<!DOCTYPE r [\n<!ATTLIST r a CDATA '&e;'>\n<!ENTITY e 'x'>\n]>\n<r/>", 2, "must precede"},
		{"<!DOCTYPE r [<!ENTITY e '&#60;'>\n<!ATTLIST r a CDATA '&e;'>\n]>\n<r/>", 2, "unescaped <"},
		{"<!DOCTYPE r [<!NOTATION n SYSTEM 'n'><!ENTITY e SYSTEM 'x' NDATA n>\n<!ATTLIST r a CDATA '&e;'>\n]>\n<r/>", 2, "unparsed XML entity"},
		{"<!DOCTYPE r [<!ENTITY % p \"<!ENTITY e '&#60;'><!ATTLIST r a CDATA '&e;'>\">\n%p;\n]>\n<r/>", 2, "unescaped <"},
	} {
		_, _, _, err := sourceEntityScan(t, tc.source)
		var syntax *xml.SyntaxError
		if !errors.As(err, &syntax) || syntax.Line != tc.line || !strings.Contains(syntax.Msg, tc.message) {
			t.Errorf("want line %d %q, got %v", tc.line, tc.message, err)
		}
	}
}

func TestSourceDTDReviewClosure(t *testing.T) {
	for _, source := range []string{
		`<!DOCTYPE r [<!ENTITY % p "x">]><r/>`,
		`<!DOCTYPE r [<!ENTITY % p "<!ELEMENT x EMPTY>"><!ENTITY % p "&#37;p;">%p;]><r/>`,
		`<!DOCTYPE r [<!ENTITY co "A &amp; B">]><r>&co;</r>`,
		`<!DOCTYPE r [<!ENTITY e "&amp;&lt;&gt;&apos;&quot;">]><r/>`,
	} {
		_, _, summary, err := sourceEntityScan(t, source)
		if err != nil || !summary.SyntaxComplete {
			t.Errorf("want syntax closure: %s: err=%v summary=%+v", source, err, summary)
		}
	}
	for _, source := range []string{
		`<!DOCTYPE r [<!ENTITY % p "&#37;p;">]><r/>`,
		`<!DOCTYPE r [<!ENTITY e "&e;">]><r/>`,
		`<!DOCTYPE r [<!ENTITY e "]]>">]><r a="&e;"/>`,
	} {
		_, _, summary, err := sourceEntityScan(t, source)
		if err != nil || summary.SyntaxComplete {
			t.Errorf("unproved replacement: %s: err=%v summary=%+v", source, err, summary)
		}
	}
	for _, source := range []string{
		`<!DOCTYPE r [<!ENTITY e "<x/>">]><r>&e;<x a="&e;"/></r>`,
		`<!DOCTYPE r [<!ENTITY e "]]>">]><r a="&e;">&e;</r>`,
	} {
		_, _, _, err := sourceEntityScan(t, source)
		var syntax *xml.SyntaxError
		if !errors.As(err, &syntax) {
			t.Errorf("successful first context suppressed later failure: %v", err)
		}
	}
}

func TestSourceDTDReviewPredefinedReplacementClosure(t *testing.T) {
	for _, declaration := range []string{
		`<!ENTITY lt "&#38;#60;">`, `<!ENTITY lt "&#38;#x03c;">`,
		`<!ENTITY amp "&#38;#38;">`, `<!ENTITY amp "&#38;#x026;">`,
		`<!ENTITY gt ">">`, `<!ENTITY gt "&#62;">`, `<!ENTITY gt "&#38;#x3e;">`,
		`<!ENTITY apos "'">`, `<!ENTITY apos "&#39;">`, `<!ENTITY apos "&#38;#x27;">`,
		`<!ENTITY quot '"'>`, `<!ENTITY quot "&#34;">`, `<!ENTITY quot "&#38;#x22;">`,
	} {
		_, _, summary, err := sourceEntityScan(t, `<!DOCTYPE r [`+declaration+`]><r/>`)
		if err != nil || !summary.SyntaxComplete {
			t.Errorf("%s: err=%v summary=%+v", declaration, err, summary)
		}
	}
	for _, declaration := range []string{
		`<!ENTITY lt "&#60;">`, `<!ENTITY amp "&#38;">`, `<!ENTITY gt "&gt;">`, `<!ENTITY apos "wrong">`, `<!ENTITY quot "">`,
	} {
		_, _, summary, err := sourceEntityScan(t, `<!DOCTYPE r [`+declaration+`]><r/>`)
		if err != nil || summary.SyntaxComplete {
			t.Errorf("nonconforming declaration claimed closed: %s: err=%v summary=%+v", declaration, err, summary)
		}
	}
}

type sourceDTDReviewOriginCounter struct {
	*sourceXMLTokenReader
	snapshots int
}

func (in *sourceDTDReviewOriginCounter) DTDParameterOrigin() (sourceDTDOrigin, error) {
	in.snapshots++
	return in.sourceXMLTokenReader.DTDParameterOrigin()
}
func TestSourceDTDReviewOriginsAreDeclarationOnly(t *testing.T) {
	source := ` r [<!ENTITY % leaf "<!----><?pi x?>"><!ENTITY % outer "` + strings.Repeat(`&#37;leaf;`, 500) + `">%outer;]>`
	r := dtdStoreParameterReader(t, source)
	store, err := newSourceDTDStore(r.cursor.ctx, r.account, sourceDTDStandaloneAbsent)
	if err != nil {
		t.Fatal(err)
	}
	r.dtdStore = store
	in := &sourceDTDReviewOriginCounter{sourceXMLTokenReader: r}
	if _, err = parseSourceDTDWithStore(in, store); err != nil {
		t.Fatal(err)
	}
	if in.snapshots != 2 {
		t.Errorf("comments, PIs or PE uses allocated ancestry: got %d snapshots, want 2 declarations", in.snapshots)
	}
	if store.State().WellFormednessIncomplete {
		t.Errorf("processed PE comments remained incomplete: %+v", store.State())
	}
}
