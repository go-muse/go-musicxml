package musicxml

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// wellFormedXMLTokenReader fills two gaps in encoding/xml: duplicate
// attributes and DOCTYPE placement. It observes lexical tokens below every
// model/namespace skip, and forwards them unchanged so the wrapping decoder
// still expands namespaces exactly once and checks matching element names.
// It does not parse DTD declarations or load external resources.
type wellFormedXMLTokenReader struct {
	source     xml.TokenReader
	started    bool
	doctype    bool
	namespaces map[string]string
	scopes     [][]xmlNamespaceUndo
}

type xmlNamespaceUndo struct {
	prefix string
	uri    string
	bound  bool
}

// An undeclared prefix is not a namespace URI, even if a different attribute
// explicitly binds a namespace whose literal URI happens to equal it. Leave
// existing undeclared-prefix handling to the wrapping decoder.
type expandedXMLAttribute struct {
	name    xml.Name
	unbound bool
}

func (r *wellFormedXMLTokenReader) Token() (xml.Token, error) {
	token, err := r.source.Token()
	if err != nil {
		return nil, err
	}

	switch value := token.(type) {
	case xml.StartElement:
		r.started = true
		if err := r.checkAttributes(value.Attr); err != nil {
			return nil, err
		}
	case xml.EndElement:
		if len(r.scopes) > 0 {
			last := len(r.scopes) - 1
			for _, previous := range r.scopes[last] {
				if previous.bound {
					r.namespaces[previous.prefix] = previous.uri
				} else {
					delete(r.namespaces, previous.prefix)
				}
			}
			r.scopes[last] = nil
			r.scopes = r.scopes[:last]
		}
	case xml.Directive:
		// Match the keyword, not a substring in another directive. The
		// underlying XML reader handles quoting and the internal subset.
		directive := string(value)
		if directive == "DOCTYPE" || strings.HasPrefix(directive, "DOCTYPE ") ||
			strings.HasPrefix(directive, "DOCTYPE\t") || strings.HasPrefix(directive, "DOCTYPE\r") ||
			strings.HasPrefix(directive, "DOCTYPE\n") {
			if r.started {
				return nil, fmt.Errorf("musicxml: DOCTYPE must precede the root element")
			}
			if r.doctype {
				return nil, fmt.Errorf("musicxml: multiple DOCTYPE declarations")
			}
			r.doctype = true
		}
	}
	return token, nil
}

func (r *wellFormedXMLTokenReader) checkAttributes(attributes []xml.Attr) error {
	lexical := make(map[xml.Name]struct{}, len(attributes))
	var undo []xmlNamespaceUndo
	for _, attribute := range attributes {
		if _, duplicate := lexical[attribute.Name]; duplicate {
			return fmt.Errorf("musicxml: duplicate XML attribute {%s}%s", attribute.Name.Space, attribute.Name.Local)
		}
		lexical[attribute.Name] = struct{}{}
		if attribute.Name.Space == "xmlns" {
			if r.namespaces == nil {
				r.namespaces = make(map[string]string)
			}
			prefix := attribute.Name.Local
			previous, bound := r.namespaces[prefix]
			undo = append(undo, xmlNamespaceUndo{prefix: prefix, uri: previous, bound: bound})
			r.namespaces[prefix] = attribute.Value
		}
	}
	r.scopes = append(r.scopes, undo)

	// All declarations on this element apply to every attribute on it,
	// regardless of lexical order. Default namespaces never apply to
	// unprefixed attributes. Namespace declarations are checked lexically
	// above, separately from ordinary attributes in a URI named "xmlns".
	expanded := make(map[expandedXMLAttribute]struct{}, len(attributes))
	for _, attribute := range attributes {
		name := attribute.Name
		if name.Space == "xmlns" || (name.Space == "" && name.Local == "xmlns") {
			continue
		}
		key := expandedXMLAttribute{name: name}
		if name.Space == "xml" {
			key.name.Space = "http://www.w3.org/XML/1998/namespace"
		} else if name.Space != "" {
			if uri, bound := r.namespaces[name.Space]; bound {
				key.name.Space = uri
			} else {
				key.unbound = true
			}
		}
		if _, duplicate := expanded[key]; duplicate {
			return fmt.Errorf("musicxml: duplicate XML attribute {%s}%s", key.name.Space, key.name.Local)
		}
		expanded[key] = struct{}{}
	}
	return nil
}
