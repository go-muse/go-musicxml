package validation

import (
	"context"
	"strings"
)

// ModelAxis identifies the current-model evidence affected by a representation
// defect. A node-level defect does not automatically invalidate other axes.
type ModelAxis string

// Supported representation axes. Attribute problems name one exact attribute.
const (
	ModelAxisScalar    ModelAxis = "scalar"
	ModelAxisAttribute ModelAxis = "attribute"
	ModelAxisChildren  ModelAxis = "children"
	ModelAxisNode      ModelAxis = "node"
)

// ModelProblem describes an established current-object defect, not an XML
// syntax or normative schema failure. Code is a stable identifier. Location
// identifies the affected Go field/index on Owner; Attribute is populated only
// for ModelAxisAttribute. Expected and Actual are optional UTF-8 previews, each
// bounded to 4096 bytes. They must never contain invalid raw string bytes.
type ModelProblem struct {
	Code             Code
	Axis             ModelAxis
	Attribute        QName
	Location         Location
	Expected, Actual string
}

// ModelRepresentationFact seals the local representation examination for one
// currently open occurrence. Complete is explicit examination evidence, not a
// claim that its values are valid. Problems remain authoritative even when the
// examination is incomplete. At most 4096 problems may be supplied per owner.
// After this event, only Close may extend that owner's facts.
type ModelRepresentationFact struct {
	Owner    NodeID
	Complete Evidence[bool]
	Problems []ModelProblem
}

// NewModelRepresentationPlan creates a sealed model-only node predicate for the
// selected schema assembly. It requires a mandatory CategoryModel rule with an
// identical assembly selector and no prerequisites. A missing examination fact
// is unsupported, never an implicit pass. Failures add required model blockers;
// they do not increment normative Counts.Fail.
func NewModelRepresentationPlan(assembly Assembly) (Plan, error) {
	return checkedPlan(Plan{kind: planModelRepresentation, modelAssembly: assembly})
}

