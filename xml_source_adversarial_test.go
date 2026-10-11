package musicxml

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/go-muse/go-musicxml/validation"
)

func TestSourceForeignRootRetainsLiteralURI(t *testing.T) {
	for _, uri := range []string{"xml", "xmlns", "xlink", "urn:vendor"} {
		_, err := Decode(strings.NewReader(`<p:opus xmlns:p="` + uri + `"/>`))
		var root *UnsupportedRootError
		if !errors.As(err, &root) {
			t.Fatalf("%s: %v", uri, err)
		}
		if root.Name.Space != uri {
			t.Errorf("namespace %q became %q", uri, root.Name.Space)
		}
	}
}
func TestSourceEmptyDeclarationValues(t *testing.T) {
	for _, input := range []string{`<?xml version="1.0" encoding=""?><a/>`, `<?xml version="1.0" standalone=""?><a/>`} {
		if _, err := sourceScan(t, input); err == nil {
			t.Errorf("accepted %s", input)
		}
	}
}
func TestSourceMetadataOriginalBytes(t *testing.T) {
	source := `<a xmlns:p="u" p:x="é&#x9;">𝄞</a>`
	for _, tc := range []struct {
		name             string
		input            []byte
		tag, attr, value int64
	}{
		{"utf8", []byte(source), 0, 15, 20},
		{"utf8-bom", append([]byte{239, 187, 191}, []byte(source)...), 3, 18, 23},
		{"utf16be", encodeUTF16(source, binary.BigEndian), 2, 32, 42},
		{"utf16le", encodeUTF16(source, binary.LittleEndian), 2, 32, 42},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := newSourceXMLTokenReader(iotest.OneByteReader(bytes.NewReader(tc.input)), sourceXMLConfig{Context: context.Background()})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			d := xml.NewTokenDecoder(s)
			token, err := d.Token()
			if err != nil {
				t.Fatal(err)
			}
			start := token.(xml.StartElement)
			meta := s.Metadata()
			if meta.Span.Start.ByteOffset != tc.tag || meta.Attributes[1].NameSpan.Start.ByteOffset != tc.attr || meta.Attributes[1].ValueSpan.Start.ByteOffset != tc.value {
				t.Fatalf("metadata %+v", meta)
			}
			if !meta.Attributes[0].NamespaceDeclaration || meta.Attributes[1].NamespaceDeclaration || start.Attr[1].Value != "é\t" {
				t.Fatalf("metadata/value %+v %+v", meta, start)
			}
			for err == nil {
				_, err = d.Token()
			}
			if err != io.EOF {
				t.Fatal(err)
			}
			summary := s.Summary()
			if !summary.DigestComplete || !summary.SyntaxComplete || summary.BytesConsumed != int64(len(tc.input)) || summary.ReadPrefixSHA256 != sha256.Sum256(tc.input) {
				t.Fatalf("summary %+v", summary)
			}
		})
	}
}
func TestSourceBudgetCleanupAtEveryReservation(t *testing.T) {
	input := `<a xmlns:p="u" p:x="abcdef"><p:b xmlns:q="v" q:y="0123456789">text</p:b></a>`
	for limit := int64(1); limit < 15000; limit += 31 {
		a, _ := validation.NewBudgetAccount(validation.Budgets{MaxBufferedBytes: limit})
		s, err := newSourceXMLTokenReader(strings.NewReader(input), sourceXMLConfig{Context: context.Background(), Account: a})
		if err == nil {
			for err == nil {
				_, err = s.Token()
			}
			s.Close()
		}
		used, _ := a.Usage(validation.BudgetBufferedBytes)
		if used != 0 {
			t.Fatalf("limit %d leak %d error %v", limit, used, err)
		}
	}
}
func TestSourceInputIOAndPartialDigest(t *testing.T) {
	failure := errors.New("source I/O sentinel")
	for _, tail := range []error{io.EOF, failure} {
		r := &sourceDataErrorReader{data: []byte(`<a/>`), err: tail}
		s, err := newSourceXMLTokenReader(r, sourceXMLConfig{Context: context.Background()})
		if err != nil {
			t.Fatal(err)
		}
		for err == nil {
			_, err = s.Token()
		}
		if !errors.Is(err, tail) {
			t.Fatalf("want %v got %v", tail, err)
		}
		if s.Summary().DigestComplete != (tail == io.EOF) {
			t.Fatalf("summary %+v", s.Summary())
		}
		s.Close()
	}
}

type sourceDataErrorReader struct {
	data []byte
	err  error
}

func (r *sourceDataErrorReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, r.err
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 {
		return n, r.err
	}
	return n, nil
}

