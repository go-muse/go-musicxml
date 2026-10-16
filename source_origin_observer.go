package musicxml

import (
	"context"

	"github.com/go-muse/go-musicxml/validation"
)

// sourceOriginObserver is elected only by the matching internal-DTD profile.
// It projects occurrence provenance; it does not claim per-character value
// segments or reconstruct source history from a current model.
type sourceOriginObserver struct {
	engine  *validation.Engine
	session *validation.Session
	profile validation.ProfileID
	key     validation.RuleKey
}

func (o *sourceFactObserver) publishSourceOrigin(frame *sourceObserverFrame, attribute *validation.QName, name validation.QName, location validation.Location, in *sourceXMLOrigin) error {
	if o.origins == nil {
		return nil
	}
	if o.origins.key == (validation.RuleKey{}) {
		// The root has already passed normal admission. Route only exact
		// no-namespace roots, never a foreign same-local-name occurrence.
		var assembly validation.Assembly
		switch name {
		case validation.QName{Local: "score-partwise"}, validation.QName{Local: "score-timewise"}:
			assembly = validation.AssemblyMusicXML
		case validation.QName{Local: "opus"}:
			assembly = validation.AssemblyOpus
		case validation.QName{Local: "container"}:
			assembly = validation.AssemblyContainer
		case validation.QName{Local: "sounds"}:
			assembly = validation.AssemblySounds
		default:
			return &validation.ConfigurationError{Field: "source origin", Reason: "exact supported source root required"}
		}
		key, ok := o.origins.engine.SourceOriginProvider(o.origins.profile, assembly)
		if !ok {
			return &validation.ConfigurationError{Field: "source origin", Reason: "selected assembly provider is unavailable"}
		}
		o.origins.key = key
	}
	if in == nil {
		return &validation.SessionError{Operation: "source origin", Reason: "selected scanner omitted occurrence metadata"}
	}
	// The session owns its own copies. This allowance covers only temporary
	// typed transport records and physical-site pointers during the call.
	if len(in.Ancestry) > validation.SourceOriginAncestryLimit {
		return &validation.IncompleteError{Reason: "source-origin-ancestry-limit"}
	}
	temporary := int64(1024 + 1024*len(in.Ancestry))
	if err := o.account.Charge(validation.BudgetBufferedBytes, temporary); err != nil {
		return err
	}
	defer o.account.Release(validation.BudgetBufferedBytes, temporary)
	absent := validation.Evidence[validation.Location]{State: validation.EvidenceAbsent}
	out := validation.SourceOriginContextResult{
		Subject: validation.NodeRef{DocumentID: o.documentID, Node: frame.id}, Attribute: attribute,
		Specified: validation.Evidence[bool]{State: validation.EvidenceKnown, Value: in.Specified},
		Physical:  absent, Declaration: absent, Use: absent,
		AncestryComplete: validation.Evidence[bool]{State: validation.EvidenceKnown, Value: in.AncestryComplete},
	}
	if !in.AncestryComplete {
		out.AncestryComplete.Reason = "source-origin-ancestry-unavailable"
	}
	switch in.Kind {
	case sourceOriginPhysical:
		out.Kind = validation.SourceOriginPhysical
	case sourceOriginDefault:
		out.Kind = validation.SourceOriginDefault
	case sourceOriginEntity:
		out.Kind = validation.SourceOriginEntity
	default:
		out.Kind = validation.SourceOriginUnavailable
	}
	if in.PhysicalAvailable {
		out.Physical = validation.Evidence[validation.Location]{State: validation.EvidenceKnown, Value: location}
	}
	if in.DeclarationAvailable {
		out.Declaration = o.sourceOriginSite(in.Declaration)
	} else if in.Kind == sourceOriginDefault || in.Kind == sourceOriginEntity || in.DeclarationExpected {
		reason := "source-origin-declaration-unavailable"
		if in.DeclarationUnavailableReason != "" {
			reason = in.DeclarationUnavailableReason
		}
		out.Declaration = validation.Evidence[validation.Location]{State: validation.EvidenceUnknown, Reason: validation.Code(reason)}
	}
	if in.UseAvailable {
		out.Use = o.sourceOriginSite(in.Use)
	} else if in.Kind == sourceOriginDefault || in.Kind == sourceOriginEntity || in.UseExpected {
		out.Use = validation.Evidence[validation.Location]{State: validation.EvidenceUnknown, Reason: "source-origin-use-unavailable"}
	}
	if len(in.Ancestry) != 0 {
		out.Ancestry = make([]validation.SourceOriginStep, len(in.Ancestry))
		for i, step := range in.Ancestry {
			if err := o.ctx.Err(); err != nil {
				return err
			}
			out.Ancestry[i] = validation.SourceOriginStep{Entity: step.Entity,
				Declaration: validation.Evidence[validation.Location]{State: validation.EvidenceUnknown, Reason: "source-origin-declaration-unavailable"},
				Use:         validation.Evidence[validation.Location]{State: validation.EvidenceUnknown, Reason: "source-origin-use-unavailable"}}
			if step.DeclarationAvailable {
				out.Ancestry[i].Declaration = o.sourceOriginSite(step.Declaration)
			}
			if step.UseAvailable {
				out.Ancestry[i].Use = o.sourceOriginSite(step.Use)
			}
		}
	}
	return o.origins.session.SourceOrigin(o.ctx, o.origins.key, out)
}

func (o *sourceFactObserver) sourceOriginSite(span sourceXMLSpan) validation.Evidence[validation.Location] {
	return validation.Evidence[validation.Location]{State: validation.EvidenceKnown, Value: o.location("", span.Start)}
}

func (o *sourceFactObserver) sealSourceOrigins(ctx context.Context) error {
	if o == nil || o.origins == nil || o.origins.key == (validation.RuleKey{}) {
		return nil
	}
	complete := validation.Evidence[bool]{State: validation.EvidenceKnown, Value: true}
	if o.dtdUnavailable {
		complete = validation.Evidence[bool]{State: validation.EvidenceUnsupported, Reason: "dtd-infoset-unavailable"}
	}
	return o.origins.session.CompleteSourceOrigins(ctx, o.origins.key, complete)
}
