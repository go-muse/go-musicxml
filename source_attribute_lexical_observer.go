package musicxml

import (
	"context"

	"github.com/go-muse/go-musicxml/validation"
)

// This optional synchronous ingress is used only by the explicitly selected
// physical-lexical resource contract. Like Attribute, the sink owns its copies.
type sourceAttributeLexicalSink interface {
	SourceAttributeLexical(context.Context, validation.RuleKey, validation.NodeRef, validation.QName, validation.Evidence[string]) error
}

func (o *sourceFactObserver) publishSourceAttributeLexical(frame *sourceObserverFrame, name validation.QName, meta sourceXMLAttribute) error {
	if !o.captureAttributeLexical {
		return nil
	}
	if err := o.ctx.Err(); err != nil {
		return err
	}
	sink, ok := o.sink.(sourceAttributeLexicalSink)
	if !ok || o.origins == nil || o.origins.key == (validation.RuleKey{}) {
		return &validation.ConfigurationError{Field: "source attribute lexical", Reason: "selected physical lexical ingress and source-origin provider required"}
	}
	if err := o.account.Charge(validation.BudgetRuleSteps, 1); err != nil {
		return err
	}
	raw := validation.Evidence[string]{State: validation.EvidenceUnknown, Reason: "source-attribute-lexical-unavailable"}
	if meta.Origin != nil && !meta.Origin.Specified {
		raw = validation.Evidence[string]{State: validation.EvidenceAbsent}
	} else if meta.Origin != nil && meta.Origin.Kind == sourceOriginPhysical && meta.Origin.PhysicalAvailable && meta.Lexical.State != "" {
		raw = meta.Lexical
	}
	// Attribute and its exact source origin have already been published. The
	// retained scanner string remains alive throughout this synchronous call.
	return sink.SourceAttributeLexical(o.ctx, o.origins.key, validation.NodeRef{DocumentID: o.documentID, Node: frame.id}, name, raw)
}
