package validation

import "context"

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
	if err := contextRetainProjection(ctx, s, value); err != nil {
		return nil, err
	}
	q := stateClone(value)
	return &q, nil
}

type contextLayoutService struct {
	Final bool
	State *layoutStateIndex
}
