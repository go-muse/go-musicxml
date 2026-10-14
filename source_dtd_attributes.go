package musicxml

import (
	"encoding/xml"

	"github.com/go-muse/go-musicxml/validation"
)

// applyDTDAttributes runs before namespace expansion. Declaration keys and
// suppression use lexical qualified names, not their eventual expanded names.
func (r *sourceXMLTokenReader) applyDTDAttributes(element string, attributes []xml.Attr, metadata []sourceXMLAttribute) ([]xml.Attr, []sourceXMLAttribute, error) {
	if !r.internalDTD || r.dtdStore == nil {
		return attributes, metadata, nil
	}
	if len(attributes) != len(metadata) {
		return nil, nil, &validation.SessionError{Operation: "DTD attributes", Reason: "attribute metadata mismatch"}
	}
	declared := r.dtdStore.Attributes(element)
	declarationsComplete := r.dtdStore.State().AttributeDeclarationsComplete
	if len(declared) == 0 && declarationsComplete {
		return attributes, metadata, nil
	}
	// Keys borrow the already owned attribute strings. Per-entry allowance also
	// covers map growth while the temporary index lives until token advance.
	if int64(len(attributes)) > (int64(^uint64(0)>>1)-64)/128 {
		return nil, nil, &validation.ResourceError{Kind: validation.BudgetBufferedBytes, Requested: int64(len(attributes))}
	}
	if err := r.tokenReserve(64 + 128*int64(len(attributes))); err != nil {
		return nil, nil, err
	}
	present := make(map[xml.Name]bool, len(attributes))
	for i := range attributes {
		if err := r.cursor.contextErr(); err != nil {
			return nil, nil, err
		}
		if err := r.event(); err != nil {
			return nil, nil, err
		}
		name := attributes[i].Name
		present[name] = true
		lexical := name.Local
		if name.Space != "" {
			if err := r.tokenReserve(int64(len(name.Space)) + 1 + int64(len(name.Local))); err != nil {
				return nil, nil, err
			}
			lexical = name.Space + ":" + name.Local
		}
		declaration, found := r.dtdStore.Attribute(element, lexical)
		if !found {
			if !declarationsComplete && metadata[i].Origin != nil {
				// Missing from the read store is not known absence when unread
				// declarations may still determine this physical attribute.
				metadata[i].Origin.DeclarationExpected = true
				metadata[i].Origin.DeclarationUnavailableReason = "dtd-infoset-unavailable"
			}
			continue
		}
		if metadata[i].Origin != nil {
			metadata[i].Origin.DeclarationExpected = true
			metadata[i].Origin.Declaration = declaration.Origin.Span
			metadata[i].Origin.DeclarationAvailable = declaration.Origin.SpanAvailable
		}
		if declaration.AttributeType != "CDATA" {
			value, err := r.normalizeDTDTokenized(attributes[i].Value)
			if err != nil {
				return nil, nil, err
			}
			attributes[i].Value = value
		}
	}
	for _, declaration := range declared {
		if err := r.cursor.contextErr(); err != nil {
			return nil, nil, err
		}
		if err := r.event(); err != nil {
			return nil, nil, err
		}
		if declaration.DefaultKind != sourceDTDDefaultValue && declaration.DefaultKind != sourceDTDDefaultFixed {
			continue
		}
		name, valid := sourceXMLQName(declaration.Name)
		if !valid {
			return nil, nil, r.syntax("invalid qualified DTD attribute name")
		}
		if present[name] {
			continue
		}
		used, err := r.account.Usage(validation.BudgetAttributes)
		if err != nil {
			return nil, nil, err
		}
		limit := r.account.Limits().MaxAttributes
		if int64(len(attributes)) >= limit-used {
			return nil, nil, &validation.ResourceError{Kind: validation.BudgetAttributes, Limit: limit, Used: used, Requested: int64(len(attributes)) + 1}
		}
		value, err := r.dtdAttributeValue(declaration.RawValue, declaration.Origin)
		if err != nil {
			return nil, nil, err
		}
		if declaration.AttributeType != "CDATA" {
			value, err = r.normalizeDTDTokenized(value)
			if err != nil {
				return nil, nil, err
			}
		}
		if len(attributes) == cap(attributes) {
			capacity := max(int64(4), 2*int64(cap(attributes)), int64(len(attributes))+1)
			capacity = min(capacity, limit)
			count, err := validation.AllocationSize(capacity)
			if err != nil || capacity > int64(^uint64(0)>>1)/256 {
				if err == nil {
					err = &validation.ResourceError{Kind: validation.BudgetBufferedBytes, Requested: capacity}
				}
				return nil, nil, err
			}
			if err := r.tokenReserve(capacity * 256); err != nil {
				return nil, nil, err
			}
			next, nextMeta := make([]xml.Attr, len(attributes), count), make([]sourceXMLAttribute, len(metadata), count)
			copy(next, attributes)
			copy(nextMeta, metadata)
			old := int64(cap(attributes)) * 256
			attributes, metadata = next, nextMeta
			_ = r.account.Release(validation.BudgetBufferedBytes, old)
			r.tokenOwned -= old
		}
		if err := r.tokenReserve(256); err != nil {
			return nil, nil, err
		}
		origin := &sourceXMLOrigin{Kind: sourceOriginDefault, Specified: false, DeclarationExpected: true, UseExpected: true, Declaration: declaration.Origin.Span, DeclarationAvailable: declaration.Origin.SpanAvailable, AncestryComplete: true}
		if r.metadata.Origin != nil {
			origin.UseAvailable = r.metadata.Origin.PhysicalAvailable
			if origin.UseAvailable {
				origin.Use = sourceXMLSpan{Start: r.metadata.Span.Start, End: r.position()}
			}
		}
		attributes = append(attributes, xml.Attr{Name: name, Value: value})
		m := sourceXMLAttribute{NamespaceDeclaration: name.Space == "xmlns" || name.Space == "" && name.Local == "xmlns", Origin: origin}
		if r.captureAttributeLexical {
			m.Lexical = validation.Evidence[string]{State: validation.EvidenceAbsent}
		}
		metadata = append(metadata, m)
	}
	return attributes, metadata, nil
}

