package musicxml

// Origins are private scanner evidence. A generated value has no fabricated
// physical byte coordinates; its declaration and use sites are separate facts.
type sourceXMLOriginKind uint8

const (
	sourceOriginUnavailable sourceXMLOriginKind = iota
	sourceOriginPhysical
	sourceOriginDefault
	sourceOriginEntity
)

type sourceXMLOriginStep struct {
	Entity                             string
	DeclarationAvailable, UseAvailable bool
	Declaration, Use                   sourceXMLSpan
}
type sourceXMLOrigin struct {
	Kind                               sourceXMLOriginKind
	DeclarationExpected, UseExpected   bool
	DeclarationUnavailableReason       string
	Specified                          bool
	PhysicalAvailable                  bool
	DeclarationAvailable, UseAvailable bool
	Declaration, Use                   sourceXMLSpan
	Ancestry                           []sourceXMLOriginStep
	AncestryComplete                   bool
}

func (r *sourceXMLTokenReader) captureOrigin(target **sourceXMLOrigin, specified bool) error {
	if !r.internalDTD {
		return nil
	}
	if err := r.tokenReserve(256); err != nil {
		return err
	}
	out := &sourceXMLOrigin{Kind: sourceOriginPhysical, Specified: specified, PhysicalAvailable: true, AncestryComplete: true}
	*target = out
	if r.entity == nil {
		return nil
	}
	frame := r.entity
	out.Kind = sourceOriginEntity
	out.DeclarationExpected, out.UseExpected = true, true
	out.Specified = false
	out.PhysicalAvailable = false
	out.Declaration = frame.input.Declaration
	out.DeclarationAvailable = frame.input.DeclarationAvailable
	out.Use = frame.use
	out.UseAvailable = frame.useAvailable
	if err := r.tokenReserve(int64(frame.depth) * 160); err != nil {
		return err
	}
	out.Ancestry = make([]sourceXMLOriginStep, frame.depth)
	for i := frame.depth - 1; frame != nil; i-- {
		out.Ancestry[i] = sourceXMLOriginStep{Entity: frame.input.Name, DeclarationAvailable: frame.input.DeclarationAvailable, Declaration: frame.input.Declaration, UseAvailable: frame.useAvailable, Use: frame.use}
		frame = frame.parent
	}
	return nil
}
