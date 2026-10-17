package musicxml

import (
	"context"
	"github.com/go-muse/go-musicxml/validation"
)

type sourceDTDIDSink interface {
	SourceDTDIDsSelected() bool
	SourceDTDIDDomain(context.Context, validation.SourceDTDIDDomain) error
	SourceDTDIDAttribute(context.Context, validation.SourceDTDIDAttribute) error
	CompleteSourceDTDIDs(context.Context, string) error
}

func (o *sourceFactObserver) publishSourceDTDIDDomain(r *sourceXMLTokenReader) error {
	if !o.sourceDTDIDs {
		return nil
	}
	sink, ok := o.sink.(sourceDTDIDSink)
	if ok && !sink.SourceDTDIDsSelected() {
		// Scanner capture is profile-wide until Open admits the root. Keep
		// publication and sealing confined to the exact selected assembly.
		o.sourceDTDIDs = false
		return nil
	}
	if !ok || r == nil || !r.internalDTD || !r.captureDTDIDs || !r.seenRoot || r.account != o.account {
		return &validation.ConfigurationError{Field: "source DTD IDs", Reason: "selected scanner and synchronous same-account ingress required"}
	}
	complete := validation.Evidence[bool]{State: validation.EvidenceUnknown, Reason: "source-dtd-id-declarations-unavailable"}
	if r.dtdStore != nil {
		complete = validation.Evidence[bool]{State: validation.EvidenceKnown, Value: r.dtdStore.State().AttributeDeclarationsComplete}
		if !complete.Value {
			complete.Reason = "source-dtd-id-declarations-incomplete"
		}
	} else if !r.doctype {
		complete = validation.Evidence[bool]{State: validation.EvidenceKnown, Value: true}
	}
	return sink.SourceDTDIDDomain(o.ctx, validation.SourceDTDIDDomain{DocumentID: o.documentID, Complete: complete})
}
func (o *sourceFactObserver) publishSourceDTDIDAttribute(frame *sourceObserverFrame, name validation.QName, value string, location validation.Location, m sourceXMLAttribute) error {
	if !o.sourceDTDIDs || m.NamespaceDeclaration {
		return nil
	}
	sink, ok := o.sink.(sourceDTDIDSink)
	if !ok || m.Origin == nil {
		return &validation.ConfigurationError{Field: "source DTD IDs", Reason: "selected ingress and source occurrence origin required"}
	}
	// Temporary typed record, optional declaration and source-position pointers
	// are admitted before creation; strings borrow scanner-owned token/store data.
	const temporary int64 = 2048
	if err := o.account.Charge(validation.BudgetBufferedBytes, temporary); err != nil {
		return err
	}
	defer o.account.Release(validation.BudgetBufferedBytes, temporary)
	f := validation.SourceDTDIDAttribute{Subject: validation.NodeRef{DocumentID: o.documentID, Node: frame.id}, Attribute: name, Value: value, IsID: validation.Evidence[bool]{State: validation.EvidenceUnknown, Reason: "source-dtd-id-declarations-incomplete"}, Specified: validation.Evidence[bool]{State: validation.EvidenceKnown, Value: m.Origin.Specified}, Use: validation.Evidence[validation.Location]{State: validation.EvidenceAbsent}}
	if m.DTDIDDomainComplete {
		f.IsID = validation.Evidence[bool]{State: validation.EvidenceKnown, Reason: "source-dtd-id-not-declared"}
	}
	if m.Origin.PhysicalAvailable {
		f.Use = validation.Evidence[validation.Location]{State: validation.EvidenceKnown, Value: location}
	} else if m.Origin.UseAvailable {
		f.Use = o.sourceOriginSite(m.Origin.Use)
	} else if m.Origin.UseExpected {
		f.Use = validation.Evidence[validation.Location]{State: validation.EvidenceUnknown, Reason: "source-dtd-id-use-unavailable"}
	}
	if d := m.DTDID; d != nil {
		f.IsID = validation.Evidence[bool]{State: validation.EvidenceKnown, Value: true}
		f.Declaration = &validation.SourceDTDIDDeclaration{Sequence: uint64(d.Origin.Sequence), Element: d.Element, Attribute: d.Name, DefaultKind: string(d.DefaultKind), Origin: validation.Evidence[validation.Location]{State: validation.EvidenceUnknown, Reason: "source-dtd-id-declaration-origin-unavailable"}}
		if d.Origin.SpanAvailable {
			f.Declaration.Origin = o.sourceDeclarationSite(d.Origin.Entity, d.Origin.Span)
		}
	}
	return sink.SourceDTDIDAttribute(o.ctx, f)
}
func (o *sourceFactObserver) sealSourceDTDIDs(ctx context.Context) error {
	if o == nil || !o.sourceDTDIDs {
		return nil
	}
	sink, ok := o.sink.(sourceDTDIDSink)
	if !ok {
		return &validation.ConfigurationError{Field: "source DTD IDs", Reason: "selected ingress required"}
	}
	return sink.CompleteSourceDTDIDs(ctx, o.documentID)
}