// ModelRepresentation supplies one copied and bounded local examination seal.
// It accepts only model targets and the currently open owner, once, before
// Close. Invalid scalar/attribute values must initially be unknown/unsupported;
// invalid children must have unavailable order and completeness at Close. The
// session records a real model verdict before publishing causal invalid facts.
func (s *Session) ModelRepresentation(ctx context.Context, fact ModelRepresentationFact) error {
	if err := s.event(ctx, "ModelRepresentation"); err != nil {
		return err
	}
	reject := func(reason string) error {
		return s.fail(&SessionError{Operation: "ModelRepresentation", Reason: reason})
	}
	if s.profile.Target != TargetModel {
		return reject("model target required")
	}
	if len(s.stack) == 0 {
		return reject("no open owner")
	}
	n := s.stack[len(s.stack)-1]
	if fact.Owner != n.header.ID || n.modelRepresentation != nil {
		return reject("wrong owner or repeated examination")
	}
	if len(fact.Problems) > 4096 {
		return reject("too many representation problems")
	}
	if err := validateEvidence(fact.Complete, "model representation completeness"); err != nil {
		return s.fail(err)
	}
	if fact.Complete.State == EvidenceAbsent || fact.Complete.State == EvidenceInvalid {
		return reject("completeness must be known or unavailable")
	}
	size := int64(128) + evidenceLogicalBytes(fact.Complete)
	for i, p := range fact.Problems {
		// Check lengths before UTF-8 scans, allocation sizing, or retained copies.
		if len(p.Code) > maxIdentityBytes || len(p.Axis) > 32 || len(p.Attribute.Space) > 4096 || len(p.Attribute.Local) > maxIdentityBytes || len(p.Location.Target) > 32 || len(p.Location.DocumentID) > maxIdentityBytes || len(p.Location.Path) > 16384 || len(p.Location.Entry) > 4096 || len(p.Expected) > 4096 || len(p.Actual) > 4096 {
			return reject("oversized representation problem")
		}
		if p.Location.Position != nil {
			return reject("model problem cannot have source coordinates")
		}
		bytes := modelProblemLogicalBytes(p)
		if err := s.step(ctx, 1+bytes/64); err != nil {
			return s.fail(err)
		}
		if !validID(string(p.Code)) || !validText(p.Expected, 4096) || !validText(p.Actual, 4096) {
			return reject("invalid problem code or detail")
		}
		if err := validateLocation(p.Location, TargetModel, s.documentID); err != nil {
			return s.fail(err)
		}
		switch p.Axis {
		case ModelAxisScalar, ModelAxisChildren, ModelAxisNode:
			if p.Attribute != (QName{}) {
				return reject("attribute slot on non-attribute problem")
			}
		case ModelAxisAttribute:
			if !validQName(p.Attribute) {
				return reject("exact attribute slot required")
			}
			found := false
			for _, a := range n.attributes {
				if err := s.step(ctx, 1); err != nil {
					return s.fail(err)
				}
				if a.Name == p.Attribute {
					found = true
					if a.Presence.State != EvidenceKnown || !a.Presence.Value || !modelUnavailable(a.Scalar.Value.State) {
						return reject("attribute problem contradicts its slot")
					}
					break
				}
			}
			if !found {
				return reject("attribute problem has no supplied slot")
			}
		default:
			return reject("unknown representation axis")
		}
		for j := 0; j < i; j++ {
			if err := s.step(ctx, 1); err != nil {
				return s.fail(err)
			}
			previous := fact.Problems[j]
			if previous.Code == p.Code && previous.Axis == p.Axis && previous.Attribute == p.Attribute && previous.Location.Path == p.Location.Path {
				return reject("duplicate representation problem")
			}
		}
		size += bytes
	}
	if err := s.step(ctx, 1+evidenceLogicalBytes(fact.Complete)/64); err != nil {
		return s.fail(err)
	}
	if err := s.retain(size); err != nil {
		return s.fail(err)
	}
	owned := fact
	owned.Complete = cloneEvidence(fact.Complete)
	owned.Problems = make([]ModelProblem, len(fact.Problems))
	for i, p := range fact.Problems {
		p.Code = Code(strings.Clone(string(p.Code)))
		p.Axis = ModelAxis(strings.Clone(string(p.Axis)))
		p.Attribute = QName{strings.Clone(p.Attribute.Space), strings.Clone(p.Attribute.Local)}
		p.Location = cloneLocation(p.Location)
		p.Expected, p.Actual = strings.Clone(p.Expected), strings.Clone(p.Actual)
		owned.Problems[i] = p
	}
	n.modelRepresentation = &owned
	return nil
}

func modelProblemLogicalBytes(p ModelProblem) int64 {
	return 192 + int64(len(p.Code)+len(p.Axis)+len(p.Attribute.Space)+len(p.Attribute.Local)+len(p.Expected)+len(p.Actual)) + locationLogicalBytes(p.Location)
}
func modelUnavailable(state EvidenceState) bool {
	return state == EvidenceUnknown || state == EvidenceUnsupported
}

func (s *Session) validateModelRepresentationEnd(ctx context.Context, n *factNode, end NodeEnd) error {
	if n.modelRepresentation == nil {
		return nil
	}
	for _, problem := range n.modelRepresentation.Problems {
		if err := s.step(ctx, 1); err != nil {
			return err
		}
		switch problem.Axis {
		case ModelAxisScalar:
			if !modelUnavailable(end.Scalar.Value.State) {
				return contract("model representation", "scalar problem contradicts closure value")
			}
		case ModelAxisChildren:
			if !(modelUnavailable(end.ChildrenComplete.State) || end.ChildrenComplete.State == EvidenceKnown && !end.ChildrenComplete.Value) || !modelUnavailable(end.Order.State) {
				return contract("model representation", "children problem contradicts closure enumeration or order")
			}
		}
	}
	return nil
}

