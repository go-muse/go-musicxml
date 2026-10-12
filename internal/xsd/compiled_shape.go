package xsd

// TypeContentShape is a conservative metadata inspection result, not a value or
// schema-validity judgment. A union preserves all member families; complex
// simple content additionally carries ShapeComplex. Unsupported or cyclic
// metadata has no shape. This is used only by sealed configuration binders.
type TypeContentShape uint8

const (
	ShapeElements TypeContentShape = 1 << iota
	ShapeString
	ShapeInteger
	ShapeDecimal
	ShapeUnion
	ShapeComplex
)

// ContentShape inspects at most 512 metadata edges and 128 nested type layers.
// Returned flags contain no mutable metadata. It never parses a lexical value,
// executes a validation run, or changes the compiled schema. Lists and primitive
// families outside exact integers/decimals/strings remain unavailable here.
func (t TypeID) ContentShape() (TypeContentShape, bool) {
	const maxDepth = 128
	remaining := 512
	active := make(map[TypeID]bool)
	memo := make(map[TypeID]TypeContentShape)
	var walk func(TypeID, int) (TypeContentShape, bool)
	walk = func(id TypeID, depth int) (TypeContentShape, bool) {
		if remaining == 0 || depth > maxDepth || id.schema == nil {
			return 0, false
		}
		remaining--
		if shape, ok := memo[id]; ok {
			return shape, true
		}
		if active[id] {
			return 0, false
		}
		active[id] = true
		defer delete(active, id)
		finish := func(shape TypeContentShape, ok bool) (TypeContentShape, bool) {
			if ok {
				memo[id] = shape
			}
			return shape, ok
		}
		if id.builtin != "" {
			if _, _, integer := IntegerLimits(id.builtin); integer {
				return finish(ShapeInteger, true)
			}
			switch id.builtin {
			case "decimal":
				return finish(ShapeDecimal, true)
			case "string", "normalizedString", "token", "language", "Name", "NCName", "NMTOKEN", "ID", "IDREF", "ENTITY":
				return finish(ShapeString, true)
			default:
				return 0, false
			}
		}
		base := func(ref *TypeRef) (TypeContentShape, bool) {
			next, ok := id.schema.typeID(ref)
			if !ok {
				return 0, false
			}
			return walk(next, depth+1)
		}
		member := func(m *SimpleMember) (TypeContentShape, bool) {
			if m == nil {
				return 0, false
			}
			return base(&TypeRef{Name: m.Name, InlineSimple: m.Inline})
		}
		if id.simple != nil && id.complex == nil {
			switch id.simple.Form {
			case SimpleRestriction:
				shape, ok := member(id.simple.Base)
				if shape&ShapeComplex != 0 {
					return 0, false
				}
				return finish(shape, ok)
			case SimpleUnion:
				if len(id.simple.Members) == 0 || len(id.simple.Members) > remaining {
					return 0, false
				}
				shape := ShapeUnion
				for _, m := range id.simple.Members {
					part, ok := member(m)
					if !ok || part&(ShapeComplex|ShapeElements) != 0 {
						return 0, false
					}
					shape |= part
				}
				return finish(shape, true)
			default:
				return 0, false
			}
		}
		if id.complex != nil && id.simple == nil {
			c := id.complex
			if c.Mixed {
				return 0, false
			}
			switch c.Form {
			case ComplexDirect:
				if c.Base != nil || c.SimpleContent != nil {
					return 0, false
				}
				return finish(ShapeComplex|ShapeElements, true)
			case ComplexComplexContentExtension:
				if c.SimpleContent != nil {
					return 0, false
				}
				shape, ok := base(c.Base)
				if !ok || shape != ShapeComplex|ShapeElements {
					return 0, false
				}
				return finish(shape, true)
			case ComplexSimpleContentExtension:
				if c.SimpleContent != nil || c.Particle != nil {
					return 0, false
				}
				shape, ok := base(c.Base)
				if !ok || shape&ShapeElements != 0 {
					return 0, false
				}
				return finish(shape|ShapeComplex, true)
			default:
				return 0, false
			}
		}
		return 0, false
	}
	return walk(t, 0)
}
