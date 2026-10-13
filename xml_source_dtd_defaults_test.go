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

func TestSourceDTDUnusedDefaultWellFormedness(t *testing.T) {
	for _, test := range []struct{ name, decls string }{
		{"external", `<!ENTITY ext SYSTEM 'never'><!ENTITY alias '&ext;'>`},
		{"less-than", `<!ENTITY alias '&#60;'>`},
		{"recursion", `<!ENTITY alias '&alias;'>`},
	} {
		for _, suffix := range []string{`<r/>`, `<r a='explicit'/>`, `<other/>`} {
			t.Run(test.name+suffix, func(t *testing.T) {
				source := `<!DOCTYPE r [` + test.decls + `<!ATTLIST r a CDATA '&alias;'>]>` + suffix
				_, _, _, err := sourceEntityScan(t, source)
				var syntax *xml.SyntaxError
				if !errors.As(err, &syntax) {
					t.Fatalf("unused default: %v", err)
				}
			})
		}
	}
}

func TestSourceDTDDefaultWFCClosureIndependentOfEffectiveAttributes(t *testing.T) {
	for _, source := range []string{
		`<!DOCTYPE r [<!ATTLIST r a CDATA 'value'>]><r/>`,
		`<!DOCTYPE r [<!ENTITY e 'value'>]><r>&e;</r>`,
		`<!DOCTYPE r [<!ENTITY external SYSTEM 'never'>]><r/>`,
	} {
		_, _, summary, err := sourceEntityScan(t, source)
		if err != nil {
			t.Fatal(err)
		}
		if summary.DTD.Unsupported {
			t.Fatalf("closed effective attributes marked unsupported: %+v", summary.DTD)
		}
	}
}

func TestSourceDTDParseFailureKeepsFirstCause(t *testing.T) {
	boom := errors.New("original DTD read failure")
	for _, readFailure := range []bool{false, true} {
		account, _ := validation.NewBudgetAccount(validation.Budgets{})
		calls := 0
		var held int64
		input := sourceEntityReadFunc(func(p []byte) (int, error) {
			calls++
			if calls == 1 {
				return copy(p, "<!DOCTYPE r "), nil
			}
			if calls == 2 {
				used, _ := account.Usage(validation.BudgetBufferedBytes)
				held = account.Limits().MaxBufferedBytes - used
				if err := account.Charge(validation.BudgetBufferedBytes, held); err != nil {
					t.Fatal(err)
				}
				if readFailure {
					return 0, boom
				}
				return copy(p, "+"), nil
			}
			return 0, io.EOF
		})
		r, err := newSourceXMLTokenReader(input, sourceXMLConfig{Context: context.Background(), Account: account, InternalDTD: true})
		if err != nil {
			t.Fatal(err)
		}
		_, err = r.Token()
		if readFailure {
			if !errors.Is(err, boom) {
				t.Errorf("read cause replaced: %v", err)
			}
		} else {
			var syntax *xml.SyntaxError
			if !errors.As(err, &syntax) {
				t.Errorf("syntax cause replaced: %v", err)
			}
		}
		r.Close()
		if err = account.Release(validation.BudgetBufferedBytes, held); err != nil {
			t.Fatal(err)
		}
		used, _ := account.Usage(validation.BudgetBufferedBytes)
		if used != 0 {
			t.Fatalf("retained %d", used)
		}
	}
}

func TestSourceDTDDefaultValidationDoesNotRetainDiscardedValues(t *testing.T) {
	var source strings.Builder
	source.WriteString("<!DOCTYPE r [")
	for i := 0; i < 64; i++ {
		source.WriteString("<!ATTLIST r a CDATA '")
		source.WriteString(strings.Repeat("x", 256))
		source.WriteString("'>")
	}
	source.WriteString("]><r/>")
	account, _ := validation.NewBudgetAccount(validation.Budgets{})
	r, err := newSourceXMLTokenReader(strings.NewReader(source.String()), sourceXMLConfig{Context: context.Background(), Account: account, InternalDTD: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Token(); err != nil {
		t.Fatal(err)
	}
	// No per-default output or expansion frame should survive validation. Current
	// directive metadata and its small spelling are the only token-owned values.
	if r.tokenOwned > 1024 || r.entity != nil {
		t.Errorf("discarded default retention=%d frame=%v", r.tokenOwned, r.entity != nil)
	}
	r.Close()
	used, _ := account.Usage(validation.BudgetBufferedBytes)
	if used != 0 {
		t.Fatalf("retained %d", used)
	}
}

func TestSourceDTDReadParameterMissingGeneralIsValidity(t *testing.T) {
	for _, standalone := range []string{"", "no", "yes"} {
		declaration := ""
		if standalone != "" {
			declaration = `<?xml version="1.0" standalone="` + standalone + `"?>`
		}
		source := declaration + `<!DOCTYPE r [<!ENTITY % p ''>%p;]><r>&missing;</r>`
		_, _, _, err := sourceEntityScan(t, source)
		if standalone == "yes" {
			var syntax *xml.SyntaxError
			if !errors.As(err, &syntax) {
				t.Fatalf("standalone yes: %v", err)
			}
		} else {
			var unsupported *sourceXMLUnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("standalone %q: want validity unavailable, got %v", standalone, err)
			}
		}
	}
	_, _, _, err := sourceEntityScan(t, `<!DOCTYPE r []><r>&missing;</r>`)
	var syntax *xml.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("no PE contrast: %v", err)
	}
}

func TestSourceDTDStandaloneParameterDefaultOrigin(t *testing.T) {
	const declarations = `<!ENTITY % p "<!ENTITY e 'value'><!ATTLIST r a CDATA '&e;'>">%p;`
	good := `<?xml version="1.0" standalone="yes"?><!DOCTYPE r [` + declarations + `]><r/>`
	if _, _, _, err := sourceEntityScan(t, good); err != nil {
		t.Fatalf("reference inside PE default: %v", err)
	}
	bad := `<?xml version="1.0" standalone="yes"?><!DOCTYPE r [` + declarations + `]><r>&e;</r>`
	_, _, _, err := sourceEntityScan(t, bad)
	var syntax *xml.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("physical reference to PE declaration: %v", err)
	}
	nested := `<?xml version="1.0" standalone="yes"?><!DOCTYPE r [<!ENTITY % p "<!ENTITY e 'value'><!ENTITY alias '&e;'><!ATTLIST r a CDATA '&alias;'>">%p;]><r/>`
	if _, _, _, err := sourceEntityScan(t, nested); err != nil {
		t.Fatalf("nested reference inside PE default: %v", err)
	}
}
