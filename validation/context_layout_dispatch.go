package validation

import "strconv"

// Dispatch uses only the exact, admitted binding roles returned by the shared
// reader. Attribute owners and element declarations are independent axes. These
// finite tables retain no per-node state and do not infer roles from input names.
type layoutFamilyMask uint8

const (
	layoutStateFamily layoutFamilyMask = 1 << iota
	layoutStaticFamily
	layoutFormatFamily
)

type layoutNodeDispatch struct {
	families  layoutFamilyMask
	constants uint64
}

const layoutConstantDispatchCapacity = 64

var layoutElementDispatch = func() map[contextRole]layoutFamilyMask {
	out := make(map[contextRole]layoutFamilyMask)
	for _, name := range []string{"note", "clef", "key", "for-part", "line-detail", "staff-details", "staff-size", "rest", "stem", "unpitched", "measure-numbering", "string", "attributes", "part-symbol", "dot"} {
		out[layoutStateRole(name)] |= layoutStateFamily
	}
	for _, name := range []string{"staff-layout", "page-layout", "system-layout", "measure-layout", "print", "group-name", "group-abbreviation", "dynamics", "lyric", "elision", "tap", "harmony", "kind", "arrowhead", "page-margins", "scaling"} {
		out[layoutRole(name)] |= layoutStaticFamily
	}
	for _, spec := range layoutElements {
		if spec.typ == "tenths" {
			out[layoutRole(spec.name)] |= layoutStaticFamily
		}
	}
	return out
}()

var layoutAttributeDispatch = func() map[contextRole]layoutFamilyMask {
	out := map[contextRole]layoutFamilyMask{
		layoutStateRole("accord.@string"):       layoutStateFamily,
		layoutStateRole("measure.@implicit"):    layoutStateFamily,
		layoutRole("@system-mark"):              layoutStaticFamily,
		layoutRole("measure-numbering.@system"): layoutStaticFamily,
	}
	for _, name := range []string{"default-x", "default-y", "relative-x", "relative-y", "bezier-x", "bezier-y", "bezier-x2", "bezier-y2", "bezier-offset", "bezier-offset2"} {
		out[layoutRole("@"+name)] |= layoutStaticFamily
	}
	for _, name := range []string{"font-family", "justify", "valign"} {
		out[layoutRole("@"+name)] |= layoutFormatFamily
	}
	return out
}()

// Constant roles are generated from the same fixed rule order as their binding
// specs. The runtime checks capacity before reading any node, so an enlarged
// table can never silently lose a constant through a truncated shift.
var layoutConstantDispatch = func() [2]map[contextRole]uint64 {
	out := [2]map[contextRole]uint64{make(map[contextRole]uint64), make(map[contextRole]uint64)}
	for i, rule := range layoutConstantRules {
		role := layoutRole("constant." + strconv.Itoa(i) + ".attribute")
		axis := 1
		if rule.Element != "" {
			role = layoutRole("constant." + strconv.Itoa(i) + ".element")
			axis = 0
		}
		out[axis][role] = uint64(1) << uint(i)
	}
	return out
}()

func layoutDispatchRoles(elements, attributes []contextRole) layoutNodeDispatch {
	var out layoutNodeDispatch
	// The shared reader has charged exact identity lookup and each returned role
	// key before these fixed-size dispatch lookups.
	for _, role := range elements {
		out.families |= layoutElementDispatch[role]
		out.constants |= layoutConstantDispatch[0][role]
	}
	for _, role := range attributes {
		out.families |= layoutAttributeDispatch[role]
		out.constants |= layoutConstantDispatch[1][role]
	}
	return out
}

func (d layoutNodeDispatch) has(family layoutFamilyMask) bool { return d.families&family != 0 }