func TestSourceExactTokenContentLimit(t *testing.T) {
	for _, input := range []string{`<!--xxxxxxxxxxxxxxxx--><a/>`, `<?p xxxxxxxxxxxxxxxx?><a/>`, `<a><![CDATA[xxxxxxxxxxxxxxxx]]></a>`, `<a>xxxxxxxxxxxxxxxx</a>`, `<a x="xxxxxxxxxxxxxxxx"/>`} {
		a, _ := validation.NewBudgetAccount(validation.Budgets{MaxScalarBytes: 16})
		s, err := newSourceXMLTokenReader(strings.NewReader(input), sourceXMLConfig{Context: context.Background(), Account: a})
		if err != nil {
			t.Fatal(err)
		}
		for err == nil {
			_, err = s.Token()
		}
		s.Close()
		if err != io.EOF {
			t.Errorf("exact-limit %s: %v", input, err)
		}
	}
}

func TestSourceDTDDependentNamespaceIsUnsupported(t *testing.T) {
	for _, input := range []string{`<!DOCTYPE a [<!ATTLIST a xmlns:p CDATA "urn:p">]><a><p:b/></a>`, `<!DOCTYPE a SYSTEM "unused.dtd"><a p:x="value"/>`} {
		_, err := sourceScan(t, input)
		var unsupported *sourceXMLUnsupportedError
		if !errors.As(err, &unsupported) {
			t.Fatalf("DTD-dependent binding classified as %T: %v", err, err)
		}
		var syntax *xml.SyntaxError
		if errors.As(err, &syntax) {
			t.Fatalf("unavailable default is not malformed: %v", err)
		}
	}
}

func TestSourceDTDWellFormednessIsNotInvented(t *testing.T) {
	for _, input := range []string{`<!DOCTYPE a [<!ATTLIST a x CDATA "&missing;">]><a/>`, `<!DOCTYPE a [<!ENTITY e "&e;">]><a/>`} {
		s, err := newSourceXMLTokenReader(strings.NewReader(input), sourceXMLConfig{Context: context.Background()})
		if err != nil {
			t.Fatal(err)
		}
		for err == nil {
			_, err = s.Token()
		}
		if err != io.EOF {
			t.Fatal(err)
		}
		summary := s.Summary()
		if summary.SyntaxComplete {
			t.Fatalf("unassessed declaration WFCs called complete: %+v", summary)
		}
		if !summary.DigestComplete {
			t.Fatal("valid EOF observation was lost")
		}
		s.Close()
	}
}

func TestSourceCursorAvoidsPerCharacterAllocation(t *testing.T) {
	input := "<r>" + strings.Repeat("<a>x</a>", 1000) + "</r>"
	allocations := testing.AllocsPerRun(3, func() {
		s, err := newSourceXMLTokenReader(strings.NewReader(input), sourceXMLConfig{Context: context.Background()})
		if err != nil {
			panic(err)
		}
		for err == nil {
			_, err = s.Token()
		}
		if err != io.EOF {
			panic(err)
		}
		s.Close()
	})
	if allocations > 12000 {
		t.Fatalf("source scan allocated %.0f objects; per-character caching must be allocation-free", allocations)
	}
}

func TestSourceDepthLimitPreservesResourceIdentity(t *testing.T) {
	for _, test := range []struct {
		name                  string
		legacy                int
		account               int64
		resource, legacyError bool
	}{
		{"legacy only", 2, 0, false, true},
		{"legacy tighter", 2, 3, true, true},
		{"account tighter", 3, 2, true, false},
		{"account equal", 2, 2, true, false},
		{"account only", 0, 2, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var account *validation.Account
			if test.account != 0 {
				var err error
				account, err = validation.NewBudgetAccount(validation.Budgets{MaxDepth: test.account})
				if err != nil {
					t.Fatal(err)
				}
			}
			reader, err := newSourceXMLTokenReader(strings.NewReader(`<a><b><c/></b></a>`), sourceXMLConfig{Context: context.Background(), MaxDepth: test.legacy, Account: account})
			if err != nil {
				t.Fatal(err)
			}
			for err == nil {
				_, err = reader.Token()
			}
			var limit *validation.ResourceError
			if errors.As(err, &limit) != test.resource || errors.Is(err, ErrXMLTooDeep) != test.legacyError {
				t.Fatalf("wrong error identity: %v", err)
			}
			if test.resource && (limit.Kind != validation.BudgetDepth || limit.Limit != 2 || limit.Used != 2 || limit.Requested != 1) {
				t.Fatalf("wrong limit details: %+v", limit)
			}
			if err := reader.Close(); err != nil {
				t.Fatal(err)
			}
			if account != nil {
				for _, kind := range []validation.BudgetKind{validation.BudgetDepth, validation.BudgetBufferedBytes} {
					used, _ := account.Usage(kind)
					if used != 0 {
						t.Fatalf("%s gauge = %d", kind, used)
					}
				}
			}
		})
	}
}
