package validation

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

type fragmentPointerPart struct{ Name, Data string }

// resolveXMLFragment consumes the once URI-decoded component. The Framework
// performs only its circumflex escaping, never another percent/XML decode.
func (s *PackageSession) resolveXMLFragment(ctx context.Context, snap *ResourceSnapshot, fragment string) (Evidence[NodeRef], error) {
	f := snap.fragments
	if f == nil {
		return stateUnknown[NodeRef]("fragment-authority-unselected"), nil
	}
	if len(fragment) > 4096 {
		return Evidence[NodeRef]{}, s.fail(contract("XML fragment", "bounded Unicode fragment required"))
	}
	if err := s.account.Check(BudgetScalarBytes, int64(len(fragment))); err != nil {
		return Evidence[NodeRef]{}, s.fail(err)
	}
	if err := s.scan(ctx, 1+5*int64(len(fragment))); err != nil {
		return Evidence[NodeRef]{}, err
	}
	if !utf8.ValidString(fragment) {
		return Evidence[NodeRef]{}, s.fail(contract("XML fragment", "Unicode fragment required"))
	}
	// A bounded pointer is at most 4096 bytes. The prepaid envelope covers
	// overlapping pointer-part slice growth (two string views per part), decoded
	// scheme-data scratch plus copied strings, fixed parser state, and the
	// underlying XML-name regex VM allowance (256 + 16*input bytes). Unsupported
	// scheme data remains borrowed; no recursive grammar or unbounded integer is
	// constructed. Every consumed byte also charges cancellable scan work.
	temporary := int64(len(fragment))*80 + 1024
	if err := s.account.Charge(BudgetBufferedBytes, temporary); err != nil {
		return Evidence[NodeRef]{}, s.fail(err)
	}
	defer s.account.Release(BudgetBufferedBytes, temporary)
	if xsd.ValidXSDNCName(fragment) {
		return s.resolveFragmentID(ctx, snap, fragment)
	}
	if f.Mode != fragmentFramework {
		return Evidence[NodeRef]{State: EvidenceUnsupported, Reason: "fragment-grammar-unimplemented"}, nil
	}
	parts, valid, err := s.fragmentFrameworkParts(ctx, fragment)
	if err != nil {
		return Evidence[NodeRef]{}, err
	}
	if !valid {
		return Evidence[NodeRef]{State: EvidenceAbsent, Reason: "fragment-framework-malformed"}, nil
	}
	supported := false
	for _, part := range parts {
		if err := s.scan(ctx, 1); err != nil {
			return Evidence[NodeRef]{}, err
		}
		// No namespace-binding scheme is elected. The Framework initial xml
		// binding does not name element(); qualified schemes remain unsupported.
		if part.Name != "element" {
			continue
		}
		supported = true
		data, err := s.fragmentUnescape(ctx, part.Data)
		if err != nil {
			return Evidence[NodeRef]{}, err
		}
		result, err := s.fragmentElement(ctx, snap, data)
		if err != nil {
			return Evidence[NodeRef]{}, err
		}
		if result.State != EvidenceAbsent {
			return result, nil
		}
	}
	if !supported {
		return Evidence[NodeRef]{State: EvidenceUnsupported, Reason: "fragment-schemes-unimplemented"}, nil
	}
	return Evidence[NodeRef]{State: EvidenceAbsent, Reason: "fragment-pointer-empty"}, nil
}

// Validate the entire Framework production before evaluating any scheme. Its
// grammar permits inter-part XML whitespace, but no leading/trailing whitespace.
// A malformed supported scheme's *data* is handled later as an empty part.
// https://www.w3.org/TR/2003/REC-xptr-framework-20030325/#syntax
func (s *PackageSession) fragmentFrameworkParts(ctx context.Context, pointer string) ([]fragmentPointerPart, bool, error) {
	var parts []fragmentPointerPart
	at := 0
	for at < len(pointer) {
		start := at
		for at < len(pointer) && pointer[at] != '(' {
			if err := s.scan(ctx, 1); err != nil {
				return nil, false, err
			}
			at++
		}
		if at == len(pointer) {
			return nil, false, nil
		}
		name := pointer[start:at]
		if err := s.scan(ctx, 1+5*int64(len(name))); err != nil {
			return nil, false, err
		}
		if !xsd.ValidXSDQName(name) {
			return nil, false, nil
		}
		at++
		start = at
		depth := 1
		for at < len(pointer) && depth != 0 {
			if err := s.scan(ctx, 1); err != nil {
				return nil, false, err
			}
			switch pointer[at] {
			case '^':
				at++
				if at == len(pointer) || pointer[at] != '(' && pointer[at] != ')' && pointer[at] != '^' {
					return nil, false, nil
				}
				if err := s.scan(ctx, 1); err != nil {
					return nil, false, err
				}
			case '(':
				depth++
			case ')':
				depth--
			}
			at++
		}
		if depth != 0 {
			return nil, false, nil
		}
		parts = append(parts, fragmentPointerPart{Name: name, Data: pointer[start : at-1]})
		if at == len(pointer) {
			return parts, true, nil
		}
		skipped := false
		for at < len(pointer) && fragmentXMLSpace(pointer[at]) {
			if err := s.scan(ctx, 1); err != nil {
				return nil, false, err
			}
			at++
			skipped = true
		}
		if skipped && at == len(pointer) {
			return nil, false, nil
		}
	}
	return parts, len(parts) != 0, nil
}
func fragmentXMLSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\r' || b == '\n' }
func (s *PackageSession) fragmentUnescape(ctx context.Context, raw string) (string, error) {
	if err := s.scan(ctx, int64(len(raw))); err != nil {
		return "", err
	}
	if !strings.ContainsRune(raw, '^') {
		return raw, nil
	}
	out := make([]byte, 0, len(raw)) // admitted by resolveXMLFragment's live lease
	for at := 0; at < len(raw); at++ {
		if err := s.scan(ctx, 1); err != nil {
			return "", err
		}
		if raw[at] == '^' {
			at++
			if err := s.scan(ctx, 1); err != nil {
				return "", err
			}
		}
		out = append(out, raw[at])
	}
	if err := s.scan(ctx, int64(len(out))); err != nil {
		return "", err
	}
	return string(out), nil
}

