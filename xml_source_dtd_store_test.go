package musicxml

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unsafe"

	"github.com/go-muse/go-musicxml/validation"
)

func dtdStoreAccount(t *testing.T, limits validation.Budgets) *validation.Account {
	t.Helper()
	a, err := validation.NewBudgetAccount(limits)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func dtdStoreParse(t *testing.T, text string, standalone sourceDTDStandalone) (*sourceDTDStore, sourceDTDInfo) {
	t.Helper()
	a := dtdStoreAccount(t, validation.Budgets{})
	s, err := newSourceDTDStore(context.Background(), a, standalone)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
		if n, _ := a.Usage(validation.BudgetBufferedBytes); n != 0 {
			t.Errorf("leaked %d bytes", n)
		}
	})
	info, err := parseSourceDTDWithStore(&sourceDTDTestInput{text: text}, s)
	if err != nil {
		t.Fatal(err)
	}
	return s, info
}

func TestSourceDTDStoreFirstBindingAndLexicalKeys(t *testing.T) {
	s, _ := dtdStoreParse(t, ` p:r [<!ATTLIST p:r p:a CDATA 'first'><!ATTLIST p:r p:a ID #REQUIRED b NMTOKENS #FIXED ' one  two '><!ATTLIST q:r p:a CDATA 'other'><!ENTITY e 'first'><!ENTITY e 'second'><!ENTITY % e 'parameter'><!NOTATION n PUBLIC 'image/gif'><!ENTITY image SYSTEM 'image.gif' NDATA n>]>`, sourceDTDStandaloneAbsent)
	a, ok := s.Attribute("p:r", "p:a")
	if !ok || a.RawValue != "first" || a.AttributeType != "CDATA" || !a.Binding || !a.ActuallyRead {
		t.Fatalf("first attribute: %+v, %v", a, ok)
	}
	b, ok := s.Attribute("p:r", "b")
	if !ok || b.AttributeType != "NMTOKENS" || b.DefaultKind != sourceDTDDefaultFixed || b.RawValue != " one  two " {
		t.Errorf("merged attribute: %+v, %v", b, ok)
	}
	if a, ok := s.Attribute("q:r", "p:a"); !ok || a.RawValue != "other" {
		t.Errorf("lexical other prefix: %+v %v", a, ok)
	}
	if _, ok := s.Attribute("r", "p:a"); ok {
		t.Error("expanded-name lookup was invented")
	}
	if e, ok := s.Entity("e", false); !ok || e.RawValue != "first" || e.Parameter {
		t.Errorf("general entity: %+v %v", e, ok)
	}
	if e, ok := s.Entity("e", true); !ok || e.RawValue != "parameter" || !e.Parameter {
		t.Errorf("parameter entity: %+v %v", e, ok)
	}
	if e, ok := s.Entity("image", false); !ok || !e.External || e.SystemID != "image.gif" || e.Notation != "n" {
		t.Errorf("unparsed entity: %+v %v", e, ok)
	}
	if n, ok := s.Notation("n"); !ok || n.PublicID != "image/gif" || n.SystemID != "" {
		t.Errorf("notation: %+v %v", n, ok)
	}
	decls := s.Declarations()
	if len(decls) != 9 {
		t.Fatalf("declarations: %d, want 9", len(decls))
	}
	if decls[1].Binding || decls[1].IgnoredReason != sourceDTDIgnoredDuplicate || !decls[1].ActuallyRead {
		t.Errorf("ignored duplicate: %+v", decls[1])
	}
	for i, d := range decls {
		if d.Origin.Sequence != int64(i+1) || d.Origin.Region != sourceDTDInternalSubset || d.Origin.ExternalMarkup {
			t.Errorf("origin %d: %+v", i, d.Origin)
		}
	}
}