// normalizeDTDTokenized is XML 1.0 section 3.3.3's final non-CDATA step.
// Earlier processing has already normalized literal XML whitespace. Numeric
// references can still contribute tabs/newlines; only U+0020 is collapsed here.
func (r *sourceXMLTokenReader) normalizeDTDTokenized(value string) (string, error) {
	if err := r.cursor.contextErr(); err != nil {
		return "", err
	}
	if r.account == nil {
		return "", &validation.ConfigurationError{Field: "Account", Reason: "DTD normalization requires the shared assessment account"}
	}
	if err := r.checkScalar(int64(len(value))); err != nil {
		return "", err
	}
	if int64(len(value)) > (int64(^uint64(0)>>1)-64)/2 {
		return "", &validation.ResourceError{Kind: validation.BudgetBufferedBytes, Requested: int64(len(value))}
	}
	if err := r.tokenReserve(64 + 2*int64(len(value))); err != nil {
		return "", err
	}
	// Reserve the simultaneously live scratch and returned string before either
	// allocation. Their token-owned allowance is released on advance or Close.
	out := make([]byte, 0, len(value))
	space := false
	for at := 0; at < len(value); {
		if err := r.cursor.contextErr(); err != nil {
			return "", err
		}
		end := at + min(4096, len(value)-at)
		if err := r.account.Charge(validation.BudgetEvents, int64(end-at)); err != nil {
			return "", err
		}
		for i := at; i < end; i++ {
			b := value[i]
			if b == ' ' {
				space = len(out) != 0
				continue
			}
			if space {
				out = append(out, ' ')
				space = false
			}
			out = append(out, b)
		}
		at = end
	}
	return string(out), nil
}
