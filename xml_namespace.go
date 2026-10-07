package musicxml

import "encoding/xml"

// encoding/xml treats a struct tag without a namespace as a local-name match
// in every namespace. MusicXML 4.0 and MXL container elements are unqualified. Filter
// extensions before they can overwrite an ordinary generated field. The
// underlying decoder still sees every token and enforces XML depth limits.
type namespaceXMLTokenReader struct {
	source                  *xml.Decoder
	allowQualifiedAttribute func(element string, name xml.Name) bool
	started                 bool
	depth                   int
}

func (r *namespaceXMLTokenReader) Token() (xml.Token, error) {
	for {
		token, err := r.source.Token()
		if err != nil {
			return nil, err
		}
		// Leave the document tail visible to readDocumentTail, including
		// foreign-namespace elements that would be ignored inside the root.
		if r.started && r.depth == 0 {
			return token, nil
		}
		switch value := token.(type) {
		case xml.StartElement:
			if r.started && value.Name.Space != "" {
				if err := r.source.Skip(); err != nil {
					return nil, err
				}
				continue
			}
			r.started = true
			r.depth++
			value.Attr = r.attributes(value.Name.Local, value.Attr)
			return value, nil
		case xml.EndElement:
			r.depth--
		}
		return token, nil
	}
}

func (r *namespaceXMLTokenReader) attributes(element string, attributes []xml.Attr) []xml.Attr {
	retained := attributes[:0]
	for _, attribute := range attributes {
		if attribute.Name.Space == "" {
			if attribute.Name.Local == "xmlns" {
				continue
			}
		} else if r.allowQualifiedAttribute == nil || !r.allowQualifiedAttribute(element, attribute.Name) {
			// Names have already been expanded by source, so namespace
			// declarations are no longer needed by the model decoder.
			continue
		}
		retained = append(retained, attribute)
	}
	return retained
}

func musicXMLQualifiedAttribute(element string, name xml.Name) bool {
	switch name.Space {
	case "http://www.w3.org/XML/1998/namespace":
		return name.Local == "lang" || name.Local == "space"
	case "http://www.w3.org/1999/xlink":
		// Only these MusicXML elements use the XLink attribute group.
		// Keeping xlink:type on an unrelated element would still match
		// its unqualified type field in encoding/xml.
		switch element {
		case "link", "opus", "part-link", "opus-link", "score":
		default:
			return false
		}
		switch name.Local {
		case "href", "type", "role", "title", "show", "actuate":
			return true
		}
	}
	return false
}
