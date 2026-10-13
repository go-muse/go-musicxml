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
	scratch := int64(len(value))*4 + 1024
	if err := account.Charge(BudgetBufferedBytes, scratch); err != nil {
		return Evidence[ResourceRepresentation]{}, err
	}
	defer func() { _ = account.Release(BudgetBufferedBytes, scratch) }()
	typ, parameters, err := mime.ParseMediaType(value)
	if err := ctx.Err(); err != nil {
		return Evidence[ResourceRepresentation]{}, err
	}
	if err != nil {
		return unknown("media-type-syntax-unresolved"), nil
	}
	typ = strings.ToLower(typ)
	// IANA registers charset only for the +xml type; the compressed type has
	// no optional parameters. Unreviewed extensions remain unresolved here.
	// https://www.iana.org/assignments/media-types/application/vnd.recordare.musicxml+xml
	// https://www.iana.org/assignments/media-types/application/vnd.recordare.musicxml
	if typ == "application/vnd.recordare.musicxml+xml" || typ == "application/vnd.recordare.musicxml" {
		for key := range parameters {
			if err := ctx.Err(); err != nil {
				return Evidence[ResourceRepresentation]{}, err
			}
			if err := account.Charge(BudgetRuleSteps, 1); err != nil {
				return Evidence[ResourceRepresentation]{}, err
			}
			if typ != "application/vnd.recordare.musicxml+xml" || key != "charset" {
				return unknown("musicxml-media-parameter-unresolved"), nil
			}
		}
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
