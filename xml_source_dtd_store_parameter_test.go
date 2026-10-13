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

func dtdStoreParameterReader(t *testing.T, text string) *sourceXMLTokenReader {
	t.Helper()
	a := dtdStoreAccount(t, validation.Budgets{})
	r, err := newSourceXMLTokenReader(strings.NewReader(text), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = r.Close()
		if n, _ := a.Usage(validation.BudgetBufferedBytes); n != 0 {
			t.Errorf("leaked %d", n)
		}
	})
	return r
}
func TestSourceDTDStoreParameterWholeDeclarations(t *testing.T) {
	r := dtdStoreParameterReader(t, `<!DOCTYPE r [<!ENTITY % p "<!ENTITY title 'Bounded title'><!ATTLIST r mode CDATA 'major'>">%p;]><r>&title;</r>`)
	if _, err := r.Token(); err != nil {
		t.Fatal(err)
	}
	entity, ok := r.dtdStore.Entity("title", false)
	if !ok || entity.RawValue != "Bounded title" {
		t.Fatalf("missing PE declaration: %+v %v", entity, ok)
	}
	if entity.Origin.Region != sourceDTDParameterEntityRegion || !entity.Origin.ExternalMarkup || entity.Origin.SpanAvailable {
		t.Errorf("generated declaration origin: %+v", entity.Origin)
	}
	attr, ok := r.dtdStore.Attribute("r", "mode")
	if !ok || attr.RawValue != "major" || !attr.Origin.ExternalMarkup {
		t.Errorf("generated attribute: %+v %v", attr, ok)
	}
	state := r.dtdStore.State()
	if state.ParameterEntityUnread || state.BindingCutoff || !state.DeclarationsComplete || state.ParameterEntityWellFormednessIncomplete {
		t.Errorf("read PE state: %+v", state)
	}
	var value strings.Builder
	for {
		tok, err := r.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if text, ok := tok.(xml.CharData); ok {
			value.Write(text)
		}
	}
	if value.String() != "Bounded title" {
		t.Errorf("generated GE value: %q", value.String())
	}
}

func TestSourceDTDStoreParameterNestedAndGeneratedDeclaration(t *testing.T) {
	for _, text := range []string{
		`<!DOCTYPE r [<!ENTITY % leaf "<!ENTITY value 'ok'>"><!ENTITY % outer '&#37;leaf;'>%outer;]><r/>`,
		`<!DOCTYPE r [<!ENTITY % maker "<!ENTITY &#37; child '<!ENTITY value &#34;ok&#34;>'>">%maker;%child;]><r/>`,
	} {
		r := dtdStoreParameterReader(t, text)
		if _, err := r.Token(); err != nil {
			t.Fatal(err)
		}
		value, ok := r.dtdStore.Entity("value", false)
		if !ok || value.RawValue != "ok" || !value.Origin.ExternalMarkup || value.Origin.SpanAvailable {
			t.Fatalf("nested value: %+v %v", value, ok)
		}
		if !value.Origin.AncestryComplete || len(value.Origin.Ancestry) == 0 {
			t.Errorf("missing ancestry: %+v", value.Origin)
		}
	}
}

func TestSourceDTDStoreParameterCannotBorrowBoundaries(t *testing.T) {
	for _, body := range []string{`<!ENTITY e 'x'`, `<!--missing end`, `<?pi missing end`, `<!ELEMENT r (r`, `%missing`, `]>`} {
		text := `<!DOCTYPE r [<!ENTITY % p "` + body + `">%p;]><r/>`
		r := dtdStoreParameterReader(t, text)
		_, err := r.Token()
		var syntax *xml.SyntaxError
		if !errors.As(err, &syntax) {
			t.Errorf("body %q: got %T %v, want syntax", body, err, err)
		}
	}
}

func TestSourceDTDStoreParameterUnreadBarrierSurvivesExpansion(t *testing.T) {
	for _, standalone := range []string{"no", "yes"} {
		r := dtdStoreParameterReader(t, `<?xml version="1.0" standalone="`+standalone+`"?><!DOCTYPE r [<!ENTITY % p "<!ENTITY earlier 'kept'>&#37;unread;<!ENTITY later 'after'>">%p;<!ENTITY final 'tail'>]><r/>`)
		if _, err := r.Token(); err != nil {
			t.Fatal(err)
		}
		if _, err := r.Token(); err != nil {
			t.Fatal(err)
		}
		if _, ok := r.dtdStore.Entity("earlier", false); !ok {
			t.Error("earlier generated binding lost")
		}
		for _, name := range []string{"later", "final"} {
			if _, ok := r.dtdStore.Entity(name, false); ok != (standalone == "yes") {
				t.Errorf("standalone %s entity %s binding=%v", standalone, name, ok)
			}
		}
		state := r.dtdStore.State()
		if !state.ParameterEntityUnread || state.DeclarationsComplete || state.BindingCutoff != (standalone != "yes") {
			t.Errorf("barrier state: %+v", state)
		}
	}
}

func TestSourceDTDStoreParameterRecursionAndUnusedResidual(t *testing.T) {
	for _, text := range []string{
		`<!DOCTYPE r [<!ENTITY % a '&#37;a;'>%a;]><r/>`,
		`<!DOCTYPE r [<!ENTITY % a '&#37;b;'><!ENTITY % b '&#37;a;'>%a;]><r/>`,
	} {
		r := dtdStoreParameterReader(t, text)
		_, err := r.Token()
		var syntax *xml.SyntaxError
		if !errors.As(err, &syntax) {
			t.Errorf("got %T %v, want reached recursion syntax", err, err)
		}
	}
	r := dtdStoreParameterReader(t, `<!DOCTYPE r [<!ENTITY % unused '&#37;unused;'>]><r/>`)
	if _, err := r.Token(); err != nil {
		t.Fatal(err)
	}
	if !r.dtdStore.State().ParameterEntityWellFormednessIncomplete {
		t.Error("unused recursion claimed closed")
	}
}

