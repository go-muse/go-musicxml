package musicxml

// A non-XML sentinel is exposed only to the DTD parser's between-declarations
// dispatch. Read never crosses it, so a declaration cannot borrow caller input.
const sourceDTDParameterBoundary rune = -2

func (r *sourceXMLTokenReader) PushDTDParameter(declaration *sourceDTDDeclaration, use sourceDTDOrigin) error {
	if err := r.event(); err != nil {
		return err
	}
	input := sourceXMLEntityInput{Name: declaration.Name, Replacement: declaration.Replacement, Parameter: true, Padded: true, ExternalMarkup: true, Declaration: declaration.Origin.Span, DeclarationAvailable: declaration.Origin.SpanAvailable, DeclarationRecord: declaration}
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

// The snapshot is temporary token-owned storage. The declaration store copies
// and accounts its own retained ancestry before this DOCTYPE token is released.
func (r *sourceXMLTokenReader) DTDParameterOrigin() (sourceDTDOrigin, error) {
	origin := sourceDTDOrigin{Region: sourceDTDInternalSubset, SpanAvailable: true, Span: sourceXMLSpan{Start: r.position()}, AncestryComplete: true}
	if !r.DTDInParameter() {
		return origin, nil
	}
	origin.Region = sourceDTDParameterEntityRegion
	origin.SpanAvailable = false
	origin.Span = sourceXMLSpan{}
	origin.ExternalMarkup = true
	depth := r.entity.depth
	if err := r.tokenReserve(int64(depth) * 160); err != nil {
		return sourceDTDOrigin{}, err
	}
	origin.Ancestry = make([]sourceDTDOriginStep, depth)
	frame := r.entity
	for i := depth - 1; frame != nil; i-- {
		origin.Ancestry[i] = sourceDTDOriginStep{Entity: frame.input.Name, DeclarationAvailable: frame.input.DeclarationAvailable, UseAvailable: frame.useAvailable, Declaration: frame.input.Declaration, Use: frame.use, ReplacementOffset: int64(frame.offset), DeclarationRecord: frame.input.DeclarationRecord}
		frame = frame.parent
	}
	return origin, nil
}
