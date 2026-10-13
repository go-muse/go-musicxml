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

	"github.com/go-muse/go-musicxml/validation"
)

func TestDTDAuditContextAndBoundaryMatrix(t *testing.T) {
	for _, tc := range []struct {
		name, source, want string
		syntax             bool
	}{
		{"literal-pi", `<!DOCTYPE r [<!ENTITY e "<?p &e; &#37;p;?>text">]><r>&e;</r>`, "text", false},
		{"literal-cdata", `<!DOCTYPE r [<!ENTITY e "<![CDATA[&e; &#37;p;]]>">]><r>&e;</r>`, "&e; %p;", false},
		{"numeric-constructs-reference", `<!DOCTYPE r [<!ENTITY e "&#38;f;"><!ENTITY f "ok">]><r>&e;</r>`, "ok", false},
		{"numeric-constructs-cycle", `<!DOCTYPE r [<!ENTITY e "&#38;e;">]><r>&e;</r>`, "", true},
		{"double-escaped-literal", `<!DOCTYPE r [<!ENTITY e "&#38;#38;e;">]><r>&e;</r>`, "&e;", false},
		{"percent-literal", `<!DOCTYPE r [<!ENTITY e "&#37;p;">]><r a="%p;">&e;%p;</r>`, "%p;%p;", false},
		{"nested-empty-sibling", `<!DOCTYPE r [<!ENTITY e "<x>&f;</x><y/>"><!ENTITY f "">]><r>&e;tail</r>`, "tail", false},
		{"split-start-name", `<!DOCTYPE r [<!ENTITY e "<x">]><r>&e;/></r>`, "", true},
		{"split-end-name", `<!DOCTYPE r [<!ENTITY e "<x></x">]><r>&e;></r>`, "", true},
		{"split-comment", `<!DOCTYPE r [<!ENTITY e "<!--x--">]><r>&e;></r>`, "", true},
		{"split-cdata", `<!DOCTYPE r [<!ENTITY e "<![CDATA[x]]">]><r>&e;></r>`, "", true},
		{"split-pi", `<!DOCTYPE r [<!ENTITY e "<?p x?">]><r>&e;></r>`, "", true},
		{"split-attribute", `<!DOCTYPE r [<!ENTITY e "<x a='v">]><r>&e;'/></r>`, "", true},
		{"split-numeric-ref", `<!DOCTYPE r [<!ENTITY e "&#38;#65">]><r>&e;;</r>`, "", true},
		{"split-general-ref", `<!DOCTYPE r [<!ENTITY e "&#38;amp">]><r>&e;;</r>`, "", true},
		{"nested-close-parent", `<!DOCTYPE r [<!ENTITY e "<x>&f;</x>"><!ENTITY f "</x><x>">]><r>&e;</r>`, "", true},
		{"nested-open-parent-close", `<!DOCTYPE r [<!ENTITY e "&f;</x>"><!ENTITY f "<x>">]><r>&e;</r>`, "", true},
		{"bom-is-content", `<!DOCTYPE r [<!ENTITY e "&#xFEFF;">]><r>&e;</r>`, "\ufeff", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokens, _, _, err := sourceEntityScan(t, tc.source)
			var syntax *xml.SyntaxError
			if tc.syntax {
				if !errors.As(err, &syntax) {
					t.Fatalf("want XML syntax error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var b strings.Builder
			for _, token := range tokens {
				if c, ok := token.(xml.CharData); ok {
					b.Write(c)
				}
			}
			if b.String() != tc.want {
				t.Fatalf("text=%q want %q", b.String(), tc.want)
			}
		})
	}
}

func TestDTDAuditOriginalBytesAndEarlyClose(t *testing.T) {
	source := `<!DOCTYPE r [<!ENTITY e "<x a='&quot;'/>é&#13;">]><r>&e;&e;</r>`
	for name, data := range map[string][]byte{"utf8": []byte(source), "bom": append([]byte{0xef, 0xbb, 0xbf}, []byte(source)...), "utf16le": encodeUTF16(source, binary.LittleEndian), "utf16be": encodeUTF16(source, binary.BigEndian)} {
		for _, stop := range []int{-1, 1, 2, 3, 4, 5} {
			t.Run(name+string(rune('A'+stop+1)), func(t *testing.T) {
				a, _ := validation.NewBudgetAccount(validation.Budgets{})
				const foreign = 33
				if err := a.Charge(validation.BudgetBufferedBytes, foreign); err != nil {
					t.Fatal(err)
				}
				r, err := newSourceXMLTokenReader(bytes.NewReader(data), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true})
				if err != nil {
					t.Fatal(err)
				}
				count := 0
				for err == nil && (stop < 0 || count < stop) {
					_, err = r.Token()
					count++
				}
				if stop < 0 {
					if err != io.EOF {
						t.Fatal(err)
					}
					s := r.Summary()
					if !s.InputComplete || !s.DigestComplete || s.BytesConsumed != int64(len(data)) || s.ReadPrefixSHA256 != sha256.Sum256(data) {
						t.Fatalf("bad original summary %+v", s)
					}
				}
				r.Close()
				r.Close()
				if n, _ := a.Usage(validation.BudgetBufferedBytes); n != foreign {
					t.Fatalf("foreign ownership altered: %d", n)
				}
				if n, _ := a.Usage(validation.BudgetInputBytes); n != r.Summary().BytesRead {
					t.Fatalf("physical bytes charged %d read %d", n, r.Summary().BytesRead)
				}
				if n, _ := a.Usage(validation.BudgetExternalBytes); n != 0 {
					t.Fatalf("external bytes charged %d", n)
				}
			})
		}
	}
}

func TestDTDAuditBudgetSweepCleanup(t *testing.T) {
	source := `<!DOCTYPE r [<!ENTITY e "<x xmlns:p='u' p:a='&quot;'>abcdef</x>"><!ATTLIST r a CDATA 'x'>]><r>&e;&e;</r>`
	for _, kind := range []validation.BudgetKind{validation.BudgetEvents, validation.BudgetBufferedBytes, validation.BudgetScalarBytes} {
		max := int64(150)
		if kind == validation.BudgetBufferedBytes {
			max = 20000
		}
		step := int64(1)
		if kind == validation.BudgetBufferedBytes {
			step = 83
		}
		for n := int64(1); n < max; n += step {
			limits := validation.Budgets{}
			switch kind {
			case validation.BudgetEvents:
				limits.MaxEvents = n
			case validation.BudgetBufferedBytes:
				limits.MaxBufferedBytes = n
			case validation.BudgetScalarBytes:
				limits.MaxScalarBytes = n
			}
			a, _ := validation.NewBudgetAccount(limits)
			r, err := newSourceXMLTokenReader(strings.NewReader(source), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true})
			if r != nil {
				for err == nil {
					_, err = r.Token()
				}
				r.Close()
			}
			var resource *validation.ResourceError
			if err != io.EOF && !errors.As(err, &resource) {
				t.Fatalf("kind %s limit %d unexpected %v", kind, n, err)
			}
			if live, _ := a.Usage(validation.BudgetBufferedBytes); live != 0 {
				t.Fatalf("kind %s limit %d retained %d", kind, n, live)
			}
		}
	}
}