func TestSourceDTDStoreParameterAncestrySurvivesTokenRelease(t *testing.T) {
	r := dtdStoreParameterReader(t, `<!DOCTYPE r [<!ENTITY % maker "<!ENTITY &#37; child '<!ENTITY value &#34;ok&#34;>'>">%maker;%child;]><r/>`)
	if _, err := r.Token(); err != nil {
		t.Fatal(err)
	}
	value, _ := r.dtdStore.Entity("value", false)
	if len(value.Origin.Ancestry) != 1 || value.Origin.Ancestry[0].Entity != "child" {
		t.Fatalf("active use ancestry: %+v", value.Origin)
	}
	child := value.Origin.Ancestry[0].DeclarationRecord
	if child == nil || len(child.Origin.Ancestry) != 1 || child.Origin.Ancestry[0].Entity != "maker" {
		t.Fatalf("earlier declaration ancestry: %+v", child)
	}
	if !child.Origin.Ancestry[0].DeclarationAvailable || !child.Origin.Ancestry[0].UseAvailable || value.Origin.Ancestry[0].DeclarationAvailable || !value.Origin.Ancestry[0].UseAvailable {
		t.Errorf("availability: child=%+v value=%+v", child.Origin, value.Origin)
	}
	// Advancing frees the DOCTYPE token's temporary ancestry snapshots. Retained
	// records keep separately charged slices and the original declared identities.
	if _, err := r.Token(); err != nil {
		t.Fatal(err)
	}
	if value.Origin.Ancestry[0].DeclarationRecord != child || child.Origin.Ancestry[0].Entity != "maker" {
		t.Error("ancestry lost when token snapshots released")
	}
}

func TestSourceDTDStoreDefaultDeclarationDomainWithStandalone(t *testing.T) {
	for _, standalone := range []string{"yes", "no"} {
		r := dtdStoreParameterReader(t, `<?xml version="1.0" standalone="`+standalone+`"?><!DOCTYPE r [<!ENTITY % p "<!ENTITY e 'known'>">%p;<!ATTLIST r a CDATA '&e;'>]><r/>`)
		if _, err := r.Token(); err != nil {
			t.Fatal(err)
		}
		_, err := r.Token()
		var syntax *xml.SyntaxError
		if standalone == "yes" && !errors.As(err, &syntax) {
			t.Errorf("standalone=yes default accepted PE-only declaration: %v", err)
		}
		if standalone == "no" && err != nil {
			t.Error(err)
		}
	}
	// Entity Declared excludes references inside a parameter entity. Preserve an
	// unresolved declaration obligation rather than inventing a WFC violation.
	r := dtdStoreParameterReader(t, `<?xml version="1.0" standalone="yes"?><!DOCTYPE r [<!ENTITY % p "<!ATTLIST r a CDATA '&missing;'>">%p;]><r/>`)
	if _, err := r.Token(); err != nil {
		t.Fatal(err)
	}
	_, err := r.Token()
	var unsupported *sourceXMLUnsupportedError
	var syntax *xml.SyntaxError
	if !errors.As(err, &unsupported) || unsupported.Reason != "dtd-entity-declared-validity-unassessed" || errors.As(err, &syntax) {
		t.Errorf("PE-local unavailable default requires capability evidence, not a WFC violation: %v", err)
	}
}

type sourceDTDStoreCheckContext struct {
	context.Context
	cancel        context.CancelFunc
	checks, after int
}

func (c *sourceDTDStoreCheckContext) Err() error {
	c.checks++
	if c.after > 0 && c.checks >= c.after {
		c.cancel()
	}
	return c.Context.Err()
}
func TestSourceDTDStoreParameterCancellationReleasesOwners(t *testing.T) {
	text := `<!DOCTYPE r [<!ENTITY % leaf "<!ENTITY value 'ok'>"><!ENTITY % middle '&#37;leaf;'><!ENTITY % outer '&#37;middle;'>%outer;]><r>&value;</r>`
	run := func(after int) (int, error) {
		a := dtdStoreAccount(t, validation.Budgets{})
		const foreign = int64(19)
		if err := a.Charge(validation.BudgetBufferedBytes, foreign); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		counted := &sourceDTDStoreCheckContext{Context: ctx, cancel: cancel, after: after}
		r, err := newSourceXMLTokenReader(strings.NewReader(text), sourceXMLConfig{Context: counted, Account: a, InternalDTD: true})
		if err == nil {
			for {
				_, err = r.Token()
				if err != nil {
					break
				}
			}
			if err == io.EOF {
				err = nil
			}
			_ = r.Close()
		}
		cancel()
		if n, _ := a.Usage(validation.BudgetBufferedBytes); n != foreign {
			t.Errorf("cancel check%d changed foreign ownership: %d", after, n)
		}
		return counted.checks, err
	}
	total, err := run(0)
	if err != nil {
		t.Fatal(err)
	}
	for at := 1; at <= total; at++ {
		if _, err := run(at); !errors.Is(err, context.Canceled) {
			t.Errorf("cancel check %d/%d: %v", at, total, err)
		}
	}
}
