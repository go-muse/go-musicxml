package musicxml

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/go-muse/go-musicxml/validation"
	"io"
	"strings"
	"testing"
)

func TestDTDAuditParameterRecognitionAndFirstBinding(t *testing.T) {
	for _, tc := range []struct{ name, subset, want string }{
		{"literal-pi", `<!ENTITY % p "<?pi &#37;p; &missing;?>">%p;`, ""},
		{"literal-comment", `<!ENTITY % p "<!-- &#37;p; &missing; -->">%p;`, ""},
		{"first-before-pe", `<!ENTITY value 'first'><!ENTITY % p "<!ENTITY value 'second'>">%p;`, "first"},
		{"first-in-pe", `<!ENTITY % p "<!ENTITY value 'first'>">%p;<!ENTITY value 'second'>`, "first"},
		{"repeat-pe", `<!ENTITY % p "<!ENTITY value 'first'>">%p;%p;`, "first"},
		{"numeric-percent-reference", `<!ENTITY % leaf "<!ENTITY value 'ok'>"><!ENTITY % p '&#37;leaf;'>%p;`, "ok"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := dtdStoreParameterReader(t, `<!DOCTYPE r [`+tc.subset+`]><r/>`)
			if _, err := r.Token(); err != nil {
				t.Fatal(err)
			}
			if tc.want != "" {
				d, ok := r.dtdStore.Entity("value", false)
				if !ok || d.Replacement != tc.want {
					t.Fatalf("binding %+v", d)
				}
			}
			s := r.dtdStore.State()
			if !s.DeclarationsComplete || s.ParameterEntityUnread || !s.ParameterEntityReferences {
				t.Fatalf("state %+v", s)
			}
		})
	}
}

func TestDTDAuditParameterFrameDepth(t *testing.T) {
	for _, depth := range []int{64, 65} {
		var b strings.Builder
		b.WriteString(`<!DOCTYPE r [`)
		for n := depth - 1; n >= 0; n-- {
			value := ""
			if n < depth-1 {
				value = fmt.Sprintf("&#37;p%d;", n+1)
			}
			fmt.Fprintf(&b, `<!ENTITY %% p%d "%s">`, n, value)
		}
		b.WriteString(`%p0;]><r/>`)
		_, _, _, err := sourceEntityScan(t, b.String())
		if depth == 64 && err != nil {
			t.Fatal(err)
		}
		var resource *validation.ResourceError
		if depth == 65 && (!errors.As(err, &resource) || resource.Kind != validation.BudgetDepth) {
			t.Fatalf("depth65=%v", err)
		}
	}
}

func TestDTDAuditParameterCannotSupplyPartialDeclaration(t *testing.T) {
	for _, body := range []string{`<!ENTITY`, `<!ENTITY e`, `<!ENTITY e 'x'`, `<!ATTLIST r a CDATA 'x'`, `<!--x--`, `<?pi x?`, `&#37;later`, `]`, `>`} {
		source := `<!DOCTYPE r [<!ENTITY % p "` + body + `">%p;>]><r/>`
		_, _, _, err := sourceEntityScan(t, source)
		var syntax *xml.SyntaxError
		if !errors.As(err, &syntax) {
			t.Errorf("body=%q err=%v", body, err)
		}
	}
}

func TestDTDAuditParameterBudgetOwnership(t *testing.T) {
	source := `<!DOCTYPE r [<!ENTITY % leaf "<!ENTITY value 'text'><!ATTLIST r a CDATA 'v'>"><!ENTITY % middle '&#37;leaf;'><!ENTITY % top '&#37;middle;'>%top;%top;]><r>&value;</r>`
	for _, kind := range []validation.BudgetKind{validation.BudgetEvents, validation.BudgetBufferedBytes} {
		max, step := int64(500), int64(1)
		if kind == validation.BudgetBufferedBytes {
			max, step = 40000, 97
		}
		successes := 0
		for n := int64(1); n <= max; n += step {
			limits := validation.Budgets{}
			if kind == validation.BudgetEvents {
				limits.MaxEvents = n
			} else {
				limits.MaxBufferedBytes = n
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
			if err == io.EOF {
				successes++
			} else if !errors.As(err, &resource) {
				t.Fatalf("kind %s at %d: %v", kind, n, err)
			}
			if live, _ := a.Usage(validation.BudgetBufferedBytes); live != 0 {
				t.Fatalf("kind %s at %d retained %d", kind, n, live)
			}
		}
		if successes == 0 {
			t.Fatalf("sweep %s never reached success", kind)
		}
	}
}

func TestDTDAuditStandaloneDirectAliasThroughParameterDefault(t *testing.T) {
	source := `<?xml version="1.0" standalone="yes"?><!DOCTYPE r [<!ENTITY alias '&e;'><!ENTITY % p "<!ENTITY e 'value'><!ATTLIST r a CDATA '&alias;'>">%p;]><r/>`
	_, _, _, err := sourceEntityScan(t, source)
	var syntax *xml.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("reference in directly declared alias must retain its own origin: got %v", err)
	}
}

func TestDTDStandaloneReferenceOriginDoesNotFollowCaller(t *testing.T) {
	for _, tc := range []struct {
		name, subset string
		yesMalformed bool
	}{
		{"direct-alias", `<!ENTITY alias '&e;'><!ENTITY % p "<!ENTITY e 'value'><!ATTLIST r a CDATA '&alias;'>">%p;`, true},
		{"numeric-direct-alias", `<!ENTITY alias '&#38;e;'><!ENTITY % p "<!ENTITY e 'value'><!ATTLIST r a CDATA '&alias;'>">%p;`, true},
		{"nested-direct-alias", `<!ENTITY alias '&middle;'><!ENTITY middle '&e;'><!ENTITY % p "<!ENTITY e 'value'><!ATTLIST r a CDATA '&alias;'>">%p;`, true},
		{"pe-alias", `<!ENTITY % p "<!ENTITY e 'value'><!ENTITY alias '&e;'><!ATTLIST r a CDATA '&alias;'>">%p;`, false},
		{"pe-alias-through-direct", `<!ENTITY middle '&e;'><!ENTITY % p "<!ENTITY e 'value'><!ENTITY alias '&middle;'><!ATTLIST r a CDATA '&alias;'>">%p;`, true},
		{"direct-target", `<!ENTITY e 'value'><!ENTITY alias '&e;'><!ENTITY % p "<!ATTLIST r a CDATA '&alias;'>">%p;`, false},
	} {
		for _, standalone := range []string{"yes", "no"} {
			t.Run(tc.name+"/"+standalone, func(t *testing.T) {
				input := `<?xml version="1.0" standalone="` + standalone + `"?><!DOCTYPE r [` + tc.subset + `]><r/>`
				_, _, _, err := sourceEntityScan(t, input)
				var syntax *xml.SyntaxError
				if standalone == "yes" && tc.yesMalformed {
					if !errors.As(err, &syntax) {
						t.Fatalf("direct declaration origin lost: %v", err)
					}
				} else if err != nil {
					t.Fatalf("valid reference origin rejected: %v", err)
				}
			})
		}
	}
}