type dtdAuditCountingContext struct {
	context.Context
	cancel    context.CancelFunc
	calls, at int
}

func (c *dtdAuditCountingContext) Err() error {
	c.calls++
	if c.calls >= c.at {
		c.cancel()
	}
	return c.Context.Err()
}

func TestDTDAuditCancellationSweep(t *testing.T) {
	source := `<!DOCTYPE r [<!ENTITY a "<x a='&b;'>&b;</x>"><!ENTITY b "&c;&c;"><!ENTITY c "abcdef&#13;">]><r>&a;</r>`
	for at := 1; at < 700; at++ {
		ctx, cancel := context.WithCancel(context.Background())
		counted := &dtdAuditCountingContext{Context: ctx, cancel: cancel, at: at}
		a, _ := validation.NewBudgetAccount(validation.Budgets{})
		r, err := newSourceXMLTokenReader(strings.NewReader(source), sourceXMLConfig{Context: counted, Account: a, InternalDTD: true})
		if r != nil {
			for err == nil {
				_, err = r.Token()
			}
			r.Close()
		}
		if err != io.EOF && !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel %d: %v", at, err)
		}
		if used, _ := a.Usage(validation.BudgetBufferedBytes); used != 0 {
			t.Fatalf("cancel %d retained %d", at, used)
		}
		cancel()
	}
}

type dtdAuditFinalReader struct {
	data  []byte
	final error
}

func (r *dtdAuditFinalReader) Read(p []byte) (int, error) {
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 {
		return n, r.final
	}
	return n, nil
}
func TestDTDAuditDataAndFinalError(t *testing.T) {
	data := []byte(`<!DOCTYPE r [<!ENTITY e "<x/>abc">]><r>&e;</r>`)
	sentinel := errors.New("audit read failure")
	for _, final := range []error{io.EOF, sentinel} {
		a, _ := validation.NewBudgetAccount(validation.Budgets{})
		r, err := newSourceXMLTokenReader(&dtdAuditFinalReader{data: data, final: final}, sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true})
		if err != nil {
			t.Fatal(err)
		}
		for err == nil {
			_, err = r.Token()
		}
		if !errors.Is(err, final) {
			t.Fatalf("final %v got %v", final, err)
		}
		s := r.Summary()
		if s.ReadPrefixSHA256 != sha256.Sum256(data) || s.BytesRead != int64(len(data)) || s.InputComplete != (final == io.EOF) {
			t.Fatalf("summary %+v", s)
		}
		r.Close()
		if used, _ := a.Usage(validation.BudgetBufferedBytes); used != 0 {
			t.Fatalf("retained %d", used)
		}
	}
}
