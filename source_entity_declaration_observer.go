package musicxml

import (
	"context"
	"math"
	"strings"
	"unsafe"

	"github.com/go-muse/go-musicxml/validation"
)

// This separate synchronous capability leaves legacy source fact sinks
// unchanged. The selected session admits and owns every retained declaration.
type sourceEntityDeclarationSink interface {
	SourceEntityDeclarations(context.Context, validation.SourceEntityDeclarations) error
}

func (o *sourceFactObserver) publishSourceEntityDeclarations(reader *sourceXMLTokenReader) error {
	sink, ok := o.sink.(sourceEntityDeclarationSink)
	if !ok || reader == nil {
		return &validation.ConfigurationError{Field: "source entity declarations", Reason: "selected scanner and declaration ingress required"}
	}
	if reader.account != o.account {
		return &validation.ConfigurationError{Field: "source entity declarations", Reason: "scanner and observer must share the same account"}
	}
	return reader.exportEntityDeclarations(o.ctx, o.documentID, o.entry, sink)
}

// exportEntityDeclarations projects only effective first general bindings. It
// runs after root admission, while the scanner still owns its declaration store.
// The snapshot owns all strings and positions during the synchronous sink call;
// the session's independently admitted copies overlap this temporary charge.
// Neither scanner pointers nor resolver/reader callbacks enter the fact stream.
func (r *sourceXMLTokenReader) exportEntityDeclarations(ctx context.Context, documentID, entry string, sink sourceEntityDeclarationSink) error {
	if ctx == nil {
		return &validation.ConfigurationError{Field: "Context", Reason: "declaration export requires a nonnil context"}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if r == nil || r.closed || !r.internalDTD || r.account == nil || !r.seenRoot || sink == nil {
		return &validation.ConfigurationError{Field: "source entity declarations", Reason: "open selected scanner after first root and synchronous ingress required"}
	}
	complete := validation.Evidence[bool]{State: validation.EvidenceUnknown, Reason: "source-entity-declarations-unavailable"}
	var declarations []*sourceDTDDeclaration
	if r.dtdStore != nil {
		if err := r.dtdStore.check(); err != nil {
			return err
		}
		declarations = r.dtdStore.Declarations()
		complete = validation.Evidence[bool]{State: validation.EvidenceKnown, Value: r.dtdStore.State().UnparsedEntitiesComplete}
		if !complete.Value {
			complete.Reason = "source-entity-declarations-incomplete"
		}
	} else if !r.doctype {
		// The elected scanner has actually completed the prolog. A missing store
		// by itself, or an unelected scanner, cannot prove the empty domain.
		complete = validation.Evidence[bool]{State: validation.EvidenceKnown, Value: true}
	}
	temporary := int64(unsafe.Sizeof(validation.SourceEntityDeclarations{}))
	add := func(n int64) error {
		if n < 0 || n > math.MaxInt64-temporary {
			return &validation.ResourceError{Kind: validation.BudgetBufferedBytes, Requested: math.MaxInt64}
		}
		temporary += n
		return nil
	}
	if err := add(int64(len(documentID))); err != nil {
		return err
	}
	if err := add(int64(len(entry))); err != nil {
		return err
	}
	count := 0
	for _, d := range declarations {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := r.account.Charge(validation.BudgetRuleSteps, 1); err != nil {
			return err
		}
		if !sourceEffectiveGeneralDeclaration(d) {
			continue
		}
		count++
		if err := add(int64(unsafe.Sizeof(validation.SourceEntityDeclaration{}))); err != nil {
			return err
		}
		if err := add(int64(len(d.Name))); err != nil {
			return err
		}
		if d.Origin.SpanAvailable {
			if err := add(int64(unsafe.Sizeof(validation.SourcePosition{}))); err != nil {
				return err
			}
		}
	}
	if _, err := validation.AllocationSize(temporary); err != nil {
		return err
	}
	if err := r.account.Charge(validation.BudgetBufferedBytes, temporary); err != nil {
		return err
	}
	defer r.account.Release(validation.BudgetBufferedBytes, temporary)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.account.Charge(validation.BudgetRuleSteps, int64(len(documentID))+int64(len(entry))+1); err != nil {
		return err
	}
	ownedID, ownedEntry := strings.Clone(documentID), strings.Clone(entry)
	facts := validation.SourceEntityDeclarations{DocumentID: ownedID, Declarations: make([]validation.SourceEntityDeclaration, 0, count), Complete: complete}
	for _, d := range declarations {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := r.account.Charge(validation.BudgetRuleSteps, 1); err != nil {
			return err
		}
		if !sourceEffectiveGeneralDeclaration(d) {
			continue
		}
		if err := r.account.Charge(validation.BudgetRuleSteps, int64(len(d.Name))+1); err != nil {
			return err
		}
		fact := validation.SourceEntityDeclaration{Name: strings.Clone(d.Name), Unparsed: d.Notation != "", Sequence: uint64(d.Origin.Sequence), Origin: validation.Evidence[validation.Location]{State: validation.EvidenceUnknown, Reason: "source-entity-declaration-origin-unavailable"}}
		if d.Origin.SpanAvailable {
			p := d.Origin.Span.Start
			fact.Origin = validation.Evidence[validation.Location]{State: validation.EvidenceKnown, Value: validation.Location{Target: validation.TargetSource, DocumentID: ownedID, Entry: ownedEntry, Position: &validation.SourcePosition{ByteOffset: p.ByteOffset, Line: p.Line, Column: p.Column, CoordinateSpace: "original-bytes"}}}
		}
		facts.Declarations = append(facts.Declarations, fact)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return sink.SourceEntityDeclarations(ctx, facts)
}

func sourceEffectiveGeneralDeclaration(d *sourceDTDDeclaration) bool {
	return d != nil && d.Kind == sourceDTDGeneralEntityDeclaration && !d.Parameter && d.ActuallyRead && d.Binding
}
