package validation

import (
	"context"
	"strings"
)

// ContextFontFamilyCategories is an ordered parallel classification of the
// comma-separated Raw names: music, text, or specific. It does not assert font
// installation, substitution, glyph availability, or dictionary membership.
const ContextFontFamilyCategories ContextProperty = "local.font-family-categories"

func localFontCategory(name string) string {
	switch strings.Trim(name, " \t\r\n") {
	case "music", "engraved", "handwritten":
		return "music"
	case "text", "serif", "sans-serif", "cursive", "fantasy", "monospace":
		return "text"
	default:
		return "specific"
	}
}

// The production caller keeps the admitted builder live through owned publication.
func localFontCategoriesOwned(ctx context.Context, s *Session, raw Evidence[Atomic]) (Evidence[Atomic], func(), error) {
	if raw.State != EvidenceKnown {
		return raw, nil, nil
	}
	if raw.Value.Kind != AtomicString {
		return stateUnknown[Atomic]("font-family-value-unavailable"), nil, nil
	}
	text := raw.Value.Text
	if err := s.step(ctx, int64(len(text))+1); err != nil {
		return Evidence[Atomic]{}, nil, err
	}
	cost := int64(0)
	for rest := text; ; {
		part, tail, more := strings.Cut(rest, ",")
		var err error
		cost, err = rationalCheckedAdd(BudgetBufferedBytes, cost, int64(len(localFontCategory(part)))+1)
		if err != nil {
			return Evidence[Atomic]{}, nil, err
		}
		if !more {
			break
		}
		rest = tail
	}
	cost--
	if err := s.account.Check(BudgetScalarBytes, cost); err != nil {
		return Evidence[Atomic]{}, nil, err
	}
	reservation, err := rationalCheckedAdd(BudgetBufferedBytes, cost, 256)
	if err != nil {
		return Evidence[Atomic]{}, nil, err
	}
	if err = s.account.Charge(BudgetBufferedBytes, reservation); err != nil {
		return Evidence[Atomic]{}, nil, err
	}
	release := func() { _ = s.account.Release(BudgetBufferedBytes, reservation) }
	size, err := AllocationSize(cost)
	if err != nil {
		release()
		return Evidence[Atomic]{}, nil, err
	}
	var builder strings.Builder
	builder.Grow(size)
	for rest := text; ; {
		part, tail, more := strings.Cut(rest, ",")
		builder.WriteString(localFontCategory(part))
		if !more {
			break
		}
		builder.WriteByte(',')
		rest = tail
	}
	return localKnownText(builder.String()), release, nil
}
