package musicxml

import (
	"context"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/go-muse/go-musicxml/validation"
)

type recordingSourceFacts struct {
	headers    []validation.NodeHeader
	attributes []validation.AttributeFact
	ends       []validation.NodeEnd
}

func (r *recordingSourceFacts) OpenNode(_ context.Context, h validation.NodeHeader) error {
	h.Bindings = append([]validation.Binding(nil), h.Bindings...)
	r.headers = append(r.headers, h)
	return nil
}
func (r *recordingSourceFacts) Attribute(_ context.Context, a validation.AttributeFact) error {
	r.attributes = append(r.attributes, a)
	return nil
}
func (r *recordingSourceFacts) CompleteAttributes(context.Context, validation.NodeID, validation.Evidence[bool]) error {
	return nil
}
func (r *recordingSourceFacts) CloseNode(_ context.Context, e validation.NodeEnd) error {
	r.ends = append(r.ends, e)
	return nil
}

func recordSourceFacts(t *testing.T, input string, budgets validation.Budgets) (*recordingSourceFacts, *validation.Account, error) {
	t.Helper()
	account, err := validation.NewBudgetAccount(budgets)
	if err != nil {
		t.Fatal(err)
	}
	parser, err := newSourceXMLTokenReader(strings.NewReader(input), sourceXMLConfig{Context: context.Background(), Account: account, MaxDepth: 256})
	if err != nil {
		t.Fatal(err)
	}
	facts := &recordingSourceFacts{}
	observer, err := newSourceFactObserver(context.Background(), facts, account)
	if err != nil {
		t.Fatal(err)
	}
	source := &observingXMLSource{ctx: context.Background(), lexical: parser, decoder: xml.NewTokenDecoder(parser), observer: observer}
	for {
		_, err = source.Token()
		if err != nil {
			break
		}
	}
	if errors.Is(err, io.EOF) {
		err = nil
	}
	if e := observer.close(); e != nil {
		t.Error(e)
	}
	if e := parser.Close(); e != nil {
		t.Error(e)
	}
	return facts, account, err
}

func TestSourceObserverRetainsUnprojectedFactsAndScopes(t *testing.T) {
	input := `<score-partwise xmlns:f="urn:foreign" xmlns:a="xmlns" version="4.0" f:version="original"><f:ignored xmlns:f="urn:inner" a:x="physical"><staves rubbish="yes">1<!-- split -->2<![CDATA[3]]></staves></f:ignored><f:again/></score-partwise>`
	facts, account, err := recordSourceFacts(t, input, validation.Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.headers) != 4 || len(facts.attributes) != 7 || len(facts.ends) != 4 {
		t.Fatalf("facts: headers=%d attributes=%d ends=%d", len(facts.headers), len(facts.attributes), len(facts.ends))
	}
	if facts.headers[1].Name.Space != "urn:inner" || facts.headers[3].Name.Space != "urn:foreign" {
		t.Errorf("namespace scopes not preserved: %+v", facts.headers)
	}
	if got := facts.attributes[0].Name; got != (validation.QName{Space: sourceXMLNSNamespace, Local: "f"}) {
		t.Errorf("declaration name=%+v", got)
	}
	if got := facts.attributes[5].Name; got != (validation.QName{Space: "xmlns", Local: "x"}) {
		t.Errorf("lookalike ordinary attribute=%+v", got)
	}
	if facts.attributes[3].Scalar.Lexeme.Value != "original" || facts.attributes[6].Scalar.Lexeme.Value != "yes" || facts.ends[0].Scalar.Lexeme.Value != "123" {
		t.Errorf("lost original lexemes: %+v / %+v", facts.attributes, facts.ends)
	}
	for _, h := range facts.headers {
		if h.Type.State != validation.EvidenceUnknown || h.Type.Reason != "schema-binding-pending" || h.Declaration.State != validation.EvidenceUnknown {
			t.Errorf("invented binding: %+v", h)
		}
	}
	position := facts.attributes[6].Location.Position
	if position.CoordinateSpace != "original-bytes" || position.ByteOffset != int64(strings.Index(input, "rubbish")) || position.Line != 1 || position.Column != strings.Index(input, "rubbish")+1 {
		t.Errorf("physical attribute position=%+v", position)
	}
	for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth, validation.BudgetNodes, validation.BudgetAttributes} {
		used, _ := account.Usage(kind)
		if used != 0 {
			t.Errorf("observer incorrectly owns/leaks %s: %d", kind, used)
		}
	}
}

