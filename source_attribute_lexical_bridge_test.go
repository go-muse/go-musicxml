package musicxml

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestSourcePhysicalLexicalCapture(t *testing.T) {
	for _, tc := range []struct {
		name, input, element, effective, raw string
		state                                validation.EvidenceState
		enabled                              bool
	}{
		{"physical-before-tokenized", `<!DOCTYPE r [<!ATTLIST r a NMTOKENS #IMPLIED>]><r a="  one   two  "/>`, "r", "one two", "  one   two  ", validation.EvidenceKnown, true},
		{"physical-xml-normalized", "<!DOCTYPE r [<!ATTLIST r a CDATA #IMPLIED>]><r a=\"a\r\nb&#x9;&amp;c\"/>", "r", "a b\t&c", "a b\t&c", validation.EvidenceKnown, true},
		{"physical-general-replacement", `<!DOCTYPE r [<!ENTITY e 'one'><!ATTLIST r a NMTOKENS #IMPLIED>]><r a="  &e;  two "/>`, "r", "one two", "", validation.EvidenceUnknown, true},
		{"physical-after-transformed", `<!DOCTYPE r [<!ENTITY e 'one'>]><r z="&e;" a=" original "/>`, "r", " original ", " original ", validation.EvidenceKnown, true},
		{"physical-empty-replacement", `<!DOCTYPE r [<!ENTITY e ''>]><r a="&e;"/>`, "r", "", "", validation.EvidenceUnknown, true},
		{"defaulted", `<!DOCTYPE r [<!ATTLIST r a NMTOKENS ' one   two '>]><r/>`, "r", "one two", "", validation.EvidenceAbsent, true},
		{"entity-generated", `<!DOCTYPE r [<!ENTITY e '<child a="one"/>'>]><r>&e;</r>`, "child", "one", "", validation.EvidenceAbsent, true},
		{"unselected", `<!DOCTYPE r [<!ATTLIST r a NMTOKENS #IMPLIED>]><r a=" one   two "/>`, "r", "one two", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := dtdStoreAccount(t, validation.Budgets{})
			config := sourceXMLConfig{Context: context.Background(), Account: account, InternalDTD: true}
			config.CaptureAttributeLexical = tc.enabled
			reader, err := newSourceXMLTokenReader(strings.NewReader(tc.input), config)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for {
				token, err := reader.Token()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				start, ok := token.(xml.StartElement)
				if !ok || start.Name.Local != tc.element {
					continue
				}
				for i, attribute := range start.Attr {
					if attribute.Name.Local != "a" {
						continue
					}
					found = true
					raw := reader.Metadata().Attributes[i].Lexical
					if attribute.Value != tc.effective || raw.State != tc.state || raw.Value != tc.raw {
						t.Errorf("effective=%q raw=%+v, want effective=%q raw=%s/%q", attribute.Value, raw, tc.effective, tc.state, tc.raw)
					}
				}
			}
			if !found {
				t.Error("expected attribute missing")
			}
			if err := reader.Close(); err != nil {
				t.Fatal(err)
			}
			if used, _ := account.Usage(validation.BudgetBufferedBytes); used != 0 {
				t.Errorf("live buffer charge=%d", used)
			}
		})
	}
}

type sourceLexicalTestSink struct {
	recordingSourceFacts
	calls     int
	provider  validation.RuleKey
	subject   validation.NodeRef
	attribute validation.QName
	raw       validation.Evidence[string]
}

func (s *sourceLexicalTestSink) SourceAttributeLexical(_ context.Context, provider validation.RuleKey, subject validation.NodeRef, attribute validation.QName, raw validation.Evidence[string]) error {
	s.calls++
	s.provider, s.subject, s.attribute, s.raw = provider, subject, attribute, raw
	return nil
}
func TestSourcePhysicalLexicalObserverTruthfulProjection(t *testing.T) {
	key := validation.RuleKey{ID: "test.origin", Revision: 1}
	for _, tc := range []struct {
		name  string
		meta  sourceXMLAttribute
		state validation.EvidenceState
		value string
	}{
		{"captured", sourceXMLAttribute{Origin: &sourceXMLOrigin{Kind: sourceOriginPhysical, Specified: true, PhysicalAvailable: true}, Lexical: validation.Evidence[string]{State: validation.EvidenceKnown, Value: " before  normalization "}}, validation.EvidenceKnown, " before  normalization "},
		{"unretained", sourceXMLAttribute{Origin: &sourceXMLOrigin{Kind: sourceOriginPhysical, Specified: true, PhysicalAvailable: true}}, validation.EvidenceUnknown, ""},
		{"unavailable-origin", sourceXMLAttribute{Lexical: validation.Evidence[string]{State: validation.EvidenceKnown, Value: "must not infer"}}, validation.EvidenceUnknown, ""},
		{"defaulted", sourceXMLAttribute{Origin: &sourceXMLOrigin{Kind: sourceOriginDefault}, Lexical: validation.Evidence[string]{State: validation.EvidenceKnown, Value: "must not infer"}}, validation.EvidenceAbsent, ""},
		{"entity-generated", sourceXMLAttribute{Origin: &sourceXMLOrigin{Kind: sourceOriginEntity}}, validation.EvidenceAbsent, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := dtdStoreAccount(t, validation.Budgets{})
			sink := &sourceLexicalTestSink{}
			observer, err := newSourceFactObserver(context.Background(), sink, account)
			if err != nil {
				t.Fatal(err)
			}
			observer.captureAttributeLexical = true
			observer.origins = &sourceOriginObserver{key: key}
			if err := observer.publishSourceAttributeLexical(&sourceObserverFrame{id: 7}, validation.QName{Local: "href"}, tc.meta); err != nil {
				t.Fatal(err)
			}
			if sink.calls != 1 || sink.provider != key || sink.subject != (validation.NodeRef{DocumentID: "source", Node: 7}) || sink.attribute != (validation.QName{Local: "href"}) || sink.raw.State != tc.state || sink.raw.Value != tc.value {
				t.Errorf("lexical=%+v", sink)
			}
			if sink.raw.State == validation.EvidenceUnknown && sink.raw.Reason == "" {
				t.Error("missing unavailable reason")
			}
			if err := observer.close(); err != nil {
				t.Fatal(err)
			}
			if used, _ := account.Usage(validation.BudgetBufferedBytes); used != 0 {
				t.Errorf("live buffers=%d", used)
			}
		})
	}
}
func TestSourcePhysicalLexicalRequiresElectedIngress(t *testing.T) {
	account := dtdStoreAccount(t, validation.Budgets{})
	observer, err := newSourceFactObserver(context.Background(), &recordingSourceFacts{}, account)
	if err != nil {
		t.Fatal(err)
	}
	defer observer.close()
	frame := &sourceObserverFrame{id: 1}
	if err := observer.publishSourceAttributeLexical(frame, validation.QName{Local: "href"}, sourceXMLAttribute{}); err != nil {
		t.Errorf("unelected observer changed: %v", err)
	}
	observer.captureAttributeLexical = true
	if err := observer.publishSourceAttributeLexical(frame, validation.QName{Local: "href"}, sourceXMLAttribute{}); !errors.Is(err, validation.ErrInvalidConfiguration) {
		t.Errorf("missing selected ingress=%v", err)
	}
	if _, err := newSourceXMLTokenReader(strings.NewReader(`<r/>`), sourceXMLConfig{Context: context.Background(), Account: account, CaptureAttributeLexical: true}); !errors.Is(err, validation.ErrInvalidConfiguration) {
		t.Errorf("capture without DTD capability=%v", err)
	}
}
