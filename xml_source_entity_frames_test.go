package musicxml

import (
	"context"
	"crypto/sha256"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"unsafe"

	"github.com/go-muse/go-musicxml/validation"
)

func TestSourceEntityFrameAdmissionAndWork(t *testing.T) {
	for _, test := range []struct {
		name  string
		limit int64
		ok    bool
	}{{"exact", 4, true}, {"one-short", 3, false}} {
		t.Run(test.name, func(t *testing.T) {
			account, _ := validation.NewBudgetAccount(validation.Budgets{MaxEvents: test.limit})
			r, err := newSourceXMLTokenReader(strings.NewReader("physical"), sourceXMLConfig{Context: context.Background(), Account: account})
			if err != nil {
				t.Fatal(err)
			}
			err = r.pushEntity(sourceXMLEntityInput{Name: "e", Replacement: "éx"}, sourceEntityContent, sourceXMLSpan{})
			if test.ok && err != nil || !test.ok && err == nil {
				t.Fatalf("push: %v", err)
			}
			if !test.ok && r.entity != nil {
				t.Fatal("failed admission mutated frame stack")
			}
			if test.ok {
				for _, want := range "éx" {
					u, e := r.take()
					if e != nil || u.Rune != want {
						t.Fatalf("take=%q,%v", u.Rune, e)
					}
				}
				if _, err = r.peek(); !errors.Is(err, errSourceEntityBoundary) {
					t.Fatalf("boundary: %v", err)
				}
				if r.Summary().BytesRead != 0 || r.Summary().InputComplete {
					t.Fatal("replacement changed physical summary")
				}
				if err = r.finishEntity(sourceEntityContent); err != nil {
					t.Fatal(err)
				}
				u, e := r.take()
				if e != nil || u.Rune != 'p' {
					t.Fatalf("caller: %q,%v", u.Rune, e)
				}
			}
			r.Close()
			used, _ := account.Usage(validation.BudgetBufferedBytes)
			if used != 0 {
				t.Fatalf("retained %d", used)
			}
		})
	}
}

func TestSourceEntityFrameRecursionAndDepth(t *testing.T) {
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	r, err := newSourceXMLTokenReader(strings.NewReader(""), sourceXMLConfig{Context: context.Background(), Account: a})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err = r.pushEntity(sourceXMLEntityInput{Name: "e"}, sourceEntityContent, sourceXMLSpan{}); err != nil {
		t.Fatal(err)
	}
	if err = r.pushEntity(sourceXMLEntityInput{Name: "e"}, sourceEntityContent, sourceXMLSpan{}); err == nil {
		t.Fatal("accepted active cycle")
	}
	for i := 1; i < sourceXMLEntityMaxDepth; i++ {
		if err = r.pushEntity(sourceXMLEntityInput{Name: string(rune(0x100 + i))}, sourceEntityContent, sourceXMLSpan{}); err != nil {
			t.Fatal(err)
		}
	}
	err = r.pushEntity(sourceXMLEntityInput{Name: "over"}, sourceEntityContent, sourceXMLSpan{})
	var resource *validation.ResourceError
	if !errors.As(err, &resource) {
		t.Fatalf("depth: %v", err)
	}
}

func TestSourceEntityFrameCancellationAndNormalization(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	r, err := newSourceXMLTokenReader(strings.NewReader("\r\n"), sourceXMLConfig{Context: ctx, Account: a})
	if err != nil {
		t.Fatal(err)
	}
	if err = r.pushEntity(sourceXMLEntityInput{Name: "e", Replacement: "\r\n"}, sourceEntityAttribute, sourceXMLSpan{}); err != nil {
		t.Fatal(err)
	}
	u, err := r.take()
	if err != nil || u.Rune != '\r' {
		t.Fatalf("replacement CR normalized: %q,%v", u.Rune, err)
	}
	u, err = r.take()
	if err != nil || u.Rune != '\n' {
		t.Fatalf("replacement LF: %q,%v", u.Rune, err)
	}
	if err = r.finishEntity(sourceEntityAttribute); err != nil {
		t.Fatal(err)
	}
	u, err = r.take()
	if err != nil || u.Rune != '\n' {
		t.Fatalf("physical EOL: %q,%v", u.Rune, err)
	}
	if _, err = r.take(); err != io.EOF {
		t.Fatalf("physical EOF: %v", err)
	}
	if err = r.pushEntity(sourceXMLEntityInput{Name: "e", Replacement: "x"}, sourceEntityContent, sourceXMLSpan{}); err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, err = r.peek(); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	r.Close()
	used, _ := a.Usage(validation.BudgetBufferedBytes)
	if used != 0 {
		t.Fatalf("retained %d", used)
	}
}