func TestSourceObserverBoundsConcatenatedScalar(t *testing.T) {
	_, account, err := recordSourceFacts(t, `<r>a<!--s-->b<![CDATA[c]]></r>`, validation.Budgets{MaxScalarBytes: 2})
	var resource *validation.ResourceError
	if !errors.As(err, &resource) || resource.Kind != validation.BudgetScalarBytes {
		t.Fatalf("aggregate scalar: %v", err)
	}
	used, _ := account.Usage(validation.BudgetBufferedBytes)
	if used != 0 {
		t.Errorf("leaked buffers=%d", used)
	}
}

func TestSourceObserverDTDUnavailableFactsRemainExplicit(t *testing.T) {
	facts, _, err := recordSourceFacts(t, `<!DOCTYPE r [<!ATTLIST r a NMTOKENS #IMPLIED>]><r a="a  b"/>`, validation.Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	if facts.headers[0].Declaration.State != validation.EvidenceUnsupported || facts.headers[0].Declaration.Reason != "dtd-infoset-unavailable" || facts.attributes[0].Scalar.Lexeme.State != validation.EvidenceUnsupported || facts.ends[0].AttributesComplete.State != validation.EvidenceUnsupported {
		t.Fatalf("invented DTD infoset: %+v %+v %+v", facts.headers, facts.attributes, facts.ends)
	}
}

func TestSourceObserverOriginalCoordinatesAcrossEncoding(t *testing.T) {
	for _, input := range []string{"\ufeff<r>\r\n <a bad=\"x\"/>\r</r>", "<r>é\n <a bad=\"x\"/></r>"} {
		facts, _, err := recordSourceFacts(t, input, validation.Budgets{})
		if err != nil {
			t.Fatal(err)
		}
		got := facts.attributes[0].Location.Position
		want := &validation.SourcePosition{ByteOffset: int64(strings.Index(input, "bad")), Line: 2, Column: 5, CoordinateSpace: "original-bytes"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("position got %+v want %+v", got, want)
		}
	}
}

func TestSourceObserverReleasesOnlyOwnedBuffersAfterFailure(t *testing.T) {
	account, _ := validation.NewBudgetAccount(validation.Budgets{MaxScalarBytes: 1})
	if err := account.Charge(validation.BudgetBufferedBytes, 73); err != nil {
		t.Fatal(err)
	}
	observer, err := newSourceFactObserver(context.Background(), &recordingSourceFacts{}, account)
	if err != nil {
		t.Fatal(err)
	}
	if err = observer.observe(xml.StartElement{Name: xml.Name{Local: "r"}}, sourceXMLTokenMeta{Span: sourceXMLSpan{Start: sourceXMLPosition{Line: 1, Column: 1}}}); err != nil {
		t.Fatal(err)
	}
	if err = observer.observe(xml.CharData("ab"), sourceXMLTokenMeta{}); !errors.Is(err, validation.ErrResourceLimit) {
		t.Fatalf("scalar failure=%v", err)
	}
	if err = observer.close(); err != nil {
		t.Fatal(err)
	}
	used, _ := account.Usage(validation.BudgetBufferedBytes)
	if used != 73 {
		t.Errorf("sibling ownership was released: %d", used)
	}
}

func TestSourceObserverOriginalUTF16AndLatin1Coordinates(t *testing.T) {
	text := "<r>𝄞\r\n<a bad=\"x\"/></r>"
	prefix := text[:strings.Index(text, "bad")]
	for _, order := range []binary.ByteOrder{binary.BigEndian, binary.LittleEndian} {
		units := append([]uint16{0xfeff}, utf16.Encode([]rune(text))...)
		data := make([]byte, 2*len(units))
		for i, u := range units {
			order.PutUint16(data[i*2:], u)
		}
		facts, _, err := recordSourceFacts(t, string(data), validation.Budgets{})
		if err != nil {
			t.Fatal(err)
		}
		got := facts.attributes[0].Location.Position
		if got.ByteOffset != int64(2+2*len(utf16.Encode([]rune(prefix)))) || got.Line != 2 || got.Column != 4 || got.CoordinateSpace != "original-bytes" {
			t.Errorf("UTF16 original position: %+v", got)
		}
	}
	latin := []byte("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><r>\xe9\r\n<a bad=\"x\"/></r>")
	facts, _, err := recordSourceFacts(t, string(latin), validation.Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	got := facts.attributes[0].Location.Position
	if got.ByteOffset != int64(strings.Index(string(latin), "bad")) || got.Line != 2 || got.Column != 4 {
		t.Errorf("Latin1 original position: %+v", got)
	}
}

func TestSourceObserverIdentityCapacityIsCapability(t *testing.T) {
	for _, input := range []string{
		"<" + strings.Repeat("a", 1025) + "/>",
		`<r ` + strings.Repeat("a", 1025) + `="x"/>`,
		`<r xmlns="` + strings.Repeat("a", 4097) + `"/>`,
		`<r xmlns:` + strings.Repeat("a", 1025) + `="urn:test"/>`,
	} {
		facts, account, err := recordSourceFacts(t, input, validation.Budgets{})
		var incomplete *validation.IncompleteError
		var malformed *xml.SyntaxError
		if !errors.As(err, &incomplete) || incomplete.Reason != "source-fact-identity-limit" || errors.As(err, &malformed) || errors.Is(err, validation.ErrInvalidConfiguration) {
			t.Errorf("legal source capacity classification: %v", err)
		}
		if len(facts.headers) != 0 {
			t.Errorf("over-capacity facts admitted=%+v", facts.headers)
		}
		used, _ := account.Usage(validation.BudgetBufferedBytes)
		if used != 0 {
			t.Errorf("capacity leak=%d", used)
		}
	}
}

func TestSourceObserverOmitsOverlongDisplayPath(t *testing.T) {
	name := strings.Repeat("a", 1000)
	input := strings.Repeat("<"+name+">", 20) + "<child/>" + strings.Repeat("</"+name+">", 20)
	facts, _, err := recordSourceFacts(t, input, validation.Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.headers) != 21 {
		t.Fatalf("wrong occurrence count=%d", len(facts.headers))
	}
	for i, h := range facts.headers {
		if h.ID != validation.NodeID(i+1) || h.Parent != validation.NodeID(i) || h.Location.Position == nil {
			t.Errorf("lost occurrence/position: %+v", h)
		}
		if len(h.Location.Path) > 16384 {
			t.Errorf("oversized path=%d", len(h.Location.Path))
		}
		if i >= 16 && h.Location.Path != "" {
			t.Errorf("invented truncated path on node%d", i+1)
		}
	}
}

func TestSourceObserverScalarCapacityAdmission(t *testing.T) {
	account, _ := validation.NewBudgetAccount(validation.Budgets{})
	observer, err := newSourceFactObserver(context.Background(), &recordingSourceFacts{}, account)
	if err != nil {
		t.Fatal(err)
	}
	defer observer.close()
	if err := observer.observe(xml.StartElement{Name: xml.Name{Local: "r"}}, sourceXMLTokenMeta{}); err != nil {
		t.Fatal(err)
	}
	frame := observer.stack[0]
	before := frame.retained
	for i := 0; i < 100; i++ {
		if err := observer.observe(xml.CharData("x"), sourceXMLTokenMeta{}); err != nil {
			t.Fatal(err)
		}
		if retained := frame.retained - before; retained < int64(frame.text.Cap()) {
			t.Fatalf("text backing capacity %d exceeds admitted bytes %d", frame.text.Cap(), retained)
		}
	}
}

func TestSourceObserverNamespaceCapacityPersistsAfterScope(t *testing.T) {
	account, _ := validation.NewBudgetAccount(validation.Budgets{})
	observer, err := newSourceFactObserver(context.Background(), &recordingSourceFacts{}, account)
	if err != nil {
		t.Fatal(err)
	}
	defer observer.close()
	// Isolate namespace table capacity from independently retained last-location
	// snapshots and open-scope storage. Deleting bindings does not shrink its map.
	before := observer.retained - observer.lastRetained
	for _, prefix := range []string{"p", "q", "r"} {
		token := xml.StartElement{Name: xml.Name{Local: "root"}, Attr: []xml.Attr{{Name: xml.Name{Space: "xmlns", Local: prefix}, Value: "urn:test"}}}
		if err := observer.observe(token, sourceXMLTokenMeta{Attributes: []sourceXMLAttribute{{NamespaceDeclaration: true}}}); err != nil {
			t.Fatal(err)
		}
		if err := observer.observe(xml.EndElement{Name: token.Name}, sourceXMLTokenMeta{}); err != nil {
			t.Fatal(err)
		}
	}
	if retained := observer.retained - observer.lastRetained; retained <= before {
		t.Fatalf("namespace backing map released at scope close: before=%d after=%d", before, retained)
	}
}
