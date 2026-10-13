package musicxml

// A non-XML sentinel is exposed only to the DTD parser's between-declarations
// dispatch. Read never crosses it, so a declaration cannot borrow caller input.
const sourceDTDParameterBoundary rune = -2

func (r *sourceXMLTokenReader) PushDTDParameter(declaration *sourceDTDDeclaration, use sourceDTDOrigin) error {
	if err := r.event(); err != nil {
		return err
	}
	input := sourceXMLEntityInput{Name: declaration.Name, Replacement: declaration.Replacement, Parameter: true, Padded: true, Declaration: declaration.Origin.Span, DeclarationAvailable: declaration.Origin.SpanAvailable}
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