// Invalid element() data is an empty part, not a Framework syntax error.
// https://www.w3.org/TR/2003/REC-xptr-element-20030325/#language
func (s *PackageSession) fragmentElement(ctx context.Context, snap *ResourceSnapshot, data string) (Evidence[NodeRef], error) {
	empty := Evidence[NodeRef]{State: EvidenceAbsent}
	if data == "" {
		return empty, nil
	}
	if err := s.scan(ctx, 1+5*int64(len(data))); err != nil {
		return Evidence[NodeRef]{}, err
	}
	anchor, sequence, hasPath := strings.Cut(data, "/")
	if anchor != "" && !xsd.ValidXSDNCName(anchor) {
		return empty, nil
	}
	if anchor == "" && !hasPath {
		return empty, nil
	}
	// Parse/validate the complete path before consulting any potentially unknown
	// ID/tree evidence. A malformed element part is definitively empty and can
	// fall back even when its invalid anchor/path would otherwise be unavailable.
	if hasPath {
		if sequence == "" {
			return empty, nil
		}
		first := true
		for i := 0; i < len(sequence); i++ {
			if err := s.scan(ctx, 1); err != nil {
				return Evidence[NodeRef]{}, err
			}
			b := sequence[i]
			if b == '/' {
				if first {
					return empty, nil
				}
				first = true
				continue
			}
			if b < '0' || b > '9' || first && b == '0' {
				return empty, nil
			}
			first = false
		}
		if first {
			return empty, nil
		}
	}
	var owner NodeRef
	if anchor != "" {
		selected, err := s.resolveFragmentID(ctx, snap, anchor)
		if err != nil {
			return Evidence[NodeRef]{}, err
		}
		if selected.State != EvidenceKnown {
			return selected, nil
		}
		owner = selected.Value
		if !hasPath {
			return selected, nil
		}
	} else {
		owner = NodeRef{DocumentID: snap.header.DocumentID}
	}
	if !knownComplete(snap.fragments.Complete) {
		return stateUnknown[NodeRef]("fragment-element-order-incomplete"), nil
	}
	if err := s.scan(ctx, int64(len(sequence))); err != nil {
		return Evidence[NodeRef]{}, err
	}
	for len(sequence) != 0 {
		digits, rest, more := strings.Cut(sequence, "/")
		// Saturate above the finite admitted element count. Even a thousands-digit
		// legal ordinal proves out of range in a complete tree without truncation or
		// arbitrary-precision allocation. Digit validity was already checked above.
		limit := uint64(len(snap.fragments.Elements)) + 1
		ordinal := uint64(0)
		for i := 0; i < len(digits); i++ {
			if err := s.scan(ctx, 1); err != nil {
				return Evidence[NodeRef]{}, err
			}
			digit := uint64(digits[i] - '0')
			if ordinal > limit/10 || ordinal == limit/10 && digit > limit%10 {
				ordinal = limit
			} else {
				ordinal = ordinal*10 + digit
			}
		}
		count := uint64(0)
		next := NodeRef{}
		for _, element := range snap.fragments.Elements {
			if err := s.scan(ctx, 1); err != nil {
				return Evidence[NodeRef]{}, err
			}
			if element.Parent == owner {
				count++
				if count == ordinal {
					next = element.Subject
					break
				}
			}
		}
		if next.Node == 0 {
			return empty, nil
		}
		owner = next
		if !more {
			break
		}
		sequence = rest
	}
	return fragmentKnown(owner), nil
}