func sourceEntityScan(t *testing.T, source string) ([]xml.Token, []sourceXMLTokenMeta, sourceXMLSummary, error) {
	t.Helper()
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	r, err := newSourceXMLTokenReader(strings.NewReader(source), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true})
	if err != nil {
		return nil, nil, sourceXMLSummary{}, err
	}
	var tokens []xml.Token
	var metas []sourceXMLTokenMeta
	for {
		token, e := r.Token()
		if e != nil {
			err = e
			break
		}
		tokens = append(tokens, xml.CopyToken(token))
		metas = append(metas, r.Metadata())
	}
	summary := r.Summary()
	r.Close()
	used, _ := a.Usage(validation.BudgetBufferedBytes)
	if used != 0 {
		t.Fatalf("retained %d", used)
	}
	if err == io.EOF {
		err = nil
	}
	return tokens, metas, summary, err
}
func TestSourceInternalEntityContexts(t *testing.T) {
	for _, test := range []struct {
		name, source, text, attribute string
		elements                      int
	}{
		{"text-chain", `<!DOCTYPE r [<!ENTITY a "a&b;c"><!ENTITY b "é">]><r>&a;&a;</r>`, "aécaéc", "", 1},
		{"siblings", `<!DOCTYPE r [<!ENTITY a "<x/><y>z</y>">]><r>&a;</r>`, "z", "", 3},
		{"quotes", `<!DOCTYPE r [<!ENTITY a '"&#x9;&#xD;&quot;'><!ENTITY b "&a;">]><r a="x&b;y&#x9;"/>`, "", "x\"  \"y\t", 1},
		{"numeric-markup", `<!DOCTYPE r [<!ENTITY a "&#60;x/>">]><r>&a;&#60;y/&gt;</r>`, "<y/>", "", 2},
		{"protected-less-than", `<!DOCTYPE r [<!ENTITY a "&lt;x/>">]><r>&a;</r>`, "<x/>", "", 1},
		{"comment-reference", `<!DOCTYPE r [<!ENTITY a "<!-- &a; -->ok">]><r>&a;</r>`, "ok", "", 1},
		{"cdata-reference", `<!DOCTYPE r [<!ENTITY a "<![CDATA[&a;]]>">]><r>&a;</r>`, "&a;", "", 1},
		{"empty-repeat", `<!DOCTYPE r [<!ENTITY a "">]><r>a&a;&a;b</r>`, "ab", "", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			tokens, _, summary, err := sourceEntityScan(t, test.source)
			if err != nil {
				t.Fatal(err)
			}
			var text strings.Builder
			elements := 0
			attribute := ""
			for _, token := range tokens {
				switch token := token.(type) {
				case xml.CharData:
					text.Write(token)
				case xml.StartElement:
					elements++
					for _, a := range token.Attr {
						if a.Name.Local == "a" {
							attribute = a.Value
						}
					}
				}
			}
			if text.String() != test.text || attribute != test.attribute || elements != test.elements {
				t.Fatalf("text=%q attribute=%q elements=%d", text.String(), attribute, elements)
			}
			if !summary.InputComplete || summary.BytesConsumed != int64(len(test.source)) || summary.ReadPrefixSHA256 != sha256.Sum256([]byte(test.source)) {
				t.Fatalf("physical summary=%+v", summary)
			}
		})
	}
}
func TestSourceInternalEntityBoundaries(t *testing.T) {
	for _, replacement := range []string{"<x", "<!-- x", "<![CDATA[x", "<?p x", "&amp", "<x>", "</r><r>", "&#38;"} {
		source := `<!DOCTYPE r [<!ENTITY e "` + replacement + `">]><r>&e;</r>`
		_, _, _, err := sourceEntityScan(t, source)
		var syntax *xml.SyntaxError
		if !errors.As(err, &syntax) {
			t.Errorf("replacement %q: want syntax, got %v", replacement, err)
		}
	}
	for _, source := range []string{
		`<!DOCTYPE r [<!ENTITY a "&a;">]><r>&a;</r>`,
		`<!DOCTYPE r [<!ENTITY a "&b;"><!ENTITY b "&a;">]><r a="&a;"/>`,
		`<!DOCTYPE r [<!ENTITY a SYSTEM "never-read">]><r a="&a;"/>`,
		`<!DOCTYPE r [<!ENTITY a SYSTEM "never-read" NDATA n>]><r>&a;</r>`,
	} {
		_, _, _, err := sourceEntityScan(t, source)
		var syntax *xml.SyntaxError
		if !errors.As(err, &syntax) {
			t.Errorf("%s: %v", source, err)
		}
	}
}

