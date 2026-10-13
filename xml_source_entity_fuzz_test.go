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

	"github.com/go-muse/go-musicxml/validation"
)

func FuzzSourceInternalDTDFrames(f *testing.F) {
	for _, source := range []string{
		"",
		`<!DOCTYPE r [<!ENTITY e "<x a='v'/>">]><r>&e;&e;</r>`,
		`<!DOCTYPE r [<!ENTITY e "&#38;e;">]><r>&e;</r>`,
		`<!DOCTYPE r [<!ENTITY e "<!-- &e; -->">]><r>&e;</r>`,
		`<!DOCTYPE r [<!ENTITY e '"&#xD;&#x9;'><!ENTITY b '&e;'>]><r a="&b;"/>`,
		`<!DOCTYPE r [<!ENTITY % p "<!ENTITY e 'v'>">%p;]><r>&e;</r>`,
		`<!DOCTYPE r [<!ENTITY e "<x">]><r>&e; /></r>`,
		`<!DOCTYPE r [<!ENTITY e "&amp;">]><r>&e;#60;</r>`,
		`<!DOCTYPE r [<!ATTLIST r a CDATA 'default' b NMTOKENS ' a  b '>]><r/>`,
		`<!DOCTYPE r [<!ATTLIST r a (one|two) #FIXED 'one'>]><r a='two'/>`,
		`<!DOCTYPE p:r [<!ATTLIST p:r xmlns:p CDATA 'urn:default'>]><p:r/>`,
		`<!DOCTYPE r [<!ENTITY % p '&#37;p;'>%p;]><r/>`,
		`<!DOCTYPE r [<!ENTITY e 'value'><!ATTLIST r a CDATA '&e;'>]><r a='explicit'/>`,
		`<!DOCTYPE r [<!NOTATION n SYSTEM 'unused'><!ENTITY e SYSTEM 'unused' NDATA n>]><r>&e;</r>`,
		`<!DOCTYPE r SYSTEM 'unread.dtd'><r a='explicit'/>`,
		`<!DOCTYPE r [<!ENTITY % p SYSTEM 'unread.ent'>%p;<!ATTLIST r a CDATA 'later'>]><r/>`,
		`<?xml version="1.0" standalone="yes"?><!DOCTYPE r [<!ENTITY % p "<!ENTITY e 'value'>">%p;]><r>&e;</r>`,
		`<?xml version="1.0" standalone="no"?><!DOCTYPE r [<!ENTITY % p "<!ENTITY e 'value'>">%p;]><r>&e;</r>`,
	} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		a, err := validation.NewBudgetAccount(validation.Budgets{MaxInputBytes: 1 << 16, MaxBufferedBytes: 1 << 20, MaxEvents: 1 << 15, MaxScalarBytes: 1 << 15, MaxDepth: 128})
		if err != nil {
			t.Fatal(err)
		}
		r, err := newSourceXMLTokenReader(strings.NewReader(source), sourceXMLConfig{Context: context.Background(), Account: a, InternalDTD: true})
		if err != nil {
			t.Fatal(err)
		}
		for err == nil {
			_, err = r.Token()
		}
		if invariant := sourceDTDFrameFuzzInvariant(source, r.Summary(), err, r.entity != nil, len(r.frames)); invariant != nil {
			t.Fatal(invariant)
		}
		// The physical cursor records an over-limit probe in its prefix hash,
		// but only admitted bytes occupy the finite input quota.
		inputUsed, usageErr := a.Usage(validation.BudgetInputBytes)
		if usageErr != nil || inputUsed != min(r.Summary().BytesRead, a.Limits().MaxInputBytes) {
			t.Fatalf("input quota=%d read=%d error=%v", inputUsed, r.Summary().BytesRead, usageErr)
		}
		r.Close()
		used, err := a.Usage(validation.BudgetBufferedBytes)
		if err != nil || used != 0 {
			t.Fatalf("retention=%d err=%v", used, err)
		}
	})
}

// The corpus reader is a strings.Reader, so arbitrary I/O failures are not
// admissible. Read-ahead belongs to BytesRead and the prefix digest, independently
// of the token cursor's BytesConsumed and terminal syntax classification.
func sourceDTDFrameFuzzInvariant(source string, summary sourceXMLSummary, terminal error, entityLive bool, elements int) error {
	var syntax *xml.SyntaxError
	var unsupported *sourceXMLUnsupportedError
	var resource *validation.ResourceError
	if terminal != io.EOF && !errors.Is(terminal, ErrEmptyDocument) && !errors.As(terminal, &syntax) && !errors.As(terminal, &unsupported) && !errors.As(terminal, &resource) {
		return fmt.Errorf("unexpected DTD terminal class: %T: %v", terminal, terminal)
	}
	if summary.BytesRead < 0 || summary.BytesRead > int64(len(source)) || summary.BytesConsumed < 0 || summary.BytesConsumed > summary.BytesRead {
		return fmt.Errorf("original-byte accounting: read=%d consumed=%d length=%d", summary.BytesRead, summary.BytesConsumed, len(source))
	}
	if summary.ReadPrefixSHA256 != sha256.Sum256([]byte(source[:int(summary.BytesRead)])) {
		return errors.New("original-prefix digest includes replacement bytes or omits read-ahead")
	}
	if terminal == io.EOF && (!summary.InputComplete || !summary.DigestComplete || summary.BytesRead != int64(len(source)) || entityLive || elements != 0) {
		return fmt.Errorf("EOF without complete physical input and closed frames: summary=%+v entity=%t elements=%d", summary, entityLive, elements)
	}
	return nil
}

func TestSourceDTDFrameFuzzInvariantRejectsCorruption(t *testing.T) {
	const source = "<r/>"
	good := sourceXMLSummary{BytesRead: 4, BytesConsumed: 4, ReadPrefixSHA256: sha256.Sum256([]byte(source)), InputComplete: true, DigestComplete: true}
	if err := sourceDTDFrameFuzzInvariant(source, good, io.EOF, false, 0); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		alter    func(*sourceXMLSummary)
		terminal error
		entity   bool
		elements int
	}{
		{"arbitrary-error", func(*sourceXMLSummary) {}, errors.New("injected unexpected failure"), false, 0},
		{"replacement-byte-count", func(s *sourceXMLSummary) { s.BytesRead++ }, io.EOF, false, 0},
		{"consumed-after-read", func(s *sourceXMLSummary) { s.BytesConsumed++ }, io.EOF, false, 0},
		{"wrong-digest", func(s *sourceXMLSummary) { s.ReadPrefixSHA256[0]++ }, io.EOF, false, 0},
		{"incomplete-eof", func(s *sourceXMLSummary) { s.InputComplete = false }, io.EOF, false, 0},
		{"live-entity-eof", func(*sourceXMLSummary) {}, io.EOF, true, 0},
		{"open-element-eof", func(*sourceXMLSummary) {}, io.EOF, false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := good
			tc.alter(&s)
			if sourceDTDFrameFuzzInvariant(source, s, tc.terminal, tc.entity, tc.elements) == nil {
				t.Fatal("corruption accepted")
			}
		})
	}
}