func (s *Session) evaluateModelRepresentation(ctx context.Context, n *factNode, r RuleDefinition) error {
	d := Decision{Rule: r.Key, Subject: NodeRef{s.documentID, n.header.ID}, Outcome: OutcomeUnsupported, Reason: "model-representation-unavailable"}
	complete := false
	fact := n.modelRepresentation
	if fact != nil {
		complete = fact.Complete.State == EvidenceKnown && fact.Complete.Value
		switch {
		case len(fact.Problems) != 0:
			d.Outcome, d.Reason = OutcomeFail, "model-representation-invalid"
		case complete:
			d.Outcome, d.Reason = OutcomePass, ""
		default:
			d = evidenceDecision(fact.Complete.State, fact.Complete.Reason, fact.Complete.Cause, false)
			d.Rule, d.Subject = r.Key, NodeRef{s.documentID, n.header.ID}
			if d.Outcome == OutcomePass {
				d.Outcome, d.Reason = OutcomeUnknown, "model-representation-incomplete"
			}
		}
	}
	// Establish authoritative blockers before cache allocation, causal publication,
	// detail work, cancellation or diagnostic storage. Display quotas cannot erase
	// known defects, and incomplete examination remains an independent axis.
	if err := s.accumulator.recordCompositeOutcome(r, d, complete); err != nil {
		return err
	}
	if err := s.cacheDecision(r, n, d); err != nil {
		return err
	}
	if d.Outcome == OutcomeFail {
		if err := s.invalidateModelRepresentation(ctx, n, d); err != nil {
			return err
		}
		for _, problem := range fact.Problems {
			if s.accumulator.report.DiagnosticsTruncated {
				break
			}
			if err := s.step(ctx, 1); err != nil {
				return err
			}
			detail := d
			detail.Reason, detail.Expected, detail.Actual = problem.Code, problem.Expected, problem.Actual
			if err := s.accumulator.storeDiagnostic(r, detail, problem.Location); err != nil {
				return err
			}
		}
		return nil
	}
	return s.accumulator.storeDiagnostic(r, d, n.header.Location)
}

func (s *Session) invalidateModelRepresentation(ctx context.Context, n *factNode, d Decision) error {
	if err := s.retain(int64(128 + len(d.Rule.ID) + len(d.Subject.DocumentID))); err != nil {
		return err
	}
	cause := &DecisionRef{Rule: d.Rule, Subject: d.Subject}
	for _, p := range n.modelRepresentation.Problems {
		if err := s.step(ctx, 1); err != nil {
			return err
		}
		invalid := Evidence[Atomic]{State: EvidenceInvalid, Reason: p.Code, Cause: cause}
		switch p.Axis {
		case ModelAxisScalar:
			n.end.Scalar.Value = invalid
		case ModelAxisAttribute:
			for i := range n.attributes {
				if err := s.step(ctx, 1); err != nil {
					return err
				}
				if n.attributes[i].Name == p.Attribute {
					n.attributes[i].Scalar.Value = invalid
					break
				}
			}
		case ModelAxisChildren:
			n.end.ChildrenComplete = Evidence[bool]{State: EvidenceInvalid, Reason: p.Code, Cause: cause}
			n.end.Order = Evidence[OrderKind]{State: EvidenceInvalid, Reason: p.Code, Cause: cause}
		}
	}
	return nil
}

// Model-only assembly selection can use a proved root assembly without requiring
// schema binding for each descendant. Compiled schemas remain authoritative when
// selected; generic streams must supply exact root declaration/type evidence.
func (s *Session) modelAssemblyDecision(assembly Assembly) Decision {
	active := Assembly("")
	if s.schema != nil && s.schema.active != nil {
		active = s.schema.active.assembly
	} else if s.root != nil {
		h := s.root.header
		if h.Declaration.State == EvidenceKnown {
			active = h.Declaration.Value.Assembly
		}
		if h.Type.State == EvidenceKnown {
			if active != "" && active != h.Type.Value.Assembly {
				return Decision{Outcome: OutcomeUnsupported, Reason: "model-assembly-contradiction"}
			}
			active = h.Type.Value.Assembly
		}
	}
	if active == "" {
		return Decision{Outcome: OutcomeUnsupported, Reason: "model-assembly-unavailable"}
	}
	if active != assembly {
		return Decision{Outcome: OutcomeNotApplicable, Reason: "model-assembly-mismatch"}
	}
	return Decision{Outcome: OutcomePass}
}
