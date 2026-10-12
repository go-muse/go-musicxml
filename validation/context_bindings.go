package validation

import (
	"cmp"
	"slices"
	"strings"
	"sync"
)

// Roles and their specifications are private, fixed programs. The strings are
// never accepted from adapters, a profile caller, or document content.
type contextRole string

const contextRoleFigureNumber contextRole = "figure-number-numeric"

const contextRoleTimeOnly contextRole = "time-only"

type contextBindingSpec struct {
	Role                     contextRole
	Owner, Space, Name, Type string
	Attribute                bool
	// Count is the exact number of selected uses. An empty Owner selects
	// exact Name/Space/Type uses across owners; -1 accepts a nonempty set.
	Count int
}
type contextAttributeRole struct {
	role     contextRole
	selector AttributeSelector
}
type contextBindings struct {
	Elements       map[contextRole]map[DeclarationID]struct{}
	AttributeRoles map[DeclarationID][]contextAttributeRole
	Roles          map[contextRole][]SchemaUse
	Attributes     map[contextRole]map[ComponentID]AttributeSelector
	AttributeUses  map[contextRole]map[DeclarationID]AttributeSelector
	bytes          int64
}
type contextBindingsCache struct {
	once     sync.Once
	bindings *contextBindings
	err      error
}

func (schema *CompiledSchema) sharedContextBindings() (*contextBindings, error) {
	if schema == nil || schema.context == nil {
		return compileContextBindings(schema)
	}
	c := schema.context
	c.once.Do(func() { c.bindings, c.err = compileContextBindings(schema) })
	if e, ok := c.err.(*ConfigurationError); ok {
		copy := *e
		return c.bindings, &copy
	}
	return c.bindings, c.err
}
func contextNamedType(name string) ComponentID {
	if strings.HasPrefix(name, "xml:") {
		return ComponentID{Assembly: AssemblyMusicXML, Kind: ComponentType, Name: QName{Space: "http://www.w3.org/XML/1998/namespace", Local: strings.TrimPrefix(name, "xml:")}}
	}
	return timelineNamedType(name)
}
func contextAssemblyType(assembly Assembly, name string) ComponentID {
	t := contextNamedType(name)
	t.Assembly = assembly
	return t
}
func contextBindingSpecs() []contextBindingSpec {
	out := []contextBindingSpec{{Role: contextRoleTimeOnly, Name: "time-only", Type: "time-only", Attribute: true, Count: -1}}
	out = append(out, contextBindingSpec{Role: contextRoleFigureNumber, Owner: "figure", Name: "figure-number", Type: "style-text", Count: 1})
	out = append(out, contextLocalBindingSpecs()...)
	out = append(out, contextSpanBindingSpecs()...)
	out = append(out, contextRepeatBindingSpecs()...)
	out = append(out, contextPerformanceBindingSpecs()...)
	return out
}
func compileContextBindings(schema *CompiledSchema) (*contextBindings, error) {
	if schema != nil && schema.kernel != nil && schema.assembly == AssemblyOpus {
		return compileContextBindingSpecs(schema, contextOpusLayoutBindingSpecs())
	}
	if schema == nil || schema.kernel == nil || schema.assembly != AssemblyMusicXML {
		return nil, contract("context schema", "compiled MusicXML assembly required")
	}
	return compileContextBindingSpecs(schema, contextBindingSpecs())
}
func compileContextBindingSpecs(schema *CompiledSchema, specs []contextBindingSpec) (*contextBindings, error) {
	if schema == nil || schema.kernel == nil || (schema.assembly != AssemblyMusicXML && schema.assembly != AssemblyOpus) || len(specs) > 4096 {
		return nil, contract("context bindings", "bounded MusicXML roles required")
	}
	type useKey struct {
		owner     ComponentID
		name      QName
		attribute bool
	}
	index := make(map[useKey][]SchemaUse)
	uses := schema.Uses()
	for _, u := range uses {
		if u.OwnerTypeKnown {
			k := useKey{u.OwnerType, u.Name, u.Attribute}
			index[k] = append(index[k], u)
		}
	}
	b := &contextBindings{Elements: make(map[contextRole]map[DeclarationID]struct{}), AttributeRoles: make(map[DeclarationID][]contextAttributeRole), Roles: make(map[contextRole][]SchemaUse), Attributes: make(map[contextRole]map[ComponentID]AttributeSelector), AttributeUses: make(map[contextRole]map[DeclarationID]AttributeSelector), bytes: 512}
	for _, spec := range specs {
		if spec.Role == "" || spec.Name == "" || spec.Type == "" || (spec.Count < 1 && spec.Count != -1) || (spec.Count == -1 && spec.Owner != "") {
			return nil, contract("context binding spec", "exact fixed role or declared-type family required")
		}
		if _, exists := b.Roles[spec.Role]; exists {
			return nil, contract("context binding role", "duplicate role")
		}
		candidates := index[useKey{contextAssemblyType(schema.assembly, spec.Owner), QName{Space: spec.Space, Local: spec.Name}, spec.Attribute}]
		if spec.Owner == "" {
			candidates = nil
			for _, u := range uses {
				if u.Attribute == spec.Attribute && u.Name == (QName{Space: spec.Space, Local: spec.Name}) && u.DeclaredTypeKnown && u.DeclaredType == contextAssemblyType(schema.assembly, spec.Type) {
					candidates = append(candidates, u)
				}
			}
		}
		if len(candidates) == 0 || (spec.Count != -1 && len(candidates) != spec.Count) {
			return nil, contract("context binding "+spec.Owner+"/"+spec.Name, "exact use count mismatch")
		}
		bound := make([]SchemaUse, len(candidates))
		for i, u := range candidates {
			if !validDeclaration(u.Declaration) || !u.OwnerTypeKnown || !u.DeclaredTypeKnown || (spec.Type != "#anonymous" && u.DeclaredType != contextAssemblyType(schema.assembly, spec.Type)) || (spec.Type == "#anonymous" && (u.DeclaredType.Name != (QName{}) || u.DeclaredType.Occurrence == "")) {
				return nil, contract("context binding "+spec.Owner+"/"+spec.Name, "exact declared type unavailable or mismatched")
			}
			// Schema.Uses returns owned values; retain only this fixed selected table.
			bound[i] = u
			b.bytes += int64(640 + len(u.Name.Space) + len(u.Name.Local) + len(u.Declaration.OwnerOccurrence) + len(u.Declaration.Occurrence) + len(u.OwnerType.Occurrence) + len(u.DeclaredType.Occurrence) + len(u.OwnerType.Name.Space) + len(u.OwnerType.Name.Local) + len(u.DeclaredType.Name.Space) + len(u.DeclaredType.Name.Local))
			if b.bytes > 4<<20 {
				return nil, contract("context bindings", "finite configuration allowance exceeded")
			}
		}
		slices.SortFunc(bound, func(a, b SchemaUse) int { return compareLocalDeclaration(a.Declaration, b.Declaration) })
		b.Roles[spec.Role] = bound
		if spec.Attribute {
			owners := make(map[ComponentID]AttributeSelector)
			declarations := make(map[DeclarationID]AttributeSelector)
			for _, u := range bound {
				a := AttributeSelector{Name: u.Name, Declaration: u.Declaration}
				if old, ok := owners[u.OwnerType]; ok && old != a {
					return nil, contract("context attribute role", "ambiguous exact owner slot")
				}
				owners[u.OwnerType] = a
				declarations[u.Declaration] = a
				b.bytes += 512
			}
			for d, a := range declarations {
				b.AttributeRoles[d] = append(b.AttributeRoles[d], contextAttributeRole{spec.Role, a})
				b.bytes += 256 + contextDeclarationBytes(d) + int64(len(spec.Role))
			}
			b.Attributes[spec.Role] = owners
			b.AttributeUses[spec.Role] = declarations
		}
		if !spec.Attribute {
			elements := make(map[DeclarationID]struct{}, len(bound))
			for _, u := range bound {
				elements[u.Declaration] = struct{}{}
				b.bytes += 256 + contextDeclarationBytes(u.Declaration)
			}
			b.Elements[spec.Role] = elements
		}
		b.bytes += int64(128 + len(spec.Role))
		if b.bytes > 4<<20 {
			return nil, contract("context bindings", "finite configuration allowance exceeded")
		}
	}
	return b, nil
}
func (b *contextBindings) size() int64 {
	if b == nil {
		return 0
	}
	return b.bytes
}
func (b *contextBindings) digest() any {
	type entry struct {
		Role contextRole
		Uses []SchemaUse
	}
	if b == nil {
		return []entry(nil)
	}
	out := make([]entry, 0, len(b.Roles))
	for k, v := range b.Roles {
		out = append(out, entry{k, v})
	}
	slices.SortFunc(out, func(a, b entry) int { return cmp.Compare(a.Role, b.Role) })
	return out
}
func contextDeclarationBytes(d DeclarationID) int64 {
	return int64(1 + len(d.Assembly) + len(d.OwnerOccurrence) + len(d.Occurrence))
}
func contextComponentBytes(c ComponentID) int64 {
	return int64(1 + len(c.Assembly) + len(c.Name.Space) + len(c.Name.Local) + len(c.Occurrence))
}
func contextRoleOwned(r contextRole) contextRole { return contextRole(strings.Clone(string(r))) }
