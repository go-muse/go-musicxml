package validation

import (
	"context"
	"mime"
	"strings"
)

// ClassifyResourceMediaType classifies a declared MIME value for bounded MusicXML
// applicability. A Known representation describes the declaration, never observed
// content, a verified pin, or permission to retrieve a resource. Callers preserve
// absent/invalid evidence separately; a present empty value remains unresolved.
// The shared account pays for scalar/step work and transient parser scratch. The
// input is bounded to 4096 bytes. Cancellation and quota errors are returned as
// errors, not converted into Unknown evidence.
func ClassifyResourceMediaType(ctx context.Context, account *Account, value string) (Evidence[ResourceRepresentation], error) {
	unknown := func(reason Code) Evidence[ResourceRepresentation] {
		return Evidence[ResourceRepresentation]{State: EvidenceUnknown, Reason: reason}
	}
	if ctx == nil {
		return Evidence[ResourceRepresentation]{}, contract("context", "must not be nil")
	}
	if err := ctx.Err(); err != nil {
		return Evidence[ResourceRepresentation]{}, err
	}
	if err := account.Check(BudgetScalarBytes, int64(len(value))); err != nil {
		return Evidence[ResourceRepresentation]{}, err
	}
	if len(value) > 4096 {
		return Evidence[ResourceRepresentation]{}, contract("media type", "maximum 4096 bytes")
	}
	if err := account.Charge(BudgetRuleSteps, int64(len(value))+1); err != nil {
		return Evidence[ResourceRepresentation]{}, err
	}
	// Bound both the standard parser map and the raw-name duplicate index,
	// including short parameter names, decoded strings and overlapping storage.
	scratch := int64(len(value))*128 + 4096
	if err := account.Charge(BudgetBufferedBytes, scratch); err != nil {
		return Evidence[ResourceRepresentation]{}, err
	}
	defer func() { _ = account.Release(BudgetBufferedBytes, scratch) }()
	typ, _, err := mime.ParseMediaType(value)
	if err := ctx.Err(); err != nil {
		return Evidence[ResourceRepresentation]{}, err
	}
	if err != nil {
		return unknown("media-type-syntax-unresolved"), nil
	}
	parameterReason, err := resourceMediaParameterStatus(ctx, account, value)
	if err != nil {
		return Evidence[ResourceRepresentation]{}, err
	}
	if parameterReason != "" {
		return unknown(parameterReason), nil
	}
	typ = strings.ToLower(typ)
	// RFC 2045 sections 5 and 5.1: parameters do not change the base content
	// nature, and unrecognized parameter names are ignored. This classifies
	// declaration identity only; it neither selects decoding nor grants I/O.
	if typ == "application/vnd.recordare.musicxml+xml" || typ == "application/vnd.recordare.musicxml" {
		representation := ResourceSourceXML
		if typ == "application/vnd.recordare.musicxml" {
			representation = ResourceRawMXL
		}
		return Evidence[ResourceRepresentation]{State: EvidenceKnown, Value: representation}, nil
	}
	if typ == "application/pdf" || strings.HasPrefix(typ, "image/") || strings.HasPrefix(typ, "audio/") || strings.HasPrefix(typ, "video/") {
		return Evidence[ResourceRepresentation]{State: EvidenceKnown, Value: ResourceBinary}, nil
	}
	return unknown("musicxml-media-type-set-unresolved"), nil
}

// The standard parser validates syntax first, but collapses repeated names with
// equal values. RFC 6838 section 4.3 prohibits repetitions regardless of value.
// Scan raw parameter names so that quoted semicolons/escapes cannot hide a
// duplicate. All maps/strings are transient under the caller's scratch lease.
func resourceMediaParameterStatus(ctx context.Context, account *Account, value string) (Code, error) {
	// Prepay bounded scans, case folding, name hashing and map operations.
	if err := account.Charge(BudgetRuleSteps, int64(len(value))*8+1); err != nil {
		return "", err
	}
	// This bounded declaration classifier does not accept internationalized
	// header syntax or raw control bytes through the permissive Go parser.
	for i := range len(value) {
		if i%64 == 0 {
			if err := ctx.Err(); err != nil {
				return "", err
			}
		}
		if value[i] >= 127 || value[i] < 32 && value[i] != '\t' {
			return "media-type-syntax-unresolved", nil
		}
	}
	seen := make(map[string]struct{})
	quoted := false
	for i := 0; i < len(value); i++ {
		if i%64 == 0 {
			if err := ctx.Err(); err != nil {
				return "", err
			}
		}
		if quoted && value[i] == '\\' {
			i++
			continue
		}
		if value[i] == '"' {
			quoted = !quoted
			continue
		}
		if value[i] != ';' || quoted {
			continue
		}
		start, end := i+1, i+1
		for end < len(value) && value[end] != '=' {
			if end%64 == 0 {
				if err := ctx.Err(); err != nil {
					return "", err
				}
			}
			end++
		}
		if end == len(value) {
			return "media-type-syntax-unresolved", nil
		}
		name := strings.ToLower(strings.TrimSpace(value[start:end]))
		if _, exists := seen[name]; exists {
			return "media-type-duplicate-parameter", nil
		}
		seen[name] = struct{}{}
		i = end
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "", nil
}
