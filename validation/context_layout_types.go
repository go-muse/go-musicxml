package validation

import (
	"context"
	"strings"
	"unsafe"
)

// ContextUnit identifies the source-defined unit of a layout scalar.
type ContextUnit string

// Layout quantities retain their encoded unit independently of physical conversion.
const (
	ContextUnitNone        ContextUnit = ""
	ContextUnitTenths      ContextUnit = "global-tenths"
	ContextUnitDivisions   ContextUnit = "divisions"
	ContextUnitMillimeters ContextUnit = "millimeters"
	ContextUnitPercent     ContextUnit = "percent"
	ContextUnitPoints      ContextUnit = "points"
	ContextUnitEm          ContextUnit = "em"
)

// ContextProvenance identifies why an effective context field has its value.
type ContextProvenance string

// Supported context origins distinguish explicit, default and inherited values.
const (
	// ContextExplicit marks a direct encoded observation, including observed
	// absence; it does not assert that an effective value exists.
	ContextExplicit          ContextProvenance = "explicit"
	ContextDocumentedDefault ContextProvenance = "documented-default"
	ContextInherited         ContextProvenance = "inherited"
	ContextApplicationChoice ContextProvenance = "application-choice"
)

// LayoutContextResult retains scalar scope, applicability and default provenance
// for both local scalar and layout projections, without pretending
// that MusicXML specifies an engraver's complete physical geometry.
type LayoutContextResult struct {
	Scope, Origin NodeRef
	// Part identifies the accepted score part for per-part rows, including timewise measures.
	Part Evidence[NodeRef]
	// Page and System identify established encoded layout occurrences.
	Page, System Evidence[NodeRef]
	// Staff is a sparse within-part selector, never an expanded staff array.
	Staff Evidence[Integer]
	// Conversion preserves an exact ratio when decimal output is insufficient.
	Conversion Evidence[Rational]
	// Basis records a source-defined coordinate relation, not invented geometry.
	Basis         string
	Raw           Evidence[Atomic]
	Applicability Evidence[bool]
	Unit          ContextUnit
	Provenance    ContextProvenance
}

func cloneLayoutContext(ctx context.Context, s *Session, p *LayoutContextResult) (*LayoutContextResult, error) {
	if p == nil {
		return nil, nil
	}
	value := *p
	if value.Part.State == "" {
		value.Part.State = EvidenceAbsent
	}
	size, work, err := layoutContextSize(value)
	if err != nil {
		return nil, err
	}
	if err = s.step(ctx, work); err != nil {
		return nil, err
	}
	if _, err = allocationSize(size); err != nil {
		return nil, err
	}
	if err = s.retain(size); err != nil {
		return nil, err
	}
	q := cloneLayoutContextValue(value)
	return &q, nil
}

// layoutContextSize counts the flat object once, every owned string/cause, and
// the immutable numeric storage retained by all four wrappers. The fixed 512
// byte allowance covers numeric object headers and allocator rounding; report
// backing-array growth is independently covered by publishContextResult.
func layoutContextSize(p LayoutContextResult) (size, work int64, err error) {
	size = int64(unsafe.Sizeof(LayoutContextResult{})) + 512
	work = 16
	add := func(n int64) {
		if err != nil {
			return
		}
		size, err = rationalCheckedAdd(BudgetBufferedBytes, size, n)
		if err == nil {
			work, err = rationalCheckedAdd(BudgetRuleSteps, work, n)
		}
	}
	for _, v := range []string{p.Scope.DocumentID, p.Origin.DocumentID, p.Part.Value.DocumentID, p.Page.Value.DocumentID, p.System.Value.DocumentID, p.Basis, string(p.Unit), string(p.Provenance), string(p.Raw.Value.Kind)} {
		add(int64(len(v)))
	}
	add(layoutEvidenceBytes(p.Part))
	add(layoutEvidenceBytes(p.Page))
	add(layoutEvidenceBytes(p.System))
	add(layoutEvidenceBytes(p.Staff))
	add(layoutEvidenceBytes(p.Conversion))
	add(layoutEvidenceBytes(p.Raw))
	add(layoutEvidenceBytes(p.Applicability))
	add(p.Staff.Value.ByteSize())
	add(p.Conversion.Value.ByteSize())
	add(atomicLogicalBytes(p.Raw.Value))
	return
}
func layoutEvidenceBytes[T any](v Evidence[T]) int64 {
	return int64(len(v.State)) + evidenceLogicalBytes(v)
}

// Numeric wrappers are immutable and may be shared; all editable causes and
// every string, including values carried by unavailable evidence, are owned.
func cloneLayoutContextValue(p LayoutContextResult) LayoutContextResult {
	q := p
	q.Scope.DocumentID = strings.Clone(p.Scope.DocumentID)
	q.Origin.DocumentID = strings.Clone(p.Origin.DocumentID)
	q.Part = cloneEvidence(p.Part)
	q.Part.Value.DocumentID = strings.Clone(p.Part.Value.DocumentID)
	q.Page = cloneEvidence(p.Page)
	q.Page.Value.DocumentID = strings.Clone(p.Page.Value.DocumentID)
	q.System = cloneEvidence(p.System)
	q.System.Value.DocumentID = strings.Clone(p.System.Value.DocumentID)
	q.Staff = cloneEvidence(p.Staff)
	q.Conversion = cloneEvidence(p.Conversion)
	q.Basis = strings.Clone(p.Basis)
	q.Raw = cloneEvidence(p.Raw)
	q.Raw.Value = cloneAtomic(p.Raw.Value)
	q.Applicability = cloneEvidence(p.Applicability)
	q.Unit = ContextUnit(strings.Clone(string(p.Unit)))
	q.Provenance = ContextProvenance(strings.Clone(string(p.Provenance)))
	return q
}

type contextLayoutService struct {
	Final       bool
	State       *layoutStateIndex
	Blocks      []timelineMeasureBlock
	BlocksReady bool
}
