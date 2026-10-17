package musicxml

import (
	"encoding/xml"
	"errors"

	"github.com/go-muse/go-musicxml/validation"
)

// Each read default is an attribute-literal context even if no instance uses it.
// Do not turn an ignored binding into permission to skip its independent WFCs.
// Section 5.1 cutoff declarations remain unprocessed, preserving uncertainty.
func (r *sourceXMLTokenReader) validateDTDDefaults() error {
	metadata := r.metadata
	defer func() { r.metadata = metadata }()
	for _, declaration := range r.dtdStore.Declarations() {
		if err := r.cursor.contextErr(); err != nil {
			return err
		}
		if declaration.Kind != sourceDTDAttributeDeclaration || !declaration.ActuallyRead || declaration.IgnoredReason == sourceDTDIgnoredCutoff {
			continue
		}
		if declaration.DefaultKind != sourceDTDDefaultValue && declaration.DefaultKind != sourceDTDDefaultFixed {
			continue
		}
		before := r.tokenOwned
		if _, err := r.dtdAttributeValue(declaration.RawValue, declaration.Origin); err != nil {
			if declaration.Origin.Region == sourceDTDExternalSubsetRegion {
				return r.externalDTDDefect(err, declaration.Origin)
			}
			return sourceDTDDiagnosticAt(err, declaration.Origin)
		}
		// The normalized value is deliberately discarded. Its retained scratch,
		// temporary names and result bytes must not accumulate across declarations.
		release := r.tokenOwned - before
		if release > 0 {
			_ = r.account.Release(validation.BudgetBufferedBytes, release)
			r.tokenOwned = before
		}
	}
	return r.dtdStore.completeDefaultWellFormedness()
}

// externalDTDDefect reports a deferred check of an external declaration as a
// *DTDError: the defect is in the supplied DTD, not in the document.
// Resource, cancellation and other non-syntax errors keep their identity.
func (r *sourceXMLTokenReader) externalDTDDefect(err error, origin sourceDTDOrigin) error {
	var syntax *xml.SyntaxError
	if !errors.As(err, &syntax) {
		return err
	}
	state := r.dtdStore.State()
	return &DTDError{PublicID: state.PublicID, SystemID: state.SystemID, Err: sourceDTDExternalDefect(syntax.Msg, origin)}
}

// A deferred declaration check reports the declaration's actual source line,
// or the enclosing physical PE use for generated text. Resource/cancellation
// errors keep their original identity; the source cursor is never rewound.
func sourceDTDDiagnosticAt(err error, origin sourceDTDOrigin) error {
	var syntax *xml.SyntaxError
	if !errors.As(err, &syntax) {
		return err
	}
	line := 0
	if origin.SpanAvailable {
		line = origin.Span.Start.Line
	} else {
		for _, step := range origin.Ancestry {
			if step.UseAvailable && step.Use.Start.Line > 0 {
				line = step.Use.Start.Line
				break
			}
		}
	}
	if line == 0 {
		return err
	}
	return &xml.SyntaxError{Msg: syntax.Msg, Line: line}
}