func TestSourceInternalDTDRequiresAccountBeforeInput(t *testing.T) {
	calls := 0
	_, err := newSourceXMLTokenReader(sourceEntityReadFunc(func([]byte) (int, error) { calls++; return 0, io.EOF }), sourceXMLConfig{Context: context.Background(), InternalDTD: true})
	var configuration *validation.ConfigurationError
	if !errors.As(err, &configuration) || calls != 0 {
		t.Fatalf("err=%v reads=%d", err, calls)
	}
}

type sourceEntityReadFunc func([]byte) (int, error)

func (f sourceEntityReadFunc) Read(p []byte) (int, error) { return f(p) }

func TestSourceEntityFrameRetentionBeforeAllocation(t *testing.T) {
	for _, extra := range []int64{sourceXMLEntityFrameBytes, sourceXMLEntityFrameBytes - 1} {
		a, _ := validation.NewBudgetAccount(validation.Budgets{MaxBufferedBytes: sourceXMLCursorBytes + 512 + sourceXMLReaderBytes + extra})
		r, err := newSourceXMLTokenReader(strings.NewReader(""), sourceXMLConfig{Context: context.Background(), Account: a})
		if err != nil {
			t.Fatal(err)
		}
		err = r.pushEntity(sourceXMLEntityInput{Name: "e"}, sourceEntityContent, sourceXMLSpan{})
		if (err == nil) != (extra == sourceXMLEntityFrameBytes) {
			t.Fatalf("extra=%d err=%v", extra, err)
		}
		if err != nil && r.entity != nil {
			t.Fatal("retention refusal allocated frame")
		}
		r.Close()
		used, _ := a.Usage(validation.BudgetBufferedBytes)
		if used != 0 {
			t.Fatalf("retained %d", used)
		}
	}
}

func TestSourceInternalEntityUnusedCycleAndLegacyCompatibility(t *testing.T) {
	source := `<!DOCTYPE r [<!ENTITY a "&a;">]><r/>`
	_, _, summary, err := sourceEntityScan(t, source)
	if err != nil {
		t.Fatalf("unused cycle promoted to fatal: %v", err)
	}
	if summary.SyntaxComplete {
		t.Fatal("unused recursion closure was guessed")
	}
	_, err = sourceScan(t, `<!DOCTYPE r [<!ENTITY a "x">]><r>&a;</r>`)
	var unsupported *sourceXMLUnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("legacy expansion changed: %v", err)
	}
}

func TestSourceEntityFrameAccountedSizes(t *testing.T) {
	if unsafe.Sizeof(sourceXMLEntityFrame{}) > sourceXMLEntityFrameBytes {
		t.Fatalf("frame size %d", unsafe.Sizeof(sourceXMLEntityFrame{}))
	}
	if unsafe.Sizeof(sourceXMLOrigin{}) > 256 {
		t.Fatalf("origin size %d", unsafe.Sizeof(sourceXMLOrigin{}))
	}
	if unsafe.Sizeof(sourceXMLOriginStep{}) > 160 {
		t.Fatalf("ancestry size %d", unsafe.Sizeof(sourceXMLOriginStep{}))
	}
	if unsafe.Sizeof(xml.Attr{})+unsafe.Sizeof(sourceXMLAttribute{}) > 256 {
		t.Fatalf("attribute+metadata size %d", unsafe.Sizeof(xml.Attr{})+unsafe.Sizeof(sourceXMLAttribute{}))
	}
}
func TestSourceEntityOriginKeepsByteOffsetAfterCoordinateOverflow(t *testing.T) {
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	r, err := newSourceXMLTokenReader(strings.NewReader("<r/>"), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	r.cursor.position.Line = 0
	r.cursor.position.Column = 0
	if _, err = r.Token(); err != nil {
		t.Fatal(err)
	}
	origin := r.Metadata().Origin
	if origin == nil || !origin.PhysicalAvailable || origin.Physical.End.ByteOffset != 4 || origin.Physical.Start.Line != 0 {
		t.Fatalf("origin=%+v", origin)
	}
}

func TestSourceInternalEntityOriginAndEmptyEnd(t *testing.T) {
	tokens, metas, _, err := sourceEntityScan(t, `<!DOCTYPE r [<!ENTITY e "<x v='z'/>">]><r>&e;</r>`)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for i, token := range tokens {
		name := ""
		switch token := token.(type) {
		case xml.StartElement:
			name = token.Name.Local
		case xml.EndElement:
			name = token.Name.Local
		}
		if name != "x" {
			continue
		}
		seen++
		origin := metas[i].Origin
		if origin == nil || origin.Kind != sourceOriginEntity || origin.Specified || origin.PhysicalAvailable || !origin.UseAvailable || !origin.DeclarationAvailable || len(origin.Ancestry) != 1 || origin.Ancestry[0].Entity != "e" {
			t.Fatalf("generated origin=%+v", origin)
		}
		if metas[i].Span != (sourceXMLSpan{}) {
			t.Fatalf("generated physical span=%+v", metas[i].Span)
		}
		if start, ok := token.(xml.StartElement); ok && len(start.Attr) != 0 {
			a := metas[i].Attributes[0].Origin
			if a == nil || a.Specified || a.PhysicalAvailable || a.Kind != sourceOriginEntity {
				t.Fatalf("attribute origin=%+v", a)
			}
		}
	}
	if seen != 2 {
		t.Fatalf("generated start/end count=%d", seen)
	}
}

func TestSourceInternalEntityAggregateWorkBoundary(t *testing.T) {
	source := `<!DOCTYPE r [<!ENTITY a "é&amp;x">]><r>&a;&a;&a;</r>`
	run := func(limit int64) (int64, error) {
		a, _ := validation.NewBudgetAccount(validation.Budgets{MaxEvents: limit})
		r, err := newSourceXMLTokenReader(strings.NewReader(source), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true})
		if err != nil {
			return 0, err
		}
		for err == nil {
			_, err = r.Token()
		}
		used, _ := a.Usage(validation.BudgetEvents)
		r.Close()
		retained, _ := a.Usage(validation.BudgetBufferedBytes)
		if retained != 0 {
			t.Fatalf("retained %d", retained)
		}
		return used, err
	}
	required, err := run(0)
	if err != io.EOF {
		t.Fatal(err)
	}
	if _, err = run(required); err != io.EOF {
		t.Fatalf("exact %d: %v", required, err)
	}
	_, err = run(required - 1)
	var resource *validation.ResourceError
	if !errors.As(err, &resource) || resource.Kind != validation.BudgetEvents {
		t.Fatalf("one-over %d: %v", required-1, err)
	}
}

