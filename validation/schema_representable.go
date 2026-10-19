package validation

import (
	"context"
	"fmt"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

// xsdCheckRepresentable is the source-line operation that each numeric value
// the generated Go model holds survives conversion unchanged: an integer
// within its Go type's range, a decimal whose shortest binary64 form is the
// same number. It is not an XSD constraint but a limit of the model, so only
// the registered constructor creates it and only source rules use it.
const xsdCheckRepresentable XSDCheck = "representable"

// newXSDRepresentabilityPlan creates the representability operation for one
// assembly. sourceDeclarations must match the other XSD operations of the
// assembly in the same profile.
func newXSDRepresentabilityPlan(schema *CompiledSchema, sourceDeclarations bool) (Plan, error) {
	p := Plan{kind: planXSD, schema: schema, xsdCheck: xsdCheckRepresentable}
	if sourceDeclarations {
		p.xsdMode = xsdSourceDeclarationsV1
	}
	return checkedPlan(p)
}

// checkRepresentable records once, after the node's binding completes, which
// of its known atomic numeric values the model cannot hold unchanged.
func (s *Session) checkRepresentable(ctx context.Context, n *factNode) error {
	state := s.schema
	if len(state.proofRules[state.active.assembly][xsdCheckRepresentable]) == 0 {
		return nil
	}
	bound := state.nodes[n.header.ID]
	if bound.representable || bound.skipped || !bound.available {
		return nil
	}
	var details []xsd.Detail
	check := func(scalar xsd.ScalarResult, path string, attribute QName) error {
		if err := s.step(ctx, 1); err != nil {
			return err
		}
		if !scalar.Known || !scalar.Atomic || scalar.Value.Domain != xsd.NumberDomain {
			return nil
		}
		representation, ok := xsd.ModelRepresentationOf(scalar.Builtin)
		if !ok || representation.Represents(scalar.Value.Number) {
			return nil
		}
		form := "a binary64"
		if !representation.Binary64 {
			form = "its Go integer range"
		}
		message := fmt.Sprintf("value %q of xs:%s has no exact form in %s", truncateRepresentable(scalar.EffectiveLexeme), scalar.Builtin, form)
		if err := s.retain(int64(64 + len(path) + len(message))); err != nil {
			return err
		}
		details = append(details, xsd.Detail{Issue: xsd.Issue{Path: path, Constraint: "representable", Message: message}, Subject: xsd.SubjectID(n.header.ID), Attribute: xsd.QName{Space: attribute.Space, Local: attribute.Local}})
		return nil
	}
	path := n.header.Location.Path
	if err := check(state.run.Scalar(bound.binding), path, QName{}); err != nil {
		return err
	}
	for attribute := range state.run.Attributes(bound.binding) {
		if !attribute.Present {
			continue
		}
		name := QName{Space: attribute.Name.Space, Local: attribute.Name.Local}
		if err := check(attribute.Scalar, path+"/@"+name.Local, name); err != nil {
			return err
		}
	}
	bound.representable, bound.unrepresentable = true, details
	state.nodes[n.header.ID] = bound
	return nil
}

// truncateRepresentable keeps diagnostics bounded for very long numerals.
func truncateRepresentable(lexeme string) string {
	if len(lexeme) > 64 {
		return lexeme[:61] + "..."
	}
	return lexeme
}
