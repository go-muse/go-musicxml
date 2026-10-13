package validation

const (
	planContextMusicXMLLinks  planKind = "musicxml-local-links"
	planContextOpusLinks      planKind = "opus-local-links"
	planContextMusicXMLHints  planKind = "musicxml-schema-hints"
	planContextOpusHints      planKind = "opus-schema-hints"
	planContextContainerHints planKind = "container-schema-hints"
	planContextSoundsHints    planKind = "sounds-schema-hints"
)

// NewLocalLinkContextPlan constructs exact MusicXML or Opus local resource
// projections. It performs no I/O, URI resolution, package parsing or traversal.
func NewLocalLinkContextPlan(schema *CompiledSchema) (Plan, error) {
	if schema == nil || schema.kernel == nil || (schema.assembly != AssemblyMusicXML && schema.assembly != AssemblyOpus) {
		return Plan{}, contract("local link schema", "compiled MusicXML or Opus assembly required")
	}
	kind := planContextMusicXMLLinks
	if schema.assembly == AssemblyOpus {
		kind = planContextOpusLinks
	}
	bindings, err := compileContextBindingSpecs(schema, resourceLinkBindingSpecs(schema.assembly))
	if err != nil {
		return Plan{}, err
	}
	return checkedPlan(Plan{kind: kind, context: &contextPlan{Schema: schema, Bindings: bindings, revision: contextSemanticRevision}})
}

// NewSchemaHintContextPlan projects standard source xsi discovery hints for one
// exact selected schema assembly. It does not load or select hinted schemas.
func NewSchemaHintContextPlan(schema *CompiledSchema) (Plan, error) {
	if schema == nil || schema.kernel == nil {
		return Plan{}, contract("schema hints", "compiled schema required")
	}
	var kind planKind
	switch schema.assembly {
	case AssemblyMusicXML:
		kind = planContextMusicXMLHints
	case AssemblyOpus:
		kind = planContextOpusHints
	case AssemblyContainer:
		kind = planContextContainerHints
	case AssemblySounds:
		kind = planContextSoundsHints
	default:
		return Plan{}, contract("schema hints", "exact supported assembly required")
	}
	b := &contextBindings{assembly: schema.assembly, requiredSchemaDigest: schema.Digest(), bindingDigest: "standard-xsi-schema-hints/revision-1", bytes: 256}
	return checkedPlan(Plan{kind: kind, context: &contextPlan{Schema: schema, Bindings: b, revision: contextSemanticRevision}})
}
func resourceHintPlan(kind planKind) bool {
	switch kind {
	case planContextMusicXMLHints, planContextOpusHints, planContextContainerHints, planContextSoundsHints:
		return true
	}
	return false
}
func resourceLinkPlan(kind planKind) bool {
	return kind == planContextMusicXMLLinks || kind == planContextOpusLinks
}

func resourceRole(name string) contextRole { return contextRole("resource." + name) }
func resourceLinkBindingSpecs(assembly Assembly) []contextBindingSpec {
	var out []contextBindingSpec
	element := func(name, typ string) {
		out = append(out, contextBindingSpec{Role: resourceRole(name), Name: name, Type: typ, Count: -1})
	}
	attribute := func(owner, space, name, typ string) {
		out = append(out, contextBindingSpec{Role: resourceRole(owner + ".@" + name), Owner: owner, Space: space, Name: name, Type: typ, Attribute: true, Count: 1})
	}
	owners := []string{"score", "opus-link"}
	if assembly == AssemblyMusicXML {
		owners = []string{"link", "part-link", "opus"}
		for _, name := range []string{"link", "bookmark", "part-link", "opus", "instrument-link", "score-part", "score-instrument"} {
			element(name, name)
		}
		element("image", "image")
		element("credit-image", "image")
		element("group", "xs:string")
		element("group-link", "xs:string")
		for _, owner := range []string{"link", "bookmark"} {
			attribute(owner, "", "element", "xs:NMTOKEN")
			attribute(owner, "", "position", "xs:positiveInteger")
		}
		attribute("instrument-link", "", "id", "xs:IDREF")
		for _, spec := range []struct{ name, typ string }{{"source", "xs:anyURI"}, {"type", "xs:token"}, {"width", "tenths"}, {"height", "tenths"}} {
			attribute("image", "", spec.name, spec.typ)
		}
	} else {
		element("score", "score")
		element("opus-link", "opus-link")
	}
	for _, owner := range owners {
		attribute(owner, "http://www.w3.org/1999/xlink", "href", "xs:anyURI")
		attribute(owner, "http://www.w3.org/1999/xlink", "show", "#anonymous")
		attribute(owner, "http://www.w3.org/1999/xlink", "actuate", "#anonymous")
	}
	return out
}
