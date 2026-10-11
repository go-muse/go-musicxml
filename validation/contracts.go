package validation

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxIdentityBytes = 1024

func contract(field, reason string) error { return &ConfigurationError{Field: field, Reason: reason} }
func validText(s string, max int) bool {
	return len(s) <= max && utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}
func validID(s string) bool {
	return s != "" && validText(s, maxIdentityBytes) && strings.IndexFunc(s, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) < 0
}
func validTarget(v Target) bool { return v == TargetSource || v == TargetModel || v == TargetPackage }
func validAssembly(v Assembly) bool {
	return v == AssemblyMusicXML || v == AssemblyOpus || v == AssemblySounds || v == AssemblyContainer
}
func validCategory(v DiagnosticCategory) bool {
	switch v {
	case CategoryNormative, CategoryModel, CategoryCapability, CategoryResource, CategorySyntax, CategoryAdvisory, CategoryPolicy:
		return true
	}
	return false
}
func validRuleKey(v RuleKey) bool { return validID(string(v.ID)) && v.Revision > 0 }
func validQName(v QName) bool {
	return validID(v.Local) && !strings.ContainsAny(v.Local, "/:<>&\"'=") && validText(v.Space, 4096)
}
func validDeclaration(v DeclarationID) bool {
	return validAssembly(v.Assembly) && validID(v.OwnerOccurrence) && validID(v.Occurrence)
}
func validComponent(v ComponentID) bool {
	if !validAssembly(v.Assembly) {
		return false
	}
	switch v.Kind {
	case ComponentElement, ComponentType, ComponentGroup, ComponentAttribute:
	default:
		return false
	}
	return (validQName(v.Name) && v.Occurrence == "") || (v.Name == (QName{}) && validID(v.Occurrence))
}
func validRef(v DecisionRef) bool {
	return validRuleKey(v.Rule) && validID(v.Subject.DocumentID) && v.Subject.Node != 0
}
func validateEvidence[T any](v Evidence[T], field string) error {
	switch v.State {
	case EvidenceKnown:
		if v.Cause != nil {
			return contract(field, "known evidence cannot have invalid cause")
		}
	case EvidenceAbsent:
		if v.Cause != nil {
			return contract(field, "absent evidence cannot have invalid cause")
		}
	case EvidenceUnknown, EvidenceUnsupported:
		if !validID(string(v.Reason)) || v.Cause != nil {
			return contract(field, "unavailable evidence requires reason and no invalid cause")
		}
	case EvidenceInvalid:
		if !validID(string(v.Reason)) || v.Cause == nil || !validRef(*v.Cause) {
			return contract(field, "invalid evidence requires exact causal rule/subject")
		}
	default:
		return contract(field, "unknown evidence state")
	}
	if v.State != EvidenceKnown {
		var zero T
		if !reflect.DeepEqual(v.Value, zero) {
			return contract(field, "non-known evidence carries a value")
		}
	}
	if !validText(string(v.Reason), maxIdentityBytes) {
		return contract(field, "invalid reason")
	}
	return nil
}
func validateAtomic(v Atomic) error {
	emptyInt := !v.Integer.Valid()
	emptyDecimal := !v.Decimal.Valid()
	emptyName := v.Name == (QName{})
	switch v.Kind {
	case AtomicString:
		if !emptyInt || !emptyDecimal || !emptyName || !utf8.ValidString(v.Text) {
			return contract("atomic", "invalid string payload")
		}
	case AtomicBoolean:
		if !emptyInt || !emptyDecimal || !emptyName || (v.Text != "true" && v.Text != "false") {
			return contract("atomic", "invalid boolean payload")
		}
	case AtomicInteger:
		if emptyInt || !emptyDecimal || !emptyName || v.Text != "" {
			return contract("atomic", "invalid integer payload")
		}
	case AtomicDecimal:
		if !emptyInt || emptyDecimal || !emptyName || v.Text != "" {
			return contract("atomic", "invalid decimal payload")
		}
	case AtomicQName:
		if !emptyInt || !emptyDecimal || !validQName(v.Name) || v.Text != "" {
			return contract("atomic", "invalid QName payload")
		}
	default:
		return contract("atomic", "unknown kind")
	}
	return nil
}
func validateScalar(v Scalar) error {
	if err := validateEvidence(v.Lexeme, "scalar lexeme"); err != nil {
		return err
	}
	if err := validateEvidence(v.Value, "scalar value"); err != nil {
		return err
	}
	if v.Lexeme.State == EvidenceKnown && !utf8.ValidString(v.Lexeme.Value) {
		return contract("scalar lexeme", "invalid UTF-8")
	}
	if v.Value.State == EvidenceKnown {
		return validateAtomic(v.Value.Value)
	}
	return nil
}
func validateLocation(v Location, target Target, doc string) error {
	if v.Target != target || v.DocumentID != doc || !validID(doc) {
		return contract("location", "target/document mismatch")
	}
	if !validText(v.Path, 16384) || !validText(v.Entry, 4096) {
		return contract("location", "invalid or excessive path")
	}
	if p := v.Position; p != nil {
		if target != TargetSource {
			return contract("location", "only source facts may have source coordinates")
		}
		if p.ByteOffset < 0 || p.Line < 0 || p.Column < 0 || (p.Line == 0) != (p.Column == 0) || !validID(p.CoordinateSpace) {
			return contract("location", "invalid coordinate")
		}
	}
	return nil
}
func validateHeader(v NodeHeader, target Target, doc string) error {
	if v.ID == 0 || !validQName(v.Name) {
		return contract("node", "invalid ID/name")
	}
	if err := validateLocation(v.Location, target, doc); err != nil {
		return err
	}
	if err := validateEvidence(v.Type, "node type"); err != nil {
		return err
	}
	if v.Type.State == EvidenceKnown && (!validComponent(v.Type.Value) || v.Type.Value.Kind != ComponentType) {
		return contract("node type", "not an exact type component")
	}
	if err := validateEvidence(v.Declaration, "node declaration"); err != nil {
		return err
	}
	if v.Declaration.State == EvidenceKnown && !validDeclaration(v.Declaration.Value) {
		return contract("node declaration", "invalid identity")
	}
	if len(v.Bindings) > 65536 {
		return contract("bindings", "too many bindings")
	}
	seen := make(map[string]bool, len(v.Bindings))
	for _, b := range v.Bindings {
		if !validText(b.Prefix, 1024) || !validText(b.URI, 4096) || seen[b.Prefix] {
			return contract("bindings", "invalid or duplicate prefix")
		}
		seen[b.Prefix] = true
	}
	return nil
}
func validateAttribute(v AttributeFact, target Target, doc string) error {
	if v.Owner == 0 || !validQName(v.Name) {
		return contract("attribute", "invalid owner/name")
	}
	if err := validateLocation(v.Location, target, doc); err != nil {
		return err
	}
	if err := validateEvidence(v.Declaration, "attribute declaration"); err != nil {
		return err
	}
	if v.Declaration.State == EvidenceKnown && !validDeclaration(v.Declaration.Value) {
		return contract("attribute declaration", "invalid identity")
	}
	if err := validateEvidence(v.Presence, "attribute presence"); err != nil {
		return err
	}
	if v.Presence.State == EvidenceKnown && !v.Presence.Value {
		return contract("attribute presence", "use absent evidence for proven absence")
	}
	if v.Presence.State == EvidenceAbsent && (v.Scalar.Lexeme.State == EvidenceKnown || v.Scalar.Value.State == EvidenceKnown) {
		return contract("attribute presence", "proven absence cannot carry known scalar evidence")
	}
	return validateScalar(v.Scalar)
}
func validateEnd(v NodeEnd, target Target) error {
	if v.ID == 0 {
		return contract("close", "zero node")
	}
	if err := validateScalar(v.Scalar); err != nil {
		return err
	}
	if err := validateEvidence(v.ChildrenComplete, "children completeness"); err != nil {
		return err
	}
	if err := validateEvidence(v.AttributesComplete, "attribute completeness"); err != nil {
		return err
	}
	if v.ChildrenComplete.State == EvidenceAbsent || v.AttributesComplete.State == EvidenceAbsent {
		return contract("completeness", "absence is not a completeness state")
	}
	if err := validateEvidence(v.Order, "order"); err != nil {
		return err
	}
	if v.Order.State == EvidenceKnown {
		switch v.Order.Value {
		case OrderSource:
			if target != TargetSource {
				return contract("order", "source order on non-source target")
			}
		case OrderModelContent, OrderModelFields:
			if target != TargetModel {
				return contract("order", "model order on non-model target")
			}
		default:
			return contract("order", fmt.Sprintf("unknown order %q", v.Order.Value))
		}
	}
	return nil
}