func TestSourceDTDStoreParameterCutoffStandalone(t *testing.T) {
	for _, standalone := range []sourceDTDStandalone{sourceDTDStandaloneAbsent, sourceDTDStandaloneNo, sourceDTDStandaloneYes} {
		t.Run(string(standalone), func(t *testing.T) {
			s, info := dtdStoreParse(t, ` r SYSTEM 'unread.dtd' [<!ENTITY early 'kept'><!ATTLIST r early CDATA 'kept'>%missing;<!ENTITY late 'later'><!ATTLIST r late CDATA 'later'><!NOTATION n PUBLIC 'n'>]>`, standalone)
			if _, ok := s.Entity("early", false); !ok {
				t.Error("lost earlier entity")
			}
			if _, ok := s.Attribute("r", "early"); !ok {
				t.Error("lost earlier attribute")
			}
			wantLate := standalone == sourceDTDStandaloneYes
			if _, ok := s.Entity("late", false); ok != wantLate {
				t.Errorf("late entity binding = %v", ok)
			}
			if _, ok := s.Attribute("r", "late"); ok != wantLate {
				t.Errorf("late attribute binding = %v", ok)
			}
			state := s.State()
			if !state.GrammarComplete || state.DeclarationsComplete || !state.ExternalSubsetUnread || !state.ParameterEntityUnread || state.BindingCutoff != !wantLate || !info.HasParameterReferences {
				t.Errorf("state: %+v info: %+v", state, info)
			}
			if state.FirstUnreadParameter != "missing" {
				t.Errorf("unread identity = %q", state.FirstUnreadParameter)
			}
		})
	}
}

func TestSourceDTDStoreIgnoredDeclarationsStillChecked(t *testing.T) {
	for _, tail := range []string{`<!ENTITY e '&broken'>`, `<!ENTITY e '&#0;'>`, `<!ENTITY e '%p;'>`, `<!ATTLIST r a UNKNOWN #IMPLIED>`, `<!ATTLIST r a CDATA '<'>`} {
		for _, prefix := range []string{`<!ENTITY e 'valid'><!ATTLIST r a CDATA 'valid'>`, `%unread;`} {
			a := dtdStoreAccount(t, validation.Budgets{})
			s, err := newSourceDTDStore(context.Background(), a, sourceDTDStandaloneAbsent)
			if err != nil {
				t.Fatal(err)
			}
			_, err = parseSourceDTDWithStore(&sourceDTDTestInput{text: ` r [` + prefix + tail + `]>`}, s)
			var syntax *xml.SyntaxError
			if !errors.As(err, &syntax) {
				t.Errorf("%s %s: got %T %v, want syntax", prefix, tail, err, err)
			}
			if s.State().GrammarComplete {
				t.Error("failed grammar marked complete")
			}
			_ = s.Close()
			if n, _ := a.Usage(validation.BudgetBufferedBytes); n != 0 {
				t.Errorf("leaked %d bytes", n)
			}
		}
	}
}

func TestSourceDTDStoreRawReferencesAndValiditySeparation(t *testing.T) {
	s, _ := dtdStoreParse(t, ` r [<!ELEMENT different EMPTY><!ELEMENT different ANY><!ATTLIST r a ID 'duplicate' b ID 'duplicate' c (x|x) #FIXED 'other'><!ENTITY e 'literal &amp; &#x9; &#13; &later;'><!NOTATION n PUBLIC 'first'><!NOTATION n PUBLIC 'second'>]>`, sourceDTDStandaloneAbsent)
	e, _ := s.Entity("e", false)
	if e.RawValue != "literal &amp; &#x9; &#13; &later;" {
		t.Errorf("replacement prematurely expanded: %q", e.RawValue)
	}
	a, _ := s.Attribute("r", "c")
	if a.AttributeType != "ENUMERATION" || a.RawType != "(x|x)" {
		t.Errorf("enumeration: %+v", a)
	}
	if !s.State().WellFormednessIncomplete {
		t.Error("unresolved unused entity semantics claimed closed")
	}
}

