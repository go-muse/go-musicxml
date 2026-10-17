package musicxml

import "github.com/go-muse/go-musicxml/validation"

// A non-XML sentinel is exposed only to the DTD parser's between-declarations
// dispatch. Read never crosses it, so a declaration cannot borrow caller input.
const sourceDTDParameterBoundary rune = -2

func (r *sourceXMLTokenReader) PushDTDParameter(declaration *sourceDTDDeclaration, use sourceDTDOrigin) error {
	if err := r.event(); err != nil {
		return err
	}
	input := sourceXMLEntityInput{Name: declaration.Name, Replacement: declaration.Replacement, Parameter: true, Padded: true, ExternalMarkup: true, Declaration: declaration.Origin.Span, DeclarationAvailable: declaration.Origin.SpanAvailable, DeclarationEntity: declaration.Origin.Entity, DeclarationRecord: declaration}
	if err := r.pushEntity(input, sourceEntityDTD, use.Span); err != nil {
		return err
	}
	r.entity.useAvailable = use.SpanAvailable
	return nil
}
func (r *sourceXMLTokenReader) FinishDTDParameter() error { return r.finishEntity(sourceEntityDTD) }
func (r *sourceXMLTokenReader) DTDInParameter() bool {
	return r.entity != nil && r.entity.input.Parameter
}

// The ancestry snapshot borrows reader-owned scratch valid until the next call
// to DTDParameterOrigin or completion of DOCTYPE parsing. The declaration store
// must copy and account retained ancestry synchronously before either boundary.
func (r *sourceXMLTokenReader) DTDParameterOrigin() (sourceDTDOrigin, error) {
	if err := r.cursor.contextErr(); err != nil {
		return sourceDTDOrigin{}, err
	}
	origin := sourceDTDOrigin{Region: sourceDTDInternalSubset, SpanAvailable: true, Span: sourceXMLSpan{Start: r.position()}, AncestryComplete: true}
	if !r.DTDInParameter() {
		return origin, nil
	}
	origin.Region = sourceDTDParameterEntityRegion
	origin.SpanAvailable = false
	origin.Span = sourceXMLSpan{}
	origin.ExternalMarkup = true
	depth := r.entity.depth
	if depth > cap(r.dtdOriginScratch) {
		capacity := max(depth, 2*cap(r.dtdOriginScratch))
		if err := r.reserve(int64(capacity) * 160); err != nil {
			return sourceDTDOrigin{}, err
		}
		next := make([]sourceDTDOriginStep, capacity)
		// The new slice is fully admitted while the old backing array is live.
		r.releaseDTDOriginScratch()
		r.dtdOriginScratch = next
		r.retained += int64(capacity) * 160
	}
	clear(r.dtdOriginScratch)
	origin.Ancestry = r.dtdOriginScratch[:depth]
	frame := r.entity
	for i := depth - 1; frame != nil; i-- {
		origin.Ancestry[i] = sourceDTDOriginStep{Entity: frame.input.Name, DeclarationAvailable: frame.input.DeclarationAvailable, UseAvailable: frame.useAvailable, Declaration: frame.input.Declaration, Use: frame.use, ReplacementOffset: frame.replacementOffset(), DeclarationRecord: frame.input.DeclarationRecord}
		frame = frame.parent
	}
	return origin, nil
}

func (r *sourceXMLTokenReader) releaseDTDOriginScratch() {
	bytes := int64(cap(r.dtdOriginScratch)) * 160
	if r.account != nil && bytes != 0 {
		_ = r.account.Release(validation.BudgetBufferedBytes, bytes)
	}
	r.retained -= bytes
	r.dtdOriginScratch = nil
}
