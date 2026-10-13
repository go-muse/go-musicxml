package musicxml

import (
	"errors"
	"io"
)

const sourceXMLExpandedRune rune = -1

func (r *sourceXMLTokenReader) entityReference(name string, context sourceXMLEntityContext, use sourceXMLSpan) (rune, error) {
	if r.dtdStore != nil {
		if declaration, ok := r.dtdStore.Entity(name, false); ok {
			if declaration.Notation != "" {
				return 0, r.syntax("unparsed XML entity cannot be referenced")
			}
			if declaration.External {
				if context == sourceEntityAttribute {
					return 0, r.syntax("external XML entity is forbidden in an attribute value")
				}
				return 0, &sourceXMLUnsupportedError{Feature: "external general entity replacement", Reason: "dtd-external-entity-unread"}
			}
			if r.standalone == sourceDTDStandaloneYes && declaration.Origin.ExternalMarkup {
				return 0, r.syntax("standalone XML entity reference requires an internal declaration")
			}
			err := r.pushEntity(sourceXMLEntityInput{Name: name, Replacement: declaration.Replacement, Declaration: declaration.Origin.Span, DeclarationAvailable: declaration.Origin.SpanAvailable}, context, use)
			return sourceXMLExpandedRune, err
		}
		state := r.dtdStore.State()
		if r.standalone != sourceDTDStandaloneYes && (state.ExternalSubsetUnread || state.ParameterEntityUnread) {
			return 0, &sourceXMLUnsupportedError{Feature: "entity declaration depends on unread DTD content", Reason: "dtd-entity-declaration-unread"}
		}
	}
	return 0, r.syntax("undeclared XML entity " + sourceXMLDiagnosticValue(name))
}

// attributeValue is shared by physical attributes and DTD default values.
// Only a delimiter in the owning input frame closes a physical literal. Quotes
// from nested replacement text remain data. Numeric references append directly;
// replacement literal whitespace takes the XML attribute normalization branch.
func (r *sourceXMLTokenReader) attributeValue(quote rune, owner *sourceXMLEntityFrame, normalize, references bool) (string, sourceXMLPosition, error) {
	b := sourceXMLBuilder{r: r, data: r.valueScratch[:0], borrowed: true}
	for {
		if consumed, err := r.appendASCII(&b, sourceASCIIQuoted, byte(quote)); err != nil {
			return "", sourceXMLPosition{}, err
		} else if consumed {
			continue
		}
		u, err := r.peek()
		if errors.Is(err, errSourceEntityBoundary) {
			if r.entity == owner {
				if quote != 0 {
					return "", sourceXMLPosition{}, r.syntax("entity boundary splits an attribute value")
				}
				value, err := b.string()
				return value, sourceXMLPosition{}, err
			}
			if err = r.finishEntity(sourceEntityAttribute); err != nil {
				return "", sourceXMLPosition{}, err
			}
			continue
		}
		if err != nil {
			if err == io.EOF {
				err = r.syntax("unexpected EOF in attribute value")
			}
			return "", sourceXMLPosition{}, err
		}
		if _, err = r.take(); err != nil {
			return "", sourceXMLPosition{}, err
		}
		if u.Rune == quote && r.entity == owner {
			value, err := b.string()
			return value, u.Start, err
		}
		c := u.Rune
		if c == '<' {
			return "", sourceXMLPosition{}, r.syntax("unescaped < inside quoted string")
		}
		if c == '&' {
			if !references {
				return "", sourceXMLPosition{}, r.syntax("references are not allowed in an XML declaration")
			}
			c, err = r.reference(sourceEntityAttribute, u.Start)
			if err != nil {
				return "", sourceXMLPosition{}, err
			}
			if c == sourceXMLExpandedRune {
				continue
			}
		} else if normalize && sourceXMLSpace(c) {
			c = ' '
		}
		if err = b.append(c); err != nil {
			return "", sourceXMLPosition{}, err
		}
	}
}
func (r *sourceXMLTokenReader) dtdAttributeValue(raw string, declaration sourceDTDOrigin) (string, error) {
	input := sourceXMLEntityInput{Replacement: raw, Declaration: declaration.Span, DeclarationAvailable: declaration.SpanAvailable}
	if err := r.pushEntity(input, sourceEntityAttribute, r.metadata.Span); err != nil {
		return "", err
	}
	owner := r.entity
	value, _, err := r.attributeValue(0, owner, true, true)
	if err == nil {
		err = r.finishEntity(sourceEntityAttribute)
	}
	return value, err
}