func TestSourceDTDStorePreflightAndCleanup(t *testing.T) {
	for _, a := range []*validation.Account{nil, {}} {
		if s, err := newSourceDTDStore(context.Background(), a, sourceDTDStandaloneAbsent); err == nil || s != nil {
			t.Errorf("invalid account = %v %v", s, err)
		}
	}
	a := dtdStoreAccount(t, validation.Budgets{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if s, err := newSourceDTDStore(ctx, a, sourceDTDStandaloneAbsent); !errors.Is(err, context.Canceled) || s != nil {
		t.Errorf("cancelled constructor = %v %v", s, err)
	}
	s, err := newSourceDTDStore(context.Background(), a, sourceDTDStandaloneAbsent)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	in := &sourceDTDTestInput{text: ` r>`}
	if _, err := parseSourceDTDWithStore(in, s); err == nil || in.offset != 0 {
		t.Errorf("closed store read: %v offset %d", err, in.offset)
	}
	if n, _ := a.Usage(validation.BudgetBufferedBytes); n != 0 {
		t.Errorf("leaked %d bytes", n)
	}
}

func TestSourceDTDStoreScalarLimit(t *testing.T) {
	for _, n := range []int64{5, 4} {
		a := dtdStoreAccount(t, validation.Budgets{MaxScalarBytes: n})
		s, err := newSourceDTDStore(context.Background(), a, sourceDTDStandaloneAbsent)
		if err != nil {
			t.Fatal(err)
		}
		_, err = parseSourceDTDWithStore(&sourceDTDTestInput{text: ` r [<!ENTITY e '` + strings.Repeat("x", 5) + `'>]>`}, s)
		if (err == nil) != (n == 5) {
			t.Errorf("scalar %d: %v", n, err)
		}
		_ = s.Close()
		if used, _ := a.Usage(validation.BudgetBufferedBytes); used != 0 {
			t.Errorf("leaked %d", used)
		}
	}
}

func TestSourceDTDStoreSpecificClosureAndReplacement(t *testing.T) {
	for _, tc := range []struct {
		input                                   string
		entity, defaults, predefined, parameter bool
	}{
		{input: ` r [<!ENTITY title 'Bounded title'><!ATTLIST r a CDATA 'literal' b CDATA '&#9;'>]>`},
		{input: ` r [<!ENTITY e '&later;'>]>`, entity: true},
		{input: ` r [<!ENTITY e '&#38;later;'>]>`, entity: true},
		{input: ` r [<!ENTITY e 'safe'><!ATTLIST r a CDATA '&e;'>]>`, defaults: true},
		{input: ` r [<!ENTITY amp 'wrong'>]>`, predefined: true},
		{input: ` r [<!ENTITY % p '<!ELEMENT r EMPTY>'>]>`, parameter: true},
	} {
		s, info := dtdStoreParse(t, tc.input, sourceDTDStandaloneAbsent)
		state := s.State()
		if state.EntityWellFormednessIncomplete != tc.entity || state.DefaultWellFormednessIncomplete != tc.defaults || state.PredefinedEntityWellFormednessIncomplete != tc.predefined || state.ParameterEntityWellFormednessIncomplete != tc.parameter {
			t.Errorf("%s: %+v", tc.input, state)
		}
		if info.WellFormednessIncomplete != (tc.entity || tc.defaults || tc.predefined || tc.parameter) {
			t.Errorf("%s: info %+v", tc.input, info)
		}
	}
	s, _ := dtdStoreParse(t, ` r [<!ENTITY e 'a&#x9;&#13;&amp;&#38;later;'>]>`, sourceDTDStandaloneAbsent)
	e, _ := s.Entity("e", false)
	if e.Replacement != "a\t\r&amp;&later;" || e.RawValue != "a&#x9;&#13;&amp;&#38;later;" || !e.HasGeneralReferences {
		t.Errorf("stage-one replacement: %+v", e)
	}
}

func TestSourceDTDStoreExactRetentionAndTransientCopies(t *testing.T) {
	base := int64(unsafe.Sizeof(sourceDTDStore{})) + 2*sourceDTDStoreMapBytes
	final := base + 4 + 4*int64(unsafe.Sizeof((*sourceDTDDeclaration)(nil))) + int64(unsafe.Sizeof(sourceDTDDeclaration{})) + sourceDTDStoreIndexBytes
	for _, tc := range []struct {
		name, value string
		required    int64
	}{
		{name: "record and index", value: "x", required: final},
		{name: "transient raw and replacement copies", value: strings.Repeat("x", 4097), required: base + 2 + 2*8192 + 2*4097},
	} {
		for _, delta := range []int64{0, -1} {
			t.Run(tc.name+fmt.Sprint(delta), func(t *testing.T) {
				const foreign = int64(17)
				a := dtdStoreAccount(t, validation.Budgets{MaxBufferedBytes: tc.required + foreign + delta})
				if err := a.Charge(validation.BudgetBufferedBytes, foreign); err != nil {
					t.Fatal(err)
				}
				s, err := newSourceDTDStore(context.Background(), a, sourceDTDStandaloneAbsent)
				if err != nil {
					t.Fatal(err)
				}
				_, err = parseSourceDTDWithStore(&sourceDTDTestInput{text: ` r [<!ENTITY e '` + tc.value + `'>]>`}, s)
				if delta == 0 && err != nil {
					t.Fatalf("exact limit refused: %v", err)
				}
				var resource *validation.ResourceError
				if delta == -1 && (!errors.As(err, &resource) || resource.Kind != validation.BudgetBufferedBytes) {
					t.Errorf("one under: got %T %v", err, err)
				}
				if delta == 0 && tc.name == "record and index" {
					if n, _ := a.Usage(validation.BudgetBufferedBytes); n != final+foreign {
						t.Errorf("retained %d, want %d", n, final+foreign)
					}
				}
				if err := s.Close(); err != nil {
					t.Error(err)
				}
				if n, _ := a.Usage(validation.BudgetBufferedBytes); n != foreign {
					t.Errorf("closed store changed foreign ownership: %d", n)
				}
			})
		}
	}
}

type sourceDTDStoreCancelInput struct {
	sourceDTDTestInput
	cancel   context.CancelFunc
	cancelAt int
}

func (in *sourceDTDStoreCancelInput) Read() (rune, error) {
	r, err := in.sourceDTDTestInput.Read()
	if in.offset >= in.cancelAt {
		in.cancel()
	}
	return r, err
}
func (in *sourceDTDStoreCancelInput) DTDPosition() sourceXMLPosition {
	return sourceXMLPosition{ByteOffset: int64(in.offset), Line: 1, Column: in.offset + 1}
}
func TestSourceDTDStoreCancellationAndOrigins(t *testing.T) {
	text := ` r [<!ENTITY e 'numeric &#65; and &amp;'><!ATTLIST r a CDATA 'default'>]>`
	for at := 1; at <= len(text); at++ {
		a := dtdStoreAccount(t, validation.Budgets{})
		ctx, cancel := context.WithCancel(context.Background())
		s, err := newSourceDTDStore(ctx, a, sourceDTDStandaloneAbsent)
		if err != nil {
			t.Fatal(err)
		}
		in := &sourceDTDStoreCancelInput{sourceDTDTestInput: sourceDTDTestInput{text: text}, cancel: cancel, cancelAt: at}
		_, err = parseSourceDTDWithStore(in, s)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("cancel at %d: %v", at, err)
		}
		_ = s.Close()
		cancel()
		if n, _ := a.Usage(validation.BudgetBufferedBytes); n != 0 {
			t.Errorf("cancel at %d leaked %d", at, n)
		}
	}
	a := dtdStoreAccount(t, validation.Budgets{})
	s, err := newSourceDTDStore(context.Background(), a, sourceDTDStandaloneAbsent)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	in := &sourceDTDStoreCancelInput{sourceDTDTestInput: sourceDTDTestInput{text: text}, cancel: func() {}, cancelAt: len(text) + 1}
	if _, err := parseSourceDTDWithStore(in, s); err != nil {
		t.Fatal(err)
	}
	e, _ := s.Entity("e", false)
	if !e.Origin.SpanAvailable || text[e.Origin.Span.Start.ByteOffset:e.Origin.Span.End.ByteOffset] != `<!ENTITY e 'numeric &#65; and &amp;'>` {
		t.Errorf("entity origin: %+v", e.Origin)
	}
	attrs := s.Attributes("r")
	if len(attrs) != 1 || !attrs[0].Origin.SpanAvailable || text[attrs[0].Origin.Span.Start.ByteOffset:attrs[0].Origin.Span.End.ByteOffset] != `a CDATA 'default'` {
		t.Errorf("attribute definition origin: %+v", attrs)
	}
}

func TestSourceDTDStoreDirectDefaultReferenceOrder(t *testing.T) {
	for _, text := range []string{
		` r [<!ATTLIST r a CDATA '&later;'><!ENTITY later 'x'>]>`,
		` r SYSTEM 'unknown.dtd' [<!ATTLIST r a CDATA '&later;'><!ENTITY later 'x'>]>`,
		` r [<!ATTLIST r a CDATA 'first'><!ATTLIST r a CDATA '&later;'><!ENTITY later 'x'>]>`,
		` r [<!ENTITY ext SYSTEM 'never-fetch.xml'><!ATTLIST r a CDATA '&ext;'>]>`,
		` r [<!ATTLIST unused a CDATA '&missing;'>]>`,
	} {
		a := dtdStoreAccount(t, validation.Budgets{})
		s, err := newSourceDTDStore(context.Background(), a, sourceDTDStandaloneAbsent)
		if err != nil {
			t.Fatal(err)
		}
		_, err = parseSourceDTDWithStore(&sourceDTDTestInput{text: text}, s)
		var syntax *xml.SyntaxError
		if !errors.As(err, &syntax) {
			t.Errorf("%s: got %T %v, want syntax", text, err, err)
		}
		_ = s.Close()
		if n, _ := a.Usage(validation.BudgetBufferedBytes); n != 0 {
			t.Errorf("leaked %d", n)
		}
	}
	for _, text := range []string{
		` r [<!ENTITY e 'x'><!ATTLIST r a CDATA '&e; &amp; &#9;'>]>`,
		` r [<!ENTITY e '&later;'><!ATTLIST r a CDATA '&e;'><!ENTITY later 'x'>]>`,
		` r SYSTEM 'unknown.dtd' [<!ATTLIST r a CDATA '&unknown;'>]>`,
		` r [%unread;<!ATTLIST r a CDATA '&unknown;'>]>`,
	} {
		dtdStoreParse(t, text, sourceDTDStandaloneAbsent)
	}
}

func TestSourceDTDStoreDefaultClosureDischargesOnlyVerifiedCell(t *testing.T) {
	s, _ := dtdStoreParse(t, ` r [<!ENTITY e '&later;'><!ENTITY later 'x'><!ENTITY amp 'wrong'><!ENTITY % p 'unused'><!ATTLIST r a CDATA '&e;'>]>`, sourceDTDStandaloneAbsent)
	if err := s.completeDefaultWellFormedness(); err != nil {
		t.Fatal(err)
	}
	state := s.State()
	if state.DefaultWellFormednessIncomplete || !state.EntityWellFormednessIncomplete || !state.PredefinedEntityWellFormednessIncomplete || !state.ParameterEntityWellFormednessIncomplete || !state.WellFormednessIncomplete {
		t.Errorf("unrelated closure erased: %+v", state)
	}
	cutoff, _ := dtdStoreParse(t, ` r [%unread;<!ATTLIST r a CDATA '&unknown;'>]>`, sourceDTDStandaloneAbsent)
	if err := cutoff.completeDefaultWellFormedness(); err != nil {
		t.Fatal(err)
	}
	if !cutoff.State().DefaultWellFormednessIncomplete {
		t.Error("unprocessed cutoff default claimed checked")
	}
	a := dtdStoreAccount(t, validation.Budgets{})
	fresh, err := newSourceDTDStore(context.Background(), a, sourceDTDStandaloneAbsent)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	if err := fresh.completeDefaultWellFormedness(); err == nil {
		t.Error("unparsed store claimed checked")
	}
}
