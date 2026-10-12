package validation

import (
	"context"
	"math/big"
	"strconv"
)

type contextLocalService struct{ Final bool }
type localRuleRoles struct {
	element, attribute, child contextRole
	property                  ContextProperty
}

var localRuleBindings = func() []localRuleRoles {
	out := make([]localRuleRoles, len(localConstantRules))
	for i, r := range localConstantRules {
		prefix := "local."
		if r.Layout {
			prefix = "layout."
		}
		base := "local.constant." + strconv.Itoa(i)
		out[i] = localRuleRoles{contextRole(base + ".element"), contextRole(base + ".attribute"), contextRole(base + ".child"), ContextProperty(prefix + r.Property)}
	}
	return out
}()
var localByteIntegers = func() [256]Integer {
	var out [256]Integer
	for i := range out {
		out[i], _ = NewInteger(big.NewInt(int64(i)))
	}
	return out
}()
var localOne = localByteIntegers[1]

func localConstantAtomic(rule localConstantRule) Atomic {
	if rule.Kind == AtomicInteger {
		return Atomic{Kind: AtomicInteger, Integer: localOne}
	}
	return Atomic{Kind: AtomicString, Text: rule.Default}
}
func localConstantEffective(rule localConstantRule, raw Evidence[Atomic]) Evidence[Atomic] {
	if raw.State == EvidenceAbsent || (rule.Empty && raw.State == EvidenceKnown && raw.Value.Kind == AtomicString && raw.Value.Text == "") {
		if rule.Unknown != "" {
			return Evidence[Atomic]{State: EvidenceUnknown, Reason: Code(rule.Unknown)}
		}
		return Evidence[Atomic]{State: EvidenceKnown, Value: localConstantAtomic(rule)}
	}
	return raw
}

// localPublish records a direct input observation, including observed absence.
// It does not assert that an effective value exists or a default was applied.
func localPublish(ctx context.Context, s *Session, key RuleKey, n *factNode, property ContextProperty, raw, value Evidence[Atomic], active Evidence[bool], scope NodeRef, unit ContextUnit) error {
	return localPublishWithProvenance(ctx, s, key, n, property, raw, value, active, scope, unit, ContextExplicit)
}

func localPublishWithProvenance(ctx context.Context, s *Session, key RuleKey, n *factNode, property ContextProperty, raw, value Evidence[Atomic], active Evidence[bool], scope NodeRef, unit ContextUnit, provenance ContextProvenance) error {
	ref := NodeRef{DocumentID: s.documentID, Node: n.header.ID}
	if scope.Node == 0 {
		scope = ref
	}
	return s.publishContextResult(ctx, ContextResult{Provider: key, Subject: ref, Property: property, Value: value, Origins: []Location{n.header.Location}, Layout: &LayoutContextResult{Part: Evidence[NodeRef]{State: EvidenceAbsent}, Page: Evidence[NodeRef]{State: EvidenceAbsent}, System: Evidence[NodeRef]{State: EvidenceAbsent}, Staff: Evidence[Integer]{State: EvidenceAbsent}, Conversion: Evidence[Rational]{State: EvidenceAbsent}, Scope: scope, Origin: ref, Raw: raw, Applicability: active, Unit: unit, Provenance: provenance}})
}
func localPublishConstant(ctx context.Context, s *Session, key RuleKey, n *factNode, r *contextReader, i int) error {
	spec := localConstantRules[i]
	roles := localRuleBindings[i]
	if spec.Element != "" {
		if !r.matches(n, roles.element) {
			return r.err
		}
	} else if !r.attributeOwner(n, roles.attribute) {
		return r.err
	}
	var raw Evidence[Atomic]
	switch {
	case spec.Attribute != "":
		raw = localRawAttribute(r, n, roles.attribute)
	case spec.Child != "":
		raw = r.childScalar(n, roles.child)
	default:
		raw = r.scalar(n)
	}
	if r.err != nil {
		return r.err
	}
	value := localConstantEffective(spec, raw)
	return localPublishWithProvenance(ctx, s, key, n, roles.property, raw, value, Evidence[bool]{State: EvidenceKnown, Value: true}, NodeRef{}, ContextUnitNone, localConstantProvenance(spec, raw))
}

// localRawAttribute preserves physical omission before applying reviewed
// prose defaults. The shared scalar assessment still owns malformed values.
func localRawAttribute(r *contextReader, n *factNode, role contextRole) Evidence[Atomic] {
	value := r.attribute(n, role)
	presence := r.presence(n, role)
	if value.State == EvidenceInvalid {
		return value
	}
	if presence.State == EvidenceAbsent || (presence.State == EvidenceKnown && !presence.Value) {
		return Evidence[Atomic]{State: EvidenceAbsent}
	}
	if presence.State != EvidenceKnown {
		return stateUnavailable[bool, Atomic](presence)
	}
	return value
}

// localPublishAtOrigin records a genuinely inherited or child-selected fact.
// Raw absence remains visible; it does not mislabel an encoded override as a
// documented default merely because the subject has no scalar of its own.
func localPublishAtOrigin(ctx context.Context, s *Session, key RuleKey, n, origin *factNode, property ContextProperty, raw, value Evidence[Atomic], active Evidence[bool], provenance ContextProvenance) error {
	ref := NodeRef{s.documentID, n.header.ID}
	from := NodeRef{s.documentID, origin.header.ID}
	return s.publishContextResult(ctx, ContextResult{Provider: key, Subject: ref, Property: property, Value: value, Origins: []Location{n.header.Location, origin.header.Location}, Layout: &LayoutContextResult{Part: Evidence[NodeRef]{State: EvidenceAbsent}, Page: Evidence[NodeRef]{State: EvidenceAbsent}, System: Evidence[NodeRef]{State: EvidenceAbsent}, Staff: Evidence[Integer]{State: EvidenceAbsent}, Conversion: Evidence[Rational]{State: EvidenceAbsent}, Scope: ref, Origin: from, Raw: raw, Applicability: active, Unit: ContextUnitNone, Provenance: provenance}})
}

func localConstantProvenance(spec localConstantRule, raw Evidence[Atomic]) ContextProvenance {
	if raw.State == EvidenceAbsent || (spec.Empty && raw.State == EvidenceKnown && raw.Value.Kind == AtomicString && raw.Value.Text == "") {
		if spec.Unknown != "" {
			return ContextApplicationChoice
		}
		return ContextDocumentedDefault
	}
	return ContextExplicit
}

// Callers identify the source-defined absence branch; unavailable evidence is
// never inferred to be an application choice or documented default.
func localAbsentProvenance(raw Evidence[Atomic], absent ContextProvenance) ContextProvenance {
	if raw.State == EvidenceAbsent {
		return absent
	}
	return ContextExplicit
}