func TestSourceInternalEntityFiniteAcyclicDepth(t *testing.T) {
	for _, depth := range []int{sourceXMLEntityMaxDepth, sourceXMLEntityMaxDepth + 1} {
		var source strings.Builder
		source.WriteString("<!DOCTYPE r [")
		for i := 0; i < depth; i++ {
			replacement := ""
			if i+1 < depth {
				replacement = fmt.Sprintf("&e%d;", i+1)
			}
			fmt.Fprintf(&source, "<!ENTITY e%d %q>", i, replacement)
		}
		source.WriteString("]><r>&e0;</r>")
		_, _, _, err := sourceEntityScan(t, source.String())
		if depth == sourceXMLEntityMaxDepth && err != nil {
			t.Fatal(err)
		}
		if depth > sourceXMLEntityMaxDepth {
			var resource *validation.ResourceError
			if !errors.As(err, &resource) {
				t.Fatalf("depth refusal: %v", err)
			}
		}
	}
}

func TestSourceDefaultLiteralUsesAttributeFrames(t *testing.T) {
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	r, err := newSourceXMLTokenReader(strings.NewReader(""), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true})
	if err != nil {
		t.Fatal(err)
	}
	value, err := r.dtdAttributeValue("a&#x9;'\"%x;\t\r\n", sourceDTDOrigin{})
	if err != nil || value != "a\t'\"%x;   " {
		t.Fatalf("default=%q err=%v", value, err)
	}
	if r.entity != nil {
		t.Fatal("default frame not unwound")
	}
	r.Close()
	used, _ := a.Usage(validation.BudgetBufferedBytes)
	if used != 0 {
		t.Fatalf("retained %d", used)
	}
}

func TestSourceEntityParameterFramePadding(t *testing.T) {
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	r, err := newSourceXMLTokenReader(strings.NewReader("x"), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err = r.PushDTDParameter(&sourceDTDDeclaration{Name: "p", Replacement: "é", Parameter: true}, sourceDTDOrigin{}); err != nil {
		t.Fatal(err)
	}
	for _, want := range " é " {
		got, err := r.Read()
		if err != nil || got != want {
			t.Fatalf("read=%q,%v want=%q", got, err, want)
		}
	}
	got, err := r.Peek()
	if err != nil || got != sourceDTDParameterBoundary {
		t.Fatalf("boundary=%q,%v", got, err)
	}
	if err = r.FinishDTDParameter(); err != nil {
		t.Fatal(err)
	}
	got, err = r.Read()
	if err != nil || got != 'x' {
		t.Fatalf("caller=%q,%v", got, err)
	}
	used, _ := a.Usage(validation.BudgetEvents)
	if used != 6 {
		t.Fatalf("padding/reference/frame/UTF8 work=%d want6", used)
	}
}
